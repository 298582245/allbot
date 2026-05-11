package web

import (
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// LogEntry 日志条目
type LogEntry struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

// LogManager 日志管理器
type LogManager struct {
	logs     []LogEntry
	mu       sync.RWMutex
	maxLogs  int
	logFile  *os.File
	logChan  chan LogEntry
	stopChan chan struct{}
}

// NewLogManager 创建日志管理器
func NewLogManager(maxLogs int) *LogManager {
	lm := &LogManager{
		logs:     make([]LogEntry, 0, maxLogs),
		maxLogs:  maxLogs,
		logChan:  make(chan LogEntry, 100),
		stopChan: make(chan struct{}),
	}

	// 启动日志收集协程
	go lm.collectLogs()

	return lm
}

// collectLogs 收集日志
func (lm *LogManager) collectLogs() {
	for {
		select {
		case entry := <-lm.logChan:
			lm.mu.Lock()
			lm.logs = append([]LogEntry{entry}, lm.logs...)
			if len(lm.logs) > lm.maxLogs {
				lm.logs = lm.logs[:lm.maxLogs]
			}
			lm.mu.Unlock()
		case <-lm.stopChan:
			return
		}
	}
}

// AddLog 添加日志
func (lm *LogManager) AddLog(level, message string) {
	entry := LogEntry{
		Time:    time.Now().Format("15:04:05"),
		Level:   level,
		Message: message,
	}

	select {
	case lm.logChan <- entry:
	default:
		// 通道满了，丢弃日志
	}
}

// GetLogs 获取日志
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

// ClearLogs 清空日志
func (lm *LogManager) ClearLogs() {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.logs = make([]LogEntry, 0, lm.maxLogs)
}

// Stop 停止日志管理器
func (lm *LogManager) Stop() {
	close(lm.stopChan)
	if lm.logFile != nil {
		lm.logFile.Close()
	}
}

// CustomLogger 自定义日志记录器
type CustomLogger struct {
	logManager *LogManager
	logger     *log.Logger
}

// NewCustomLogger 创建自定义日志记录器
func NewCustomLogger(lm *LogManager) *CustomLogger {
	return &CustomLogger{
		logManager: lm,
		logger:     log.New(os.Stdout, "", log.LstdFlags),
	}
}

// Write 实现 io.Writer 接口
func (cl *CustomLogger) Write(p []byte) (n int, err error) {
	message := string(p)

	// 解析日志级别
	level := "info"
	if len(message) > 0 {
		if message[0] == '[' {
			// 尝试解析级别
			if len(message) > 7 && message[1:6] == "ERROR" {
				level = "error"
			} else if len(message) > 6 && message[1:5] == "WARN" {
				level = "warn"
			} else if len(message) > 7 && message[1:6] == "DEBUG" {
				level = "debug"
			}
		}
	}

	// 添加到日志管理器
	cl.logManager.AddLog(level, message)

	// 同时输出到标准输出
	return cl.logger.Writer().Write(p)
}

// handleLogs 处理日志请求
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// 获取日志
		logs := s.logManager.GetLogs(100)
		s.jsonResponse(w, logs)
	} else if r.Method == http.MethodDelete {
		// 清空日志
		s.logManager.ClearLogs()
		s.jsonResponse(w, map[string]interface{}{
			"message": "日志已清空",
		})
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
