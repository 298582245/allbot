package config

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type PluginAuthorization struct {
	ID        int64                  `json:"id"`
	PluginID  string                 `json:"plugin_id"`
	TableName string                 `json:"table_name,omitempty"`
	UnionID   string                 `json:"union_id"`
	Status    string                 `json:"status"`
	Plan      string                 `json:"plan"`
	Source    string                 `json:"source"`
	Metadata  map[string]interface{} `json:"metadata"`
	ExpiresAt *time.Time             `json:"expires_at,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

const defaultPluginAuthorizationTable = "plugin_authorizations"

func (d *Database) SavePluginAuthorization(item *PluginAuthorization) (*PluginAuthorization, error) {
	if err := normalizePluginAuthorization(item); err != nil {
		return nil, err
	}
	tableName, err := pluginAuthorizationTableName(item.PluginID, item.TableName)
	if err != nil {
		return nil, err
	}
	if err := d.ensurePluginAuthorizationTable(item.PluginID, tableName); err != nil {
		return nil, err
	}
	metadataJSON, err := json.Marshal(item.Metadata)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	_, err = d.db.Exec(fmt.Sprintf(`
		INSERT INTO %s (plugin_id, union_id, status, plan, source, metadata, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(plugin_id, union_id) DO UPDATE SET
			status = excluded.status,
			plan = excluded.plan,
			source = excluded.source,
			metadata = excluded.metadata,
			expires_at = excluded.expires_at,
			updated_at = excluded.updated_at
	`, quoteIdentifier(tableName)), item.PluginID, item.UnionID, item.Status, item.Plan, item.Source, string(metadataJSON), item.ExpiresAt, now, now)
	if err != nil {
		return nil, err
	}
	return d.GetPluginAuthorization(item.PluginID, item.TableName, item.UnionID)
}

func (d *Database) GetPluginAuthorization(pluginID, storeName, unionID string) (*PluginAuthorization, error) {
	tableName, err := pluginAuthorizationTableName(pluginID, storeName)
	if err != nil {
		return nil, err
	}
	if err := d.ensurePluginAuthorizationTable(pluginID, tableName); err != nil {
		return nil, err
	}
	row := d.db.QueryRow(fmt.Sprintf(`
		SELECT id, plugin_id, union_id, status, plan, source, metadata, expires_at, created_at, updated_at
		FROM %s
		WHERE plugin_id = ? AND union_id = ?
	`, quoteIdentifier(tableName)), strings.TrimSpace(pluginID), strings.TrimSpace(unionID))
	item, err := scanPluginAuthorization(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return item, err
}

func (d *Database) RevokePluginAuthorization(pluginID, storeName, unionID string) error {
	tableName, err := pluginAuthorizationTableName(pluginID, storeName)
	if err != nil {
		return err
	}
	if err := d.ensurePluginAuthorizationTable(pluginID, tableName); err != nil {
		return err
	}
	_, err = d.db.Exec(fmt.Sprintf(`
		UPDATE %s
		SET status = 'revoked', updated_at = ?
		WHERE plugin_id = ? AND union_id = ?
	`, quoteIdentifier(tableName)), time.Now(), strings.TrimSpace(pluginID), strings.TrimSpace(unionID))
	return err
}

func normalizePluginAuthorization(item *PluginAuthorization) error {
	if item == nil {
		return fmt.Errorf("插件授权不能为空")
	}
	item.PluginID = strings.TrimSpace(item.PluginID)
	item.TableName = strings.TrimSpace(item.TableName)
	item.UnionID = strings.TrimSpace(item.UnionID)
	item.Status = strings.TrimSpace(item.Status)
	item.Plan = strings.TrimSpace(item.Plan)
	item.Source = strings.TrimSpace(item.Source)
	if item.Status == "" {
		item.Status = "active"
	}
	if item.Metadata == nil {
		item.Metadata = map[string]interface{}{}
	}
	if item.PluginID == "" || item.UnionID == "" {
		return fmt.Errorf("插件授权缺少必要字段")
	}
	return nil
}

func pluginAuthorizationTableName(pluginID, storeName string) (string, error) {
	storeName = strings.TrimSpace(storeName)
	if storeName == "" {
		return defaultPluginAuthorizationTable, nil
	}
	if !sqlIdentifierPattern.MatchString(storeName) {
		return "", fmt.Errorf("授权表名无效: %s", storeName)
	}
	return storeName, nil
}

func (d *Database) ensurePluginAuthorizationTable(pluginID, tableName string) error {
	if tableName == defaultPluginAuthorizationTable {
		return nil
	}
	if !sqlIdentifierPattern.MatchString(tableName) {
		return fmt.Errorf("授权表名无效: %s", tableName)
	}
	quoted := quoteIdentifier(tableName)
	if _, err := d.db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			plugin_id TEXT NOT NULL,
			union_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			plan TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT '',
			metadata TEXT NOT NULL DEFAULT '{}',
			expires_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(plugin_id, union_id)
		);
	`, quoted)); err != nil {
		return err
	}
	if _, err := d.db.Exec(fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s(plugin_id, union_id)`, quoteIdentifier("idx_"+tableName+"_union"), quoted)); err != nil {
		return err
	}
	if _, err := d.db.Exec(fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s(plugin_id, status, expires_at)`, quoteIdentifier("idx_"+tableName+"_active"), quoted)); err != nil {
		return err
	}
	return d.migratePluginAuthorizationsToCustomTable(pluginID, tableName)
}

func (d *Database) migratePluginAuthorizationsToCustomTable(pluginID, tableName string) error {
	var count int
	if err := d.db.QueryRow(fmt.Sprintf(`SELECT COUNT(1) FROM %s WHERE plugin_id = ?`, quoteIdentifier(tableName)), strings.TrimSpace(pluginID)).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := d.db.Exec(fmt.Sprintf(`
		INSERT INTO %s (id, plugin_id, union_id, status, plan, source, metadata, expires_at, created_at, updated_at)
		SELECT id, plugin_id, union_id, status, plan, source, metadata, expires_at, created_at, updated_at
		FROM plugin_authorizations
		WHERE plugin_id = ?
	`, quoteIdentifier(tableName)), strings.TrimSpace(pluginID))
	return err
}

type pluginAuthorizationScanner interface {
	Scan(dest ...interface{}) error
}

func scanPluginAuthorization(scanner pluginAuthorizationScanner) (*PluginAuthorization, error) {
	var item PluginAuthorization
	var metadata string
	var expiresAt sql.NullTime
	if err := scanner.Scan(&item.ID, &item.PluginID, &item.UnionID, &item.Status, &item.Plan, &item.Source, &metadata, &expiresAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return nil, err
	}
	item.Metadata = map[string]interface{}{}
	_ = json.Unmarshal([]byte(metadata), &item.Metadata)
	if expiresAt.Valid {
		item.ExpiresAt = &expiresAt.Time
	}
	return &item, nil
}

func (item *PluginAuthorization) IsActive(now time.Time) bool {
	if item == nil || item.Status != "active" {
		return false
	}
	return item.ExpiresAt == nil || item.ExpiresAt.After(now)
}
