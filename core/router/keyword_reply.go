package router

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/allbot/allbot/core/adapter"
	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/types"
)

const FrameworkVersion = "AllBot v1.0.0"

type RestartRequest struct {
	MessageKey string
	Platform   string
	AdapterID  string
	UserID     string
	GroupID    string
	Target     string
	StartedAt  time.Time
}

type KeywordReplyManager struct {
	database         *config.Database
	adapterFor       func(msg *types.Message) adapter.Adapter
	adminCheck       func(platform, userID string) bool
	startTime        time.Time
	restartHandler   func(RestartRequest) error
	restartMu        sync.Mutex
	restartRequested bool
}

func NewKeywordReplyManager(database *config.Database, adapterFor func(msg *types.Message) adapter.Adapter, adminCheck func(platform, userID string) bool, startTime time.Time) *KeywordReplyManager {
	return &KeywordReplyManager{database: database, adapterFor: adapterFor, adminCheck: adminCheck, startTime: startTime}
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
		return m.sendText(adp, target, msg, m.rechargePoints(msg))
	case "绑定码":
		return m.sendText(adp, target, msg, m.createBindCode(msg))
	case "绑定":
		return m.sendText(adp, target, msg, m.bindUser(msg))
	case "groupId":
		if msg.GroupID == "" {
			return nil
		}
		return m.sendText(adp, target, msg, msg.GroupID)
	case "system":
		return m.sendText(adp, target, msg, m.systemInfo())
	case "version":
		return m.sendText(adp, target, msg, FrameworkVersion)
	case "重启":
		return m.replyRestart(adp, target, msg)
	default:
		return nil
	}
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

func (m *KeywordReplyManager) rechargePoints(msg *types.Message) string {
	if m.adminCheck == nil || !m.adminCheck(msg.Platform, msg.UserID) {
		return "仅平台管理员可操作积分充值"
	}
	unit := m.pointsUnit()
	args := strings.Fields(strings.TrimSpace(strings.TrimPrefix(msg.Content, "积分充值")))
	if len(args) != 2 {
		return fmt.Sprintf("用法：积分充值 <unionId或平台:userId> <数量>\n示例：积分充值 U_telegram_123456 100\n示例：积分充值 telegram:123456 100\n当前单位：%s", unit)
	}
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

func (m *KeywordReplyManager) systemInfo() string {
	mem := memoryInfo()
	disk := diskInfo(".")
	return fmt.Sprintf("系统信息\n系统：%s\n运行时间：%s\n内存信息：%s\n磁盘信息：%s\nallBot\n内存占用：%s\n磁盘占用：%s", runtime.GOOS, formatReplyDuration(time.Since(m.startTime)), mem, disk, allBotMemoryUsage(), allBotDiskUsage())
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
