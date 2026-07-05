package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	webadapter "github.com/allbot/allbot/core/adapter/web"
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
	for _, platform := range []string{"dingtalk", "qq", "telegram", "qq_office", "wechat_official", "web"} {
		if byPlatform[platform] == nil {
			t.Fatalf("missing platform %s in %#v", platform, items)
		}
	}
	dingTalk := byPlatform["dingtalk"]
	if dingTalk["display_name"] != "钉钉机器人（Stream）" {
		t.Fatalf("dingtalk display_name = %#v", dingTalk["display_name"])
	}
	dingTalkSchema, ok := dingTalk["config_schema"].([]interface{})
	if !ok || len(dingTalkSchema) == 0 {
		t.Fatalf("dingtalk config_schema = %#v", dingTalk["config_schema"])
	}
	dingTalkKeys := make(map[string]bool)
	for _, field := range dingTalkSchema {
		fieldMap, ok := field.(map[string]interface{})
		if !ok {
			continue
		}
		key, _ := fieldMap["key"].(string)
		dingTalkKeys[key] = true
	}
	if !dingTalkKeys["client_id"] || !dingTalkKeys["client_secret"] || !dingTalkKeys["robot_code"] || !dingTalkKeys["open_api_host"] || !dingTalkKeys["proxy_url"] {
		t.Fatalf("dingtalk schema keys = %#v", dingTalkKeys)
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

	wechatOfficial := byPlatform["wechat_official"]
	if wechatOfficial["display_name"] != "微信公众号" {
		t.Fatalf("wechat_official display_name = %#v", wechatOfficial["display_name"])
	}
	wechatSchema, ok := wechatOfficial["config_schema"].([]interface{})
	if !ok || len(wechatSchema) == 0 {
		t.Fatalf("wechat_official config_schema = %#v", wechatOfficial["config_schema"])
	}
	wechatKeys := make(map[string]bool)
	for _, field := range wechatSchema {
		fieldMap, ok := field.(map[string]interface{})
		if !ok {
			continue
		}
		key, _ := fieldMap["key"].(string)
		wechatKeys[key] = true
	}
	if !wechatKeys["app_id"] || !wechatKeys["app_secret"] || !wechatKeys["token"] || !wechatKeys["callback_path"] {
		t.Fatalf("wechat_official schema keys = %#v", wechatKeys)
	}

	web := byPlatform["web"]
	webSchema, ok := web["config_schema"].([]interface{})
	if !ok || len(webSchema) == 0 {
		t.Fatalf("web config_schema = %#v", web["config_schema"])
	}
	var subjectField map[string]interface{}
	for _, field := range webSchema {
		fieldMap, ok := field.(map[string]interface{})
		if !ok {
			continue
		}
		if fieldMap["key"] == "smtp_subject" {
			subjectField = fieldMap
			break
		}
	}
	if subjectField == nil {
		t.Fatalf("missing smtp_subject in web schema: %#v", webSchema)
	}
	if required, _ := subjectField["required"].(bool); required {
		t.Fatalf("smtp_subject should not be required: %#v", subjectField)
	}
	if subjectField["default"] != webadapter.DefaultSMTPSubject {
		t.Fatalf("smtp_subject default = %#v, want %q", subjectField["default"], webadapter.DefaultSMTPSubject)
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
