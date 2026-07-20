package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"testing"
)

const (
	saveMemoryPluginID    = "save_memory_plugin"
	saveMemoryOpenAPIID   = "save_memory_api"
	saveMemoryWarmupCount = 20
	saveMemoryBatchCount  = 8
	saveMemoryBatchSize   = 25
)

type saveMemorySnapshot struct {
	Goroutines     int
	HeapAlloc      uint64
	HeapObjects    uint64
	HeapInuse      uint64
	Sys            uint64
	ManagerPlugins int
	RouterPlugins  int
	StatsStarted   bool
	StatsPending   int
}

func TestPluginConfigSaveDoesNotGrowRuntimeState(t *testing.T) {
	withTempWorkdir(t, func() {
		server, cleanup := newSaveMemoryTestServer(t)
		defer cleanup()
		prepareSaveMemoryPlugin(t, server)
		payload := saveMemoryPluginConfig()

		snapshots := runSaveMemoryLoop(t, server, func() {
			recorder := performOpenAPIJSONRequest(t, server.handlePluginConfig, http.MethodPut, "/api/plugins/config/"+saveMemoryPluginID, payload)
			assertSaveResponseOK(t, recorder.Code, recorder.Body.String())
		})
		assertSaveMemoryStableWithProfile(t, snapshots, 1, 1, "plugin-config")

		stored := readPluginJSON(t, filepath.Join("plugins", saveMemoryPluginID))
		if stored["name"] != "保存内存测试插件" || stored["entry"] != "main.js" {
			t.Fatalf("插件配置未正确落盘: %#v", stored)
		}
	})
}

func TestPluginCodeSaveDoesNotGrowRuntimeState(t *testing.T) {
	withTempWorkdir(t, func() {
		server, cleanup := newSaveMemoryTestServer(t)
		defer cleanup()
		prepareSaveMemoryPlugin(t, server)
		code := "module.exports.action = async function action() { return { ok: true } }\n"

		snapshots := runSaveMemoryLoop(t, server, func() {
			recorder := performOpenAPIJSONRequest(t, server.handlePluginCode, http.MethodPut, "/api/plugins/code/"+saveMemoryPluginID, map[string]string{"code": code})
			assertSaveResponseOK(t, recorder.Code, recorder.Body.String())
		})
		assertSaveMemoryStableWithProfile(t, snapshots, 1, 1, "plugin-code")
		if stored := readTextFile(t, filepath.Join("plugins", saveMemoryPluginID, "main.js")); stored != code {
			t.Fatalf("插件入口代码未正确落盘: %q", stored)
		}
	})
}

func TestPluginFileSaveDoesNotGrowRuntimeState(t *testing.T) {
	withTempWorkdir(t, func() {
		server, cleanup := newSaveMemoryTestServer(t)
		defer cleanup()
		prepareSaveMemoryPlugin(t, server)
		code := "module.exports.helper = function helper() { return 'ok' }\n"

		snapshots := runSaveMemoryLoop(t, server, func() {
			recorder := performOpenAPIJSONRequest(t, server.handlePluginFiles, http.MethodPut, "/api/plugins/files/"+saveMemoryPluginID, map[string]string{"path": "helper.js", "code": code})
			assertSaveResponseOK(t, recorder.Code, recorder.Body.String())
		})
		assertSaveMemoryStableWithProfile(t, snapshots, 1, 1, "plugin-file")
		if stored := readTextFile(t, filepath.Join("plugins", saveMemoryPluginID, "helper.js")); stored != code {
			t.Fatalf("插件文件未正确落盘: %q", stored)
		}
	})
}

func TestOpenAPIConfigSaveDoesNotGrowRuntimeState(t *testing.T) {
	withTempWorkdir(t, func() {
		server, cleanup := newSaveMemoryTestServer(t)
		defer cleanup()
		prepareSaveMemoryOpenAPI(t)
		statsRecorder := server.openAPIStats
		payload := saveMemoryOpenAPIConfig()

		snapshots := runSaveMemoryLoop(t, server, func() {
			recorder := performOpenAPIJSONRequest(t, server.handleOpenAPIConfigDetail, http.MethodPut, "/api/open-apis/"+saveMemoryOpenAPIID, payload)
			assertSaveResponseOK(t, recorder.Code, recorder.Body.String())
		})
		assertSaveMemoryStableWithProfile(t, snapshots, 0, 0, "open-api-config")
		assertOpenAPIStatsRecorderIdle(t, server, statsRecorder)

		stored, err := loadOpenAPIEndpoint(saveMemoryOpenAPIID)
		if err != nil {
			t.Fatal(err)
		}
		if stored.Name != "保存内存测试接口" || stored.Entry != "main.js" || stored.Builtin != "" {
			t.Fatalf("Open API 配置未正确落盘: %#v", stored)
		}
		if code := readOpenAPICodeForTest(t); code != saveMemoryOpenAPICode() {
			t.Fatalf("保存配置不应改写 Open API 代码: %q", code)
		}
	})
}

func TestOpenAPICodeSaveDoesNotGrowRuntimeState(t *testing.T) {
	withTempWorkdir(t, func() {
		server, cleanup := newSaveMemoryTestServer(t)
		defer cleanup()
		prepareSaveMemoryOpenAPI(t)
		statsRecorder := server.openAPIStats
		code := "module.exports.action = async function action(ctx, req, res) { res.json({ saved: true }) }\n"
		payload := map[string]string{"code": code, "runtime": "nodejs", "entry": "main.js"}

		snapshots := runSaveMemoryLoop(t, server, func() {
			recorder := performOpenAPIJSONRequest(t, func(w http.ResponseWriter, r *http.Request) {
				server.handleOpenAPICode(w, r, saveMemoryOpenAPIID)
			}, http.MethodPut, "/api/open-apis/"+saveMemoryOpenAPIID+"/code", payload)
			assertSaveResponseOK(t, recorder.Code, recorder.Body.String())
		})
		assertSaveMemoryStableWithProfile(t, snapshots, 0, 0, "open-api-code")
		assertOpenAPIStatsRecorderIdle(t, server, statsRecorder)
		if stored := readOpenAPICodeForTest(t); stored != code {
			t.Fatalf("Open API 代码未正确落盘: %q", stored)
		}
	})
}

func newSaveMemoryTestServer(t *testing.T) (*Server, func()) {
	t.Helper()
	server := testServer(t)
	return server, func() {
		if server.openAPIStats != nil {
			if err := server.openAPIStats.Close(); err != nil {
				t.Errorf("关闭 Open API 统计 recorder 失败: %v", err)
			}
		}
		if server.logManager != nil {
			server.logManager.Stop()
		}
	}
}

func prepareSaveMemoryPlugin(t *testing.T, server *Server) {
	t.Helper()
	pluginDir := filepath.Join("plugins", saveMemoryPluginID)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	configData, err := json.Marshal(saveMemoryPluginConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), configData, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "main.js"), []byte("module.exports.action = async function action() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "helper.js"), []byte("module.exports.helper = function helper() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	loaded, err := server.pluginManager.LoadPlugin(pluginDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.router.RegisterPlugin(loaded); err != nil {
		t.Fatal(err)
	}
}

func saveMemoryPluginConfig() map[string]interface{} {
	return map[string]interface{}{
		"name": "保存内存测试插件", "version": "1.0.0", "runtime": "nodejs", "entry": "main.js",
		"platforms": []string{}, "allowed_adapter_ids": []string{}, "priority": 0, "pinned": false,
		"trigger": "^save-memory$", "enabled": true, "user_config": map[string]interface{}{},
	}
}

func prepareSaveMemoryOpenAPI(t *testing.T) {
	t.Helper()
	endpointDir := filepath.Join(openAPIStorageDir, saveMemoryOpenAPIID)
	if err := os.MkdirAll(endpointDir, 0755); err != nil {
		t.Fatal(err)
	}
	configData, err := json.Marshal(saveMemoryOpenAPIConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(endpointDir, openAPIConfigFile), configData, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(endpointDir, "main.js"), []byte(saveMemoryOpenAPICode()), 0644); err != nil {
		t.Fatal(err)
	}
}

func saveMemoryOpenAPIConfig() map[string]interface{} {
	return map[string]interface{}{
		"id": saveMemoryOpenAPIID, "name": "保存内存测试接口", "path": "save-memory", "method": "POST",
		"enabled": true, "token": "test-token", "runtime": "nodejs", "entry": "main.js", "description": "保存稳定性测试",
	}
}

func saveMemoryOpenAPICode() string {
	return "module.exports.action = async function action(ctx, req, res) { res.json({ ok: true }) }\n"
}

func readOpenAPICodeForTest(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(openAPIStorageDir, saveMemoryOpenAPIID, "main.js"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func runSaveMemoryLoop(t *testing.T, server *Server, save func()) []saveMemorySnapshot {
	t.Helper()
	for i := 0; i < saveMemoryWarmupCount; i++ {
		save()
	}
	snapshots := []saveMemorySnapshot{captureSaveMemorySnapshot(server)}
	for batch := 0; batch < saveMemoryBatchCount; batch++ {
		for i := 0; i < saveMemoryBatchSize; i++ {
			save()
		}
		snapshots = append(snapshots, captureSaveMemorySnapshot(server))
	}
	for index, snapshot := range snapshots {
		t.Logf("内存采样[%d] saves=%d goroutines=%d heap_alloc=%d heap_objects=%d heap_inuse=%d sys=%d manager_plugins=%d router_plugins=%d stats_started=%t stats_pending=%d",
			index, saveMemoryWarmupCount+index*saveMemoryBatchSize, snapshot.Goroutines, snapshot.HeapAlloc, snapshot.HeapObjects,
			snapshot.HeapInuse, snapshot.Sys, snapshot.ManagerPlugins, snapshot.RouterPlugins, snapshot.StatsStarted, snapshot.StatsPending)
	}
	return snapshots
}

func captureSaveMemorySnapshot(server *Server) saveMemorySnapshot {
	runtime.GC()
	runtime.GC()
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	started, pending := openAPIStatsState(server.openAPIStats)
	snapshot := saveMemorySnapshot{
		Goroutines: runtime.NumGoroutine(), HeapAlloc: stats.HeapAlloc, HeapObjects: stats.HeapObjects,
		HeapInuse: stats.HeapInuse, Sys: stats.Sys, StatsStarted: started, StatsPending: pending,
	}
	if server.pluginManager != nil {
		snapshot.ManagerPlugins = len(server.pluginManager.GetAllPlugins())
	}
	if server.router != nil {
		snapshot.RouterPlugins = len(server.router.GetPlugins())
	}
	return snapshot
}

func openAPIStatsState(recorder *openAPIStatsRecorder) (bool, int) {
	if recorder == nil {
		return false, 0
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return recorder.started, recorder.pending
}

func assertSaveMemoryStableWithProfile(t *testing.T, snapshots []saveMemorySnapshot, managerPlugins, routerPlugins int, profileName string) {
	t.Helper()
	defer writeSaveMemoryHeapProfile(t, profileName)
	assertSaveMemoryStable(t, snapshots, managerPlugins, routerPlugins)
}

func assertSaveMemoryStable(t *testing.T, snapshots []saveMemorySnapshot, managerPlugins, routerPlugins int) {
	t.Helper()
	if len(snapshots) < 2 {
		t.Fatal("内存采样数量不足")
	}
	for _, snapshot := range snapshots {
		if snapshot.ManagerPlugins != managerPlugins || snapshot.RouterPlugins != routerPlugins {
			t.Fatalf("插件容器数量发生变化: manager=%d/%d router=%d/%d", snapshot.ManagerPlugins, managerPlugins, snapshot.RouterPlugins, routerPlugins)
		}
	}
	first := snapshots[0]
	last := snapshots[len(snapshots)-1]
	if last.Goroutines > first.Goroutines+1 {
		t.Fatalf("goroutine 数量增长: %d -> %d", first.Goroutines, last.Goroutines)
	}
	assertNoRetainedLinearGrowth(t, "HeapAlloc", snapshots, func(item saveMemorySnapshot) uint64 { return item.HeapAlloc }, 64*1024, 4*1024*1024)
	assertNoRetainedLinearGrowth(t, "HeapObjects", snapshots, func(item saveMemorySnapshot) uint64 { return item.HeapObjects }, 50, 5000)
	assertNoRetainedLinearGrowth(t, "HeapInuse", snapshots, func(item saveMemorySnapshot) uint64 { return item.HeapInuse }, 256*1024, 8*1024*1024)
}

func assertNoRetainedLinearGrowth(t *testing.T, name string, snapshots []saveMemorySnapshot, value func(saveMemorySnapshot) uint64, linearThreshold, absoluteThreshold uint64) {
	t.Helper()
	first := value(snapshots[0])
	last := value(snapshots[len(snapshots)-1])
	if last <= first {
		return
	}
	growth := last - first
	if growth > absoluteThreshold {
		t.Fatalf("GC 后 %s 保留量增长过大: %d -> %d (+%d)", name, first, last, growth)
	}
	if growth < linearThreshold {
		return
	}
	values := make([]uint64, len(snapshots))
	for index, snapshot := range snapshots {
		values[index] = value(snapshot)
	}
	if saveMemoryLinearRSquared(values) >= 0.90 {
		t.Fatalf("GC 后 %s 随保存次数近似线性增长: %v", name, values)
	}
}

func saveMemoryLinearRSquared(values []uint64) float64 {
	if len(values) < 3 {
		return 0
	}
	var sumX, sumY float64
	for index, value := range values {
		sumX += float64(index)
		sumY += float64(value)
	}
	meanX := sumX / float64(len(values))
	meanY := sumY / float64(len(values))
	var covariance, varianceX, varianceY float64
	for index, value := range values {
		dx := float64(index) - meanX
		dy := float64(value) - meanY
		covariance += dx * dy
		varianceX += dx * dx
		varianceY += dy * dy
	}
	if varianceX == 0 || varianceY == 0 || covariance <= 0 {
		return 0
	}
	return covariance * covariance / (varianceX * varianceY)
}

func assertOpenAPIStatsRecorderIdle(t *testing.T, server *Server, original *openAPIStatsRecorder) {
	t.Helper()
	if server.openAPIStats != original {
		t.Fatal("保存操作替换了 Open API 统计 recorder")
	}
	started, pending := openAPIStatsState(server.openAPIStats)
	if started || pending != 0 {
		t.Fatalf("管理端保存不应启动或写入 Open API 统计 recorder: started=%t pending=%d", started, pending)
	}
}

func assertSaveResponseOK(t *testing.T, status int, body string) {
	t.Helper()
	if status != http.StatusOK {
		t.Fatalf("保存请求返回 %d: %s", status, body)
	}
}

func writeSaveMemoryHeapProfile(t *testing.T, name string) {
	t.Helper()
	profileDir := os.Getenv("ALLBOT_SAVE_MEMORY_HEAP_PROFILE")
	if profileDir == "" {
		return
	}
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatalf("创建 heap profile 目录失败: %v", err)
	}
	path := filepath.Join(profileDir, fmt.Sprintf("%s.heap.pprof", name))
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("创建 heap profile 失败: %v", err)
	}
	if err := pprof.WriteHeapProfile(file); err != nil {
		_ = file.Close()
		t.Fatalf("写入 heap profile 失败: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("关闭 heap profile 失败: %v", err)
	}
	t.Logf("heap profile 已写入 %s", path)
}
