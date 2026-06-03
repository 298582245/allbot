package template

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/allbot/allbot/core/adapter/_contract"
	"github.com/allbot/allbot/core/adapter/_registry"
)

type Config struct {
	Token   string `json:"token"`
	BaseURL string `json:"base_url,omitempty"`
}

func init() {
	registry.Register(registry.Descriptor{
		Platform:    platformName,
		DisplayName: "Example 模板适配器",
		Description: "复制本目录作为新平台适配器的起点",
		ConfigSchema: []registry.ConfigField{
			{Key: "token", Label: "Token", Type: "password", Required: true, Help: "平台访问令牌"},
			{Key: "base_url", Label: "API 地址", Type: "text", Required: false, Placeholder: "https://api.example.com", Help: "可选，自定义平台 API 地址"},
		},
		Capabilities: registry.Capabilities{
			SendText:       true,
			SendImage:      false,
			SendFile:       false,
			PrivateMessage: true,
			GroupMessage:   true,
			Mention:        true,
		},
		ParseConfig: parseConfigForRegistry,
		NewAdapter:  newAdapterFromRegistry,
	})
}

func parseConfigForRegistry(raw string) (interface{}, error) {
	var config Config
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		return nil, err
	}
	config.Token = strings.TrimSpace(config.Token)
	config.BaseURL = strings.TrimSpace(config.BaseURL)
	if config.Token == "" {
		return nil, fmt.Errorf("token 不能为空")
	}
	return &config, nil
}

func newAdapterFromRegistry(parsed interface{}) (contract.Adapter, error) {
	config, ok := parsed.(*Config)
	if !ok {
		return nil, fmt.Errorf("Example 配置类型错误: %T", parsed)
	}
	return NewExampleAdapter(config.Token), nil
}
