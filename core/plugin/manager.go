package plugin

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/allbot/allbot/core/types"
)

// Manager 插件管理器
type Manager struct {
	plugins   map[string]*PluginProcess
	mu        sync.RWMutex
	pluginDir string
}

// PluginProcess 插件进程
type PluginProcess struct {
	Plugin  *types.Plugin
	Cmd     *exec.Cmd
	Port    int    // gRPC 端口
	Status  string // running/stopped/error
}

// NewManager 创建插件管理器
func NewManager(pluginDir string) *Manager {
	return &Manager{
		plugins:   make(map[string]*PluginProcess),
		pluginDir: pluginDir,
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

	plugin := &types.Plugin{
		ID:        pluginID,
		Name:      config.Name,
		Version:   config.Version,
		Runtime:   config.Runtime,
		Entry:     config.Entry,
		Platforms: config.Platforms,
		Trigger:   config.Trigger,
	}

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
		cmd = exec.Command("python", entryPath, fmt.Sprintf("--port=%d", port))
	case "nodejs":
		cmd = exec.Command("node", entryPath, fmt.Sprintf("--port=%d", port))
	default:
		return fmt.Errorf("unsupported runtime: %s", plugin.Runtime)
	}

	// 设置环境变量
	cmd.Env = append(os.Environ(),
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

	delete(m.plugins, pluginID)
	log.Printf("Plugin stopped: %s", pluginID)

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
