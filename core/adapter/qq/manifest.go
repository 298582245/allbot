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
		Description: "基于 NapCat/OneBot HTTP 的 QQ 适配器",
		ConfigSchema: []registry.ConfigField{
			{Key: "server_url", Label: "服务地址", Type: "text", Required: true, Placeholder: "http://127.0.0.1:3000", Help: "OneBot HTTP API 地址"},
			{Key: "access_token", Label: "访问令牌", Type: "password", Required: false, Help: "OneBot HTTP API 访问令牌，可留空"},
		},
		Capabilities: registry.Capabilities{
			SendText:       true,
			SendImage:      true,
			PrivateMessage: true,
			GroupMessage:   true,
			Mention:        true,
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
	return NewQQAdapter(qqConfig.ServerURL, qqConfig.AccessToken), nil
}
