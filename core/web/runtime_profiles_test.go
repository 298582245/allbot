package web

import (
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/allbot/allbot/core/deps"
)

func TestRuntimeProfileInitSucceeds(t *testing.T) {
	withTempWorkdir(t, func() {
		nodePath, err := exec.LookPath("node")
		if err != nil {
			t.Skip("node 不可用，跳过初始化接口测试")
		}
		server := testServer(t)
		_, err = server.pluginManager.GetDepsManager().SaveRuntimeProfiles([]deps.RuntimeProfile{
			{ID: "node-default", Name: "默认 Node.js", Runtime: "nodejs", Executable: "node", Enabled: true, Default: true},
			{ID: "node18", Name: "Node.js 18", Runtime: "nodejs", Executable: nodePath, Enabled: true},
			{ID: "python-default", Name: "默认 Python", Runtime: "python", Executable: "python", Enabled: true, Default: true},
		})
		if err != nil {
			t.Fatal(err)
		}
		recorder := performOpenAPIJSONRequest(t, server.handleRuntimeProfileInit, http.MethodPost, "/api/runtime-profiles/init", map[string]interface{}{"profile_id": "node18"})
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		var response runtimeProfileInitJob
		decodeOpenAPIResponse(t, recorder, &response)
		if response.ProfileID != "node18" || response.Status != "running" || response.ID == "" {
			t.Fatalf("unexpected response: %#v", response)
		}
		completed := waitRuntimeProfileInitJob(t, server, response.ID)
		if completed.Status != "completed" || completed.Result == nil || completed.Result.VersionOutput == "" {
			t.Fatalf("unexpected completed job: %#v", completed)
		}
	})
}

func TestRuntimeProfileInitRejectsUnknownProfile(t *testing.T) {
	withTempWorkdir(t, func() {
		server := testServer(t)
		recorder := performOpenAPIJSONRequest(t, server.handleRuntimeProfileInit, http.MethodPost, "/api/runtime-profiles/init", map[string]interface{}{"profile_id": "missing"})
		if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "不存在") {
			t.Fatalf("expected missing profile 400, got %d: %s", recorder.Code, recorder.Body.String())
		}
	})
}

func TestRuntimeProfileInitJobCanBeReadByID(t *testing.T) {
	withTempWorkdir(t, func() {
		server := testServer(t)
		job := server.runtimeProfileInitJobs().start("node18", func(progress deps.RuntimeProfileInitProgressFunc) (deps.RuntimeProfileInitResult, error) {
			progress(deps.RuntimeProfileInitProgress{Stage: "download", Message: "测试进度", Progress: 50, DownloadedBytes: 10, TotalBytes: 20})
			return deps.RuntimeProfileInitResult{ProfileID: "node18", Status: "initialized", Message: "完成"}, nil
		})
		completed := waitRuntimeProfileInitJob(t, server, job.ID)
		recorder := performOpenAPIJSONRequest(t, server.handleRuntimeProfileInitJob, http.MethodGet, "/api/runtime-profiles/init/"+completed.ID, map[string]interface{}{})
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		var response runtimeProfileInitJob
		decodeOpenAPIResponse(t, recorder, &response)
		if response.Status != "completed" || response.Result == nil || response.Progress != 100 {
			t.Fatalf("unexpected job response: %#v", response)
		}
	})
}

func TestRuntimeProfileInitFailureKeepsSavedProfilesReadable(t *testing.T) {
	withTempWorkdir(t, func() {
		server := testServer(t)
		_, err := server.pluginManager.GetDepsManager().SaveRuntimeProfiles([]deps.RuntimeProfile{
			{ID: "node-default", Name: "默认 Node.js", Runtime: "nodejs", Executable: "node", Enabled: true, Default: true},
			{ID: "bad-node", Name: "Bad Node", Runtime: "nodejs", Executable: "runtime/missing/node.exe", Enabled: true},
			{ID: "python-default", Name: "默认 Python", Runtime: "python", Executable: "python", Enabled: true, Default: true},
		})
		if err != nil {
			t.Fatal(err)
		}
		recorder := performOpenAPIJSONRequest(t, server.handleRuntimeProfileInit, http.MethodPost, "/api/runtime-profiles/init", map[string]interface{}{"profile_id": "bad-node"})
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected init job 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		var job runtimeProfileInitJob
		decodeOpenAPIResponse(t, recorder, &job)
		failed := waitRuntimeProfileInitJob(t, server, job.ID)
		if failed.Status != "failed" || failed.Error == "" {
			t.Fatalf("expected failed job, got %#v", failed)
		}
		listRecorder := performOpenAPIJSONRequest(t, server.handleRuntimeProfiles, http.MethodGet, "/api/runtime-profiles", map[string]interface{}{})
		if listRecorder.Code != http.StatusOK || !strings.Contains(listRecorder.Body.String(), "bad-node") {
			t.Fatalf("saved profiles not readable: %d %s", listRecorder.Code, listRecorder.Body.String())
		}
	})
}

func TestRuntimeProfileStatusReturnsAllProfiles(t *testing.T) {
	withTempWorkdir(t, func() {
		server := testServer(t)
		_, err := server.pluginManager.GetDepsManager().SaveRuntimeProfiles([]deps.RuntimeProfile{
			{ID: "node-default", Name: "默认 Node.js", Runtime: "nodejs", Executable: "node", Enabled: true, Default: true},
			{ID: "node18", Name: "Node.js 18", Runtime: "nodejs", Executable: "runtime/missing/node.exe", Enabled: true},
			{ID: "python-default", Name: "默认 Python", Runtime: "python", Executable: "python", Enabled: true, Default: true},
		})
		if err != nil {
			t.Fatal(err)
		}
		recorder := performOpenAPIJSONRequest(t, server.handleRuntimeProfileStatus, http.MethodGet, "/api/runtime-profiles/status", map[string]interface{}{})
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		var response []deps.RuntimeProfileStatus
		decodeOpenAPIResponse(t, recorder, &response)
		if len(response) != 3 {
			t.Fatalf("expected 3 statuses, got %#v", response)
		}
	})
}

func waitRuntimeProfileInitJob(t *testing.T, server *Server, jobID string) runtimeProfileInitJob {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		job, ok := server.runtimeProfileInitJobs().get(jobID)
		if ok && (job.Status == "completed" || job.Status == "failed") {
			return job
		}
		time.Sleep(50 * time.Millisecond)
	}
	job, _ := server.runtimeProfileInitJobs().get(jobID)
	t.Fatalf("初始化任务未完成: %#v", job)
	return runtimeProfileInitJob{}
}
