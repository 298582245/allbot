package config

import (
	"path/filepath"
	"testing"
	"time"
)

func TestScriptRunLogSavesAndFiltersRuntimeProfile(t *testing.T) {
	database, err := NewDatabase(filepath.Join(t.TempDir(), "allbot.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	startedAt := time.Now()
	id, err := database.SaveScriptRunLog(ScriptRunLog{PluginID: "plugin-a", UnionID: "union-a", ScriptPath: "scripts/task.js", Runtime: "nodejs", RuntimeProfile: "node18", RunMode: "manual", Status: "success", Output: "ok", StartedAt: startedAt, FinishedAt: startedAt})
	if err != nil {
		t.Fatal(err)
	}
	item, err := database.GetScriptRunLog(id)
	if err != nil {
		t.Fatal(err)
	}
	if item == nil || item.RuntimeProfile != "node18" {
		t.Fatalf("unexpected runtime profile: %#v", item)
	}

	items, err := database.ListScriptRunLogs(ScriptRunLogFilter{RuntimeProfile: "node18"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != id {
		t.Fatalf("expected node18 item, got %#v", items)
	}
	items, err = database.ListScriptRunLogs(ScriptRunLogFilter{RuntimeProfile: "python310"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty result, got %#v", items)
	}
}

func TestScriptRunLogUpsertSeparatesRuntimeProfiles(t *testing.T) {
	database, err := NewDatabase(filepath.Join(t.TempDir(), "allbot.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	startedAt := time.Now()
	base := ScriptRunLog{PluginID: "plugin-a", UnionID: "union-a", ScriptPath: "scripts/task.js", Runtime: "nodejs", RunMode: "manual", Status: "running", StartedAt: startedAt, FinishedAt: startedAt}
	node18 := base
	node18.RuntimeProfile = "node18"
	id1, reused, err := database.UpsertScriptRunLog(node18)
	if err != nil {
		t.Fatal(err)
	}
	if reused {
		t.Fatalf("first insert should not be reused")
	}
	node20 := base
	node20.RuntimeProfile = "node20"
	id2, reused, err := database.UpsertScriptRunLog(node20)
	if err != nil {
		t.Fatal(err)
	}
	if reused || id1 == id2 {
		t.Fatalf("different profiles should create separate logs: id1=%d id2=%d reused=%v", id1, id2, reused)
	}
	id3, reused, err := database.UpsertScriptRunLog(node18)
	if err != nil {
		t.Fatal(err)
	}
	if !reused || id3 != id1 {
		t.Fatalf("same profile should reuse latest log: id1=%d id3=%d reused=%v", id1, id3, reused)
	}
}

func TestFindRunningScriptRunLogSeparatesRuntimeProfiles(t *testing.T) {
	database, err := NewDatabase(filepath.Join(t.TempDir(), "allbot.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	startedAt := time.Now()
	_, err = database.SaveScriptRunLog(ScriptRunLog{PluginID: "plugin-a", UnionID: "union-a", ScriptPath: "scripts/task.js", Runtime: "nodejs", RuntimeProfile: "node18", RunMode: "single_account", Status: "running", StartedAt: startedAt, FinishedAt: startedAt})
	if err != nil {
		t.Fatal(err)
	}
	missing, err := database.FindRunningScriptRunLog("plugin-a", "scripts/task.js", "single_account", "union-a", "node20")
	if err != nil {
		t.Fatal(err)
	}
	if missing != nil {
		t.Fatalf("unexpected running log for node20: %#v", missing)
	}
	item, err := database.FindRunningScriptRunLog("plugin-a", "scripts/task.js", "single_account", "union-a", "node18")
	if err != nil {
		t.Fatal(err)
	}
	if item == nil || item.RuntimeProfile != "node18" {
		t.Fatalf("expected node18 running log, got %#v", item)
	}
}
