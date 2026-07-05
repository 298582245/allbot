package web

import (
	"encoding/json"
	"fmt"

	"github.com/allbot/allbot/core/adapter/_contract"
	"github.com/allbot/allbot/core/adapter/_registry"
)

func init() {
	registry.Register(registry.Descriptor{
		Platform:    platformName,
		DisplayName: "Web 聊天室",
		Description: "网页聊天室适配器，支持浏览器用户通过 router 调用插件",
		ConfigSchema: []registry.ConfigField{
			{Key: "smtp_host", Label: "SMTP 主机", Type: "text", Required: true, Help: "用于发送 Web 聊天室邮箱验证码的 SMTP 主机"},
			{Key: "smtp_port", Label: "SMTP 端口", Type: "text", Required: true, Help: "SMTP 服务端口，例如 465、587 或 25"},
			{Key: "smtp_username", Label: "SMTP 用户名", Type: "text", Required: true, Help: "SMTP 登录用户名"},
			{Key: "smtp_password", Label: "SMTP 密码", Type: "password", Required: true, Help: "SMTP 登录密码或授权码"},
			{Key: "smtp_from", Label: "发件人", Type: "text", Required: true, Help: "验证码邮件发件人地址"},
			{Key: "smtp_subject", Label: "验证码邮件标题", Type: "text", Required: false, Default: DefaultSMTPSubject, Help: "留空时使用默认标题"},
		},
		Capabilities: registry.Capabilities{SendText: true, SendImage: true, SendFile: true, SendMarkdown: true, SendRich: true, SendMixedContent: true, PrivateMessage: true},
		ParseConfig:  parseConfigForRegistry,
		NewAdapter:   newAdapterFromRegistry,
	})
}

func parseConfigForRegistry(raw string) (interface{}, error) {
	var cfg Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, err
	}
	cfg = normalizeConfig(cfg)
	if cfg.SMTPHost == "" {
		return nil, fmt.Errorf("smtp_host 不能为空")
	}
	if cfg.SMTPPort == "" {
		return nil, fmt.Errorf("smtp_port 不能为空")
	}
	if cfg.SMTPUsername == "" {
		return nil, fmt.Errorf("smtp_username 不能为空")
	}
	if cfg.SMTPPassword == "" {
		return nil, fmt.Errorf("smtp_password 不能为空")
	}
	if cfg.SMTPFrom == "" {
		return nil, fmt.Errorf("smtp_from 不能为空")
	}
	return &cfg, nil
}

func newAdapterFromRegistry(parsed interface{}) (contract.Adapter, error) {
	config, ok := parsed.(*Config)
	if !ok {
		return nil, fmt.Errorf("Web 聊天室配置类型错误: %T", parsed)
	}
	return NewAdapterWithConfig(config), nil
}
