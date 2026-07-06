package web

import (
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/allbot/allbot/core/config"
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

func TestHandleRuntimeProfileDownloadCandidates(t *testing.T) {
	withTempWorkdir(t, func() {
		metadataServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/python/index.json" {
				t.Fatalf("unexpected path %s", r.URL.Path)
			}
			_, _ = w.Write([]byte(`{"items":[{"items":[{"catalogEntry":{"version":"3.12.4"}}]}]}`))
		}))
		defer metadataServer.Close()
		oldTransport := http.DefaultTransport
		http.DefaultTransport = metadataServer.Client().Transport
		t.Cleanup(func() { http.DefaultTransport = oldTransport })
		server := testServer(t)
		err := server.runtimeProfilesDatabase().SaveRuntimeDownloadSettings(config.RuntimeDownloadSettings{PythonMetadataURL: metadataServer.URL + "/python/index.json"})
		if err != nil {
			t.Fatal(err)
		}
		recorder := performOpenAPIJSONRequest(t, server.handleRuntimeProfileDownloadCandidates, http.MethodGet, "/api/runtime-profiles/download-candidates?runtime=python&architecture=win-x64&limit=10", map[string]interface{}{})
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
		}
		var response deps.RuntimeDownloadCandidateResult
		decodeOpenAPIResponse(t, recorder, &response)
		if response.Runtime != "python" || response.Architecture != "win-x64" || response.Source != "nuget" || len(response.Candidates) != 1 || response.Candidates[0].Version != "3.12.4" {
			t.Fatalf("unexpected response: %#v", response)
		}
	})
}

func TestHandleRuntimeProfileDownloadCandidatesRejectsInvalidParams(t *testing.T) {
	server := testServer(t)
	missing := performOpenAPIJSONRequest(t, server.handleRuntimeProfileDownloadCandidates, http.MethodGet, "/api/runtime-profiles/download-candidates", map[string]interface{}{})
	if missing.Code != http.StatusBadRequest {
		t.Fatalf("expected missing runtime 400, got %d: %s", missing.Code, missing.Body.String())
	}
	invalidRuntime := performOpenAPIJSONRequest(t, server.handleRuntimeProfileDownloadCandidates, http.MethodGet, "/api/runtime-profiles/download-candidates?runtime=ruby", map[string]interface{}{})
	if invalidRuntime.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid runtime 400, got %d: %s", invalidRuntime.Code, invalidRuntime.Body.String())
	}
	invalidArchitecture := performOpenAPIJSONRequest(t, server.handleRuntimeProfileDownloadCandidates, http.MethodGet, "/api/runtime-profiles/download-candidates?runtime=nodejs&architecture=darwin-x64", map[string]interface{}{})
	if invalidArchitecture.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid architecture 400, got %d: %s", invalidArchitecture.Code, invalidArchitecture.Body.String())
	}
}

func TestHandleRuntimeProfileDownloadCandidatesUsesDefaultArchitecture(t *testing.T) {
	metadataServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[{"items":[{"catalogEntry":{"version":"3.12.4"}}]}]}`))
	}))
	defer metadataServer.Close()
	oldTransport := http.DefaultTransport
	http.DefaultTransport = metadataServer.Client().Transport
	t.Cleanup(func() { http.DefaultTransport = oldTransport })
	server := testServer(t)
	if err := server.runtimeProfilesDatabase().SaveRuntimeDownloadSettings(config.RuntimeDownloadSettings{PythonMetadataURL: metadataServer.URL}); err != nil {
		t.Fatal(err)
	}
	recorder := performOpenAPIJSONRequest(t, server.handleRuntimeProfileDownloadCandidates, http.MethodGet, "/api/runtime-profiles/download-candidates?runtime=python&limit=1", map[string]interface{}{})
	if recorder.Code == http.StatusBadRequest && strings.Contains(recorder.Body.String(), "架构") {
		t.Fatalf("architecture should default instead of 400: %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandleRuntimeProfileDownloadSettings(t *testing.T) {
	server := testServer(t)

	getRecorder := performOpenAPIJSONRequest(t, server.handleRuntimeProfileDownloadSettings, http.MethodGet, "/api/runtime-profiles/download-settings", map[string]interface{}{})
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", getRecorder.Code, getRecorder.Body.String())
	}
	var defaults config.RuntimeDownloadSettings
	decodeOpenAPIResponse(t, getRecorder, &defaults)
	if defaults != config.DefaultRuntimeDownloadSettings() {
		t.Fatalf("unexpected defaults: %#v", defaults)
	}

	putRecorder := performOpenAPIJSONRequest(t, server.handleRuntimeProfileDownloadSettings, http.MethodPut, "/api/runtime-profiles/download-settings", map[string]interface{}{
		"proxy_url":                 "http://127.0.0.1:7890",
		"node_mirror_url":           "https://npmmirror.com/mirrors/node",
		"python_package_mirror_url": "",
		"python_metadata_url":       "",
	})
	if putRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", putRecorder.Code, putRecorder.Body.String())
	}
	var response struct {
		Message  string                         `json:"message"`
		Settings config.RuntimeDownloadSettings `json:"settings"`
	}
	decodeOpenAPIResponse(t, putRecorder, &response)
	if response.Message != "保存成功" || response.Settings.ProxyURL != "http://127.0.0.1:7890" || response.Settings.NodeMirrorURL != "https://npmmirror.com/mirrors/node" {
		t.Fatalf("unexpected response: %#v", response)
	}
	if response.Settings.PythonPackageMirrorURL != config.DefaultRuntimeDownloadSettings().PythonPackageMirrorURL {
		t.Fatalf("expected python package default, got %#v", response.Settings)
	}
}

func TestHandleRuntimeProfileDownloadSettingsRejectsInvalidConfig(t *testing.T) {
	server := testServer(t)
	recorder := performOpenAPIJSONRequest(t, server.handleRuntimeProfileDownloadSettings, http.MethodPut, "/api/runtime-profiles/download-settings", map[string]interface{}{
		"proxy_url":       "ftp://proxy.example.com",
		"node_mirror_url": "https://nodejs.org/dist",
	})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
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
