package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/allbot/allbot/core/config"
)

func TestHybridTemplateEditorGetPutSingleAndMultipleSections(t *testing.T) {
	withTempWorkdir(t, func() {
		server := createAccountQLPluginForTemplateEditor(t)
		root := filepath.Join("plugins", "ql_demo")
		entry := readTextFile(t, filepath.Join(root, "main.js"))
		task := readTextFile(t, filepath.Join(root, "scripts", "demo_task.js"))
		source := accountQLTemplateSource{
			Version: 2, Mode: "hybrid", Template: "nodejs_account_ql", Plugin: accountQLPluginSource{ID: "ql_demo", Template: "nodejs_account_ql", Name: "青龙演示", Version: "1.0.0", Runtime: "nodejs"},
			Files: []templateSourceFile{
				{Path: "main.js", Role: "entry", Ownership: "patchable", SHA256: sha256Text(entry)},
				{Path: "scripts/demo_task.js", Role: "task_script", Ownership: "preserved", SHA256: sha256Text(task)},
			},
			Sections: []templateSourceSection{
				{ID: "entry-prefix", Category: "login", Label: "入口前缀", Path: "main.js", Content: `prefix: "演示"`},
				{ID: "entry-query", Category: "query", Label: "查询代码", Path: "main.js", Content: "async function query"},
			},
			TaskScripts: map[string]interface{}{"reference_existing": true}, Compatibility: map[string]interface{}{"v1": true}, Migration: map[string]interface{}{"source": "test"},
		}
		metadata := &config.PluginTemplateMetadata{PluginID: "ql_demo", Template: "nodejs_account_ql", TemplateVersion: "3.0.0", Runtime: "nodejs", Structure: "account_ql", TemplateSource: templateSourceMapForTest(t, &source)}
		if err := server.pluginTemplateMetadataDatabase().SavePluginTemplateMetadata(metadata); err != nil {
			t.Fatal(err)
		}
		configValue := readPluginJSON(t, root)
		configValue["template_source"] = templateSourceMapForTest(t, &source)
		configData, _ := json.MarshalIndent(configValue, "", "  ")
		if err := os.WriteFile(filepath.Join(root, "plugin.json"), configData, 0644); err != nil {
			t.Fatal(err)
		}
		get := performTemplateEditorRequest(t, server, http.MethodGet, "ql_demo", nil)
		if get.Code != http.StatusOK {
			t.Fatalf("get status=%d body=%s", get.Code, get.Body.String())
		}
		var state templateEditorState
		decodeUnifiedResponseData(t, get, &state)
		if state.Version != 2 || state.Mode != "hybrid" || !state.Editable || len(state.TemplateSource.Sections) != 2 {
			t.Fatalf("unexpected v2 state: %#v", state)
		}
		updated := *state.TemplateSource
		updated.Sections = append([]templateSourceSection(nil), state.TemplateSource.Sections...)
		updated.Sections[0].Content = `prefix: "混合"`
		updated.Sections[1].Content = "async function query"
		put := performTemplateEditorRequest(t, server, http.MethodPut, "ql_demo", templateEditorRequest{TemplateSource: &updated, OverwriteGeneratedFiles: true, Force: true})
		if put.Code != http.StatusOK {
			t.Fatalf("put status=%d body=%s", put.Code, put.Body.String())
		}
		assertContains(t, readTextFile(t, filepath.Join(root, "main.js")), `prefix: "混合"`)
		if readTextFile(t, filepath.Join(root, "scripts", "demo_task.js")) != task {
			t.Fatal("preserved file changed")
		}
	})
}

func TestHybridTemplateEditorSupportsRegisteredSourceWithoutPluginTemplateFields(t *testing.T) {
	withTempWorkdir(t, func() {
		server := createAccountQLPluginForTemplateEditor(t)
		root := filepath.Join("plugins", "ql_demo")
		entry := readTextFile(t, filepath.Join(root, "main.js"))
		source := accountQLTemplateSource{
			Version: 2, Mode: "hybrid", Template: "nodejs_account_ql",
			Plugin:   accountQLPluginSource{ID: "ql_demo", Template: "nodejs_account_ql", Name: "青龙演示", Version: "1.0.0", Runtime: "nodejs"},
			Files:    []templateSourceFile{{Path: "main.js", Role: "entry", Ownership: "patchable", SHA256: sha256Text(entry)}},
			Sections: []templateSourceSection{{ID: "entry-prefix", Category: "login", Label: "入口前缀", Path: "main.js", Content: `prefix: "演示"`}},
		}
		metadata := &config.PluginTemplateMetadata{
			PluginID: "ql_demo", Template: "nodejs_account_ql", TemplateVersion: "3.0.0", Runtime: "nodejs", Structure: "account_ql",
			Metadata: map[string]interface{}{"migration": "registered"}, TemplateSource: templateSourceMapForTest(t, &source),
		}
		if err := server.pluginTemplateMetadataDatabase().SavePluginTemplateMetadata(metadata); err != nil {
			t.Fatal(err)
		}
		configValue := readPluginJSON(t, root)
		delete(configValue, "template")
		delete(configValue, "template_version")
		configValue["template_source"] = templateSourceMapForTest(t, &source)
		configData, err := json.MarshalIndent(configValue, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "plugin.json"), configData, 0644); err != nil {
			t.Fatal(err)
		}

		get := performTemplateEditorRequest(t, server, http.MethodGet, "ql_demo", nil)
		if get.Code != http.StatusOK {
			t.Fatalf("get status=%d body=%s", get.Code, get.Body.String())
		}
		var state templateEditorState
		decodeUnifiedResponseData(t, get, &state)
		if !state.Editable || state.Version != hybridTemplateSourceVersion || state.Template != "nodejs_account_ql" || state.TemplateVersion != "3.0.0" {
			t.Fatalf("unexpected legacy v2 state: %#v", state)
		}

		templateChanged := *state.TemplateSource
		templateChanged.Template = "python_account_ql"
		templateChanged.Plugin.Template = "python_account_ql"
		if response := performTemplateEditorRequest(t, server, http.MethodPut, "ql_demo", templateEditorRequest{TemplateSource: &templateChanged}); response.Code != http.StatusBadRequest {
			t.Fatalf("template tamper status=%d body=%s", response.Code, response.Body.String())
		}
		runtimeChanged := *state.TemplateSource
		runtimeChanged.Plugin.Runtime = "python"
		if response := performTemplateEditorRequest(t, server, http.MethodPut, "ql_demo", templateEditorRequest{TemplateSource: &runtimeChanged}); response.Code != http.StatusBadRequest {
			t.Fatalf("runtime tamper status=%d body=%s", response.Code, response.Body.String())
		}

		updated := *state.TemplateSource
		updated.Sections = append([]templateSourceSection(nil), state.TemplateSource.Sections...)
		updated.Sections[0].Content = `prefix: "兼容"`
		put := performTemplateEditorRequest(t, server, http.MethodPut, "ql_demo", templateEditorRequest{TemplateSource: &updated})
		if put.Code != http.StatusOK {
			t.Fatalf("put status=%d body=%s", put.Code, put.Body.String())
		}
		assertContains(t, readTextFile(t, filepath.Join(root, "main.js")), `prefix: "兼容"`)
		stored, err := server.pluginTemplateMetadataDatabase().GetPluginTemplateMetadata("ql_demo")
		if err != nil {
			t.Fatal(err)
		}
		if stored == nil || stored.Template != "nodejs_account_ql" || stored.TemplateVersion != "3.0.0" || stored.Runtime != "nodejs" {
			t.Fatalf("registered template metadata was not preserved: %#v", stored)
		}
	})
}

func TestHybridTemplateEditorRejectsDriftMissingDuplicateAndUnsafeInput(t *testing.T) {
	withTempWorkdir(t, func() {
		server := createAccountQLPluginForTemplateEditor(t)
		root := filepath.Join("plugins", "ql_demo")
		entry := readTextFile(t, filepath.Join(root, "main.js"))
		source := accountQLTemplateSource{Version: 2, Mode: "hybrid", Template: "nodejs_account_ql", Plugin: accountQLPluginSource{ID: "ql_demo", Template: "nodejs_account_ql", Name: "青龙演示", Version: "1.0.0", Runtime: "nodejs"}, Files: []templateSourceFile{{Path: "main.js", Role: "entry", Ownership: "patchable", SHA256: sha256Text(entry)}}, Sections: []templateSourceSection{{ID: "one", Category: "login", Label: "一", Path: "main.js", Content: `prefix: "演示"`}}}
		if err := server.pluginTemplateMetadataDatabase().SavePluginTemplateMetadata(&config.PluginTemplateMetadata{PluginID: "ql_demo", Template: "nodejs_account_ql", TemplateVersion: "3.0.0", Runtime: "nodejs", Structure: "account_ql", TemplateSource: templateSourceMapForTest(t, &source)}); err != nil {
			t.Fatal(err)
		}
		configValue := readPluginJSON(t, root)
		configValue["template_source"] = templateSourceMapForTest(t, &source)
		data, _ := json.MarshalIndent(configValue, "", "  ")
		if err := os.WriteFile(filepath.Join(root, "plugin.json"), data, 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "main.js"), []byte(entry+"\nexternal"), 0644); err != nil {
			t.Fatal(err)
		}
		changed := source
		changed.Sections = append([]templateSourceSection(nil), source.Sections...)
		changed.Sections[0].Content = `prefix: "拒绝"`
		if response := performTemplateEditorRequest(t, server, http.MethodPut, "ql_demo", templateEditorRequest{TemplateSource: &changed, Force: true}); response.Code != http.StatusConflict {
			t.Fatalf("drift status=%d body=%s", response.Code, response.Body.String())
		}
		if got := readTextFile(t, filepath.Join(root, "main.js")); got != entry+"\nexternal" {
			t.Fatal("drift was overwritten")
		}
		changed = source
		changed.Files = []templateSourceFile{{Path: "../escape.js", Role: "entry", Ownership: "patchable", SHA256: source.Files[0].SHA256}}
		if response := performTemplateEditorRequest(t, server, http.MethodPut, "ql_demo", templateEditorRequest{TemplateSource: &changed}); response.Code != http.StatusBadRequest {
			t.Fatalf("unsafe path status=%d", response.Code)
		}
		changed = source
		changed.Sections = []templateSourceSection{{ID: "one", Category: "login", Label: "一", Path: "main.js", Content: "x"}, {ID: "one", Category: "query", Label: "二", Path: "main.js", Content: "y"}}
		if response := performTemplateEditorRequest(t, server, http.MethodPut, "ql_demo", templateEditorRequest{TemplateSource: &changed}); response.Code != http.StatusBadRequest {
			t.Fatalf("duplicate status=%d body=%s", response.Code, response.Body.String())
		}
	})
}

func TestHybridTemplateEditorRejectsOwnershipAndSectionShapeChanges(t *testing.T) {
	withTempWorkdir(t, func() {
		server := createAccountQLPluginForTemplateEditor(t)
		root := filepath.Join("plugins", "ql_demo")
		entry := readTextFile(t, filepath.Join(root, "main.js"))
		source := accountQLTemplateSource{
			Version: 2, Mode: "hybrid", Template: "nodejs_account_ql", Plugin: accountQLPluginSource{ID: "ql_demo", Template: "nodejs_account_ql", Name: "青龙演示", Version: "1.0.0", Runtime: "nodejs"},
			Files:    []templateSourceFile{{Path: "main.js", Role: "entry", Ownership: "patchable", SHA256: sha256Text(entry)}},
			Sections: []templateSourceSection{{ID: "entry-prefix", Category: "login", Label: "入口前缀", Path: "main.js", Content: `prefix: "演示"`}},
		}
		installHybridSourceForTest(t, server, root, &source)
		changed := source
		changed.Files = []templateSourceFile{{Path: "main.js", Role: "entry", Ownership: "preserved", SHA256: source.Files[0].SHA256}}
		if response := performTemplateEditorRequest(t, server, http.MethodPut, "ql_demo", templateEditorRequest{TemplateSource: &changed, Force: true, OverwriteGeneratedFiles: true}); response.Code != http.StatusBadRequest {
			t.Fatalf("ownership status=%d body=%s", response.Code, response.Body.String())
		}
		changed = source
		changed.Sections = nil
		if response := performTemplateEditorRequest(t, server, http.MethodPut, "ql_demo", templateEditorRequest{TemplateSource: &changed, Force: true}); response.Code != http.StatusBadRequest {
			t.Fatalf("missing section status=%d body=%s", response.Code, response.Body.String())
		}
	})
}

func TestHybridTemplateEditorRollsBackAfterReloadFailure(t *testing.T) {
	withTempWorkdir(t, func() {
		server := createAccountQLPluginForTemplateEditor(t)
		root := filepath.Join("plugins", "ql_demo")
		entryPath := filepath.Join(root, "main.js")
		originalEntry := readTextFile(t, entryPath)
		source := accountQLTemplateSource{
			Version: 2, Mode: "hybrid", Template: "nodejs_account_ql", Plugin: accountQLPluginSource{ID: "ql_demo", Template: "nodejs_account_ql", Name: "青龙演示", Version: "1.0.0", Runtime: "nodejs"},
			Files:    []templateSourceFile{{Path: "main.js", Role: "entry", Ownership: "patchable", SHA256: sha256Text(originalEntry)}},
			Sections: []templateSourceSection{{ID: "entry-prefix", Category: "login", Label: "入口前缀", Path: "main.js", Content: `prefix: "演示"`}},
		}
		installHybridSourceForTest(t, server, root, &source)
		beforeConfig := readTextFile(t, filepath.Join(root, "plugin.json"))
		beforeMetadata, err := server.pluginTemplateMetadataDatabase().GetPluginTemplateMetadata("ql_demo")
		if err != nil {
			t.Fatal(err)
		}
		updated := source
		updated.Sections = append([]templateSourceSection(nil), source.Sections...)
		updated.Sections[0].Content = `prefix: "回滚"`
		server.pluginManager = nil
		response := performTemplateEditorRequest(t, server, http.MethodPut, "ql_demo", templateEditorRequest{TemplateSource: &updated})
		if response.Code == http.StatusOK {
			t.Fatalf("expected reload failure, got %d", response.Code)
		}
		if got := readTextFile(t, entryPath); got != originalEntry {
			t.Fatal("entry file was not restored after reload failure")
		}
		if got := readTextFile(t, filepath.Join(root, "plugin.json")); got != beforeConfig {
			t.Fatal("plugin.json was not restored after reload failure")
		}
		afterMetadata, err := server.pluginTemplateMetadataDatabase().GetPluginTemplateMetadata("ql_demo")
		if err != nil {
			t.Fatal(err)
		}
		beforeJSON, _ := json.Marshal(beforeMetadata.TemplateSource)
		afterJSON, _ := json.Marshal(afterMetadata.TemplateSource)
		if string(beforeJSON) != string(afterJSON) {
			t.Fatal("metadata was not restored after reload failure")
		}
	})
}

func TestHybridTemplateEditorAllowsPluginNameAndVersionChanges(t *testing.T) {
	withTempWorkdir(t, func() {
		server := createAccountQLPluginForTemplateEditor(t)
		root := filepath.Join("plugins", "ql_demo")
		entry := readTextFile(t, filepath.Join(root, "main.js"))
		source := accountQLTemplateSource{
			Version: 2, Mode: "hybrid", Template: "nodejs_account_ql",
			Plugin: accountQLPluginSource{ID: "ql_demo", Template: "nodejs_account_ql", Name: "青龙演示", Version: "1.0.0", Runtime: "nodejs"},
			Files:  []templateSourceFile{{Path: "main.js", Role: "entry", Ownership: "patchable", SHA256: sha256Text(entry)}},
		}
		installHybridSourceForTest(t, server, root, &source)
		updated := source
		updated.Plugin.Name = "青龙新版"
		updated.Plugin.Version = "2.0.0"
		response := performTemplateEditorRequest(t, server, http.MethodPut, "ql_demo", templateEditorRequest{TemplateSource: &updated})
		if response.Code != http.StatusOK {
			t.Fatalf("put status=%d body=%s", response.Code, response.Body.String())
		}
		configValue := readPluginJSON(t, root)
		if configValue["name"] != "青龙新版" || configValue["version"] != "2.0.0" {
			t.Fatalf("plugin config was not updated: %#v", configValue)
		}
	})
}

func TestHybridPluginFilesAllowEditingEveryFile(t *testing.T) {
	withTempWorkdir(t, func() {
		server := createAccountQLPluginForTemplateEditor(t)
		root := filepath.Join("plugins", "ql_demo")
		entry := readTextFile(t, filepath.Join(root, "main.js"))
		task := readTextFile(t, filepath.Join(root, "scripts", "demo_task.js"))
		source := accountQLTemplateSource{
			Version: 2, Mode: "hybrid", Template: "nodejs_account_ql",
			Plugin: accountQLPluginSource{ID: "ql_demo", Template: "nodejs_account_ql", Name: "青龙演示", Version: "1.0.0", Runtime: "nodejs"},
			Files: []templateSourceFile{
				{Path: "plugin.json", Role: "config", Ownership: "generated", SHA256: "config-baseline"},
				{Path: "main.js", Role: "entry", Ownership: "patchable", SHA256: sha256Text(entry)},
				{Path: "scripts/demo_task.js", Role: "task_script", Ownership: "preserved", SHA256: sha256Text(task)},
			},
			Sections: []templateSourceSection{
				{ID: "entry-prefix", Category: "login", Label: "入口前缀", Path: "main.js", Content: `prefix: "演示"`},
			},
		}
		installHybridSourceForTest(t, server, root, &source)
		if err := os.WriteFile(filepath.Join(root, "notes.custom"), []byte("unregistered=true"), 0644); err != nil {
			t.Fatal(err)
		}

		pluginJSON := performPluginFileRequest(t, server, http.MethodGet, "ql_demo", "plugin.json", nil)
		if pluginJSON.Code != http.StatusOK {
			t.Fatalf("plugin.json get status=%d body=%s", pluginJSON.Code, pluginJSON.Body.String())
		}
		var pluginFile struct {
			Code     string `json:"code"`
			SHA256   string `json:"sha256"`
			Editable bool   `json:"editable"`
			Text     bool   `json:"text"`
		}
		decodeUnifiedResponseData(t, pluginJSON, &pluginFile)
		if !pluginFile.Editable || !pluginFile.Text {
			t.Fatalf("plugin.json should be editable: %#v", pluginFile)
		}
		var pluginConfig map[string]interface{}
		if err := json.Unmarshal([]byte(pluginFile.Code), &pluginConfig); err != nil {
			t.Fatal(err)
		}
		pluginConfig["name"] = "直接编辑配置"
		nextPluginJSON, err := json.MarshalIndent(pluginConfig, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		putPluginJSON := performPluginFileRequest(t, server, http.MethodPut, "ql_demo", "", map[string]interface{}{
			"path": "plugin.json", "code": string(nextPluginJSON), "expected_sha256": pluginFile.SHA256,
		})
		if putPluginJSON.Code != http.StatusOK {
			t.Fatalf("plugin.json put status=%d body=%s", putPluginJSON.Code, putPluginJSON.Body.String())
		}
		if got := readPluginJSON(t, root)["name"]; got != "直接编辑配置" {
			t.Fatalf("plugin.json name=%v", got)
		}

		for _, testCase := range []struct {
			name string
			path string
			code string
		}{
			{name: "preserved script", path: "scripts/demo_task.js", code: task + "\n// direct edit"},
			{name: "unregistered extension", path: "notes.custom", code: "unregistered=false"},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				get := performPluginFileRequest(t, server, http.MethodGet, "ql_demo", testCase.path, nil)
				if get.Code != http.StatusOK {
					t.Fatalf("get status=%d body=%s", get.Code, get.Body.String())
				}
				var file struct {
					SHA256   string `json:"sha256"`
					Editable bool   `json:"editable"`
					Text     bool   `json:"text"`
				}
				decodeUnifiedResponseData(t, get, &file)
				if !file.Editable || !file.Text {
					t.Fatalf("file should be editable: %#v", file)
				}
				put := performPluginFileRequest(t, server, http.MethodPut, "ql_demo", "", map[string]interface{}{
					"path": testCase.path, "code": testCase.code, "expected_sha256": file.SHA256,
				})
				if put.Code != http.StatusOK {
					t.Fatalf("put status=%d body=%s", put.Code, put.Body.String())
				}
				if got := readTextFile(t, filepath.Join(root, filepath.FromSlash(testCase.path))); got != testCase.code {
					t.Fatalf("file content=%q", got)
				}
			})
		}
	})
}

func TestHybridPluginFilesAllowCreateDeleteAndDetectConflicts(t *testing.T) {
	withTempWorkdir(t, func() {
		server := createAccountQLPluginForTemplateEditor(t)
		root := filepath.Join("plugins", "ql_demo")
		entry := readTextFile(t, filepath.Join(root, "main.js"))
		source := accountQLTemplateSource{
			Version: 2, Mode: "hybrid", Template: "nodejs_account_ql",
			Plugin:   accountQLPluginSource{ID: "ql_demo", Template: "nodejs_account_ql", Name: "青龙演示", Version: "1.0.0", Runtime: "nodejs"},
			Files:    []templateSourceFile{{Path: "main.js", Role: "entry", Ownership: "patchable", SHA256: sha256Text(entry)}},
			Sections: []templateSourceSection{{ID: "entry-prefix", Category: "login", Label: "入口前缀", Path: "main.js", Content: `prefix: "演示"`}},
		}
		installHybridSourceForTest(t, server, root, &source)

		create := performPluginFileRequest(t, server, http.MethodPost, "ql_demo", "", map[string]interface{}{
			"path": "extra/new.script", "type": "file", "code": "created=true",
		})
		if create.Code != http.StatusOK {
			t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
		}
		if got := readTextFile(t, filepath.Join(root, "extra", "new.script")); got != "created=true" {
			t.Fatalf("created content=%q", got)
		}

		get := performPluginFileRequest(t, server, http.MethodGet, "ql_demo", "extra/new.script", nil)
		if get.Code != http.StatusOK {
			t.Fatalf("get status=%d body=%s", get.Code, get.Body.String())
		}
		var file struct {
			SHA256 string `json:"sha256"`
		}
		decodeUnifiedResponseData(t, get, &file)
		if err := os.WriteFile(filepath.Join(root, "extra", "new.script"), []byte("external=true"), 0644); err != nil {
			t.Fatal(err)
		}
		conflict := performPluginFileRequest(t, server, http.MethodPut, "ql_demo", "", map[string]interface{}{
			"path": "extra/new.script", "code": "editor=true", "expected_sha256": file.SHA256,
		})
		if conflict.Code != http.StatusConflict {
			t.Fatalf("conflict status=%d body=%s", conflict.Code, conflict.Body.String())
		}
		if got := readTextFile(t, filepath.Join(root, "extra", "new.script")); got != "external=true" {
			t.Fatalf("conflict overwrote file: %q", got)
		}

		remove := performPluginFileRequest(t, server, http.MethodDelete, "ql_demo", "extra/new.script", nil)
		if remove.Code != http.StatusOK {
			t.Fatalf("delete status=%d body=%s", remove.Code, remove.Body.String())
		}
		if _, err := os.Stat(filepath.Join(root, "extra", "new.script")); !os.IsNotExist(err) {
			t.Fatalf("deleted file still exists: %v", err)
		}
	})
}

func performPluginFileRequest(t *testing.T, server *Server, method, pluginID, pathValue string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	payload := ""
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		payload = string(data)
	}
	requestURL := "/api/plugins/files/" + pluginID
	if pathValue != "" {
		requestURL += "?path=" + url.QueryEscape(pathValue)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, requestURL, strings.NewReader(payload))
	server.handlePluginFiles(recorder, request)
	return recorder
}

func installHybridSourceForTest(t *testing.T, server *Server, root string, source *accountQLTemplateSource) {
	t.Helper()
	metadata := &config.PluginTemplateMetadata{PluginID: "ql_demo", Template: "nodejs_account_ql", TemplateVersion: "3.0.0", Runtime: "nodejs", Structure: "account_ql", TemplateSource: templateSourceMapForTest(t, source)}
	if err := server.pluginTemplateMetadataDatabase().SavePluginTemplateMetadata(metadata); err != nil {
		t.Fatal(err)
	}
	configValue := readPluginJSON(t, root)
	configValue["template_source"] = templateSourceMapForTest(t, source)
	configData, err := json.MarshalIndent(configValue, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plugin.json"), configData, 0644); err != nil {
		t.Fatal(err)
	}
}

func templateSourceMapForTest(t *testing.T, source *accountQLTemplateSource) map[string]interface{} {
	t.Helper()
	data, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
