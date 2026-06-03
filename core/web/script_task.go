package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/allbot/allbot/core/config"
)

func (s *Server) handleScriptTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		retentionDays := s.scriptTaskRetentionDays()
		if _, err := s.adapterManager.GetDatabase().CleanupScriptRunLogs(retentionDays); err != nil {
			s.jsonError(w, "清理脚本任务失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		filter := scriptRunLogFilterFromRequest(r)
		total, err := s.adapterManager.GetDatabase().CountScriptRunLogs(filter)
		if err != nil {
			s.jsonError(w, "统计脚本任务失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		items, err := s.adapterManager.GetDatabase().ListScriptRunLogs(filter)
		if err != nil {
			s.jsonError(w, "获取脚本任务失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"items": items, "total": total, "page": pageFromFilter(filter), "page_size": filter.Limit, "retention_days": retentionDays})
	case http.MethodPost:
		if r.URL.Query().Get("action") != "cleanup" {
			s.jsonError(w, "不支持的脚本任务操作", http.StatusBadRequest)
			return
		}
		days, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("days")))
		if err != nil || days < 0 {
			s.jsonError(w, "清理天数无效", http.StatusBadRequest)
			return
		}
		if err := s.adapterManager.GetDatabase().SetSetting("script_tasks.retention_days", strconv.Itoa(days), "脚本任务日志自动清理天数，0 表示不自动清理"); err != nil {
			s.jsonError(w, "保存清理设置失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		removed, err := s.adapterManager.GetDatabase().CleanupScriptRunLogs(days)
		if err != nil {
			s.jsonError(w, "清理脚本任务失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"message": "脚本任务清理设置已保存", "retention_days": days, "removed": removed})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
	if item.Status != "running" {
		s.jsonResponse(w, map[string]interface{}{"message": "脚本任务当前不可暂停", "status": item.Status})
		return
	}
	paused := s.pluginManager.PauseScriptRun(id)
	if !paused {
		s.jsonResponse(w, map[string]interface{}{"message": "脚本任务已不在运行中", "status": item.Status})
		return
	}
	if err := s.adapterManager.GetDatabase().UpdateScriptRunLog(id, "pausing", item.Output, "正在暂停脚本任务", time.Now()); err != nil {
		s.jsonError(w, "更新脚本任务失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, map[string]interface{}{"message": "脚本任务暂停请求已发送"})
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
	if item.Status == "running" {
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
		Keyword:    query.Get("keyword"),
		UnionID:    query.Get("union_id"),
		PluginID:   query.Get("plugin_id"),
		ScriptPath: query.Get("script_path"),
		RunMode:    query.Get("run_mode"),
		Status:     query.Get("status"),
		Limit:      limit,
		Offset:     (page - 1) * limit,
	}
}

func pageFromFilter(filter config.ScriptRunLogFilter) int {
	if filter.Limit <= 0 {
		return 1
	}
	return filter.Offset/filter.Limit + 1
}

func (s *Server) scriptTaskRetentionDays() int {
	value, err := s.adapterManager.GetDatabase().GetSetting("script_tasks.retention_days")
	if err != nil {
		return 0
	}
	days, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || days < 0 {
		return 0
	}
	return days
}
