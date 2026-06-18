package web

import (
	"fmt"
	"sync"
	"time"

	"github.com/allbot/allbot/core/deps"
)

type runtimeProfileInitJob struct {
	ID              string                         `json:"id"`
	ProfileID       string                         `json:"profile_id"`
	Status          string                         `json:"status"`
	Stage           string                         `json:"stage"`
	Message         string                         `json:"message"`
	Progress        int                            `json:"progress"`
	DownloadedBytes int64                          `json:"downloaded_bytes,omitempty"`
	TotalBytes      int64                          `json:"total_bytes,omitempty"`
	Result          *deps.RuntimeProfileInitResult `json:"result,omitempty"`
	Error           string                         `json:"error,omitempty"`
	CreatedAt       time.Time                      `json:"created_at"`
	UpdatedAt       time.Time                      `json:"updated_at"`
	FinishedAt      *time.Time                     `json:"finished_at,omitempty"`
}

type runtimeProfileInitJobStore struct {
	mu     sync.RWMutex
	jobs   map[string]*runtimeProfileInitJob
	latest map[string]string
}

func newRuntimeProfileInitJobStore() *runtimeProfileInitJobStore {
	return &runtimeProfileInitJobStore{jobs: map[string]*runtimeProfileInitJob{}, latest: map[string]string{}}
}

func (s *runtimeProfileInitJobStore) start(profileID string, run func(progress deps.RuntimeProfileInitProgressFunc) (deps.RuntimeProfileInitResult, error)) runtimeProfileInitJob {
	now := time.Now()
	job := &runtimeProfileInitJob{ID: fmt.Sprintf("rpinit-%d", now.UnixNano()), ProfileID: profileID, Status: "running", Stage: "queued", Message: "初始化任务已开始", Progress: 1, CreatedAt: now, UpdatedAt: now}
	s.mu.Lock()
	s.jobs[job.ID] = job
	s.latest[profileID] = job.ID
	s.cleanupLocked(now)
	snapshot := *job
	s.mu.Unlock()

	go func() {
		progress := func(item deps.RuntimeProfileInitProgress) {
			s.update(job.ID, func(target *runtimeProfileInitJob) {
				target.Stage = item.Stage
				target.Message = item.Message
				if item.Progress > target.Progress {
					target.Progress = item.Progress
				}
				if item.DownloadedBytes > 0 {
					target.DownloadedBytes = item.DownloadedBytes
				}
				if item.TotalBytes > 0 {
					target.TotalBytes = item.TotalBytes
				}
			})
		}
		result, err := run(progress)
		s.update(job.ID, func(target *runtimeProfileInitJob) {
			finishedAt := time.Now()
			target.FinishedAt = &finishedAt
			if err != nil {
				target.Status = "failed"
				target.Stage = "failed"
				target.Message = "运行环境初始化失败"
				target.Error = err.Error()
				if target.Progress < 100 {
					target.Progress = 100
				}
				return
			}
			target.Status = "completed"
			target.Stage = "completed"
			target.Message = result.Message
			target.Progress = 100
			target.Result = &result
		})
	}()
	return snapshot
}

func (s *runtimeProfileInitJobStore) get(id string) (runtimeProfileInitJob, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	if !ok {
		return runtimeProfileInitJob{}, false
	}
	return *job, true
}

func (s *runtimeProfileInitJobStore) latestForProfile(profileID string) (runtimeProfileInitJob, bool) {
	s.mu.RLock()
	id := s.latest[profileID]
	job, ok := s.jobs[id]
	s.mu.RUnlock()
	if !ok {
		return runtimeProfileInitJob{}, false
	}
	return *job, true
}

func (s *runtimeProfileInitJobStore) list() []runtimeProfileInitJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobs := make([]runtimeProfileInitJob, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, *job)
	}
	return jobs
}

func (s *runtimeProfileInitJobStore) update(id string, mutate func(*runtimeProfileInitJob)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return
	}
	mutate(job)
	job.UpdatedAt = time.Now()
}

func (s *runtimeProfileInitJobStore) cleanupLocked(now time.Time) {
	for id, job := range s.jobs {
		if job.FinishedAt == nil || now.Sub(*job.FinishedAt) < time.Hour {
			continue
		}
		delete(s.jobs, id)
		if s.latest[job.ProfileID] == id {
			delete(s.latest, job.ProfileID)
		}
	}
}
