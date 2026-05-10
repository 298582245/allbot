package vfs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// VirtualFS 虚拟文件系统（内存文件系统）
type VirtualFS struct {
	files map[string]*VirtualFile
	mu    sync.RWMutex
}

// VirtualFile 虚拟文件
type VirtualFile struct {
	Name    string
	Content []byte
	Mode    os.FileMode
	ModTime time.Time
	IsDir   bool
}

// NewVirtualFS 创建虚拟文件系统
func NewVirtualFS() *VirtualFS {
	return &VirtualFS{
		files: make(map[string]*VirtualFile),
	}
}

// WriteFile 写入文件到虚拟文件系统
func (vfs *VirtualFS) WriteFile(path string, content []byte, mode os.FileMode) error {
	vfs.mu.Lock()
	defer vfs.mu.Unlock()

	vfs.files[path] = &VirtualFile{
		Name:    filepath.Base(path),
		Content: content,
		Mode:    mode,
		ModTime: time.Now(),
		IsDir:   false,
	}

	return nil
}

// ReadFile 从虚拟文件系统读取文件
func (vfs *VirtualFS) ReadFile(path string) ([]byte, error) {
	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	file, exists := vfs.files[path]
	if !exists {
		return nil, fmt.Errorf("file not found: %s", path)
	}

	if file.IsDir {
		return nil, fmt.Errorf("is a directory: %s", path)
	}

	return file.Content, nil
}

// Exists 检查文件是否存在
func (vfs *VirtualFS) Exists(path string) bool {
	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	_, exists := vfs.files[path]
	return exists
}

// Remove 删除文件
func (vfs *VirtualFS) Remove(path string) error {
	vfs.mu.Lock()
	defer vfs.mu.Unlock()

	if _, exists := vfs.files[path]; !exists {
		return fmt.Errorf("file not found: %s", path)
	}

	delete(vfs.files, path)
	return nil
}

// List 列出目录下的文件
func (vfs *VirtualFS) List(dir string) ([]string, error) {
	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	var files []string
	for path := range vfs.files {
		if filepath.Dir(path) == dir {
			files = append(files, filepath.Base(path))
		}
	}

	return files, nil
}

// LoadFromDisk 从磁盘加载文件到虚拟文件系统
func (vfs *VirtualFS) LoadFromDisk(diskPath, virtualPath string) error {
	return filepath.Walk(diskPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// 读取文件内容
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// 计算虚拟路径
		relPath, err := filepath.Rel(diskPath, path)
		if err != nil {
			return err
		}
		vPath := filepath.Join(virtualPath, relPath)

		// 写入虚拟文件系统
		return vfs.WriteFile(vPath, content, info.Mode())
	})
}

// SaveToDisk 将虚拟文件系统保存到磁盘
func (vfs *VirtualFS) SaveToDisk(virtualPath, diskPath string) error {
	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	for path, file := range vfs.files {
		if !filepath.HasPrefix(path, virtualPath) {
			continue
		}

		if file.IsDir {
			continue
		}

		// 计算磁盘路径
		relPath, err := filepath.Rel(virtualPath, path)
		if err != nil {
			return err
		}
		dPath := filepath.Join(diskPath, relPath)

		// 创建目录
		if err := os.MkdirAll(filepath.Dir(dPath), 0755); err != nil {
			return err
		}

		// 写入文件
		if err := os.WriteFile(dPath, file.Content, file.Mode); err != nil {
			return err
		}
	}

	return nil
}

// Open 打开文件（实现 io.Reader 接口）
func (vfs *VirtualFS) Open(path string) (io.Reader, error) {
	content, err := vfs.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return &virtualFileReader{
		content: content,
		offset:  0,
	}, nil
}

// virtualFileReader 虚拟文件读取器
type virtualFileReader struct {
	content []byte
	offset  int
}

func (r *virtualFileReader) Read(p []byte) (n int, err error) {
	if r.offset >= len(r.content) {
		return 0, io.EOF
	}

	n = copy(p, r.content[r.offset:])
	r.offset += n
	return n, nil
}

// Clear 清空虚拟文件系统
func (vfs *VirtualFS) Clear() {
	vfs.mu.Lock()
	defer vfs.mu.Unlock()

	vfs.files = make(map[string]*VirtualFile)
}

// Size 获取虚拟文件系统大小
func (vfs *VirtualFS) Size() int {
	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	return len(vfs.files)
}
