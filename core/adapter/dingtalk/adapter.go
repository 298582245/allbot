package dingtalk

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/allbot/allbot/core/adapter/_contract"
	"github.com/allbot/allbot/core/types"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
)

type UserInfo = contract.UserInfo
type GroupInfo = contract.GroupInfo

const (
	platformName = "dingtalk"

	dingTalkTargetConversation = "conversation"
	dingTalkTargetUser         = "user"
	dingTalkTargetWebhook      = "webhook"
)

type dingTalkStreamClient interface {
	RegisterChatBotCallbackRouter(chatbot.IChatBotMessageHandler)
	Start(context.Context) error
	Close()
}

type dingTalkReplier interface {
	SimpleReplyText(context.Context, string, []byte) error
	SimpleReplyMarkdown(context.Context, string, []byte, []byte) error
	ReplyMessage(context.Context, string, map[string]interface{}) error
}

type DingTalkAdapter struct {
	clientID     string
	clientSecret string
	robotCode    string
	openAPIHost  string
	proxyURL     string

	messageHandler func(*types.Message)

	streamClient dingTalkStreamClient
	replier      dingTalkReplier
	cancel       context.CancelFunc
	stopOnce     sync.Once
	mu           sync.Mutex
}

type dingTalkMessageTarget struct {
	kind string
	id   string
}

// NewDingTalkAdapter 创建钉钉 Stream 机器人适配器。
func NewDingTalkAdapter(clientID, clientSecret, robotCode, openAPIHost, proxyURL string) *DingTalkAdapter {
	return &DingTalkAdapter{
		clientID:     strings.TrimSpace(clientID),
		clientSecret: strings.TrimSpace(clientSecret),
		robotCode:    strings.TrimSpace(robotCode),
		openAPIHost:  strings.TrimSpace(openAPIHost),
		proxyURL:     strings.TrimSpace(proxyURL),
		replier:      chatbot.NewChatbotReplier(),
	}
}

func (a *DingTalkAdapter) GetPlatform() string {
	return platformName
}

func (a *DingTalkAdapter) SetMessageHandler(handler func(*types.Message)) {
	a.messageHandler = handler
}

// ReplyTarget 按钉钉目标格式解析被动回复目标。
func (a *DingTalkAdapter) ReplyTarget(msg *types.Message) string {
	if msg == nil {
		return ""
	}
	if msg.Metadata != nil {
		if replyTarget := strings.TrimSpace(msg.Metadata["reply_target"]); replyTarget != "" {
			return replyTarget
		}
		if sessionWebhook := strings.TrimSpace(msg.Metadata["dingtalk_session_webhook"]); sessionWebhook != "" {
			return dingTalkWebhookTarget(sessionWebhook)
		}
		if conversationID := strings.TrimSpace(msg.Metadata["dingtalk_conversation_id"]); conversationID != "" {
			return "conversation_" + conversationID
		}
	}
	if msg.GroupID != "" {
		if hasDingTalkTargetPrefix(msg.GroupID) {
			return msg.GroupID
		}
		return "conversation_" + msg.GroupID
	}
	if hasDingTalkTargetPrefix(msg.UserID) {
		return msg.UserID
	}
	if msg.UserID == "" {
		return ""
	}
	return "user_" + msg.UserID
}

// FormatReplyText 在钉钉群聊回复前拼接统一 CQ at 码。
func (a *DingTalkAdapter) FormatReplyText(msg *types.Message, text string) string {
	if msg == nil || msg.GroupID == "" || strings.TrimSpace(msg.UserID) == "" {
		return text
	}
	return fmt.Sprintf("[CQ:at,qq=%s] %s", strings.TrimSpace(msg.UserID), text)
}

// SendTarget 按钉钉目标格式解析插件主动发送目标。
func (a *DingTalkAdapter) SendTarget(userID string, groupID string) string {
	groupID = strings.TrimSpace(groupID)
	userID = strings.TrimSpace(userID)
	if groupID != "" {
		if hasDingTalkTargetPrefix(groupID) {
			return groupID
		}
		return "conversation_" + groupID
	}
	if hasDingTalkTargetPrefix(userID) {
		return userID
	}
	if userID == "" {
		return ""
	}
	return "user_" + userID
}

func hasDingTalkTargetPrefix(value string) bool {
	return strings.HasPrefix(value, "conversation_") || strings.HasPrefix(value, "user_") || strings.HasPrefix(value, "webhook_b64_")
}

func (a *DingTalkAdapter) Start() error {
	if a.clientID == "" {
		return fmt.Errorf("client_id 不能为空")
	}
	if a.clientSecret == "" {
		return fmt.Errorf("client_secret 不能为空")
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.streamClient == nil {
		a.streamClient = a.newStreamClient()
	}
	if a.replier == nil {
		a.replier = chatbot.NewChatbotReplier()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.stopOnce = sync.Once{}
	a.streamClient.RegisterChatBotCallbackRouter(a.handleChatBotMessage)
	if err := a.streamClient.Start(ctx); err != nil {
		cancel()
		a.cancel = nil
		return fmt.Errorf("启动钉钉 Stream 连接失败: %w", err)
	}
	log.Printf("钉钉机器人 Stream Adapter 已启动")
	return nil
}

func (a *DingTalkAdapter) Stop() error {
	a.stopOnce.Do(func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.cancel != nil {
			a.cancel()
			a.cancel = nil
		}
		if client, ok := a.streamClient.(*client.StreamClient); ok {
			client.AutoReconnect = false
		}
		if a.streamClient != nil {
			a.streamClient.Close()
			a.streamClient = nil
		}
		log.Printf("钉钉机器人 Stream Adapter 已停止")
	})
	return nil
}

func (a *DingTalkAdapter) SendMessage(target string, text string) error {
	targetInfo, err := parseDingTalkMessageTarget(target)
	if err != nil {
		return err
	}
	switch targetInfo.kind {
	case dingTalkTargetWebhook:
		if a.replier == nil {
			a.replier = chatbot.NewChatbotReplier()
		}
		log.Printf("[发送][钉钉][%s]：%s", target, text)
		parsed := parseDingTalkCQAtText(text)
		if !parsed.hasAt {
			return a.replier.SimpleReplyText(context.Background(), targetInfo.id, []byte(text))
		}
		return a.replier.ReplyMessage(context.Background(), targetInfo.id, map[string]interface{}{
			"msgtype": "text",
			"text": map[string]interface{}{
				"content": parsed.content,
			},
			"at": map[string]interface{}{
				"atUserIds": parsed.atUserIDs,
				"isAtAll":   parsed.isAtAll,
			},
		})
	case dingTalkTargetConversation:
		return fmt.Errorf("钉钉 Stream 适配器暂未实现会话主动发送，请使用最近消息的 session webhook 回复目标")
	case dingTalkTargetUser:
		return fmt.Errorf("钉钉 Stream 适配器暂未实现用户主动发送，请使用最近消息的 session webhook 回复目标")
	default:
		return fmt.Errorf("钉钉消息目标类型无效: %s", targetInfo.kind)
	}
}

func (a *DingTalkAdapter) SendMarkdown(target string, markdown string) error {
	markdown = strings.TrimSpace(markdown)
	if markdown == "" {
		return fmt.Errorf("钉钉 Markdown 内容不能为空")
	}
	targetInfo, err := parseDingTalkMessageTarget(target)
	if err != nil {
		return err
	}
	switch targetInfo.kind {
	case dingTalkTargetWebhook:
		if a.replier == nil {
			a.replier = chatbot.NewChatbotReplier()
		}
		log.Printf("[发送][钉钉][%s]：[Markdown] %s", target, markdown)
		return a.replier.SimpleReplyMarkdown(context.Background(), targetInfo.id, []byte(dingTalkMarkdownTitle(markdown)), []byte(markdown))
	case dingTalkTargetConversation:
		return fmt.Errorf("钉钉 Stream 适配器暂未实现会话主动发送 Markdown，请使用最近消息的 session webhook 回复目标")
	case dingTalkTargetUser:
		return fmt.Errorf("钉钉 Stream 适配器暂未实现用户主动发送 Markdown，请使用最近消息的 session webhook 回复目标")
	default:
		return fmt.Errorf("钉钉消息目标类型无效: %s", targetInfo.kind)
	}
}

func (a *DingTalkAdapter) SendImage(target string, imageURL string) error {
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return fmt.Errorf("钉钉图片地址不能为空")
	}
	targetInfo, err := parseDingTalkMessageTarget(target)
	if err != nil {
		return err
	}
	switch targetInfo.kind {
	case dingTalkTargetWebhook:
		if a.replier == nil {
			a.replier = chatbot.NewChatbotReplier()
		}
		markdown := fmt.Sprintf("![图片](%s)", imageURL)
		log.Printf("[发送][钉钉][%s]：[图片]%s", target, imageURL)
		return a.replier.SimpleReplyMarkdown(context.Background(), targetInfo.id, []byte("图片"), []byte(markdown))
	case dingTalkTargetConversation:
		return fmt.Errorf("钉钉 Stream 适配器暂未实现会话主动发送图片，请使用最近消息的 session webhook 回复目标")
	case dingTalkTargetUser:
		return fmt.Errorf("钉钉 Stream 适配器暂未实现用户主动发送图片，请使用最近消息的 session webhook 回复目标")
	default:
		return fmt.Errorf("钉钉消息目标类型无效: %s", targetInfo.kind)
	}
}

func (a *DingTalkAdapter) SendFile(target string, filePath string) error {
	return fmt.Errorf("钉钉 Stream 适配器暂未实现文件发送")
}

func (a *DingTalkAdapter) GetUserInfo(userID string) (*UserInfo, error) {
	return nil, fmt.Errorf("钉钉 Stream 适配器暂未实现用户信息查询")
}

func (a *DingTalkAdapter) GetGroupInfo(groupID string) (*GroupInfo, error) {
	return nil, fmt.Errorf("钉钉 Stream 适配器暂未实现群组信息查询")
}

func (a *DingTalkAdapter) AtUser(groupID string, userID string) error {
	return fmt.Errorf("钉钉 Stream 适配器暂未实现 @ 用户")
}

func (a *DingTalkAdapter) newStreamClient() dingTalkStreamClient {
	options := []client.ClientOption{
		client.WithAppCredential(client.NewAppCredentialConfig(a.clientID, a.clientSecret)),
		client.WithAutoReconnect(true),
	}
	if a.openAPIHost != "" {
		options = append(options, client.WithOpenApiHost(a.openAPIHost))
	}
	if a.proxyURL != "" {
		options = append(options, client.WithProxy(a.proxyURL))
	}
	return client.NewStreamClient(options...)
}

func (a *DingTalkAdapter) handleChatBotMessage(ctx context.Context, data *chatbot.BotCallbackDataModel) ([]byte, error) {
	msg := a.buildMessage(data)
	if msg == nil {
		return nil, nil
	}
	log.Printf("[接收][钉钉][%s][user=%s][group=%s][conversationType=%s][conversation=%s]：%s", msg.Metadata["message_type"], msg.UserID, msg.GroupID, msg.Metadata["dingtalk_conversation_type"], msg.Metadata["dingtalk_conversation_id"], msg.Content)
	go a.dispatchMessage(msg)
	return nil, nil
}

func (a *DingTalkAdapter) buildMessage(data *chatbot.BotCallbackDataModel) *types.Message {
	if data == nil || strings.TrimSpace(data.Msgtype) != "text" {
		return nil
	}
	content := strings.TrimSpace(data.Text.Content)
	if content == "" {
		return nil
	}
	userID := firstNonEmpty(data.SenderStaffId, data.SenderId)
	conversationID := strings.TrimSpace(data.ConversationId)
	conversationType := strings.TrimSpace(data.ConversationType)
	messageType := dingTalkMessageType(conversationType)
	groupID := ""
	if messageType == "group" {
		groupID = conversationID
	}
	atUserIDs, atStaffIDs := dingTalkAtUsersMetadata(data.AtUsers)
	metadata := map[string]string{
		"message_type":                messageType,
		"dingtalk_message_id":         strings.TrimSpace(data.MsgId),
		"dingtalk_conversation_id":    conversationID,
		"dingtalk_conversation_type":  conversationType,
		"dingtalk_sender_id":          strings.TrimSpace(data.SenderId),
		"dingtalk_sender_staff_id":    strings.TrimSpace(data.SenderStaffId),
		"dingtalk_sender_nick":        strings.TrimSpace(data.SenderNick),
		"dingtalk_robot_code":         a.robotCode,
		"dingtalk_chatbot_user_id":    strings.TrimSpace(data.ChatbotUserId),
		"dingtalk_chatbot_corp_id":    strings.TrimSpace(data.ChatbotCorpId),
		"dingtalk_sender_corp_id":     strings.TrimSpace(data.SenderCorpId),
		"dingtalk_conversation_title": strings.TrimSpace(data.ConversationTitle),
		"dingtalk_session_webhook":    strings.TrimSpace(data.SessionWebhook),
		"dingtalk_webhook_expired_at": fmt.Sprintf("%d", data.SessionWebhookExpiredTime),
		"dingtalk_session_create_at":  fmt.Sprintf("%d", data.CreateAt),
		"dingtalk_is_admin":           fmt.Sprintf("%t", data.IsAdmin),
		"dingtalk_is_in_at_list":      fmt.Sprintf("%t", data.IsInAtList),
		"dingtalk_at_user_ids":        atUserIDs,
		"dingtalk_at_staff_ids":       atStaffIDs,
	}
	metadata["reply_target"] = a.ReplyTarget(&types.Message{UserID: userID, GroupID: groupID, Metadata: metadata})
	return &types.Message{
		ID:       strings.TrimSpace(data.MsgId),
		Platform: platformName,
		UserID:   userID,
		GroupID:  groupID,
		Content:  content,
		Metadata: metadata,
	}
}

func (a *DingTalkAdapter) dispatchMessage(msg *types.Message) {
	if a.messageHandler != nil {
		a.messageHandler(msg)
	}
}

func dingTalkMessageType(conversationType string) string {
	switch strings.ToLower(strings.TrimSpace(conversationType)) {
	case "2", "group", "groupchat", "group_chat", "chat":
		return "group"
	default:
		return "private"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

type dingTalkCQAtText struct {
	content   string
	atUserIDs []string
	isAtAll   bool
	hasAt     bool
}

func parseDingTalkCQAtText(text string) dingTalkCQAtText {
	var builder strings.Builder
	seenUsers := make(map[string]struct{})
	parsed := dingTalkCQAtText{content: text}
	for i := 0; i < len(text); {
		if !strings.HasPrefix(text[i:], "[CQ:") {
			builder.WriteByte(text[i])
			i++
			continue
		}
		end := strings.IndexByte(text[i:], ']')
		if end < 0 {
			builder.WriteString(text[i:])
			break
		}
		cqCode := text[i+4 : i+end]
		qq, ok := dingTalkCQAtQQ(cqCode)
		if !ok {
			builder.WriteString(text[i : i+end+1])
			i += end + 1
			continue
		}
		parsed.hasAt = true
		if strings.EqualFold(qq, "all") {
			parsed.isAtAll = true
		} else if _, exists := seenUsers[qq]; !exists {
			seenUsers[qq] = struct{}{}
			parsed.atUserIDs = append(parsed.atUserIDs, qq)
		}
		i += end + 1
	}
	parsed.content = strings.TrimSpace(builder.String())
	if parsed.content == "" && parsed.hasAt {
		parsed.content = " "
	}
	return parsed
}

func dingTalkCQAtQQ(cqCode string) (string, bool) {
	parts := strings.Split(cqCode, ",")
	if len(parts) < 2 || strings.TrimSpace(parts[0]) != "at" {
		return "", false
	}
	for _, part := range parts[1:] {
		key, value, ok := strings.Cut(part, "=")
		if !ok || strings.TrimSpace(key) != "qq" {
			continue
		}
		if qq := strings.TrimSpace(value); qq != "" {
			return qq, true
		}
		return "", false
	}
	return "", false
}

func dingTalkAtUsersMetadata(atUsers []chatbot.BotCallbackDataAtUserModel) (string, string) {
	userIDs := make([]string, 0, len(atUsers))
	staffIDs := make([]string, 0, len(atUsers))
	for _, atUser := range atUsers {
		if dingtalkID := strings.TrimSpace(atUser.DingtalkId); dingtalkID != "" {
			userIDs = append(userIDs, dingtalkID)
		}
		if staffID := strings.TrimSpace(atUser.StaffId); staffID != "" {
			staffIDs = append(staffIDs, staffID)
		}
	}
	return strings.Join(userIDs, ","), strings.Join(staffIDs, ",")
}

func dingTalkWebhookTarget(sessionWebhook string) string {
	return "webhook_b64_" + base64.RawURLEncoding.EncodeToString([]byte(sessionWebhook))
}

func dingTalkMarkdownTitle(markdown string) string {
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
		if line != "" {
			return line
		}
	}
	return "Markdown"
}

func parseDingTalkMessageTarget(target string) (dingTalkMessageTarget, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return dingTalkMessageTarget{}, fmt.Errorf("钉钉消息目标不能为空")
	}
	if strings.HasPrefix(target, "webhook_b64_") {
		encoded := strings.TrimSpace(strings.TrimPrefix(target, "webhook_b64_"))
		if encoded == "" {
			return dingTalkMessageTarget{}, fmt.Errorf("钉钉 session webhook 不能为空")
		}
		decoded, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil {
			return dingTalkMessageTarget{}, fmt.Errorf("钉钉 session webhook 编码无效: %w", err)
		}
		webhook := strings.TrimSpace(string(decoded))
		if webhook == "" {
			return dingTalkMessageTarget{}, fmt.Errorf("钉钉 session webhook 不能为空")
		}
		return dingTalkMessageTarget{kind: dingTalkTargetWebhook, id: webhook}, nil
	}
	if strings.HasPrefix(target, "conversation_") {
		id := strings.TrimSpace(strings.TrimPrefix(target, "conversation_"))
		if id == "" {
			return dingTalkMessageTarget{}, fmt.Errorf("钉钉 conversation_id 不能为空")
		}
		return dingTalkMessageTarget{kind: dingTalkTargetConversation, id: id}, nil
	}
	if strings.HasPrefix(target, "user_") {
		id := strings.TrimSpace(strings.TrimPrefix(target, "user_"))
		if id == "" {
			return dingTalkMessageTarget{}, fmt.Errorf("钉钉 user_id 不能为空")
		}
		return dingTalkMessageTarget{kind: dingTalkTargetUser, id: id}, nil
	}
	return dingTalkMessageTarget{}, fmt.Errorf("钉钉消息目标格式无效")
}
