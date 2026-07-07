package web

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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

// LogFileInfo 表示可查看的单个磁盘日志文件。
type LogFileInfo struct {
	Date    string    `json:"date"`
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

// LogQueryResult 表示日志历史查询结果。
type LogQueryResult struct {
	Items         []LogEntry    `json:"items"`
	Total         int           `json:"total"`
	Page          int           `json:"page"`
	PageSize      int           `json:"page_size"`
	Date          string        `json:"date"`
	Dates         []LogFileInfo `json:"dates"`
	RetentionDays int           `json:"retention_days"`
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

const (
	logFileRepeatSummaryInterval = 10
	logDateLayout                = "2006-01-02"
	defaultLogPageSize           = 100
	maxLogPageSize               = 1000
)

var (
	telegramBotTokenPattern = regexp.MustCompile(`/bot[^/\s\"]+`)
	urlVolatileQueryPattern = regexp.MustCompile(`(?i)([?&](?:offset|timeout|limit|timestamp|ts|nonce|retry|attempt|t)=)[^&\s\"]+`)
	whitespacePattern       = regexp.MustCompile(`\s+`)
	logFileNamePattern      = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}\.log$`)
	logLineHeaderPattern    = regexp.MustCompile(`^\d{2}:\d{2}:\d{2}\s+(?i:INFO|WARN|ERROR|DEBUG)\s+`)
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
	date := time.Now().Format(logDateLayout)
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

	path := logFilePath(date)
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

// QueryLogFile 按日期读取磁盘日志，并在服务端完成筛选与分页。
func (lm *LogManager) QueryLogFile(date string, page, pageSize int, keyword, level string) (LogQueryResult, error) {
	page, pageSize = normalizeLogPage(page, pageSize)
	dates, err := lm.ListLogFiles()
	if err != nil {
		return LogQueryResult{}, err
	}
	date, err = resolveLogQueryDate(date, dates)
	if err != nil {
		return LogQueryResult{}, err
	}
	if date != "" {
		if err := lm.flushLogFileForRead(date); err != nil {
			return LogQueryResult{}, err
		}
	}

	items, err := readLogEntriesFromFile(date, keyword, level)
	if err != nil {
		return LogQueryResult{}, err
	}
	total := len(items)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return LogQueryResult{Items: items[start:end], Total: total, Page: page, PageSize: pageSize, Date: date, Dates: dates}, nil
}

// ListLogFiles 只列出 logs 目录下符合 YYYY-MM-DD.log 的普通文件。
func (lm *LogManager) ListLogFiles() ([]LogFileInfo, error) {
	entries, err := os.ReadDir("logs")
	if os.IsNotExist(err) {
		return []LogFileInfo{}, nil
	}
	if err != nil {
		return nil, err
	}
	items := make([]LogFileInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !logFileNamePattern.MatchString(entry.Name()) {
			continue
		}
		date := strings.TrimSuffix(entry.Name(), ".log")
		if _, err := parseLogDate(date); err != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		items = append(items, LogFileInfo{Date: date, Name: entry.Name(), Size: info.Size(), ModTime: info.ModTime()})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Date > items[j].Date })
	return items, nil
}

// DeleteLogDate 删除指定日期日志文件，删除当天文件前会关闭当前句柄。
func (lm *LogManager) DeleteLogDate(date string) error {
	if _, err := parseLogDate(date); err != nil {
		return err
	}
	if err := lm.closeLogFileForDelete(date); err != nil {
		return err
	}
	if err := os.Remove(logFilePath(date)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// DeleteAllLogFiles 删除全部白名单日志文件，并清空内存日志。
func (lm *LogManager) DeleteAllLogFiles() (int, error) {
	if err := lm.closeLogFileForDelete(""); err != nil {
		return 0, err
	}
	dates, err := lm.ListLogFiles()
	if err != nil {
		return 0, err
	}
	deleted := 0
	for _, item := range dates {
		if err := os.Remove(logFilePath(item.Date)); err != nil && !os.IsNotExist(err) {
			return deleted, err
		}
		deleted++
	}
	lm.mu.Lock()
	lm.logs = make([]LogEntry, 0, lm.maxLogs)
	lm.lastLogKey = ""
	lm.mu.Unlock()
	return deleted, nil
}

// CleanupExpiredLogs 按保留天数删除过期日志文件，0 表示不清理。
func (lm *LogManager) CleanupExpiredLogs(retentionDays int) (int, error) {
	if retentionDays <= 0 {
		return 0, nil
	}
	today := time.Now().Truncate(24 * time.Hour)
	cutoff := today.AddDate(0, 0, -retentionDays+1)
	dates, err := lm.ListLogFiles()
	if err != nil {
		return 0, err
	}
	deleted := 0
	for _, item := range dates {
		fileDate, err := parseLogDate(item.Date)
		if err != nil || !fileDate.Before(cutoff) {
			continue
		}
		if err := lm.DeleteLogDate(item.Date); err != nil {
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
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
	lm.resetFileRepeatState()
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

func (lm *LogManager) flushLogFileForRead(date string) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	if lm.logFile == nil || lm.logDate != date {
		return nil
	}
	if err := lm.flushFileRepeatSummary(); err != nil {
		return err
	}
	return lm.logFile.Sync()
}

func (lm *LogManager) closeLogFileForDelete(date string) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	if date == "" || lm.logDate == date {
		if err := lm.flushFileRepeatSummary(); err != nil {
			return err
		}
		if lm.logFile != nil {
			if err := lm.logFile.Close(); err != nil {
				return err
			}
			lm.logFile = nil
		}
		lm.logDate = ""
		lm.resetFileRepeatState()
	}
	return nil
}

func (lm *LogManager) resetFileRepeatState() {
	lm.fileRepeatKey = ""
	lm.fileRepeat = 0
	lm.fileRepeatSummarized = 0
	lm.fileLastEntry = LogEntry{}
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
	switch r.Method {
	case http.MethodGet:
		query := r.URL.Query()
		page, _ := strconv.Atoi(query.Get("page"))
		pageSize, _ := strconv.Atoi(query.Get("page_size"))
		result, err := s.logManager.QueryLogFile(query.Get("date"), page, pageSize, query.Get("keyword"), query.Get("level"))
		if err != nil {
			s.jsonError(w, "查询日志失败: "+err.Error(), http.StatusBadRequest)
			return
		}
		if db := s.logSettingsDatabase(); db != nil {
			if days, err := db.GetLogRetentionDays(); err == nil {
				result.RetentionDays = days
			}
		}
		s.jsonResponse(w, result)
	case http.MethodDelete:
		date := strings.TrimSpace(r.URL.Query().Get("date"))
		if date != "" {
			if err := s.logManager.DeleteLogDate(date); err != nil {
				s.jsonError(w, "删除日志失败: "+err.Error(), http.StatusBadRequest)
				return
			}
			s.jsonResponse(w, map[string]interface{}{"message": "日志文件已删除"})
			return
		}
		deleted, err := s.logManager.DeleteAllLogFiles()
		if err != nil {
			s.jsonError(w, "清空日志失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"message": "日志已清空", "deleted": deleted})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleLogSettings(w http.ResponseWriter, r *http.Request) {
	db := s.logSettingsDatabase()
	if db == nil {
		s.jsonError(w, "配置数据库未初始化", http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodGet:
		days, err := db.GetLogRetentionDays()
		if err != nil {
			s.jsonError(w, "读取日志设置失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"retention_days": days})
	case http.MethodPut:
		var req struct {
			RetentionDays int `json:"retention_days"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "请求数据无效", http.StatusBadRequest)
			return
		}
		if err := db.SaveLogRetentionDays(req.RetentionDays); err != nil {
			s.jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.jsonResponse(w, map[string]interface{}{"message": "日志设置已保存", "retention_days": req.RetentionDays})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleLogCleanup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	days := 0
	if db := s.logSettingsDatabase(); db != nil {
		value, err := db.GetLogRetentionDays()
		if err != nil && err != sql.ErrNoRows {
			s.jsonError(w, "读取日志设置失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		days = value
	}
	var req struct {
		RetentionDays *int `json:"retention_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		s.jsonError(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	if req.RetentionDays != nil {
		days = *req.RetentionDays
	}
	if days < 0 || days > 3650 {
		s.jsonError(w, "日志保留天数必须在 0 到 3650 之间", http.StatusBadRequest)
		return
	}
	deleted, err := s.logManager.CleanupExpiredLogs(days)
	if err != nil {
		s.jsonError(w, "清理日志失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, map[string]interface{}{"message": "日志清理完成", "deleted": deleted, "retention_days": days})
}

func (s *Server) logSettingsDatabase() interface {
	GetLogRetentionDays() (int, error)
	SaveLogRetentionDays(int) error
} {
	if s == nil || s.adapterManager == nil {
		return nil
	}
	return s.adapterManager.GetDatabase()
}

func readLogEntriesFromFile(date, keyword, level string) ([]LogEntry, error) {
	if date == "" {
		return []LogEntry{}, nil
	}
	path := logFilePath(date)
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return []LogEntry{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	keyword = strings.ToLower(strings.TrimSpace(keyword))
	level = strings.ToLower(strings.TrimSpace(level))
	items := make([]LogEntry, 0)
	var current *LogEntry
	appendCurrent := func() {
		if current == nil {
			return
		}
		entry := normalizeLogEntry(*current)
		if level != "" && strings.ToLower(entry.Level) != level {
			current = nil
			return
		}
		if keyword != "" && !logEntryContainsKeyword(entry, keyword) {
			current = nil
			return
		}
		items = append(items, entry)
		current = nil
	}
	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 0, 64*1024)
	scanner.Buffer(buffer, 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if isLogLineHeader(line) {
			appendCurrent()
			entry := parseLogLine(line)
			current = &entry
			continue
		}
		if current == nil {
			entry := parseLogLine(line)
			current = &entry
			continue
		}
		current.Message += "\n" + line
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	appendCurrent()
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
	return items, nil
}

func parseLogLine(line string) LogEntry {
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 3 {
		return normalizeLogEntry(LogEntry{Time: "", Level: "info", Message: line})
	}
	return normalizeLogEntry(LogEntry{Time: parts[0], Level: strings.ToLower(parts[1]), Message: parts[2]})
}

func isLogLineHeader(line string) bool {
	return logLineHeaderPattern.MatchString(line)
}

func logEntryContainsKeyword(entry LogEntry, keyword string) bool {
	values := []string{entry.Time, entry.LastTime, entry.Level, entry.Message, strconv.Itoa(entry.Repeat)}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), keyword) {
			return true
		}
	}
	return false
}

func normalizeLogPage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultLogPageSize
	}
	if pageSize > maxLogPageSize {
		pageSize = maxLogPageSize
	}
	return page, pageSize
}

func resolveLogQueryDate(date string, dates []LogFileInfo) (string, error) {
	date = strings.TrimSpace(date)
	if date != "" {
		_, err := parseLogDate(date)
		return date, err
	}
	today := time.Now().Format(logDateLayout)
	for _, item := range dates {
		if item.Date == today {
			return today, nil
		}
	}
	if len(dates) > 0 {
		return dates[0].Date, nil
	}
	return today, nil
}

func parseLogDate(date string) (time.Time, error) {
	date = strings.TrimSpace(date)
	parsed, err := time.ParseInLocation(logDateLayout, date, time.Local)
	if err != nil || parsed.Format(logDateLayout) != date {
		return time.Time{}, fmt.Errorf("日志日期必须为 YYYY-MM-DD")
	}
	return parsed, nil
}

func logFilePath(date string) string {
	return filepath.Join("logs", date+".log")
}
