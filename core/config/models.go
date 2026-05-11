package config

import (
	"time"
)

// AdapterConfig 平台适配器配置
type AdapterConfig struct {
	ID        int64     `json:"id"`
	Platform  string    `json:"platform"`  // qq, wechat, telegram, discord
	Enabled   bool      `json:"enabled"`   // 是否启用
	Config    string    `json:"config"`    // JSON 配置（不同平台配置不同）
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// QQConfig QQ 平台配置
type QQConfig struct {
	APIURL     string `json:"api_url"`      // go-cqhttp API 地址
	ListenAddr string `json:"listen_addr"`  // 监听地址
}

// WeChatConfig 微信平台配置
type WeChatConfig struct {
	AppID     string `json:"app_id"`
	AppSecret string `json:"app_secret"`
}

// TelegramConfig Telegram 平台配置
type TelegramConfig struct {
	BotToken  string `json:"bot_token"`
	ProxyURL  string `json:"proxy_url,omitempty"`  // 代理地址，如：http://127.0.0.1:7890 或 socks5://127.0.0.1:1080
}

