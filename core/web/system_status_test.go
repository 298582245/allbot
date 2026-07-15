package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"sync"
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
	assertPercentField(t, response, "allBotHistoricalPeakCpuUsagePercent")
	assertPercentField(t, response, "allBotHistoricalPeakMemoryUsagePercent")
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
	if response["allBotHistoricalPeakMemoryUsedBytes"].(float64) <= 0 {
		t.Fatalf("allBotHistoricalPeakMemoryUsedBytes = %#v", response["allBotHistoricalPeakMemoryUsedBytes"])
	}
	if response["allBotAverageMemoryUsedBytes"].(float64) <= 0 {
		t.Fatalf("allBotAverageMemoryUsedBytes = %#v", response["allBotAverageMemoryUsedBytes"])
	}
}

func TestHistoricalResourcePeaksPersistOnlyForStrictIncreases(t *testing.T) {
	server := testServer(t)
	var mu sync.Mutex
	writes := make([]allBotHistoricalResourcePeaks, 0)
	server.historicalResourceGetSetting = func() (string, error) {
		return `{"cpuUsagePercent":10,"memoryUsedBytes":100,"memoryUsagePercent":20}`, nil
	}
	server.historicalResourceSetSetting = func(value string) error {
		var peaks allBotHistoricalResourcePeaks
		if err := json.Unmarshal([]byte(value), &peaks); err != nil {
			return err
		}
		mu.Lock()
		writes = append(writes, peaks)
		mu.Unlock()
		return nil
	}

	first := server.updateHistoricalResourcePeaks(allBotHistoricalResourcePeaks{CPUUsagePercent: 15, MemoryUsedBytes: 90, MemoryUsagePercent: 20})
	assertHistoricalResourcePeaks(t, first, allBotHistoricalResourcePeaks{CPUUsagePercent: 15, MemoryUsedBytes: 100, MemoryUsagePercent: 20})
	server.updateHistoricalResourcePeaks(first)
	server.updateHistoricalResourcePeaks(allBotHistoricalResourcePeaks{CPUUsagePercent: 14, MemoryUsedBytes: 99, MemoryUsagePercent: 19})
	second := server.updateHistoricalResourcePeaks(allBotHistoricalResourcePeaks{CPUUsagePercent: 15, MemoryUsedBytes: 120, MemoryUsagePercent: 25})
	assertHistoricalResourcePeaks(t, second, allBotHistoricalResourcePeaks{CPUUsagePercent: 15, MemoryUsedBytes: 120, MemoryUsagePercent: 25})

	mu.Lock()
	defer mu.Unlock()
	if len(writes) != 2 {
		t.Fatalf("write count = %d, writes = %#v", len(writes), writes)
	}
	assertHistoricalResourcePeaks(t, writes[0], allBotHistoricalResourcePeaks{CPUUsagePercent: 15, MemoryUsedBytes: 100, MemoryUsagePercent: 20})
	assertHistoricalResourcePeaks(t, writes[1], allBotHistoricalResourcePeaks{CPUUsagePercent: 15, MemoryUsedBytes: 120, MemoryUsagePercent: 25})
}

func TestHistoricalResourcePeaksRetryAfterLoadAndWriteFailures(t *testing.T) {
	server := testServer(t)
	loadAttempts := 0
	writeAttempts := 0
	server.historicalResourceGetSetting = func() (string, error) {
		loadAttempts++
		if loadAttempts == 1 {
			return "", errors.New("load failed")
		}
		return `{"cpuUsagePercent":30,"memoryUsedBytes":300,"memoryUsagePercent":40}`, nil
	}
	server.historicalResourceSetSetting = func(value string) error {
		writeAttempts++
		if writeAttempts == 1 {
			return errors.New("write failed")
		}
		return nil
	}

	candidate := allBotHistoricalResourcePeaks{CPUUsagePercent: 35, MemoryUsedBytes: 250, MemoryUsagePercent: 45}
	first := server.updateHistoricalResourcePeaks(candidate)
	assertHistoricalResourcePeaks(t, first, candidate)
	second := server.updateHistoricalResourcePeaks(allBotHistoricalResourcePeaks{})
	assertHistoricalResourcePeaks(t, second, allBotHistoricalResourcePeaks{CPUUsagePercent: 35, MemoryUsedBytes: 300, MemoryUsagePercent: 45})
	third := server.updateHistoricalResourcePeaks(allBotHistoricalResourcePeaks{})
	assertHistoricalResourcePeaks(t, third, second)
	if loadAttempts != 2 || writeAttempts != 2 {
		t.Fatalf("loadAttempts=%d writeAttempts=%d", loadAttempts, writeAttempts)
	}
	assertHistoricalResourcePeaks(t, server.persistedHistoricalPeaks, second)
}

func TestHistoricalResourcePeaksRestoreAcrossServers(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "allbot.db")
	first := testServerWithDatabase(t, databasePath)
	persisted := first.updateHistoricalResourcePeaks(allBotHistoricalResourcePeaks{CPUUsagePercent: 42, MemoryUsedBytes: 4096, MemoryUsagePercent: 12})
	assertHistoricalResourcePeaks(t, persisted, allBotHistoricalResourcePeaks{CPUUsagePercent: 42, MemoryUsedBytes: 4096, MemoryUsagePercent: 12})

	second := testServerWithDatabase(t, databasePath)
	restored := second.updateHistoricalResourcePeaks(allBotHistoricalResourcePeaks{CPUUsagePercent: 1, MemoryUsedBytes: 2, MemoryUsagePercent: 3})
	assertHistoricalResourcePeaks(t, restored, persisted)
	if second.peakAllBotCPUPercent != 0 || second.peakAllBotMemoryUsedBytes != 0 || second.peakAllBotMemoryUsagePercent != 0 {
		t.Fatalf("new server runtime peaks should start empty: %#v", second)
	}
}

func TestHistoricalResourcePeaksConcurrentUpdatesKeepPerMetricMaximum(t *testing.T) {
	server := testServer(t)
	server.historicalResourceGetSetting = func() (string, error) { return "", sql.ErrNoRows }
	var mu sync.Mutex
	writeCount := 0
	server.historicalResourceSetSetting = func(string) error {
		mu.Lock()
		writeCount++
		mu.Unlock()
		return nil
	}
	candidates := []allBotHistoricalResourcePeaks{
		{CPUUsagePercent: 70, MemoryUsedBytes: 100, MemoryUsagePercent: 10},
		{CPUUsagePercent: 20, MemoryUsedBytes: 900, MemoryUsagePercent: 30},
		{CPUUsagePercent: 40, MemoryUsedBytes: 500, MemoryUsagePercent: 80},
	}
	var wg sync.WaitGroup
	for _, candidate := range candidates {
		candidate := candidate
		wg.Add(1)
		go func() {
			defer wg.Done()
			server.updateHistoricalResourcePeaks(candidate)
		}()
	}
	wg.Wait()

	assertHistoricalResourcePeaks(t, server.historicalResourcePeaks, allBotHistoricalResourcePeaks{CPUUsagePercent: 70, MemoryUsedBytes: 900, MemoryUsagePercent: 80})
	assertHistoricalResourcePeaks(t, server.persistedHistoricalPeaks, server.historicalResourcePeaks)
	mu.Lock()
	defer mu.Unlock()
	if writeCount < 1 || writeCount > len(candidates) {
		t.Fatalf("writeCount = %d", writeCount)
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

func assertHistoricalResourcePeaks(t *testing.T, actual, expected allBotHistoricalResourcePeaks) {
	t.Helper()
	if actual != expected {
		t.Fatalf("historical peaks = %#v, want %#v", actual, expected)
	}
}

func testServerWithDatabase(t *testing.T, databasePath string) *Server {
	t.Helper()
	database, err := config.NewDatabase(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return NewServer("0", nil, nil, config.NewAdapterManager(database), nil)
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
