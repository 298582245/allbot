package plugin

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/allbot/allbot/core/deps"
	"github.com/allbot/allbot/core/types"
)

// Manager 插件管理器
type Manager struct {
	plugins    map[string]*PluginProcess
	mu         sync.RWMutex
	pluginDir  string
	depsManager *deps.Manager // 依赖管理器
}

// PluginProcess 插件进程
type PluginProcess struct {
	Plugin  *types.Plugin
	Cmd     *exec.Cmd
	Port    int    // gRPC 端口
	Status  string // running/stopped/error
}

// NewManager 创建插件管理器
func NewManager(pluginDir string, depsManager *deps.Manager) *Manager {
	return &Manager{
		plugins:     make(map[string]*PluginProcess),
		pluginDir:   pluginDir,
		depsManager: depsManager,
	}
}

// LoadPlugin 加载插件
func (m *Manager) LoadPlugin(pluginPath string) (*types.Plugin, error) {
	// 读取 plugin.json
	configPath := filepath.Join(pluginPath, "plugin.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin.json: %w", err)
	}

	var config types.PluginConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse plugin.json: %w", err)
	}

	// 生成插件ID（使用目录名）
	pluginID := filepath.Base(pluginPath)

	// 安装依赖（失败不阻塞插件注册）
	if len(config.Dependencies) > 0 {
		log.Printf("Installing dependencies for plugin: %s", config.Name)

		switch config.Runtime {
		case "python":
			if err := m.depsManager.InstallPythonDeps(config.Dependencies); err != nil {
				log.Printf("[SYSTEM] 警告：安装插件 %s 的Python依赖失败: %v", config.Name, err)
			}
		case "nodejs":
			if err := m.depsManager.InstallNodeDeps(config.Dependencies); err != nil {
				log.Printf("[SYSTEM] 警告：安装插件 %s 的Node.js依赖失败: %v", config.Name, err)
			}
		}
	}

	plugin := &types.Plugin{
		ID:        pluginID,
		Name:      config.Name,
		Version:   config.Version,
		Runtime:   config.Runtime,
		Entry:     config.Entry,
		Platforms: config.Platforms,
		Trigger:   config.Trigger,
		Enabled:   config.Enabled,
	}

	// 在插件管理器中注册插件（不再需要启动进程）
	m.mu.Lock()
	if _, exists := m.plugins[pluginID]; !exists {
		m.plugins[pluginID] = &PluginProcess{
			Plugin: plugin,
			Status: "ready", // 改为ready状态，表示可以执行
			Port:   0,
		}
	}
	m.mu.Unlock()

	return plugin, nil
}

// StartPlugin 启动插件进程
func (m *Manager) StartPlugin(plugin *types.Plugin, pluginPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已启动
	if _, exists := m.plugins[plugin.ID]; exists {
		return fmt.Errorf("plugin already started: %s", plugin.ID)
	}

	// 分配端口（简单实现，实际应该动态分配）
	port := 50051 + len(m.plugins)

	var cmd *exec.Cmd
	entryPath := filepath.Join(pluginPath, plugin.Entry)

	switch plugin.Runtime {
	case "python":
		// 使用全局虚拟环境的 Python
		pythonPath := m.depsManager.GetPythonPath()

		// 转换为绝对路径
		absPythonPath, err := filepath.Abs(pythonPath)
		if err != nil {
			return fmt.Errorf("无法获取Python绝对路径: %w", err)
		}

		// 检查Python路径是否存在
		if _, err := os.Stat(absPythonPath); os.IsNotExist(err) {
			return fmt.Errorf("Python解释器不存在: %s", absPythonPath)
		}

		// 检查插件入口文件是否存在
		if _, err := os.Stat(entryPath); os.IsNotExist(err) {
			return fmt.Errorf("插件入口文件不存在: %s", entryPath)
		}

		log.Printf("启动Python插件: python=%s, entry=%s, dir=%s", absPythonPath, plugin.Entry, pluginPath)

		cmd = exec.Command(absPythonPath, plugin.Entry, fmt.Sprintf("--port=%d", port))
		// 设置工作目录为插件目录
		cmd.Dir = pluginPath
		// 捕获标准输出和错误输出
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	case "nodejs":
		// 使用 Node.js，设置 NODE_PATH 指向全局 node_modules
		cmd = exec.Command("node", entryPath, fmt.Sprintf("--port=%d", port))
		// 设置工作目录为插件目录
		cmd.Dir = pluginPath
		nodePath := m.depsManager.GetNodePath()
		cmd.Env = append(os.Environ(),
			fmt.Sprintf("NODE_PATH=%s", nodePath),
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	default:
		return fmt.Errorf("unsupported runtime: %s", plugin.Runtime)
	}

	// 设置环境变量
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env,
		fmt.Sprintf("ALLBOT_PLUGIN_ID=%s", plugin.ID),
		fmt.Sprintf("ALLBOT_GRPC_PORT=%d", port),
	)

	// 启动进程
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start plugin: %w", err)
	}

	process := &PluginProcess{
		Plugin: plugin,
		Cmd:    cmd,
		Port:   port,
		Status: "running",
	}

	m.plugins[plugin.ID] = process

	log.Printf("Plugin started: %s (runtime: %s, port: %d)", plugin.Name, plugin.Runtime, port)

	// 监控进程退出
	go m.monitorProcess(plugin.ID, cmd)

	return nil
}

// StopPlugin 停止插件
func (m *Manager) StopPlugin(pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	process, exists := m.plugins[pluginID]
	if !exists {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}

	if process.Cmd != nil && process.Cmd.Process != nil {
		if err := process.Cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill plugin process: %w", err)
		}
	}

	// 不删除插件，只标记为停止状态
	process.Status = "stopped"
	process.Cmd = nil
	log.Printf("Plugin stopped: %s", pluginID)

	return nil
}

// StartPluginByID 通过ID启动已停止的插件
func (m *Manager) StartPluginByID(pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	process, exists := m.plugins[pluginID]
	if !exists {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}

	if process.Status == "running" {
		return fmt.Errorf("plugin already running: %s", pluginID)
	}

	// 重新启动插件
	pluginPath := filepath.Join(m.pluginDir, pluginID)
	return m.startPluginProcess(process.Plugin, pluginPath)
}

// startPluginProcess 启动插件进程（内部方法，调用时需要持有锁）
func (m *Manager) startPluginProcess(plugin *types.Plugin, pluginPath string) error {
	port := 50051 + len(m.plugins)

	var cmd *exec.Cmd
	entryPath := filepath.Join(pluginPath, plugin.Entry)

	switch plugin.Runtime {
	case "python":
		pythonPath := m.depsManager.GetPythonPath()

		// 转换为绝对路径
		absPythonPath, err := filepath.Abs(pythonPath)
		if err != nil {
			return fmt.Errorf("无法获取Python绝对路径: %w", err)
		}

		cmd = exec.Command(absPythonPath, plugin.Entry, fmt.Sprintf("--port=%d", port))
		// 设置工作目录为插件目录
		cmd.Dir = pluginPath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	case "nodejs":
		cmd = exec.Command("node", entryPath, fmt.Sprintf("--port=%d", port))
		// 设置工作目录为插件目录
		cmd.Dir = pluginPath
		nodePath := m.depsManager.GetNodePath()
		cmd.Env = append(os.Environ(),
			fmt.Sprintf("NODE_PATH=%s", nodePath),
		)
	default:
		return fmt.Errorf("unsupported runtime: %s", plugin.Runtime)
	}

	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env,
		fmt.Sprintf("ALLBOT_PLUGIN_ID=%s", plugin.ID),
		fmt.Sprintf("ALLBOT_GRPC_PORT=%d", port),
	)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start plugin: %w", err)
	}

	// 更新进程信息
	if process, exists := m.plugins[plugin.ID]; exists {
		process.Cmd = cmd
		process.Port = port
		process.Status = "running"
	} else {
		m.plugins[plugin.ID] = &PluginProcess{
			Plugin: plugin,
			Cmd:    cmd,
			Port:   port,
			Status: "running",
		}
	}

	log.Printf("Plugin started: %s (runtime: %s, port: %d)", plugin.Name, plugin.Runtime, port)

	go m.monitorProcess(plugin.ID, cmd)

	return nil
}

// monitorProcess 监控进程退出
func (m *Manager) monitorProcess(pluginID string, cmd *exec.Cmd) {
	err := cmd.Wait()

	m.mu.Lock()
	defer m.mu.Unlock()

	if process, exists := m.plugins[pluginID]; exists {
		if err != nil {
			process.Status = "error"
			log.Printf("Plugin process exited with error: %s, error: %v", pluginID, err)
		} else {
			process.Status = "stopped"
			log.Printf("Plugin process exited: %s", pluginID)
		}
	}
}

// GetPlugin 获取插件进程
func (m *Manager) GetPlugin(pluginID string) *PluginProcess {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.plugins[pluginID]
}

// GetAllPlugins 获取所有插件
func (m *Manager) GetAllPlugins() []*PluginProcess {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugins := make([]*PluginProcess, 0, len(m.plugins))
	for _, p := range m.plugins {
		plugins = append(plugins, p)
	}
	return plugins
}

// LoadAllPlugins 加载所有插件
func (m *Manager) LoadAllPlugins() ([]*types.Plugin, error) {
	entries, err := os.ReadDir(m.pluginDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin directory: %w", err)
	}

	var plugins []*types.Plugin
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pluginPath := filepath.Join(m.pluginDir, entry.Name())
		plugin, err := m.LoadPlugin(pluginPath)
		if err != nil {
			log.Printf("Failed to load plugin %s: %v", entry.Name(), err)
			continue
		}

		plugins = append(plugins, plugin)
	}

	return plugins, nil
}

// ExecutePlugin 直接执行插件（无端口模式，支持并发）
func (m *Manager) ExecutePlugin(plugin *types.Plugin, pluginPath string, messageJSON []byte) ([]byte, error) {
	var cmd *exec.Cmd

	// 根据运行时选择解释器
	switch plugin.Runtime {
	case "python":
		// 获取Python路径
		pythonPath := m.depsManager.GetPythonPath()
		absPythonPath, err := filepath.Abs(pythonPath)
		if err != nil {
			return nil, fmt.Errorf("无法获取Python绝对路径: %w", err)
		}

		// 构建命令
		cmd = exec.Command(absPythonPath, plugin.Entry, "--mode=direct")
		cmd.Dir = pluginPath

		// 设置环境变量（强制 Python 使用 UTF-8 编码）
		cmd.Env = append(os.Environ(),
			fmt.Sprintf("ALLBOT_PLUGIN_ID=%s", plugin.ID),
			"PYTHONUTF8=1", // 强制 Python 3.7+ 使用 UTF-8 模式
		)

	case "nodejs":
		// 构建命令
		cmd = exec.Command("node", plugin.Entry, "--mode=direct")
		cmd.Dir = pluginPath

		// 设置环境变量
		nodePath := m.depsManager.GetNodePath()
		cmd.Env = append(os.Environ(),
			fmt.Sprintf("ALLBOT_PLUGIN_ID=%s", plugin.ID),
			fmt.Sprintf("NODE_PATH=%s", nodePath),
		)

	default:
		return nil, fmt.Errorf("不支持的运行时: %s", plugin.Runtime)
	}

	// 创建管道
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("创建stdin管道失败: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("创建stdout管道失败: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("创建stderr管道失败: %w", err)
	}

	// 启动进程
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动插件进程失败: %w", err)
	}

	// 写入消息JSON到stdin
	if _, err := stdin.Write(messageJSON); err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("写入消息失败: %w", err)
	}
	stdin.Close()

	// 读取stdout和stderr
	output, err := io.ReadAll(stdout)
	if err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("读取输出失败: %w", err)
	}

	errOutput, _ := io.ReadAll(stderr)
	if len(errOutput) > 0 {
		log.Printf("[SYSTEM] Plugin %s stderr: %s", plugin.ID, string(errOutput))
	}

	// 等待进程结束
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("插件执行失败: %w, stderr: %s", err, string(errOutput))
	}

	return output, nil
}

// TogglePlugin 切换插件启用状态
func (m *Manager) TogglePlugin(pluginID string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	process, exists := m.plugins[pluginID]
	if !exists {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}

	process.Plugin.Enabled = enabled

	// 同步更新 plugin.json 文件
	configPath := filepath.Join(m.pluginDir, pluginID, "plugin.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	config["enabled"] = enabled

	newData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.WriteFile(configPath, newData, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	log.Printf("Plugin %s enabled=%v", pluginID, enabled)

	return nil
}

// ReloadPlugin 重新加载插件（热重载）
func (m *Manager) ReloadPlugin(pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	process, exists := m.plugins[pluginID]
	if !exists {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}

	// 重新加载插件配置
	pluginPath := filepath.Join(m.pluginDir, pluginID)
	newPlugin, err := m.loadPluginConfig(pluginPath)
	if err != nil {
		return fmt.Errorf("failed to reload plugin config: %w", err)
	}

	// 更新插件信息
	process.Plugin = newPlugin
	log.Printf("[SYSTEM] Plugin %s reloaded", pluginID)

	return nil
}

// loadPluginConfig 加载插件配置（内部方法）
func (m *Manager) loadPluginConfig(pluginPath string) (*types.Plugin, error) {
	// 读取 plugin.json
	configPath := filepath.Join(pluginPath, "plugin.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin.json: %w", err)
	}

	var config types.PluginConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse plugin.json: %w", err)
	}

	// 生成插件ID（使用目录名）
	pluginID := filepath.Base(pluginPath)

	plugin := &types.Plugin{
		ID:        pluginID,
		Name:      config.Name,
		Version:   config.Version,
		Runtime:   config.Runtime,
		Entry:     config.Entry,
		Platforms: config.Platforms,
		Trigger:   config.Trigger,
		Enabled:   config.Enabled,
	}

	return plugin, nil
}

// GetDepsManager 获取依赖管理器
func (m *Manager) GetDepsManager() *deps.Manager {
	return m.depsManager
}
