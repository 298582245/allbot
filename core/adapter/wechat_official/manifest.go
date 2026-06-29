package wechat_official

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/allbot/allbot/core/adapter/_contract"
	"github.com/allbot/allbot/core/adapter/_registry"
)

type Config struct {
	AppID        string `json:"app_id"`
	AppSecret    string `json:"app_secret"`
	Token        string `json:"token"`
	CallbackPath string `json:"callback_path,omitempty"`
	APIBaseURL   string `json:"api_base_url,omitempty"`
	TokenURL     string `json:"token_url,omitempty"`
}

func init() {
	registry.Register(registry.Descriptor{
		Platform:    platformName,
		DisplayName: "微信公众号",
		Description: "微信公众号明文模式回调与客服消息适配器",
		ConfigSchema: []registry.ConfigField{
			{Key: "app_id", Label: "AppID", Type: "text", Required: true, Help: "微信公众号 AppID"},
			{Key: "app_secret", Label: "AppSecret", Type: "password", Required: true, Help: "微信公众号 AppSecret"},
			{Key: "token", Label: "服务器 Token", Type: "password", Required: true, Help: "微信公众号后台服务器配置中的 Token"},
			{Key: "callback_path", Label: "回调路径", Type: "text", Required: false, Placeholder: wechatOfficialDefaultPath, Help: "回调 URL 最后一段路径，默认 callback"},
			{Key: "api_base_url", Label: "API 基础地址", Type: "text", Required: false, Placeholder: wechatOfficialDefaultAPIBase, Help: "一般保持默认即可"},
			{Key: "token_url", Label: "Token 地址", Type: "text", Required: false, Placeholder: wechatOfficialDefaultTokenURL, Help: "一般保持默认即可"},
		},
		Capabilities: registry.Capabilities{
			SendText:       true,
			SendImage:      false,
			SendFile:       false,
			PrivateMessage: true,
			GroupMessage:   false,
			Mention:        false,
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
	config.AppID = strings.TrimSpace(config.AppID)
	config.AppSecret = strings.TrimSpace(config.AppSecret)
	config.Token = strings.TrimSpace(config.Token)
	config.CallbackPath = normalizeCallbackPath(config.CallbackPath)
	config.APIBaseURL = strings.TrimSpace(config.APIBaseURL)
	config.TokenURL = strings.TrimSpace(config.TokenURL)
	if config.AppID == "" {
		return nil, fmt.Errorf("app_id 不能为空")
	}
	if config.AppSecret == "" {
		return nil, fmt.Errorf("app_secret 不能为空")
	}
	if config.Token == "" {
		return nil, fmt.Errorf("token 不能为空")
	}
	return &config, nil
}

func newAdapterFromRegistry(parsed interface{}) (contract.Adapter, error) {
	config, ok := parsed.(*Config)
	if !ok {
		return nil, fmt.Errorf("微信公众号配置类型错误: %T", parsed)
	}
	return NewWeChatOfficialAdapter(
		config.AppID,
		config.AppSecret,
		config.Token,
		config.CallbackPath,
		config.APIBaseURL,
		config.TokenURL,
	), nil
}
