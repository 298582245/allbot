package builtin

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/allbot/allbot/core/config"
	plugincore "github.com/allbot/allbot/core/plugin"
	"github.com/allbot/allbot/core/types"
)

const (
	pluginListPageSize      = 10
	pluginListListenTimeout = 60
)

func replyPluginList(ctx *Context) error {
	if !ctx.IsAdmin() {
		return ctx.SendText("仅平台管理员可使用插件列表")
	}
	if ctx.PluginStore == nil {
		return ctx.SendText("插件管理器未初始化")
	}
	if ctx.Listen == nil {
		return ctx.SendText("连续对话功能未初始化")
	}
	plugins := sortedPluginProcesses(ctx)
	if len(plugins) == 0 {
		return ctx.SendText("暂无插件")
	}
	if err := sendPluginListPage(ctx, plugins, 0); err != nil {
		return err
	}
	go runPluginListConversation(ctx, 0)
	return nil
}

func runPluginListConversation(ctx *Context, page int) {
	for {
		plugins := sortedPluginProcesses(ctx)
		if len(plugins) == 0 {
			_ = ctx.SendText("暂无插件")
			return
		}
		page = clampPluginListPage(page, len(plugins))
		input := listenPluginList(ctx)
		if input == "" {
			return
		}
		switch {
		case isQuitInput(input):
			_ = ctx.SendText("已退出插件列表")
			return
		case isNextPageInput(input):
			if page >= pluginListPageCount(len(plugins))-1 {
				_ = ctx.SendText("已经是最后一页\n\n" + formatPluginListPage(plugins, page))
				continue
			}
			page++
			_ = sendPluginListPage(ctx, plugins, page)
		case isPrevPageInput(input):
			if page <= 0 {
				_ = ctx.SendText("已经是第一页\n\n" + formatPluginListPage(plugins, page))
				continue
			}
			page--
			_ = sendPluginListPage(ctx, plugins, page)
		default:
			plugin := pluginByPageChoice(plugins, page, input)
			if plugin == nil {
				_ = ctx.SendText("请输入列表中的数字，或发送 下一页/上一页/q")
				continue
			}
			if !runPluginOperationConversation(ctx, plugin.Plugin.ID) {
				return
			}
			_ = ctx.SendText(formatPluginListPage(sortedPluginProcesses(ctx), page))
		}
	}
}

func runPluginOperationConversation(ctx *Context, pluginID string) bool {
	for {
		process := findPluginProcess(ctx, pluginID)
		if process == nil || process.Plugin == nil {
			_ = ctx.SendText("插件不存在或已卸载")
			return true
		}
		_ = sendPluginOperationMenu(ctx, process.Plugin)
		input := listenPluginList(ctx)
		if input == "" {
			return false
		}
		if isQuitInput(input) {
			_ = ctx.SendText("已退出插件列表")
			return false
		}
		if isBackInput(input) {
			return true
		}
		switch input {
		case "1":
			if !togglePluginFromConversation(ctx, pluginID) {
				return true
			}
		case "2":
			if !runPluginAccessControlConversation(ctx, pluginID) {
				return false
			}
		default:
			_ = ctx.SendText("请输入 1 或 2，发送 b 返回插件列表，q 退出")
		}
	}
}

func runPluginAccessControlConversation(ctx *Context, pluginID string) bool {
	for {
		process := findPluginProcess(ctx, pluginID)
		if process == nil || process.Plugin == nil {
			_ = ctx.SendText("插件不存在或已卸载")
			return true
		}
		_ = sendPluginAccessControlMenu(ctx, process.Plugin)
		input := listenPluginList(ctx)
		if input == "" {
			return false
		}
		if isQuitInput(input) {
			_ = ctx.SendText("已退出插件列表")
			return false
		}
		if isBackInput(input) {
			return true
		}
		field, ok := pluginAccessControlField(input)
		if !ok {
			_ = ctx.SendText("请输入 1-6，发送 b 返回上一级，q 退出")
			continue
		}
		if !updatePluginAccessControlField(ctx, pluginID, field) {
			return false
		}
	}
}

func updatePluginAccessControlField(ctx *Context, pluginID string, field pluginAccessField) bool {
	_ = ctx.SendText(fmt.Sprintf("请输入要修改的%s：\n+123,+456,-789\n+ 表示添加，- 表示删除，多个用英文逗号分隔\n发送 b 返回上一级，q 退出", field.label))
	input := listenPluginList(ctx)
	if input == "" {
		return false
	}
	if isQuitInput(input) {
		_ = ctx.SendText("已退出插件列表")
		return false
	}
	if isBackInput(input) {
		return true
	}
	process := findPluginProcess(ctx, pluginID)
	if process == nil || process.Plugin == nil {
		_ = ctx.SendText("插件不存在或已卸载")
		return true
	}
	accessControl := process.Plugin.AccessControl
	updated, err := applyPluginAccessControlOperations(field.values(accessControl), input)
	if err != nil {
		_ = ctx.SendText(err.Error())
		return true
	}
	field.assign(&accessControl, updated)
	accessControl = config.NormalizeAccessControlConfig(accessControl)
	if pluginAccessControlHasRules(accessControl) {
		accessControl.InheritSystem = false
	}
	if err := ctx.PluginStore.SavePluginAccessControl(pluginID, accessControl); err != nil {
		_ = ctx.SendText("访问控制保存失败：" + err.Error())
		return true
	}
	if err := registerManagedPlugin(ctx, pluginID); err != nil {
		_ = ctx.SendText("访问控制已保存，但路由刷新失败：" + err.Error())
		return true
	}
	_ = ctx.SendText(fmt.Sprintf("已更新【%s】%s\n当前值：%s", pluginDisplayName(process.Plugin), field.label, formatPluginAccessValues(updated)))
	return true
}

func togglePluginFromConversation(ctx *Context, pluginID string) bool {
	process := findPluginProcess(ctx, pluginID)
	if process == nil || process.Plugin == nil {
		_ = ctx.SendText("插件不存在或已卸载")
		return false
	}
	nextEnabled := !process.Plugin.Enabled
	if err := ctx.PluginStore.TogglePlugin(pluginID, nextEnabled); err != nil {
		_ = ctx.SendText("插件状态修改失败：" + err.Error())
		return false
	}
	if err := registerManagedPlugin(ctx, pluginID); err != nil {
		_ = ctx.SendText("插件状态已修改，但路由刷新失败：" + err.Error())
		return false
	}
	status := "关闭"
	if nextEnabled {
		status = "启动"
	}
	_ = ctx.SendText(fmt.Sprintf("已%s【%s】", status, pluginDisplayName(process.Plugin)))
	return true
}

func sortedPluginProcesses(ctx *Context) []*plugincore.PluginProcess {
	if ctx.PluginStore == nil {
		return nil
	}
	items := ctx.PluginStore.GetAllPlugins()
	plugins := make([]*plugincore.PluginProcess, 0, len(items))
	for _, item := range items {
		if item == nil || item.Plugin == nil || strings.TrimSpace(item.Plugin.ID) == "" {
			continue
		}
		plugins = append(plugins, item)
	}
	sort.SliceStable(plugins, func(i, j int) bool {
		left := plugins[i].Plugin
		right := plugins[j].Plugin
		if left.Order != right.Order {
			if left.Order == 0 {
				return false
			}
			if right.Order == 0 {
				return true
			}
			return left.Order < right.Order
		}
		leftName := strings.ToLower(pluginDisplayName(left))
		rightName := strings.ToLower(pluginDisplayName(right))
		if leftName != rightName {
			return leftName < rightName
		}
		return left.ID < right.ID
	})
	return plugins
}

func findPluginProcess(ctx *Context, pluginID string) *plugincore.PluginProcess {
	pluginID = strings.TrimSpace(pluginID)
	for _, item := range sortedPluginProcesses(ctx) {
		if item.Plugin.ID == pluginID {
			return item
		}
	}
	return nil
}

func listenPluginList(ctx *Context) string {
	return ctx.ListenText(pluginListListenTimeout)
}

func registerManagedPlugin(ctx *Context, pluginID string) error {
	if ctx.RegisterPlugin == nil {
		return nil
	}
	process := findPluginProcess(ctx, pluginID)
	if process == nil || process.Plugin == nil {
		return fmt.Errorf("插件不存在或已卸载")
	}
	return ctx.RegisterPlugin(process.Plugin)
}

func sendPluginListPage(ctx *Context, plugins []*plugincore.PluginProcess, page int) error {
	text := formatPluginListPage(plugins, page)
	if ctx.ReplyButtons == nil || len(plugins) == 0 {
		return ctx.SendText(text)
	}
	page = clampPluginListPage(page, len(plugins))
	start := page * pluginListPageSize
	end := start + pluginListPageSize
	if end > len(plugins) {
		end = len(plugins)
	}
	buttons := make([][]types.ButtonOption, 0, end-start+2)
	for index, process := range plugins[start:end] {
		buttons = append(buttons, []types.ButtonOption{{Text: fmt.Sprintf("%d. %s", index+1, shortButtonText(pluginDisplayName(process.Plugin), 18)), Value: strconv.Itoa(index + 1)}})
	}
	if pluginListPageCount(len(plugins)) > 1 {
		buttons = append(buttons, []types.ButtonOption{{Text: "上一页", Value: "上一页"}, {Text: "下一页", Value: "下一页"}})
	}
	buttons = append(buttons, []types.ButtonOption{{Text: "退出", Value: "q"}})
	return ctx.ReplyButtons(text, buttons)
}

func sendPluginOperationMenu(ctx *Context, plugin *types.Plugin) error {
	text := formatPluginOperationMenu(plugin)
	if ctx.ReplyButtons == nil {
		return ctx.SendText(text)
	}
	action := "启动插件"
	if plugin != nil && plugin.Enabled {
		action = "关闭插件"
	}
	return ctx.ReplyButtons(text, [][]types.ButtonOption{{{Text: action, Value: "1"}, {Text: "访问控制设置", Value: "2"}}, {{Text: "返回列表", Value: "b"}, {Text: "退出", Value: "q"}}})
}

func sendPluginAccessControlMenu(ctx *Context, plugin *types.Plugin) error {
	text := formatPluginAccessControlMenu(plugin)
	if ctx.ReplyButtons == nil {
		return ctx.SendText(text)
	}
	return ctx.ReplyButtons(text, [][]types.ButtonOption{
		{{Text: "白名单群", Value: "1"}, {Text: "屏蔽群消息", Value: "2"}},
		{{Text: "白名单 ID", Value: "3"}, {Text: "黑名单 ID", Value: "4"}},
		{{Text: "白名单 union_id", Value: "5"}, {Text: "黑名单 union_id", Value: "6"}},
		{{Text: "返回上一级", Value: "b"}, {Text: "退出", Value: "q"}},
	})
}

func formatPluginListPage(plugins []*plugincore.PluginProcess, page int) string {
	if len(plugins) == 0 {
		return "暂无插件"
	}
	page = clampPluginListPage(page, len(plugins))
	pageCount := pluginListPageCount(len(plugins))
	start := page * pluginListPageSize
	end := start + pluginListPageSize
	if end > len(plugins) {
		end = len(plugins)
	}
	lines := []string{fmt.Sprintf("插件列表 第%d/%d页（共%d个）", page+1, pageCount, len(plugins))}
	for index, process := range plugins[start:end] {
		status := "❌"
		if process.Plugin.Enabled {
			status = "✅"
		}
		lines = append(lines, fmt.Sprintf("%d. %s %s", index+1, pluginDisplayNameWithID(process.Plugin), status))
	}
	lines = append(lines, "", "发送数字选择插件")
	if pageCount > 1 {
		lines = append(lines, "发送 下一页/n 或 上一页/p 翻页")
	}
	lines = append(lines, "发送 q 退出")
	return strings.Join(lines, "\n")
}

func formatPluginOperationMenu(plugin *types.Plugin) string {
	action := "启动插件"
	if plugin.Enabled {
		action = "关闭插件"
	}
	return fmt.Sprintf("请对【%s】进行操作\n[1] %s\n[2] 访问控制设置\n\n发送 b 返回插件列表，q 退出", pluginDisplayName(plugin), action)
}

func formatPluginAccessControlMenu(plugin *types.Plugin) string {
	accessControl := plugin.AccessControl
	return fmt.Sprintf("【%s】访问控制设置\n[1] 白名单群（%d）\n[2] 屏蔽群消息（%d）\n[3] 白名单 ID（%d）\n[4] 黑名单 ID（%d）\n[5] 白名单 union_id（%d）\n[6] 黑名单 union_id（%d）\n\n发送 b 返回上一级，q 退出", pluginDisplayName(plugin), len(accessControl.WhitelistGroups), len(accessControl.BlockedGroups), len(accessControl.WhitelistUserIDs), len(accessControl.BlockedUserIDs), len(accessControl.WhitelistUnionIDs), len(accessControl.BlockedUnionIDs))
}

func shortButtonText(value string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes]) + "…"
}

func pluginDisplayName(plugin *types.Plugin) string {
	if plugin == nil {
		return "未知插件"
	}
	if strings.TrimSpace(plugin.Name) != "" {
		return strings.TrimSpace(plugin.Name)
	}
	return plugin.ID
}

func pluginDisplayNameWithID(plugin *types.Plugin) string {
	name := pluginDisplayName(plugin)
	if plugin == nil || strings.TrimSpace(plugin.ID) == "" || name == plugin.ID {
		return name
	}
	return fmt.Sprintf("%s(%s)", name, plugin.ID)
}

func pluginListPageCount(total int) int {
	if total <= 0 {
		return 1
	}
	return (total + pluginListPageSize - 1) / pluginListPageSize
}

func clampPluginListPage(page int, total int) int {
	pageCount := pluginListPageCount(total)
	if page < 0 {
		return 0
	}
	if page >= pageCount {
		return pageCount - 1
	}
	return page
}

func pluginByPageChoice(plugins []*plugincore.PluginProcess, page int, input string) *plugincore.PluginProcess {
	choice, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil || choice <= 0 || choice > pluginListPageSize {
		return nil
	}
	index := clampPluginListPage(page, len(plugins))*pluginListPageSize + choice - 1
	if index < 0 || index >= len(plugins) {
		return nil
	}
	return plugins[index]
}

func isQuitInput(input string) bool {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "q", "quit", "退出", "取消":
		return true
	default:
		return false
	}
}

func isBackInput(input string) bool {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "b", "back", "返回", "上一级":
		return true
	default:
		return false
	}
}

func isNextPageInput(input string) bool {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "n", "next", "下一页", "下页":
		return true
	default:
		return false
	}
}

func isPrevPageInput(input string) bool {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "p", "prev", "previous", "上一页", "上页":
		return true
	default:
		return false
	}
}

type pluginAccessField struct {
	label  string
	values func(types.AccessControlConfig) []string
	assign func(*types.AccessControlConfig, []string)
}

func pluginAccessControlField(input string) (pluginAccessField, bool) {
	switch strings.TrimSpace(input) {
	case "1":
		return pluginAccessField{label: "白名单群", values: func(config types.AccessControlConfig) []string { return config.WhitelistGroups }, assign: func(config *types.AccessControlConfig, values []string) { config.WhitelistGroups = values }}, true
	case "2":
		return pluginAccessField{label: "屏蔽群消息", values: func(config types.AccessControlConfig) []string { return config.BlockedGroups }, assign: func(config *types.AccessControlConfig, values []string) { config.BlockedGroups = values }}, true
	case "3":
		return pluginAccessField{label: "白名单 ID", values: func(config types.AccessControlConfig) []string { return config.WhitelistUserIDs }, assign: func(config *types.AccessControlConfig, values []string) { config.WhitelistUserIDs = values }}, true
	case "4":
		return pluginAccessField{label: "黑名单 ID", values: func(config types.AccessControlConfig) []string { return config.BlockedUserIDs }, assign: func(config *types.AccessControlConfig, values []string) { config.BlockedUserIDs = values }}, true
	case "5":
		return pluginAccessField{label: "白名单 union_id", values: func(config types.AccessControlConfig) []string { return config.WhitelistUnionIDs }, assign: func(config *types.AccessControlConfig, values []string) { config.WhitelistUnionIDs = values }}, true
	case "6":
		return pluginAccessField{label: "黑名单 union_id", values: func(config types.AccessControlConfig) []string { return config.BlockedUnionIDs }, assign: func(config *types.AccessControlConfig, values []string) { config.BlockedUnionIDs = values }}, true
	default:
		return pluginAccessField{}, false
	}
}

func applyPluginAccessControlOperations(current []string, input string) ([]string, error) {
	items := make([]string, 0, len(current))
	seen := make(map[string]bool)
	for _, item := range current {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		items = append(items, item)
		seen[item] = true
	}
	changed := false
	for _, token := range strings.Split(input, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if len(token) < 2 || token[0] != '+' && token[0] != '-' {
			return nil, fmt.Errorf("格式错误：%s 必须以 + 或 - 开头", token)
		}
		value := strings.TrimSpace(token[1:])
		if value == "" {
			return nil, fmt.Errorf("格式错误：%s 缺少 ID", token)
		}
		switch token[0] {
		case '+':
			if !seen[value] {
				items = append(items, value)
				seen[value] = true
			}
		case '-':
			if seen[value] {
				items = removePluginAccessValue(items, value)
				delete(seen, value)
			}
		}
		changed = true
	}
	if !changed {
		return nil, fmt.Errorf("请输入至少一个 +ID 或 -ID")
	}
	return items, nil
}

func removePluginAccessValue(items []string, value string) []string {
	result := items[:0]
	for _, item := range items {
		if item != value {
			result = append(result, item)
		}
	}
	return result
}

func formatPluginAccessValues(values []string) string {
	if len(values) == 0 {
		return "空"
	}
	return strings.Join(values, ",")
}

func pluginAccessControlHasRules(accessControl types.AccessControlConfig) bool {
	return len(accessControl.WhitelistGroups) > 0 ||
		len(accessControl.BlockedGroups) > 0 ||
		len(accessControl.WhitelistUserIDs) > 0 ||
		len(accessControl.BlockedUserIDs) > 0 ||
		len(accessControl.WhitelistUnionIDs) > 0 ||
		len(accessControl.BlockedUnionIDs) > 0
}
