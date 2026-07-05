package builtin

import (
	"testing"

	"github.com/allbot/allbot/core/adapter/_registry"
)

func TestBuiltinAdaptersRegistered(t *testing.T) {
	tests := []struct {
		platform     string
		displayName  string
		markdown     bool
		mixedContent bool
	}{
		{platform: "dingtalk", displayName: "钉钉机器人（Stream）", markdown: true, mixedContent: true},
		{platform: "feishu", displayName: "飞书机器人", markdown: true, mixedContent: true},
		{platform: "qq", displayName: "QQ", mixedContent: true},
		{platform: "qq_office", displayName: "QQ 官方机器人", markdown: true, mixedContent: true},
		{platform: "telegram", displayName: "Telegram", markdown: true, mixedContent: true},
		{platform: "web", displayName: "Web 聊天室", markdown: true, mixedContent: true},
		{platform: "wechat_official", displayName: "微信公众号"},
	}
	for _, tt := range tests {
		t.Run(tt.platform, func(t *testing.T) {
			desc, ok := registry.Get(tt.platform)
			if !ok {
				t.Fatalf("平台未注册: %s", tt.platform)
			}
			if desc.DisplayName != tt.displayName {
				t.Fatalf("DisplayName = %q, expected %q", desc.DisplayName, tt.displayName)
			}
			if desc.ParseConfig == nil || desc.NewAdapter == nil {
				t.Fatalf("平台 %s 缺少配置解析器或构造器", tt.platform)
			}
			if !desc.Capabilities.SendText || !desc.Capabilities.PrivateMessage {
				t.Fatalf("Capabilities = %+v", desc.Capabilities)
			}
			if desc.Capabilities.SendMarkdown != tt.markdown {
				t.Fatalf("SendMarkdown = %v, expected %v", desc.Capabilities.SendMarkdown, tt.markdown)
			}
			if !desc.Capabilities.SendRich {
				t.Fatalf("SendRich = false, expected true")
			}
			if desc.Capabilities.SendMixedContent != tt.mixedContent {
				t.Fatalf("SendMixedContent = %v, expected %v", desc.Capabilities.SendMixedContent, tt.mixedContent)
			}
			if len(desc.ConfigSchema) == 0 {
				t.Fatalf("平台 %s 缺少配置 schema", tt.platform)
			}
		})
	}
}
