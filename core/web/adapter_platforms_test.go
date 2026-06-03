package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleAdapterPlatforms(t *testing.T) {
	server := &Server{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/adapter-platforms", nil)

	server.handleAdapterPlatforms(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "ParseConfig") || strings.Contains(recorder.Body.String(), "NewAdapter") {
		t.Fatalf("response should not expose function fields: %s", recorder.Body.String())
	}
	var items []map[string]interface{}
	if err := json.NewDecoder(recorder.Body).Decode(&items); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	byPlatform := make(map[string]map[string]interface{})
	for _, item := range items {
		platform, _ := item["platform"].(string)
		if platform != "" {
			byPlatform[platform] = item
		}
	}
	for _, platform := range []string{"qq", "telegram", "qq_office"} {
		if byPlatform[platform] == nil {
			t.Fatalf("missing platform %s in %#v", platform, items)
		}
	}
	qqOffice := byPlatform["qq_office"]
	if qqOffice["display_name"] != "QQ 官方机器人" {
		t.Fatalf("qq_office display_name = %#v", qqOffice["display_name"])
	}
	schema, ok := qqOffice["config_schema"].([]interface{})
	if !ok || len(schema) == 0 {
		t.Fatalf("qq_office config_schema = %#v", qqOffice["config_schema"])
	}
	keys := make(map[string]bool)
	for _, field := range schema {
		fieldMap, ok := field.(map[string]interface{})
		if !ok {
			continue
		}
		key, _ := fieldMap["key"].(string)
		keys[key] = true
	}
	if !keys["app_id"] || !keys["client_secret"] {
		t.Fatalf("qq_office schema keys = %#v", keys)
	}
}

func TestHandleAdapterPlatformsRejectsNonGet(t *testing.T) {
	server := &Server{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/adapter-platforms", nil)

	server.handleAdapterPlatforms(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", recorder.Code)
	}
}
