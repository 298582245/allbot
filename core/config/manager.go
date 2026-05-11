package config

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/allbot/allbot/core/adapter"
	"github.com/allbot/allbot/core/types"
)

// AdapterManager 适配器管理器
type AdapterManager struct {
	db              *Database
	adapters        map[string]adapter.Adapter // platform -> adapter
	messageHandler  func(*types.Message)
	mu              sync.RWMutex
}

// NewAdapterManager 创建适配器管理器
func NewAdapterManager(db *Database) *AdapterManager {
	return &AdapterManager{
		db:       db,
		adapters: make(map[string]adapter.Adapter),
	}
}

// GetDatabase 获取数据库实例
func (m *AdapterManager) GetDatabase() *Database {
	return m.db
}

// SetMessageHandler 设置消息处理器
func (m *AdapterManager) SetMessageHandler(handler func(*types.Message)) {
	m.messageHandler = handler
}

// LoadAndStartAdapters 加载并启动所有启用的适配器
func (m *AdapterManager) LoadAndStartAdapters() error {
	configs, err := m.db.GetAllAdapters()
	if err != nil {
		return fmt.Errorf("加载适配器配置失败: %w", err)
	}

	for _, config := range configs {
		if config.Enabled {
			if err := m.startAdapter(config); err != nil {
				log.Printf("警告：启动适配器失败 %s: %v", config.Platform, err)
			}
		}
	}

	return nil
}

// startAdapter 启动适配器
func (m *AdapterManager) startAdapter(config *AdapterConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 如果已存在，先停止
	if existing, ok := m.adapters[config.Platform]; ok {
		existing.Stop()
		delete(m.adapters, config.Platform)
	}

	// 创建适配器
	var adp adapter.Adapter
	var err error

	switch config.Platform {
	case "qq":
		qqConfig, err := ParseQQConfig(config.Config)
		if err != nil {
			return fmt.Errorf("解析 QQ 配置失败: %w", err)
		}
		adp = adapter.NewQQAdapter(qqConfig.APIURL, qqConfig.ListenAddr)

	case "wechat":
		// TODO: 实现微信适配器
		return fmt.Errorf("微信适配器尚未实现")

	case "telegram":
		// TODO: 实现 Telegram 适配器
		return fmt.Errorf("Telegram 适配器尚未实现")

	default:
		return fmt.Errorf("不支持的平台: %s", config.Platform)
	}

	// 设置消息处理器
	if m.messageHandler != nil {
		adp.SetMessageHandler(m.messageHandler)
	}

	// 启动适配器
	if err = adp.Start(); err != nil {
		return fmt.Errorf("启动适配器失败: %w", err)
	}

	m.adapters[config.Platform] = adp
	log.Printf("适配器已启动: %s", config.Platform)

	return nil
}

// StopAdapter 停止适配器
func (m *AdapterManager) StopAdapter(platform string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	adp, ok := m.adapters[platform]
	if !ok {
		return nil // 已经停止
	}

	if err := adp.Stop(); err != nil {
		return fmt.Errorf("停止适配器失败: %w", err)
	}

	delete(m.adapters, platform)
	log.Printf("适配器已停止: %s", platform)

	return nil
}

// ReloadAdapter 重新加载适配器
func (m *AdapterManager) ReloadAdapter(platform string) error {
	// 获取配置
	config, err := m.db.GetAdapter(platform)
	if err != nil {
		return fmt.Errorf("获取配置失败: %w", err)
	}

	if config == nil {
		return fmt.Errorf("配置不存在: %s", platform)
	}

	// 停止旧适配器
	if err := m.StopAdapter(platform); err != nil {
		log.Printf("警告：停止旧适配器失败: %v", err)
	}

	// 如果启用，启动新适配器
	if config.Enabled {
		return m.startAdapter(config)
	}

	return nil
}

// GetAdapter 获取适配器
func (m *AdapterManager) GetAdapter(platform string) adapter.Adapter {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.adapters[platform]
}

// GetAllAdapters 获取所有适配器
func (m *AdapterManager) GetAllAdapters() map[string]adapter.Adapter {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]adapter.Adapter)
	for k, v := range m.adapters {
		result[k] = v
	}
	return result
}

// StopAll 停止所有适配器
func (m *AdapterManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for platform, adp := range m.adapters {
		if err := adp.Stop(); err != nil {
			log.Printf("警告：停止适配器失败 %s: %v", platform, err)
		}
	}

	m.adapters = make(map[string]adapter.Adapter)
}

// SaveAdapterConfig 保存适配器配置并重新加载
func (m *AdapterManager) SaveAdapterConfig(platform string, enabled bool, configData interface{}) error {
	// 序列化配置
	configJSON, err := json.Marshal(configData)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	// 保存到数据库
	config := &AdapterConfig{
		Platform: platform,
		Enabled:  enabled,
		Config:   string(configJSON),
	}

	if err := m.db.SaveAdapter(config); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	// 重新加载适配器
	return m.ReloadAdapter(platform)
}
