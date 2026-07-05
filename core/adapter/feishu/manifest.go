package feishu

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/allbot/allbot/core/adapter/_contract"
	"github.com/allbot/allbot/core/adapter/_registry"
)

type Config struct {
	AppID             string `json:"app_id"`
	AppSecret         string `json:"app_secret"`
	VerificationToken string `json:"verification_token"`
	EncryptKey        string `json:"encrypt_key,omitempty"`
	CallbackPath      string `json:"callback_path,omitempty"`
	APIBaseURL        string `json:"api_base_url,omitempty"`
	TokenURL          string `json:"token_url,omitempty"`
}

func init() {
	registry.Register(registry.Descriptor{
		Platform:    platformName,
		DisplayName: "飞书机器人",
		Description: "飞书长连接事件订阅与消息发送适配器",
		ConfigSchema: []registry.ConfigField{
			{Key: "app_id", Label: "App ID", Type: "text", Required: true, Help: "飞书自建应用 App ID"},
			{Key: "app_secret", Label: "App Secret", Type: "password", Required: true, Help: "飞书自建应用 App Secret"},
			{Key: "verification_token", Label: "Verification Token", Type: "password", Required: false, Help: "可选，仅 HTTP 回调兜底模式需要；长连接模式不使用"},
			{Key: "encrypt_key", Label: "Encrypt Key", Type: "password", Required: false, Help: "可选，仅 HTTP 回调兜底模式预留；长连接模式不使用"},
			{Key: "callback_path", Label: "回调路径", Type: "text", Required: false, Placeholder: feishuDefaultCallbackPath, Help: "可选 HTTP 回调兜底路径，长连接模式无需配置公网回调"},
			{Key: "api_base_url", Label: "API 基础地址", Type: "text", Required: false, Placeholder: feishuDefaultAPIBaseURL, Help: "一般保持默认即可"},
			{Key: "token_url", Label: "Token 地址", Type: "text", Required: false, Placeholder: feishuDefaultTokenURL, Help: "测试或代理场景可单独指定 token 地址"},
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
	config.AppID = strings.TrimSpace(config.AppID)
	config.AppSecret = strings.TrimSpace(config.AppSecret)
	config.VerificationToken = strings.TrimSpace(config.VerificationToken)
	config.EncryptKey = strings.TrimSpace(config.EncryptKey)
	config.CallbackPath = normalizeCallbackPath(config.CallbackPath)
	config.APIBaseURL = strings.TrimSpace(config.APIBaseURL)
	config.TokenURL = strings.TrimSpace(config.TokenURL)
	if config.AppID == "" {
		return nil, fmt.Errorf("app_id 不能为空")
	}
	if config.AppSecret == "" {
		return nil, fmt.Errorf("app_secret 不能为空")
	}
	return &config, nil
}

func newAdapterFromRegistry(parsed interface{}) (contract.Adapter, error) {
	config, ok := parsed.(*Config)
	if !ok {
		return nil, fmt.Errorf("飞书机器人配置类型错误: %T", parsed)
	}
	return NewFeishuAdapter(
		config.AppID,
		config.AppSecret,
		config.VerificationToken,
		config.EncryptKey,
		config.CallbackPath,
		config.APIBaseURL,
		config.TokenURL,
	), nil
}
