package adapter

import "github.com/allbot/allbot/core/adapter/_contract"

// Adapter 平台适配器接口。
type Adapter = contract.Adapter

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
