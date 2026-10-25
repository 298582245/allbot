package router

import (
	"testing"

	"github.com/allbot/allbot/core/adapter"
	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/types"
)

func TestRequireRegisteredUserStopsPromptingAfterLimit(t *testing.T) {
	db, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()
	if err := db.SetSetting("user.unregistered_prompt_limit", "2", "test"); err != nil {
		t.Fatalf("SetSetting returned error: %v", err)
	}
	fake := &keywordReplyFakeAdapter{}
	r := NewRouter(nil)
	r.SetDatabase(db)
	r.SetAdapters(map[string]adapter.Adapter{"qq": fake})
	msg := &types.Message{Platform: "qq", UserID: "user-1", Content: "hello"}
	for i := 0; i < 3; i++ {
		if r.requireRegisteredUser(msg, db) {
			t.Fatal("unregistered user unexpectedly passed registration check")
		}
	}
	if messages := fake.sentMessages(); len(messages) != 2 {
		t.Fatalf("prompt messages = %d, expected 2", len(messages))
	}
}

func TestKeywordReplyRegistrationGuideStopsAfterLimit(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()
	if err := db.SetSetting("user.unregistered_prompt_limit", "2", "test"); err != nil {
		t.Fatalf("SetSetting returned error: %v", err)
	}
	for i := 0; i < 3; i++ {
		if !manager.Handle(&types.Message{Platform: "qq", UserID: "user-2", Content: "myid"}) {
			t.Fatal("Handle returned false")
		}
	}
	if messages := fake.sentMessages(); len(messages) != 2 {
		t.Fatalf("prompt messages = %d, expected 2", len(messages))
	}
}
