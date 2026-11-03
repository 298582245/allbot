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

func TestDownloaderReportsProgressPercentages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("binary"))
	}))
	defer server.Close()

	var lastDownloaded, lastTotal int64
	target := filepath.Join(t.TempDir(), "allbot-new.exe")
	err := (Downloader{}).DownloadWithProgress(context.Background(), ReleaseAsset{DownloadURL: server.URL, Size: 6}, target, func(downloaded, total int64) {
		lastDownloaded, lastTotal = downloaded, total
	})
	if err != nil {
		t.Fatalf("DownloadWithProgress returned error: %v", err)
	}
	if lastDownloaded != 6 || lastTotal != 6 {
		t.Fatalf("progress = %d/%d, expected 6/6", lastDownloaded, lastTotal)
	}
}

func TestFormatDownloadProgress(t *testing.T) {
	if got := formatDownloadProgress(1234, 10000); got != "正在下载升级包(12.34%/100%)" {
		t.Fatalf("formatDownloadProgress = %q", got)
	}
	if got := formatDownloadProgress(12, 10); got != "正在下载升级包(100.00%/100%)" {
		t.Fatalf("formatDownloadProgress over total = %q", got)
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
