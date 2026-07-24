package session

import (
	"strings"
	"sync"
	"time"
)

// Scope 标识等待会话所属的平台、适配器、用户、会话目标和命名空间。
type Scope struct {
	Platform  string
	AdapterID string
	UserID    string
	GroupID   string
	Namespace string
}

// WaitingSession 等待会话
type WaitingSession struct {
	Scope    Scope
	PluginID string
	Timeout  time.Time
	Channel  chan string
}

// Manager 会话管理器
type Manager struct {
	sessions map[string]*WaitingSession
	mu       sync.RWMutex
}

// NewManager 创建会话管理器
func NewManager() *Manager {
	return &Manager{sessions: make(map[string]*WaitingSession)}
}

// CreateSession 创建等待会话
func (m *Manager) CreateSession(scope Scope, timeout int) <-chan string {
	ch, _ := m.CreateCancellableSession(scope, timeout)
	return ch
}

// CreateCancellableSession 创建可主动取消的等待会话
func (m *Manager) CreateCancellableSession(scope Scope, timeout int) (<-chan string, func()) {
	scope = normalizeScope(scope)
	key := makeKey(scope)
	baseKey := makeBaseKey(scope)

	ch := make(chan string, 1)
	waiting := &WaitingSession{
		Scope:    scope,
		PluginID: scope.Namespace,
		Timeout:  time.Now().Add(time.Duration(timeout) * time.Second),
		Channel:  ch,
	}

	m.mu.Lock()
	// 同一完整来源同时只保留一个等待者，防止普通消息在多个命名空间间产生歧义。
	for existingKey, existing := range m.sessions {
		if makeBaseKey(existing.Scope) == baseKey {
			close(existing.Channel)
			delete(m.sessions, existingKey)
		}
	}
	m.sessions[key] = waiting
	m.mu.Unlock()

	cancel := func() {
		m.mu.Lock()
		if existing, ok := m.sessions[key]; ok && existing == waiting {
			close(ch)
			delete(m.sessions, key)
		}
		m.mu.Unlock()
	}

	go func() {
		timer := time.NewTimer(time.Duration(timeout) * time.Second)
		defer timer.Stop()
		<-timer.C
		cancel()
	}()

	return ch, cancel
}

// HandleMessage 处理消息，如果同一来源存在等待会话则拦截。
func (m *Manager) HandleMessage(scope Scope, content string) bool {
	return m.handleMessage(scope, "", content)
}

// HandleMessageForPlugin 只允许指定插件命名空间消费等待输入。
func (m *Manager) HandleMessageForPlugin(scope Scope, pluginID, content string) bool {
	return m.handleMessage(scope, strings.TrimSpace(pluginID), content)
}

func (m *Manager) handleMessage(scope Scope, namespace, content string) bool {
	scope = normalizeScope(scope)
	baseKey := makeBaseKey(scope)

	m.mu.Lock()
	defer m.mu.Unlock()
	for key, waiting := range m.sessions {
		if makeBaseKey(waiting.Scope) != baseKey {
			continue
		}
		if namespace != "" && waiting.Scope.Namespace != namespace {
			return false
		}
		delete(m.sessions, key)
		select {
		case waiting.Channel <- content:
			close(waiting.Channel)
			return true
		default:
			close(waiting.Channel)
			return false
		}
	}
	return false
}

// GetSession 获取指定完整作用域的等待会话。
func (m *Manager) GetSession(scope Scope) *WaitingSession {
	scope = normalizeScope(scope)
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[makeKey(scope)]
}

// CleanExpired 清理过期会话
func (m *Manager) CleanExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for key, waiting := range m.sessions {
		if now.After(waiting.Timeout) {
			close(waiting.Channel)
			delete(m.sessions, key)
		}
	}
}

func normalizeScope(scope Scope) Scope {
	scope.Platform = strings.TrimSpace(scope.Platform)
	scope.AdapterID = strings.TrimSpace(scope.AdapterID)
	scope.UserID = strings.TrimSpace(scope.UserID)
	scope.GroupID = strings.TrimSpace(scope.GroupID)
	scope.Namespace = strings.TrimSpace(scope.Namespace)
	return scope
}

func makeBaseKey(scope Scope) string {
	return strings.Join([]string{scope.Platform, scope.AdapterID, scope.UserID, scope.GroupID}, "\x00")
}

func makeKey(scope Scope) string {
	return makeBaseKey(scope) + "\x00" + scope.Namespace
}
