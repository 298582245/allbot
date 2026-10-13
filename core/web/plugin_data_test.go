package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPluginConfigSavesUserValuesToData(t *testing.T) {
	withTempWorkdir(t, func() {
		server := testServer(t)
		writePinnedTestPluginConfig(t, "demo", false)
		root := filepath.Join("plugins", "demo")
		if err := os.WriteFile(filepath.Join(root, "entry.js"), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
		payload := readPluginJSON(t, root)
		payload["user_config_schema"] = []map[string]interface{}{{"key": "token", "default": "default"}}
		payload["user_config"] = map[string]interface{}{"token": "saved", "removed": "ignore"}
		recorder := performOpenAPIJSONRequest(t, server.handlePluginConfig, http.MethodPut, "/api/plugins/config/demo", payload)
		if recorder.Code != http.StatusOK {
			t.Fatalf("save status=%d body=%s", recorder.Code, recorder.Body.String())
		}
		if _, exists := readPluginJSON(t, root)["user_config"]; exists {
			t.Fatal("saving user values must not add user_config to plugin.json")
		}
		if _, err := os.Stat(filepath.Join(root, "plugin.data")); err != nil {
			t.Fatal(err)
		}
		if got := server.pluginManager.GetPlugin("demo").Plugin.UserConfig; len(got) != 1 || got["token"] != "saved" {
			t.Fatalf("runtime config = %#v", got)
		}
		get := httptest.NewRecorder()
		server.handlePluginConfig(get, httptest.NewRequest(http.MethodGet, "/api/plugins/config/demo", nil))
		var document map[string]interface{}
		decodeUnifiedResponseData(t, get, &document)
		if got := document["user_config"].(map[string]interface{}); got["token"] != "saved" || len(got) != 1 {
			t.Fatalf("GET config = %#v", got)
		}
		list := httptest.NewRecorder()
		server.handlePlugins(list, httptest.NewRequest(http.MethodGet, "/api/plugins", nil))
		var items []map[string]interface{}
		decodeUnifiedResponseData(t, list, &items)
		if items[0]["user_config"].(map[string]interface{})["token"] != "saved" {
			t.Fatalf("list = %#v", items)
		}
	})
}
