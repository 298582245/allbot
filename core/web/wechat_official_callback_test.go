package web

import (
	"crypto/sha1"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	_ "github.com/allbot/allbot/core/adapter/_loader"
	"github.com/allbot/allbot/core/config"
)

func TestWechatOfficialCallbackRejectsInvalidAdapterID(t *testing.T) {
	server := &Server{}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/open/adapters/wechat-official/not-number/callback", nil)

	server.handleWechatOfficialAdapterCallback(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestWechatOfficialCallbackReturnsNotFoundForMissingAdapter(t *testing.T) {
	server := newWechatOfficialCallbackTestServer(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/open/adapters/wechat-official/999/callback", nil)

	server.handleWechatOfficialAdapterCallback(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestWechatOfficialCallbackDelegatesToRunningAdapter(t *testing.T) {
	server := newWechatOfficialCallbackTestServer(t)
	adapterID := saveRunningWechatOfficialAdapter(t, server.adapterManager, "callback")
	query := "timestamp=123&nonce=nonce&echostr=ok&signature=" + testWechatOfficialCallbackSignature("token", "123", "nonce")
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/open/adapters/wechat-official/"+adapterID+"/callback?"+query, nil)

	server.handleWechatOfficialAdapterCallback(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "ok" {
		t.Fatalf("status = %d body = %q", response.Code, response.Body.String())
	}
}

func TestWechatOfficialCallbackOpenPathBypassesAdminAuth(t *testing.T) {
	server := newWechatOfficialCallbackTestServer(t)
	adapterID := saveRunningWechatOfficialAdapter(t, server.adapterManager, "callback")
	query := "timestamp=123&nonce=nonce&echostr=ok&signature=" + testWechatOfficialCallbackSignature("token", "123", "nonce")
	mux := http.NewServeMux()
	mux.HandleFunc("/api/open/adapters/wechat-official/", server.handleWechatOfficialAdapterCallback)
	handler := server.authMiddleware(mux)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/open/adapters/wechat-official/"+adapterID+"/callback?"+query, nil)

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "ok" {
		t.Fatalf("status = %d body = %q", response.Code, response.Body.String())
	}
}

func TestWechatOfficialCallbackRejectsUnsupportedMethod(t *testing.T) {
	server := newWechatOfficialCallbackTestServer(t)
	adapterID := saveRunningWechatOfficialAdapter(t, server.adapterManager, "callback")
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/open/adapters/wechat-official/"+adapterID+"/callback", nil)

	server.handleWechatOfficialAdapterCallback(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", response.Code)
	}
}

func newWechatOfficialCallbackTestServer(t *testing.T) *Server {
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

func saveRunningWechatOfficialAdapter(t *testing.T, manager *config.AdapterManager, callbackPath string) string {
	t.Helper()
	err := manager.SaveAdapterConfig(0, "wechat_official", "", "", true, map[string]interface{}{
		"app_id":        "app",
		"app_secret":    "secret",
		"token":         "token",
		"callback_path": callbackPath,
	})
	if err != nil {
		t.Fatalf("SaveAdapterConfig returned error: %v", err)
	}
	adapterConfig, err := manager.GetDatabase().GetAdapter("wechat_official")
	if err != nil {
		t.Fatalf("GetAdapter returned error: %v", err)
	}
	if adapterConfig == nil {
		t.Fatal("adapter config is nil")
	}
	return strconv.FormatInt(adapterConfig.ID, 10)
}

func testWechatOfficialCallbackSignature(token, timestamp, nonce string) string {
	values := []string{token, timestamp, nonce}
	sort.Strings(values)
	sum := sha1.Sum([]byte(strings.Join(values, "")))
	return hex.EncodeToString(sum[:])
}
