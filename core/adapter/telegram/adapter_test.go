package telegram

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/allbot/allbot/core/types"
)

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
