package config

import "testing"

func TestScriptEnvVarsSaveListMapDelete(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()

	saved, err := db.SaveScriptEnvVar(&ScriptEnvVar{Name: "API_TOKEN", Value: "secret", Remark: "接口令牌", Enabled: true})
	if err != nil {
		t.Fatalf("SaveScriptEnvVar returned error: %v", err)
	}
	if saved.ID == 0 || saved.Name != "API_TOKEN" || !saved.Enabled {
		t.Fatalf("unexpected saved item: %#v", saved)
	}
	if _, err := db.SaveScriptEnvVar(&ScriptEnvVar{Name: "DISABLED", Value: "skip", Enabled: false}); err != nil {
		t.Fatalf("SaveScriptEnvVar disabled returned error: %v", err)
	}
	items, err := db.ListScriptEnvVars(ScriptEnvQuery{Keyword: "TOKEN"})
	if err != nil {
		t.Fatalf("ListScriptEnvVars returned error: %v", err)
	}
	if len(items) != 1 || items[0].Name != "API_TOKEN" {
		t.Fatalf("unexpected list: %#v", items)
	}
	env, err := db.ScriptEnvMap(nil)
	if err != nil {
		t.Fatalf("ScriptEnvMap returned error: %v", err)
	}
	if env["API_TOKEN"] != "secret" {
		t.Fatalf("API_TOKEN missing from map: %#v", env)
	}
	if _, ok := env["DISABLED"]; ok {
		t.Fatalf("disabled env should not be returned: %#v", env)
	}
	env, err = db.ScriptEnvMap([]string{"API_TOKEN"})
	if err != nil {
		t.Fatalf("ScriptEnvMap filtered returned error: %v", err)
	}
	if len(env) != 1 || env["API_TOKEN"] != "secret" {
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
