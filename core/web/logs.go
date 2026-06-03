package web

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// LogEntry 表示前端展示和文件落盘共用的日志条目。
type LogEntry struct {
	Time     string `json:"time"`
	Level    string `json:"level"`
	Message  string `json:"message"`
	Repeat   int    `json:"repeat"`
	LastTime string `json:"lastTime"`
}

// LogManager 同时维护内存日志和按日期追加的文件日志。
type LogManager struct {
	logs                 []LogEntry
	mu                   sync.RWMutex
	maxLogs              int
	logFile              *os.File
	logDate              string
	logChan              chan LogEntry
	stopChan             chan struct{}
	lastLogKey           string
	fileRepeatKey        string
	fileRepeat           int
	fileRepeatSummarized int
	fileLastEntry        LogEntry
}

const logFileRepeatSummaryInterval = 10

var (
	telegramBotTokenPattern = regexp.MustCompile(`/bot[^/\s\"]+`)
	urlVolatileQueryPattern = regexp.MustCompile(`(?i)([?&](?:offset|timeout|limit|timestamp|ts|nonce|retry|attempt|t)=)[^&\s\"]+`)
	whitespacePattern       = regexp.MustCompile(`\s+`)
)

// NewLogManager 创建日志管理器。
func NewLogManager(maxLogs int) *LogManager {
	lm := &LogManager{
		logs:     make([]LogEntry, 0, maxLogs),
		maxLogs:  maxLogs,
		logChan:  make(chan LogEntry, 100),
		stopChan: make(chan struct{}),
	}

	go lm.collectLogs()

	return lm
}

func (lm *LogManager) collectLogs() {
	for {
		select {
		case entry := <-lm.logChan:
			lm.appendLog(entry)
		case <-lm.stopChan:
			return
		}
	}
}

// AddLog 添加日志，内存保留最新 maxLogs 条，文件按日期追加保存。
func (lm *LogManager) AddLog(level, message string) {
	entry := LogEntry{
		Time:    time.Now().Format("15:04:05"),
		Level:   level,
		Message: strings.TrimSpace(message),
	}

	select {
	case lm.logChan <- entry:
	default:
		lm.appendLog(entry)
	}
}

func (lm *LogManager) appendLog(entry LogEntry) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	entry = normalizeLogEntry(entry)
	entryKey := logEntryKey(entry)
	if len(lm.logs) > 0 && lm.lastLogKey == entryKey {
		lm.logs[0].Repeat++
		lm.logs[0].LastTime = entry.LastTime
	} else {
		lm.logs = append([]LogEntry{entry}, lm.logs...)
		if len(lm.logs) > lm.maxLogs {
			lm.logs = lm.logs[:lm.maxLogs]
		}
		lm.lastLogKey = entryKey
	}

	if err := lm.writeLogFile(entry, entryKey); err != nil {
		fmt.Fprintf(os.Stderr, "写入日志文件失败: %v\n", err)
	}
}

func normalizeLogEntry(entry LogEntry) LogEntry {
	entry.Message = strings.TrimSpace(entry.Message)
	if entry.Repeat <= 0 {
		entry.Repeat = 1
	}
	if entry.LastTime == "" {
		entry.LastTime = entry.Time
	}
	return entry
}

func logEntryKey(entry LogEntry) string {
	level := strings.ToLower(strings.TrimSpace(entry.Level))
	return level + "\x00" + normalizeLogMessageKey(entry.Message)
}

func normalizeLogMessageKey(message string) string {
	normalized := strings.TrimSpace(message)
	normalized = telegramBotTokenPattern.ReplaceAllString(normalized, "/bot<TOKEN>")
	normalized = urlVolatileQueryPattern.ReplaceAllString(normalized, "${1}<VALUE>")
	normalized = whitespacePattern.ReplaceAllString(normalized, " ")
	return normalized
}

func (lm *LogManager) writeLogFile(entry LogEntry, entryKey string) error {
	if lm.fileRepeatKey == "" {
		if err := lm.writeLogLine(entry); err != nil {
			return err
		}
		lm.fileRepeatKey = entryKey
		lm.fileRepeat = 1
		lm.fileRepeatSummarized = 1
		lm.fileLastEntry = entry
		return nil
	}

	if lm.fileRepeatKey == entryKey {
		lm.fileRepeat++
		lm.fileLastEntry = entry
		if lm.fileRepeat-lm.fileRepeatSummarized >= logFileRepeatSummaryInterval {
			return lm.flushFileRepeatSummary()
		}
		return nil
	}

	if err := lm.flushFileRepeatSummary(); err != nil {
		return err
	}
	if err := lm.writeLogLine(entry); err != nil {
		return err
	}
	lm.fileRepeatKey = entryKey
	lm.fileRepeat = 1
	lm.fileRepeatSummarized = 1
	lm.fileLastEntry = entry
	return nil
}

func (lm *LogManager) flushFileRepeatSummary() error {
	pendingRepeat := lm.fileRepeat - lm.fileRepeatSummarized
	if pendingRepeat <= 0 {
		return nil
	}

	repeatEntry := lm.fileLastEntry
	repeatEntry.Message = fmt.Sprintf("上一条日志继续重复 %d 次（累计 %d 次，末次时间 %s）", pendingRepeat, lm.fileRepeat, lm.fileLastEntry.LastTime)
	if err := lm.writeLogLine(repeatEntry); err != nil {
		return err
	}
	lm.fileRepeatSummarized = lm.fileRepeat
	return nil
}

func (lm *LogManager) writeLogLine(entry LogEntry) error {
	if err := lm.ensureLogFile(); err != nil {
		return err
	}
	_, err := fmt.Fprintf(lm.logFile, "%s %s %s\n", entry.LastTime, strings.ToUpper(entry.Level), entry.Message)
	return err
}

func (lm *LogManager) ensureLogFile() error {
	date := time.Now().Format("2006-01-02")
	if lm.logFile != nil && lm.logDate == date {
		return nil
	}

	if lm.logFile != nil {
		lm.logFile.Close()
		lm.logFile = nil
	}

	if err := os.MkdirAll("logs", 0755); err != nil {
		return err
	}

	path := filepath.Join("logs", date+".log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	lm.logFile = file
	lm.logDate = date
	return nil
}

// GetLogs 获取内存中的最新日志。
func (lm *LogManager) GetLogs(limit int) []LogEntry {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	if limit <= 0 || limit > len(lm.logs) {
		limit = len(lm.logs)
	}

	result := make([]LogEntry, limit)
	copy(result, lm.logs[:limit])
	return result
}

// ClearLogs 只清空前端内存日志，不删除 logs 目录中的持久化日志。
func (lm *LogManager) ClearLogs() {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	if err := lm.flushFileRepeatSummary(); err != nil {
		fmt.Fprintf(os.Stderr, "写入日志汇总失败: %v\n", err)
	}
	lm.logs = make([]LogEntry, 0, lm.maxLogs)
	lm.lastLogKey = ""
	lm.fileRepeatKey = ""
	lm.fileRepeat = 0
	lm.fileRepeatSummarized = 0
	lm.fileLastEntry = LogEntry{}
}

// Stop 停止日志管理器并关闭当前日志文件。
func (lm *LogManager) Stop() {
	close(lm.stopChan)
	lm.mu.Lock()
	defer lm.mu.Unlock()
	if err := lm.flushFileRepeatSummary(); err != nil {
		fmt.Fprintf(os.Stderr, "写入日志汇总失败: %v\n", err)
	}
	if lm.logFile != nil {
		lm.logFile.Close()
		lm.logFile = nil
	}
}

// CustomLogger 将标准库 log 输出同步到前端日志和标准输出。
type CustomLogger struct {
	logManager *LogManager
	logger     *log.Logger
}

// NewCustomLogger 创建自定义日志输出器。
func NewCustomLogger(lm *LogManager) *CustomLogger {
	return &CustomLogger{
		logManager: lm,
		logger:     log.New(os.Stdout, "", log.LstdFlags),
	}
}

// Write 实现 io.Writer 接口。
func (cl *CustomLogger) Write(p []byte) (n int, err error) {
	message := string(p)
	content := message

	if len(message) > 20 && message[4] == '/' && message[7] == '/' && message[10] == ' ' {
		content = message[20:]
	}

	level := "info"
	contentLower := strings.ToLower(content)
	if strings.Contains(contentLower, "warn") || strings.Contains(content, "警告") {
		level = "warn"
	} else if strings.Contains(contentLower, "error") || strings.Contains(contentLower, "failed") || strings.Contains(content, "失败") {
		level = "error"
	} else if strings.Contains(content, "[DEBUG]") {
		level = "debug"
	}

	cl.logManager.AddLog(level, content)

	return cl.logger.Writer().Write(p)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		logs := s.logManager.GetLogs(100)
		s.jsonResponse(w, logs)
	} else if r.Method == http.MethodDelete {
		s.logManager.ClearLogs()
		s.jsonResponse(w, map[string]interface{}{
			"message": "日志已清空",
		})
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
