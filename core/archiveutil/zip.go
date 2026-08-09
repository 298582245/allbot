package archiveutil

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
)

// ZipLimits 定义 ZIP 解压时的文件数量和大小上限。
type ZipLimits struct {
	MaxEntries  int
	MaxFileSize int64
	MaxTotal    int64
}

// ExtractZipFile 校验并解压 ZIP 文件，拒绝路径穿越、链接、特殊文件和路径冲突。
func ExtractZipFile(zipPath, destination string, limits ZipLimits) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("ZIP 文件无法读取: %w", err)
	}
	defer reader.Close()
	return ExtractZipFiles(reader.File, destination, limits)
}

// ExtractZipFiles 校验并解压已打开的 ZIP 条目。
func ExtractZipFiles(files []*zip.File, destination string, limits ZipLimits) error {
	if len(files) == 0 {
		return fmt.Errorf("ZIP 文件为空")
	}
	if limits.MaxEntries > 0 && len(files) > limits.MaxEntries {
		return fmt.Errorf("ZIP 条目数量超过限制")
	}
	if err := os.MkdirAll(destination, 0755); err != nil {
		return err
	}

	seen := make(map[string]bool, len(files))
	seenFold := make(map[string]string, len(files))
	filePaths := make(map[string]bool, len(files))
	var total int64
	for _, file := range files {
		name, isDir, err := validateZipPath(file)
		if err != nil {
			return err
		}
		key := strings.TrimSuffix(name, "/")
		if seen[key] {
			return fmt.Errorf("ZIP 包含重复路径: %s", key)
		}
		folded := strings.ToLower(key)
		if previous, ok := seenFold[folded]; ok && previous != key {
			return fmt.Errorf("ZIP 包含大小写冲突路径: %s 与 %s", previous, key)
		}
		for parent := pathpkg.Dir(key); parent != "."; parent = pathpkg.Dir(parent) {
			if filePaths[parent] {
				return fmt.Errorf("ZIP 包含文件与目录冲突: %s", parent)
			}
		}
		if !isDir {
			for existing := range seen {
				if strings.HasPrefix(existing, key+"/") {
					return fmt.Errorf("ZIP 包含文件与目录冲突: %s", key)
				}
			}
			filePaths[key] = true
		}
		seen[key] = true
		seenFold[folded] = key
		if isDir {
			continue
		}
		if limits.MaxFileSize > 0 && file.UncompressedSize64 > uint64(limits.MaxFileSize) {
			return fmt.Errorf("ZIP 单文件超过限制: %s", key)
		}
		if limits.MaxTotal > 0 && file.UncompressedSize64 > uint64(limits.MaxTotal-total) {
			return fmt.Errorf("ZIP 解压后大小超过限制")
		}
		total += int64(file.UncompressedSize64)
	}

	var actualTotal int64
	for _, file := range files {
		name, isDir, err := validateZipPath(file)
		if err != nil {
			return err
		}
		target, err := safeJoin(destination, strings.TrimSuffix(name, "/"))
		if err != nil {
			return err
		}
		if isDir {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := extractFile(file, target, limits, &actualTotal); err != nil {
			return err
		}
	}
	return nil
}

func validateZipPath(file *zip.File) (string, bool, error) {
	name := file.Name
	if strings.TrimSpace(name) == "" || strings.ContainsRune(name, '\x00') {
		return "", false, fmt.Errorf("ZIP 包含空路径")
	}
	if strings.Contains(name, "\\") {
		return "", false, fmt.Errorf("ZIP 包含反斜杠路径: %s", name)
	}
	if strings.HasPrefix(name, "/") || filepath.IsAbs(name) || hasWindowsDrive(name) {
		return "", false, fmt.Errorf("ZIP 包含绝对路径: %s", name)
	}
	clean := pathpkg.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false, fmt.Errorf("ZIP 包含路径穿越: %s", name)
	}
	mode := file.Mode()
	isDir := file.FileInfo().IsDir()
	if mode&os.ModeSymlink != 0 || mode&os.ModeIrregular != 0 || (!isDir && !mode.IsRegular()) {
		return "", false, fmt.Errorf("ZIP 包含不支持的文件类型: %s", clean)
	}
	return clean, isDir, nil
}

func extractFile(file *zip.File, target string, limits ZipLimits, total *int64) error {
	input, err := file.Open()
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	remainingFile := limits.MaxFileSize
	if remainingFile <= 0 {
		remainingFile = 1<<63 - 1
	}
	remainingTotal := limits.MaxTotal - *total
	if limits.MaxTotal <= 0 {
		remainingTotal = 1<<63 - 1
	}
	limit := remainingFile
	if remainingTotal < limit {
		limit = remainingTotal
	}
	written, copyErr := io.Copy(output, io.LimitReader(input, limit+1))
	closeErr := output.Close()
	if copyErr != nil {
		_ = os.Remove(target)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(target)
		return closeErr
	}
	if written > limit {
		_ = os.Remove(target)
		if limits.MaxTotal > 0 && remainingTotal <= remainingFile {
			return fmt.Errorf("ZIP 解压后大小超过限制")
		}
		return fmt.Errorf("ZIP 单文件超过限制: %s", file.Name)
	}
	*total += written
	return nil
}

func safeJoin(root, name string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	targetAbs, err := filepath.Abs(filepath.Join(rootAbs, filepath.FromSlash(name)))
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("ZIP 解压路径越界: %s", name)
	}
	return targetAbs, nil
}

func hasWindowsDrive(name string) bool {
	return len(name) >= 2 && name[1] == ':' && ((name[0] >= 'A' && name[0] <= 'Z') || (name[0] >= 'a' && name[0] <= 'z'))
}
