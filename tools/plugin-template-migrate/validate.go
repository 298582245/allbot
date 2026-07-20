package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var ownershipValues = map[string]bool{
	"generated": true, "patchable": true, "referenced": true, "preserved": true,
}

type validatedPlugin struct {
	plugin       ManifestPlugin
	pluginRoot   string
	configPath   string
	configRaw    map[string]any
	configBytes  []byte
	sourceMap    map[string]any
	sourceBytes  []byte
	stableSHA256 string
}

func validatePlugin(root string, plugin ManifestPlugin) (*validatedPlugin, error) {
	if plugin.ID == "" || plugin.Template == "" || plugin.TemplateVersion == "" || plugin.Runtime == "" || plugin.Structure == "" {
		return nil, fmt.Errorf("插件 %s 缺少模板元数据字段", plugin.ID)
	}
	pluginRoot, err := safeJoin(root, filepath.Join("plugins", plugin.ID))
	if err != nil {
		return nil, err
	}
	backupPath, err := safeJoin(root, filepath.FromSlash(plugin.Backup.Path))
	if err != nil {
		return nil, fmt.Errorf("插件 %s 备份路径无效: %w", plugin.ID, err)
	}
	backupHash, err := hashFile(backupPath)
	if err != nil {
		return nil, fmt.Errorf("插件 %s 读取备份失败: %w", plugin.ID, err)
	}
	if !strings.EqualFold(backupHash, plugin.Backup.SHA256) {
		return nil, fmt.Errorf("插件 %s 备份 ZIP hash 不匹配", plugin.ID)
	}
	if err := validateBackupZIP(backupPath, plugin.ID); err != nil {
		return nil, fmt.Errorf("插件 %s 备份 ZIP 路径不安全: %w", plugin.ID, err)
	}
	configPath, err := safeJoin(pluginRoot, "plugin.json")
	if err != nil {
		return nil, err
	}
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("插件 %s 读取 plugin.json 失败: %w", plugin.ID, err)
	}
	var configRaw map[string]any
	if err := json.Unmarshal(configBytes, &configRaw); err != nil {
		return nil, fmt.Errorf("插件 %s plugin.json 无效: %w", plugin.ID, err)
	}
	if runtime, _ := configRaw["runtime"].(string); runtime != plugin.Runtime {
		return nil, fmt.Errorf("插件 %s runtime 不匹配: %q", plugin.ID, runtime)
	}
	if err := validateConfigTemplate(plugin, configRaw); err != nil {
		return nil, err
	}
	stableHash, err := stableConfigHash(configRaw)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(stableHash, plugin.PluginJSONStableSHA256) {
		return nil, fmt.Errorf("插件 %s plugin.json 稳定摘要漂移", plugin.ID)
	}
	if err := validateSourceIdentity(plugin); err != nil {
		return nil, err
	}
	if err := validateFiles(pluginRoot, plugin.TemplateSource.Files, stableHash); err != nil {
		return nil, fmt.Errorf("插件 %s: %w", plugin.ID, err)
	}
	if err := validateSections(pluginRoot, plugin.TemplateSource.Files, plugin.TemplateSource.Sections); err != nil {
		return nil, fmt.Errorf("插件 %s: %w", plugin.ID, err)
	}
	if err := validateTaskScripts(plugin.TemplateSource); err != nil {
		return nil, fmt.Errorf("插件 %s: %w", plugin.ID, err)
	}
	sourceBytes, err := json.Marshal(plugin.TemplateSource)
	if err != nil {
		return nil, fmt.Errorf("插件 %s template_source 不可序列化: %w", plugin.ID, err)
	}
	var sourceMap map[string]any
	if err := json.Unmarshal(sourceBytes, &sourceMap); err != nil {
		return nil, err
	}
	return &validatedPlugin{plugin: plugin, pluginRoot: pluginRoot, configPath: configPath, configRaw: configRaw, configBytes: configBytes, sourceMap: sourceMap, sourceBytes: sourceBytes, stableSHA256: stableHash}, nil
}

func validateConfigTemplate(plugin ManifestPlugin, configRaw map[string]any) error {
	actual, _ := configRaw["template"].(string)
	actualVersion, _ := configRaw["template_version"].(string)
	legacyAbsent, _ := plugin.TemplateSource.Compatibility["legacy_plugin_config_template"].(string)
	if actual == "" && legacyAbsent == "absent" {
		if actualVersion != "" {
			return fmt.Errorf("插件 %s template 为空但 template_version 非空", plugin.ID)
		}
		return nil
	}
	if actual != plugin.Template || actualVersion != plugin.TemplateVersion {
		return fmt.Errorf("插件 %s template/template_version 不匹配", plugin.ID)
	}
	return nil
}

func validateSourceIdentity(plugin ManifestPlugin) error {
	source := plugin.TemplateSource
	if source.Version != 2 || strings.ToLower(strings.TrimSpace(source.Mode)) != "hybrid" {
		return fmt.Errorf("插件 %s template_source 必须为 v2 hybrid", plugin.ID)
	}
	if source.Plugin.ID != plugin.ID || source.Plugin.Runtime != plugin.Runtime || source.Plugin.Name == "" || source.Plugin.Version == "" {
		return fmt.Errorf("插件 %s template_source plugin 标识无效", plugin.ID)
	}
	if source.Template == "" || source.Template != plugin.Template {
		return fmt.Errorf("插件 %s template_source template 缺失或不匹配", plugin.ID)
	}
	if source.Plugin.Template == "" || source.Plugin.Template != source.Template {
		return fmt.Errorf("插件 %s template_source plugin.template 缺失或不匹配", plugin.ID)
	}
	expectedRuntime := "nodejs"
	if source.Template == "python_account_ql" {
		expectedRuntime = "python"
	} else if source.Template != "nodejs_account_ql" {
		return fmt.Errorf("插件 %s template_source template 不受支持", plugin.ID)
	}
	if source.Plugin.Runtime != expectedRuntime {
		return fmt.Errorf("插件 %s template_source template/runtime 不匹配", plugin.ID)
	}
	return nil
}

func validateFiles(pluginRoot string, files []SourceFile, stableHash string) error {
	if len(files) == 0 {
		return fmt.Errorf("文件集合不能为空")
	}
	expected := make(map[string]SourceFile, len(files))
	for _, file := range files {
		clean, err := normalizeRelativePath(file.Path)
		if err != nil || clean != file.Path {
			return fmt.Errorf("文件路径无效: %s", file.Path)
		}
		if _, exists := expected[clean]; exists {
			return fmt.Errorf("文件路径重复: %s", clean)
		}
		if file.Role == "" || !ownershipValues[file.Ownership] || len(file.SHA256) != 64 {
			return fmt.Errorf("文件元数据无效: %s", clean)
		}
		if file.Ownership != "patchable" && file.ReadOnlyReason == "" {
			return fmt.Errorf("只读文件缺少原因: %s", clean)
		}
		if file.Ownership == "patchable" && file.Role != "entry" {
			return fmt.Errorf("只有入口文件可 patchable: %s", clean)
		}
		expected[clean] = file
	}
	actual, err := listFiles(pluginRoot)
	if err != nil {
		return err
	}
	if len(actual) != len(expected) {
		return fmt.Errorf("完整文件集合不匹配: manifest=%d actual=%d", len(expected), len(actual))
	}
	for _, path := range actual {
		file, exists := expected[path]
		if !exists {
			return fmt.Errorf("发现未登记文件: %s", path)
		}
		actualHash := stableHash
		if path != "plugin.json" {
			fullPath, err := safeJoin(pluginRoot, filepath.FromSlash(path))
			if err != nil {
				return err
			}
			actualHash, err = hashFile(fullPath)
			if err != nil {
				return err
			}
		}
		if !strings.EqualFold(actualHash, file.SHA256) {
			return fmt.Errorf("文件 hash 漂移: %s", path)
		}
	}
	configFile, exists := expected["plugin.json"]
	if !exists || configFile.Role != "config" || configFile.Ownership != "generated" {
		return fmt.Errorf("plugin.json 必须登记为 generated config")
	}
	return nil
}

func validateSections(pluginRoot string, files []SourceFile, sections []SourceSection) error {
	if len(sections) == 0 {
		return fmt.Errorf("section 不能为空")
	}
	fileByPath := make(map[string]SourceFile, len(files))
	for _, file := range files {
		fileByPath[file.Path] = file
	}
	type span struct{ start, end int }
	spans := make(map[string][]span)
	ids := make(map[string]bool, len(sections))
	for _, section := range sections {
		if strings.TrimSpace(section.ID) == "" || ids[section.ID] {
			return fmt.Errorf("section ID 为空或重复: %s", section.ID)
		}
		ids[section.ID] = true
		if section.Category == "" || section.Label == "" || section.Content == "" {
			return fmt.Errorf("section 字段或内容为空: %s", section.ID)
		}
		file, exists := fileByPath[section.Path]
		if !exists || file.Ownership != "patchable" || file.Role == "config" || section.Path == "plugin.json" {
			return fmt.Errorf("section %s 未指向 patchable 目标", section.ID)
		}
		path, err := safeJoin(pluginRoot, filepath.FromSlash(section.Path))
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(data)
		if strings.Count(text, section.Content) != 1 {
			return fmt.Errorf("section %s 内容无法唯一匹配", section.ID)
		}
		start := strings.Index(text, section.Content)
		current := span{start: start, end: start + len(section.Content)}
		for _, old := range spans[section.Path] {
			if current.start < old.end && old.start < current.end {
				return fmt.Errorf("section %s 与其他 section 重叠", section.ID)
			}
		}
		spans[section.Path] = append(spans[section.Path], current)
	}
	return nil
}

func validateTaskScripts(source TemplateSource) error {
	if !source.TaskScripts.ReferenceExisting || len(source.TaskScripts.Paths) == 0 {
		return fmt.Errorf("task_scripts 必须 reference_existing")
	}
	files := make(map[string]SourceFile, len(source.Files))
	for _, file := range source.Files {
		files[file.Path] = file
	}
	for _, path := range source.TaskScripts.Paths {
		file, exists := files[path]
		if !exists || file.Role != "task_script" || file.Ownership != "referenced" {
			return fmt.Errorf("任务脚本未登记为 referenced: %s", path)
		}
	}
	return nil
}

func stableConfigHash(raw map[string]any) (string, error) {
	clone := make(map[string]any, len(raw))
	for key, value := range raw {
		if key != "template_source" {
			clone[key] = value
		}
	}
	data, err := json.Marshal(clone)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func safeJoin(root, relative string) (string, error) {
	if strings.TrimSpace(root) == "" || strings.TrimSpace(relative) == "" || filepath.IsAbs(relative) || filepath.VolumeName(relative) != "" {
		return "", fmt.Errorf("路径无效: %s", relative)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	joined, err := filepath.Abs(filepath.Join(rootAbs, filepath.Clean(relative)))
	if err != nil {
		return "", err
	}
	if joined != rootAbs && !strings.HasPrefix(joined, rootAbs+string(os.PathSeparator)) {
		return "", fmt.Errorf("路径越界: %s", relative)
	}
	return joined, nil
}

func normalizeRelativePath(value string) (string, error) {
	trimmed := strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if trimmed == "" || strings.Contains(trimmed, ":") {
		return "", fmt.Errorf("路径无效")
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(trimmed)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || filepath.IsAbs(filepath.FromSlash(clean)) {
		return "", fmt.Errorf("路径无效")
	}
	return clean, nil
}

func listFiles(root string) ([]string, error) {
	var result []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("不允许符号链接: %s", path)
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result = append(result, filepath.ToSlash(relative))
		return nil
	})
	sort.Strings(result)
	return result, err
}

func validateBackupZIP(path, pluginID string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer reader.Close()
	prefix := pluginID + "/"
	if len(reader.File) == 0 {
		return fmt.Errorf("备份 ZIP 为空")
	}
	for _, file := range reader.File {
		name := strings.ReplaceAll(file.Name, "\\", "/")
		clean, err := normalizeRelativePath(name)
		if err != nil || clean != name || (!file.FileInfo().IsDir() && !strings.HasPrefix(name, prefix)) {
			return fmt.Errorf("ZIP 条目路径无效: %s", file.Name)
		}
		if file.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("ZIP 不允许符号链接: %s", file.Name)
		}
	}
	return nil
}

func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
