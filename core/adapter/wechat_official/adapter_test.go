package wechat_official

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/allbot/allbot/core/adapter/_contract"
	"github.com/allbot/allbot/core/types"
)

func TestWeChatOfficialAdapterImplementsContracts(t *testing.T) {
	adapter := NewWeChatOfficialAdapter("app", "secret", "token", "", "", "")
	var _ contract.Adapter = adapter
	var _ contract.ReplyTargetResolver = adapter
	var _ contract.ReplyTextFormatter = adapter
	var _ contract.SendTargetResolver = adapter
	var _ contract.HTTPCallbackHandler = adapter
}

func TestParseConfigForRegistry(t *testing.T) {
	parsed, err := parseConfigForRegistry(`{"app_id":" app ","app_secret":" secret ","token":" token ","callback_path":"/wechat/callback/"}`)
	if err != nil {
		t.Fatalf("parseConfigForRegistry returned error: %v", err)
	}
	config := parsed.(*Config)
	if config.AppID != "app" || config.AppSecret != "secret" || config.Token != "token" {
		t.Fatalf("config trim failed: %#v", config)
	}
	if config.CallbackPath != "wechat/callback" {
		t.Fatalf("CallbackPath = %q", config.CallbackPath)
	}

	parsed, err = parseConfigForRegistry(`{"app_id":"app","app_secret":"secret","token":"token"}`)
	if err != nil {
		t.Fatalf("parseConfigForRegistry returned error: %v", err)
	}
	if parsed.(*Config).CallbackPath != wechatOfficialDefaultPath {
		t.Fatalf("default CallbackPath = %q", parsed.(*Config).CallbackPath)
	}

	for _, raw := range []string{
		`{"app_secret":"secret","token":"token"}`,
		`{"app_id":"app","token":"token"}`,
		`{"app_id":"app","app_secret":"secret"}`,
	} {
		if _, err := parseConfigForRegistry(raw); err == nil {
			t.Fatalf("expected error for %s", raw)
		}
	}
}

func TestVerifySignature(t *testing.T) {
	adapter := NewWeChatOfficialAdapter("app", "secret", "token", "", "", "")
	signature := testWeChatSignature("token", "123", "nonce")
	if !adapter.verifySignature(signature, "123", "nonce") {
		t.Fatal("expected signature to pass")
	}
	if adapter.verifySignature("wrong", "123", "nonce") {
		t.Fatal("expected wrong signature to fail")
	}
	if adapter.verifySignature(signature, "", "nonce") {
		t.Fatal("expected missing timestamp to fail")
	}
}

func TestHandleVerifyCallback(t *testing.T) {
	adapter := NewWeChatOfficialAdapter("app", "secret", "token", "callback", "", "")
	query := url.Values{}
	query.Set("timestamp", "123")
	query.Set("nonce", "nonce")
	query.Set("echostr", "hello")
	query.Set("signature", testWeChatSignature("token", "123", "nonce"))
	request := httptest.NewRequest(http.MethodGet, "/?"+query.Encode(), nil)
	response := httptest.NewRecorder()

	adapter.HandleHTTPCallback("callback", response, request)

	if response.Code != http.StatusOK || response.Body.String() != "hello" {
		t.Fatalf("status = %d body = %q", response.Code, response.Body.String())
	}

	query.Set("signature", "bad")
	request = httptest.NewRequest(http.MethodGet, "/?"+query.Encode(), nil)
	response = httptest.NewRecorder()
	adapter.HandleHTTPCallback("callback", response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestParseTextMessageXML(t *testing.T) {
	adapter := NewWeChatOfficialAdapter("app", "secret", "token", "", "", "")
	msg, err := adapter.parseMessageXML([]byte(`<xml>
<ToUserName><![CDATA[gh_app]]></ToUserName>
<FromUserName><![CDATA[openid]]></FromUserName>
<CreateTime>1710000000</CreateTime>
<MsgType><![CDATA[text]]></MsgType>
<Content><![CDATA[  你好  ]]></Content>
<MsgId>123456</MsgId>
</xml>`))
	if err != nil {
		t.Fatalf("parseMessageXML returned error: %v", err)
	}
	assertMessage(t, msg, "123456", "openid", "你好")
	if msg.Metadata["wechat_msg_type"] != "text" || msg.Metadata["reply_target"] != "openid" || msg.Metadata["wechat_to_user_name"] != "gh_app" {
		t.Fatalf("metadata = %#v", msg.Metadata)
	}
}

func TestParseEventMessageXML(t *testing.T) {
	adapter := NewWeChatOfficialAdapter("app", "secret", "token", "", "", "")
	msg, err := adapter.parseMessageXML([]byte(`<xml>
<ToUserName><![CDATA[gh_app]]></ToUserName>
<FromUserName><![CDATA[openid]]></FromUserName>
<CreateTime>1710000000</CreateTime>
<MsgType><![CDATA[event]]></MsgType>
<Event><![CDATA[CLICK]]></Event>
<EventKey><![CDATA[MENU_KEY]]></EventKey>
</xml>`))
	if err != nil {
		t.Fatalf("parseMessageXML returned error: %v", err)
	}
	assertMessage(t, msg, "event:CLICK:openid:1710000000", "openid", "event:CLICK:MENU_KEY")
	if msg.Metadata["wechat_event"] != "CLICK" || msg.Metadata["wechat_event_key"] != "MENU_KEY" {
		t.Fatalf("metadata = %#v", msg.Metadata)
	}

	msg, err = adapter.parseMessageXML([]byte(`<xml><FromUserName>openid</FromUserName><CreateTime>1</CreateTime><MsgType>event</MsgType><Event>subscribe</Event></xml>`))
	if err != nil {
		t.Fatalf("parseMessageXML returned error: %v", err)
	}
	if msg.Content != "event:subscribe" {
		t.Fatalf("Content = %q", msg.Content)
	}
}

func TestPostCallbackDispatchesMessage(t *testing.T) {
	adapter := NewWeChatOfficialAdapter("app", "secret", "token", "callback", "", "")
	messages := make(chan *types.Message, 1)
	adapter.SetMessageHandler(func(msg *types.Message) { messages <- msg })
	query := url.Values{}
	query.Set("timestamp", "123")
	query.Set("nonce", "nonce")
	query.Set("signature", testWeChatSignature("token", "123", "nonce"))
	request := httptest.NewRequest(http.MethodPost, "/?"+query.Encode(), strings.NewReader(`<xml><FromUserName>openid</FromUserName><CreateTime>1</CreateTime><MsgType>text</MsgType><Content>ping</Content><MsgId>9</MsgId></xml>`))
	response := httptest.NewRecorder()

	adapter.HandleHTTPCallback("callback", response, request)

	if response.Code != http.StatusOK || response.Body.String() != "success" {
		t.Fatalf("status = %d body = %q", response.Code, response.Body.String())
	}
	select {
	case msg := <-messages:
		assertMessage(t, msg, "9", "openid", "ping")
	case <-time.After(time.Second):
		t.Fatal("message was not dispatched")
	}
}

func TestReplyAndSendTarget(t *testing.T) {
	adapter := NewWeChatOfficialAdapter("app", "secret", "token", "", "", "")
	msg := &types.Message{UserID: "user", Metadata: map[string]string{"reply_target": "target"}}
	if got := adapter.ReplyTarget(msg); got != "target" {
		t.Fatalf("ReplyTarget = %q", got)
	}
	msg.Metadata = map[string]string{"wechat_openid": "openid"}
	if got := adapter.ReplyTarget(msg); got != "openid" {
		t.Fatalf("ReplyTarget = %q", got)
	}
	if got := adapter.SendTarget("user", "group"); got != "user" {
		t.Fatalf("SendTarget = %q", got)
	}
	if got := adapter.FormatReplyText(msg, "hello"); got != "hello" {
		t.Fatalf("FormatReplyText = %q", got)
	}
}

func TestAccessTokenCacheAndRefresh(t *testing.T) {
	var tokenCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		atomic.AddInt32(&tokenCalls, 1)
		if r.URL.Query().Get("grant_type") != "client_credential" || r.URL.Query().Get("appid") != "app" || r.URL.Query().Get("secret") != "secret" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"access_token":"token","expires_in":7200}`))
	}))
	defer server.Close()

	adapter := NewWeChatOfficialAdapter("app", "secret", "token", "", server.URL, server.URL+"/token")
	first, err := adapter.getAccessToken()
	if err != nil {
		t.Fatalf("getAccessToken returned error: %v", err)
	}
	second, err := adapter.getAccessToken()
	if err != nil {
		t.Fatalf("getAccessToken returned error: %v", err)
	}
	if first != "token" || second != "token" || atomic.LoadInt32(&tokenCalls) != 1 {
		t.Fatalf("tokens = %q/%q calls = %d", first, second, tokenCalls)
	}

	adapter.tokenExpiresAt = time.Now().Add(time.Minute)
	if _, err := adapter.getAccessToken(); err != nil {
		t.Fatalf("refresh getAccessToken returned error: %v", err)
	}
	if atomic.LoadInt32(&tokenCalls) != 2 {
		t.Fatalf("tokenCalls = %d", tokenCalls)
	}
}

func TestSendMessageUsesCustomerServiceAPI(t *testing.T) {
	var sendCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_, _ = w.Write([]byte(`{"access_token":"access","expires_in":7200}`))
		case "/cgi-bin/message/custom/send":
			atomic.AddInt32(&sendCalls, 1)
			if r.URL.Query().Get("access_token") != "access" {
				t.Fatalf("access_token = %q", r.URL.Query().Get("access_token"))
			}
			payload, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll returned error: %v", err)
			}
			var body map[string]interface{}
			if err := json.Unmarshal(payload, &body); err != nil {
				t.Fatalf("json.Unmarshal returned error: %v", err)
			}
			if body["touser"] != "openid" || body["msgtype"] != "text" {
				t.Fatalf("body = %#v", body)
			}
			text, _ := body["text"].(map[string]interface{})
			if text["content"] != "hello" {
				t.Fatalf("text = %#v", text)
			}
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	adapter := NewWeChatOfficialAdapter("app", "secret", "token", "", server.URL, server.URL+"/token")
	if err := adapter.SendMessage("openid", "hello"); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}
	if atomic.LoadInt32(&sendCalls) != 1 {
		t.Fatalf("sendCalls = %d", sendCalls)
	}
}

func TestPostCallbackReturnsBeforeSlowHandler(t *testing.T) {
	adapter := NewWeChatOfficialAdapter("app", "secret", "token", "callback", "", "")
	started := make(chan struct{})
	release := make(chan struct{})
	adapter.SetMessageHandler(func(msg *types.Message) {
		close(started)
		<-release
	})
	query := url.Values{}
	query.Set("timestamp", "123")
	query.Set("nonce", "nonce")
	query.Set("signature", testWeChatSignature("token", "123", "nonce"))
	request := httptest.NewRequest(http.MethodPost, "/?"+query.Encode(), strings.NewReader(`<xml><FromUserName>openid</FromUserName><CreateTime>1</CreateTime><MsgType>text</MsgType><Content>ping</Content><MsgId>9</MsgId></xml>`))
	response := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		adapter.HandleHTTPCallback("callback", response, request)
		close(done)
	}()
	defer close(release)

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("HandleHTTPCallback did not return before slow handler finished")
	}
	if response.Code != http.StatusOK || response.Body.String() != "success" {
		t.Fatalf("status = %d body = %q", response.Code, response.Body.String())
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("message handler was not started")
	}
}

func TestPostCallbackRejectsBadSignatureAndXML(t *testing.T) {
	adapter := NewWeChatOfficialAdapter("app", "secret", "token", "callback", "", "")
	request := httptest.NewRequest(http.MethodPost, "/?timestamp=123&nonce=nonce&signature=bad", strings.NewReader(`<xml></xml>`))
	response := httptest.NewRecorder()
	adapter.HandleHTTPCallback("callback", response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("bad signature status = %d", response.Code)
	}

	query := url.Values{}
	query.Set("timestamp", "123")
	query.Set("nonce", "nonce")
	query.Set("signature", testWeChatSignature("token", "123", "nonce"))
	request = httptest.NewRequest(http.MethodPost, "/?"+query.Encode(), strings.NewReader(`<xml>`))
	response = httptest.NewRecorder()
	adapter.HandleHTTPCallback("callback", response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("bad xml status = %d", response.Code)
	}
}

func TestSendMessageRefreshesExpiredAccessTokenOnce(t *testing.T) {
	for _, errCode := range []int{40001, 40014, 42001} {
		t.Run(strconv.Itoa(errCode), func(t *testing.T) {
			var tokenCalls int32
			var sendCalls int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/token":
					call := atomic.AddInt32(&tokenCalls, 1)
					_, _ = w.Write([]byte(`{"access_token":"access-` + strconv.Itoa(int(call)) + `","expires_in":7200}`))
				case "/cgi-bin/message/custom/send":
					call := atomic.AddInt32(&sendCalls, 1)
					if call == 1 {
						_, _ = w.Write([]byte(`{"errcode":` + strconv.Itoa(errCode) + `,"errmsg":"access_token expired"}`))
						return
					}
					if r.URL.Query().Get("access_token") != "access-2" {
						t.Fatalf("access_token = %q", r.URL.Query().Get("access_token"))
					}
					_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
				default:
					t.Fatalf("unexpected path %s", r.URL.Path)
				}
			}))
			defer server.Close()

			adapter := NewWeChatOfficialAdapter("app", "secret", "token", "", server.URL, server.URL+"/token")
			if err := adapter.SendMessage("openid", "hello"); err != nil {
				t.Fatalf("SendMessage returned error: %v", err)
			}
			if atomic.LoadInt32(&tokenCalls) != 2 || atomic.LoadInt32(&sendCalls) != 2 {
				t.Fatalf("tokenCalls = %d sendCalls = %d", tokenCalls, sendCalls)
			}
		})
	}
}

func TestSendMessageDoesNotRetryExpiredAccessTokenTwice(t *testing.T) {
	var tokenCalls int32
	var sendCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			call := atomic.AddInt32(&tokenCalls, 1)
			_, _ = w.Write([]byte(`{"access_token":"access-` + strconv.Itoa(int(call)) + `","expires_in":7200}`))
		case "/cgi-bin/message/custom/send":
			atomic.AddInt32(&sendCalls, 1)
			_, _ = w.Write([]byte(`{"errcode":42001,"errmsg":"access_token expired"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	adapter := NewWeChatOfficialAdapter("app", "secret", "token", "", server.URL, server.URL+"/token")
	err := adapter.SendMessage("openid", "hello")
	if err == nil || !strings.Contains(err.Error(), "42001") {
		t.Fatalf("err = %v", err)
	}
	if atomic.LoadInt32(&tokenCalls) != 2 || atomic.LoadInt32(&sendCalls) != 2 {
		t.Fatalf("tokenCalls = %d sendCalls = %d", tokenCalls, sendCalls)
	}
}

func TestSendMessageReturnsWechatError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_, _ = w.Write([]byte(`{"access_token":"access","expires_in":7200}`))
		case "/cgi-bin/message/custom/send":
			_, _ = w.Write([]byte(`{"errcode":45015,"errmsg":"response out of time limit"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	adapter := NewWeChatOfficialAdapter("app", "secret", "token", "", server.URL, server.URL+"/token")
	err := adapter.SendMessage("openid", "hello")
	if err == nil || !strings.Contains(err.Error(), "45015") || !strings.Contains(err.Error(), "response out of time limit") {
		t.Fatalf("err = %v", err)
	}
}

func assertMessage(t *testing.T, msg *types.Message, id string, userID string, content string) {
	t.Helper()
	if msg == nil {
		t.Fatal("message is nil")
	}
	if msg.ID != id || msg.Platform != platformName || msg.UserID != userID || msg.GroupID != "" || msg.Content != content {
		t.Fatalf("message = %#v", msg)
	}
	if msg.Metadata["message_type"] != "private" || msg.Metadata["wechat_openid"] != userID {
		t.Fatalf("metadata = %#v", msg.Metadata)
	}
}

func testWeChatSignature(token, timestamp, nonce string) string {
	values := []string{token, timestamp, nonce}
	sort.Strings(values)
	sum := sha1.Sum([]byte(strings.Join(values, "")))
	return hex.EncodeToString(sum[:])
}
