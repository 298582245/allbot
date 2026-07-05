package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/types"
)

const accessCodeTestValue = "open-sesame"

func TestAccessCodeDisabledAllowsLoginPage(t *testing.T) {
	server := newAccessCodeTestServer(t, false, accessCodeTestValue)

	request := httptest.NewRequest(http.MethodGet, "/login", nil)
	response := httptest.NewRecorder()

	server.handleIndex(response, request)

	assertAccessCodeStatus(t, response, http.StatusOK)
}

func TestAccessCodeEnabledRequiresAccessCookieForLoginPage(t *testing.T) {
	server := newAccessCodeTestServer(t, true, accessCodeTestValue)

	request := httptest.NewRequest(http.MethodGet, "/login", nil)
	response := httptest.NewRecorder()

	server.handleIndex(response, request)

	assertAccessCodeStatus(t, response, http.StatusNotFound)
}

func TestAccessCodeEntrySetsPassCookie(t *testing.T) {
	server := newAccessCodeTestServer(t, true, accessCodeTestValue)

	entryRequest := httptest.NewRequest(http.MethodGet, "/login/"+accessCodeTestValue, nil)
	entryResponse := httptest.NewRecorder()

	server.handleAccessCodeEntry(entryResponse, entryRequest)

	assertAccessCodeStatus(t, entryResponse, http.StatusFound)
	if location := entryResponse.Header().Get("Location"); location != "/login" {
		t.Fatalf("Location = %q, expected /login", location)
	}
	cookie := accessCodeResponseCookie(t, entryResponse, accessPassCookieName)
	if cookie.Value != accessCodeHash(accessCodeTestValue) {
		t.Fatalf("access cookie value = %q, expected hash", cookie.Value)
	}

	loginRequest := httptest.NewRequest(http.MethodGet, "/login", nil)
	loginRequest.AddCookie(cookie)
	loginResponse := httptest.NewRecorder()

	server.handleIndex(loginResponse, loginRequest)

	assertAccessCodeStatus(t, loginResponse, http.StatusOK)
}

func TestAccessCodeEntryRejectsWrongCode(t *testing.T) {
	server := newAccessCodeTestServer(t, true, accessCodeTestValue)

	request := httptest.NewRequest(http.MethodGet, "/login/wrong-code", nil)
	response := httptest.NewRecorder()

	server.handleAccessCodeEntry(response, request)

	assertAccessCodeStatus(t, response, http.StatusNotFound)
}

func TestAccessCodeEntryAllowsLoginSlashWithPassCookie(t *testing.T) {
	server := newAccessCodeTestServer(t, true, accessCodeTestValue)
	request := httptest.NewRequest(http.MethodGet, "/login/", nil)
	request.AddCookie(&http.Cookie{Name: accessPassCookieName, Value: accessCodeHash(accessCodeTestValue), Path: "/"})
	response := httptest.NewRecorder()

	server.handleAccessCodeEntry(response, request)

	assertAccessCodeStatus(t, response, http.StatusOK)
}

func TestAccessCodeLoginRouteDoesNotRedirectToSlash(t *testing.T) {
	server := newAccessCodeTestServer(t, true, accessCodeTestValue)
	mux := http.NewServeMux()
	mux.HandleFunc("/login", server.handleIndex)
	mux.HandleFunc("/login/", server.handleAccessCodeEntry)
	mux.HandleFunc("/", server.handleIndex)

	request := httptest.NewRequest(http.MethodGet, "/login", nil)
	request.AddCookie(&http.Cookie{Name: accessPassCookieName, Value: accessCodeHash(accessCodeTestValue), Path: "/"})
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	assertAccessCodeStatus(t, response, http.StatusOK)
}

func TestAccessCodeEnabledAllowsValidSession(t *testing.T) {
	server := newAccessCodeTestServer(t, true, accessCodeTestValue)
	token, err := server.createAdminSession()
	if err != nil {
		t.Fatalf("createAdminSession returned error: %v", err)
	}
	cookie := &http.Cookie{Name: sessionCookieName, Value: token, Path: "/"}

	for _, target := range []string{"/dashboard", "/login"} {
		request := httptest.NewRequest(http.MethodGet, target, nil)
		request.AddCookie(cookie)
		response := httptest.NewRecorder()

		server.handleIndex(response, request)

		assertAccessCodeStatus(t, response, http.StatusOK)
	}
}

func TestLogoutClearsSessionCookie(t *testing.T) {
	server := newAccessCodeTestServer(t, true, accessCodeTestValue)
	token, err := server.createAdminSession()
	if err != nil {
		t.Fatalf("createAdminSession returned error: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token, Path: "/"})
	response := httptest.NewRecorder()

	server.handleLogout(response, request)

	assertAccessCodeStatus(t, response, http.StatusOK)
	cookie := accessCodeResponseCookie(t, response, sessionCookieName)
	if cookie.Value != "" || cookie.MaxAge != -1 {
		t.Fatalf("session clear cookie = value %q maxAge %d, expected empty value and maxAge -1", cookie.Value, cookie.MaxAge)
	}
	if server.validAdminToken(token) {
		t.Fatal("expected logout to invalidate allbot_session")
	}
}

func newAccessCodeTestServer(t *testing.T, enabled bool, code string) *Server {
	t.Helper()
	database, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	settings, err := database.GetSystemSettings()
	if err != nil {
		t.Fatalf("GetSystemSettings returned error: %v", err)
	}
	settings.AccessCodeEnabled = enabled
	settings.AccessCode = code
	if err := database.SaveSystemSettings(settings); err != nil {
		t.Fatalf("SaveSystemSettings returned error: %v", err)
	}
	adapterManager := config.NewAdapterManager(database)
	server := NewServer("0", nil, nil, adapterManager, fstest.MapFS{
		"index.html": {Data: []byte("access code index")},
	})
	t.Cleanup(func() {
		server.logManager.Stop()
		_ = database.Close()
	})
	return server
}

func assertAccessCodeStatus(t *testing.T, response *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if response.Code != expected {
		t.Fatalf("status = %d, expected %d", response.Code, expected)
	}
}

func accessCodeResponseCookie(t *testing.T, response *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("response missing %s cookie", name)
	return nil
}

func TestAccessCodeConfigPersistsThroughSaveSystemSettings(t *testing.T) {
	database, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer database.Close()
	settings, err := database.GetSystemSettings()
	if err != nil {
		t.Fatalf("GetSystemSettings returned error: %v", err)
	}
	settings.AccessCodeEnabled = true
	settings.AccessCode = "  persisted-code  "
	if err := database.SaveSystemSettings(settings); err != nil {
		t.Fatalf("SaveSystemSettings returned error: %v", err)
	}

	saved, err := database.GetSystemSettings()
	if err != nil {
		t.Fatalf("GetSystemSettings returned error: %v", err)
	}
	if !saved.AccessCodeEnabled {
		t.Fatal("expected access code enabled to persist")
	}
	if saved.AccessCode != "persisted-code" {
		t.Fatalf("AccessCode = %q, expected persisted-code", saved.AccessCode)
	}
}

func TestAccessCodeEnabledIgnoresExpiredSession(t *testing.T) {
	server := newAccessCodeTestServer(t, true, accessCodeTestValue)
	server.sessionMu.Lock()
	server.sessions["expired-token"] = time.Now().Add(-time.Minute)
	server.sessionMu.Unlock()
	request := httptest.NewRequest(http.MethodGet, "/login", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "expired-token", Path: "/"})
	response := httptest.NewRecorder()

	server.handleIndex(response, request)

	assertAccessCodeStatus(t, response, http.StatusNotFound)
}

func TestSettingsPutPreservesPermissionFieldsWhenMissing(t *testing.T) {
	server := newAccessCodeTestServer(t, false, "")
	database := server.adapterManager.GetDatabase()
	settings, err := database.GetSystemSettings()
	if err != nil {
		t.Fatalf("GetSystemSettings returned error: %v", err)
	}
	settings.PlatformAdmins = []config.PlatformAdmin{{UnionID: "U_qq_3264695977"}}
	settings.AccessControl = types.AccessControlConfig{WhitelistGroups: []string{"group-1"}, WhitelistUnionIDs: []string{"U_qq_3264695977"}}
	if err := database.SaveSystemSettings(settings); err != nil {
		t.Fatalf("SaveSystemSettings returned error: %v", err)
	}

	request := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(`{"admin_username":"admin","script_task_concurrent_limit":5,"access_code_enabled":true,"access_code":"new-code"}`))
	response := httptest.NewRecorder()

	server.handleSettings(response, request)

	assertAccessCodeStatus(t, response, http.StatusOK)
	saved, err := database.GetSystemSettings()
	if err != nil {
		t.Fatalf("GetSystemSettings returned error: %v", err)
	}
	if len(saved.PlatformAdmins) != 1 || saved.PlatformAdmins[0].UnionID != "U_qq_3264695977" {
		t.Fatalf("PlatformAdmins = %#v, expected original admin", saved.PlatformAdmins)
	}
	if len(saved.AccessControl.WhitelistGroups) != 1 || saved.AccessControl.WhitelistGroups[0] != "group-1" || len(saved.AccessControl.WhitelistUnionIDs) != 1 || saved.AccessControl.WhitelistUnionIDs[0] != "U_qq_3264695977" {
		t.Fatalf("AccessControl = %#v, expected original whitelist", saved.AccessControl)
	}
	if saved.ScriptTaskConcurrentLimit != 5 {
		t.Fatalf("ScriptTaskConcurrentLimit = %d, expected 5", saved.ScriptTaskConcurrentLimit)
	}
	if !saved.AccessCodeEnabled || saved.AccessCode != "new-code" {
		t.Fatalf("access code settings = enabled %v code %q, expected enabled new-code", saved.AccessCodeEnabled, saved.AccessCode)
	}
}
