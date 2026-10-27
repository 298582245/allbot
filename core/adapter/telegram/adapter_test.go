package telegram

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/allbot/allbot/core/adapter/_contract"
	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/types"
)

func telegramTestDatabase(t *testing.T) *config.Database {
	t.Helper()
	db, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	settings := config.DefaultImageHostSettings()
	settings.StorageDir = t.TempDir()
	settings.PublicBaseURL = "https://bot.example"
	if err := db.SaveImageHostSettings(settings); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestTelegramHandleMediaBuildsUnifiedMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/botTOKEN/getFile" {
			var body map[string]interface{}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["file_id"] == "" {
				t.Fatalf("getFile body = %#v", body)
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"ok":true,"result":{"file_path":"photos/photo.jpg"}}`))
			return
		}
		_, _ = writer.Write([]byte("downloaded media"))
	}))
	defer server.Close()
	adapter := NewTelegramAdapter("TOKEN", "")
	adapter.apiURL = server.URL + "/botTOKEN"
	adapter.SetDatabase(telegramTestDatabase(t))
	messages := make(chan *types.Message, 1)
	adapter.SetMessageHandler(func(message *types.Message) { messages <- message })
	adapter.handleUpdate(map[string]interface{}{
		"message": map[string]interface{}{
			"message_id": float64(8),
			"text":       "说明文字",
			"from":       map[string]interface{}{"id": float64(9), "username": "tester"},
			"chat":       map[string]interface{}{"id": float64(10), "type": "private"},
			"photo":      []interface{}{map[string]interface{}{"file_id": "photo-id", "file_size": float64(100), "file_unique_id": "photo-unique"}},
			"audio":      map[string]interface{}{"file_id": "audio-id", "file_name": "voice.mp3"},
		},
	})

	message := <-messages
	if message.MediaOnly || len(message.Medias) != 2 || message.Medias[0].Type != types.MessageMediaTypeImage || message.Medias[1].Type != types.MessageMediaTypeAudio {
		t.Fatalf("message = %#v", message)
	}
	if !strings.HasPrefix(message.Medias[0].URL, "https://bot.example/api/open/temp-media/") || !strings.HasPrefix(message.Medias[1].URL, "https://bot.example/api/open/temp-media/") {
		t.Fatalf("medias = %#v", message.Medias)
	}
	if strings.Contains(message.Content, "TOKEN") || !strings.Contains(message.Content, "说明文字 [图片]("+message.Medias[0].URL+") [voice.mp3]("+message.Medias[1].URL+")") {
		t.Fatalf("content = %q", message.Content)
	}
}

func TestTelegramHandleSingleVideoSetsMediaOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/botTOKEN/getFile" {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"ok":true,"result":{"file_path":"videos/clip.mp4"}}`))
			return
		}
		_, _ = writer.Write([]byte("video bytes"))
	}))
	defer server.Close()
	adapter := NewTelegramAdapter("TOKEN", "")
	adapter.apiURL = server.URL + "/botTOKEN"
	adapter.SetDatabase(telegramTestDatabase(t))
	messages := make(chan *types.Message, 1)
	adapter.SetMessageHandler(func(message *types.Message) { messages <- message })
	adapter.handleUpdate(map[string]interface{}{"message": map[string]interface{}{
		"message_id": float64(18),
		"from":       map[string]interface{}{"id": float64(19)},
		"chat":       map[string]interface{}{"id": float64(20), "type": "group"},
		"video":      map[string]interface{}{"file_id": "video-id", "file_name": "clip.mp4"},
	}})
	message := <-messages
	if !message.MediaOnly || message.GroupID != "20" || len(message.Medias) != 1 || message.Medias[0].Type != types.MessageMediaTypeVideo || !strings.HasPrefix(message.Medias[0].URL, "https://bot.example/api/open/temp-media/") {
		t.Fatalf("message = %#v", message)
	}
}

func TestTelegramSendAudioAndVideo(t *testing.T) {
	requests := make(chan map[string]interface{}, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		requests <- map[string]interface{}{"path": request.URL.Path, "body": body}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	adapter := NewTelegramAdapter("TOKEN", "")
	adapter.apiURL = server.URL + "/botTOKEN"
	if err := adapter.SendAudio("123", "https://media.example/audio.mp3"); err != nil {
		t.Fatal(err)
	}
	if err := adapter.SendVideo("456", "https://media.example/video.mp4"); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []struct{ path, field, url string }{
		{"/botTOKEN/sendAudio", "audio", "https://media.example/audio.mp3"},
		{"/botTOKEN/sendVideo", "video", "https://media.example/video.mp4"},
	} {
		request := <-requests
		body := request["body"].(map[string]interface{})
		if request["path"] != expected.path || body[expected.field] != expected.url {
			t.Fatalf("request = %#v", request)
		}
	}
}

func TestTelegramSendFileUploadsLocalFileAsMultipart(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(filePath, []byte("file contents"), 0600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/botTOKEN/sendDocument" || !strings.HasPrefix(request.Header.Get("Content-Type"), "multipart/form-data;") {
			t.Fatalf("request path/content-type = %q / %q", request.URL.Path, request.Header.Get("Content-Type"))
		}
		if err := request.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		if request.FormValue("chat_id") != "123" {
			t.Fatalf("chat_id = %q", request.FormValue("chat_id"))
		}
		file, header, err := request.FormFile("document")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		content, err := io.ReadAll(file)
		if err != nil {
			t.Fatal(err)
		}
		if header.Filename != "notes.txt" || string(content) != "file contents" {
			t.Fatalf("uploaded file = %q, %q", header.Filename, content)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	adapter := NewTelegramAdapter("TOKEN", "")
	adapter.apiURL = server.URL + "/botTOKEN"
	if err := adapter.SendFile("123", filePath); err != nil {
		t.Fatal(err)
	}
}

func TestTelegramVerifyTokenCachesBotIdentity(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"ok":true,"result":{"id":123456789}}`))
	}))
	defer server.Close()

	adapter := NewTelegramAdapter("token", "")
	var _ contract.BotIdentityProvider = adapter
	adapter.apiURL = server.URL
	adapter.httpClient = server.Client()
	if err := adapter.verifyToken(); err != nil {
		t.Fatalf("verifyToken returned error: %v", err)
	}
	identity := adapter.GetBotIdentity(nil)
	if identity.Label != "Telegram Bot ID" || identity.Value != "123456789" {
		t.Fatalf("identity = %#v", identity)
	}
	if calls.Load() != 1 {
		t.Fatalf("calls after identity read = %d", calls.Load())
	}
}

func TestTelegramVerifyTokenRejectsInvalidResponses(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{name: "http error", statusCode: http.StatusUnauthorized, body: `{}`},
		{name: "ok false", statusCode: http.StatusOK, body: `{"ok":false,"description":"invalid"}`},
		{name: "invalid json", statusCode: http.StatusOK, body: `{`},
		{name: "missing id", statusCode: http.StatusOK, body: `{"ok":true,"result":{}}`},
		{name: "decimal id", statusCode: http.StatusOK, body: `{"ok":true,"result":{"id":1.5}}`},
		{name: "zero id", statusCode: http.StatusOK, body: `{"ok":true,"result":{"id":0}}`},
		{name: "negative id", statusCode: http.StatusOK, body: `{"ok":true,"result":{"id":-1}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			adapter := NewTelegramAdapter("token", "")
			adapter.apiURL = server.URL
			adapter.httpClient = server.Client()
			if err := adapter.verifyToken(); err == nil {
				t.Fatal("expected verifyToken error")
			}
			if identity := adapter.GetBotIdentity(nil); identity.Value != "" {
				t.Fatalf("identity = %#v", identity)
			}
		})
	}
}

func TestTelegramVerifyTokenClearsCachedIdentityOnFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	adapter := NewTelegramAdapter("token", "")
	adapter.apiURL = server.URL
	adapter.httpClient = server.Client()
	adapter.botID = "123456789"

	if err := adapter.verifyToken(); err == nil {
		t.Fatal("expected verifyToken error")
	}
	if identity := adapter.GetBotIdentity(nil); identity.Value != "" {
		t.Fatalf("identity = %#v", identity)
	}
}

func TestTelegramDeleteAndMuteUseBotAPI(t *testing.T) {
	var deleteBody map[string]interface{}
	var muteBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body failed: %v", err)
		}
		switch r.URL.Path {
		case "/deleteMessage":
			deleteBody = body
		case "/restrictChatMember":
			muteBody = body
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	adapter := NewTelegramAdapter("token", "")
	adapter.apiURL = server.URL
	adapter.httpClient = server.Client()
	message := &types.Message{
		ID:      "456",
		GroupID: "-100123",
		Metadata: map[string]string{
			"chat_id": "-100123",
		},
	}
	if err := adapter.DeleteMessage(message); err != nil {
		t.Fatalf("DeleteMessage returned error: %v", err)
	}
	if err := adapter.Mute("-100123", "<@789>", 60); err != nil {
		t.Fatalf("Mute returned error: %v", err)
	}
	if deleteBody["chat_id"] != float64(-100123) || deleteBody["message_id"] != float64(456) {
		t.Fatalf("delete body = %#v", deleteBody)
	}
	if muteBody["chat_id"] != float64(-100123) || muteBody["user_id"] != float64(789) || muteBody["until_date"] == nil {
		t.Fatalf("mute body = %#v", muteBody)
	}
	permissions := muteBody["permissions"].(map[string]interface{})
	if permissions["can_send_messages"] != false {
		t.Fatalf("mute permissions = %#v", permissions)
	}
}

func TestTelegramMuteRejectsWholeGroupAndNonNumericUserID(t *testing.T) {
	adapter := NewTelegramAdapter("token", "")
	if err := adapter.Mute("-100123", "", 60); err == nil {
		t.Fatal("expected whole-group mute to be unsupported")
	}
	if err := adapter.Mute("-100123", "@username", 60); err == nil {
		t.Fatal("expected non-numeric user ID to be rejected")
	}
}

func TestTelegramSendMarkdownUsesMarkdownParseMode(t *testing.T) {
	var body map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bot-token/sendMessage" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body failed: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	adapter := NewTelegramAdapter("token", "")
	adapter.apiURL = server.URL + "/bot-token"
	adapter.httpClient = server.Client()
	if err := adapter.SendMarkdown("123", "**hi**"); err != nil {
		t.Fatalf("SendMarkdown returned error: %v", err)
	}
	if body["chat_id"] != float64(123) || body["text"] != "**hi**" || body["parse_mode"] != "Markdown" {
		t.Fatalf("body = %#v", body)
	}
}

func TestNormalizeTelegramCommandTextDropsSlashWithoutEntities(t *testing.T) {
	if got := normalizeTelegramCommandText("/md示例", map[string]interface{}{}); got != "md示例" {
		t.Fatalf("got %q", got)
	}
	if got := normalizeTelegramCommandText("/重启@allbot hello", map[string]interface{}{}); got != "重启 hello" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeTelegramCommandTextDropsSlashWithEntity(t *testing.T) {
	message := map[string]interface{}{"entities": []interface{}{map[string]interface{}{"type": "bot_command", "offset": float64(0), "length": float64(len("/md示例@allbot"))}}}
	if got := normalizeTelegramCommandText("/md示例@allbot 参数", message); got != "md示例 参数" {
		t.Fatalf("got %q", got)
	}
}

func TestTelegramSendButtonsEncodesInlineKeyboard(t *testing.T) {
	var body map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sendMessage" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body failed: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	adapter := NewTelegramAdapter("token", "")
	adapter.apiURL = server.URL
	adapter.httpClient = server.Client()
	if err := adapter.SendButtons("123", "请选择", [][]types.ButtonOption{{{Text: "A", Value: "1"}, {Text: "", Value: "x"}}, {{Text: "B", Value: "2"}}}); err != nil {
		t.Fatalf("SendButtons returned error: %v", err)
	}
	markup, ok := body["reply_markup"].(map[string]interface{})
	if !ok {
		t.Fatalf("reply_markup missing: %#v", body)
	}
	rows, ok := markup["inline_keyboard"].([]interface{})
	if !ok || len(rows) != 2 {
		t.Fatalf("inline_keyboard = %#v", markup)
	}
	firstRow := rows[0].([]interface{})
	firstButton := firstRow[0].(map[string]interface{})
	if firstButton["callback_data"] != "absel:1" {
		t.Fatalf("firstButton = %#v", firstButton)
	}
}

func TestTelegramSendButtonsEncodesRestrictedInlineKeyboard(t *testing.T) {
	var body map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body failed: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	adapter := NewTelegramAdapter("token", "")
	adapter.apiURL = server.URL
	adapter.httpClient = server.Client()
	if err := adapter.SendButtons("123", "请选择", [][]types.ButtonOption{{{Text: "A", Value: "1", UserID: "456"}}}); err != nil {
		t.Fatalf("SendButtons returned error: %v", err)
	}
	markup := body["reply_markup"].(map[string]interface{})
	rows := markup["inline_keyboard"].([]interface{})
	firstRow := rows[0].([]interface{})
	firstButton := firstRow[0].(map[string]interface{})
	if firstButton["callback_data"] != "abselu:456:1" {
		t.Fatalf("firstButton = %#v", firstButton)
	}
}

func TestTelegramHandleCallbackQueryConvertsButtonDataToMessage(t *testing.T) {
	var callbackBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/answerCallbackQuery" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&callbackBody); err != nil {
			t.Fatalf("decode body failed: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	adapter := NewTelegramAdapter("token", "")
	adapter.apiURL = server.URL
	adapter.httpClient = server.Client()
	var received *types.Message
	adapter.messageHandler = func(msg *types.Message) {
		received = msg
	}
	adapter.handleCallbackQuery(map[string]interface{}{
		"id":   "cb-1",
		"data": "absel:0",
		"from": map[string]interface{}{"id": float64(123), "first_name": "张", "last_name": "三"},
		"message": map[string]interface{}{
			"message_id": float64(456),
			"chat":       map[string]interface{}{"id": float64(789), "type": "group"},
		},
	})
	if callbackBody["callback_query_id"] != "cb-1" {
		t.Fatalf("callbackBody = %#v", callbackBody)
	}
	if received == nil || received.Content != "0" || received.GroupID != "789" || received.Metadata["telegram_input_source"] != "inline_keyboard" || received.Metadata["telegram_callback_data"] != "absel:0" {
		t.Fatalf("received = %#v", received)
	}
}

func TestTelegramHandleCallbackQueryRejectsRestrictedButtonFromOtherUser(t *testing.T) {
	var callbackBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/answerCallbackQuery" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&callbackBody); err != nil {
			t.Fatalf("decode body failed: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	adapter := NewTelegramAdapter("token", "")
	adapter.apiURL = server.URL
	adapter.httpClient = server.Client()
	adapter.messageHandler = func(msg *types.Message) {
		t.Fatalf("unexpected message: %#v", msg)
	}
	adapter.handleCallbackQuery(map[string]interface{}{
		"id":   "cb-2",
		"data": "abselu:456:0",
		"from": map[string]interface{}{"id": float64(123)},
		"message": map[string]interface{}{
			"message_id": float64(456),
			"chat":       map[string]interface{}{"id": float64(789), "type": "group"},
		},
	})
	if callbackBody["callback_query_id"] != "cb-2" || callbackBody["text"] != "此按钮只能由发起用户使用" {
		t.Fatalf("callbackBody = %#v", callbackBody)
	}
}

func TestTelegramHandleCallbackQueryAcceptsRestrictedButtonFromAllowedUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	adapter := NewTelegramAdapter("token", "")
	adapter.apiURL = server.URL
	adapter.httpClient = server.Client()
	var received *types.Message
	adapter.messageHandler = func(msg *types.Message) {
		received = msg
	}
	adapter.handleCallbackQuery(map[string]interface{}{
		"id":   "cb-3",
		"data": "abselu:123:0",
		"from": map[string]interface{}{"id": float64(123)},
		"message": map[string]interface{}{
			"message_id": float64(456),
			"chat":       map[string]interface{}{"id": float64(789), "type": "group"},
		},
	})
	if received == nil || received.Content != "0" || received.UserID != "123" {
		t.Fatalf("received = %#v", received)
	}
}

func TestTelegramSendMarkdownFallsBackOnEntityParseError(t *testing.T) {
	requests := make([]map[string]interface{}, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body failed: %v", err)
		}
		requests = append(requests, body)
		if len(requests) == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"ok":false,"description":"Bad Request: can't parse entities"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	adapter := NewTelegramAdapter("token", "")
	adapter.apiURL = server.URL
	adapter.httpClient = server.Client()
	if err := adapter.SendMarkdown("chat", "**hi**"); err != nil {
		t.Fatalf("SendMarkdown returned error: %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("requests len = %d", len(requests))
	}
	if _, ok := requests[1]["parse_mode"]; ok || strings.Contains(requests[1]["text"].(string), "**") {
		t.Fatalf("fallback request = %#v", requests[1])
	}
}
