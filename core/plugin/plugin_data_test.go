package plugin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/allbot/allbot/core/deps"
	"github.com/allbot/allbot/core/types"
)

func TestPluginDataMigratesAndSurvivesManifestReplacement(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "demo")
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "main.js"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := types.PluginConfig{
		Name: "demo", Runtime: "nodejs", Entry: "main.js", Trigger: "^demo$", Enabled: true,
		UserConfigSchema: []types.PluginUserConfigField{
			{Key: "token", Default: "author-token"}, {Key: "flag", Default: true},
			{Key: "count", Default: 10}, {Key: "removed", Default: "old"},
		},
		UserConfig: map[string]interface{}{"token": "", "flag": false, "count": 0, "removed": "saved"},
	}
	write := func() []byte {
		t.Helper()
		data, err := json.Marshal(manifest)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "plugin.json"), data, 0644); err != nil {
			t.Fatal(err)
		}
		return data
	}
	original := write()
	manager := NewManager(root, deps.NewManager(t.TempDir()))
	if _, err := manager.ValidatePluginConfig(path, "demo"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(path, PluginDataFileName)); !os.IsNotExist(err) {
		t.Fatalf("validation must not migrate data: %v", err)
	}
	if _, err := manager.LoadPlugin(path); err != nil {
		t.Fatal(err)
	}
	dataBefore, err := os.ReadFile(filepath.Join(path, PluginDataFileName))
	if err != nil {
		t.Fatal(err)
	}
	manifestBefore, err := os.ReadFile(filepath.Join(path, "plugin.json"))
	if err != nil || string(manifestBefore) != string(original) {
		t.Fatalf("migration must preserve manifest: %v", err)
	}
	manifest.Version = "2.0.0"
	manifest.UserConfig = nil
	manifest.UserConfigSchema = manifest.UserConfigSchema[:3]
	manifest.UserConfigSchema = append(manifest.UserConfigSchema, types.PluginUserConfigField{Key: "added", Default: "new"})
	write()
	if err := manager.ReloadPlugin("demo"); err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{"token": "", "flag": false, "count": float64(0), "added": "new"}
	if got := manager.GetPlugin("demo").Plugin.UserConfig; !reflect.DeepEqual(got, want) {
		t.Fatalf("reloaded config = %#v, want %#v", got, want)
	}
	document, err := ReadPluginConfigDocument(path)
	if err != nil || !reflect.DeepEqual(document["user_config"], want) {
		t.Fatalf("document = %#v, error = %v", document, err)
	}
	dataAfter, err := os.ReadFile(filepath.Join(path, PluginDataFileName))
	if err != nil || string(dataAfter) != string(dataBefore) {
		t.Fatalf("reload must not replace existing data: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, PluginDataFileName), []byte("invalid"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := manager.ReloadPlugin("demo"); err == nil {
		t.Fatal("invalid plugin.data must return an error")
	}
}
