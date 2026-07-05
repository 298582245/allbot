package web

import (
	"log"
	"sync"
	"time"

	"github.com/allbot/allbot/core/config"
)

// LogCleanupService 按系统设置定期清理过期日志文件。
type LogCleanupService struct {
	database *config.Database
	manager  *LogManager
	now      func() time.Time

	mu   sync.Mutex
	stop chan struct{}
	done chan struct{}
}

func NewLogCleanupService(database *config.Database, manager *LogManager) *LogCleanupService {
	return &LogCleanupService{database: database, manager: manager, now: time.Now}
}

func (s *LogCleanupService) Start() {
	if s == nil || s.database == nil || s.manager == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stop != nil {
		return
	}
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	go s.loop(s.stop, s.done)
}

func (s *LogCleanupService) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	stop := s.stop
	done := s.done
	s.stop = nil
	s.done = nil
	s.mu.Unlock()
	if stop == nil {
		return
	}
	close(stop)
	<-done
}

func (s *LogCleanupService) runOnce() {
	days, err := s.database.GetLogRetentionDays()
	if err != nil {
		log.Printf("[SYSTEM] 读取日志保留设置失败: %v", err)
		return
	}
	deleted, err := s.manager.CleanupExpiredLogs(days)
	if err != nil {
		log.Printf("[SYSTEM] 自动清理日志失败: %v", err)
		return
	}
	if deleted > 0 {
		log.Printf("[SYSTEM] 自动清理过期日志 %d 个", deleted)
	}
}

func (s *LogCleanupService) loop(stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	s.runOnce()
	for {
		duration := time.Until(nextLogCleanupAt(s.now()))
		if duration < 0 {
			duration = time.Minute
		}
		timer := time.NewTimer(duration)
		select {
		case <-timer.C:
			s.runOnce()
		case <-stop:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		}
	}
}

func nextLogCleanupAt(now time.Time) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), 3, 20, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}
