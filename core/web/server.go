package web

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/allbot/allbot/core/plugin"
	"github.com/allbot/allbot/core/router"
)

// Server Web 服务器
type Server struct {
	port          string
	pluginManager *plugin.Manager
	router        *router.Router
	adminUsername string
	adminPassword string
}

// NewServer 创建 Web 服务器
func NewServer(port string, pluginManager *plugin.Manager, router *router.Router, username, password string) *Server {
	return &Server{
		port:          port,
		pluginManager: pluginManager,
		router:        router,
		adminUsername: username,
		adminPassword: password,
	}
}

// Start 启动 Web 服务器
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// API 路由
	mux.HandleFunc("/api/login", s.handleLogin)
	mux.HandleFunc("/api/plugins", s.handlePlugins)
	mux.HandleFunc("/api/plugins/", s.handlePluginDetail)
	mux.HandleFunc("/api/system/status", s.handleSystemStatus)

	// 静态文件（Web UI）
	// TODO: 嵌入 Vue 构建产物
	mux.HandleFunc("/", s.handleIndex)

	server := &http.Server{
		Addr:    ":" + s.port,
		Handler: s.corsMiddleware(s.authMiddleware(mux)),
	}

	log.Printf("Web UI 启动: http://localhost:%s", s.port)
	return server.ListenAndServe()
}

// handleLogin 登录接口
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Username != s.adminUsername || req.Password != s.adminPassword {
		s.jsonError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// 简单实现：返回固定 token（生产环境应使用 JWT）
	s.jsonResponse(w, map[string]interface{}{
		"token": "admin-token-" + time.Now().Format("20060102"),
		"user": map[string]string{
			"username": req.Username,
		},
	})
}

// handlePlugins 插件列表接口
func (s *Server) handlePlugins(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// 获取插件列表
		plugins := s.pluginManager.GetAllPlugins()
		result := make([]map[string]interface{}, 0, len(plugins))

		for _, p := range plugins {
			result = append(result, map[string]interface{}{
				"id":       p.Plugin.ID,
				"name":     p.Plugin.Name,
				"version":  p.Plugin.Version,
				"runtime":  p.Plugin.Runtime,
				"status":   p.Status,
				"port":     p.Port,
				"trigger":  p.Plugin.Trigger,
				"platforms": p.Plugin.Platforms,
			})
		}

		s.jsonResponse(w, result)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePluginDetail 插件详情接口
func (s *Server) handlePluginDetail(w http.ResponseWriter, r *http.Request) {
	// TODO: 实现插件详情、启用/禁用/卸载
	s.jsonError(w, "Not implemented", http.StatusNotImplemented)
}

// handleSystemStatus 系统状态接口
func (s *Server) handleSystemStatus(w http.ResponseWriter, r *http.Request) {
	plugins := s.pluginManager.GetAllPlugins()
	runningCount := 0
	for _, p := range plugins {
		if p.Status == "running" {
			runningCount++
		}
	}

	s.jsonResponse(w, map[string]interface{}{
		"uptime":        time.Since(time.Now()).String(), // TODO: 记录启动时间
		"pluginCount":   len(plugins),
		"runningCount":  runningCount,
		"messageCount":  0, // TODO: 统计消息数
	})
}

// handleIndex 首页
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	// TODO: 返回 Vue 构建的 index.html
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>AllBot 管理界面</title>
</head>
<body>
    <h1>AllBot 管理界面</h1>
    <p>Web UI 开发中...</p>
    <p>API 端点：</p>
    <ul>
        <li>POST /api/login - 登录</li>
        <li>GET /api/plugins - 插件列表</li>
        <li>GET /api/system/status - 系统状态</li>
    </ul>
</body>
</html>
	`))
}

// authMiddleware 认证中间件
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 跳过登录接口和静态文件
		if r.URL.Path == "/api/login" || r.URL.Path == "/" {
			next.ServeHTTP(w, r)
			return
		}

		// 检查 Authorization header
		token := r.Header.Get("Authorization")
		if token == "" {
			s.jsonError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// 简单验证（生产环境应使用 JWT）
		if token != "Bearer admin-token-"+time.Now().Format("20060102") {
			s.jsonError(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// corsMiddleware CORS 中间件
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// jsonResponse 返回 JSON 响应
func (s *Server) jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// jsonError 返回 JSON 错误
func (s *Server) jsonError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
