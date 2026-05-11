package types

import "regexp"

// Message 消息结构
type Message struct {
	ID       string            // 消息ID
	Platform string            // 平台：qq/wechat/telegram
	UserID   string            // 发送者ID
	GroupID  string            // 群组ID（私聊为空）
	Content  string            // 消息内容
	Metadata map[string]string // 额外元数据
}

// Plugin 插件配置
type Plugin struct {
	ID           string   // 插件ID
	Name         string   // 插件名称
	Version      string   // 版本
	Runtime      string   // 运行时：python/nodejs
	Entry        string   // 入口文件
	Platforms    []string // 支持的平台
	Trigger      string   // 触发正则表达式
	TriggerRegex *regexp.Regexp // 编译后的正则
	Enabled      bool     // 是否启用（控制是否响应触发）
}

// PluginConfig 插件配置文件结构
type PluginConfig struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Runtime      string            `json:"runtime"`
	Entry        string            `json:"entry"`
	Platforms    []string          `json:"platforms"`
	Trigger      string            `json:"trigger"`
	Enabled      bool              `json:"enabled"`       // 是否启用，默认true
	Dependencies map[string]string `json:"dependencies"` // 依赖包: 版本
}
