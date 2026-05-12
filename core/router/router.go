package router

import (
	"encoding/json"
	"log"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/allbot/allbot/core/adapter"
	"github.com/allbot/allbot/core/plugin"
	"github.com/allbot/allbot/core/session"
	"github.com/allbot/allbot/core/types"
)

// Router 消息路由器
type Router struct {
	plugins        map[string]*types.Plugin
	pluginManager  *plugin.Manager
	sessionManager *session.Manager
	adapters       map[string]adapter.Adapter // platform -> adapter
	mu             sync.RWMutex
}

// NewRouter 创建消息路由器
func NewRouter(sessionManager *session.Manager) *Router {
	return &Router{
		plugins:        make(map[string]*types.Plugin),
		sessionManager: sessionManager,
		adapters:       make(map[string]adapter.Adapter),
	}
}

// SetPluginManager 设置插件管理器
func (r *Router) SetPluginManager(pm *plugin.Manager) {
	r.pluginManager = pm
}

// SetAdapters 设置适配器映射
func (r *Router) SetAdapters(adapters map[string]adapter.Adapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters = adapters
}

// GetSessionManager 获取会话管理器
func (r *Router) GetSessionManager() *session.Manager {
	return r.sessionManager
}

// RegisterPlugin 注册插件
func (r *Router) RegisterPlugin(plugin *types.Plugin) error {
	// 编译正则表达式
	regex, err := regexp.Compile(plugin.Trigger)
	if err != nil {
		return err
	}
	plugin.TriggerRegex = regex

	r.mu.Lock()
	r.plugins[plugin.ID] = plugin
	r.mu.Unlock()

	log.Printf("Plugin registered: %s (trigger: %s)", plugin.Name, plugin.Trigger)
	return nil
}

// UnregisterPlugin 注销插件
func (r *Router) UnregisterPlugin(pluginID string) {
	r.mu.Lock()
	delete(r.plugins, pluginID)
	r.mu.Unlock()

	log.Printf("Plugin unregistered: %s", pluginID)
}

// HandleMessage 处理消息
func (r *Router) HandleMessage(msg *types.Message) {
	// 1. 优先检查是否有等待会话（listen）
	if r.sessionManager.HandleMessage(msg.UserID, msg.GroupID, msg.Content) {
		log.Printf("Message intercepted by waiting session: user=%s, content=%s", msg.UserID, msg.Content)
		return
	}

	// 2. 正常匹配插件
	matchedPlugins := r.matchPlugins(msg)

	if len(matchedPlugins) == 0 {
		log.Printf("No plugin matched for message: %s", msg.Content)
		return
	}

	// 3. 只执行第一个匹配的插件
	r.callPlugin(matchedPlugins[0], msg)
}

// matchPlugins 匹配插件（只返回第一个匹配的）
func (r *Router) matchPlugins(msg *types.Message) []*types.Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, plugin := range r.plugins {
		// 检查插件是否启用
		if !plugin.Enabled {
			continue
		}

		// 检查平台支持
		if !r.supportsPlatform(plugin, msg.Platform) {
			continue
		}

		// 正则匹配
		if plugin.TriggerRegex.MatchString(msg.Content) {
			log.Printf("[SYSTEM] Plugin matched: %s for message: %s", plugin.Name, msg.Content)
			return []*types.Plugin{plugin}
		}
	}

	return nil
}

// supportsPlatform 检查插件是否支持该平台
func (r *Router) supportsPlatform(plugin *types.Plugin, platform string) bool {
	if len(plugin.Platforms) == 0 {
		return true // 未指定平台，支持所有平台
	}

	for _, p := range plugin.Platforms {
		if p == platform {
			return true
		}
	}
	return false
}

// callPlugin 调用插件（流式协议模式）
func (r *Router) callPlugin(plugin *types.Plugin, msg *types.Message) {
	// 检查插件是否启用
	if !plugin.Enabled {
		log.Printf("Plugin %s is disabled, skipping", plugin.Name)
		return
	}

	// 获取插件管理器
	if r.pluginManager == nil {
		log.Printf("Plugin manager not set")
		return
	}

	// 构建消息JSON
	messageJSON, err := json.Marshal(map[string]interface{}{
		"plugin_id":  plugin.ID,
		"platform":   msg.Platform,
		"user_id":    msg.UserID,
		"group_id":   msg.GroupID,
		"content":    msg.Content,
		"message_id": msg.ID,
		"metadata":   msg.Metadata,
	})
	if err != nil {
		log.Printf("Failed to marshal message: %v", err)
		return
	}

	// 确定发送目标
	target := msg.UserID
	if msg.GroupID != "" {
		target = msg.GroupID
	}

	// 对于Telegram，优先使用Metadata中的chat_id
	if msg.Platform == "telegram" {
		if chatID, ok := msg.Metadata["chat_id"]; ok && chatID != "" {
			target = chatID
		}
	}

	// 获取适配器
	r.mu.RLock()
	adp, ok := r.adapters[msg.Platform]
	r.mu.RUnlock()

	if !ok {
		log.Printf("Adapter not found for platform: %s", msg.Platform)
		return
	}

	// 回复回调：立即发送消息给用户
	replyFunc := func(text string) error {
		return adp.SendMessage(target, text)
	}

	// listen 回调：创建等待会话，等待用户输入
	listenFunc := func(timeout int) string {
		ch := r.sessionManager.CreateSession(plugin.ID, msg.UserID, msg.GroupID, timeout)
		content, ok := <-ch
		if !ok {
			return "" // 超时或通道关闭
		}
		return content
	}

	// 直接执行插件
	pluginPath := filepath.Join("plugins", plugin.ID)
	if err := r.pluginManager.ExecutePlugin(plugin, pluginPath, messageJSON, replyFunc, listenFunc); err != nil {
		log.Printf("Failed to execute plugin %s: %v", plugin.Name, err)
	}
}

// GetPlugins 获取所有插件
func (r *Router) GetPlugins() []*types.Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugins := make([]*types.Plugin, 0, len(r.plugins))
	for _, plugin := range r.plugins {
		plugins = append(plugins, plugin)
	}
	return plugins
}

// GetPlugin 获取指定插件
func (r *Router) GetPlugin(pluginID string) *types.Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.plugins[pluginID]
}
