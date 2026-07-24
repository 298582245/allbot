package updater

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAndLoadApplyUpdateRequest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrade.json")
	request := ApplyUpdateRequest{
		ParentPID:   123,
		CurrentPath: "old.exe",
		NewPath:     "new.exe",
		BackupPath:  "backup.exe",
		WorkDir:     "work",
		Args:        []string{"--plugins", "plugins"},
		FromVersion: "v1.0.0",
		ToVersion:   "v1.0.1",
	}
	if err := SaveApplyUpdateRequest(path, request); err != nil {
		t.Fatalf("SaveApplyUpdateRequest returned error: %v", err)
	}
	loaded, err := LoadApplyUpdateRequest(path)
	if err != nil {
		t.Fatalf("LoadApplyUpdateRequest returned error: %v", err)
	}
	if loaded.CurrentPath != request.CurrentPath || loaded.NewPath != request.NewPath || loaded.ToVersion != request.ToVersion || len(loaded.Args) != 2 {
		t.Fatalf("loaded request = %#v", loaded)
	}
}

func TestValidateApplyUpdateRequest(t *testing.T) {
	if err := validateApplyUpdateRequest(ApplyUpdateRequest{}); err == nil {
		t.Fatal("expected validation error")
	}
	root := t.TempDir()
	request := ApplyUpdateRequest{
		CurrentPath:     filepath.Join(t.TempDir(), "old"),
		NewPath:         filepath.Join(root, "staging", "new"),
		BackupPath:      filepath.Join(root, "backup", "old"),
		UpdateRoot:      root,
		ExpectedSHA256:  strings.Repeat("a", 64),
		AssetName:       "allbot-test",
		ChecksumPayload: base64.StdEncoding.EncodeToString([]byte(strings.Repeat("a", 64) + "  allbot-test\n")),
		UpdateSignature: "signature",
	}
	if err := validateApplyUpdateRequest(request); err != nil {
		t.Fatalf("validateApplyUpdateRequest returned error: %v", err)
	}
	request.NewPath = filepath.Join(root, "..", "outside")
	if err := validateApplyUpdateRequest(request); err == nil {
		t.Fatal("expected path outside update root to be rejected")
	}
}

func TestVerifyApplyUpdateSignatureUsesRawChecksumAndExpectedHash(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(updatePublicKeyEnv, base64.StdEncoding.EncodeToString(publicKey))
	assetName := "allbot-test"
	expectedSHA256 := strings.Repeat("a", 64)
	checksumBytes := []byte(expectedSHA256 + "  " + assetName + "\r\n")
	request := ApplyUpdateRequest{
		ExpectedSHA256:  expectedSHA256,
		AssetName:       assetName,
		ChecksumPayload: base64.StdEncoding.EncodeToString(checksumBytes),
		UpdateSignature: base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, checksumBytes)),
	}
	if err := verifyApplyUpdateSignature(request); err != nil {
		t.Fatalf("verifyApplyUpdateSignature returned error: %v", err)
	}
	request.ExpectedSHA256 = strings.Repeat("b", 64)
	if err := verifyApplyUpdateSignature(request); err == nil {
		t.Fatal("expected hash not sourced from signed checksum to be rejected")
	}
	request.ExpectedSHA256 = expectedSHA256
	request.ChecksumPayload = base64.StdEncoding.EncodeToString(append(checksumBytes, '\n'))
	if err := verifyApplyUpdateSignature(request); err == nil {
		t.Fatal("expected modified raw checksum bytes to be rejected")
	}
}

func TestValidateUpdateFileRejectsSymlinkAndReplacement(t *testing.T) {
	root := t.TempDir()
	data := []byte("verified binary")
	target := filepath.Join(root, "staging", "allbot-new")
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, data, 0700); err != nil {
		t.Fatal(err)
	}
	if err := validateUpdateFile(target, root); err != nil {
		t.Fatalf("validateUpdateFile returned error: %v", err)
	}
	expected := fmt.Sprintf("%x", sha256.Sum256(data))
	if err := VerifyFileSHA256(target, expected); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("replaced"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := VerifyFileSHA256(target, expected); err == nil {
		t.Fatal("expected replaced binary to fail helper verification")
	}

	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, data, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "staging", "linked")
	if err := os.Symlink(outside, link); err == nil {
		if err := validateUpdateFile(link, root); err == nil {
			t.Fatal("expected symlink to be rejected")
		}
	}
}

func TestSaveApplyUpdateRequestUsesExclusiveCreate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "upgrade.json")
	if err := SaveApplyUpdateRequest(path, ApplyUpdateRequest{}); err != nil {
		t.Fatal(err)
	}
	if err := SaveApplyUpdateRequest(path, ApplyUpdateRequest{}); err == nil {
		t.Fatal("expected existing request path to be rejected")
	}
}

func TestDockerUpgradeRunnerWritesRequest(t *testing.T) {
	workDir := t.TempDir()
	updateRoot := filepath.Join(workDir, "runtime", "update")
	request := ApplyUpdateRequest{
		CurrentPath:     filepath.Join(workDir, "allbot"),
		NewPath:         filepath.Join(updateRoot, "staging", "allbot-new"),
		BackupPath:      filepath.Join(updateRoot, "backup", "allbot.bak"),
		UpdateRoot:      updateRoot,
		ExpectedSHA256:  strings.Repeat("a", 64),
		AssetName:       "allbot-new",
		ChecksumPayload: base64.StdEncoding.EncodeToString([]byte(strings.Repeat("a", 64) + "  allbot-new\n")),
		UpdateSignature: "signature",
		WorkDir:         workDir,
		Args:            []string{"--plugins=/data/plugins"},
		FromVersion:     "v1.0.0",
		ToVersion:       "v1.0.1",
		RestartDelay:    "2000",
	}

	if err := DockerUpgradeRunner(request); err != nil {
		t.Fatalf("DockerUpgradeRunner returned error: %v", err)
	}

	loaded, err := LoadApplyUpdateRequest(filepath.Join(workDir, "runtime", "update", "upgrade.json"))
	if err != nil {
		t.Fatalf("LoadApplyUpdateRequest returned error: %v", err)
	}
	if loaded.NewPath != request.NewPath || loaded.ToVersion != request.ToVersion || loaded.RestartDelay != "" || loaded.RestartedFlag != "1" {
		t.Fatalf("loaded request = %#v", loaded)
	}
}

func TestDockerUpgradeRequestPathCanUseEnv(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "custom-upgrade.json")
	t.Setenv("ALLBOT_DOCKER_UPGRADE_REQUEST", custom)
	if got := DockerUpgradeRequestPath("work"); got != custom {
		t.Fatalf("DockerUpgradeRequestPath() = %q, expected %q", got, custom)
	}
}
