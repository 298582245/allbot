package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/allbot/allbot/core/backup"
	"github.com/allbot/allbot/core/config"
)

type statisticsOverview struct {
	System      statisticsSystemSummary       `json:"system"`
	Messages    config.MessageCountSummary    `json:"messages"`
	Payments    config.PaymentStatsSummary    `json:"payments"`
	Images      config.ImageAssetStatsSummary `json:"images"`
	ScriptTasks config.ScriptRunStatsSummary  `json:"script_tasks"`
	Backups     statisticsBackupSummary       `json:"backups"`
}

type statisticsSystemSummary struct {
	Uptime               string `json:"uptime"`
	TotalUptimeSeconds   int64  `json:"total_uptime_seconds"`
	CurrentUptimeSeconds int64  `json:"current_uptime_seconds"`
	PluginCount          int    `json:"plugin_count"`
	EnabledPluginCount   int    `json:"enabled_plugin_count"`
	AdapterCount         int    `json:"adapter_count"`
	RunningAdapterCount  int    `json:"running_adapter_count"`
}

type statisticsBackupSummary struct {
	Enabled        bool               `json:"enabled"`
	Running        bool               `json:"running"`
	FileCount      int                `json:"file_count"`
	TotalSizeBytes int64              `json:"total_size_bytes"`
	Latest         *backup.BackupFile `json:"latest,omitempty"`
	NextRunAt      *time.Time         `json:"next_run_at,omitempty"`
	LastRunAt      *time.Time         `json:"last_run_at,omitempty"`
	LastError      string             `json:"last_error,omitempty"`
}

func (s *Server) handleStatisticsOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.adapterManager == nil || s.adapterManager.GetDatabase() == nil {
		s.jsonError(w, "数据库未初始化", http.StatusInternalServerError)
		return
	}
	db := s.adapterManager.GetDatabase()
	summary := statisticsOverview{System: s.statisticsSystemSummary()}
	var err error
	if summary.Messages, err = db.GetMessageCountSummary(); err != nil {
		s.jsonError(w, "获取消息统计失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if summary.Payments, err = db.GetPaymentStatsSummary(); err != nil {
		s.jsonError(w, "获取支付统计失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if summary.Images, err = db.GetImageAssetStatsSummary(); err != nil {
		s.jsonError(w, "获取图床统计失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if summary.ScriptTasks, err = db.GetScriptRunStatsSummary(); err != nil {
		s.jsonError(w, "获取脚本任务统计失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	summary.Backups = s.statisticsBackupSummary(db)
	s.jsonResponse(w, summary)
}

func (s *Server) handleMessageTotalTrend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.adapterManager == nil || s.adapterManager.GetDatabase() == nil {
		s.jsonError(w, "数据库未初始化", http.StatusInternalServerError)
		return
	}
	query := r.URL.Query()
	stats, err := s.adapterManager.GetDatabase().GetMessageTotalTrend(query.Get("granularity"), query.Get("start"), query.Get("end"))
	if err != nil {
		s.jsonError(w, "获取消息总量趋势失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	s.jsonResponse(w, stats)
}

func (s *Server) handlePluginTriggerTrend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.adapterManager == nil || s.adapterManager.GetDatabase() == nil {
		s.jsonError(w, "数据库未初始化", http.StatusInternalServerError)
		return
	}
	query := r.URL.Query()
	limit := 8
	if rawLimit := strings.TrimSpace(query.Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			s.jsonError(w, "limit 必须为正整数", http.StatusBadRequest)
			return
		}
		limit = parsed
	}
	stats, err := s.adapterManager.GetDatabase().GetPluginTriggerTrend(query.Get("granularity"), query.Get("start"), query.Get("end"), limit)
	if err != nil {
		s.jsonError(w, "获取插件触发趋势失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	s.jsonResponse(w, stats)
}

func (s *Server) statisticsSystemSummary() statisticsSystemSummary {
	pluginCount := 0
	enabledPluginCount := 0
	if s.pluginManager != nil {
		plugins := s.pluginManager.GetAllPlugins()
		pluginCount = len(plugins)
		for _, item := range plugins {
			if item.Plugin != nil && item.Plugin.Enabled {
				enabledPluginCount++
			}
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
	totalUptimeSeconds, currentUptimeSeconds := s.runtimeSeconds()
	return statisticsSystemSummary{Uptime: formatDuration(time.Duration(currentUptimeSeconds) * time.Second), TotalUptimeSeconds: totalUptimeSeconds, CurrentUptimeSeconds: currentUptimeSeconds, PluginCount: pluginCount, EnabledPluginCount: enabledPluginCount, AdapterCount: adapterCount, RunningAdapterCount: runningAdapterCount}
}

func (s *Server) statisticsBackupSummary(db *config.Database) statisticsBackupSummary {
	settings, err := db.GetBackupSettings()
	result := statisticsBackupSummary{}
	if err == nil {
		result.Enabled = settings.Enabled
	}
	if s.backupService == nil {
		return result
	}
	status := s.backupService.Status()
	result.Running = status.Running
	result.NextRunAt = status.NextRunAt
	result.LastRunAt = status.LastRunAt
	result.LastError = status.LastError
	files, err := s.backupService.List()
	if err != nil {
		if result.LastError == "" {
			result.LastError = err.Error()
		}
		return result
	}
	result.FileCount = len(files)
	for index := range files {
		file := files[index]
		result.TotalSizeBytes += file.Size
		if result.Latest == nil || file.CreatedAt.After(result.Latest.CreatedAt) {
			latest := file
			result.Latest = &latest
		}
	}
	return result
}
