package updater

import (
	"path/filepath"
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
	request := ApplyUpdateRequest{CurrentPath: "old", NewPath: "new", BackupPath: "backup"}
	if err := validateApplyUpdateRequest(request); err != nil {
		t.Fatalf("validateApplyUpdateRequest returned error: %v", err)
	}
}

func TestDockerUpgradeRunnerWritesRequest(t *testing.T) {
	workDir := t.TempDir()
	request := ApplyUpdateRequest{
		CurrentPath:  filepath.Join(workDir, "allbot"),
		NewPath:      filepath.Join(workDir, "runtime", "update", "allbot-new"),
		BackupPath:   filepath.Join(workDir, "runtime", "update", "backup", "allbot.bak"),
		WorkDir:      workDir,
		Args:         []string{"--plugins=/data/plugins"},
		FromVersion:  "v1.0.0",
		ToVersion:    "v1.0.1",
		RestartDelay: "2000",
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
