package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/allbot/allbot/core/adapter"
	"github.com/allbot/allbot/core/adapter/_registry"
	customadapter "github.com/allbot/allbot/core/adapter/custom"
	"github.com/allbot/allbot/core/types"
	"github.com/allbot/allbot/core/utils"
)

type AdapterManager struct {
	db               *Database
	customAdapterDir string
	adapters         map[int64]adapter.Adapter
	lifecycleLocks   map[int64]*sync.Mutex
	restartAttempts  map[int64]int
	restartEligible  map[int64]bool
	messageHandler   func(*types.Message)
	mu               sync.RWMutex
	healthStop       chan struct{}
	healthStopOnce   sync.Once
}

func (m *AdapterManager) SetCustomAdapterDir(dir string) error {
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("自定义适配器目录不能为空")
	}
	m.mu.Lock()
	m.customAdapterDir = dir
	m.mu.Unlock()
	return customadapter.Load(dir)
}

const adapterPlatformPinsKey = "adapter.platform_pins"

func (m *AdapterManager) CustomAdapterDir() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.customAdapterDir
}

func (m *AdapterManager) SetAdapterPlatformPinned(platform string, pinned bool) error {
	raw, err := m.db.GetSetting(adapterPlatformPinsKey)
	pins := map[string]bool{}
	if err == nil {
		_ = json.Unmarshal([]byte(raw), &pins)
	}
	if pinned {
		pins[platform] = true
	} else {
		delete(pins, platform)
	}
	data, err := json.Marshal(pins)
	if err != nil {
		return err
	}
	return m.db.SetSetting(adapterPlatformPinsKey, string(data), "适配器平台置顶")
}

func (m *AdapterManager) AdapterPlatformPins() map[string]bool {
	pins := map[string]bool{}
	raw, err := m.db.GetSetting(adapterPlatformPinsKey)
	if err == nil {
		_ = json.Unmarshal([]byte(raw), &pins)
	}
	return pins
}

func (m *AdapterManager) CreateCustomAdapterDefinition(raw json.RawMessage, code string) error {
	return customadapter.CreateDefinition(m.CustomAdapterDir(), raw, code)
}

func (m *AdapterManager) SaveCustomAdapterDefinition(platform string, raw json.RawMessage) error {
	if err := m.stopCustomPlatformInstances(platform); err != nil {
		return err
	}
	if err := customadapter.SaveDefinition(m.CustomAdapterDir(), platform, raw); err != nil {
		return err
	}
	return m.reloadCustomPlatformInstances(platform)
}

func (m *AdapterManager) ReloadCustomAdapterDefinition(platform string) error {
	if err := customadapter.LoadOne(m.CustomAdapterDir(), platform, true); err != nil {
		return err
	}
	return m.reloadCustomPlatformInstances(platform)
}

func (m *AdapterManager) DeleteCustomAdapterDefinition(platform string) error {
	desc, ok := registry.Get(platform)
	if !ok || !desc.Custom {
		return fmt.Errorf("Go 内置适配器不允许删除")
	}
	if err := m.stopCustomPlatformInstances(platform); err != nil {
		return err
	}
	items, err := m.db.GetAllAdapters()
	if err != nil {
		return err
	}
	for _, item := range items {
		if item != nil && item.Platform == platform {
			if err := adapter.DeleteCustomAdapterData(filepath.Join(m.CustomAdapterDir(), platform), strconv.FormatInt(item.ID, 10)); err != nil {
				return err
			}
			if err := m.db.DeleteAdapterByID(item.ID); err != nil {
				return err
			}
		}
	}
	if err := customadapter.DeleteDefinition(m.CustomAdapterDir(), platform); err != nil {
		return err
	}
	return m.SetAdapterPlatformPinned(platform, false)
}

func (m *AdapterManager) stopCustomPlatformInstances(platform string) error {
	items, err := m.db.GetAllAdapters()
	if err != nil {
		return err
	}
	for _, item := range items {
		if item != nil && item.Platform == platform {
			if err := m.StopAdapterByID(item.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *AdapterManager) reloadCustomPlatformInstances(platform string) error {
	items, err := m.db.GetAllAdapters()
	if err != nil {
		return err
	}
	var failures []string
	for _, item := range items {
		if item != nil && item.Platform == platform && item.Enabled {
			if err := m.ReloadAdapterByID(item.ID); err != nil {
				failures = append(failures, fmt.Sprintf("#%d: %v", item.ID, err))
			}
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("部分实例重载失败: %s", strings.Join(failures, "; "))
	}
	return nil
}

const adapterHealthCheckInterval = 30 * time.Second
const adapterAutoRestartMaxAttempts = 20

func NewAdapterManager(db *Database) *AdapterManager {
	return &AdapterManager{
		db:              db,
		adapters:        make(map[int64]adapter.Adapter),
		lifecycleLocks:  make(map[int64]*sync.Mutex),
		restartAttempts: make(map[int64]int),
		restartEligible: make(map[int64]bool),
		healthStop:      make(chan struct{}),
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
				log.Printf("警告：启动适配器失败 %s#%d: %s", config.Platform, config.ID, utils.MaskSensitiveError(err))
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
		if config == nil || !config.Enabled {
			continue
		}
		// 首次启动失败时没有自动重启资格。只有已经成功启动过、
		// 后续变得不健康的实例才允许由健康监控自动重启。
		if !m.isRestartEligible(config.ID) {
			continue
		}
		if m.IsAdapterRunning(config.ID) {
			m.resetRestartAttempts(config.ID)
			continue
		}
		attempt := m.nextRestartAttempt(config.ID)
		if attempt > adapterAutoRestartMaxAttempts {
			if attempt == adapterAutoRestartMaxAttempts+1 {
				log.Printf("警告：适配器自动重启已达到上限（%d 次），停止继续重试: %s#%d", adapterAutoRestartMaxAttempts, config.Platform, config.ID)
			}
			continue
		}
		log.Printf("警告：适配器未运行，准备自动重启（第 %d/%d 次）: %s#%d", attempt, adapterAutoRestartMaxAttempts, config.Platform, config.ID)
		if err := m.ReloadAdapterByID(config.ID); err != nil {
			log.Printf("警告：自动重启适配器失败 %s#%d: %s", config.Platform, config.ID, utils.MaskSensitiveError(err))
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
	if desc.Custom {
		m.mu.RLock()
		adapterDir := m.customAdapterDir
		m.mu.RUnlock()
		if adapterDir == "" {
			return fmt.Errorf("自定义适配器目录未配置")
		}
		stored, loadErr := adapter.LoadCustomAdapterData(filepath.Join(adapterDir, config.Platform))
		if loadErr != nil {
			return loadErr
		}
		raw, ok := stored[strconv.FormatInt(config.ID, 10)]
		if !ok {
			raw = json.RawMessage(config.Config)
		}
		values := map[string]interface{}{}
		if json.Unmarshal(raw, &values) != nil {
			return fmt.Errorf("解析自定义适配器配置失败")
		}
		if _, exists := values[customadapter.CallbackTokenField]; !exists {
			token, tokenErr := customadapter.NewCallbackToken()
			if tokenErr != nil {
				return fmt.Errorf("生成回调令牌失败: %w", tokenErr)
			}
			values[customadapter.CallbackTokenField] = token
			if saveErr := adapter.SaveCustomAdapterData(filepath.Join(adapterDir, config.Platform), strconv.FormatInt(config.ID, 10), values); saveErr != nil {
				return fmt.Errorf("保存回调令牌失败: %w", saveErr)
			}
		}
		parsedConfig, err = desc.ParseConfig(string(raw))
		if err == nil {
			encoded, marshalErr := json.Marshal(values)
			if marshalErr != nil {
				return marshalErr
			}
			parsedConfig, err = desc.ParseConfig(string(encoded))
		}
	}
	if err != nil {
		return fmt.Errorf("解析 %s 配置失败: %w", desc.DisplayName, err)
	}
	adp, err := desc.NewAdapter(parsedConfig)
	if err != nil {
		return fmt.Errorf("创建 %s 适配器失败: %w", desc.DisplayName, err)
	}
	if remarkAware, ok := adp.(interface{ SetRemark(string) }); ok {
		remarkAware.SetRemark(config.Remark)
	}
	if instanceAware, ok := adp.(interface{ SetInstanceID(int64) }); ok {
		instanceAware.SetInstanceID(config.ID)
	}
	if callbackBaseAware, ok := adp.(interface{ SetCallbackBaseURL(string) }); ok {
		if settings, settingsErr := m.db.GetSystemSettings(); settingsErr == nil {
			callbackBaseAware.SetCallbackBaseURL(settings.PublicBaseURL)
		}
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
			if msg == nil {
				return
			}
			msg.Platform = platform
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
	m.restartEligible[config.ID] = true
	m.mu.Unlock()
	log.Printf("适配器已启动: %s#%d", config.Platform, config.ID)

	return nil
}

func (m *AdapterManager) nextRestartAttempt(id int64) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.restartAttempts[id]++
	return m.restartAttempts[id]
}

func (m *AdapterManager) resetRestartAttempts(id int64) {
	m.mu.Lock()
	delete(m.restartAttempts, id)
	m.mu.Unlock()
}

func (m *AdapterManager) isRestartEligible(id int64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.restartEligible[id]
}

func (m *AdapterManager) clearRestartState(id int64) {
	m.mu.Lock()
	delete(m.restartAttempts, id)
	delete(m.restartEligible, id)
	m.mu.Unlock()
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
	if err := m.stopAdapterLocked(id); err != nil {
		return err
	}
	m.clearRestartState(id)
	return nil
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

	m.clearRestartState(id)
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
			log.Printf("警告：停止适配器失败 #%d: %s", id, utils.MaskSensitiveError(err))
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

func (m *AdapterManager) DeleteCustomAdapterData(id int64) error {
	item, err := m.db.GetAdapterByID(id)
	if err != nil || item == nil {
		return err
	}
	desc, ok := registry.Get(item.Platform)
	if !ok || !desc.Custom {
		return nil
	}
	m.mu.RLock()
	adapterDir := m.customAdapterDir
	m.mu.RUnlock()
	if adapterDir == "" {
		return fmt.Errorf("自定义适配器目录未配置")
	}
	return adapter.DeleteCustomAdapterData(filepath.Join(adapterDir, item.Platform), strconv.FormatInt(id, 10))
}

func (m *AdapterManager) AdapterConfigData(id int64) (string, error) {
	item, err := m.db.GetAdapterByID(id)
	if err != nil || item == nil {
		return "", err
	}
	desc, ok := registry.Get(item.Platform)
	if !ok || !desc.Custom {
		return item.Config, nil
	}
	m.mu.RLock()
	adapterDir := m.customAdapterDir
	m.mu.RUnlock()
	if adapterDir == "" {
		return "", fmt.Errorf("自定义适配器目录未配置")
	}
	stored, err := adapter.LoadCustomAdapterData(filepath.Join(adapterDir, item.Platform))
	if err != nil {
		return "", err
	}
	data, ok := stored[strconv.FormatInt(id, 10)]
	if !ok {
		return "{}", nil
	}
	return string(data), nil
}

func (m *AdapterManager) AdapterCallbackToken(id int64) (string, error) {
	raw, err := m.AdapterConfigData(id)
	if err != nil {
		return "", err
	}
	var values map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return "", err
	}
	token, _ := values[customadapter.CallbackTokenField].(string)
	return token, nil
}

func (m *AdapterManager) GetCustomAdapterCode(platform string) (string, string, error) {
	desc, ok := registry.Get(platform)
	if !ok || !desc.Custom {
		return "", "", fmt.Errorf("仅支持查看自定义适配器代码")
	}
	m.mu.RLock()
	root := m.customAdapterDir
	m.mu.RUnlock()
	entry := filepath.Join(root, platform, desc.Entry)
	code, err := os.ReadFile(entry)
	if err != nil {
		return "", "", err
	}
	return desc.Entry, string(code), nil
}

func (m *AdapterManager) SaveCustomAdapterCode(platform, code string) error {
	desc, ok := registry.Get(platform)
	if !ok || !desc.Custom {
		return fmt.Errorf("仅支持修改自定义适配器代码")
	}
	m.mu.RLock()
	root := m.customAdapterDir
	m.mu.RUnlock()
	entry := filepath.Join(root, platform, desc.Entry)
	if err := os.WriteFile(entry, []byte(code), 0644); err != nil {
		return err
	}
	items, err := m.db.GetAllAdapters()
	if err != nil {
		return err
	}
	var failures []string
	for _, item := range items {
		if item != nil && item.Platform == platform && item.Enabled {
			if err := m.ReloadAdapterByID(item.ID); err != nil {
				failures = append(failures, fmt.Sprintf("#%d: %v", item.ID, err))
			}
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("代码已保存，部分实例重载失败: %s", strings.Join(failures, "; "))
	}
	return nil
}

func (m *AdapterManager) MaskAdapterConfigData(id int64, raw string) string {
	item, err := m.db.GetAdapterByID(id)
	if err != nil || item == nil {
		return raw
	}
	desc, ok := registry.Get(item.Platform)
	if !ok || !desc.Custom {
		return utils.MaskSensitiveConfig(raw)
	}
	var values map[string]interface{}
	if json.Unmarshal([]byte(raw), &values) != nil {
		return "{}"
	}
	for _, field := range desc.ConfigSchema {
		if field.Type == "password" {
			if value, exists := values[field.Key]; exists && value != nil && fmt.Sprint(value) != "" {
				values[field.Key] = "****"
			}
		}
	}
	delete(values, customadapter.CallbackTokenField)
	encoded, err := json.Marshal(values)
	if err != nil {
		return "{}"
	}
	return utils.MaskSensitiveConfig(string(encoded))
}

func (m *AdapterManager) SaveAdapterConfig(id int64, platform, remark, description string, enabled bool, configData interface{}) error {
	platform = strings.TrimSpace(platform)
	if desc, ok := registry.Get(platform); ok && desc.Custom {
		values, valueOK := configData.(map[string]interface{})
		if !valueOK {
			return fmt.Errorf("自定义适配器配置必须为对象")
		}
		for _, field := range desc.ConfigSchema {
			if !field.Required {
				continue
			}
			value, exists := values[field.Key]
			if !exists || value == nil || (fmt.Sprint(value) == "") {
				return fmt.Errorf("配置项 %s 不能为空", field.Label)
			}
		}
	}
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
	if desc, ok := registry.Get(platform); ok && desc.Custom {
		values := mergedConfigData.(map[string]interface{})
		if token, _ := values[customadapter.CallbackTokenField].(string); token == "" {
			if id > 0 {
				token, _ = m.AdapterCallbackToken(id)
			}
			if token == "" {
				token, err = customadapter.NewCallbackToken()
				if err != nil {
					return fmt.Errorf("生成回调令牌失败: %w", err)
				}
			}
			values[customadapter.CallbackTokenField] = token
		}
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
	if desc, ok := registry.Get(platform); ok && desc.Custom {
		config.Config = "{}"
	}

	if err := m.db.SaveAdapter(config); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}
	if desc, ok := registry.Get(platform); ok && desc.Custom {
		m.mu.RLock()
		adapterDir := m.customAdapterDir
		m.mu.RUnlock()
		if adapterDir == "" {
			return fmt.Errorf("自定义适配器目录未配置")
		}
		if err := adapter.SaveCustomAdapterData(filepath.Join(adapterDir, platform), strconv.FormatInt(config.ID, 10), mergedConfigData); err != nil {
			return fmt.Errorf("写入 adapter.data 失败: %w", err)
		}
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
	if id == 0 {
		if desc, exists := registry.Get(platform); exists && desc.Custom {
			return configData, nil
		}
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
	existingJSON := existingAdapter.Config
	if desc, ok := registry.Get(platform); ok && desc.Custom {
		if saved, loadErr := m.AdapterConfigData(existingAdapter.ID); loadErr == nil && saved != "" {
			existingJSON = saved
		}
	}

	var existingConfig map[string]interface{}
	if err := json.Unmarshal([]byte(existingJSON), &existingConfig); err != nil {
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
