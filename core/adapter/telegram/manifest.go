package telegram

import (
	"fmt"

	"github.com/allbot/allbot/core/adapter/_contract"
	"github.com/allbot/allbot/core/adapter/_registry"
	"github.com/allbot/allbot/core/config"
)

func init() {
	registry.Register(registry.Descriptor{
		Platform:    "telegram",
		DisplayName: "Telegram",
		Description: "Telegram Bot API 适配器",
		ConfigSchema: []registry.ConfigField{
			{Key: "bot_token", Label: "Bot Token", Type: "password", Required: true, Placeholder: "123456:ABC-DEF", Help: "Telegram BotFather 分配的 Token"},
			{Key: "proxy_url", Label: "代理地址", Type: "text", Required: false, Placeholder: "http://127.0.0.1:7890", Help: "可选 HTTP 或 SOCKS5 代理地址"},
		},
		Capabilities: registry.Capabilities{
			SendText:       true,
			SendImage:      true,
			SendFile:       true,
			PrivateMessage: true,
			GroupMessage:   true,
			Mention:        true,
		},
		ParseConfig: parseConfigForRegistry,
		NewAdapter:  newAdapterFromRegistry,
	})
}

func parseConfigForRegistry(raw string) (interface{}, error) {
	return config.ParseTelegramConfig(raw)
}

func newAdapterFromRegistry(parsed interface{}) (contract.Adapter, error) {
	telegramConfig, ok := parsed.(*config.TelegramConfig)
	if !ok {
		return nil, fmt.Errorf("Telegram 配置类型错误: %T", parsed)
	}
	return NewTelegramAdapter(telegramConfig.BotToken, telegramConfig.ProxyURL), nil
}
