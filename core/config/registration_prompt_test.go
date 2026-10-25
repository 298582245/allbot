package config

import "testing"

func TestConsumeUnregisteredPromptHonorsConfiguredLimit(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()

	limit, err := db.GetUnregisteredPromptLimit()
	if err != nil {
		t.Fatalf("GetUnregisteredPromptLimit returned error: %v", err)
	}
	if limit != 10 {
		t.Fatalf("default prompt limit = %d, expected 10", limit)
	}
	if err := db.SetSetting(unregisteredPromptLimitKey, "2", unregisteredPromptLimitDescription); err != nil {
		t.Fatalf("SetSetting returned error: %v", err)
	}
	for i := 0; i < 2; i++ {
		allowed, err := db.ConsumeUnregisteredPrompt("qq", "user-1")
		if err != nil || !allowed {
			t.Fatalf("prompt %d allowed=%v err=%v, expected allowed", i+1, allowed, err)
		}
	}
	allowed, err := db.ConsumeUnregisteredPrompt("qq", "user-1")
	if err != nil {
		t.Fatalf("third ConsumeUnregisteredPrompt returned error: %v", err)
	}
	if allowed {
		t.Fatal("expected prompt limit to suppress the third guide")
	}
}

func TestConsumeUnregisteredPromptCanBeDisabled(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()

	if err := db.SetSetting(unregisteredPromptLimitKey, "0", unregisteredPromptLimitDescription); err != nil {
		t.Fatalf("SetSetting returned error: %v", err)
	}
	allowed, err := db.ConsumeUnregisteredPrompt("qq", "user-2")
	if err != nil {
		t.Fatalf("ConsumeUnregisteredPrompt returned error: %v", err)
	}
	if allowed {
		t.Fatal("expected disabled prompt setting to suppress the guide")
	}
}
