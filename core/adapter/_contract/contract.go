package contract

import (
	"net/http"

	"github.com/allbot/allbot/core/types"
)

// Adapter 定义平台适配器的统一能力契约。
type Adapter interface {
	// GetPlatform 获取平台名称。
	GetPlatform() string

	// SendMessage 发送文本消息。
	SendMessage(target string, text string) error

	// SendImage 发送图片消息。
	SendImage(target string, imageURL string) error

	// SendFile 发送文件消息。
	SendFile(target string, filePath string) error

	// GetUserInfo 获取用户信息。
	GetUserInfo(userID string) (*UserInfo, error)

	// GetGroupInfo 获取群组信息。
	GetGroupInfo(groupID string) (*GroupInfo, error)

	// AtUser 在群组中 @ 用户。
	AtUser(groupID string, userID string) error

	// Start 启动适配器。
	Start() error

	// Stop 停止适配器。
	Stop() error

	// SetMessageHandler 设置消息处理器。
	SetMessageHandler(handler func(*types.Message))
}

// BotIdentity 表示平台可可靠取得的机器人公开身份。
type BotIdentity struct {
	Label string
	Value string
}

// BotIdentityProvider 由适配器提供当前消息对应的机器人公开身份。
// msg 可为 nil；实现只能读取已缓存或非敏感的公开标识，不得临时发起网络请求。
// 返回空 Value 表示当前无法可靠取得身份。
type BotIdentityProvider interface {
	GetBotIdentity(msg *types.Message) BotIdentity
}

// MarkdownSender 由适配器按自身能力发送 Markdown 消息。
type MarkdownSender interface {
	SendMarkdown(target string, markdown string) error
}

// RichMessageSender 由适配器按自身能力发送富文本消息。
type RichMessageSender interface {
	SendRichMessage(target string, message types.RichMessage) error
}

// ButtonSender 由适配器按自身能力发送按钮消息。
type ButtonSender interface {
	SendButtons(target string, text string, buttons [][]types.ButtonOption) error
}

// MessageSequenceSender 由适配器按协议指定回复消息序号。
type MessageSequenceSender interface {
	SendMessageWithSequence(target string, text string, sequence int) error
}

// ReplyTargetResolver 由适配器按自身目标格式解析回复目标。
type ReplyTargetResolver interface {
	ReplyTarget(msg *types.Message) string
}

// ReplyTextFormatter 由适配器按自身消息格式处理回复文本。
type ReplyTextFormatter interface {
	FormatReplyText(msg *types.Message, text string) string
}

// SendTargetResolver 由适配器按自身目标格式解析插件主动发送目标。
type SendTargetResolver interface {
	SendTarget(userID string, groupID string) string
}

// HTTPCallbackHandler 由需要接收开放 HTTP 回调的平台适配器实现。
type HTTPCallbackHandler interface {
	HandleHTTPCallback(relativePath string, w http.ResponseWriter, r *http.Request)
}

// UserInfo 表示平台用户信息。
type UserInfo struct {
	UserID   string
	Nickname string
	Avatar   string
	Extra    map[string]string
}

// GroupInfo 表示平台群组信息。
type GroupInfo struct {
	GroupID     string
	Name        string
	MemberCount int
	Extra       map[string]string
}
