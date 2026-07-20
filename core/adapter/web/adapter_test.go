package web

import (
	"testing"
	"time"

	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/types"
)

func TestAdapterSendMessageSavesAndPushes(t *testing.T) {
	db, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()
	if err := db.CreateWebChatEmailCode("u@example.com", "123456", config.WebChatEmailPurposeRegister, "127.0.0.1"); err != nil {
		t.Fatalf("CreateWebChatEmailCode returned error: %v", err)
	}
	user, err := db.RegisterWebChatUser(config.WebChatRegisterInput{Email: "u@example.com", Code: "123456", Username: "user_1", Password: "password123"})
	if err != nil {
		t.Fatalf("RegisterWebChatUser returned error: %v", err)
	}
	adp := NewAdapter()
	adp.SetDatabase(db)
	ch, cancel := adp.Subscribe(user.UserID, 1)
	defer cancel()
	if err := adp.SendMessage(adp.SendTarget(user.UserID, ""), "hello"); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}
	select {
	case msg := <-ch:
		if msg.Content != "hello" || msg.Direction != "out" {
			t.Fatalf("unexpected pushed message: %#v", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("expected pushed message")
	}
	items, err := db.ListWebChatMessages(user.UserID, 0, 10)
	if err != nil {
		t.Fatalf("ListWebChatMessages returned error: %v", err)
	}
	if len(items) != 1 || items[0].Content != "hello" || items[0].PluginID != "" {
		t.Fatalf("unexpected history: %#v", items)
	}
}

func TestAdapterReplyTargetKeepsPrivateWithoutPluginMetadata(t *testing.T) {
	adp := NewAdapter()
	if got := adp.ReplyTarget(&types.Message{UserID: "web_u"}); got != "user_web_u" {
		t.Fatalf("unexpected private reply target: %s", got)
	}
	if got := adp.ReplyTarget(&types.Message{UserID: "web_u", Metadata: map[string]string{"web_chat_plugin_id": "p1"}}); got != "user_web_u#plugin_p1" {
		t.Fatalf("unexpected plugin reply target: %s", got)
	}
}

func TestParseConfigForRegistryTrimsAndBuildsAdapter(t *testing.T) {
	parsed, err := parseConfigForRegistry(`{"smtp_host":" smtp.example.com ","smtp_port":" 587 ","smtp_username":" user ","smtp_password":" pass ","smtp_from":" bot@example.com ","smtp_subject":" 自定义标题 "}`)
	if err != nil {
		t.Fatalf("parseConfigForRegistry returned error: %v", err)
	}
	adp, err := newAdapterFromRegistry(parsed)
	if err != nil {
		t.Fatalf("newAdapterFromRegistry returned error: %v", err)
	}
	cfg := adp.(*Adapter).SMTPConfig()
	if cfg.SMTPHost != "smtp.example.com" || cfg.SMTPPort != "587" || cfg.SMTPUsername != "user" || cfg.SMTPPassword != "pass" || cfg.SMTPFrom != "bot@example.com" || cfg.SMTPSubject != "自定义标题" {
		t.Fatalf("unexpected SMTP config: %#v", cfg)
	}
}

func TestParseConfigForRegistryUsesMessageLimit(t *testing.T) {
	parsed, err := parseConfigForRegistry(`{"smtp_host":"smtp.example.com","smtp_port":"587","smtp_username":"user","smtp_password":"pass","smtp_from":"bot@example.com","message_limit_per_minute":15}`)
	if err != nil {
		t.Fatalf("parseConfigForRegistry returned error: %v", err)
	}
	cfg := parsed.(*Config)
	if cfg.MessageLimitPerMinute != 15 {
		t.Fatalf("MessageLimitPerMinute = %d, want 15", cfg.MessageLimitPerMinute)
	}
	adp, err := newAdapterFromRegistry(parsed)
	if err != nil {
		t.Fatalf("newAdapterFromRegistry returned error: %v", err)
	}
	if got := adp.(*Adapter).MessageLimitPerMinute(); got != 15 {
		t.Fatalf("MessageLimitPerMinute() = %d, want 15", got)
	}
}

func TestParseConfigForRegistryUsesDefaultMessageLimit(t *testing.T) {
	cases := map[string]string{
		"missing":  `{"smtp_host":"smtp.example.com","smtp_port":"587","smtp_username":"user","smtp_password":"pass","smtp_from":"bot@example.com"}`,
		"zero":     `{"smtp_host":"smtp.example.com","smtp_port":"587","smtp_username":"user","smtp_password":"pass","smtp_from":"bot@example.com","message_limit_per_minute":0}`,
		"negative": `{"smtp_host":"smtp.example.com","smtp_port":"587","smtp_username":"user","smtp_password":"pass","smtp_from":"bot@example.com","message_limit_per_minute":-1}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			parsed, err := parseConfigForRegistry(raw)
			if err != nil {
				t.Fatalf("parseConfigForRegistry returned error: %v", err)
			}
			cfg := parsed.(*Config)
			if cfg.MessageLimitPerMinute != DefaultMessageLimitPerMinute {
				t.Fatalf("MessageLimitPerMinute = %d, want %d", cfg.MessageLimitPerMinute, DefaultMessageLimitPerMinute)
			}
		})
	}
}

func TestParseConfigForRegistryUsesDefaultSMTPSubjectWhenMissing(t *testing.T) {
	parsed, err := parseConfigForRegistry(`{"smtp_host":"smtp.example.com","smtp_port":"587","smtp_username":"user","smtp_password":"pass","smtp_from":"bot@example.com"}`)
	if err != nil {
		t.Fatalf("parseConfigForRegistry returned error: %v", err)
	}
	cfg := parsed.(*Config)
	if cfg.SMTPSubject != DefaultSMTPSubject {
		t.Fatalf("SMTPSubject = %q, want %q", cfg.SMTPSubject, DefaultSMTPSubject)
	}
}

func TestParseConfigForRegistryUsesDefaultSMTPSubjectWhenBlank(t *testing.T) {
	parsed, err := parseConfigForRegistry(`{"smtp_host":"smtp.example.com","smtp_port":"587","smtp_username":"user","smtp_password":"pass","smtp_from":"bot@example.com","smtp_subject":"   "}`)
	if err != nil {
		t.Fatalf("parseConfigForRegistry returned error: %v", err)
	}
	cfg := parsed.(*Config)
	if cfg.SMTPSubject != DefaultSMTPSubject {
		t.Fatalf("SMTPSubject = %q, want %q", cfg.SMTPSubject, DefaultSMTPSubject)
	}
}

func TestParseConfigForRegistryRequiresSMTPFields(t *testing.T) {
	cases := map[string]string{
		"smtp_host":     `{"smtp_port":"587","smtp_username":"user","smtp_password":"pass","smtp_from":"bot@example.com"}`,
		"smtp_port":     `{"smtp_host":"smtp.example.com","smtp_username":"user","smtp_password":"pass","smtp_from":"bot@example.com"}`,
		"smtp_username": `{"smtp_host":"smtp.example.com","smtp_port":"587","smtp_password":"pass","smtp_from":"bot@example.com"}`,
		"smtp_password": `{"smtp_host":"smtp.example.com","smtp_port":"587","smtp_username":"user","smtp_from":"bot@example.com"}`,
		"smtp_from":     `{"smtp_host":"smtp.example.com","smtp_port":"587","smtp_username":"user","smtp_password":"pass"}`,
	}
	for field, raw := range cases {
		t.Run(field, func(t *testing.T) {
			_, err := parseConfigForRegistry(raw)
			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestAdapterReceiveMessageUsesRouterHandler(t *testing.T) {
	adp := NewAdapter()
	got := make(chan *types.Message, 1)
	adp.SetMessageHandler(func(msg *types.Message) { got <- msg })
	adp.ReceiveMessage("web_u", "ping", "text", "", "")
	select {
	case msg := <-got:
		if msg.Platform != platformName || msg.UserID != "web_u" || msg.Content != "ping" {
			t.Fatalf("unexpected message: %#v", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("expected message")
	}
}
