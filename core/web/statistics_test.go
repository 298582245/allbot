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

func TestHandleMessageTotalTrend(t *testing.T) {
	server := newStatisticsTestServer(t)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/statistics/message-total-trend?granularity=day&start=2026-06-16&end=2026-06-17", nil)
	server.handleMessageTotalTrend(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d %s", recorder.Code, recorder.Body.String())
	}
	var response config.MessageTotalTrendSummary
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response returned error: %v", err)
	}
	if response.Granularity != "day" || len(response.Labels) != 2 || len(response.Totals) != 2 {
		t.Fatalf("unexpected trend response: %+v", response)
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
