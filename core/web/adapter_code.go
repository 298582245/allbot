package web

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (s *Server) handleAdapterCode(w http.ResponseWriter, r *http.Request) {
	platform := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/adapter-code/"), "/")
	if platform == "" || strings.Contains(platform, "/") {
		s.jsonError(w, "适配器 ID 无效", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		filename, code, err := s.adapterManager.GetCustomAdapterCode(platform)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"platform": platform, "filename": filename, "code": code})
	case http.MethodPut:
		var request struct {
			Code string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			s.jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		if err := s.adapterManager.SaveCustomAdapterCode(platform, request.Code); err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"message": "代码已保存并重载"})
	default:
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
