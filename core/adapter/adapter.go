package adapter

import "github.com/allbot/allbot/core/adapter/_contract"

// Adapter 平台适配器接口。
type Adapter = contract.Adapter

// MessageDeleter 由适配器按平台能力撤回消息。
type MessageDeleter = contract.MessageDeleter

// GroupMuter 由适配器按平台能力执行群成员或全体禁言。
type GroupMuter = contract.GroupMuter

// ErrUnsupported 表示适配器明确不支持当前操作。
var ErrUnsupported = contract.ErrUnsupported

// NormalizeUserID 将常见平台的 @ 用户格式解析为用户 ID。
func NormalizeUserID(value string) string {
	return contract.NormalizeUserID(value)
}

// BotIdentity 平台机器人公开身份。
type BotIdentity = contract.BotIdentity

// BotIdentityProvider 由适配器提供当前消息对应的机器人公开身份。
type BotIdentityProvider = contract.BotIdentityProvider

// MarkdownSender 由适配器按自身能力发送 Markdown 消息。
type MarkdownSender = contract.MarkdownSender

// RichMessageSender 由适配器按自身能力发送富文本消息。
type RichMessageSender = contract.RichMessageSender

// ButtonSender 由适配器按自身能力发送按钮消息。
type ButtonSender = contract.ButtonSender

// MessageSequenceSender 由适配器按协议指定回复消息序号。
type MessageSequenceSender = contract.MessageSequenceSender

// ReplyTargetResolver 由适配器按自身目标格式解析回复目标。
type ReplyTargetResolver = contract.ReplyTargetResolver

// ReplyTextFormatter 由适配器按自身消息格式处理回复文本。
type ReplyTextFormatter = contract.ReplyTextFormatter

// SendTargetResolver 由适配器按自身目标格式解析插件主动发送目标。
type SendTargetResolver = contract.SendTargetResolver

// UserInfo 用户信息。
type UserInfo = contract.UserInfo

// GroupInfo 群组信息。
type GroupInfo = contract.GroupInfo
