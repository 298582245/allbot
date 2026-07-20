package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/allbot/allbot/core/config"
)

func (s *Server) handleScriptEnvs(w http.ResponseWriter, r *http.Request) {
	database := s.adapterManager.GetDatabase()
	switch r.Method {
	case http.MethodGet:
		query := config.ScriptEnvQuery{Keyword: r.URL.Query().Get("keyword")}
		items, err := database.ListScriptEnvVars(query)
		if err != nil {
			s.jsonError(w, "获取脚本环境变量失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"items": items})
	case http.MethodPost:
		var item config.ScriptEnvVar
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			s.jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		saved, err := database.SaveScriptEnvVar(&item)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.jsonResponse(w, saved)
	case http.MethodPatch:
		var request struct {
			Action string  `json:"action"`
			IDs    []int64 `json:"ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			s.jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		affected, err := runScriptEnvBatchAction(database, request.Action, request.IDs)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"message": "批量操作已完成", "affected": affected})
	default:
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleScriptEnvImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var items []config.ScriptEnvImportItem
	if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
		s.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}
	affected, err := s.adapterManager.GetDatabase().ImportScriptEnvVars(items)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.jsonResponse(w, map[string]interface{}{"message": "脚本环境变量已导入", "affected": affected})
}

func runScriptEnvBatchAction(database *config.Database, action string, ids []int64) (int64, error) {
	switch strings.TrimSpace(action) {
	case "delete":
		return database.DeleteScriptEnvVars(ids)
	case "enable":
		return database.UpdateScriptEnvVarsEnabled(ids, true)
	case "disable":
		return database.UpdateScriptEnvVarsEnabled(ids, false)
	case "pin":
		return database.UpdateScriptEnvVarsPinned(ids, true)
	case "unpin":
		return database.UpdateScriptEnvVarsPinned(ids, false)
	default:
		return 0, fmt.Errorf("不支持的批量操作")
	}
}

func (s *Server) handleScriptEnvDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/script-envs/"), 10, 64)
	if err != nil || id <= 0 {
		s.jsonError(w, "脚本环境变量 ID 无效", http.StatusBadRequest)
		return
	}
	database := s.adapterManager.GetDatabase()
	switch r.Method {
	case http.MethodGet:
		item, err := database.GetScriptEnvVar(id)
		if err != nil {
			s.jsonError(w, "获取脚本环境变量失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if item == nil {
			s.jsonError(w, "脚本环境变量不存在", http.StatusNotFound)
			return
		}
		s.jsonResponse(w, item)
	case http.MethodPut:
		var item config.ScriptEnvVar
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			s.jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		item.ID = id
		saved, err := database.SaveScriptEnvVar(&item)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.jsonResponse(w, saved)
	case http.MethodDelete:
		if err := database.DeleteScriptEnvVar(id); err != nil {
			s.jsonError(w, "删除脚本环境变量失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"message": "脚本环境变量已删除"})
	default:
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
