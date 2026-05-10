package adapter

import "github.com/allbot/allbot/core/types"

// Adapter 平台适配器接口
type Adapter interface {
	// GetPlatform 获取平台名称
	GetPlatform() string

	// SendMessage 发送消息
	SendMessage(target string, text string) error

	// SendImage 发送图片
	SendImage(target string, imageURL string) error

	// SendFile 发送文件
	SendFile(target string, filePath string) error

	// GetUserInfo 获取用户信息
	GetUserInfo(userID string) (*UserInfo, error)

	// GetGroupInfo 获取群组信息
	GetGroupInfo(groupID string) (*GroupInfo, error)

	// AtUser @某人
	AtUser(groupID string, userID string) error

	// Start 启动适配器
	Start() error

	// Stop 停止适配器
	Stop() error

	// SetMessageHandler 设置消息处理器
	SetMessageHandler(handler func(*types.Message))
}

// UserInfo 用户信息
type UserInfo struct {
	UserID   string
	Nickname string
	Avatar   string
	Extra    map[string]string
}

// GroupInfo 群组信息
type GroupInfo struct {
	GroupID     string
	Name        string
	MemberCount int
	Extra       map[string]string
}
