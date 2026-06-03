package config

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type PluginAccount struct {
	ID          int64                  `json:"id"`
	PluginID    string                 `json:"plugin_id"`
	TableName   string                 `json:"table_name,omitempty"`
	UnionID     string                 `json:"union_id"`
	Platform    string                 `json:"platform"`
	UserID      string                 `json:"user_id"`
	AccountName string                 `json:"account_name"`
	EnvName     string                 `json:"env_name"`
	EnvValue    string                 `json:"env_value"`
	Remark      string                 `json:"remark"`
	Status      string                 `json:"status"`
	Metadata    map[string]interface{} `json:"metadata"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type PluginAccountQuery struct {
	TableName string `json:"table_name"`
	Scope     string `json:"scope"`
	UnionID   string `json:"union_id"`
	EnvName   string `json:"env_name"`
	Status    string `json:"status"`
}

const defaultPluginAccountTable = "plugin_accounts"

func (d *Database) ensurePluginAccountBaseTableColumns() error {
	return d.ensurePluginAccountTableColumns(defaultPluginAccountTable)
}

func (d *Database) SavePluginAccount(item *PluginAccount) (*PluginAccount, error) {
	if err := normalizePluginAccount(item); err != nil {
		return nil, err
	}
	tableName, err := pluginAccountTableName(item.PluginID, item.TableName)
	if err != nil {
		return nil, err
	}
	if err := d.ensurePluginAccountTable(item.PluginID, tableName); err != nil {
		return nil, err
	}
	metadataJSON, err := json.Marshal(item.Metadata)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if item.ID == 0 {
		result, err := d.db.Exec(fmt.Sprintf(`
			INSERT INTO %s (plugin_id, union_id, platform, user_id, account_name, env_name, env_value, remark, status, metadata, expires_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, quoteIdentifier(tableName)), item.PluginID, item.UnionID, item.Platform, item.UserID, item.AccountName, item.EnvName, item.EnvValue, item.Remark, item.Status, string(metadataJSON), item.ExpiresAt, now, now)
		if err != nil {
			return nil, err
		}
		item.ID, _ = result.LastInsertId()
	} else {
		_, err := d.db.Exec(fmt.Sprintf(`
			UPDATE %s
			SET plugin_id = ?, union_id = ?, platform = ?, user_id = ?, account_name = ?, env_name = ?, env_value = ?, remark = ?, status = ?, metadata = ?, expires_at = ?, updated_at = ?
			WHERE id = ? AND plugin_id = ?
		`, quoteIdentifier(tableName)), item.PluginID, item.UnionID, item.Platform, item.UserID, item.AccountName, item.EnvName, item.EnvValue, item.Remark, item.Status, string(metadataJSON), item.ExpiresAt, now, item.ID, item.PluginID)
		if err != nil {
			return nil, err
		}
	}
	return d.GetPluginAccount(item.PluginID, item.TableName, item.ID)
}

func (d *Database) GetPluginAccount(pluginID, storeName string, id int64) (*PluginAccount, error) {
	tableName, err := pluginAccountTableName(pluginID, storeName)
	if err != nil {
		return nil, err
	}
	if err := d.ensurePluginAccountTable(pluginID, tableName); err != nil {
		return nil, err
	}
	row := d.db.QueryRow(fmt.Sprintf(`
		SELECT id, plugin_id, union_id, platform, user_id, account_name, env_name, env_value, remark, status, metadata, expires_at, created_at, updated_at
		FROM %s
		WHERE plugin_id = ? AND id = ?
	`, quoteIdentifier(tableName)), strings.TrimSpace(pluginID), id)
	item, err := scanPluginAccount(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return item, err
}

func (d *Database) ListPluginAccounts(pluginID string, query PluginAccountQuery) ([]*PluginAccount, error) {
	tableName, err := pluginAccountTableName(pluginID, query.TableName)
	if err != nil {
		return nil, err
	}
	if err := d.ensurePluginAccountTable(pluginID, tableName); err != nil {
		return nil, err
	}
	conditions := []string{"plugin_id = ?"}
	args := []interface{}{strings.TrimSpace(pluginID)}
	if strings.TrimSpace(query.Scope) != "all" && strings.TrimSpace(query.UnionID) != "" {
		conditions = append(conditions, "union_id = ?")
		args = append(args, strings.TrimSpace(query.UnionID))
	}
	if strings.TrimSpace(query.EnvName) != "" {
		conditions = append(conditions, "env_name = ?")
		args = append(args, strings.TrimSpace(query.EnvName))
	}
	if strings.TrimSpace(query.Status) != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, strings.TrimSpace(query.Status))
	}
	rows, err := d.db.Query(fmt.Sprintf(`
		SELECT id, plugin_id, union_id, platform, user_id, account_name, env_name, env_value, remark, status, metadata, expires_at, created_at, updated_at
		FROM %s
		WHERE %s
		ORDER BY updated_at DESC, id DESC
	`, quoteIdentifier(tableName), strings.Join(conditions, " AND ")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]*PluginAccount, 0)
	for rows.Next() {
		item, err := scanPluginAccount(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (d *Database) DeletePluginAccount(pluginID, storeName string, id int64, unionID string, all bool) error {
	tableName, err := pluginAccountTableName(pluginID, storeName)
	if err != nil {
		return err
	}
	if err := d.ensurePluginAccountTable(pluginID, tableName); err != nil {
		return err
	}
	query := fmt.Sprintf(`DELETE FROM %s WHERE plugin_id = ? AND id = ?`, quoteIdentifier(tableName))
	args := []interface{}{strings.TrimSpace(pluginID), id}
	if !all {
		query += ` AND union_id = ?`
		args = append(args, strings.TrimSpace(unionID))
	}
	_, err = d.db.Exec(query, args...)
	return err
}

func normalizePluginAccount(item *PluginAccount) error {
	if item == nil {
		return fmt.Errorf("插件账号不能为空")
	}
	item.PluginID = strings.TrimSpace(item.PluginID)
	item.TableName = strings.TrimSpace(item.TableName)
	item.UnionID = strings.TrimSpace(item.UnionID)
	item.Platform = strings.TrimSpace(item.Platform)
	item.UserID = strings.TrimSpace(item.UserID)
	item.AccountName = strings.TrimSpace(item.AccountName)
	item.EnvName = strings.TrimSpace(item.EnvName)
	item.EnvValue = strings.TrimSpace(item.EnvValue)
	item.Remark = strings.TrimSpace(item.Remark)
	item.Status = strings.TrimSpace(item.Status)
	if item.Status == "" {
		item.Status = "active"
	}
	if item.Metadata == nil {
		item.Metadata = map[string]interface{}{}
	}
	if item.PluginID == "" || item.UnionID == "" || item.EnvName == "" || item.EnvValue == "" {
		return fmt.Errorf("插件账号缺少必要字段")
	}
	if item.AccountName == "" {
		item.AccountName = item.Remark
	}
	if item.AccountName == "" {
		item.AccountName = item.EnvName
	}
	return nil
}

func pluginAccountTableName(pluginID, storeName string) (string, error) {
	storeName = strings.TrimSpace(storeName)
	if storeName == "" {
		return defaultPluginAccountTable, nil
	}
	if !sqlIdentifierPattern.MatchString(storeName) {
		return "", fmt.Errorf("账号表名无效: %s", storeName)
	}
	return storeName, nil
}

func (d *Database) ensurePluginAccountTable(pluginID, tableName string) error {
	if tableName == defaultPluginAccountTable {
		return d.ensurePluginAccountBaseTableColumns()
	}
	if !sqlIdentifierPattern.MatchString(tableName) {
		return fmt.Errorf("账号表名无效: %s", tableName)
	}
	quoted := quoteIdentifier(tableName)
	if _, err := d.db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			plugin_id TEXT NOT NULL,
			union_id TEXT NOT NULL,
			platform TEXT NOT NULL DEFAULT '',
			user_id TEXT NOT NULL DEFAULT '',
			account_name TEXT NOT NULL DEFAULT '',
			env_name TEXT NOT NULL DEFAULT '',
			env_value TEXT NOT NULL DEFAULT '',
			remark TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'active',
			metadata TEXT NOT NULL DEFAULT '{}',
			expires_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`, quoted)); err != nil {
		return err
	}
	if err := d.ensurePluginAccountTableColumns(tableName); err != nil {
		return err
	}
	if _, err := d.db.Exec(fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s(plugin_id, union_id, env_name, status)`, quoteIdentifier("idx_"+tableName+"_query"), quoted)); err != nil {
		return err
	}
	if _, err := d.db.Exec(fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s(plugin_id, env_name, status)`, quoteIdentifier("idx_"+tableName+"_all_query"), quoted)); err != nil {
		return err
	}
	return d.migratePluginAccountsToCustomTable(pluginID, tableName)
}

func (d *Database) ensurePluginAccountTableColumns(tableName string) error {
	columns, err := d.TableColumns(tableName)
	if err != nil {
		return err
	}
	for _, column := range columns {
		if column.Name == "expires_at" {
			return nil
		}
	}
	_, err = d.db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN expires_at DATETIME`, quoteIdentifier(tableName)))
	return err
}

func (d *Database) migratePluginAccountsToCustomTable(pluginID, tableName string) error {
	if err := d.ensurePluginAccountBaseTableColumns(); err != nil {
		return err
	}
	var count int
	if err := d.db.QueryRow(fmt.Sprintf(`SELECT COUNT(1) FROM %s WHERE plugin_id = ?`, quoteIdentifier(tableName)), strings.TrimSpace(pluginID)).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := d.db.Exec(fmt.Sprintf(`
		INSERT INTO %s (id, plugin_id, union_id, platform, user_id, account_name, env_name, env_value, remark, status, metadata, expires_at, created_at, updated_at)
		SELECT id, plugin_id, union_id, platform, user_id, account_name, env_name, env_value, remark, status, metadata, expires_at, created_at, updated_at
		FROM plugin_accounts
		WHERE plugin_id = ?
	`, quoteIdentifier(tableName)), strings.TrimSpace(pluginID))
	return err
}

type pluginAccountScanner interface {
	Scan(dest ...interface{}) error
}

func scanPluginAccount(scanner pluginAccountScanner) (*PluginAccount, error) {
	var item PluginAccount
	var metadata string
	var expiresAt sql.NullTime
	if err := scanner.Scan(&item.ID, &item.PluginID, &item.UnionID, &item.Platform, &item.UserID, &item.AccountName, &item.EnvName, &item.EnvValue, &item.Remark, &item.Status, &metadata, &expiresAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return nil, err
	}
	item.Metadata = map[string]interface{}{}
	_ = json.Unmarshal([]byte(metadata), &item.Metadata)
	if expiresAt.Valid {
		item.ExpiresAt = &expiresAt.Time
	}
	return &item, nil
}
