package web

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/allbot/allbot/core/deps"
)

func (s *Server) handleRuntimeProfiles(w http.ResponseWriter, r *http.Request) {
	manager := s.runtimeDepsManager()
	if manager == nil {
		s.jsonError(w, "依赖管理器不可用", http.StatusInternalServerError)
		return
	}
	switch r.Method {
	case http.MethodGet:
		profiles, err := manager.ListRuntimeProfiles()
		if err != nil {
			s.jsonError(w, "读取运行环境失败: "+err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, profiles)
	case http.MethodPut:
		var req struct {
			Profiles []deps.RuntimeProfile `json:"profiles"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "请求数据无效", http.StatusBadRequest)
			return
		}
		profiles, err := manager.SaveRuntimeProfiles(req.Profiles)
		if err != nil {
			s.jsonError(w, "保存运行环境失败: "+err.Error(), http.StatusBadRequest)
			return
		}
		s.jsonResponse(w, profiles)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleRuntimeProfileTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	manager := s.runtimeDepsManager()
	if manager == nil {
		s.jsonError(w, "依赖管理器不可用", http.StatusInternalServerError)
		return
	}
	var profile deps.RuntimeProfile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		s.jsonError(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	result, err := manager.TestRuntimeProfile(profile)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.jsonResponse(w, result)
}

func (s *Server) handleRuntimeProfileInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	manager := s.runtimeDepsManager()
	if manager == nil {
		s.jsonError(w, "依赖管理器不可用", http.StatusInternalServerError)
		return
	}
	var req struct {
		ProfileID         string `json:"profile_id"`
		ProfileId         string `json:"profileId"`
		Force             bool   `json:"force"`
		AutoDownload      bool   `json:"auto_download"`
		AutoDownloadCamel bool   `json:"autoDownload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "请求数据无效", http.StatusBadRequest)
		return
	}
	profileID := req.ProfileID
	if profileID == "" {
		profileID = req.ProfileId
	}
	autoDownload := req.AutoDownload || req.AutoDownloadCamel
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		s.jsonError(w, "运行环境 Profile ID 不能为空", http.StatusBadRequest)
		return
	}
	profiles, err := manager.ListRuntimeProfiles()
	if err != nil {
		s.jsonError(w, "读取运行环境失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	found := false
	for _, profile := range profiles {
		if profile.ID == profileID {
			found = true
			break
		}
	}
	if !found {
		s.jsonError(w, "运行环境 Profile 不存在: "+profileID, http.StatusBadRequest)
		return
	}
	store := s.runtimeProfileInitJobs()
	job := store.start(profileID, func(progress deps.RuntimeProfileInitProgressFunc) (deps.RuntimeProfileInitResult, error) {
		return manager.InitializeRuntimeProfile(profileID, deps.RuntimeProfileInitOptions{ProfileID: profileID, Force: req.Force, AutoDownload: autoDownload, Progress: progress})
	})
	s.jsonResponse(w, job)
}

func (s *Server) handleRuntimeProfileInitJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobID := strings.TrimPrefix(r.URL.Path, "/api/runtime-profiles/init/")
	jobID = strings.TrimSpace(jobID)
	if jobID == "" || jobID == "latest" {
		profileID := strings.TrimSpace(r.URL.Query().Get("profile_id"))
		if profileID == "" {
			profileID = strings.TrimSpace(r.URL.Query().Get("profileId"))
		}
		if profileID == "" {
			s.jsonError(w, "运行环境 Profile ID 不能为空", http.StatusBadRequest)
			return
		}
		job, ok := s.runtimeProfileInitJobs().latestForProfile(profileID)
		if !ok {
			s.jsonError(w, "初始化任务不存在", http.StatusNotFound)
			return
		}
		s.jsonResponse(w, job)
		return
	}
	job, ok := s.runtimeProfileInitJobs().get(jobID)
	if !ok {
		s.jsonError(w, "初始化任务不存在", http.StatusNotFound)
		return
	}
	s.jsonResponse(w, job)
}

func (s *Server) handleRuntimeProfileStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	manager := s.runtimeDepsManager()
	if manager == nil {
		s.jsonError(w, "依赖管理器不可用", http.StatusInternalServerError)
		return
	}
	statuses, err := manager.ListRuntimeProfileStatuses()
	if err != nil {
		s.jsonError(w, "读取运行环境状态失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, statuses)
}

func (s *Server) runtimeDepsManager() *deps.Manager {
	if s.pluginManager == nil {
		return nil
	}
	return s.pluginManager.GetDepsManager()
}

func (s *Server) runtimeProfileInitJobs() *runtimeProfileInitJobStore {
	s.serverMu.Lock()
	defer s.serverMu.Unlock()
	if s.runtimeInitJobs == nil {
		s.runtimeInitJobs = newRuntimeProfileInitJobStore()
	}
	return s.runtimeInitJobs
}
