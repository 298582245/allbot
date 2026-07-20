package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/allbot/allbot/core/config"
)

func (s *Server) handleScriptTasks(w http.ResponseWriter, r *http.Request) {
	database := s.adapterManager.GetDatabase()
	switch r.Method {
	case http.MethodGet:
		settings, err := database.GetScriptTaskSettings()
		if err != nil {
			s.jsonError(w, "读取脚本任务设置失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := database.CleanupScriptRunLogs(settings.RetentionDays); err != nil {
			s.jsonError(w, "清理脚本任务失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		filter := scriptRunLogFilterFromRequest(r)
		total, err := database.CountScriptRunLogs(filter)
		if err != nil {
			s.jsonError(w, "统计脚本任务失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		items, err := database.ListScriptRunLogs(filter)
		if err != nil {
			s.jsonError(w, "获取脚本任务失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"items": items, "total": total, "page": pageFromFilter(filter), "page_size": filter.Limit, "retention_days": settings.RetentionDays, "settings": settings})
	case http.MethodPost:
		switch r.URL.Query().Get("action") {
		case "cleanup":
			s.saveScriptTaskCleanupSettings(w, r)
		case "settings":
			s.saveScriptTaskSettings(w, r)
		default:
			s.jsonError(w, "不支持的脚本任务操作", http.StatusBadRequest)
		}
	default:
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) saveScriptTaskCleanupSettings(w http.ResponseWriter, r *http.Request) {
	days, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("days")))
	if err != nil || days < 0 {
		s.jsonError(w, "清理天数无效", http.StatusBadRequest)
		return
	}
	database := s.adapterManager.GetDatabase()
	settings, err := database.GetScriptTaskSettings()
	if err != nil {
		s.jsonError(w, "读取脚本任务设置失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	settings.RetentionDays = days
	if err := database.SaveScriptTaskSettings(settings); err != nil {
		s.jsonError(w, "保存清理设置失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	removed, err := database.CleanupScriptRunLogs(settings.RetentionDays)
	if err != nil {
		s.jsonError(w, "清理脚本任务失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, map[string]interface{}{"message": "脚本任务清理设置已保存", "retention_days": settings.RetentionDays, "removed": removed})
}

func (s *Server) saveScriptTaskSettings(w http.ResponseWriter, r *http.Request) {
	var settings config.ScriptTaskSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		s.jsonError(w, "脚本任务设置格式无效", http.StatusBadRequest)
		return
	}
	database := s.adapterManager.GetDatabase()
	if err := database.SaveScriptTaskSettings(settings); err != nil {
		s.jsonError(w, "保存脚本任务设置失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	saved, err := database.GetScriptTaskSettings()
	if err != nil {
		s.jsonError(w, "读取脚本任务设置失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	removed, err := database.CleanupScriptRunLogs(saved.RetentionDays)
	if err != nil {
		s.jsonError(w, "清理脚本任务失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, map[string]interface{}{"message": "脚本任务设置已保存", "settings": saved, "removed": removed})
}

func (s *Server) handleScriptTaskDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/script-tasks/"), 10, 64)
	if err != nil || id <= 0 {
		s.jsonError(w, "脚本任务 ID 无效", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.getScriptTask(w, id)
	case http.MethodPut:
		if r.URL.Query().Get("action") != "pause" {
			s.jsonError(w, "不支持的脚本任务操作", http.StatusBadRequest)
			return
		}
		s.pauseScriptTask(w, id)
	case http.MethodDelete:
		s.deleteScriptTask(w, id)
	default:
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) getScriptTask(w http.ResponseWriter, id int64) {
	item, err := s.adapterManager.GetDatabase().GetScriptRunLog(id)
	if err != nil {
		s.jsonError(w, "获取脚本任务失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if item == nil {
		s.jsonError(w, "脚本任务不存在", http.StatusNotFound)
		return
	}
	s.jsonResponse(w, item)
}

func (s *Server) pauseScriptTask(w http.ResponseWriter, id int64) {
	item, err := s.adapterManager.GetDatabase().GetScriptRunLog(id)
	if err != nil {
		s.jsonError(w, "获取脚本任务失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if item == nil {
		s.jsonError(w, "脚本任务不存在", http.StatusNotFound)
		return
	}
	if !isCancelableScriptTaskStatus(item.Status) {
		s.jsonResponse(w, map[string]interface{}{"message": "脚本任务当前不可暂停", "status": item.Status})
		return
	}
	if item.Status == "pausing" {
		s.jsonResponse(w, map[string]interface{}{"message": "脚本任务暂停请求已发送", "status": "pausing"})
		return
	}
	paused := s.pluginManager.PauseScriptRun(id)
	if !paused {
		if item.Status == "running" || item.Status == "pausing" {
			if err := s.adapterManager.GetDatabase().UpdateScriptRunLog(id, "paused", item.Output, "脚本任务已失去运行进程，已标记为暂停", time.Now()); err != nil {
				s.jsonError(w, "更新脚本任务失败: "+err.Error(), http.StatusInternalServerError)
				return
			}
			s.jsonResponse(w, map[string]interface{}{"message": "脚本任务已失去运行进程，已标记为暂停", "status": "paused"})
			return
		}
		s.jsonResponse(w, map[string]interface{}{"message": "脚本任务已不在运行中", "status": item.Status})
		return
	}
	if item.Status == config.ScriptRunStatusQueued {
		s.jsonResponse(w, map[string]interface{}{"message": "脚本任务暂停请求已发送", "status": config.ScriptRunStatusQueued})
		return
	}
	if err := s.adapterManager.GetDatabase().UpdateScriptRunLog(id, "pausing", item.Output, "正在暂停脚本任务", time.Now()); err != nil {
		s.jsonError(w, "更新脚本任务失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, map[string]interface{}{"message": "脚本任务暂停请求已发送", "status": "pausing"})
}

func (s *Server) deleteScriptTask(w http.ResponseWriter, id int64) {
	item, err := s.adapterManager.GetDatabase().GetScriptRunLog(id)
	if err != nil {
		s.jsonError(w, "获取脚本任务失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if item == nil {
		s.jsonError(w, "脚本任务不存在", http.StatusNotFound)
		return
	}
	if isCancelableScriptTaskStatus(item.Status) {
		_ = s.pluginManager.PauseScriptRun(id)
	}
	if err := s.adapterManager.GetDatabase().DeleteScriptRunLog(id); err != nil {
		s.jsonError(w, "删除脚本任务失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, map[string]interface{}{"message": "脚本任务已删除"})
}

func scriptRunLogFilterFromRequest(r *http.Request) config.ScriptRunLogFilter {
	query := r.URL.Query()
	limit, _ := strconv.Atoi(strings.TrimSpace(query.Get("limit")))
	page, _ := strconv.Atoi(strings.TrimSpace(query.Get("page")))
	pageSize, _ := strconv.Atoi(strings.TrimSpace(query.Get("page_size")))
	if pageSize > 0 {
		limit = pageSize
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	if page <= 0 {
		page = 1
	}
	return config.ScriptRunLogFilter{
		Keyword:        query.Get("keyword"),
		UnionID:        query.Get("union_id"),
		PluginID:       query.Get("plugin_id"),
		ScriptPath:     query.Get("script_path"),
		RuntimeProfile: query.Get("runtime_profile"),
		RunMode:        query.Get("run_mode"),
		Status:         normalizeScriptTaskStatus(query.Get("status")),
		Limit:          limit,
		Offset:         (page - 1) * limit,
	}
}

func pageFromFilter(filter config.ScriptRunLogFilter) int {
	if filter.Limit <= 0 {
		return 1
	}
	return filter.Offset/filter.Limit + 1
}

func normalizeScriptTaskStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "queued", "running", "pausing", "paused", "success", "failed", "error":
		return strings.TrimSpace(status)
	default:
		return strings.TrimSpace(status)
	}
}

func isCancelableScriptTaskStatus(status string) bool {
	return status == config.ScriptRunStatusQueued || status == "running" || status == "pausing"
}

func (s *Server) scriptTaskRetentionDays() int {
	settings, err := s.adapterManager.GetDatabase().GetScriptTaskSettings()
	if err != nil {
		return 0
	}
	return settings.RetentionDays
}
