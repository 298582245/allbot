package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/allbot/allbot/core/types"
)

func TestAccountQLCreatePersistsCompleteTemplateSourceAndHashes(t *testing.T) {
	withTempWorkdir(t, func() {
		server := testServer(t)
		recorder := performPluginCreateRequest(t, server, http.MethodPost, "/api/plugins", validAccountQLCreateRequest(), server.handleCreatePlugin)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected create ok, got %d: %s", recorder.Code, recorder.Body.String())
		}
		configValue := readPluginJSON(t, filepath.Join("plugins", "ql_demo"))
		source, ok := configValue["template_source"].(map[string]interface{})
		if !ok {
			t.Fatalf("template_source missing: %#v", configValue)
		}
		account, ok := source["account_ql"].(map[string]interface{})
		if !ok || account["parse_input_code"] != accountQLParseInputCode || account["query_code"] != accountQLQueryCode || account["check_ck_code"] != accountQLCheckCKCode {
			t.Fatalf("template_source does not retain complete source: %#v", source)
		}
		files, ok := source["files"].([]interface{})
		if !ok || len(files) != 3 {
			t.Fatalf("unexpected generated file hashes: %#v", source["files"])
		}
		for _, raw := range files {
			file := raw.(map[string]interface{})
			pathValue := file["path"].(string)
			if file["role"] == "config" {
				config, err := readPluginConfig(filepath.Join("plugins", "ql_demo"))
				if err != nil {
					t.Fatal(err)
				}
				if file["sha256"] != pluginConfigSHA256(config) {
					t.Fatalf("config hash mismatch: %#v", file)
				}
				continue
			}
			data, err := os.ReadFile(filepath.Join("plugins", "ql_demo", filepath.FromSlash(pathValue)))
			if err != nil {
				t.Fatal(err)
			}
			if file["sha256"] != sha256Bytes(data) {
				t.Fatalf("hash mismatch for %s: %#v", pathValue, file)
			}
		}
		stored, err := server.pluginTemplateMetadataDatabase().GetPluginTemplateMetadata("ql_demo")
		if err != nil {
			t.Fatal(err)
		}
		if stored == nil || stored.TemplateSource["template"] != "nodejs_account_ql" {
			t.Fatalf("database template source missing: %#v", stored)
		}
	})
}

func TestTemplateEditorGetAndPutRegeneratePlugin(t *testing.T) {
	withTempWorkdir(t, func() {
		server := createAccountQLPluginForTemplateEditor(t)
		get := performTemplateEditorRequest(t, server, http.MethodGet, "ql_demo", nil)
		if get.Code != http.StatusOK {
			t.Fatalf("expected get ok, got %d: %s", get.Code, get.Body.String())
		}
		var state templateEditorState
		if err := json.Unmarshal(get.Body.Bytes(), &state); err != nil {
			t.Fatal(err)
		}
		if !state.Editable || state.RequiresConvert || state.TemplateSource == nil || state.SourceChanged {
			t.Fatalf("unexpected editor state: %#v", state)
		}
		state.TemplateSource.Account.Prefix = "新版"
		state.TemplateSource.Account.QueryCode = strings.Replace(state.TemplateSource.Account.QueryCode, "账号信息", "新版账号信息", 1)
		put := performTemplateEditorRequest(t, server, http.MethodPut, "ql_demo", templateEditorRequest{TemplateSource: state.TemplateSource})
		if put.Code != http.StatusOK {
			t.Fatalf("expected put ok, got %d: %s", put.Code, put.Body.String())
		}
		configValue := readPluginJSON(t, filepath.Join("plugins", "ql_demo"))
		if !strings.Contains(configValue["trigger"].(string), "新版") {
			t.Fatalf("trigger not regenerated: %#v", configValue["trigger"])
		}
		entry := readTextFile(t, filepath.Join("plugins", "ql_demo", "main.js"))
		assertContains(t, entry, `prefix: "新版"`)
		if server.pluginManager.GetPlugin("ql_demo") == nil || server.router.GetPlugin("ql_demo") == nil {
			t.Fatal("updated plugin was not reloaded and registered")
		}
	})
}

func TestTemplateEditorProtectsExternallyModifiedGeneratedFile(t *testing.T) {
	withTempWorkdir(t, func() {
		server := createAccountQLPluginForTemplateEditor(t)
		entryPath := filepath.Join("plugins", "ql_demo", "main.js")
		original := readTextFile(t, entryPath)
		modified := original + "\n// external edit\n"
		if err := os.WriteFile(entryPath, []byte(modified), 0644); err != nil {
			t.Fatal(err)
		}
		get := performTemplateEditorRequest(t, server, http.MethodGet, "ql_demo", nil)
		var state templateEditorState
		if err := json.Unmarshal(get.Body.Bytes(), &state); err != nil {
			t.Fatal(err)
		}
		if !state.SourceChanged || len(state.ModifiedFiles) != 1 || state.ModifiedFiles[0] != "main.js" {
			t.Fatalf("external modification not detected: %#v", state)
		}
		state.TemplateSource.Account.Prefix = "拒绝覆盖"
		put := performTemplateEditorRequest(t, server, http.MethodPut, "ql_demo", templateEditorRequest{TemplateSource: state.TemplateSource})
		if put.Code != http.StatusConflict {
			t.Fatalf("expected conflict, got %d: %s", put.Code, put.Body.String())
		}
		if current := readTextFile(t, entryPath); current != modified {
			t.Fatal("conflicting save overwrote the external modification")
		}
	})
}

func TestTemplateEditorProtectsPluginConfigModification(t *testing.T) {
	withTempWorkdir(t, func() {
		server := createAccountQLPluginForTemplateEditor(t)
		pluginRoot := filepath.Join("plugins", "ql_demo")
		configValue, err := readPluginConfig(pluginRoot)
		if err != nil {
			t.Fatal(err)
		}
		configValue.Pinned = true
		data, _ := json.MarshalIndent(configValue, "", "  ")
		if err := os.WriteFile(filepath.Join(pluginRoot, "plugin.json"), data, 0644); err != nil {
			t.Fatal(err)
		}
		get := performTemplateEditorRequest(t, server, http.MethodGet, "ql_demo", nil)
		var state templateEditorState
		if err := json.Unmarshal(get.Body.Bytes(), &state); err != nil {
			t.Fatal(err)
		}
		if !state.SourceChanged || !containsString(state.ModifiedFiles, "plugin.json") {
			t.Fatalf("plugin.json modification was not detected: %#v", state)
		}
	})
}

func TestTemplateEditorRejectsUnexpectedGeneratedTarget(t *testing.T) {
	withTempWorkdir(t, func() {
		server := createAccountQLPluginForTemplateEditor(t)
		pluginRoot := filepath.Join("plugins", "ql_demo")
		get := performTemplateEditorRequest(t, server, http.MethodGet, "ql_demo", nil)
		var state templateEditorState
		if err := json.Unmarshal(get.Body.Bytes(), &state); err != nil {
			t.Fatal(err)
		}
		state.TemplateSource.Account.TaskScript = "scripts/custom.js"
		customPath := filepath.Join(pluginRoot, "scripts", "custom.js")
		if err := os.WriteFile(customPath, []byte("user content"), 0644); err != nil {
			t.Fatal(err)
		}
		put := performTemplateEditorRequest(t, server, http.MethodPut, "ql_demo", templateEditorRequest{TemplateSource: state.TemplateSource})
		if put.Code != http.StatusConflict || readTextFile(t, customPath) != "user content" {
			t.Fatalf("unexpected target was overwritten: %d %s", put.Code, put.Body.String())
		}
	})
}

func TestTemplateEditorExplicitlyConvertsLegacyAccountPlugin(t *testing.T) {
	withTempWorkdir(t, func() {
		server := createAccountQLPluginForTemplateEditor(t)
		pluginRoot := filepath.Join("plugins", "ql_demo")
		configValue := readPluginJSON(t, pluginRoot)
		delete(configValue, "template_source")
		data, err := json.MarshalIndent(configValue, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pluginRoot, "plugin.json"), data, 0644); err != nil {
			t.Fatal(err)
		}
		if err := server.pluginTemplateMetadataDatabase().DeletePluginTemplateMetadata("ql_demo"); err != nil {
			t.Fatal(err)
		}

		get := performTemplateEditorRequest(t, server, http.MethodGet, "ql_demo", nil)
		var before templateEditorState
		if err := json.Unmarshal(get.Body.Bytes(), &before); err != nil {
			t.Fatal(err)
		}
		if before.Editable || !before.RequiresConvert || before.ConversionSource == nil {
			t.Fatalf("legacy plugin should require conversion: %#v", before)
		}
		if before.ConversionSource.Account.ParseInputCode != "" || before.ConversionSource.Account.QueryCode != "" || before.ConversionSource.Account.CheckCKCode != "" {
			t.Fatalf("legacy conversion must not infer code from entry: %#v", before.ConversionSource.Account)
		}
		withoutFlag := performTemplateEditorRequest(t, server, http.MethodPut, "ql_demo", templateEditorRequest{})
		if withoutFlag.Code != http.StatusBadRequest {
			t.Fatalf("implicit conversion should fail, got %d: %s", withoutFlag.Code, withoutFlag.Body.String())
		}
		withoutSource := performTemplateEditorRequest(t, server, http.MethodPut, "ql_demo", templateEditorRequest{ConvertLegacy: true})
		if withoutSource.Code != http.StatusBadRequest {
			t.Fatalf("conversion without complete source should fail, got %d: %s", withoutSource.Code, withoutSource.Body.String())
		}
		before.ConversionSource.Account.ParseInputCode = accountQLParseInputCode
		before.ConversionSource.Account.QueryCode = accountQLQueryCode
		before.ConversionSource.Account.CheckCKCode = accountQLCheckCKCode
		converted := performTemplateEditorRequest(t, server, http.MethodPut, "ql_demo", templateEditorRequest{
			ConvertLegacy: true, TemplateSource: before.ConversionSource, OverwriteGeneratedFiles: true,
		})
		if converted.Code != http.StatusOK {
			t.Fatalf("explicit conversion failed, got %d: %s", converted.Code, converted.Body.String())
		}
		var after templateEditorState
		if err := json.Unmarshal(converted.Body.Bytes(), &after); err != nil {
			t.Fatal(err)
		}
		if !after.Editable || after.RequiresConvert || after.TemplateSource == nil {
			t.Fatalf("unexpected converted state: %#v", after)
		}
	})
}

func TestTemplateEditorPreservesUnmanagedPluginConfig(t *testing.T) {
	withTempWorkdir(t, func() {
		server := createAccountQLPluginForTemplateEditor(t)
		pluginRoot := filepath.Join("plugins", "ql_demo")
		configValue, err := readPluginConfig(pluginRoot)
		if err != nil {
			t.Fatal(err)
		}
		webChatEnabled := true
		configValue.Pinned = true
		configValue.AllowedAdapterIDs = []string{"qq:demo"}
		configValue.Dependencies = map[string]string{"axios": "^1.0.0"}
		configValue.AccessControl = &types.AccessControlConfig{InheritSystem: false, WhitelistGroups: []string{"group-1"}}
		configValue.OpenAPI = types.OpenAPIConfig{Enabled: true, Path: "custom", Method: "POST", Token: "secret", Runtime: "nodejs"}
		configValue.WebUI = types.PluginWebUIConfig{Enabled: true, Title: "面板", Entry: "web/index.html"}
		configValue.WebChat = types.PluginWebChatConfig{Enabled: &webChatEnabled, Title: "会话"}
		configValue.UserConfigSchema = append(configValue.UserConfigSchema, types.PluginUserConfigField{Key: "custom_key", Label: "自定义", Type: "text"})
		configValue.UserConfig["custom_key"] = "keep"
		data, err := json.MarshalIndent(configValue, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pluginRoot, "plugin.json"), data, 0644); err != nil {
			t.Fatal(err)
		}

		get := performTemplateEditorRequest(t, server, http.MethodGet, "ql_demo", nil)
		var state templateEditorState
		if err := json.Unmarshal(get.Body.Bytes(), &state); err != nil {
			t.Fatal(err)
		}
		state.TemplateSource.Account.EnableCKCheck = boolPtr(false)
		put := performTemplateEditorRequest(t, server, http.MethodPut, "ql_demo", templateEditorRequest{TemplateSource: state.TemplateSource, OverwriteGeneratedFiles: true})
		if put.Code != http.StatusOK {
			t.Fatalf("expected update ok, got %d: %s", put.Code, put.Body.String())
		}
		updated, err := readPluginConfig(pluginRoot)
		if err != nil {
			t.Fatal(err)
		}
		if !updated.Pinned || len(updated.AllowedAdapterIDs) != 1 || updated.Dependencies["axios"] != "^1.0.0" {
			t.Fatalf("unmanaged basic config was lost: %#v", updated)
		}
		if updated.AccessControl == nil || len(updated.AccessControl.WhitelistGroups) != 1 || !updated.OpenAPI.Enabled || !updated.WebUI.Enabled || updated.WebChat.Enabled == nil || !*updated.WebChat.Enabled {
			t.Fatalf("unmanaged feature config was lost: %#v", updated)
		}
		if updated.UserConfig["custom_key"] != "keep" {
			t.Fatalf("custom user config was lost: %#v", updated.UserConfig)
		}
		if _, exists := updated.UserConfig["ck_check_cron"]; exists {
			t.Fatalf("disabled template config was not removed: %#v", updated.UserConfig)
		}
	})
}

func TestTemplateEditorValidationFailureDoesNotChangeState(t *testing.T) {
	withTempWorkdir(t, func() {
		server := createAccountQLPluginForTemplateEditor(t)
		pluginRoot := filepath.Join("plugins", "ql_demo")
		originalConfig := readTextFile(t, filepath.Join(pluginRoot, "plugin.json"))
		originalEntry := readTextFile(t, filepath.Join(pluginRoot, "main.js"))
		originalMetadata, err := server.pluginTemplateMetadataDatabase().GetPluginTemplateMetadata("ql_demo")
		if err != nil {
			t.Fatal(err)
		}
		get := performTemplateEditorRequest(t, server, http.MethodGet, "ql_demo", nil)
		var state templateEditorState
		if err := json.Unmarshal(get.Body.Bytes(), &state); err != nil {
			t.Fatal(err)
		}
		state.TemplateSource.Account.ParseInputCode = ""
		put := performTemplateEditorRequest(t, server, http.MethodPut, "ql_demo", templateEditorRequest{TemplateSource: state.TemplateSource})
		if put.Code != http.StatusBadRequest {
			t.Fatalf("expected validation failure, got %d: %s", put.Code, put.Body.String())
		}
		if readTextFile(t, filepath.Join(pluginRoot, "plugin.json")) != originalConfig || readTextFile(t, filepath.Join(pluginRoot, "main.js")) != originalEntry {
			t.Fatal("validation failure changed generated files")
		}
		currentMetadata, err := server.pluginTemplateMetadataDatabase().GetPluginTemplateMetadata("ql_demo")
		if err != nil {
			t.Fatal(err)
		}
		beforeJSON, _ := json.Marshal(originalMetadata.TemplateSource)
		afterJSON, _ := json.Marshal(currentMetadata.TemplateSource)
		if string(beforeJSON) != string(afterJSON) {
			t.Fatal("validation failure changed database metadata")
		}
	})
}

func TestTemplateEditorGetFallbackStates(t *testing.T) {
	withTempWorkdir(t, func() {
		server := testServer(t)
		if err := os.MkdirAll(filepath.Join("plugins", "plain"), 0755); err != nil {
			t.Fatal(err)
		}
		plain, _ := json.Marshal(types.PluginConfig{Name: "普通插件", Runtime: "nodejs", Entry: "main.js", Template: "basic"})
		if err := os.WriteFile(filepath.Join("plugins", "plain", "plugin.json"), plain, 0644); err != nil {
			t.Fatal(err)
		}
		ordinary := performTemplateEditorRequest(t, server, http.MethodGet, "plain", nil)
		var state templateEditorState
		if ordinary.Code != http.StatusOK || json.Unmarshal(ordinary.Body.Bytes(), &state) != nil || state.Editable || state.RequiresConvert {
			t.Fatalf("unexpected ordinary plugin state: %d %s", ordinary.Code, ordinary.Body.String())
		}
		if missing := performTemplateEditorRequest(t, server, http.MethodGet, "missing", nil); missing.Code != http.StatusNotFound {
			t.Fatalf("missing plugin status = %d", missing.Code)
		}
		if invalid := performTemplateEditorRequest(t, server, http.MethodGet, "../invalid", nil); invalid.Code != http.StatusBadRequest {
			t.Fatalf("invalid plugin id status = %d", invalid.Code)
		}
	})
}

func TestTemplateEditorRestoresFilesWhenReloadFails(t *testing.T) {
	withTempWorkdir(t, func() {
		server := createAccountQLPluginForTemplateEditor(t)
		pluginRoot := filepath.Join("plugins", "ql_demo")
		originalConfig := readTextFile(t, filepath.Join(pluginRoot, "plugin.json"))
		originalEntry := readTextFile(t, filepath.Join(pluginRoot, "main.js"))
		get := performTemplateEditorRequest(t, server, http.MethodGet, "ql_demo", nil)
		var state templateEditorState
		if err := json.Unmarshal(get.Body.Bytes(), &state); err != nil {
			t.Fatal(err)
		}
		state.TemplateSource.Plugin.Runtime = "python"
		state.TemplateSource.Template = "python_account_ql"
		state.TemplateSource.Account.ScriptRuntime = "python"
		state.TemplateSource.Account.TaskScript = "scripts/demo_task.py"
		state.TemplateSource.Account.ParseInputCode = pythonAccountQLParseInputCode
		state.TemplateSource.Account.QueryCode = pythonAccountQLQueryCode
		state.TemplateSource.Account.CheckCKCode = pythonAccountQLCheckCKCode
		state.TemplateSource.Account.TaskScript = "blocked/demo_task.py"
		// 将新任务脚本的父路径预先建成普通文件，预提交会失败且原文件必须保持不变。
		if err := os.WriteFile(filepath.Join(pluginRoot, "blocked"), []byte("not a directory"), 0644); err != nil {
			t.Fatal(err)
		}
		put := performTemplateEditorRequest(t, server, http.MethodPut, "ql_demo", templateEditorRequest{TemplateSource: state.TemplateSource})
		if put.Code == http.StatusOK {
			t.Fatalf("expected update failure, got %d: %s", put.Code, put.Body.String())
		}
		if current := readTextFile(t, filepath.Join(pluginRoot, "plugin.json")); current != originalConfig {
			t.Fatal("plugin.json changed after failed transactional update")
		}
		if current := readTextFile(t, filepath.Join(pluginRoot, "main.js")); current != originalEntry {
			t.Fatal("entry changed after failed transactional update")
		}
	})
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func createAccountQLPluginForTemplateEditor(t *testing.T) *Server {
	t.Helper()
	server := testServer(t)
	recorder := performPluginCreateRequest(t, server, http.MethodPost, "/api/plugins", validAccountQLCreateRequest(), server.handleCreatePlugin)
	if recorder.Code != http.StatusOK {
		t.Fatalf("create plugin failed: %d: %s", recorder.Code, recorder.Body.String())
	}
	return server
}

func performTemplateEditorRequest(t *testing.T, server *Server, method, pluginID string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	payload := ""
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		payload = string(data)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, "/api/plugins/template-editor/"+pluginID, strings.NewReader(payload))
	server.handlePluginTemplateEditor(recorder, request)
	return recorder
}
