package web

import (
	"log"
	"sync"
	"time"

	"github.com/allbot/allbot/core/config"
)

const (
	openAPIStatsFlushInterval  = time.Second
	openAPIStatsFlushThreshold = 100
	openAPIStatsMaxLogs        = 5000
	openAPIStatsCleanupBatch   = 500
)

type openAPIStatsWriter interface {
	WriteOpenAPICallBatch([]config.OpenAPICallStatDelta, []config.OpenAPICallLog) error
	CleanupOpenAPICallLogsBatch(int, int, time.Time) (int64, error)
}

type openAPIStatsRecorder struct {
	mu             sync.Mutex
	flushMu        sync.Mutex
	writer         openAPIStatsWriter
	retentionDays  func() int
	stats          map[string]config.OpenAPICallStatDelta
	logs           []config.OpenAPICallLog
	pending        int
	wake           chan struct{}
	stop           chan struct{}
	done           chan struct{}
	closeOnce      sync.Once
	runOnce        sync.Once
	started        bool
	closed         bool
	closeErr       error
	lastWarning    time.Time
	lastCleanupDay string
	flushInterval  time.Duration
	flushThreshold int
	maxLogs        int
}

func newOpenAPIStatsRecorder(writer openAPIStatsWriter, retentionDays func() int) *openAPIStatsRecorder {
	recorder := &openAPIStatsRecorder{
		writer: writer, retentionDays: retentionDays, stats: make(map[string]config.OpenAPICallStatDelta),
		wake: make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{}),
		flushInterval: openAPIStatsFlushInterval, flushThreshold: openAPIStatsFlushThreshold, maxLogs: openAPIStatsMaxLogs,
	}
	return recorder
}

func (r *openAPIStatsRecorder) Start() {
	if r == nil {
		return
	}
	r.runOnce.Do(func() {
		r.mu.Lock()
		if r.closed {
			r.mu.Unlock()
			return
		}
		r.started = true
		r.mu.Unlock()
		go r.run()
	})
}

func (r *openAPIStatsRecorder) Record(item config.OpenAPICallLog) {
	if r == nil || item.EndpointID == "" {
		return
	}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.mu.Unlock()
	r.Start()
	delta := config.OpenAPICallStatDelta{EndpointID: item.EndpointID, Total: 1, LastStatusCode: item.StatusCode, LastOutcome: item.Outcome, LastCalledAt: item.StartedAt}
	if delta.LastCalledAt.IsZero() {
		delta.LastCalledAt = time.Now()
		item.StartedAt = delta.LastCalledAt
	}
	switch item.Outcome {
	case config.OpenAPICallOutcomeSuccess:
		delta.Success = 1
	case config.OpenAPICallOutcomeIPDenied, config.OpenAPICallOutcomeTokenDenied:
		delta.Rejected = 1
	default:
		delta.Failed = 1
	}

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	current := r.stats[item.EndpointID]
	current.EndpointID = item.EndpointID
	current.Total += delta.Total
	current.Success += delta.Success
	current.Rejected += delta.Rejected
	current.Failed += delta.Failed
	if current.LastCalledAt.IsZero() || !delta.LastCalledAt.Before(current.LastCalledAt) {
		current.LastStatusCode = delta.LastStatusCode
		current.LastOutcome = delta.LastOutcome
		current.LastCalledAt = delta.LastCalledAt
	}
	r.stats[item.EndpointID] = current
	logDropped := len(r.logs) >= r.maxLogs
	if !logDropped {
		r.logs = append(r.logs, item)
	}
	r.pending++
	shouldWake := r.pending >= r.flushThreshold
	r.mu.Unlock()
	if logDropped {
		r.warnLimited("OpenAPI 调用明细队列已满，当前明细已丢弃", nil)
	}
	if shouldWake {
		select {
		case r.wake <- struct{}{}:
		default:
		}
	}
}

func (r *openAPIStatsRecorder) run() {
	defer close(r.done)
	ticker := time.NewTicker(r.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			_ = r.Flush()
		case <-r.wake:
			_ = r.Flush()
		case <-r.stop:
			err := r.Flush()
			r.mu.Lock()
			r.closeErr = err
			r.mu.Unlock()
			return
		}
	}
}

func (r *openAPIStatsRecorder) Flush() error {
	if r == nil {
		return nil
	}
	r.flushMu.Lock()
	defer r.flushMu.Unlock()
	stats, logs := r.takeBatch()
	if len(stats) == 0 && len(logs) == 0 {
		r.cleanupIfNeeded(time.Now())
		return nil
	}
	if r.writer == nil {
		return nil
	}
	if err := r.writer.WriteOpenAPICallBatch(stats, logs); err != nil {
		r.mergeFailedStats(stats)
		r.warnLimited("OpenAPI 调用统计写入失败，累计统计已回存，明细可能丢失", err)
		return err
	}
	r.cleanupIfNeeded(time.Now())
	return nil
}

func (r *openAPIStatsRecorder) takeBatch() ([]config.OpenAPICallStatDelta, []config.OpenAPICallLog) {
	r.mu.Lock()
	defer r.mu.Unlock()
	stats := make([]config.OpenAPICallStatDelta, 0, len(r.stats))
	for _, item := range r.stats {
		stats = append(stats, item)
	}
	logs := r.logs
	r.stats = make(map[string]config.OpenAPICallStatDelta)
	r.logs = nil
	r.pending = 0
	return stats, logs
}

func (r *openAPIStatsRecorder) mergeFailedStats(stats []config.OpenAPICallStatDelta) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range stats {
		current := r.stats[item.EndpointID]
		current.EndpointID = item.EndpointID
		current.Total += item.Total
		current.Success += item.Success
		current.Rejected += item.Rejected
		current.Failed += item.Failed
		if current.LastCalledAt.IsZero() || !item.LastCalledAt.Before(current.LastCalledAt) {
			current.LastStatusCode = item.LastStatusCode
			current.LastOutcome = item.LastOutcome
			current.LastCalledAt = item.LastCalledAt
		}
		r.stats[item.EndpointID] = current
		r.pending += int(item.Total)
	}
}

func (r *openAPIStatsRecorder) cleanupIfNeeded(now time.Time) {
	if r.writer == nil || r.retentionDays == nil {
		return
	}
	retention := r.retentionDays()
	if retention <= 0 {
		return
	}
	day := now.Format("2006-01-02")
	r.mu.Lock()
	if r.lastCleanupDay == day {
		r.mu.Unlock()
		return
	}
	r.mu.Unlock()
	for {
		deleted, err := r.writer.CleanupOpenAPICallLogsBatch(retention, openAPIStatsCleanupBatch, now)
		if err != nil {
			r.warnLimited("OpenAPI 调用明细清理失败", err)
			return
		}
		if deleted < openAPIStatsCleanupBatch {
			r.mu.Lock()
			r.lastCleanupDay = day
			r.mu.Unlock()
			return
		}
	}
}

func (r *openAPIStatsRecorder) warnLimited(message string, err error) {
	now := time.Now()
	r.mu.Lock()
	if !r.lastWarning.IsZero() && now.Sub(r.lastWarning) < time.Minute {
		r.mu.Unlock()
		return
	}
	r.lastWarning = now
	r.mu.Unlock()
	if err != nil {
		log.Printf("[WARN] %s: %v", message, err)
		return
	}
	log.Printf("[WARN] %s", message)
}

func (r *openAPIStatsRecorder) Close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		r.mu.Lock()
		started := r.started
		r.closed = true
		r.mu.Unlock()
		if started {
			close(r.stop)
			<-r.done
		} else {
			err := r.Flush()
			r.mu.Lock()
			r.closeErr = err
			r.mu.Unlock()
		}
	})
	r.mu.Lock()
	err := r.closeErr
	r.mu.Unlock()
	return err
}

func (s *Server) openAPIRetentionDays() int {
	return s.currentOpenAPIAccess().retentionDays
}

func (s *Server) recordOpenAPICall(item config.OpenAPICallLog) {
	if s != nil && s.openAPIStats != nil {
		s.openAPIStats.Record(item)
	}
}
