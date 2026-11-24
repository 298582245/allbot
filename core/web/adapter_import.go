package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	registry "github.com/allbot/allbot/core/adapter/_registry"
	customadapter "github.com/allbot/allbot/core/adapter/custom"
	"github.com/allbot/allbot/core/archiveutil"
)

const (
	adapterImportMaxHTTPSize = 64 << 20
	adapterImportMaxFiles    = 5000
	adapterImportMaxFileSize = 64 << 20
	adapterImportMaxTotal    = 256 << 20
	adapterImportMaxManifest = 1 << 20
)

func (s *Server) handleAdapterImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, adapterImportMaxHTTPSize)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		s.jsonError(w, "解析上传文件失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if s.adapterManager == nil {
		s.jsonError(w, "适配器管理器未初始化", http.StatusInternalServerError)
		return
	}
	root := s.adapterManager.CustomAdapterDir()
	if strings.TrimSpace(root) == "" {
		s.jsonError(w, "适配器目录未初始化", http.StatusInternalServerError)
		return
	}
	staging, err := os.MkdirTemp(root, ".adapter-import-")
	if err != nil {
		s.jsonError(w, "创建导入暂存目录失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(staging)
	payload := filepath.Join(staging, "payload")
	if err := os.MkdirAll(payload, 0755); err != nil {
		s.jsonError(w, "创建导入目录失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	sourceType := strings.ToLower(strings.TrimSpace(r.FormValue("source_type")))
	switch sourceType {
	case "archive":
		file, _, openErr := r.FormFile("archive")
		if openErr != nil {
			s.jsonError(w, "请选择 ZIP 适配器包", http.StatusBadRequest)
			return
		}
		zipPath := filepath.Join(staging, "adapter.zip")
		out, createErr := os.OpenFile(zipPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		var written int64
		if createErr == nil {
			written, createErr = io.Copy(out, io.LimitReader(file, adapterImportMaxHTTPSize+1))
			if closeErr := out.Close(); createErr == nil {
				createErr = closeErr
			}
		}
		_ = file.Close()
		if createErr == nil && (written == 0 || written > adapterImportMaxHTTPSize) {
			createErr = fmt.Errorf("ZIP 文件大小无效")
		}
		if createErr != nil {
			s.jsonError(w, "保存 ZIP 文件失败: "+createErr.Error(), http.StatusBadRequest)
			return
		}
		if err := archiveutil.ExtractZipFile(zipPath, payload, archiveutil.ZipLimits{MaxEntries: adapterImportMaxFiles, MaxFileSize: adapterImportMaxFileSize, MaxTotal: adapterImportMaxTotal}); err != nil {
			s.jsonError(w, "适配器 ZIP 校验失败: "+err.Error(), http.StatusUnprocessableEntity)
			return
		}
	case "directory":
		if err := stageAdapterDirectory(r, payload); err != nil {
			s.jsonError(w, "适配器文件校验失败: "+err.Error(), http.StatusUnprocessableEntity)
			return
		}
	default:
		s.jsonError(w, "source_type 必须是 directory 或 archive", http.StatusBadRequest)
		return
	}

	adapterRoot, err := resolveImportedAdapterRoot(payload)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	manifestData, err := readImportedAdapterManifest(adapterRoot)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	var manifest struct {
		ID    string `json:"id"`
		Entry string `json:"entry"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		s.jsonError(w, "adapter.json 无法解析: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}
	manifest.ID = strings.TrimSpace(manifest.ID)
	if manifest.ID == "" || filepath.Base(manifest.ID) != manifest.ID || manifest.ID == "." || manifest.ID == ".." {
		s.jsonError(w, "adapter.json 中的 id 无效", http.StatusUnprocessableEntity)
		return
	}
	if _, exists := registry.Get(manifest.ID); exists {
		s.jsonError(w, "适配器 ID 已存在", http.StatusConflict)
		return
	}
	finalPath := filepath.Join(root, manifest.ID)
	if _, err := os.Lstat(finalPath); err == nil {
		s.jsonError(w, "适配器目录已存在", http.StatusConflict)
		return
	} else if !os.IsNotExist(err) {
		s.jsonError(w, "检查适配器目录失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := os.Rename(adapterRoot, finalPath); err != nil {
		s.jsonError(w, "提交适配器目录失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := customadapter.LoadOne(root, manifest.ID, false); err != nil {
		_ = os.RemoveAll(finalPath)
		s.jsonError(w, "加载适配器失败: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}
	s.jsonResponse(w, map[string]string{"message": "适配器导入成功", "id": manifest.ID})
}

func stageAdapterDirectory(r *http.Request, destination string) error {
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		return fmt.Errorf("请选择适配器目录文件")
	}
	if len(files) > adapterImportMaxFiles {
		return fmt.Errorf("适配器文件数量超过限制")
	}
	var paths []string
	if raw := strings.TrimSpace(r.FormValue("paths")); raw != "" {
		if err := json.Unmarshal([]byte(raw), &paths); err != nil {
			return fmt.Errorf("paths 字段无效")
		}
	}
	if len(paths) != len(files) {
		return fmt.Errorf("files 与 paths 数量不一致")
	}
	seen := make(map[string]bool, len(paths))
	var total int64
	for index, header := range files {
		clean, err := normalizeImportPath(paths[index])
		if err != nil {
			return err
		}
		if seen[clean] {
			return fmt.Errorf("包含重复文件路径: %s", clean)
		}
		seen[clean] = true
		if header.Size > adapterImportMaxFileSize {
			return fmt.Errorf("单文件超过限制: %s", clean)
		}
		file, err := header.Open()
		if err != nil {
			return err
		}
		target := filepath.Join(destination, filepath.FromSlash(clean))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			_ = file.Close()
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			_ = file.Close()
			return fmt.Errorf("写入文件失败: %w", err)
		}
		written, copyErr := io.Copy(out, io.LimitReader(file, adapterImportMaxFileSize+1))
		_ = file.Close()
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if written > adapterImportMaxFileSize {
			return fmt.Errorf("单文件超过限制: %s", clean)
		}
		total += written
		if total > adapterImportMaxTotal {
			return fmt.Errorf("适配器解压后大小超过限制")
		}
	}
	return nil
}

func resolveImportedAdapterRoot(payload string) (string, error) {
	if _, err := os.Stat(filepath.Join(payload, "adapter.json")); err == nil {
		return payload, nil
	}
	entries, err := os.ReadDir(payload)
	if err != nil {
		return "", err
	}
	if len(entries) != 1 || !entries[0].IsDir() || strings.HasPrefix(entries[0].Name(), ".") {
		return "", fmt.Errorf("adapter.json 必须位于适配器根目录")
	}
	root := filepath.Join(payload, entries[0].Name())
	if _, err := os.Stat(filepath.Join(root, "adapter.json")); err != nil {
		return "", fmt.Errorf("适配器根目录缺少 adapter.json")
	}
	return root, nil
}

func readImportedAdapterManifest(root string) ([]byte, error) {
	file, err := os.Open(filepath.Join(root, "adapter.json"))
	if err != nil {
		return nil, fmt.Errorf("适配器根目录缺少 adapter.json")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, adapterImportMaxManifest+1))
	if err != nil {
		return nil, err
	}
	if len(data) > adapterImportMaxManifest {
		return nil, fmt.Errorf("adapter.json 超过 1 MiB 限制")
	}
	return data, nil
}
