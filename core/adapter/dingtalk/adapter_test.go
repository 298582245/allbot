package dingtalk

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/allbot/allbot/core/adapter/_contract"
	"github.com/allbot/allbot/core/types"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
)

func TestDingTalkAdapterImplementsContracts(t *testing.T) {
	adapter := NewDingTalkAdapter("client", "secret", "robot", "", "")
	var _ contract.Adapter = adapter
	var _ contract.MarkdownSender = adapter
	var _ contract.ReplyTargetResolver = adapter
	var _ contract.ReplyTextFormatter = adapter
	var _ contract.SendTargetResolver = adapter
}

func TestParseConfigForRegistry(t *testing.T) {
	parsed, err := parseConfigForRegistry(`{"client_id":" client ","client_secret":" secret ","robot_code":" robot ","open_api_host":" host ","proxy_url":" proxy "}`)
	if err != nil {
		t.Fatalf("parseConfigForRegistry returned error: %v", err)
	}
	config := parsed.(*Config)
	if config.ClientID != "client" || config.ClientSecret != "secret" || config.RobotCode != "robot" || config.OpenAPIHost != "host" || config.ProxyURL != "proxy" {
		t.Fatalf("config trim failed: %#v", config)
	}
	for _, raw := range []string{
		`{"client_secret":"secret"}`,
		`{"client_id":"client"}`,
		`{"client_id":"","client_secret":"secret"}`,
	} {
		if _, err := parseConfigForRegistry(raw); err == nil {
			t.Fatalf("expected error for %s", raw)
		}
	}
	if _, err := parseConfigForRegistry(`{bad json}`); err == nil {
		t.Fatal("expected json error")
	}
}

func TestNewAdapterFromRegistryRejectsWrongConfig(t *testing.T) {
	if _, err := newAdapterFromRegistry(Config{}); err == nil {
		t.Fatal("expected config type error")
	}
	adapter, err := newAdapterFromRegistry(&Config{ClientID: "client", ClientSecret: "secret"})
	if err != nil {
		t.Fatalf("newAdapterFromRegistry returned error: %v", err)
	}
	if adapter.GetPlatform() != platformName {
		t.Fatalf("platform = %q", adapter.GetPlatform())
	}
}

func TestReplyTarget(t *testing.T) {
	adapter := NewDingTalkAdapter("client", "secret", "", "", "")
	if got := adapter.ReplyTarget(nil); got != "" {
		t.Fatalf("ReplyTarget(nil) = %q", got)
	}
	msg := &types.Message{UserID: "user", GroupID: "group", Metadata: map[string]string{"reply_target": "target"}}
	if got := adapter.ReplyTarget(msg); got != "target" {
		t.Fatalf("ReplyTarget reply_target = %q", got)
	}
	msg.Metadata = map[string]string{"dingtalk_session_webhook": "https://example.com/hook"}
	if got := adapter.ReplyTarget(msg); got != dingTalkWebhookTarget("https://example.com/hook") {
		t.Fatalf("ReplyTarget webhook = %q", got)
	}
	msg.Metadata = map[string]string{"dingtalk_conversation_id": "cid"}
	if got := adapter.ReplyTarget(msg); got != "conversation_cid" {
		t.Fatalf("ReplyTarget conversation metadata = %q", got)
	}
	msg.Metadata = nil
	if got := adapter.ReplyTarget(msg); got != "conversation_group" {
		t.Fatalf("ReplyTarget group = %q", got)
	}
	msg.GroupID = ""
	if got := adapter.ReplyTarget(msg); got != "user_user" {
		t.Fatalf("ReplyTarget user = %q", got)
	}
	msg.UserID = "conversation_existing"
	if got := adapter.ReplyTarget(msg); got != "conversation_existing" {
		t.Fatalf("ReplyTarget existing prefix = %q", got)
	}
}

func TestDingTalkFormatReplyTextMentionsGroupSender(t *testing.T) {
	adapter := NewDingTalkAdapter("client", "secret", "", "", "")
	group := &types.Message{UserID: "staff-1", GroupID: "cid"}
	if got := adapter.FormatReplyText(group, "你好"); got != "[CQ:at,qq=staff-1] 你好" {
		t.Fatalf("group reply text = %q", got)
	}
	private := &types.Message{UserID: "staff-1"}
	if got := adapter.FormatReplyText(private, "你好"); got != "你好" {
		t.Fatalf("private reply text = %q", got)
	}
	withoutUser := &types.Message{GroupID: "cid"}
	if got := adapter.FormatReplyText(withoutUser, "你好"); got != "你好" {
		t.Fatalf("empty user reply text = %q", got)
	}
}

func TestSendTarget(t *testing.T) {
	adapter := NewDingTalkAdapter("client", "secret", "", "", "")
	tests := []struct {
		userID  string
		groupID string
		want    string
	}{
		{userID: "user", groupID: "group", want: "conversation_group"},
		{userID: "user", groupID: "conversation_group", want: "conversation_group"},
		{userID: "user", want: "user_user"},
		{userID: "user_existing", want: "user_existing"},
		{userID: dingTalkWebhookTarget("https://example.com/hook"), want: dingTalkWebhookTarget("https://example.com/hook")},
		{want: ""},
	}
	for _, tt := range tests {
		if got := adapter.SendTarget(tt.userID, tt.groupID); got != tt.want {
			t.Fatalf("SendTarget(%q, %q) = %q, want %q", tt.userID, tt.groupID, got, tt.want)
		}
	}
}

func TestParseDingTalkMessageTarget(t *testing.T) {
	webhook := "https://example.com/hook"
	tests := []struct {
		target string
		kind   string
		id     string
	}{
		{target: dingTalkWebhookTarget(webhook), kind: dingTalkTargetWebhook, id: webhook},
		{target: "conversation_cid", kind: dingTalkTargetConversation, id: "cid"},
		{target: "user_uid", kind: dingTalkTargetUser, id: "uid"},
	}
	for _, tt := range tests {
		got, err := parseDingTalkMessageTarget(tt.target)
		if err != nil {
			t.Fatalf("parseDingTalkMessageTarget(%q) returned error: %v", tt.target, err)
		}
		if got.kind != tt.kind || got.id != tt.id {
			t.Fatalf("target = %#v, want kind=%s id=%s", got, tt.kind, tt.id)
		}
	}
	for _, target := range []string{"", "webhook_b64_", "webhook_b64_bad@@", "conversation_", "user_", "plain"} {
		if _, err := parseDingTalkMessageTarget(target); err == nil {
			t.Fatalf("expected error for %q", target)
		}
	}
}

func TestBuildMessagePrivateAndGroup(t *testing.T) {
	adapter := NewDingTalkAdapter("client", "secret", "robot", "", "")
	private := adapter.buildMessage(&chatbot.BotCallbackDataModel{
		Msgtype:                   "text",
		Text:                      chatbot.BotCallbackDataTextModel{Content: "  你好  "},
		MsgId:                     "msg1",
		ConversationId:            "cid1",
		ConversationType:          "1",
		SenderId:                  "sender",
		SenderStaffId:             "staff",
		SenderNick:                "nick",
		ChatbotUserId:             "robot-user",
		ChatbotCorpId:             "robot-corp",
		SenderCorpId:              "sender-corp",
		ConversationTitle:         "title",
		SessionWebhook:            "https://example.com/hook",
		SessionWebhookExpiredTime: 123,
		CreateAt:                  456,
		IsAdmin:                   true,
		IsInAtList:                true,
	})
	if private == nil {
		t.Fatal("expected private message")
	}
	if private.ID != "msg1" || private.Platform != platformName || private.UserID != "staff" || private.GroupID != "" || private.Content != "你好" {
		t.Fatalf("private message = %#v", private)
	}
	if private.Metadata["message_type"] != "private" || private.Metadata["dingtalk_sender_id"] != "sender" || private.Metadata["dingtalk_robot_code"] != "robot" {
		t.Fatalf("private metadata = %#v", private.Metadata)
	}
	if private.Metadata["reply_target"] != dingTalkWebhookTarget("https://example.com/hook") {
		t.Fatalf("reply_target = %q", private.Metadata["reply_target"])
	}
	group := adapter.buildMessage(&chatbot.BotCallbackDataModel{Msgtype: "text", Text: chatbot.BotCallbackDataTextModel{Content: "群消息"}, MsgId: "msg2", ConversationId: "cid2", ConversationType: "2", SenderId: "sender", SessionWebhook: "https://example.com/group-hook"})
	if group == nil || group.UserID != "sender" || group.GroupID != "cid2" || group.Metadata["message_type"] != "group" || group.Metadata["reply_target"] != dingTalkWebhookTarget("https://example.com/group-hook") {
		t.Fatalf("group message = %#v", group)
	}

	groupWithoutWebhook := adapter.buildMessage(&chatbot.BotCallbackDataModel{Msgtype: "text", Text: chatbot.BotCallbackDataTextModel{Content: "群消息"}, MsgId: "msg3", ConversationId: "cid3", ConversationType: "2", SenderId: "sender"})
	if groupWithoutWebhook == nil || groupWithoutWebhook.GroupID != "cid3" || groupWithoutWebhook.Metadata["reply_target"] != "conversation_cid3" {
		t.Fatalf("groupWithoutWebhook message = %#v", groupWithoutWebhook)
	}
}

func TestBuildMessageStoresAtUsersMetadata(t *testing.T) {
	adapter := NewDingTalkAdapter("client", "secret", "robot", "", "")
	msg := adapter.buildMessage(&chatbot.BotCallbackDataModel{
		Msgtype: "text",
		Text:    chatbot.BotCallbackDataTextModel{Content: "你好"},
		AtUsers: []chatbot.BotCallbackDataAtUserModel{
			{DingtalkId: " user-1 ", StaffId: " staff-1 "},
			{DingtalkId: "user-2"},
			{StaffId: "staff-2"},
		},
	})
	if msg == nil {
		t.Fatal("expected message")
	}
	if msg.Metadata["dingtalk_at_user_ids"] != "user-1,user-2" {
		t.Fatalf("dingtalk_at_user_ids = %q", msg.Metadata["dingtalk_at_user_ids"])
	}
	if msg.Metadata["dingtalk_at_staff_ids"] != "staff-1,staff-2" {
		t.Fatalf("dingtalk_at_staff_ids = %q", msg.Metadata["dingtalk_at_staff_ids"])
	}
}

func TestBuildMessageCompatibleGroupTypes(t *testing.T) {
	adapter := NewDingTalkAdapter("client", "secret", "robot", "", "")
	for _, conversationType := range []string{"group", "groupChat", "group_chat", "chat"} {
		msg := adapter.buildMessage(&chatbot.BotCallbackDataModel{Msgtype: "text", Text: chatbot.BotCallbackDataTextModel{Content: "群消息"}, MsgId: "msg-" + conversationType, ConversationId: "cid-" + conversationType, ConversationType: conversationType, SenderId: "sender"})
		if msg == nil {
			t.Fatalf("expected message for conversationType=%s", conversationType)
		}
		if msg.GroupID != "cid-"+conversationType || msg.Metadata["message_type"] != "group" || msg.Metadata["dingtalk_conversation_type"] != conversationType {
			t.Fatalf("message for conversationType=%s = %#v", conversationType, msg)
		}
	}
}

func TestBuildMessageIgnoresUnsupportedMessage(t *testing.T) {
	adapter := NewDingTalkAdapter("client", "secret", "", "", "")
	if msg := adapter.buildMessage(nil); msg != nil {
		t.Fatalf("nil data message = %#v", msg)
	}
	if msg := adapter.buildMessage(&chatbot.BotCallbackDataModel{Msgtype: "image"}); msg != nil {
		t.Fatalf("non text message = %#v", msg)
	}
	if msg := adapter.buildMessage(&chatbot.BotCallbackDataModel{Msgtype: "text", Text: chatbot.BotCallbackDataTextModel{Content: "   "}}); msg != nil {
		t.Fatalf("empty text message = %#v", msg)
	}
}

func TestHandleChatBotMessageDispatchesAsync(t *testing.T) {
	adapter := NewDingTalkAdapter("client", "secret", "", "", "")
	messages := make(chan *types.Message, 1)
	adapter.SetMessageHandler(func(msg *types.Message) { messages <- msg })
	if data, err := adapter.handleChatBotMessage(context.Background(), &chatbot.BotCallbackDataModel{Msgtype: "text", Text: chatbot.BotCallbackDataTextModel{Content: "ping"}, MsgId: "1", SenderId: "sender"}); err != nil || data != nil {
		t.Fatalf("handleChatBotMessage data=%v err=%v", data, err)
	}
	select {
	case msg := <-messages:
		if msg.Content != "ping" || msg.UserID != "sender" {
			t.Fatalf("message = %#v", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("message was not dispatched")
	}
}

func TestSendMessageUsesWebhookReplier(t *testing.T) {
	adapter := NewDingTalkAdapter("client", "secret", "", "", "")
	replier := &recordingDingTalkReplier{}
	adapter.replier = replier
	if err := adapter.SendMessage(dingTalkWebhookTarget("https://example.com/hook"), "你好"); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}
	if replier.webhook != "https://example.com/hook" || replier.content != "你好" || atomic.LoadInt32(&replier.calls) != 1 || atomic.LoadInt32(&replier.replyMessageCalls) != 0 {
		t.Fatalf("replier = %#v", replier)
	}
}

func TestSendMessageConvertsCQAtToDingTalkAtPayload(t *testing.T) {
	adapter := NewDingTalkAdapter("client", "secret", "", "", "")
	replier := &recordingDingTalkReplier{}
	adapter.replier = replier
	if err := adapter.SendMessage(dingTalkWebhookTarget("https://example.com/hook"), "[CQ:at,qq=staff-1] 你好"); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}
	if atomic.LoadInt32(&replier.calls) != 0 || atomic.LoadInt32(&replier.replyMessageCalls) != 1 {
		t.Fatalf("replier calls = text:%d reply:%d", replier.calls, replier.replyMessageCalls)
	}
	assertDingTalkAtPayload(t, replier.payload, "你好", []string{"staff-1"}, false)
}

func TestSendMessageConvertsCQAtAll(t *testing.T) {
	adapter := NewDingTalkAdapter("client", "secret", "", "", "")
	replier := &recordingDingTalkReplier{}
	adapter.replier = replier
	if err := adapter.SendMessage(dingTalkWebhookTarget("https://example.com/hook"), "[CQ:at,qq=all] 全体注意"); err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}
	assertDingTalkAtPayload(t, replier.payload, "全体注意", nil, true)
}

func TestParseDingTalkCQAtTextDeduplicatesUsers(t *testing.T) {
	parsed := parseDingTalkCQAtText("[CQ:at,qq=u1] [CQ:at,qq=u2] [CQ:at,qq=u1] 你好")
	if !parsed.hasAt || parsed.isAtAll || parsed.content != "你好" {
		t.Fatalf("parsed = %#v", parsed)
	}
	if len(parsed.atUserIDs) != 2 || parsed.atUserIDs[0] != "u1" || parsed.atUserIDs[1] != "u2" {
		t.Fatalf("atUserIDs = %#v", parsed.atUserIDs)
	}
}

func TestParseDingTalkCQAtTextKeepsMalformedCQ(t *testing.T) {
	text := "[CQ:at] [CQ:image,file=a.png] 你好"
	parsed := parseDingTalkCQAtText(text)
	if parsed.hasAt || parsed.content != text || len(parsed.atUserIDs) != 0 || parsed.isAtAll {
		t.Fatalf("parsed = %#v", parsed)
	}
}

func TestSendMessageUnsupportedTargets(t *testing.T) {
	adapter := NewDingTalkAdapter("client", "secret", "", "", "")
	for _, target := range []string{"conversation_cid", "user_uid"} {
		err := adapter.SendMessage(target, "你好")
		if err == nil || !strings.Contains(err.Error(), "暂未实现") || !strings.Contains(err.Error(), "session webhook") {
			t.Fatalf("SendMessage(%q) error = %v", target, err)
		}
	}
	if err := adapter.SendMessage("bad", "你好"); err == nil || !strings.Contains(err.Error(), "格式无效") {
		t.Fatalf("invalid target error = %v", err)
	}
}

func TestSendMarkdownUsesWebhookMarkdown(t *testing.T) {
	adapter := NewDingTalkAdapter("client", "secret", "", "", "")
	replier := &recordingDingTalkReplier{}
	adapter.replier = replier
	if err := adapter.SendMarkdown(dingTalkWebhookTarget("https://example.com/hook"), "## 标题\n内容"); err != nil {
		t.Fatalf("SendMarkdown returned error: %v", err)
	}
	if replier.webhook != "https://example.com/hook" || replier.markdownTitle != "标题" || replier.markdownContent != "## 标题\n内容" || atomic.LoadInt32(&replier.markdownCalls) != 1 {
		t.Fatalf("replier = %#v", replier)
	}
}

func TestSendMarkdownUnsupportedTargets(t *testing.T) {
	adapter := NewDingTalkAdapter("client", "secret", "", "", "")
	if err := adapter.SendMarkdown(dingTalkWebhookTarget("https://example.com/hook"), " "); err == nil || !strings.Contains(err.Error(), "Markdown 内容不能为空") {
		t.Fatalf("empty markdown error = %v", err)
	}
	for _, target := range []string{"conversation_cid", "user_uid"} {
		err := adapter.SendMarkdown(target, "## 标题")
		if err == nil || !strings.Contains(err.Error(), "暂未实现") || !strings.Contains(err.Error(), "session webhook") {
			t.Fatalf("SendMarkdown(%q) error = %v", target, err)
		}
	}
}

func TestSendImageUsesWebhookMarkdown(t *testing.T) {
	adapter := NewDingTalkAdapter("client", "secret", "", "", "")
	replier := &recordingDingTalkReplier{}
	adapter.replier = replier
	if err := adapter.SendImage(dingTalkWebhookTarget("https://example.com/hook"), " https://example.com/image.png "); err != nil {
		t.Fatalf("SendImage returned error: %v", err)
	}
	if replier.webhook != "https://example.com/hook" || replier.markdownTitle != "图片" || replier.markdownContent != "![图片](https://example.com/image.png)" || atomic.LoadInt32(&replier.markdownCalls) != 1 {
		t.Fatalf("replier = %#v", replier)
	}
}

func TestSendImageUnsupportedTargets(t *testing.T) {
	adapter := NewDingTalkAdapter("client", "secret", "", "", "")
	if err := adapter.SendImage(dingTalkWebhookTarget("https://example.com/hook"), " "); err == nil || !strings.Contains(err.Error(), "图片地址不能为空") {
		t.Fatalf("empty image error = %v", err)
	}
	for _, target := range []string{"conversation_cid", "user_uid"} {
		err := adapter.SendImage(target, "https://example.com/image.png")
		if err == nil || !strings.Contains(err.Error(), "暂未实现") || !strings.Contains(err.Error(), "session webhook") {
			t.Fatalf("SendImage(%q) error = %v", target, err)
		}
	}
	if err := adapter.SendImage("bad", "https://example.com/image.png"); err == nil || !strings.Contains(err.Error(), "格式无效") {
		t.Fatalf("invalid target error = %v", err)
	}
}

func TestStopIsIdempotent(t *testing.T) {
	stream := &recordingDingTalkStreamClient{}
	adapter := NewDingTalkAdapter("client", "secret", "", "", "")
	adapter.streamClient = stream
	adapter.cancel = func() {}
	if err := adapter.Stop(); err != nil {
		t.Fatalf("first Stop returned error: %v", err)
	}
	if err := adapter.Stop(); err != nil {
		t.Fatalf("second Stop returned error: %v", err)
	}
	if atomic.LoadInt32(&stream.closeCalls) != 1 {
		t.Fatalf("closeCalls = %d, expected 1", stream.closeCalls)
	}
}

func TestStartUsesInjectedStreamClient(t *testing.T) {
	stream := &recordingDingTalkStreamClient{}
	adapter := NewDingTalkAdapter("client", "secret", "", "", "")
	adapter.streamClient = stream
	if err := adapter.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if atomic.LoadInt32(&stream.registerCalls) != 1 || atomic.LoadInt32(&stream.startCalls) != 1 {
		t.Fatalf("stream register=%d start=%d", stream.registerCalls, stream.startCalls)
	}
	_ = adapter.Stop()
}

type recordingDingTalkReplier struct {
	calls             int32
	markdownCalls     int32
	replyMessageCalls int32
	webhook           string
	content           string
	markdownTitle     string
	markdownContent   string
	payload           map[string]interface{}
}

func (r *recordingDingTalkReplier) SimpleReplyText(ctx context.Context, sessionWebhook string, content []byte) error {
	atomic.AddInt32(&r.calls, 1)
	r.webhook = sessionWebhook
	r.content = string(content)
	return nil
}

func (r *recordingDingTalkReplier) SimpleReplyMarkdown(ctx context.Context, sessionWebhook string, title []byte, content []byte) error {
	atomic.AddInt32(&r.markdownCalls, 1)
	r.webhook = sessionWebhook
	r.markdownTitle = string(title)
	r.markdownContent = string(content)
	return nil
}

func (r *recordingDingTalkReplier) ReplyMessage(ctx context.Context, sessionWebhook string, requestBody map[string]interface{}) error {
	atomic.AddInt32(&r.replyMessageCalls, 1)
	r.webhook = sessionWebhook
	r.payload = requestBody
	return nil
}

func assertDingTalkAtPayload(t *testing.T, payload map[string]interface{}, content string, atUserIDs []string, isAtAll bool) {
	t.Helper()
	if payload["msgtype"] != "text" {
		t.Fatalf("msgtype = %#v", payload["msgtype"])
	}
	textPayload, ok := payload["text"].(map[string]interface{})
	if !ok || textPayload["content"] != content {
		t.Fatalf("text payload = %#v", payload["text"])
	}
	atPayload, ok := payload["at"].(map[string]interface{})
	if !ok {
		t.Fatalf("at payload = %#v", payload["at"])
	}
	gotUserIDs, ok := atPayload["atUserIds"].([]string)
	if !ok {
		t.Fatalf("atUserIds = %#v", atPayload["atUserIds"])
	}
	if len(gotUserIDs) != len(atUserIDs) {
		t.Fatalf("atUserIds = %#v, want %#v", gotUserIDs, atUserIDs)
	}
	for i := range gotUserIDs {
		if gotUserIDs[i] != atUserIDs[i] {
			t.Fatalf("atUserIds = %#v, want %#v", gotUserIDs, atUserIDs)
		}
	}
	if atPayload["isAtAll"] != isAtAll {
		t.Fatalf("isAtAll = %#v", atPayload["isAtAll"])
	}
}

type recordingDingTalkStreamClient struct {
	registerCalls int32
	startCalls    int32
	closeCalls    int32
}

func (c *recordingDingTalkStreamClient) RegisterChatBotCallbackRouter(chatbot.IChatBotMessageHandler) {
	atomic.AddInt32(&c.registerCalls, 1)
}

func (c *recordingDingTalkStreamClient) Start(context.Context) error {
	atomic.AddInt32(&c.startCalls, 1)
	return nil
}

func (c *recordingDingTalkStreamClient) Close() {
	atomic.AddInt32(&c.closeCalls, 1)
}
