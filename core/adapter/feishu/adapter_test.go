package feishu

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/allbot/allbot/core/adapter/_contract"
	"github.com/allbot/allbot/core/types"
	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestFeishuAdapterImplementsContracts(t *testing.T) {
	adapter := NewFeishuAdapter("app", "secret", "token", "", "", "", "")
	var _ contract.Adapter = adapter
	var _ contract.HTTPCallbackHandler = adapter
	var _ contract.ReplyTargetResolver = adapter
	var _ contract.SendTargetResolver = adapter
	var _ contract.ReplyTextFormatter = adapter
	var _ contract.MarkdownSender = adapter
	var _ contract.BotIdentityProvider = adapter
}

func TestFeishuBotIdentityReturnsOnlyAppID(t *testing.T) {
	adapter := NewFeishuAdapter("app-id", "app-secret", "token", "", "", "", "")
	identity := adapter.GetBotIdentity(nil)
	if identity.Label != "机器人 App ID" || identity.Value != "app-id" {
		t.Fatalf("identity = %#v", identity)
	}
	if strings.Contains(identity.Value, "app-secret") {
		t.Fatalf("identity leaked app secret: %#v", identity)
	}
}

func TestParseConfigForRegistry(t *testing.T) {
	parsed, err := parseConfigForRegistry(`{"app_id":" app ","app_secret":" secret ","verification_token":" token ","encrypt_key":" key ","callback_path":"/feishu/callback/","api_base_url":" https://example.com/open-apis ","token_url":" https://example.com/token "}`)
	if err != nil {
		t.Fatalf("parseConfigForRegistry returned error: %v", err)
	}
	config := parsed.(*Config)
	if config.AppID != "app" || config.AppSecret != "secret" || config.VerificationToken != "token" || config.EncryptKey != "key" {
		t.Fatalf("config trim failed: %#v", config)
	}
	if config.CallbackPath != "feishu/callback" || config.APIBaseURL != "https://example.com/open-apis" || config.TokenURL != "https://example.com/token" {
		t.Fatalf("config defaults failed: %#v", config)
	}

	parsed, err = parseConfigForRegistry(`{"app_id":"app","app_secret":"secret","verification_token":"token"}`)
	if err != nil {
		t.Fatalf("parseConfigForRegistry returned error: %v", err)
	}
	if parsed.(*Config).CallbackPath != feishuDefaultCallbackPath {
		t.Fatalf("default CallbackPath = %q", parsed.(*Config).CallbackPath)
	}

	for _, raw := range []string{
		`{"app_secret":"secret","verification_token":"token"}`,
		`{"app_id":"app","verification_token":"token"}`,
		`{`,
	} {
		if _, err := parseConfigForRegistry(raw); err == nil {
			t.Fatalf("expected error for %s", raw)
		}
	}
	if _, err := newAdapterFromRegistry(Config{}); err == nil {
		t.Fatal("expected config type error")
	}
}

func TestFeishuStartValidatesRequiredConfig(t *testing.T) {
	for _, adapter := range []*FeishuAdapter{
		NewFeishuAdapter("", "secret", "token", "", "", "", ""),
		NewFeishuAdapter("app", "", "token", "", "", "", ""),
	} {
		if err := adapter.Start(); err == nil {
			t.Fatalf("expected Start error for %#v", adapter)
		}
	}
	adapter := NewFeishuAdapter("app", "secret", "", "", "", "", "")
	var started int32
	adapter.startLongConnectionFunc = func() error {
		atomic.AddInt32(&started, 1)
		return nil
	}
	if err := adapter.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if atomic.LoadInt32(&started) != 1 {
		t.Fatalf("started = %d", started)
	}
	if err := adapter.Stop(); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
	if err := adapter.Stop(); err != nil {
		t.Fatalf("second Stop returned error: %v", err)
	}
}

func TestFeishuTargetParsing(t *testing.T) {
	tests := []struct {
		target string
		kind   string
		id     string
	}{
		{target: "user_ou_x", kind: "open_id", id: "ou_x"},
		{target: "chat_oc_x", kind: "chat_id", id: "oc_x"},
		{target: "reply_om_x", kind: "reply", id: "om_x"},
	}
	for _, tt := range tests {
		got, err := parseFeishuMessageTarget(tt.target)
		if err != nil {
			t.Fatalf("parseFeishuMessageTarget(%q) returned error: %v", tt.target, err)
		}
		if got.kind != tt.kind || got.id != tt.id {
			t.Fatalf("target = %#v", got)
		}
	}
	for _, target := range []string{"", "user_", "chat_", "reply_", "open_id"} {
		if _, err := parseFeishuMessageTarget(target); err == nil {
			t.Fatalf("expected error for target %q", target)
		}
	}
}

func TestFeishuReplyAndSendTarget(t *testing.T) {
	adapter := NewFeishuAdapter("app", "secret", "token", "", "", "", "")
	msg := &types.Message{UserID: "ou_user", GroupID: "oc_chat", Metadata: map[string]string{"reply_target": "reply_om_1"}}
	if got := adapter.ReplyTarget(msg); got != "reply_om_1" {
		t.Fatalf("ReplyTarget = %q", got)
	}
	msg.Metadata = map[string]string{"feishu_message_id": "om_2"}
	if got := adapter.ReplyTarget(msg); got != "reply_om_2" {
		t.Fatalf("ReplyTarget = %q", got)
	}
	msg.Metadata = nil
	if got := adapter.ReplyTarget(msg); got != "chat_oc_chat" {
		t.Fatalf("ReplyTarget = %q", got)
	}
	msg.GroupID = ""
	if got := adapter.ReplyTarget(msg); got != "user_ou_user" {
		t.Fatalf("ReplyTarget = %q", got)
	}
	if got := adapter.SendTarget("ou_user", "oc_chat"); got != "chat_oc_chat" {
		t.Fatalf("SendTarget group = %q", got)
	}
	if got := adapter.SendTarget("user_ou_user", ""); got != "user_ou_user" {
		t.Fatalf("SendTarget prefixed = %q", got)
	}
	if got := adapter.SendTarget("", "reply_om_1"); got != "reply_om_1" {
		t.Fatalf("SendTarget reply = %q", got)
	}
	if got := adapter.FormatReplyText(msg, "hello"); got != "hello" {
		t.Fatalf("FormatReplyText = %q", got)
	}
}

func TestFeishuURLVerification(t *testing.T) {
	adapter := NewFeishuAdapter("app", "secret", "token", "", "callback", "", "")
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"type":"url_verification","token":"token","challenge":"challenge-value"}`))

	adapter.HandleHTTPCallback("callback", response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if body["challenge"] != "challenge-value" {
		t.Fatalf("body = %#v", body)
	}

	response = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"type":"url_verification","token":"bad","challenge":"challenge-value"}`))
	adapter.HandleHTTPCallback("callback", response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("bad token status = %d", response.Code)
	}
}

func TestFeishuMessageCallbackDispatchesPrivateText(t *testing.T) {
	adapter := NewFeishuAdapter("app", "secret", "token", "", "callback", "", "")
	messages := make(chan *types.Message, 1)
	adapter.SetMessageHandler(func(msg *types.Message) { messages <- msg })
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(feishuMessagePayload("p2p", "oc_private", `{"text":" 你好 "}`)))

	adapter.HandleHTTPCallback("callback", response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %q", response.Code, response.Body.String())
	}
	select {
	case msg := <-messages:
		assertFeishuMessage(t, msg, "private", "", "你好")
	case <-time.After(time.Second):
		t.Fatal("message was not dispatched")
	}
}

func TestFeishuMessageCallbackDispatchesGroupText(t *testing.T) {
	adapter := NewFeishuAdapter("app", "secret", "token", "", "callback", "", "")
	messages := make(chan *types.Message, 1)
	adapter.SetMessageHandler(func(msg *types.Message) { messages <- msg })
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(feishuMessagePayload("group", "oc_group", `{"text":"群消息"}`)))

	adapter.HandleHTTPCallback("callback", response, request)

	select {
	case msg := <-messages:
		assertFeishuMessage(t, msg, "group", "oc_group", "群消息")
	case <-time.After(time.Second):
		t.Fatal("message was not dispatched")
	}
}

func TestFeishuMessageCallbackRejectsStaleReplayAndTenantMismatch(t *testing.T) {
	adapter := NewFeishuAdapter("app", "secret", "token", "", "callback", "", "")
	messages := make(chan *types.Message, 3)
	adapter.SetMessageHandler(func(msg *types.Message) { messages <- msg })

	send := func(payload string) {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
		adapter.HandleHTTPCallback("callback", response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d body = %q", response.Code, response.Body.String())
		}
	}

	stale := feishuMessagePayloadAt("p2p", "oc_private", `{"text":"过期"}`, time.Now().Add(-feishuCallbackFreshness-time.Second))
	send(stale)
	select {
	case <-messages:
		t.Fatal("过期事件不应派发")
	case <-time.After(30 * time.Millisecond):
	}

	mismatch := strings.Replace(feishuMessagePayload("p2p", "oc_private", `{"text":"租户错误"}`), `"tenant_key":"tenant"`, `"tenant_key":"other"`, 1)
	send(mismatch)
	select {
	case <-messages:
		t.Fatal("租户不一致事件不应派发")
	case <-time.After(30 * time.Millisecond):
	}

	payload := feishuMessagePayload("p2p", "oc_private", `{"text":"首次"}`)
	send(payload)
	select {
	case <-messages:
	case <-time.After(time.Second):
		t.Fatal("首次事件未派发")
	}
	send(payload)
	select {
	case <-messages:
		t.Fatal("重复事件不应派发")
	case <-time.After(30 * time.Millisecond):
	}

	sameEvent := strings.Replace(feishuMessagePayload("p2p", "oc_private", `{"text":"事件重复"}`), `"message_id":"om_msg"`, `"message_id":"om_other"`, 1)
	send(sameEvent)
	select {
	case <-messages:
		t.Fatal("相同 EventID 不应因 MessageID 不同而再次派发")
	case <-time.After(30 * time.Millisecond):
	}

	sameMessage := strings.Replace(feishuMessagePayload("p2p", "oc_private", `{"text":"消息重复"}`), `"event_id":"evt_1"`, `"event_id":"evt_other"`, 1)
	send(sameMessage)
	select {
	case <-messages:
		t.Fatal("相同 MessageID 不应因 EventID 不同而再次派发")
	case <-time.After(30 * time.Millisecond):
	}
}

func TestFeishuCallbackRejectsOversizedBody(t *testing.T) {
	adapter := NewFeishuAdapter("app", "secret", "token", "", "callback", "", "")
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", feishuCallbackBodyLimit+1)))

	adapter.HandleHTTPCallback("callback", response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestFeishuLongConnectionEventDispatchesMessage(t *testing.T) {
	adapter := NewFeishuAdapter("app", "secret", "", "", "callback", "", "")
	messages := make(chan *types.Message, 1)
	adapter.SetMessageHandler(func(msg *types.Message) { messages <- msg })
	messageID := "om_msg"
	chatID := "oc_group"
	chatType := "group"
	messageType := "text"
	content := `{"text":"长连接消息"}`
	openID := "ou_open"
	userID := "user_id"
	unionID := "union_id"
	tenantKey := "tenant"

	adapter.handleP2MessageReceiveV1(&larkim.P2MessageReceiveV1{
		EventV2Base: &larkevent.EventV2Base{Header: &larkevent.EventHeader{EventID: "evt_1", EventType: "im.message.receive_v1", TenantKey: tenantKey}},
		Event: &larkim.P2MessageReceiveV1Data{
			Sender:  &larkim.EventSender{SenderId: &larkim.UserId{OpenId: &openID, UserId: &userID, UnionId: &unionID}, TenantKey: &tenantKey},
			Message: &larkim.EventMessage{MessageId: &messageID, ChatId: &chatID, ChatType: &chatType, MessageType: &messageType, Content: &content},
		},
	})

	select {
	case msg := <-messages:
		assertFeishuMessage(t, msg, "group", "oc_group", "长连接消息")
	case <-time.After(time.Second):
		t.Fatal("message was not dispatched")
	}
}

func TestFeishuMessageCallbackIgnoresUnsupportedMessages(t *testing.T) {
	adapter := NewFeishuAdapter("app", "secret", "token", "", "callback", "", "")
	messages := make(chan *types.Message, 1)
	adapter.SetMessageHandler(func(msg *types.Message) { messages <- msg })
	for _, payload := range []string{
		feishuMessagePayload("p2p", "oc_private", `{"text":"   "}`),
		strings.Replace(feishuMessagePayload("p2p", "oc_private", `{"text":"hello"}`), `"message_type":"text"`, `"message_type":"image"`, 1),
		feishuMessagePayload("p2p", "oc_private", `{`),
		`{"schema":"2.0","header":{"token":"token","event_type":"other.event"},"event":{}}`,
	} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
		adapter.HandleHTTPCallback("callback", response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d body = %q", response.Code, response.Body.String())
		}
	}
	select {
	case msg := <-messages:
		t.Fatalf("unexpected message: %#v", msg)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestFeishuCallbackRejectsBadRequests(t *testing.T) {
	adapter := NewFeishuAdapter("app", "secret", "token", "", "callback", "", "")
	tests := []struct {
		path   string
		method string
		body   string
		status int
	}{
		{path: "other", method: http.MethodPost, body: `{}`, status: http.StatusNotFound},
		{path: "callback", method: http.MethodGet, body: `{}`, status: http.StatusMethodNotAllowed},
		{path: "callback", method: http.MethodPost, body: `{`, status: http.StatusBadRequest},
		{path: "callback", method: http.MethodPost, body: `{"encrypt":"abc","token":"token"}`, status: http.StatusBadRequest},
	}
	for _, tt := range tests {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(tt.method, "/", strings.NewReader(tt.body))
		adapter.HandleHTTPCallback(tt.path, response, request)
		if response.Code != tt.status {
			t.Fatalf("status = %d expected %d for %#v", response.Code, tt.status, tt)
		}
	}
}

func TestFeishuTenantAccessTokenCacheAndRefresh(t *testing.T) {
	var tokenCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		atomic.AddInt32(&tokenCalls, 1)
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll returned error: %v", err)
		}
		var body map[string]string
		if err := json.Unmarshal(payload, &body); err != nil {
			t.Fatalf("json.Unmarshal returned error: %v", err)
		}
		if body["app_id"] != "app" || body["app_secret"] != "secret" {
			t.Fatalf("body = %#v", body)
		}
		_, _ = w.Write([]byte(`{"code":0,"msg":"ok","tenant_access_token":"tenant-token","expire":7200}`))
	}))
	defer server.Close()

	adapter := NewFeishuAdapter("app", "secret", "token", "", "", server.URL, server.URL+"/token")
	first, err := adapter.getTenantAccessToken()
	if err != nil {
		t.Fatalf("getTenantAccessToken returned error: %v", err)
	}
	second, err := adapter.getTenantAccessToken()
	if err != nil {
		t.Fatalf("getTenantAccessToken returned error: %v", err)
	}
	if first != "tenant-token" || second != "tenant-token" || atomic.LoadInt32(&tokenCalls) != 1 {
		t.Fatalf("tokens = %q/%q calls = %d", first, second, tokenCalls)
	}

	adapter.tokenExpiresAt = time.Now().Add(time.Minute)
	if _, err := adapter.getTenantAccessToken(); err != nil {
		t.Fatalf("refresh getTenantAccessToken returned error: %v", err)
	}
	if atomic.LoadInt32(&tokenCalls) != 2 {
		t.Fatalf("tokenCalls = %d", tokenCalls)
	}
}

func TestFeishuSendMessagePathsAndBodies(t *testing.T) {
	for _, tt := range []struct {
		target        string
		expectedPath  string
		expectedQuery string
	}{
		{target: "user_ou_user", expectedPath: "/im/v1/messages", expectedQuery: "open_id"},
		{target: "chat_oc_chat", expectedPath: "/im/v1/messages", expectedQuery: "chat_id"},
		{target: "reply_om_msg", expectedPath: "/im/v1/messages/om_msg/reply"},
	} {
		t.Run(tt.target, func(t *testing.T) {
			server := newFeishuSendTestServer(t, func(r *http.Request, body map[string]interface{}) {
				if r.URL.Path != tt.expectedPath {
					t.Fatalf("path = %s expected %s", r.URL.Path, tt.expectedPath)
				}
				if tt.expectedQuery != "" && r.URL.Query().Get("receive_id_type") != tt.expectedQuery {
					t.Fatalf("receive_id_type = %q", r.URL.Query().Get("receive_id_type"))
				}
				if r.Header.Get("Authorization") != "Bearer tenant-token" {
					t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
				}
				if body["msg_type"] != "text" {
					t.Fatalf("body = %#v", body)
				}
				var content map[string]string
				if err := json.Unmarshal([]byte(body["content"].(string)), &content); err != nil {
					t.Fatalf("content json.Unmarshal returned error: %v", err)
				}
				if content["text"] != "hello" {
					t.Fatalf("content = %#v", content)
				}
			})
			defer server.Close()
			adapter := NewFeishuAdapter("app", "secret", "token", "", "", server.URL, server.URL+"/token")
			if err := adapter.SendMessage(tt.target, "hello"); err != nil {
				t.Fatalf("SendMessage returned error: %v", err)
			}
		})
	}
}

func TestFeishuSendMarkdownUsesInteractiveCard(t *testing.T) {
	server := newFeishuSendTestServer(t, func(r *http.Request, body map[string]interface{}) {
		if body["msg_type"] != "interactive" {
			t.Fatalf("body = %#v", body)
		}
		var content map[string]interface{}
		if err := json.Unmarshal([]byte(body["content"].(string)), &content); err != nil {
			t.Fatalf("content json.Unmarshal returned error: %v", err)
		}
		elements, ok := content["elements"].([]interface{})
		if !ok || len(elements) != 1 {
			t.Fatalf("content = %#v", content)
		}
		markdownElement, ok := elements[0].(map[string]interface{})
		if !ok || markdownElement["tag"] != "markdown" || markdownElement["content"] != "**hello**" {
			t.Fatalf("markdown element = %#v", markdownElement)
		}
	})
	defer server.Close()
	adapter := NewFeishuAdapter("app", "secret", "token", "", "", server.URL, server.URL+"/token")
	if err := adapter.SendMarkdown("chat_oc_chat", "**hello**"); err != nil {
		t.Fatalf("SendMarkdown returned error: %v", err)
	}
}

func TestFeishuSendImageUploadsLocalFileAndSendsImage(t *testing.T) {
	imageFile, err := os.CreateTemp(t.TempDir(), "feishu-*.png")
	if err != nil {
		t.Fatalf("CreateTemp returned error: %v", err)
	}
	if _, err := imageFile.Write([]byte("image-bytes")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if err := imageFile.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	server := newFeishuImageSendTestServer(t, []byte("image-bytes"), func(r *http.Request, body map[string]interface{}) {
		if r.URL.Path != "/im/v1/messages" || r.URL.Query().Get("receive_id_type") != "chat_id" {
			t.Fatalf("url = %s", r.URL.String())
		}
		if body["msg_type"] != "image" || body["receive_id"] != "oc_chat" {
			t.Fatalf("body = %#v", body)
		}
		var content map[string]string
		if err := json.Unmarshal([]byte(body["content"].(string)), &content); err != nil {
			t.Fatalf("content json.Unmarshal returned error: %v", err)
		}
		if content["image_key"] != "img_key" {
			t.Fatalf("content = %#v", content)
		}
	})
	defer server.Close()
	adapter := NewFeishuAdapter("app", "secret", "token", "", "", server.URL, server.URL+"/token")
	if err := adapter.SendImage("chat_oc_chat", imageFile.Name()); err != nil {
		t.Fatalf("SendImage returned error: %v", err)
	}
}

func TestFeishuSendImageUploadsRemoteURLAndReplies(t *testing.T) {
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("remote-image"))
	}))
	defer imageServer.Close()
	server := newFeishuImageSendTestServer(t, []byte("remote-image"), func(r *http.Request, body map[string]interface{}) {
		if r.URL.Path != "/im/v1/messages/om_msg/reply" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if body["msg_type"] != "image" {
			t.Fatalf("body = %#v", body)
		}
	})
	defer server.Close()
	adapter := NewFeishuAdapter("app", "secret", "token", "", "", server.URL, server.URL+"/token")
	if err := adapter.SendImage("reply_om_msg", imageServer.URL+"/image.png"); err != nil {
		t.Fatalf("SendImage returned error: %v", err)
	}
}

func TestFeishuSendImageRejectsEmptyInput(t *testing.T) {
	adapter := NewFeishuAdapter("app", "secret", "token", "", "", "", "")
	if err := adapter.SendImage("chat_oc_chat", " "); err == nil || !strings.Contains(err.Error(), "图片地址不能为空") {
		t.Fatalf("empty image err = %v", err)
	}
	if err := adapter.SendImage("bad", "image.png"); err == nil || !strings.Contains(err.Error(), "目标格式无效") {
		t.Fatalf("bad target err = %v", err)
	}
}

func TestFeishuSendMessageRefreshesTokenOnce(t *testing.T) {
	var tokenCalls int32
	var sendCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			call := atomic.AddInt32(&tokenCalls, 1)
			_, _ = w.Write([]byte(`{"code":0,"tenant_access_token":"tenant-` + strconv.Itoa(int(call)) + `","expire":7200}`))
		case "/im/v1/messages":
			call := atomic.AddInt32(&sendCalls, 1)
			if call == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"code":99991663,"msg":"token expired"}`))
				return
			}
			if r.Header.Get("Authorization") != "Bearer tenant-2" {
				t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
			}
			_, _ = w.Write([]byte(`{"code":0,"msg":"ok"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	adapter := NewFeishuAdapter("app", "secret", "token", "", "", server.URL, server.URL+"/token")
	if err := adapter.SendMessage("chat_oc_chat", "hello"); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}
	if atomic.LoadInt32(&tokenCalls) != 2 || atomic.LoadInt32(&sendCalls) != 2 {
		t.Fatalf("tokenCalls = %d sendCalls = %d", tokenCalls, sendCalls)
	}
}

func TestFeishuUnsupportedCapabilitiesReturnErrors(t *testing.T) {
	adapter := NewFeishuAdapter("app", "secret", "token", "", "", "", "")
	for name, err := range map[string]error{
		"SendFile": adapter.SendFile("target", "file"),
		"AtUser":   adapter.AtUser("group", "user"),
	} {
		if err == nil || !strings.Contains(err.Error(), "暂未实现") {
			t.Fatalf("%s err = %v", name, err)
		}
	}
}

func newFeishuSendTestServer(t *testing.T, check func(*http.Request, map[string]interface{})) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_, _ = w.Write([]byte(`{"code":0,"msg":"ok","tenant_access_token":"tenant-token","expire":7200}`))
		case "/im/v1/messages", "/im/v1/messages/om_msg/reply":
			payload, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll returned error: %v", err)
			}
			var body map[string]interface{}
			if err := json.Unmarshal(payload, &body); err != nil {
				t.Fatalf("json.Unmarshal returned error: %v", err)
			}
			check(r, body)
			_, _ = w.Write([]byte(`{"code":0,"msg":"ok"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
}

func newFeishuImageSendTestServer(t *testing.T, expectedImage []byte, checkSend func(*http.Request, map[string]interface{})) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_, _ = w.Write([]byte(`{"code":0,"msg":"ok","tenant_access_token":"tenant-token","expire":7200}`))
		case "/im/v1/images":
			if r.Header.Get("Authorization") != "Bearer tenant-token" {
				t.Fatalf("upload Authorization = %q", r.Header.Get("Authorization"))
			}
			if err := r.ParseMultipartForm(10 << 20); err != nil {
				t.Fatalf("ParseMultipartForm returned error: %v", err)
			}
			if r.FormValue("image_type") != "message" {
				t.Fatalf("image_type = %q", r.FormValue("image_type"))
			}
			file, header, err := r.FormFile("image")
			if err != nil {
				t.Fatalf("FormFile returned error: %v", err)
			}
			defer file.Close()
			if header.Filename == "" {
				t.Fatal("filename is empty")
			}
			payload, err := io.ReadAll(file)
			if err != nil {
				t.Fatalf("ReadAll returned error: %v", err)
			}
			if string(payload) != string(expectedImage) {
				t.Fatalf("image payload = %q", payload)
			}
			_, _ = w.Write([]byte(`{"code":0,"msg":"ok","data":{"image_key":"img_key"}}`))
		case "/im/v1/messages", "/im/v1/messages/om_msg/reply":
			if r.Header.Get("Authorization") != "Bearer tenant-token" {
				t.Fatalf("send Authorization = %q", r.Header.Get("Authorization"))
			}
			payload, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll returned error: %v", err)
			}
			var body map[string]interface{}
			if err := json.Unmarshal(payload, &body); err != nil {
				t.Fatalf("json.Unmarshal returned error: %v", err)
			}
			checkSend(r, body)
			_, _ = w.Write([]byte(`{"code":0,"msg":"ok"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
}

func feishuMessagePayload(chatType, chatID, content string) string {
	return feishuMessagePayloadAt(chatType, chatID, content, time.Now())
}

func feishuMessagePayloadAt(chatType, chatID, content string, createdAt time.Time) string {
	createTime := strconv.FormatInt(createdAt.UnixMilli(), 10)
	payload := map[string]interface{}{
		"schema": "2.0",
		"header": map[string]string{
			"event_id":    "evt_1",
			"event_type":  "im.message.receive_v1",
			"tenant_key":  "tenant",
			"token":       "token",
			"create_time": createTime,
		},
		"event": map[string]interface{}{
			"sender": map[string]interface{}{
				"sender_id":  map[string]string{"open_id": "ou_open", "user_id": "user_id", "union_id": "union_id"},
				"tenant_key": "tenant",
			},
			"message": map[string]string{
				"message_id":   "om_msg",
				"create_time":  createTime,
				"chat_id":      chatID,
				"chat_type":    chatType,
				"message_type": "text",
				"content":      content,
			},
		},
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

func assertFeishuMessage(t *testing.T, msg *types.Message, messageType string, groupID string, content string) {
	t.Helper()
	if msg == nil {
		t.Fatal("message is nil")
	}
	if msg.ID != "om_msg" || msg.Platform != platformName || msg.UserID != "ou_open" || msg.GroupID != groupID || msg.Content != content {
		t.Fatalf("message = %#v", msg)
	}
	if msg.Metadata["message_type"] != messageType || msg.Metadata["reply_target"] != "reply_om_msg" || msg.Metadata["feishu_chat_id"] == "" || msg.Metadata["feishu_sender_open_id"] != "ou_open" {
		t.Fatalf("metadata = %#v", msg.Metadata)
	}
}
