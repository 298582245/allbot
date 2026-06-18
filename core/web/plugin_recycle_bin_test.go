package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHandlePluginRecycleBinListsBackupFiles(t *testing.T) {
	withPluginRecycleBinTempDir(t)
	if err := os.WriteFile(filepath.Join("plugins", "demo.backup.zip"), []byte("zip"), 0644); err != nil {
		t.Fatalf("write backup returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join("plugins", "demo.backup.20260618120000.zip"), []byte("zip2"), 0644); err != nil {
		t.Fatalf("write timestamp backup returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join("plugins", "normal.zip"), []byte("ignore"), 0644); err != nil {
		t.Fatalf("write ignored file returned error: %v", err)
	}
	server := NewServer("0", nil, nil, nil, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/plugins/recycle-bin", nil)
	server.handlePluginRecycleBin(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Items []pluginBackupFile `json:"items"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response returned error: %v", err)
	}
	if len(response.Items) != 2 {
		t.Fatalf("items len = %d, expected 2: %+v", len(response.Items), response.Items)
	}
	for _, item := range response.Items {
		if item.PluginID != "demo" || item.Path == "" || item.Size <= 0 {
			t.Fatalf("unexpected recycle item: %+v", item)
		}
	}
}

func TestHandlePluginRecycleBinDeletesBackupFile(t *testing.T) {
	withPluginRecycleBinTempDir(t)
	backupPath := filepath.Join("plugins", "demo.backup.zip")
	if err := os.WriteFile(backupPath, []byte("zip"), 0644); err != nil {
		t.Fatalf("write backup returned error: %v", err)
	}
	server := NewServer("0", nil, nil, nil, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/plugins/recycle-bin?name=demo.backup.zip", nil)
	server.handlePluginRecycleBin(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d %s", recorder.Code, recorder.Body.String())
	}
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Fatalf("backup still exists or stat returned unexpected error: %v", err)
	}
}

func TestHandlePluginRecycleBinRejectsInvalidName(t *testing.T) {
	withPluginRecycleBinTempDir(t)
	server := NewServer("0", nil, nil, nil, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/plugins/recycle-bin?name=../demo.backup.zip", nil)
	server.handlePluginRecycleBin(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d %s", recorder.Code, recorder.Body.String())
	}
}

func withPluginRecycleBinTempDir(t *testing.T) {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd returned error: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir returned error: %v", err)
	}
	if err := os.MkdirAll("plugins", 0755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(original); err != nil {
			t.Fatalf("restore Chdir returned error: %v", err)
		}
	})
}
