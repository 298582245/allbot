package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/allbot/allbot/core/backup"
	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/router"
)

func (s *Server) handleBackups(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleBackupOverview(w, r)
	case http.MethodPost:
		s.handleCreateBackup(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleBackupDetail(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/backups/"), "/")
	switch rest {
	case "settings":
		s.handleBackupSettings(w, r)
		return
	case "status":
		s.handleBackupStatus(w, r)
		return
	case "import":
		s.handleImportBackup(w, r)
		return
	}

	parts := strings.Split(rest, "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		s.jsonError(w, "备份路径无效", http.StatusBadRequest)
		return
	}
	name, err := url.PathUnescape(parts[0])
	if err != nil {
		s.jsonError(w, "备份文件名无效", http.StatusBadRequest)
		return
	}
	switch parts[1] {
	case "download":
		s.handleDownloadBackup(w, r, name)
	case "delete":
		s.handleDeleteBackup(w, r, name)
	case "restore":
		s.handleRestoreBackup(w, r, name)
	default:
		s.jsonError(w, "备份路径无效", http.StatusBadRequest)
	}
}

func (s *Server) handleBackupOverview(w http.ResponseWriter, r *http.Request) {
	service, ok := s.requireBackupService(w)
	if !ok {
		return
	}
	settings, err := s.adapterManager.GetDatabase().GetBackupSettings()
	if err != nil {
		s.jsonError(w, "获取备份配置失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	files, err := service.List()
	if err != nil {
		s.jsonError(w, "获取备份列表失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, map[string]interface{}{"settings": settings, "status": service.Status(), "files": files})
}

func (s *Server) handleBackupSettings(w http.ResponseWriter, r *http.Request) {
	service, ok := s.requireBackupService(w)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		settings, err := s.adapterManager.GetDatabase().GetBackupSettings()
		if err != nil {
			s.jsonError(w, "获取备份配置失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, settings)
	case http.MethodPut:
		var settings config.BackupSettings
		if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
			s.jsonError(w, "请求数据无效", http.StatusBadRequest)
			return
		}
		settings = config.NormalizeBackupSettings(settings)
		if _, err := router.NextCronTime(settings.Cron, time.Now()); err != nil {
			s.jsonError(w, "定时表达式无效: "+err.Error(), http.StatusBadRequest)
			return
		}
		if !settings.IncludePlugins && !settings.IncludeData && !settings.IncludeImages && !settings.IncludeLogs && !settings.IncludeRuntimeEnv {
			s.jsonError(w, "至少需要选择插件、数据、图片、日志或运行环境中的一项", http.StatusBadRequest)
			return
		}
		if err := s.adapterManager.GetDatabase().SaveBackupSettings(settings); err != nil {
			s.jsonError(w, "保存备份配置失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		service.Reload()
		s.jsonResponse(w, map[string]interface{}{"message": "保存成功", "settings": settings, "status": service.Status()})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleBackupStatus(w http.ResponseWriter, r *http.Request) {
	service, ok := s.requireBackupService(w)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.jsonResponse(w, service.Status())
}

func (s *Server) handleCreateBackup(w http.ResponseWriter, r *http.Request) {
	service, ok := s.requireBackupService(w)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	file, err := service.Create(ctx, "manual")
	if err != nil {
		s.jsonError(w, "创建备份失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if s.logManager != nil {
		s.logManager.AddLog("info", fmt.Sprintf("创建系统备份: %s", file.Name))
	}
	s.jsonResponse(w, map[string]interface{}{"message": "备份创建成功", "file": file})
}

func (s *Server) handleImportBackup(w http.ResponseWriter, r *http.Request) {
	service, ok := s.requireBackupService(w)
	if !ok {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 512<<20)
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		s.jsonError(w, "解析上传文件失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		s.jsonError(w, "请选择备份文件", http.StatusBadRequest)
		return
	}
	defer file.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	result, err := service.Import(ctx, backup.ImportOptions{Reader: file, OriginalName: header.Filename, MaxSize: 512 << 20})
	if err != nil {
		s.jsonError(w, "导入备份失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	if s.logManager != nil {
		s.logManager.AddLog("info", fmt.Sprintf("导入系统备份: %s", result.File.Name))
	}
	s.jsonResponse(w, result)
}

func (s *Server) handleRestoreBackup(w http.ResponseWriter, r *http.Request, name string) {
	service, ok := s.requireBackupService(w)
	if !ok {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var options backup.RestoreOptions
	if err := json.NewDecoder(r.Body).Decode(&options); err != nil {
		s.jsonError(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	if !options.Confirm || (!options.IncludeData && !options.IncludePlugins && !options.IncludeOpenAPIs && !options.IncludeImages && !options.IncludeLogs && !options.IncludeRuntimeEnv) {
		s.jsonError(w, "请确认并至少选择一项恢复内容", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
	defer cancel()
	result, err := service.Restore(ctx, name, options)
	if err != nil {
		s.jsonError(w, "恢复备份失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	if s.logManager != nil {
		s.logManager.AddLog("info", fmt.Sprintf("恢复系统备份: %s", name))
	}
	s.jsonResponse(w, result)
}

func (s *Server) handleDownloadBackup(w http.ResponseWriter, r *http.Request, name string) {
	service, ok := s.requireBackupService(w)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	file, err := service.Resolve(name)
	if err != nil {
		s.jsonError(w, "读取备份失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Name))
	http.ServeFile(w, r, file.Path)
}

func (s *Server) handleDeleteBackup(w http.ResponseWriter, r *http.Request, name string) {
	service, ok := s.requireBackupService(w)
	if !ok {
		return
	}
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := service.Delete(name); err != nil {
		s.jsonError(w, "删除备份失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	s.jsonResponse(w, map[string]interface{}{"message": "备份已删除"})
}

func (s *Server) requireBackupService(w http.ResponseWriter) (*backup.Service, bool) {
	if s.backupService == nil {
		s.jsonError(w, "备份服务未初始化", http.StatusInternalServerError)
		return nil, false
	}
	return s.backupService, true
}
