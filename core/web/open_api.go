package web

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/allbot/allbot/core/config"
	plugincore "github.com/allbot/allbot/core/plugin"
	"github.com/allbot/allbot/core/types"
)

const openAPIPrefix = "/api/open/"
const openAPIStorageDir = "openapis"
const openAPIConfigFile = "config.json"

type openAPIAdminRequest struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Path           string    `json:"path"`
	Method         string    `json:"method"`
	Enabled        bool      `json:"enabled"`
	Token          string    `json:"token"`
	Runtime        string    `json:"runtime"`
	RuntimeProfile string    `json:"runtime_profile"`
	Entry          string    `json:"entry"`
	Description    string    `json:"description"`
	Builtin        string    `json:"builtin"`
	IPWhitelist    *[]string `json:"-"`
	Script         *string   `json:"script"`
	Code           *string   `json:"code"`
}

type openAPIStatusWriter struct {
	http.ResponseWriter
	status int
}

func (w *openAPIStatusWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *openAPIStatusWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func (w *openAPIStatusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (s *Server) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	startedAt := time.Now()
	openPath := strings.Trim(strings.TrimPrefix(r.URL.Path, openAPIPrefix), "/")
	if openPath == "" {
		s.jsonError(w, "Open API 路径不能为空", http.StatusNotFound)
		return
	}
	endpoint, err := s.matchOpenAPIEndpoint(openPath, r.Method)
	if err != nil {
		s.jsonError(w, "读取 Open API 配置失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if endpoint == nil {
		s.jsonError(w, "Open API 不存在或未启用", http.StatusNotFound)
		return
	}

	statusWriter := &openAPIStatusWriter{ResponseWriter: w}
	clientAddr := s.resolveOpenAPIClientIP(r)
	clientIP := ""
	if clientAddr.IsValid() {
		clientIP = clientAddr.String()
	}
	outcome := config.OpenAPICallOutcomeFailed
	defer func() {
		status := statusWriter.status
		if status == 0 {
			status = http.StatusOK
		}
		if outcome != config.OpenAPICallOutcomeIPDenied && outcome != config.OpenAPICallOutcomeTokenDenied {
			if status >= 100 && status <= 399 {
				outcome = config.OpenAPICallOutcomeSuccess
			} else {
				outcome = config.OpenAPICallOutcomeFailed
			}
		}
		s.recordOpenAPICall(config.OpenAPICallLog{
			EndpointID: endpoint.ID, EndpointName: endpoint.Name, Method: r.Method, RequestPath: r.URL.Path,
			ClientIP: clientIP, StatusCode: status, Outcome: outcome, DurationMS: time.Since(startedAt).Milliseconds(), StartedAt: startedAt,
		})
	}()

	if !s.openAPIIPAllowed(endpoint, clientAddr) {
		outcome = config.OpenAPICallOutcomeIPDenied
		logOpenAPICall("WARN", endpoint.ID, r.Method, r.URL.Path, clientIP, http.StatusForbidden, startedAt, "IP 不在白名单")
		s.jsonError(statusWriter, "客户端 IP 不在 Open API 白名单", http.StatusForbidden)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.jsonError(statusWriter, "读取请求体失败", http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	requestData, tokenSources := buildOpenAPIRequest(r, openPath, body, clientAddr)
	if !openAPITokenMatched(endpoint.Token, tokenSources) {
		outcome = config.OpenAPICallOutcomeTokenDenied
		logOpenAPICall("WARN", endpoint.ID, r.Method, requestData.RawPath, clientIP, http.StatusUnauthorized, startedAt, "token 无效")
		s.jsonError(statusWriter, "Open API token 无效", http.StatusUnauthorized)
		return
	}
	requestData = sanitizeOpenAPIRequest(requestData, tokenSources)
	if endpoint.Builtin != "" {
		s.handleBuiltinOpenAPI(statusWriter, r, endpoint, requestData, startedAt)
		return
	}
	if s.pluginManager == nil {
		s.jsonError(statusWriter, "Open API 执行器不可用", http.StatusInternalServerError)
		return
	}
	log.Printf("[INFO] OpenAPI 调用开始 endpoint=%s method=%s path=%s client=%s body=%dB", endpoint.ID, requestData.Method, requestData.RawPath, requestData.ClientIP, len(body))
	response, err := s.pluginManager.ExecuteOpenAPI(
		*endpoint,
		openAPIEndpointDir(endpoint.ID),
		requestData,
		s.openAPIDBExecutor(),
		s.openAPISendMessageExecutor(),
		plugincore.OpenAPIExecutors{
			SendRichMessage: s.openAPISendRichMessageExecutor(),
			SendImage:       s.openAPISendImageMessageExecutor(),
		},
	)
	if err != nil {
		logOpenAPICall("ERROR", endpoint.ID, requestData.Method, requestData.RawPath, requestData.ClientIP, http.StatusInternalServerError, startedAt, "执行失败: "+err.Error())
		s.jsonError(statusWriter, "Open API 执行失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeOpenAPIResponse(statusWriter, response)
}

func logOpenAPICall(level, endpoint, method, path, client string, status int, startedAt time.Time, message string) {
	log.Printf("[%s] OpenAPI %s endpoint=%s method=%s path=%s client=%s status=%d cost=%s", level, message, endpoint, method, path, client, status, time.Since(startedAt).Round(time.Millisecond))
}

func (s *Server) handleOpenAPIConfigs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := listOpenAPIEndpoints()
		if err != nil {
			s.jsonError(w, "获取 Open API 列表失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if s.openAPIStats != nil {
			if err := s.openAPIStats.Flush(); err != nil {
				s.jsonError(w, "刷新 Open API 调用统计失败: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
		ids := make([]string, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.ID)
		}
		stats := map[string]config.OpenAPICallStat{}
		if database := s.runtimeDatabase(); database != nil {
			stats, err = database.GetOpenAPICallStats(ids)
			if err != nil {
				s.jsonError(w, "获取 Open API 调用统计失败: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
		result := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			result = append(result, s.openAPIAdminResponse(item, "", stats[item.ID]))
		}
		s.jsonResponse(w, result)
	case http.MethodPost:
		s.saveOpenAPIFromRequest(w, r, "")
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleOpenAPIConfigDetail(w http.ResponseWriter, r *http.Request) {
	if isOpenAPIAdminCollectionPath(r.URL.Path) {
		s.handleOpenAPIConfigs(w, r)
		return
	}
	id, action, ok := parseOpenAPIAdminPath(r.URL.Path)
	if !ok {
		s.jsonError(w, "Open API 管理路径无效", http.StatusNotFound)
		return
	}
	if action == "settings" {
		s.handleOpenAPISettings(w, r)
		return
	}
	if _, err := normalizeOpenAPIID(id); err != nil {
		s.jsonError(w, "Open API ID 无效: "+err.Error(), http.StatusBadRequest)
		return
	}
	if action == "code" {
		s.handleOpenAPICode(w, r, id)
		return
	}
	if action == "calls" {
		s.handleOpenAPICalls(w, r, id)
		return
	}
	switch r.Method {
	case http.MethodGet:
		endpoint, err := loadOpenAPIEndpoint(id)
		if err != nil {
			if os.IsNotExist(err) {
				s.jsonError(w, "Open API 不存在", http.StatusNotFound)
				return
			}
			s.jsonError(w, "读取 Open API 失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if s.openAPIStats != nil {
			if statsErr := s.openAPIStats.Flush(); statsErr != nil {
				s.jsonError(w, "刷新 Open API 调用统计失败: "+statsErr.Error(), http.StatusInternalServerError)
				return
			}
		}
		stat := config.OpenAPICallStat{}
		if database := s.runtimeDatabase(); database != nil {
			stats, statsErr := database.GetOpenAPICallStats([]string{id})
			if statsErr != nil {
				s.jsonError(w, "获取 Open API 调用统计失败: "+statsErr.Error(), http.StatusInternalServerError)
				return
			}
			stat = stats[id]
		}
		s.jsonResponse(w, s.openAPIAdminResponse(endpoint, "", stat))
	case http.MethodPut:
		s.saveOpenAPIFromRequest(w, r, id)
	case http.MethodDelete:
		if _, err := loadOpenAPIEndpoint(id); err != nil {
			if os.IsNotExist(err) {
				s.jsonError(w, "Open API 不存在", http.StatusNotFound)
			} else {
				s.jsonError(w, "读取 Open API 失败: "+err.Error(), http.StatusInternalServerError)
			}
			return
		}
		database := s.runtimeDatabase()
		if s.openAPIStats != nil {
			if err := s.openAPIStats.Flush(); err != nil {
				s.jsonError(w, "删除 Open API 前刷新统计失败: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
		if err := removeOpenAPIEndpoint(id); err != nil {
			s.jsonError(w, "删除 Open API 失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if database != nil {
			if err := database.DeleteOpenAPICallData(id); err != nil {
				s.jsonError(w, "Open API 已删除，但删除对应统计失败: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
		s.jsonResponse(w, map[string]interface{}{"message": "Open API 已删除"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func isOpenAPIAdminCollectionPath(path string) bool {
	return path == "/api/open-apis/" || path == "/api/openapis/"
}

func parseOpenAPIAdminPath(path string) (string, string, bool) {
	var rest string
	switch {
	case strings.HasPrefix(path, "/api/open-apis/"):
		rest = strings.Trim(strings.TrimPrefix(path, "/api/open-apis/"), "/")
	case strings.HasPrefix(path, "/api/openapis/"):
		rest = strings.Trim(strings.TrimPrefix(path, "/api/openapis/"), "/")
	default:
		return "", "", false
	}
	if rest == "settings" {
		return "", "settings", true
	}
	if rest == "" {
		return "", "", false
	}
	parts := strings.Split(rest, "/")
	if len(parts) == 1 {
		id, err := url.PathUnescape(parts[0])
		return id, "", err == nil && id != ""
	}
	if len(parts) == 2 && (parts[1] == "code" || parts[1] == "calls") {
		id, err := url.PathUnescape(parts[0])
		return id, parts[1], err == nil && id != ""
	}
	return "", "", false
}

func (s *Server) handleOpenAPISettings(w http.ResponseWriter, r *http.Request) {
	database := s.runtimeDatabase()
	if database == nil {
		s.jsonError(w, "数据库不可用", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		settings, err := database.GetOpenAPISettings()
		if err != nil {
			s.jsonError(w, "获取 Open API 设置失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, openAPISettingsResponse(settings))
	case http.MethodPut:
		var request struct {
			IPWhitelist         *[]string `json:"ip_whitelist"`
			TrustedProxies      *[]string `json:"trusted_proxies"`
			CallLogRetentionDay *int      `json:"call_log_retention_days"`
			RetentionDays       *int      `json:"retention_days"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			s.jsonError(w, "请求数据无效", http.StatusBadRequest)
			return
		}
		settings, err := database.GetOpenAPISettings()
		if err != nil {
			s.jsonError(w, "获取 Open API 设置失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if request.IPWhitelist != nil {
			settings.IPWhitelist = *request.IPWhitelist
		}
		if request.TrustedProxies != nil {
			settings.TrustedProxies = *request.TrustedProxies
		}
		if request.CallLogRetentionDay != nil {
			settings.RetentionDays = *request.CallLogRetentionDay
		} else if request.RetentionDays != nil {
			settings.RetentionDays = *request.RetentionDays
		}
		compiled, err := compileOpenAPIAccessConfig(settings)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		settings.IPWhitelist = append([]string(nil), compiled.global.raw...)
		settings.TrustedProxies = append([]string(nil), compiled.trustedProxies.raw...)
		if err := database.SaveOpenAPISettings(settings); err != nil {
			s.jsonError(w, "保存 Open API 设置失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		compiled.globalSource = "global"
		s.openAPIAccess.Store(compiled)
		s.jsonResponse(w, openAPISettingsResponse(settings))
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func openAPISettingsResponse(settings config.OpenAPISettings) map[string]interface{} {
	return map[string]interface{}{
		"ip_whitelist": settings.IPWhitelist, "trusted_proxies": settings.TrustedProxies,
		"call_log_retention_days": settings.RetentionDays, "retention_days": settings.RetentionDays,
	}
}

func (s *Server) handleOpenAPICalls(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, err := loadOpenAPIEndpoint(id); err != nil {
		if os.IsNotExist(err) {
			s.jsonError(w, "Open API 不存在", http.StatusNotFound)
		} else {
			s.jsonError(w, "读取 Open API 失败: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}
	database := s.runtimeDatabase()
	if database == nil {
		s.jsonError(w, "数据库不可用", http.StatusServiceUnavailable)
		return
	}
	filter, err := parseOpenAPICallFilter(r, id)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if s.openAPIStats != nil {
		if err := s.openAPIStats.Flush(); err != nil {
			s.jsonError(w, "刷新 Open API 调用统计失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
	items, total, err := database.ListOpenAPICallLogs(filter)
	if err != nil {
		s.jsonError(w, "获取 Open API 调用明细失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	stats, err := database.GetOpenAPICallStats([]string{id})
	if err != nil {
		s.jsonError(w, "获取 Open API 调用统计失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	mapped := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		mapped = append(mapped, map[string]interface{}{
			"id": item.ID, "endpoint_id": item.EndpointID, "endpoint_name": item.EndpointName,
			"method": item.Method, "request_path": item.RequestPath, "endpoint_path": item.RequestPath,
			"client_ip": item.ClientIP, "status_code": item.StatusCode, "outcome": item.Outcome,
			"duration_ms": item.DurationMS, "started_at": item.StartedAt,
		})
	}
	s.jsonResponse(w, map[string]interface{}{"summary": stats[id], "total": total, "retention_days": s.openAPIRetentionDays(), "items": mapped})
}

func parseOpenAPICallFilter(r *http.Request, endpointID string) (config.OpenAPICallLogFilter, error) {
	query := r.URL.Query()
	filter := config.OpenAPICallLogFilter{EndpointID: endpointID, Limit: 50}
	var err error
	if value := query.Get("limit"); value != "" {
		filter.Limit, err = strconv.Atoi(value)
		if err != nil || filter.Limit < 1 || filter.Limit > 200 {
			return filter, fmt.Errorf("limit 必须是 1 到 200 的整数")
		}
	}
	if value := query.Get("offset"); value != "" {
		filter.Offset, err = strconv.Atoi(value)
		if err != nil || filter.Offset < 0 {
			return filter, fmt.Errorf("offset 必须是非负整数")
		}
	}
	filter.Outcome = strings.TrimSpace(query.Get("outcome"))
	if filter.Outcome != "" && filter.Outcome != config.OpenAPICallOutcomeSuccess && filter.Outcome != config.OpenAPICallOutcomeIPDenied && filter.Outcome != config.OpenAPICallOutcomeTokenDenied && filter.Outcome != config.OpenAPICallOutcomeFailed {
		return filter, fmt.Errorf("outcome 无效")
	}
	filter.ClientIP = strings.TrimSpace(query.Get("client_ip"))
	if filter.ClientIP != "" {
		address := parseOpenAPIAddress(filter.ClientIP)
		if !address.IsValid() {
			return filter, fmt.Errorf("client_ip 无效")
		}
		filter.ClientIP = address.String()
	}
	if value := query.Get("status_code"); value != "" {
		filter.StatusCode, err = strconv.Atoi(value)
		if err != nil || filter.StatusCode < 100 || filter.StatusCode > 999 {
			return filter, fmt.Errorf("status_code 必须是 100 到 999 的整数")
		}
	}
	if filter.StartedFrom, err = parseOpenAPITimeQuery(query.Get("start")); err != nil {
		return filter, fmt.Errorf("start 无效: %w", err)
	}
	if filter.StartedTo, err = parseOpenAPITimeQuery(query.Get("end")); err != nil {
		return filter, fmt.Errorf("end 无效: %w", err)
	}
	if filter.StartedFrom != nil && filter.StartedTo != nil && filter.StartedFrom.After(*filter.StartedTo) {
		return filter, fmt.Errorf("start 不能晚于 end")
	}
	return filter, nil
}

func parseOpenAPITimeQuery(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05"} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return &parsed, nil
		}
	}
	return nil, fmt.Errorf("必须使用 RFC3339 或 YYYY-MM-DD HH:mm:ss")
}

func (s *Server) handleOpenAPICode(w http.ResponseWriter, r *http.Request, id string) {
	endpoint, err := loadOpenAPIEndpoint(id)
	if err != nil {
		if os.IsNotExist(err) {
			s.jsonError(w, "Open API 不存在", http.StatusNotFound)
			return
		}
		s.jsonError(w, "读取 Open API 失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if endpoint.Builtin != "" {
		s.jsonError(w, "内置 Open API 没有可编辑脚本", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		code, err := readOpenAPIScript(endpoint)
		if err != nil {
			s.jsonError(w, "读取 Open API 代码失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"id": endpoint.ID, "runtime": endpoint.Runtime, "runtime_profile": endpoint.RuntimeProfile, "entry": endpoint.Entry, "file": endpoint.Entry, "code": code, "content": code})
	case http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			s.jsonError(w, "读取请求失败", http.StatusBadRequest)
			return
		}
		var req struct {
			Code           string `json:"code"`
			Content        string `json:"content"`
			Runtime        string `json:"runtime"`
			RuntimeProfile string `json:"runtime_profile"`
			File           string `json:"file"`
			Entry          string `json:"entry"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			s.jsonError(w, "请求数据无效", http.StatusBadRequest)
			return
		}
		fields := map[string]bool{}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err == nil {
			for key := range raw {
				fields[key] = true
			}
		}
		if strings.TrimSpace(req.Runtime) != "" {
			endpoint.Runtime = req.Runtime
		}
		if fields["runtime_profile"] {
			endpoint.RuntimeProfile = req.RuntimeProfile
		}
		if strings.TrimSpace(req.Entry) != "" {
			endpoint.Entry = req.Entry
		} else if strings.TrimSpace(req.File) != "" {
			endpoint.Entry = req.File
		}
		if err := s.validateOpenAPIRuntimeProfile(endpoint); err != nil {
			s.jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		code := req.Code
		if code == "" && req.Content != "" {
			code = req.Content
		}
		saved, err := saveOpenAPIEndpoint(*endpoint, &code)
		if err != nil {
			s.jsonError(w, "保存 Open API 代码失败: "+err.Error(), http.StatusBadRequest)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"id": saved.ID, "runtime": saved.Runtime, "runtime_profile": saved.RuntimeProfile, "entry": saved.Entry, "file": saved.Entry, "code": code, "content": code})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) saveOpenAPIFromRequest(w http.ResponseWriter, r *http.Request, pathID string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.jsonError(w, "读取请求失败", http.StatusBadRequest)
		return
	}
	var req openAPIAdminRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.jsonError(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	fields := map[string]bool{}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err == nil {
		for key := range raw {
			fields[key] = true
		}
		if value, ok := raw["ip_whitelist"]; ok {
			if string(value) == "null" {
				req.IPWhitelist = nil
			} else {
				var rules []string
				if err := json.Unmarshal(value, &rules); err != nil {
					s.jsonError(w, "ip_whitelist 必须是数组或 null", http.StatusBadRequest)
					return
				}
				req.IPWhitelist = &rules
			}
		}
	}

	var endpoint types.OpenAPIEndpoint
	if pathID != "" {
		existing, err := loadOpenAPIEndpoint(pathID)
		if err != nil && !os.IsNotExist(err) {
			s.jsonError(w, "读取 Open API 失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if existing != nil {
			endpoint = *existing
		}
		endpoint.ID = pathID
		if req.ID != "" && req.ID != pathID {
			s.jsonError(w, "Open API ID 与路径不一致", http.StatusBadRequest)
			return
		}
	} else {
		endpoint.ID = req.ID
	}
	applyOpenAPIRequestFields(&endpoint, req, fields)
	if endpoint.IPWhitelist != nil {
		rules, err := compileOpenAPIIPRules(*endpoint.IPWhitelist, true, true)
		if err != nil {
			s.jsonError(w, "Open API IP 白名单无效: "+err.Error(), http.StatusBadRequest)
			return
		}
		normalized := append([]string(nil), rules.raw...)
		endpoint.IPWhitelist = &normalized
	}
	if fields["path"] && !validOpenAPIRawPath(req.Path) {
		s.jsonError(w, "Open API 路径只能输入单个词，且只能包含字母、数字、横线和下划线", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(endpoint.Token) == "" {
		s.jsonError(w, "Open API token 不能为空", http.StatusBadRequest)
		return
	}
	if err := s.validateOpenAPIRuntimeProfile(&endpoint); err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	script := openAPIRequestScript(req, fields)
	saved, err := saveOpenAPIEndpoint(endpoint, script)
	if err != nil {
		s.jsonError(w, "保存 Open API 失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	stat := config.OpenAPICallStat{}
	if database := s.runtimeDatabase(); database != nil {
		stats, statsErr := database.GetOpenAPICallStats([]string{saved.ID})
		if statsErr != nil {
			s.jsonError(w, "读取 Open API 调用统计失败: "+statsErr.Error(), http.StatusInternalServerError)
			return
		}
		stat = stats[saved.ID]
	}
	s.jsonResponse(w, s.openAPIAdminResponse(saved, "", stat))
}

func applyOpenAPIRequestFields(endpoint *types.OpenAPIEndpoint, req openAPIAdminRequest, fields map[string]bool) {
	if fields["id"] && endpoint.ID == "" {
		endpoint.ID = req.ID
	}
	if fields["name"] {
		endpoint.Name = req.Name
	}
	if fields["path"] {
		endpoint.Path = req.Path
	}
	if fields["method"] {
		endpoint.Method = req.Method
	}
	if fields["enabled"] {
		endpoint.Enabled = req.Enabled
	}
	if fields["token"] {
		endpoint.Token = req.Token
	}
	if fields["runtime"] {
		endpoint.Runtime = req.Runtime
	}
	if fields["runtime_profile"] {
		endpoint.RuntimeProfile = req.RuntimeProfile
	}
	if fields["entry"] {
		endpoint.Entry = req.Entry
	}
	if fields["description"] {
		endpoint.Description = req.Description
	}
	if fields["builtin"] {
		endpoint.Builtin = req.Builtin
	}
	if fields["ip_whitelist"] {
		if req.IPWhitelist == nil {
			endpoint.IPWhitelist = nil
		} else {
			copied := append([]string(nil), (*req.IPWhitelist)...)
			endpoint.IPWhitelist = &copied
		}
	}
}

func openAPIRequestScript(req openAPIAdminRequest, fields map[string]bool) *string {
	if fields["script"] {
		return req.Script
	}
	if fields["code"] {
		return req.Code
	}
	return nil
}

func (s *Server) matchOpenAPIEndpoint(openPath, method string) (*types.OpenAPIEndpoint, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	openPath = normalizeOpenAPIPath(openPath)
	items, err := listOpenAPIEndpoints()
	if err != nil {
		return nil, err
	}
	for _, endpoint := range items {
		if endpoint == nil || !endpoint.Enabled {
			continue
		}
		if normalizeOpenAPIPath(endpoint.Path) == openPath && strings.EqualFold(endpoint.Method, method) {
			return endpoint, nil
		}
	}
	return nil, nil
}

func (s *Server) handleBuiltinOpenAPI(w http.ResponseWriter, r *http.Request, endpoint *types.OpenAPIEndpoint, requestData types.OpenAPIRequest, startedAt time.Time) {
	log.Printf("[INFO] OpenAPI 内置接口调用开始 endpoint=%s builtin=%s method=%s path=%s client=%s", endpoint.ID, endpoint.Builtin, requestData.Method, requestData.RawPath, requestData.ClientIP)
	switch endpoint.Builtin {
	case "qrcode":
		s.handleOpenAPIQRCode(w, r)
	default:
		logOpenAPICall("ERROR", endpoint.ID, requestData.Method, requestData.RawPath, requestData.ClientIP, http.StatusInternalServerError, startedAt, "内置接口不存在")
		s.jsonError(w, "内置 Open API 不存在", http.StatusInternalServerError)
	}
}

func (s *Server) openAPIDBExecutor() func(string, plugincore.PluginDBAction) plugincore.PluginDBResult {
	return func(openAPIID string, action plugincore.PluginDBAction) plugincore.PluginDBResult {
		if s.router == nil {
			return plugincore.PluginDBResult{Success: false, Error: "数据库执行器不可用"}
		}
		return s.router.ExecutePluginDBAction(openAPIID, action)
	}
}

func (s *Server) openAPISendMessageExecutor() func(string, plugincore.SendMessageAction) plugincore.PluginUserResult {
	return func(openAPIID string, action plugincore.SendMessageAction) plugincore.PluginUserResult {
		if s.router == nil {
			return plugincore.PluginUserResult{Success: false, Error: "消息发送器不可用"}
		}
		return s.router.SendPluginMessage(openAPIID, action)
	}
}

func (s *Server) openAPISendRichMessageExecutor() func(string, plugincore.RichMessageAction) plugincore.PluginUserResult {
	return func(openAPIID string, action plugincore.RichMessageAction) plugincore.PluginUserResult {
		if s.router == nil {
			return plugincore.PluginUserResult{Success: false, Error: "富文本消息发送器不可用"}
		}
		return s.router.SendPluginRichMessage(openAPIID, action)
	}
}

func (s *Server) openAPISendImageMessageExecutor() func(string, plugincore.ImageMessageAction) plugincore.PluginUserResult {
	return func(openAPIID string, action plugincore.ImageMessageAction) plugincore.PluginUserResult {
		if s.router == nil {
			return plugincore.PluginUserResult{Success: false, Error: "图片消息发送器不可用"}
		}
		return s.router.SendPluginImageMessage(openAPIID, action)
	}
}

func listOpenAPIEndpoints() ([]*types.OpenAPIEndpoint, error) {
	entries, err := os.ReadDir(openAPIStorageDir)
	if os.IsNotExist(err) {
		return []*types.OpenAPIEndpoint{}, nil
	}
	if err != nil {
		return nil, err
	}
	items := make([]*types.OpenAPIEndpoint, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		item, err := loadOpenAPIEndpoint(entry.Name())
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func loadOpenAPIEndpoint(id string) (*types.OpenAPIEndpoint, error) {
	normalizedID, err := normalizeOpenAPIID(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(openAPIEndpointDir(normalizedID), openAPIConfigFile))
	if err != nil {
		return nil, err
	}
	var endpoint types.OpenAPIEndpoint
	if err := json.Unmarshal(data, &endpoint); err != nil {
		return nil, err
	}
	if endpoint.ID == "" {
		endpoint.ID = normalizedID
	}
	if endpoint.ID != normalizedID {
		return nil, fmt.Errorf("Open API 配置 ID 与目录不一致: %s", normalizedID)
	}
	if err := normalizeOpenAPIEndpoint(&endpoint); err != nil {
		return nil, err
	}
	return &endpoint, nil
}

func saveOpenAPIEndpoint(endpoint types.OpenAPIEndpoint, script *string) (*types.OpenAPIEndpoint, error) {
	if err := normalizeOpenAPIEndpoint(&endpoint); err != nil {
		return nil, err
	}
	if endpoint.Token == "" {
		return nil, fmt.Errorf("Open API token 不能为空")
	}
	if !validOpenAPISinglePath(endpoint.Path) {
		return nil, fmt.Errorf("Open API 路径只能包含字母、数字、横线和下划线，且不能包含斜杠")
	}
	if err := ensureOpenAPIEndpointUnique(endpoint); err != nil {
		return nil, err
	}
	endpointDir := openAPIEndpointDir(endpoint.ID)
	if err := os.MkdirAll(endpointDir, 0755); err != nil {
		return nil, err
	}
	if endpoint.Builtin == "" && script == nil {
		entryPath, err := safeOpenAPIFilePath(endpointDir, endpoint.Entry)
		if err != nil {
			return nil, err
		}
		if _, statErr := os.Stat(entryPath); os.IsNotExist(statErr) {
			defaultScript := defaultOpenAPIScript(endpoint.Runtime)
			script = &defaultScript
		} else if statErr != nil {
			return nil, statErr
		}
	}
	if endpoint.Builtin == "" && script != nil {
		entryPath, err := safeOpenAPIFilePath(endpointDir, endpoint.Entry)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(entryPath), 0755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(entryPath, []byte(*script), 0644); err != nil {
			return nil, err
		}
	}
	data, err := json.MarshalIndent(endpoint, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(endpointDir, openAPIConfigFile), data, 0644); err != nil {
		return nil, err
	}
	return &endpoint, nil
}

func removeOpenAPIEndpoint(id string) error {
	normalizedID, err := normalizeOpenAPIID(id)
	if err != nil {
		return err
	}
	return os.RemoveAll(openAPIEndpointDir(normalizedID))
}

func ensureOpenAPIEndpointUnique(endpoint types.OpenAPIEndpoint) error {
	items, err := listOpenAPIEndpoints()
	if err != nil {
		return err
	}
	endpointPath := normalizeOpenAPIPath(endpoint.Path)
	for _, item := range items {
		if item == nil || item.ID == endpoint.ID {
			continue
		}
		if normalizeOpenAPIPath(item.Path) == endpointPath {
			return fmt.Errorf("Open API 路径已被 %s 使用", item.ID)
		}
	}
	return nil
}

func readOpenAPIScript(endpoint *types.OpenAPIEndpoint) (string, error) {
	entryPath, err := safeOpenAPIFilePath(openAPIEndpointDir(endpoint.ID), endpoint.Entry)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(entryPath)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Server) openAPIAdminResponse(endpoint *types.OpenAPIEndpoint, script string, stat config.OpenAPICallStat) map[string]interface{} {
	effective, mode, source, err := s.effectiveOpenAPIIPRules(endpoint)
	if err != nil {
		effective = openAPIIPRules{}
	}
	var configured interface{}
	if endpoint.IPWhitelist != nil {
		configured = append([]string(nil), (*endpoint.IPWhitelist)...)
	}
	return map[string]interface{}{
		"id":                     endpoint.ID,
		"name":                   endpoint.Name,
		"path":                   endpoint.Path,
		"method":                 endpoint.Method,
		"enabled":                endpoint.Enabled,
		"has_token":              strings.TrimSpace(endpoint.Token) != "",
		"runtime":                endpoint.Runtime,
		"runtime_profile":        endpoint.RuntimeProfile,
		"entry":                  endpoint.Entry,
		"description":            endpoint.Description,
		"builtin":                endpoint.Builtin,
		"script":                 script,
		"ip_whitelist":           configured,
		"ip_whitelist_mode":      mode,
		"effective_ip_whitelist": append([]string(nil), effective.raw...),
		"ip_whitelist_source":    source,
		"call_stats":             stat,
	}
}

func (s *Server) validateOpenAPIRuntimeProfile(endpoint *types.OpenAPIEndpoint) error {
	if endpoint == nil {
		return fmt.Errorf("Open API 配置不能为空")
	}
	originalRuntime := strings.ToLower(strings.TrimSpace(endpoint.Runtime))
	originalBuiltin := strings.TrimSpace(endpoint.Builtin)
	originalProfile := strings.TrimSpace(endpoint.RuntimeProfile)
	if originalProfile != "" && (originalBuiltin != "" || originalRuntime == "builtin") {
		return fmt.Errorf("内置 Open API 不允许配置运行环境 Profile")
	}
	copyEndpoint := *endpoint
	if err := normalizeOpenAPIEndpoint(&copyEndpoint); err != nil {
		return err
	}
	endpoint.Runtime = copyEndpoint.Runtime
	endpoint.RuntimeProfile = copyEndpoint.RuntimeProfile
	if copyEndpoint.Runtime == "builtin" || copyEndpoint.Builtin != "" {
		endpoint.RuntimeProfile = ""
		return nil
	}
	if copyEndpoint.RuntimeProfile == "" {
		return nil
	}
	manager := s.runtimeDepsManager()
	if manager == nil {
		return fmt.Errorf("依赖管理器不可用，无法校验运行环境 Profile")
	}
	if _, err := manager.ResolveRuntime(copyEndpoint.Runtime, copyEndpoint.RuntimeProfile); err != nil {
		return fmt.Errorf("运行环境 Profile 无效: %w", err)
	}
	return nil
}

func normalizeOpenAPIEndpoint(endpoint *types.OpenAPIEndpoint) error {
	if endpoint == nil {
		return fmt.Errorf("Open API 配置不能为空")
	}
	id, err := normalizeOpenAPIID(endpoint.ID)
	if err != nil {
		return err
	}
	endpoint.ID = id
	endpoint.Name = strings.TrimSpace(endpoint.Name)
	if endpoint.Name == "" {
		endpoint.Name = endpoint.ID
	}
	endpoint.Path = normalizeOpenAPIPath(endpoint.Path)
	if endpoint.Path == "" {
		endpoint.Path = endpoint.ID
	}
	endpoint.Description = strings.TrimSpace(endpoint.Description)
	endpoint.Builtin = strings.ToLower(strings.TrimSpace(endpoint.Builtin))
	if endpoint.Builtin != "" && endpoint.Builtin != "qrcode" {
		return fmt.Errorf("不支持的内置 Open API: %s", endpoint.Builtin)
	}
	endpoint.Method = strings.ToUpper(strings.TrimSpace(endpoint.Method))
	if endpoint.Method == "" {
		endpoint.Method = http.MethodPost
	}
	if !validOpenAPIMethod(endpoint.Method) {
		return fmt.Errorf("不支持的请求方法: %s", endpoint.Method)
	}
	endpoint.Token = strings.TrimSpace(endpoint.Token)
	endpoint.Runtime = strings.ToLower(strings.TrimSpace(endpoint.Runtime))
	if endpoint.Runtime == "node" {
		endpoint.Runtime = "nodejs"
	}
	if endpoint.Runtime == "py" || endpoint.Runtime == "python3" {
		endpoint.Runtime = "python"
	}
	endpoint.RuntimeProfile = strings.TrimSpace(endpoint.RuntimeProfile)
	if endpoint.Runtime == "" {
		endpoint.Runtime = "nodejs"
	}
	if endpoint.Builtin != "" {
		endpoint.Runtime = "builtin"
		endpoint.RuntimeProfile = ""
		endpoint.Entry = ""
		return nil
	}
	if endpoint.Runtime != "nodejs" && endpoint.Runtime != "python" {
		return fmt.Errorf("不支持的运行时: %s", endpoint.Runtime)
	}
	entry := strings.TrimSpace(endpoint.Entry)
	if entry == "" {
		entry = defaultOpenAPIEntry(endpoint.Path, endpoint.Runtime)
	}
	normalizedEntry, err := normalizeOpenAPIEntry(entry, endpoint.Runtime)
	if err != nil {
		return err
	}
	endpoint.Entry = normalizedEntry
	return nil
}

func validOpenAPIMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions:
		return true
	default:
		return false
	}
}

func validOpenAPIRawPath(path string) bool {
	path = strings.TrimSpace(path)
	if strings.Contains(path, "/") || strings.Contains(path, "\\") {
		return false
	}
	return validOpenAPISinglePath(path)
}

func validOpenAPISinglePath(path string) bool {
	if path == "" {
		return false
	}
	for _, char := range path {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' {
			continue
		}
		return false
	}
	return true
}

func normalizeOpenAPIID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("ID 不能为空")
	}
	for _, char := range id {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' {
			continue
		}
		return "", fmt.Errorf("ID 只能包含字母、数字、横线和下划线")
	}
	return id, nil
}

func normalizeOpenAPIPath(path string) string {
	path = strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
	path = strings.TrimPrefix(path, "/api/open/")
	return strings.Trim(path, "/")
}

func normalizeOpenAPIEntry(entry, runtime string) (string, error) {
	entry = strings.TrimSpace(strings.ReplaceAll(entry, "\\", "/"))
	if entry == "" {
		entry = defaultOpenAPIEntry("main", runtime)
	}
	if strings.HasPrefix(entry, "/") || filepath.IsAbs(entry) {
		return "", fmt.Errorf("入口文件路径无效")
	}
	clean := filepath.Clean(entry)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("入口文件路径越界")
	}
	return filepath.ToSlash(clean), nil
}

func defaultOpenAPIEntry(path, runtime string) string {
	name := normalizeOpenAPIPath(path)
	if !validOpenAPISinglePath(name) {
		name = "main"
	}
	if runtime == "python" {
		return name + ".py"
	}
	return name + ".js"
}

func safeOpenAPIFilePath(root, relative string) (string, error) {
	cleanRelative, err := normalizeOpenAPIEntry(relative, "nodejs")
	if err != nil {
		return "", err
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	fullPath, err := filepath.Abs(filepath.Join(root, cleanRelative))
	if err != nil {
		return "", err
	}
	if fullPath != rootAbs && !strings.HasPrefix(fullPath, rootAbs+string(os.PathSeparator)) {
		return "", fmt.Errorf("入口文件路径越界")
	}
	return fullPath, nil
}

func openAPIEndpointDir(id string) string {
	return filepath.Join(openAPIStorageDir, id)
}

func defaultOpenAPIScript(runtime string) string {
	if runtime == "python" {
		return "async def action(ctx, req, res):\n    res.json({\"ok\": True})\n"
	}
	return "module.exports.action = async function action(ctx, req, res) {\n  res.json({ ok: true })\n}\n"
}

func buildOpenAPIRequest(r *http.Request, openPath string, body []byte, clientAddr netip.Addr) (types.OpenAPIRequest, map[string]string) {
	query := map[string][]string(r.URL.Query())
	headers := map[string][]string(r.Header)
	jsonBody := parseOpenAPIJSON(body, r.Header.Get("Content-Type"))
	formBody := parseOpenAPIForm(body, r.Header.Get("Content-Type"))
	tokens := map[string]string{
		"query":  strings.TrimSpace(r.URL.Query().Get("token")),
		"header": openAPIHeaderToken(r),
		"body":   openAPIBodyToken(jsonBody, formBody),
	}
	clientIP := ""
	if clientAddr.IsValid() {
		clientIP = clientAddr.Unmap().String()
	}
	return types.OpenAPIRequest{
		Method:       r.Method,
		Path:         openPath,
		RawPath:      r.URL.Path,
		Query:        query,
		Headers:      headers,
		Body:         string(body),
		JSON:         jsonBody,
		Form:         formBody,
		TokenSources: maskOpenAPITokens(tokens),
		ClientIP:     clientIP,
	}, tokens
}

func parseOpenAPIJSON(body []byte, contentType string) map[string]interface{} {
	mediaType, _, _ := mime.ParseMediaType(contentType)
	if mediaType != "application/json" || len(body) == 0 {
		return nil
	}
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil
	}
	return data
}

func parseOpenAPIForm(body []byte, contentType string) map[string][]string {
	mediaType, _, _ := mime.ParseMediaType(contentType)
	if mediaType != "application/x-www-form-urlencoded" || len(body) == 0 {
		return nil
	}
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return nil
	}
	return map[string][]string(values)
}

func openAPIHeaderToken(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("X-Open-Token")); value != "" {
		return value
	}
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(value), "bearer ") {
		return strings.TrimSpace(value[7:])
	}
	return value
}

func openAPIBodyToken(jsonBody map[string]interface{}, formBody map[string][]string) string {
	if jsonBody != nil {
		if value, ok := jsonBody["token"].(string); ok {
			return strings.TrimSpace(value)
		}
	}
	if formBody != nil {
		if values := formBody["token"]; len(values) > 0 {
			return strings.TrimSpace(values[0])
		}
	}
	return ""
}

func sanitizeOpenAPIRequest(request types.OpenAPIRequest, tokens map[string]string) types.OpenAPIRequest {
	request.TokenSources = maskOpenAPITokens(tokens)
	request.Query = sanitizeOpenAPIValues(request.Query, "token")
	request.Headers = sanitizeOpenAPIValues(request.Headers, "authorization", "x-open-token")
	request.Form = sanitizeOpenAPIValues(request.Form, "token")
	if request.JSON != nil {
		request.JSON = sanitizeOpenAPIJSON(request.JSON, "token")
	}
	body := request.Body
	for _, token := range tokens {
		if token != "" {
			body = strings.ReplaceAll(body, token, "***")
		}
	}
	request.Body = body
	return request
}

func sanitizeOpenAPIValues(values map[string][]string, keys ...string) map[string][]string {
	if values == nil {
		return nil
	}
	keySet := map[string]bool{}
	for _, key := range keys {
		keySet[strings.ToLower(key)] = true
	}
	result := map[string][]string{}
	for key, items := range values {
		copied := append([]string(nil), items...)
		if keySet[strings.ToLower(key)] {
			for index := range copied {
				if copied[index] != "" {
					copied[index] = "***"
				}
			}
		}
		result[key] = copied
	}
	return result
}

func sanitizeOpenAPIJSON(values map[string]interface{}, keys ...string) map[string]interface{} {
	keySet := map[string]bool{}
	for _, key := range keys {
		keySet[strings.ToLower(key)] = true
	}
	result := map[string]interface{}{}
	for key, value := range values {
		if keySet[strings.ToLower(key)] {
			result[key] = "***"
			continue
		}
		result[key] = value
	}
	return result
}

func maskOpenAPITokens(tokens map[string]string) map[string]string {
	masked := map[string]string{}
	for source, token := range tokens {
		if token != "" {
			masked[source] = "***"
		} else {
			masked[source] = ""
		}
	}
	return masked
}

func openAPITokenMatched(expected string, tokens map[string]string) bool {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return false
	}
	matched := false
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
			return false
		}
		matched = true
	}
	return matched
}

func writeOpenAPIResponse(w http.ResponseWriter, response types.OpenAPIResponse) {
	if response.Headers == nil {
		response.Headers = map[string]string{"Content-Type": "application/json; charset=utf-8"}
	}
	for key, value := range response.Headers {
		if strings.TrimSpace(key) != "" {
			w.Header().Set(key, value)
		}
	}
	status := response.Status
	if status <= 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	if response.JSON != nil {
		_ = json.NewEncoder(w).Encode(response.JSON)
		return
	}
	if response.Data != nil {
		_ = json.NewEncoder(w).Encode(response.Data)
		return
	}
	_, _ = w.Write([]byte(response.Body))
}
