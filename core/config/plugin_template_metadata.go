package config

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type PluginTemplateMetadata struct {
	PluginID        string                 `json:"plugin_id"`
	Template        string                 `json:"template"`
	TemplateVersion string                 `json:"template_version"`
	Runtime         string                 `json:"runtime"`
	Structure       string                 `json:"structure"`
	Metadata        map[string]interface{} `json:"metadata"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

func (d *Database) SavePluginTemplateMetadata(item *PluginTemplateMetadata) error {
	if item == nil {
		return fmt.Errorf("插件模板元数据不能为空")
	}
	item.PluginID = strings.TrimSpace(item.PluginID)
	item.Template = strings.TrimSpace(item.Template)
	item.TemplateVersion = strings.TrimSpace(item.TemplateVersion)
	item.Runtime = strings.TrimSpace(item.Runtime)
	item.Structure = strings.TrimSpace(item.Structure)
	if item.PluginID == "" || item.Template == "" || item.TemplateVersion == "" || item.Runtime == "" {
		return fmt.Errorf("插件模板元数据缺少必要字段")
	}
	if item.Metadata == nil {
		item.Metadata = map[string]interface{}{}
	}
	metadataJSON, err := json.Marshal(item.Metadata)
	if err != nil {
		return err
	}
	_, err = d.db.Exec(`
		INSERT INTO plugin_template_metadata (plugin_id, template, template_version, runtime, structure, metadata, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(plugin_id) DO UPDATE SET
			template = excluded.template,
			template_version = excluded.template_version,
			runtime = excluded.runtime,
			structure = excluded.structure,
			metadata = excluded.metadata,
			updated_at = CURRENT_TIMESTAMP
	`, item.PluginID, item.Template, item.TemplateVersion, item.Runtime, item.Structure, string(metadataJSON))
	return err
}

func (d *Database) GetPluginTemplateMetadata(pluginID string) (*PluginTemplateMetadata, error) {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return nil, fmt.Errorf("插件 ID 不能为空")
	}
	row := d.db.QueryRow(`
		SELECT plugin_id, template, template_version, runtime, structure, metadata, created_at, updated_at
		FROM plugin_template_metadata
		WHERE plugin_id = ?
	`, pluginID)
	item, err := scanPluginTemplateMetadata(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return item, err
}

func (d *Database) DeletePluginTemplateMetadata(pluginID string) error {
	_, err := d.db.Exec(`DELETE FROM plugin_template_metadata WHERE plugin_id = ?`, strings.TrimSpace(pluginID))
	return err
}

func scanPluginTemplateMetadata(scanner interface {
	Scan(dest ...interface{}) error
}) (*PluginTemplateMetadata, error) {
	var item PluginTemplateMetadata
	var metadataJSON string
	if err := scanner.Scan(&item.PluginID, &item.Template, &item.TemplateVersion, &item.Runtime, &item.Structure, &metadataJSON, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(metadataJSON), &item.Metadata); err != nil {
		item.Metadata = map[string]interface{}{}
	}
	return &item, nil
}
