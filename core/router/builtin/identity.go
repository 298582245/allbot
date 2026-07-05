package builtin

import (
	"database/sql"
	"fmt"
	"strings"
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
	if ctx.Message.GroupID == "" {
		return nil
	}
	return ctx.SendText(ctx.Message.GroupID)
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
	return fmt.Sprintf("绑定码：%s\n请在其他平台私聊机器人发送：绑定 %s\n有效期：10分钟", code.Code, code.Code)
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
		return "请输入绑定码，例如：绑定 123456"
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
