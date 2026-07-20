package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/allbot/allbot/core/config"
)

type Manifest struct {
	ManifestVersion       int              `json:"manifest_version"`
	ID                    string           `json:"id"`
	TemplateSourceVersion int              `json:"template_source_version"`
	Mode                  string           `json:"mode"`
	Order                 []string         `json:"order"`
	Migration             map[string]any   `json:"migration"`
	Invariants            []string         `json:"invariants"`
	Plugins               []ManifestPlugin `json:"plugins"`
}

type ManifestPlugin struct {
	ID                     string         `json:"id"`
	Backup                 Backup         `json:"backup"`
	Template               string         `json:"template"`
	TemplateVersion        string         `json:"template_version"`
	Runtime                string         `json:"runtime"`
	Structure              string         `json:"structure"`
	PluginJSONStableSHA256 string         `json:"plugin_json_stable_sha256"`
	Metadata               map[string]any `json:"metadata"`
	TemplateSource         TemplateSource `json:"template_source"`
	Invariants             map[string]any `json:"invariants"`
}

type Backup struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type TemplateSource struct {
	Version       int             `json:"version"`
	Mode          string          `json:"mode"`
	Template      string          `json:"template,omitempty"`
	Plugin        SourcePlugin    `json:"plugin"`
	Files         []SourceFile    `json:"files"`
	Sections      []SourceSection `json:"sections"`
	TaskScripts   TaskScripts     `json:"task_scripts"`
	Compatibility map[string]any  `json:"compatibility"`
	Migration     map[string]any  `json:"migration"`
}

type SourcePlugin struct {
	ID             string         `json:"id"`
	Template       string         `json:"template,omitempty"`
	Name           string         `json:"name"`
	Version        string         `json:"version"`
	Runtime        string         `json:"runtime"`
	RuntimeProfile string         `json:"runtime_profile"`
	Priority       int            `json:"priority"`
	Platforms      []string       `json:"platforms"`
	Enabled        bool           `json:"enabled"`
	ScriptEnv      map[string]any `json:"script_env"`
}

type SourceFile struct {
	Path           string `json:"path"`
	Role           string `json:"role"`
	Ownership      string `json:"ownership"`
	SHA256         string `json:"sha256"`
	ReadOnlyReason string `json:"read_only_reason,omitempty"`
}

type SourceSection struct {
	ID             string `json:"id"`
	Category       string `json:"category"`
	Label          string `json:"label"`
	Path           string `json:"path"`
	Content        string `json:"content"`
	Ownership      string `json:"ownership,omitempty"`
	ReadOnlyReason string `json:"read_only_reason,omitempty"`
}

type TaskScripts struct {
	ReferenceExisting bool     `json:"reference_existing"`
	Paths             []string `json:"paths"`
}

type Options struct {
	ManifestPath string
	Root         string
	DBPath       string
	Plugin       string
}

type Result struct {
	Plugin   string `json:"plugin"`
	Command  string `json:"command"`
	Status   string `json:"status"`
	Files    int    `json:"files"`
	Sections int    `json:"sections"`
	Message  string `json:"message,omitempty"`
}

type metadataDatabase interface {
	SavePluginTemplateMetadata(*config.PluginTemplateMetadata) error
	GetPluginTemplateMetadata(string) (*config.PluginTemplateMetadata, error)
	DeletePluginTemplateMetadata(string) error
	Close() error
}

func validPluginID(value string) bool {
	if value == "" || value != strings.TrimSpace(value) {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '_' && char != '-' {
			return false
		}
	}
	return true
}

func loadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 manifest 失败: %w", err)
	}
	var manifest Manifest
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("解析 manifest 失败: %w", err)
	}
	if manifest.ManifestVersion != 1 || manifest.TemplateSourceVersion != 2 || manifest.Mode != "hybrid" {
		return nil, fmt.Errorf("manifest 版本或模式无效")
	}
	return &manifest, nil
}

func selectPlugins(manifest *Manifest, pluginID string) ([]ManifestPlugin, error) {
	byID := make(map[string]ManifestPlugin, len(manifest.Plugins))
	for _, plugin := range manifest.Plugins {
		if !validPluginID(plugin.ID) {
			return nil, fmt.Errorf("manifest 包含无效插件 ID: %s", plugin.ID)
		}
		if _, exists := byID[plugin.ID]; exists {
			return nil, fmt.Errorf("manifest 插件重复: %s", plugin.ID)
		}
		byID[plugin.ID] = plugin
	}
	if pluginID != "" {
		plugin, exists := byID[pluginID]
		if !exists {
			return nil, fmt.Errorf("manifest 不包含插件: %s", pluginID)
		}
		return []ManifestPlugin{plugin}, nil
	}
	if len(manifest.Order) != len(manifest.Plugins) {
		return nil, fmt.Errorf("manifest order 与插件数量不一致")
	}
	selected := make([]ManifestPlugin, 0, len(manifest.Order))
	seen := make(map[string]bool, len(manifest.Order))
	for _, id := range manifest.Order {
		plugin, exists := byID[id]
		if !exists || seen[id] {
			return nil, fmt.Errorf("manifest order 无效: %s", id)
		}
		seen[id] = true
		selected = append(selected, plugin)
	}
	return selected, nil
}
