package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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

func TestSelectSignatureAssetUsesChecksumName(t *testing.T) {
	checksumAsset := ReleaseAsset{Name: "checksums-v1.2.3.txt"}
	assets := []ReleaseAsset{{Name: "checksums-v1.2.3.txt.sig", DownloadURL: "signature"}}
	signatureAsset, ok := SelectSignatureAsset(assets, checksumAsset)
	if !ok || signatureAsset.DownloadURL != "signature" {
		t.Fatalf("signature asset = %#v ok = %v", signatureAsset, ok)
	}
	if _, ok := SelectSignatureAsset(nil, checksumAsset); ok {
		t.Fatal("expected missing signature asset")
	}
}

func TestVerifyUpdateSignatureUsesRawChecksumBytes(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	checksumBytes := []byte(strings.Repeat("a", 64) + "  allbot-windows-amd64.exe\n")
	signature := []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, checksumBytes)))
	if err := verifyUpdateSignature(publicKey, signature, checksumBytes); err != nil {
		t.Fatalf("verifyUpdateSignature returned error: %v", err)
	}
	if err := verifyUpdateSignature(publicKey, signature, append([]byte(nil), checksumBytes[:len(checksumBytes)-1]...)); err == nil {
		t.Fatal("expected modified raw checksum bytes to fail verification")
	}
}

func TestVerifyUpdateSignatureRejectsInvalidFormatAndSignature(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("checksums")
	if err := verifyUpdateSignature(publicKey, []byte("not-base64"), payload); err == nil {
		t.Fatal("expected invalid signature format")
	}
	otherPublicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signature := []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, payload)))
	if err := verifyUpdateSignature(otherPublicKey, signature, payload); err == nil {
		t.Fatal("expected signature mismatch")
	}
}

func TestTrustedUpdatePublicKeyFailsClosed(t *testing.T) {
	t.Setenv(updatePublicKeyEnv, "")
	if _, err := trustedUpdatePublicKey(); err == nil {
		t.Fatal("expected missing public key to fail")
	}
	t.Setenv(updatePublicKeyEnv, base64.StdEncoding.EncodeToString([]byte("short")))
	if _, err := trustedUpdatePublicKey(); err == nil {
		t.Fatal("expected invalid public key to fail")
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
	content := strings.Repeat("b", 64) + "  allbot-linux-amd64\r\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(content))
	}))
	defer server.Close()

	raw, err := DownloadChecksumBytes(context.Background(), ReleaseAsset{DownloadURL: server.URL})
	if err != nil {
		t.Fatalf("DownloadChecksumBytes returned error: %v", err)
	}
	if string(raw) != content {
		t.Fatalf("raw checksum bytes changed: %q", raw)
	}
	file, err := ParseChecksumFile(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("ParseChecksumFile returned error: %v", err)
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
