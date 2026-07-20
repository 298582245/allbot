package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/allbot/allbot/core/config"
)

type fixture struct {
	root         string
	manifestPath string
	dbPath       string
	manifest     Manifest
}

func TestRealManifestHasCategorizedUniqueNonOverlappingSections(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(root, "backups", "plugin-template-migration-20260719", "manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Skip("真实迁移数据未包含在当前工作副本中")
	} else if err != nil {
		t.Fatal(err)
	}
	manifest, err := loadManifest(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	wantedOrder := []string{"xyyx", "fxsh", "ppcs", "rain_cloud", "ydyp", "zglt"}
	if !reflect.DeepEqual(manifest.Order, wantedOrder) {
		t.Fatalf("unexpected real manifest order: %v", manifest.Order)
	}
	requiredCategories := map[string][]string{
		"xyyx":       {"login", "parse", "query", "ck_check", "auth", "ql_registration", "ql_env", "schedules"},
		"fxsh":       {"login", "parse", "query", "ck_check", "auth", "ql_registration", "ql_env", "schedules", "routes", "after_run"},
		"ppcs":       {"login", "parse", "query", "ck_check", "auth", "ql_registration", "ql_env", "schedules", "routes"},
		"rain_cloud": {"login", "parse", "query", "ck_check", "auth", "ql_registration", "ql_env", "schedules", "routes"},
		"ydyp":       {"login", "parse", "query", "ck_check", "auth", "ql_registration", "ql_env", "schedules", "routes"},
		"zglt":       {"login", "parse", "query", "ck_check", "auth", "ql_registration", "ql_env", "schedules", "routes"},
	}
	plugins, err := selectPlugins(manifest, "")
	if err != nil {
		t.Fatal(err)
	}
	validationRoot := restoreRealManifestFixture(t, root, plugins)
	for _, plugin := range plugins {
		if len(plugin.TemplateSource.Sections) <= 1 {
			t.Errorf("plugin %s must have multiple sections, got %d", plugin.ID, len(plugin.TemplateSource.Sections))
			continue
		}
		categories := make(map[string]bool)
		for _, section := range plugin.TemplateSource.Sections {
			if section.Ownership != "patchable" {
				t.Errorf("plugin %s section %s ownership=%q", plugin.ID, section.ID, section.Ownership)
			}
			categories[section.Category] = true
		}
		for _, category := range requiredCategories[plugin.ID] {
			if !categories[category] {
				t.Errorf("plugin %s missing category %s; got %v", plugin.ID, category, categories)
			}
		}
		validated, err := validatePlugin(validationRoot, plugin)
		if err != nil {
			t.Errorf("plugin %s real dry-run validation failed: %v", plugin.ID, err)
			continue
		}
		if len(validated.plugin.TemplateSource.Sections) != len(plugin.TemplateSource.Sections) {
			t.Errorf("plugin %s section count changed during validation", plugin.ID)
		}
	}
}

func restoreRealManifestFixture(t *testing.T, root string, plugins []ManifestPlugin) string {
	t.Helper()
	validationRoot := t.TempDir()
	for _, plugin := range plugins {
		backupPath, err := safeJoin(root, filepath.FromSlash(plugin.Backup.Path))
		if err != nil {
			t.Fatal(err)
		}
		backupData, err := os.ReadFile(backupPath)
		if err != nil {
			t.Fatal(err)
		}
		fixtureBackupPath, err := safeJoin(validationRoot, filepath.FromSlash(plugin.Backup.Path))
		if err != nil {
			t.Fatal(err)
		}
		mustWrite(t, fixtureBackupPath, backupData)
		archive, err := zip.OpenReader(backupPath)
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range archive.File {
			name := strings.ReplaceAll(file.Name, "\\", "/")
			clean, err := normalizeRelativePath(name)
			if err != nil || clean != name {
				_ = archive.Close()
				t.Fatalf("ZIP 条目路径无效: %s", file.Name)
			}
			target, err := safeJoin(filepath.Join(validationRoot, "plugins"), filepath.FromSlash(clean))
			if err != nil {
				_ = archive.Close()
				t.Fatal(err)
			}
			if file.FileInfo().IsDir() {
				if err := os.MkdirAll(target, 0755); err != nil {
					_ = archive.Close()
					t.Fatal(err)
				}
				continue
			}
			reader, err := file.Open()
			if err != nil {
				_ = archive.Close()
				t.Fatal(err)
			}
			data, readErr := io.ReadAll(reader)
			closeErr := reader.Close()
			if readErr != nil {
				_ = archive.Close()
				t.Fatal(readErr)
			}
			if closeErr != nil {
				_ = archive.Close()
				t.Fatal(closeErr)
			}
			mustWrite(t, target, data)
		}
		if err := archive.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return validationRoot
}

func TestDryRunApplyAndVerifyWithoutBusinessFileChanges(t *testing.T) {
	item := newFixture(t, []string{"alpha"})
	options := item.options()
	r := newRunner()

	results, err := r.run("dry-run", options)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Plugin != "alpha" {
		t.Fatalf("unexpected dry-run results: %#v", results)
	}
	before := businessFileSnapshot(t, item.root, "alpha")
	results, err = r.run("apply", options)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != "applied" {
		t.Fatalf("unexpected apply status: %#v", results[0])
	}
	if after := businessFileSnapshot(t, item.root, "alpha"); !reflect.DeepEqual(before, after) {
		t.Fatalf("business files changed: before=%v after=%v", before, after)
	}
	if _, err := r.run("verify", options); err != nil {
		t.Fatal(err)
	}
	results, err = r.run("apply", options)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != "unchanged" {
		t.Fatalf("second apply should be idempotent: %#v", results[0])
	}
}

func TestDryRunRejectsDriftUnsafePathDuplicateNonUniqueAndOverlap(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Manifest, string)
		want   string
	}{
		{
			name: "drift",
			mutate: func(_ *Manifest, root string) {
				mustWrite(t, filepath.Join(root, "plugins", "alpha", "task.js"), []byte("changed\n"))
			},
			want: "hash 漂移",
		},
		{
			name: "unsafe path",
			mutate: func(manifest *Manifest, _ string) {
				manifest.Plugins[0].TemplateSource.Files[0].Path = "../escape.js"
			},
			want: "文件路径无效",
		},
		{
			name: "unsafe plugin id",
			mutate: func(manifest *Manifest, _ string) {
				manifest.Plugins[0].ID = "../alpha"
				manifest.Order[0] = "../alpha"
			},
			want: "无效插件 ID",
		},
		{
			name: "duplicate section",
			mutate: func(manifest *Manifest, _ string) {
				section := manifest.Plugins[0].TemplateSource.Sections[0]
				manifest.Plugins[0].TemplateSource.Sections = append(manifest.Plugins[0].TemplateSource.Sections, section)
			},
			want: "section ID 为空或重复",
		},
		{
			name: "non unique section",
			mutate: func(manifest *Manifest, root string) {
				mustWrite(t, filepath.Join(root, "plugins", "alpha", "main.js"), []byte("const value = 1;\nconst value = 1;\n"))
				manifest.Plugins[0].TemplateSource.Sections[0].Content = "const value = 1;"
				refreshFileHash(t, &manifest.Plugins[0], root, "main.js")
			},
			want: "无法唯一匹配",
		},
		{
			name: "overlap",
			mutate: func(manifest *Manifest, _ string) {
				manifest.Plugins[0].TemplateSource.Sections = []SourceSection{
					{ID: "whole", Category: "entry", Label: "整体", Path: "main.js", Content: "const value = 1;\n", Ownership: "patchable"},
					{ID: "part", Category: "entry", Label: "局部", Path: "main.js", Content: "value = 1", Ownership: "patchable"},
				}
			},
			want: "重叠",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			item := newFixture(t, []string{"alpha"})
			test.mutate(&item.manifest, item.root)
			writeManifest(t, item.manifestPath, item.manifest)
			_, err := newRunner().run("dry-run", item.options())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v, want substring %q", err, test.want)
			}
		})
	}
}

func TestDryRunRejectsTemplateRuntimeAndTaskScriptContract(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Manifest)
		want   string
	}{
		{"runtime", func(manifest *Manifest) { manifest.Plugins[0].Runtime = "python" }, "runtime 不匹配"},
		{"template", func(manifest *Manifest) {
			manifest.Plugins[0].TemplateSource.Compatibility["legacy_plugin_config_template"] = "present"
		}, "template/template_version 不匹配"},
		{"missing source template", func(manifest *Manifest) { manifest.Plugins[0].TemplateSource.Template = "" }, "template 缺失或不匹配"},
		{"source template mismatch", func(manifest *Manifest) { manifest.Plugins[0].TemplateSource.Template = "python_account_ql" }, "template 缺失或不匹配"},
		{"missing plugin template", func(manifest *Manifest) { manifest.Plugins[0].TemplateSource.Plugin.Template = "" }, "plugin.template 缺失或不匹配"},
		{"plugin template mismatch", func(manifest *Manifest) { manifest.Plugins[0].TemplateSource.Plugin.Template = "python_account_ql" }, "plugin.template 缺失或不匹配"},
		{"template runtime mismatch", func(manifest *Manifest) {
			manifest.Plugins[0].Template = "python_account_ql"
			manifest.Plugins[0].TemplateSource.Template = "python_account_ql"
			manifest.Plugins[0].TemplateSource.Plugin.Template = "python_account_ql"
		}, "template/runtime 不匹配"},
		{"unsupported template", func(manifest *Manifest) {
			manifest.Plugins[0].Template = "custom_account_ql"
			manifest.Plugins[0].TemplateSource.Template = "custom_account_ql"
			manifest.Plugins[0].TemplateSource.Plugin.Template = "custom_account_ql"
		}, "template 不受支持"},
		{"task script", func(manifest *Manifest) { manifest.Plugins[0].TemplateSource.TaskScripts.ReferenceExisting = false }, "reference_existing"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			item := newFixture(t, []string{"alpha"})
			test.mutate(&item.manifest)
			writeManifest(t, item.manifestPath, item.manifest)
			_, err := newRunner().run("dry-run", item.options())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v, want substring %q", err, test.want)
			}
		})
	}
}

func TestApplyRollsBackPluginConfigAndMetadataOnDatabaseFailure(t *testing.T) {
	item := newFixture(t, []string{"alpha"})
	realDatabase, err := config.NewDatabase(item.dbPath)
	if err != nil {
		t.Fatal(err)
	}
	previous := &config.PluginTemplateMetadata{
		PluginID: "alpha", Template: "old_template", TemplateVersion: "1.0.0", Runtime: "nodejs",
		Structure: "old", Metadata: map[string]any{"old": true}, TemplateSource: map[string]any{"version": float64(1)},
	}
	if err := realDatabase.SavePluginTemplateMetadata(previous); err != nil {
		t.Fatal(err)
	}
	failing := &failOnceDatabase{metadataDatabase: realDatabase, failSave: true}
	r := newRunner()
	r.openDatabase = func(string) (metadataDatabase, error) { return failing, nil }
	configPath := filepath.Join(item.root, "plugins", "alpha", "plugin.json")
	beforeConfig := mustRead(t, configPath)

	_, err = r.run("apply", item.options())
	if err == nil || !strings.Contains(err.Error(), "保存数据库镜像失败") {
		t.Fatalf("unexpected error: %v", err)
	}
	if after := mustRead(t, configPath); !bytes.Equal(beforeConfig, after) {
		t.Fatal("plugin.json was not restored")
	}
	check, err := config.NewDatabase(item.dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer check.Close()
	stored, err := check.GetPluginTemplateMetadata("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil || stored.Template != previous.Template || !sourceEqual(stored.TemplateSource, previous.TemplateSource) {
		t.Fatalf("metadata was not restored: %#v", stored)
	}
}

func TestApplyFileFailureDoesNotChangeConfigOrDatabase(t *testing.T) {
	item := newFixture(t, []string{"alpha"})
	r := newRunner()
	r.writeConfig = func(string, []byte, os.FileMode) error { return errors.New("injected write failure") }
	configPath := filepath.Join(item.root, "plugins", "alpha", "plugin.json")
	before := mustRead(t, configPath)
	_, err := r.run("apply", item.options())
	if err == nil || !strings.Contains(err.Error(), "原子写入 plugin.json 失败") {
		t.Fatalf("unexpected error: %v", err)
	}
	if after := mustRead(t, configPath); !bytes.Equal(before, after) {
		t.Fatal("plugin.json changed after file failure")
	}
	database, err := config.NewDatabase(item.dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	stored, err := database.GetPluginTemplateMetadata("alpha")
	if err != nil || stored != nil {
		t.Fatalf("unexpected metadata after file failure: %#v err=%v", stored, err)
	}
}

func TestVerifyRejectsMissingRegistrationAndOwnershipDrift(t *testing.T) {
	item := newFixture(t, []string{"alpha"})
	if _, err := newRunner().run("verify", item.options()); err == nil || !strings.Contains(err.Error(), "template_source") {
		t.Fatalf("verify should reject missing registration: %v", err)
	}
	if _, err := newRunner().run("apply", item.options()); err != nil {
		t.Fatal(err)
	}
	item.manifest.Plugins[0].TemplateSource.Files[0].Ownership = "preserved"
	item.manifest.Plugins[0].TemplateSource.Files[0].ReadOnlyReason = "注入 ownership 漂移"
	writeManifest(t, item.manifestPath, item.manifest)
	if _, err := newRunner().run("verify", item.options()); err == nil || !strings.Contains(err.Error(), "未指向 patchable") {
		t.Fatalf("verify should reject ownership drift: %v", err)
	}
}

func TestExecuteSequentialStopsAtFirstFailureInManifestOrder(t *testing.T) {
	plugins := []ManifestPlugin{{ID: "xyyx"}, {ID: "fxsh"}, {ID: "ppcs"}}
	visited := []string{}
	results, err := executeSequential(plugins, func(plugin ManifestPlugin) (Result, error) {
		visited = append(visited, plugin.ID)
		if plugin.ID == "fxsh" {
			return Result{}, errors.New("stop")
		}
		return Result{Plugin: plugin.ID}, nil
	})
	if err == nil || !reflect.DeepEqual(visited, []string{"xyyx", "fxsh"}) || len(results) != 1 {
		t.Fatalf("visited=%v results=%v err=%v", visited, results, err)
	}
}

func TestSelectPluginsUsesStrictManifestOrderAndSingleSelection(t *testing.T) {
	manifest := &Manifest{Order: []string{"b", "a"}, Plugins: []ManifestPlugin{{ID: "a"}, {ID: "b"}}}
	all, err := selectPlugins(manifest, "")
	if err != nil || all[0].ID != "b" || all[1].ID != "a" {
		t.Fatalf("strict order failed: %#v err=%v", all, err)
	}
	one, err := selectPlugins(manifest, "a")
	if err != nil || len(one) != 1 || one[0].ID != "a" {
		t.Fatalf("single selection failed: %#v err=%v", one, err)
	}
}

type failOnceDatabase struct {
	metadataDatabase
	failSave bool
}

func (database *failOnceDatabase) SavePluginTemplateMetadata(item *config.PluginTemplateMetadata) error {
	if database.failSave {
		database.failSave = false
		return errors.New("injected database failure")
	}
	return database.metadataDatabase.SavePluginTemplateMetadata(item)
}

func newFixture(t *testing.T, ids []string) fixture {
	t.Helper()
	root := t.TempDir()
	manifest := Manifest{ManifestVersion: 1, ID: "test", TemplateSourceVersion: 2, Mode: "hybrid", Order: append([]string(nil), ids...)}
	for _, id := range ids {
		pluginRoot := filepath.Join(root, "plugins", id)
		if err := os.MkdirAll(pluginRoot, 0755); err != nil {
			t.Fatal(err)
		}
		configRaw := map[string]any{
			"name": id, "version": "1.0.0", "runtime": "nodejs", "runtime_profile": "", "entry": "main.js",
			"platforms": []any{"qq", "web"}, "priority": float64(1), "enabled": true,
			"script_env": map[string]any{"enabled": false, "names": []any{}},
		}
		configBytes, _ := json.MarshalIndent(configRaw, "", "  ")
		mustWrite(t, filepath.Join(pluginRoot, "plugin.json"), configBytes)
		mainContent := []byte("const value = 1;\n")
		taskContent := []byte("console.log('task');\n")
		mustWrite(t, filepath.Join(pluginRoot, "main.js"), mainContent)
		mustWrite(t, filepath.Join(pluginRoot, "task.js"), taskContent)
		stableHash, err := stableConfigHash(configRaw)
		if err != nil {
			t.Fatal(err)
		}
		backupPath := filepath.Join(root, "backups", id+".zip")
		writeZip(t, backupPath, map[string][]byte{id + "/plugin.json": configBytes, id + "/main.js": mainContent, id + "/task.js": taskContent})
		plugin := ManifestPlugin{
			ID: id, Backup: Backup{Path: filepath.ToSlash(filepath.Join("backups", id+".zip")), SHA256: fileHash(t, backupPath)},
			Template: "nodejs_account_ql", TemplateVersion: "3.0.0", Runtime: "nodejs", Structure: "account_ql",
			PluginJSONStableSHA256: stableHash, Metadata: map[string]any{"source": "test"},
			TemplateSource: TemplateSource{
				Version: 2, Mode: "hybrid", Template: "nodejs_account_ql",
				Plugin: SourcePlugin{ID: id, Template: "nodejs_account_ql", Name: id, Version: "1.0.0", Runtime: "nodejs", Platforms: []string{"qq", "web"}, Enabled: true, ScriptEnv: map[string]any{"enabled": false, "names": []any{}}},
				Files: []SourceFile{
					{Path: "main.js", Role: "entry", Ownership: "patchable", SHA256: hashBytes(mainContent)},
					{Path: "plugin.json", Role: "config", Ownership: "generated", SHA256: stableHash, ReadOnlyReason: "仅更新 template_source"},
					{Path: "task.js", Role: "task_script", Ownership: "referenced", SHA256: hashBytes(taskContent), ReadOnlyReason: "引用现有任务脚本"},
				},
				Sections:      []SourceSection{{ID: "entry", Category: "entry", Label: "入口", Path: "main.js", Content: string(mainContent), Ownership: "patchable"}},
				TaskScripts:   TaskScripts{ReferenceExisting: true, Paths: []string{"task.js"}},
				Compatibility: map[string]any{"legacy_plugin_config_template": "absent"}, Migration: map[string]any{"id": "test"},
			},
		}
		manifest.Plugins = append(manifest.Plugins, plugin)
	}
	manifestPath := filepath.Join(root, "manifest.json")
	writeManifest(t, manifestPath, manifest)
	return fixture{root: root, manifestPath: manifestPath, dbPath: filepath.Join(root, "config.db"), manifest: manifest}
}

func (item fixture) options() Options {
	return Options{ManifestPath: item.manifestPath, Root: item.root, DBPath: item.dbPath}
}

func refreshFileHash(t *testing.T, plugin *ManifestPlugin, root, relative string) {
	t.Helper()
	for index := range plugin.TemplateSource.Files {
		if plugin.TemplateSource.Files[index].Path == relative {
			plugin.TemplateSource.Files[index].SHA256 = fileHash(t, filepath.Join(root, "plugins", plugin.ID, relative))
			return
		}
	}
	t.Fatalf("missing file in manifest: %s", relative)
}

func businessFileSnapshot(t *testing.T, root, id string) map[string]string {
	t.Helper()
	result := map[string]string{}
	for _, name := range []string{"main.js", "task.js"} {
		result[name] = fileHash(t, filepath.Join(root, "plugins", id, name))
	}
	return result
}

func writeManifest(t *testing.T, path string, manifest Manifest) {
	t.Helper()
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, path, data)
}

func writeZip(t *testing.T, path string, files map[string][]byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func fileHash(t *testing.T, path string) string {
	t.Helper()
	return hashBytes(mustRead(t, path))
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
