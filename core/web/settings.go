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
		var settings config.SystemSettings
		if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
			s.jsonError(w, "请求数据无效", http.StatusBadRequest)
			return
		}
		if err := s.adapterManager.GetDatabase().SaveSystemSettings(&settings); err != nil {
			s.jsonError(w, "保存系统设置失败: "+err.Error(), http.StatusBadRequest)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"message": "保存成功"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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
