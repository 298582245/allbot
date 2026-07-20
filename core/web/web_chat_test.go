package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/allbot/allbot/core/adapter/_contract"
	"github.com/allbot/allbot/core/adapter/_registry"
	webadapter "github.com/allbot/allbot/core/adapter/web"
	"github.com/allbot/allbot/core/config"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/allbot/allbot/core/plugin"
	"github.com/allbot/allbot/core/router"
	"github.com/allbot/allbot/core/session"
	"github.com/allbot/allbot/core/types"
)

type fakeWebChatMailer struct{ code string }

type webChatPlatformFakeAdapter struct {
	platform string
	target   string
	text     string
}

func (m *fakeWebChatMailer) SendWebChatCode(to, code string) error {
	m.code = code
	return nil
}

func (a *webChatPlatformFakeAdapter) GetPlatform() string { return a.platform }
func (a *webChatPlatformFakeAdapter) SendMessage(target string, text string) error {
	a.target = target
	a.text = text
	return nil
}
func (a *webChatPlatformFakeAdapter) SendImage(target string, imageURL string) error { return nil }
func (a *webChatPlatformFakeAdapter) SendFile(target string, filePath string) error  { return nil }
func (a *webChatPlatformFakeAdapter) GetUserInfo(userID string) (*contract.UserInfo, error) {
	return &contract.UserInfo{UserID: userID}, nil
}
func (a *webChatPlatformFakeAdapter) GetGroupInfo(groupID string) (*contract.GroupInfo, error) {
	return &contract.GroupInfo{GroupID: groupID}, nil
}
func (a *webChatPlatformFakeAdapter) AtUser(groupID string, userID string) error     { return nil }
func (a *webChatPlatformFakeAdapter) Start() error                                   { return nil }
func (a *webChatPlatformFakeAdapter) Stop() error                                    { return nil }
func (a *webChatPlatformFakeAdapter) SetMessageHandler(handler func(*types.Message)) {}
func (a *webChatPlatformFakeAdapter) SendTarget(userID string, groupID string) string {
	return "user_" + userID
}

const webChatTestPlatform = "webchat_test"

func init() {
	registry.Register(registry.Descriptor{
		Platform:     webChatTestPlatform,
		DisplayName:  "WebChat 测试平台",
		Description:  "WebChat 平台验证码测试适配器",
		Capabilities: registry.Capabilities{SendText: true, PrivateMessage: true},
		ParseConfig:  func(raw string) (interface{}, error) { return raw, nil },
		NewAdapter: func(config interface{}) (contract.Adapter, error) {
			return &webChatPlatformFakeAdapter{platform: webChatTestPlatform}, nil
		},
	})
}

func TestBuildWebChatCodeEmailUsesCustomSubject(t *testing.T) {
	body := buildWebChatCodeEmail("bot@example.com", "u@example.com", " 自定义标题\r\n换行 ", "123456")
	if !strings.Contains(body, "Subject: 自定义标题  换行\r\n") {
		t.Fatalf("expected sanitized custom subject, got %q", body)
	}
	if !strings.Contains(body, "您的验证码是：123456") {
		t.Fatalf("expected code in body, got %q", body)
	}
}

func TestBuildWebChatCodeEmailUsesDefaultSubjectWhenBlank(t *testing.T) {
	body := buildWebChatCodeEmail("bot@example.com", "u@example.com", "   ", "123456")
	if !strings.Contains(body, "Subject: "+webadapter.DefaultSMTPSubject+"\r\n") {
		t.Fatalf("expected default subject, got %q", body)
	}
}

func TestWebChatJSONResponsesUseUnifiedEnvelope(t *testing.T) {
	server, _ := newWebChatTestServer(t, true)

	recorder := httptest.NewRecorder()
	server.handleWebChatAPI(recorder, httptest.NewRequest(http.MethodGet, "/api/open/web-chat/platforms", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("platforms status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	assertJSONContentType(t, recorder)
	var platforms struct {
		Code int               `json:"code"`
		Msg  string            `json:"msg"`
		Data []json.RawMessage `json:"data"`
	}
	decodeResponseJSON(t, recorder, &platforms)
	if platforms.Code != http.StatusOK || platforms.Msg != "成功" || platforms.Data == nil {
		t.Fatalf("unexpected platforms response: %#v", platforms)
	}

	recorder = httptest.NewRecorder()
	server.handleWebChatAPI(recorder, httptest.NewRequest(http.MethodGet, "/api/open/web-chat/me", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("me status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	assertUnifiedErrorBody(t, recorder, http.StatusUnauthorized)

	recorder = httptest.NewRecorder()
	server.handleWebChatAPI(recorder, httptest.NewRequest(http.MethodGet, "/api/open/web-chat/missing", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("missing status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	assertUnifiedErrorBody(t, recorder, http.StatusNotFound)
}

func TestWebChatRegisterLoginAndMessages(t *testing.T) {
	server, mailer := newWebChatTestServer(t, true)
	rr := httptest.NewRecorder()
	server.handleWebChatAPI(rr, jsonRequest("/api/open/web-chat/email-code", map[string]string{"email": "u@example.com"}, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("email-code status=%d body=%s", rr.Code, rr.Body.String())
	}
	if mailer.code == "" {
		t.Fatal("expected captured code")
	}
	rr = httptest.NewRecorder()
	server.handleWebChatAPI(rr, jsonRequest("/api/open/web-chat/register", map[string]string{"email": "u@example.com", "code": mailer.code, "username": "user_1", "password": "password123"}, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("register status=%d body=%s", rr.Code, rr.Body.String())
	}
	cookie := rr.Result().Cookies()[0]
	var sessionResp config.WebChatSession
	decodeUnifiedResponseData(t, rr, &sessionResp)
	if sessionResp.CSRFToken == "" || sessionResp.User == nil {
		t.Fatalf("unexpected session: %#v", sessionResp)
	}
	registerWebChatRouterPlugin(t, server, "p1")
	rr = httptest.NewRecorder()
	req := jsonRequest("/api/open/web-chat/messages", map[string]string{"plugin_id": "p1", "type": "text", "content": "hello"}, sessionResp.CSRFToken)
	req.AddCookie(cookie)
	server.handleWebChatAPI(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("send status=%d body=%s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/open/web-chat/messages?plugin_id=p1&after_id=0&limit=10", nil)
	req.AddCookie(cookie)
	server.handleWebChatAPI(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("messages status=%d body=%s", rr.Code, rr.Body.String())
	}
	var messages []config.WebChatMessage
	decodeUnifiedResponseData(t, rr, &messages)
	if len(messages) != 1 || messages[0].Content != "hello" {
		t.Fatalf("unexpected messages: %#v", messages)
	}
}

func TestWebChatEmailLoginFlow(t *testing.T) {
	server, mailer := newWebChatTestServer(t, true)
	registerWebChatTestUser(t, server, mailer)

	rr := httptest.NewRecorder()
	server.handleWebChatAPI(rr, jsonRequest("/api/open/web-chat/email-code", map[string]string{"email": "u@example.com", "purpose": config.WebChatEmailPurposeLogin}, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("login email-code status=%d body=%s", rr.Code, rr.Body.String())
	}
	loginCode := mailer.code
	if loginCode == "" {
		t.Fatal("expected login code")
	}
	rr = httptest.NewRecorder()
	server.handleWebChatAPI(rr, jsonRequest("/api/open/web-chat/email-login", map[string]string{"email": "u@example.com", "code": loginCode}, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("email-login status=%d body=%s", rr.Code, rr.Body.String())
	}
	if len(rr.Result().Cookies()) == 0 {
		t.Fatal("expected login cookie")
	}
}

func TestWebChatEmailLoginRejectsRegisterCode(t *testing.T) {
	server, mailer := newWebChatTestServer(t, true)
	registerWebChatTestUser(t, server, mailer)

	rr := httptest.NewRecorder()
	server.handleWebChatAPI(rr, jsonRequest("/api/open/web-chat/email-code", map[string]string{"email": "another@example.com"}, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("register email-code status=%d body=%s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	server.handleWebChatAPI(rr, jsonRequest("/api/open/web-chat/email-login", map[string]string{"email": "u@example.com", "code": mailer.code}, ""))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected register code email login to fail, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestWebChatEmailLoginCodeHidesMissingEmailAndLimits(t *testing.T) {
	server, mailer := newWebChatTestServer(t, true)
	rr := httptest.NewRecorder()
	payload := map[string]string{"email": "missing@example.com", "purpose": config.WebChatEmailPurposeLogin}
	server.handleWebChatAPI(rr, jsonRequest("/api/open/web-chat/email-code", payload, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("expected missing email login code to return ok, got %d body=%s", rr.Code, rr.Body.String())
	}
	if mailer.code != "" {
		t.Fatalf("expected no email to be sent for missing account, got code %s", mailer.code)
	}
	rr = httptest.NewRecorder()
	server.handleWebChatAPI(rr, jsonRequest("/api/open/web-chat/email-code", payload, ""))
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected repeated missing email login request to be limited, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestWebChatResetPasswordFlow(t *testing.T) {
	server, mailer := newWebChatTestServer(t, true)
	registerWebChatTestUser(t, server, mailer)

	rr := httptest.NewRecorder()
	server.handleWebChatAPI(rr, jsonRequest("/api/open/web-chat/email-code", map[string]string{"email": "u@example.com", "purpose": config.WebChatEmailPurposeResetPassword}, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("reset email-code status=%d body=%s", rr.Code, rr.Body.String())
	}
	resetCode := mailer.code
	if resetCode == "" {
		t.Fatal("expected reset code")
	}
	rr = httptest.NewRecorder()
	server.handleWebChatAPI(rr, jsonRequest("/api/open/web-chat/reset-password", map[string]string{"email": "u@example.com", "code": resetCode, "password": "newpassword123"}, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("reset-password status=%d body=%s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	server.handleWebChatAPI(rr, jsonRequest("/api/open/web-chat/login", map[string]string{"login": "u@example.com", "password": "newpassword123"}, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("login new password status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestWebChatResetPasswordRejectsRegisterCode(t *testing.T) {
	server, mailer := newWebChatTestServer(t, true)
	registerWebChatTestUser(t, server, mailer)

	rr := httptest.NewRecorder()
	server.handleWebChatAPI(rr, jsonRequest("/api/open/web-chat/email-code", map[string]string{"email": "another@example.com"}, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("register email-code status=%d body=%s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	server.handleWebChatAPI(rr, jsonRequest("/api/open/web-chat/reset-password", map[string]string{"email": "u@example.com", "code": mailer.code, "password": "newpassword123"}, ""))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected register code reset to fail, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestWebChatResetPasswordEmailCodeHidesMissingEmailAndLimits(t *testing.T) {
	server, mailer := newWebChatTestServer(t, true)
	rr := httptest.NewRecorder()
	payload := map[string]string{"email": "missing@example.com", "purpose": config.WebChatEmailPurposeResetPassword}
	server.handleWebChatAPI(rr, jsonRequest("/api/open/web-chat/email-code", payload, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("expected missing email reset code to return ok, got %d body=%s", rr.Code, rr.Body.String())
	}
	if mailer.code != "" {
		t.Fatalf("expected no email to be sent for missing account, got code %s", mailer.code)
	}
	rr = httptest.NewRecorder()
	server.handleWebChatAPI(rr, jsonRequest("/api/open/web-chat/email-code", payload, ""))
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected repeated missing email request to be limited, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestWebChatEmailCodeRejectsInvalidPurpose(t *testing.T) {
	server, _ := newWebChatTestServer(t, true)
	rr := httptest.NewRecorder()
	server.handleWebChatAPI(rr, jsonRequest("/api/open/web-chat/email-code", map[string]string{"email": "u@example.com", "purpose": "unknown"}, ""))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid purpose 400, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestWebChatPlatformCodeAndLoginFlow(t *testing.T) {
	server, _ := newWebChatTestServer(t, true)
	platformAdapter, adapterID := saveRunningWebChatTestPlatformAdapter(t, server.adapterManager)
	user := createWebChatRuntimeUser(t, server.runtimeDatabase(), "platform_user", "platform@example.com")
	bindCode, err := server.runtimeDatabase().CreateUserBindCode(webChatTestPlatform, "u-platform")
	if err != nil {
		t.Fatalf("CreateUserBindCode returned error: %v", err)
	}
	if _, _, err := server.runtimeDatabase().BindWebChatUserByCode(user.UserID, bindCode.Code); err != nil {
		t.Fatalf("BindWebChatUserByCode returned error: %v", err)
	}
	account, err := server.runtimeDatabase().GetUserAccount(webChatTestPlatform, "u-platform")
	if err != nil {
		t.Fatalf("GetUserAccount returned error: %v", err)
	}
	rr := httptest.NewRecorder()
	server.handleWebChatAPI(rr, httptest.NewRequest(http.MethodGet, "/api/open/web-chat/platforms", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("platforms status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "WebChat 测试平台") || strings.Contains(rr.Body.String(), "token") {
		t.Fatalf("unexpected platforms response: %s", rr.Body.String())
	}
	rr = httptest.NewRecorder()
	server.handleWebChatAPI(rr, jsonRequest("/api/open/web-chat/platform-code", map[string]string{"adapter_id": adapterID, "platform": webChatTestPlatform, "username": "platform_user"}, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("platform-code status=%d body=%s", rr.Code, rr.Body.String())
	}
	if platformAdapter.target != "user_"+account.UserID || !strings.Contains(platformAdapter.text, "你的web登录验证码为：") {
		t.Fatalf("expected captured platform code, target=%q text=%q", platformAdapter.target, platformAdapter.text)
	}
	code := platformAdapter.text[strings.LastIndex(platformAdapter.text, "：")+len("："):]
	rr = httptest.NewRecorder()
	server.handleWebChatAPI(rr, jsonRequest("/api/open/web-chat/platform-login", map[string]string{"adapter_id": adapterID, "platform": webChatTestPlatform, "username": "platform_user", "code": code}, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("platform-login status=%d body=%s", rr.Code, rr.Body.String())
	}
	if len(rr.Result().Cookies()) == 0 {
		t.Fatal("expected platform login cookie")
	}
}

func TestWebChatPlatformCodeHidesMissingUser(t *testing.T) {
	server, _ := newWebChatTestServer(t, true)
	platformAdapter, adapterID := saveRunningWebChatTestPlatformAdapter(t, server.adapterManager)
	rr := httptest.NewRecorder()
	server.handleWebChatAPI(rr, jsonRequest("/api/open/web-chat/platform-code", map[string]string{"adapter_id": adapterID, "platform": webChatTestPlatform, "username": "missing_user"}, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("expected missing user to return ok, got %d body=%s", rr.Code, rr.Body.String())
	}
	if platformAdapter.text != "" {
		t.Fatalf("expected no platform message for missing user, got %q", platformAdapter.text)
	}
}

func TestWebChatPlatformCodeHidesUnboundUser(t *testing.T) {
	server, _ := newWebChatTestServer(t, true)
	platformAdapter, adapterID := saveRunningWebChatTestPlatformAdapter(t, server.adapterManager)
	createWebChatRuntimeUser(t, server.runtimeDatabase(), "unbound_user", "unbound@example.com")
	rr := httptest.NewRecorder()
	server.handleWebChatAPI(rr, jsonRequest("/api/open/web-chat/platform-code", map[string]string{"adapter_id": adapterID, "platform": webChatTestPlatform, "username": "unbound_user"}, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("expected unbound user to return ok, got %d body=%s", rr.Code, rr.Body.String())
	}
	if platformAdapter.text != "" {
		t.Fatalf("expected no platform message for unbound user, got %q", platformAdapter.text)
	}
}

func TestWebChatPlatformLoginRejectsWrongCode(t *testing.T) {
	server, _ := newWebChatTestServer(t, true)
	_, adapterID := saveRunningWebChatTestPlatformAdapter(t, server.adapterManager)
	user := createWebChatRuntimeUser(t, server.runtimeDatabase(), "wrong_code_user", "wrong-code@example.com")
	bindCode, err := server.runtimeDatabase().CreateUserBindCode(webChatTestPlatform, "wrong-platform")
	if err != nil {
		t.Fatalf("CreateUserBindCode returned error: %v", err)
	}
	if _, _, err := server.runtimeDatabase().BindWebChatUserByCode(user.UserID, bindCode.Code); err != nil {
		t.Fatalf("BindWebChatUserByCode returned error: %v", err)
	}
	account, err := server.runtimeDatabase().GetUserAccount(webChatTestPlatform, "wrong-platform")
	if err != nil {
		t.Fatalf("GetUserAccount returned error: %v", err)
	}
	if err := server.runtimeDatabase().CreateWebChatPlatformCode(webChatTestPlatform, adapterID, account.UserID, account.UnionID, "123456", ""); err != nil {
		t.Fatalf("CreateWebChatPlatformCode returned error: %v", err)
	}
	rr := httptest.NewRecorder()
	server.handleWebChatAPI(rr, jsonRequest("/api/open/web-chat/platform-login", map[string]string{"adapter_id": adapterID, "platform": webChatTestPlatform, "username": "wrong_code_user", "code": "000000"}, ""))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected wrong code 401, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestWebChatPlatformsExcludeWebAndDingTalk(t *testing.T) {
	server, _ := newWebChatTestServer(t, true)
	if err := server.adapterManager.SaveAdapterConfig(0, "dingtalk", "钉钉", "", false, map[string]interface{}{"client_id": "id", "client_secret": "secret", "robot_code": "robot"}); err != nil {
		t.Fatalf("SaveAdapterConfig dingtalk returned error: %v", err)
	}
	saveRunningWebChatTestPlatformAdapter(t, server.adapterManager)
	rr := httptest.NewRecorder()
	server.handleWebChatAPI(rr, httptest.NewRequest(http.MethodGet, "/api/open/web-chat/platforms", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("platforms status=%d body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if strings.Contains(body, `"platform":"web"`) || strings.Contains(body, `"platform":"dingtalk"`) || !strings.Contains(body, webChatTestPlatform) {
		t.Fatalf("unexpected platforms response: %s", body)
	}
}

func TestWebChatResetPasswordRequiresRunningAdapter(t *testing.T) {
	server, _ := newWebChatTestServer(t, false)
	rr := httptest.NewRecorder()
	server.handleWebChatAPI(rr, jsonRequest("/api/open/web-chat/reset-password", map[string]string{"email": "u@example.com", "code": "123456", "password": "newpassword123"}, ""))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), webChatAdapterUnavailableMessage) {
		t.Fatalf("expected adapter unavailable message, got %s", rr.Body.String())
	}
}

func TestWebChatWriteRequiresCSRF(t *testing.T) {
	server, mailer := newWebChatTestServer(t, true)
	sessionCookie, _ := registerWebChatTestUser(t, server, mailer)
	rr := httptest.NewRecorder()
	req := jsonRequest("/api/open/web-chat/messages", map[string]string{"type": "text", "content": "hello"}, "")
	req.AddCookie(sessionCookie)
	server.handleWebChatAPI(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestWebChatMessageLimitUsesAdapterConfig(t *testing.T) {
	server, mailer := newWebChatTestServer(t, false)
	saveRunningWebChatAdapterWithLimit(t, server.adapterManager, 1)
	cookie, csrf := registerWebChatTestUser(t, server, mailer)

	for attempt := 1; attempt <= 2; attempt++ {
		rr := httptest.NewRecorder()
		req := jsonRequest("/api/open/web-chat/messages", map[string]string{"type": "text", "content": "hello"}, csrf)
		req.AddCookie(cookie)
		server.handleWebChatAPI(rr, req)
		if attempt == 1 && rr.Code != http.StatusOK {
			t.Fatalf("first message status=%d body=%s", rr.Code, rr.Body.String())
		}
		if attempt == 2 {
			if rr.Code != http.StatusTooManyRequests {
				t.Fatalf("second message status=%d body=%s", rr.Code, rr.Body.String())
			}
			assertUnifiedErrorBody(t, rr, http.StatusTooManyRequests)
		}
	}
}

func TestWebChatUserSendOnlyAllowsText(t *testing.T) {
	server, mailer := newWebChatTestServer(t, true)
	sessionCookie, csrf := registerWebChatTestUser(t, server, mailer)
	registerWebChatRouterPlugin(t, server, "p1")
	cases := []map[string]interface{}{
		{"plugin_id": "p1", "type": "markdown", "content": "# hello"},
		{"plugin_id": "p1", "type": "image", "image_url": "https://example.com/a.png"},
		{"plugin_id": "p1", "type": "rich", "parts": []map[string]string{{"type": "text", "text": "hello"}}},
	}
	for _, payload := range cases {
		rr := httptest.NewRecorder()
		req := jsonRequest("/api/open/web-chat/messages", payload, csrf)
		req.AddCookie(sessionCookie)
		server.handleWebChatAPI(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected bad request for %#v, got %d body=%s", payload, rr.Code, rr.Body.String())
		}
	}
}

func TestWebChatRequiresRunningAdapter(t *testing.T) {
	server, mailer := newWebChatTestServer(t, false)
	endpoints := []struct {
		name   string
		method string
		path   string
		req    *http.Request
	}{
		{name: "email-code", req: jsonRequest("/api/open/web-chat/email-code", map[string]string{"email": "u@example.com"}, "")},
		{name: "register", req: jsonRequest("/api/open/web-chat/register", map[string]string{"email": "u@example.com", "code": "123456", "username": "user_1", "password": "password123"}, "")},
		{name: "login", req: jsonRequest("/api/open/web-chat/login", map[string]string{"login": "user_1", "password": "password123"}, "")},
		{name: "plugins", req: httptest.NewRequest(http.MethodGet, "/api/open/web-chat/plugins", nil)},
		{name: "messages-get", req: httptest.NewRequest(http.MethodGet, "/api/open/web-chat/messages", nil)},
		{name: "messages-post", req: jsonRequest("/api/open/web-chat/messages", map[string]string{"type": "text", "content": "hello"}, "csrf")},
		{name: "message-counts", req: httptest.NewRequest(http.MethodGet, "/api/open/web-chat/message-counts", nil)},
		{name: "read-state", req: jsonRequest("/api/open/web-chat/read-state", map[string]string{}, "csrf")},
		{name: "events", req: httptest.NewRequest(http.MethodGet, "/api/open/web-chat/events", nil)},
		{name: "images", req: httptest.NewRequest(http.MethodPost, "/api/open/web-chat/images", nil)},
	}
	for _, endpoint := range endpoints {
		t.Run(endpoint.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			server.handleWebChatAPI(rr, endpoint.req)
			if rr.Code != http.StatusServiceUnavailable {
				t.Fatalf("expected 503, got %d body=%s", rr.Code, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), webChatAdapterUnavailableMessage) {
				t.Fatalf("expected adapter unavailable message, got %s", rr.Body.String())
			}
		})
	}
	if mailer.code != "" {
		t.Fatalf("email sender should not be called, got code %s", mailer.code)
	}
}

func TestWebChatLogoutDoesNotRequireRunningAdapter(t *testing.T) {
	server, _ := newWebChatTestServer(t, false)
	rr := httptest.NewRecorder()
	server.handleWebChatAPI(rr, jsonRequest("/api/open/web-chat/logout", map[string]string{}, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("logout status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestWebChatSendMessageWithoutAdapterDoesNotPersist(t *testing.T) {
	server, mailer := newWebChatTestServer(t, true)
	cookie, csrf := registerWebChatTestUser(t, server, mailer)
	registerWebChatRouterPlugin(t, server, "p1")
	server.adapterManager.StopAdapter(config.WebChatPlatform)
	rr := httptest.NewRecorder()
	req := jsonRequest("/api/open/web-chat/messages", map[string]string{"plugin_id": "p1", "type": "text", "content": "hello"}, csrf)
	req.AddCookie(cookie)
	server.handleWebChatAPI(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d body=%s", rr.Code, rr.Body.String())
	}
	session, err := server.adapterManager.GetDatabase().GetWebChatSession(cookie.Value)
	if err != nil {
		t.Fatalf("GetWebChatSession returned error: %v", err)
	}
	messages, err := server.adapterManager.GetDatabase().ListWebChatMessages(session.User.UserID, 0, 10)
	if err != nil {
		t.Fatalf("ListWebChatMessages returned error: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("expected no persisted messages, got %#v", messages)
	}
}

func TestWebChatPluginsFiltersByRouter(t *testing.T) {
	server, mailer := newWebChatTestServer(t, true)
	cookie, csrf := registerWebChatTestUser(t, server, mailer)
	pluginDir := t.TempDir()
	server.pluginManager = plugin.NewManager(pluginDir, nil)
	writeWebChatTestPlugin(t, pluginDir, "p1", `{"enabled":true,"title":"客服入口","description":"处理订单问题","placeholder":"请输入订单号","entry_text":"订单查询","quick_actions":[{"label":"查订单","text":"订单查询"}],"keywords":["订单"]}`)
	loaded, err := server.pluginManager.LoadPlugin(filepath.Join(pluginDir, "p1"))
	if err != nil {
		t.Fatalf("LoadPlugin returned error: %v", err)
	}
	if err := server.router.RegisterPlugin(loaded); err != nil {
		t.Fatalf("RegisterPlugin returned error: %v", err)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/open/web-chat/plugins", nil)
	req.AddCookie(cookie)
	req.Header.Set("X-AllBot-WebChat-CSRF", csrf)
	server.handleWebChatAPI(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("plugins status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte("客服入口")) || !bytes.Contains(rr.Body.Bytes(), []byte("订单查询")) {
		t.Fatalf("expected web_chat metadata in response: %s", rr.Body.String())
	}
}

func TestWebChatPrivateMessagesUseNormalRouter(t *testing.T) {
	server, mailer := newWebChatTestServer(t, true)
	cookie, csrf := registerWebChatTestUser(t, server, mailer)
	userID := webChatTestUserID(t, server, cookie)
	registerWebChatRouterPluginWithTrigger(t, server, "p1", "^hello$", nil)
	ch := server.router.GetSessionManager().CreateSession("builtin", userID, "", 30)

	rr := httptest.NewRecorder()
	req := jsonRequest("/api/open/web-chat/messages", map[string]string{"type": "text", "content": "myid"}, csrf)
	req.AddCookie(cookie)
	server.handleWebChatAPI(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("send private status=%d body=%s", rr.Code, rr.Body.String())
	}
	if got := <-ch; got != "myid" {
		t.Fatalf("normal router session got %q", got)
	}
	messages, err := server.runtimeDatabase().ListWebChatMessagesByPlugin(userID, "", 0, 10)
	if err != nil {
		t.Fatalf("ListWebChatMessagesByPlugin private returned error: %v", err)
	}
	if len(messages) != 1 || messages[0].PluginID != "" || messages[0].Content != "myid" {
		t.Fatalf("unexpected private messages: %#v", messages)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/open/web-chat/messages?after_id=0&limit=10", nil)
	req.AddCookie(cookie)
	server.handleWebChatAPI(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("get private status=%d body=%s", rr.Code, rr.Body.String())
	}
	var response []config.WebChatMessage
	decodeUnifiedResponseData(t, rr, &response)
	if len(response) != 1 || response[0].PluginID != "" || response[0].Content != "myid" {
		t.Fatalf("unexpected private response: %#v", response)
	}
}

func TestWebChatMessageCounts(t *testing.T) {
	server, mailer := newWebChatTestServer(t, true)
	cookie, csrf := registerWebChatTestUser(t, server, mailer)
	userID := webChatTestUserID(t, server, cookie)
	registerWebChatRouterPlugin(t, server, "p1")

	for _, payload := range []map[string]string{{"type": "text", "content": "private"}, {"plugin_id": "p1", "type": "text", "content": "hello"}} {
		rr := httptest.NewRecorder()
		req := jsonRequest("/api/open/web-chat/messages", payload, csrf)
		req.AddCookie(cookie)
		server.handleWebChatAPI(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("send %#v status=%d body=%s", payload, rr.Code, rr.Body.String())
		}
	}
	if _, err := server.runtimeDatabase().SaveWebChatMessage(&config.WebChatMessage{UserID: userID, Direction: "out", MessageType: "text", Content: "push", Target: "user_" + userID}); err != nil {
		t.Fatalf("SaveWebChatMessage returned error: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/open/web-chat/message-counts", nil)
	req.AddCookie(cookie)
	server.handleWebChatAPI(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("message-counts status=%d body=%s", rr.Code, rr.Body.String())
	}
	var counts []config.WebChatMessageCount
	decodeUnifiedResponseData(t, rr, &counts)
	countByPlugin := map[string]int64{}
	for _, item := range counts {
		countByPlugin[item.PluginID] = item.Count
	}
	if countByPlugin[""] != 2 || countByPlugin["p1"] != 1 {
		t.Fatalf("unexpected counts: %#v", counts)
	}
	for _, item := range counts {
		if item.UnreadCount != item.Count {
			t.Fatalf("expected all messages unread before read-state, got %#v", item)
		}
	}

	rr = httptest.NewRecorder()
	req = jsonRequest("/api/open/web-chat/read-state", map[string]string{"plugin_id": "p1"}, csrf)
	req.AddCookie(cookie)
	server.handleWebChatAPI(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("read-state status=%d body=%s", rr.Code, rr.Body.String())
	}
	var readState config.WebChatMessageCount
	decodeUnifiedResponseData(t, rr, &readState)
	if readState.PluginID != "p1" || readState.Count != 1 || readState.UnreadCount != 0 || readState.LastReadMessageID == 0 {
		t.Fatalf("unexpected read state: %#v", readState)
	}
	if _, err := server.runtimeDatabase().SaveWebChatMessage(&config.WebChatMessage{UserID: userID, Direction: "out", MessageType: "text", Content: "p1-push", PluginID: "p1", Target: "user_" + userID + "#plugin_p1"}); err != nil {
		t.Fatalf("SaveWebChatMessage plugin push returned error: %v", err)
	}
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/open/web-chat/message-counts", nil)
	req.AddCookie(cookie)
	server.handleWebChatAPI(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("message-counts after read status=%d body=%s", rr.Code, rr.Body.String())
	}
	counts = nil
	decodeUnifiedResponseData(t, rr, &counts)
	unreadByPlugin := map[string]int64{}
	for _, item := range counts {
		unreadByPlugin[item.PluginID] = item.UnreadCount
	}
	if unreadByPlugin["p1"] != 1 || unreadByPlugin[""] != 2 {
		t.Fatalf("unexpected unread counts after read: %#v", counts)
	}
}

func TestWebChatMessagesAreIsolatedByPlugin(t *testing.T) {
	server, mailer := newWebChatTestServer(t, true)
	cookie, csrf := registerWebChatTestUser(t, server, mailer)
	registerWebChatRouterPlugin(t, server, "p1")
	registerWebChatRouterPlugin(t, server, "p2")
	for _, pluginID := range []string{"p1", "p2"} {
		rr := httptest.NewRecorder()
		req := jsonRequest("/api/open/web-chat/messages", map[string]string{"plugin_id": pluginID, "type": "text", "content": "hello " + pluginID}, csrf)
		req.AddCookie(cookie)
		server.handleWebChatAPI(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("send %s status=%d body=%s", pluginID, rr.Code, rr.Body.String())
		}
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/open/web-chat/messages?plugin_id=p1&after_id=0&limit=10", nil)
	req.AddCookie(cookie)
	server.handleWebChatAPI(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("messages status=%d body=%s", rr.Code, rr.Body.String())
	}
	var messages []config.WebChatMessage
	decodeUnifiedResponseData(t, rr, &messages)
	if len(messages) != 1 || messages[0].PluginID != "p1" || messages[0].Content != "hello p1" {
		t.Fatalf("unexpected isolated messages: %#v", messages)
	}
}

func TestWebChatSuggestsSwitchForOtherPluginTrigger(t *testing.T) {
	server, mailer := newWebChatTestServer(t, true)
	cookie, csrf := registerWebChatTestUser(t, server, mailer)
	userID := webChatTestUserID(t, server, cookie)
	registerWebChatRouterPluginWithTrigger(t, server, "p1", "^current$", nil)
	registerWebChatRouterPluginWithTrigger(t, server, "p2", "^other$", &types.PluginWebChatConfig{Title: "星韵查询", Description: "查询插件", QuickActions: []types.PluginWebChatQuickAction{{Label: "星韵查询", Text: "other"}}})

	rr := httptest.NewRecorder()
	req := jsonRequest("/api/open/web-chat/messages", map[string]string{"plugin_id": "p1", "type": "text", "content": "other"}, csrf)
	req.AddCookie(cookie)
	server.handleWebChatAPI(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("send status=%d body=%s", rr.Code, rr.Body.String())
	}
	messages, err := server.runtimeDatabase().ListWebChatMessagesByPlugin(userID, "p1", 0, 10)
	if err != nil {
		t.Fatalf("ListWebChatMessagesByPlugin p1 returned error: %v", err)
	}
	if len(messages) != 2 || messages[0].Direction != "in" || messages[1].Direction != "out" || messages[1].MessageType != "rich" {
		t.Fatalf("expected inbound plus rich suggestion, got %#v", messages)
	}
	if !strings.Contains(messages[1].RichJSON, `"type":"plugin_card"`) || !strings.Contains(messages[1].RichJSON, `"plugin_id":"p2"`) || !strings.Contains(messages[1].RichJSON, `"type":"switch_plugin"`) {
		t.Fatalf("unexpected suggestion rich json: %s", messages[1].RichJSON)
	}
	targetMessages, err := server.runtimeDatabase().ListWebChatMessagesByPlugin(userID, "p2", 0, 10)
	if err != nil {
		t.Fatalf("ListWebChatMessagesByPlugin p2 returned error: %v", err)
	}
	if len(targetMessages) != 0 {
		t.Fatalf("target plugin should not receive messages automatically: %#v", targetMessages)
	}
}

func TestWebChatDoesNotSuggestWhenCurrentPluginMatches(t *testing.T) {
	server, mailer := newWebChatTestServer(t, true)
	cookie, csrf := registerWebChatTestUser(t, server, mailer)
	userID := webChatTestUserID(t, server, cookie)
	registerWebChatRouterPluginWithTrigger(t, server, "p1", "^same$", nil)
	registerWebChatRouterPluginWithTrigger(t, server, "p2", "^same$", nil)

	rr := httptest.NewRecorder()
	req := jsonRequest("/api/open/web-chat/messages", map[string]string{"plugin_id": "p1", "type": "text", "content": "same"}, csrf)
	req.AddCookie(cookie)
	server.handleWebChatAPI(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("send status=%d body=%s", rr.Code, rr.Body.String())
	}
	messages, err := server.runtimeDatabase().ListWebChatMessagesByPlugin(userID, "p1", 0, 10)
	if err != nil {
		t.Fatalf("ListWebChatMessagesByPlugin returned error: %v", err)
	}
	if len(messages) != 1 || messages[0].Direction != "in" {
		t.Fatalf("expected only inbound message, got %#v", messages)
	}
}

func TestWebChatDoesNotSuggestWhenCurrentPluginHasWaitingSession(t *testing.T) {
	server, mailer := newWebChatTestServer(t, true)
	cookie, csrf := registerWebChatTestUser(t, server, mailer)
	userID := webChatTestUserID(t, server, cookie)
	registerWebChatRouterPluginWithTrigger(t, server, "p1", "^current$", nil)
	registerWebChatRouterPluginWithTrigger(t, server, "p2", "^other$", nil)
	ch := server.router.GetSessionManager().CreateSession("p1", userID, webChatPluginGroupID("p1"), 30)

	rr := httptest.NewRecorder()
	req := jsonRequest("/api/open/web-chat/messages", map[string]string{"plugin_id": "p1", "type": "text", "content": "other"}, csrf)
	req.AddCookie(cookie)
	server.handleWebChatAPI(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("send status=%d body=%s", rr.Code, rr.Body.String())
	}
	if got := <-ch; got != "other" {
		t.Fatalf("waiting session got %q", got)
	}
	messages, err := server.runtimeDatabase().ListWebChatMessagesByPlugin(userID, "p1", 0, 10)
	if err != nil {
		t.Fatalf("ListWebChatMessagesByPlugin returned error: %v", err)
	}
	if len(messages) != 1 || messages[0].MessageType == "rich" {
		t.Fatalf("expected no suggestion, got %#v", messages)
	}
}

func TestWebChatPluginWaitingSessionsAreIsolatedByGroupID(t *testing.T) {
	server, mailer := newWebChatTestServer(t, true)
	cookie, csrf := registerWebChatTestUser(t, server, mailer)
	userID := webChatTestUserID(t, server, cookie)
	registerWebChatRouterPluginWithTrigger(t, server, "p1", "^p1$", nil)
	registerWebChatRouterPluginWithTrigger(t, server, "p2", "^p2$", nil)
	ch1 := server.router.GetSessionManager().CreateSession("p1", userID, webChatPluginGroupID("p1"), 30)
	ch2 := server.router.GetSessionManager().CreateSession("p2", userID, webChatPluginGroupID("p2"), 30)

	rr := httptest.NewRecorder()
	req := jsonRequest("/api/open/web-chat/messages", map[string]string{"plugin_id": "p2", "type": "text", "content": "reply p2"}, csrf)
	req.AddCookie(cookie)
	server.handleWebChatAPI(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("send p2 status=%d body=%s", rr.Code, rr.Body.String())
	}
	if got := <-ch2; got != "reply p2" {
		t.Fatalf("p2 waiting session got %q", got)
	}
	select {
	case got := <-ch1:
		t.Fatalf("p1 waiting session should remain isolated, got %q", got)
	default:
	}
}

func TestWebChatPluginSessionGroupDoesNotAffectAccessControl(t *testing.T) {
	server, mailer := newWebChatTestServer(t, true)
	cookie, csrf := registerWebChatTestUser(t, server, mailer)
	userID := webChatTestUserID(t, server, cookie)
	registerWebChatRouterPluginWithTrigger(t, server, "p1", "^login$", nil)
	plugin := server.router.GetPlugin("p1")
	plugin.AccessControl.WhitelistUserIDs = []string{userID}
	plugin.AccessControl.WhitelistGroups = []string{"real-group"}

	rr := httptest.NewRecorder()
	req := jsonRequest("/api/open/web-chat/messages", map[string]string{"plugin_id": "p1", "type": "text", "content": "login"}, csrf)
	req.AddCookie(cookie)
	server.handleWebChatAPI(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("send status=%d body=%s", rr.Code, rr.Body.String())
	}
	messages, err := server.runtimeDatabase().ListWebChatMessagesByPlugin(userID, "p1", 0, 10)
	if err != nil {
		t.Fatalf("ListWebChatMessagesByPlugin returned error: %v", err)
	}
	if len(messages) != 1 || messages[0].Direction != "in" {
		t.Fatalf("session group should not create access-control denial reply: %#v", messages)
	}
}

func TestWebChatPluginPermissionDeniedSendsMessage(t *testing.T) {
	server, mailer := newWebChatTestServer(t, true)
	cookie, csrf := registerWebChatTestUser(t, server, mailer)
	userID := webChatTestUserID(t, server, cookie)
	registerWebChatRouterPluginWithTrigger(t, server, "p1", "^login$", nil)
	plugin := server.router.GetPlugin("p1")
	plugin.AccessControl.WhitelistUserIDs = []string{"other-user"}

	rr := httptest.NewRecorder()
	req := jsonRequest("/api/open/web-chat/messages", map[string]string{"plugin_id": "p1", "type": "text", "content": "login"}, csrf)
	req.AddCookie(cookie)
	server.handleWebChatAPI(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("send status=%d body=%s", rr.Code, rr.Body.String())
	}
	messages, err := server.runtimeDatabase().ListWebChatMessagesByPlugin(userID, "p1", 0, 10)
	if err != nil {
		t.Fatalf("ListWebChatMessagesByPlugin returned error: %v", err)
	}
	if len(messages) != 2 || messages[1].Direction != "out" || messages[1].Content != "你没有权限使用该插件" {
		t.Fatalf("expected permission denied reply, got %#v", messages)
	}
}

func TestWebChatDoesNotSuggestUnavailableTargetPlugin(t *testing.T) {
	server, mailer := newWebChatTestServer(t, true)
	cookie, csrf := registerWebChatTestUser(t, server, mailer)
	userID := webChatTestUserID(t, server, cookie)
	registerWebChatRouterPluginWithTrigger(t, server, "p1", "^current$", nil)
	visible := false
	registerWebChatRouterPluginWithTrigger(t, server, "p2", "^hidden$", &types.PluginWebChatConfig{Enabled: &visible})

	rr := httptest.NewRecorder()
	req := jsonRequest("/api/open/web-chat/messages", map[string]string{"plugin_id": "p1", "type": "text", "content": "hidden"}, csrf)
	req.AddCookie(cookie)
	server.handleWebChatAPI(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("send status=%d body=%s", rr.Code, rr.Body.String())
	}
	messages, err := server.runtimeDatabase().ListWebChatMessagesByPlugin(userID, "p1", 0, 10)
	if err != nil {
		t.Fatalf("ListWebChatMessagesByPlugin returned error: %v", err)
	}
	if len(messages) != 1 || messages[0].MessageType == "rich" {
		t.Fatalf("expected no suggestion for hidden target, got %#v", messages)
	}
}

func newWebChatTestServer(t *testing.T, withAdapter bool) (*Server, *fakeWebChatMailer) {
	t.Helper()
	defaultWebChatLimiter = &webChatRateLimiter{events: map[string][]time.Time{}}
	db, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	r := router.NewRouter(session.NewManager())
	r.SetDatabase(db)
	adapterManager := config.NewAdapterManager(db)
	adapterManager.SetMessageHandler(r.HandleMessage)
	r.SetAdapterGetter(adapterManager.GetAdapter)
	r.SetMessageAdapterGetter(adapterManager.GetAdapterForMessage)
	if withAdapter {
		saveRunningWebChatAdapter(t, adapterManager)
	}
	server := NewServer("0", plugin.NewManager(t.TempDir(), nil), r, adapterManager, nil)
	mailer := &fakeWebChatMailer{}
	server.SetWebChatEmailSender(mailer)
	return server, mailer
}

func registerWebChatRouterPlugin(t *testing.T, server *Server, pluginID string) {
	t.Helper()
	registerWebChatRouterPluginWithTrigger(t, server, pluginID, "^hi$", nil)
}

func registerWebChatRouterPluginWithTrigger(t *testing.T, server *Server, pluginID, trigger string, webChat *types.PluginWebChatConfig) {
	t.Helper()
	item := &types.Plugin{ID: pluginID, Name: "Web 插件 " + pluginID, Trigger: trigger, Enabled: true, Platforms: []string{"web"}}
	if webChat != nil {
		item.WebChat = *webChat
	}
	if err := server.router.RegisterPlugin(item); err != nil {
		t.Fatalf("RegisterPlugin returned error: %v", err)
	}
	if server.pluginManager == nil {
		server.pluginManager = plugin.NewManager(t.TempDir(), nil)
	}
}

func webChatTestUserID(t *testing.T, server *Server, cookie *http.Cookie) string {
	t.Helper()
	session, err := server.runtimeDatabase().GetWebChatSession(cookie.Value)
	if err != nil {
		t.Fatalf("GetWebChatSession returned error: %v", err)
	}
	return session.User.UserID
}

func writeWebChatTestPlugin(t *testing.T, pluginDir, pluginID, webChatJSON string) {
	t.Helper()
	pluginPath := filepath.Join(pluginDir, pluginID)
	if err := os.MkdirAll(pluginPath, 0755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	manifest := `{"name":"Web 插件","version":"1.0.0","runtime":"nodejs","entry":"main.js","platforms":["web"],"trigger":"^hi$","enabled":true,"web_chat":` + webChatJSON + `}`
	if err := os.WriteFile(filepath.Join(pluginPath, "plugin.json"), []byte(manifest), 0644); err != nil {
		t.Fatalf("WriteFile plugin.json returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginPath, "main.js"), []byte(`console.log('ok')`), 0644); err != nil {
		t.Fatalf("WriteFile main.js returned error: %v", err)
	}
}

func saveRunningWebChatAdapter(t *testing.T, adapterManager *config.AdapterManager) {
	t.Helper()
	saveRunningWebChatAdapterWithLimit(t, adapterManager, 0)
}

func saveRunningWebChatAdapterWithLimit(t *testing.T, adapterManager *config.AdapterManager, messageLimit int) {
	t.Helper()
	adapterConfig := map[string]interface{}{
		"smtp_host":     "smtp.example.com",
		"smtp_port":     "587",
		"smtp_username": "user",
		"smtp_password": "pass",
		"smtp_from":     "bot@example.com",
	}
	if messageLimit > 0 {
		adapterConfig["message_limit_per_minute"] = messageLimit
	}
	err := adapterManager.SaveAdapterConfig(0, config.WebChatPlatform, "", "", true, adapterConfig)
	if err != nil {
		t.Fatalf("SaveAdapterConfig returned error: %v", err)
	}
}

func saveRunningWebChatTestPlatformAdapter(t *testing.T, adapterManager *config.AdapterManager) (*webChatPlatformFakeAdapter, string) {
	t.Helper()
	if err := adapterManager.SaveAdapterConfig(0, webChatTestPlatform, "测试机器人", "公开描述", true, map[string]interface{}{"token": "secret"}); err != nil {
		t.Fatalf("SaveAdapterConfig test platform returned error: %v", err)
	}
	items, err := adapterManager.GetDatabase().GetAllAdapters()
	if err != nil {
		t.Fatalf("GetAllAdapters returned error: %v", err)
	}
	for _, item := range items {
		if item.Platform != webChatTestPlatform {
			continue
		}
		adp, ok := adapterManager.GetAdapterByID(item.ID).(*webChatPlatformFakeAdapter)
		if !ok || adp == nil {
			t.Fatalf("unexpected test platform adapter: %#v", adapterManager.GetAdapterByID(item.ID))
		}
		return adp, strconv.FormatInt(item.ID, 10)
	}
	t.Fatal("test platform adapter not found")
	return nil, ""
}

func registerWebChatTestUser(t *testing.T, server *Server, mailer *fakeWebChatMailer) (*http.Cookie, string) {
	t.Helper()
	rr := httptest.NewRecorder()
	server.handleWebChatAPI(rr, jsonRequest("/api/open/web-chat/email-code", map[string]string{"email": "u@example.com"}, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("email code failed: %s", rr.Body.String())
	}
	rr = httptest.NewRecorder()
	server.handleWebChatAPI(rr, jsonRequest("/api/open/web-chat/register", map[string]string{"email": "u@example.com", "code": mailer.code, "username": "user_1", "password": "password123"}, ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("register failed: %s", rr.Body.String())
	}
	var sessionResp config.WebChatSession
	decodeUnifiedResponseData(t, rr, &sessionResp)
	return rr.Result().Cookies()[0], sessionResp.CSRFToken
}

func createWebChatRuntimeUser(t *testing.T, db *config.Database, username, email string) *config.WebChatUser {
	t.Helper()
	if err := db.CreateWebChatEmailCode(email, "123456", config.WebChatEmailPurposeRegister, ""); err != nil {
		t.Fatalf("CreateWebChatEmailCode returned error: %v", err)
	}
	user, err := db.RegisterWebChatUser(config.WebChatRegisterInput{Email: email, Code: "123456", Username: username, Password: "password123"})
	if err != nil {
		t.Fatalf("RegisterWebChatUser returned error: %v", err)
	}
	return user
}

func jsonRequest(path string, payload interface{}, csrf string) *http.Request {
	data, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://example.com")
	req.Host = "example.com"
	if csrf != "" {
		req.Header.Set("X-AllBot-WebChat-CSRF", csrf)
	}
	return req
}
