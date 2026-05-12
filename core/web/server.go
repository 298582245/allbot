package web

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/plugin"
	"github.com/allbot/allbot/core/router"
	"github.com/allbot/allbot/core/utils"
)

// Server Web 服务器
type Server struct {
	port           string
	pluginManager  *plugin.Manager
	router         *router.Router
	adapterManager *config.AdapterManager
	logManager     *LogManager
	adminUsername  string
	adminPassword  string
	startTime      time.Time
}

// NewServer 创建 Web 服务器
func NewServer(port string, pluginManager *plugin.Manager, router *router.Router, adapterManager *config.AdapterManager, username, password string) *Server {
	return &Server{
		port:           port,
		pluginManager:  pluginManager,
		router:         router,
		adapterManager: adapterManager,
		logManager:     NewLogManager(500),
		adminUsername:  username,
		adminPassword:  password,
		startTime:      time.Now(),
	}
}

// GetLogManager 获取日志管理器
func (s *Server) GetLogManager() *LogManager {
	return s.logManager
}

// Start 启动 Web 服务器
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// API 路由
	mux.HandleFunc("/api/login", s.handleLogin)
	mux.HandleFunc("/api/plugins", s.handlePlugins)
	mux.HandleFunc("/api/plugins/", s.handlePluginDetail)
	mux.HandleFunc("/api/system/status", s.handleSystemStatus)

	// 配置管理 API
	mux.HandleFunc("/api/adapters", s.handleAdapters)
	mux.HandleFunc("/api/adapters/", s.handleAdapterDetail)

	// 日志 API
	mux.HandleFunc("/api/logs", s.handleLogs)

	// 插件 API（供插件调用）
	mux.HandleFunc("/api/plugin/listen", s.handlePluginListen)

	// 插件配置 API
	mux.HandleFunc("/api/plugins/config/", s.handlePluginConfig)

	// 依赖管理 API
	mux.HandleFunc("/api/dependencies", s.handleDependencies)
	mux.HandleFunc("/api/dependencies/", s.handleDependencyDetail)

	// 插件代码编辑 API
	mux.HandleFunc("/api/plugins/code/", s.handlePluginCode)

	// 静态文件服务（assets目录）
	fs := http.FileServer(http.Dir("web"))
	mux.Handle("/assets/", fs)

	// 首页
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
		// 扫描插件目录，显示所有插件（包括加载失败的）
		result := make([]map[string]interface{}, 0)

		entries, err := os.ReadDir("plugins")
		if err != nil {
			s.jsonError(w, "读取插件目录失败: "+err.Error(), http.StatusInternalServerError)
			return
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			pluginID := entry.Name()
			configPath := filepath.Join("plugins", pluginID, "plugin.json")

			// 尝试读取 plugin.json
			data, err := os.ReadFile(configPath)
			if err != nil {
				// 配置文件不存在或读取失败
				result = append(result, map[string]interface{}{
					"id":        pluginID,
					"name":      pluginID,
					"version":   "unknown",
					"runtime":   "unknown",
					"status":    "error",
					"port":      0,
					"trigger":   "",
					"platforms": []string{},
					"enabled":   false,
					"error":     "配置文件不存在或读取失败",
				})
				continue
			}

			var config map[string]interface{}
			if err := json.Unmarshal(data, &config); err != nil {
				// 配置文件解析失败
				result = append(result, map[string]interface{}{
					"id":        pluginID,
					"name":      pluginID,
					"version":   "unknown",
					"runtime":   "unknown",
					"status":    "error",
					"port":      0,
					"trigger":   "",
					"platforms": []string{},
					"enabled":   false,
					"error":     "配置文件解析失败",
				})
				continue
			}

			// 从配置文件读取信息
			name := pluginID
			if n, ok := config["name"].(string); ok {
				name = n
			}

			version := "unknown"
			if v, ok := config["version"].(string); ok {
				version = v
			}

			runtime := "unknown"
			if r, ok := config["runtime"].(string); ok {
				runtime = r
			}

			trigger := ""
			if t, ok := config["trigger"].(string); ok {
				trigger = t
			}

			enabled := true
			if e, ok := config["enabled"].(bool); ok {
				enabled = e
			}

			platforms := []string{}
			if p, ok := config["platforms"].([]interface{}); ok {
				for _, platform := range p {
					if ps, ok := platform.(string); ok {
						platforms = append(platforms, ps)
					}
				}
			}

			// 检查插件是否已加载
			process := s.pluginManager.GetPlugin(pluginID)
			status := "ready"
			port := 0
			if process != nil {
				status = process.Status
				port = process.Port
			}

			result = append(result, map[string]interface{}{
				"id":        pluginID,
				"name":      name,
				"version":   version,
				"runtime":   runtime,
				"status":    status,
				"port":      port,
				"trigger":   trigger,
				"platforms": platforms,
				"enabled":   enabled,
			})
		}

		s.jsonResponse(w, result)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePluginDetail 插件详情接口
func (s *Server) handlePluginDetail(w http.ResponseWriter, r *http.Request) {
	// 提取插件 ID
	pluginID := strings.TrimPrefix(r.URL.Path, "/api/plugins/")
	if pluginID == "" {
		s.jsonError(w, "插件 ID 不能为空", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodPost {
		// 控制插件（启动/停止）
		var req struct {
			Action string `json:"action"` // start, stop, restart
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}

		switch req.Action {
		case "enable":
			// 启用插件
			if err := s.pluginManager.TogglePlugin(pluginID, true); err != nil {
				s.logManager.AddLog("error", fmt.Sprintf("启用插件失败 %s: %v", pluginID, err))
				s.jsonError(w, "启用插件失败: "+err.Error(), http.StatusInternalServerError)
				return
			}
			s.logManager.AddLog("info", fmt.Sprintf("启用插件: %s", pluginID))
			s.jsonResponse(w, map[string]interface{}{
				"message": "插件已启用",
			})
		case "disable":
			// 禁用插件
			if err := s.pluginManager.TogglePlugin(pluginID, false); err != nil {
				s.logManager.AddLog("error", fmt.Sprintf("禁用插件失败 %s: %v", pluginID, err))
				s.jsonError(w, "禁用插件失败: "+err.Error(), http.StatusInternalServerError)
				return
			}
			s.logManager.AddLog("info", fmt.Sprintf("禁用插件: %s", pluginID))
			s.jsonResponse(w, map[string]interface{}{
				"message": "插件已禁用",
			})
case "reload":		// 重新加载插件		if err := s.pluginManager.ReloadPlugin(pluginID); err != nil {			s.logManager.AddLog("error", fmt.Sprintf("重新加载插件失败 %s: %v", pluginID, err))			s.jsonError(w, "重新加载插件失败: "+err.Error(), http.StatusInternalServerError)			return		}		// 重新注册到路由器		plugin := s.pluginManager.GetPlugin(pluginID)		if plugin != nil && plugin.Plugin != nil {			if err := s.router.RegisterPlugin(plugin.Plugin); err != nil {				s.logManager.AddLog("error", fmt.Sprintf("重新注册插件失败 %s: %v", pluginID, err))				s.jsonError(w, "重新注册插件失败: "+err.Error(), http.StatusInternalServerError)				return			}		}		s.logManager.AddLog("info", fmt.Sprintf("重新加载插件: %s", pluginID))		s.jsonResponse(w, map[string]interface{}{			"message": "插件已重新加载",		})
		case "start":
			// 启动插件（兼容旧版，现在无意义）
			s.jsonResponse(w, map[string]interface{}{
				"message": "插件采用按需执行模式，无需手动启动",
			})
		case "stop":
			// 停止插件（兼容旧版，现在无意义）
			s.jsonResponse(w, map[string]interface{}{
				"message": "插件采用按需执行模式，无需手动停止",
			})
		case "restart":
			// 重启插件（兼容旧版，现在无意义）
			s.jsonResponse(w, map[string]interface{}{
				"message": "插件采用按需执行模式，无需重启",
			})
			s.pluginManager.StopPlugin(pluginID)
			time.Sleep(500 * time.Millisecond) // 等待进程完全停止
			if err := s.pluginManager.StartPluginByID(pluginID); err != nil {
				s.logManager.AddLog("error", fmt.Sprintf("重启插件失败 %s: %v", pluginID, err))
				s.jsonError(w, "重启插件失败: "+err.Error(), http.StatusInternalServerError)
				return
			}
			s.logManager.AddLog("info", fmt.Sprintf("重启插件: %s", pluginID))
			s.jsonResponse(w, map[string]interface{}{
				"message": "插件重启成功",
			})
		default:
			s.jsonError(w, "不支持的操作: "+req.Action, http.StatusBadRequest)
		}
	} else if r.Method == http.MethodDelete {
		// 删除插件
		if err := s.pluginManager.StopPlugin(pluginID); err != nil {
			log.Printf("警告：停止插件失败: %v", err)
		}
		// TODO: 实现删除插件文件
		s.logManager.AddLog("info", fmt.Sprintf("删除插件: %s", pluginID))
		s.jsonResponse(w, map[string]interface{}{
			"message": "插件删除成功",
		})
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
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

	uptime := time.Since(s.startTime)
	uptimeStr := formatDuration(uptime)

	s.jsonResponse(w, map[string]interface{}{
		"uptime":       uptimeStr,
		"pluginCount":  len(plugins),
		"runningCount": runningCount,
		"messageCount": 0, // TODO: 统计消息数
	})
}

// formatDuration 格式化时长
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// handleAdapters 适配器列表接口
func (s *Server) handleAdapters(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// 获取所有适配器配置
		adapters, err := s.adapterManager.GetDatabase().GetAllAdapters()
		if err != nil {
			s.jsonError(w, "获取适配器配置失败: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// 获取运行状态
		runningAdapters := s.adapterManager.GetAllAdapters()
		result := make([]map[string]interface{}, 0, len(adapters))

		for _, adapter := range adapters {
			_, isRunning := runningAdapters[adapter.Platform]
			result = append(result, map[string]interface{}{
				"id":         adapter.ID,
				"platform":   adapter.Platform,
				"enabled":    adapter.Enabled,
				"config":     utils.MaskSensitiveConfig(adapter.Config),
				"running":    isRunning,
				"created_at": adapter.CreatedAt,
				"updated_at": adapter.UpdatedAt,
			})
		}

		s.jsonResponse(w, result)

	} else if r.Method == http.MethodPost {
		// 创建或更新适配器配置
		var req struct {
			Platform string                 `json:"platform"`
			Enabled  bool                   `json:"enabled"`
			Config   map[string]interface{} `json:"config"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// 保存配置并重新加载
		if err := s.adapterManager.SaveAdapterConfig(req.Platform, req.Enabled, req.Config); err != nil {
			s.jsonError(w, "保存配置失败: "+err.Error(), http.StatusInternalServerError)
			return
		}

		s.jsonResponse(w, map[string]interface{}{
			"message": "配置已保存并生效",
		})

	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAdapterDetail 适配器详情接口
func (s *Server) handleAdapterDetail(w http.ResponseWriter, r *http.Request) {
	// 提取平台名称
	platform := r.URL.Path[len("/api/adapters/"):]
	if platform == "" {
		s.jsonError(w, "平台名称不能为空", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodGet {
		// 获取适配器配置
		adapter, err := s.adapterManager.GetDatabase().GetAdapter(platform)
		if err != nil {
			s.jsonError(w, "获取配置失败: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if adapter == nil {
			s.jsonError(w, "配置不存在", http.StatusNotFound)
			return
		}

		// 检查运行状态
		isRunning := s.adapterManager.GetAdapter(platform) != nil

		s.jsonResponse(w, map[string]interface{}{
			"id":         adapter.ID,
			"platform":   adapter.Platform,
			"enabled":    adapter.Enabled,
			"config":     utils.MaskSensitiveConfig(adapter.Config),
			"running":    isRunning,
			"created_at": adapter.CreatedAt,
			"updated_at": adapter.UpdatedAt,
		})

	} else if r.Method == http.MethodDelete {
		// 删除适配器配置
		// 先停止适配器
		if err := s.adapterManager.StopAdapter(platform); err != nil {
			log.Printf("警告：停止适配器失败: %v", err)
		}

		// 删除配置
		if err := s.adapterManager.GetDatabase().DeleteAdapter(platform); err != nil {
			s.jsonError(w, "删除配置失败: "+err.Error(), http.StatusInternalServerError)
			return
		}

		s.jsonResponse(w, map[string]interface{}{
			"message": "配置已删除",
		})

	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleIndex 首页
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	// 读取 HTML 文件
	htmlPath := "web/index.html"
	htmlContent, err := os.ReadFile(htmlPath)
	if err != nil {
		// 如果文件不存在，返回简单的 HTML
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
    <p>Web UI 文件未找到，请确保 web/index.html 存在</p>
    <p>API 端点：</p>
    <ul>
        <li>POST /api/login - 登录</li>
        <li>GET /api/plugins - 插件列表</li>
        <li>GET /api/system/status - 系统状态</li>
    </ul>
</body>
</html>
		`))
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write(htmlContent)
}

// authMiddleware 认证中间件
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 只对 API 路径进行认证检查（除了登录接口）
		if strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/api/login" {
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

// handlePluginListen 处理插件的listen请求
func (s *Server) handlePluginListen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		PluginID string `json:"plugin_id"`
		UserID   string `json:"user_id"`
		GroupID  string `json:"group_id"`
		Timeout  int    `json:"timeout"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// 通过router获取sessionManager
	sessionManager := s.router.GetSessionManager()
	if sessionManager == nil {
		s.jsonError(w, "Session manager not available", http.StatusInternalServerError)
		return
	}

	// 创建等待会话
	ch := sessionManager.CreateSession(req.PluginID, req.UserID, req.GroupID, req.Timeout)

	// 等待消息或超时
	content := ""
	select {
	case msg, ok := <-ch:
		if ok {
			content = msg
		}
	}

	s.jsonResponse(w, map[string]interface{}{
		"content": content,
	})
}

// handlePluginConfig 处理插件配置请求
func (s *Server) handlePluginConfig(w http.ResponseWriter, r *http.Request) {
	// 提取插件 ID
	pluginID := strings.TrimPrefix(r.URL.Path, "/api/plugins/config/")
	if pluginID == "" {
		s.jsonError(w, "插件 ID 不能为空", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodGet {
		// 获取插件配置
		configPath := filepath.Join("plugins", pluginID, "plugin.json")
		data, err := os.ReadFile(configPath)
		if err != nil {
			s.jsonError(w, "读取配置失败: "+err.Error(), http.StatusInternalServerError)
			return
		}

		var config map[string]interface{}
		if err := json.Unmarshal(data, &config); err != nil {
			s.jsonError(w, "解析配置失败: "+err.Error(), http.StatusInternalServerError)
			return
		}

		s.jsonResponse(w, config)

	} else if r.Method == http.MethodPut {
		// 更新插件配置
		var config map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			s.jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// 写入配置文件
		configPath := filepath.Join("plugins", pluginID, "plugin.json")
		data, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			s.jsonError(w, "序列化配置失败: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := os.WriteFile(configPath, data, 0644); err != nil {
			s.jsonError(w, "写入配置失败: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// 自动重新加载插件
		if err := s.pluginManager.ReloadPlugin(pluginID); err != nil {
			s.logManager.AddLog("error", fmt.Sprintf("重新加载插件失败 %s: %v", pluginID, err))
			s.jsonError(w, "重新加载插件失败: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// 重新注册到路由器
		plugin := s.pluginManager.GetPlugin(pluginID)
		if plugin != nil && plugin.Plugin != nil {
			if err := s.router.RegisterPlugin(plugin.Plugin); err != nil {
				s.logManager.AddLog("error", fmt.Sprintf("重新注册插件失败 %s: %v", pluginID, err))
				s.jsonError(w, "重新注册插件失败: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		s.logManager.AddLog("info", fmt.Sprintf("更新插件配置: %s", pluginID))
		s.jsonResponse(w, map[string]interface{}{
			"message": "配置已更新并生效",
		})

	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleDependencies 处理依赖管理请求
func (s *Server) handleDependencies(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// 获取所有依赖
		pythonDeps, err := s.pluginManager.GetDepsManager().GetPythonDeps()
		if err != nil {
			s.jsonError(w, "获取 Python 依赖失败: "+err.Error(), http.StatusInternalServerError)
			return
		}

		nodeDeps, err := s.pluginManager.GetDepsManager().GetNodeDeps()
		if err != nil {
			s.jsonError(w, "获取 Node.js 依赖失败: "+err.Error(), http.StatusInternalServerError)
			return
		}

		s.jsonResponse(w, map[string]interface{}{
			"python": pythonDeps,
			"nodejs": nodeDeps,
		})

	} else if r.Method == http.MethodPost {
		// 安装新依赖
		var req struct {
			Runtime string `json:"runtime"`
			Name    string `json:"name"`
			Version string `json:"version"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// 验证输入
		if !isValidPackageName(req.Name) {
			s.jsonError(w, "无效的包名", http.StatusBadRequest)
			return
		}

		if req.Version != "" && req.Version != "latest" && !isValidVersion(req.Version) {
			s.jsonError(w, "无效的版本号", http.StatusBadRequest)
			return
		}

		// 安装依赖
		deps := map[string]string{req.Name: req.Version}
		if req.Version == "latest" || req.Version == "" {
			deps[req.Name] = ""
		}

		var err error
		if req.Runtime == "python" {
			err = s.pluginManager.GetDepsManager().InstallPythonDeps(deps)
		} else if req.Runtime == "nodejs" {
			err = s.pluginManager.GetDepsManager().InstallNodeDeps(deps)
		} else {
			s.jsonError(w, "不支持的运行时: "+req.Runtime, http.StatusBadRequest)
			return
		}

		if err != nil {
			s.logManager.AddLog("error", fmt.Sprintf("安装依赖失败 %s: %v", req.Name, err))
			s.jsonError(w, "安装依赖失败: "+err.Error(), http.StatusInternalServerError)
			return
		}

		s.logManager.AddLog("info", fmt.Sprintf("安装依赖: %s@%s (%s)", req.Name, req.Version, req.Runtime))
		s.jsonResponse(w, map[string]interface{}{
			"message": "依赖安装成功",
		})

	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleDependencyDetail 处理依赖详情请求
func (s *Server) handleDependencyDetail(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/dependencies/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		s.jsonError(w, "无效的请求路径", http.StatusBadRequest)
		return
	}

	runtime := parts[0]
	name := parts[1]

	if r.Method == http.MethodDelete {
		var err error
		if runtime == "python" {
			err = s.pluginManager.GetDepsManager().UninstallPythonDep(name)
		} else if runtime == "nodejs" {
			err = s.pluginManager.GetDepsManager().UninstallNodeDep(name)
		} else {
			s.jsonError(w, "不支持的运行时: "+runtime, http.StatusBadRequest)
			return
		}

		if err != nil {
			s.logManager.AddLog("error", fmt.Sprintf("卸载依赖失败 %s: %v", name, err))
			s.jsonError(w, "卸载依赖失败: "+err.Error(), http.StatusInternalServerError)
			return
		}

		s.logManager.AddLog("info", fmt.Sprintf("卸载依赖: %s (%s)", name, runtime))
		s.jsonResponse(w, map[string]interface{}{
			"message": "依赖卸载成功",
		})

	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// isValidPackageName 验证包名
func isValidPackageName(name string) bool {
	if len(name) == 0 || len(name) > 100 {
		return false
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' || c == '.') {
			return false
		}
	}
	return true
}

// isValidVersion 验证版本号
func isValidVersion(version string) bool {
	if len(version) == 0 || len(version) > 50 {
		return false
	}
	for _, c := range version {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' || c == '.') {
			return false
		}
	}
	return true
}

// handlePluginCode 处理插件代码编辑请求
func (s *Server) handlePluginCode(w http.ResponseWriter, r *http.Request) {
	pluginID := strings.TrimPrefix(r.URL.Path, "/api/plugins/code/")
	if pluginID == "" {
		s.jsonError(w, "插件ID不能为空", http.StatusBadRequest)
		return
	}

	pluginPath := filepath.Join("plugins", pluginID)

	if r.Method == http.MethodGet {
		// 获取插件代码
		plugin := s.router.GetPlugin(pluginID)
		if plugin == nil {
			s.jsonError(w, "插件不存在", http.StatusNotFound)
			return
		}

		entryPath := filepath.Join(pluginPath, plugin.Entry)
		code, err := os.ReadFile(entryPath)
		if err != nil {
			s.jsonError(w, "读取代码失败: "+err.Error(), http.StatusInternalServerError)
			return
		}

		s.jsonResponse(w, map[string]interface{}{
			"code":        string(code),
			"filename":    plugin.Entry,
			"plugin_name": plugin.Name,
		})

	} else if r.Method == http.MethodPut {
		// 保存插件代码
		var req struct {
			Code string `json:"code"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "无效的请求数据", http.StatusBadRequest)
			return
		}

		plugin := s.router.GetPlugin(pluginID)
		if plugin == nil {
			s.jsonError(w, "插件不存在", http.StatusNotFound)
			return
		}

		entryPath := filepath.Join(pluginPath, plugin.Entry)
		if err := os.WriteFile(entryPath, []byte(req.Code), 0644); err != nil {
			s.jsonError(w, "保存代码失败: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// 重新加载插件
		if err := s.pluginManager.ReloadPlugin(pluginID); err != nil {
			s.logManager.AddLog("error", fmt.Sprintf("重新加载插件失败 %s: %v", pluginID, err))
			s.jsonError(w, "重新加载插件失败: "+err.Error(), http.StatusInternalServerError)
			return
		}

		s.logManager.AddLog("info", fmt.Sprintf("插件代码已更新: %s", plugin.Name))
		s.jsonResponse(w, map[string]interface{}{
			"message": "代码已保存并生效",
		})

	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
