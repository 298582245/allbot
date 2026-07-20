package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/allbot/allbot/core/config"
)

type runner struct {
	openDatabase func(string) (metadataDatabase, error)
	writeConfig  func(string, []byte, os.FileMode) error
}

func newRunner() *runner {
	return &runner{
		openDatabase: func(path string) (metadataDatabase, error) { return config.NewDatabase(path) },
		writeConfig:  writeFileAtomically,
	}
}

func (r *runner) run(command string, options Options) ([]Result, error) {
	manifest, err := loadManifest(options.ManifestPath)
	if err != nil {
		return nil, err
	}
	plugins, err := selectPlugins(manifest, options.Plugin)
	if err != nil {
		return nil, err
	}
	if command == "inspect" {
		return inspectPlugins(plugins), nil
	}
	var database metadataDatabase
	if command == "apply" || command == "verify" {
		database, err = r.openDatabase(options.DBPath)
		if err != nil {
			return nil, fmt.Errorf("打开数据库失败: %w", err)
		}
		defer database.Close()
	}
	return executeSequential(plugins, func(plugin ManifestPlugin) (Result, error) {
		validated, err := validatePlugin(options.Root, plugin)
		if err != nil {
			return Result{}, err
		}
		result := Result{Plugin: plugin.ID, Command: command, Status: "ok", Files: len(plugin.TemplateSource.Files), Sections: len(plugin.TemplateSource.Sections)}
		switch command {
		case "dry-run":
			result.Message = "全部迁移前校验通过"
		case "apply":
			status, err := r.applyPlugin(database, validated)
			if err != nil {
				return Result{}, err
			}
			result.Status = status
			if status == "unchanged" {
				result.Message = "template_source 与数据库镜像已正确登记"
			} else {
				result.Message = "已原子登记 template_source 与数据库镜像"
			}
		case "verify":
			if err := verifyRegistration(database, validated); err != nil {
				return Result{}, err
			}
			result.Message = "文件、ownership、稳定摘要与数据库镜像一致"
		default:
			return Result{}, fmt.Errorf("未知命令: %s", command)
		}
		return result, nil
	})
}

func executeSequential(plugins []ManifestPlugin, operation func(ManifestPlugin) (Result, error)) ([]Result, error) {
	results := make([]Result, 0, len(plugins))
	for _, plugin := range plugins {
		result, err := operation(plugin)
		if err != nil {
			return results, fmt.Errorf("插件 %s 处理失败，已停止: %w", plugin.ID, err)
		}
		results = append(results, result)
	}
	return results, nil
}

func inspectPlugins(plugins []ManifestPlugin) []Result {
	results := make([]Result, 0, len(plugins))
	for _, plugin := range plugins {
		results = append(results, Result{
			Plugin: plugin.ID, Command: "inspect", Status: "planned", Files: len(plugin.TemplateSource.Files),
			Sections: len(plugin.TemplateSource.Sections),
			Message:  fmt.Sprintf("%s %s / %s / %s", plugin.Template, plugin.TemplateVersion, plugin.Runtime, plugin.Structure),
		})
	}
	return results
}

func (r *runner) applyPlugin(database metadataDatabase, validated *validatedPlugin) (string, error) {
	current, err := database.GetPluginTemplateMetadata(validated.plugin.ID)
	if err != nil {
		return "", fmt.Errorf("读取原数据库镜像失败: %w", err)
	}
	if sourceEqual(validated.configRaw["template_source"], validated.sourceMap) && metadataMatches(current, validated.plugin, validated.sourceMap) {
		return "unchanged", nil
	}
	updated := cloneMap(validated.configRaw)
	updated["template_source"] = validated.sourceMap
	updatedBytes, err := json.MarshalIndent(updated, "", "  ")
	if err != nil {
		return "", err
	}
	info, err := os.Stat(validated.configPath)
	if err != nil {
		return "", err
	}
	if err := r.writeConfig(validated.configPath, updatedBytes, info.Mode()); err != nil {
		fileRestoreErr := restoreConfigSnapshot(validated.configPath, validated.configBytes, info.Mode())
		return "", errors.Join(fmt.Errorf("原子写入 plugin.json 失败: %w", err), wrapRestoreError("plugin.json", fileRestoreErr))
	}
	metadata := desiredMetadata(validated.plugin, validated.sourceMap)
	if err := database.SavePluginTemplateMetadata(metadata); err != nil {
		fileRestoreErr := restoreConfigSnapshot(validated.configPath, validated.configBytes, info.Mode())
		metadataRestoreErr := restoreMetadata(database, validated.plugin.ID, current)
		return "", errors.Join(fmt.Errorf("保存数据库镜像失败: %w", err), wrapRestoreError("plugin.json", fileRestoreErr), wrapRestoreError("数据库镜像", metadataRestoreErr))
	}
	return "applied", nil
}

func verifyRegistration(database metadataDatabase, validated *validatedPlugin) error {
	if !sourceEqual(validated.configRaw["template_source"], validated.sourceMap) {
		return fmt.Errorf("plugin.json.template_source 与 manifest 不一致")
	}
	metadata, err := database.GetPluginTemplateMetadata(validated.plugin.ID)
	if err != nil {
		return fmt.Errorf("读取数据库镜像失败: %w", err)
	}
	if !metadataMatches(metadata, validated.plugin, validated.sourceMap) {
		return fmt.Errorf("数据库 PluginTemplateMetadata 镜像不一致")
	}
	return nil
}

func desiredMetadata(plugin ManifestPlugin, source map[string]any) *config.PluginTemplateMetadata {
	return &config.PluginTemplateMetadata{
		PluginID: plugin.ID, Template: plugin.Template, TemplateVersion: plugin.TemplateVersion,
		Runtime: plugin.Runtime, Structure: plugin.Structure, Metadata: plugin.Metadata, TemplateSource: source,
	}
}

func metadataMatches(actual *config.PluginTemplateMetadata, plugin ManifestPlugin, source map[string]any) bool {
	if actual == nil || actual.PluginID != plugin.ID || actual.Template != plugin.Template || actual.TemplateVersion != plugin.TemplateVersion || actual.Runtime != plugin.Runtime || actual.Structure != plugin.Structure {
		return false
	}
	return sourceEqual(actual.Metadata, plugin.Metadata) && sourceEqual(actual.TemplateSource, source)
}

func restoreConfigSnapshot(target string, data []byte, mode os.FileMode) error {
	return writeFileAtomically(target, data, mode)
}

func restoreMetadata(database metadataDatabase, pluginID string, previous *config.PluginTemplateMetadata) error {
	if previous == nil {
		return database.DeletePluginTemplateMetadata(pluginID)
	}
	return database.SavePluginTemplateMetadata(previous)
}

func wrapRestoreError(target string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("恢复%s失败: %w", target, err)
}

func sourceEqual(left, right any) bool {
	return reflect.DeepEqual(normalizeJSON(left), normalizeJSON(right))
}

func normalizeJSON(value any) any {
	data, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var normalized any
	if json.Unmarshal(data, &normalized) != nil {
		return value
	}
	return normalized
}

func cloneMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func writeFileAtomically(target string, data []byte, mode os.FileMode) error {
	directory := filepath.Dir(target)
	temp, err := os.CreateTemp(directory, ".plugin-template-migrate-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(mode.Perm()); err == nil {
		_, err = temp.Write(data)
	}
	if err == nil {
		err = temp.Sync()
	}
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	backup := target + ".plugin-template-migrate-old"
	_ = os.Remove(backup)
	if err := os.Rename(target, backup); err != nil {
		return err
	}
	if err := os.Rename(tempPath, target); err != nil {
		_ = os.Rename(backup, target)
		return err
	}
	if err := os.Remove(backup); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
