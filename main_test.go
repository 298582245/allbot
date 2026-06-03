package main

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/allbot/allbot/core/router"
)

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

func TestFormatRestartDuration(t *testing.T) {
	if got := formatRestartDuration(250 * time.Millisecond); got != "250ms" {
		t.Fatalf("formatRestartDuration returned %q", got)
	}
	if got := formatRestartDuration(1500 * time.Millisecond); got != "1.5s" {
		t.Fatalf("formatRestartDuration returned %q", got)
	}
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
