package config

import (
	"fmt"
	"testing"
	"time"

	"github.com/allbot/allbot/core/types"
)

func TestRecordPluginTriggerStatUsesPluginIDAndAccumulates(t *testing.T) {
	db := newPluginTriggerStatsTestDB(t)
	plugin := &types.Plugin{ID: "weather", Name: "天气插件", Trigger: "^天气"}
	msg := &types.Message{Platform: "qq", AdapterID: "1", Metadata: map[string]string{"adapter_remark": "主机器人"}}
	if err := db.RecordPluginTriggerStat(plugin, msg); err != nil {
		t.Fatalf("RecordPluginTriggerStat returned error: %v", err)
	}
	plugin.Name = "天气插件新版"
	plugin.Trigger = "^查天气"
	if err := db.RecordPluginTriggerStat(plugin, msg); err != nil {
		t.Fatalf("RecordPluginTriggerStat second call returned error: %v", err)
	}
	var pluginID, pluginName, triggerPattern, adapterName string
	var count int64
	if err := db.db.QueryRow(`SELECT plugin_id, plugin_name, trigger_pattern, adapter_name, count FROM plugin_trigger_stats`).Scan(&pluginID, &pluginName, &triggerPattern, &adapterName, &count); err != nil {
		t.Fatalf("query stat returned error: %v", err)
	}
	if pluginID != "weather" || pluginName != "天气插件新版" || triggerPattern != "^查天气" || adapterName != "主机器人" || count != 2 {
		t.Fatalf("unexpected stat row: id=%s name=%s trigger=%s adapter=%s count=%d", pluginID, pluginName, triggerPattern, adapterName, count)
	}
}

func TestGetPluginTriggerTrendByDayFillsMissingDatesAndSorts(t *testing.T) {
	db := newPluginTriggerStatsTestDB(t)
	insertPluginTriggerStat(t, db, "2026-06-16", "alpha", "Alpha", 2)
	insertPluginTriggerStat(t, db, "2026-06-17", "beta", "Beta", 5)
	insertPluginTriggerStat(t, db, "2026-06-18", "alpha", "Alpha", 1)
	trend, err := db.GetPluginTriggerTrend("day", "2026-06-16", "2026-06-18", 8)
	if err != nil {
		t.Fatalf("GetPluginTriggerTrend returned error: %v", err)
	}
	if trend.Total != 8 || len(trend.Points) != 3 || trend.Points[0].Total != 2 || trend.Points[1].Total != 5 || trend.Points[2].Total != 1 {
		t.Fatalf("unexpected points: %+v", trend)
	}
	if len(trend.Plugins) != 2 || trend.Plugins[0].PluginID != "beta" || trend.Plugins[1].PluginID != "alpha" {
		t.Fatalf("unexpected plugin order: %+v", trend.Plugins)
	}
	if fmt.Sprint(trend.Plugins[0].Counts) != "[0 5 0]" || fmt.Sprint(trend.Plugins[1].Counts) != "[2 0 1]" {
		t.Fatalf("unexpected plugin counts: %+v", trend.Plugins)
	}
}

func TestGetPluginTriggerTrendByMonthAggregates(t *testing.T) {
	db := newPluginTriggerStatsTestDB(t)
	insertPluginTriggerStat(t, db, "2026-05-31", "alpha", "Alpha", 2)
	insertPluginTriggerStat(t, db, "2026-06-01", "alpha", "Alpha", 3)
	insertPluginTriggerStat(t, db, "2026-06-18", "beta", "Beta", 4)
	trend, err := db.GetPluginTriggerTrend("month", "2026-05", "2026-06", 8)
	if err != nil {
		t.Fatalf("GetPluginTriggerTrend returned error: %v", err)
	}
	if trend.Total != 9 || len(trend.Points) != 2 || trend.Points[0].Total != 2 || trend.Points[1].Total != 7 {
		t.Fatalf("unexpected monthly points: %+v", trend)
	}
	if len(trend.Plugins) != 2 || trend.Plugins[0].PluginID != "alpha" || fmt.Sprint(trend.Plugins[0].Counts) != "[2 3]" {
		t.Fatalf("unexpected monthly plugin counts: %+v", trend.Plugins)
	}
}

func TestGetPluginTriggerTrendRejectsLongRanges(t *testing.T) {
	db := newPluginTriggerStatsTestDB(t)
	if _, err := db.GetPluginTriggerTrend("day", "2026-06-01", "2026-06-30", 8); err == nil {
		t.Fatal("expected day range error")
	}
	if _, err := db.GetPluginTriggerTrend("month", "2025-01", "2026-06", 8); err == nil {
		t.Fatal("expected month range error")
	}
}

func TestGetPluginTriggerTrendLimitClamp(t *testing.T) {
	db := newPluginTriggerStatsTestDB(t)
	for i := 0; i < 13; i++ {
		insertPluginTriggerStat(t, db, "2026-06-18", fmt.Sprintf("plugin-%02d", i), fmt.Sprintf("插件%02d", i), int64(i+1))
	}
	defaultTrend, err := db.GetPluginTriggerTrend("day", "2026-06-18", "2026-06-18", 0)
	if err != nil {
		t.Fatalf("GetPluginTriggerTrend default returned error: %v", err)
	}
	if len(defaultTrend.Plugins) != 8 {
		t.Fatalf("default limit plugin count = %d, expected 8", len(defaultTrend.Plugins))
	}
	maxTrend, err := db.GetPluginTriggerTrend("day", "2026-06-18", "2026-06-18", 20)
	if err != nil {
		t.Fatalf("GetPluginTriggerTrend max returned error: %v", err)
	}
	if len(maxTrend.Plugins) != 12 {
		t.Fatalf("max limit plugin count = %d, expected 12", len(maxTrend.Plugins))
	}
}

func newPluginTriggerStatsTestDB(t *testing.T) *Database {
	t.Helper()
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func insertPluginTriggerStat(t *testing.T, db *Database, date, pluginID, pluginName string, count int64) {
	t.Helper()
	if _, err := db.db.Exec(`
		INSERT INTO plugin_trigger_stats (stat_date, stat_hour, plugin_id, plugin_name, trigger_pattern, platform, adapter_id, adapter_name, count, created_at, updated_at)
		VALUES (?, ?, ?, ?, '^test', 'qq', '1', '机器人', ?, ?, ?)
	`, date, 8, pluginID, pluginName, count, time.Now(), time.Now()); err != nil {
		t.Fatalf("insert plugin trigger stat returned error: %v", err)
	}
}
