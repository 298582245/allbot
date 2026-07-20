package web

import (
	"encoding/json"
	"net/http"

	"github.com/allbot/allbot/core/config"
)

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		settings, err := s.adapterManager.GetDatabase().GetSystemSettings()
		if err != nil {
			s.jsonError(w, "获取系统设置失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, settings)
	case http.MethodPut:
		var payload map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			s.jsonError(w, "请求数据无效", http.StatusBadRequest)
			return
		}
		settings, err := s.adapterManager.GetDatabase().GetSystemSettings()
		if err != nil {
			s.jsonError(w, "获取系统设置失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := mergeSystemSettingsPayload(settings, payload); err != nil {
			s.jsonError(w, "请求数据无效", http.StatusBadRequest)
			return
		}
		if err := s.adapterManager.GetDatabase().SaveSystemSettings(settings); err != nil {
			s.jsonError(w, "保存系统设置失败: "+err.Error(), http.StatusBadRequest)
			return
		}
		if s.pluginManager != nil {
			s.pluginManager.SetScriptLimit(settings.ScriptTaskConcurrentLimit)
		}
		s.jsonResponse(w, map[string]interface{}{"message": "保存成功"})
	default:
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func mergeSystemSettingsPayload(settings *config.SystemSettings, payload map[string]json.RawMessage) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, settings)
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	if err := s.adapterManager.GetDatabase().ChangeAdminPassword(req.OldPassword, req.NewPassword); err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.jsonResponse(w, map[string]interface{}{"message": "密码已修改"})
}
