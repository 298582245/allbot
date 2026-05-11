package deps

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// Manager 全局依赖管理器
type Manager struct {
	pythonVenv    string // Python 虚拟环境路径
	nodeModules   string // Node.js 全局 node_modules 路径
	pythonDepsFile string // Python 依赖清单文件
	nodeDepsFile   string // Node.js 依赖清单文件
	mu            sync.RWMutex
}

// PythonDeps Python 依赖清单
type PythonDeps struct {
	Packages map[string]string `json:"packages"` // package: version
}

// NodeDeps Node.js 依赖清单
type NodeDeps struct {
	Dependencies map[string]string `json:"dependencies"`
}

// NewManager 创建依赖管理器
func NewManager(runtimeDir string) *Manager {
	return &Manager{
		pythonVenv:     filepath.Join(runtimeDir, ".venv"),
		nodeModules:    filepath.Join(runtimeDir, "node_modules"),
		pythonDepsFile: filepath.Join(runtimeDir, "python_deps.json"),
		nodeDepsFile:   filepath.Join(runtimeDir, "package.json"),
	}
}

// InitPythonEnv 初始化 Python 虚拟环境
func (m *Manager) InitPythonEnv() error {
	// 检查虚拟环境是否存在
	if _, err := os.Stat(m.pythonVenv); os.IsNotExist(err) {
		fmt.Println("正在创建 Python 虚拟环境...")
		cmd := exec.Command("python", "-m", "venv", m.pythonVenv)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("创建虚拟环境失败: %w", err)
		}
		fmt.Println("Python 虚拟环境创建成功")
	}

	// 初始化依赖清单文件
	if _, err := os.Stat(m.pythonDepsFile); os.IsNotExist(err) {
		deps := PythonDeps{Packages: make(map[string]string)}
		data, _ := json.MarshalIndent(deps, "", "  ")
		os.WriteFile(m.pythonDepsFile, data, 0644)
	}

	return nil
}

// InitNodeEnv 初始化 Node.js 环境
func (m *Manager) InitNodeEnv() error {
	// 创建 runtime 目录
	runtimeDir := filepath.Dir(m.nodeModules)
	os.MkdirAll(runtimeDir, 0755)

	// 初始化 package.json
	if _, err := os.Stat(m.nodeDepsFile); os.IsNotExist(err) {
		fmt.Println("正在初始化 Node.js 环境...")
		deps := NodeDeps{Dependencies: make(map[string]string)}
		data, _ := json.MarshalIndent(deps, "", "  ")
		os.WriteFile(m.nodeDepsFile, data, 0644)
		fmt.Println("Node.js 环境初始化成功")
	}

	return nil
}

// InstallPythonDeps 安装 Python 依赖
func (m *Manager) InstallPythonDeps(deps map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 读取已安装的依赖
	installed, err := m.loadPythonDeps()
	if err != nil {
		return err
	}

	// 获取 pip 路径
	pipPath := m.getPipPath()

	// 安装缺失的依赖
	for pkg, version := range deps {
		installedVersion, exists := installed.Packages[pkg]
		if exists && installedVersion == version {
			fmt.Printf("Python 包 %s==%s 已安装\n", pkg, version)
			continue
		}

		fmt.Printf("正在安装 Python 包: %s==%s\n", pkg, version)
		cmd := exec.Command(pipPath, "install", fmt.Sprintf("%s==%s", pkg, version))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("安装 %s 失败: %w", pkg, err)
		}

		// 更新已安装清单
		installed.Packages[pkg] = version
	}

	// 保存依赖清单
	return m.savePythonDeps(installed)
}

// InstallNodeDeps 安装 Node.js 依赖
func (m *Manager) InstallNodeDeps(deps map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 读取 package.json
	nodeDeps, err := m.loadNodeDeps()
	if err != nil {
		return err
	}

	// 合并依赖
	needInstall := false
	for pkg, version := range deps {
		if nodeDeps.Dependencies[pkg] != version {
			nodeDeps.Dependencies[pkg] = version
			needInstall = true
			fmt.Printf("添加 Node.js 包: %s@%s\n", pkg, version)
		}
	}

	if !needInstall {
		fmt.Println("Node.js 依赖已是最新")
		return nil
	}

	// 保存 package.json
	if err := m.saveNodeDeps(nodeDeps); err != nil {
		return err
	}

	// 执行 npm install
	fmt.Println("正在安装 Node.js 依赖...")
	runtimeDir := filepath.Dir(m.nodeModules)
	cmd := exec.Command("npm", "install")
	cmd.Dir = runtimeDir // 设置工作目录为 runtime 目录
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm install 失败: %w", err)
	}

	fmt.Println("Node.js 依赖安装成功")
	return nil
}

// GetPythonPath 获取 Python 解释器路径
func (m *Manager) GetPythonPath() string {
	if os.PathSeparator == '\\' {
		// Windows
		return filepath.Join(m.pythonVenv, "Scripts", "python.exe")
	}
	// Linux/Mac
	return filepath.Join(m.pythonVenv, "bin", "python")
}

// GetNodePath 获取 Node.js NODE_PATH 环境变量
func (m *Manager) GetNodePath() string {
	return m.nodeModules
}

// getPipPath 获取 pip 路径
func (m *Manager) getPipPath() string {
	if os.PathSeparator == '\\' {
		// Windows
		return filepath.Join(m.pythonVenv, "Scripts", "pip.exe")
	}
	// Linux/Mac
	return filepath.Join(m.pythonVenv, "bin", "pip")
}

// loadPythonDeps 加载 Python 依赖清单
func (m *Manager) loadPythonDeps() (*PythonDeps, error) {
	data, err := os.ReadFile(m.pythonDepsFile)
	if err != nil {
		return &PythonDeps{Packages: make(map[string]string)}, nil
	}

	var deps PythonDeps
	if err := json.Unmarshal(data, &deps); err != nil {
		return nil, err
	}

	if deps.Packages == nil {
		deps.Packages = make(map[string]string)
	}

	return &deps, nil
}

// savePythonDeps 保存 Python 依赖清单
func (m *Manager) savePythonDeps(deps *PythonDeps) error {
	data, err := json.MarshalIndent(deps, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.pythonDepsFile, data, 0644)
}

// loadNodeDeps 加载 Node.js 依赖清单
func (m *Manager) loadNodeDeps() (*NodeDeps, error) {
	data, err := os.ReadFile(m.nodeDepsFile)
	if err != nil {
		return &NodeDeps{Dependencies: make(map[string]string)}, nil
	}

	var deps NodeDeps
	if err := json.Unmarshal(data, &deps); err != nil {
		return nil, err
	}

	if deps.Dependencies == nil {
		deps.Dependencies = make(map[string]string)
	}

	return &deps, nil
}

// saveNodeDeps 保存 Node.js 依赖清单
func (m *Manager) saveNodeDeps(deps *NodeDeps) error {
	data, err := json.MarshalIndent(deps, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.nodeDepsFile, data, 0644)
}
