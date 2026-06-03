package contract

import "github.com/allbot/allbot/core/types"

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
