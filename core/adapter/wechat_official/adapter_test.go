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
	adapter := NewWeChatOfficialAdapter("app", "gh_app", "secret", "token", "", "", "")
	var _ contract.Adapter = adapter
	var _ contract.ReplyTargetResolver = adapter
	var _ contract.ReplyTextFormatter = adapter
	var _ contract.SendTargetResolver = adapter
	var _ contract.HTTPCallbackHandler = adapter
	var _ contract.BotIdentityProvider = adapter
}

func TestWeChatOfficialBotIdentityPrefersOriginalID(t *testing.T) {
	adapter := NewWeChatOfficialAdapter("app-id", "gh_original", "app-secret", "token", "", "", "")
	identity := adapter.GetBotIdentity(&types.Message{Metadata: map[string]string{"wechat_to_user_name": "gh_original"}})
	if identity.Label != "公众号原始 ID" || identity.Value != "gh_original" {
		t.Fatalf("metadata identity = %#v", identity)
	}
	identity = adapter.GetBotIdentity(&types.Message{})
	if identity.Label != "公众号 App ID" || identity.Value != "app-id" {
		t.Fatalf("fallback identity = %#v", identity)
	}
	if strings.Contains(identity.Value, "app-secret") {
		t.Fatalf("identity leaked app secret: %#v", identity)
	}
}

func TestParseConfigForRegistry(t *testing.T) {
	parsed, err := parseConfigForRegistry(`{"app_id":" app ","original_id":" gh_app ","app_secret":" secret ","token":" token ","callback_path":"/wechat/callback/"}`)
	if err != nil {
		t.Fatalf("parseConfigForRegistry returned error: %v", err)
	}
	config := parsed.(*Config)
	if config.AppID != "app" || config.OriginalID != "gh_app" || config.AppSecret != "secret" || config.Token != "token" {
		t.Fatalf("config trim failed: %#v", config)
	}
	if config.CallbackPath != "wechat/callback" {
		t.Fatalf("CallbackPath = %q", config.CallbackPath)
	}

	parsed, err = parseConfigForRegistry(`{"app_id":"app","original_id":"gh_app","app_secret":"secret","token":"token"}`)
	if err != nil {
		t.Fatalf("parseConfigForRegistry returned error: %v", err)
	}
	if parsed.(*Config).CallbackPath != wechatOfficialDefaultPath {
		t.Fatalf("default CallbackPath = %q", parsed.(*Config).CallbackPath)
	}

	parsed, err = parseConfigForRegistry(`{"app_id":"app","app_secret":"secret","token":"token"}`)
	if err != nil || parsed.(*Config).OriginalID != "" {
		t.Fatalf("旧配置应保持兼容: parsed=%#v err=%v", parsed, err)
	}

	for _, raw := range []string{
		`{"original_id":"gh_app","app_secret":"secret","token":"token"}`,
		`{"app_id":"app","original_id":"gh_app","token":"token"}`,
		`{"app_id":"app","original_id":"gh_app","app_secret":"secret"}`,
	} {
		if _, err := parseConfigForRegistry(raw); err == nil {
			t.Fatalf("expected error for %s", raw)
		}
	}
}

func TestVerifySignature(t *testing.T) {
	adapter := NewWeChatOfficialAdapter("app", "gh_app", "secret", "token", "", "", "")
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
	adapter := NewWeChatOfficialAdapter("app", "gh_app", "secret", "token", "callback", "", "")
	query := url.Values{}
	query.Set("timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	query.Set("nonce", "nonce")
	query.Set("echostr", "hello")
	query.Set("signature", testWeChatSignature("token", query.Get("timestamp"), "nonce"))
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

func TestWeChatOfficialStartAllowsMissingOriginalID(t *testing.T) {
	adapter := NewWeChatOfficialAdapter("app", "", "secret", "token", "callback", "", "")
	if err := adapter.Start(); err != nil {
		t.Fatalf("旧配置启动失败: %v", err)
	}
}

func TestParseTextMessageXML(t *testing.T) {
	adapter := NewWeChatOfficialAdapter("app", "gh_app", "secret", "token", "", "", "")
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
	adapter := NewWeChatOfficialAdapter("app", "gh_app", "secret", "token", "", "", "")
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

	msg, err = adapter.parseMessageXML([]byte(`<xml><FromUserName>openid</FromUserName><CreateTime>` + strconv.FormatInt(time.Now().Unix(), 10) + `</CreateTime><MsgType>event</MsgType><Event>subscribe</Event></xml>`))
	if err != nil {
		t.Fatalf("parseMessageXML returned error: %v", err)
	}
	if msg.Content != "event:subscribe" {
		t.Fatalf("Content = %q", msg.Content)
	}
}

func TestPostCallbackDispatchesMessage(t *testing.T) {
	adapter := NewWeChatOfficialAdapter("app", "gh_app", "secret", "token", "callback", "", "")
	wechatOfficialPassiveReplyWait = 10 * time.Millisecond
	defer func() { wechatOfficialPassiveReplyWait = 2 * time.Second }()
	messages := make(chan *types.Message, 1)
	adapter.SetMessageHandler(func(msg *types.Message) { messages <- msg })
	query := url.Values{}
	query.Set("timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	query.Set("nonce", "nonce")
	query.Set("signature", testWeChatSignature("token", query.Get("timestamp"), "nonce"))
	request := httptest.NewRequest(http.MethodPost, "/?"+query.Encode(), strings.NewReader(testWeChatMessageXML("gh_app", "9")))
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

func TestVerifyFreshSignatureRejectsStaleAndReplay(t *testing.T) {
	adapter := NewWeChatOfficialAdapter("app", "gh_app", "secret", "token", "callback", "", "")
	now := time.Unix(1_720_000_000, 0)
	adapter.now = func() time.Time { return now }

	for _, timestamp := range []string{
		strconv.FormatInt(now.Add(-wechatOfficialCallbackFreshness-time.Second).Unix(), 10),
		strconv.FormatInt(now.Add(wechatOfficialCallbackFreshness+time.Second).Unix(), 10),
	} {
		signature := testWeChatSignature("token", timestamp, "nonce")
		if adapter.verifyFreshSignature(signature, timestamp, "nonce", "message") {
			t.Fatalf("越界时间戳 %s 不应通过", timestamp)
		}
	}

	timestamp := strconv.FormatInt(now.Unix(), 10)
	signature := testWeChatSignature("token", timestamp, "nonce")
	if !adapter.verifyFreshSignature(signature, timestamp, "nonce", "message") {
		t.Fatal("首次有效签名应通过")
	}
	if adapter.verifyFreshSignature(signature, timestamp, "nonce", "message") {
		t.Fatal("重复签名不应通过")
	}
}

func TestPostCallbackRejectsWrongTargetAndDuplicateMessage(t *testing.T) {
	adapter := NewWeChatOfficialAdapter("app", "gh_app", "secret", "token", "callback", "", "")
	wechatOfficialPassiveReplyWait = 10 * time.Millisecond
	defer func() { wechatOfficialPassiveReplyWait = 2 * time.Second }()
	messages := make(chan *types.Message, 2)
	adapter.SetMessageHandler(func(msg *types.Message) { messages <- msg })

	send := func(nonce, target, messageID string) *httptest.ResponseRecorder {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		query := url.Values{}
		query.Set("timestamp", timestamp)
		query.Set("nonce", nonce)
		query.Set("signature", testWeChatSignature("token", timestamp, nonce))
		request := httptest.NewRequest(http.MethodPost, "/?"+query.Encode(), strings.NewReader(testWeChatMessageXML(target, messageID)))
		response := httptest.NewRecorder()
		adapter.HandleHTTPCallback("callback", response, request)
		return response
	}

	if response := send("wrong-target", "gh_other", "wrong"); response.Code != http.StatusOK {
		t.Fatalf("错误目标响应状态 = %d", response.Code)
	}
	select {
	case <-messages:
		t.Fatal("错误目标消息不应派发")
	case <-time.After(30 * time.Millisecond):
	}

	if response := send("first", "gh_app", "duplicate"); response.Code != http.StatusOK {
		t.Fatalf("首次消息响应状态 = %d", response.Code)
	}
	select {
	case <-messages:
	case <-time.After(time.Second):
		t.Fatal("首次消息未派发")
	}
	if response := send("second", "gh_app", "duplicate"); response.Code != http.StatusOK {
		t.Fatalf("重复消息响应状态 = %d", response.Code)
	}
	select {
	case <-messages:
		t.Fatal("重复消息不应派发")
	case <-time.After(30 * time.Millisecond):
	}
}

func TestPostCallbackRejectsOversizedBody(t *testing.T) {
	adapter := NewWeChatOfficialAdapter("app", "gh_app", "secret", "token", "callback", "", "")
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	query := url.Values{}
	query.Set("timestamp", timestamp)
	query.Set("nonce", "large")
	query.Set("signature", testWeChatSignature("token", timestamp, "large"))
	request := httptest.NewRequest(http.MethodPost, "/?"+query.Encode(), strings.NewReader(strings.Repeat("x", wechatOfficialCallbackBodyLimit+1)))
	response := httptest.NewRecorder()

	adapter.HandleHTTPCallback("callback", response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestPostCallbackAllowsUnconfiguredOriginalID(t *testing.T) {
	adapter := NewWeChatOfficialAdapter("app", "", "secret", "token", "callback", "", "")
	wechatOfficialPassiveReplyWait = 10 * time.Millisecond
	defer func() { wechatOfficialPassiveReplyWait = 2 * time.Second }()
	messages := make(chan *types.Message, 1)
	adapter.SetMessageHandler(func(msg *types.Message) { messages <- msg })
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	query := url.Values{}
	query.Set("timestamp", timestamp)
	query.Set("nonce", "legacy")
	query.Set("signature", testWeChatSignature("token", timestamp, "legacy"))
	request := httptest.NewRequest(http.MethodPost, "/?"+query.Encode(), strings.NewReader(testWeChatMessageXML("gh_legacy", "legacy-message")))
	response := httptest.NewRecorder()

	adapter.HandleHTTPCallback("callback", response, request)

	select {
	case <-messages:
	case <-time.After(time.Second):
		t.Fatal("未配置原始 ID 的旧部署消息未派发")
	}
}

func TestReplyAndSendTarget(t *testing.T) {
	adapter := NewWeChatOfficialAdapter("app", "gh_app", "secret", "token", "", "", "")
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

	adapter := NewWeChatOfficialAdapter("app", "gh_app", "secret", "token", "", server.URL, server.URL+"/token")
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

func TestPostCallbackUsesPassiveTextReply(t *testing.T) {
	adapter := NewWeChatOfficialAdapter("app", "gh_app", "secret", "token", "callback", "", "")
	adapter.SetMessageHandler(func(msg *types.Message) {
		if err := adapter.SendMessage(msg.UserID, "你好 <allbot>"); err != nil {
			t.Errorf("SendMessage returned error: %v", err)
		}
	})
	query := url.Values{}
	query.Set("timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	query.Set("nonce", "nonce")
	query.Set("signature", testWeChatSignature("token", query.Get("timestamp"), "nonce"))
	request := httptest.NewRequest(http.MethodPost, "/?"+query.Encode(), strings.NewReader(testWeChatMessageXML("gh_app", "9")))
	response := httptest.NewRecorder()

	adapter.HandleHTTPCallback("callback", response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	body := response.Body.String()
	for _, want := range []string{"<ToUserName>openid</ToUserName>", "<FromUserName>gh_app</FromUserName>", "<MsgType>text</MsgType>", "<Content>你好 &lt;allbot&gt;</Content>"} {
		if !strings.Contains(body, want) {
			t.Fatalf("passive reply missing %q in %s", want, body)
		}
	}
}

func TestPostCallbackMergesPassiveReplies(t *testing.T) {
	adapter := NewWeChatOfficialAdapter("app", "gh_app", "secret", "token", "callback", "", "")
	wechatOfficialPassiveReplyWait = 20 * time.Millisecond
	defer func() { wechatOfficialPassiveReplyWait = 2 * time.Second }()
	adapter.SetMessageHandler(func(msg *types.Message) {
		if err := adapter.SendMessage(msg.UserID, "正在加载二维码，请稍候..."); err != nil {
			t.Errorf("SendMessage returned error: %v", err)
		}
		if err := adapter.SendImage(msg.UserID, "https://example.com/qrcode.png"); err != nil {
			t.Errorf("SendImage returned error: %v", err)
		}
	})
	query := url.Values{}
	query.Set("timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	query.Set("nonce", "nonce")
	query.Set("signature", testWeChatSignature("token", query.Get("timestamp"), "nonce"))
	request := httptest.NewRequest(http.MethodPost, "/?"+query.Encode(), strings.NewReader(testWeChatMessageXML("gh_app", "9")))
	response := httptest.NewRecorder()

	adapter.HandleHTTPCallback("callback", response, request)

	body := response.Body.String()
	for _, want := range []string{"正在加载二维码，请稍候...", "暂不支持图片，点击链接-&gt;https://example.com/qrcode.png"} {
		if !strings.Contains(body, want) {
			t.Fatalf("passive reply missing %q in %s", want, body)
		}
	}
}

func TestSendRichMessageUsesImageURLPrompt(t *testing.T) {
	adapter := NewWeChatOfficialAdapter("app", "gh_app", "secret", "token", "callback", "", "")
	adapter.SetMessageHandler(func(msg *types.Message) {
		err := adapter.SendRichMessage(msg.UserID, types.RichMessage{Parts: []types.RichMessagePart{
			{Type: "text", Text: "请使用微信扫描二维码登录"},
			{Type: "image", URL: "https://example.com/qrcode.png", Alt: "朴朴微信登录二维码"},
		}})
		if err != nil {
			t.Errorf("SendRichMessage returned error: %v", err)
		}
	})
	query := url.Values{}
	query.Set("timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	query.Set("nonce", "nonce")
	query.Set("signature", testWeChatSignature("token", query.Get("timestamp"), "nonce"))
	request := httptest.NewRequest(http.MethodPost, "/?"+query.Encode(), strings.NewReader(testWeChatMessageXML("gh_app", "9")))
	response := httptest.NewRecorder()

	adapter.HandleHTTPCallback("callback", response, request)

	body := response.Body.String()
	for _, want := range []string{"请使用微信扫描二维码登录", "暂不支持图片，点击链接-&gt;https://example.com/qrcode.png"} {
		if !strings.Contains(body, want) {
			t.Fatalf("rich reply missing %q in %s", want, body)
		}
	}
	if strings.Contains(body, "朴朴微信登录二维码") {
		t.Fatalf("rich reply should not use image alt as fallback: %s", body)
	}
}

func TestSendImageSendsImageURLAsText(t *testing.T) {
	adapter := NewWeChatOfficialAdapter("app", "gh_app", "secret", "token", "callback", "", "")
	adapter.SetMessageHandler(func(msg *types.Message) {
		if err := adapter.SendImage(msg.UserID, "https://example.com/a.png"); err != nil {
			t.Errorf("SendImage returned error: %v", err)
		}
	})
	query := url.Values{}
	query.Set("timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	query.Set("nonce", "nonce")
	query.Set("signature", testWeChatSignature("token", query.Get("timestamp"), "nonce"))
	request := httptest.NewRequest(http.MethodPost, "/?"+query.Encode(), strings.NewReader(testWeChatMessageXML("gh_app", "9")))
	response := httptest.NewRecorder()

	adapter.HandleHTTPCallback("callback", response, request)

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "<Content>暂不支持图片，点击链接-&gt;https://example.com/a.png</Content>") {
		t.Fatalf("status = %d body = %q", response.Code, response.Body.String())
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

	adapter := NewWeChatOfficialAdapter("app", "gh_app", "secret", "token", "", server.URL, server.URL+"/token")
	if err := adapter.SendMessage("openid", "hello"); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}
	if atomic.LoadInt32(&sendCalls) != 1 {
		t.Fatalf("sendCalls = %d", sendCalls)
	}
}

func TestPostCallbackReturnsBeforeSlowHandler(t *testing.T) {
	adapter := NewWeChatOfficialAdapter("app", "gh_app", "secret", "token", "callback", "", "")
	wechatOfficialPassiveReplyWait = 10 * time.Millisecond
	defer func() { wechatOfficialPassiveReplyWait = 2 * time.Second }()
	started := make(chan struct{})
	release := make(chan struct{})
	adapter.SetMessageHandler(func(msg *types.Message) {
		close(started)
		<-release
	})
	query := url.Values{}
	query.Set("timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	query.Set("nonce", "nonce")
	query.Set("signature", testWeChatSignature("token", query.Get("timestamp"), "nonce"))
	request := httptest.NewRequest(http.MethodPost, "/?"+query.Encode(), strings.NewReader(testWeChatMessageXML("gh_app", "9")))
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
	adapter := NewWeChatOfficialAdapter("app", "gh_app", "secret", "token", "callback", "", "")
	request := httptest.NewRequest(http.MethodPost, "/?timestamp=123&nonce=nonce&signature=bad", strings.NewReader(`<xml></xml>`))
	response := httptest.NewRecorder()
	adapter.HandleHTTPCallback("callback", response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("bad signature status = %d", response.Code)
	}

	query := url.Values{}
	query.Set("timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	query.Set("nonce", "nonce")
	query.Set("signature", testWeChatSignature("token", query.Get("timestamp"), "nonce"))
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

			adapter := NewWeChatOfficialAdapter("app", "gh_app", "secret", "token", "", server.URL, server.URL+"/token")
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

	adapter := NewWeChatOfficialAdapter("app", "gh_app", "secret", "token", "", server.URL, server.URL+"/token")
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

	adapter := NewWeChatOfficialAdapter("app", "gh_app", "secret", "token", "", server.URL, server.URL+"/token")
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

func testWeChatMessageXML(toUserName, messageID string) string {
	return `<xml><ToUserName><![CDATA[` + toUserName + `]]></ToUserName><FromUserName><![CDATA[openid]]></FromUserName><CreateTime>` + strconv.FormatInt(time.Now().Unix(), 10) + `</CreateTime><MsgType><![CDATA[text]]></MsgType><Content><![CDATA[ping]]></Content><MsgId>` + messageID + `</MsgId></xml>`
}

func testWeChatSignature(token, timestamp, nonce string) string {
	values := []string{token, timestamp, nonce}
	sort.Strings(values)
	sum := sha1.Sum([]byte(strings.Join(values, "")))
	return hex.EncodeToString(sum[:])
}
