package updater

import (
	"bytes"
	"encoding/base64"
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
	ParentPID       int      `json:"parentPid"`
	CurrentPath     string   `json:"currentPath"`
	NewPath         string   `json:"newPath"`
	BackupPath      string   `json:"backupPath"`
	UpdateRoot      string   `json:"updateRoot"`
	ExpectedSHA256  string   `json:"expectedSha256"`
	AssetName       string   `json:"assetName"`
	ChecksumPayload string   `json:"checksumPayload"`
	UpdateSignature string   `json:"updateSignature"`
	WorkDir         string   `json:"workDir"`
	Args            []string `json:"args"`
	FromVersion     string   `json:"fromVersion"`
	ToVersion       string   `json:"toVersion"`
	RestartDelay    string   `json:"restartDelay"`
	RestartedFlag   string   `json:"restartedFlag"`
}

func SaveApplyUpdateRequest(path string, request ApplyUpdateRequest) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("更新请求路径不能为空")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	return file.Close()
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
	if err := validateUpdateFile(request.NewPath, request.UpdateRoot); err != nil {
		return err
	}
	if err := VerifyFileSHA256(request.NewPath, request.ExpectedSHA256); err != nil {
		return fmt.Errorf("替换前升级包校验失败: %w", err)
	}
	if err := verifyApplyUpdateSignature(request); err != nil {
		return err
	}
	waitForParentExit(request.ParentPID, 2*time.Second)
	if err := validateUpdateFile(request.NewPath, request.UpdateRoot); err != nil {
		return err
	}
	if err := VerifyFileSHA256(request.NewPath, request.ExpectedSHA256); err != nil {
		return fmt.Errorf("替换前升级包二次校验失败: %w", err)
	}
	if err := verifyApplyUpdateSignature(request); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(request.BackupPath), 0700); err != nil {
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
	if strings.TrimSpace(request.UpdateRoot) == "" {
		return fmt.Errorf("受信任更新根目录不能为空")
	}
	if strings.TrimSpace(request.ExpectedSHA256) == "" {
		return fmt.Errorf("升级包期望 SHA256 不能为空")
	}
	if strings.TrimSpace(request.AssetName) == "" {
		return fmt.Errorf("升级资产名称不能为空")
	}
	if strings.TrimSpace(request.ChecksumPayload) == "" {
		return fmt.Errorf("升级校验文件原文不能为空")
	}
	if strings.TrimSpace(request.UpdateSignature) == "" {
		return fmt.Errorf("升级签名不能为空")
	}
	if err := ensurePathWithinRoot(request.NewPath, request.UpdateRoot); err != nil {
		return fmt.Errorf("新程序路径无效: %w", err)
	}
	if err := ensurePathWithinRoot(request.BackupPath, request.UpdateRoot); err != nil {
		return fmt.Errorf("备份路径无效: %w", err)
	}
	return nil
}

func verifyApplyUpdateSignature(request ApplyUpdateRequest) error {
	publicKey, err := trustedUpdatePublicKey()
	if err != nil {
		return fmt.Errorf("替换前更新签名信任根不可用: %w", err)
	}
	checksumBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(request.ChecksumPayload))
	if err != nil || len(checksumBytes) == 0 || len(checksumBytes) > int(maxChecksumFileBytes) {
		return fmt.Errorf("替换前校验文件原文格式无效")
	}
	if err := verifyUpdateSignature(publicKey, []byte(request.UpdateSignature), checksumBytes); err != nil {
		return fmt.Errorf("替换前更新签名校验失败: %w", err)
	}
	checksumFile, err := ParseChecksumFile(bytes.NewReader(checksumBytes))
	if err != nil {
		return fmt.Errorf("替换前解析已验签校验文件失败: %w", err)
	}
	expectedSHA256, ok := checksumFile.ExpectedSHA256(request.AssetName)
	if !ok {
		return fmt.Errorf("替换前校验文件缺少 %s", request.AssetName)
	}
	if !strings.EqualFold(expectedSHA256, strings.TrimSpace(request.ExpectedSHA256)) {
		return fmt.Errorf("替换前期望 SHA256 与已验签校验文件不一致")
	}
	return nil
}

func ensurePathWithinRoot(path string, root string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("路径超出更新根目录")
	}
	return nil
}

func validateUpdateFile(path string, root string) error {
	if err := ensurePathWithinRoot(path, root); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("升级包必须是普通文件且不能是符号链接")
	}
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	pathReal, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	return ensurePathWithinRoot(pathReal, rootReal)
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
