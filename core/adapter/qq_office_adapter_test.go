package adapter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/allbot/allbot/core/types"
)

func TestQQOfficeGetAccessTokenCachesToken(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, expected POST", r.Method)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode token body failed: %v", err)
		}
		if body["appId"] != "app123" || body["clientSecret"] != "secret456" {
			t.Fatalf("token body = %#v", body)
		}
		atomic.AddInt32(&calls, 1)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "test-token", "expires_in": 7200})
	}))
	defer server.Close()

	adp := NewQQOfficeAdapter("app123", "secret456", "", server.URL)
	first, err := adp.getAccessToken()
	if err != nil {
		t.Fatalf("first getAccessToken returned error: %v", err)
	}
	second, err := adp.getAccessToken()
	if err != nil {
		t.Fatalf("second getAccessToken returned error: %v", err)
	}
	if first != "test-token" || second != "test-token" {
		t.Fatalf("tokens = %q/%q, expected test-token", first, second)
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("token endpoint calls = %d, expected 1", calls)
	}
}

func TestQQOfficeGetAccessTokenRefreshesWithinOfficialWindow(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := atomic.AddInt32(&calls, 1)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "test-token-" + string(rune('0'+call)), "expires_in": 30})
	}))
	defer server.Close()

	adp := NewQQOfficeAdapter("app123", "secret456", "", server.URL)
	first, err := adp.getAccessToken()
	if err != nil {
		t.Fatalf("first getAccessToken returned error: %v", err)
	}
	second, err := adp.getAccessToken()
	if err != nil {
		t.Fatalf("second getAccessToken returned error: %v", err)
	}
	if first == second {
		t.Fatalf("expected token refresh inside official 60s window, got %q twice", first)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("token endpoint calls = %d, expected 2", calls)
	}
}

func TestQQOfficeGetAccessTokenRequiresOfficialResponseFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 11253, "message": "invalid app secret"})
	}))
	defer server.Close()

	adp := NewQQOfficeAdapter("app123", "secret456", "", server.URL)
	if _, err := adp.getAccessToken(); err == nil || !strings.Contains(err.Error(), "code=11253") || !strings.Contains(err.Error(), "message=invalid app secret") {
		t.Fatalf("error = %v, expected token error summary", err)
	}
}

func TestQQOfficeSendMessagePostsDMS(t *testing.T) {
	var tokenCalls int32
	var messageCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			atomic.AddInt32(&tokenCalls, 1)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "test-token", "expires_in": 7200})
		case "/dms/guild123/messages":
			atomic.AddInt32(&messageCalls, 1)
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, expected POST", r.Method)
			}
			if got := r.Header.Get("Authorization"); got != "QQBot test-token" {
				t.Fatalf("Authorization = %q", got)
			}
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode message body failed: %v", err)
			}
			if body["content"] != "你好" || body["msg_id"] != "msg456" {
				t.Fatalf("message body = %#v", body)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	adp := NewQQOfficeAdapter("app123", "secret456", server.URL, server.URL+"/token")
	if err := adp.SendMessage("dms_guild123|msg_msg456", "你好"); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}
	if atomic.LoadInt32(&tokenCalls) != 1 || atomic.LoadInt32(&messageCalls) != 1 {
		t.Fatalf("tokenCalls=%d messageCalls=%d, expected 1/1", tokenCalls, messageCalls)
	}
}

func TestQQOfficeSendMessageIncrementsReplySeq(t *testing.T) {
	bodies := make(chan map[string]interface{}, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "test-token", "expires_in": 7200})
		case "/v2/users/user-openid/messages":
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode message body failed: %v", err)
			}
			bodies <- body
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	adp := NewQQOfficeAdapter("app123", "secret456", server.URL, server.URL+"/token")
	for _, text := range []string{"第一条", "第二条"} {
		if err := adp.SendMessage("user_user-openid|msg_msg-c2c", text); err != nil {
			t.Fatalf("SendMessage returned error: %v", err)
		}
	}
	first := <-bodies
	second := <-bodies
	if first["msg_seq"] != float64(1) || second["msg_seq"] != float64(2) {
		t.Fatalf("msg_seq = %v/%v, expected 1/2", first["msg_seq"], second["msg_seq"])
	}
}

func TestQQOfficeSendImageUploadsAndSendsMedia(t *testing.T) {
	var uploadCalls int32
	var messageCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "test-token", "expires_in": 7200})
		case "/v2/users/user-openid/files":
			atomic.AddInt32(&uploadCalls, 1)
			if r.Method != http.MethodPost {
				t.Fatalf("upload method = %s, expected POST", r.Method)
			}
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode upload body failed: %v", err)
			}
			if body["file_type"] != float64(1) || body["url"] != "https://example.com/a.jpg" || body["srv_send_msg"] != false {
				t.Fatalf("upload body = %#v", body)
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"file_uuid": "file-uuid", "file_info": "file-info", "ttl": 3600})
		case "/v2/users/user-openid/messages":
			atomic.AddInt32(&messageCalls, 1)
			if r.Method != http.MethodPost {
				t.Fatalf("message method = %s, expected POST", r.Method)
			}
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode message body failed: %v", err)
			}
			media, ok := body["media"].(map[string]interface{})
			if !ok {
				t.Fatalf("media = %#v", body["media"])
			}
			if body["msg_type"] != float64(7) || body["msg_id"] != "msg-c2c" || body["msg_seq"] != float64(1) || media["file_info"] != "file-info" {
				t.Fatalf("message body = %#v", body)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	adp := NewQQOfficeAdapter("app123", "secret456", server.URL, server.URL+"/token")
	if err := adp.SendImage("user_user-openid|msg_msg-c2c", "https://example.com/a.jpg"); err != nil {
		t.Fatalf("SendImage returned error: %v", err)
	}
	if atomic.LoadInt32(&uploadCalls) != 1 || atomic.LoadInt32(&messageCalls) != 1 {
		t.Fatalf("uploadCalls=%d messageCalls=%d, expected 1/1", uploadCalls, messageCalls)
	}
}

func TestQQOfficeSendMessagePostsC2CAndGroup(t *testing.T) {
	var userCalls int32
	var groupCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "test-token", "expires_in": 7200})
		case "/v2/users/user-openid/messages":
			atomic.AddInt32(&userCalls, 1)
			assertQQOfficeMessageRequest(t, r, "你好", "msg-c2c")
		case "/v2/groups/group-openid/messages":
			atomic.AddInt32(&groupCalls, 1)
			assertQQOfficeMessageRequest(t, r, "群回复", "msg-group")
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	adp := NewQQOfficeAdapter("app123", "secret456", server.URL, server.URL+"/token")
	if err := adp.SendMessage("user_user-openid|msg_msg-c2c", "你好"); err != nil {
		t.Fatalf("SendMessage C2C returned error: %v", err)
	}
	if err := adp.SendMessage("group_group-openid|msg_msg-group", "群回复"); err != nil {
		t.Fatalf("SendMessage group returned error: %v", err)
	}
	if atomic.LoadInt32(&userCalls) != 1 || atomic.LoadInt32(&groupCalls) != 1 {
		t.Fatalf("userCalls=%d groupCalls=%d, expected 1/1", userCalls, groupCalls)
	}
}

func assertQQOfficeMessageRequest(t *testing.T, r *http.Request, content string, msgID string) {
	t.Helper()
	if r.Method != http.MethodPost {
		t.Fatalf("method = %s, expected POST", r.Method)
	}
	if got := r.Header.Get("Authorization"); got != "QQBot test-token" {
		t.Fatalf("Authorization = %q", got)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("decode message body failed: %v", err)
	}
	if body["content"] != content || body["msg_id"] != msgID || body["msg_type"] != float64(0) || body["msg_seq"] != float64(1) {
		t.Fatalf("message body = %#v", body)
	}
}

func TestQQOfficeParseMessageTarget(t *testing.T) {
	tests := []struct {
		name      string
		target    string
		kind      string
		id        string
		messageID string
		wantErr   string
	}{
		{name: "dms", target: "dms_guild123", kind: "dms", id: "guild123"},
		{name: "dms reply", target: "dms_guild123|msg_msg456", kind: "dms", id: "guild123", messageID: "msg456"},
		{name: "raw guild", target: "guild123", kind: "dms", id: "guild123"},
		{name: "c2c reply", target: "user_user-openid|msg_msg-c2c", kind: "user", id: "user-openid", messageID: "msg-c2c"},
		{name: "group reply", target: "group_group-openid|msg_msg-group", kind: "group", id: "group-openid", messageID: "msg-group"},
		{name: "empty msg", target: "dms_guild123|msg_", wantErr: "msg_id 不能为空"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parseQQOfficeMessageTarget(tt.target)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, expected containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseQQOfficeMessageTarget returned error: %v", err)
			}
			if parsed.kind != tt.kind || parsed.id != tt.id || parsed.msgID != tt.messageID {
				t.Fatalf("target = %+v, expected kind=%q id=%q msgID=%q", parsed, tt.kind, tt.id, tt.messageID)
			}
		})
	}
}

func TestQQOfficeHandleDirectMessageBuildsMessage(t *testing.T) {
	adp := NewQQOfficeAdapter("app123", "secret456", "", "")
	got := make(chan *types.Message, 1)
	adp.SetMessageHandler(func(msg *types.Message) { got <- msg })

	adp.handleDispatch("DIRECT_MESSAGE_CREATE", map[string]interface{}{
		"id":         "msg456",
		"guild_id":   "guild123",
		"channel_id": "channel789",
		"content":    "  你好  ",
		"author": map[string]interface{}{
			"id":       "user123",
			"username": "测试用户",
		},
	})

	msg := <-got
	if msg.Platform != "qq_office" || msg.GroupID != "" || msg.UserID != "user123" || msg.Content != "你好" {
		t.Fatalf("message = %+v", msg)
	}
	if msg.Metadata["reply_target"] != "dms_guild123|msg_msg456" {
		t.Fatalf("reply_target = %q", msg.Metadata["reply_target"])
	}
	if msg.Metadata["qq_office_channel_id"] != "channel789" || msg.Metadata["qq_office_author_name"] != "测试用户" {
		t.Fatalf("metadata = %#v", msg.Metadata)
	}
}

func TestQQOfficeHandleC2CMessageBuildsMessage(t *testing.T) {
	adp := NewQQOfficeAdapter("app123", "secret456", "", "")
	got := make(chan *types.Message, 1)
	adp.SetMessageHandler(func(msg *types.Message) { got <- msg })

	adp.handleDispatch("C2C_MESSAGE_CREATE", map[string]interface{}{
		"id":      "msg-c2c",
		"content": "  你好  ",
		"author": map[string]interface{}{
			"user_openid": "user-openid",
		},
	})

	msg := <-got
	if msg.Platform != "qq_office" || msg.GroupID != "" || msg.UserID != "user-openid" || msg.Content != "你好" {
		t.Fatalf("message = %+v", msg)
	}
	if msg.Metadata["message_type"] != "c2c" || msg.Metadata["reply_target"] != "user_user-openid|msg_msg-c2c" {
		t.Fatalf("metadata = %#v", msg.Metadata)
	}
}

func TestQQOfficeHandleGroupAtMessageBuildsMessage(t *testing.T) {
	adp := NewQQOfficeAdapter("app123", "secret456", "", "")
	got := make(chan *types.Message, 1)
	adp.SetMessageHandler(func(msg *types.Message) { got <- msg })

	adp.handleDispatch("GROUP_AT_MESSAGE_CREATE", map[string]interface{}{
		"id":           "msg-group",
		"group_openid": "group-openid",
		"content":      "  @bot 查询天气  ",
		"author": map[string]interface{}{
			"member_openid": "member-openid",
		},
	})

	msg := <-got
	if msg.Platform != "qq_office" || msg.GroupID != "group-openid" || msg.UserID != "member-openid" || msg.Content != "@bot 查询天气" {
		t.Fatalf("message = %+v", msg)
	}
	if msg.Metadata["message_type"] != "group" || msg.Metadata["reply_target"] != "group_group-openid|msg_msg-group" {
		t.Fatalf("metadata = %#v", msg.Metadata)
	}
}

func TestQQOfficeStopIsIdempotent(t *testing.T) {
	adp := NewQQOfficeAdapter("app123", "secret456", "", "")
	if err := adp.Stop(); err != nil {
		t.Fatalf("first Stop returned error: %v", err)
	}
	if err := adp.Stop(); err != nil {
		t.Fatalf("second Stop returned error: %v", err)
	}
}
