package deps

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// Manager 全局依赖管理器。
type Manager struct {
	pythonVenv     string // Python 虚拟环境路径
	nodeModules    string // Node.js 全局 node_modules 路径
	pythonDepsFile string // Python 依赖清单文件
	nodeDepsFile   string // Node.js 依赖清单文件
	mu             sync.RWMutex
}

// PythonDeps Python 依赖清单。
type PythonDeps struct {
	Packages map[string]string `json:"packages"`
}

// NodeDeps Node.js 依赖清单。
type NodeDeps struct {
	Dependencies map[string]string `json:"dependencies"`
}

// NewManager 创建依赖管理器。
func NewManager(runtimeDir string) *Manager {
	return &Manager{
		pythonVenv:     filepath.Join(runtimeDir, ".venv"),
		nodeModules:    filepath.Join(runtimeDir, "node_modules"),
		pythonDepsFile: filepath.Join(runtimeDir, "python_deps.json"),
		nodeDepsFile:   filepath.Join(runtimeDir, "package.json"),
	}
}

// InitPythonEnv 初始化 Python 虚拟环境。
func (m *Manager) InitPythonEnv() error {
	if _, err := os.Stat(m.pythonVenv); os.IsNotExist(err) {
		fmt.Println("正在创建 Python 虚拟环境...")
		cmd := exec.Command("python", "-m", "venv", m.pythonVenv)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("创建虚拟环境失败: %w", err)
		}
		fmt.Println("Python 虚拟环境创建成功")
	}

	if _, err := os.Stat(m.pythonDepsFile); os.IsNotExist(err) {
		deps := PythonDeps{Packages: make(map[string]string)}
		data, _ := json.MarshalIndent(deps, "", "  ")
		if err := os.WriteFile(m.pythonDepsFile, data, 0644); err != nil {
			return err
		}
	}

	return nil
}

// InitNodeEnv 初始化 Node.js 环境。
func (m *Manager) InitNodeEnv() error {
	runtimeDir := filepath.Dir(m.nodeModules)
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		return err
	}

	if _, err := os.Stat(m.nodeDepsFile); os.IsNotExist(err) {
		fmt.Println("正在初始化 Node.js 环境...")
		deps := NodeDeps{Dependencies: make(map[string]string)}
		data, _ := json.MarshalIndent(deps, "", "  ")
		if err := os.WriteFile(m.nodeDepsFile, data, 0644); err != nil {
			return err
		}
		fmt.Println("Node.js 环境初始化成功")
	}

	return nil
}

// InstallPythonDeps 安装 Python 依赖，版本为空或 latest 时安装最新版并记录实际版本号。
func (m *Manager) InstallPythonDeps(deps map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	installed, err := m.loadPythonDeps()
	if err != nil {
		return err
	}

	pipPath := m.getPipPath()
	for pkg, version := range deps {
		packageSpec := pkg
		forceUpgrade := version == "" || version == "latest"
		if forceUpgrade {
			fmt.Printf("正在安装或更新 Python 包: %s（最新版）\n", pkg)
		} else {
			if installedVersion, exists := installed.Packages[pkg]; exists && isVersionSatisfied(installedVersion, version) {
				fmt.Printf("Python 包 %s==%s 已安装\n", pkg, version)
				continue
			}
			fmt.Printf("正在安装 Python 包: %s==%s\n", pkg, version)
			packageSpec = fmt.Sprintf("%s==%s", pkg, version)
		}

		args := []string{"install"}
		if forceUpgrade {
			args = append(args, "--upgrade")
		}
		args = append(args, packageSpec)
		cmd := exec.Command(pipPath, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("安装 %s 失败: %w", pkg, err)
		}

		actualVersion, err := m.getPythonPackageVersion(pkg)
		if err != nil {
			return err
		}
		installed.Packages[pkg] = actualVersion
	}

	return m.savePythonDeps(installed)
}

// InstallNodeDeps 安装 Node.js 依赖，版本为空或 latest 时安装最新版并记录实际版本号。
func (m *Manager) InstallNodeDeps(deps map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	nodeDeps, err := m.loadNodeDeps()
	if err != nil {
		return err
	}

	runtimeDir := filepath.Dir(m.nodeModules)
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		return err
	}

	for pkg, version := range deps {
		forceLatest := version == "" || version == "latest"
		packageSpec := pkg
		if forceLatest {
			if installedVersion := nodeDeps.Dependencies[pkg]; installedVersion != "" {
				fmt.Printf("Node.js 包 %s@%s 已安装\n", pkg, installedVersion)
				continue
			}
			if actualVersion, err := m.getNodePackageVersion(pkg); err == nil && actualVersion != "" {
				nodeDeps.Dependencies[pkg] = actualVersion
				fmt.Printf("Node.js 包 %s@%s 已安装\n", pkg, actualVersion)
				continue
			}
			fmt.Printf("正在安装 Node.js 包: %s（最新版）\n", pkg)
		} else {
			if installedVersion := nodeDeps.Dependencies[pkg]; isVersionSatisfied(installedVersion, version) {
				fmt.Printf("Node.js 包 %s@%s 已安装\n", pkg, version)
				continue
			}
			fmt.Printf("正在安装 Node.js 包: %s@%s\n", pkg, version)
			packageSpec = fmt.Sprintf("%s@%s", pkg, version)
		}

		cmd := exec.Command("npm", "install", packageSpec, "--save-exact")
		cmd.Dir = runtimeDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("npm install 失败: %w", err)
		}

		actualVersion, err := m.getNodePackageVersion(pkg)
		if err != nil {
			return err
		}
		nodeDeps.Dependencies[pkg] = actualVersion
	}

	return m.saveNodeDeps(nodeDeps)
}

func isVersionSatisfied(installedVersion, requestedVersion string) bool {
	installedVersion = strings.TrimSpace(installedVersion)
	requestedVersion = strings.TrimSpace(requestedVersion)
	if installedVersion == "" || requestedVersion == "" || requestedVersion == "latest" {
		return false
	}
	if installedVersion == requestedVersion {
		return true
	}
	trimmedRequested := strings.TrimLeft(requestedVersion, "^~=")
	trimmedInstalled := strings.TrimLeft(installedVersion, "^~=")
	return trimmedInstalled == trimmedRequested
}

// GetPythonPath 获取 Python 解释器路径。
func (m *Manager) GetPythonPath() string {
	pythonPath := ""
	if os.PathSeparator == '\\' {
		pythonPath = filepath.Join(m.pythonVenv, "Scripts", "python.exe")
	} else {
		pythonPath = filepath.Join(m.pythonVenv, "bin", "python")
	}
	if absPath, err := filepath.Abs(pythonPath); err == nil {
		return absPath
	}
	return pythonPath
}

// GetNodePath 获取 Node.js NODE_PATH 环境变量。
func (m *Manager) GetNodePath() string {
	if absPath, err := filepath.Abs(m.nodeModules); err == nil {
		return absPath
	}
	return m.nodeModules
}

func (m *Manager) getPipPath() string {
	pipPath := ""
	if os.PathSeparator == '\\' {
		pipPath = filepath.Join(m.pythonVenv, "Scripts", "pip.exe")
	} else {
		pipPath = filepath.Join(m.pythonVenv, "bin", "pip")
	}
	if absPath, err := filepath.Abs(pipPath); err == nil {
		return absPath
	}
	return pipPath
}

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

func (m *Manager) savePythonDeps(deps *PythonDeps) error {
	data, err := json.MarshalIndent(deps, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.pythonDepsFile, data, 0644)
}

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

func (m *Manager) saveNodeDeps(deps *NodeDeps) error {
	data, err := json.MarshalIndent(deps, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.nodeDepsFile, data, 0644)
}

// GetPythonDeps 获取已安装的 Python 依赖。
func (m *Manager) GetPythonDeps() (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	deps, err := m.loadPythonDeps()
	if err != nil {
		return nil, err
	}
	changed := false
	for pkg, version := range deps.Packages {
		if version != "latest" && version != "" {
			continue
		}
		actualVersion, err := m.getPythonPackageVersion(pkg)
		if err == nil && actualVersion != "" {
			deps.Packages[pkg] = actualVersion
			changed = true
		}
	}
	if changed {
		if err := m.savePythonDeps(deps); err != nil {
			return nil, err
		}
	}
	return deps.Packages, nil
}

// GetNodeDeps 获取已安装的 Node.js 依赖。
func (m *Manager) GetNodeDeps() (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	deps, err := m.loadNodeDeps()
	if err != nil {
		return nil, err
	}
	changed := false
	for pkg, version := range deps.Dependencies {
		if version != "latest" && version != "" {
			continue
		}
		actualVersion, err := m.getNodePackageVersion(pkg)
		if err == nil && actualVersion != "" {
			deps.Dependencies[pkg] = actualVersion
			changed = true
		}
	}
	if changed {
		if err := m.saveNodeDeps(deps); err != nil {
			return nil, err
		}
	}
	return deps.Dependencies, nil
}

// UninstallPythonDep 卸载 Python 依赖。
func (m *Manager) UninstallPythonDep(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	installed, err := m.loadPythonDeps()
	if err != nil {
		return err
	}
	if _, exists := installed.Packages[name]; !exists {
		return fmt.Errorf("依赖 %s 未安装", name)
	}

	pipPath := m.getPipPath()
	fmt.Printf("正在卸载 Python 包: %s\n", name)
	cmd := exec.Command(pipPath, "uninstall", "-y", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("卸载 %s 失败: %w", name, err)
	}

	delete(installed.Packages, name)
	return m.savePythonDeps(installed)
}

// UninstallNodeDep 卸载 Node.js 依赖。
func (m *Manager) UninstallNodeDep(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	nodeDeps, err := m.loadNodeDeps()
	if err != nil {
		return err
	}
	if _, exists := nodeDeps.Dependencies[name]; !exists {
		return fmt.Errorf("依赖 %s 未安装", name)
	}

	fmt.Printf("正在卸载 Node.js 包: %s\n", name)
	runtimeDir := filepath.Dir(m.nodeModules)
	cmd := exec.Command("npm", "uninstall", name)
	cmd.Dir = runtimeDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm uninstall 失败: %w", err)
	}

	delete(nodeDeps.Dependencies, name)
	if err := m.saveNodeDeps(nodeDeps); err != nil {
		return err
	}
	fmt.Println("Node.js 依赖卸载成功")
	return nil
}

func (m *Manager) getPythonPackageVersion(pkg string) (string, error) {
	cmd := exec.Command(m.GetPythonPath(), "-m", "pip", "show", pkg)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("获取 Python 包 %s 版本失败: %w", pkg, err)
	}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "Version:") {
			version := strings.TrimSpace(strings.TrimPrefix(line, "Version:"))
			if version != "" {
				return version, nil
			}
		}
	}
	return "", fmt.Errorf("获取 Python 包 %s 版本失败: 未找到版本号", pkg)
}

func (m *Manager) getNodePackageVersion(pkg string) (string, error) {
	packageJSON := filepath.Join(m.nodeModules, pkg, "package.json")
	data, err := os.ReadFile(packageJSON)
	if err != nil {
		return "", fmt.Errorf("获取 Node.js 包 %s 版本失败: %w", pkg, err)
	}
	var info struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return "", err
	}
	if info.Version == "" {
		return "", fmt.Errorf("获取 Node.js 包 %s 版本失败: 未找到版本号", pkg)
	}
	return info.Version, nil
}
