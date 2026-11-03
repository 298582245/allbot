package web

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	"github.com/allbot/allbot/core/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const developerTokenLifetime = 24 * time.Hour

type developerSubject struct {
	app       *config.DeveloperApp
	admin     bool
	adminName string
}

type developerTokenClaims struct {
	AppID             string `json:"aid"`
	CredentialVersion int64  `json:"cv"`
	IssuedAt          int64  `json:"iat"`
	ExpiresAt         int64  `json:"exp"`
	Nonce             string `json:"jti"`
}

type developerCapability struct {
	Code        string `json:"code"`
	Module      string `json:"module"`
	Action      string `json:"action"`
	Description string `json:"description"`
	Risk        string `json:"risk"`
}

type developerAuditWriter struct {
	http.ResponseWriter
	status int
}

func (w *developerAuditWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *developerAuditWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

var developerCapabilities = []developerCapability{
	{Code: "system:read", Module: "系统", Action: "读取", Description: "读取系统状态与统计", Risk: "low"},
	{Code: "system:update", Module: "系统", Action: "更新", Description: "检查并应用系统更新", Risk: "critical"},
	{Code: "settings:read", Module: "设置", Action: "读取", Description: "读取系统配置", Risk: "medium"},
	{Code: "settings:update", Module: "设置", Action: "修改", Description: "修改系统配置", Risk: "high"},
	{Code: "plugins:read", Module: "插件", Action: "读取", Description: "读取插件信息与配置", Risk: "medium"},
	{Code: "plugins:create", Module: "插件", Action: "创建/导入", Description: "创建或导入插件", Risk: "high"},
	{Code: "plugins:update", Module: "插件", Action: "修改", Description: "修改插件配置、代码及文件", Risk: "high"},
	{Code: "plugins:delete", Module: "插件", Action: "删除", Description: "删除或清理插件", Risk: "critical"},
	{Code: "plugins:execute", Module: "插件", Action: "执行", Description: "执行插件操作或任务", Risk: "high"},
	{Code: "adapters:read", Module: "适配器", Action: "读取", Description: "读取适配器及平台信息", Risk: "medium"},
	{Code: "adapters:create", Module: "适配器", Action: "创建", Description: "创建适配器", Risk: "high"},
	{Code: "adapters:update", Module: "适配器", Action: "修改", Description: "修改适配器配置与状态", Risk: "high"},
	{Code: "adapters:delete", Module: "适配器", Action: "删除", Description: "删除适配器", Risk: "critical"},
	{Code: "users:read", Module: "用户", Action: "读取", Description: "查询用户、账号与关联关系", Risk: "high"},
	{Code: "users:update", Module: "用户", Action: "修改", Description: "修改用户资料或积分", Risk: "critical"},
	{Code: "replies:read", Module: "关键字回复", Action: "读取", Description: "查询关键字回复", Risk: "low"},
	{Code: "replies:create", Module: "关键字回复", Action: "创建", Description: "创建关键字回复", Risk: "medium"},
	{Code: "replies:update", Module: "关键字回复", Action: "修改", Description: "修改关键字回复", Risk: "medium"},
	{Code: "replies:delete", Module: "关键字回复", Action: "删除", Description: "删除关键字回复", Risk: "high"},
	{Code: "tasks:read", Module: "任务", Action: "读取", Description: "查询定时、脚本和插件任务", Risk: "medium"},
	{Code: "tasks:create", Module: "任务", Action: "创建", Description: "创建任务", Risk: "high"},
	{Code: "tasks:update", Module: "任务", Action: "修改", Description: "修改或启停任务", Risk: "high"},
	{Code: "tasks:delete", Module: "任务", Action: "删除", Description: "删除任务", Risk: "high"},
	{Code: "tasks:execute", Module: "任务", Action: "执行", Description: "立即执行或停止任务", Risk: "critical"},
	{Code: "script_envs:read", Module: "脚本环境", Action: "读取", Description: "读取脚本环境变量", Risk: "high"},
	{Code: "script_envs:update", Module: "脚本环境", Action: "修改", Description: "创建、导入或修改环境变量", Risk: "critical"},
	{Code: "dependencies:read", Module: "依赖与运行环境", Action: "读取", Description: "读取依赖及运行环境", Risk: "medium"},
	{Code: "dependencies:update", Module: "依赖与运行环境", Action: "修改", Description: "安装依赖或初始化运行环境", Risk: "critical"},
	{Code: "payments:read", Module: "支付", Action: "读取", Description: "读取支付配置与订单", Risk: "high"},
	{Code: "payments:update", Module: "支付", Action: "修改", Description: "修改支付配置或订单", Risk: "critical"},
	{Code: "images:read", Module: "图床", Action: "读取", Description: "读取图片资源", Risk: "medium"},
	{Code: "images:update", Module: "图床", Action: "修改", Description: "上传或修改图片资源", Risk: "high"},
	{Code: "images:delete", Module: "图床", Action: "删除", Description: "删除图片资源", Risk: "high"},
	{Code: "logs:read", Module: "日志", Action: "读取", Description: "查询或下载日志", Risk: "medium"},
	{Code: "logs:cleanup", Module: "日志", Action: "清理", Description: "清理日志或修改保留策略", Risk: "high"},
	{Code: "backups:read", Module: "备份", Action: "读取", Description: "查询或下载备份", Risk: "high"},
	{Code: "backups:create", Module: "备份", Action: "创建", Description: "创建备份", Risk: "high"},
	{Code: "backups:delete", Module: "备份", Action: "删除", Description: "删除备份", Risk: "critical"},
	{Code: "backups:restore", Module: "备份", Action: "恢复", Description: "恢复备份", Risk: "critical"},
	{Code: "data:read", Module: "数据管理", Action: "读取/导出", Description: "读取表和视图或导出数据", Risk: "high"},
	{Code: "data:update", Module: "数据管理", Action: "修改/导入", Description: "导入或修改数据", Risk: "critical"},
	{Code: "statistics:read", Module: "统计", Action: "读取", Description: "读取统计数据", Risk: "low"},
	{Code: "sdk:read", Module: "SDK", Action: "读取", Description: "读取 SDK 文件及参考信息", Risk: "low"},
	{Code: "open_apis:read", Module: "自定义开放接口管理", Action: "读取", Description: "查看自定义开放接口配置，不包括脚本调用", Risk: "medium"},
	{Code: "open_apis:update", Module: "自定义开放接口管理", Action: "管理", Description: "管理自定义开放接口配置，不包括脚本调用", Risk: "high"},
}

func (s *Server) registerDeveloperRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/developer-apps", s.handleDeveloperApps)
	mux.HandleFunc("/api/developer-apps/capabilities", s.handleAdminDeveloperCapabilities)
	mux.HandleFunc("/api/developer-apps/", s.handleDeveloperAppAction)
	mux.HandleFunc("/api/v1/open-api/token", s.handleDeveloperToken)
	mux.HandleFunc("/api/v1/capabilities", s.handleDeveloperCapabilities)
	mux.Handle("/api/v1/mcp", s.mcpHandler(mux))
	mux.Handle("/api/v1/", s.developerAPIHandler(mux))
}

func (s *Server) handleDeveloperApps(w http.ResponseWriter, r *http.Request) {
	db := s.runtimeDatabase()
	if db == nil {
		s.jsonError(w, "数据库未初始化", http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodGet:
		query := r.URL.Query()
		page, _ := strconv.Atoi(query.Get("page"))
		pageSize, _ := strconv.Atoi(query.Get("page_size"))
		if page < 1 {
			page = 1
		}
		if pageSize < 1 {
			pageSize = 20
		} else if pageSize > 200 {
			pageSize = 200
		}
		apps, total, page, err := db.ListDeveloperApps(query.Get("keyword"), page, pageSize)
		if err != nil {
			s.jsonError(w, "读取应用失败", http.StatusInternalServerError)
			return
		}
		for i := range apps {
			apps[i].Permissions = normalizeDeveloperPermissions(apps[i].Permissions)
		}
		s.jsonSuccess(w, map[string]any{"items": apps, "total": total, "page": page, "page_size": pageSize})
	case http.MethodPost:
		var input struct {
			Name        string   `json:"name"`
			Permissions []string `json:"permissions"`
		}
		if err := decodeDeveloperJSON(r, &input); err != nil {
			s.jsonError(w, "请求参数无效", http.StatusBadRequest)
			return
		}
		input.Name = strings.TrimSpace(input.Name)
		if input.Name == "" || len([]rune(input.Name)) > 100 || !validDeveloperPermissions(input.Permissions) {
			s.jsonError(w, "应用名称或权限无效", http.StatusBadRequest)
			return
		}
		app, secret, err := s.createDeveloperApp(input.Name, normalizeDeveloperPermissions(input.Permissions))
		if err != nil {
			s.jsonError(w, "创建应用失败", http.StatusInternalServerError)
			return
		}
		s.jsonSuccess(w, map[string]any{"app": app, "appSecret": secret})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAdminDeveloperCapabilities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.jsonResponse(w, developerPermissionModules())
}

func (s *Server) handleDeveloperAppAction(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/api/developer-apps/") {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/developer-apps/"), "/"), "/")
	if len(parts) != 2 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	db := s.runtimeDatabase()
	if db == nil {
		s.jsonError(w, "数据库未初始化", http.StatusInternalServerError)
		return
	}
	appID, action := parts[0], parts[1]
	switch {
	case action == "permissions" && r.Method == http.MethodPut:
		var input struct {
			Name        string   `json:"name"`
			Permissions []string `json:"permissions"`
		}
		if decodeDeveloperJSON(r, &input) != nil {
			s.jsonError(w, "请求参数无效", http.StatusBadRequest)
			return
		}
		input.Name = strings.TrimSpace(input.Name)
		if input.Name == "" || len([]rune(input.Name)) > 100 || !validDeveloperPermissions(input.Permissions) {
			s.jsonError(w, "应用名称或权限无效", http.StatusBadRequest)
			return
		}
		if err := db.UpdateDeveloperApp(appID, input.Name, normalizeDeveloperPermissions(input.Permissions)); err != nil {
			developerAppDBError(s, w, err)
			return
		}
		s.jsonSuccess(w, map[string]bool{"updated": true})
	case action == "secret" && r.Method == http.MethodPost:
		secret, err := randomDeveloperValue(32)
		if err != nil {
			s.jsonError(w, "重置凭据失败", http.StatusInternalServerError)
			return
		}
		if _, err := db.ResetDeveloperAppSecret(appID, hashDeveloperSecret(secret)); err != nil {
			developerAppDBError(s, w, err)
			return
		}
		s.jsonSuccess(w, map[string]string{"appSecret": secret})
	case action == "delete" && r.Method == http.MethodDelete:
		if err := db.DeleteDeveloperApp(appID); err != nil {
			developerAppDBError(s, w, err)
			return
		}
		s.jsonSuccess(w, map[string]bool{"deleted": true})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleDeveloperToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var input struct {
		AppKey    string `json:"app_key"`
		AppSecret string `json:"app_secret"`
	}
	if decodeDeveloperJSON(r, &input) != nil || input.AppKey == "" || input.AppSecret == "" {
		s.writeDeveloperError(w, http.StatusUnauthorized, "令牌无效或已过期")
		return
	}
	db := s.runtimeDatabase()
	if db == nil {
		s.writeDeveloperError(w, http.StatusInternalServerError, "服务暂不可用")
		return
	}
	app, err := db.GetDeveloperAppByClientID(input.AppKey)
	if err != nil || subtleCompare(hashDeveloperSecret(input.AppSecret), app.SecretHash) == false {
		s.writeDeveloperError(w, http.StatusUnauthorized, "令牌无效或已过期")
		return
	}
	token, err := s.issueDeveloperToken(app)
	if err != nil {
		s.jsonError(w, "签发令牌失败", http.StatusInternalServerError)
		return
	}
	_ = db.TouchDeveloperApp(app.ID)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"access_token": token, "token_type": "Bearer", "expires_in": int64(developerTokenLifetime.Seconds())})
}

func (s *Server) handleDeveloperCapabilities(w http.ResponseWriter, r *http.Request) {
	subject, status, message := s.authenticateDeveloperBearer(r.Header.Get("Authorization"))
	if status != 0 {
		s.writeDeveloperError(w, status, message)
		return
	}
	s.jsonSuccess(w, s.visibleCapabilities(subject))
}

func (s *Server) developerAPIHandler(internal http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/" || strings.HasSuffix(r.URL.Path, "/open-api/token") || strings.HasSuffix(r.URL.Path, "/capabilities") || strings.HasSuffix(r.URL.Path, "/mcp") {
			http.NotFound(w, r)
			return
		}
		subject, status, message := s.authenticateDeveloperBearer(r.Header.Get("Authorization"))
		if status != 0 {
			s.writeDeveloperError(w, status, message)
			return
		}
		originalPath := "/api" + strings.TrimPrefix(r.URL.Path, "/api/v1")
		capability, ok := developerRouteCapability(r.Method, originalPath)
		if !ok {
			s.recordDeveloperAudit(subject, "rest", "route:unknown", http.StatusNotFound, r)
			s.writeDeveloperError(w, http.StatusNotFound, "接口不存在")
			return
		}
		if !subject.admin && !developerSubjectAllows(subject, capability) {
			s.recordDeveloperAudit(subject, "rest", capability, http.StatusForbidden, r)
			s.writeDeveloperError(w, http.StatusForbidden, "权限不足")
			return
		}
		if isUnsafeHTTPMethod(r.Method) && r.ContentLength > 10<<20 {
			s.writeDeveloperError(w, http.StatusRequestEntityTooLarge, "请求体过大")
			return
		}
		if isHighImpactCapability(capability) {
			if err := s.recordDeveloperAudit(subject, "rest", capability+":started", 0, r); err != nil {
				s.writeDeveloperError(w, http.StatusInternalServerError, "操作审计不可用，已拒绝本次操作")
				return
			}
		}
		clone := r.Clone(r.Context())
		clone.URL.Path = originalPath
		clone.URL.RawPath = ""
		if clone.Body != nil {
			clone.Body = http.MaxBytesReader(w, clone.Body, 10<<20)
		}
		writer := &developerAuditWriter{ResponseWriter: w}
		if !s.serveDeveloperSettings(writer, clone, originalPath) {
			internal.ServeHTTP(writer, clone)
		}
		statusCode := writer.status
		if statusCode == 0 {
			statusCode = http.StatusOK
		}
		if err := s.recordDeveloperAudit(subject, "rest", capability, statusCode, r); err != nil && isHighImpactCapability(capability) && statusCode < 400 {
			log.Printf("[ERROR] developer API audit write failed for capability %s", capability)
		}
	})
}

func (s *Server) mcpHandler(internal http.Handler) http.Handler {
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		subject, status, _ := s.authenticateMCP(r.Header.Get("Authorization"))
		if status != 0 {
			return nil
		}
		server := mcp.NewServer(&mcp.Implementation{Name: "AllBot", Version: "1.0.0"}, nil)
		server.AddTool(&mcp.Tool{Name: "list_capabilities", Description: "列出当前身份可调用的 AllBot 管理能力", InputSchema: json.RawMessage(`{"type":"object","properties":{}}`)}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			data, _ := json.Marshal(s.visibleCapabilities(subject))
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(data)}}}, nil
		})
		inputSchema := json.RawMessage(`{"type":"object","required":["method","path"],"properties":{"method":{"type":"string","enum":["GET","POST","PUT","PATCH","DELETE"]},"path":{"type":"string"},"query":{"type":"string"},"body":{"type":"object"},"raw_body_base64":{"type":"string","description":"用于 multipart 或二进制请求体；不能与 body 同时提供"},"content_type":{"type":"string","description":"原始请求体的 Content-Type，multipart/form-data 需包含 boundary"}}}`)
		for _, capability := range s.visibleCapabilities(subject) {
			capability := capability
			toolName := "api_" + strings.ReplaceAll(capability.Code, ":", "_")
			server.AddTool(&mcp.Tool{Name: toolName, Description: capability.Description + "。需要 " + capability.Code + "；调用路径必须属于该能力。", InputSchema: inputSchema}, s.mcpCapabilityHandler(internal, subject, capability.Code, r))
		}
		return server
	}, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, MaxRequestBodyBytes: 16 << 20})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, status, message := s.authenticateMCP(r.Header.Get("Authorization")); status != 0 {
			s.writeDeveloperError(w, status, message)
			return
		}
		handler.ServeHTTP(w, r)
	})
}

func (s *Server) mcpCapabilityHandler(internal http.Handler, subject *developerSubject, expectedCapability string, source *http.Request) mcp.ToolHandler {
	return func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var input struct {
			Method        string          `json:"method"`
			Path          string          `json:"path"`
			Query         string          `json:"query,omitempty"`
			Body          json.RawMessage `json:"body"`
			RawBodyBase64 string          `json:"raw_body_base64,omitempty"`
			ContentType   string          `json:"content_type,omitempty"`
		}
		if err := json.Unmarshal(request.Params.Arguments, &input); err != nil {
			return toolError("参数无效"), nil
		}
		if !strings.HasPrefix(input.Path, "/api/v1/") {
			return toolError("path 必须以 /api/v1/ 开头"), nil
		}
		capability, ok := developerRouteCapability(strings.ToUpper(input.Method), "/api"+strings.TrimPrefix(input.Path, "/api/v1"))
		if !ok || capability != expectedCapability {
			return toolError("请求路径与工具权限不匹配"), nil
		}
		if isHighImpactCapability(capability) {
			if err := s.recordDeveloperAudit(subject, "mcp", capability+":started", 0, source); err != nil {
				return toolError("操作审计不可用，已拒绝本次操作"), nil
			}
		}
		var rawBody []byte
		if len(input.Body) > 0 {
			rawBody = input.Body
		}
		if input.RawBodyBase64 != "" {
			if len(rawBody) != 0 {
				return toolError("body 与 raw_body_base64 不能同时提供"), nil
			}
			decoded, err := base64.StdEncoding.DecodeString(input.RawBodyBase64)
			if err != nil {
				return toolError("raw_body_base64 格式无效"), nil
			}
			rawBody = decoded
		}
		internalPath := "/api" + strings.TrimPrefix(input.Path, "/api/v1")
		req := httptest.NewRequest(strings.ToUpper(input.Method), internalPath, bytes.NewReader(rawBody))
		if input.ContentType != "" {
			req.Header.Set("Content-Type", input.ContentType)
		} else if len(input.Body) > 0 {
			req.Header.Set("Content-Type", "application/json")
		}
		response := httptest.NewRecorder()
		clone := req.Clone(ctx)
		clone.URL.RawQuery = strings.TrimPrefix(input.Query, "?")
		if !s.serveDeveloperSettings(response, clone, internalPath) {
			internal.ServeHTTP(response, clone)
		}
		if err := s.recordDeveloperAudit(subject, "mcp", capability, response.Code, source); err != nil && isHighImpactCapability(capability) && response.Code < 400 {
			return toolError("操作审计写入失败"), nil
		}
		responseText := response.Body.String()
		mediaType, _, _ := mime.ParseMediaType(response.Header().Get("Content-Type"))
		if mediaType != "" && mediaType != "application/json" && !strings.HasPrefix(mediaType, "text/") {
			encoded, _ := json.Marshal(map[string]string{"content_type": mediaType, "encoding": "base64", "data": base64.StdEncoding.EncodeToString(response.Body.Bytes())})
			responseText = string(encoded)
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: responseText}}, IsError: response.Code >= 400}, nil
	}
}

func (s *Server) serveDeveloperSettings(w http.ResponseWriter, r *http.Request, path string) bool {
	if path != "/api/settings" {
		return false
	}
	db := s.runtimeDatabase()
	if db == nil {
		s.writeDeveloperError(w, http.StatusInternalServerError, "服务暂不可用")
		return true
	}
	if r.Method == http.MethodGet {
		settings, err := db.GetSystemSettings()
		if err != nil {
			s.writeDeveloperError(w, http.StatusInternalServerError, "读取设置失败")
			return true
		}
		s.jsonSuccess(w, map[string]any{
			"auto_refresh":                 settings.AutoRefresh,
			"refresh_interval":             settings.RefreshInterval,
			"auto_load_plugins":            settings.AutoLoadPlugins,
			"script_task_concurrent_limit": settings.ScriptTaskConcurrentLimit,
			"points_unit":                  settings.PointsUnit,
		})
		return true
	}
	if r.Method != http.MethodPut {
		return false
	}
	var payload map[string]json.RawMessage
	if decodeDeveloperJSON(r, &payload) != nil {
		s.writeDeveloperError(w, http.StatusBadRequest, "请求参数无效")
		return true
	}
	allowed := map[string]bool{
		"auto_refresh":                 true,
		"refresh_interval":             true,
		"auto_load_plugins":            true,
		"script_task_concurrent_limit": true,
		"points_unit":                  true,
	}
	for key := range payload {
		if !allowed[key] {
			s.writeDeveloperError(w, http.StatusForbidden, "此设置属于管理员安全控制面，不能通过开发者接口修改")
			return true
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		s.writeDeveloperError(w, http.StatusBadRequest, "请求参数无效")
		return true
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	return false
}

func (s *Server) recordDeveloperAudit(subject *developerSubject, protocol, capability string, status int, r *http.Request) error {
	if subject == nil {
		return nil
	}
	appID := ""
	actor := protocol
	if subject.app != nil {
		appID = subject.app.ID
	} else if subject.admin {
		actor = "admin:" + protocol
	}
	if r != nil && r.RemoteAddr != "" {
		actor += ":" + r.RemoteAddr
	}
	return s.runtimeDatabase().RecordDeveloperAppAudit(appID, fmt.Sprintf("%s:%s:%d", protocol, capability, status), actor)
}

func isHighImpactCapability(capability string) bool {
	for _, item := range developerCapabilities {
		if item.Code == capability {
			return item.Risk == "high" || item.Risk == "critical"
		}
	}
	return true
}

func (s *Server) createDeveloperApp(name string, permissions []string) (*config.DeveloperApp, string, error) {
	clientID, err := randomDeveloperValue(18)
	if err != nil {
		return nil, "", err
	}
	secret, err := randomDeveloperValue(32)
	if err != nil {
		return nil, "", err
	}
	id, err := randomDeveloperValue(16)
	if err != nil {
		return nil, "", err
	}
	app := &config.DeveloperApp{ID: id, Name: name, ClientID: "ab_" + clientID, SecretHash: hashDeveloperSecret(secret), Permissions: permissions, CredentialVersion: 1, Status: "active"}
	if err := s.runtimeDatabase().CreateDeveloperApp(*app); err != nil {
		return nil, "", err
	}
	app.SecretHash = ""
	return app, secret, nil
}

func (s *Server) authenticateDeveloperBearer(header string) (*developerSubject, int, string) {
	header = strings.TrimSpace(header)
	if header == "" || strings.HasPrefix(strings.ToLower(header), "basic ") {
		return nil, http.StatusUnauthorized, "缺少授权令牌"
	}
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return nil, http.StatusUnauthorized, "缺少授权令牌"
	}
	token := strings.TrimSpace(header[7:])
	if token == "" {
		return nil, http.StatusUnauthorized, "缺少授权令牌"
	}
	if s.validAdminToken("Bearer " + token) {
		return nil, http.StatusUnauthorized, "令牌无效或已过期"
	}
	claims, err := s.parseDeveloperToken(token)
	if err != nil {
		return nil, http.StatusUnauthorized, "令牌无效或已过期"
	}
	app, err := s.runtimeDatabase().GetDeveloperAppByID(claims.AppID)
	if err != nil || app.CredentialVersion != claims.CredentialVersion {
		return nil, http.StatusUnauthorized, "令牌无效或已过期"
	}
	_ = s.runtimeDatabase().TouchDeveloperApp(app.ID)
	return &developerSubject{app: app}, 0, ""
}

func (s *Server) authenticateMCP(header string) (*developerSubject, int, string) {
	header = strings.TrimSpace(header)
	if strings.HasPrefix(strings.ToLower(header), "basic ") {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(header[6:]))
		if err != nil {
			return nil, http.StatusUnauthorized, "令牌无效或已过期"
		}
		clientID, secret, ok := strings.Cut(string(decoded), ":")
		if !ok {
			return nil, http.StatusUnauthorized, "令牌无效或已过期"
		}
		app, err := s.runtimeDatabase().GetDeveloperAppByClientID(clientID)
		if err != nil || !subtleCompare(hashDeveloperSecret(secret), app.SecretHash) {
			return nil, http.StatusUnauthorized, "令牌无效或已过期"
		}
		_ = s.runtimeDatabase().TouchDeveloperApp(app.ID)
		return &developerSubject{app: app}, 0, ""
	}
	if strings.HasPrefix(strings.ToLower(header), "bearer ") && s.validAdminToken(header) {
		return &developerSubject{admin: true, adminName: "admin"}, 0, ""
	}
	return s.authenticateDeveloperBearer(header)
}

func (s *Server) visibleCapabilities(subject *developerSubject) []developerCapability {
	if subject != nil && subject.admin {
		return append([]developerCapability(nil), developerCapabilities...)
	}
	result := make([]developerCapability, 0, len(developerCapabilities))
	for _, capability := range developerCapabilities {
		if developerSubjectAllows(subject, capability.Code) {
			result = append(result, capability)
		}
	}
	return result
}

func validDeveloperPermissions(permissions []string) bool {
	if len(permissions) > 1 {
		for _, permission := range permissions {
			if permission == "*" {
				return false
			}
		}
	}
	known := make(map[string]bool, len(developerCapabilities)*2)
	for _, capability := range developerCapabilities {
		known[capability.Code] = true
		known[developerCapabilityModule(capability.Code)] = true
	}
	seen := make(map[string]bool, len(permissions))
	for _, permission := range permissions {
		if permission != "*" && !known[permission] || seen[permission] {
			return false
		}
		seen[permission] = true
	}
	return true
}

func developerCapabilityModule(capability string) string {
	module, _, _ := strings.Cut(capability, ":")
	return module
}

func normalizeDeveloperPermissions(permissions []string) []string {
	result := make([]string, 0, len(permissions))
	seen := make(map[string]bool, len(permissions))
	for _, permission := range permissions {
		if permission != "*" {
			permission = developerCapabilityModule(permission)
		}
		if !seen[permission] {
			seen[permission] = true
			result = append(result, permission)
		}
	}
	return result
}

func developerPermissionModules() []developerCapability {
	modules := make([]developerCapability, 0)
	indexes := make(map[string]int)
	for _, capability := range developerCapabilities {
		code := developerCapabilityModule(capability.Code)
		if index, exists := indexes[code]; exists {
			if capability.Risk == "critical" || capability.Risk == "high" {
				modules[index].Risk = "high"
			}
			continue
		}
		indexes[code] = len(modules)
		modules = append(modules, developerCapability{
			Code:        code,
			Module:      capability.Module,
			Action:      "全部接口",
			Description: "允许调用此模块下的全部已开放接口",
			Risk:        map[bool]string{true: "high", false: "low"}[capability.Risk == "critical" || capability.Risk == "high"],
		})
	}
	return modules
}

func developerSubjectAllows(subject *developerSubject, capability string) bool {
	if subject == nil || subject.app == nil {
		return false
	}
	for _, permission := range subject.app.Permissions {
		if permission == "*" || permission == capability || permission == developerCapabilityModule(capability) {
			return true
		}
	}
	return false
}

func developerRouteCapability(method, path string) (string, bool) {
	path = strings.TrimSuffix(path, "/")
	if path == "/api/settings/password" || strings.HasPrefix(path, "/api/open/") || strings.HasPrefix(path, "/api/open/web-chat") {
		return "", false
	}
	segments := strings.Split(strings.TrimPrefix(path, "/api/"), "/")
	if len(segments) == 0 {
		return "", false
	}
	root := segments[0]
	module := ""
	read, create, update, remove := http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete
	if method == http.MethodPatch {
		method = update
	}
	switch root {
	case "plugins", "plugin", "plugin-web":
		module = "plugins"
	case "adapters", "adapter-platforms":
		module = "adapters"
	case "users", "user-accounts":
		module = "users"
	case "replies":
		module = "replies"
	case "scheduled-tasks", "script-tasks", "plugin-tasks":
		module = "tasks"
	case "script-envs":
		module = "script_envs"
	case "dependencies", "runtime-profiles":
		module = "dependencies"
	case "payments":
		module = "payments"
	case "images":
		module = "images"
	case "logs":
		module = "logs"
	case "backups":
		module = "backups"
	case "data":
		module = "data"
	case "statistics":
		module = "statistics"
	case "sdk":
		module = "sdk"
	case "open-apis", "openapis":
		module = "open_apis"
	case "system":
		module = "system"
	case "settings":
		module = "settings"
	default:
		return "", false
	}
	action := "read"
	if method == create {
		action = "create"
	}
	if method == update {
		action = "update"
	}
	if method == remove {
		action = "delete"
	}
	collectionPath := path == "/api/"+root || module == "replies" && path == "/api/replies/keywords"
	if method == create && !collectionPath {
		action = "update"
	}
	if module == "system" && strings.Contains(path, "/upgrade") {
		action = "update"
	}
	for _, highRisk := range []string{"/execute", "/listen", "/run", "/action", "/stop"} {
		if strings.Contains(path, highRisk) {
			action = "execute"
		}
	}
	if module == "plugins" && (strings.Contains(path, "/import") || strings.Contains(path, "/preview") || strings.Contains(path, "/validate")) {
		action = "create"
	}
	if (module == "script_envs" || module == "images") && method != read {
		action = "update"
	}
	if (module == "dependencies" && (strings.Contains(path, "/init") || strings.Contains(path, "/test") || strings.Contains(path, "/install"))) || module == "dependencies" && method != read {
		action = "update"
	}
	if module == "backups" && strings.Contains(path, "/restore") {
		action = "restore"
	}
	if module == "logs" && (strings.Contains(path, "/cleanup") || strings.Contains(path, "/settings")) {
		action = "cleanup"
	}
	if module == "data" && (strings.Contains(path, "/export") || strings.Contains(path, "/tables") && method == read) {
		action = "read"
	}
	if module == "data" && strings.Contains(path, "/import") {
		action = "update"
	}
	if (module == "data" || module == "script_envs" || module == "payments" || module == "users" || module == "open_apis" || module == "images") && method != read {
		action = "update"
	}
	if module == "plugins" && strings.HasPrefix(path, "/api/plugin-web/") && method == create {
		action = "execute"
	}
	if method != read && method != create && method != update && method != remove {
		return "", false
	}
	code := module + ":" + action
	known := false
	for _, capability := range developerCapabilities {
		if capability.Code == code {
			known = true
			break
		}
	}
	if !known {
		return "", false
	}
	return code, true
}

func (s *Server) issueDeveloperToken(app *config.DeveloperApp) (string, error) {
	secret, err := s.developerTokenKey()
	if err != nil {
		return "", err
	}
	nonce, err := randomDeveloperValue(16)
	if err != nil {
		return "", err
	}
	now := time.Now()
	claims, err := json.Marshal(developerTokenClaims{AppID: app.ID, CredentialVersion: app.CredentialVersion, IssuedAt: now.Unix(), ExpiresAt: now.Add(developerTokenLifetime).Unix(), Nonce: nonce})
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(claims)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte("v1." + payload))
	return "v1." + payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (s *Server) parseDeveloperToken(token string) (*developerTokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != "v1" {
		return nil, errors.New("invalid token")
	}
	secret, err := s.developerTokenKey()
	if err != nil {
		return nil, err
	}
	provided, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, err
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return nil, errors.New("invalid token")
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var claims developerTokenClaims
	if err := json.Unmarshal(data, &claims); err != nil {
		return nil, err
	}
	if claims.AppID == "" || claims.ExpiresAt <= time.Now().Unix() {
		return nil, errors.New("expired token")
	}
	return &claims, nil
}

func (s *Server) developerTokenKey() ([]byte, error) {
	s.developerSigningKeyMu.Lock()
	defer s.developerSigningKeyMu.Unlock()
	if len(s.developerSigningKey) > 0 {
		return append([]byte(nil), s.developerSigningKey...), nil
	}
	db := s.runtimeDatabase()
	if db == nil {
		return nil, errors.New("database unavailable")
	}
	const setting = "developer_api.signing_key"
	key, err := db.GetSetting(setting)
	if err == nil && key != "" {
		decoded, decodeErr := hex.DecodeString(key)
		if decodeErr != nil || len(decoded) != 32 {
			return nil, errors.New("invalid signing key")
		}
		s.developerSigningKey = decoded
		return append([]byte(nil), decoded...), nil
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		return nil, err
	}
	raw := hex.EncodeToString(rawBytes)
	decoded, err := hex.DecodeString(raw)
	if err != nil {
		return nil, err
	}
	if err := db.SetSetting(setting, raw, "开发者应用令牌签名密钥"); err != nil {
		return nil, err
	}
	s.developerSigningKey = decoded
	return append([]byte(nil), decoded...), nil
}

func randomDeveloperValue(byteCount int) (string, error) {
	data := make([]byte, byteCount)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func hashDeveloperSecret(secret string) string {
	hash := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(hash[:])
}

func subtleCompare(a, b string) bool { return hmac.Equal([]byte(a), []byte(b)) }

func decodeDeveloperJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	data, err := io.ReadAll(io.LimitReader(r.Body, (10<<20)+1))
	if err != nil || len(data) > 10<<20 {
		return errors.New("invalid body size")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("unexpected trailing JSON")
	}
	return nil
}

func developerAppDBError(s *Server, w http.ResponseWriter, err error) {
	if config.IsDeveloperAppNotFound(err) {
		s.jsonError(w, "应用不存在", http.StatusNotFound)
		return
	}
	s.jsonError(w, "应用操作失败", http.StatusInternalServerError)
}

func (s *Server) writeDeveloperError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func toolError(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: message}}, IsError: true}
}

func (s *Server) jsonSuccess(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(jsonEnvelope{Code: 0, Msg: "success", Data: data})
}
