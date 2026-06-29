package router

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/allbot/allbot/core/adapter"
	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/payment"
	plugincore "github.com/allbot/allbot/core/plugin"
	"github.com/allbot/allbot/core/types"
	"github.com/allbot/allbot/core/updater"
	"github.com/allbot/allbot/core/version"
)

type RestartRequest struct {
	MessageKey string
	Platform   string
	AdapterID  string
	UserID     string
	GroupID    string
	Target     string
	StartedAt  time.Time
}

type UpdateHandler interface {
	StartUpgrade(ctx context.Context) (updater.UpgradeState, error)
	CurrentState() updater.UpgradeState
}

type keywordPluginAdminStore interface {
	GetAllPlugins() []*plugincore.PluginProcess
	TogglePlugin(pluginID string, enabled bool) error
	SavePluginAccessControl(pluginID string, accessControl types.AccessControlConfig) error
}

type keywordListenFunc func(msg *types.Message, timeout int) string
type keywordListenUntilFunc func(msg *types.Message, timeout int, done <-chan struct{}) string

type KeywordReplyManager struct {
	database         *config.Database
	adapterFor       func(msg *types.Message) adapter.Adapter
	adminCheck       func(platform, userID string) bool
	pluginStore      keywordPluginAdminStore
	registerPlugin   func(*types.Plugin) error
	listen           keywordListenFunc
	listenUntil      keywordListenUntilFunc
	startTime        time.Time
	releaseClient    updater.ReleaseClient
	updateHandler   UpdateHandler
	restartHandler   func(RestartRequest) error
	restartMu        sync.Mutex
	restartRequested bool
}

func NewKeywordReplyManager(database *config.Database, adapterFor func(msg *types.Message) adapter.Adapter, adminCheck func(platform, userID string) bool, startTime time.Time) *KeywordReplyManager {
	return &KeywordReplyManager{database: database, adapterFor: adapterFor, adminCheck: adminCheck, startTime: startTime, releaseClient: updater.NewGitHubClient()}
}

func (m *KeywordReplyManager) SetReleaseClient(client updater.ReleaseClient) {
	if client == nil {
		client = updater.NewGitHubClient()
	}
	m.releaseClient = client
}

func (m *KeywordReplyManager) SetUpdateHandler(handler UpdateHandler) {
	m.updateHandler = handler
}

func (m *KeywordReplyManager) SetPluginAdminStore(store keywordPluginAdminStore) {
	m.pluginStore = store
}

func (m *KeywordReplyManager) SetListenFunc(listen keywordListenFunc) {
	m.listen = listen
}

func (m *KeywordReplyManager) SetListenUntilFunc(listen keywordListenUntilFunc) {
	m.listenUntil = listen
}

func (m *KeywordReplyManager) SetRegisterPluginFunc(register func(*types.Plugin) error) {
	m.registerPlugin = register
}

func (m *KeywordReplyManager) SetRestartHandler(handler func(RestartRequest) error) {
	m.restartMu.Lock()
	defer m.restartMu.Unlock()
	m.restartHandler = handler
	m.restartRequested = false
}

func (m *KeywordReplyManager) Handle(msg *types.Message) bool {
	if m == nil || m.database == nil || msg == nil {
		return false
	}
	if m.shouldIgnoreRestartMessage(msg) {
		return true
	}
	items, err := m.database.ListKeywordReplies()
	if err != nil {
		log.Printf("[SYSTEM] 关键字回复加载失败: %v", err)
		return false
	}
	for _, item := range items {
		if !item.Enabled || !m.match(item, msg.Content) {
			continue
		}
		if item.AdminOnly && (m.adminCheck == nil || !m.adminCheck(msg.Platform, msg.UserID)) {
			return true
		}
		if err := m.reply(item, msg); err != nil {
			log.Printf("[SYSTEM] 关键字回复失败: %v", err)
		}
		return true
	}
	return false
}

func (m *KeywordReplyManager) shouldIgnoreRestartMessage(msg *types.Message) bool {
	if strings.TrimSpace(msg.Content) != "重启" {
		return false
	}
	ignoredKey := strings.TrimSpace(os.Getenv("ALLBOT_IGNORE_RESTART_MESSAGE_KEY"))
	return ignoredKey != "" && ignoredKey == RestartMessageKey(msg)
}

func RestartMessageKey(msg *types.Message) string {
	if msg == nil {
		return ""
	}
	adapterID := msg.AdapterID
	if adapterID == "" && msg.Metadata != nil {
		adapterID = msg.Metadata["adapter_id"]
	}
	parts := []string{msg.Platform, adapterID, msg.UserID, msg.GroupID, msg.ID, msg.Content}
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	return hex.EncodeToString(digest[:])
}

func (m *KeywordReplyManager) match(item *config.KeywordReply, content string) bool {
	if item.Builtin && item.Keyword == "绑定" {
		return content == "绑定" || strings.HasPrefix(content, "绑定 ")
	}
	if item.Builtin && item.Keyword == "积分充值" {
		return content == "积分充值" || strings.HasPrefix(content, "积分充值 ")
	}
	if item.MatchType == "exact" {
		return content == item.Keyword
	}
	matched, err := regexp.MatchString(item.Keyword, content)
	return err == nil && matched
}

func (m *KeywordReplyManager) reply(item *config.KeywordReply, msg *types.Message) error {
	if item.Builtin {
		return m.replyBuiltin(item.Keyword, msg)
	}
	adp, target := m.adapterAndTarget(msg)
	if adp == nil {
		return fmt.Errorf("适配器不存在: %s", msg.Platform)
	}
	switch item.ReplyType {
	case "image":
		return adp.SendImage(target, item.Content)
	case "audio":
		return adp.SendFile(target, item.Content)
	default:
		return adp.SendMessage(target, formatReplyText(adp, msg, item.Content))
	}
}

func (m *KeywordReplyManager) replyBuiltin(keyword string, msg *types.Message) error {
	adp, target := m.adapterAndTarget(msg)
	if adp == nil {
		return fmt.Errorf("适配器不存在: %s", msg.Platform)
	}
	switch keyword {
	case "myid":
		return m.sendText(adp, target, msg, m.userIdentityInfo(msg))
	case "注册":
		return m.sendText(adp, target, msg, m.registerUser(msg))
	case "积分充值":
		return m.replyRechargePoints(adp, target, msg)
	case "绑定码":
		return m.sendText(adp, target, msg, m.createBindCode(msg))
	case "绑定":
		return m.sendText(adp, target, msg, m.bindUser(msg))
	case "groupId":
		if msg.GroupID == "" {
			return nil
		}
		return m.sendText(adp, target, msg, msg.GroupID)
	case "插件列表":
		return m.replyPluginList(adp, target, msg)
	case "system":
		return m.sendText(adp, target, msg, m.systemInfo())
	case "version":
		return m.sendText(adp, target, msg, m.versionInfo())
	case "更新":
		return m.replyUpdate(adp, target, msg)
	case "重启":
		return m.replyRestart(adp, target, msg)
	default:
		return nil
	}
}

func (m *KeywordReplyManager) replyUpdate(adp adapter.Adapter, target string, msg *types.Message) error {
	if m.adminCheck == nil || !m.adminCheck(msg.Platform, msg.UserID) {
		return m.sendText(adp, target, msg, "仅平台管理员可使用更新")
	}
	if m.updateHandler == nil {
		return m.sendText(adp, target, msg, "更新功能未初始化")
	}
	state := m.updateHandler.CurrentState()
	if state.Status == updater.UpgradeStatusDownloading || state.Status == updater.UpgradeStatusRestarting {
		return m.sendText(adp, target, msg, "更新已在执行："+state.Message)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	state, err := m.updateHandler.StartUpgrade(ctx)
	if err != nil {
		return m.sendText(adp, target, msg, err.Error())
	}
	return m.sendText(adp, target, msg, fmt.Sprintf("已开始更新到 %s，资产：%s\n%s", state.Version, state.AssetName, state.Message))
}

func (m *KeywordReplyManager) replyRestart(adp adapter.Adapter, target string, msg *types.Message) error {
	handler, alreadyRequested := m.reserveRestart()
	if handler == nil {
		return m.sendText(adp, target, msg, "重启功能未初始化")
	}
	if alreadyRequested {
		return m.sendText(adp, target, msg, "重启已在执行")
	}
	request := RestartRequest{
		MessageKey: RestartMessageKey(msg),
		Platform:   msg.Platform,
		AdapterID:  msg.AdapterID,
		UserID:     msg.UserID,
		GroupID:    msg.GroupID,
		Target:     target,
		StartedAt:  time.Now(),
	}
	if request.AdapterID == "" && msg.Metadata != nil {
		request.AdapterID = msg.Metadata["adapter_id"]
	}
	if err := m.sendText(adp, target, msg, "已收到重启指令，AllBot 正在重启"); err != nil {
		m.releaseRestart()
		return err
	}
	go func() {
		if err := handler(request); err != nil {
			m.releaseRestart()
			_ = m.sendText(adp, target, msg, err.Error())
		}
	}()
	return nil
}

const (
	pluginListPageSize      = 10
	pluginListListenTimeout = 60
)

func (m *KeywordReplyManager) replyPluginList(adp adapter.Adapter, target string, msg *types.Message) error {
	if m.adminCheck == nil || !m.adminCheck(msg.Platform, msg.UserID) {
		return m.sendText(adp, target, msg, "仅平台管理员可使用插件列表")
	}
	if m.pluginStore == nil {
		return m.sendText(adp, target, msg, "插件管理器未初始化")
	}
	if m.listen == nil {
		return m.sendText(adp, target, msg, "连续对话功能未初始化")
	}
	plugins := m.sortedPluginProcesses()
	if len(plugins) == 0 {
		return m.sendText(adp, target, msg, "暂无插件")
	}
	if err := m.sendText(adp, target, msg, formatPluginListPage(plugins, 0)); err != nil {
		return err
	}
	go m.runPluginListConversation(adp, target, msg, 0)
	return nil
}

func (m *KeywordReplyManager) runPluginListConversation(adp adapter.Adapter, target string, msg *types.Message, page int) {
	for {
		plugins := m.sortedPluginProcesses()
		if len(plugins) == 0 {
			_ = m.sendText(adp, target, msg, "暂无插件")
			return
		}
		page = clampPluginListPage(page, len(plugins))
		input := m.listenPluginList(msg)
		if input == "" {
			return
		}
		switch {
		case isQuitInput(input):
			_ = m.sendText(adp, target, msg, "已退出插件列表")
			return
		case isNextPageInput(input):
			if page >= pluginListPageCount(len(plugins))-1 {
				_ = m.sendText(adp, target, msg, "已经是最后一页\n\n"+formatPluginListPage(plugins, page))
				continue
			}
			page++
			_ = m.sendText(adp, target, msg, formatPluginListPage(plugins, page))
		case isPrevPageInput(input):
			if page <= 0 {
				_ = m.sendText(adp, target, msg, "已经是第一页\n\n"+formatPluginListPage(plugins, page))
				continue
			}
			page--
			_ = m.sendText(adp, target, msg, formatPluginListPage(plugins, page))
		default:
			plugin := pluginByPageChoice(plugins, page, input)
			if plugin == nil {
				_ = m.sendText(adp, target, msg, "请输入列表中的数字，或发送 下一页/上一页/q")
				continue
			}
			if !m.runPluginOperationConversation(adp, target, msg, plugin.Plugin.ID) {
				return
			}
			_ = m.sendText(adp, target, msg, formatPluginListPage(m.sortedPluginProcesses(), page))
		}
	}
}

func (m *KeywordReplyManager) runPluginOperationConversation(adp adapter.Adapter, target string, msg *types.Message, pluginID string) bool {
	for {
		process := m.findPluginProcess(pluginID)
		if process == nil || process.Plugin == nil {
			_ = m.sendText(adp, target, msg, "插件不存在或已卸载")
			return true
		}
		_ = m.sendText(adp, target, msg, formatPluginOperationMenu(process.Plugin))
		input := m.listenPluginList(msg)
		if input == "" {
			return false
		}
		if isQuitInput(input) {
			_ = m.sendText(adp, target, msg, "已退出插件列表")
			return false
		}
		if isBackInput(input) {
			return true
		}
		switch input {
		case "1":
			if !m.togglePluginFromConversation(adp, target, msg, pluginID) {
				return true
			}
		case "2":
			if !m.runPluginAccessControlConversation(adp, target, msg, pluginID) {
				return false
			}
		default:
			_ = m.sendText(adp, target, msg, "请输入 1 或 2，发送 b 返回插件列表，q 退出")
		}
	}
}

func (m *KeywordReplyManager) runPluginAccessControlConversation(adp adapter.Adapter, target string, msg *types.Message, pluginID string) bool {
	for {
		process := m.findPluginProcess(pluginID)
		if process == nil || process.Plugin == nil {
			_ = m.sendText(adp, target, msg, "插件不存在或已卸载")
			return true
		}
		_ = m.sendText(adp, target, msg, formatPluginAccessControlMenu(process.Plugin))
		input := m.listenPluginList(msg)
		if input == "" {
			return false
		}
		if isQuitInput(input) {
			_ = m.sendText(adp, target, msg, "已退出插件列表")
			return false
		}
		if isBackInput(input) {
			return true
		}
		field, ok := pluginAccessControlField(input)
		if !ok {
			_ = m.sendText(adp, target, msg, "请输入 1-6，发送 b 返回上一级，q 退出")
			continue
		}
		if !m.updatePluginAccessControlField(adp, target, msg, pluginID, field) {
			return false
		}
	}
}

func (m *KeywordReplyManager) updatePluginAccessControlField(adp adapter.Adapter, target string, msg *types.Message, pluginID string, field pluginAccessField) bool {
	_ = m.sendText(adp, target, msg, fmt.Sprintf("请输入要修改的%s：\n+123,+456,-789\n+ 表示添加，- 表示删除，多个用英文逗号分隔\n发送 b 返回上一级，q 退出", field.label))
	input := m.listenPluginList(msg)
	if input == "" {
		return false
	}
	if isQuitInput(input) {
		_ = m.sendText(adp, target, msg, "已退出插件列表")
		return false
	}
	if isBackInput(input) {
		return true
	}
	process := m.findPluginProcess(pluginID)
	if process == nil || process.Plugin == nil {
		_ = m.sendText(adp, target, msg, "插件不存在或已卸载")
		return true
	}
	accessControl := process.Plugin.AccessControl
	updated, err := applyPluginAccessControlOperations(field.values(accessControl), input)
	if err != nil {
		_ = m.sendText(adp, target, msg, err.Error())
		return true
	}
	field.assign(&accessControl, updated)
	accessControl = config.NormalizeAccessControlConfig(accessControl)
	if pluginAccessControlHasRules(accessControl) {
		accessControl.InheritSystem = false
	}
	if err := m.pluginStore.SavePluginAccessControl(pluginID, accessControl); err != nil {
		_ = m.sendText(adp, target, msg, "访问控制保存失败："+err.Error())
		return true
	}
	if err := m.registerManagedPlugin(pluginID); err != nil {
		_ = m.sendText(adp, target, msg, "访问控制已保存，但路由刷新失败："+err.Error())
		return true
	}
	_ = m.sendText(adp, target, msg, fmt.Sprintf("已更新【%s】%s\n当前值：%s", pluginDisplayName(process.Plugin), field.label, formatPluginAccessValues(updated)))
	return true
}

func (m *KeywordReplyManager) togglePluginFromConversation(adp adapter.Adapter, target string, msg *types.Message, pluginID string) bool {
	process := m.findPluginProcess(pluginID)
	if process == nil || process.Plugin == nil {
		_ = m.sendText(adp, target, msg, "插件不存在或已卸载")
		return false
	}
	nextEnabled := !process.Plugin.Enabled
	if err := m.pluginStore.TogglePlugin(pluginID, nextEnabled); err != nil {
		_ = m.sendText(adp, target, msg, "插件状态修改失败："+err.Error())
		return false
	}
	if err := m.registerManagedPlugin(pluginID); err != nil {
		_ = m.sendText(adp, target, msg, "插件状态已修改，但路由刷新失败："+err.Error())
		return false
	}
	status := "关闭"
	if nextEnabled {
		status = "启动"
	}
	_ = m.sendText(adp, target, msg, fmt.Sprintf("已%s【%s】", status, pluginDisplayName(process.Plugin)))
	return true
}

func (m *KeywordReplyManager) sortedPluginProcesses() []*plugincore.PluginProcess {
	if m.pluginStore == nil {
		return nil
	}
	items := m.pluginStore.GetAllPlugins()
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

func (m *KeywordReplyManager) findPluginProcess(pluginID string) *plugincore.PluginProcess {
	pluginID = strings.TrimSpace(pluginID)
	for _, item := range m.sortedPluginProcesses() {
		if item.Plugin.ID == pluginID {
			return item
		}
	}
	return nil
}

func (m *KeywordReplyManager) listenPluginList(msg *types.Message) string {
	if m.listen == nil {
		return ""
	}
	return strings.TrimSpace(m.listen(msg, pluginListListenTimeout))
}

func (m *KeywordReplyManager) registerManagedPlugin(pluginID string) error {
	if m.registerPlugin == nil {
		return nil
	}
	process := m.findPluginProcess(pluginID)
	if process == nil || process.Plugin == nil {
		return fmt.Errorf("插件不存在或已卸载")
	}
	return m.registerPlugin(process.Plugin)
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

func (m *KeywordReplyManager) reserveRestart() (func(RestartRequest) error, bool) {
	m.restartMu.Lock()
	defer m.restartMu.Unlock()
	if m.restartHandler == nil {
		return nil, false
	}
	if m.restartRequested {
		return m.restartHandler, true
	}
	m.restartRequested = true
	return m.restartHandler, false
}

func (m *KeywordReplyManager) releaseRestart() {
	m.restartMu.Lock()
	defer m.restartMu.Unlock()
	m.restartRequested = false
}

func (m *KeywordReplyManager) sendText(adp adapter.Adapter, target string, msg *types.Message, text string) error {
	return adp.SendMessage(target, formatReplyText(adp, msg, text))
}

func (m *KeywordReplyManager) adapterAndTarget(msg *types.Message) (adapter.Adapter, string) {
	if m.adapterFor == nil {
		return nil, ""
	}
	adp := m.adapterFor(msg)
	return adp, resolveReplyTarget(adp, msg)
}

func (m *KeywordReplyManager) userIdentityInfo(msg *types.Message) string {
	account, err := m.database.GetUserAccount(msg.Platform, msg.UserID)
	if err != nil {
		return userRegisterGuide()
	}
	unit := m.pointsUnit()
	return fmt.Sprintf("用户信息\n平台：%s\n用户ID：%s\nUnionID：%s\n%s：%d", account.Platform, account.UserID, account.UnionID, unit, account.Points)
}

func (m *KeywordReplyManager) registerUser(msg *types.Message) string {
	account, err := m.database.GetUserAccount(msg.Platform, msg.UserID)
	alreadyRegistered := err == nil
	if err != nil {
		if err != sql.ErrNoRows {
			return "注册失败：" + err.Error()
		}
		account, err = m.database.EnsureUserAccount(msg.Platform, msg.UserID)
		if err != nil {
			return "注册失败：" + err.Error()
		}
	}
	unit := m.pointsUnit()
	if alreadyRegistered {
		return fmt.Sprintf("已注册，无需重复注册\n平台：%s\n用户ID：%s\nUnionID：%s\n%s：%d", account.Platform, account.UserID, account.UnionID, unit, account.Points)
	}
	return fmt.Sprintf("注册成功\n平台：%s\n用户ID：%s\nUnionID：%s\n%s：%d", account.Platform, account.UserID, account.UnionID, unit, account.Points)
}

func (m *KeywordReplyManager) replyRechargePoints(adp adapter.Adapter, target string, msg *types.Message) error {
	unit := m.pointsUnit()
	args := strings.Fields(strings.TrimSpace(strings.TrimPrefix(msg.Content, "积分充值")))
	if m.adminCheck != nil && m.adminCheck(msg.Platform, msg.UserID) && len(args) == 2 {
		return m.sendText(adp, target, msg, m.rechargePointsByAdmin(args, unit))
	}
	if len(args) != 1 {
		return m.sendText(adp, target, msg, fmt.Sprintf("用法：积分充值 <金额>\n示例：积分充值 1\n示例：积分充值 9.90\n充值成功后按支付配置兑换为%s\n管理员给用户加%s：积分充值 <unionId或平台:userId> <数量>", unit, unit))
	}
	amountRaw := json.RawMessage(strconv.Quote(args[0]))
	amountCents, err := payment.ParseRMBToCents(amountRaw)
	if err != nil {
		return m.sendText(adp, target, msg, "充值失败："+err.Error())
	}
	account, err := m.database.EnsureUserAccount(msg.Platform, msg.UserID)
	if err != nil {
		return m.sendText(adp, target, msg, "充值失败："+err.Error())
	}
	settings, err := m.database.GetPaymentSettings()
	if err != nil {
		return m.sendText(adp, target, msg, "读取支付配置失败："+err.Error())
	}
	settingsValue := config.NormalizePaymentSettings(settings)
	methods := enabledRechargePaymentMethods(&settingsValue)
	if len(methods) == 0 {
		return m.sendText(adp, target, msg, "充值失败：请先在支付配置中启用第三方支付方式")
	}
	pointsAmount, err := config.CalculatePointsAmount(amountCents, settingsValue.PointsPerRMB)
	if err != nil {
		return m.sendText(adp, target, msg, "充值失败："+err.Error())
	}
	promptTitle := rechargePaymentTitle(amountCents, pointsAmount, paymentCurrencyUnitName(settingsValue), unit)
	go m.runRechargePayment(adp, target, msg, account.UnionID, amountRaw, methods, unit, promptTitle)
	return nil
}

func (m *KeywordReplyManager) rechargePointsByAdmin(args []string, unit string) string {
	amount, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil || amount <= 0 {
		return fmt.Sprintf("充值%s数量必须是大于 0 的整数", unit)
	}
	unionID, err := m.resolveRechargeTarget(args[0])
	if err != nil {
		return "充值失败：" + err.Error()
	}
	remaining, err := m.database.AddUserPoints(unionID, amount)
	if err != nil {
		return "充值失败：" + err.Error()
	}
	return fmt.Sprintf("充值成功\nUnionID：%s\n本次充值：%d%s\n当前余额：%d%s", unionID, amount, unit, remaining, unit)
}

func (m *KeywordReplyManager) runRechargePayment(adp adapter.Adapter, target string, msg *types.Message, unionID string, amountRaw json.RawMessage, methods []config.PaymentMethodSetting, unit string, promptTitle string) {
	if m.listen == nil {
		_ = m.sendText(adp, target, msg, "连续对话功能未初始化")
		return
	}
	adapterID := msg.AdapterID
	if adapterID == "" && msg.Metadata != nil {
		adapterID = msg.Metadata["adapter_id"]
	}
	methodCodes := make([]string, 0, len(methods))
	for _, method := range methods {
		methodCodes = append(methodCodes, method.Code)
	}
	service := payment.NewService(m.database)
	result, err := service.WaitPay(payment.WaitPayRequest{PluginID: "builtin:recharge_points", Platform: msg.Platform, AdapterID: adapterID, UserID: msg.UserID, GroupID: msg.GroupID, UnionID: unionID, Subject: "积分充值", AmountRaw: amountRaw, Timeout: 300, PointsUnit: unit, Methods: methodCodes, Metadata: map[string]interface{}{"source": "builtin_recharge_points"}, PromptTitle: promptTitle}, payment.Interaction{Reply: func(text string) error {
		return m.sendText(adp, target, msg, text)
	}, SendImage: func(imageURL string) error {
		if adp == nil {
			return fmt.Errorf("适配器不存在: %s", msg.Platform)
		}
		return adp.SendImage(target, imageURL)
	}, Listen: func(timeout int) string {
		return m.listen(msg, timeout)
	}, ListenUntil: func(timeout int, done <-chan struct{}) string {
		return m.listenUntilDone(msg, timeout, done)
	}})
	if err != nil {
		_ = m.sendText(adp, target, msg, "充值失败："+err.Error())
		return
	}
	if result.Status != "paid" {
		_ = m.sendText(adp, target, msg, "充值未完成："+result.Message)
		return
	}
	remaining, err := m.database.CreditPaymentPoints(result.OrderNo, "充值积分")
	if err != nil {
		_ = m.sendText(adp, target, msg, "支付成功，但积分入账失败："+err.Error())
		return
	}
	_ = m.sendText(adp, target, msg, fmt.Sprintf("充值成功\n订单号：%s\n本次充值：%d%s\n当前余额：%d%s", result.OrderNo, result.PointsAmount, unit, remaining, unit))
}

func (m *KeywordReplyManager) listenUntilDone(msg *types.Message, timeout int, done <-chan struct{}) string {
	if m.listenUntil != nil {
		return strings.TrimSpace(m.listenUntil(msg, timeout, done))
	}
	if m.listen == nil {
		return ""
	}
	ch := make(chan string, 1)
	go func() {
		ch <- m.listen(msg, timeout)
	}()
	select {
	case value := <-ch:
		return strings.TrimSpace(value)
	case <-done:
		return ""
	}
}

func enabledRechargePaymentMethods(settings *config.PaymentSettings) []config.PaymentMethodSetting {
	all := payment.EnabledMethods(settings, nil)
	methods := make([]config.PaymentMethodSetting, 0, len(all))
	for _, method := range all {
		if strings.EqualFold(strings.TrimSpace(method.Provider), "epay") {
			methods = append(methods, method)
		}
	}
	return methods
}

func rechargePaymentTitle(amountCents, pointsAmount int64, currencyUnit string, pointsUnit string) string {
	return fmt.Sprintf("当前充值 %s %s（到账 %d %s）", formatRechargeAmount(amountCents), currencyUnit, pointsAmount, pointsUnit)
}

func paymentCurrencyUnitName(settings config.PaymentSettings) string {
	unit := strings.TrimSpace(settings.CurrencyUnit)
	if unit == "" {
		return "RMB"
	}
	return unit
}

func formatRechargeAmount(amountCents int64) string {
	return fmt.Sprintf("%d.%02d", amountCents/100, amountCents%100)
}

func (m *KeywordReplyManager) resolveRechargeTarget(target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", fmt.Errorf("充值目标不能为空")
	}
	if parts := strings.SplitN(target, ":", 2); len(parts) == 2 {
		platform := strings.TrimSpace(parts[0])
		userID := strings.TrimSpace(parts[1])
		if platform == "" || userID == "" {
			return "", fmt.Errorf("平台和用户 ID 不能为空")
		}
		account, err := m.database.GetUserAccount(platform, userID)
		if err != nil {
			return "", fmt.Errorf("账号不存在，请确认用户已发送【注册】或已绑定过账号")
		}
		return account.UnionID, nil
	}
	exists, err := m.database.UserUnionExists(target)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", fmt.Errorf("账号不存在，请检查 UnionID 是否正确")
	}
	return target, nil
}

func (m *KeywordReplyManager) createBindCode(msg *types.Message) string {
	if msg.GroupID != "" {
		return "绑定码只能私聊获取，请私聊机器人发送：绑定码"
	}
	code, err := m.database.CreateUserBindCode(msg.Platform, msg.UserID)
	if err != nil {
		return "生成绑定码失败：" + err.Error()
	}
	return fmt.Sprintf("绑定码：%s\n请在其他平台私聊机器人发送：绑定 %s\n有效期：10分钟", code.Code, code.Code)
}

func (m *KeywordReplyManager) bindUser(msg *types.Message) string {
	if msg.GroupID != "" {
		return "绑定只能私聊操作，请私聊机器人发送：绑定 绑定码"
	}
	code := strings.TrimSpace(strings.TrimPrefix(msg.Content, "绑定"))
	if code == "" {
		return "请输入绑定码，例如：绑定 123456"
	}
	account, source, err := m.database.BindUserByCode(msg.Platform, msg.UserID, code)
	if err != nil {
		return "绑定失败：" + err.Error()
	}
	return fmt.Sprintf("绑定成功\n当前平台：%s\n来源平台：%s\nUnionID：%s", account.Platform, source.Platform, account.UnionID)
}

func (m *KeywordReplyManager) pointsUnit() string {
	unit, err := m.database.GetSetting("user.points_unit")
	if err != nil || strings.TrimSpace(unit) == "" {
		return "积分"
	}
	return strings.TrimSpace(unit)
}

func userRegisterGuide() string {
	return "当前用户还未注册。\n请选择：\n1. 发送「注册」自动注册当前平台账号\n2. 如需绑定其他平台，请先到已注册平台私聊发送「绑定码」，再回到当前平台私聊发送「绑定 绑定码」"
}

func (m *KeywordReplyManager) versionInfo() string {
	current := strings.TrimSpace(version.Version)
	if current == "" {
		current = "unknown"
	}
	lines := []string{
		version.DisplayVersion(),
		"",
		"版本信息：",
		"当前版本：" + current,
	}
	client := m.releaseClient
	if client == nil {
		client = updater.NewGitHubClient()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	release, err := client.LatestRelease(ctx)
	if err != nil {
		lines = append(lines, "最新版本：获取失败", "", "失败原因："+err.Error())
		return strings.Join(lines, "\n")
	}
	latest := strings.TrimSpace(release.Version)
	if latest == "" {
		latest = "未知"
	}
	lines = append(lines, "最新版本："+latest)
	compare, compareErr := updater.CompareVersion(current, latest)
	body := strings.TrimSpace(release.Body)
	if body != "" {
		lines = append(lines, "", "更新内容：", body)
	}
	if compareErr != nil {
		lines = append(lines, "", "版本比较失败："+compareErr.Error())
		return strings.Join(lines, "\n")
	}
	if compare < 0 {
		lines = append(lines, "", "发送「更新」可升级到最新版本。")
	} else {
		lines = append(lines, "", "当前已是最新版本。")
	}
	return strings.Join(lines, "\n")
}

func (m *KeywordReplyManager) systemInfo() string {
	return formatSystemInfo(systemDescription(), processorDescription(), coreThreadDescription(), formatReplyDuration(time.Since(m.startTime)), memoryInfo(), diskInfo("."), allBotMemoryUsage(), allBotDiskUsage())
}

func formatSystemInfo(systemName string, processor string, cores string, uptime string, memory string, disk string, appMemory string, appDisk string) string {
	if strings.TrimSpace(systemName) == "" {
		systemName = runtime.GOOS
	}
	if strings.TrimSpace(processor) == "" {
		processor = "未知"
	}
	if strings.TrimSpace(cores) == "" {
		cores = coreThreadDescription()
	}
	return fmt.Sprintf("系统信息\n系统：%s\n处理器：%s\n核心数：%s\n运行时间：%s\n内存信息：%s\n磁盘信息：%s\nallBot\n内存占用：%s\n磁盘占用：%s", systemName, processor, cores, uptime, memory, disk, appMemory, appDisk)
}

func systemDescription() string {
	switch runtime.GOOS {
	case "windows":
		return windowsSystemDescription()
	case "linux":
		return linuxSystemDescription()
	case "darwin":
		return darwinSystemDescription()
	default:
		return runtime.GOOS
	}
}

func processorDescription() string {
	switch runtime.GOOS {
	case "windows":
		if value := windowsRegistryValue(`HKLM\HARDWARE\DESCRIPTION\System\CentralProcessor\0`, "ProcessorNameString"); value != "" {
			return value
		}
	case "linux":
		if value := linuxCPUModel(); value != "" {
			return value
		}
	case "darwin":
		if value := commandOutput("sysctl", "-n", "machdep.cpu.brand_string"); value != "" {
			return value
		}
	}
	return runtime.GOARCH
}

func coreThreadDescription() string {
	threads := runtime.NumCPU()
	cores := physicalCoreCount()
	if cores <= 0 {
		cores = threads
	}
	return fmt.Sprintf("%d核心%d线程", cores, threads)
}

func physicalCoreCount() int {
	switch runtime.GOOS {
	case "windows":
		return windowsPhysicalCoreCount()
	case "linux":
		return linuxPhysicalCoreCount()
	case "darwin":
		return parsePositiveInt(commandOutput("sysctl", "-n", "hw.physicalcpu"))
	default:
		return 0
	}
}

func windowsPhysicalCoreCount() int {
	output := commandOutput("powershell", "-NoProfile", "-Command", "(Get-CimInstance Win32_Processor | Measure-Object -Property NumberOfCores -Sum).Sum")
	return parsePositiveInt(output)
}

func linuxPhysicalCoreCount() int {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return 0
	}
	physicalIDs := make(map[string]bool)
	currentPhysicalID := ""
	currentCoreID := ""
	cpuCores := 0
	flush := func() {
		if currentPhysicalID != "" && currentCoreID != "" {
			physicalIDs[currentPhysicalID+"/"+currentCoreID] = true
		}
		currentPhysicalID = ""
		currentCoreID = ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			flush()
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "physical id":
			currentPhysicalID = strings.TrimSpace(value)
		case "core id":
			currentCoreID = strings.TrimSpace(value)
		case "cpu cores":
			if parsed := parsePositiveInt(value); parsed > cpuCores {
				cpuCores = parsed
			}
		}
	}
	flush()
	if len(physicalIDs) > 0 {
		return len(physicalIDs)
	}
	return cpuCores
}

func parsePositiveInt(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}

func windowsSystemDescription() string {
	product := windowsRegistryValue(`HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "ProductName")
	edition := windowsRegistryValue(`HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "EditionID")
	displayVersion := windowsRegistryValue(`HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "DisplayVersion")
	build := windowsRegistryValue(`HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "CurrentBuildNumber")
	if product == "" {
		product = "Windows"
	}
	if buildNumber, err := strconv.Atoi(build); err == nil && buildNumber >= 22000 {
		product = strings.Replace(product, "Windows 10", "Windows 11", 1)
	}
	parts := []string{product}
	if edition != "" && !strings.Contains(strings.ToLower(product), strings.ToLower(edition)) {
		parts = append(parts, "("+edition+")")
	}
	if displayVersion != "" {
		parts = append(parts, displayVersion)
	}
	if build != "" {
		parts = append(parts, "Build "+build)
	}
	return strings.Join(parts, " ")
}

func linuxSystemDescription() string {
	info := parseKeyValueFile("/etc/os-release")
	name := firstNonEmpty(info["NAME"], runtime.GOOS)
	id := info["ID"]
	version := info["VERSION_ID"]
	if id == "debian" {
		if debianVersion := readTrimmedFile("/etc/debian_version"); debianVersion != "" {
			version = debianVersion
		}
	}
	if version == "" {
		version = info["VERSION"]
	}
	if id != "" && version != "" {
		return fmt.Sprintf("%s(%s) %s", name, id, version)
	}
	if version != "" {
		return name + " " + version
	}
	return name
}

func darwinSystemDescription() string {
	version := commandOutput("sw_vers", "-productVersion")
	build := commandOutput("sw_vers", "-buildVersion")
	if version == "" {
		return "macOS"
	}
	if build != "" {
		return fmt.Sprintf("macOS %s Build %s", version, build)
	}
	return "macOS " + version
}

func linuxCPUModel() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if key, value, ok := strings.Cut(line, ":"); ok && strings.TrimSpace(key) == "model name" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func windowsRegistryValue(path string, name string) string {
	output := commandOutput("cmd", "/C", "reg", "query", path, "/v", name)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && strings.EqualFold(fields[0], name) {
			return strings.Join(fields[2:], " ")
		}
	}
	return ""
}

func commandOutput(name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func parseKeyValueFile(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}
	}
	result := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"`)
		result[strings.TrimSpace(key)] = value
	}
	return result
}

func readTrimmedFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func firstNonEmpty(items ...string) string {
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			return strings.TrimSpace(item)
		}
	}
	return ""
}

func allBotMemoryUsage() string {
	var stat runtime.MemStats
	runtime.ReadMemStats(&stat)
	return formatUsageWithPercent(stat.Sys, totalMemoryBytes())
}

func allBotDiskUsage() string {
	root, err := os.Getwd()
	if err != nil {
		return "未知"
	}
	size, err := directorySize(root)
	if err != nil {
		return "未知"
	}
	total, _ := diskSpaceBytes(root)
	return formatUsageWithPercent(uint64(size), total)
}

func directorySize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	return total, err
}

func formatReplyDuration(d time.Duration) string {
	hours, minutes, seconds := int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

func formatUsageWithPercent(used uint64, total uint64) string {
	if total == 0 {
		return formatBytes(used)
	}
	return fmt.Sprintf("%s(%.2f%%)", formatBytes(used), float64(used)/float64(total)*100)
}

func formatBytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%dB", value)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	amount := float64(value)
	for _, name := range units {
		amount /= unit
		if amount < unit {
			return fmt.Sprintf("%.1f%s", amount, name)
		}
	}
	return fmt.Sprintf("%.1fPB", amount/unit)
}

func bytesToGB(value uint64) float64 {
	return float64(value) / 1024 / 1024 / 1024
}
