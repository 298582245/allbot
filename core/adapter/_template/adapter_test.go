package template

import (
	"strings"
	"testing"

	"github.com/allbot/allbot/core/adapter/_contract"
	"github.com/allbot/allbot/core/types"
)

func TestExampleAdapterImplementsContracts(t *testing.T) {
	adapter := NewExampleAdapter("token")
	var _ contract.Adapter = adapter
	var _ contract.ReplyTargetResolver = adapter
	var _ contract.ReplyTextFormatter = adapter
	var _ contract.SendTargetResolver = adapter
}

func TestParseConfigForRegistry(t *testing.T) {
	parsed, err := parseConfigForRegistry(`{"token":" abc ","base_url":" https://api.example.com "}`)
	if err != nil {
		t.Fatalf("parseConfigForRegistry returned error: %v", err)
	}
	config := parsed.(*Config)
	if config.Token != "abc" || config.BaseURL != "https://api.example.com" {
		t.Fatalf("config = %+v", config)
	}
	if _, err := parseConfigForRegistry(`{"token":""}`); err == nil || !strings.Contains(err.Error(), "token 不能为空") {
		t.Fatalf("expected empty token error, got %v", err)
	}
}

func TestExampleAdapterDispatchesMessage(t *testing.T) {
	adapter := NewExampleAdapter("token")
	received := make(chan *types.Message, 1)
	adapter.SetMessageHandler(func(msg *types.Message) {
		received <- msg
	})
	adapter.handleIncomingMessage("msg1", "user1", "group1", " 你好 ")
	msg := <-received
	if msg.Platform != platformName || msg.UserID != "user1" || msg.GroupID != "group1" || msg.Content != "你好" {
		t.Fatalf("message = %+v", msg)
	}
	if msg.Metadata["message_type"] != "group" {
		t.Fatalf("metadata = %+v", msg.Metadata)
	}
}

func TestExampleAdapterTargetsAndFormatting(t *testing.T) {
	adapter := NewExampleAdapter("token")
	msg := &types.Message{UserID: "user1", GroupID: "group1"}
	if target := adapter.ReplyTarget(msg); target != "group_group1" {
		t.Fatalf("ReplyTarget = %q", target)
	}
	if target := adapter.SendTarget("user1", ""); target != "user_user1" {
		t.Fatalf("SendTarget private = %q", target)
	}
	if text := adapter.FormatReplyText(msg, "你好"); text != "@user1 你好" {
		t.Fatalf("FormatReplyText = %q", text)
	}
}
