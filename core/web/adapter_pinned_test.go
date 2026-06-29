package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/allbot/allbot/core/config"
)

func TestHandleAdaptersSortsPinnedBeforeUnpinned(t *testing.T) {
	server := testServer(t)
	db := server.adapterManager.GetDatabase()
	if err := db.SaveAdapter(&config.AdapterConfig{Platform: "qq", Remark: "未置顶", Enabled: true, Config: `{}`}); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveAdapter(&config.AdapterConfig{Platform: "telegram", Remark: "置顶一", Enabled: true, Pinned: true, Config: `{}`}); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveAdapter(&config.AdapterConfig{Platform: "wechat", Remark: "置顶二", Enabled: true, Pinned: true, Config: `{}`}); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/adapters", nil)
	server.handleAdapters(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var adapters []map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &adapters); err != nil {
		t.Fatal(err)
	}
	remarks := []string{adapters[0]["remark"].(string), adapters[1]["remark"].(string), adapters[2]["remark"].(string)}
	if remarks[0] != "置顶一" || remarks[1] != "置顶二" || remarks[2] != "未置顶" {
		t.Fatalf("unexpected adapter order: %#v", remarks)
	}
	if adapters[0]["pinned"] != true || adapters[2]["pinned"] != false {
		t.Fatalf("unexpected pinned fields: %#v", adapters)
	}
}

func TestHandleAdapterActionPinsAndUnpins(t *testing.T) {
	server := testServer(t)
	adapter := &config.AdapterConfig{Platform: "qq", Remark: "演示", Enabled: true, Config: `{}`}
	if err := server.adapterManager.GetDatabase().SaveAdapter(adapter); err != nil {
		t.Fatal(err)
	}

	performAdapterPinnedAction(t, server, adapter.ID, "pin")
	assertAdapterPinned(t, server, adapter.ID, true)
	performAdapterPinnedAction(t, server, adapter.ID, "unpin")
	assertAdapterPinned(t, server, adapter.ID, false)
}

func performAdapterPinnedAction(t *testing.T, server *Server, id int64, action string) {
	t.Helper()
	body := []byte(`{"action":"` + action + `"}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/adapters/"+strconv.FormatInt(id, 10), bytes.NewReader(body))
	server.handleAdapterDetail(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for %s, got %d: %s", action, recorder.Code, recorder.Body.String())
	}
}

func assertAdapterPinned(t *testing.T, server *Server, id int64, pinned bool) {
	t.Helper()
	adapter, err := server.adapterManager.GetDatabase().GetAdapterByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if adapter == nil || adapter.Pinned != pinned {
		t.Fatalf("expected pinned=%v, got %#v", pinned, adapter)
	}
}
