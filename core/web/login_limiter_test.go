package web

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestAdminLoginLimiterBlocksAfterFailureLimit(t *testing.T) {
	now := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)
	limiter := newAdminLoginLimiter()
	limiter.now = func() time.Time { return now }

	for attempt := 0; attempt < adminLoginFailureLimit; attempt++ {
		limiter.recordFailure("admin\x00127.0.0.1")
	}

	blocked, retryAfter := limiter.blocked("admin\x00127.0.0.1")
	if !blocked {
		t.Fatal("expected login attempts to be blocked")
	}
	if retryAfter != adminLoginFailureWindow {
		t.Fatalf("retryAfter = %s, expected %s", retryAfter, adminLoginFailureWindow)
	}
}

func TestAdminLoginLimiterClearsOnSuccess(t *testing.T) {
	limiter := newAdminLoginLimiter()
	key := "admin\x00127.0.0.1"
	limiter.recordFailure(key)
	limiter.clear(key)

	blocked, _ := limiter.blocked(key)
	if blocked {
		t.Fatal("expected successful login to clear failures")
	}
}

func TestAdminLoginLimiterExpiresFailures(t *testing.T) {
	now := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)
	limiter := newAdminLoginLimiter()
	limiter.now = func() time.Time { return now }
	key := "admin\x00127.0.0.1"
	for attempt := 0; attempt < adminLoginFailureLimit; attempt++ {
		limiter.recordFailure(key)
	}

	now = now.Add(adminLoginFailureWindow)
	blocked, _ := limiter.blocked(key)
	if blocked {
		t.Fatal("expected failures to expire")
	}
}

func TestAdminLoginKeyUsesRemoteHost(t *testing.T) {
	request := httptest.NewRequest("POST", "/api/login", nil)
	request.RemoteAddr = "192.0.2.10:4321"
	if got, want := adminLoginKey(request, " Admin "), "admin\x00192.0.2.10"; got != want {
		t.Fatalf("adminLoginKey = %q, expected %q", got, want)
	}
}
