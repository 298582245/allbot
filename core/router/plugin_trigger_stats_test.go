package router

import (
	"fmt"
	"testing"
	"time"

	"github.com/allbot/allbot/core/adapter"
	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/session"
	"github.com/allbot/allbot/core/types"
)

func TestPluginTriggerStatsSkipsUnregisteredUser(t *testing.T) {
	db, router := newPluginTriggerStatsRouter(t)
	registerPluginForStats(t, router, &types.Plugin{ID: "demo", Name: "示例插件", Trigger: "^demo", Enabled: true})
	router.HandleMessage(pluginTriggerStatsMessage("demo"))
	assertPluginTriggerTotal(t, db, "demo", 0)
}

func TestPluginTriggerStatsRecordsRegisteredUser(t *testing.T) {
	db, router := newPluginTriggerStatsRouter(t)
	if _, err := db.EnsureUserAccount("qq", "user-1"); err != nil {
		t.Fatalf("EnsureUserAccount returned error: %v", err)
	}
	registerPluginForStats(t, router, &types.Plugin{ID: "demo", Name: "示例插件", Trigger: "^demo", Enabled: true})
	router.HandleMessage(pluginTriggerStatsMessage("demo"))
	assertPluginTriggerTotal(t, db, "demo", 1)
}

func TestPluginTriggerStatsRecordsOnlySelectedPlugin(t *testing.T) {
	db, router := newPluginTriggerStatsRouter(t)
	if _, err := db.EnsureUserAccount("qq", "user-1"); err != nil {
		t.Fatalf("EnsureUserAccount returned error: %v", err)
	}
	registerPluginForStats(t, router, &types.Plugin{ID: "low", Name: "低优先级", Trigger: "^demo", Priority: 1, Enabled: true})
	registerPluginForStats(t, router, &types.Plugin{ID: "high", Name: "高优先级", Trigger: "^demo", Priority: 10, Enabled: true})
	router.HandleMessage(pluginTriggerStatsMessage("demo"))
	assertPluginTriggerTotal(t, db, "high", 1)
	assertPluginTriggerTotal(t, db, "low", 0)
}

func TestPluginTriggerStatsSkipsKeywordReply(t *testing.T) {
	db, router := newPluginTriggerStatsRouter(t)
	fake := &keywordReplyFakeAdapter{}
	keywordReplies := NewKeywordReplyManager(db, func(msg *types.Message) adapter.Adapter { return fake }, func(platform, userID string) bool { return true }, time.Now())
	router.SetKeywordReplyManager(keywordReplies)
	if _, err := db.EnsureUserAccount("qq", "user-1"); err != nil {
		t.Fatalf("EnsureUserAccount returned error: %v", err)
	}
	registerPluginForStats(t, router, &types.Plugin{ID: "version-plugin", Name: "版本插件", Trigger: "^version$", Enabled: true})
	router.HandleMessage(pluginTriggerStatsMessage("version"))
	assertPluginTriggerTotal(t, db, "version-plugin", 0)
}

func newPluginTriggerStatsRouter(t *testing.T) (*config.Database, *Router) {
	t.Helper()
	db, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	router := NewRouter(session.NewManager())
	router.SetDatabase(db)
	router.SetAdapters(map[string]adapter.Adapter{"qq": &keywordReplyFakeAdapter{}})
	return db, router
}

func registerPluginForStats(t *testing.T, router *Router, plugin *types.Plugin) {
	t.Helper()
	if err := router.RegisterPlugin(plugin); err != nil {
		t.Fatalf("RegisterPlugin returned error: %v", err)
	}
}

func pluginTriggerStatsMessage(content string) *types.Message {
	return &types.Message{Platform: "qq", AdapterID: "1", UserID: "user-1", Content: content, Metadata: map[string]string{"adapter_id": "1", "adapter_remark": "测试机器人"}}
}

func assertPluginTriggerTotal(t *testing.T, db *config.Database, pluginID string, expected int64) {
	t.Helper()
	trend, err := db.GetPluginTriggerTrend("day", time.Now().Format("2006-01-02"), time.Now().Format("2006-01-02"), 8)
	if err != nil {
		t.Fatalf("GetPluginTriggerTrend returned error: %v", err)
	}
	for _, plugin := range trend.Plugins {
		if plugin.PluginID == pluginID {
			if plugin.Total != expected {
				t.Fatalf("plugin %s total = %d, expected %d; trend=%+v", pluginID, plugin.Total, expected, trend)
			}
			return
		}
	}
	if expected != 0 {
		t.Fatalf("plugin %s not found; expected %d; trend=%+v", pluginID, expected, trend)
	}
	if fmt.Sprint(trend.Plugins) == "<nil>" {
		t.Fatalf("unexpected nil plugins")
	}
}
