package web

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (s *Server) handleAdapterDefinitions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request struct {
		Manifest       json.RawMessage `json:"manifest"`
		Code           string          `json:"code"`
		RuntimeProfile string          `json:"runtime_profile"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if err := s.adapterManager.CreateCustomAdapterDefinition(request.Manifest, request.Code, request.RuntimeProfile); err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.jsonResponse(w, map[string]string{"message": "自定义适配器已创建"})
}

func (s *Server) handleAdapterDefinitionDetail(w http.ResponseWriter, r *http.Request) {
	platform := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/adapter-definitions/"), "/")
	if platform == "" || strings.Contains(platform, "/") {
		s.jsonError(w, "适配器 ID 无效", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodPut:
		var request struct {
			Manifest       json.RawMessage `json:"manifest"`
			RuntimeProfile string          `json:"runtime_profile"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			s.jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		if err := s.adapterManager.SaveCustomAdapterDefinition(platform, request.Manifest, request.RuntimeProfile); err != nil {
			s.jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.jsonResponse(w, map[string]string{"message": "适配器配置已保存并重载"})
	case http.MethodDelete:
		if err := s.adapterManager.DeleteCustomAdapterDefinition(platform); err != nil {
			s.jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.jsonResponse(w, map[string]string{"message": "自定义适配器已删除"})
	case http.MethodPost:
		var request struct {
			Action string `json:"action"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			s.jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		switch request.Action {
		case "pin", "unpin":
			if err := s.adapterManager.SetAdapterPlatformPinned(platform, request.Action == "pin"); err != nil {
				s.jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
		case "reload":
			if err := s.adapterManager.ReloadCustomAdapterDefinition(platform); err != nil {
				s.jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
		default:
			s.jsonError(w, "不支持的操作", http.StatusBadRequest)
			return
		}
		s.jsonResponse(w, map[string]string{"message": "操作成功"})
	default:
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
