package qq

import (
	"fmt"

	"github.com/allbot/allbot/core/adapter/_contract"
	"github.com/allbot/allbot/core/adapter/_registry"
	"github.com/allbot/allbot/core/config"
)

func init() {
	registry.Register(registry.Descriptor{
		Platform:    "qq",
		DisplayName: "QQ",
		Description: "基于 OneBot 11 正向通用 WebSocket 的 QQ 第三方适配器",
		ConfigSchema: []registry.ConfigField{
			{
				Key:      "framework",
				Label:    "框架名称",
				Type:     "select",
				Required: true,
				Default:  "napcat",
				Options:  []registry.ConfigOption{{Label: "NapCat", Value: "napcat"}},
			},
			{Key: "server_url", Label: "WebSocket 地址", Type: "text", Required: true, Placeholder: "ws://127.0.0.1:3001", Help: "OneBot 11 正向通用 WebSocket 地址，支持 ws:// 和 wss://"},
			{Key: "access_token", Label: "WebSocket 访问令牌", Type: "password", Required: false, Help: "正向 WebSocket 的独立访问令牌，可留空"},
			{Key: "http_api_url", Label: "HTTP API 地址", Type: "text", Required: false, Placeholder: "http://127.0.0.1:3000", Help: "可选；填写后 action 固定走该 OneBot HTTP API，事件仍走 WebSocket"},
			{Key: "http_api_access_token", Label: "HTTP API 访问令牌", Type: "password", Required: false, Help: "HTTP API 的独立访问令牌，可留空"},
		},
		Capabilities: registry.Capabilities{
			SendText:         true,
			SendImage:        true,
			SendRich:         true,
			SendMixedContent: true,
			PrivateMessage:   true,
			GroupMessage:     true,
			Mention:          true,
		},
		ParseConfig: parseConfigForRegistry,
		NewAdapter:  newAdapterFromRegistry,
	})
}

func parseConfigForRegistry(raw string) (interface{}, error) {
	return config.ParseQQConfig(raw)
}

func newAdapterFromRegistry(parsed interface{}) (contract.Adapter, error) {
	qqConfig, ok := parsed.(*config.QQConfig)
	if !ok {
		return nil, fmt.Errorf("QQ 配置类型错误: %T", parsed)
	}
	return NewQQAdapter(QQAdapterConfig{
		Framework:          qqConfig.Framework,
		ServerURL:          qqConfig.ServerURL,
		HTTPAPIURL:         qqConfig.HTTPAPIURL,
		AccessToken:        qqConfig.AccessToken,
		HTTPAPIAccessToken: qqConfig.HTTPAPIAccessToken,
	}), nil
}
