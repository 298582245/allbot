package router

import (
	"log"
	"regexp"
	"sync"

	"github.com/allbot/allbot/core/grpc"
	"github.com/allbot/allbot/core/plugin"
	"github.com/allbot/allbot/core/session"
	"github.com/allbot/allbot/core/types"
)

// Router 消息路由器
type Router struct {
	plugins        map[string]*types.Plugin
	pluginManager  *plugin.Manager
	sessionManager *session.Manager
	mu             sync.RWMutex
}

// NewRouter 创建消息路由器
func NewRouter(sessionManager *session.Manager) *Router {
	return &Router{
		plugins:        make(map[string]*types.Plugin),
		sessionManager: sessionManager,
	}
}

// SetPluginManager 设置插件管理器
func (r *Router) SetPluginManager(pm *plugin.Manager) {
	r.pluginManager = pm
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

	// 3. 并发调用匹配的插件
	for _, plugin := range matchedPlugins {
		go r.callPlugin(plugin, msg)
	}
}

// matchPlugins 匹配插件
func (r *Router) matchPlugins(msg *types.Message) []*types.Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matched []*types.Plugin

	for _, plugin := range r.plugins {
		// 检查平台支持
		if !r.supportsPlatform(plugin, msg.Platform) {
			continue
		}

		// 正则匹配
		if plugin.TriggerRegex.MatchString(msg.Content) {
			matched = append(matched, plugin)
			log.Printf("Plugin matched: %s for message: %s", plugin.Name, msg.Content)
		}
	}

	return matched
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

// callPlugin 调用插件（通过 HTTP）
func (r *Router) callPlugin(plugin *types.Plugin, msg *types.Message) {
	// 获取插件进程
	if r.pluginManager == nil {
		log.Printf("Plugin manager not set")
		return
	}

	process := r.pluginManager.GetPlugin(plugin.ID)
	if process == nil {
		log.Printf("Plugin process not found: %s", plugin.ID)
		return
	}

	// 创建 HTTP 客户端
	client := grpc.NewClient(process.Port)

	// 调用插件
	req := &grpc.MessageRequest{
		PluginID:  plugin.ID,
		Platform:  msg.Platform,
		UserID:    msg.UserID,
		GroupID:   msg.GroupID,
		Content:   msg.Content,
		MessageID: msg.ID,
		Metadata:  msg.Metadata,
	}

	resp, err := client.Handle(req)
	if err != nil {
		log.Printf("Failed to call plugin %s: %v", plugin.Name, err)
		return
	}

	if !resp.(*grpc.MessageResponse).Success {
		log.Printf("Plugin %s returned error: %s", plugin.Name, resp.(*grpc.MessageResponse).Error)
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
