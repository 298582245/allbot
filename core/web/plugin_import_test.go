package web

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	plugincore "github.com/allbot/allbot/core/plugin"
)

func makePluginMultipart(t *testing.T, sourceType, pluginID string, files map[string]string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("source_type", sourceType)
	_ = writer.WriteField("plugin_id", pluginID)
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	pathJSON, _ := json.Marshal(paths)
	_ = writer.WriteField("paths", string(pathJSON))
	for _, path := range paths {
		part, err := writer.CreateFormFile("files", filepath.Base(path))
		if err != nil {
			t.Fatal(err)
		}
		_, _ = part.Write([]byte(files[path]))
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/import", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestNormalizeImportPathRejectsTraversal(t *testing.T) {
	for _, value := range []string{"", "../main.js", "/main.js", "C:/main.js", "a\\b.js"} {
		if _, err := normalizeImportPath(value); err == nil {
			t.Fatalf("normalizeImportPath(%q) should fail", value)
		}
	}
}

func TestHandlePluginImportRejectsManifestIDMismatch(t *testing.T) {
	withTempWorkdir(t, func() {
		server := testServer(t)
		files := map[string]string{
			"demo/plugin.json": `{"id":"other","name":"Demo","runtime":"nodejs","entry":"main.js","trigger":"^demo$"}`,
			"demo/main.js":     "console.log('ok')",
		}
		recorder := httptest.NewRecorder()
		server.handlePluginImport(recorder, makePluginMultipart(t, "directory", "demo", files))
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected manifest id mismatch to fail, got %d: %s", recorder.Code, recorder.Body.String())
		}
	})
}

func TestHandlePluginImportDirectorySuccessAndConflict(t *testing.T) {
	withTempWorkdir(t, func() {
		server := testServer(t)
		files := map[string]string{
			"demo/plugin.json": `{"name":"Demo","runtime":"nodejs","entry":"main.js","trigger":"^demo$","enabled":true}`,
			"demo/main.js":     "console.log('ok')",
		}
		recorder := httptest.NewRecorder()
		server.handlePluginImport(recorder, makePluginMultipart(t, "directory", "demo", files))
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
		}
		if _, err := os.Stat(filepath.Join("plugins", "demo", "main.js")); err != nil {
			t.Fatal(err)
		}
		if server.pluginManager.GetPlugin("demo") == nil || server.router.GetPlugin("demo") == nil {
			t.Fatal("plugin should be loaded and registered")
		}

		recorder = httptest.NewRecorder()
		server.handlePluginImport(recorder, makePluginMultipart(t, "directory", "demo", files))
		if recorder.Code != http.StatusConflict {
			t.Fatalf("conflict status = %d", recorder.Code)
		}
	})
}

func TestHandlePluginImportRejectsFixedEntryAndPathsMismatch(t *testing.T) {
	withTempWorkdir(t, func() {
		server := testServer(t)
		files := map[string]string{
			"bad/plugin.json": `{"name":"Bad","runtime":"nodejs","entry":"other.js","trigger":"^bad$"}`,
			"bad/other.js":    "console.log('bad')",
		}
		recorder := httptest.NewRecorder()
		server.handlePluginImport(recorder, makePluginMultipart(t, "directory", "bad", files))
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d", recorder.Code)
		}
		if _, err := os.Stat(filepath.Join("plugins", "bad")); !os.IsNotExist(err) {
			t.Fatalf("bad plugin should not remain: %v", err)
		}

		request := makePluginMultipart(t, "directory", "mismatch", map[string]string{"plugin.json": `{}`})
		recorder = httptest.NewRecorder()
		server.handlePluginImport(recorder, request)
		if recorder.Code != http.StatusUnprocessableEntity {
			t.Fatalf("mismatch status = %d", recorder.Code)
		}
	})
}

func TestManagerValidationHasNoSideEffects(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte(`{"name":"Demo","runtime":"nodejs","entry":"main.js","trigger":"["}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.js"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	manager := plugincore.NewManager(t.TempDir(), nil)
	if _, err := manager.ValidatePluginConfig(root, "demo"); err == nil {
		t.Fatal("invalid trigger should fail")
	}
	if manager.GetPlugin("demo") != nil {
		t.Fatal("validation must not modify manager")
	}
}
