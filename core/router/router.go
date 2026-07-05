package router

import (
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/allbot/allbot/core/adapter"
	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/payment"
	plugincore "github.com/allbot/allbot/core/plugin"
	"github.com/allbot/allbot/core/session"
	"github.com/allbot/allbot/core/types"
	"github.com/allbot/allbot/core/utils"
)

type Router struct {
	plugins        map[string]*types.Plugin
	pluginOrder    int
	pluginManager  *plugincore.Manager
	sessionManager *session.Manager
	adapters       map[string]adapter.Adapter
	adapterGetter  func(platform string) adapter.Adapter
	messageGetter  func(msg *types.Message) adapter.Adapter
	dataViewSaver  func(config.DataViewConfig) error
	database       *config.Database
	adminChecker   func(platform, userID string) bool
	keywordReplies *KeywordReplyManager
	messageCount   uint64
	mu             sync.RWMutex
}

func NewRouter(sessionManager *session.Manager) *Router {
	return &Router{
		plugins:        make(map[string]*types.Plugin),
		sessionManager: sessionManager,
		adapters:       make(map[string]adapter.Adapter),
	}
}

func (r *Router) SetPluginManager(pm *plugincore.Manager) {
	r.mu.Lock()
	r.pluginManager = pm
	keywordReplies := r.keywordReplies
	r.mu.Unlock()
	if keywordReplies != nil {
		keywordReplies.SetPluginAdminStore(pm)
	}
}

func (r *Router) SetAdapters(adapters map[string]adapter.Adapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters = adapters
}

func (r *Router) SetAdapterGetter(getter func(platform string) adapter.Adapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapterGetter = getter
}

func (r *Router) SetMessageAdapterGetter(getter func(msg *types.Message) adapter.Adapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messageGetter = getter
}

func (r *Router) SetDataViewSaver(saver func(config.DataViewConfig) error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dataViewSaver = saver
}

func (r *Router) SetDatabase(database *config.Database) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.database = database
}

func (r *Router) SetAdminChecker(checker func(platform, userID string) bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adminChecker = checker
}

func (r *Router) startedPlatformAdmins(filterPlatform string) ([]map[string]string, error) {
	r.mu.RLock()
	database := r.database
	messageGetter := r.messageGetter
	adapterGetter := r.adapterGetter
	r.mu.RUnlock()
	if database == nil {
		return nil, fmt.Errorf("数据库不可用")
	}
	settings, err := database.GetSystemSettings()
	if err != nil {
		return nil, err
	}
	adapterItems, err := database.GetAllAdapters()
	if err != nil {
		return nil, err
	}
	filterPlatform = strings.TrimSpace(filterPlatform)
	runningAdapterByPlatform := make(map[string]string)
	for _, item := range adapterItems {
		if item == nil || !item.Enabled {
			continue
		}
		platform := strings.TrimSpace(item.Platform)
		if platform == "" || (filterPlatform != "" && platform != filterPlatform) {
			continue
		}
		adapterID := strconv.FormatInt(item.ID, 10)
		running := false
		if messageGetter != nil {
			running = messageGetter(&types.Message{Platform: platform, Metadata: map[string]string{"adapter_id": adapterID}}) != nil
		} else if adapterGetter != nil {
			running = adapterGetter(platform) != nil
		}
		if running && runningAdapterByPlatform[platform] == "" {
			runningAdapterByPlatform[platform] = adapterID
		}
	}

	result := make([]map[string]string, 0)
	seen := make(map[string]bool)
	for _, admin := range settings.PlatformAdmins {
		platform := strings.TrimSpace(admin.Platform)
		userID := strings.TrimSpace(admin.UserID)
		unionID := strings.TrimSpace(admin.UnionID)
		if unionID != "" {
			accounts, err := database.ListUserAccountsByUnionID(unionID)
			if err != nil {
				return nil, err
			}
			for _, account := range accounts {
				if account == nil || (filterPlatform != "" && account.Platform != filterPlatform) {
					continue
				}
				adapterID := runningAdapterByPlatform[account.Platform]
				if adapterID == "" {
					continue
				}
				key := account.Platform + "\x00" + account.UserID + "\x00" + adapterID
				if seen[key] {
					continue
				}
				seen[key] = true
				result = append(result, map[string]string{"platform": account.Platform, "user_id": account.UserID, "union_id": unionID, "adapter_id": adapterID})
			}
			continue
		}
		if platform == "" || userID == "" || (filterPlatform != "" && platform != filterPlatform) {
			continue
		}
		adapterID := runningAdapterByPlatform[platform]
		if adapterID == "" {
			continue
		}
		key := platform + "\x00" + userID + "\x00" + adapterID
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, map[string]string{"platform": platform, "user_id": userID, "adapter_id": adapterID})
	}
	return result, nil
}

func (r *Router) SetKeywordReplyManager(manager *KeywordReplyManager) {
	r.mu.Lock()
	r.keywordReplies = manager
	pluginManager := r.pluginManager
	r.mu.Unlock()
	if manager == nil {
		return
	}
	manager.SetPluginAdminStore(pluginManager)
	manager.SetRegisterPluginFunc(r.RegisterPlugin)
	manager.SetListenFunc(func(msg *types.Message, timeout int) string {
		if r.sessionManager == nil || msg == nil {
			return ""
		}
		ch := r.sessionManager.CreateSession("builtin:keyword-reply", msg.UserID, msg.GroupID, timeout)
		content, ok := <-ch
		if !ok {
			return ""
		}
		return strings.TrimSpace(content)
	})
	manager.SetListenUntilFunc(func(msg *types.Message, timeout int, done <-chan struct{}) string {
		if r.sessionManager == nil || msg == nil {
			return ""
		}
		ch, cancel := r.sessionManager.CreateCancellableSession("builtin:keyword-reply", msg.UserID, msg.GroupID, timeout)
		defer cancel()
		select {
		case content, ok := <-ch:
			if !ok {
				return ""
			}
			return strings.TrimSpace(content)
		case <-done:
			return ""
		}
	})
}

func (r *Router) GetSessionManager() *session.Manager {
	return r.sessionManager
}

func (r *Router) HasWaitingSessionForPlugin(userID, groupID, pluginID string) bool {
	pluginID = strings.TrimSpace(pluginID)
	if r.sessionManager == nil || pluginID == "" {
		return false
	}
	session := r.sessionManager.GetSession(userID, groupID)
	return session != nil && session.PluginID == pluginID
}

func (r *Router) RegisterPlugin(plugin *types.Plugin) error {
	regex, err := regexp.Compile(plugin.Trigger)
	if err != nil {
		return err
	}
	plugin.TriggerRegex = regex

	r.mu.Lock()
	r.pluginOrder++
	plugin.Order = r.pluginOrder
	r.plugins[plugin.ID] = plugin
	r.mu.Unlock()

	log.Printf("Plugin registered: %s (trigger: %s)", plugin.Name, plugin.Trigger)
	return nil
}

func (r *Router) UnregisterPlugin(pluginID string) {
	r.mu.Lock()
	delete(r.plugins, pluginID)
	r.mu.Unlock()

	log.Printf("Plugin unregistered: %s", pluginID)
}

func (r *Router) HandleMessage(msg *types.Message) {
	atomic.AddUint64(&r.messageCount, 1)
	r.mu.RLock()
	database := r.database
	r.mu.RUnlock()
	if database != nil {
		if err := database.RecordMessageStat(msg); err != nil {
			log.Printf("[SYSTEM] Record message stats failed: %v", err)
		}
	}
	if msg.Metadata == nil {
		msg.Metadata = map[string]string{}
	}
	isEvent := isEventMessage(msg)
	if !isEvent && msg.Metadata["fake"] != "true" && r.sessionManager.HandleMessage(msg.UserID, msg.GroupID, msg.Content) {
		log.Printf("%s Message intercepted by waiting session", listenLogPrefix(msg))
		return
	}
	r.mu.RLock()
	keywordReplies := r.keywordReplies
	r.mu.RUnlock()
	systemAccess := r.systemAccessControl()
	if !r.allowSystemHardBlock(systemAccess, msg) {
		log.Printf("[SYSTEM] Message blocked by system access control: platform=%s user=%s group=%s", msg.Platform, msg.UserID, msg.GroupID)
		return
	}
	if !isEvent && keywordReplies != nil && keywordReplies.Handle(msg) {
		return
	}

	matchedPlugins := r.matchPlugins(msg)
	if len(matchedPlugins) == 0 {
		log.Printf("[SYSTEM] No plugin matched")
		return
	}
	if database != nil && !isEvent {
		if _, err := database.GetUserAccount(msg.Platform, msg.UserID); err != nil {
			adp := r.getAdapterForMessage(msg)
			if adp != nil {
				_ = adp.SendMessage(resolveReplyTarget(adp, msg), formatReplyText(adp, msg, userRegisterGuide()))
			}
			return
		}
	}
	pluginsToRun := matchedPlugins[:1]
	if isEvent {
		pluginsToRun = matchedPlugins
	}
	if database != nil {
		for _, plugin := range pluginsToRun {
			if err := database.RecordPluginTriggerStat(plugin, msg); err != nil {
				log.Printf("[SYSTEM] Record plugin trigger stats failed: %v", err)
			}
		}
	}
	for _, plugin := range pluginsToRun {
		go r.callPlugin(plugin, msg)
	}
}

func (r *Router) HandleMessageForPlugin(msg *types.Message, pluginID string) error {
	pluginID = strings.TrimSpace(pluginID)
	if msg == nil || pluginID == "" {
		return fmt.Errorf("插件不能为空")
	}
	atomic.AddUint64(&r.messageCount, 1)
	r.mu.RLock()
	database := r.database
	r.mu.RUnlock()
	if database != nil {
		if err := database.RecordMessageStat(msg); err != nil {
			log.Printf("[SYSTEM] Record message stats failed: %v", err)
		}
	}
	if msg.Metadata == nil {
		msg.Metadata = map[string]string{}
	}
	msg.Metadata["web_chat_plugin_id"] = pluginID
	sessionGroupID := messageSessionGroupID(msg)
	if msg.Metadata["fake"] != "true" && r.sessionManager != nil && r.sessionManager.HandleMessageForPlugin(msg.UserID, sessionGroupID, pluginID, msg.Content) {
		log.Printf("%s Message intercepted by waiting session", listenLogPrefix(msg))
		return nil
	}
	systemAccess := r.systemAccessControl()
	if !r.allowSystemHardBlock(systemAccess, msg) {
		return fmt.Errorf("消息被系统访问控制拦截")
	}
	plugin := r.GetPlugin(pluginID)
	if plugin == nil {
		return fmt.Errorf("插件不存在")
	}
	if !plugin.Enabled {
		return fmt.Errorf("插件未启用")
	}
	if !r.supportsPlatform(plugin, msg.Platform) || !r.supportsAdapter(plugin, msg.AdapterID) || !r.allowPluginMessage(plugin, msg) {
		return fmt.Errorf("无权访问该插件")
	}
	if database != nil {
		if _, err := database.GetUserAccount(msg.Platform, msg.UserID); err != nil {
			adp := r.getAdapterForMessage(msg)
			if adp != nil {
				_ = adp.SendMessage(resolveReplyTarget(adp, msg), formatReplyText(adp, msg, userRegisterGuide()))
			}
			return fmt.Errorf("用户未注册")
		}
		if err := database.RecordPluginTriggerStat(plugin, msg); err != nil {
			log.Printf("[SYSTEM] Record plugin trigger stats failed: %v", err)
		}
	}
	go r.callPlugin(plugin, msg)
	return nil
}

func (r *Router) MessageCount() uint64 {
	return atomic.LoadUint64(&r.messageCount)
}

func isEventMessage(msg *types.Message) bool {
	if msg == nil || msg.Metadata == nil {
		return false
	}
	return msg.Metadata["message_type"] == "event" || strings.TrimSpace(msg.Metadata["event_name"]) != ""
}

func (r *Router) matchPlugins(msg *types.Message) []*types.Plugin {
	matched := r.FilterPluginsForMessage(msg, true)
	if len(matched) == 0 {
		return nil
	}
	log.Printf("[SYSTEM] Plugin matched: %s(priority=%d) for message: %s", matched[0].Name, matched[0].Priority, msg.Content)
	return matched
}

func (r *Router) FilterPluginsForMessage(msg *types.Message, requireTrigger bool) []*types.Plugin {
	if msg == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	matched := make([]*types.Plugin, 0)
	for _, plugin := range r.plugins {
		if !plugin.Enabled {
			continue
		}
		if !r.supportsPlatform(plugin, msg.Platform) {
			continue
		}
		if !r.supportsAdapter(plugin, msg.AdapterID) {
			continue
		}
		if !r.allowPluginMessage(plugin, msg) {
			continue
		}
		if requireTrigger && (plugin.TriggerRegex == nil || !plugin.TriggerRegex.MatchString(msg.Content)) {
			continue
		}
		matched = append(matched, plugin)
	}

	if len(matched) == 0 {
		return nil
	}
	sort.SliceStable(matched, func(i, j int) bool {
		if matched[i].Priority == matched[j].Priority {
			return matched[i].Order < matched[j].Order
		}
		return matched[i].Priority > matched[j].Priority
	})
	return matched
}

func (r *Router) systemAccessControl() types.AccessControlConfig {
	r.mu.RLock()
	database := r.database
	r.mu.RUnlock()
	if database == nil {
		return types.AccessControlConfig{}
	}
	settings, err := database.GetSystemSettings()
	if err != nil || settings == nil {
		return types.AccessControlConfig{}
	}
	return settings.AccessControl
}

func (r *Router) allowPluginMessage(plugin *types.Plugin, msg *types.Message) bool {
	config := plugin.AccessControl
	unionID := r.messageUnionID(msg)
	if config.InheritSystem {
		return allowMessageByAccessControl(r.systemAccessControl(), msg, unionID)
	}
	return allowMessageByAccessControl(config, msg, unionID)
}

func messageSessionGroupID(msg *types.Message) string {
	if msg == nil {
		return ""
	}
	if msg.Metadata != nil {
		if groupID := strings.TrimSpace(msg.Metadata["web_chat_session_group_id"]); groupID != "" {
			return groupID
		}
	}
	return msg.GroupID
}

func (r *Router) messageUnionID(msg *types.Message) string {
	if msg == nil {
		return ""
	}
	r.mu.RLock()
	database := r.database
	r.mu.RUnlock()
	if database == nil {
		return ""
	}
	account, err := database.GetUserAccount(msg.Platform, msg.UserID)
	if err != nil {
		return ""
	}
	return account.UnionID
}

func (r *Router) allowSystemHardBlock(config types.AccessControlConfig, msg *types.Message) bool {
	return allowSystemHardBlock(config, msg, r.messageUnionID(msg))
}

func allowSystemHardBlock(config types.AccessControlConfig, msg *types.Message, unionID string) bool {
	if containsString(config.BlockedUserIDs, msg.UserID) {
		return false
	}
	if unionID != "" && containsString(config.BlockedUnionIDs, unionID) {
		return false
	}
	if msg.GroupID != "" && containsString(config.BlockedGroups, msg.GroupID) {
		return false
	}
	return true
}

func allowMessageByAccessControl(config types.AccessControlConfig, msg *types.Message, unionID string) bool {
	if containsString(config.BlockedUserIDs, msg.UserID) {
		return false
	}
	if unionID != "" && containsString(config.BlockedUnionIDs, unionID) {
		return false
	}
	if msg.GroupID != "" && containsString(config.BlockedGroups, msg.GroupID) {
		return false
	}
	if len(config.WhitelistUserIDs) > 0 && !containsString(config.WhitelistUserIDs, msg.UserID) {
		return false
	}
	if len(config.WhitelistUnionIDs) > 0 && (unionID == "" || !containsString(config.WhitelistUnionIDs, unionID)) {
		return false
	}
	if msg.GroupID != "" && len(config.WhitelistGroups) > 0 && !containsString(config.WhitelistGroups, msg.GroupID) {
		return false
	}
	return true
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func (r *Router) supportsPlatform(plugin *types.Plugin, platform string) bool {
	if len(plugin.Platforms) == 0 {
		return true
	}
	for _, item := range plugin.Platforms {
		if item == platform {
			return true
		}
	}
	return false
}

func (r *Router) supportsAdapter(plugin *types.Plugin, adapterID string) bool {
	if len(plugin.AllowedAdapterIDs) == 0 {
		return true
	}
	for _, item := range plugin.AllowedAdapterIDs {
		if item == adapterID {
			return true
		}
	}
	return false
}

func (r *Router) callPlugin(plugin *types.Plugin, msg *types.Message) {
	if !plugin.Enabled {
		log.Printf("Plugin %s is disabled, skipping", plugin.Name)
		return
	}
	if r.pluginManager == nil {
		log.Printf("Plugin manager not set")
		return
	}

	r.mu.RLock()
	adminChecker := r.adminChecker
	database := r.database
	r.mu.RUnlock()
	isAdmin := false
	if adminChecker != nil {
		isAdmin = adminChecker(msg.Platform, msg.UserID)
	}
	unionID := ""
	points := int64(0)
	pointsUnit := "积分"
	if database != nil {
		if account, err := database.GetUserAccount(msg.Platform, msg.UserID); err == nil {
			unionID = account.UnionID
			points = account.Points
		}
		if unit, err := database.GetSetting("user.points_unit"); err == nil && strings.TrimSpace(unit) != "" {
			pointsUnit = strings.TrimSpace(unit)
		}
	}

	messageJSON, err := json.Marshal(map[string]interface{}{
		"plugin_id":      plugin.ID,
		"platform":       msg.Platform,
		"adapter_id":     msg.AdapterID,
		"user_id":        msg.UserID,
		"union_id":       unionID,
		"points":         points,
		"points_unit":    pointsUnit,
		"group_id":       msg.GroupID,
		"content":        msg.Content,
		"message_id":     msg.ID,
		"is_admin":       isAdmin,
		"metadata":       msg.Metadata,
		"user_config":    plugin.UserConfig,
		"access_control": plugin.AccessControl,
	})
	if err != nil {
		log.Printf("Failed to marshal message: %v", err)
		return
	}

	adp := r.getAdapterForMessage(msg)
	target := resolveReplyTarget(adp, msg)

	responseLogID := pluginResponseLogMessageID(msg)
	replyFunc := func(text string) error {
		log.Printf("%s：消息ID=%s", pluginResponseLogPrefix(msg, plugin), responseLogID)
		if adp == nil {
			log.Printf("[SYSTEM] Adapter not found for platform: %s, reply skipped", msg.Platform)
			return nil
		}
		return adp.SendMessage(target, formatReplyText(adp, msg, text))
	}
	replyMarkdownFunc := func(markdown string) error {
		log.Printf("%s：[Markdown] 消息ID=%s", pluginResponseLogPrefix(msg, plugin), responseLogID)
		if adp == nil {
			log.Printf("[SYSTEM] Adapter not found for platform: %s, markdown reply skipped", msg.Platform)
			return nil
		}
		return sendReplyMarkdownWithFallback(adp, msg, target, markdown)
	}
	imageFunc := func(imageURL string) error {
		log.Printf("%s：[图片] 消息ID=%s", pluginResponseLogPrefix(msg, plugin), responseLogID)
		if adp == nil {
			return fmt.Errorf("适配器不存在: %s", msg.Platform)
		}
		return adp.SendImage(target, imageURL)
	}
	fileFunc := func(filePath string) error {
		log.Printf("%s：[文件] 消息ID=%s", pluginResponseLogPrefix(msg, plugin), responseLogID)
		if adp == nil {
			log.Printf("[SYSTEM] Adapter not found for platform: %s, file skipped", msg.Platform)
			return nil
		}
		return adp.SendFile(target, filePath)
	}
	buttonsFunc := func(text string, buttons [][]types.ButtonOption) error {
		log.Printf("%s：[按钮] 消息ID=%s", pluginResponseLogPrefix(msg, plugin), responseLogID)
		if adp == nil {
			log.Printf("[SYSTEM] Adapter not found for platform: %s, buttons skipped", msg.Platform)
			return nil
		}
		return sendReplyButtonsWithFallback(adp, msg, target, text, buttons)
	}
	replyRichFunc := func(message types.RichMessage) error {
		log.Printf("%s：[富文本] 消息ID=%s", pluginResponseLogPrefix(msg, plugin), responseLogID)
		if adp == nil {
			log.Printf("[SYSTEM] Adapter not found for platform: %s, rich reply skipped", msg.Platform)
			return nil
		}
		return sendReplyRichWithFallback(adp, msg, target, message)
	}

	listenFunc := func(timeout int) string {
		ch := r.sessionManager.CreateSession(plugin.ID, msg.UserID, messageSessionGroupID(msg), timeout)
		content, ok := <-ch
		if !ok {
			return ""
		}
		return content
	}
	listenUntilFunc := func(timeout int, done <-chan struct{}) string {
		ch, cancel := r.sessionManager.CreateCancellableSession(plugin.ID, msg.UserID, messageSessionGroupID(msg), timeout)
		defer cancel()
		select {
		case content, ok := <-ch:
			if !ok {
				return ""
			}
			return content
		case <-done:
			return ""
		}
	}

	pluginPath := filepath.Join("plugins", plugin.ID)
	r.mu.RLock()
	dataViewSaver := r.dataViewSaver
	r.mu.RUnlock()

	dbFunc := func(pluginID string, action plugincore.PluginDBAction) plugincore.PluginDBResult {
		return executePluginDBAction(database, pluginID, action)
	}
	fakeMessageFunc := func(pluginID string, action plugincore.FakeMessageAction) error {
		return r.dispatchFakeMessage(pluginID, action)
	}
	sendMessageFunc := func(pluginID string, action plugincore.SendMessageAction) plugincore.PluginUserResult {
		if err := r.sendPluginMessage(pluginID, action); err != nil {
			return plugincore.PluginUserResult{Success: false, Error: err.Error()}
		}
		return plugincore.PluginUserResult{Success: true, Data: true}
	}
	sendRichMessageFunc := func(pluginID string, action plugincore.RichMessageAction) plugincore.PluginUserResult {
		return r.sendPluginRichMessage(pluginID, action)
	}
	userFunc := func() plugincore.PluginUserResult {
		if database == nil {
			return plugincore.PluginUserResult{Success: false, Error: "数据库不可用"}
		}
		account, err := database.GetUserAccount(msg.Platform, msg.UserID)
		if err != nil {
			return plugincore.PluginUserResult{Success: false, Error: userRegisterGuide()}
		}
		return plugincore.PluginUserResult{Success: true, Data: map[string]interface{}{"union_id": account.UnionID, "platform": account.Platform, "user_id": account.UserID, "points": account.Points}}
	}
	adminFunc := func(platform string) plugincore.PluginUserResult {
		items, err := r.startedPlatformAdmins(platform)
		if err != nil {
			return plugincore.PluginUserResult{Success: false, Error: err.Error()}
		}
		return plugincore.PluginUserResult{Success: true, Data: items}
	}
	configFunc := func(pluginID string, action plugincore.PluginConfigAction) plugincore.PluginUserResult {
		if err := r.pluginManager.SavePluginAccessControl(pluginID, action.AccessControl); err != nil {
			return plugincore.PluginUserResult{Success: false, Error: err.Error()}
		}
		return plugincore.PluginUserResult{Success: true, Data: action.AccessControl}
	}
	scheduleFunc := func(pluginID string, action plugincore.ScheduledTaskAction) plugincore.PluginUserResult {
		if database == nil {
			return plugincore.PluginUserResult{Success: false, Error: "数据库不可用"}
		}
		task := &config.ScheduledTask{TaskKey: action.TaskKey, Name: action.Name, Description: action.Description, Enabled: action.Enabled, Pinned: action.Pinned, Cron: action.Cron, Platform: action.Platform, AdapterID: action.AdapterID, UserID: action.UserID, GroupID: action.GroupID, Content: action.Content}
		if task.Platform == "" {
			task.Platform = msg.Platform
		}
		if task.AdapterID == "" {
			task.AdapterID = msg.AdapterID
		}
		if task.UserID == "" {
			task.UserID = msg.UserID
		}
		if task.GroupID == "" {
			task.GroupID = msg.GroupID
		}
		if task.Enabled && !isOnceCron(task.Cron) {
			if _, err := NextCronTime(task.Cron, time.Now()); err != nil {
				return plugincore.PluginUserResult{Success: false, Error: err.Error()}
			}
		}
		saved, err := database.UpsertPluginScheduledTask(pluginID, task, action.MaxCount)
		if err != nil {
			return plugincore.PluginUserResult{Success: false, Error: err.Error()}
		}
		return plugincore.PluginUserResult{Success: true, Data: saved}
	}
	accountFunc := func(pluginID string, action plugincore.PluginAccountAction) plugincore.PluginUserResult {
		if database == nil {
			return plugincore.PluginUserResult{Success: false, Error: "数据库不可用"}
		}
		accountUnionID := strings.TrimSpace(action.UnionID)
		if accountUnionID == "" {
			accountUnionID = stringDefault(unionID, fmt.Sprintf("%s:%s", msg.Platform, msg.UserID))
		}
		scopeAll := strings.TrimSpace(action.Scope) == "all"
		switch action.Action {
		case "account_save":
			var expiresAt *time.Time
			if strings.TrimSpace(action.ExpiresAt) != "" {
				parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(action.ExpiresAt))
				if err != nil {
					return plugincore.PluginUserResult{Success: false, Error: "账号过期时间格式无效，请使用 RFC3339"}
				}
				expiresAt = &parsed
			}
			item := &config.PluginAccount{ID: action.ID, PluginID: pluginID, TableName: action.TableName, UnionID: accountUnionID, Platform: stringDefault(action.Platform, msg.Platform), UserID: stringDefault(action.UserID, msg.UserID), AccountName: action.AccountName, EnvName: action.EnvName, EnvValue: action.EnvValue, Remark: action.Remark, Status: action.Status, Metadata: action.Metadata, ExpiresAt: expiresAt}
			saved, err := database.SavePluginAccount(item)
			if err != nil {
				return plugincore.PluginUserResult{Success: false, Error: err.Error()}
			}
			return plugincore.PluginUserResult{Success: true, Data: saved}
		case "account_list":
			items, err := database.ListPluginAccounts(pluginID, config.PluginAccountQuery{TableName: action.TableName, Scope: action.Scope, UnionID: accountUnionID, EnvName: action.EnvName, Status: action.Status})
			if err != nil {
				return plugincore.PluginUserResult{Success: false, Error: err.Error()}
			}
			return plugincore.PluginUserResult{Success: true, Data: items}
		case "account_delete":
			if err := database.DeletePluginAccount(pluginID, action.TableName, action.ID, accountUnionID, scopeAll); err != nil {
				return plugincore.PluginUserResult{Success: false, Error: err.Error()}
			}
			return plugincore.PluginUserResult{Success: true, Data: true}
		default:
			return plugincore.PluginUserResult{Success: false, Error: "未知账号动作"}
		}
	}
	authFunc := func(pluginID string, action plugincore.PluginAuthorizationAction) plugincore.PluginUserResult {
		if database == nil {
			return plugincore.PluginUserResult{Success: false, Error: "数据库不可用"}
		}
		authUnionID := strings.TrimSpace(action.UnionID)
		if authUnionID == "" {
			authUnionID = stringDefault(unionID, fmt.Sprintf("%s:%s", msg.Platform, msg.UserID))
		}
		switch action.Action {
		case "points_consume":
			if action.Amount <= 0 {
				return plugincore.PluginUserResult{Success: false, Error: "积分数量必须大于 0"}
			}
			remaining, err := database.ConsumeUserPoints(authUnionID, action.Amount)
			if err != nil {
				return plugincore.PluginUserResult{Success: false, Error: err.Error(), Data: map[string]interface{}{"points": remaining}}
			}
			return plugincore.PluginUserResult{Success: true, Data: map[string]interface{}{"points": remaining}}
		case "points_add":
			if action.Amount <= 0 {
				return plugincore.PluginUserResult{Success: false, Error: "积分数量必须大于 0"}
			}
			if !isAdmin {
				return plugincore.PluginUserResult{Success: false, Error: "仅平台管理员可操作积分"}
			}
			remaining, err := database.AddUserPoints(authUnionID, action.Amount)
			if err != nil {
				return plugincore.PluginUserResult{Success: false, Error: err.Error(), Data: map[string]interface{}{"points": remaining}}
			}
			return plugincore.PluginUserResult{Success: true, Data: map[string]interface{}{"points": remaining}}
		case "auth_check":
			item, err := database.GetPluginAuthorization(pluginID, action.TableName, authUnionID)
			if err != nil {
				return plugincore.PluginUserResult{Success: false, Error: err.Error()}
			}
			return plugincore.PluginUserResult{Success: true, Data: map[string]interface{}{"authorized": item.IsActive(time.Now()), "authorization": item}}
		case "auth_grant":
			var expiresAt *time.Time
			if strings.TrimSpace(action.ExpiresAt) != "" {
				parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(action.ExpiresAt))
				if err != nil {
					return plugincore.PluginUserResult{Success: false, Error: "授权过期时间格式无效，请使用 RFC3339"}
				}
				expiresAt = &parsed
			}
			saved, err := database.SavePluginAuthorization(&config.PluginAuthorization{PluginID: pluginID, TableName: action.TableName, UnionID: authUnionID, Status: stringDefault(action.Status, "active"), Plan: action.Plan, Source: action.Source, Metadata: action.Metadata, ExpiresAt: expiresAt})
			if err != nil {
				return plugincore.PluginUserResult{Success: false, Error: err.Error()}
			}
			return plugincore.PluginUserResult{Success: true, Data: saved}
		case "auth_revoke":
			if err := database.RevokePluginAuthorization(pluginID, action.TableName, authUnionID); err != nil {
				return plugincore.PluginUserResult{Success: false, Error: err.Error()}
			}
			return plugincore.PluginUserResult{Success: true, Data: true}
		default:
			return plugincore.PluginUserResult{Success: false, Error: "未知授权动作"}
		}
	}
	scriptFunc := func(pluginID string, action plugincore.ScriptRunAction) plugincore.PluginUserResult {
		if strings.TrimSpace(action.UnionID) == "" {
			action.UnionID = stringDefault(unionID, fmt.Sprintf("%s:%s", msg.Platform, msg.UserID))
		}
		action.PluginID = pluginID
		return r.pluginManager.RunPluginScript(filepath.Join("plugins", pluginID), action)
	}
	paymentFunc := func(pluginID string, action plugincore.PaymentWaitAction) plugincore.PluginUserResult {
		if database == nil {
			return plugincore.PluginUserResult{Success: false, Error: "数据库不可用"}
		}
		paymentUnionID := strings.TrimSpace(action.UnionID)
		if paymentUnionID == "" {
			paymentUnionID = unionID
		}
		if paymentUnionID == "" {
			return plugincore.PluginUserResult{Success: false, Error: "用户 union_id 不能为空"}
		}
		service := payment.NewService(database)
		result, err := service.WaitPay(payment.WaitPayRequest{PluginID: pluginID, Platform: msg.Platform, AdapterID: msg.AdapterID, UserID: msg.UserID, GroupID: msg.GroupID, UnionID: paymentUnionID, Subject: action.Subject, AmountRaw: action.AmountRaw, Timeout: action.Timeout, PointsUnit: pointsUnit, Methods: action.Methods, Metadata: action.Metadata, Remark: action.Remark}, payment.Interaction{Reply: replyFunc, ReplyButtons: buttonsFunc, SendImage: imageFunc, SendRich: replyRichFunc, Listen: listenFunc, ListenUntil: listenUntilFunc})
		if err != nil {
			return plugincore.PluginUserResult{Success: false, Error: err.Error(), Data: result}
		}
		if result.Status != "paid" {
			return plugincore.PluginUserResult{Success: false, Error: result.Message, Data: result}
		}
		return plugincore.PluginUserResult{Success: true, Error: "", Data: result}
	}

	if err := r.pluginManager.ExecutePlugin(plugin, pluginPath, messageJSON, replyFunc, imageFunc, fileFunc, listenFunc, dataViewSaver, dbFunc, fakeMessageFunc, sendMessageFunc, userFunc, adminFunc, configFunc, scheduleFunc, accountFunc, authFunc, scriptFunc, paymentFunc, buttonsFunc, replyMarkdownFunc, replyRichFunc, sendRichMessageFunc); err != nil {
		log.Printf("Failed to execute plugin %s: %v", plugin.Name, err)
	}
}

func resolveReplyTarget(adp adapter.Adapter, msg *types.Message) string {
	if msg == nil {
		return ""
	}
	if resolver, ok := adp.(adapter.ReplyTargetResolver); ok {
		if target := strings.TrimSpace(resolver.ReplyTarget(msg)); target != "" {
			return target
		}
	}
	return defaultReplyTarget(msg)
}

func defaultReplyTarget(msg *types.Message) string {
	if msg == nil {
		return ""
	}
	if msg.GroupID != "" {
		return msg.GroupID
	}
	return msg.UserID
}

func formatReplyText(adp adapter.Adapter, msg *types.Message, text string) string {
	if formatter, ok := adp.(adapter.ReplyTextFormatter); ok {
		return formatter.FormatReplyText(msg, text)
	}
	return text
}

func sendMarkdownWithFallback(adp adapter.Adapter, target string, markdown string) error {
	markdown = strings.TrimSpace(markdown)
	if markdown == "" {
		return fmt.Errorf("消息内容不能为空")
	}
	if sender, ok := adp.(adapter.MarkdownSender); ok {
		return sender.SendMarkdown(target, markdown)
	}
	return adp.SendMessage(target, utils.MarkdownToPlainText(markdown))
}

func sendReplyMarkdownWithFallback(adp adapter.Adapter, msg *types.Message, target string, markdown string) error {
	markdown = strings.TrimSpace(markdown)
	if markdown == "" {
		return fmt.Errorf("消息内容不能为空")
	}
	if sender, ok := adp.(adapter.MarkdownSender); ok {
		return sender.SendMarkdown(target, markdown)
	}
	return adp.SendMessage(target, formatReplyText(adp, msg, utils.MarkdownToPlainText(markdown)))
}

func sendReplyButtonsWithFallback(adp adapter.Adapter, msg *types.Message, target string, text string, buttons [][]types.ButtonOption) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("消息内容不能为空")
	}
	if sender, ok := adp.(adapter.ButtonSender); ok {
		return sender.SendButtons(target, text, buttons)
	}
	return adp.SendMessage(target, formatReplyText(adp, msg, text))
}

func sendRichWithFallback(adp adapter.Adapter, target string, message types.RichMessage) error {
	message.Parts = utils.RichMessageParts(message)
	if strings.TrimSpace(message.FallbackText) == "" && len(message.Parts) == 0 {
		return fmt.Errorf("消息内容不能为空")
	}
	if richSender, ok := adp.(adapter.RichMessageSender); ok && strings.TrimSpace(message.Prefer) != "markdown" && strings.TrimSpace(message.Prefer) != "split" {
		if err := richSender.SendRichMessage(target, message); err == nil {
			return nil
		}
	}
	platform := ""
	if adp != nil {
		platform = adp.GetPlatform()
	}
	if platform == "qq" && strings.TrimSpace(message.Prefer) != "markdown" && strings.TrimSpace(message.Prefer) != "split" {
		text := utils.RichMessageToCQ(message)
		if text != "" {
			return adp.SendMessage(target, text)
		}
	}
	if markdownSender, ok := adp.(adapter.MarkdownSender); ok && strings.TrimSpace(message.Prefer) != "split" && !isPlainRichFallbackPlatform(platform) && !isSplitRichFallbackPlatform(platform) {
		if markdown := utils.RichMessageToMarkdown(message); markdown != "" {
			if err := markdownSender.SendMarkdown(target, markdown); err == nil {
				return nil
			}
		}
	}
	return sendRichSplitFallback(adp, target, message, nil)
}

func sendReplyRichWithFallback(adp adapter.Adapter, msg *types.Message, target string, message types.RichMessage) error {
	message.Parts = utils.RichMessageParts(message)
	if strings.TrimSpace(message.FallbackText) == "" && len(message.Parts) == 0 {
		return fmt.Errorf("消息内容不能为空")
	}
	if richSender, ok := adp.(adapter.RichMessageSender); ok && strings.TrimSpace(message.Prefer) != "markdown" && strings.TrimSpace(message.Prefer) != "split" {
		if err := richSender.SendRichMessage(target, message); err == nil {
			return nil
		}
	}
	platform := ""
	if adp != nil {
		platform = adp.GetPlatform()
	}
	if platform == "qq" && strings.TrimSpace(message.Prefer) != "markdown" && strings.TrimSpace(message.Prefer) != "split" {
		text := utils.RichMessageToCQ(message)
		if text != "" {
			return adp.SendMessage(target, formatReplyText(adp, msg, text))
		}
	}
	if markdownSender, ok := adp.(adapter.MarkdownSender); ok && strings.TrimSpace(message.Prefer) != "split" && !isPlainRichFallbackPlatform(platform) && !isSplitRichFallbackPlatform(platform) {
		if markdown := utils.RichMessageToMarkdown(message); markdown != "" {
			if err := markdownSender.SendMarkdown(target, markdown); err == nil {
				return nil
			}
		}
	}
	return sendRichSplitFallback(adp, target, message, func(text string) string { return formatReplyText(adp, msg, text) })
}

func sendRichSplitFallback(adp adapter.Adapter, target string, message types.RichMessage, textFormatter func(string) string) error {
	platform := ""
	if adp != nil {
		platform = adp.GetPlatform()
	}
	if isPlainRichFallbackPlatform(platform) || (strings.TrimSpace(message.FallbackText) != "" && !isSplitRichFallbackPlatform(platform)) {
		return sendRichPlainFallback(adp, target, message, textFormatter)
	}
	var textBuffer strings.Builder
	flushText := func() error {
		text := strings.TrimSpace(textBuffer.String())
		textBuffer.Reset()
		if text == "" {
			return nil
		}
		if textFormatter != nil {
			text = textFormatter(text)
		}
		return adp.SendMessage(target, text)
	}
	for _, part := range message.Parts {
		switch part.Type {
		case "text":
			textBuffer.WriteString(part.Text)
		case "markdown":
			textBuffer.WriteString(utils.MarkdownToPlainText(part.Markdown))
		case "image":
			if part.URL == "" {
				continue
			}
			if err := flushText(); err != nil {
				return err
			}
			if err := adp.SendImage(target, part.URL); err != nil {
				fallback := types.RichMessage{Parts: []types.RichMessagePart{part}}
				if err := sendRichPlainFallback(adp, target, fallback, textFormatter); err != nil {
					return err
				}
			}
		}
	}
	return flushText()
}

func sendRichPlainFallback(adp adapter.Adapter, target string, message types.RichMessage, textFormatter func(string) string) error {
	text := utils.RichMessageToPlainText(message)
	if strings.TrimSpace(text) == "" {
		return nil
	}
	if textFormatter != nil {
		text = textFormatter(text)
	}
	return adp.SendMessage(target, text)
}

func isPlainRichFallbackPlatform(platform string) bool {
	return strings.EqualFold(platform, "wechat_official")
}

func isSplitRichFallbackPlatform(platform string) bool {
	return strings.EqualFold(platform, "feishu")
}

func resolveSendTarget(adp adapter.Adapter, userID string, groupID string) string {
	if resolver, ok := adp.(adapter.SendTargetResolver); ok {
		if target := strings.TrimSpace(resolver.SendTarget(userID, groupID)); target != "" {
			return target
		}
	}
	if groupID != "" {
		return groupID
	}
	return userID
}

func (r *Router) dispatchFakeMessage(pluginID string, action plugincore.FakeMessageAction) error {
	platform := strings.TrimSpace(action.Platform)
	adapterID := strings.TrimSpace(action.AdapterID)
	userID := strings.TrimSpace(action.UserID)
	groupID := strings.TrimSpace(action.GroupID)
	content := strings.TrimSpace(action.Content)
	if platform == "" && adapterID == "" {
		return fmt.Errorf("平台不能为空")
	}
	if userID == "" {
		return fmt.Errorf("用户 ID 不能为空")
	}
	if content == "" {
		return fmt.Errorf("消息内容不能为空")
	}

	resolvedPlatform, resolvedAdapterID, adapterRemark, adapterDescription, err := r.resolveAdapterInfo(platform, adapterID)
	if err != nil {
		return err
	}
	platform = resolvedPlatform
	adapterID = resolvedAdapterID
	if platform == "" {
		return fmt.Errorf("平台不能为空")
	}
	msg := &types.Message{
		ID:        fmt.Sprintf("fake-%s-%d", pluginID, time.Now().UnixNano()),
		Platform:  platform,
		AdapterID: adapterID,
		UserID:    userID,
		GroupID:   groupID,
		Content:   content,
		Metadata: map[string]string{
			"fake":               "true",
			"fake_source_plugin": pluginID,
		},
	}
	if adapterID != "" {
		msg.Metadata["adapter_id"] = adapterID
		msg.Metadata["adapter_platform"] = platform
		msg.Metadata["adapter_remark"] = adapterRemark
		msg.Metadata["adapter_description"] = adapterDescription
	}
	log.Printf("[SYSTEM] Plugin %s fake message: platform=%s adapter_id=%s user=%s group=%s content=%s", pluginID, platform, adapterID, userID, groupID, content)
	go r.HandleMessage(msg)
	return nil
}

func (r *Router) DispatchFakeMessage(pluginID string, platform string, userID string, groupID string, content string) error {
	return r.DispatchFakeMessageWithAdapter(pluginID, platform, "", userID, groupID, content)
}

func (r *Router) DispatchFakeMessageWithAdapter(pluginID string, platform string, adapterID string, userID string, groupID string, content string) error {
	return r.dispatchFakeMessage(pluginID, plugincore.FakeMessageAction{Platform: platform, AdapterID: adapterID, UserID: userID, GroupID: groupID, Content: content})
}

func (r *Router) SendPluginMessage(pluginID string, action plugincore.SendMessageAction) plugincore.PluginUserResult {
	if err := r.sendPluginMessage(pluginID, action); err != nil {
		return plugincore.PluginUserResult{Success: false, Error: err.Error()}
	}
	return plugincore.PluginUserResult{Success: true, Data: true}
}

func (r *Router) SendPluginRichMessage(pluginID string, action plugincore.RichMessageAction) plugincore.PluginUserResult {
	return r.sendPluginRichMessage(pluginID, action)
}

func (r *Router) ExecutePluginDBAction(pluginID string, action plugincore.PluginDBAction) plugincore.PluginDBResult {
	r.mu.RLock()
	database := r.database
	r.mu.RUnlock()
	return executePluginDBAction(database, pluginID, action)
}

func (r *Router) sendPluginMessage(pluginID string, action plugincore.SendMessageAction) error {
	platform := strings.TrimSpace(action.Platform)
	adapterID := strings.TrimSpace(action.AdapterID)
	userID := strings.TrimSpace(action.UserID)
	groupID := strings.TrimSpace(action.GroupID)
	unionID := strings.TrimSpace(action.UnionID)
	text := strings.TrimSpace(action.Text)
	if text == "" {
		return fmt.Errorf("消息内容不能为空")
	}
	if platform == "" && adapterID == "" && unionID == "" {
		return fmt.Errorf("平台不能为空")
	}
	if userID == "" && groupID == "" && unionID == "" {
		return fmt.Errorf("用户 ID 和群组 ID 不能同时为空")
	}
	if unionID != "" && groupID == "" {
		sent, err := r.sendPluginMessageToUnion(pluginID, unionID, platform, adapterID, text)
		if sent {
			return nil
		}
		if err != nil {
			return err
		}
		if (platform == "" && adapterID == "") || userID == "" {
			return fmt.Errorf("UnionID %s 没有可用平台账号", unionID)
		}
	}
	resolvedPlatform, resolvedAdapterID, _, _, adp, err := r.resolveSendAdapterInfo(platform, adapterID)
	if err != nil {
		return err
	}
	platform = resolvedPlatform
	adapterID = resolvedAdapterID
	if platform == "" {
		return fmt.Errorf("平台不能为空")
	}
	if adp == nil {
		return fmt.Errorf("适配器不存在: %s", platform)
	}
	target := resolveSendTarget(adp, userID, groupID)
	log.Printf("[SYSTEM] Plugin %s send message: platform=%s adapter_id=%s user=%s group=%s text=%s", pluginID, platform, adapterID, userID, groupID, text)
	return adp.SendMessage(target, text)
}

func (r *Router) sendPluginRichMessage(pluginID string, action plugincore.RichMessageAction) plugincore.PluginUserResult {
	platform := strings.TrimSpace(action.Platform)
	adapterID := strings.TrimSpace(action.AdapterID)
	userID := strings.TrimSpace(action.UserID)
	groupID := strings.TrimSpace(action.GroupID)
	unionID := strings.TrimSpace(action.UnionID)
	message := types.RichMessage{Parts: action.Parts, FallbackText: action.FallbackText, Prefer: action.Prefer}
	message.Parts = utils.RichMessageParts(message)
	if strings.TrimSpace(message.FallbackText) == "" && len(message.Parts) == 0 {
		return plugincore.PluginUserResult{Success: false, Error: "消息内容不能为空"}
	}
	if platform == "" && adapterID == "" && unionID == "" {
		return plugincore.PluginUserResult{Success: false, Error: "平台不能为空"}
	}
	if userID == "" && groupID == "" && unionID == "" {
		return plugincore.PluginUserResult{Success: false, Error: "用户 ID 和群组 ID 不能同时为空"}
	}
	if unionID != "" && groupID == "" {
		sent, err := r.sendPluginRichMessageToUnion(pluginID, unionID, platform, adapterID, message)
		if sent {
			return plugincore.PluginUserResult{Success: true, Data: true}
		}
		if err != nil {
			return plugincore.PluginUserResult{Success: false, Error: err.Error()}
		}
		if (platform == "" && adapterID == "") || userID == "" {
			return plugincore.PluginUserResult{Success: false, Error: fmt.Sprintf("UnionID %s 没有可用平台账号", unionID)}
		}
	}
	resolvedPlatform, resolvedAdapterID, _, _, adp, err := r.resolveSendAdapterInfo(platform, adapterID)
	if err != nil {
		return plugincore.PluginUserResult{Success: false, Error: err.Error()}
	}
	platform = resolvedPlatform
	adapterID = resolvedAdapterID
	if platform == "" {
		return plugincore.PluginUserResult{Success: false, Error: "平台不能为空"}
	}
	if adp == nil {
		return plugincore.PluginUserResult{Success: false, Error: fmt.Sprintf("适配器不存在: %s", platform)}
	}
	target := resolveSendTarget(adp, userID, groupID)
	log.Printf("[SYSTEM] Plugin %s send rich message: platform=%s adapter_id=%s user=%s group=%s parts=%d", pluginID, platform, adapterID, userID, groupID, len(message.Parts))
	if err := sendRichWithFallback(adp, target, message); err != nil {
		return plugincore.PluginUserResult{Success: false, Error: err.Error()}
	}
	return plugincore.PluginUserResult{Success: true, Data: true}
}

func (r *Router) sendPluginRichMessageToUnion(pluginID, unionID, platform, adapterID string, message types.RichMessage) (bool, error) {
	platform = strings.TrimSpace(platform)
	adapterID = strings.TrimSpace(adapterID)
	r.mu.RLock()
	database := r.database
	r.mu.RUnlock()
	if database == nil {
		return false, nil
	}
	accounts, err := database.ListUserAccountsByUnionID(unionID)
	if err != nil {
		log.Printf("[SYSTEM] Plugin %s load union accounts failed: union=%s err=%v", pluginID, unionID, err)
		return false, err
	}
	platforms := unionNotifyPlatforms(accounts, platform)
	if len(platforms) == 0 {
		return false, nil
	}
	var lastErr error
	for _, scope := range platforms {
		for _, account := range accounts {
			if account == nil || account.Platform == "" || account.UserID == "" || account.Platform != scope {
				continue
			}
			currentAdapterID := ""
			if scope == platform {
				currentAdapterID = adapterID
			}
			result := r.sendPluginRichMessage(pluginID, plugincore.RichMessageAction{Platform: account.Platform, AdapterID: currentAdapterID, UserID: account.UserID, Parts: message.Parts, FallbackText: message.FallbackText, Prefer: message.Prefer})
			if !result.Success {
				lastErr = fmt.Errorf("%s", result.Error)
				log.Printf("[SYSTEM] Plugin %s union rich notify failed: union=%s platform=%s user=%s err=%v", pluginID, unionID, account.Platform, account.UserID, lastErr)
				continue
			}
			return true, nil
		}
	}
	if lastErr != nil {
		return false, lastErr
	}
	return false, fmt.Errorf("UnionID %s 没有可用平台账号", unionID)
}

func (r *Router) sendPluginMessageToUnion(pluginID, unionID, platform, adapterID, text string) (bool, error) {
	platform = strings.TrimSpace(platform)
	adapterID = strings.TrimSpace(adapterID)
	r.mu.RLock()
	database := r.database
	r.mu.RUnlock()
	if database == nil {
		return false, nil
	}
	accounts, err := database.ListUserAccountsByUnionID(unionID)
	if err != nil {
		log.Printf("[SYSTEM] Plugin %s load union accounts failed: union=%s err=%v", pluginID, unionID, err)
		return false, err
	}
	platforms := unionNotifyPlatforms(accounts, platform)
	if len(platforms) == 0 {
		return false, nil
	}
	var lastErr error
	for _, scope := range platforms {
		for _, account := range accounts {
			if account == nil || account.Platform == "" || account.UserID == "" || account.Platform != scope {
				continue
			}
			currentAdapterID := ""
			if scope == platform {
				currentAdapterID = adapterID
			}
			if err := r.sendPluginMessage(pluginID, plugincore.SendMessageAction{Platform: account.Platform, AdapterID: currentAdapterID, UserID: account.UserID, Text: text}); err != nil {
				log.Printf("[SYSTEM] Plugin %s union notify failed: union=%s platform=%s user=%s err=%v", pluginID, unionID, account.Platform, account.UserID, err)
				lastErr = err
				continue
			}
			return true, nil
		}
	}
	if lastErr != nil {
		return false, lastErr
	}
	return false, fmt.Errorf("UnionID %s 没有可用平台账号", unionID)
}

func unionNotifyPlatforms(accounts []*config.UserAccount, preferred string) []string {
	seen := make(map[string]bool)
	platforms := make([]string, 0, len(accounts))
	add := func(platform string) {
		platform = strings.TrimSpace(platform)
		if platform == "" || seen[platform] {
			return
		}
		seen[platform] = true
		platforms = append(platforms, platform)
	}
	add(preferred)
	for _, account := range accounts {
		if account != nil {
			add(account.Platform)
		}
	}
	return platforms
}

func (r *Router) resolveAdapterInfo(platform string, adapterID string) (string, string, string, string, error) {
	platform = strings.TrimSpace(platform)
	adapterID = strings.TrimSpace(adapterID)
	r.mu.RLock()
	database := r.database
	r.mu.RUnlock()
	if database == nil {
		return platform, adapterID, "", "", nil
	}
	if adapterID != "" {
		id, err := strconv.ParseInt(adapterID, 10, 64)
		if err != nil || id <= 0 {
			return "", "", "", "", fmt.Errorf("适配器 ID 无效: %s", adapterID)
		}
		item, err := database.GetAdapterByID(id)
		if err != nil {
			return "", "", "", "", fmt.Errorf("加载适配器失败: %w", err)
		}
		if item == nil || !item.Enabled {
			return "", "", "", "", fmt.Errorf("适配器不存在或未启用: %s", adapterID)
		}
		if platform != "" && item.Platform != platform {
			return "", "", "", "", fmt.Errorf("适配器 %s 属于 %s，不属于 %s", adapterID, item.Platform, platform)
		}
		return item.Platform, adapterID, strings.TrimSpace(item.Remark), strings.TrimSpace(item.Description), nil
	}
	adapters, err := database.GetAllAdapters()
	if err != nil {
		log.Printf("[SYSTEM] Load adapter info failed: %v", err)
		return platform, "", "", "", nil
	}
	for _, item := range adapters {
		if item != nil && item.Enabled && item.Platform == platform {
			return platform, strconv.FormatInt(item.ID, 10), strings.TrimSpace(item.Remark), strings.TrimSpace(item.Description), nil
		}
	}
	return platform, "", "", "", nil
}

func (r *Router) resolveSendAdapterInfo(platform string, adapterID string) (string, string, string, string, adapter.Adapter, error) {
	platform = strings.TrimSpace(platform)
	adapterID = strings.TrimSpace(adapterID)
	r.mu.RLock()
	database := r.database
	r.mu.RUnlock()
	if database == nil {
		resolvedPlatform, resolvedAdapterID, remark, description, err := r.resolveAdapterInfo(platform, adapterID)
		if err != nil {
			return "", "", "", "", nil, err
		}
		msg := &types.Message{Platform: resolvedPlatform, AdapterID: resolvedAdapterID, Metadata: map[string]string{}}
		if resolvedAdapterID != "" {
			msg.Metadata["adapter_id"] = resolvedAdapterID
		}
		adp := r.getAdapterForMessage(msg)
		if adp == nil && resolvedAdapterID != "" {
			adp = r.getAdapterForMessage(&types.Message{Platform: resolvedPlatform})
		}
		return resolvedPlatform, resolvedAdapterID, remark, description, adp, nil
	}
	if adapterID != "" {
		id, err := strconv.ParseInt(adapterID, 10, 64)
		if err != nil || id <= 0 {
			return "", "", "", "", nil, fmt.Errorf("适配器 ID 无效: %s", adapterID)
		}
		item, err := database.GetAdapterByID(id)
		if err != nil {
			return "", "", "", "", nil, fmt.Errorf("加载适配器失败: %w", err)
		}
		if item == nil || !item.Enabled {
			return "", "", "", "", nil, fmt.Errorf("适配器不存在或未启用: %s", adapterID)
		}
		if platform != "" && item.Platform != platform {
			return "", "", "", "", nil, fmt.Errorf("适配器 %s 属于 %s，不属于 %s", adapterID, item.Platform, platform)
		}
		adp := r.getRunningAdapterByID(item.Platform, adapterID)
		if adp == nil {
			return "", "", "", "", nil, fmt.Errorf("适配器未运行: %s", adapterID)
		}
		return item.Platform, adapterID, strings.TrimSpace(item.Remark), strings.TrimSpace(item.Description), adp, nil
	}
	if platform == "" {
		return "", "", "", "", nil, fmt.Errorf("平台不能为空")
	}
	adapters, err := database.GetAllAdapters()
	if err != nil {
		return "", "", "", "", nil, fmt.Errorf("加载适配器失败: %w", err)
	}
	for _, item := range adapters {
		if item == nil || !item.Enabled || item.Platform != platform {
			continue
		}
		idText := strconv.FormatInt(item.ID, 10)
		adp := r.getRunningAdapterByID(platform, idText)
		if adp != nil {
			return platform, idText, strings.TrimSpace(item.Remark), strings.TrimSpace(item.Description), adp, nil
		}
	}
	return "", "", "", "", nil, fmt.Errorf("平台没有运行中的适配器: %s", platform)
}

func (r *Router) getRunningAdapterByID(platform string, adapterID string) adapter.Adapter {
	adapterID = strings.TrimSpace(adapterID)
	if adapterID == "" {
		return nil
	}
	r.mu.RLock()
	messageGetter := r.messageGetter
	r.mu.RUnlock()
	if messageGetter == nil {
		return nil
	}
	return messageGetter(&types.Message{Platform: platform, AdapterID: adapterID, Metadata: map[string]string{"adapter_id": adapterID}})
}

func executePluginDBAction(database *config.Database, pluginID string, action plugincore.PluginDBAction) plugincore.PluginDBResult {
	if database == nil {
		return plugincore.PluginDBResult{Success: false, Error: "数据库不可用"}
	}

	var data interface{}
	var err error
	switch action.Action {
	case "db_create_table":
		data, err = database.EnsurePluginTable(pluginID, action.Table, action.Columns)
	case "db_query":
		query := action.Query
		if query.Table == "" {
			query.Table = action.Table
		}
		data, err = database.QueryPluginRows(pluginID, query)
	case "db_insert":
		data, err = database.InsertPluginRow(pluginID, action.Table, action.Values)
	case "db_update":
		err = database.UpdatePluginRow(pluginID, action.Table, action.RowID, action.Values)
	case "db_delete":
		err = database.DeletePluginRow(pluginID, action.Table, action.RowID)
	case "db_clear":
		err = database.ClearPluginTable(pluginID, action.Table)
	default:
		err = fmt.Errorf("不支持的数据库动作: %s", action.Action)
	}
	if err != nil {
		return plugincore.PluginDBResult{Success: false, Error: err.Error()}
	}
	return plugincore.PluginDBResult{Success: true, Data: data}
}

func (r *Router) getAdapterForMessage(msg *types.Message) adapter.Adapter {
	if msg == nil {
		return nil
	}
	r.mu.RLock()
	messageGetter := r.messageGetter
	adapterGetter := r.adapterGetter
	adp := r.adapters[msg.Platform]
	r.mu.RUnlock()

	explicitAdapterID := strings.TrimSpace(msg.AdapterID)
	if explicitAdapterID == "" && msg.Metadata != nil {
		explicitAdapterID = strings.TrimSpace(msg.Metadata["adapter_id"])
	}
	if messageGetter != nil {
		if latestAdapter := messageGetter(msg); latestAdapter != nil {
			return latestAdapter
		}
		if explicitAdapterID != "" {
			return nil
		}
	}
	if adapterGetter != nil {
		if latestAdapter := adapterGetter(msg.Platform); latestAdapter != nil {
			return latestAdapter
		}
	}
	return adp
}

func (r *Router) GetAdapterForMessage(msg *types.Message) adapter.Adapter {
	return r.getAdapterForMessage(msg)
}

func listenLogPrefix(msg *types.Message) string {
	scope := "私聊"
	if msg.GroupID != "" {
		scope = msg.GroupID
	}
	return fmt.Sprintf("[监听][%s][%s(%s)]", msg.Platform, msg.UserID, scope)
}

func stringDefault(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func pluginResponseLogPrefix(msg *types.Message, plugin *types.Plugin) string {
	scope := "私聊"
	if msg.GroupID != "" {
		scope = msg.GroupID
	}
	return fmt.Sprintf("[响应][%s][%s][%s(%s)][插件:%s]", msg.Platform, adapterLogName(msg), msg.UserID, scope, plugin.Name)
}

func pluginResponseLogMessageID(msg *types.Message) string {
	if msg == nil {
		return "-"
	}
	if id := strings.TrimSpace(msg.ID); id != "" {
		return id
	}
	if msg.Metadata != nil {
		for _, key := range []string{"qq_office_msg_id", "message_id", "msg_id"} {
			if id := strings.TrimSpace(msg.Metadata[key]); id != "" {
				return id
			}
		}
	}
	return "-"
}

func adapterLogName(msg *types.Message) string {
	adapterID := strings.TrimSpace(msg.AdapterID)
	remark := ""
	if msg.Metadata != nil {
		if adapterID == "" {
			adapterID = strings.TrimSpace(msg.Metadata["adapter_id"])
		}
		remark = strings.TrimSpace(msg.Metadata["adapter_remark"])
	}

	base := msg.Platform
	if adapterID != "" {
		base = fmt.Sprintf("%s#%s", msg.Platform, adapterID)
	}
	if remark != "" {
		return fmt.Sprintf("%s(%s)", base, remark)
	}
	return base
}

func (r *Router) GetPlugins() []*types.Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugins := make([]*types.Plugin, 0, len(r.plugins))
	for _, plugin := range r.plugins {
		plugins = append(plugins, plugin)
	}
	return plugins
}

func (r *Router) GetPlugin(pluginID string) *types.Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.plugins[pluginID]
}
