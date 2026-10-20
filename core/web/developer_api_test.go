package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/allbot/allbot/core/config"
)

func TestDeveloperTokenPermissionUpdatesAndSecretRevocation(t *testing.T) {
	db, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := NewServer("0", nil, nil, config.NewAdapterManager(db), nil)
	app, secret, err := server.createDeveloperApp("integration", []string{"system:read"})
	if err != nil {
		t.Fatal(err)
	}
	token, err := server.issueDeveloperToken(app)
	if err != nil {
		t.Fatal(err)
	}
	if _, status, message := server.authenticateDeveloperBearer("Bearer " + token); status != 0 {
		t.Fatalf("issued token rejected: status=%d message=%s", status, message)
	}
	handler := server.developerAPIHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	call := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/system/status", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}
	if got := call().Code; got != http.StatusNoContent {
		t.Fatalf("authorized request status = %d, want %d", got, http.StatusNoContent)
	}
	if err := db.UpdateDeveloperApp(app.ID, app.Name, nil); err != nil {
		t.Fatal(err)
	}
	if got := call().Code; got != http.StatusForbidden {
		t.Fatalf("request after permission revocation status = %d, want %d", got, http.StatusForbidden)
	}
	if _, err := db.ResetDeveloperAppSecret(app.ID, hashDeveloperSecret("replacement")); err != nil {
		t.Fatal(err)
	}
	if _, status, _ := server.authenticateDeveloperBearer("Bearer " + token); status != http.StatusUnauthorized {
		t.Fatalf("token after secret reset status = %d, want %d", status, http.StatusUnauthorized)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/open-api/token", strings.NewReader(`{"app_key":"`+app.ClientID+`","app_secret":"`+secret+`"}`))
	response := httptest.NewRecorder()
	server.handleDeveloperToken(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("old secret status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestDeveloperAPIRejectsBasicAndAdminSession(t *testing.T) {
	db, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := NewServer("0", nil, nil, config.NewAdapterManager(db), nil)
	adminToken, _, err := server.createAdminSession()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		header string
		want   string
	}{
		{name: "basic", header: "Basic YXBwOnNlY3JldA==", want: "缺少授权令牌"},
		{name: "admin session", header: "Bearer " + adminToken, want: "令牌无效或已过期"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/system/status", nil)
			request.Header.Set("Authorization", test.header)
			response := httptest.NewRecorder()
			server.developerAPIHandler(http.NotFoundHandler()).ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
			}
			var body map[string]string
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body["error"] != test.want {
				t.Fatalf("error = %q, want %q", body["error"], test.want)
			}
		})
	}
}

func TestDeveloperRouteCapabilityCatalog(t *testing.T) {
	for _, test := range []struct {
		method, path, capability string
		allowed                  bool
	}{
		{http.MethodGet, "/api/plugins", "plugins:read", true},
		{http.MethodPost, "/api/plugins", "plugins:create", true},
		{http.MethodPost, "/api/plugins/import", "plugins:create", true},
		{http.MethodPost, "/api/plugin/listen", "plugins:execute", true},
		{http.MethodPost, "/api/runtime-profiles/init", "dependencies:update", true},
		{http.MethodPost, "/api/data/import", "data:update", true},
		{http.MethodPost, "/api/replies/keywords", "replies:create", true},
		{http.MethodPost, "/api/script-envs", "script_envs:update", true},
		{http.MethodPost, "/api/images", "images:update", true},
		{http.MethodGet, "/api/plugin-web/panels", "plugins:read", true},
		{http.MethodPost, "/api/backups/abc/restore", "backups:restore", true},
		{http.MethodPost, "/api/settings/password", "", false},
		{http.MethodGet, "/api/open/anything", "", false},
		{http.MethodGet, "/api/unregistered", "", false},
	} {
		capability, allowed := developerRouteCapability(test.method, test.path)
		if allowed != test.allowed || allowed && capability != test.capability {
			t.Errorf("developerRouteCapability(%q, %q) = (%q, %v), want (%q, %v)", test.method, test.path, capability, allowed, test.capability, test.allowed)
		}
	}
}

func TestDeveloperSettingsHideAndRejectSecurityFields(t *testing.T) {
	db, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	server := NewServer("0", nil, nil, config.NewAdapterManager(db), nil)
	getRequest := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	getResponse := httptest.NewRecorder()
	if !server.serveDeveloperSettings(getResponse, getRequest, "/api/settings") {
		t.Fatal("settings GET was not handled")
	}
	for _, forbidden := range []string{"admin_username", "platform_admins", "access_code", "access_code_enabled"} {
		if strings.Contains(getResponse.Body.String(), forbidden) {
			t.Errorf("developer settings response contains protected field %q", forbidden)
		}
	}
	putRequest := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(`{"access_code":"secret"}`))
	putResponse := httptest.NewRecorder()
	if !server.serveDeveloperSettings(putResponse, putRequest, "/api/settings") {
		t.Fatal("settings PUT was not handled")
	}
	if putResponse.Code != http.StatusForbidden {
		t.Fatalf("protected setting update status = %d, want %d", putResponse.Code, http.StatusForbidden)
	}
}
