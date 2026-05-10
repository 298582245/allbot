package session

import (
	"sync"
	"time"
)

// WaitingSession 等待会话
type WaitingSession struct {
	PluginID  string
	UserID    string
	GroupID   string
	Timeout   time.Time
	Channel   chan string
}

// Manager 会话管理器
type Manager struct {
	sessions map[string]*WaitingSession // key: "userID:groupID"
	mu       sync.RWMutex
}

// NewManager 创建会话管理器
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*WaitingSession),
	}
}

// CreateSession 创建等待会话
func (m *Manager) CreateSession(pluginID, userID, groupID string, timeout int) <-chan string {
	key := m.makeKey(userID, groupID)

	ch := make(chan string, 1)
	session := &WaitingSession{
		PluginID: pluginID,
		UserID:   userID,
		GroupID:  groupID,
		Timeout:  time.Now().Add(time.Duration(timeout) * time.Second),
		Channel:  ch,
	}

	m.mu.Lock()
	// 如果已有等待会话，关闭旧的
	if old, exists := m.sessions[key]; exists {
		close(old.Channel)
	}
	m.sessions[key] = session
	m.mu.Unlock()

	// 超时自动清理
	go func() {
		time.Sleep(time.Duration(timeout) * time.Second)
		m.mu.Lock()
		if s, exists := m.sessions[key]; exists && s == session {
			close(ch)
			delete(m.sessions, key)
		}
		m.mu.Unlock()
	}()

	return ch
}

// HandleMessage 处理消息，如果有等待会话则拦截
func (m *Manager) HandleMessage(userID, groupID, content string) bool {
	key := m.makeKey(userID, groupID)

	m.mu.Lock()
	session, exists := m.sessions[key]
	if exists {
		delete(m.sessions, key) // 立即删除，防止重复触发
	}
	m.mu.Unlock()

	if !exists {
		return false // 没有等待会话
	}

	// 发送消息到等待的插件
	select {
	case session.Channel <- content:
		return true // 消息已被拦截
	default:
		return false
	}
}

// makeKey 生成会话键
func (m *Manager) makeKey(userID, groupID string) string {
	if groupID == "" {
		return userID // 私聊
	}
	return userID + ":" + groupID // 群聊
}

// GetSession 获取等待会话（用于调试）
func (m *Manager) GetSession(userID, groupID string) *WaitingSession {
	key := m.makeKey(userID, groupID)
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[key]
}

// CleanExpired 清理过期会话
func (m *Manager) CleanExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for key, session := range m.sessions {
		if now.After(session.Timeout) {
			close(session.Channel)
			delete(m.sessions, key)
		}
	}
}
