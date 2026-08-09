package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHandlePluginsSortsPinnedBeforeUnpinned(t *testing.T) {
	withTempWorkdir(t, func() {
		server := testServer(t)
		writePinnedTestPluginConfig(t, "alpha", false)
		writePinnedTestPluginConfig(t, "bravo", true)
		writePinnedTestPluginConfig(t, "charlie", true)

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/plugins", nil)
		server.handlePlugins(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		var plugins []map[string]interface{}
		decodeUnifiedResponseData(t, recorder, &plugins)
		ids := []string{plugins[0]["id"].(string), plugins[1]["id"].(string), plugins[2]["id"].(string)}
		if ids[0] != "bravo" || ids[1] != "charlie" || ids[2] != "alpha" {
			t.Fatalf("unexpected plugin order: %#v", ids)
		}
		if plugins[0]["pinned"] != true || plugins[2]["pinned"] != false {
			t.Fatalf("unexpected pinned fields: %#v", plugins)
		}
	})
}

func TestHandlePluginsIgnoresInternalImportDirectory(t *testing.T) {
	withTempWorkdir(t, func() {
		server := testServer(t)
		writePinnedTestPluginConfig(t, "visible", false)
		if err := os.MkdirAll(filepath.Join("plugins", ".import-staging", "import-old", "payload"), 0755); err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		server.handlePlugins(recorder, httptest.NewRequest(http.MethodGet, "/api/plugins", nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		var plugins []map[string]interface{}
		decodeUnifiedResponseData(t, recorder, &plugins)
		if len(plugins) != 1 || plugins[0]["id"] != "visible" {
			t.Fatalf("internal directory should not appear in plugin list: %#v", plugins)
		}
	})
}

func TestHandlePluginActionPinsAndUnpins(t *testing.T) {
	withTempWorkdir(t, func() {
		server := testServer(t)
		writePinnedTestPluginConfig(t, "demo", false)
		if err := os.WriteFile(filepath.Join("plugins", "demo", "entry.js"), []byte("console.log('ok')"), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := server.pluginManager.LoadPlugin(filepath.Join("plugins", "demo")); err != nil {
			t.Fatalf("LoadPlugin returned error: %v", err)
		}

		performPinnedAction(t, server, "demo", "pin")
		assertPinnedConfig(t, "demo", true)
		process := server.pluginManager.GetPlugin("demo")
		if process == nil || process.Plugin == nil || !process.Plugin.Pinned {
			t.Fatalf("plugin should be pinned in memory: %#v", process)
		}

		performPinnedAction(t, server, "demo", "unpin")
		assertPinnedConfig(t, "demo", false)
		if process.Plugin.Pinned {
			t.Fatalf("plugin should be unpinned in memory: %#v", process.Plugin)
		}
	})
}

func writePinnedTestPluginConfig(t *testing.T, pluginID string, pinned bool) {
	t.Helper()
	pluginDir := filepath.Join("plugins", pluginID)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	config := map[string]interface{}{
		"name":         pluginID,
		"version":      "1.0.0",
		"runtime":      "nodejs",
		"entry":        "entry.js",
		"platforms":    []string{"qq"},
		"priority":     0,
		"pinned":       pinned,
		"trigger":      "^" + pluginID + "$",
		"enabled":      true,
		"dependencies": map[string]string{},
	}
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func performPinnedAction(t *testing.T, server *Server, pluginID, action string) {
	t.Helper()
	body := []byte(`{"action":"` + action + `"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/plugins/"+pluginID, bytes.NewReader(body))
	server.handlePluginDetail(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for %s, got %d: %s", action, recorder.Code, recorder.Body.String())
	}
}

func assertPinnedConfig(t *testing.T, pluginID string, pinned bool) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("plugins", pluginID, "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	if config["pinned"] != pinned {
		t.Fatalf("expected pinned=%v, got %#v", pinned, config["pinned"])
	}
}
