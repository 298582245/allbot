package web

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	plugincore "github.com/allbot/allbot/core/plugin"
	"github.com/allbot/allbot/core/types"
)

const openAPIPrefix = "/api/open/"
const openAPIStorageDir = "openapis"
const openAPIConfigFile = "config.json"

type openAPIAdminRequest struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Path        string  `json:"path"`
	Method      string  `json:"method"`
	Enabled     bool    `json:"enabled"`
	Token       string  `json:"token"`
	Runtime     string  `json:"runtime"`
	Entry       string  `json:"entry"`
	Description string  `json:"description"`
	Script      *string `json:"script"`
	Code        *string `json:"code"`
}

func (s *Server) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	startedAt := time.Now()
	openPath := strings.Trim(strings.TrimPrefix(r.URL.Path, openAPIPrefix), "/")
	requestIP := clientIP(r)
	if openPath == "" {
		logOpenAPICall("WARN", "未匹配", r.Method, r.URL.Path, requestIP, http.StatusNotFound, startedAt, "路径为空")
		s.jsonError(w, "Open API 路径不能为空", http.StatusNotFound)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		logOpenAPICall("ERROR", openPath, r.Method, r.URL.Path, requestIP, http.StatusBadRequest, startedAt, "读取请求体失败: "+err.Error())
		s.jsonError(w, "读取请求体失败", http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	requestData, tokenSources := buildOpenAPIRequest(r, openPath, body)
	endpoint, err := s.matchOpenAPIEndpoint(openPath, r.Method)
	if err != nil {
		logOpenAPICall("ERROR", openPath, r.Method, r.URL.Path, requestIP, http.StatusInternalServerError, startedAt, "读取配置失败: "+err.Error())
		s.jsonError(w, "读取 Open API 配置失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if endpoint == nil {
		logOpenAPICall("WARN", openPath, r.Method, r.URL.Path, requestIP, http.StatusNotFound, startedAt, "接口不存在或未启用")
		s.jsonError(w, "Open API 不存在或未启用", http.StatusNotFound)
		return
	}
	if !openAPITokenMatched(endpoint.Token, tokenSources) {
		logOpenAPICall("WARN", endpoint.ID, r.Method, requestData.RawPath, requestData.ClientIP, http.StatusUnauthorized, startedAt, "token 无效")
		s.jsonError(w, "Open API token 无效", http.StatusUnauthorized)
		return
	}
	requestData = sanitizeOpenAPIRequest(requestData, tokenSources)
	log.Printf("[INFO] OpenAPI 调用开始 endpoint=%s method=%s path=%s client=%s body=%dB", endpoint.ID, requestData.Method, requestData.RawPath, requestData.ClientIP, len(body))
	response, err := s.pluginManager.ExecuteOpenAPI(*endpoint, openAPIEndpointDir(endpoint.ID), requestData, s.openAPIDBExecutor(), s.openAPISendMessageExecutor())
	if err != nil {
		logOpenAPICall("ERROR", endpoint.ID, requestData.Method, requestData.RawPath, requestData.ClientIP, http.StatusInternalServerError, startedAt, "执行失败: "+err.Error())
		s.jsonError(w, "Open API 执行失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	status := response.Status
	if status <= 0 {
		status = http.StatusOK
	}
	logOpenAPICall("INFO", endpoint.ID, requestData.Method, requestData.RawPath, requestData.ClientIP, status, startedAt, "调用完成")
	writeOpenAPIResponse(w, response)
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
		result := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			result = append(result, openAPIAdminResponse(item, ""))
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
	if _, err := normalizeOpenAPIID(id); err != nil {
		s.jsonError(w, "Open API ID 无效: "+err.Error(), http.StatusBadRequest)
		return
	}
	if action == "code" {
		s.handleOpenAPICode(w, r, id)
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
		s.jsonResponse(w, openAPIAdminResponse(endpoint, ""))
	case http.MethodPut:
		s.saveOpenAPIFromRequest(w, r, id)
	case http.MethodDelete:
		if err := removeOpenAPIEndpoint(id); err != nil {
			s.jsonError(w, "删除 Open API 失败: "+err.Error(), http.StatusInternalServerError)
			return
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
	if rest == "" {
		return "", "", false
	}
	parts := strings.Split(rest, "/")
	if len(parts) == 1 {
		id, err := url.PathUnescape(parts[0])
		return id, "", err == nil && id != ""
	}
	if len(parts) == 2 && parts[1] == "code" {
		id, err := url.PathUnescape(parts[0])
		return id, "code", err == nil && id != ""
	}
	return "", "", false
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
	switch r.Method {
	case http.MethodGet:
		code, err := readOpenAPIScript(endpoint)
		if err != nil {
			s.jsonError(w, "读取 Open API 代码失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"id": endpoint.ID, "runtime": endpoint.Runtime, "entry": endpoint.Entry, "file": endpoint.Entry, "code": code, "content": code})
	case http.MethodPut:
		var req struct {
			Code    string `json:"code"`
			Content string `json:"content"`
			Runtime string `json:"runtime"`
			File    string `json:"file"`
			Entry   string `json:"entry"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "请求数据无效", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.Runtime) != "" {
			endpoint.Runtime = req.Runtime
		}
		if strings.TrimSpace(req.Entry) != "" {
			endpoint.Entry = req.Entry
		} else if strings.TrimSpace(req.File) != "" {
			endpoint.Entry = req.File
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
		s.jsonResponse(w, map[string]interface{}{"id": saved.ID, "runtime": saved.Runtime, "entry": saved.Entry, "file": saved.Entry, "code": code, "content": code})
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
	if fields["path"] && !validOpenAPIRawPath(req.Path) {
		s.jsonError(w, "Open API 路径只能输入单个词，且只能包含字母、数字、横线和下划线", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(endpoint.Token) == "" {
		s.jsonError(w, "Open API token 不能为空", http.StatusBadRequest)
		return
	}

	script := openAPIRequestScript(req, fields)
	saved, err := saveOpenAPIEndpoint(endpoint, script)
	if err != nil {
		s.jsonError(w, "保存 Open API 失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	s.jsonResponse(w, openAPIAdminResponse(saved, ""))
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
	if fields["entry"] {
		endpoint.Entry = req.Entry
	}
	if fields["description"] {
		endpoint.Description = req.Description
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
	if script == nil {
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
	if script != nil {
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

func openAPIAdminResponse(endpoint *types.OpenAPIEndpoint, script string) map[string]interface{} {
	return map[string]interface{}{
		"id":          endpoint.ID,
		"name":        endpoint.Name,
		"path":        endpoint.Path,
		"method":      endpoint.Method,
		"enabled":     endpoint.Enabled,
		"has_token":   strings.TrimSpace(endpoint.Token) != "",
		"runtime":     endpoint.Runtime,
		"entry":       endpoint.Entry,
		"description": endpoint.Description,
		"script":      script,
	}
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
	endpoint.Method = strings.ToUpper(strings.TrimSpace(endpoint.Method))
	if endpoint.Method == "" {
		endpoint.Method = http.MethodPost
	}
	if !validOpenAPIMethod(endpoint.Method) {
		return fmt.Errorf("不支持的请求方法: %s", endpoint.Method)
	}
	endpoint.Token = strings.TrimSpace(endpoint.Token)
	endpoint.Runtime = strings.ToLower(strings.TrimSpace(endpoint.Runtime))
	if endpoint.Runtime == "" {
		endpoint.Runtime = "nodejs"
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

func buildOpenAPIRequest(r *http.Request, openPath string, body []byte) (types.OpenAPIRequest, map[string]string) {
	query := map[string][]string(r.URL.Query())
	headers := map[string][]string(r.Header)
	jsonBody := parseOpenAPIJSON(body, r.Header.Get("Content-Type"))
	formBody := parseOpenAPIForm(body, r.Header.Get("Content-Type"))
	tokens := map[string]string{
		"query":  strings.TrimSpace(r.URL.Query().Get("token")),
		"header": openAPIHeaderToken(r),
		"body":   openAPIBodyToken(jsonBody, formBody),
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
		ClientIP:     clientIP(r),
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

func clientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		return strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
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
