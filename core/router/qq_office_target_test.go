package router

import (
	"testing"

	"github.com/allbot/allbot/core/adapter"
	qqadapter "github.com/allbot/allbot/core/adapter/qq"
	qqofficeadapter "github.com/allbot/allbot/core/adapter/qq_office"
	telegramadapter "github.com/allbot/allbot/core/adapter/telegram"
	plugincore "github.com/allbot/allbot/core/plugin"
	"github.com/allbot/allbot/core/types"
)

func TestRouterQQOfficeReplyTarget(t *testing.T) {
	adp := qqofficeadapter.NewQQOfficeAdapter("app123", "secret456", "", "")
	withReplyTarget := &types.Message{Platform: "qq_office", UserID: "user123", Metadata: map[string]string{"reply_target": "dms_guild123|msg_msg456", "qq_office_guild_id": "guild999"}}
	if got := resolveReplyTarget(adp, withReplyTarget); got != "dms_guild123|msg_msg456" {
		t.Fatalf("replyTarget with reply_target = %q", got)
	}
	withGuild := &types.Message{Platform: "qq_office", UserID: "user123", Metadata: map[string]string{"qq_office_guild_id": "guild123"}}
	if got := resolveReplyTarget(adp, withGuild); got != "dms_guild123" {
		t.Fatalf("replyTarget with guild = %q", got)
	}
	withGroupOpenID := &types.Message{Platform: "qq_office", UserID: "member123", Metadata: map[string]string{"qq_office_group_openid": "group123"}}
	if got := resolveReplyTarget(adp, withGroupOpenID); got != "group_group123" {
		t.Fatalf("replyTarget with group_openid = %q", got)
	}
	withUserOpenID := &types.Message{Platform: "qq_office", UserID: "user123", Metadata: map[string]string{"qq_office_user_openid": "user-openid"}}
	if got := resolveReplyTarget(adp, withUserOpenID); got != "user_user-openid" {
		t.Fatalf("replyTarget with user_openid = %q", got)
	}
}

func TestAdapterReplyFormatters(t *testing.T) {
	qq := qqadapter.NewQQAdapter("ws://127.0.0.1:3001", "")
	qqMsg := &types.Message{Platform: "qq", UserID: "1001", GroupID: "2001"}
	if got := resolveReplyTarget(qq, qqMsg); got != "group_2001" {
		t.Fatalf("QQ target = %q", got)
	}
	if got := formatReplyText(qq, qqMsg, "你好"); got != "[CQ:at,qq=1001] 你好" {
		t.Fatalf("QQ reply text = %q", got)
	}

	telegram := telegramadapter.NewTelegramAdapter("token", "")
	tgMsg := &types.Message{Platform: "telegram", UserID: "7089240306", GroupID: "-1001", Metadata: map[string]string{"chat_id": "-1001", "from_name": "A&B"}}
	if got := resolveReplyTarget(telegram, tgMsg); got != "-1001" {
		t.Fatalf("Telegram target = %q", got)
	}
	wantTelegram := `<a href="tg://user?id=7089240306">A&amp;B</a> 你好&lt;test&gt;`
	if got := formatReplyText(telegram, tgMsg, "你好<test>"); got != wantTelegram {
		t.Fatalf("Telegram reply text = %q", got)
	}

	qqOffice := qqofficeadapter.NewQQOfficeAdapter("app123", "secret456", "", "")
	qqOfficeMsg := &types.Message{Platform: "qq_office", UserID: "member-openid", GroupID: "group-openid"}
	if got := resolveReplyTarget(qqOffice, qqOfficeMsg); got != "group_group-openid" {
		t.Fatalf("QQ 官方 target = %q", got)
	}
	if got := formatReplyText(qqOffice, qqOfficeMsg, "你好"); got != "你好" {
		t.Fatalf("QQ 官方 reply text = %q", got)
	}
}

func TestPluginResponseLogMessageID(t *testing.T) {
	if got := pluginResponseLogMessageID(&types.Message{ID: "msg123"}); got != "msg123" {
		t.Fatalf("message id = %q", got)
	}
	if got := pluginResponseLogMessageID(&types.Message{Metadata: map[string]string{"qq_office_msg_id": "qq-msg"}}); got != "qq-msg" {
		t.Fatalf("metadata message id = %q", got)
	}
	if got := pluginResponseLogMessageID(&types.Message{}); got != "-" {
		t.Fatalf("empty message id = %q", got)
	}
}

func TestRouterSendPluginMessageQQOfficeTarget(t *testing.T) {
	qqOffice := qqofficeadapter.NewQQOfficeAdapter("app123", "secret456", "", "")
	fake := newReplyCapableKeywordReplyFakeAdapter(qqOffice, qqOffice)
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
