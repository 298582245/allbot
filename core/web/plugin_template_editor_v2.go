package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/types"
)

var hybridOwnershipValues = map[string]bool{
	"generated":  true,
	"patchable":  true,
	"referenced": true,
	"preserved":  true,
}

type hybridTemplateRegistration struct {
	Template        string
	TemplateVersion string
	Runtime         string
	Metadata        map[string]interface{}
}

func validateHybridTemplateSourceIdentity(pluginID string, configValue types.PluginConfig, source *accountQLTemplateSource) error {
	if source == nil || source.Version != hybridTemplateSourceVersion || strings.ToLower(strings.TrimSpace(source.Mode)) != hybridTemplateSourceMode {
		return fmt.Errorf("template_source v2 必须使用 mode=hybrid")
	}
	if strings.TrimSpace(source.Plugin.ID) != pluginID {
		return fmt.Errorf("v2 模板 pluginID 固定为 %s", pluginID)
	}
	templateName := strings.TrimSpace(source.Template)
	pluginTemplate := strings.TrimSpace(source.Plugin.Template)
	if !isAccountQLTemplate(templateName) {
		return fmt.Errorf("v2 模板 template 必须是受支持的账号青龙模板")
	}
	if pluginTemplate != templateName {
		return fmt.Errorf("v2 模板 source.template 与 source.plugin.template 必须一致")
	}
	expectedRuntime := "nodejs"
	if templateName == "python_account_ql" {
		expectedRuntime = "python"
	}
	sourceRuntime := strings.TrimSpace(source.Plugin.Runtime)
	if sourceRuntime != expectedRuntime {
		return fmt.Errorf("v2 模板 runtime 必须与 template 一致")
	}
	configTemplate := strings.TrimSpace(configValue.Template)
	if configTemplate != "" && templateName != configTemplate {
		return fmt.Errorf("v2 模板 template 固定为 %s", configTemplate)
	}
	if sourceRuntime != strings.TrimSpace(configValue.Runtime) {
		return fmt.Errorf("v2 模板 runtime 固定为 %s", configValue.Runtime)
	}
	if strings.TrimSpace(source.Plugin.Name) == "" {
		return fmt.Errorf("v2 模板 plugin.name 不能为空")
	}
	if strings.TrimSpace(source.Plugin.Version) == "" {
		return fmt.Errorf("v2 模板 plugin.version 不能为空")
	}
	return nil
}

func resolveHybridTemplateRegistration(configValue types.PluginConfig, source *accountQLTemplateSource, stored *config.PluginTemplateMetadata) (*hybridTemplateRegistration, error) {
	templateName := strings.TrimSpace(source.Template)
	templateVersion := strings.TrimSpace(configValue.TemplateVersion)
	metadata := configValue.TemplateMetadata
	if stored != nil {
		storedTemplate := strings.TrimSpace(stored.Template)
		if storedTemplate != "" && storedTemplate != templateName {
			return nil, fmt.Errorf("v2 模板 template 与数据库登记冲突")
		}
		if storedRuntime := strings.TrimSpace(stored.Runtime); storedRuntime != "" && storedRuntime != strings.TrimSpace(source.Plugin.Runtime) {
			return nil, fmt.Errorf("v2 模板 runtime 与数据库登记冲突")
		}
		storedVersion := strings.TrimSpace(stored.TemplateVersion)
		if templateVersion != "" && storedVersion != "" && templateVersion != storedVersion {
			return nil, fmt.Errorf("v2 模板 template_version 与数据库登记冲突")
		}
		if templateVersion == "" {
			templateVersion = storedVersion
		}
		if len(metadata) == 0 {
			metadata = stored.Metadata
		}
	}
	if templateVersion == "" {
		templateVersion = accountQLTemplateVersion
	}
	return &hybridTemplateRegistration{
		Template:        templateName,
		TemplateVersion: templateVersion,
		Runtime:         strings.TrimSpace(source.Plugin.Runtime),
		Metadata:        metadata,
	}, nil
}

func validateHybridManifest(pluginID string, configValue types.PluginConfig, source, previous *accountQLTemplateSource) error {
	if err := validateHybridTemplateSourceIdentity(pluginID, configValue, source); err != nil {
		return err
	}
	if previous == nil {
		return fmt.Errorf("v2 模板必须先由迁移工具登记")
	}
	if err := validateHybridTemplateSourceIdentity(pluginID, configValue, previous); err != nil {
		return err
	}
	if strings.TrimSpace(source.Template) != strings.TrimSpace(previous.Template) || strings.TrimSpace(source.Plugin.Template) != strings.TrimSpace(previous.Plugin.Template) {
		return fmt.Errorf("v2 模板 template 不允许由请求修改")
	}
	if strings.TrimSpace(source.Plugin.Runtime) != strings.TrimSpace(previous.Plugin.Runtime) {
		return fmt.Errorf("v2 模板 runtime 不允许由请求修改")
	}
	if len(source.Files) != len(previous.Files) {
		return fmt.Errorf("v2 文件清单不允许增删")
	}
	previousFiles := make(map[string]templateSourceFile, len(previous.Files))
	for _, file := range previous.Files {
		pathValue, err := normalizeHybridPath(file.Path)
		if err != nil {
			return err
		}
		if file.Path != pathValue || !hybridOwnershipValues[strings.ToLower(strings.TrimSpace(file.Ownership))] {
			return fmt.Errorf("v2 文件清单无效: %s", file.Path)
		}
		if _, exists := previousFiles[pathValue]; exists {
			return fmt.Errorf("v2 文件路径重复: %s", pathValue)
		}
		if strings.TrimSpace(file.SHA256) == "" {
			return fmt.Errorf("v2 文件缺少基线 hash: %s", pathValue)
		}
		previousFiles[pathValue] = file
	}
	seenFiles := make(map[string]bool, len(source.Files))
	for _, file := range source.Files {
		pathValue, err := normalizeHybridPath(file.Path)
		if err != nil {
			return err
		}
		if file.Path != pathValue || seenFiles[pathValue] {
			return fmt.Errorf("v2 文件路径重复: %s", pathValue)
		}
		seenFiles[pathValue] = true
		old, exists := previousFiles[pathValue]
		if !exists || file.Role != old.Role || file.Ownership != old.Ownership || file.ReadOnlyReason != old.ReadOnlyReason {
			return fmt.Errorf("v2 文件 ownership 不允许由请求修改: %s", pathValue)
		}
		if strings.TrimSpace(file.SHA256) == "" {
			return fmt.Errorf("v2 文件缺少基线 hash: %s", pathValue)
		}
		if file.SHA256 != old.SHA256 {
			return fmt.Errorf("v2 文件基线已被修改: %s", pathValue)
		}
	}
	previousSections := make(map[string]templateSourceSection, len(previous.Sections))
	for _, section := range previous.Sections {
		if strings.TrimSpace(section.ID) == "" {
			return fmt.Errorf("v2 section ID 不能为空")
		}
		if _, exists := previousSections[section.ID]; exists {
			return fmt.Errorf("v2 section ID 重复: %s", section.ID)
		}
		previousSections[section.ID] = section
	}
	seenSections := make(map[string]bool, len(source.Sections))
	for _, section := range source.Sections {
		if strings.TrimSpace(section.ID) == "" || seenSections[section.ID] {
			return fmt.Errorf("v2 section ID 重复或为空")
		}
		seenSections[section.ID] = true
		old, exists := previousSections[section.ID]
		if !exists || section.Path != old.Path || section.Category != old.Category || section.Label != old.Label || section.Ownership != old.Ownership || section.ReadOnlyReason != old.ReadOnlyReason {
			return fmt.Errorf("v2 section 只能修改已登记 section 的 content")
		}
		file, exists := previousFiles[section.Path]
		if !exists || strings.ToLower(strings.TrimSpace(file.Ownership)) != "patchable" || file.Role == "config" || section.Path == "plugin.json" {
			return fmt.Errorf("v2 section %s 只能指向 patchable 文件", section.ID)
		}
	}
	if len(seenSections) != len(previousSections) {
		return fmt.Errorf("v2 section 清单不允许增删")
	}
	return nil
}

func normalizeHybridPath(value string) (string, error) {
	trimmed := strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if trimmed == "" {
		return "", fmt.Errorf("模板文件路径不能为空")
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(trimmed)))
	if clean == "." || filepath.IsAbs(filepath.FromSlash(clean)) || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, ":") {
		return "", fmt.Errorf("模板文件路径无效: %s", value)
	}
	return clean, nil
}

type hybridPatch struct {
	section templateSourceSection
	start   int
	end     int
}

func applyHybridSections(pluginRoot string, source, previous *accountQLTemplateSource) ([]createGeneratedFile, error) {
	sectionsByPath := make(map[string][]hybridPatch)
	previousSections := make(map[string]templateSourceSection, len(previous.Sections))
	for _, section := range previous.Sections {
		previousSections[section.ID] = section
	}
	for _, section := range source.Sections {
		old := previousSections[section.ID]
		if section.Content == old.Content {
			continue
		}
		data, err := readHybridFile(pluginRoot, section.Path)
		if err != nil {
			return nil, err
		}
		oldBytes := []byte(old.Content)
		if len(oldBytes) == 0 || strings.Count(string(data), old.Content) != 1 {
			return nil, fmt.Errorf("section %s 原 content 无法唯一定位，拒绝保存", section.ID)
		}
		start := strings.Index(string(data), old.Content)
		sectionsByPath[section.Path] = append(sectionsByPath[section.Path], hybridPatch{section: section, start: start, end: start + len(oldBytes)})
	}
	paths := make([]string, 0, len(sectionsByPath))
	for pathValue := range sectionsByPath {
		paths = append(paths, pathValue)
	}
	sort.Strings(paths)
	files := make([]createGeneratedFile, 0, len(paths))
	for _, pathValue := range paths {
		data, err := readHybridFile(pluginRoot, pathValue)
		if err != nil {
			return nil, err
		}
		patches := sectionsByPath[pathValue]
		sort.Slice(patches, func(i, j int) bool { return patches[i].start > patches[j].start })
		for index, patch := range patches {
			if index > 0 && patch.end > patches[index-1].start {
				return nil, fmt.Errorf("section %s 与其他 section 在文件中重叠", patch.section.ID)
			}
			data = append(append(append([]byte(nil), data[:patch.start]...), []byte(patch.section.Content)...), data[patch.end:]...)
		}
		files = append(files, createGeneratedFile{Path: pathValue, Role: fileRole(previous.Files, pathValue), Content: string(data), Bytes: len(data), SHA256: sha256Bytes(data)})
	}
	return files, nil
}

func readHybridFile(pluginRoot, pathValue string) ([]byte, error) {
	fullPath, err := safeTemplateFilePath(pluginRoot, pathValue)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(fullPath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("v2 文件不存在: %s", pathValue)
	}
	return data, err
}

func fileRole(files []templateSourceFile, pathValue string) string {
	for _, file := range files {
		if file.Path == pathValue {
			return file.Role
		}
	}
	return "patchable"
}

func verifyHybridFiles(pluginRoot string, source, previous *accountQLTemplateSource) error {
	previousByPath := make(map[string]templateSourceFile, len(previous.Files))
	for _, file := range previous.Files {
		previousByPath[file.Path] = file
	}
	for _, file := range source.Files {
		old := previousByPath[file.Path]
		actual, err := hybridFileSHA256(pluginRoot, old)
		if err != nil {
			return err
		}
		if actual != old.SHA256 {
			return fmt.Errorf("v2 文件基线漂移: %s", file.Path)
		}
	}
	return nil
}

func hybridFileSHA256(pluginRoot string, file templateSourceFile) (string, error) {
	data, err := readHybridFile(pluginRoot, file.Path)
	if err != nil {
		return "", err
	}
	if file.Role == "config" || file.Path == "plugin.json" {
		return hybridPluginConfigSHA256(data)
	}
	return sha256Bytes(data), nil
}

func hybridPluginConfigSHA256(data []byte) (string, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", fmt.Errorf("plugin.json 无效: %w", err)
	}
	delete(raw, "template_source")
	canonical, err := json.Marshal(raw)
	if err != nil {
		return "", err
	}
	return sha256Bytes(canonical), nil
}

func refreshHybridHashes(source *accountQLTemplateSource, projected map[string][]byte, configHash string, pluginRoot string) error {
	for index := range source.Files {
		file := &source.Files[index]
		if file.Role == "config" || file.Path == "plugin.json" {
			file.SHA256 = configHash
			continue
		}
		if data, exists := projected[file.Path]; exists {
			file.SHA256 = sha256Bytes(data)
			continue
		}
		data, err := readHybridFile(pluginRoot, file.Path)
		if err != nil {
			return err
		}
		file.SHA256 = sha256Bytes(data)
	}
	return nil
}

func cloneHybridSource(source *accountQLTemplateSource) (*accountQLTemplateSource, error) {
	data, err := json.Marshal(source)
	if err != nil {
		return nil, err
	}
	var clone accountQLTemplateSource
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil, err
	}
	return &clone, nil
}

func hybridSourceMap(source *accountQLTemplateSource) (map[string]interface{}, error) {
	data, err := json.Marshal(source)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func updateHybridPluginConfig(current types.PluginConfig, source *accountQLTemplateSource) types.PluginConfig {
	result := current
	plugin := source.Plugin
	result.Name = plugin.Name
	result.Version = plugin.Version
	result.RuntimeProfile = plugin.RuntimeProfile
	result.Priority = plugin.Priority
	result.Platforms = append([]string(nil), plugin.Platforms...)
	result.Enabled = plugin.Enabled
	result.ScriptEnv = plugin.ScriptEnv
	return result
}

func renderHybridPluginConfig(pluginRoot string, updated types.PluginConfig, sourceMap map[string]interface{}) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(pluginRoot, "plugin.json"))
	if err != nil {
		return nil, err
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("plugin.json 无效: %w", err)
	}
	raw["name"] = updated.Name
	raw["version"] = updated.Version
	raw["runtime_profile"] = updated.RuntimeProfile
	raw["priority"] = updated.Priority
	raw["platforms"] = updated.Platforms
	raw["enabled"] = updated.Enabled
	raw["script_env"] = updated.ScriptEnv
	raw["template_source"] = sourceMap
	return json.MarshalIndent(raw, "", "  ")
}

func (s *Server) updateHybridTemplateEditor(pluginID string, currentConfig types.PluginConfig, source, previous *accountQLTemplateSource) (*templateEditorState, error) {
	if err := validateHybridManifest(pluginID, currentConfig, source, previous); err != nil {
		return nil, err
	}
	pluginRoot := filepath.Join("plugins", pluginID)
	if err := verifyHybridFiles(pluginRoot, source, previous); err != nil {
		return nil, err
	}
	patchFiles, err := applyHybridSections(pluginRoot, source, previous)
	if err != nil {
		return nil, err
	}
	updatedConfig := updateHybridPluginConfig(currentConfig, source)
	updatedSource, err := cloneHybridSource(source)
	if err != nil {
		return nil, err
	}
	projected := make(map[string][]byte, len(patchFiles))
	for _, file := range patchFiles {
		projected[file.Path] = []byte(file.Content)
	}
	projectedConfig, err := renderHybridPluginConfig(pluginRoot, updatedConfig, map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	configHash, err := hybridPluginConfigSHA256(projectedConfig)
	if err != nil {
		return nil, err
	}
	if err := refreshHybridHashes(updatedSource, projected, configHash, pluginRoot); err != nil {
		return nil, err
	}
	sourceMap, err := hybridSourceMap(updatedSource)
	if err != nil {
		return nil, err
	}
	updatedConfig.TemplateSource = sourceMap
	configBytes, err := renderHybridPluginConfig(pluginRoot, updatedConfig, sourceMap)
	if err != nil {
		return nil, err
	}
	configFile := createGeneratedFile{Path: "plugin.json", Role: "config", Content: string(configBytes), Bytes: len(configBytes), SHA256: pluginConfigSHA256(updatedConfig)}
	files := append([]createGeneratedFile{configFile}, patchFiles...)
	paths := generatedFilePaths(files)
	snapshots, err := snapshotTemplateFiles(pluginRoot, uniqueStrings(paths))
	if err != nil {
		return nil, err
	}
	oldMetadata, err := s.currentTemplateMetadata(pluginID)
	if err != nil {
		return nil, err
	}
	registration, err := resolveHybridTemplateRegistration(currentConfig, previous, oldMetadata)
	if err != nil {
		return nil, err
	}
	if err := writeTemplateFilesAtomically(pluginRoot, files); err != nil {
		return nil, errors.Join(err, wrapTemplateRestoreError("文件", restoreTemplateFiles(pluginRoot, snapshots)))
	}
	metadata := &config.PluginTemplateMetadata{PluginID: pluginID, Template: registration.Template, TemplateVersion: registration.TemplateVersion, Runtime: registration.Runtime, Structure: "account_ql", Metadata: registration.Metadata, TemplateSource: sourceMap}
	if err := s.pluginTemplateMetadataDatabase().SavePluginTemplateMetadata(metadata); err != nil {
		return nil, errors.Join(
			err,
			wrapTemplateRestoreError("文件", restoreTemplateFiles(pluginRoot, snapshots)),
			wrapTemplateRestoreError("模板元数据", s.restoreTemplateMetadata(pluginID, oldMetadata)),
		)
	}
	if err := s.reloadAndRegisterPlugin(pluginID); err != nil {
		restoreErr := errors.Join(
			wrapTemplateRestoreError("文件", restoreTemplateFiles(pluginRoot, snapshots)),
			wrapTemplateRestoreError("模板元数据", s.restoreTemplateMetadata(pluginID, oldMetadata)),
			wrapTemplateRestoreError("插件运行状态", s.reloadAndRegisterPlugin(pluginID)),
		)
		if restoreErr != nil {
			return nil, errors.Join(fmt.Errorf("模板更新后重载失败: %w", err), restoreErr)
		}
		return nil, fmt.Errorf("模板更新后重载失败，已恢复原状态: %w", err)
	}
	return s.loadTemplateEditorState(pluginID)
}
