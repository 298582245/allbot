package web

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/router"
)

func (s *Server) handleScheduledTasks(w http.ResponseWriter, r *http.Request) {
	database := s.adapterManager.GetDatabase()
	if r.Method == http.MethodGet {
		items, err := database.ListScheduledTasks()
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, items)
		return
	}
	if r.Method == http.MethodPost {
		var item config.ScheduledTask
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			s.jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		if err := setScheduledTaskNextRun(&item); err != nil {
			s.jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := database.SaveScheduledTask(&item); err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"message": "定时任务已保存"})
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleScheduledTaskDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/scheduled-tasks/"), 10, 64)
	if err != nil || id <= 0 {
		s.jsonError(w, "定时任务 ID 无效", http.StatusBadRequest)
		return
	}
	database := s.adapterManager.GetDatabase()
	if r.Method == http.MethodPut {
		if r.URL.Query().Get("action") == "run" {
			s.runScheduledTaskNow(w, id)
			return
		}
		var item config.ScheduledTask
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			s.jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		item.ID = id
		if err := setScheduledTaskNextRun(&item); err != nil {
			s.jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := database.SaveScheduledTask(&item); err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"message": "定时任务已更新"})
		return
	}
	if r.Method == http.MethodDelete {
		if err := database.DeleteScheduledTask(id); err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"message": "定时任务已删除"})
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) runScheduledTaskNow(w http.ResponseWriter, id int64) {
	database := s.adapterManager.GetDatabase()
	task, err := database.GetScheduledTask(id)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if task == nil {
		s.jsonError(w, "定时任务不存在", http.StatusNotFound)
		return
	}
	pluginID := strings.TrimSpace(task.PluginID)
	if pluginID == "" {
		pluginID = "scheduled-task-" + strconv.FormatInt(task.ID, 10)
	}
	if err := s.router.DispatchFakeMessageWithAdapter(pluginID, task.Platform, task.AdapterID, task.UserID, task.GroupID, task.Content); err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	var nextRunAt *time.Time
	if task.Enabled && !isOnceScheduledTask(task.Cron) {
		next, err := router.NextCronTime(task.Cron, time.Now())
		if err != nil {
			nextRunAt = nil
		} else {
			nextRunAt = &next
		}
	}
	if err := database.MarkScheduledTaskRun(task.ID, time.Now(), nextRunAt); err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, map[string]interface{}{"message": "定时任务已立即执行"})
}

func setScheduledTaskNextRun(item *config.ScheduledTask) error {
	if item.Enabled && !isOnceScheduledTask(item.Cron) {
		next, err := router.NextCronTime(item.Cron, time.Now())
		if err != nil {
			return err
		}
		item.NextRunAt = &next
	} else {
		item.NextRunAt = nil
	}
	return nil
}

func isOnceScheduledTask(expression string) bool {
	return strings.EqualFold(strings.TrimSpace(expression), "@once")
}
