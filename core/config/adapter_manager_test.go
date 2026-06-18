package config

import (
	"strconv"
	"testing"

	"github.com/allbot/allbot/core/adapter"
	"github.com/allbot/allbot/core/types"
)

type managerTestAdapter struct {
	platform string
}

func (a *managerTestAdapter) GetPlatform() string                                  { return a.platform }
func (a *managerTestAdapter) SendMessage(target string, text string) error         { return nil }
func (a *managerTestAdapter) SendImage(target string, imageURL string) error       { return nil }
func (a *managerTestAdapter) SendFile(target string, filePath string) error        { return nil }
func (a *managerTestAdapter) GetUserInfo(userID string) (*adapter.UserInfo, error) { return nil, nil }
func (a *managerTestAdapter) GetGroupInfo(groupID string) (*adapter.GroupInfo, error) {
	return nil, nil
}
func (a *managerTestAdapter) AtUser(groupID string, userID string) error     { return nil }
func (a *managerTestAdapter) Start() error                                   { return nil }
func (a *managerTestAdapter) Stop() error                                    { return nil }
func (a *managerTestAdapter) SetMessageHandler(handler func(*types.Message)) {}

func newAdapterManagerSelectionTest(t *testing.T) (*Database, *AdapterManager, int64, int64) {
	t.Helper()
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	first := &AdapterConfig{Platform: "qq", Enabled: true, Config: `{}`}
	if err := db.SaveAdapter(first); err != nil {
		t.Fatal(err)
	}
	second := &AdapterConfig{Platform: "qq", Enabled: true, Config: `{}`}
	if err := db.SaveAdapter(second); err != nil {
		t.Fatal(err)
	}
	manager := NewAdapterManager(db)
	manager.adapters[second.ID] = &managerTestAdapter{platform: "qq"}
	return db, manager, first.ID, second.ID
}

func TestAdapterManagerGetAdapterForMessageExplicitMissingDoesNotFallback(t *testing.T) {
	db, manager, firstID, _ := newAdapterManagerSelectionTest(t)
	defer db.Close()
	if got := manager.GetAdapterForMessage(&types.Message{Platform: "qq", AdapterID: strconv.FormatInt(firstID, 10)}); got != nil {
		t.Fatalf("expected nil for explicit missing adapter, got %#v", got)
	}
}

func TestAdapterManagerGetAdapterForMessageFallsBackOnlyWithoutExplicitAdapter(t *testing.T) {
	db, manager, _, secondID := newAdapterManagerSelectionTest(t)
	defer db.Close()
	got := manager.GetAdapterForMessage(&types.Message{Platform: "qq"})
	if got == nil || got != manager.adapters[secondID] {
		t.Fatalf("unexpected fallback adapter: %#v", got)
	}
}
