package config

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/allbot/allbot/core/adapter"
	"github.com/allbot/allbot/core/types"
)

type AdapterManager struct {
	db             *Database
	adapters       map[int64]adapter.Adapter
	messageHandler func(*types.Message)
	mu             sync.RWMutex
}

func NewAdapterManager(db *Database) *AdapterManager {
	return &AdapterManager{
		db:       db,
		adapters: make(map[int64]adapter.Adapter),
	}
}

func (m *AdapterManager) GetDatabase() *Database {
	return m.db
}

func (m *AdapterManager) SetMessageHandler(handler func(*types.Message)) {
	m.messageHandler = handler
}

func (m *AdapterManager) LoadAndStartAdapters() error {
	configs, err := m.db.GetAllAdapters()
	if err != nil {
		return fmt.Errorf("加载适配器配置失败: %w", err)
	}

	for _, config := range configs {
		if config.Enabled {
			if err := m.startAdapter(config); err != nil {
				log.Printf("警告：启动适配器失败 %s#%d: %v", config.Platform, config.ID, err)
			}
		}
	}

	return nil
}

func (m *AdapterManager) startAdapter(config *AdapterConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.ID == 0 {
		return fmt.Errorf("适配器 ID 不能为空")
	}

	if existing, ok := m.adapters[config.ID]; ok {
		existing.Stop()
		delete(m.adapters, config.ID)
	}

	var adp adapter.Adapter
	var err error

	switch config.Platform {
	case "qq":
		qqConfig, err := ParseQQConfig(config.Config)
		if err != nil {
			return fmt.Errorf("解析 QQ 配置失败: %w", err)
		}
		adp = adapter.NewQQAdapter(qqConfig.ServerURL, qqConfig.AccessToken)

	case "qq_office":
		qqOfficeConfig, err := ParseQQOfficeConfig(config.Config)
		if err != nil {
			return fmt.Errorf("解析 QQ 官方机器人配置失败: %w", err)
		}
		adp = adapter.NewQQOfficeAdapter(
			qqOfficeConfig.AppID,
			qqOfficeConfig.ClientSecret,
			qqOfficeConfig.APIBaseURL,
			qqOfficeConfig.TokenURL,
		)

	case "wechat":
		return fmt.Errorf("微信适配器尚未实现")

	case "telegram":
		telegramConfig, err := ParseTelegramConfig(config.Config)
		if err != nil {
			return fmt.Errorf("解析 Telegram 配置失败: %w", err)
		}
		adp = adapter.NewTelegramAdapter(telegramConfig.BotToken, telegramConfig.ProxyURL)

	default:
		return fmt.Errorf("不支持的平台: %s", config.Platform)
	}

	if m.messageHandler != nil {
		adapterID := config.ID
		platform := config.Platform
		remark := strings.TrimSpace(config.Remark)
		description := strings.TrimSpace(config.Description)
		adp.SetMessageHandler(func(msg *types.Message) {
			if msg.Metadata == nil {
				msg.Metadata = make(map[string]string)
			}
			adapterIDText := strconv.FormatInt(adapterID, 10)
			msg.AdapterID = adapterIDText
			msg.Metadata["adapter_id"] = adapterIDText
			msg.Metadata["adapter_platform"] = platform
			msg.Metadata["adapter_remark"] = remark
			msg.Metadata["adapter_description"] = description
			m.messageHandler(msg)
		})
	}

	if err = adp.Start(); err != nil {
		return fmt.Errorf("启动适配器失败: %w", err)
	}

	m.adapters[config.ID] = adp
	log.Printf("适配器已启动: %s#%d", config.Platform, config.ID)

	return nil
}

func (m *AdapterManager) StopAdapter(platform string) error {
	adapters, err := m.db.GetAllAdapters()
	if err != nil {
		return err
	}

	for _, adapterConfig := range adapters {
		if adapterConfig.Platform == platform {
			if err := m.StopAdapterByID(adapterConfig.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *AdapterManager) StopAdapterByID(id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	adp, ok := m.adapters[id]
	if !ok {
		return nil
	}

	if err := adp.Stop(); err != nil {
		return fmt.Errorf("停止适配器失败: %w", err)
	}

	delete(m.adapters, id)
	log.Printf("适配器已停止: #%d", id)

	return nil
}

func (m *AdapterManager) ReloadAdapter(platform string) error {
	adapters, err := m.db.GetAllAdapters()
	if err != nil {
		return err
	}

	for _, adapterConfig := range adapters {
		if adapterConfig.Platform == platform {
			if err := m.ReloadAdapterByID(adapterConfig.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *AdapterManager) ReloadAdapterByID(id int64) error {
	config, err := m.db.GetAdapterByID(id)
	if err != nil {
		return fmt.Errorf("获取配置失败: %w", err)
	}
	if config == nil {
		return fmt.Errorf("配置不存在: %d", id)
	}

	if err := m.StopAdapterByID(id); err != nil {
		log.Printf("警告：停止旧适配器失败: %v", err)
	}

	if config.Enabled {
		return m.startAdapter(config)
	}

	return nil
}

func (m *AdapterManager) GetAdapter(platform string) adapter.Adapter {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for id, adp := range m.adapters {
		config, err := m.db.GetAdapterByID(id)
		if err == nil && config != nil && config.Platform == platform {
			return adp
		}
	}
	return nil
}

func (m *AdapterManager) GetAdapterByID(id int64) adapter.Adapter {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.adapters[id]
}

func (m *AdapterManager) GetAdapterForMessage(msg *types.Message) adapter.Adapter {
	if msg != nil && msg.Metadata != nil {
		if adapterIDText := msg.Metadata["adapter_id"]; adapterIDText != "" {
			if adapterID, err := strconv.ParseInt(adapterIDText, 10, 64); err == nil {
				if adp := m.GetAdapterByID(adapterID); adp != nil {
					return adp
				}
			}
		}
	}

	if msg == nil {
		return nil
	}
	return m.GetAdapter(msg.Platform)
}

func (m *AdapterManager) GetAllAdapters() map[int64]adapter.Adapter {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[int64]adapter.Adapter)
	for key, value := range m.adapters {
		result[key] = value
	}
	return result
}

func (m *AdapterManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, adp := range m.adapters {
		if err := adp.Stop(); err != nil {
			log.Printf("警告：停止适配器失败 #%d: %v", id, err)
		}
	}

	m.adapters = make(map[int64]adapter.Adapter)
}

func (m *AdapterManager) SaveAdapterConfig(id int64, platform, remark, description string, enabled bool, configData interface{}) error {
	mergedConfigData, err := m.mergeExistingSensitiveConfig(id, platform, configData)
	if err != nil {
		return err
	}

	configJSON, err := json.Marshal(mergedConfigData)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	config := &AdapterConfig{
		ID:          id,
		Platform:    platform,
		Remark:      remark,
		Description: description,
		Enabled:     enabled,
		Config:      string(configJSON),
	}

	if err := m.db.SaveAdapter(config); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	if err := m.ReloadAdapterByID(config.ID); err != nil {
		if enabled {
			config.Enabled = false
			if saveErr := m.db.SaveAdapter(config); saveErr != nil {
				return fmt.Errorf("启动适配器失败: %w；回写停止状态失败: %v", err, saveErr)
			}
		}
		return err
	}

	return nil
}

func (m *AdapterManager) mergeExistingSensitiveConfig(id int64, platform string, configData interface{}) (interface{}, error) {
	newConfig, ok := configData.(map[string]interface{})
	if !ok {
		return configData, nil
	}

	var existingAdapter *AdapterConfig
	var err error
	if id > 0 {
		existingAdapter, err = m.db.GetAdapterByID(id)
	} else {
		existingAdapter, err = m.db.GetAdapter(platform)
	}
	if err != nil {
		return nil, fmt.Errorf("获取原配置失败: %w", err)
	}
	if existingAdapter == nil || existingAdapter.Config == "" {
		return configData, nil
	}

	var existingConfig map[string]interface{}
	if err := json.Unmarshal([]byte(existingAdapter.Config), &existingConfig); err != nil {
		return configData, nil
	}

	for key, value := range newConfig {
		text, ok := value.(string)
		if !ok || !isSensitiveConfigKey(key) || !isMaskedConfigValue(text) {
			continue
		}

		if existingValue, exists := existingConfig[key]; exists {
			if existingText, ok := existingValue.(string); ok && existingText != "" && !isMaskedConfigValue(existingText) {
				newConfig[key] = existingText
			}
		}
	}

	return newConfig, nil
}

func isSensitiveConfigKey(key string) bool {
	keyLower := strings.ToLower(key)
	sensitiveFields := []string{
		"token", "bot_token", "access_token", "refresh_token",
		"secret", "app_secret", "client_secret",
		"password", "passwd", "pwd",
		"key", "api_key", "private_key",
	}

	for _, field := range sensitiveFields {
		if strings.Contains(keyLower, field) {
			return true
		}
	}
	return false
}

func isMaskedConfigValue(value string) bool {
	return strings.HasPrefix(value, "****")
}
