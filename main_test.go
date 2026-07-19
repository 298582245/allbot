package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/allbot/allbot/core/adapter"
	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/router"
	"github.com/allbot/allbot/core/types"
	"github.com/allbot/allbot/core/web"
)

func TestFormatAccessCodeStatus(t *testing.T) {
	cases := []struct {
		name     string
		enabled  bool
		code     string
		expected string
	}{
		{name: "enabled with code", enabled: true, code: "abc123", expected: "安全访问码已开启\n访问码: abc123\n访问入口: /login/abc123\n"},
		{name: "disabled with code", enabled: false, code: "abc123", expected: "安全访问码未开启\n当前保存的访问码: abc123\n"},
		{name: "enabled without code", enabled: true, code: "", expected: "安全访问码已开启\n当前未设置访问码\n"},
		{name: "disabled without code", enabled: false, code: "", expected: "安全访问码未开启\n当前未设置访问码\n"},
		{name: "trim code", enabled: true, code: "  abc123  ", expected: "安全访问码已开启\n访问码: abc123\n访问入口: /login/abc123\n"},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			if got := formatAccessCodeStatus(item.enabled, item.code); got != item.expected {
				t.Fatalf("formatAccessCodeStatus() = %q, expected %q", got, item.expected)
			}
		})
	}
}

func TestResetAdminPassword(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "config.db")
	database, err := config.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	if err := database.SetSetting("admin.username", "root", "管理员用户名"); err != nil {
		t.Fatalf("SetSetting username returned error: %v", err)
	}
	if err := database.SetSetting("admin.password", "old-password", "管理员密码哈希"); err != nil {
		t.Fatalf("SetSetting password returned error: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	var output bytes.Buffer
	if err := resetAdminPassword(dbPath, &output); err != nil {
		t.Fatalf("resetAdminPassword returned error: %v", err)
	}
	text := output.String()
	if !strings.Contains(text, "管理员账号: root") {
		t.Fatalf("output missing username: %q", text)
	}
	password := passwordFromResetOutput(text)
	if password == "" {
		t.Fatalf("output missing password: %q", text)
	}

	database, err = config.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("NewDatabase returned error after reset: %v", err)
	}
	defer database.Close()
	ok, err := database.VerifyAdminPassword(password)
	if err != nil {
		t.Fatalf("VerifyAdminPassword returned error: %v", err)
	}
	if !ok {
		t.Fatal("new password should verify")
	}
	generatedPassword, err := database.GetSetting("admin.generated_password")
	if err != nil {
		t.Fatalf("GetSetting generated password returned error: %v", err)
	}
	if generatedPassword != password {
		t.Fatal("generated password setting should match output password")
	}
}

func TestResetAdminPasswordRejectsMissingDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "config.db")
	var output bytes.Buffer
	if err := resetAdminPassword(dbPath, &output); err == nil {
		t.Fatal("resetAdminPassword expected missing database error")
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("database should not be created, stat error: %v", err)
	}
}

func TestPrintAccessCode(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "config.db")
	database, err := config.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	settings, err := database.GetSystemSettings()
	if err != nil {
		t.Fatalf("GetSystemSettings returned error: %v", err)
	}
	settings.AccessCodeEnabled = true
	settings.AccessCode = "test-code"
	if err := database.SaveSystemSettings(settings); err != nil {
		t.Fatalf("SaveSystemSettings returned error: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	var output bytes.Buffer
	if err := printAccessCode(dbPath, &output); err != nil {
		t.Fatalf("printAccessCode returned error: %v", err)
	}
	text := output.String()
	if !strings.Contains(text, "访问码: test-code") {
		t.Fatalf("output missing access code: %q", text)
	}
	if !strings.Contains(text, "/login/test-code") {
		t.Fatalf("output missing login path: %q", text)
	}
}

func TestPrintAccessCodeRejectsMissingDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "config.db")
	var output bytes.Buffer
	if err := printAccessCode(dbPath, &output); err == nil {
		t.Fatal("printAccessCode expected missing database error")
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("database should not be created, stat error: %v", err)
	}
}

func TestResolveWebPortDefault(t *testing.T) {
	t.Setenv("ALLBOT_WEB_PORT", "")
	port, err := resolveWebPort()
	if err != nil {
		t.Fatalf("resolveWebPort returned error: %v", err)
	}
	if port != "3000" {
		t.Fatalf("port = %q, expected %q", port, "3000")
	}
}

func TestResolveWebPortCustom(t *testing.T) {
	t.Setenv("ALLBOT_WEB_PORT", "3100")
	port, err := resolveWebPort()
	if err != nil {
		t.Fatalf("resolveWebPort returned error: %v", err)
	}
	if port != "3100" {
		t.Fatalf("port = %q, expected %q", port, "3100")
	}
}

func TestResolveWebPortBlank(t *testing.T) {
	t.Setenv("ALLBOT_WEB_PORT", "  ")
	port, err := resolveWebPort()
	if err != nil {
		t.Fatalf("resolveWebPort returned error: %v", err)
	}
	if port != "3000" {
		t.Fatalf("port = %q, expected %q", port, "3000")
	}
}

func TestResolveWebPortInvalid(t *testing.T) {
	invalidValues := []string{"abc", "0", "65536", "30a0", "+3000", "3000.0"}
	for _, value := range invalidValues {
		t.Run(value, func(t *testing.T) {
			t.Setenv("ALLBOT_WEB_PORT", value)
			if port, err := resolveWebPort(); err == nil {
				t.Fatalf("resolveWebPort() = %q, expected error", port)
			}
		})
	}
}

func TestResolveWebAssetMode(t *testing.T) {
	cases := []struct {
		name     string
		value    string
		expected web.WebAssetMode
	}{
		{name: "empty", value: "", expected: web.WebAssetModeEmbedded},
		{name: "blank", value: "  ", expected: web.WebAssetModeEmbedded},
		{name: "embedded", value: "embedded", expected: web.WebAssetModeEmbedded},
		{name: "external", value: "external", expected: web.WebAssetModeExternal},
		{name: "case and trim", value: " External ", expected: web.WebAssetModeExternal},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			t.Setenv("ALLBOT_WEB_MODE", item.value)
			mode, err := resolveWebAssetMode()
			if err != nil {
				t.Fatalf("resolveWebAssetMode returned error: %v", err)
			}
			if mode != item.expected {
				t.Fatalf("mode = %q, expected %q", mode, item.expected)
			}
		})
	}
}

func TestResolveWebAssetModeInvalid(t *testing.T) {
	invalidValues := []string{"auto", "1", "true"}
	for _, value := range invalidValues {
		t.Run(value, func(t *testing.T) {
			t.Setenv("ALLBOT_WEB_MODE", value)
			if mode, err := resolveWebAssetMode(); err == nil {
				t.Fatalf("resolveWebAssetMode() = %q, expected error", mode)
			}
		})
	}
}

func TestValidateExternalWebDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := validateExternalWebDir(dir); err != nil {
		t.Fatalf("validateExternalWebDir returned error: %v", err)
	}
}

func TestValidateExternalWebDirRejectsMissingIndex(t *testing.T) {
	if err := validateExternalWebDir(t.TempDir()); err == nil {
		t.Fatal("validateExternalWebDir expected missing index error")
	}
}

func TestRestartDelayFromEnv(t *testing.T) {
	cases := []struct {
		name     string
		value    string
		expected time.Duration
	}{
		{name: "empty", value: "", expected: 0},
		{name: "blank", value: "  ", expected: 0},
		{name: "valid", value: "1500", expected: 1500 * time.Millisecond},
		{name: "invalid", value: "abc", expected: 0},
		{name: "zero", value: "0", expected: 0},
		{name: "negative", value: "-1", expected: 0},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			t.Setenv("ALLBOT_RESTART_DELAY_MS", item.value)
			if got := restartDelayFromEnv(); got != item.expected {
				t.Fatalf("restartDelayFromEnv() = %v, expected %v", got, item.expected)
			}
		})
	}
}

func TestBuildRestartCommand(t *testing.T) {
	t.Setenv("ALLBOT_RESTART_DELAY_MS", "99")
	t.Setenv("ALLBOT_IGNORE_RESTART_MESSAGE_KEY", "message-key")
	t.Setenv("ALLBOT_RESTART_NOTIFY_PLATFORM", "qq")
	t.Setenv("ALLBOT_RESTART_NOTIFY_TARGET", "1001")
	t.Setenv("ALLBOT_RESTART_STARTED_AT_NS", "123456")
	exe := "D:/test/allbot.exe"
	args := []string{"--plugins", "./plugins"}
	wd := "D:/work/allbot"

	cmd := buildRestartCommand(exe, args, wd)
	if cmd.Path != exe {
		t.Fatalf("Path = %q, expected %q", cmd.Path, exe)
	}
	if len(cmd.Args) != len(args)+1 {
		t.Fatalf("Args len = %d, expected %d", len(cmd.Args), len(args)+1)
	}
	if cmd.Args[0] != exe {
		t.Fatalf("Args[0] = %q, expected %q", cmd.Args[0], exe)
	}
	for i, arg := range args {
		if cmd.Args[i+1] != arg {
			t.Fatalf("Args[%d] = %q, expected %q", i+1, cmd.Args[i+1], arg)
		}
	}
	if cmd.Dir != wd {
		t.Fatalf("Dir = %q, expected %q", cmd.Dir, wd)
	}
	if envValue(cmd.Env, "ALLBOT_RESTARTED") != "1" {
		t.Fatal("ALLBOT_RESTARTED env missing")
	}
	if envValue(cmd.Env, "ALLBOT_RESTART_DELAY_MS") != "2000" {
		t.Fatalf("ALLBOT_RESTART_DELAY_MS = %q, expected 2000", envValue(cmd.Env, "ALLBOT_RESTART_DELAY_MS"))
	}
	if envValue(cmd.Env, "ALLBOT_PARENT_PID") != strconv.Itoa(os.Getpid()) {
		t.Fatalf("ALLBOT_PARENT_PID = %q, expected current pid", envValue(cmd.Env, "ALLBOT_PARENT_PID"))
	}
	if envValue(cmd.Env, "ALLBOT_IGNORE_RESTART_MESSAGE_KEY") != "message-key" {
		t.Fatal("ALLBOT_IGNORE_RESTART_MESSAGE_KEY env missing")
	}
	if envValue(cmd.Env, "ALLBOT_RESTART_NOTIFY_PLATFORM") != "qq" {
		t.Fatal("ALLBOT_RESTART_NOTIFY_PLATFORM env missing")
	}
	if envValue(cmd.Env, "ALLBOT_RESTART_NOTIFY_TARGET") != "1001" {
		t.Fatal("ALLBOT_RESTART_NOTIFY_TARGET env missing")
	}
	if envValue(cmd.Env, "ALLBOT_RESTART_STARTED_AT_NS") != "123456" {
		t.Fatal("ALLBOT_RESTART_STARTED_AT_NS env missing")
	}
	if cmd.Stdin != os.Stdin || cmd.Stdout != os.Stdout || cmd.Stderr != os.Stderr {
		t.Fatal("restart command should inherit standard streams")
	}
}

func TestSaveRestartContext(t *testing.T) {
	request := router.RestartRequest{
		MessageKey: "message-key",
		Platform:   "qq",
		AdapterID:  "7",
		UserID:     "1001",
		GroupID:    "2001",
		Target:     "group_2001",
		StartedAt:  time.Unix(0, 123456),
	}
	if err := saveRestartContext(request); err != nil {
		t.Fatalf("saveRestartContext returned error: %v", err)
	}
	if os.Getenv("ALLBOT_IGNORE_RESTART_MESSAGE_KEY") != request.MessageKey {
		t.Fatal("message key was not saved")
	}
	if os.Getenv("ALLBOT_RESTART_NOTIFY_PLATFORM") != request.Platform {
		t.Fatal("platform was not saved")
	}
	if os.Getenv("ALLBOT_RESTART_NOTIFY_ADAPTER_ID") != request.AdapterID {
		t.Fatal("adapter id was not saved")
	}
	if os.Getenv("ALLBOT_RESTART_NOTIFY_TARGET") != request.Target {
		t.Fatal("target was not saved")
	}
	if os.Getenv("ALLBOT_RESTART_STARTED_AT_NS") != "123456" {
		t.Fatal("started time was not saved")
	}
}

func TestSendRestartCompletedMessageUsesSequenceCapability(t *testing.T) {
	base := &restartRecordingAdapter{}
	adp := &restartSequenceRecordingAdapter{restartRecordingAdapter: base}
	target := " group_group-openid|msg_msg-group|at_member-openid "
	if err := sendRestartCompletedMessage(adp, target, "完成"); err != nil {
		t.Fatalf("sendRestartCompletedMessage returned error: %v", err)
	}
	if base.sendCalls != 0 || adp.sequenceCalls != 1 {
		t.Fatalf("sendCalls=%d sequenceCalls=%d", base.sendCalls, adp.sequenceCalls)
	}
	if adp.target != strings.TrimSpace(target) || adp.text != "完成" || adp.sequence != 2 {
		t.Fatalf("target=%q text=%q sequence=%d", adp.target, adp.text, adp.sequence)
	}
}

func TestSendRestartCompletedMessageFallsBackToSendMessage(t *testing.T) {
	adp := &restartRecordingAdapter{}
	target := " group_group-openid|msg_msg-group|at_member-openid "
	if err := sendRestartCompletedMessage(adp, target, "完成"); err != nil {
		t.Fatalf("sendRestartCompletedMessage returned error: %v", err)
	}
	if adp.sendCalls != 1 || adp.target != strings.TrimSpace(target) || adp.text != "完成" {
		t.Fatalf("sendCalls=%d target=%q text=%q", adp.sendCalls, adp.target, adp.text)
	}
}

func TestBuildRestartCompletedMessage(t *testing.T) {
	now := time.Unix(20, 0)
	cases := []struct {
		name      string
		value     string
		expected  string
		wantError bool
	}{
		{name: "seconds", value: strconv.FormatInt(now.Add(-5800*time.Millisecond).UnixNano(), 10), expected: "AllBot 重启完成，耗时：5.8s"},
		{name: "milliseconds", value: strconv.FormatInt(now.Add(-250*time.Millisecond).UnixNano(), 10), expected: "AllBot 重启完成，耗时：250ms"},
		{name: "missing", value: "", expected: "AllBot 重启完成，耗时：未知", wantError: true},
		{name: "invalid", value: "invalid", expected: "AllBot 重启完成，耗时：未知", wantError: true},
		{name: "zero", value: "0", expected: "AllBot 重启完成，耗时：未知", wantError: true},
		{name: "negative", value: "-1", expected: "AllBot 重启完成，耗时：未知", wantError: true},
		{name: "future", value: strconv.FormatInt(now.Add(time.Second).UnixNano(), 10), expected: "AllBot 重启完成，耗时：未知", wantError: true},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			message, err := buildRestartCompletedMessage(item.value, now)
			if message != item.expected {
				t.Fatalf("message = %q, expected %q", message, item.expected)
			}
			if (err != nil) != item.wantError {
				t.Fatalf("error = %v, wantError=%v", err, item.wantError)
			}
		})
	}
}

func TestFormatRestartDuration(t *testing.T) {
	if got := formatRestartDuration(250 * time.Millisecond); got != "250ms" {
		t.Fatalf("formatRestartDuration returned %q", got)
	}
	if got := formatRestartDuration(1500 * time.Millisecond); got != "1.5s" {
		t.Fatalf("formatRestartDuration returned %q", got)
	}
}

type restartRecordingAdapter struct {
	sendCalls int
	target    string
	text      string
}

func (a *restartRecordingAdapter) GetPlatform() string { return "test" }
func (a *restartRecordingAdapter) SendMessage(target string, text string) error {
	a.sendCalls++
	a.target = target
	a.text = text
	return nil
}
func (a *restartRecordingAdapter) SendImage(string, string) error { return nil }
func (a *restartRecordingAdapter) SendFile(string, string) error  { return nil }
func (a *restartRecordingAdapter) GetUserInfo(string) (*adapter.UserInfo, error) {
	return nil, nil
}
func (a *restartRecordingAdapter) GetGroupInfo(string) (*adapter.GroupInfo, error) {
	return nil, nil
}
func (a *restartRecordingAdapter) AtUser(string, string) error { return nil }
func (a *restartRecordingAdapter) Start() error                { return nil }
func (a *restartRecordingAdapter) Stop() error                 { return nil }
func (a *restartRecordingAdapter) SetMessageHandler(func(*types.Message)) {
}

type restartSequenceRecordingAdapter struct {
	*restartRecordingAdapter
	sequenceCalls int
	sequence      int
}

func (a *restartSequenceRecordingAdapter) SendMessageWithSequence(target string, text string, sequence int) error {
	a.sequenceCalls++
	a.target = target
	a.text = text
	a.sequence = sequence
	return nil
}

func passwordFromResetOutput(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "新密码: ") {
			return strings.TrimPrefix(line, "新密码: ")
		}
	}
	return ""
}

func envValue(env []string, key string) string {
	prefix := key + "="
	value := ""
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			value = strings.TrimPrefix(item, prefix)
		}
	}
	return value
}
