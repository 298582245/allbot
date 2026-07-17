package builtin

import (
	"regexp"
	"strings"

	"github.com/allbot/allbot/core/config"
)

type Handler func(ctx *Context) error

type Command struct {
	Name    string
	Handler Handler
}

var commands = map[string]Command{
	"myid":    {Name: "myid", Handler: replyMyID},
	"注册":      {Name: "注册", Handler: replyRegister},
	"积分充值":    {Name: "积分充值", Handler: replyRechargePoints},
	"用户搜索":    {Name: "用户搜索", Handler: replyUserSearch},
	"绑定码":     {Name: "绑定码", Handler: replyBindCode},
	"绑定":      {Name: "绑定", Handler: replyBind},
	"我的平台":    {Name: "我的平台", Handler: replyMyPlatforms},
	"groupid": {Name: "groupid", Handler: replyGroupID},
	"botid":   {Name: "botid", Handler: replyBotID},
	"插件列表":    {Name: "插件列表", Handler: replyPluginList},
	"system":  {Name: "system", Handler: replySystem},
	"version": {Name: "version", Handler: replyVersion},
	"更新":      {Name: "更新", Handler: replyUpdate},
	"重启":      {Name: "重启", Handler: replyRestart},
}

func Match(item *config.KeywordReply, content string) bool {
	if item == nil {
		return false
	}
	if item.Builtin {
		switch strings.ToLower(strings.TrimSpace(item.Keyword)) {
		case "myid":
			return strings.EqualFold(content, "myid")
		case "groupid":
			return strings.EqualFold(content, "groupid")
		case "botid":
			return strings.EqualFold(content, "botid")
		case "version":
			return strings.EqualFold(content, "version")
		}
		if strings.EqualFold(item.Keyword, "绑定") {
			return content == "绑定" || strings.HasPrefix(content, "绑定 ")
		}
		if strings.EqualFold(item.Keyword, "积分充值") {
			return content == "积分充值" || strings.HasPrefix(content, "积分充值 ")
		}
		if strings.EqualFold(item.Keyword, "用户搜索") {
			return content == "用户搜索" || strings.HasPrefix(content, "用户搜索 ")
		}
	}
	if strings.EqualFold(item.MatchType, "regex") {
		matched, err := regexp.MatchString(item.Keyword, content)
		return err == nil && matched
	}
	if strings.EqualFold(item.MatchType, "exact") {
		return strings.EqualFold(content, item.Keyword)
	}
	return strings.EqualFold(content, item.Keyword)
}

func Dispatch(ctx *Context, keyword string) error {
	command, ok := commands[normalizeCommandName(keyword)]
	if !ok || command.Handler == nil {
		return nil
	}
	return command.Handler(ctx)
}

func normalizeCommandName(keyword string) string {
	keyword = strings.TrimSpace(keyword)
	switch strings.ToLower(keyword) {
	case "myid":
		return "myid"
	case "groupid":
		return "groupid"
	case "botid":
		return "botid"
	case "version":
		return "version"
	case "system":
		return "system"
	default:
		return keyword
	}
}
