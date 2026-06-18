package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if response.Total != 1 || len(response.Items) != 1 || response.Items[0].RuntimeProfile != "node18" {
		t.Fatalf("unexpected response: %#v", response)
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
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if response.RuntimeProfile != "node18" {
		t.Fatalf("runtime_profile missing: %#v", response)
	}
}
