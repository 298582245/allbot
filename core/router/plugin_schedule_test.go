package router

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/allbot/allbot/core/adapter"
	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/deps"
	plugincore "github.com/allbot/allbot/core/plugin"
	"github.com/allbot/allbot/core/types"
)

func TestAccountSchedulesPersistSkipAndForceRefresh(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node unavailable")
	}
	sdk, err := filepath.Abs("../../sdk/nodejs/account_ql_plugin.js")
	if err != nil {
		t.Fatal(err)
	}
	sdkJSON, _ := json.Marshal(sdk)
	root := t.TempDir()
	pluginPath := filepath.Join(root, "demo")
	if err := os.Mkdir(pluginPath, 0755); err != nil {
		t.Fatal(err)
	}
	entry := "const {createAccountQLPlugin}=require(" + string(sdkJSON) + "); createAccountQLPlugin({prefix:'演示',tableName:'demo_accounts',envName:'DEMO_COOKIE',schedules:{run:{taskKey:'run',cron:'0 8 * * *',content:'演示运行'}}});"
	if err := os.WriteFile(filepath.Join(pluginPath, "main.js"), []byte(entry), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := types.PluginConfig{Name: "演示", Runtime: "nodejs", Entry: "main.js", Enabled: true, Trigger: "^演示", TemplateSource: map[string]interface{}{"template": "nodejs_account_ql"}, UserConfigSchema: []types.PluginUserConfigField{{Key: "token", Default: "keep"}}}
	manifestJSON, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(pluginPath, "plugin.json"), manifestJSON, 0644); err != nil {
		t.Fatal(err)
	}
	dm := deps.NewManager(t.TempDir())
	if _, err := dm.SaveRuntimeProfiles([]deps.RuntimeProfile{{ID: "node", Name: "Node", Runtime: "nodejs", Executable: "node", Enabled: true, Default: true}}); err != nil {
		t.Fatal(err)
	}
	m := plugincore.NewManager(root, dm)
	if err := m.ReloadPlugin("demo"); err != nil {
		t.Fatal(err)
	}
	db, _, _ := newSendPluginMessageTestDB(t)
	defer db.Close()
	settings, err := db.GetSystemSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.PlatformAdmins = []config.PlatformAdmin{{Platform: "qq", UserID: "admin"}}
	if err := db.SaveSystemSettings(settings); err != nil {
		t.Fatal(err)
	}
	r := NewRouter(nil)
	r.SetPluginManager(m)
	r.SetDatabase(db)
	adminLookups := 0
	r.SetAdapterGetter(func(platform string) adapter.Adapter {
		adminLookups++
		return &keywordReplyFakeAdapter{}
	})
	if err := r.RegisterPlugin(m.GetPlugin("demo").Plugin); err != nil {
		t.Fatal(err)
	}
	msg := &types.Message{Platform: "qq", UserID: "user", Content: "演示管理"}
	if err := r.callPlugin(r.GetPlugin("demo"), msg); err != nil {
		t.Fatal(err)
	}
	document, err := plugincore.ReadPluginConfigDocument(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	values := document["user_config"].(map[string]interface{})
	if values["schedule_is_register"] != true || values["token"] != "keep" {
		t.Fatalf("registration not persisted: %#v", values)
	}
	if r.GetPlugin("demo").UserConfig["schedule_is_register"] != true {
		t.Fatal("router still holds stale user config")
	}
	// Removing administrators makes any unexpected registration visible.
	settings.PlatformAdmins = nil
	if err := db.SaveSystemSettings(settings); err != nil {
		t.Fatal(err)
	}
	before := adminLookups
	if err := r.callPlugin(r.GetPlugin("demo"), msg); err != nil {
		t.Fatal(err)
	}
	// Adapter resolution for the reply is the only remaining lookup.
	if adminLookups != before+1 {
		t.Fatalf("registered plugin still queries admins: before=%d after=%d", before, adminLookups)
	}
	if err := r.RefreshPluginSchedules("demo"); err == nil {
		t.Fatal("force refresh without administrators must not report success, even with a persisted true flag")
	}
	settings.PlatformAdmins = []config.PlatformAdmin{{Platform: "qq", UserID: "admin"}}
	if err := db.SaveSystemSettings(settings); err != nil {
		t.Fatal(err)
	}
	tasks, err := db.ListScheduledTasks()
	if err != nil || len(tasks) != 1 {
		t.Fatalf("tasks=%#v error=%v", tasks, err)
	}
	if err := db.DeleteScheduledTask(tasks[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := m.ReloadPlugin("demo"); err != nil {
		t.Fatal(err)
	}
	if err := r.RegisterPlugin(m.GetPlugin("demo").Plugin); err != nil {
		t.Fatal(err)
	}
	if err := r.RefreshPluginSchedules("demo"); err != nil {
		t.Fatal(err)
	}
	tasks, err = db.ListScheduledTasks()
	if err != nil || len(tasks) != 1 {
		t.Fatalf("force refresh did not recreate task: tasks=%#v error=%v", tasks, err)
	}
}
