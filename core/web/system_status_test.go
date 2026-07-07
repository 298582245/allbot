package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/allbot/allbot/core/config"
)

func TestSystemStatusSeparatesAdapterTotalAndRunning(t *testing.T) {
	server := testServer(t)
	if err := server.adapterManager.GetDatabase().SaveAdapter(&config.AdapterConfig{Platform: "qq", Remark: "运行机器人", Enabled: true, Config: `{}`}); err != nil {
		t.Fatal(err)
	}
	if err := server.adapterManager.GetDatabase().SaveAdapter(&config.AdapterConfig{Platform: "telegram", Remark: "关闭机器人", Enabled: false, Config: `{}`}); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/system/status", nil)
	server.handleSystemStatus(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["adapterCount"].(float64) != 2 {
		t.Fatalf("adapterCount = %#v", response["adapterCount"])
	}
	if response["runningAdapterCount"].(float64) != 0 {
		t.Fatalf("runningAdapterCount = %#v", response["runningAdapterCount"])
	}
}

func TestSystemStatusIncludesAllBotResourceFields(t *testing.T) {
	server := testServer(t)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/system/status", nil)
	server.handleSystemStatus(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	assertPercentField(t, response, "allBotCpuUsagePercent")
	assertPercentField(t, response, "allBotMemoryUsagePercent")
	assertPercentField(t, response, "allBotPeakCpuUsagePercent")
	assertPercentField(t, response, "allBotPeakMemoryUsagePercent")
	assertPercentField(t, response, "allBotAverageCpuUsagePercent")
	assertPercentField(t, response, "allBotAverageMemoryUsagePercent")
	if response["allBotMemoryUsedBytes"].(float64) <= 0 {
		t.Fatalf("allBotMemoryUsedBytes = %#v", response["allBotMemoryUsedBytes"])
	}
	if _, ok := response["allBotMemoryTotalBytes"]; !ok {
		t.Fatal("missing allBotMemoryTotalBytes")
	}
	if response["allBotPeakMemoryUsedBytes"].(float64) <= 0 {
		t.Fatalf("allBotPeakMemoryUsedBytes = %#v", response["allBotPeakMemoryUsedBytes"])
	}
	if response["allBotAverageMemoryUsedBytes"].(float64) <= 0 {
		t.Fatalf("allBotAverageMemoryUsedBytes = %#v", response["allBotAverageMemoryUsedBytes"])
	}
}

func TestSystemStatusReturnsTotalAndCurrentUptimeSeconds(t *testing.T) {
	server := testServer(t)
	server.startTime = time.Now().Add(-2 * time.Hour)
	if err := server.adapterManager.GetDatabase().SetSetting(runtimeTotalSecondsKey, "86400", runtimeTotalSecondsDescription); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/system/status", nil)
	server.handleSystemStatus(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	currentSeconds := int64(response["currentUptimeSeconds"].(float64))
	totalSeconds := int64(response["totalUptimeSeconds"].(float64))
	if currentSeconds < 7190 || currentSeconds > 7210 {
		t.Fatalf("currentUptimeSeconds = %d", currentSeconds)
	}
	if totalSeconds < 93590 || totalSeconds > 93610 {
		t.Fatalf("totalUptimeSeconds = %d", totalSeconds)
	}
}

func assertPercentField(t *testing.T, response map[string]interface{}, key string) {
	t.Helper()
	value, ok := response[key].(float64)
	if !ok {
		t.Fatalf("missing percent field %s", key)
	}
	if value < 0 || value > 100 {
		t.Fatalf("%s = %f", key, value)
	}
}

func TestServerShutdownPersistsRuntimeSeconds(t *testing.T) {
	server := testServer(t)
	server.startTime = time.Now().Add(-90 * time.Second)
	if err := server.adapterManager.GetDatabase().SetSetting(runtimeTotalSecondsKey, "10", runtimeTotalSecondsDescription); err != nil {
		t.Fatal(err)
	}

	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	value, err := server.adapterManager.GetDatabase().GetSetting(runtimeTotalSecondsKey)
	if err != nil {
		t.Fatal(err)
	}
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if seconds < 100 || seconds > 102 {
		t.Fatalf("runtime total = %d", seconds)
	}
}
