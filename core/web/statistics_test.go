package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/types"
)

func TestHandleStatisticsOverviewIncludesLogStorage(t *testing.T) {
	withTempWorkdirForLogTests(t, func() {
		server := newStatisticsTestServer(t)
		writeTestLogFile(t, "2026-06-25", "12345")
		writeTestLogFile(t, "2026-06-26", "abcdef")
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/statistics/overview", nil)

		server.handleStatisticsOverview(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d %s", recorder.Code, recorder.Body.String())
		}
		var response statisticsOverview
		if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
			t.Fatalf("decode response returned error: %v", err)
		}
		if response.Logs.FileCount != 2 || response.Logs.TotalSizeBytes != 11 {
			t.Fatalf("unexpected log storage summary: %+v", response.Logs)
		}
	})
}

func TestHandleStatisticsOverviewKeepsScriptRunStatsAfterCleanup(t *testing.T) {
	server := newStatisticsTestServer(t)
	db := server.adapterManager.GetDatabase()
	startedAt := time.Now().AddDate(0, 0, -3)
	id, err := db.SaveScriptRunLog(config.ScriptRunLog{PluginID: "plugin-a", ScriptPath: "scripts/a.js", Runtime: "nodejs", RuntimeProfile: "node18", RunMode: "manual", Status: "running", StartedAt: startedAt, FinishedAt: startedAt})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateScriptRunLog(id, "success", "ok", "", startedAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CleanupScriptRunLogs(1); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/statistics/overview", nil)
	server.handleStatisticsOverview(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d %s", recorder.Code, recorder.Body.String())
	}
	var response statisticsOverview
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response returned error: %v", err)
	}
	if response.ScriptTasks.Total != 1 || response.ScriptTasks.Success != 1 || response.ScriptTasks.Failed != 0 {
		t.Fatalf("unexpected script task summary: %+v", response.ScriptTasks)
	}
}

func TestHandleMessageTotalTrend(t *testing.T) {
	server := newStatisticsTestServer(t)
	db := server.adapterManager.GetDatabase()
	if err := db.RecordMessageStat(&types.Message{Platform: "qq", AdapterID: "1"}); err != nil {
		t.Fatalf("RecordMessageStat private returned error: %v", err)
	}
	if err := db.RecordMessageStat(&types.Message{Platform: "qq", AdapterID: "1", GroupID: "group-1"}); err != nil {
		t.Fatalf("RecordMessageStat group returned error: %v", err)
	}
	today := time.Now().Format("2006-01-02")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/statistics/message-total-trend?granularity=day&start="+today+"&end="+today, nil)
	server.handleMessageTotalTrend(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d %s", recorder.Code, recorder.Body.String())
	}
	var response config.MessageTotalTrendSummary
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response returned error: %v", err)
	}
	if response.Granularity != "day" || len(response.Labels) != 1 || len(response.Totals) != 1 || len(response.PrivateTotals) != 1 || len(response.GroupTotals) != 1 {
		t.Fatalf("unexpected trend response: %+v", response)
	}
	if response.Totals[0] != 2 || response.PrivateTotals[0] != 1 || response.GroupTotals[0] != 1 {
		t.Fatalf("unexpected trend split: %+v", response)
	}
}

func TestHandleMessageTotalTrendRejectsLongDayRange(t *testing.T) {
	server := newStatisticsTestServer(t)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/statistics/message-total-trend?granularity=day&start=2026-06-01&end=2026-06-30", nil)
	server.handleMessageTotalTrend(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlePluginTriggerTrend(t *testing.T) {
	server := newStatisticsTestServer(t)
	db := server.adapterManager.GetDatabase()
	insertWebPluginTriggerStat(t, db, "alpha", 5)
	today := time.Now().Format("2006-01-02")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/statistics/plugin-trigger-trend?granularity=day&start="+today+"&end="+today, nil)
	server.handlePluginTriggerTrend(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d %s", recorder.Code, recorder.Body.String())
	}
	var response config.PluginTriggerTrendSummary
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response returned error: %v", err)
	}
	if response.Total != 5 || len(response.Labels) != 1 || len(response.Plugins) != 1 || response.Plugins[0].PluginID != "alpha" || response.Plugins[0].Counts[0] != 5 {
		t.Fatalf("unexpected plugin trigger trend response: %+v", response)
	}
}

func TestHandlePluginTriggerTrendRejectsLongDayRange(t *testing.T) {
	server := newStatisticsTestServer(t)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/statistics/plugin-trigger-trend?granularity=day&start=2026-06-01&end=2026-06-30", nil)
	server.handlePluginTriggerTrend(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandlePluginTriggerTrendWithMonthRange(t *testing.T) {
	server := newStatisticsTestServer(t)
	db := server.adapterManager.GetDatabase()
	insertWebPluginTriggerStat(t, db, "alpha", 5)
	month := time.Now().Format("2006-01")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/statistics/plugin-trigger-trend?granularity=month&start="+month+"&end="+month, nil)
	server.handlePluginTriggerTrend(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d %s", recorder.Code, recorder.Body.String())
	}
	var response config.PluginTriggerTrendSummary
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response returned error: %v", err)
	}
	if response.Granularity != "month" || response.Total != 5 || len(response.Plugins) != 1 || response.Plugins[0].Counts[0] != 5 {
		t.Fatalf("unexpected monthly plugin trigger trend response: %+v", response)
	}
}

func newStatisticsTestServer(t *testing.T) *Server {
	t.Helper()
	database, err := config.NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return NewServer("0", nil, nil, config.NewAdapterManager(database), nil)
}

func insertWebPluginTriggerStat(t *testing.T, db *config.Database, pluginID string, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		if err := db.RecordPluginTriggerStat(&types.Plugin{ID: pluginID, Name: pluginID, Trigger: "^" + pluginID}, &types.Message{Platform: "qq", AdapterID: "1", Metadata: map[string]string{"adapter_id": "1"}}); err != nil {
			t.Fatalf("RecordPluginTriggerStat returned error: %v", err)
		}
	}
}
