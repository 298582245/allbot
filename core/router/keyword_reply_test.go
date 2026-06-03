package router

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/allbot/allbot/core/adapter"
	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/types"
)

type keywordReplyFakeAdapter struct {
	mu       sync.Mutex
	messages []sentKeywordReplyMessage
}

type sentKeywordReplyMessage struct {
	target string
	text   string
}

func (a *keywordReplyFakeAdapter) GetPlatform() string { return "qq" }

func (a *keywordReplyFakeAdapter) SendMessage(target string, text string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages = append(a.messages, sentKeywordReplyMessage{target: target, text: text})
	return nil
}

func (a *keywordReplyFakeAdapter) sentMessages() []sentKeywordReplyMessage {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]sentKeywordReplyMessage(nil), a.messages...)
}

func (a *keywordReplyFakeAdapter) SendImage(target string, imageURL string) error { return nil }
func (a *keywordReplyFakeAdapter) SendFile(target string, filePath string) error  { return nil }
func (a *keywordReplyFakeAdapter) GetUserInfo(userID string) (*adapter.UserInfo, error) {
	return nil, nil
}
func (a *keywordReplyFakeAdapter) GetGroupInfo(groupID string) (*adapter.GroupInfo, error) {
	return nil, nil
}
func (a *keywordReplyFakeAdapter) AtUser(groupID string, userID string) error     { return nil }
func (a *keywordReplyFakeAdapter) Start() error                                   { return nil }
func (a *keywordReplyFakeAdapter) Stop() error                                    { return nil }
func (a *keywordReplyFakeAdapter) SetMessageHandler(handler func(*types.Message)) {}

func newKeywordReplyTestManager(t *testing.T, fake *keywordReplyFakeAdapter, admin bool) (*config.Database, *KeywordReplyManager) {
	t.Helper()
	db, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	manager := NewKeywordReplyManager(
		db,
		func(msg *types.Message) adapter.Adapter { return fake },
		func(platform, userID string) bool { return admin },
		time.Now(),
	)
	return db, manager
}

func TestKeywordReplyRegisterExistingUserRepliesAlreadyRegistered(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()

	if !manager.Handle(&types.Message{Platform: "telegram", UserID: "7089240306", Content: "注册"}) {
		t.Fatal("first register Handle returned false")
	}
	if !manager.Handle(&types.Message{Platform: "telegram", UserID: "7089240306", Content: "注册"}) {
		t.Fatal("second register Handle returned false")
	}
	messages := fake.sentMessages()
	if len(messages) != 2 {
		t.Fatalf("messages len = %d, expected 2", len(messages))
	}
	if !strings.Contains(messages[0].text, "注册成功") {
		t.Fatalf("first message = %q, expected register success", messages[0].text)
	}
	if !strings.Contains(messages[1].text, "已注册，无需重复注册") {
		t.Fatalf("second message = %q, expected already registered tip", messages[1].text)
	}
}

func TestSystemInfoIncludesAllBotUsage(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()

	info := manager.systemInfo()
	for _, expected := range []string{"系统信息", "allBot", "内存占用：", "磁盘占用：", "%"} {
		if !strings.Contains(info, expected) {
			t.Fatalf("systemInfo missing %q: %s", expected, info)
		}
	}
}

func TestKeywordReplyRestartAdminTriggersHandler(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()

	restarted := make(chan RestartRequest, 1)
	manager.SetRestartHandler(func(request RestartRequest) error {
		restarted <- request
		return nil
	})

	msg := &types.Message{ID: "m1", Platform: "qq", AdapterID: "7", UserID: "1001", Content: "重启"}
	handled := manager.Handle(msg)
	if !handled {
		t.Fatal("Handle returned false")
	}
	messages := fake.sentMessages()
	if len(messages) != 1 {
		t.Fatalf("messages len = %d, expected 1", len(messages))
	}
	if messages[0].target != "1001" {
		t.Fatalf("target = %q, expected 1001", messages[0].target)
	}
	if !strings.Contains(messages[0].text, "AllBot 正在重启") {
		t.Fatalf("message = %q, expected restart confirmation", messages[0].text)
	}

	select {
	case request := <-restarted:
		if request.MessageKey != RestartMessageKey(msg) {
			t.Fatal("restart request should include source message key")
		}
		if request.AdapterID != "7" || request.Target != "1001" || request.UserID != "1001" {
			t.Fatalf("unexpected restart request: %+v", request)
		}
	case <-time.After(time.Second):
		t.Fatal("restart handler was not called")
	}
}

func TestKeywordReplyRestartIgnoresSourceMessageAfterProcessRestart(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()

	msg := &types.Message{ID: "same-message", Platform: "telegram", AdapterID: "3", UserID: "1001", GroupID: "2001", Content: "重启"}
	t.Setenv("ALLBOT_IGNORE_RESTART_MESSAGE_KEY", RestartMessageKey(msg))
	called := false
	manager.SetRestartHandler(func(request RestartRequest) error {
		called = true
		return nil
	})

	handled := manager.Handle(msg)
	if !handled {
		t.Fatal("Handle returned false")
	}
	if called {
		t.Fatal("restart handler should not be called for ignored restart message")
	}
	if messages := fake.sentMessages(); len(messages) != 0 {
		t.Fatalf("messages len = %d, expected 0", len(messages))
	}
}

func TestKeywordReplyRestartWithoutHandlerRepliesNotInitialized(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()

	handled := manager.Handle(&types.Message{Platform: "qq", UserID: "1001", Content: "重启"})
	if !handled {
		t.Fatal("Handle returned false")
	}
	messages := fake.sentMessages()
	if len(messages) != 1 {
		t.Fatalf("messages len = %d, expected 1", len(messages))
	}
	if !strings.Contains(messages[0].text, "重启功能未初始化") {
		t.Fatalf("message = %q, expected initialization failure", messages[0].text)
	}
}

func TestKeywordReplyRestartNonAdminIsConsumedWithoutHandler(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, false)
	defer db.Close()

	called := false
	manager.SetRestartHandler(func(request RestartRequest) error {
		called = true
		return nil
	})

	handled := manager.Handle(&types.Message{Platform: "qq", UserID: "1001", Content: "重启"})
	if !handled {
		t.Fatal("Handle returned false")
	}
	if called {
		t.Fatal("restart handler should not be called for non-admin user")
	}
	if messages := fake.sentMessages(); len(messages) != 0 {
		t.Fatalf("messages len = %d, expected 0", len(messages))
	}
}

func TestKeywordReplyRestartDuplicateRequest(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()

	block := make(chan struct{})
	manager.SetRestartHandler(func(request RestartRequest) error {
		<-block
		return nil
	})
	defer close(block)

	if !manager.Handle(&types.Message{Platform: "qq", UserID: "1001", Content: "重启"}) {
		t.Fatal("first Handle returned false")
	}
	if !manager.Handle(&types.Message{Platform: "qq", UserID: "1001", Content: "重启"}) {
		t.Fatal("second Handle returned false")
	}
	messages := fake.sentMessages()
	if len(messages) != 2 {
		t.Fatalf("messages len = %d, expected 2", len(messages))
	}
	if !strings.Contains(messages[1].text, "重启已在执行") {
		t.Fatalf("message = %q, expected duplicate restart warning", messages[1].text)
	}
}

func TestKeywordReplyRestartHandlerFailureReleasesRequest(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()

	calls := 0
	done := make(chan struct{}, 2)
	manager.SetRestartHandler(func(request RestartRequest) error {
		calls++
		done <- struct{}{}
		return errors.New("重启失败")
	})

	if !manager.Handle(&types.Message{Platform: "qq", UserID: "1001", Content: "重启"}) {
		t.Fatal("first Handle returned false")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("first restart handler was not called")
	}
	if !manager.Handle(&types.Message{Platform: "qq", UserID: "1001", Content: "重启"}) {
		t.Fatal("second Handle returned false")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("second restart handler was not called")
	}
	if calls != 2 {
		t.Fatalf("calls = %d, expected 2", calls)
	}
}

func TestKeywordReplyQQOfficeUsesReplyTarget(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()

	if !manager.Handle(&types.Message{Platform: "qq_office", UserID: "user123", Content: "version", Metadata: map[string]string{"reply_target": "dms_guild123|msg_msg456"}}) {
		t.Fatal("Handle returned false")
	}
	messages := fake.sentMessages()
	if len(messages) != 1 {
		t.Fatalf("messages len = %d, expected 1", len(messages))
	}
	if messages[0].target != "dms_guild123|msg_msg456" {
		t.Fatalf("target = %q", messages[0].target)
	}
}

func TestKeywordReplyQQOfficeUsesGuildIDFallback(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()

	if !manager.Handle(&types.Message{Platform: "qq_office", UserID: "user123", Content: "version", Metadata: map[string]string{"qq_office_guild_id": "guild123"}}) {
		t.Fatal("Handle returned false")
	}
	messages := fake.sentMessages()
	if len(messages) != 1 {
		t.Fatalf("messages len = %d, expected 1", len(messages))
	}
	if messages[0].target != "dms_guild123" {
		t.Fatalf("target = %q", messages[0].target)
	}
	if strings.Contains(messages[0].text, "[CQ:at") {
		t.Fatalf("message should not contain CQ at: %q", messages[0].text)
	}
}

func TestKeywordReplyQQOfficeUsesOpenIDFallbacks(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	db, manager := newKeywordReplyTestManager(t, fake, true)
	defer db.Close()

	if !manager.Handle(&types.Message{Platform: "qq_office", UserID: "user-openid", Content: "version", Metadata: map[string]string{"qq_office_user_openid": "user-openid"}}) {
		t.Fatal("Handle C2C returned false")
	}
	if !manager.Handle(&types.Message{Platform: "qq_office", UserID: "member-openid", GroupID: "group-openid", Content: "version", Metadata: map[string]string{"qq_office_group_openid": "group-openid"}}) {
		t.Fatal("Handle group returned false")
	}
	messages := fake.sentMessages()
	if len(messages) != 2 {
		t.Fatalf("messages len = %d, expected 2", len(messages))
	}
	if messages[0].target != "user_user-openid" {
		t.Fatalf("first target = %q", messages[0].target)
	}
	if messages[1].target != "group_group-openid" {
		t.Fatalf("second target = %q", messages[1].target)
	}
	if strings.Contains(messages[1].text, "@member-openid") {
		t.Fatalf("QQ official group reply should not add text mention: %q", messages[1].text)
	}
}
