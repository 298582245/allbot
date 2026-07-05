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
	payload := map[string]interface{}{"name": "API_TOKEN", "value": "secret", "remark": "接口令牌", "enabled": true, "pinned": true}
	recorder := performOpenAPIJSONRequest(t, server.handleScriptEnvs, http.MethodPost, "/api/script-envs", payload)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var created config.ScriptEnvVar
	decodeOpenAPIResponse(t, recorder, &created)
	if created.ID == 0 || created.Name != "API_TOKEN" || created.Value != "secret" || !created.Pinned {
		t.Fatalf("unexpected created item: %#v", created)
	}
	recorder = performOpenAPIJSONRequest(t, server.handleScriptEnvs, http.MethodPost, "/api/script-envs", map[string]interface{}{"name": "API_TOKEN", "value": "second", "enabled": true})
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected duplicate name with different value to pass, got %d: %s", recorder.Code, recorder.Body.String())
	}
	recorder = performOpenAPIJSONRequest(t, server.handleScriptEnvs, http.MethodPost, "/api/script-envs", payload)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected duplicate name+value 400, got %d: %s", recorder.Code, recorder.Body.String())
	}

	recorder = performOpenAPIJSONRequest(t, server.handleScriptEnvDetail, http.MethodPut, "/api/script-envs/"+jsonNumber(created.ID), map[string]interface{}{"name": "API_TOKEN", "value": "updated", "enabled": false, "pinned": false})
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var updated config.ScriptEnvVar
	decodeOpenAPIResponse(t, recorder, &updated)
	if updated.Value != "updated" || updated.Enabled || updated.Pinned {
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
	if len(list.Items) != 2 || list.Items[1].Value != "updated" || list.Items[1].Pinned {
		t.Fatalf("unexpected list: %#v", list)
	}

	recorder = httptestDeleteScriptEnv(t, server, created.ID)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandleScriptEnvsListIDDesc(t *testing.T) {
	server := testServer(t)
	fixtures := []map[string]interface{}{
		{"name": "B_NORMAL", "value": "1", "enabled": true, "pinned": false},
		{"name": "C_PINNED", "value": "2", "enabled": true, "pinned": true},
		{"name": "A_PINNED", "value": "3", "enabled": true, "pinned": true},
	}
	for _, fixture := range fixtures {
		recorder := performOpenAPIJSONRequest(t, server.handleScriptEnvs, http.MethodPost, "/api/script-envs", fixture)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
	}

	recorder := performOpenAPIJSONRequest(t, server.handleScriptEnvs, http.MethodGet, "/api/script-envs", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var list struct {
		Items []config.ScriptEnvVar `json:"items"`
	}
	decodeOpenAPIResponse(t, recorder, &list)
	want := []string{"A_PINNED", "C_PINNED", "B_NORMAL"}
	if len(list.Items) != len(want) {
		t.Fatalf("unexpected list length: %#v", list)
	}
	for i, name := range want {
		if list.Items[i].Name != name {
			t.Fatalf("items[%d].Name = %q, want %q; list=%#v", i, list.Items[i].Name, name, list)
		}
	}
}

func TestHandleScriptEnvsBatchAndImport(t *testing.T) {
	server := testServer(t)
	created := make([]config.ScriptEnvVar, 0, 2)
	for _, payload := range []map[string]interface{}{{"name": "A", "value": "1", "enabled": true}, {"name": "B", "value": "2", "enabled": true}} {
		recorder := performOpenAPIJSONRequest(t, server.handleScriptEnvs, http.MethodPost, "/api/script-envs", payload)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		var item config.ScriptEnvVar
		decodeOpenAPIResponse(t, recorder, &item)
		created = append(created, item)
	}
	ids := []int64{created[0].ID, created[1].ID}
	for _, action := range []string{"disable", "enable", "pin", "unpin"} {
		recorder := performOpenAPIJSONRequest(t, server.handleScriptEnvs, http.MethodPatch, "/api/script-envs", map[string]interface{}{"action": action, "ids": ids})
		if recorder.Code != http.StatusOK {
			t.Fatalf("action %s expected 200, got %d: %s", action, recorder.Code, recorder.Body.String())
		}
		var response struct {
			Affected int64 `json:"affected"`
		}
		decodeOpenAPIResponse(t, recorder, &response)
		if response.Affected != 2 {
			t.Fatalf("action %s affected=%d", action, response.Affected)
		}
	}
	for _, payload := range []map[string]interface{}{{"action": "bad", "ids": ids}, {"action": "enable", "ids": []int64{}}} {
		recorder := performOpenAPIJSONRequest(t, server.handleScriptEnvs, http.MethodPatch, "/api/script-envs", payload)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for %#v, got %d: %s", payload, recorder.Code, recorder.Body.String())
		}
	}
	recorder := performOpenAPIJSONRequest(t, server.handleScriptEnvImport, http.MethodPost, "/api/script-envs/import", []map[string]interface{}{{"name": "C", "value": "3", "remark": "导入"}})
	if recorder.Code != http.StatusOK {
		t.Fatalf("import expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	for _, payload := range []interface{}{
		[]map[string]interface{}{{"name": "D", "value": "4"}, {"name": "D", "value": "4"}},
		[]map[string]interface{}{{"name": "C", "value": "3"}},
		map[string]interface{}{"name": "bad"},
	} {
		recorder := performOpenAPIJSONRequest(t, server.handleScriptEnvImport, http.MethodPost, "/api/script-envs/import", payload)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected import 400 for %#v, got %d: %s", payload, recorder.Code, recorder.Body.String())
		}
	}
	recorder = performOpenAPIJSONRequest(t, server.handleScriptEnvs, http.MethodPatch, "/api/script-envs", map[string]interface{}{"action": "delete", "ids": ids})
	if recorder.Code != http.StatusOK {
		t.Fatalf("delete expected 200, got %d: %s", recorder.Code, recorder.Body.String())
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
