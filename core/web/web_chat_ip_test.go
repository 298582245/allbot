package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebChatClientIPIgnoresForwardedHeadersFromUntrustedPeer(t *testing.T) {
	server := &Server{}
	request := httptest.NewRequest(http.MethodGet, "/api/open/web-chat/me", nil)
	request.RemoteAddr = "198.51.100.10:4321"
	request.Header.Set("X-Forwarded-For", "203.0.113.20")
	request.Header.Set("X-Real-IP", "203.0.113.21")

	if got, want := server.webChatClientIP(request), "198.51.100.10"; got != want {
		t.Fatalf("webChatClientIP = %q, expected %q", got, want)
	}
}

func TestWebChatClientIPUsesForwardedHeadersFromTrustedProxy(t *testing.T) {
	trusted, err := compileOpenAPIIPRules([]string{"198.51.100.10/32"}, false, false)
	if err != nil {
		t.Fatalf("compileOpenAPIIPRules returned error: %v", err)
	}
	server := &Server{}
	server.openAPIAccess.Store(&openAPIAccessConfig{trustedProxies: trusted})
	request := httptest.NewRequest(http.MethodGet, "/api/open/web-chat/me", nil)
	request.RemoteAddr = "198.51.100.10:4321"
	request.Header.Set("X-Forwarded-For", "203.0.113.20, 198.51.100.10")

	if got, want := server.webChatClientIP(request), "203.0.113.20"; got != want {
		t.Fatalf("webChatClientIP = %q, expected %q", got, want)
	}
}
