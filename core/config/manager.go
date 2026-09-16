package config

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/allbot/allbot/core/adapter"
	"github.com/allbot/allbot/core/adapter/_registry"
	"github.com/allbot/allbot/core/types"
)

type AdapterManager struct {
	db             *Database
	adapters       map[int64]adapter.Adapter
	lifecycleLocks map[int64]*sync.Mutex
	messageHandler func(*types.Message)
	mu             sync.RWMutex
	healthStop     chan struct{}
	healthStopOnce sync.Once
}

const adapterHealthCheckInterval = 30 * time.Second

func NewAdapterManager(db *Database) *AdapterManager {
	return &AdapterManager{
		db:             db,
		adapters:       make(map[int64]adapter.Adapter),
		lifecycleLocks: make(map[int64]*sync.Mutex),
		healthStop:     make(chan struct{}),
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

func (m *AdapterManager) StartHealthMonitor() {
	go m.healthMonitorLoop()
}

func (m *AdapterManager) StopHealthMonitor() {
	m.healthStopOnce.Do(func() {
		close(m.healthStop)
	})
}

func (m *AdapterManager) healthMonitorLoop() {
	ticker := time.NewTicker(adapterHealthCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-m.healthStop:
			return
		case <-ticker.C:
			m.RestartUnhealthyAdapters()
		}
	}
}

func (m *AdapterManager) RestartUnhealthyAdapters() {
	if m == nil || m.db == nil {
		return
	}
	configs, err := m.db.GetAllAdapters()
	if err != nil {
		log.Printf("警告：检查适配器健康状态失败: %v", err)
		return
	}
	for _, config := range configs {
		if config == nil || !config.Enabled || m.IsAdapterRunning(config.ID) {
			continue
		}
		log.Printf("警告：适配器未运行，准备自动重启: %s#%d", config.Platform, config.ID)
		if err := m.ReloadAdapterByID(config.ID); err != nil {
			log.Printf("警告：自动重启适配器失败 %s#%d: %v", config.Platform, config.ID, err)
		}
	}
}

func (m *AdapterManager) adapterLifecycleLock(id int64) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	lock := m.lifecycleLocks[id]
	if lock == nil {
		lock = &sync.Mutex{}
		m.lifecycleLocks[id] = lock
	}
	return lock
}

func (m *AdapterManager) startAdapter(config *AdapterConfig) error {
	if config.ID == 0 {
		return fmt.Errorf("适配器 ID 不能为空")
	}
	lifecycleLock := m.adapterLifecycleLock(config.ID)
	lifecycleLock.Lock()
	defer lifecycleLock.Unlock()
	return m.startAdapterLocked(config)
}

func (m *AdapterManager) startAdapterLocked(config *AdapterConfig) error {
	m.mu.Lock()
	existing := m.adapters[config.ID]
	delete(m.adapters, config.ID)
	m.mu.Unlock()
	if existing != nil {
		if err := existing.Stop(); err != nil {
			m.mu.Lock()
			m.adapters[config.ID] = existing
			m.mu.Unlock()
			return fmt.Errorf("停止旧适配器失败: %w", err)
		}
	}

	desc, ok := registry.Get(config.Platform)
	if !ok {
		return fmt.Errorf("不支持的平台: %s", config.Platform)
	}
	parsedConfig, err := desc.ParseConfig(config.Config)
	if err != nil {
		return fmt.Errorf("解析 %s 配置失败: %w", desc.DisplayName, err)
	}
	adp, err := desc.NewAdapter(parsedConfig)
	if err != nil {
		return fmt.Errorf("创建 %s 适配器失败: %w", desc.DisplayName, err)
	}
	if databaseAware, ok := adp.(interface{ SetDatabase(*Database) }); ok {
		databaseAware.SetDatabase(m.db)
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

	m.mu.Lock()
	m.adapters[config.ID] = adp
	m.mu.Unlock()
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
	lifecycleLock := m.adapterLifecycleLock(id)
	lifecycleLock.Lock()
	defer lifecycleLock.Unlock()
	return m.stopAdapterLocked(id)
}

func (m *AdapterManager) stopAdapterLocked(id int64) error {
	m.mu.Lock()
	adp, ok := m.adapters[id]
	if ok {
		delete(m.adapters, id)
	}
	m.mu.Unlock()
	if !ok {
		return nil
	}
	if err := adp.Stop(); err != nil {
		m.mu.Lock()
		m.adapters[id] = adp
		m.mu.Unlock()
		return fmt.Errorf("停止适配器失败: %w", err)
	}
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
	lifecycleLock := m.adapterLifecycleLock(id)
	lifecycleLock.Lock()
	defer lifecycleLock.Unlock()

	config, err := m.db.GetAdapterByID(id)
	if err != nil {
		return fmt.Errorf("获取配置失败: %w", err)
	}
	if config == nil {
		return fmt.Errorf("配置不存在: %d", id)
	}

	if err := m.stopAdapterLocked(id); err != nil {
		return fmt.Errorf("停止旧适配器失败: %w", err)
	}

	if config.Enabled {
		return m.startAdapterLocked(config)
	}

	return nil
}

func (m *AdapterManager) GetAdapter(platform string) adapter.Adapter {
	platform = strings.TrimSpace(platform)
	if platform == "" {
		return nil
	}
	if m.db != nil {
		configs, err := m.db.GetAllAdapters()
		if err == nil {
			for _, config := range configs {
				if config == nil || !config.Enabled || config.Platform != platform {
					continue
				}
				if adp := m.GetAdapterByID(config.ID); adp != nil {
					return adp
				}
			}
			return nil
		}
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]int64, 0, len(m.adapters))
	for id := range m.adapters {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		adp := m.adapters[id]
		if adp != nil && adp.GetPlatform() == platform {
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

func (m *AdapterManager) IsAdapterRunning(id int64) bool {
	adp := m.GetAdapterByID(id)
	if adp == nil {
		return false
	}
	if healthChecker, ok := adp.(adapter.HealthChecker); ok {
		return healthChecker.IsHealthy()
	}
	return true
}

func (m *AdapterManager) RunningAdapterCount() int {
	m.mu.RLock()
	ids := make([]int64, 0, len(m.adapters))
	for id := range m.adapters {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	running := 0
	for _, id := range ids {
		if m.IsAdapterRunning(id) {
			running++
		}
	}
	return running
}

func (m *AdapterManager) GetAdapterForMessage(msg *types.Message) adapter.Adapter {
	if msg == nil {
		return nil
	}
	adapterIDText := strings.TrimSpace(msg.AdapterID)
	if adapterIDText == "" && msg.Metadata != nil {
		adapterIDText = strings.TrimSpace(msg.Metadata["adapter_id"])
	}
	if adapterIDText != "" {
		adapterID, err := strconv.ParseInt(adapterIDText, 10, 64)
		if err != nil || adapterID <= 0 {
			return nil
		}
		return m.GetAdapterByID(adapterID)
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
	m.StopHealthMonitor()
	m.mu.RLock()
	ids := make([]int64, 0, len(m.adapters))
	for id := range m.adapters {
		ids = append(ids, id)
	}
	m.mu.RUnlock()
	for _, id := range ids {
		if err := m.StopAdapterByID(id); err != nil {
			log.Printf("警告：停止适配器失败 #%d: %v", id, err)
		}
	}
}

func (m *AdapterManager) SetAdapterPinned(id int64, pinned bool) error {
	config, err := m.db.GetAdapterByID(id)
	if err != nil {
		return err
	}
	if config == nil {
		return fmt.Errorf("适配器不存在: %d", id)
	}
	config.Pinned = pinned
	return m.db.SaveAdapter(config)
}

func (m *AdapterManager) SaveAdapterConfig(id int64, platform, remark, description string, enabled bool, configData interface{}) error {
	platform = strings.TrimSpace(platform)
	if platform == WebChatPlatform {
		existing, err := m.db.GetAdapter(WebChatPlatform)
		if err != nil {
			return fmt.Errorf("检查 Web 聊天室实例失败: %w", err)
		}
		if existing != nil && existing.ID != id {
			return fmt.Errorf("Web 聊天室只允许创建一个实例")
		}
	}

	mergedConfigData, err := m.mergeExistingSensitiveConfig(id, platform, configData)
	if err != nil {
		return err
	}

	existingPinned := false
	if id > 0 {
		if existing, err := m.db.GetAdapterByID(id); err != nil {
			return fmt.Errorf("获取原配置失败: %w", err)
		} else if existing != nil {
			existingPinned = existing.Pinned
		}
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
		Pinned:      existingPinned,
		Config:      string(configJSON),
	}

	if err := m.db.SaveAdapter(config); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	if err := m.ReloadAdapterByID(config.ID); err != nil {
		// 启动失败不等于用户禁用，保留数据库中的启用配置，便于后续重试。
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
