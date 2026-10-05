package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/allbot/allbot/core/config"
)

func (s *Server) handlePluginTasks(w http.ResponseWriter, r *http.Request) {
	database := s.adapterManager.GetDatabase()
	switch r.Method {
	case http.MethodGet:
		settings, err := database.GetPluginTaskSettings()
		if err != nil {
			s.jsonError(w, "读取插件任务设置失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := database.CleanupPluginTaskLogs(settings.RetentionDays); err != nil {
			s.jsonError(w, "清理插件任务失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		filter := pluginTaskLogFilterFromRequest(r)
		total, err := database.CountPluginTaskLogs(filter)
		if err != nil {
			s.jsonError(w, "统计插件任务失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		items, err := database.ListPluginTaskLogs(filter)
		if err != nil {
			s.jsonError(w, "获取插件任务失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"items": items, "total": total, "page": pluginTaskPageFromFilter(filter), "page_size": filter.Limit, "settings": settings})
	case http.MethodPost:
		if r.URL.Query().Get("action") != "settings" {
			s.jsonError(w, "不支持的插件任务操作", http.StatusBadRequest)
			return
		}
		var settings config.PluginTaskSettings
		if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
			s.jsonError(w, "插件任务设置格式无效", http.StatusBadRequest)
			return
		}
		if err := database.SavePluginTaskSettings(settings); err != nil {
			s.jsonError(w, "保存插件任务设置失败: "+err.Error(), http.StatusBadRequest)
			return
		}
		saved, err := database.GetPluginTaskSettings()
		if err != nil {
			s.jsonError(w, "读取插件任务设置失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		removed, err := database.CleanupPluginTaskLogs(saved.RetentionDays)
		if err != nil {
			s.jsonError(w, "清理插件任务失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"message": "插件任务设置已保存", "settings": saved, "removed": removed})
	default:
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handlePluginTaskDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/plugin-tasks/"), 10, 64)
	if err != nil || id <= 0 {
		s.jsonError(w, "插件任务 ID 无效", http.StatusBadRequest)
		return
	}
	database := s.adapterManager.GetDatabase()
	item, err := database.GetPluginTaskLog(id)
	if err != nil {
		s.jsonError(w, "获取插件任务失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if item == nil {
		s.jsonError(w, "插件任务不存在", http.StatusNotFound)
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.jsonResponse(w, item)
	case http.MethodPut:
		if r.URL.Query().Get("action") != "stop" {
			s.jsonError(w, "不支持的插件任务操作", http.StatusBadRequest)
			return
		}
		if !isActivePluginTaskStatus(item.Status) {
			s.jsonResponse(w, map[string]interface{}{"message": "插件任务当前不可停止", "status": item.Status})
			return
		}
		if s.pluginManager.StopPluginTask(id) {
			_ = database.UpdatePluginTaskLog(id, config.PluginTaskStatusStopping, item.Output, "正在停止插件任务", time.Time{})
			s.jsonResponse(w, map[string]interface{}{"message": "插件任务停止请求已发送", "status": config.PluginTaskStatusStopping})
			return
		}
		_ = database.UpdatePluginTaskLog(id, config.PluginTaskStatusStopped, item.Output, "插件任务已失去运行进程，已标记为停止", time.Now())
		s.jsonResponse(w, map[string]interface{}{"message": "插件任务已失去运行进程，已标记为停止", "status": config.PluginTaskStatusStopped})
	case http.MethodDelete:
		if isActivePluginTaskStatus(item.Status) {
			_ = s.pluginManager.StopPluginTask(id)
		}
		if err := database.DeletePluginTaskLog(id); err != nil {
			s.jsonError(w, "删除插件任务失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"message": "插件任务已删除"})
	default:
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func pluginTaskLogFilterFromRequest(r *http.Request) config.PluginTaskLogFilter {
	query := r.URL.Query()
	limit, _ := strconv.Atoi(strings.TrimSpace(query.Get("page_size")))
	page, _ := strconv.Atoi(strings.TrimSpace(query.Get("page")))
	if limit <= 0 || limit > 500 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}
	return config.PluginTaskLogFilter{Keyword: query.Get("keyword"), PluginID: query.Get("plugin_id"), Platform: query.Get("platform"), Status: query.Get("status"), Limit: limit, Offset: (page - 1) * limit}
}

func pluginTaskPageFromFilter(filter config.PluginTaskLogFilter) int {
	if filter.Limit <= 0 {
		return 1
	}
	return filter.Offset/filter.Limit + 1
}

func isActivePluginTaskStatus(status string) bool {
	return status == config.PluginTaskStatusRunning || status == config.PluginTaskStatusStopping
}
