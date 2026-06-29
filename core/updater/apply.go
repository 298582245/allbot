package updater

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type ApplyUpdateRequest struct {
	ParentPID     int      `json:"parentPid"`
	CurrentPath   string   `json:"currentPath"`
	NewPath       string   `json:"newPath"`
	BackupPath    string   `json:"backupPath"`
	WorkDir       string   `json:"workDir"`
	Args          []string `json:"args"`
	FromVersion   string   `json:"fromVersion"`
	ToVersion     string   `json:"toVersion"`
	RestartDelay  string   `json:"restartDelay"`
	RestartedFlag string   `json:"restartedFlag"`
}

func SaveApplyUpdateRequest(path string, request ApplyUpdateRequest) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("更新请求路径不能为空")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func LoadApplyUpdateRequest(path string) (ApplyUpdateRequest, error) {
	var request ApplyUpdateRequest
	data, err := os.ReadFile(path)
	if err != nil {
		return request, err
	}
	if err := json.Unmarshal(data, &request); err != nil {
		return request, err
	}
	return request, nil
}

func ApplyUpdate(path string) error {
	request, err := LoadApplyUpdateRequest(path)
	if err != nil {
		return err
	}
	if err := validateApplyUpdateRequest(request); err != nil {
		return err
	}
	waitForParentExit(request.ParentPID, 2*time.Second)
	if err := os.MkdirAll(filepath.Dir(request.BackupPath), 0755); err != nil {
		return err
	}
	_ = os.Remove(request.BackupPath)
	if err := renameWithRetry(request.CurrentPath, request.BackupPath, 30*time.Second); err != nil {
		return fmt.Errorf("备份旧程序失败: %w", err)
	}
	if err := renameWithRetry(request.NewPath, request.CurrentPath, 30*time.Second); err != nil {
		_ = os.Rename(request.BackupPath, request.CurrentPath)
		return fmt.Errorf("替换新程序失败: %w", err)
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(request.CurrentPath, 0755)
	}
	return startUpdatedProcess(request)
}

func validateApplyUpdateRequest(request ApplyUpdateRequest) error {
	if strings.TrimSpace(request.CurrentPath) == "" {
		return fmt.Errorf("当前程序路径不能为空")
	}
	if strings.TrimSpace(request.NewPath) == "" {
		return fmt.Errorf("新程序路径不能为空")
	}
	if strings.TrimSpace(request.BackupPath) == "" {
		return fmt.Errorf("备份路径不能为空")
	}
	return nil
}

func waitForParentExit(pid int, timeout time.Duration) {
	if pid <= 0 {
		return
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		process, err := os.FindProcess(pid)
		if err != nil || process == nil {
			return
		}
		if runtime.GOOS != "windows" {
			if err := process.Signal(os.Signal(nil)); err != nil {
				return
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func renameWithRetry(oldPath string, newPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		if err := os.Rename(oldPath, newPath); err != nil {
			lastErr = err
			if time.Now().After(deadline) {
				return lastErr
			}
			time.Sleep(300 * time.Millisecond)
			continue
		}
		return nil
	}
}

func startUpdatedProcess(request ApplyUpdateRequest) error {
	cmd := exec.Command(request.CurrentPath, request.Args...)
	if strings.TrimSpace(request.WorkDir) != "" {
		cmd.Dir = request.WorkDir
	}
	env := append(os.Environ(), "ALLBOT_UPDATED=1")
	if strings.TrimSpace(request.FromVersion) != "" {
		env = append(env, "ALLBOT_UPDATED_FROM="+request.FromVersion)
	}
	if strings.TrimSpace(request.ToVersion) != "" {
		env = append(env, "ALLBOT_UPDATED_TO="+request.ToVersion)
	}
	if strings.TrimSpace(request.RestartDelay) != "" {
		env = append(env, "ALLBOT_RESTART_DELAY_MS="+request.RestartDelay)
	}
	if strings.TrimSpace(request.RestartedFlag) != "" {
		env = append(env, "ALLBOT_RESTARTED="+request.RestartedFlag)
	}
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}
