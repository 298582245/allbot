package builtin

import (
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"
)

func replyMyID(ctx *Context) error {
	return ctx.SendText(userIdentityInfo(ctx))
}

func replyRegister(ctx *Context) error {
	return ctx.SendText(registerUser(ctx))
}

func replyBindCode(ctx *Context) error {
	return ctx.SendText(createBindCode(ctx))
}

func replyBind(ctx *Context) error {
	return ctx.SendText(bindUser(ctx))
}

func replyGroupID(ctx *Context) error {
	groupID := groupTargetID(ctx)
	if groupID == "" {
		return nil
	}
	return ctx.SendText(groupID)
}

func replyBotID(ctx *Context) error {
	label := strings.TrimSpace(ctx.BotIdentityLabel)
	value := strings.TrimSpace(ctx.BotIdentityValue)
	if label == "" {
		label = "平台机器人身份"
	}
	if value == "" {
		value = "未知"
	}
	adapterID := strings.TrimSpace(ctx.adapterID())
	if adapterID == "" {
		adapterID = "未知"
	}
	return ctx.SendText(fmt.Sprintf("%s：%s\nAllBot 适配器实例 ID：%s", label, value, adapterID))
}

func groupTargetID(ctx *Context) string {
	if ctx == nil || ctx.Message == nil {
		return ""
	}
	msg := ctx.Message
	if msg.Platform != "qq_office" {
		return msg.GroupID
	}
	if msg.Metadata != nil {
		if groupOpenID := strings.TrimSpace(msg.Metadata["qq_office_group_openid"]); groupOpenID != "" {
			return qqOfficeGroupTarget(groupOpenID)
		}
		if guildID := strings.TrimSpace(msg.Metadata["qq_office_guild_id"]); guildID != "" {
			return qqOfficeDMSTarget(guildID)
		}
	}
	if groupID := strings.TrimSpace(msg.GroupID); groupID != "" {
		return qqOfficeGroupTarget(groupID)
	}
	return ""
}

func qqOfficeGroupTarget(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "group_") || strings.HasPrefix(value, "dms_") {
		return value
	}
	return "group_" + value
}

func qqOfficeDMSTarget(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "group_") || strings.HasPrefix(value, "dms_") {
		return value
	}
	return "dms_" + value
}

func replyMyPlatforms(ctx *Context) error {
	return ctx.SendText(userPlatformBindings(ctx))
}

func userIdentityInfo(ctx *Context) string {
	account, err := ctx.Database.GetUserAccount(ctx.Message.Platform, ctx.Message.UserID)
	if err != nil {
		return UserRegisterGuide()
	}
	unit := pointsUnit(ctx)
	return fmt.Sprintf("用户信息\n平台：%s\n用户ID：%s\nUnionID：%s\n%s：%d", account.Platform, account.UserID, account.UnionID, unit, account.Points)
}

func registerUser(ctx *Context) string {
	account, err := ctx.Database.GetUserAccount(ctx.Message.Platform, ctx.Message.UserID)
	alreadyRegistered := err == nil
	if err != nil {
		if err != sql.ErrNoRows {
			return "注册失败：" + err.Error()
		}
		account, err = ctx.Database.EnsureUserAccount(ctx.Message.Platform, ctx.Message.UserID)
		if err != nil {
			return "注册失败：" + err.Error()
		}
	}
	unit := pointsUnit(ctx)
	if alreadyRegistered {
		return fmt.Sprintf("已注册，无需重复注册\n平台：%s\n用户ID：%s\nUnionID：%s\n%s：%d", account.Platform, account.UserID, account.UnionID, unit, account.Points)
	}
	return fmt.Sprintf("注册成功\n平台：%s\n用户ID：%s\nUnionID：%s\n%s：%d", account.Platform, account.UserID, account.UnionID, unit, account.Points)
}

func createBindCode(ctx *Context) string {
	if ctx.Message.GroupID != "" {
		return "绑定码只能私聊获取，请私聊机器人发送：绑定码"
	}
	code, err := ctx.Database.CreateUserBindCode(ctx.Message.Platform, ctx.Message.UserID)
	if err != nil {
		return "生成绑定码失败：" + err.Error()
	}
	remainingMinutes := int(math.Ceil(time.Until(code.ExpiresAt).Minutes()))
	if remainingMinutes < 1 {
		remainingMinutes = 1
	}
	return fmt.Sprintf("绑定码：%s\n请在其他平台私聊机器人发送：绑定 %s\n剩余有效期：%d分钟", code.Code, code.Code, remainingMinutes)
}

func userPlatformBindings(ctx *Context) string {
	if ctx.Message.GroupID != "" {
		return "我的平台只能私聊查看，请私聊机器人发送：我的平台"
	}
	account, err := ctx.Database.GetUserAccount(ctx.Message.Platform, ctx.Message.UserID)
	if err != nil {
		return UserRegisterGuide()
	}
	accounts, err := ctx.Database.ListUserAccountsByUnionID(account.UnionID)
	if err != nil {
		return "查询已绑定平台失败：" + err.Error()
	}
	lines := []string{fmt.Sprintf("UnionID：%s", account.UnionID), "已绑定平台："}
	for _, item := range accounts {
		lines = append(lines, fmt.Sprintf("- %s：%s", item.Platform, item.UserID))
	}
	return strings.Join(lines, "\n")
}

func bindUser(ctx *Context) string {
	if ctx.Message.GroupID != "" {
		return "绑定只能私聊操作，请私聊机器人发送：绑定 绑定码"
	}
	code := strings.TrimSpace(strings.TrimPrefix(ctx.Message.Content, "绑定"))
	if code == "" {
		return "请输入绑定码，例如：绑定 AbCdEfGhIjKlMnOpQrStUv"
	}
	account, source, err := ctx.Database.BindUserByCode(ctx.Message.Platform, ctx.Message.UserID, code)
	if err != nil {
		return "绑定失败：" + err.Error()
	}
	return fmt.Sprintf("绑定成功\n当前平台：%s\n来源平台：%s\nUnionID：%s", account.Platform, source.Platform, account.UnionID)
}

func pointsUnit(ctx *Context) string {
	unit, err := ctx.Database.GetSetting("user.points_unit")
	if err != nil || strings.TrimSpace(unit) == "" {
		return "积分"
	}
	return strings.TrimSpace(unit)
}

func UserRegisterGuide() string {
	return "当前用户还未注册。\n请选择：\n1. 发送「注册」自动注册当前平台账号\n2. 如需绑定其他平台，请先到已注册平台私聊发送「绑定码」，再回到当前平台私聊发送「绑定 绑定码」"
}
