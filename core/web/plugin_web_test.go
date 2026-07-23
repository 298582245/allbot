package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/allbot/allbot/core/deps"
	plugincore "github.com/allbot/allbot/core/plugin"
)

func TestHandlePluginWebPanelsListsEnabledPanels(t *testing.T) {
	withTempWorkdir(t, func() {
		server := testServer(t)
		writePluginWebTestPlugin(t, "shop", "商城", 20, true)
		writePluginWebTestPlugin(t, "tools", "工具", 10, true)
		writePluginWebTestPlugin(t, "disabled", "禁用", 1, false)
		loadPluginWebTestPlugin(t, server, "shop")
		loadPluginWebTestPlugin(t, server, "tools")
		loadPluginWebTestPlugin(t, server, "disabled")

		recorder := httptest.NewRecorder()
		server.handlePluginWebPanels(recorder, httptest.NewRequest(http.MethodGet, "/api/plugin-web/panels", nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		var panels []pluginWebPanel
		if err := json.Unmarshal(recorder.Body.Bytes(), &panels); err != nil {
			t.Fatal(err)
		}
		if len(panels) != 2 || panels[0].PluginID != "tools" || panels[1].PluginID != "shop" {
			t.Fatalf("unexpected panels: %#v", panels)
		}
		if panels[0].EntryURL != "/plugin-web/tools/index.html" || panels[0].Title != "工具" || !panels[0].Enabled {
			t.Fatalf("unexpected first panel: %#v", panels[0])
		}
	})
}

func TestHandlePluginWebStaticServesEntryAndBlocksTraversal(t *testing.T) {
	withTempWorkdir(t, func() {
		server := testServer(t)
		writePluginWebTestPlugin(t, "shop", "商城", 1, true)
		loadPluginWebTestPlugin(t, server, "shop")

		entryRecorder := httptest.NewRecorder()
		server.handlePluginWebStatic(entryRecorder, httptest.NewRequest(http.MethodGet, "/plugin-web/shop/", nil))
		if entryRecorder.Code != http.StatusOK || !strings.Contains(entryRecorder.Body.String(), "shop panel") {
			t.Fatalf("entry response = %d %s", entryRecorder.Code, entryRecorder.Body.String())
		}
		if entryRecorder.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("entry Cache-Control = %q", entryRecorder.Header().Get("Cache-Control"))
		}

		assetRecorder := httptest.NewRecorder()
		server.handlePluginWebStatic(assetRecorder, httptest.NewRequest(http.MethodGet, "/plugin-web/shop/app.js", nil))
		if assetRecorder.Code != http.StatusOK || !strings.Contains(assetRecorder.Body.String(), "plugin app") {
			t.Fatalf("asset response = %d %s", assetRecorder.Code, assetRecorder.Body.String())
		}
		if assetRecorder.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("asset Cache-Control = %q", assetRecorder.Header().Get("Cache-Control"))
		}

		headRecorder := httptest.NewRecorder()
		server.handlePluginWebStatic(headRecorder, httptest.NewRequest(http.MethodHead, "/plugin-web/shop/app.js", nil))
		if headRecorder.Code != http.StatusOK || headRecorder.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("HEAD response = %d, Cache-Control = %q", headRecorder.Code, headRecorder.Header().Get("Cache-Control"))
		}
		if headRecorder.Body.Len() != 0 {
			t.Fatalf("HEAD response body = %q", headRecorder.Body.String())
		}

		blockedRecorder := httptest.NewRecorder()
		server.handlePluginWebStatic(blockedRecorder, httptest.NewRequest(http.MethodGet, "/plugin-web/shop/../plugin.json", nil))
		if blockedRecorder.Code != http.StatusNotFound {
			t.Fatalf("expected traversal 404, got %d", blockedRecorder.Code)
		}
	})
}

func TestHandlePluginWebAPIDispatchesToPlugin(t *testing.T) {
	withTempWorkdir(t, func() {
		nodePath, err := exec.LookPath("node")
		if err != nil {
			t.Skip("node 不可用，跳过插件 Web API 执行测试")
		}
		server := testServer(t)
		configurePluginWebTestProfiles(t, server, nodePath)
		writePluginWebTestPlugin(t, "shop", "商城", 1, true)
		entry := `const { runDirect } = require(` + quotedJSString(filepath.Join(repoRootForPluginWebTest(t), "sdk", "nodejs", "allbot_direct")) + `);
runDirect(async (ctx) => {
  ctx.web.post('/orders', async (req) => req.jsonResponse({ method: req.method, path: req.path, body: await req.json() }, 202));
});
`
		if err := os.WriteFile(filepath.Join("plugins", "shop", "entry.js"), []byte(entry), 0644); err != nil {
			t.Fatal(err)
		}
		loadPluginWebTestPlugin(t, server, "shop")

		recorder := performOpenAPIJSONRequest(t, server.handlePluginWebAPI, http.MethodPost, "/api/plugin-web/shop/orders", map[string]interface{}{"sku": "A"})
		if recorder.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d: %s", recorder.Code, recorder.Body.String())
		}
		var response map[string]interface{}
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response["method"] != "POST" || response["path"] != "/orders" {
			t.Fatalf("unexpected response: %#v", response)
		}
	})
}

func TestPluginWebPathHelpersRejectUnsafePaths(t *testing.T) {
	if _, err := safePluginWebFile("/tmp/plugin", "../plugin.json"); err == nil {
		t.Fatal("expected traversal path to be rejected")
	}
	if _, _, ok := splitPluginWebPath("/plugin-web/../x", pluginWebStaticPrefix); ok {
		t.Fatal("expected unsafe plugin id to be rejected")
	}
	if got := pluginWebEntryURL("shop plugin", "index.html"); got != "/plugin-web/shop%20plugin/index.html" {
		t.Fatalf("unexpected entry URL: %s", got)
	}
}

func TestWritePluginWebResponse(t *testing.T) {
	recorder := httptest.NewRecorder()
	writePluginWebResponse(recorder, plugincore.PluginWebResponse{
		Status:  201,
		Headers: map[string]string{"Content-Type": "application/json; charset=utf-8", "X-Blocked": "no"},
		JSON:    map[string]interface{}{"ok": true},
	})
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d", recorder.Code)
	}
	if recorder.Header().Get("X-Blocked") != "" {
		t.Fatalf("unexpected blocked header: %s", recorder.Header().Get("X-Blocked"))
	}
	var payload map[string]bool
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload["ok"] {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func writePluginWebTestPlugin(t *testing.T, pluginID, title string, order int, webEnabled bool) {
	t.Helper()
	pluginDir := filepath.Join("plugins", pluginID)
	if err := os.MkdirAll(filepath.Join(pluginDir, "web"), 0755); err != nil {
		t.Fatal(err)
	}
	config := map[string]interface{}{
		"name":      pluginID,
		"version":   "1.0.0",
		"runtime":   "nodejs",
		"entry":     "entry.js",
		"platforms": []string{"qq"},
		"trigger":   "^" + pluginID + "$",
		"enabled":   true,
		"web_ui": map[string]interface{}{
			"enabled": webEnabled,
			"title":   title,
			"entry":   "web/index.html",
			"order":   order,
		},
	}
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "entry.js"), []byte("module.exports = async function() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "web", "index.html"), []byte("<html>shop panel</html>"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "web", "app.js"), []byte("console.log('plugin app')"), 0644); err != nil {
		t.Fatal(err)
	}
}

func loadPluginWebTestPlugin(t *testing.T, server *Server, pluginID string) {
	t.Helper()
	if _, err := server.pluginManager.LoadPlugin(filepath.Join("plugins", pluginID)); err != nil {
		t.Fatalf("LoadPlugin(%s) returned error: %v", pluginID, err)
	}
}

func configurePluginWebTestProfiles(t *testing.T, server *Server, nodePath string) {
	t.Helper()
	_, err := server.pluginManager.GetDepsManager().SaveRuntimeProfiles([]deps.RuntimeProfile{
		{ID: "node-default", Name: "默认 Node.js", Runtime: "nodejs", Executable: nodePath, Enabled: true, Default: true, Architecture: "win-x64"},
		{ID: "python-default", Name: "默认 Python", Runtime: "python", Executable: "python", Enabled: true, Default: true, Architecture: "win-x64"},
	})
	if err != nil {
		t.Fatal(err)
	}
	versionData, err := exec.Command(nodePath, "--version").CombinedOutput()
	version := strings.TrimSpace(string(versionData))
	if err != nil || version == "" {
		version = "test"
	}
	if err := os.MkdirAll(filepath.Join("runtime", "profiles"), 0755); err != nil {
		t.Fatal(err)
	}
	status := deps.RuntimeProfileInitResult{ProfileID: "node-default", Runtime: "nodejs", VersionOutput: version, Status: "initialized"}
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("runtime", "profiles", "node-default.status.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func repoRootForPluginWebTest(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("无法定位测试文件路径")
	}
	return filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..")))
}

func quotedJSString(value string) string {
	return strconv.Quote(value)
}
