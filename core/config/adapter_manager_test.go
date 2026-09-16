package config

import (
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/allbot/allbot/core/adapter"
	"github.com/allbot/allbot/core/adapter/_registry"
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

type managerHealthTestAdapter struct {
	platform string
	healthy  bool
	starts   *int32
	stops    *int32
}

func (a *managerHealthTestAdapter) GetPlatform() string                          { return a.platform }
func (a *managerHealthTestAdapter) SendMessage(target string, text string) error { return nil }
func (a *managerHealthTestAdapter) SendImage(target string, imageURL string) error {
	return nil
}
func (a *managerHealthTestAdapter) SendFile(target string, filePath string) error { return nil }
func (a *managerHealthTestAdapter) GetUserInfo(userID string) (*adapter.UserInfo, error) {
	return nil, nil
}
func (a *managerHealthTestAdapter) GetGroupInfo(groupID string) (*adapter.GroupInfo, error) {
	return nil, nil
}
func (a *managerHealthTestAdapter) AtUser(groupID string, userID string) error { return nil }
func (a *managerHealthTestAdapter) Start() error {
	atomic.AddInt32(a.starts, 1)
	a.healthy = true
	return nil
}
func (a *managerHealthTestAdapter) Stop() error {
	atomic.AddInt32(a.stops, 1)
	a.healthy = false
	return nil
}
func (a *managerHealthTestAdapter) SetMessageHandler(handler func(*types.Message)) {}
func (a *managerHealthTestAdapter) IsHealthy() bool                                { return a.healthy }

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

func TestAdapterManagerRestartUnhealthyEnabledAdapter(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	platform := "health_test_restart_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	var starts int32
	var stops int32
	registry.Register(registry.Descriptor{
		Platform:    platform,
		DisplayName: "Health Test",
		ParseConfig: func(raw string) (interface{}, error) { return nil, nil },
		NewAdapter: func(config interface{}) (adapter.Adapter, error) {
			return &managerHealthTestAdapter{platform: platform, starts: &starts, stops: &stops}, nil
		},
	})

	adapterConfig := &AdapterConfig{Platform: platform, Enabled: true, Config: `{}`}
	if err := db.SaveAdapter(adapterConfig); err != nil {
		t.Fatal(err)
	}
	manager := NewAdapterManager(db)
	manager.adapters[adapterConfig.ID] = &managerHealthTestAdapter{platform: platform, healthy: false, starts: &starts, stops: &stops}

	manager.RestartUnhealthyAdapters()

	if atomic.LoadInt32(&stops) != 1 || atomic.LoadInt32(&starts) != 1 {
		t.Fatalf("stops=%d starts=%d, expected 1/1", stops, starts)
	}
	if !manager.IsAdapterRunning(adapterConfig.ID) {
		t.Fatal("expected restarted adapter to be healthy")
	}
}
