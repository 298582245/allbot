package router

import (
	"strings"
	"testing"

	"github.com/allbot/allbot/core/config"
	plugincore "github.com/allbot/allbot/core/plugin"
)

func TestExecutePluginDBActionProtectsSystemSettings(t *testing.T) {
	database, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	read := executePluginDBAction(database, "demo", plugincore.PluginDBAction{
		Action: "db_query",
		Query:  config.PluginDBQuery{Table: "system_settings"},
	})
	if !read.Success || read.Data != "123456" {
		t.Fatalf("protected read = %#v", read)
	}

	for _, action := range []string{"db_create_table", "db_insert", "db_update", "db_delete", "db_clear"} {
		result := executePluginDBAction(database, "demo", plugincore.PluginDBAction{Action: action, Table: "system_settings"})
		if result.Success || !strings.Contains(result.Error, "system_settings") {
			t.Fatalf("protected %s = %#v", action, result)
		}
	}
}
