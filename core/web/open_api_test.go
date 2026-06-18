package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/allbot/allbot/core/deps"
)

func TestOpenAPIConfigSavesRuntimeProfile(t *testing.T) {
	withTempWorkdir(t, func() {
		server := testServer(t)
		configureOpenAPITestProfiles(t, server)
		payload := map[string]interface{}{
			"id": "profile_api", "name": "Profile API", "path": "profile-api", "method": "POST", "enabled": true,
			"token": "secret", "runtime": "nodejs", "runtime_profile": "node18", "entry": "main.js", "script": "module.exports.action = async function action(ctx, req, res) { res.json({ ok: true }) }\n",
		}
		recorder := performOpenAPIJSONRequest(t, server.handleOpenAPIConfigs, http.MethodPost, "/api/open-apis", payload)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		var response map[string]interface{}
		decodeOpenAPIResponse(t, recorder, &response)
		if response["runtime_profile"] != "node18" {
			t.Fatalf("runtime_profile missing from response: %#v", response)
		}
		loaded, err := loadOpenAPIEndpoint("profile_api")
		if err != nil {
			t.Fatal(err)
		}
		if loaded.RuntimeProfile != "node18" {
			t.Fatalf("unexpected saved profile: %#v", loaded)
		}
	})
}

func TestOpenAPIConfigRejectsInvalidRuntimeProfile(t *testing.T) {
	withTempWorkdir(t, func() {
		server := testServer(t)
		configureOpenAPITestProfiles(t, server)
		payload := map[string]interface{}{
			"id": "bad_profile", "name": "Bad Profile", "path": "bad-profile", "method": "POST", "enabled": true,
			"token": "secret", "runtime": "nodejs", "runtime_profile": "python310", "entry": "main.js",
		}
		recorder := performOpenAPIJSONRequest(t, server.handleOpenAPIConfigs, http.MethodPost, "/api/open-apis", payload)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
		}
	})
}

func TestOpenAPIConfigRejectsBuiltinRuntimeProfile(t *testing.T) {
	withTempWorkdir(t, func() {
		server := testServer(t)
		configureOpenAPITestProfiles(t, server)
		payload := map[string]interface{}{
			"id": "builtin_profile", "name": "Builtin Profile", "path": "builtin-profile", "method": "POST", "enabled": true,
			"token": "secret", "runtime": "builtin", "runtime_profile": "node18", "builtin": "qrcode",
		}
		recorder := performOpenAPIJSONRequest(t, server.handleOpenAPIConfigs, http.MethodPost, "/api/open-apis", payload)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
		}
	})
}

func TestOpenAPICodeUpdatesRuntimeProfile(t *testing.T) {
	withTempWorkdir(t, func() {
		server := testServer(t)
		configureOpenAPITestProfiles(t, server)
		endpointDir := filepath.Join("openapis", "code_profile")
		if err := os.MkdirAll(endpointDir, 0755); err != nil {
			t.Fatal(err)
		}
		configJSON := `{"id":"code_profile","name":"Code Profile","path":"code-profile","method":"POST","enabled":true,"token":"secret","runtime":"nodejs","entry":"main.js"}`
		if err := os.WriteFile(filepath.Join(endpointDir, "config.json"), []byte(configJSON), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(endpointDir, "main.js"), []byte("module.exports.action = async function action() {}\n"), 0644); err != nil {
			t.Fatal(err)
		}
		payload := map[string]interface{}{"code": "module.exports.action = async function action(ctx, req, res) { res.json({ ok: true }) }\n", "runtime": "nodejs", "runtime_profile": "node18", "entry": "main.js"}
		recorder := performOpenAPIJSONRequest(t, func(w http.ResponseWriter, r *http.Request) { server.handleOpenAPICode(w, r, "code_profile") }, http.MethodPut, "/api/open-apis/code_profile/code", payload)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		var response map[string]interface{}
		decodeOpenAPIResponse(t, recorder, &response)
		if response["runtime_profile"] != "node18" {
			t.Fatalf("runtime_profile missing from code response: %#v", response)
		}
		loaded, err := loadOpenAPIEndpoint("code_profile")
		if err != nil {
			t.Fatal(err)
		}
		if loaded.RuntimeProfile != "node18" {
			t.Fatalf("unexpected saved profile: %#v", loaded)
		}
	})
}

func configureOpenAPITestProfiles(t *testing.T, server *Server) {
	t.Helper()
	_, err := server.pluginManager.GetDepsManager().SaveRuntimeProfiles([]deps.RuntimeProfile{
		{ID: "node-default", Name: "默认 Node.js", Runtime: "nodejs", Executable: "node", Enabled: true, Default: true},
		{ID: "node18", Name: "Node.js 18", Runtime: "nodejs", Executable: "node", Enabled: true},
		{ID: "python-default", Name: "默认 Python", Runtime: "python", Executable: "python", Enabled: true, Default: true},
		{ID: "python310", Name: "Python 3.10", Runtime: "python", Executable: "python", Enabled: true},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func performOpenAPIJSONRequest(t *testing.T, handler func(http.ResponseWriter, *http.Request), method, path string, payload interface{}) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	handler(recorder, request)
	return recorder
}

func decodeOpenAPIResponse(t *testing.T, recorder *httptest.ResponseRecorder, target interface{}) {
	t.Helper()
	if err := json.Unmarshal(recorder.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response failed: %v, body=%s", err, recorder.Body.String())
	}
}
