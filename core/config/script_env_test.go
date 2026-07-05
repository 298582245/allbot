package config

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestScriptEnvVarsSaveListMapDelete(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()

	saved, err := db.SaveScriptEnvVar(&ScriptEnvVar{Name: "API_TOKEN", Value: "secret", Remark: "接口令牌", Enabled: true, Pinned: true})
	if err != nil {
		t.Fatalf("SaveScriptEnvVar returned error: %v", err)
	}
	if saved.ID == 0 || saved.Name != "API_TOKEN" || !saved.Enabled || !saved.Pinned {
		t.Fatalf("unexpected saved item: %#v", saved)
	}
	if _, err := db.SaveScriptEnvVar(&ScriptEnvVar{Name: "API_TOKEN", Value: "second", Enabled: true}); err != nil {
		t.Fatalf("SaveScriptEnvVar duplicate name returned error: %v", err)
	}
	if _, err := db.SaveScriptEnvVar(&ScriptEnvVar{Name: "API_TOKEN", Value: "secret", Enabled: true}); err == nil {
		t.Fatal("expected duplicate name+value create error")
	}
	if _, err := db.SaveScriptEnvVar(&ScriptEnvVar{Name: "DISABLED", Value: "skip", Enabled: false}); err != nil {
		t.Fatalf("SaveScriptEnvVar disabled returned error: %v", err)
	}
	items, err := db.ListScriptEnvVars(ScriptEnvQuery{Keyword: "TOKEN"})
	if err != nil {
		t.Fatalf("ListScriptEnvVars returned error: %v", err)
	}
	if len(items) != 2 || items[0].Value != "secret" || items[1].Value != "second" {
		t.Fatalf("unexpected list: %#v", items)
	}
	saved.Pinned = false
	saved.Value = "updated"
	updated, err := db.SaveScriptEnvVar(saved)
	if err != nil {
		t.Fatalf("SaveScriptEnvVar update returned error: %v", err)
	}
	if updated.Value != "updated" || updated.Pinned {
		t.Fatalf("unexpected updated item: %#v", updated)
	}
	updated.Value = "second"
	if _, err := db.SaveScriptEnvVar(updated); err == nil {
		t.Fatal("expected duplicate name+value update error")
	}
	env, err := db.ScriptEnvMap(nil)
	if err != nil {
		t.Fatalf("ScriptEnvMap returned error: %v", err)
	}
	if env["API_TOKEN"] != "second&updated" {
		t.Fatalf("API_TOKEN should join duplicate names, got map: %#v", env)
	}
	if _, ok := env["DISABLED"]; ok {
		t.Fatalf("disabled env should not be returned: %#v", env)
	}
	env, err = db.ScriptEnvMap([]string{"API_TOKEN"})
	if err != nil {
		t.Fatalf("ScriptEnvMap filtered returned error: %v", err)
	}
	if len(env) != 1 || env["API_TOKEN"] != "second&updated" {
		t.Fatalf("unexpected filtered map: %#v", env)
	}
	if err := db.DeleteScriptEnvVar(saved.ID); err != nil {
		t.Fatalf("DeleteScriptEnvVar returned error: %v", err)
	}
	item, err := db.GetScriptEnvVar(saved.ID)
	if err != nil {
		t.Fatalf("GetScriptEnvVar returned error: %v", err)
	}
	if item != nil {
		t.Fatalf("expected deleted item nil, got %#v", item)
	}
}

func TestScriptEnvVarsListPinnedThenIDDesc(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()

	fixtures := []ScriptEnvVar{
		{Name: "C_PINNED", Value: "1", Enabled: true, Pinned: true},
		{Name: "B_NORMAL", Value: "2", Enabled: true},
		{Name: "A_PINNED", Value: "3", Enabled: true, Pinned: true},
	}
	for i := range fixtures {
		if _, err := db.SaveScriptEnvVar(&fixtures[i]); err != nil {
			t.Fatalf("SaveScriptEnvVar fixture %d returned error: %v", i, err)
		}
	}

	items, err := db.ListScriptEnvVars(ScriptEnvQuery{})
	if err != nil {
		t.Fatalf("ListScriptEnvVars returned error: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("unexpected list length: %#v", items)
	}
	want := []string{"A_PINNED", "C_PINNED", "B_NORMAL"}
	for i, name := range want {
		if items[i].Name != name {
			t.Fatalf("items[%d].Name = %q, want %q; list=%#v", i, items[i].Name, name, items)
		}
	}
	if !items[0].Pinned || !items[1].Pinned || items[2].Pinned {
		t.Fatalf("pinned items should be listed before normal items: %#v", items)
	}
	if items[0].ID <= items[1].ID {
		t.Fatalf("pinned items should be sorted by id desc: %#v", items)
	}
}

func TestScriptEnvVarsBatchAndImport(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()

	first, err := db.SaveScriptEnvVar(&ScriptEnvVar{Name: "A", Value: "1", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	second, err := db.SaveScriptEnvVar(&ScriptEnvVar{Name: "B", Value: "2", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if affected, err := db.UpdateScriptEnvVarsEnabled([]int64{first.ID, second.ID}, false); err != nil || affected != 2 {
		t.Fatalf("UpdateScriptEnvVarsEnabled affected=%d err=%v", affected, err)
	}
	if affected, err := db.UpdateScriptEnvVarsPinned([]int64{first.ID, second.ID}, true); err != nil || affected != 2 {
		t.Fatalf("UpdateScriptEnvVarsPinned affected=%d err=%v", affected, err)
	}
	items, err := db.ListScriptEnvVars(ScriptEnvQuery{})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.Enabled || !item.Pinned {
			t.Fatalf("unexpected batch updated item: %#v", item)
		}
	}
	if affected, err := db.DeleteScriptEnvVars([]int64{first.ID}); err != nil || affected != 1 {
		t.Fatalf("DeleteScriptEnvVars affected=%d err=%v", affected, err)
	}
	if affected, err := db.ImportScriptEnvVars([]ScriptEnvImportItem{{Name: "C", Value: "3", Remark: "导入"}, {Name: "C", Value: "4"}}); err != nil || affected != 2 {
		t.Fatalf("ImportScriptEnvVars affected=%d err=%v", affected, err)
	}
	if _, err := db.ImportScriptEnvVars([]ScriptEnvImportItem{{Name: "D", Value: "5"}, {Name: "D", Value: "5"}}); err == nil {
		t.Fatal("expected duplicate import file error")
	}
	if _, err := db.ImportScriptEnvVars([]ScriptEnvImportItem{{Name: "C", Value: "3"}}); err == nil {
		t.Fatal("expected duplicate database import error")
	}
}

func TestScriptEnvVarsMigratesPinnedColumn(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "allbot.db")
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	if _, err := raw.Exec(`
		CREATE TABLE script_env_vars (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			value TEXT NOT NULL DEFAULT '',
			remark TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO script_env_vars (name, value, remark, enabled, created_at, updated_at)
		VALUES ('LEGACY_ENV', 'legacy', '', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
	`); err != nil {
		raw.Close()
		t.Fatalf("create legacy table returned error: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("raw.Close returned error: %v", err)
	}

	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()

	items, err := db.ListScriptEnvVars(ScriptEnvQuery{})
	if err != nil {
		t.Fatalf("ListScriptEnvVars returned error: %v", err)
	}
	if len(items) != 1 || items[0].Name != "LEGACY_ENV" || items[0].Pinned {
		t.Fatalf("unexpected migrated item: %#v", items)
	}
	if _, err := db.db.Exec(`SELECT pinned FROM script_env_vars LIMIT 1`); err != nil {
		t.Fatalf("pinned column missing after migration: %v", err)
	}
	if _, err := db.SaveScriptEnvVar(&ScriptEnvVar{Name: "LEGACY_ENV", Value: "second", Enabled: true}); err != nil {
		t.Fatalf("migrated table should allow duplicate name: %v", err)
	}
	if _, err := db.SaveScriptEnvVar(&ScriptEnvVar{Name: "LEGACY_ENV", Value: "second", Enabled: true}); err == nil {
		t.Fatal("migrated table should reject duplicate name+value")
	}
}

func TestScriptEnvVarRejectsInvalidName(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()

	if _, err := db.SaveScriptEnvVar(&ScriptEnvVar{Name: "BAD=NAME", Value: "x", Enabled: true}); err == nil {
		t.Fatal("expected invalid name error")
	}
}
