package config

import "testing"

func TestPluginTemplateMetadataSaveGetUpsertDelete(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	first := &PluginTemplateMetadata{
		PluginID:        "demo",
		Template:        "nodejs_account_ql",
		TemplateVersion: "3.0.0",
		Runtime:         "nodejs",
		Structure:       "account_ql",
		Metadata:        map[string]interface{}{"env_name": "DEMO_CK"},
	}
	if err := db.SavePluginTemplateMetadata(first); err != nil {
		t.Fatal(err)
	}
	stored, err := db.GetPluginTemplateMetadata("demo")
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil || stored.Template != "nodejs_account_ql" || stored.Metadata["env_name"] != "DEMO_CK" {
		t.Fatalf("unexpected stored metadata: %#v", stored)
	}

	updated := &PluginTemplateMetadata{
		PluginID:        "demo",
		Template:        "python_account_ql",
		TemplateVersion: "3.0.1",
		Runtime:         "python",
		Structure:       "account_ql",
		Metadata:        map[string]interface{}{"env_name": "PY_CK"},
	}
	if err := db.SavePluginTemplateMetadata(updated); err != nil {
		t.Fatal(err)
	}
	stored, err = db.GetPluginTemplateMetadata("demo")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Template != "python_account_ql" || stored.TemplateVersion != "3.0.1" || stored.Runtime != "python" || stored.Metadata["env_name"] != "PY_CK" {
		t.Fatalf("unexpected upsert metadata: %#v", stored)
	}

	if err := db.DeletePluginTemplateMetadata("demo"); err != nil {
		t.Fatal(err)
	}
	stored, err = db.GetPluginTemplateMetadata("demo")
	if err != nil {
		t.Fatal(err)
	}
	if stored != nil {
		t.Fatalf("expected metadata deleted, got %#v", stored)
	}
}
