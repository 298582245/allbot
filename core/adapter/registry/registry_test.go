package registry

import (
	"errors"
	"testing"

	"github.com/allbot/allbot/core/adapter"
	"github.com/allbot/allbot/core/types"
)

type fakeAdapter struct{}

func (fakeAdapter) GetPlatform() string                            { return "fake" }
func (fakeAdapter) SendMessage(target string, text string) error   { return nil }
func (fakeAdapter) SendImage(target string, imageURL string) error { return nil }
func (fakeAdapter) SendFile(target string, filePath string) error  { return nil }
func (fakeAdapter) GetUserInfo(userID string) (*adapter.UserInfo, error) {
	return &adapter.UserInfo{UserID: userID}, nil
}
func (fakeAdapter) GetGroupInfo(groupID string) (*adapter.GroupInfo, error) {
	return &adapter.GroupInfo{GroupID: groupID}, nil
}
func (fakeAdapter) AtUser(groupID string, userID string) error     { return nil }
func (fakeAdapter) Start() error                                   { return nil }
func (fakeAdapter) Stop() error                                    { return nil }
func (fakeAdapter) SetMessageHandler(handler func(*types.Message)) {}

func TestRegisterAndGetDescriptor(t *testing.T) {
	resetRegistryForTest()
	desc := testDescriptor("fake")
	Register(desc)

	got, ok := Get("fake")
	if !ok {
		t.Fatal("expected descriptor to be registered")
	}
	if got.Platform != "fake" || got.DisplayName != "测试平台" || !got.Capabilities.SendText {
		t.Fatalf("descriptor = %+v", got)
	}
	parsed, err := got.ParseConfig("{}")
	if err != nil || parsed != "parsed" {
		t.Fatalf("ParseConfig result = %#v, err = %v", parsed, err)
	}
	adp, err := got.NewAdapter(parsed)
	if err != nil || adp.GetPlatform() != "fake" {
		t.Fatalf("NewAdapter result = %#v, err = %v", adp, err)
	}
}

func TestGetMissingDescriptor(t *testing.T) {
	resetRegistryForTest()
	if _, ok := Get("missing"); ok {
		t.Fatal("expected missing descriptor")
	}
}

func TestRegisterRejectsInvalidDescriptor(t *testing.T) {
	tests := []struct {
		name string
		desc Descriptor
	}{
		{name: "empty platform", desc: Descriptor{ParseConfig: parseConfigForTest, NewAdapter: newAdapterForTest}},
		{name: "missing parser", desc: Descriptor{Platform: "fake", NewAdapter: newAdapterForTest}},
		{name: "missing constructor", desc: Descriptor{Platform: "fake", ParseConfig: parseConfigForTest}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetRegistryForTest()
			defer func() {
				if recover() == nil {
					t.Fatal("expected Register to panic")
				}
			}()
			Register(tt.desc)
		})
	}
}

func TestRegisterRejectsDuplicatePlatform(t *testing.T) {
	resetRegistryForTest()
	Register(testDescriptor("fake"))
	defer func() {
		if recover() == nil {
			t.Fatal("expected duplicate Register to panic")
		}
	}()
	Register(testDescriptor("fake"))
}

func TestListReturnsStableSortedCopies(t *testing.T) {
	resetRegistryForTest()
	Register(testDescriptor("telegram"))
	Register(testDescriptor("qq"))
	Register(testDescriptor("qq_office"))

	items := List()
	if len(items) != 3 {
		t.Fatalf("len(items) = %d, expected 3", len(items))
	}
	platforms := []string{items[0].Platform, items[1].Platform, items[2].Platform}
	want := []string{"qq", "qq_office", "telegram"}
	for i := range want {
		if platforms[i] != want[i] {
			t.Fatalf("platforms = %#v, expected %#v", platforms, want)
		}
	}

	items[0].ConfigSchema[0].Key = "changed"
	got, ok := Get("qq")
	if !ok {
		t.Fatal("expected qq descriptor")
	}
	if got.ConfigSchema[0].Key != "token" {
		t.Fatalf("registry descriptor was mutated: %#v", got.ConfigSchema)
	}
}

func testDescriptor(platform string) Descriptor {
	return Descriptor{
		Platform:    platform,
		DisplayName: "测试平台",
		Description: "用于注册中心测试",
		ConfigSchema: []ConfigField{
			{Key: "token", Label: "Token", Type: "password", Required: true},
		},
		Capabilities: Capabilities{SendText: true, PrivateMessage: true},
		ParseConfig:  parseConfigForTest,
		NewAdapter:   newAdapterForTest,
	}
}

func parseConfigForTest(raw string) (interface{}, error) {
	if raw == "error" {
		return nil, errors.New("parse error")
	}
	return "parsed", nil
}

func newAdapterForTest(config interface{}) (adapter.Adapter, error) {
	return fakeAdapter{}, nil
}

func resetRegistryForTest() {
	registryMu.Lock()
	defer registryMu.Unlock()
	descriptors = make(map[string]Descriptor)
}
