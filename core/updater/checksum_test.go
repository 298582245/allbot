package updater

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelectChecksumAsset(t *testing.T) {
	assets := []ReleaseAsset{
		{Name: "allbot-windows-amd64.exe"},
		{Name: "checksums-v1.2.3.txt", DownloadURL: "checksum"},
	}
	asset, ok := SelectChecksumAsset(assets, "v1.2.3")
	if !ok {
		t.Fatal("expected checksum asset")
	}
	if asset.DownloadURL != "checksum" {
		t.Fatalf("asset = %#v", asset)
	}
}

func TestParseChecksumFile(t *testing.T) {
	content := "# checksums\n" + strings.Repeat("a", 64) + "  allbot-windows-amd64.exe\n"
	file, err := ParseChecksumFile(strings.NewReader(content))
	if err != nil {
		t.Fatalf("ParseChecksumFile returned error: %v", err)
	}
	sum, ok := file.ExpectedSHA256("allbot-windows-amd64.exe")
	if !ok || sum != strings.Repeat("a", 64) {
		t.Fatalf("checksum = %q, ok = %v", sum, ok)
	}
}

func TestDownloadChecksumFile(t *testing.T) {
	content := strings.Repeat("b", 64) + "  allbot-linux-amd64\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(content))
	}))
	defer server.Close()

	file, err := DownloadChecksumFile(context.Background(), ReleaseAsset{DownloadURL: server.URL})
	if err != nil {
		t.Fatalf("DownloadChecksumFile returned error: %v", err)
	}
	if sum, ok := file.ExpectedSHA256("allbot-linux-amd64"); !ok || sum != strings.Repeat("b", 64) {
		t.Fatalf("checksum = %q, ok = %v", sum, ok)
	}
}

func TestVerifyFileSHA256(t *testing.T) {
	path := filepath.Join(t.TempDir(), "allbot.exe")
	data := []byte("binary")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	sum := fmt.Sprintf("%x", sha256.Sum256(data))
	if err := VerifyFileSHA256(path, sum); err != nil {
		t.Fatalf("VerifyFileSHA256 returned error: %v", err)
	}
	if err := VerifyFileSHA256(path, strings.Repeat("0", 64)); err == nil {
		t.Fatal("expected checksum mismatch")
	}
}
