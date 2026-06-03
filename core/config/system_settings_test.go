package config

import "testing"

func TestNewDatabaseRemovesWebPortSettings(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()

	if err := db.SetSetting("web.port", "3100", "旧端口配置"); err != nil {
		t.Fatalf("SetSetting web.port returned error: %v", err)
	}
	if err := db.SetSetting("web_port", "3200", "旧端口配置"); err != nil {
		t.Fatalf("SetSetting web_port returned error: %v", err)
	}
	if err := ensureDefaultSystemSettings(db.db); err != nil {
		t.Fatalf("ensureDefaultSystemSettings returned error: %v", err)
	}

	items, err := db.getSettingsMap()
	if err != nil {
		t.Fatalf("getSettingsMap returned error: %v", err)
	}
	if _, ok := items["web.port"]; ok {
		t.Fatal("system_settings still contains web.port")
	}
	if _, ok := items["web_port"]; ok {
		t.Fatal("system_settings still contains web_port")
	}
}

func TestSaveSystemSettingsIgnoresWebPort(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()

	settings := &SystemSettings{
		AdminUsername:   "admin",
		PlatformAdmins:  []PlatformAdmin{},
		AutoRefresh:     true,
		RefreshInterval: 5,
		PluginDir:       "./plugins",
		AutoLoadPlugins: true,
		PointsUnit:      "积分",
	}
	if err := db.SaveSystemSettings(settings); err != nil {
		t.Fatalf("SaveSystemSettings returned error: %v", err)
	}

	items, err := db.getSettingsMap()
	if err != nil {
		t.Fatalf("getSettingsMap returned error: %v", err)
	}
	if _, ok := items["web.port"]; ok {
		t.Fatal("SaveSystemSettings wrote web.port")
	}
	if _, ok := items["web_port"]; ok {
		t.Fatal("SaveSystemSettings wrote web_port")
	}
}
