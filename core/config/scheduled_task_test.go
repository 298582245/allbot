package config

import (
	"testing"
	"time"
)

func TestDisablePluginScheduledTasksOnlyClosesPluginDeclaredTasks(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	nextRunAt := time.Now().Add(time.Hour)
	items := []*ScheduledTask{
		{PluginID: "demo", TaskKey: "plugin-task", Name: "插件任务", Enabled: true, Cron: "0 8 * * *", Platform: "qq", UserID: "1001", Content: "插件消息", Source: "plugin", NextRunAt: &nextRunAt},
		{PluginID: "demo", TaskKey: "admin-task", Name: "管理员任务", Enabled: true, Cron: "0 9 * * *", Platform: "qq", UserID: "1002", Content: "管理员消息", Source: "user", NextRunAt: &nextRunAt},
		{PluginID: "other", TaskKey: "other-task", Name: "其他插件任务", Enabled: true, Cron: "0 10 * * *", Platform: "qq", UserID: "1003", Content: "其他消息", Source: "plugin", NextRunAt: &nextRunAt},
	}
	for _, item := range items {
		if err := db.SaveScheduledTask(item); err != nil {
			t.Fatal(err)
		}
	}

	changed, err := db.DisablePluginScheduledTasks(" demo ")
	if err != nil {
		t.Fatal(err)
	}
	if changed != 1 {
		t.Fatalf("changed = %d, expected 1", changed)
	}

	stored, err := db.ListScheduledTasks()
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]*ScheduledTask{}
	for _, item := range stored {
		byKey[item.TaskKey] = item
	}
	if task := byKey["plugin-task"]; task == nil || task.Enabled || task.NextRunAt != nil {
		t.Fatalf("plugin task should be disabled with nil next_run_at: %#v", task)
	}
	if task := byKey["admin-task"]; task == nil || !task.Enabled || task.NextRunAt == nil {
		t.Fatalf("admin task should stay enabled: %#v", task)
	}
	if task := byKey["other-task"]; task == nil || !task.Enabled || task.NextRunAt == nil {
		t.Fatalf("other plugin task should stay enabled: %#v", task)
	}

	changed, err = db.DisablePluginScheduledTasks("demo")
	if err != nil {
		t.Fatal(err)
	}
	if changed != 0 {
		t.Fatalf("second changed = %d, expected 0", changed)
	}
}

func TestDisablePluginScheduledTasksRejectsBlankPluginID(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.DisablePluginScheduledTasks("  "); err == nil {
		t.Fatal("expected blank plugin id to be rejected")
	}
}

func TestUpsertPluginScheduledTaskDoesNotOverwriteExistingTask(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	firstNextRunAt := time.Now().Add(time.Hour)
	first, err := db.UpsertPluginScheduledTask("demo", &ScheduledTask{TaskKey: "daily", Name: "默认任务", Description: "默认描述", Enabled: true, Pinned: false, Cron: "0 8 * * *", Platform: "qq", AdapterID: "1", UserID: "admin", GroupID: "", Content: "默认内容", NextRunAt: &firstNextRunAt}, 3)
	if err != nil {
		t.Fatal(err)
	}

	changedNextRunAt := time.Now().Add(2 * time.Hour)
	second, err := db.UpsertPluginScheduledTask("demo", &ScheduledTask{TaskKey: "daily", Name: "覆盖任务", Description: "覆盖描述", Enabled: false, Pinned: true, Cron: "0 9 * * *", Platform: "telegram", AdapterID: "2", UserID: "other-admin", GroupID: "group", Content: "覆盖内容", NextRunAt: &changedNextRunAt}, 3)
	if err != nil {
		t.Fatal(err)
	}

	if second.ID != first.ID {
		t.Fatalf("id = %d, expected %d", second.ID, first.ID)
	}
	if second.Name != "默认任务" || second.Description != "默认描述" || !second.Enabled || second.Pinned || second.Cron != "0 8 * * *" || second.Platform != "qq" || second.AdapterID != "1" || second.UserID != "admin" || second.GroupID != "" || second.Content != "默认内容" {
		t.Fatalf("existing task was overwritten: %#v", second)
	}
}
