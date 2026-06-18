package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
