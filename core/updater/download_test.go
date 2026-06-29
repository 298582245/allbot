package updater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloaderDownload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("binary"))
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "allbot-new.exe")
	err := (Downloader{}).Download(context.Background(), ReleaseAsset{DownloadURL: server.URL, Size: 6}, target)
	if err != nil {
		t.Fatalf("Download returned error: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "binary" {
		t.Fatalf("downloaded content = %q", data)
	}
}

func TestDownloaderRejectsSizeMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("binary"))
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "allbot-new.exe")
	err := (Downloader{}).Download(context.Background(), ReleaseAsset{DownloadURL: server.URL, Size: 99}, target)
	if err == nil {
		t.Fatal("expected size mismatch error")
	}
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatalf("target should not exist after failed download: %v", statErr)
	}
}
