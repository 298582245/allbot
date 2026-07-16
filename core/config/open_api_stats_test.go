package config

import (
	"database/sql"
	"testing"
	"time"
)

func TestOpenAPISettingsDefaultsAndRawPersistence(t *testing.T) {
	db := newOpenAPIStatsTestDB(t)
	settings, err := db.GetOpenAPISettings()
	if err != nil {
		t.Fatalf("GetOpenAPISettings returned error: %v", err)
	}
	if len(settings.IPWhitelist) != 1 || settings.IPWhitelist[0] != "*" || len(settings.TrustedProxies) != 2 || settings.RetentionDays != 30 {
		t.Fatalf("unexpected default settings: %#v", settings)
	}
	if _, err := db.GetSetting(openAPISettingsKey); err != sql.ErrNoRows {
		t.Fatalf("default settings should not be persisted before explicit save: %v", err)
	}

	saved := OpenAPISettings{IPWhitelist: []string{"invalid rule"}, TrustedProxies: []string{"also invalid"}, RetentionDays: -7}
	if err := db.SaveOpenAPISettings(saved); err != nil {
		t.Fatalf("SaveOpenAPISettings returned error: %v", err)
	}
	loaded, err := db.GetOpenAPISettings()
	if err != nil {
		t.Fatalf("GetOpenAPISettings returned error: %v", err)
	}
	if len(loaded.IPWhitelist) != 1 || loaded.IPWhitelist[0] != "invalid rule" || loaded.RetentionDays != -7 {
		t.Fatalf("settings should persist outside validation boundaries: %#v", loaded)
	}
}

func TestWriteOpenAPICallBatchUpsertsStatsAndLogs(t *testing.T) {
	db := newOpenAPIStatsTestDB(t)
	first := time.Date(2026, 7, 15, 10, 0, 0, 0, time.Local)
	second := first.Add(time.Minute)
	if err := db.WriteOpenAPICallBatch(
		[]OpenAPICallStatDelta{{EndpointID: "weather", Total: 2, Success: 1, Rejected: 1, LastStatusCode: 403, LastOutcome: OpenAPICallOutcomeIPDenied, LastCalledAt: first}},
		[]OpenAPICallLog{
			{EndpointID: "weather", EndpointName: "天气", Method: "get", RequestPath: "/api/open/weather", ClientIP: "192.0.2.1", StatusCode: 200, Outcome: OpenAPICallOutcomeSuccess, DurationMS: 12, StartedAt: first},
			{EndpointID: "weather", EndpointName: "天气", Method: "GET", RequestPath: "/api/open/weather", ClientIP: "192.0.2.2", StatusCode: 403, Outcome: OpenAPICallOutcomeIPDenied, DurationMS: 1, StartedAt: second},
		},
	); err != nil {
		t.Fatalf("WriteOpenAPICallBatch returned error: %v", err)
	}
	if err := db.WriteOpenAPICallBatch(
		[]OpenAPICallStatDelta{{EndpointID: "weather", Total: 1, Failed: 1, LastStatusCode: 500, LastOutcome: OpenAPICallOutcomeFailed, LastCalledAt: second}}, nil,
	); err != nil {
		t.Fatalf("WriteOpenAPICallBatch second call returned error: %v", err)
	}

	stats, err := db.GetOpenAPICallStats([]string{"weather", "weather", "missing"})
	if err != nil {
		t.Fatalf("GetOpenAPICallStats returned error: %v", err)
	}
	stat := stats["weather"]
	if stat.Total != 3 || stat.Success != 1 || stat.Rejected != 1 || stat.Failed != 1 || stat.LastStatusCode != 500 || stat.LastOutcome != OpenAPICallOutcomeFailed || stat.LastCalledAt == nil || !stat.LastCalledAt.Equal(second) {
		t.Fatalf("unexpected accumulated stat: %#v", stat)
	}
	items, total, err := db.ListOpenAPICallLogs(OpenAPICallLogFilter{EndpointID: "weather", Limit: 10})
	if err != nil {
		t.Fatalf("ListOpenAPICallLogs returned error: %v", err)
	}
	if total != 2 || len(items) != 2 || items[0].ClientIP != "192.0.2.2" || items[1].Method != "GET" {
		t.Fatalf("unexpected logs total=%d items=%#v", total, items)
	}
}

func TestWriteOpenAPICallBatchRollsBackInvalidStats(t *testing.T) {
	db := newOpenAPIStatsTestDB(t)
	err := db.WriteOpenAPICallBatch(
		[]OpenAPICallStatDelta{{EndpointID: "weather", Total: 1, Success: 1}, {EndpointID: "", Total: 1, Success: 1}},
		[]OpenAPICallLog{{EndpointID: "weather", Outcome: OpenAPICallOutcomeSuccess}},
	)
	if err == nil {
		t.Fatal("expected invalid batch to fail")
	}
	stats, err := db.GetOpenAPICallStats(nil)
	if err != nil {
		t.Fatalf("GetOpenAPICallStats returned error: %v", err)
	}
	if len(stats) != 0 {
		t.Fatalf("transaction should roll back stats: %#v", stats)
	}
	_, total, err := db.ListOpenAPICallLogs(OpenAPICallLogFilter{})
	if err != nil {
		t.Fatalf("ListOpenAPICallLogs returned error: %v", err)
	}
	if total != 0 {
		t.Fatalf("transaction should roll back logs, total=%d", total)
	}
}

func TestListOpenAPICallLogsFiltersAndPaginates(t *testing.T) {
	db := newOpenAPIStatsTestDB(t)
	base := time.Date(2026, 7, 15, 10, 0, 0, 0, time.Local)
	logs := []OpenAPICallLog{
		{EndpointID: "alpha", ClientIP: "192.0.2.1", StatusCode: 200, Outcome: OpenAPICallOutcomeSuccess, StartedAt: base},
		{EndpointID: "alpha", ClientIP: "192.0.2.2", StatusCode: 401, Outcome: OpenAPICallOutcomeTokenDenied, StartedAt: base.Add(time.Minute)},
		{EndpointID: "alpha", ClientIP: "192.0.2.2", StatusCode: 500, Outcome: OpenAPICallOutcomeFailed, StartedAt: base.Add(2 * time.Minute)},
		{EndpointID: "beta", ClientIP: "192.0.2.2", StatusCode: 500, Outcome: OpenAPICallOutcomeFailed, StartedAt: base.Add(3 * time.Minute)},
	}
	if err := db.WriteOpenAPICallBatch(nil, logs); err != nil {
		t.Fatalf("WriteOpenAPICallBatch returned error: %v", err)
	}
	from := base.Add(time.Minute)
	to := base.Add(2 * time.Minute)
	items, total, err := db.ListOpenAPICallLogs(OpenAPICallLogFilter{EndpointID: "alpha", ClientIP: "192.0.2.2", StartedFrom: &from, StartedTo: &to, Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("ListOpenAPICallLogs returned error: %v", err)
	}
	if total != 2 || len(items) != 1 || items[0].StatusCode != 401 {
		t.Fatalf("unexpected filtered page total=%d items=%#v", total, items)
	}
	items, total, err = db.ListOpenAPICallLogs(OpenAPICallLogFilter{EndpointID: "alpha", Outcome: OpenAPICallOutcomeFailed, StatusCode: 500})
	if err != nil || total != 1 || len(items) != 1 {
		t.Fatalf("unexpected outcome/status filter total=%d items=%#v err=%v", total, items, err)
	}
}

func TestCleanupOpenAPICallLogsBatchKeepsStats(t *testing.T) {
	db := newOpenAPIStatsTestDB(t)
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.Local)
	logs := []OpenAPICallLog{
		{EndpointID: "alpha", Outcome: OpenAPICallOutcomeSuccess, StartedAt: now.AddDate(0, 0, -40)},
		{EndpointID: "alpha", Outcome: OpenAPICallOutcomeSuccess, StartedAt: now.AddDate(0, 0, -35)},
		{EndpointID: "alpha", Outcome: OpenAPICallOutcomeSuccess, StartedAt: now.AddDate(0, 0, -1)},
	}
	stats := []OpenAPICallStatDelta{{EndpointID: "alpha", Total: 3, Success: 3, LastStatusCode: 200, LastOutcome: OpenAPICallOutcomeSuccess, LastCalledAt: now}}
	if err := db.WriteOpenAPICallBatch(stats, logs); err != nil {
		t.Fatalf("WriteOpenAPICallBatch returned error: %v", err)
	}
	deleted, err := db.CleanupOpenAPICallLogsBatch(30, 1, now)
	if err != nil || deleted != 1 {
		t.Fatalf("first cleanup deleted=%d err=%v", deleted, err)
	}
	deleted, err = db.CleanupOpenAPICallLogsBatch(30, 10, now)
	if err != nil || deleted != 1 {
		t.Fatalf("second cleanup deleted=%d err=%v", deleted, err)
	}
	_, total, err := db.ListOpenAPICallLogs(OpenAPICallLogFilter{EndpointID: "alpha"})
	if err != nil || total != 1 {
		t.Fatalf("unexpected remaining logs total=%d err=%v", total, err)
	}
	statMap, err := db.GetOpenAPICallStats([]string{"alpha"})
	if err != nil || statMap["alpha"].Total != 3 {
		t.Fatalf("cleanup should not change totals: %#v err=%v", statMap, err)
	}
	deleted, err = db.CleanupOpenAPICallLogsBatch(0, 10, now)
	if err != nil || deleted != 0 {
		t.Fatalf("disabled cleanup deleted=%d err=%v", deleted, err)
	}
}

func TestDeleteOpenAPICallDataDeletesLogsAndTotals(t *testing.T) {
	db := newOpenAPIStatsTestDB(t)
	if err := db.WriteOpenAPICallBatch(
		[]OpenAPICallStatDelta{{EndpointID: "alpha", Total: 1, Success: 1}},
		[]OpenAPICallLog{{EndpointID: "alpha", Outcome: OpenAPICallOutcomeSuccess}},
	); err != nil {
		t.Fatalf("WriteOpenAPICallBatch returned error: %v", err)
	}
	if err := db.DeleteOpenAPICallData("alpha"); err != nil {
		t.Fatalf("DeleteOpenAPICallData returned error: %v", err)
	}
	stats, err := db.GetOpenAPICallStats([]string{"alpha"})
	if err != nil || len(stats) != 0 {
		t.Fatalf("unexpected stats after delete: %#v err=%v", stats, err)
	}
	_, total, err := db.ListOpenAPICallLogs(OpenAPICallLogFilter{EndpointID: "alpha"})
	if err != nil || total != 0 {
		t.Fatalf("unexpected logs after delete total=%d err=%v", total, err)
	}
}

func TestOpenAPIStatsSchemaIndexes(t *testing.T) {
	db := newOpenAPIStatsTestDB(t)
	indexes := map[string]bool{}
	rows, err := db.db.Query(`SELECT name FROM sqlite_master WHERE type = 'index' AND tbl_name = 'open_api_call_logs'`)
	if err != nil {
		t.Fatalf("query indexes returned error: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan index returned error: %v", err)
		}
		indexes[name] = true
	}
	for _, name := range []string{"idx_open_api_call_logs_endpoint_started", "idx_open_api_call_logs_endpoint_outcome_started", "idx_open_api_call_logs_started"} {
		if !indexes[name] {
			t.Fatalf("missing required index %s: %#v", name, indexes)
		}
	}
}

func newOpenAPIStatsTestDB(t *testing.T) *Database {
	t.Helper()
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
