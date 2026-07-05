package config

import (
	"database/sql"
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

func TestScriptRunLogUpsertSeparatesUnionIDForSameMode(t *testing.T) {
	database, err := NewDatabase(filepath.Join(t.TempDir(), "allbot.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	startedAt := time.Now()
	base := ScriptRunLog{PluginID: "plugin-a", ScriptPath: "scripts/task.js", Runtime: "nodejs", RuntimeProfile: "node18", RunMode: "current_user", Status: "running", StartedAt: startedAt, FinishedAt: startedAt}
	unionA := base
	unionA.UnionID = "union-a"
	id1, reused, err := database.UpsertScriptRunLog(unionA)
	if err != nil {
		t.Fatal(err)
	}
	if reused {
		t.Fatalf("first union insert should not be reused")
	}
	unionB := base
	unionB.UnionID = "union-b"
	id2, reused, err := database.UpsertScriptRunLog(unionB)
	if err != nil {
		t.Fatal(err)
	}
	if reused || id1 == id2 {
		t.Fatalf("different union IDs should create separate logs: id1=%d id2=%d reused=%v", id1, id2, reused)
	}
	id3, reused, err := database.UpsertScriptRunLog(unionA)
	if err != nil {
		t.Fatal(err)
	}
	if !reused || id3 != id1 {
		t.Fatalf("same union ID should reuse latest log: id1=%d id3=%d reused=%v", id1, id3, reused)
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

func TestScriptRunStatsSurviveLogCleanup(t *testing.T) {
	database, err := NewDatabase(filepath.Join(t.TempDir(), "allbot.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	startedAt := time.Now().AddDate(0, 0, -3)
	successID, err := database.SaveScriptRunLog(ScriptRunLog{PluginID: "plugin-a", ScriptPath: "scripts/a.js", Runtime: "nodejs", RuntimeProfile: "node18", RunMode: "manual", Status: "running", StartedAt: startedAt, FinishedAt: startedAt})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.UpdateScriptRunLog(successID, "success", "ok", "", startedAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	failedID, err := database.SaveScriptRunLog(ScriptRunLog{PluginID: "plugin-a", ScriptPath: "scripts/b.js", Runtime: "nodejs", RuntimeProfile: "node18", RunMode: "manual", Status: "running", StartedAt: startedAt, FinishedAt: startedAt})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.UpdateScriptRunLog(failedID, "failed", "", "boom", startedAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if deleted, err := database.CleanupScriptRunLogs(1); err != nil || deleted != 2 {
		t.Fatalf("cleanup deleted=%d err=%v", deleted, err)
	}
	summary, err := database.GetScriptRunStatsSummary()
	if err != nil {
		t.Fatal(err)
	}
	if summary.Total != 2 || summary.Success != 1 || summary.Failed != 1 || summary.Running != 0 || summary.Pausing != 0 {
		t.Fatalf("unexpected summary after cleanup: %+v", summary)
	}
}

func TestScriptRunStatsDoNotDoubleCountRepeatedTerminalUpdate(t *testing.T) {
	database, err := NewDatabase(filepath.Join(t.TempDir(), "allbot.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	startedAt := time.Now()
	id, err := database.SaveScriptRunLog(ScriptRunLog{PluginID: "plugin-a", ScriptPath: "scripts/a.js", Runtime: "nodejs", RuntimeProfile: "node18", RunMode: "manual", Status: "running", StartedAt: startedAt, FinishedAt: startedAt})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.UpdateScriptRunLog(id, "failed", "", "boom", startedAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := database.UpdateScriptRunLog(id, "failed", "", "boom again", startedAt.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	summary, err := database.GetScriptRunStatsSummary()
	if err != nil {
		t.Fatal(err)
	}
	if summary.Total != 1 || summary.Failed != 1 || summary.Success != 0 {
		t.Fatalf("unexpected repeated terminal summary: %+v", summary)
	}
}

func TestScriptRunStatsCountTerminalTransitionsAndLiveStatuses(t *testing.T) {
	database, err := NewDatabase(filepath.Join(t.TempDir(), "allbot.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	startedAt := time.Now()
	successID, err := database.SaveScriptRunLog(ScriptRunLog{PluginID: "plugin-a", ScriptPath: "scripts/a.js", Runtime: "nodejs", RuntimeProfile: "node18", RunMode: "manual", Status: "running", StartedAt: startedAt, FinishedAt: startedAt})
	if err != nil {
		t.Fatal(err)
	}
	failedID, err := database.SaveScriptRunLog(ScriptRunLog{PluginID: "plugin-a", ScriptPath: "scripts/b.js", Runtime: "nodejs", RuntimeProfile: "node18", RunMode: "manual", Status: "running", StartedAt: startedAt, FinishedAt: startedAt})
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.SaveScriptRunLog(ScriptRunLog{PluginID: "plugin-a", ScriptPath: "scripts/c.js", Runtime: "nodejs", RuntimeProfile: "node18", RunMode: "manual", Status: "pausing", StartedAt: startedAt, FinishedAt: startedAt})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.UpdateScriptRunLog(successID, "success", "ok", "", startedAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := database.UpdateScriptRunLog(failedID, "error", "", "boom", startedAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	summary, err := database.GetScriptRunStatsSummary()
	if err != nil {
		t.Fatal(err)
	}
	if summary.Total != 3 || summary.Today != 3 || summary.Success != 1 || summary.Failed != 1 || summary.Running != 0 || summary.Pausing != 1 {
		t.Fatalf("unexpected terminal/live summary: %+v", summary)
	}
}

func TestBackfillScriptRunStatsIsIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "allbot.db")
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rawDB.Exec(`
		CREATE TABLE script_run_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			plugin_id TEXT NOT NULL,
			union_id TEXT NOT NULL DEFAULT '',
			script_path TEXT NOT NULL DEFAULT '',
			runtime TEXT NOT NULL DEFAULT '',
			runtime_profile TEXT NOT NULL DEFAULT '',
			run_mode TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT '',
			run_total INTEGER NOT NULL DEFAULT 0,
			failed_total INTEGER NOT NULL DEFAULT 0,
			output TEXT NOT NULL DEFAULT '',
			error TEXT NOT NULL DEFAULT '',
			started_at DATETIME NOT NULL,
			finished_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`); err != nil {
		t.Fatal(err)
	}
	startedAt := time.Now().AddDate(0, 0, -1).Truncate(time.Hour)
	if _, err := rawDB.Exec(`
		INSERT INTO script_run_logs (plugin_id, script_path, runtime, runtime_profile, run_mode, status, run_total, failed_total, started_at, finished_at)
		VALUES ('plugin-a', 'scripts/a.js', 'nodejs', 'node18', 'manual', 'success', 3, 1, ?, ?)
	`, startedAt, startedAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := rawDB.Close(); err != nil {
		t.Fatal(err)
	}

	database, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := database.GetScriptRunStatsSummary()
	if err != nil {
		t.Fatal(err)
	}
	if summary.Total != 3 || summary.Success != 2 || summary.Failed != 1 {
		t.Fatalf("unexpected backfilled summary: %+v", summary)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	database, err = NewDatabase(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	summary, err = database.GetScriptRunStatsSummary()
	if err != nil {
		t.Fatal(err)
	}
	if summary.Total != 3 || summary.Success != 2 || summary.Failed != 1 {
		t.Fatalf("backfill should be idempotent, got %+v", summary)
	}
}
