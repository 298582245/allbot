package config

import (
	"testing"
	"time"
)

func TestPluginTaskLogReuseRequiresCompletedPreviousRun(t *testing.T) {
	database, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	startedAt := time.Now().Add(-time.Minute)
	item := PluginTaskLog{PluginID: "plugin-a", Platform: "test", UserID: "user-a", Content: "hello", Status: PluginTaskStatusRunning, StartedAt: startedAt, FinishedAt: startedAt}
	id, err := database.SavePluginTaskLog(item)
	if err != nil {
		t.Fatal(err)
	}
	if reused, err := database.ReusePluginTaskLog(id, item); err != nil || reused {
		t.Fatalf("running task must not be reused: reused=%v err=%v", reused, err)
	}
	if err := database.UpdatePluginTaskLog(id, PluginTaskStatusSuccess, "done", "", time.Now()); err != nil {
		t.Fatal(err)
	}
	item.StartedAt = time.Now()
	item.FinishedAt = item.StartedAt
	reused, err := database.ReusePluginTaskLog(id, item)
	if err != nil || !reused {
		t.Fatalf("completed task should be reused: reused=%v err=%v", reused, err)
	}
	detail, err := database.GetPluginTaskLog(id)
	if err != nil {
		t.Fatal(err)
	}
	if detail == nil || detail.Status != PluginTaskStatusRunning || detail.RunTotal != 2 || detail.Output != "" {
		t.Fatalf("unexpected reused task: %#v", detail)
	}
}
