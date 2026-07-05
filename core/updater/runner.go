package updater

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func DefaultUpgradeRunner(request ApplyUpdateRequest) error {
	if DockerUpdateModeEnabled() {
		return DockerUpgradeRunner(request)
	}
	currentExe, err := os.Executable()
	if err != nil {
		return err
	}
	runnerPath := filepath.Join(filepath.Dir(request.NewPath), updaterRunnerName())
	if err := copyFile(currentExe, runnerPath, 0755); err != nil {
		return fmt.Errorf("准备更新器失败: %w", err)
	}
	requestPath := filepath.Join(filepath.Dir(request.NewPath), "upgrade.json")
	if err := SaveApplyUpdateRequest(requestPath, request); err != nil {
		return err
	}
	cmd := exec.Command(runnerPath, "--apply-update", requestPath)
	cmd.Dir = request.WorkDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}

func DockerUpdateModeEnabled() bool {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("ALLBOT_UPDATE_MODE")))
	return mode == "docker"
}

func DockerUpgradeRunner(request ApplyUpdateRequest) error {
	request.RestartDelay = ""
	request.RestartedFlag = "1"
	return SaveApplyUpdateRequest(DockerUpgradeRequestPath(request.WorkDir), request)
}

func DockerUpgradeRequestPath(workDir string) string {
	base := strings.TrimSpace(os.Getenv("ALLBOT_DOCKER_UPGRADE_REQUEST"))
	if base != "" {
		return base
	}
	if strings.TrimSpace(workDir) == "" {
		workDir = "."
	}
	return filepath.Join(workDir, "runtime", "update", "upgrade.json")
}

func updaterRunnerName() string {
	if runtime.GOOS == "windows" {
		return "allbot-updater.exe"
	}
	return "allbot-updater"
}

func copyFile(source string, target string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
