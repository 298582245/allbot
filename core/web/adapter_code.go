package web

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/allbot/allbot/core/adapter/_registry"
)

func (s *Server) handleAdapterCode(w http.ResponseWriter, r *http.Request) {
	platform := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/adapter-code/"), "/")
	if platform == "" || strings.Contains(platform, "/") {
		s.jsonError(w, "适配器 ID 无效", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		desc, ok := registry.Get(platform)
		if !ok || !desc.Custom {
			s.jsonError(w, "仅支持查看自定义适配器文件", http.StatusNotFound)
			return
		}
		if r.URL.Query().Get("tree") == "1" {
			tree, err := s.adapterManager.CustomAdapterFileTree(platform)
			if err != nil {
				s.jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			s.jsonResponse(w, map[string]interface{}{"tree": tree, "entry": desc.Entry})
			return
		}
		filename := r.URL.Query().Get("path")
		if filename == "" {
			filename = desc.Entry
		}
		code, editable, err := s.adapterManager.ReadCustomAdapterFile(platform, filename)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		response := map[string]interface{}{"platform": platform, "filename": filepath.ToSlash(filename), "editable": editable}
		if editable {
			response["code"] = string(code)
			response["sha256"] = sha256Bytes(code)
		}
		s.jsonResponse(w, response)
	case http.MethodPut:
		lock := s.pluginMutationLock("adapter:" + platform)
		lock.Lock()
		defer lock.Unlock()
		var request struct {
			Code           string `json:"code"`
			Path           string `json:"path"`
			ExpectedSHA256 string `json:"expected_sha256"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<20)).Decode(&request); err != nil {
			s.jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}
		if request.Path == "" {
			desc, ok := registry.Get(platform)
			if !ok || !desc.Custom {
				s.jsonError(w, "仅支持修改自定义适配器文件", http.StatusBadRequest)
				return
			}
			request.Path = desc.Entry
		}
		if request.ExpectedSHA256 != "" {
			current, _, err := s.adapterManager.ReadCustomAdapterFile(platform, request.Path)
			if err != nil {
				s.jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			if request.ExpectedSHA256 != sha256Bytes(current) {
				s.jsonError(w, "文件已在编辑器之外修改，请重新加载后再保存", http.StatusConflict)
				return
			}
		}
		if err := s.adapterManager.SaveCustomAdapterFile(platform, request.Path, request.Code); err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"message": "文件已保存并重载", "sha256": sha256Bytes([]byte(request.Code))})
	default:
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
