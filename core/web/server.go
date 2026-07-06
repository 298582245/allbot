package web

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/allbot/allbot/core/backup"
	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/imagehost"
	"github.com/allbot/allbot/core/plugin"
	"github.com/allbot/allbot/core/router"
	"github.com/allbot/allbot/core/updater"
	"github.com/allbot/allbot/core/utils"
)

type pluginBackupFile struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	ModTime   time.Time `json:"mod_time"`
	PluginID  string    `json:"plugin_id"`
	Timestamp string    `json:"timestamp,omitempty"`
}

type WebAssetMode string

const (
	WebAssetModeEmbedded WebAssetMode = "embedded"
	WebAssetModeExternal WebAssetMode = "external"

	runtimeTotalSecondsKey               = "runtime.total_seconds"
	runtimeTotalSecondsDescription       = "AllBot 累计运行秒数"
	runtimePersistIntervalSeconds  int64 = 30
)

type Server struct {
	port                        string
	pluginManager               *plugin.Manager
	router                      *router.Router
	adapterManager              *config.AdapterManager
	logManager                  *LogManager
	startTime                   time.Time
	webFS                       fs.FS
	webAssetMode                WebAssetMode
	externalWebDir              string
	updateService               *updater.Service
	releaseClient               updater.ReleaseClient
	upgradeRunner               func(updater.ApplyUpdateRequest) error
	upgradeExit                 func()
	upgradeMu                   sync.Mutex
	upgradeState                updater.UpgradeState
	backupService               *backup.Service
	imageHostService            *imagehost.Service
	webChatMailer               webChatEmailSender
	runtimeInitJobs             *runtimeProfileInitJobStore
	runtimeMu                   sync.Mutex
	runtimeBaseSeconds          int64
	runtimeLoaded               bool
	lastPersistedRuntimeSeconds int64
	sessionMu                   sync.RWMutex
	sessions                    map[string]time.Time
	resourceMu                  sync.Mutex
	lastCPUIdle                 uint64
	lastCPUTotal                uint64
	lastCPUPercent              float64
	lastCPUAt                   time.Time
	lastProcessSystemCPUTotal   uint64
	lastProcessCPUTotal         uint64
	lastProcessCPUPercent       float64
	serverMu                    sync.Mutex
	httpServer                  *http.Server
}

func NewServer(port string, pluginManager *plugin.Manager, router *router.Router, adapterManager *config.AdapterManager, webFS fs.FS) *Server {
	updateService := updater.NewService(updater.NewGitHubClient(), updater.DefaultUpgradeRunner)
	return &Server{port: port, pluginManager: pluginManager, router: router, adapterManager: adapterManager, logManager: NewLogManager(500), startTime: time.Now(), webFS: webFS, webAssetMode: WebAssetModeEmbedded, externalWebDir: "web", updateService: updateService, releaseClient: updater.NewGitHubClient(), upgradeRunner: updater.DefaultUpgradeRunner, upgradeState: updater.UpgradeState{Status: updater.UpgradeStatusIdle, Message: "暂无升级任务"}, runtimeInitJobs: newRuntimeProfileInitJobStore(), sessions: map[string]time.Time{}}
}

func (s *Server) SetWebAssetMode(mode WebAssetMode) {
	s.webAssetMode = mode
}

func (s *Server) SetExternalWebDir(dir string) {
	s.externalWebDir = dir
}

func (s *Server) GetLogManager() *LogManager { return s.logManager }

func (s *Server) SetBackupService(service *backup.Service) {
	s.backupService = service
}

func (s *Server) SetImageHostService(service *imagehost.Service) {
	s.imageHostService = service
}

func (s *Server) SetWebChatEmailSender(sender webChatEmailSender) {
	s.webChatMailer = sender
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", s.handleLogin)
	mux.HandleFunc("/api/logout", s.handleLogout)
	mux.HandleFunc("/api/open/payments/notify/epay", s.handlePaymentNotifyEpay)
	mux.HandleFunc("/api/open/payments/return/epay", s.handlePaymentReturnEpay)
	mux.HandleFunc("/api/open/payments/qrcode/", s.handlePaymentQRCode)
	mux.HandleFunc("/api/open/payments/alipay-bill/cashier/", s.handleAlipayBillCashier)
	mux.HandleFunc("/api/open/images/", s.handleOpenImage)
	mux.HandleFunc("/api/open/web-chat/", s.handleWebChatAPI)
	mux.HandleFunc("/api/open/adapters/feishu/", s.handleFeishuAdapterCallback)
	mux.HandleFunc("/api/open/adapters/wechat-official/", s.handleWechatOfficialAdapterCallback)
	mux.HandleFunc("/api/open/", s.handleOpenAPI)
	mux.HandleFunc("/api/open-apis", s.handleOpenAPIConfigs)
	mux.HandleFunc("/api/open-apis/", s.handleOpenAPIConfigDetail)
	mux.HandleFunc("/api/openapis", s.handleOpenAPIConfigs)
	mux.HandleFunc("/api/openapis/", s.handleOpenAPIConfigDetail)
	mux.HandleFunc("/api/plugins/config/", s.handlePluginConfig)
	mux.HandleFunc("/api/plugins/files/", s.handlePluginFiles)
	mux.HandleFunc("/api/plugins/code/", s.handlePluginCode)
	mux.HandleFunc("/api/plugins/templates", s.handlePluginTemplates)
	mux.HandleFunc("/api/plugins/preview", s.handlePluginCreatePreview)
	mux.HandleFunc("/api/plugins/validate", s.handlePluginCreateValidate)
	mux.HandleFunc("/api/plugins/recycle-bin", s.handlePluginRecycleBin)
	mux.HandleFunc("/api/plugins", s.handlePlugins)
	mux.HandleFunc("/api/plugins/", s.handlePluginDetail)
	mux.HandleFunc("/api/plugin-web/panels", s.handlePluginWebPanels)
	mux.HandleFunc("/api/plugin-web/", s.handlePluginWebAPI)
	mux.HandleFunc("/plugin-web/", s.handlePluginWebStatic)
	mux.HandleFunc("/api/system/status", s.handleSystemStatus)
	mux.HandleFunc("/api/system/update/status", s.handleSystemUpdateStatus)
	mux.HandleFunc("/api/system/update/upgrade", s.handleSystemUpgrade)
	mux.HandleFunc("/api/system/update", s.handleSystemUpdate)
	mux.HandleFunc("/api/system/message-stats", s.handleMessageStats)
	mux.HandleFunc("/api/statistics/overview", s.handleStatisticsOverview)
	mux.HandleFunc("/api/statistics/message-total-trend", s.handleMessageTotalTrend)
	mux.HandleFunc("/api/statistics/plugin-trigger-trend", s.handlePluginTriggerTrend)
	mux.HandleFunc("/api/settings", s.handleSettings)
	mux.HandleFunc("/api/settings/password", s.handleChangePassword)
	mux.HandleFunc("/api/payments/settings", s.handlePaymentSettings)
	mux.HandleFunc("/api/payments/orders", s.handlePaymentOrders)
	mux.HandleFunc("/api/payments/orders/", s.handlePaymentOrderDetail)
	mux.HandleFunc("/api/images", s.handleImages)
	mux.HandleFunc("/api/images/", s.handleImageDetail)
	mux.HandleFunc("/api/adapter-platforms", s.handleAdapterPlatforms)
	mux.HandleFunc("/api/adapters", s.handleAdapters)
	mux.HandleFunc("/api/adapters/", s.handleAdapterDetail)
	mux.HandleFunc("/api/logs/settings", s.handleLogSettings)
	mux.HandleFunc("/api/logs/cleanup", s.handleLogCleanup)
	mux.HandleFunc("/api/logs", s.handleLogs)
	mux.HandleFunc("/api/scheduled-tasks", s.handleScheduledTasks)
	mux.HandleFunc("/api/scheduled-tasks/", s.handleScheduledTaskDetail)
	mux.HandleFunc("/api/script-tasks", s.handleScriptTasks)
	mux.HandleFunc("/api/script-tasks/", s.handleScriptTaskDetail)
	mux.HandleFunc("/api/script-envs", s.handleScriptEnvs)
	mux.HandleFunc("/api/script-envs/import", s.handleScriptEnvImport)
	mux.HandleFunc("/api/script-envs/", s.handleScriptEnvDetail)
	mux.HandleFunc("/api/replies/keywords", s.handleKeywordReplies)
	mux.HandleFunc("/api/replies/keywords/", s.handleKeywordReplyDetail)
	mux.HandleFunc("/api/plugin/listen", s.handlePluginListen)
	mux.HandleFunc("/api/dependencies", s.handleDependencies)
	mux.HandleFunc("/api/dependencies/", s.handleDependencyDetail)
	mux.HandleFunc("/api/runtime-profiles", s.handleRuntimeProfiles)
	mux.HandleFunc("/api/runtime-profiles/download-candidates", s.handleRuntimeProfileDownloadCandidates)
	mux.HandleFunc("/api/runtime-profiles/download-settings", s.handleRuntimeProfileDownloadSettings)
	mux.HandleFunc("/api/runtime-profiles/init", s.handleRuntimeProfileInit)
	mux.HandleFunc("/api/runtime-profiles/init/", s.handleRuntimeProfileInitJob)
	mux.HandleFunc("/api/runtime-profiles/status", s.handleRuntimeProfileStatus)
	mux.HandleFunc("/api/runtime-profiles/test", s.handleRuntimeProfileTest)
	mux.HandleFunc("/api/backups", s.handleBackups)
	mux.HandleFunc("/api/backups/", s.handleBackupDetail)
	mux.HandleFunc("/api/sdk/files", s.handleSDKFiles)
	mux.HandleFunc("/api/sdk/reference", s.handleSDKReference)
	mux.HandleFunc("/api/data/tables", s.handleDataTables)
	mux.HandleFunc("/api/data/tables/", s.handleDataTableDetail)
	mux.HandleFunc("/api/data/views", s.handleDataViews)
	mux.HandleFunc("/api/data/export", s.handleDataExport)
	mux.HandleFunc("/api/data/import", s.handleDataImport)
	mux.Handle("/assets/", s.webAssetsHandler())
	mux.HandleFunc("/chat", s.handleChatIndex)
	mux.HandleFunc("/chat/", s.handleChatIndex)
	mux.HandleFunc("/login", s.handleIndex)
	mux.HandleFunc("/login/", s.handleAccessCodeEntry)
	mux.HandleFunc("/", s.handleIndex)

	server := &http.Server{Addr: ":" + s.port, Handler: s.corsMiddleware(s.authMiddleware(mux))}
	s.serverMu.Lock()
	s.httpServer = server
	s.serverMu.Unlock()
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) runtimeSeconds() (int64, int64) {
	currentSeconds := int64(time.Since(s.startTime).Seconds())
	if currentSeconds < 0 {
		currentSeconds = 0
	}
	baseSeconds := s.runtimeBase()
	totalSeconds := baseSeconds + currentSeconds
	if totalSeconds-s.lastPersistedRuntimeSeconds >= runtimePersistIntervalSeconds {
		s.persistRuntimeSeconds(totalSeconds)
	}
	return totalSeconds, currentSeconds
}

func (s *Server) runtimeBase() int64 {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	if !s.runtimeLoaded {
		s.runtimeBaseSeconds = s.loadPersistedRuntimeSeconds()
		s.lastPersistedRuntimeSeconds = s.runtimeBaseSeconds
		s.runtimeLoaded = true
	}
	return s.runtimeBaseSeconds
}

func (s *Server) loadPersistedRuntimeSeconds() int64 {
	database := s.runtimeDatabase()
	if database == nil {
		return s.lastPersistedRuntimeSeconds
	}
	value, err := database.GetSetting(runtimeTotalSecondsKey)
	if err != nil {
		return s.lastPersistedRuntimeSeconds
	}
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || seconds < 0 {
		return 0
	}
	return seconds
}

func (s *Server) persistRuntimeSeconds(totalSeconds int64) {
	if totalSeconds <= 0 {
		return
	}
	database := s.runtimeDatabase()
	if database == nil {
		s.runtimeMu.Lock()
		s.lastPersistedRuntimeSeconds = totalSeconds
		s.runtimeMu.Unlock()
		return
	}
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	if totalSeconds <= s.lastPersistedRuntimeSeconds {
		return
	}
	if err := database.SetSetting(runtimeTotalSecondsKey, strconv.FormatInt(totalSeconds, 10), runtimeTotalSecondsDescription); err == nil {
		s.lastPersistedRuntimeSeconds = totalSeconds
	}
}

func (s *Server) runtimeDatabase() *config.Database {
	if s.adapterManager == nil {
		return nil
	}
	return s.adapterManager.GetDatabase()
}

func (s *Server) Shutdown(ctx context.Context) error {
	totalSeconds, _ := s.runtimeSeconds()
	s.persistRuntimeSeconds(totalSeconds)
	s.serverMu.Lock()
	server := s.httpServer
	s.serverMu.Unlock()
	if server == nil {
		return nil
	}
	return server.Shutdown(ctx)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct{ Username, Password string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}
	username, err := s.currentAdminUsername()
	if err != nil {
		s.jsonError(w, "读取管理员配置失败", http.StatusInternalServerError)
		return
	}
	ok, err := s.adapterManager.GetDatabase().VerifyAdminPassword(req.Password)
	if err != nil {
		s.jsonError(w, "校验管理员密码失败", http.StatusInternalServerError)
		return
	}
	if req.Username != username || !ok {
		s.jsonError(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	token, err := s.createAdminSession()
	if err != nil {
		s.jsonError(w, "创建登录会话失败", http.StatusInternalServerError)
		return
	}
	s.setSessionCookie(w, token)
	s.jsonResponse(w, map[string]interface{}{"token": token, "user": map[string]string{"username": req.Username}})
}

func (s *Server) currentAdminUsername() (string, error) {
	username, err := s.adapterManager.GetDatabase().GetSetting("admin.username")
	if err != nil || strings.TrimSpace(username) == "" {
		return "admin", err
	}
	return username, nil
}

func (s *Server) handlePlugins(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.handleCreatePlugin(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	entries, err := os.ReadDir("plugins")
	if err != nil {
		s.jsonError(w, "读取插件目录失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	result := make([]map[string]interface{}, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pluginID := entry.Name()
		configData, err := os.ReadFile(filepath.Join("plugins", pluginID, "plugin.json"))
		if err != nil {
			result = append(result, pluginError(pluginID, "配置文件不存在或读取失败"))
			continue
		}
		var config map[string]interface{}
		if err := json.Unmarshal(configData, &config); err != nil {
			result = append(result, pluginError(pluginID, "配置文件解析失败"))
			continue
		}
		process := s.pluginManager.GetPlugin(pluginID)
		status, port := "ready", 0
		if process != nil {
			status, port = process.Status, process.Port
		}
		result = append(result, map[string]interface{}{
			"id":                  pluginID,
			"name":                stringValue(config, "name", pluginID),
			"version":             stringValue(config, "version", "unknown"),
			"runtime":             stringValue(config, "runtime", "unknown"),
			"runtime_profile":     stringValue(config, "runtime_profile", ""),
			"status":              status,
			"port":                port,
			"trigger":             stringValue(config, "trigger", ""),
			"priority":            intValue(config, "priority", 0),
			"pinned":              boolValue(config, "pinned", false),
			"platforms":           stringSliceValue(config, "platforms"),
			"allowed_adapter_ids": stringSliceValue(config, "allowed_adapter_ids"),
			"user_config_schema":  config["user_config_schema"],
			"user_config":         config["user_config"],
			"enabled":             boolValue(config, "enabled", true),
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		return boolValue(result[i], "pinned", false) && !boolValue(result[j], "pinned", false)
	})
	s.jsonResponse(w, result)
}

func pluginError(pluginID, message string) map[string]interface{} {
	return map[string]interface{}{"id": pluginID, "name": pluginID, "version": "unknown", "runtime": "unknown", "runtime_profile": "", "status": "error", "port": 0, "trigger": "", "priority": 0, "pinned": false, "platforms": []string{}, "allowed_adapter_ids": []string{}, "user_config_schema": []interface{}{}, "user_config": map[string]interface{}{}, "enabled": false, "error": message}
}

func (s *Server) handlePluginDetail(w http.ResponseWriter, r *http.Request) {
	pluginID := strings.TrimPrefix(r.URL.Path, "/api/plugins/")
	if pluginID == "" {
		s.jsonError(w, "插件 ID 不能为空", http.StatusBadRequest)
		return
	}
	switch pluginID {
	case "templates":
		s.handlePluginTemplates(w, r)
		return
	case "preview":
		s.handlePluginCreatePreview(w, r)
		return
	case "validate":
		s.handlePluginCreateValidate(w, r)
		return
	}
	if r.Method == http.MethodPost {
		var req struct {
			Action string `json:"action"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		s.handlePluginAction(w, pluginID, req.Action)
		return
	}
	if r.Method == http.MethodDelete {
		backupPath, err := s.backupAndDeletePlugin(pluginID)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.logManager.AddLog("info", fmt.Sprintf("删除插件: %s，备份: %s", pluginID, backupPath))
		s.jsonResponse(w, map[string]interface{}{"message": "插件删除成功", "backup": backupPath})
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handlePluginRecycleBin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		items, err := listPluginBackupFiles()
		if err != nil {
			s.jsonError(w, "读取插件回收站失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"items": items})
	case http.MethodDelete:
		name := strings.TrimSpace(r.URL.Query().Get("name"))
		if name == "" {
			var req struct {
				Name string `json:"name"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			name = strings.TrimSpace(req.Name)
		}
		backupPath, err := safePluginBackupPath(name)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := os.Remove(backupPath); err != nil {
			if os.IsNotExist(err) {
				s.jsonError(w, "备份文件不存在", http.StatusNotFound)
				return
			}
			s.jsonError(w, "删除备份文件失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if s.logManager != nil {
			s.logManager.AddLog("info", fmt.Sprintf("删除插件回收站备份: %s", name))
		}
		s.jsonResponse(w, map[string]interface{}{"message": "备份文件已删除"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func listPluginBackupFiles() ([]pluginBackupFile, error) {
	entries, err := os.ReadDir("plugins")
	if err != nil {
		if os.IsNotExist(err) {
			return []pluginBackupFile{}, nil
		}
		return nil, err
	}
	items := make([]pluginBackupFile, 0)
	for _, entry := range entries {
		if entry.IsDir() || !isPluginBackupZipName(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		pluginID, timestamp := parsePluginBackupName(entry.Name())
		items = append(items, pluginBackupFile{Name: entry.Name(), Path: filepath.ToSlash(filepath.Join("plugins", entry.Name())), Size: info.Size(), ModTime: info.ModTime(), PluginID: pluginID, Timestamp: timestamp})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].ModTime.After(items[j].ModTime)
	})
	return items, nil
}

func safePluginBackupPath(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("备份文件名不能为空")
	}
	if name != filepath.Base(filepath.Clean(name)) || strings.ContainsAny(name, `/\\`) {
		return "", fmt.Errorf("备份文件名无效")
	}
	if !isPluginBackupZipName(name) {
		return "", fmt.Errorf("只能删除插件备份 zip 文件")
	}
	return filepath.Join("plugins", name), nil
}

func isPluginBackupZipName(name string) bool {
	if !strings.HasSuffix(strings.ToLower(name), ".zip") {
		return false
	}
	base := strings.TrimSuffix(name, filepath.Ext(name))
	parts := strings.Split(base, ".backup")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		return false
	}
	return parts[1] == "" || (strings.HasPrefix(parts[1], ".") && len(parts[1]) > 1)
}

func parsePluginBackupName(name string) (string, string) {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	parts := strings.SplitN(base, ".backup", 2)
	if len(parts) != 2 {
		return strings.TrimSpace(base), ""
	}
	timestamp := strings.TrimPrefix(parts[1], ".")
	return parts[0], timestamp
}

func (s *Server) backupAndDeletePlugin(pluginID string) (string, error) {
	pluginID = filepath.Base(filepath.Clean(pluginID))
	if pluginID == "." || pluginID == string(filepath.Separator) || pluginID == "" {
		return "", fmt.Errorf("插件 ID 无效")
	}
	pluginPath := filepath.Join("plugins", pluginID)
	info, err := os.Stat(pluginPath)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("插件目录不存在: %s", pluginID)
	}
	backupPath := filepath.Join("plugins", pluginID+".backup.zip")
	if _, err := os.Stat(backupPath); err == nil {
		backupPath = filepath.Join("plugins", fmt.Sprintf("%s.backup.%s.zip", pluginID, time.Now().Format("20060102150405")))
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := s.pluginManager.StopPlugin(pluginID); err != nil {
		return "", err
	}
	if _, err := s.adapterManager.GetDatabase().DisablePluginScheduledTasks(pluginID); err != nil {
		return "", err
	}
	s.router.UnregisterPlugin(pluginID)
	if err := zipDirectory(pluginPath, backupPath); err != nil {
		return "", err
	}
	if err := os.RemoveAll(pluginPath); err != nil {
		return "", err
	}
	if err := s.deletePlanTemplateMetadata(pluginID); err != nil && s.logManager != nil {
		s.logManager.AddLog("warn", fmt.Sprintf("删除插件 %s 的模板元数据失败: %v", pluginID, err))
	}
	return backupPath, nil
}

func zipDirectory(sourceDir, zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	writer := zip.NewWriter(zipFile)
	defer writer.Close()

	return filepath.WalkDir(sourceDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(filepath.Join(filepath.Base(sourceDir), relPath))
		header.Method = zip.Deflate
		fileWriter, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(fileWriter, file)
		return err
	})
}

func (s *Server) handlePluginAction(w http.ResponseWriter, pluginID, action string) {
	switch action {
	case "enable":
		if err := s.pluginManager.TogglePlugin(pluginID, true); err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case "disable":
		if err := s.pluginManager.TogglePlugin(pluginID, false); err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case "reload", "restart":
		if err := s.pluginManager.ReloadPlugin(pluginID); err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if process := s.pluginManager.GetPlugin(pluginID); process != nil && process.Plugin != nil {
			if err := s.router.RegisterPlugin(process.Plugin); err != nil {
				s.jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	case "pin":
		if err := s.pluginManager.SetPluginPinned(pluginID, true); err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case "unpin":
		if err := s.pluginManager.SetPluginPinned(pluginID, false); err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case "start", "stop":
	default:
		s.jsonError(w, "不支持的操作: "+action, http.StatusBadRequest)
		return
	}
	s.jsonResponse(w, map[string]interface{}{"message": "操作成功"})
}

func (s *Server) handleSystemStatus(w http.ResponseWriter, r *http.Request) {
	plugins := s.pluginManager.GetAllPlugins()
	enabledPluginCount := 0
	for _, item := range plugins {
		if item.Plugin != nil && item.Plugin.Enabled {
			enabledPluginCount++
		}
	}
	adapterCount := 0
	runningAdapterCount := 0
	if s.adapterManager != nil {
		runningAdapterCount = len(s.adapterManager.GetAllAdapters())
		if s.adapterManager.GetDatabase() != nil {
			if adapters, err := s.adapterManager.GetDatabase().GetAllAdapters(); err == nil {
				adapterCount = len(adapters)
			}
		}
		if adapterCount == 0 {
			adapterCount = runningAdapterCount
		}
	}
	messageCount := int64(0)
	todayMessageCount := int64(0)
	if s.adapterManager != nil && s.adapterManager.GetDatabase() != nil {
		if summary, err := s.adapterManager.GetDatabase().GetMessageCountSummary(); err == nil {
			messageCount = summary.Total
			todayMessageCount = summary.Today
		}
	} else if s.router != nil {
		messageCount = int64(s.router.MessageCount())
	}
	totalUptimeSeconds, currentUptimeSeconds := s.runtimeSeconds()
	response := map[string]interface{}{"uptime": formatDuration(time.Duration(currentUptimeSeconds) * time.Second), "totalUptimeSeconds": totalUptimeSeconds, "currentUptimeSeconds": currentUptimeSeconds, "pluginCount": len(plugins), "enabledPluginCount": enabledPluginCount, "adapterCount": adapterCount, "runningAdapterCount": runningAdapterCount, "messageCount": messageCount, "todayMessageCount": todayMessageCount}
	for key, value := range s.systemResourceStatus() {
		response[key] = value
	}
	s.jsonResponse(w, response)
}

func (s *Server) handleMessageStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.adapterManager == nil {
		s.jsonError(w, "适配器管理器未初始化", http.StatusInternalServerError)
		return
	}
	date := strings.TrimSpace(r.URL.Query().Get("date"))
	mode := strings.TrimSpace(r.URL.Query().Get("mode"))
	stats, err := s.adapterManager.GetDatabase().GetMessageStats(date, mode)
	if err != nil {
		s.jsonError(w, "获取消息统计失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, stats)
}

func formatDuration(d time.Duration) string {
	hours, minutes, seconds := int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

func (s *Server) handleAdapters(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		adapters, err := s.adapterManager.GetDatabase().GetAllAdapters()
		if err != nil {
			s.jsonError(w, "获取适配器配置失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		runningAdapters := s.adapterManager.GetAllAdapters()
		result := make([]map[string]interface{}, 0, len(adapters))
		for _, item := range adapters {
			_, running := runningAdapters[item.ID]
			result = append(result, map[string]interface{}{"id": item.ID, "platform": item.Platform, "remark": item.Remark, "description": item.Description, "enabled": item.Enabled, "pinned": item.Pinned, "config": utils.MaskSensitiveConfig(item.Config), "running": running, "created_at": item.CreatedAt, "updated_at": item.UpdatedAt})
		}
		sort.SliceStable(result, func(i, j int) bool {
			return boolValue(result[i], "pinned", false) && !boolValue(result[j], "pinned", false)
		})
		s.jsonResponse(w, result)
		return
	}
	if r.Method == http.MethodPost {
		var req struct {
			ID          int64                  `json:"id"`
			Platform    string                 `json:"platform"`
			Remark      string                 `json:"remark"`
			Description string                 `json:"description"`
			Enabled     bool                   `json:"enabled"`
			Config      map[string]interface{} `json:"config"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		if err := s.adapterManager.SaveAdapterConfig(req.ID, req.Platform, req.Remark, req.Description, req.Enabled, req.Config); err != nil {
			s.jsonError(w, "保存配置失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"message": "配置已保存并生效"})
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleAdapterDetail(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/api/adapters/")
	if key == "" {
		s.jsonError(w, "适配器不能为空", http.StatusBadRequest)
		return
	}
	id, idErr := strconv.ParseInt(key, 10, 64)
	if r.Method == http.MethodDelete {
		if idErr == nil {
			_ = s.adapterManager.StopAdapterByID(id)
			if err := s.adapterManager.GetDatabase().DeleteAdapterByID(id); err != nil {
				s.jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			_ = s.adapterManager.StopAdapter(key)
			if err := s.adapterManager.GetDatabase().DeleteAdapter(key); err != nil {
				s.jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		s.jsonResponse(w, map[string]interface{}{"message": "配置已删除"})
		return
	}
	if r.Method == http.MethodPost {
		if idErr != nil {
			s.jsonError(w, "适配器 ID 无效", http.StatusBadRequest)
			return
		}
		var req struct {
			Action string `json:"action"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		s.handleAdapterAction(w, id, req.Action)
		return
	}
	if r.Method == http.MethodGet {
		var item *config.AdapterConfig
		var err error
		if idErr == nil {
			item, err = s.adapterManager.GetDatabase().GetAdapterByID(id)
		} else {
			item, err = s.adapterManager.GetDatabase().GetAdapter(key)
		}
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if item == nil {
			s.jsonError(w, "配置不存在", http.StatusNotFound)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"id": item.ID, "platform": item.Platform, "remark": item.Remark, "description": item.Description, "enabled": item.Enabled, "pinned": item.Pinned, "config": utils.MaskSensitiveConfig(item.Config), "running": s.adapterManager.GetAdapterByID(item.ID) != nil, "created_at": item.CreatedAt, "updated_at": item.UpdatedAt})
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleAdapterAction(w http.ResponseWriter, id int64, action string) {
	switch action {
	case "pin":
		if err := s.adapterManager.SetAdapterPinned(id, true); err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case "unpin":
		if err := s.adapterManager.SetAdapterPinned(id, false); err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
	default:
		s.jsonError(w, "不支持的操作: "+action, http.StatusBadRequest)
		return
	}
	s.jsonResponse(w, map[string]interface{}{"message": "操作成功"})
}

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
	ch := s.router.GetSessionManager().CreateSession(req.PluginID, req.UserID, req.GroupID, req.Timeout)
	content := ""
	select {
	case msg, ok := <-ch:
		if ok {
			content = msg
		}
	case <-r.Context().Done():
	}
	s.jsonResponse(w, map[string]interface{}{"content": content})
}

func (s *Server) handlePluginConfig(w http.ResponseWriter, r *http.Request) {
	pluginID := strings.TrimPrefix(r.URL.Path, "/api/plugins/config/")
	if pluginID == "" {
		s.jsonError(w, "插件 ID 不能为空", http.StatusBadRequest)
		return
	}
	configPath := filepath.Join("plugins", pluginID, "plugin.json")
	if r.Method == http.MethodGet {
		data, err := os.ReadFile(configPath)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var config map[string]interface{}
		if err := json.Unmarshal(data, &config); err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, ok := config["allowed_adapter_ids"]; !ok {
			config["allowed_adapter_ids"] = []string{}
		}
		if _, ok := config["priority"]; !ok {
			config["priority"] = 0
		}
		if _, ok := config["pinned"]; !ok {
			config["pinned"] = false
		}
		if _, ok := config["user_config_schema"]; !ok {
			config["user_config_schema"] = []interface{}{}
		}
		if _, ok := config["user_config"]; !ok {
			config["user_config"] = map[string]interface{}{}
		}
		if _, ok := config["access_control"]; !ok {
			config["access_control"] = map[string]interface{}{"inherit_system": true, "whitelist_groups": []string{}, "blocked_groups": []string{}, "whitelist_user_ids": []string{}, "blocked_user_ids": []string{}, "whitelist_union_ids": []string{}, "blocked_union_ids": []string{}}
		}
		if _, ok := config["runtime_profile"]; !ok {
			config["runtime_profile"] = ""
		}
		if _, ok := config["open_api"]; !ok {
			config["open_api"] = map[string]interface{}{"enabled": false, "path": pluginID, "method": "POST", "token": "", "runtime": stringValue(config, "runtime", "nodejs"), "runtime_profile": stringValue(config, "runtime_profile", "")}
		}
		if _, ok := config["script_env"]; !ok {
			config["script_env"] = map[string]interface{}{"enabled": false, "names": []string{}}
		}
		s.jsonResponse(w, config)
		return
	}
	if r.Method == http.MethodPut {
		var config map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			s.jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		data, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := os.WriteFile(configPath, data, 0644); err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := s.pluginManager.ReloadPlugin(pluginID); err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if process := s.pluginManager.GetPlugin(pluginID); process != nil && process.Plugin != nil {
			if err := s.router.RegisterPlugin(process.Plugin); err != nil {
				s.jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		s.jsonResponse(w, map[string]interface{}{"message": "配置已更新并生效"})
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleDependencies(w http.ResponseWriter, r *http.Request) {
	depsManager := s.pluginManager.GetDepsManager()
	switch r.Method {
	case http.MethodGet:
		runtimeName := normalizeDependencyRuntime(r.URL.Query().Get("runtime"))
		profileID := dependencyProfileID(r)
		if runtimeName == "" {
			pythonDeps, err := depsManager.GetPythonDeps()
			if err != nil {
				s.jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			nodeDeps, err := depsManager.GetNodeDeps()
			if err != nil {
				s.jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			s.jsonResponse(w, map[string]interface{}{"python": pythonDeps, "nodejs": nodeDeps})
			return
		}
		if runtimeName != "python" && runtimeName != "nodejs" {
			s.jsonError(w, "不支持的运行时", http.StatusBadRequest)
			return
		}
		deps, err := dependencyListByRuntime(depsManager, runtimeName, profileID)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"runtime": runtimeName, "profile_id": profileID, "dependencies": deps})
		return
	case http.MethodPost:
		var req struct {
			Runtime   string `json:"runtime"`
			ProfileID string `json:"profile_id"`
			Name      string `json:"name"`
			Version   string `json:"version"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		runtimeName := normalizeDependencyRuntime(req.Runtime)
		name := strings.TrimSpace(req.Name)
		if name == "" {
			s.jsonError(w, "依赖包名不能为空", http.StatusBadRequest)
			return
		}
		deps := map[string]string{name: strings.TrimSpace(req.Version)}
		var err error
		if runtimeName == "python" {
			err = depsManager.InstallPythonDepsForProfile(req.ProfileID, deps)
		} else if runtimeName == "nodejs" {
			err = depsManager.InstallNodeDepsForProfile(req.ProfileID, deps)
		} else {
			s.jsonError(w, "不支持的运行时", http.StatusBadRequest)
			return
		}
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"message": "依赖安装成功"})
		return
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleDependencyDetail(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/dependencies/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	runtimeName := normalizeDependencyRuntime(parts[0])
	name, err := url.PathUnescape(parts[1])
	if err != nil || strings.TrimSpace(name) == "" {
		s.jsonError(w, "依赖包名无效", http.StatusBadRequest)
		return
	}
	name = strings.TrimSpace(name)
	profileID := dependencyProfileID(r)
	depsManager := s.pluginManager.GetDepsManager()
	if runtimeName == "python" {
		err = depsManager.UninstallPythonDepForProfile(profileID, name)
	} else if runtimeName == "nodejs" {
		err = depsManager.UninstallNodeDepForProfile(profileID, name)
	} else {
		s.jsonError(w, "不支持的运行时", http.StatusBadRequest)
		return
	}
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, map[string]interface{}{"message": "依赖卸载成功"})
}

func normalizeDependencyRuntime(runtime string) string {
	runtime = strings.ToLower(strings.TrimSpace(runtime))
	if runtime == "node" {
		return "nodejs"
	}
	if runtime == "py" || runtime == "python3" {
		return "python"
	}
	return runtime
}

func dependencyProfileID(r *http.Request) string {
	profileID := strings.TrimSpace(r.URL.Query().Get("profile_id"))
	if profileID == "" {
		profileID = strings.TrimSpace(r.URL.Query().Get("profileId"))
	}
	return profileID
}

func dependencyListByRuntime(depsManager dependencyRuntimeManager, runtimeName, profileID string) (map[string]string, error) {
	if runtimeName == "python" {
		return depsManager.GetPythonDepsForProfile(profileID)
	}
	if runtimeName == "nodejs" {
		return depsManager.GetNodeDepsForProfile(profileID)
	}
	return nil, fmt.Errorf("不支持的运行时")
}

type dependencyRuntimeManager interface {
	GetPythonDepsForProfile(profileID string) (map[string]string, error)
	GetNodeDepsForProfile(profileID string) (map[string]string, error)
}

func (s *Server) handlePluginCode(w http.ResponseWriter, r *http.Request) {
	pluginID := strings.TrimPrefix(r.URL.Path, "/api/plugins/code/")
	pluginInfo := s.router.GetPlugin(pluginID)
	if pluginInfo == nil {
		s.jsonError(w, "插件不存在", http.StatusNotFound)
		return
	}
	entryPath, err := safePluginEntryPath(filepath.Join("plugins", pluginID), pluginInfo.Runtime, pluginInfo.Entry)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if r.Method == http.MethodGet {
		code, err := os.ReadFile(entryPath)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"code": string(code), "filename": pluginInfo.Entry, "plugin_name": pluginInfo.Name})
		return
	}
	if r.Method == http.MethodPut {
		var req struct {
			Code string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		if err := os.WriteFile(entryPath, []byte(req.Code), 0644); err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = s.pluginManager.ReloadPlugin(pluginID)
		s.jsonResponse(w, map[string]interface{}{"message": "代码已保存并生效"})
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handlePluginFiles(w http.ResponseWriter, r *http.Request) {
	pluginID := strings.TrimPrefix(r.URL.Path, "/api/plugins/files/")
	pluginInfo := s.router.GetPlugin(pluginID)
	if pluginInfo == nil {
		s.jsonError(w, "插件不存在", http.StatusNotFound)
		return
	}
	pluginRoot := filepath.Join("plugins", pluginID)

	switch r.Method {
	case http.MethodGet:
		filePath := strings.TrimSpace(r.URL.Query().Get("path"))
		if filePath == "" {
			tree, err := buildPluginFileTree(pluginRoot, "")
			if err != nil {
				s.jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			s.jsonResponse(w, map[string]interface{}{"plugin_name": pluginInfo.Name, "entry": pluginInfo.Entry, "tree": tree})
			return
		}

		fullPath, err := safePluginPath(pluginRoot, filePath)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		info, err := os.Stat(fullPath)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		if info.IsDir() {
			s.jsonError(w, "不能预览文件夹", http.StatusBadRequest)
			return
		}
		if !isTextPreviewFile(filePath) {
			s.jsonResponse(w, map[string]interface{}{"path": filepath.ToSlash(filePath), "filename": filepath.Base(filePath), "editable": false, "text": false, "size": info.Size()})
			return
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"path": filepath.ToSlash(filePath), "filename": filepath.Base(filePath), "code": string(data), "editable": true, "text": true, "size": info.Size()})
	case http.MethodPut:
		var req struct {
			Path string `json:"path"`
			Code string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.Path) == "" {
			s.jsonError(w, "文件路径不能为空", http.StatusBadRequest)
			return
		}
		if !isTextPreviewFile(req.Path) {
			s.jsonError(w, "该文件类型不支持在线编辑", http.StatusBadRequest)
			return
		}
		fullPath, err := safePluginPath(pluginRoot, req.Path)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		info, err := os.Stat(fullPath)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		if info.IsDir() {
			s.jsonError(w, "不能保存文件夹", http.StatusBadRequest)
			return
		}
		if err := os.WriteFile(fullPath, []byte(req.Code), 0644); err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = s.pluginManager.ReloadPlugin(pluginID)
		s.jsonResponse(w, map[string]interface{}{"message": "文件已保存并生效"})
	case http.MethodPost:
		var req struct {
			Path string `json:"path"`
			Type string `json:"type"`
			Code string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		filePath := strings.TrimSpace(req.Path)
		if filePath == "" {
			s.jsonError(w, "路径不能为空", http.StatusBadRequest)
			return
		}
		entryType := strings.TrimSpace(req.Type)
		if entryType == "" {
			entryType = "file"
		}
		fullPath, err := safePluginPath(pluginRoot, filePath)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if _, err := os.Stat(fullPath); err == nil {
			s.jsonError(w, "路径已存在", http.StatusBadRequest)
			return
		} else if !os.IsNotExist(err) {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		switch entryType {
		case "directory":
			if err := os.MkdirAll(fullPath, 0755); err != nil {
				s.jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			s.jsonResponse(w, map[string]interface{}{"message": "文件夹已创建", "path": filepath.ToSlash(filePath), "type": entryType})
		case "file":
			if !isTextPreviewFile(filePath) {
				s.jsonError(w, "该文件类型不支持在线编辑", http.StatusBadRequest)
				return
			}
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				s.jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if err := os.WriteFile(fullPath, []byte(req.Code), 0644); err != nil {
				s.jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_ = s.pluginManager.ReloadPlugin(pluginID)
			s.jsonResponse(w, map[string]interface{}{"message": "文件已创建", "path": filepath.ToSlash(filePath), "type": entryType})
		default:
			s.jsonError(w, "创建类型无效", http.StatusBadRequest)
		}
	case http.MethodDelete:
		filePath := strings.TrimSpace(r.URL.Query().Get("path"))
		if filePath == "" {
			var req struct {
				Path string `json:"path"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			filePath = strings.TrimSpace(req.Path)
		}
		if filePath == "" {
			s.jsonError(w, "路径不能为空", http.StatusBadRequest)
			return
		}
		fullPath, err := safePluginPath(pluginRoot, filePath)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if _, err := os.Stat(fullPath); err != nil {
			if os.IsNotExist(err) {
				s.jsonError(w, "路径不存在", http.StatusNotFound)
				return
			}
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := os.RemoveAll(fullPath); err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = s.pluginManager.ReloadPlugin(pluginID)
		s.jsonResponse(w, map[string]interface{}{"message": "路径已删除", "path": filepath.ToSlash(filePath)})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func buildPluginFileTree(root, relative string) ([]map[string]interface{}, error) {
	base := filepath.Join(root, relative)
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]interface{}, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if name == "__pycache__" || name == "node_modules" || name == ".venv" || strings.HasPrefix(name, ".git") {
			continue
		}
		relPath := filepath.ToSlash(filepath.Join(relative, name))
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		item := map[string]interface{}{"name": name, "path": relPath, "type": "file", "text": isTextPreviewFile(relPath), "size": info.Size()}
		if entry.IsDir() {
			children, err := buildPluginFileTree(root, relPath)
			if err != nil {
				return nil, err
			}
			item["type"] = "directory"
			item["text"] = false
			item["children"] = children
		}
		result = append(result, item)
	}
	return result, nil
}

func safePluginPath(root, relative string) (string, error) {
	cleanRelative := filepath.Clean(strings.TrimPrefix(strings.ReplaceAll(relative, "\\", "/"), "/"))
	if cleanRelative == "." || strings.HasPrefix(cleanRelative, "..") || filepath.IsAbs(cleanRelative) {
		return "", fmt.Errorf("文件路径无效")
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
		return "", fmt.Errorf("文件路径越界")
	}
	return fullPath, nil
}

func safePluginEntryPath(root, runtimeName, entry string) (string, error) {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return "", fmt.Errorf("入口文件不能为空")
	}
	entry = strings.ReplaceAll(entry, "\\", "/")
	if strings.HasPrefix(entry, "/") || filepath.IsAbs(entry) {
		return "", fmt.Errorf("入口文件必须是相对路径")
	}
	cleanEntry := filepath.Clean(entry)
	if cleanEntry == "." || cleanEntry == ".." || strings.HasPrefix(cleanEntry, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("入口文件路径越界")
	}
	switch runtimeName {
	case "python":
		if !strings.EqualFold(filepath.Ext(cleanEntry), ".py") {
			return "", fmt.Errorf("Python 插件入口文件必须是 .py")
		}
	case "nodejs":
		ext := strings.ToLower(filepath.Ext(cleanEntry))
		if ext != ".js" && ext != ".mjs" && ext != ".cjs" {
			return "", fmt.Errorf("Node.js 插件入口文件必须是 .js、.mjs 或 .cjs")
		}
	default:
		return "", fmt.Errorf("不支持的运行时: %s", runtimeName)
	}
	fullPath, err := safePluginPath(root, cleanEntry)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return "", fmt.Errorf("入口文件不存在: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("入口文件不能是目录")
	}
	return fullPath, nil
}

func isTextPreviewFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".js", ".ts", ".jsx", ".tsx", ".vue", ".py", ".json", ".md", ".txt", ".yaml", ".yml", ".toml", ".html", ".css", ".scss", ".sql", ".sh", ".bat", ".ps1", ".env":
		return true
	default:
		return false
	}
}

func (s *Server) webAssetsHandler() http.Handler {
	if s.webAssetMode == WebAssetModeExternal {
		return http.FileServer(http.Dir(s.externalWebDir))
	}
	if s.webFS != nil {
		return http.FileServer(http.FS(s.webFS))
	}
	return http.NotFoundHandler()
}

func (s *Server) readWebIndex() ([]byte, error) {
	if s.webAssetMode == WebAssetModeExternal {
		return os.ReadFile(filepath.Join(s.externalWebDir, "index.html"))
	}
	if s.webFS == nil {
		return nil, fs.ErrNotExist
	}
	return fs.ReadFile(s.webFS, "index.html")
}

func (s *Server) handleChatIndex(w http.ResponseWriter, r *http.Request) {
	data, err := s.readWebIndex()
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><body><h1>AllBot</h1><p>Web UI files not found.</p></body></html>`))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if !s.accessGranted(r) {
		http.NotFound(w, r)
		return
	}
	data, err := s.readWebIndex()
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><body><h1>AllBot</h1><p>Web UI files not found.</p></body></html>`))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

// accessGranted 判断当前 HTML 页面请求是否被允许渲染。
// 未开启访问码时始终允许；开启后需满足以下任一条件：已登录（携带有效会话 cookie）、或携带有效的访问码通行 cookie。
func (s *Server) accessGranted(r *http.Request) bool {
	enabled, code := s.accessCodeConfig()
	if !enabled || code == "" {
		return true
	}
	if s.requestHasValidSession(r) {
		return true
	}
	return s.requestHasAccessPass(r, code)
}

// handleAccessCodeEntry 处理形如 /login/<访问码> 的入口请求。
// 访问码正确时种下通行 cookie 并重定向到 /login；错误时返回 404，避免暴露入口存在。
func (s *Server) handleAccessCodeEntry(w http.ResponseWriter, r *http.Request) {
	enabled, code := s.accessCodeConfig()
	// 未开启访问码时，/login/xxx 视为普通前端路由，交回 SPA 渲染。
	if !enabled || code == "" {
		s.handleIndex(w, r)
		return
	}
	provided := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/login/"))
	provided = strings.Trim(provided, "/")
	if provided == "" {
		s.handleIndex(w, r)
		return
	}
	if subtle.ConstantTimeCompare([]byte(provided), []byte(code)) != 1 {
		http.NotFound(w, r)
		return
	}
	s.setAccessPassCookie(w, code)
	http.Redirect(w, r, "/login", http.StatusFound)
}

// handleLogout 注销当前会话并清除相关 cookie。
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if token := s.requestSessionToken(r); token != "" {
		s.sessionMu.Lock()
		delete(s.sessions, token)
		s.sessionMu.Unlock()
	}
	s.clearSessionCookie(w)
	s.jsonResponse(w, map[string]bool{"success": true})
}

// accessCodeConfig 读取访问码开关与码值，读取失败时按未开启处理，避免误锁后台。
func (s *Server) accessCodeConfig() (bool, string) {
	database := s.runtimeDatabase()
	if database == nil {
		return false, ""
	}
	settings, err := database.GetSystemSettings()
	if err != nil || settings == nil {
		return false, ""
	}
	return settings.AccessCodeEnabled, strings.TrimSpace(settings.AccessCode)
}

const (
	sessionCookieName    = "allbot_session"
	accessPassCookieName = "allbot_access"
)

func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((24 * time.Hour).Seconds()),
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (s *Server) setAccessPassCookie(w http.ResponseWriter, code string) {
	http.SetCookie(w, &http.Cookie{
		Name:     accessPassCookieName,
		Value:    accessCodeHash(code),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((30 * 24 * time.Hour).Seconds()),
	})
}

// accessCodeHash 生成访问码的 SHA-256 十六进制摘要，避免在 cookie 中明文存储访问码。
func accessCodeHash(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

func (s *Server) requestSessionToken(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}

func (s *Server) requestHasValidSession(r *http.Request) bool {
	token := s.requestSessionToken(r)
	if token == "" {
		return false
	}
	return s.validAdminToken(token)
}

func (s *Server) requestHasAccessPass(r *http.Request, code string) bool {
	cookie, err := r.Cookie(accessPassCookieName)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(strings.TrimSpace(cookie.Value)), []byte(accessCodeHash(code))) == 1
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api/login" || r.URL.Path == "/api/logout" || strings.HasPrefix(r.URL.Path, "/api/open/") {
			next.ServeHTTP(w, r)
			return
		}
		if !s.validAdminToken(r.Header.Get("Authorization")) && !s.requestHasValidSession(r) {
			s.jsonError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) createAdminSession() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buffer)
	s.sessionMu.Lock()
	s.sessions[token] = time.Now().Add(24 * time.Hour)
	s.sessionMu.Unlock()
	return token, nil
}

func (s *Server) validAdminToken(header string) bool {
	token := strings.TrimSpace(header)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	if token == "" {
		return false
	}
	now := time.Now()
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	for sessionToken, expiresAt := range s.sessions {
		if now.After(expiresAt) {
			delete(s.sessions, sessionToken)
			continue
		}
		if subtle.ConstantTimeCompare([]byte(token), []byte(sessionToken)) == 1 {
			return true
		}
	}
	return false
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Open-Token")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Server) jsonError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func stringValue(config map[string]interface{}, key, fallback string) string {
	if value, ok := config[key].(string); ok {
		return value
	}
	return fallback
}

func boolValue(config map[string]interface{}, key string, fallback bool) bool {
	if value, ok := config[key].(bool); ok {
		return value
	}
	return fallback
}

func intValue(config map[string]interface{}, key string, fallback int) int {
	switch value := config[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	case json.Number:
		parsed, err := value.Int64()
		if err == nil {
			return int(parsed)
		}
	}
	return fallback
}

func stringSliceValue(config map[string]interface{}, key string) []string {
	result := []string{}
	items, ok := config[key].([]interface{})
	if !ok {
		return result
	}
	for _, item := range items {
		if value, ok := item.(string); ok {
			result = append(result, value)
		}
	}
	return result
}
