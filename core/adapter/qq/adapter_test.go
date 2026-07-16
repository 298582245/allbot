package qq

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/allbot/allbot/core/types"
	"github.com/gorilla/websocket"
)

var testUpgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

type oneBotRequest struct {
	Action string                 `json:"action"`
	Params map[string]interface{} `json:"params"`
	Echo   string                 `json:"echo"`
}

func websocketURL(server *httptest.Server) string {
	return "ws" + strings.TrimPrefix(server.URL, "http")
}

func TestQQAdapterWebSocketActionsEventsAndPing(t *testing.T) {
	authorizations := make(chan string, 1)
	actions := make(chan oneBotRequest, 4)
	pongReceived := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		authorizations <- request.Header.Get("Authorization")
		conn, err := testUpgrader.Upgrade(writer, request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetPongHandler(func(string) error {
			select {
			case pongReceived <- struct{}{}:
			default:
			}
			return nil
		})
		for {
			var action oneBotRequest
			if err := conn.ReadJSON(&action); err != nil {
				return
			}
			actions <- action
			switch action.Action {
			case "get_login_info":
				_ = conn.WriteJSON(map[string]interface{}{"status": "ok", "retcode": 0, "data": map[string]interface{}{"user_id": 10001}, "echo": action.Echo})
				_ = conn.WriteControl(websocket.PingMessage, []byte("health"), time.Now().Add(time.Second))
				_ = conn.WriteJSON(map[string]interface{}{
					"post_type": "message", "message_type": "group", "message_id": 9,
					"self_id": 10001, "user_id": 20002, "group_id": 30003,
					"message": []interface{}{map[string]interface{}{"type": "text", "data": map[string]interface{}{"text": "你好"}}},
				})
			case "send_msg":
				_ = conn.WriteJSON(map[string]interface{}{"status": "ok", "retcode": 0, "data": map[string]interface{}{"message_id": 10}, "echo": action.Echo})
			}
		}
	}))
	defer server.Close()

	adapter := NewQQAdapter(QQAdapterConfig{Framework: "napcat", ServerURL: websocketURL(server), AccessToken: "secret"})
	defer adapter.Stop()
	messages := make(chan *types.Message, 1)
	adapter.SetMessageHandler(func(message *types.Message) { messages <- message })
	if err := adapter.Start(); err != nil {
		t.Fatal(err)
	}
	if authorization := <-authorizations; authorization != "Bearer secret" {
		t.Fatalf("Authorization = %q", authorization)
	}
	if err := adapter.SendMessage("group_30003", "回复"); err != nil {
		t.Fatal(err)
	}

	first := <-actions
	second := <-actions
	if first.Action != "get_login_info" || second.Action != "send_msg" {
		t.Fatalf("actions = %q, %q", first.Action, second.Action)
	}
	if second.Params["message_type"] != "group" || second.Params["message"] != "回复" {
		t.Fatalf("send params = %#v", second.Params)
	}
	select {
	case message := <-messages:
		if message.UserID != "20002" || message.GroupID != "30003" || message.Content != "你好" {
			t.Fatalf("message = %+v", message)
		}
	case <-time.After(time.Second):
		t.Fatal("message event not received")
	}
	select {
	case <-pongReceived:
	case <-time.After(time.Second):
		t.Fatal("pong not received")
	}
}

func TestQQAdapterExplicitHTTPActionChannel(t *testing.T) {
	var wsActionCount atomic.Int32
	wsAuthorizations := make(chan string, 1)
	wsServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		wsAuthorizations <- request.Header.Get("Authorization")
		conn, err := testUpgrader.Upgrade(writer, request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
			wsActionCount.Add(1)
		}
	}))
	defer wsServer.Close()

	var mu sync.Mutex
	httpActions := make([]string, 0, 2)
	httpAuthorizations := make(chan string, 2)
	httpServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		httpAuthorizations <- request.Header.Get("Authorization")
		mu.Lock()
		httpActions = append(httpActions, strings.TrimPrefix(request.URL.Path, "/"))
		mu.Unlock()
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/get_login_info" {
			_, _ = writer.Write([]byte(`{"status":"ok","retcode":0,"data":{"user_id":10001}}`))
			return
		}
		_, _ = writer.Write([]byte(`{"status":"ok","retcode":0,"data":{"message_id":1}}`))
	}))
	defer httpServer.Close()

	adapter := NewQQAdapter(QQAdapterConfig{
		Framework:          "napcat",
		ServerURL:          websocketURL(wsServer),
		HTTPAPIURL:         httpServer.URL,
		AccessToken:        "ws-secret",
		HTTPAPIAccessToken: "http-secret",
	})
	defer adapter.Stop()
	if err := adapter.Start(); err != nil {
		t.Fatal(err)
	}
	if err := adapter.SendMessage("20002", "hello"); err != nil {
		t.Fatal(err)
	}
	wsAuthorization := <-wsAuthorizations
	firstHTTPAuthorization := <-httpAuthorizations
	secondHTTPAuthorization := <-httpAuthorizations
	if wsAuthorization != "Bearer ws-secret" || firstHTTPAuthorization != "Bearer http-secret" || secondHTTPAuthorization != "Bearer http-secret" {
		t.Fatalf("authorization ws=%q http=%q,%q", wsAuthorization, firstHTTPAuthorization, secondHTTPAuthorization)
	}
	if wsActionCount.Load() != 0 {
		t.Fatalf("WebSocket action count = %d", wsActionCount.Load())
	}
	mu.Lock()
	defer mu.Unlock()
	if len(httpActions) != 2 || httpActions[0] != "get_login_info" || httpActions[1] != "send_msg" {
		t.Fatalf("HTTP actions = %#v", httpActions)
	}
}

func TestQQAdapterHTTPDoesNotReuseWebSocketToken(t *testing.T) {
	authorization := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		authorization <- request.Header.Get("Authorization")
		_, _ = writer.Write([]byte(`{"status":"ok","retcode":0,"data":{"user_id":10001}}`))
	}))
	defer server.Close()

	adapter := NewQQAdapter(QQAdapterConfig{HTTPAPIURL: server.URL, AccessToken: "ws-secret"})
	defer adapter.Stop()
	var result map[string]interface{}
	if err := adapter.callAPIWithResult("get_login_info", map[string]interface{}{}, &result); err != nil {
		t.Fatal(err)
	}
	if header := <-authorization; header != "" {
		t.Fatalf("HTTP Authorization = %q", header)
	}
}

func TestQQAdapterDoesNotInferHTTPFallback(t *testing.T) {
	var ordinaryHTTPCount atomic.Int32
	connectionClosed := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !strings.EqualFold(request.Header.Get("Upgrade"), "websocket") {
			ordinaryHTTPCount.Add(1)
			writer.WriteHeader(http.StatusUpgradeRequired)
			return
		}
		conn, err := testUpgrader.Upgrade(writer, request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		var action oneBotRequest
		if err := conn.ReadJSON(&action); err != nil {
			return
		}
		if _, _, err := conn.ReadMessage(); err != nil {
			connectionClosed <- struct{}{}
		}
	}))
	defer server.Close()

	adapter := NewQQAdapter(QQAdapterConfig{Framework: "napcat", ServerURL: websocketURL(server)})
	adapter.actionTimeout = 50 * time.Millisecond
	err := adapter.Start()
	if err == nil || !strings.Contains(err.Error(), "响应超时") {
		t.Fatalf("Start error = %v", err)
	}
	if ordinaryHTTPCount.Load() != 0 {
		t.Fatalf("ordinary HTTP requests = %d", ordinaryHTTPCount.Load())
	}
	select {
	case <-connectionClosed:
	case <-time.After(time.Second):
		t.Fatal("connection was not closed after startup failure")
	}
}

func TestQQAdapterRejectsOneBotErrorWhenEitherStatusOrRetcodeFails(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		retcode  int
		message  string
		wording  string
		contains string
	}{
		{name: "failed status", status: "failed", retcode: 0, message: "denied", wording: "bad token", contains: "denied: bad token"},
		{name: "nonzero retcode", status: "ok", retcode: 100, wording: "unsupported action", contains: "unsupported action"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				conn, err := testUpgrader.Upgrade(writer, request, nil)
				if err != nil {
					return
				}
				defer conn.Close()
				var action oneBotRequest
				if err := conn.ReadJSON(&action); err != nil {
					return
				}
				_ = conn.WriteJSON(map[string]interface{}{"status": tt.status, "retcode": tt.retcode, "message": tt.message, "wording": tt.wording, "echo": action.Echo})
			}))
			defer server.Close()

			adapter := NewQQAdapter(QQAdapterConfig{Framework: "napcat", ServerURL: websocketURL(server)})
			err := adapter.Start()
			if err == nil || !strings.Contains(err.Error(), tt.contains) {
				t.Fatalf("Start error = %v", err)
			}
		})
	}
}

func TestQQAdapterHTTPTimeoutDoesNotReplayOnWebSocket(t *testing.T) {
	var wsActionCount atomic.Int32
	wsServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		conn, err := testUpgrader.Upgrade(writer, request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
			wsActionCount.Add(1)
		}
	}))
	defer wsServer.Close()

	var sendCount atomic.Int32
	httpServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/get_login_info" {
			_, _ = writer.Write([]byte(`{"status":"ok","retcode":0,"data":{"user_id":10001}}`))
			return
		}
		sendCount.Add(1)
		time.Sleep(150 * time.Millisecond)
		_, _ = writer.Write([]byte(`{"status":"ok","retcode":0}`))
	}))
	defer httpServer.Close()

	adapter := NewQQAdapter(QQAdapterConfig{Framework: "napcat", ServerURL: websocketURL(wsServer), HTTPAPIURL: httpServer.URL})
	adapter.httpClient = &http.Client{Timeout: 30 * time.Millisecond}
	defer adapter.Stop()
	if err := adapter.Start(); err != nil {
		t.Fatal(err)
	}
	if err := adapter.SendMessage("20002", "timeout"); err == nil {
		t.Fatal("expected timeout error")
	}
	if sendCount.Load() != 1 || wsActionCount.Load() != 0 {
		t.Fatalf("send count=%d ws action count=%d", sendCount.Load(), wsActionCount.Load())
	}
}

func TestQQAdapterMessageHandlerCanReplyOverWebSocket(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		conn, err := testUpgrader.Upgrade(writer, request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			var action oneBotRequest
			if err := conn.ReadJSON(&action); err != nil {
				return
			}
			if action.Action == "get_login_info" {
				_ = conn.WriteJSON(map[string]interface{}{"status": "ok", "retcode": 0, "data": map[string]interface{}{"user_id": 10001}, "echo": action.Echo})
				_ = conn.WriteJSON(map[string]interface{}{"post_type": "message", "message_type": "private", "message_id": 9, "self_id": 10001, "user_id": 20002, "message": "ping"})
				continue
			}
			_ = conn.WriteJSON(map[string]interface{}{"status": "ok", "retcode": 0, "echo": action.Echo})
		}
	}))
	defer server.Close()

	adapter := NewQQAdapter(QQAdapterConfig{Framework: "napcat", ServerURL: websocketURL(server)})
	defer adapter.Stop()
	replied := make(chan error, 1)
	adapter.SetMessageHandler(func(message *types.Message) {
		replied <- adapter.SendMessage(message.UserID, "pong")
	})
	if err := adapter.Start(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-replied:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("message handler reply timed out")
	}
}

func TestQQAdapterHTTPProbeFailsWhenWebSocketCloses(t *testing.T) {
	probeStarted := make(chan struct{})
	allowProbeResponse := make(chan struct{})
	httpServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		close(probeStarted)
		<-allowProbeResponse
		_, _ = writer.Write([]byte(`{"status":"ok","retcode":0,"data":{"user_id":10001}}`))
	}))
	defer httpServer.Close()
	wsServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		conn, err := testUpgrader.Upgrade(writer, request, nil)
		if err != nil {
			return
		}
		<-probeStarted
		_ = conn.Close()
	}))
	defer wsServer.Close()

	adapter := NewQQAdapter(QQAdapterConfig{Framework: "napcat", ServerURL: websocketURL(wsServer), HTTPAPIURL: httpServer.URL})
	startResult := make(chan error, 1)
	go func() {
		startResult <- adapter.Start()
	}()
	<-probeStarted
	<-adapter.closed
	close(allowProbeResponse)
	err := <-startResult
	if err == nil || (!strings.Contains(err.Error(), "WebSocket") && !strings.Contains(err.Error(), "context canceled")) {
		t.Fatalf("Start error = %v", err)
	}
}

func TestQQAdapterStartupReturnsDisconnectError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		conn, err := testUpgrader.Upgrade(writer, request, nil)
		if err != nil {
			return
		}
		var action json.RawMessage
		_ = conn.ReadJSON(&action)
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "action disabled"), time.Now().Add(time.Second))
		_ = conn.Close()
	}))
	defer server.Close()

	adapter := NewQQAdapter(QQAdapterConfig{Framework: "napcat", ServerURL: websocketURL(server)})
	err := adapter.Start()
	if err == nil || (!strings.Contains(err.Error(), "WebSocket 已关闭") && !strings.Contains(err.Error(), "context canceled")) {
		t.Fatalf("Start error = %v", err)
	}
}
