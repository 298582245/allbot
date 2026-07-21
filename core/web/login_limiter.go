package web

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	adminLoginFailureLimit  = 5
	adminLoginFailureWindow = 5 * time.Minute
)

type adminLoginAttempt struct {
	windowStart time.Time
	failures    int
}

type adminLoginLimiter struct {
	mu      sync.Mutex
	entries map[string]adminLoginAttempt
	now     func() time.Time
}

func newAdminLoginLimiter() *adminLoginLimiter {
	return &adminLoginLimiter{entries: map[string]adminLoginAttempt{}, now: time.Now}
}

func (s *Server) loginLimiterForRequest() *adminLoginLimiter {
	s.loginLimiterMu.Lock()
	defer s.loginLimiterMu.Unlock()
	if s.loginLimiter == nil {
		s.loginLimiter = newAdminLoginLimiter()
	}
	return s.loginLimiter
}

func (l *adminLoginLimiter) blocked(key string) (bool, time.Duration) {
	if l == nil || strings.TrimSpace(key) == "" {
		return false, 0
	}
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cleanupLocked(now)
	entry, ok := l.entries[key]
	if !ok || entry.failures < adminLoginFailureLimit {
		return false, 0
	}
	retryAfter := entry.windowStart.Add(adminLoginFailureWindow).Sub(now)
	if retryAfter <= 0 {
		delete(l.entries, key)
		return false, 0
	}
	return true, retryAfter
}

func (l *adminLoginLimiter) recordFailure(key string) {
	if l == nil || strings.TrimSpace(key) == "" {
		return
	}
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cleanupLocked(now)
	entry, ok := l.entries[key]
	if !ok || !now.Before(entry.windowStart.Add(adminLoginFailureWindow)) {
		entry = adminLoginAttempt{windowStart: now}
	}
	entry.failures++
	l.entries[key] = entry
}

func (l *adminLoginLimiter) clear(key string) {
	if l == nil || strings.TrimSpace(key) == "" {
		return
	}
	l.mu.Lock()
	delete(l.entries, key)
	l.mu.Unlock()
}

func (l *adminLoginLimiter) cleanupLocked(now time.Time) {
	for key, entry := range l.entries {
		if !now.Before(entry.windowStart.Add(adminLoginFailureWindow)) {
			delete(l.entries, key)
		}
	}
}

func adminLoginKey(r *http.Request, username string) string {
	remote := strings.TrimSpace(r.RemoteAddr)
	if host, _, err := net.SplitHostPort(remote); err == nil {
		remote = host
	}
	if remote == "" {
		remote = "unknown"
	}
	return strings.ToLower(strings.TrimSpace(username)) + "\x00" + remote
}
