package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestWebStaticDefaultsToEmbeddedAssets(t *testing.T) {
	server := NewServer("0", nil, nil, nil, fstest.MapFS{
		"index.html":    {Data: []byte("embedded index")},
		"assets/app.js": {Data: []byte("embedded asset")},
	})
	externalDir := t.TempDir()
	writeExternalWebFiles(t, externalDir, "external index", "external asset")
	server.SetExternalWebDir(externalDir)

	assertIndexResponse(t, server, "embedded index")
	assertAssetResponse(t, server, "/assets/app.js", "embedded asset", http.StatusOK)
}

func TestWebStaticUsesExternalAssetsWhenEnabled(t *testing.T) {
	server := NewServer("0", nil, nil, nil, fstest.MapFS{
		"index.html":    {Data: []byte("embedded index")},
		"assets/app.js": {Data: []byte("embedded asset")},
	})
	externalDir := t.TempDir()
	writeExternalWebFiles(t, externalDir, "external index", "external asset")
	server.SetWebAssetMode(WebAssetModeExternal)
	server.SetExternalWebDir(externalDir)

	assertIndexResponse(t, server, "external index")
	assertAssetResponse(t, server, "/assets/app.js", "external asset", http.StatusOK)
}

func TestWebStaticEmbeddedMissingResources(t *testing.T) {
	server := NewServer("0", nil, nil, nil, fstest.MapFS{})

	assertIndexResponse(t, server, "Web UI files not found")
	assertAssetResponse(t, server, "/assets/app.js", "404 page not found", http.StatusNotFound)
}

func TestWebStaticExternalMissingIndexDoesNotFallbackToEmbedded(t *testing.T) {
	server := NewServer("0", nil, nil, nil, fstest.MapFS{
		"index.html": {Data: []byte("embedded index")},
	})
	server.SetWebAssetMode(WebAssetModeExternal)
	server.SetExternalWebDir(t.TempDir())

	assertIndexResponse(t, server, "Web UI files not found")
}

func assertIndexResponse(t *testing.T, server *Server, expected string) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	server.handleIndex(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("index status = %d, expected %d", response.Code, http.StatusOK)
	}
	if body := response.Body.String(); !strings.Contains(body, expected) {
		t.Fatalf("index body %q does not contain %q", body, expected)
	}
}

func assertAssetResponse(t *testing.T, server *Server, target string, expected string, expectedStatus int) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	response := httptest.NewRecorder()

	server.webAssetsHandler().ServeHTTP(response, request)

	if response.Code != expectedStatus {
		t.Fatalf("asset status = %d, expected %d", response.Code, expectedStatus)
	}
	if body := response.Body.String(); !strings.Contains(body, expected) {
		t.Fatalf("asset body %q does not contain %q", body, expected)
	}
}

func writeExternalWebFiles(t *testing.T, dir string, indexContent string, assetContent string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(indexContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte(assetContent), 0644); err != nil {
		t.Fatal(err)
	}
}
