package router

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/allbot/allbot/core/adapter"
	"github.com/allbot/allbot/core/config"
	plugincore "github.com/allbot/allbot/core/plugin"
	"github.com/allbot/allbot/core/types"
)

type failingSendAdapter struct {
	*keywordReplyFakeAdapter
	err error
}

func (a *failingSendAdapter) SendMessage(target string, text string) error {
	return a.err
}

type transientFailingSendAdapter struct {
	*keywordReplyFakeAdapter
	failed bool
}

func (a *transientFailingSendAdapter) SendMessage(target string, text string) error {
	if !a.failed {
		a.failed = true
		return errors.New("send failed")
	}
	return a.keywordReplyFakeAdapter.SendMessage(target, text)
}

type sentPluginImage struct {
	target string
	url    string
}

type recordingImageAdapter struct {
	*keywordReplyFakeAdapter
	images []sentPluginImage
	err    error
}

func (a *recordingImageAdapter) SendImage(target string, imageURL string) error {
	if a.err != nil {
		return a.err
	}
	a.images = append(a.images, sentPluginImage{target: target, url: imageURL})
	return nil
}

type prefixedSendTargetResolver struct{}

func (prefixedSendTargetResolver) SendTarget(userID string, groupID string) string {
	if groupID != "" {
		return "group_" + groupID
	}
	return "user_" + userID
}

func newSendPluginMessageTestDB(t *testing.T) (*config.Database, int64, int64) {
	t.Helper()
	db, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	first := &config.AdapterConfig{Platform: "qq", Enabled: true, Config: `{}`}
	if err := db.SaveAdapter(first); err != nil {
		t.Fatal(err)
	}
	second := &config.AdapterConfig{Platform: "qq", Enabled: true, Config: `{}`}
	if err := db.SaveAdapter(second); err != nil {
		t.Fatal(err)
	}
	return db, first.ID, second.ID
}

func TestStartedPlatformAdminsExpandsUnionIDAdmins(t *testing.T) {
	db, _, secondID := newSendPluginMessageTestDB(t)
	defer db.Close()
	account, err := db.EnsureUserAccount("qq", "admin-user")
	if err != nil {
		t.Fatal(err)
	}
	settings, err := db.GetSystemSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.PlatformAdmins = []config.PlatformAdmin{{UnionID: account.UnionID}}
	if err := db.SaveSystemSettings(settings); err != nil {
		t.Fatal(err)
	}

	r := NewRouter(nil)
	r.SetDatabase(db)
	r.SetMessageAdapterGetter(func(msg *types.Message) adapter.Adapter {
		if msg != nil && msg.Metadata != nil && msg.Metadata["adapter_id"] == strconv.FormatInt(secondID, 10) {
			return &keywordReplyFakeAdapter{}
		}
		return nil
	})
	items, err := r.startedPlatformAdmins("qq")
	if err != nil {
		t.Fatalf("startedPlatformAdmins returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one admin, got %#v", items)
	}
	if items[0]["user_id"] != "admin-user" || items[0]["union_id"] != account.UnionID || items[0]["adapter_id"] != strconv.FormatInt(secondID, 10) {
		t.Fatalf("unexpected admin item: %#v", items[0])
	}
}

func TestNotifyScriptTaskTimeoutSkipsWhenDisabled(t *testing.T) {
	db, _, secondID := newSendPluginMessageTestDB(t)
	defer db.Close()
	settings, err := db.GetSystemSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.PlatformAdmins = []config.PlatformAdmin{{Platform: "qq", UserID: "admin-user"}}
	if err := db.SaveSystemSettings(settings); err != nil {
		t.Fatal(err)
	}
	startedAt := nowForRouterTest()
	logID, err := db.SaveScriptRunLog(config.ScriptRunLog{PluginID: "plugin-a", ScriptPath: "task.js", Runtime: "nodejs", RuntimeProfile: "default", RunMode: "manual", Status: "failed", Error: "timeout", StartedAt: startedAt, FinishedAt: startedAt})
	if err != nil {
		t.Fatal(err)
	}
	fake := &keywordReplyFakeAdapter{}
	r := NewRouter(nil)
	r.SetDatabase(db)
	r.SetMessageAdapterGetter(func(msg *types.Message) adapter.Adapter {
		if msg != nil && msg.Metadata != nil && msg.Metadata["adapter_id"] == strconv.FormatInt(secondID, 10) {
			return fake
		}
		return nil
	})

	r.NotifyScriptTaskTimeout(plugincore.ScriptTimeoutNotification{LogID: logID, TimeoutSeconds: 60, Error: "timeout", FinishedAt: startedAt})
	if len(fake.sentMessages()) != 0 {
		t.Fatalf("disabled notification should not send messages: %#v", fake.sentMessages())
	}
}

func TestNotifyScriptTaskTimeoutSendsOneReachableAdmin(t *testing.T) {
	db, firstID, secondID := newSendPluginMessageTestDB(t)
	defer db.Close()
	settings, err := db.GetSystemSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.PlatformAdmins = []config.PlatformAdmin{{Platform: "qq", UserID: "admin-1"}, {Platform: "qq", UserID: "admin-2"}}
	if err := db.SaveSystemSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveScriptTaskSettings(config.ScriptTaskSettings{TimeoutNotifyAdminEnabled: true}); err != nil {
		t.Fatal(err)
	}
	startedAt := nowForRouterTest()
	logID, err := db.SaveScriptRunLog(config.ScriptRunLog{PluginID: "plugin-a", ScriptPath: "task.js", Runtime: "nodejs", RuntimeProfile: "default", RunMode: "manual", Status: "failed", Error: "脚本任务运行超过 60 秒，已自动停止", StartedAt: startedAt, FinishedAt: startedAt})
	if err != nil {
		t.Fatal(err)
	}
	second := &transientFailingSendAdapter{keywordReplyFakeAdapter: &keywordReplyFakeAdapter{}}
	r := NewRouter(nil)
	r.SetDatabase(db)
	r.SetMessageAdapterGetter(func(msg *types.Message) adapter.Adapter {
		if msg == nil || msg.Metadata == nil {
			return nil
		}
		if msg.Metadata["adapter_id"] == strconv.FormatInt(firstID, 10) || msg.Metadata["adapter_id"] == strconv.FormatInt(secondID, 10) {
			return second
		}
		return nil
	})

	r.NotifyScriptTaskTimeout(plugincore.ScriptTimeoutNotification{LogID: logID, TimeoutSeconds: 60, Error: "脚本任务运行超过 60 秒，已自动停止", FinishedAt: startedAt})
	if len(second.sentMessages()) != 1 {
		t.Fatalf("expected one successful notification, got %#v", second.sentMessages())
	}
	message := second.sentMessages()[0]
	if message.target != "admin-2" || !strings.Contains(message.text, "脚本任务运行超时") || !strings.Contains(message.text, "任务ID") {
		t.Fatalf("unexpected notification message: %#v", message)
	}
}

func nowForRouterTest() time.Time {
	return time.Date(2026, 7, 7, 12, 0, 0, 0, time.Local)
}

func TestRouterSendPluginMessageChoosesFirstRunningAdapterByDatabaseOrder(t *testing.T) {
	db, firstID, secondID := newSendPluginMessageTestDB(t)
	defer db.Close()
	fake := &keywordReplyFakeAdapter{}
	r := NewRouter(nil)
	r.SetDatabase(db)
	r.SetMessageAdapterGetter(func(msg *types.Message) adapter.Adapter {
		if msg != nil && msg.AdapterID == strconv.FormatInt(secondID, 10) {
			return fake
		}
		return nil
	})

	if err := r.sendPluginMessage("plugin", plugincore.SendMessageAction{Platform: "qq", UserID: "u1", Text: "hello"}); err != nil {
		t.Fatalf("sendPluginMessage returned error: %v", err)
	}
	if len(fake.sentMessages()) != 1 {
		t.Fatalf("expected one message through second adapter, got %#v", fake.sentMessages())
	}
	if firstID >= secondID {
		t.Fatalf("test setup expected increasing IDs, got first=%d second=%d", firstID, secondID)
	}
}

func TestRouterSendPluginMessageReturnsErrorWhenNoRunningAdapter(t *testing.T) {
	db, _, _ := newSendPluginMessageTestDB(t)
	defer db.Close()
	r := NewRouter(nil)
	r.SetDatabase(db)
	r.SetMessageAdapterGetter(func(msg *types.Message) adapter.Adapter { return nil })

	err := r.sendPluginMessage("plugin", plugincore.SendMessageAction{Platform: "qq", UserID: "u1", Text: "hello"})
	if err == nil || !strings.Contains(err.Error(), "平台没有运行中的适配器") {
		t.Fatalf("error = %v", err)
	}
}

func TestRouterSendPluginMessageExplicitStoppedAdapterDoesNotFallback(t *testing.T) {
	db, firstID, secondID := newSendPluginMessageTestDB(t)
	defer db.Close()
	fake := &keywordReplyFakeAdapter{}
	r := NewRouter(nil)
	r.SetDatabase(db)
	r.SetMessageAdapterGetter(func(msg *types.Message) adapter.Adapter {
		if msg != nil && msg.AdapterID == strconv.FormatInt(secondID, 10) {
			return fake
		}
		return nil
	})

	err := r.sendPluginMessage("plugin", plugincore.SendMessageAction{Platform: "qq", AdapterID: strconv.FormatInt(firstID, 10), UserID: "u1", Text: "hello"})
	if err == nil || !strings.Contains(err.Error(), "适配器未运行") {
		t.Fatalf("error = %v", err)
	}
	if len(fake.sentMessages()) != 0 {
		t.Fatalf("explicit stopped adapter fell back and sent messages: %#v", fake.sentMessages())
	}
}

func TestRouterSendPluginMessageUnionWithPlatformFallsBackToOtherPlatform(t *testing.T) {
	db, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	qqOffice := &config.AdapterConfig{Platform: "qq_office", Enabled: true, Config: `{}`}
	if err := db.SaveAdapter(qqOffice); err != nil {
		t.Fatal(err)
	}
	telegram := &config.AdapterConfig{Platform: "telegram", Enabled: true, Config: `{}`}
	if err := db.SaveAdapter(telegram); err != nil {
		t.Fatal(err)
	}
	if _, err := db.EnsureUserAccount("qq_office", "office-user"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.BindUserByCode("telegram", "telegram-user", mustBindCode(t, db, "qq_office", "office-user")); err != nil {
		t.Fatal(err)
	}

	r := NewRouter(nil)
	r.SetDatabase(db)
	telegramFake := &keywordReplyFakeAdapter{}
	r.SetMessageAdapterGetter(func(msg *types.Message) adapter.Adapter {
		if msg == nil {
			return nil
		}
		switch msg.AdapterID {
		case strconv.FormatInt(qqOffice.ID, 10):
			return &failingSendAdapter{keywordReplyFakeAdapter: &keywordReplyFakeAdapter{}, err: errors.New("qq office failed")}
		case strconv.FormatInt(telegram.ID, 10):
			return telegramFake
		default:
			return nil
		}
	})

	err = r.sendPluginMessage("plugin", plugincore.SendMessageAction{Platform: "qq_office", AdapterID: strconv.FormatInt(qqOffice.ID, 10), UnionID: "U_qq_office_office_user", Text: "hello"})
	if err != nil {
		t.Fatalf("sendPluginMessage returned error: %v", err)
	}
	messages := telegramFake.sentMessages()
	if len(messages) != 1 || messages[0].target != "telegram-user" || messages[0].text != "hello" {
		t.Fatalf("fallback messages = %#v", messages)
	}
}

func mustBindCode(t *testing.T, db *config.Database, platform, userID string) string {
	t.Helper()
	code, err := db.CreateUserBindCode(platform, userID)
	if err != nil {
		t.Fatal(err)
	}
	return code.Code
}

func TestRouterSendPluginMessageExplicitRunningAdapterSends(t *testing.T) {
	db, _, secondID := newSendPluginMessageTestDB(t)
	defer db.Close()
	fake := &keywordReplyFakeAdapter{}
	r := NewRouter(nil)
	r.SetDatabase(db)
	r.SetMessageAdapterGetter(func(msg *types.Message) adapter.Adapter {
		if msg != nil && msg.AdapterID == strconv.FormatInt(secondID, 10) {
			return fake
		}
		return nil
	})

	if err := r.sendPluginMessage("plugin", plugincore.SendMessageAction{Platform: "qq", AdapterID: strconv.FormatInt(secondID, 10), UserID: "u1", Text: "hello"}); err != nil {
		t.Fatalf("sendPluginMessage returned error: %v", err)
	}
	messages := fake.sentMessages()
	if len(messages) != 1 || messages[0].target != "u1" || messages[0].text != "hello" {
		t.Fatalf("messages = %#v", messages)
	}
}

func TestSendPluginImageMessageValidatesLikeOtherProactiveMessages(t *testing.T) {
	r := NewRouter(nil)
	tests := []struct {
		name   string
		action plugincore.ImageMessageAction
		want   string
	}{
		{name: "empty image", action: plugincore.ImageMessageAction{Platform: "qq", UserID: "u1"}, want: "图片地址不能为空"},
		{name: "empty platform", action: plugincore.ImageMessageAction{UserID: "u1", URL: "https://example.com/a.png"}, want: "平台不能为空"},
		{name: "empty target", action: plugincore.ImageMessageAction{Platform: "qq", URL: "https://example.com/a.png"}, want: "用户 ID 和群组 ID 不能同时为空"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := r.SendPluginImageMessage("plugin", test.action)
			if result.Success || !strings.Contains(result.Error, test.want) {
				t.Fatalf("result = %#v", result)
			}
		})
	}
}

func TestSendPluginImageMessageUsesResolvedGroupTargetBeforeUnion(t *testing.T) {
	fake := &recordingImageAdapter{keywordReplyFakeAdapter: &keywordReplyFakeAdapter{sendResolver: prefixedSendTargetResolver{}}}
	r := NewRouter(nil)
	r.SetAdapters(map[string]adapter.Adapter{"qq": fake})

	result := r.SendPluginImageMessage("plugin", plugincore.ImageMessageAction{
		Platform: "qq",
		UserID:   "u1",
		GroupID:  "g1",
		UnionID:  "missing-union",
		URL:      " https://example.com/group.png ",
	})
	if !result.Success {
		t.Fatalf("SendPluginImageMessage failed: %#v", result)
	}
	if len(fake.images) != 1 || fake.images[0].target != "group_g1" || fake.images[0].url != "https://example.com/group.png" {
		t.Fatalf("images = %#v", fake.images)
	}
}

func TestSendPluginImageMessageUsesAdapterIDWithoutPlatform(t *testing.T) {
	db, _, adapterID := newSendPluginMessageTestDB(t)
	defer db.Close()
	fake := &recordingImageAdapter{keywordReplyFakeAdapter: &keywordReplyFakeAdapter{sendResolver: prefixedSendTargetResolver{}}}
	r := NewRouter(nil)
	r.SetDatabase(db)
	r.SetMessageAdapterGetter(func(msg *types.Message) adapter.Adapter {
		if msg != nil && msg.AdapterID == strconv.FormatInt(adapterID, 10) {
			return fake
		}
		return nil
	})

	result := r.SendPluginImageMessage("plugin", plugincore.ImageMessageAction{
		AdapterID: strconv.FormatInt(adapterID, 10),
		UserID:    "u1",
		URL:       "https://example.com/image.png",
	})
	if !result.Success {
		t.Fatalf("SendPluginImageMessage failed: %#v", result)
	}
	if len(fake.images) != 1 || fake.images[0].target != "user_u1" {
		t.Fatalf("images = %#v", fake.images)
	}
}

func TestSendPluginImageMessageUnionRejectsDisabledExplicitAdapter(t *testing.T) {
	db, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	disabled := &config.AdapterConfig{Platform: "qq", Enabled: false, Config: `{}`}
	if err := db.SaveAdapter(disabled); err != nil {
		t.Fatal(err)
	}
	fallbackConfig := &config.AdapterConfig{Platform: "telegram", Enabled: true, Config: `{}`}
	if err := db.SaveAdapter(fallbackConfig); err != nil {
		t.Fatal(err)
	}
	account, err := db.EnsureUserAccount("qq", "qq-user")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.BindUserByCode("telegram", "telegram-user", mustBindCode(t, db, "qq", "qq-user")); err != nil {
		t.Fatal(err)
	}
	fallback := &recordingImageAdapter{keywordReplyFakeAdapter: &keywordReplyFakeAdapter{}}
	r := NewRouter(nil)
	r.SetDatabase(db)
	r.SetMessageAdapterGetter(func(msg *types.Message) adapter.Adapter {
		if msg != nil && msg.AdapterID == strconv.FormatInt(fallbackConfig.ID, 10) {
			return fallback
		}
		return nil
	})

	result := r.SendPluginImageMessage("plugin", plugincore.ImageMessageAction{
		AdapterID: strconv.FormatInt(disabled.ID, 10),
		UnionID:   account.UnionID,
		URL:       "https://example.com/private.png",
	})
	if result.Success || !strings.Contains(result.Error, "不存在或未启用") {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(fallback.images) != 0 {
		t.Fatalf("explicit disabled adapter must not fall back: %#v", fallback.images)
	}
}

func TestSendPluginImageMessageUnionFallsBackFromPreferredBinding(t *testing.T) {
	db, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	preferredConfig := &config.AdapterConfig{Platform: "qq_office", Enabled: true, Config: `{}`}
	if err := db.SaveAdapter(preferredConfig); err != nil {
		t.Fatal(err)
	}
	fallbackConfig := &config.AdapterConfig{Platform: "telegram", Enabled: true, Config: `{}`}
	if err := db.SaveAdapter(fallbackConfig); err != nil {
		t.Fatal(err)
	}
	preferredAccount, err := db.EnsureUserAccount("qq_office", "office-user")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.BindUserByCode("telegram", "telegram-user", mustBindCode(t, db, "qq_office", "office-user")); err != nil {
		t.Fatal(err)
	}

	preferred := &recordingImageAdapter{keywordReplyFakeAdapter: &keywordReplyFakeAdapter{}, err: errors.New("preferred failed")}
	fallback := &recordingImageAdapter{keywordReplyFakeAdapter: &keywordReplyFakeAdapter{}}
	r := NewRouter(nil)
	r.SetDatabase(db)
	r.SetMessageAdapterGetter(func(msg *types.Message) adapter.Adapter {
		if msg == nil {
			return nil
		}
		switch msg.AdapterID {
		case strconv.FormatInt(preferredConfig.ID, 10):
			return preferred
		case strconv.FormatInt(fallbackConfig.ID, 10):
			return fallback
		default:
			return nil
		}
	})

	result := r.SendPluginImageMessage("plugin", plugincore.ImageMessageAction{
		Platform:  "qq_office",
		AdapterID: strconv.FormatInt(preferredConfig.ID, 10),
		UnionID:   preferredAccount.UnionID,
		URL:       "https://example.com/private.png",
	})
	if !result.Success {
		t.Fatalf("SendPluginImageMessage failed: %#v", result)
	}
	if len(preferred.images) != 0 {
		t.Fatalf("preferred adapter unexpectedly recorded images: %#v", preferred.images)
	}
	if len(fallback.images) != 1 || fallback.images[0].target != "telegram-user" || fallback.images[0].url != "https://example.com/private.png" {
		t.Fatalf("fallback images = %#v", fallback.images)
	}
}
