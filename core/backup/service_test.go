package backup

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/allbot/allbot/core/config"
	_ "modernc.org/sqlite"
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

func TestImportBackupValidatesAndListsFile(t *testing.T) {
	workspace := t.TempDir()
	database, service := newBackupTestService(t, workspace)
	defer database.Close()
	data := buildTestBackupZip(t, map[string][]byte{
		"manifest.json":             manifestJSON(t),
		"data/config.db":            testSQLiteDB(t),
		"plugins/demo/plugin.json":  []byte(`{"id":"demo"}`),
		"openapis/demo/config.json": []byte(`{"id":"demo"}`),
	})
	service.now = func() time.Time { return time.Date(2026, 6, 30, 1, 2, 3, 0, time.Local) }
	result, err := service.Import(context.Background(), ImportOptions{Reader: bytes.NewReader(data), OriginalName: "external.zip"})
	if err != nil {
		t.Fatal(err)
	}
	if result.File.Name != "allbot-backup-import-20260630-010203.zip" || !result.Summary.HasData || !result.Summary.HasPlugins || !result.Summary.HasOpenAPIs {
		t.Fatalf("导入结果不正确: %+v", result)
	}
	files, err := service.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Name != result.File.Name {
		t.Fatalf("导入后列表不正确: %+v", files)
	}
}

func TestImportBackupRejectsInvalidZipEntries(t *testing.T) {
	workspace := t.TempDir()
	database, service := newBackupTestService(t, workspace)
	defer database.Close()
	cases := []struct {
		name    string
		entries map[string][]byte
	}{
		{"missing manifest", map[string][]byte{"data/config.db": testSQLiteDB(t)}},
		{"bad manifest", map[string][]byte{"manifest.json": []byte("bad")}},
		{"unsupported version", map[string][]byte{"manifest.json": []byte(`{"version":2}`)}},
		{"path traversal", map[string][]byte{"manifest.json": manifestJSON(t), "../evil.txt": []byte("bad")}},
		{"windows drive", map[string][]byte{"manifest.json": manifestJSON(t), "C:/evil.txt": []byte("bad")}},
		{"unknown root", map[string][]byte{"manifest.json": manifestJSON(t), "tmp/evil.txt": []byte("bad")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.Import(context.Background(), ImportOptions{Reader: bytes.NewReader(buildTestBackupZip(t, tc.entries)), OriginalName: "bad.zip"})
			if err == nil {
				t.Fatal("非法备份包应被拒绝")
			}
		})
	}
}

func TestRestoreBackupReplacesDirectoriesAndCreatesSnapshot(t *testing.T) {
	workspace := t.TempDir()
	database, service := newBackupTestService(t, workspace)
	defer database.Close()
	pluginDir := filepath.Join(workspace, "plugins")
	openAPIDir := filepath.Join(workspace, "openapis")
	mustWriteFile(t, filepath.Join(pluginDir, "old", "stale.txt"), "old")
	mustWriteFile(t, filepath.Join(openAPIDir, "old.json"), "old")
	service.now = func() time.Time { return time.Date(2026, 6, 30, 2, 0, 0, 0, time.Local) }
	backupFile, err := service.Create(context.Background(), "manual")
	if err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(pluginDir, "current", "stale.txt"), "current")
	mustWriteFile(t, filepath.Join(openAPIDir, "current.json"), "current")
	service.now = func() time.Time { return time.Date(2026, 6, 30, 3, 0, 0, 0, time.Local) }
	result, err := service.Restore(context.Background(), backupFile.Name, RestoreOptions{IncludePlugins: true, IncludeOpenAPIs: true, Confirm: true})
	if err != nil {
		t.Fatal(err)
	}
	if !result.RestartRequired || result.Snapshot.Name == "" {
		t.Fatalf("恢复结果不正确: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(pluginDir, "current", "stale.txt")); !os.IsNotExist(err) {
		t.Fatalf("插件目录应采用替换语义，实际错误: %v", err)
	}
	if string(mustReadFile(t, filepath.Join(pluginDir, "old", "stale.txt"))) != "old" {
		t.Fatal("插件旧内容未恢复")
	}
	if _, err := os.Stat(filepath.Join(openAPIDir, "current.json")); !os.IsNotExist(err) {
		t.Fatalf("OpenAPI 目录应采用替换语义，实际错误: %v", err)
	}
	entries := zipEntries(t, result.Snapshot.Path)
	if !entries["plugins/current/stale.txt"] || !entries["openapis/current.json"] {
		t.Fatal("恢复前快照应包含恢复前状态")
	}
}

func TestRestoreBackupRejectsBadOptionsAndBadDatabase(t *testing.T) {
	workspace := t.TempDir()
	database, service := newBackupTestService(t, workspace)
	defer database.Close()
	settings := config.DefaultBackupSettings()
	settings.BackupDir = filepath.Join(workspace, "backups")
	settings.IncludeData = false
	settings.IncludePlugins = true
	if err := database.SaveBackupSettings(settings); err != nil {
		t.Fatal(err)
	}
	pluginOnly, err := service.Create(context.Background(), "manual")
	if err != nil {
		t.Fatal(err)
	}
	for _, options := range []RestoreOptions{{IncludePlugins: true}, {Confirm: true}, {IncludeData: true, Confirm: true}} {
		if _, err := service.Restore(context.Background(), pluginOnly.Name, options); err == nil {
			t.Fatalf("恢复参数应被拒绝: %+v", options)
		}
	}

	badData := buildTestBackupZip(t, map[string][]byte{"manifest.json": manifestJSON(t), "data/config.db": []byte("not sqlite")})
	result, err := service.Import(context.Background(), ImportOptions{Reader: bytes.NewReader(badData), OriginalName: "bad-db.zip"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Restore(context.Background(), result.File.Name, RestoreOptions{IncludeData: true, Confirm: true}); err == nil {
		t.Fatal("非法 sqlite 数据库应拒绝恢复")
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

func newBackupTestService(t *testing.T, workspace string) (*config.Database, *Service) {
	t.Helper()
	database, err := config.NewDatabase(filepath.Join(workspace, "config.db"))
	if err != nil {
		t.Fatal(err)
	}
	settings := config.DefaultBackupSettings()
	settings.BackupDir = filepath.Join(workspace, "backups")
	settings.IncludeData = true
	settings.IncludePlugins = true
	if err := database.SaveBackupSettings(settings); err != nil {
		t.Fatal(err)
	}
	service := NewService(database, filepath.Join(workspace, "plugins"))
	service.openAPIDir = filepath.Join(workspace, "openapis")
	return database, service
}

func buildTestBackupZip(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, data := range entries {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func manifestJSON(t *testing.T) []byte {
	t.Helper()
	data, err := json.Marshal(Manifest{Version: 1, CreatedAt: time.Now(), Trigger: "test", Includes: []string{"data", "plugins"}})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func testSQLiteDB(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE system_settings (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return mustReadFile(t, path)
}

func mustWriteFile(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
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
