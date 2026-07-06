package router

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/allbot/allbot/core/adapter"
	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/router/builtin"
	"github.com/allbot/allbot/core/types"
	"github.com/allbot/allbot/core/updater"
)

type RestartRequest = builtin.RestartRequest
type UpdateHandler = builtin.UpdateHandler
type keywordPluginAdminStore = builtin.PluginAdminStore
type keywordListenFunc = builtin.ListenFunc
type keywordListenUntilFunc = builtin.ListenUntilFunc

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
	updateHandler    UpdateHandler
	restartHandler   builtin.RestartHandler
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
	return builtin.RestartMessageKey(msg)
}

func (m *KeywordReplyManager) match(item *config.KeywordReply, content string) bool {
	return builtin.Match(item, content)
}

func (m *KeywordReplyManager) reply(item *config.KeywordReply, msg *types.Message) error {
	if item.Builtin {
		keyword := strings.TrimSpace(item.Content)
		if keyword == "" {
			keyword = item.Keyword
		}
		return m.replyBuiltin(keyword, msg)
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
	ctx := &builtin.Context{
		Database:       m.database,
		Message:        msg,
		Target:         target,
		StartTime:      m.startTime,
		ReleaseClient:  m.releaseClient,
		UpdateHandler:  m.updateHandler,
		PluginStore:    m.pluginStore,
		RegisterPlugin: m.registerPlugin,
		Listen:         m.listen,
		ListenUntil:    m.listenUntil,
		AdminCheck:     m.adminCheck,
		Reply: func(text string) error {
			return m.sendText(adp, target, msg, text)
		},
		ReplyButtons: func(text string, buttons [][]types.ButtonOption) error {
			return sendReplyButtonsWithFallback(adp, msg, target, text, buttons)
		},
		SendImage: func(imageURL string) error {
			return adp.SendImage(target, imageURL)
		},
		SendRich: func(message types.RichMessage) error {
			return sendReplyRichWithFallback(adp, msg, target, message)
		},
		ReserveRestart: m.reserveRestart,
		ReleaseRestart: m.releaseRestart,
		MessageKey:     RestartMessageKey,
	}
	return builtin.Dispatch(ctx, keyword)
}

func (m *KeywordReplyManager) reserveRestart() (builtin.RestartHandler, bool) {
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

func (m *KeywordReplyManager) systemInfo() string {
	return builtin.SystemInfo(m.startTime)
}

func formatSystemInfo(systemName string, processor string, cores string, uptime string, memory string, disk string, appMemory string, appDisk string) string {
	return builtin.FormatSystemInfo(systemName, "测试架构", processor, cores, uptime, memory, disk, appMemory, appDisk)
}

func userRegisterGuide() string {
	return builtin.UserRegisterGuide()
}

func bytesToGB(value uint64) float64 {
	return float64(value) / 1024 / 1024 / 1024
}
