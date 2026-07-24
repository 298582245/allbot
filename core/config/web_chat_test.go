package config

import (
	"errors"
	"fmt"
	"testing"
)

func TestWebChatRegisterCreatesUserAccountAndSession(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()
	if err := db.CreateWebChatEmailCode("User@Example.com", "123456", WebChatEmailPurposeRegister, "127.0.0.1"); err != nil {
		t.Fatalf("CreateWebChatEmailCode returned error: %v", err)
	}
	user, err := db.RegisterWebChatUser(WebChatRegisterInput{Email: "user@example.com", Code: "123456", Username: "user_1", Password: "password123", DisplayName: "用户"})
	if err != nil {
		t.Fatalf("RegisterWebChatUser returned error: %v", err)
	}
	if user.UserID == "" || user.UnionID == "" || user.Email != "user@example.com" || !user.EmailVerified {
		t.Fatalf("unexpected user: %#v", user)
	}
	account, err := db.GetUserAccount(WebChatPlatform, user.UserID)
	if err != nil {
		t.Fatalf("GetUserAccount returned error: %v", err)
	}
	if account.UnionID != user.UnionID {
		t.Fatalf("account union mismatch: %s != %s", account.UnionID, user.UnionID)
	}
	loginUser, err := db.VerifyWebChatLogin("USER@example.com", "password123")
	if err != nil {
		t.Fatalf("VerifyWebChatLogin returned error: %v", err)
	}
	session, err := db.CreateWebChatSession(loginUser.UserID, "agent", "127.0.0.1")
	if err != nil {
		t.Fatalf("CreateWebChatSession returned error: %v", err)
	}
	if session.Token == "" || session.CSRFToken == "" {
		t.Fatalf("session token missing: %#v", session)
	}
	loaded, err := db.GetWebChatSession(session.Token)
	if err != nil {
		t.Fatalf("GetWebChatSession returned error: %v", err)
	}
	if loaded.User.UserID != user.UserID {
		t.Fatalf("loaded wrong session user: %#v", loaded.User)
	}
}

func TestWebChatEmailCodeRateAndAttempts(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()
	if err := db.CreateWebChatEmailCode("a@example.com", "123456", WebChatEmailPurposeRegister, "1.1.1.1"); err != nil {
		t.Fatalf("CreateWebChatEmailCode returned error: %v", err)
	}
	if err := db.CreateWebChatEmailCode("a@example.com", "222222", WebChatEmailPurposeRegister, "1.1.1.1"); err == nil {
		t.Fatal("expected repeated code request to be limited")
	}
	if _, err := db.RegisterWebChatUser(WebChatRegisterInput{Email: "a@example.com", Code: "000000", Username: "user_2", Password: "password123"}); err == nil {
		t.Fatal("expected wrong code to fail")
	}
	user, err := db.RegisterWebChatUser(WebChatRegisterInput{Email: "a@example.com", Code: "123456", Username: "user_2", Password: "password123"})
	if err != nil {
		t.Fatalf("RegisterWebChatUser returned error: %v", err)
	}
	if user.Username != "user_2" {
		t.Fatalf("unexpected user: %#v", user)
	}
}

func TestVerifyWebChatEmailLoginConsumesLoginCode(t *testing.T) {
	db, user := newWebChatPasswordResetTestUser(t)
	if err := db.CreateWebChatEmailCode(user.Email, "654321", WebChatEmailPurposeLogin, ""); err != nil {
		t.Fatalf("CreateWebChatEmailCode login returned error: %v", err)
	}
	loginUser, err := db.VerifyWebChatEmailLogin(user.Email, "654321")
	if err != nil {
		t.Fatalf("VerifyWebChatEmailLogin returned error: %v", err)
	}
	if loginUser.UserID != user.UserID {
		t.Fatalf("unexpected login user: %#v", loginUser)
	}
	if _, err := db.VerifyWebChatEmailLogin(user.Email, "654321"); err == nil {
		t.Fatal("expected consumed login code to fail")
	}
}

func TestVerifyWebChatEmailLoginRejectsRegisterCode(t *testing.T) {
	db, user := newWebChatPasswordResetTestUser(t)
	if _, err := db.VerifyWebChatEmailLogin(user.Email, "123456"); err == nil {
		t.Fatal("expected register code to fail email login")
	}
}

func TestBindWebChatUserByCodeLocksRepeatedFailures(t *testing.T) {
	db, user := newWebChatPasswordResetTestUser(t)
	for attempt := 1; attempt <= bindMaxAttempts; attempt++ {
		_, _, err := db.BindWebChatUserByCode(user.UserID, "invalid")
		if err == nil {
			t.Fatalf("第 %d 次错误绑定码应失败", attempt)
		}
	}
	if _, _, err := db.BindWebChatUserByCode(user.UserID, "invalid"); !errors.Is(err, ErrUserBindLocked) {
		t.Fatalf("达到阈值后应锁定，实际 %v", err)
	}
}

func TestResolveWebChatPlatformLoginByUsernameFindsBoundAccount(t *testing.T) {
	db, user := newWebChatPasswordResetTestUser(t)
	bindCode, err := db.CreateUserBindCode("telegram", "tg1")
	if err != nil {
		t.Fatalf("CreateUserBindCode returned error: %v", err)
	}
	if _, _, err := db.BindWebChatUserByCode(user.UserID, bindCode.Code); err != nil {
		t.Fatalf("BindWebChatUserByCode returned error: %v", err)
	}
	account, ambiguous, err := db.ResolveWebChatPlatformLoginByUsername("reset_user", "telegram")
	if err != nil {
		t.Fatalf("ResolveWebChatPlatformLoginByUsername returned error: %v", err)
	}
	if ambiguous || account == nil || account.UserID != "tg1" {
		t.Fatalf("unexpected resolved account ambiguous=%v account=%#v", ambiguous, account)
	}
	if err := db.CreateWebChatPlatformCode("telegram", "1", account.UserID, account.UnionID, "123456", ""); err != nil {
		t.Fatalf("CreateWebChatPlatformCode returned error: %v", err)
	}
	loginUser, err := db.VerifyWebChatPlatformLogin("telegram", "1", account.UserID, "123456")
	if err != nil {
		t.Fatalf("VerifyWebChatPlatformLogin returned error: %v", err)
	}
	if loginUser.UserID != user.UserID {
		t.Fatalf("expected existing web user %s, got %s", user.UserID, loginUser.UserID)
	}
}

func TestResolveWebChatPlatformLoginByUsernameMissingAndUnbound(t *testing.T) {
	db, _ := newWebChatPasswordResetTestUser(t)
	account, ambiguous, err := db.ResolveWebChatPlatformLoginByUsername("missing_user", "telegram")
	if err != nil || ambiguous || account != nil {
		t.Fatalf("expected missing user to resolve empty, ambiguous=%v account=%#v err=%v", ambiguous, account, err)
	}
	account, ambiguous, err = db.ResolveWebChatPlatformLoginByUsername("reset_user", "telegram")
	if err != nil || ambiguous || account != nil {
		t.Fatalf("expected unbound platform to resolve empty, ambiguous=%v account=%#v err=%v", ambiguous, account, err)
	}
}

func TestResolveWebChatPlatformLoginByUsernameAmbiguous(t *testing.T) {
	db, user := newWebChatPasswordResetTestUser(t)
	bindCode, err := db.CreateUserBindCode("telegram", "tg1")
	if err != nil {
		t.Fatalf("CreateUserBindCode returned error: %v", err)
	}
	if _, _, err := db.BindWebChatUserByCode(user.UserID, bindCode.Code); err != nil {
		t.Fatalf("BindWebChatUserByCode returned error: %v", err)
	}
	webAccount, err := db.GetUserAccount(WebChatPlatform, user.UserID)
	if err != nil {
		t.Fatalf("GetUserAccount web returned error: %v", err)
	}
	second, err := db.EnsureUserAccount("telegram", "tg2")
	if err != nil {
		t.Fatalf("EnsureUserAccount second returned error: %v", err)
	}
	if _, err := db.db.Exec(`DROP TRIGGER trg_user_accounts_prevent_union_platform_duplicate_update`); err != nil {
		t.Fatalf("drop duplicate update trigger returned error: %v", err)
	}
	if _, err := db.db.Exec(`UPDATE user_accounts SET union_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, webAccount.UnionID, second.ID); err != nil {
		t.Fatalf("update second union returned error: %v", err)
	}
	account, ambiguous, err := db.ResolveWebChatPlatformLoginByUsername("reset_user", "telegram")
	if err != nil {
		t.Fatalf("ResolveWebChatPlatformLoginByUsername returned error: %v", err)
	}
	if !ambiguous || account != nil {
		t.Fatalf("expected ambiguous account, ambiguous=%v account=%#v", ambiguous, account)
	}
}

func TestWebChatPlatformLoginCreatesWebUser(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()
	account, err := db.EnsureUserAccount("telegram", "tg1")
	if err != nil {
		t.Fatalf("EnsureUserAccount returned error: %v", err)
	}
	if err := db.CreateWebChatPlatformCode("telegram", "1", "tg1", account.UnionID, "123456", "1.1.1.1"); err != nil {
		t.Fatalf("CreateWebChatPlatformCode returned error: %v", err)
	}
	user, err := db.VerifyWebChatPlatformLogin("telegram", "1", "tg1", "123456")
	if err != nil {
		t.Fatalf("VerifyWebChatPlatformLogin returned error: %v", err)
	}
	if user.UserID == "" || user.UnionID != account.UnionID || user.EmailVerified {
		t.Fatalf("unexpected platform login user: %#v", user)
	}
	webAccount, err := db.GetUserAccount(WebChatPlatform, user.UserID)
	if err != nil {
		t.Fatalf("GetUserAccount web returned error: %v", err)
	}
	if webAccount.UnionID != account.UnionID {
		t.Fatalf("expected web account union %s, got %s", account.UnionID, webAccount.UnionID)
	}
}

func TestWebChatPlatformLoginReusesExistingWebUser(t *testing.T) {
	db, user := newWebChatPasswordResetTestUser(t)
	bindCode, err := db.CreateUserBindCode("telegram", "tg1")
	if err != nil {
		t.Fatalf("CreateUserBindCode returned error: %v", err)
	}
	if _, _, err := db.BindWebChatUserByCode(user.UserID, bindCode.Code); err != nil {
		t.Fatalf("BindWebChatUserByCode returned error: %v", err)
	}
	account, err := db.GetUserAccount("telegram", "tg1")
	if err != nil {
		t.Fatalf("GetUserAccount telegram returned error: %v", err)
	}
	if err := db.CreateWebChatPlatformCode("telegram", "1", "tg1", account.UnionID, "123456", ""); err != nil {
		t.Fatalf("CreateWebChatPlatformCode returned error: %v", err)
	}
	loginUser, err := db.VerifyWebChatPlatformLogin("telegram", "1", "tg1", "123456")
	if err != nil {
		t.Fatalf("VerifyWebChatPlatformLogin returned error: %v", err)
	}
	if loginUser.UserID != user.UserID {
		t.Fatalf("expected existing web user %s, got %s", user.UserID, loginUser.UserID)
	}
}

func TestWebChatPlatformCodeConsumesOnceAndTracksAttempts(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()
	account, err := db.EnsureUserAccount("telegram", "tg1")
	if err != nil {
		t.Fatalf("EnsureUserAccount returned error: %v", err)
	}
	if err := db.CreateWebChatPlatformCode("telegram", "1", "tg1", account.UnionID, "123456", ""); err != nil {
		t.Fatalf("CreateWebChatPlatformCode returned error: %v", err)
	}
	for i := 0; i < webChatMaxCodeAttempts; i++ {
		if _, err := db.VerifyWebChatPlatformLogin("telegram", "1", "tg1", "000000"); err == nil {
			t.Fatal("expected wrong platform code to fail")
		}
	}
	if _, err := db.VerifyWebChatPlatformLogin("telegram", "1", "tg1", "123456"); err == nil {
		t.Fatal("expected code to be rejected after max attempts")
	}
	if _, err := db.db.Exec(`DELETE FROM web_chat_platform_codes`); err != nil {
		t.Fatalf("cleanup platform codes returned error: %v", err)
	}
	if err := db.CreateWebChatPlatformCode("telegram", "1", "tg1", account.UnionID, "654321", ""); err != nil {
		t.Fatalf("CreateWebChatPlatformCode second returned error: %v", err)
	}
	if _, err := db.VerifyWebChatPlatformLogin("telegram", "1", "tg1", "654321"); err != nil {
		t.Fatalf("expected platform code to login: %v", err)
	}
	if _, err := db.VerifyWebChatPlatformLogin("telegram", "1", "tg1", "654321"); err == nil {
		t.Fatal("expected consumed platform code to fail")
	}
}

func TestWebChatPlatformCodeRateLimit(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()
	account, err := db.EnsureUserAccount("telegram", "tg1")
	if err != nil {
		t.Fatalf("EnsureUserAccount returned error: %v", err)
	}
	if err := db.CreateWebChatPlatformCode("telegram", "1", "tg1", account.UnionID, "123456", "1.1.1.1"); err != nil {
		t.Fatalf("CreateWebChatPlatformCode returned error: %v", err)
	}
	if err := db.CreateWebChatPlatformCode("telegram", "1", "tg1", account.UnionID, "222222", "1.1.1.1"); err == nil {
		t.Fatal("expected platform code target rate limit")
	}
}

func TestResetWebChatUserPasswordConsumesResetCode(t *testing.T) {
	db, user := newWebChatPasswordResetTestUser(t)
	if err := db.CreateWebChatEmailCode(user.Email, "654321", WebChatEmailPurposeResetPassword, ""); err != nil {
		t.Fatalf("CreateWebChatEmailCode reset returned error: %v", err)
	}
	if err := db.ResetWebChatUserPassword(user.Email, "654321", "newpassword123"); err != nil {
		t.Fatalf("ResetWebChatUserPassword returned error: %v", err)
	}
	if _, err := db.VerifyWebChatLogin(user.Email, "oldpassword123"); err == nil {
		t.Fatal("expected old password to fail")
	}
	if _, err := db.VerifyWebChatLogin(user.Email, "newpassword123"); err != nil {
		t.Fatalf("expected new password to login: %v", err)
	}
}

func TestResetWebChatUserPasswordRejectsRegisterCode(t *testing.T) {
	db, user := newWebChatPasswordResetTestUser(t)
	if err := db.ResetWebChatUserPassword(user.Email, "123456", "newpassword123"); err == nil {
		t.Fatal("expected register code to fail reset")
	}
	if _, err := db.VerifyWebChatLogin(user.Email, "oldpassword123"); err != nil {
		t.Fatalf("expected old password to remain valid: %v", err)
	}
}

func TestResetWebChatUserPasswordRejectsWrongCode(t *testing.T) {
	db, user := newWebChatPasswordResetTestUser(t)
	if err := db.CreateWebChatEmailCode(user.Email, "654321", WebChatEmailPurposeResetPassword, ""); err != nil {
		t.Fatalf("CreateWebChatEmailCode reset returned error: %v", err)
	}
	if err := db.ResetWebChatUserPassword(user.Email, "000000", "newpassword123"); err == nil {
		t.Fatal("expected wrong code to fail reset")
	}
	if _, err := db.VerifyWebChatLogin(user.Email, "oldpassword123"); err != nil {
		t.Fatalf("expected old password to remain valid: %v", err)
	}
}

func TestResetWebChatUserPasswordRejectsShortPasswordWithoutConsumingCode(t *testing.T) {
	db, user := newWebChatPasswordResetTestUser(t)
	if err := db.CreateWebChatEmailCode(user.Email, "654321", WebChatEmailPurposeResetPassword, ""); err != nil {
		t.Fatalf("CreateWebChatEmailCode reset returned error: %v", err)
	}
	if err := db.ResetWebChatUserPassword(user.Email, "654321", "short"); err == nil {
		t.Fatal("expected short password to fail reset")
	}
	if err := db.ResetWebChatUserPassword(user.Email, "654321", "newpassword123"); err != nil {
		t.Fatalf("expected code to remain usable after short password: %v", err)
	}
	if _, err := db.VerifyWebChatLogin(user.Email, "newpassword123"); err != nil {
		t.Fatalf("expected new password to login: %v", err)
	}
}

func TestResetWebChatUserPasswordConsumesCodeOnce(t *testing.T) {
	db, user := newWebChatPasswordResetTestUser(t)
	if err := db.CreateWebChatEmailCode(user.Email, "654321", WebChatEmailPurposeResetPassword, ""); err != nil {
		t.Fatalf("CreateWebChatEmailCode reset returned error: %v", err)
	}
	if err := db.ResetWebChatUserPassword(user.Email, "654321", "newpassword123"); err != nil {
		t.Fatalf("first ResetWebChatUserPassword returned error: %v", err)
	}
	if err := db.ResetWebChatUserPassword(user.Email, "654321", "anotherpassword123"); err == nil {
		t.Fatal("expected consumed code to fail")
	}
	if _, err := db.VerifyWebChatLogin(user.Email, "newpassword123"); err != nil {
		t.Fatalf("expected first new password to remain valid: %v", err)
	}
}

func TestWebChatBindCodeKeepsWebUnion(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()
	if err := db.CreateWebChatEmailCode("b@example.com", "123456", WebChatEmailPurposeRegister, ""); err != nil {
		t.Fatalf("CreateWebChatEmailCode returned error: %v", err)
	}
	user, err := db.RegisterWebChatUser(WebChatRegisterInput{Email: "b@example.com", Code: "123456", Username: "user_3", Password: "password123"})
	if err != nil {
		t.Fatalf("RegisterWebChatUser returned error: %v", err)
	}
	bindCode, err := db.CreateUserBindCode("telegram", "tg1")
	if err != nil {
		t.Fatalf("CreateUserBindCode returned error: %v", err)
	}
	webAccount, sourceAccount, err := db.BindWebChatUserByCode(user.UserID, bindCode.Code)
	if err != nil {
		t.Fatalf("BindWebChatUserByCode returned error: %v", err)
	}
	if webAccount.UnionID != user.UnionID || sourceAccount.UnionID != user.UnionID {
		t.Fatalf("expected source account merged to web union, web=%#v source=%#v user=%#v", webAccount, sourceAccount, user)
	}
	secondCode, err := db.CreateUserBindCode("qq", "qq1")
	if err != nil {
		t.Fatalf("CreateUserBindCode second returned error: %v", err)
	}
	if _, _, err := db.BindWebChatUserByCode(user.UserID, secondCode.Code); err == nil {
		t.Fatal("expected second web bind to fail")
	}
}

func TestRegisterWebChatUserRejectsDisabledBoundUnionWithoutConsumingCodes(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	source, err := db.EnsureUserAccount("telegram", "disabled-register")
	if err != nil {
		t.Fatal(err)
	}
	bindCode, err := db.CreateUserBindCode(source.Platform, source.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.SetUserDisabled(source.UnionID, true); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateWebChatEmailCode("disabled@example.com", "123456", WebChatEmailPurposeRegister, ""); err != nil {
		t.Fatal(err)
	}
	input := WebChatRegisterInput{Email: "disabled@example.com", Code: "123456", Username: "disabled_register", Password: "password123", BindCode: bindCode.Code}
	if _, err := db.RegisterWebChatUser(input); err == nil {
		t.Fatal("封禁身份不应完成 Web Chat 注册")
	}
	if _, err := db.SetUserDisabled(source.UnionID, false); err != nil {
		t.Fatal(err)
	}
	user, err := db.RegisterWebChatUser(input)
	if err != nil {
		t.Fatalf("解禁后原验证码和绑定码应仍可使用: %v", err)
	}
	if user.UnionID != source.UnionID {
		t.Fatalf("union_id=%s, want %s", user.UnionID, source.UnionID)
	}
}

func newWebChatPasswordResetTestUser(t *testing.T) (*Database, *WebChatUser) {
	t.Helper()
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.CreateWebChatEmailCode("reset@example.com", "123456", WebChatEmailPurposeRegister, ""); err != nil {
		t.Fatalf("CreateWebChatEmailCode register returned error: %v", err)
	}
	user, err := db.RegisterWebChatUser(WebChatRegisterInput{Email: "reset@example.com", Code: "123456", Username: "reset_user", Password: "oldpassword123"})
	if err != nil {
		t.Fatalf("RegisterWebChatUser returned error: %v", err)
	}
	return db, user
}

func TestWebChatMessageHistoryInitialLoadReturnsLatestMessages(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()
	if err := db.CreateWebChatEmailCode("latest@example.com", "123456", WebChatEmailPurposeRegister, ""); err != nil {
		t.Fatalf("CreateWebChatEmailCode returned error: %v", err)
	}
	user, err := db.RegisterWebChatUser(WebChatRegisterInput{Email: "latest@example.com", Code: "123456", Username: "user_latest", Password: "password123"})
	if err != nil {
		t.Fatalf("RegisterWebChatUser returned error: %v", err)
	}
	for i := 1; i <= 60; i++ {
		if _, err := db.SaveWebChatMessage(&WebChatMessage{UserID: user.UserID, Direction: "in", MessageType: "text", Content: fmt.Sprintf("msg-%02d", i)}); err != nil {
			t.Fatalf("SaveWebChatMessage %d returned error: %v", i, err)
		}
	}
	items, err := db.ListWebChatMessagesByPlugin(user.UserID, "", 0, 50)
	if err != nil {
		t.Fatalf("ListWebChatMessagesByPlugin returned error: %v", err)
	}
	if len(items) != 50 || items[0].Content != "msg-11" || items[49].Content != "msg-60" {
		t.Fatalf("expected latest 50 messages in asc order, got first=%q last=%q len=%d", items[0].Content, items[len(items)-1].Content, len(items))
	}
	afterItems, err := db.ListWebChatMessagesByPlugin(user.UserID, "", items[49].MessageID, 10)
	if err != nil {
		t.Fatalf("ListWebChatMessagesByPlugin after returned error: %v", err)
	}
	if len(afterItems) != 0 {
		t.Fatalf("expected no messages after latest, got %#v", afterItems)
	}
}

func TestWebChatMessageHistoryUsesBoundParameters(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()
	if err := db.CreateWebChatEmailCode("c@example.com", "123456", WebChatEmailPurposeRegister, ""); err != nil {
		t.Fatalf("CreateWebChatEmailCode returned error: %v", err)
	}
	user, err := db.RegisterWebChatUser(WebChatRegisterInput{Email: "c@example.com", Code: "123456", Username: "user_4", Password: "password123"})
	if err != nil {
		t.Fatalf("RegisterWebChatUser returned error: %v", err)
	}
	payload := "'; DROP TABLE web_chat_users; --"
	if _, err := db.SaveWebChatMessage(&WebChatMessage{UserID: user.UserID, Direction: "in", MessageType: "text", Content: payload}); err != nil {
		t.Fatalf("SaveWebChatMessage returned error: %v", err)
	}
	items, err := db.ListWebChatMessages(user.UserID, 0, 10)
	if err != nil {
		t.Fatalf("ListWebChatMessages returned error: %v", err)
	}
	if len(items) != 1 || items[0].Content != payload {
		t.Fatalf("unexpected messages: %#v", items)
	}
	if _, err := db.SaveWebChatMessage(&WebChatMessage{UserID: user.UserID, Direction: "in", MessageType: "text", Content: "p1", PluginID: "p1"}); err != nil {
		t.Fatalf("SaveWebChatMessage p1 returned error: %v", err)
	}
	if _, err := db.SaveWebChatMessage(&WebChatMessage{UserID: user.UserID, Direction: "in", MessageType: "text", Content: "p2", PluginID: "p2"}); err != nil {
		t.Fatalf("SaveWebChatMessage p2 returned error: %v", err)
	}
	privateItems, err := db.ListWebChatMessagesByPlugin(user.UserID, "", 0, 10)
	if err != nil {
		t.Fatalf("ListWebChatMessagesByPlugin private returned error: %v", err)
	}
	if len(privateItems) != 1 || privateItems[0].PluginID != "" || privateItems[0].Content != payload {
		t.Fatalf("unexpected private messages: %#v", privateItems)
	}
	pluginItems, err := db.ListWebChatMessagesByPlugin(user.UserID, "p1", 0, 10)
	if err != nil {
		t.Fatalf("ListWebChatMessagesByPlugin returned error: %v", err)
	}
	if len(pluginItems) != 1 || pluginItems[0].PluginID != "p1" || pluginItems[0].Content != "p1" {
		t.Fatalf("unexpected plugin messages: %#v", pluginItems)
	}
	counts, err := db.CountWebChatMessagesByPlugin(user.UserID)
	if err != nil {
		t.Fatalf("CountWebChatMessagesByPlugin returned error: %v", err)
	}
	countByPlugin := map[string]int64{}
	for _, item := range counts {
		countByPlugin[item.PluginID] = item.Count
	}
	if countByPlugin[""] != 1 || countByPlugin["p1"] != 1 || countByPlugin["p2"] != 1 {
		t.Fatalf("unexpected message counts: %#v", counts)
	}
	read, err := db.MarkWebChatMessagesRead(user.UserID, "p1")
	if err != nil {
		t.Fatalf("MarkWebChatMessagesRead returned error: %v", err)
	}
	if read.UnreadCount != 0 || read.LastReadMessageID == 0 {
		t.Fatalf("unexpected read state after mark: %#v", read)
	}
	if _, err := db.SaveWebChatMessage(&WebChatMessage{UserID: user.UserID, Direction: "out", MessageType: "text", Content: "p1-new", PluginID: "p1"}); err != nil {
		t.Fatalf("SaveWebChatMessage p1-new returned error: %v", err)
	}
	read, err = db.GetWebChatMessageCount(user.UserID, "p1")
	if err != nil {
		t.Fatalf("GetWebChatMessageCount returned error: %v", err)
	}
	if read.Count != 2 || read.UnreadCount != 1 {
		t.Fatalf("expected one unread message after read mark, got %#v", read)
	}
	if _, err := db.GetWebChatUser(user.UserID); err != nil {
		t.Fatalf("web_chat_users table was affected: %v", err)
	}
}
