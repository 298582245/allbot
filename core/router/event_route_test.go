package router

import (
	"testing"
	"time"

	"github.com/allbot/allbot/core/adapter"
	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/session"
	"github.com/allbot/allbot/core/types"
)

func TestRouterEventMessagesTriggerAllMatchedPlugins(t *testing.T) {
	db, r := newPluginTriggerStatsRouter(t)
	registerPluginForStats(t, r, &types.Plugin{ID: "first", Name: "事件插件1", Trigger: "^GROUP_MEMBER_ADD$", Priority: 1, Enabled: true})
	registerPluginForStats(t, r, &types.Plugin{ID: "second", Name: "事件插件2", Trigger: "^GROUP_MEMBER_ADD$", Priority: 10, Enabled: true})

	r.HandleMessage(routerEventMessage("GROUP_MEMBER_ADD"))

	assertPluginTriggerTotal(t, db, "first", 1)
	assertPluginTriggerTotal(t, db, "second", 1)
}

func TestRouterNormalMessagesStillTriggerHighestPriorityPlugin(t *testing.T) {
	db, r := newPluginTriggerStatsRouter(t)
	if _, err := db.EnsureUserAccount("qq", "user-1"); err != nil {
		t.Fatalf("EnsureUserAccount returned error: %v", err)
	}
	registerPluginForStats(t, r, &types.Plugin{ID: "low", Name: "低优先级", Trigger: "^demo$", Priority: 1, Enabled: true})
	registerPluginForStats(t, r, &types.Plugin{ID: "high", Name: "高优先级", Trigger: "^demo$", Priority: 10, Enabled: true})

	r.HandleMessage(pluginTriggerStatsMessage("demo"))

	assertPluginTriggerTotal(t, db, "high", 1)
	assertPluginTriggerTotal(t, db, "low", 0)
}

func TestRouterEventMessageSkipsUnregisteredUserGuide(t *testing.T) {
	db, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	fake := &keywordReplyFakeAdapter{}
	r := NewRouter(session.NewManager())
	r.SetDatabase(db)
	r.SetAdapters(map[string]adapter.Adapter{"qq": fake})
	registerPluginForStats(t, r, &types.Plugin{ID: "event", Name: "事件插件", Trigger: "^GROUP_MSG_RECEIVE$", Enabled: true})

	r.HandleMessage(routerEventMessage("GROUP_MSG_RECEIVE"))

	if messages := fake.sentMessages(); len(messages) != 0 {
		t.Fatalf("sent messages = %#v, expected no register guide", messages)
	}
	assertPluginTriggerTotal(t, db, "event", 1)
}

func TestRouterEventMessageSkipsWaitingSessionIntercept(t *testing.T) {
	db, r := newPluginTriggerStatsRouter(t)
	registerPluginForStats(t, r, &types.Plugin{ID: "event", Name: "事件插件", Trigger: "^GROUP_MEMBER_ADD$", Enabled: true})
	ch := r.sessionManager.CreateSession("plugin", "member-openid", "group-openid", 1)
	defer func() {
		select {
		case <-ch:
		default:
		}
	}()

	r.HandleMessage(routerEventMessage("GROUP_MEMBER_ADD"))

	assertPluginTriggerTotal(t, db, "event", 1)
	select {
	case <-ch:
		t.Fatal("event message should not be delivered to waiting session")
	case <-time.After(10 * time.Millisecond):
	}
}

func routerEventMessage(name string) *types.Message {
	return &types.Message{
		Platform:  "qq",
		AdapterID: "1",
		UserID:    "member-openid",
		GroupID:   "group-openid",
		Content:   name,
		Metadata: map[string]string{
			"adapter_id":   "1",
			"message_type": "event",
			"event_name":   name,
		},
	}
}
