package web

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogManagerMergesRepeatedTelegramTimeoutLogs(t *testing.T) {
	withTempWorkdirForLogTests(t, func() {
		lm := newTestLogManager(t, 10)
		first := `Telegram getUpdates失败: "https://api.telegram.org/bot111111:AAA/getUpdates?offset=474441399&timeout=30": context deadline exceeded`
		second := `Telegram getUpdates失败: "https://api.telegram.org/bot222222:BBB/getUpdates?offset=474441400&timeout=30": context deadline exceeded`

		lm.appendLog(LogEntry{Time: "10:00:00", Level: "error", Message: first})
		lm.appendLog(LogEntry{Time: "10:00:01", Level: "error", Message: second})

		logs := lm.GetLogs(10)
		if len(logs) != 1 {
			t.Fatalf("expected repeated logs to merge into one entry, got %d: %#v", len(logs), logs)
		}
		if logs[0].Repeat != 2 || logs[0].LastTime != "10:00:01" {
			t.Fatalf("unexpected repeat metadata: %#v", logs[0])
		}
		data, err := json.Marshal(logs[0])
		if err != nil {
			t.Fatal(err)
		}
		jsonText := string(data)
		if !strings.Contains(jsonText, `"repeat":2`) || !strings.Contains(jsonText, `"lastTime":"10:00:01"`) {
			t.Fatalf("json should expose repeat and lastTime fields: %s", jsonText)
		}
	})
}

func TestLogManagerClearLogsResetsRepeatState(t *testing.T) {
	withTempWorkdirForLogTests(t, func() {
		lm := newTestLogManager(t, 10)
		message := "同一条异常"

		lm.appendLog(LogEntry{Time: "10:00:00", Level: "error", Message: message})
		lm.appendLog(LogEntry{Time: "10:00:01", Level: "error", Message: message})
		lm.ClearLogs()
		lm.appendLog(LogEntry{Time: "10:00:02", Level: "error", Message: message})

		logs := lm.GetLogs(10)
		if len(logs) != 1 {
			t.Fatalf("expected one log after clear, got %d: %#v", len(logs), logs)
		}
		if logs[0].Repeat != 1 || logs[0].LastTime != "10:00:02" {
			t.Fatalf("clear should reset repeat state: %#v", logs[0])
		}
	})
}

func TestLogManagerWritesRepeatSummaryForFileLogs(t *testing.T) {
	withTempWorkdirForLogTests(t, func() {
		lm := newTestLogManager(t, 10)
		message := `Telegram getUpdates失败: "https://api.telegram.org/bot111111:AAA/getUpdates?offset=1&timeout=30": context deadline exceeded`

		lm.appendLog(LogEntry{Time: "10:00:00", Level: "error", Message: message})
		lm.appendLog(LogEntry{Time: "10:00:01", Level: "error", Message: strings.Replace(message, "offset=1", "offset=2", 1)})
		lm.appendLog(LogEntry{Time: "10:00:02", Level: "error", Message: strings.Replace(message, "offset=1", "offset=3", 1)})
		lm.appendLog(LogEntry{Time: "10:00:03", Level: "info", Message: "服务恢复"})
		closeLogFileForTest(t, lm)

		path := filepath.Join("logs", time.Now().Format("2006-01-02")+".log")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		content := string(data)
		if strings.Count(content, "https://api.telegram.org") != 1 {
			t.Fatalf("file log should only keep first repeated telegram line, got:\n%s", content)
		}
		if !strings.Contains(content, "上一条日志继续重复 2 次") || !strings.Contains(content, "服务恢复") {
			t.Fatalf("file log should contain repeat summary and next log, got:\n%s", content)
		}
	})
}

func TestLogManagerWritesPeriodicRepeatSummaryForLongRuns(t *testing.T) {
	withTempWorkdirForLogTests(t, func() {
		lm := newTestLogManager(t, 10)
		message := `Telegram getUpdates失败: "https://api.telegram.org/bot111111:AAA/getUpdates?offset=1&timeout=30": context deadline exceeded`

		for i := 0; i <= logFileRepeatSummaryInterval; i++ {
			lm.appendLog(LogEntry{Time: fmt.Sprintf("10:00:%02d", i), Level: "error", Message: strings.Replace(message, "offset=1", fmt.Sprintf("offset=%d", i+1), 1)})
		}
		closeLogFileForTest(t, lm)

		path := filepath.Join("logs", time.Now().Format("2006-01-02")+".log")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		content := string(data)
		if !strings.Contains(content, "上一条日志继续重复 10 次") {
			t.Fatalf("file log should periodically flush repeat summary, got:\n%s", content)
		}
	})
}

func newTestLogManager(t *testing.T, maxLogs int) *LogManager {
	t.Helper()
	lm := &LogManager{
		logs:    make([]LogEntry, 0, maxLogs),
		maxLogs: maxLogs,
	}
	t.Cleanup(func() {
		closeLogFileForTest(t, lm)
	})
	return lm
}

func closeLogFileForTest(t *testing.T, lm *LogManager) {
	t.Helper()
	lm.mu.Lock()
	defer lm.mu.Unlock()
	if lm.logFile != nil {
		if err := lm.logFile.Close(); err != nil {
			t.Fatal(err)
		}
		lm.logFile = nil
	}
}

func withTempWorkdirForLogTests(t *testing.T, fn func()) {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(original); err != nil {
			t.Fatal(err)
		}
	}()
	fn()
}
