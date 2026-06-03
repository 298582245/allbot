package router

import (
	"testing"

	"github.com/allbot/allbot/core/adapter"
	plugincore "github.com/allbot/allbot/core/plugin"
	"github.com/allbot/allbot/core/types"
)

func TestRouterQQOfficeReplyTarget(t *testing.T) {
	r := NewRouter(nil)
	withReplyTarget := &types.Message{Platform: "qq_office", UserID: "user123", Metadata: map[string]string{"reply_target": "dms_guild123|msg_msg456", "qq_office_guild_id": "guild999"}}
	if got := r.replyTarget(withReplyTarget); got != "dms_guild123|msg_msg456" {
		t.Fatalf("replyTarget with reply_target = %q", got)
	}
	withGuild := &types.Message{Platform: "qq_office", UserID: "user123", Metadata: map[string]string{"qq_office_guild_id": "guild123"}}
	if got := r.replyTarget(withGuild); got != "dms_guild123" {
		t.Fatalf("replyTarget with guild = %q", got)
	}
	withGroupOpenID := &types.Message{Platform: "qq_office", UserID: "member123", Metadata: map[string]string{"qq_office_group_openid": "group123"}}
	if got := r.replyTarget(withGroupOpenID); got != "group_group123" {
		t.Fatalf("replyTarget with group_openid = %q", got)
	}
	withUserOpenID := &types.Message{Platform: "qq_office", UserID: "user123", Metadata: map[string]string{"qq_office_user_openid": "user-openid"}}
	if got := r.replyTarget(withUserOpenID); got != "user_user-openid" {
		t.Fatalf("replyTarget with user_openid = %q", got)
	}
}

func TestRouterSendPluginMessageQQOfficeTarget(t *testing.T) {
	fake := &keywordReplyFakeAdapter{}
	r := NewRouter(nil)
	r.SetAdapters(map[string]adapter.Adapter{"qq_office": fake})

	if err := r.sendPluginMessage("plugin", plugincore.SendMessageAction{Platform: "qq_office", UserID: "user123", GroupID: "guild123", Text: "你好"}); err != nil {
		t.Fatalf("sendPluginMessage returned error: %v", err)
	}
	if err := r.sendPluginMessage("plugin", plugincore.SendMessageAction{Platform: "qq_office", UserID: "user123", GroupID: "dms_guild456", Text: "你好"}); err != nil {
		t.Fatalf("sendPluginMessage returned error: %v", err)
	}
	if err := r.sendPluginMessage("plugin", plugincore.SendMessageAction{Platform: "qq_office", UserID: "user_user-openid", Text: "你好"}); err != nil {
		t.Fatalf("sendPluginMessage returned error: %v", err)
	}
	if err := r.sendPluginMessage("plugin", plugincore.SendMessageAction{Platform: "qq_office", UserID: "user123", GroupID: "group_group-openid", Text: "你好"}); err != nil {
		t.Fatalf("sendPluginMessage returned error: %v", err)
	}
	messages := fake.sentMessages()
	if len(messages) != 4 {
		t.Fatalf("messages len = %d, expected 4", len(messages))
	}
	if messages[0].target != "dms_guild123" {
		t.Fatalf("first target = %q", messages[0].target)
	}
	if messages[1].target != "dms_guild456" {
		t.Fatalf("second target = %q", messages[1].target)
	}
	if messages[2].target != "user_user-openid" {
		t.Fatalf("third target = %q", messages[2].target)
	}
	if messages[3].target != "group_group-openid" {
		t.Fatalf("fourth target = %q", messages[3].target)
	}
}
