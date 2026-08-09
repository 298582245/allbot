package archiveutil

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestZip(t *testing.T, entries map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plugin.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for name, data := range entries {
		item, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := item.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExtractZipFileRejectsUnsafeAndConflictingPaths(t *testing.T) {
	cases := []struct {
		name    string
		entries map[string]string
		want    string
	}{
		{"zip slip", map[string]string{"../escape.js": "x"}, "路径穿越"},
		{"absolute", map[string]string{"/absolute.js": "x"}, "绝对路径"},
		{"backslash", map[string]string{"dir\\main.js": "x"}, "反斜杠"},
		{"case conflict", map[string]string{"Main.js": "x", "main.js": "y"}, "大小写冲突"},
		{"file directory conflict", map[string]string{"main.js": "x", "main.js/child": "y"}, "文件与目录冲突"},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			zipPath := writeTestZip(t, item.entries)
			err := ExtractZipFile(zipPath, filepath.Join(t.TempDir(), "out"), ZipLimits{MaxEntries: 10, MaxFileSize: 10, MaxTotal: 20})
			if err == nil || !strings.Contains(err.Error(), item.want) {
				t.Fatalf("error = %v, want %q", err, item.want)
			}
		})
	}
}

func TestExtractZipFileEnforcesActualSizeLimits(t *testing.T) {
	zipPath := writeTestZip(t, map[string]string{"main.js": "0123456789"})
	err := ExtractZipFile(zipPath, filepath.Join(t.TempDir(), "out"), ZipLimits{MaxEntries: 10, MaxFileSize: 5, MaxTotal: 20})
	if err == nil || !strings.Contains(err.Error(), "单文件") {
		t.Fatalf("error = %v", err)
	}

	zipPath = writeTestZip(t, map[string]string{"a": "12345", "b": "67890"})
	err = ExtractZipFile(zipPath, filepath.Join(t.TempDir(), "out"), ZipLimits{MaxEntries: 10, MaxFileSize: 10, MaxTotal: 8})
	if err == nil || !strings.Contains(err.Error(), "总大小") && !strings.Contains(err.Error(), "解压后") {
		t.Fatalf("error = %v", err)
	}
}

func TestExtractZipFileSucceeds(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out")
	zipPath := writeTestZip(t, map[string]string{"plugin/plugin.json": "{}", "plugin/main.js": "console.log('ok')"})
	if err := ExtractZipFile(zipPath, out, ZipLimits{MaxEntries: 10, MaxFileSize: 1024, MaxTotal: 2048}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "plugin", "main.js")); err != nil {
		t.Fatal(err)
	}
}
