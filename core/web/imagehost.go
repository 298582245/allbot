package web

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/imagehost"
)

type imageSettingsRequest struct {
	config.ImageHostSettings
	StorageDirAction string `json:"storage_dir_action"`
}

func (s *Server) handleImages(w http.ResponseWriter, r *http.Request) {
	service, ok := s.requireImageHostService(w)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		query := r.URL.Query()
		items, total, err := service.List(config.ImageAssetQuery{Keyword: strings.TrimSpace(query.Get("keyword")), ContentType: strings.TrimSpace(query.Get("content_type")), Limit: intQueryValue(query.Get("limit"), 20), Offset: intQueryValue(query.Get("offset"), 0)}, r.Host, requestScheme(r))
		if err != nil {
			s.jsonError(w, "获取图片列表失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"items": items, "total": total})
	case http.MethodPost:
		s.handleUploadImage(w, r, service)
	default:
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleImageDetail(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/images/"), "/")
	if path == "settings" {
		s.handleImageSettings(w, r)
		return
	}
	if path == "" || strings.Contains(path, "/") {
		s.jsonError(w, "图片 ID 无效", http.StatusBadRequest)
		return
	}
	service, ok := s.requireImageHostService(w)
	if !ok {
		return
	}
	if r.Method != http.MethodDelete {
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := service.Delete(path); err != nil {
		if errors.Is(err, imagehost.ErrNotFound) || err == sql.ErrNoRows {
			s.jsonError(w, "图片不存在", http.StatusNotFound)
			return
		}
		s.jsonError(w, "删除图片失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, map[string]string{"message": "图片已删除"})
}

func (s *Server) handleImageSettings(w http.ResponseWriter, r *http.Request) {
	service, ok := s.requireImageHostService(w)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		settings, err := service.Settings()
		if err != nil {
			s.jsonError(w, "获取图床配置失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, settings)
	case http.MethodPut:
		var request imageSettingsRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			s.jsonError(w, "请求数据无效", http.StatusBadRequest)
			return
		}
		saved, err := service.SaveSettingsWithOptions(request.ImageHostSettings, imagehost.SaveSettingsOptions{StorageDirAction: request.StorageDirAction})
		if err != nil {
			s.jsonError(w, "保存图床配置失败: "+err.Error(), http.StatusBadRequest)
			return
		}
		s.jsonResponse(w, saved)
	default:
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleUploadImage(w http.ResponseWriter, r *http.Request, service *imagehost.Service) {
	settings, err := service.Settings()
	if err != nil {
		s.jsonError(w, "读取图床配置失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, settings.MaxSize+1024*1024)
	if err := r.ParseMultipartForm(settings.MaxSize + 1024*1024); err != nil {
		s.jsonError(w, "上传表单无效", http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		s.jsonError(w, "图片文件不能为空", http.StatusBadRequest)
		return
	}
	defer file.Close()
	originalName := ""
	if header != nil {
		originalName = header.Filename
	}
	asset, err := service.Upload(imagehost.UploadInput{Reader: file, OriginalName: originalName, DisplayName: r.FormValue("name"), RequestHost: r.Host, RequestScheme: requestScheme(r)})
	if err != nil {
		if errors.Is(err, imagehost.ErrInvalidInput) {
			s.jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.jsonError(w, "上传图片失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, asset)
}

func (s *Server) handleOpenImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.imageHostService == nil {
		s.rawJSONError(w, "图床服务未初始化", http.StatusInternalServerError)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/open/images/")
	asset, err := s.imageHostService.ResolvePublic(path)
	if err != nil {
		if errors.Is(err, imagehost.ErrNotFound) || errors.Is(err, imagehost.ErrInvalidInput) {
			s.rawJSONError(w, "图片不存在", http.StatusNotFound)
			return
		}
		s.rawJSONError(w, "读取图片失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", asset.ContentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	if asset.SHA256 != "" {
		w.Header().Set("ETag", `"`+asset.SHA256+`"`)
	}
	http.ServeFile(w, r, asset.Path)
}

func (s *Server) requireImageHostService(w http.ResponseWriter) (*imagehost.Service, bool) {
	if s.imageHostService == nil {
		s.jsonError(w, "图床服务未初始化", http.StatusInternalServerError)
		return nil, false
	}
	return s.imageHostService, true
}

func requestScheme(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); value != "" {
		return strings.Split(value, ",")[0]
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}
