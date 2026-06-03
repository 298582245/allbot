package config

import (
	"strings"
	"testing"
)

func TestAdapterManagerStartAdapterRejectsUnknownPlatform(t *testing.T) {
	manager := NewAdapterManager(nil)
	err := manager.startAdapter(&AdapterConfig{ID: 1, Platform: "unknown", Config: `{}`})
	if err == nil || !strings.Contains(err.Error(), "不支持的平台: unknown") {
		t.Fatalf("error = %v", err)
	}
}
