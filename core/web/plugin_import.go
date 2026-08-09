package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/allbot/allbot/core/archiveutil"
	plugin "github.com/allbot/allbot/core/plugin"
)

const (
	pluginImportMaxHTTPSize = 64 << 20
	pluginImportMaxFiles    = 5000
	pluginImportMaxFileSize = 64 << 20
	pluginImportMaxTotal    = 256 << 20
	pluginImportMaxConfig   = 1 << 20
)

func (s *Server) handlePluginImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, pluginImportMaxHTTPSize)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		s.jsonError(w, "解析上传文件失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	sourceType := strings.ToLower(strings.TrimSpace(r.FormValue("source_type")))
	if sourceType == "" {
		if _, ok := r.MultipartForm.File["archive"]; ok {
			sourceType = "archive"
		} else {
			sourceType = "directory"
		}
	}
	if sourceType != "archive" && sourceType != "directory" {
		s.jsonError(w, "source_type 必须是 directory 或 archive", http.StatusBadRequest)
		return
	}

	pluginDir := s.pluginManager.PluginDir()
	if strings.TrimSpace(pluginDir) == "" {
		s.jsonError(w, "插件目录未初始化", http.StatusInternalServerError)
		return
	}
	staging, err := os.MkdirTemp(filepath.Join(pluginDir, ".import-staging"), "import-")
	if err != nil {
		if err = os.MkdirAll(filepath.Join(pluginDir, ".import-staging"), 0755); err == nil {
			staging, err = os.MkdirTemp(filepath.Join(pluginDir, ".import-staging"), "import-")
		}
	}
	if err != nil {
		s.jsonError(w, "创建导入暂存目录失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	stagingRoot := filepath.Dir(staging)
	defer func() {
		_ = os.RemoveAll(staging)
		// 没有并发导入任务时一并移除内部暂存父目录，避免长期留下空目录。
		_ = os.Remove(stagingRoot)
	}()
	payload := filepath.Join(staging, "payload")
	if err := os.MkdirAll(payload, 0755); err != nil {
		s.jsonError(w, "创建导入目录失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var sourceName string
	switch sourceType {
	case "archive":
		sourceName, err = s.stagePluginArchive(r, staging)
		if err == nil {
			err = archiveutil.ExtractZipFile(filepath.Join(staging, "archive.zip"), payload, archiveutil.ZipLimits{MaxEntries: pluginImportMaxFiles, MaxFileSize: pluginImportMaxFileSize, MaxTotal: pluginImportMaxTotal})
		}
	case "directory":
		sourceName, err = s.stagePluginDirectory(r, payload)
	}
	if err != nil {
		s.jsonError(w, "插件文件校验失败: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}

	pluginRoot, err := resolveImportedPluginRoot(payload)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	derivedName := filepath.Base(filepath.ToSlash(sourceName))
	if sourceType == "archive" {
		derivedName = strings.TrimSuffix(derivedName, filepath.Ext(derivedName))
	}
	pluginID := sanitizePluginID(r.FormValue("plugin_id"))
	if pluginID == "" {
		pluginID = sanitizePluginID(derivedName)
	}
	if pluginID == "" {
		s.jsonError(w, "插件 ID 无效", http.StatusUnprocessableEntity)
		return
	}
	if err := validateImportedPluginID(pluginID); err != nil {
		s.jsonError(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	configData, err := readImportConfig(pluginRoot)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	var config struct {
		ID      string `json:"id"`
		Runtime string `json:"runtime"`
		Entry   string `json:"entry"`
	}
	if err := json.Unmarshal(configData, &config); err != nil {
		s.jsonError(w, "plugin.json 无法解析: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}
	if manifestID := strings.TrimSpace(config.ID); manifestID != "" {
		normalizedManifestID := sanitizePluginID(manifestID)
		if normalizedManifestID == "" || normalizedManifestID != pluginID {
			s.jsonError(w, fmt.Sprintf("plugin.json 中的 id %q 与导入插件 ID %q 不一致", manifestID, pluginID), http.StatusUnprocessableEntity)
			return
		}
	}
	runtime := strings.ToLower(strings.TrimSpace(config.Runtime))
	if runtime == "node" {
		runtime = "nodejs"
	}
	if runtime == "py" || runtime == "python3" {
		runtime = "python"
	}
	fixedEntry := map[string]string{"nodejs": "main.js", "python": "main.py"}[runtime]
	if fixedEntry == "" {
		s.jsonError(w, "导入插件仅支持 nodejs 或 python runtime", http.StatusUnprocessableEntity)
		return
	}
	if filepath.ToSlash(strings.TrimSpace(config.Entry)) != fixedEntry {
		s.jsonError(w, fmt.Sprintf("%s runtime 的 entry 必须为 %s", runtime, fixedEntry), http.StatusUnprocessableEntity)
		return
	}
	if info, statErr := os.Stat(filepath.Join(pluginRoot, fixedEntry)); statErr != nil || info.IsDir() {
		s.jsonError(w, "插件缺少固定入口文件 "+fixedEntry, http.StatusUnprocessableEntity)
		return
	}
	plugin, err := s.pluginManager.ValidatePluginConfig(pluginRoot, pluginID)
	if err != nil {
		s.jsonError(w, "插件配置无效: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}
	plugin.ID = pluginID

	lock := s.pluginMutationLock(pluginID)
	lock.Lock()
	defer lock.Unlock()
	finalPath := s.pluginManager.PluginPath(pluginID)
	identityExists, err := importedPluginIdentityExists(pluginDir, pluginID)
	if err != nil {
		s.jsonError(w, "检查插件唯一性失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if identityExists || s.pluginManager.GetPlugin(pluginID) != nil || s.router.GetPlugin(pluginID) != nil {
		s.jsonError(w, "插件 ID 已存在", http.StatusConflict)
		return
	}
	if err := os.Rename(pluginRoot, finalPath); err != nil {
		s.jsonError(w, "提交插件目录失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	committed := true
	defer func() {
		if committed {
			_ = os.RemoveAll(finalPath)
		}
	}()
	loaded, err := s.pluginManager.LoadPlugin(finalPath)
	if err != nil {
		s.pluginManager.RemoveLoadedPlugin(pluginID)
		s.jsonError(w, "加载插件失败: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}
	if err := s.router.RegisterPlugin(loaded); err != nil {
		s.router.UnregisterPlugin(pluginID)
		s.pluginManager.RemoveLoadedPlugin(pluginID)
		s.jsonError(w, "注册插件失败: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}
	committed = false
	s.jsonResponse(w, map[string]interface{}{"message": "插件导入成功", "id": pluginID, "plugin": loaded})
}

func (s *Server) stagePluginArchive(r *http.Request, staging string) (string, error) {
	file, header, err := r.FormFile("archive")
	if err != nil {
		return "", fmt.Errorf("请选择 ZIP 插件包")
	}
	defer file.Close()
	output, err := os.OpenFile(filepath.Join(staging, "archive.zip"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", err
	}
	written, copyErr := io.Copy(output, io.LimitReader(file, pluginImportMaxHTTPSize+1))
	closeErr := output.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	if written == 0 || written > pluginImportMaxHTTPSize {
		return "", fmt.Errorf("ZIP 文件大小无效")
	}
	return header.Filename, nil
}

func (s *Server) stagePluginDirectory(r *http.Request, destination string) (string, error) {
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		return "", fmt.Errorf("请选择插件目录文件")
	}
	if len(files) > pluginImportMaxFiles {
		return "", fmt.Errorf("插件文件数量超过限制")
	}
	var paths []string
	if raw := strings.TrimSpace(r.FormValue("paths")); raw != "" {
		if err := json.Unmarshal([]byte(raw), &paths); err != nil {
			return "", fmt.Errorf("paths 字段无效")
		}
	}
	if len(paths) != len(files) {
		return "", fmt.Errorf("files 与 paths 数量不一致")
	}
	normalizedPaths := make([]string, len(paths))
	seen := make(map[string]bool, len(paths))
	seenFold := make(map[string]string, len(paths))
	for index, path := range paths {
		clean, err := normalizeImportPath(path)
		if err != nil {
			return "", err
		}
		if seen[clean] {
			return "", fmt.Errorf("包含重复文件路径: %s", clean)
		}
		folded := strings.ToLower(clean)
		if previous, ok := seenFold[folded]; ok && previous != clean {
			return "", fmt.Errorf("包含大小写冲突路径: %s 与 %s", previous, clean)
		}
		for existing := range seen {
			if strings.HasPrefix(existing, clean+"/") || strings.HasPrefix(clean, existing+"/") {
				return "", fmt.Errorf("包含文件与目录冲突: %s 与 %s", existing, clean)
			}
		}
		seen[clean] = true
		seenFold[folded] = clean
		normalizedPaths[index] = clean
	}
	var total int64
	sourceName := ""
	for i, header := range files {
		clean := normalizedPaths[i]
		if sourceName == "" {
			sourceName = strings.Split(clean, "/")[0]
		}
		file, err := header.Open()
		if err != nil {
			return "", err
		}
		target := filepath.Join(destination, filepath.FromSlash(clean))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			file.Close()
			return "", err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			file.Close()
			return "", fmt.Errorf("写入文件失败: %w", err)
		}
		written, copyErr := io.Copy(output, io.LimitReader(file, pluginImportMaxFileSize+1))
		_ = file.Close()
		closeErr := output.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		if written > pluginImportMaxFileSize {
			return "", fmt.Errorf("单文件超过限制: %s", clean)
		}
		total += written
		if total > pluginImportMaxTotal {
			return "", fmt.Errorf("插件解压后大小超过限制")
		}
	}
	return sourceName, nil
}

func resolveImportedPluginRoot(payload string) (string, error) {
	if _, err := os.Stat(filepath.Join(payload, "plugin.json")); err == nil {
		return payload, nil
	}
	entries, err := os.ReadDir(payload)
	if err != nil {
		return "", err
	}
	if len(entries) != 1 || !entries[0].IsDir() {
		return "", fmt.Errorf("plugin.json 必须位于插件根目录")
	}
	entryInfo, err := entries[0].Info()
	if err != nil || entryInfo.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("插件包装目录不是普通目录")
	}
	root := filepath.Join(payload, entries[0].Name())
	if _, err := os.Stat(filepath.Join(root, "plugin.json")); err != nil {
		return "", fmt.Errorf("插件根目录缺少 plugin.json")
	}
	return root, nil
}

func readImportConfig(root string) ([]byte, error) {
	file, err := os.Open(filepath.Join(root, "plugin.json"))
	if err != nil {
		return nil, fmt.Errorf("插件根目录缺少 plugin.json")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, pluginImportMaxConfig+1))
	if err != nil {
		return nil, err
	}
	if len(data) > pluginImportMaxConfig {
		return nil, fmt.Errorf("plugin.json 超过 1 MiB 限制")
	}
	return data, nil
}

func normalizeImportPath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, "\\") || strings.ContainsRune(value, '\x00') || strings.HasPrefix(value, "/") || filepath.IsAbs(value) || regexp.MustCompile(`^[A-Za-z]:/`).MatchString(value) {
		return "", fmt.Errorf("文件路径无效: %s", value)
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("文件路径越界: %s", value)
	}
	return clean, nil
}

func validateImportedPluginID(value string) error {
	if len(value) > 64 || !regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`).MatchString(value) {
		return fmt.Errorf("插件 ID 无效")
	}
	return nil
}

func importedPluginIdentityExists(pluginDir, pluginID string) (bool, error) {
	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if !entry.IsDir() || plugin.IsInternalPluginDirectory(entry.Name()) {
			continue
		}
		if strings.EqualFold(entry.Name(), pluginID) {
			return true, nil
		}
		data, err := os.ReadFile(filepath.Join(pluginDir, entry.Name(), "plugin.json"))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return false, err
		}
		var config struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(data, &config) == nil && strings.EqualFold(sanitizePluginID(config.ID), pluginID) {
			return true, nil
		}
	}
	return false, nil
}
