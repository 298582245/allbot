package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/allbot/allbot/core/adapter/_registry"
	customadapter "github.com/allbot/allbot/core/adapter/custom"
)

const adapterEditorMaxFileSize = 2 << 20

type AdapterFileNode struct {
	Name     string            `json:"name"`
	Path     string            `json:"path"`
	Type     string            `json:"type"`
	Children []AdapterFileNode `json:"children,omitempty"`
}

func (m *AdapterManager) customAdapterFileRoot(platform string) (string, error) {
	desc, ok := registry.Get(platform)
	if !ok || !desc.Custom {
		return "", fmt.Errorf("仅支持查看和修改自定义适配器文件")
	}
	return filepath.EvalSymlinks(filepath.Join(m.CustomAdapterDir(), platform))
}

func (m *AdapterManager) CustomAdapterFileTree(platform string) ([]AdapterFileNode, error) {
	root, err := m.customAdapterFileRoot(platform)
	if err != nil {
		return nil, err
	}
	var walk func(string) ([]AdapterFileNode, error)
	walk = func(relative string) ([]AdapterFileNode, error) {
		entries, err := os.ReadDir(filepath.Join(root, relative))
		if err != nil {
			return nil, err
		}
		nodes := make([]AdapterFileNode, 0, len(entries))
		for _, entry := range entries {
			name := entry.Name()
			if name == "node_modules" || name == "__pycache__" || name == ".venv" || strings.HasPrefix(name, ".git") || entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			path := filepath.Join(relative, name)
			node := AdapterFileNode{Name: name, Path: filepath.ToSlash(path), Type: "file"}
			if entry.IsDir() {
				node.Type = "directory"
				node.Children, err = walk(path)
				if err != nil {
					return nil, err
				}
			}
			nodes = append(nodes, node)
		}
		return nodes, nil
	}
	return walk("")
}

func (m *AdapterManager) customAdapterFilePath(platform, relative string) (string, error) {
	root, err := m.customAdapterFileRoot(platform)
	if err != nil {
		return "", err
	}
	relative = strings.ReplaceAll(relative, "\\", "/")
	clean := filepath.Clean(relative)
	if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || strings.Contains(clean, ":") {
		return "", fmt.Errorf("文件路径无效")
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(root, clean))
	if err != nil {
		return "", err
	}
	within, err := filepath.Rel(root, resolved)
	if err != nil || within == ".." || strings.HasPrefix(within, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("文件路径越界")
	}
	return resolved, nil
}

func (m *AdapterManager) ReadCustomAdapterFile(platform, relative string) ([]byte, bool, error) {
	path, err := m.customAdapterFilePath(platform, relative)
	if err != nil {
		return nil, false, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, false, err
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("请选择普通文件")
	}
	if info.Size() > adapterEditorMaxFileSize {
		return nil, false, nil
	}
	data, err := io.ReadAll(io.LimitReader(file, adapterEditorMaxFileSize+1))
	if err != nil {
		return nil, false, err
	}
	editable := len(data) <= adapterEditorMaxFileSize && utf8.Valid(data) && !bytes.ContainsRune(data, 0)
	return data, editable, nil
}

func (m *AdapterManager) SaveCustomAdapterFile(platform, relative, code string) error {
	path, err := m.customAdapterFilePath(platform, relative)
	if err != nil {
		return err
	}
	if _, editable, err := m.ReadCustomAdapterFile(platform, relative); err != nil {
		return err
	} else if !editable {
		return fmt.Errorf("仅支持编辑 2 MiB 以内的 UTF-8 文本文件")
	}
	if len(code) > adapterEditorMaxFileSize || !utf8.ValidString(code) || strings.ContainsRune(code, 0) {
		return fmt.Errorf("仅支持编辑 2 MiB 以内的 UTF-8 文本文件")
	}
	root, err := m.customAdapterFileRoot(platform)
	if err != nil {
		return err
	}
	relativePath, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if strings.EqualFold(relativePath, "adapter.json") {
		if err := customadapter.SaveDefinition(m.CustomAdapterDir(), platform, json.RawMessage(code)); err != nil {
			return err
		}
	} else if strings.EqualFold(relativePath, "adapter.data") && !json.Valid([]byte(code)) {
		return fmt.Errorf("adapter.data 必须是有效 JSON")
	} else if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		return err
	}
	return m.ReloadCustomAdapterDefinition(platform)
}
