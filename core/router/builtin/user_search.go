package builtin

import (
	"fmt"
	"strings"
)

func replyUserSearch(ctx *Context) error {
	return ctx.SendText(searchUserByUnionID(ctx))
}

func searchUserByUnionID(ctx *Context) string {
	if !ctx.IsAdmin() {
		return "仅平台管理员可使用用户搜索"
	}
	args := strings.Fields(strings.TrimSpace(strings.TrimPrefix(ctx.Message.Content, "用户搜索")))
	unionID, err := resolveUserSearchUnionID(ctx, args)
	if err != nil {
		return "用户搜索失败：" + err.Error()
	}
	accounts, err := ctx.Database.ListUserAccountsByUnionID(unionID)
	if err != nil {
		return "用户搜索失败：" + err.Error()
	}
	points, err := ctx.Database.GetUserPoints(unionID)
	if err != nil {
		return "用户搜索失败：" + err.Error()
	}
	unit := pointsUnit(ctx)
	lines := []string{
		"用户搜索结果",
		"UnionID：" + unionID,
		fmt.Sprintf("%s：%d", unit, points),
		fmt.Sprintf("关联账号（%d个）：", len(accounts)),
	}
	if len(accounts) == 0 {
		lines = append(lines, "无关联平台账号")
	}
	for index, account := range accounts {
		lines = append(lines, fmt.Sprintf("%d. 平台：%s 用户号：%s", index+1, account.Platform, account.UserID))
	}
	return strings.Join(lines, "\n")
}

func resolveUserSearchUnionID(ctx *Context, args []string) (string, error) {
	if len(args) == 1 {
		if parts := strings.SplitN(args[0], ":", 2); len(parts) == 2 {
			return unionIDByPlatformUser(ctx, parts[0], parts[1])
		}
		unionID := strings.TrimSpace(args[0])
		exists, err := ctx.Database.UserUnionExists(unionID)
		if err != nil {
			return "", err
		}
		if !exists {
			return "", fmt.Errorf("账号不存在，请检查 UnionID 或平台用户号是否正确")
		}
		return unionID, nil
	}
	if len(args) == 2 {
		return unionIDByPlatformUser(ctx, args[0], args[1])
	}
	return "", fmt.Errorf("用法：用户搜索 <unionId>，或 用户搜索 <平台>:<用户号>，或 用户搜索 <平台> <用户号>")
}

func unionIDByPlatformUser(ctx *Context, platform, userID string) (string, error) {
	platform = strings.TrimSpace(platform)
	userID = strings.TrimSpace(userID)
	if platform == "" || userID == "" {
		return "", fmt.Errorf("平台和用户号不能为空")
	}
	account, err := ctx.Database.GetUserAccount(platform, userID)
	if err != nil {
		return "", fmt.Errorf("账号不存在，请确认用户已发送【注册】或已绑定过账号")
	}
	return account.UnionID, nil
}
