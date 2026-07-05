package config

import (
	"testing"

	"github.com/allbot/allbot/core/types"
)

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

func TestEnsureDefaultSystemSettingsKeepsSavedPaymentSettings(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()

	if err := db.SavePointsPerRMB(88); err != nil {
		t.Fatalf("SavePointsPerRMB returned error: %v", err)
	}
	if err := ensureDefaultSystemSettings(db.db); err != nil {
		t.Fatalf("ensureDefaultSystemSettings returned error: %v", err)
	}
	pointsPerRMB, err := db.GetPointsPerRMB()
	if err != nil {
		t.Fatalf("GetPointsPerRMB returned error: %v", err)
	}
	if pointsPerRMB != 88 {
		t.Fatalf("expected saved points_per_rmb 88, got %d", pointsPerRMB)
	}
}

func TestLogRetentionDaysDefaultsToZero(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()

	days, err := db.GetLogRetentionDays()
	if err != nil {
		t.Fatalf("GetLogRetentionDays returned error: %v", err)
	}
	if days != 0 {
		t.Fatalf("expected default retention 0, got %d", days)
	}
}

func TestSaveLogRetentionDays(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()

	if err := db.SaveLogRetentionDays(30); err != nil {
		t.Fatalf("SaveLogRetentionDays returned error: %v", err)
	}
	days, err := db.GetLogRetentionDays()
	if err != nil {
		t.Fatalf("GetLogRetentionDays returned error: %v", err)
	}
	if days != 30 {
		t.Fatalf("expected retention 30, got %d", days)
	}
}

func TestSaveLogRetentionDaysRejectsInvalidValues(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()

	if err := db.SaveLogRetentionDays(-1); err == nil {
		t.Fatal("expected negative retention to fail")
	}
	if err := db.SaveLogRetentionDays(3651); err == nil {
		t.Fatal("expected too large retention to fail")
	}
}

func TestNormalizeAccessControlConfigKeepsUnionIDs(t *testing.T) {
	config := NormalizeAccessControlConfig(types.AccessControlConfig{
		WhitelistUnionIDs: []string{"union-1", "union-1", ""},
		BlockedUnionIDs:   []string{"union-2", ""},
	})
	if len(config.WhitelistUnionIDs) != 1 || config.WhitelistUnionIDs[0] != "union-1" {
		t.Fatalf("unexpected whitelist union ids: %#v", config.WhitelistUnionIDs)
	}
	if len(config.BlockedUnionIDs) != 1 || config.BlockedUnionIDs[0] != "union-2" {
		t.Fatalf("unexpected blocked union ids: %#v", config.BlockedUnionIDs)
	}
}

func TestPlatformAdminSupportsUnionID(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()

	account, err := db.EnsureUserAccount("qq", "user-1")
	if err != nil {
		t.Fatalf("EnsureUserAccount returned error: %v", err)
	}
	settings, err := db.GetSystemSettings()
	if err != nil {
		t.Fatalf("GetSystemSettings returned error: %v", err)
	}
	settings.PlatformAdmins = []PlatformAdmin{{UnionID: account.UnionID}}
	if err := db.SaveSystemSettings(settings); err != nil {
		t.Fatalf("SaveSystemSettings returned error: %v", err)
	}
	if !db.IsPlatformAdmin("qq", "user-1") {
		t.Fatal("expected union_id platform admin to match bound platform user")
	}
	if db.IsPlatformAdmin("qq", "user-2") {
		t.Fatal("expected unrelated user not to be platform admin")
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
