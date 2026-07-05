package web

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/allbot/allbot/core/backup"
	"github.com/allbot/allbot/core/config"
)

func TestHandleBackupImportAndRestore(t *testing.T) {
	server, database, service := newBackupWebTestServer(t)
	defer database.Close()
	file, err := service.Create(context.Background(), "manual")
	if err != nil {
		t.Fatal(err)
	}
	backupData := mustReadWebFile(t, file.Path)

	importRecorder := httptest.NewRecorder()
	server.handleBackupDetail(importRecorder, newBackupUploadRequest(t, "/api/backups/import", "external.zip", backupData))
	if importRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", importRecorder.Code, importRecorder.Body.String())
	}
	var importResult backup.ImportResult
	if err := json.Unmarshal(importRecorder.Body.Bytes(), &importResult); err != nil {
		t.Fatal(err)
	}
	if importResult.File.Name == "" || !importResult.Summary.HasPlugins {
		t.Fatalf("导入响应不正确: %+v", importResult)
	}

	restoreRecorder := httptest.NewRecorder()
	payload := backup.RestoreOptions{IncludePlugins: true, IncludeOpenAPIs: true, Confirm: true}
	server.handleBackupDetail(restoreRecorder, newJSONRequest(t, http.MethodPost, "/api/backups/"+importResult.File.Name+"/restore", payload))
	if restoreRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", restoreRecorder.Code, restoreRecorder.Body.String())
	}
	var restoreResult backup.RestoreResult
	if err := json.Unmarshal(restoreRecorder.Body.Bytes(), &restoreResult); err != nil {
		t.Fatal(err)
	}
	if !restoreResult.RestartRequired || restoreResult.Snapshot.Name == "" {
		t.Fatalf("恢复响应不正确: %+v", restoreResult)
	}
}

func TestHandleBackupImportRejectsBadRequests(t *testing.T) {
	server, database, _ := newBackupWebTestServer(t)
	defer database.Close()
	cases := []struct {
		name    string
		request *http.Request
	}{
		{"missing file", newJSONRequest(t, http.MethodPost, "/api/backups/import", map[string]string{"bad": "bad"})},
		{"not zip", newBackupUploadRequest(t, "/api/backups/import", "bad.txt", []byte("not zip"))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			server.handleBackupDetail(recorder, tc.request)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestHandleBackupRestoreRejectsMissingConfirm(t *testing.T) {
	server, database, service := newBackupWebTestServer(t)
	defer database.Close()
	file, err := service.Create(context.Background(), "manual")
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	server.handleBackupDetail(recorder, newJSONRequest(t, http.MethodPost, "/api/backups/"+file.Name+"/restore", backup.RestoreOptions{IncludePlugins: true}))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func newBackupWebTestServer(t *testing.T) (*Server, *config.Database, *backup.Service) {
	t.Helper()
	workspace := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatal(err)
	}
	var server *Server
	t.Cleanup(func() {
		if server != nil && server.logManager != nil {
			server.logManager.Stop()
		}
		_ = os.Chdir(originalDir)
	})
	database, err := config.NewDatabase(filepath.Join(workspace, "config.db"))
	if err != nil {
		t.Fatal(err)
	}
	settings := config.DefaultBackupSettings()
	settings.BackupDir = filepath.Join(workspace, "backups")
	if err := database.SaveBackupSettings(settings); err != nil {
		t.Fatal(err)
	}
	pluginDir := filepath.Join(workspace, "plugins")
	openAPIDir := filepath.Join(workspace, "openapis")
	mustWriteWebFile(t, filepath.Join(pluginDir, "demo", "plugin.json"), `{"id":"demo"}`)
	mustWriteWebFile(t, filepath.Join(openAPIDir, "demo.json"), `{"id":"demo"}`)
	adapterManager := config.NewAdapterManager(database)
	server = NewServer("0", nil, nil, adapterManager, nil)
	service := backup.NewService(database, pluginDir)
	server.SetBackupService(service)
	return server, database, service
}

func newBackupUploadRequest(t *testing.T, path, filename string, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func mustWriteWebFile(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}

func mustReadWebFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
