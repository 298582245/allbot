package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	plugincore "github.com/allbot/allbot/core/plugin"
	"github.com/allbot/allbot/core/types"
)

const (
	pluginWebStaticPrefix = "/plugin-web/"
	pluginWebAPIPrefix    = "/api/plugin-web/"
)

type pluginWebPanel struct {
	PluginID string `json:"plugin_id"`
	Title    string `json:"title"`
	EntryURL string `json:"entry_url"`
	Icon     string `json:"icon,omitempty"`
	Order    int    `json:"order"`
	Enabled  bool   `json:"enabled"`
}

func (s *Server) handlePluginWebPanels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.pluginManager == nil {
		s.jsonResponse(w, []pluginWebPanel{})
		return
	}
	panels := make([]pluginWebPanel, 0)
	for _, process := range s.pluginManager.GetAllPlugins() {
		if process == nil || process.Plugin == nil || !process.Plugin.Enabled || !process.Plugin.WebUI.Enabled {
			continue
		}
		pluginItem := process.Plugin
		pluginPath := s.pluginManager.PluginPath(pluginItem.ID)
		_, entryAbs, err := pluginWebRoot(pluginPath, pluginItem.WebUI)
		if err != nil {
			continue
		}
		title := strings.TrimSpace(pluginItem.WebUI.Title)
		if title == "" {
			title = pluginItem.Name
		}
		panels = append(panels, pluginWebPanel{
			PluginID: pluginItem.ID,
			Title:    title,
			EntryURL: pluginWebEntryURL(pluginItem.ID, filepath.Base(entryAbs)),
			Icon:     pluginItem.WebUI.Icon,
			Order:    pluginItem.WebUI.Order,
			Enabled:  true,
		})
	}
	sort.Slice(panels, func(i, j int) bool {
		if panels[i].Order != panels[j].Order {
			return panels[i].Order < panels[j].Order
		}
		if panels[i].Title != panels[j].Title {
			return panels[i].Title < panels[j].Title
		}
		return panels[i].PluginID < panels[j].PluginID
	})
	s.jsonResponse(w, panels)
}

func (s *Server) handlePluginWebStatic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.accessGranted(r) {
		http.NotFound(w, r)
		return
	}
	pluginID, resourcePath, ok := splitPluginWebPath(r.URL.Path, pluginWebStaticPrefix)
	if !ok {
		http.NotFound(w, r)
		return
	}
	pluginItem, pluginPath, err := s.pluginWebPlugin(pluginID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	root, entryAbs, err := pluginWebRoot(pluginPath, pluginItem.WebUI)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if strings.Trim(resourcePath, "/") == "" {
		resourcePath = filepath.Base(entryAbs)
	}
	fullPath, err := safePluginWebFile(root, resourcePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, fullPath)
}

func (s *Server) handlePluginWebAPI(w http.ResponseWriter, r *http.Request) {
	pluginID, apiPath, ok := splitPluginWebPath(r.URL.Path, pluginWebAPIPrefix)
	if !ok || strings.Trim(apiPath, "/") == "" {
		s.jsonError(w, "插件 Web API 路径无效", http.StatusNotFound)
		return
	}
	pluginItem, pluginPath, err := s.pluginWebPlugin(pluginID)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusNotFound)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.jsonError(w, "读取请求体失败", http.StatusBadRequest)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	requestData, _ := buildOpenAPIRequest(r, "/"+strings.Trim(apiPath, "/"), body)
	requestData.RawPath = r.URL.Path
	payload, err := json.Marshal(map[string]interface{}{
		"event_type": "web_api",
		"plugin_id":  pluginItem.ID,
		"method":     r.Method,
		"path":       requestData.Path,
		"query":      requestData.Query,
		"headers":    requestData.Headers,
		"body":       requestData.Body,
		"json":       requestData.JSON,
		"form":       requestData.Form,
		"request":    requestData,
	})
	if err != nil {
		s.jsonError(w, "构造插件 Web API 请求失败", http.StatusInternalServerError)
		return
	}
	response, err := s.pluginManager.ExecutePluginWeb(pluginItem, pluginPath, payload, s.openAPIDBExecutor(), s.openAPISendMessageExecutor())
	if err != nil {
		s.jsonError(w, "插件 Web API 执行失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writePluginWebResponse(w, response)
}

func (s *Server) pluginWebPlugin(pluginID string) (*types.Plugin, string, error) {
	if s.pluginManager == nil {
		return nil, "", fmt.Errorf("插件管理器不可用")
	}
	process := s.pluginManager.GetPlugin(pluginID)
	if process == nil || process.Plugin == nil {
		return nil, "", fmt.Errorf("插件不存在")
	}
	pluginItem := process.Plugin
	if !pluginItem.Enabled || !pluginItem.WebUI.Enabled {
		return nil, "", fmt.Errorf("插件 WebUI 未启用")
	}
	return pluginItem, s.pluginManager.PluginPath(pluginID), nil
}

func pluginWebRoot(pluginPath string, webUI types.PluginWebUIConfig) (string, string, error) {
	entry := strings.TrimSpace(strings.ReplaceAll(webUI.Entry, "\\", "/"))
	if entry == "" || strings.HasPrefix(entry, "/") || filepath.IsAbs(entry) {
		return "", "", fmt.Errorf("插件 WebUI 入口必须是相对路径")
	}
	entryAbs, err := safePluginWebFile(pluginPath, entry)
	if err != nil {
		return "", "", err
	}
	info, err := os.Stat(entryAbs)
	if err != nil {
		return "", "", err
	}
	if info.IsDir() {
		return "", "", fmt.Errorf("插件 WebUI 入口不能是目录")
	}
	root := filepath.Dir(entryAbs)
	return root, entryAbs, nil
}

func safePluginWebFile(root, relative string) (string, error) {
	cleanRelative := filepath.Clean(strings.TrimPrefix(strings.ReplaceAll(relative, "\\", "/"), "/"))
	if cleanRelative == "." || strings.HasPrefix(cleanRelative, "..") || filepath.IsAbs(cleanRelative) {
		return "", fmt.Errorf("路径无效")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	fullPath, err := filepath.Abs(filepath.Join(rootAbs, cleanRelative))
	if err != nil {
		return "", err
	}
	if fullPath != rootAbs && !strings.HasPrefix(fullPath, rootAbs+string(os.PathSeparator)) {
		return "", fmt.Errorf("路径越界")
	}
	return fullPath, nil
}

func splitPluginWebPath(pathValue, prefix string) (string, string, bool) {
	trimmed := strings.TrimPrefix(pathValue, prefix)
	if trimmed == pathValue || trimmed == "" {
		return "", "", false
	}
	parts := strings.SplitN(trimmed, "/", 2)
	pluginID, err := url.PathUnescape(strings.TrimSpace(parts[0]))
	if err != nil || pluginID == "" || strings.ContainsAny(pluginID, `/\\`) || pluginID == "." || strings.HasPrefix(pluginID, "..") {
		return "", "", false
	}
	if len(parts) == 1 {
		return pluginID, "", true
	}
	return pluginID, parts[1], true
}

func pluginWebEntryURL(pluginID, entryFile string) string {
	return pluginWebStaticPrefix + url.PathEscape(pluginID) + "/" + url.PathEscape(entryFile)
}

func writePluginWebResponse(w http.ResponseWriter, response plugincore.PluginWebResponse) {
	allowedHeaders := map[string]bool{"content-type": true, "content-disposition": true}
	for key, value := range response.Headers {
		if allowedHeaders[strings.ToLower(strings.TrimSpace(key))] {
			w.Header().Set(key, value)
		}
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	status := response.Status
	if status <= 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	if response.JSON != nil {
		_ = json.NewEncoder(w).Encode(response.JSON)
		return
	}
	if response.Data != nil {
		_ = json.NewEncoder(w).Encode(response.Data)
		return
	}
	contentType, _, _ := mime.ParseMediaType(w.Header().Get("Content-Type"))
	if contentType == "application/json" && strings.TrimSpace(response.Body) == "" {
		_, _ = w.Write([]byte("{}"))
		return
	}
	_, _ = w.Write([]byte(response.Body))
}
