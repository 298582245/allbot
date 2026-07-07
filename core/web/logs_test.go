package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestLogManagerQueryLogFileReadsHistoryByDate(t *testing.T) {
	withTempWorkdirForLogTests(t, func() {
		lm := newTestLogManager(t, 10)
		writeTestLogFile(t, "2026-06-25", "10:00:00 INFO first\n10:00:01 ERROR second\n10:00:02 WARN keyword third\n")

		result, err := lm.QueryLogFile("2026-06-25", 1, 2, "keyword", "warn")
		if err != nil {
			t.Fatal(err)
		}
		if result.Total != 1 || len(result.Items) != 1 {
			t.Fatalf("expected one filtered log, got total=%d items=%#v", result.Total, result.Items)
		}
		if result.Items[0].Time != "10:00:02" || result.Items[0].Level != "warn" || result.Items[0].Message != "keyword third" {
			t.Fatalf("unexpected log entry: %#v", result.Items[0])
		}
	})
}

func TestLogManagerQueryLogFileKeepsMultilineMessageTogether(t *testing.T) {
	withTempWorkdirForLogTests(t, func() {
		lm := newTestLogManager(t, 10)
		writeTestLogFile(t, "2026-06-25", "10:00:00 INFO sendMessage 第一行\n第二行\n最后一行\n10:00:01 ERROR next\n")

		result, err := lm.QueryLogFile("2026-06-25", 1, 10, "", "")
		if err != nil {
			t.Fatal(err)
		}
		if result.Total != 2 || len(result.Items) != 2 {
			t.Fatalf("expected two log entries, got total=%d items=%#v", result.Total, result.Items)
		}
		entry := result.Items[1]
		if entry.Time != "10:00:00" || entry.Level != "info" || entry.Message != "sendMessage 第一行\n第二行\n最后一行" {
			t.Fatalf("unexpected multiline entry: %#v", entry)
		}
	})
}

func TestLogManagerListLogFilesOnlyWhitelist(t *testing.T) {
	withTempWorkdirForLogTests(t, func() {
		lm := newTestLogManager(t, 10)
		writeTestLogFile(t, "2026-06-25", "10:00:00 INFO ok\n")
		if err := os.WriteFile(filepath.Join("logs", "latest.log"), []byte("skip"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join("logs", "2026-06-25.log.bak"), []byte("skip"), 0644); err != nil {
			t.Fatal(err)
		}

		items, err := lm.ListLogFiles()
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 1 || items[0].Date != "2026-06-25" {
			t.Fatalf("unexpected log file list: %#v", items)
		}
	})
}

func TestLogManagerDeleteCurrentDateClosesFileHandle(t *testing.T) {
	withTempWorkdirForLogTests(t, func() {
		lm := newTestLogManager(t, 10)
		lm.appendLog(LogEntry{Time: "10:00:00", Level: "info", Message: "today"})
		date := time.Now().Format(logDateLayout)

		if err := lm.DeleteLogDate(date); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(logFilePath(date)); !os.IsNotExist(err) {
			t.Fatalf("expected current log file deleted, err=%v", err)
		}
		if lm.logFile != nil || lm.logDate != "" {
			t.Fatal("expected log file handle reset")
		}
	})
}

func TestLogManagerDeleteAllLogFilesOnlyWhitelist(t *testing.T) {
	withTempWorkdirForLogTests(t, func() {
		lm := newTestLogManager(t, 10)
		writeTestLogFile(t, "2026-06-24", "10:00:00 INFO old\n")
		writeTestLogFile(t, "2026-06-25", "10:00:00 INFO new\n")
		other := filepath.Join("logs", "keep.txt")
		if err := os.WriteFile(other, []byte("keep"), 0644); err != nil {
			t.Fatal(err)
		}

		deleted, err := lm.DeleteAllLogFiles()
		if err != nil {
			t.Fatal(err)
		}
		if deleted != 2 {
			t.Fatalf("expected 2 deleted files, got %d", deleted)
		}
		if _, err := os.Stat(other); err != nil {
			t.Fatalf("non-log file should remain: %v", err)
		}
	})
}

func TestLogManagerCleanupExpiredLogs(t *testing.T) {
	withTempWorkdirForLogTests(t, func() {
		lm := newTestLogManager(t, 10)
		oldDate := time.Now().AddDate(0, 0, -10).Format(logDateLayout)
		newDate := time.Now().Format(logDateLayout)
		writeTestLogFile(t, oldDate, "10:00:00 INFO old\n")
		writeTestLogFile(t, newDate, "10:00:00 INFO new\n")

		deleted, err := lm.CleanupExpiredLogs(7)
		if err != nil {
			t.Fatal(err)
		}
		if deleted != 1 {
			t.Fatalf("expected one expired file deleted, got %d", deleted)
		}
		if _, err := os.Stat(logFilePath(oldDate)); !os.IsNotExist(err) {
			t.Fatalf("old file should be deleted, err=%v", err)
		}
		if _, err := os.Stat(logFilePath(newDate)); err != nil {
			t.Fatalf("new file should remain: %v", err)
		}
	})
}

func TestHandleLogsQueryPaginationAndFilter(t *testing.T) {
	withTempWorkdirForLogTests(t, func() {
		server := &Server{logManager: newTestLogManager(t, 10)}
		writeTestLogFile(t, "2026-06-25", "10:00:00 INFO first\n10:00:01 ERROR keyword second\n10:00:02 ERROR keyword third\n")
		req := httptest.NewRequest(http.MethodGet, "/api/logs?date=2026-06-25&page=2&page_size=1&keyword=keyword&level=error", nil)
		recorder := httptest.NewRecorder()

		server.handleLogs(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("unexpected status %d: %s", recorder.Code, recorder.Body.String())
		}
		var result LogQueryResult
		if err := json.NewDecoder(recorder.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		if result.Total != 2 || len(result.Items) != 1 || result.Items[0].Message != "keyword second" {
			t.Fatalf("unexpected response: %#v", result)
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

func writeTestLogFile(t *testing.T, date, content string) {
	t.Helper()
	if err := os.MkdirAll("logs", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logFilePath(date), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
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
