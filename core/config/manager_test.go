package config

import (
	"encoding/json"
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

func TestAdapterManagerMergesMaskedSMTPPassword(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()
	manager := NewAdapterManager(db)
	if err := manager.SaveAdapterConfig(0, WebChatPlatform, "", "", false, map[string]interface{}{
		"smtp_host":     "smtp.example.com",
		"smtp_port":     "587",
		"smtp_username": "user",
		"smtp_password": "original-password",
		"smtp_from":     "bot@example.com",
	}); err != nil {
		t.Fatalf("SaveAdapterConfig returned error: %v", err)
	}
	adapter, err := db.GetAdapter(WebChatPlatform)
	if err != nil || adapter == nil {
		t.Fatalf("GetAdapter returned adapter=%#v err=%v", adapter, err)
	}
	if err := manager.SaveAdapterConfig(adapter.ID, WebChatPlatform, "", "", false, map[string]interface{}{
		"smtp_host":     "smtp2.example.com",
		"smtp_port":     "465",
		"smtp_username": "user2",
		"smtp_password": "****word",
		"smtp_from":     "bot2@example.com",
	}); err != nil {
		t.Fatalf("SaveAdapterConfig update returned error: %v", err)
	}
	updated, err := db.GetAdapterByID(adapter.ID)
	if err != nil {
		t.Fatalf("GetAdapterByID returned error: %v", err)
	}
	var cfg map[string]string
	if err := json.Unmarshal([]byte(updated.Config), &cfg); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if cfg["smtp_password"] != "original-password" {
		t.Fatalf("expected original password, got %#v", cfg)
	}
}

func TestAdapterManagerAllowsOnlyOneWebChatAdapter(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()
	manager := NewAdapterManager(db)
	configData := map[string]interface{}{
		"smtp_host":     "smtp.example.com",
		"smtp_port":     "587",
		"smtp_username": "user",
		"smtp_password": "pass",
		"smtp_from":     "bot@example.com",
	}
	if err := manager.SaveAdapterConfig(0, WebChatPlatform, "", "", false, configData); err != nil {
		t.Fatalf("SaveAdapterConfig first returned error: %v", err)
	}
	err = manager.SaveAdapterConfig(0, WebChatPlatform, "", "", false, configData)
	if err == nil || !strings.Contains(err.Error(), "Web 聊天室只允许创建一个实例") {
		t.Fatalf("expected unique web adapter error, got %v", err)
	}
}
