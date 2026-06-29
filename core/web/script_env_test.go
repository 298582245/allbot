package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/allbot/allbot/core/config"
)

func TestHandleScriptEnvsCRUD(t *testing.T) {
	server := testServer(t)
	payload := map[string]interface{}{"name": "API_TOKEN", "value": "secret", "remark": "接口令牌", "enabled": true}
	recorder := performOpenAPIJSONRequest(t, server.handleScriptEnvs, http.MethodPost, "/api/script-envs", payload)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var created config.ScriptEnvVar
	decodeOpenAPIResponse(t, recorder, &created)
	if created.ID == 0 || created.Name != "API_TOKEN" || created.Value != "secret" {
		t.Fatalf("unexpected created item: %#v", created)
	}

	recorder = performOpenAPIJSONRequest(t, server.handleScriptEnvDetail, http.MethodPut, "/api/script-envs/"+jsonNumber(created.ID), map[string]interface{}{"name": "API_TOKEN", "value": "updated", "enabled": false})
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var updated config.ScriptEnvVar
	decodeOpenAPIResponse(t, recorder, &updated)
	if updated.Value != "updated" || updated.Enabled {
		t.Fatalf("unexpected updated item: %#v", updated)
	}

	recorder = performOpenAPIJSONRequest(t, server.handleScriptEnvs, http.MethodGet, "/api/script-envs", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var list struct {
		Items []config.ScriptEnvVar `json:"items"`
	}
	decodeOpenAPIResponse(t, recorder, &list)
	if len(list.Items) != 1 || list.Items[0].Value != "updated" {
		t.Fatalf("unexpected list: %#v", list)
	}

	recorder = httptestDeleteScriptEnv(t, server, created.ID)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func httptestDeleteScriptEnv(t *testing.T, server *Server, id int64) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/script-envs/"+strconv.FormatInt(id, 10), nil)
	server.handleScriptEnvDetail(recorder, request)
	return recorder
}

func jsonNumber(value int64) string {
	return strconv.FormatInt(value, 10)
}

func TestHandlePluginConfigDefaultsScriptEnv(t *testing.T) {
	withTempWorkdir(t, func() {
		server := testServer(t)
		if err := os.MkdirAll(filepath.Join("plugins", "demo"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join("plugins", "demo", "plugin.json"), []byte(`{"name":"demo","version":"1.0.0","runtime":"nodejs","entry":"index.js","platforms":[],"priority":0,"trigger":"demo","enabled":true}`), 0644); err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/plugins/config/demo", nil)
		server.handlePluginConfig(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		var response map[string]interface{}
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		config, ok := response["script_env"].(map[string]interface{})
		if !ok || config["enabled"] != false {
			t.Fatalf("script_env default missing: %#v", response["script_env"])
		}
	})
}
