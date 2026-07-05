package web

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	_ "github.com/allbot/allbot/core/adapter/_loader"
	"github.com/allbot/allbot/core/config"
)

func TestFeishuCallbackParsePath(t *testing.T) {
	adapterID, relativePath, ok := parseFeishuCallbackPath("/api/open/adapters/feishu/12/callback/path")
	if !ok || adapterID != 12 || relativePath != "callback/path" {
		t.Fatalf("adapterID=%d relativePath=%q ok=%v", adapterID, relativePath, ok)
	}
	for _, path := range []string{
		"/api/open/adapters/feishu/",
		"/api/open/adapters/feishu/not-number/callback",
		"/api/open/adapters/feishu/0/callback",
		"/api/open/adapters/feishu/12/",
	} {
		if _, _, ok := parseFeishuCallbackPath(path); ok {
			t.Fatalf("expected parse failure for %s", path)
		}
	}
}

func TestFeishuCallbackRejectsInvalidAdapterID(t *testing.T) {
	server := &Server{}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/open/adapters/feishu/not-number/callback", nil)

	server.handleFeishuAdapterCallback(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestFeishuCallbackReturnsNotFoundForMissingAdapter(t *testing.T) {
	server := newFeishuCallbackTestServer(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/open/adapters/feishu/999/callback", strings.NewReader(`{}`))

	server.handleFeishuAdapterCallback(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestFeishuCallbackReturnsNotFoundForPlatformMismatch(t *testing.T) {
	server := newFeishuCallbackTestServer(t)
	adapterID := saveRunningWechatOfficialAdapter(t, server.adapterManager, "callback")
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/open/adapters/feishu/"+adapterID+"/callback", strings.NewReader(`{}`))

	server.handleFeishuAdapterCallback(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestFeishuCallbackDelegatesToRunningAdapter(t *testing.T) {
	server := newFeishuCallbackTestServer(t)
	adapterID := saveRunningFeishuAdapter(t, server.adapterManager, "callback")
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/open/adapters/feishu/"+adapterID+"/callback", strings.NewReader(`{"type":"url_verification","token":"verify-token","challenge":"ok"}`))

	server.handleFeishuAdapterCallback(response, request)

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "ok") {
		t.Fatalf("status = %d body = %q", response.Code, response.Body.String())
	}
}

func TestFeishuCallbackRejectsUnsupportedMethod(t *testing.T) {
	server := newFeishuCallbackTestServer(t)
	adapterID := saveRunningFeishuAdapter(t, server.adapterManager, "callback")
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/open/adapters/feishu/"+adapterID+"/callback", nil)

	server.handleFeishuAdapterCallback(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", response.Code)
	}
}

func newFeishuCallbackTestServer(t *testing.T) *Server {
	t.Helper()
	database, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	adapterManager := config.NewAdapterManager(database)
	server := NewServer("0", nil, nil, adapterManager, fstest.MapFS{"index.html": {Data: []byte("index")}})
	t.Cleanup(func() {
		server.logManager.Stop()
		adapterManager.StopAll()
		_ = database.Close()
	})
	return server
}

func saveRunningFeishuAdapter(t *testing.T, manager *config.AdapterManager, callbackPath string) string {
	t.Helper()
	err := manager.SaveAdapterConfig(0, "feishu", "", "", true, map[string]interface{}{
		"app_id":             "app",
		"app_secret":         "secret",
		"verification_token": "verify-token",
		"callback_path":      callbackPath,
	})
	if err != nil {
		t.Fatalf("SaveAdapterConfig returned error: %v", err)
	}
	adapterConfig, err := manager.GetDatabase().GetAdapter("feishu")
	if err != nil {
		t.Fatalf("GetAdapter returned error: %v", err)
	}
	if adapterConfig == nil {
		t.Fatal("adapter config is nil")
	}
	return strconv.FormatInt(adapterConfig.ID, 10)
}
