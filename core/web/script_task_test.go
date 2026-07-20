package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/allbot/allbot/core/config"
)

func TestHandleScriptTasksFiltersRuntimeProfile(t *testing.T) {
	server := testServer(t)
	startedAt := time.Now()
	if _, err := server.adapterManager.GetDatabase().SaveScriptRunLog(config.ScriptRunLog{PluginID: "plugin-a", UnionID: "union-a", ScriptPath: "scripts/a.js", Runtime: "nodejs", RuntimeProfile: "node18", RunMode: "manual", Status: "success", StartedAt: startedAt, FinishedAt: startedAt}); err != nil {
		t.Fatal(err)
	}
	if _, err := server.adapterManager.GetDatabase().SaveScriptRunLog(config.ScriptRunLog{PluginID: "plugin-a", UnionID: "union-a", ScriptPath: "scripts/a.js", Runtime: "nodejs", RuntimeProfile: "node20", RunMode: "manual", Status: "success", StartedAt: startedAt, FinishedAt: startedAt}); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/script-tasks?runtime_profile=node18", nil)
	server.handleScriptTasks(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Items []config.ScriptRunLog `json:"items"`
		Total int                   `json:"total"`
	}
	decodeUnifiedResponseData(t, recorder, &response)
	if response.Total != 1 || len(response.Items) != 1 || response.Items[0].RuntimeProfile != "node18" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestHandleScriptTasksFiltersQueuedStatus(t *testing.T) {
	server := testServer(t)
	startedAt := time.Now()
	if _, err := server.adapterManager.GetDatabase().SaveScriptRunLog(config.ScriptRunLog{PluginID: "plugin-a", UnionID: "union-a", ScriptPath: "scripts/a.js", Runtime: "nodejs", RuntimeProfile: "node18", RunMode: "manual", Status: config.ScriptRunStatusQueued, StartedAt: startedAt}); err != nil {
		t.Fatal(err)
	}
	if _, err := server.adapterManager.GetDatabase().SaveScriptRunLog(config.ScriptRunLog{PluginID: "plugin-a", UnionID: "union-a", ScriptPath: "scripts/a.js", Runtime: "nodejs", RuntimeProfile: "node20", RunMode: "manual", Status: "success", StartedAt: startedAt, FinishedAt: startedAt}); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/script-tasks?status=queued", nil)
	server.handleScriptTasks(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Items []config.ScriptRunLog `json:"items"`
		Total int                   `json:"total"`
	}
	decodeUnifiedResponseData(t, recorder, &response)
	if response.Total != 1 || len(response.Items) != 1 || response.Items[0].Status != config.ScriptRunStatusQueued {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestHandleScriptTasksReturnsSettings(t *testing.T) {
	server := testServer(t)
	settings := config.ScriptTaskSettings{RetentionDays: 3, RunTimeoutSeconds: 120, TimeoutNotifyAdminEnabled: true}
	if err := server.adapterManager.GetDatabase().SaveScriptTaskSettings(settings); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/script-tasks", nil)
	server.handleScriptTasks(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		RetentionDays int                       `json:"retention_days"`
		Settings      config.ScriptTaskSettings `json:"settings"`
	}
	decodeUnifiedResponseData(t, recorder, &response)
	if response.RetentionDays != settings.RetentionDays || response.Settings != settings {
		t.Fatalf("unexpected settings response: %#v", response)
	}
}

func TestHandleScriptTasksSaveSettings(t *testing.T) {
	server := testServer(t)
	body := bytes.NewBufferString(`{"retention_days":30,"run_timeout_seconds":600,"timeout_notify_admin_enabled":true}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/script-tasks?action=settings", body)
	server.handleScriptTasks(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	settings, err := server.adapterManager.GetDatabase().GetScriptTaskSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.RetentionDays != 30 || settings.RunTimeoutSeconds != 600 || !settings.TimeoutNotifyAdminEnabled {
		t.Fatalf("settings not saved: %#v", settings)
	}
}

func TestHandleScriptTasksRejectsInvalidRunTimeout(t *testing.T) {
	server := testServer(t)
	body := bytes.NewBufferString(`{"retention_days":0,"run_timeout_seconds":86401,"timeout_notify_admin_enabled":false}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/script-tasks?action=settings", body)
	server.handleScriptTasks(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandleScriptTasksCleanupKeepsOtherSettings(t *testing.T) {
	server := testServer(t)
	if err := server.adapterManager.GetDatabase().SaveScriptTaskSettings(config.ScriptTaskSettings{RunTimeoutSeconds: 60, TimeoutNotifyAdminEnabled: true}); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/script-tasks?action=cleanup&days=7", nil)
	server.handleScriptTasks(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	settings, err := server.adapterManager.GetDatabase().GetScriptTaskSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.RetentionDays != 7 || settings.RunTimeoutSeconds != 60 || !settings.TimeoutNotifyAdminEnabled {
		t.Fatalf("cleanup should only update retention days: %#v", settings)
	}
}

func TestHandleScriptTaskDetailIncludesRuntimeProfile(t *testing.T) {
	server := testServer(t)
	startedAt := time.Now()
	id, err := server.adapterManager.GetDatabase().SaveScriptRunLog(config.ScriptRunLog{PluginID: "plugin-a", UnionID: "union-a", ScriptPath: "scripts/a.js", Runtime: "nodejs", RuntimeProfile: "node18", RunMode: "manual", Status: "success", Output: "ok", StartedAt: startedAt, FinishedAt: startedAt})
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	server.getScriptTask(recorder, id)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response config.ScriptRunLog
	decodeUnifiedResponseData(t, recorder, &response)
	if response.RuntimeProfile != "node18" {
		t.Fatalf("runtime_profile missing: %#v", response)
	}
}

func TestHandleScriptTaskDetailIncludesQueuedStatus(t *testing.T) {
	server := testServer(t)
	startedAt := time.Now()
	id, err := server.adapterManager.GetDatabase().SaveScriptRunLog(config.ScriptRunLog{PluginID: "plugin-a", UnionID: "union-a", ScriptPath: "scripts/a.js", Runtime: "nodejs", RuntimeProfile: "node18", RunMode: "manual", Status: config.ScriptRunStatusQueued, StartedAt: startedAt})
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	server.getScriptTask(recorder, id)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response config.ScriptRunLog
	decodeUnifiedResponseData(t, recorder, &response)
	if response.Status != config.ScriptRunStatusQueued {
		t.Fatalf("queued status missing: %#v", response)
	}
}

func TestPauseQueuedScriptTaskReturnsNotRunningInTestServer(t *testing.T) {
	server := testServer(t)
	startedAt := time.Now()
	id, err := server.adapterManager.GetDatabase().SaveScriptRunLog(config.ScriptRunLog{PluginID: "plugin-a", UnionID: "union-a", ScriptPath: "scripts/a.js", Runtime: "nodejs", RuntimeProfile: "node18", RunMode: "manual", Status: config.ScriptRunStatusQueued, StartedAt: startedAt})
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/script-tasks/"+strconv.FormatInt(id, 10)+"?action=pause", nil)
	server.handleScriptTaskDetail(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]interface{}
	decodeUnifiedResponseData(t, recorder, &response)
	if response["message"] != "脚本任务已不在运行中" || response["status"] != config.ScriptRunStatusQueued {
		t.Fatalf("unexpected response: %#v", response)
	}
	item, err := server.adapterManager.GetDatabase().GetScriptRunLog(id)
	if err != nil {
		t.Fatal(err)
	}
	if item == nil || item.Status != config.ScriptRunStatusQueued {
		t.Fatalf("queued task should remain queued in test server: %#v", item)
	}
}

func TestPauseOrphanRunningScriptTaskMarksPaused(t *testing.T) {
	server := testServer(t)
	startedAt := time.Now()
	id, err := server.adapterManager.GetDatabase().SaveScriptRunLog(config.ScriptRunLog{PluginID: "plugin-a", UnionID: "union-a", ScriptPath: "scripts/a.js", Runtime: "nodejs", RuntimeProfile: "node18", RunMode: "manual", Status: "running", StartedAt: startedAt})
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/script-tasks/"+strconv.FormatInt(id, 10)+"?action=pause", nil)
	server.handleScriptTaskDetail(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]interface{}
	decodeUnifiedResponseData(t, recorder, &response)
	if response["status"] != "paused" {
		t.Fatalf("unexpected response: %#v", response)
	}
	item, err := server.adapterManager.GetDatabase().GetScriptRunLog(id)
	if err != nil {
		t.Fatal(err)
	}
	if item == nil || item.Status != "paused" {
		t.Fatalf("orphan running task should be paused: %#v", item)
	}
}

func TestPausePausingScriptTaskIsIdempotent(t *testing.T) {
	server := testServer(t)
	startedAt := time.Now()
	id, err := server.adapterManager.GetDatabase().SaveScriptRunLog(config.ScriptRunLog{PluginID: "plugin-a", UnionID: "union-a", ScriptPath: "scripts/a.js", Runtime: "nodejs", RuntimeProfile: "node18", RunMode: "manual", Status: "pausing", StartedAt: startedAt})
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/script-tasks/"+strconv.FormatInt(id, 10)+"?action=pause", nil)
	server.handleScriptTaskDetail(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]interface{}
	decodeUnifiedResponseData(t, recorder, &response)
	if response["message"] != "脚本任务暂停请求已发送" || response["status"] != "pausing" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestDeleteQueuedScriptTaskRemovesLog(t *testing.T) {
	server := testServer(t)
	startedAt := time.Now()
	id, err := server.adapterManager.GetDatabase().SaveScriptRunLog(config.ScriptRunLog{PluginID: "plugin-a", UnionID: "union-a", ScriptPath: "scripts/a.js", Runtime: "nodejs", RuntimeProfile: "node18", RunMode: "manual", Status: config.ScriptRunStatusQueued, StartedAt: startedAt})
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/script-tasks/"+strconv.FormatInt(id, 10), nil)
	server.handleScriptTaskDetail(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]interface{}
	decodeUnifiedResponseData(t, recorder, &response)
	if response["message"] != "脚本任务已删除" {
		t.Fatalf("unexpected response: %#v", response)
	}
	item, err := server.adapterManager.GetDatabase().GetScriptRunLog(id)
	if err != nil {
		t.Fatal(err)
	}
	if item != nil {
		t.Fatalf("queued task should be deleted, got %#v", item)
	}
}
