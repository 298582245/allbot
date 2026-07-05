package dingtalk

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/allbot/allbot/core/adapter/_contract"
	"github.com/allbot/allbot/core/adapter/_registry"
)

// Config 保存钉钉 Stream 机器人接入配置。
type Config struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RobotCode    string `json:"robot_code,omitempty"`
	OpenAPIHost  string `json:"open_api_host,omitempty"`
	ProxyURL     string `json:"proxy_url,omitempty"`
}

func init() {
	registry.Register(registry.Descriptor{
		Platform:    platformName,
		DisplayName: "钉钉机器人（Stream）",
		Description: "钉钉机器人 Stream 模式适配器，无需公网回调地址",
		ConfigSchema: []registry.ConfigField{
			{Key: "client_id", Label: "Client ID", Type: "text", Required: true, Help: "钉钉应用 appKey 或三方应用 suiteKey"},
			{Key: "client_secret", Label: "Client Secret", Type: "password", Required: true, Help: "钉钉应用 appSecret 或三方应用 suiteSecret"},
			{Key: "robot_code", Label: "Robot Code", Type: "text", Required: false, Help: "可选，机器人编码，仅写入消息 metadata 便于排查"},
			{Key: "open_api_host", Label: "OpenAPI Host", Type: "text", Required: false, Help: "可选，钉钉 OpenAPI 地址，一般保持默认"},
			{Key: "proxy_url", Label: "代理地址", Type: "text", Required: false, Help: "可选，企业网络代理或调试代理地址"},
		},
		Capabilities: registry.Capabilities{
			SendText:         true,
			SendImage:        true,
			SendFile:         false,
			SendMarkdown:     true,
			SendRich:         true,
			SendMixedContent: true,
			PrivateMessage:   true,
			GroupMessage:     true,
			Mention:          false,
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
	config.ClientID = strings.TrimSpace(config.ClientID)
	config.ClientSecret = strings.TrimSpace(config.ClientSecret)
	config.RobotCode = strings.TrimSpace(config.RobotCode)
	config.OpenAPIHost = strings.TrimSpace(config.OpenAPIHost)
	config.ProxyURL = strings.TrimSpace(config.ProxyURL)
	if config.ClientID == "" {
		return nil, fmt.Errorf("client_id 不能为空")
	}
	if config.ClientSecret == "" {
		return nil, fmt.Errorf("client_secret 不能为空")
	}
	return &config, nil
}

func newAdapterFromRegistry(parsed interface{}) (contract.Adapter, error) {
	config, ok := parsed.(*Config)
	if !ok {
		return nil, fmt.Errorf("钉钉机器人配置类型错误: %T", parsed)
	}
	return NewDingTalkAdapter(
		config.ClientID,
		config.ClientSecret,
		config.RobotCode,
		config.OpenAPIHost,
		config.ProxyURL,
	), nil
}
