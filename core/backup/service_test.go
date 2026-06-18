package backup

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/allbot/allbot/core/config"
)

func TestCreateBackupIncludesDataPluginsAndOpenAPIs(t *testing.T) {
	workspace := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)

	database, err := config.NewDatabase(filepath.Join(workspace, "config.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	pluginDir := filepath.Join(workspace, "plugins")
	if err := os.MkdirAll(filepath.Join(pluginDir, "demo"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "demo", "plugin.json"), []byte(`{"id":"demo"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, "openapis", "hello"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "openapis", "hello", "config.json"), []byte(`{"id":"hello"}`), 0644); err != nil {
		t.Fatal(err)
	}

	settings := config.DefaultBackupSettings()
	settings.BackupDir = filepath.Join(workspace, "backups")
	if err := database.SaveBackupSettings(settings); err != nil {
		t.Fatal(err)
	}

	service := NewService(database, pluginDir)
	service.now = func() time.Time { return time.Date(2026, 6, 8, 3, 4, 5, 0, time.Local) }
	file, err := service.Create(context.Background(), "manual")
	if err != nil {
		t.Fatal(err)
	}

	entries := zipEntries(t, file.Path)
	for _, name := range []string{"manifest.json", "data/config.db", "plugins/demo/plugin.json", "openapis/hello/config.json"} {
		if !entries[name] {
			t.Fatalf("备份包缺少文件: %s", name)
		}
	}
}

func TestCleanupKeepsNewestBackups(t *testing.T) {
	workspace := t.TempDir()
	database, err := config.NewDatabase(filepath.Join(workspace, "config.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	backupDir := filepath.Join(workspace, "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatal(err)
	}
	settings := config.DefaultBackupSettings()
	settings.BackupDir = backupDir
	if err := database.SaveBackupSettings(settings); err != nil {
		t.Fatal(err)
	}
	oldFile := filepath.Join(backupDir, "allbot-backup-20260608-030000.zip")
	newFile := filepath.Join(backupDir, "allbot-backup-20260608-040000.zip")
	if err := os.WriteFile(oldFile, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newFile, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Date(2026, 6, 8, 3, 0, 0, 0, time.Local)
	newTime := time.Date(2026, 6, 8, 4, 0, 0, 0, time.Local)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(newFile, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	service := NewService(database, "")
	if err := service.Cleanup(1); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(newFile); err != nil {
		t.Fatalf("最新备份应保留: %v", err)
	}
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Fatalf("旧备份应删除，实际错误: %v", err)
	}
}

func zipEntries(t *testing.T, path string) map[string]bool {
	t.Helper()
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	entries := map[string]bool{}
	for _, file := range reader.File {
		entries[file.Name] = true
	}
	return entries
}
