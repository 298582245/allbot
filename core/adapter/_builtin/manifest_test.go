package builtin

import (
	"testing"

	"github.com/allbot/allbot/core/adapter/_registry"
)

func TestBuiltinAdaptersRegistered(t *testing.T) {
	tests := []struct {
		platform    string
		displayName string
	}{
		{platform: "qq", displayName: "QQ"},
		{platform: "qq_office", displayName: "QQ 官方机器人"},
		{platform: "telegram", displayName: "Telegram"},
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
			if len(desc.ConfigSchema) == 0 {
				t.Fatalf("平台 %s 缺少配置 schema", tt.platform)
			}
		})
	}
}
