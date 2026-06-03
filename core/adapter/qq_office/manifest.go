package qq_office

import (
	"fmt"

	"github.com/allbot/allbot/core/adapter/_contract"
	"github.com/allbot/allbot/core/adapter/_registry"
	"github.com/allbot/allbot/core/config"
)

func init() {
	registry.Register(registry.Descriptor{
		Platform:    "qq_office",
		DisplayName: "QQ 官方机器人",
		Description: "腾讯 QQ 官方机器人适配器",
		ConfigSchema: []registry.ConfigField{
			{Key: "app_id", Label: "App ID", Type: "text", Required: true, Help: "QQ 开放平台机器人 App ID"},
			{Key: "client_secret", Label: "Client Secret", Type: "password", Required: true, Help: "QQ 开放平台机器人 Client Secret"},
			{Key: "api_base_url", Label: "API 基础地址", Type: "text", Required: false, Placeholder: "https://api.sgroup.qq.com", Help: "一般保持默认即可"},
			{Key: "token_url", Label: "Token 地址", Type: "text", Required: false, Placeholder: "https://bots.qq.com/app/getAppAccessToken", Help: "一般保持默认即可"},
		},
		Capabilities: registry.Capabilities{
			SendText:       true,
			SendImage:      true,
			PrivateMessage: true,
			GroupMessage:   true,
		},
		ParseConfig: parseConfigForRegistry,
		NewAdapter:  newAdapterFromRegistry,
	})
}

func parseConfigForRegistry(raw string) (interface{}, error) {
	return config.ParseQQOfficeConfig(raw)
}

func newAdapterFromRegistry(parsed interface{}) (contract.Adapter, error) {
	qqOfficeConfig, ok := parsed.(*config.QQOfficeConfig)
	if !ok {
		return nil, fmt.Errorf("QQ 官方机器人配置类型错误: %T", parsed)
	}
	return NewQQOfficeAdapter(
		qqOfficeConfig.AppID,
		qqOfficeConfig.ClientSecret,
		qqOfficeConfig.APIBaseURL,
		qqOfficeConfig.TokenURL,
	), nil
}
