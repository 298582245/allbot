package config

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Database 配置数据库
type Database struct {
	db *sql.DB
}

// NewDatabase 创建配置数据库
func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// 创建表
	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("创建表失败: %w", err)
	}

	return &Database{db: db}, nil
}

// createTables 创建数据库表
func createTables(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS adapters (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		platform TEXT NOT NULL UNIQUE,
		enabled INTEGER NOT NULL DEFAULT 0,
		config TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	`

	_, err := db.Exec(schema)
	return err
}

// GetAllAdapters 获取所有适配器配置
func (d *Database) GetAllAdapters() ([]*AdapterConfig, error) {
	rows, err := d.db.Query(`
		SELECT id, platform, enabled, config, created_at, updated_at
		FROM adapters
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var adapters []*AdapterConfig
	for rows.Next() {
		var a AdapterConfig
		var enabled int
		if err := rows.Scan(&a.ID, &a.Platform, &enabled, &a.Config, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		a.Enabled = enabled == 1
		adapters = append(adapters, &a)
	}

	return adapters, rows.Err()
}

// GetAdapter 获取指定平台的适配器配置
func (d *Database) GetAdapter(platform string) (*AdapterConfig, error) {
	var a AdapterConfig
	var enabled int
	err := d.db.QueryRow(`
		SELECT id, platform, enabled, config, created_at, updated_at
		FROM adapters
		WHERE platform = ?
	`, platform).Scan(&a.ID, &a.Platform, &enabled, &a.Config, &a.CreatedAt, &a.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	a.Enabled = enabled == 1
	return &a, nil
}

// SaveAdapter 保存适配器配置
func (d *Database) SaveAdapter(adapter *AdapterConfig) error {
	now := time.Now()
	adapter.UpdatedAt = now

	// 检查是否存在
	existing, err := d.GetAdapter(adapter.Platform)
	if err != nil {
		return err
	}

	enabled := 0
	if adapter.Enabled {
		enabled = 1
	}

	if existing == nil {
		// 插入
		adapter.CreatedAt = now
		result, err := d.db.Exec(`
			INSERT INTO adapters (platform, enabled, config, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)
		`, adapter.Platform, enabled, adapter.Config, adapter.CreatedAt, adapter.UpdatedAt)
		if err != nil {
			return err
		}
		id, _ := result.LastInsertId()
		adapter.ID = id
	} else {
		// 更新
		_, err := d.db.Exec(`
			UPDATE adapters
			SET enabled = ?, config = ?, updated_at = ?
			WHERE platform = ?
		`, enabled, adapter.Config, adapter.UpdatedAt, adapter.Platform)
		if err != nil {
			return err
		}
		adapter.ID = existing.ID
		adapter.CreatedAt = existing.CreatedAt
	}

	return nil
}

// DeleteAdapter 删除适配器配置
func (d *Database) DeleteAdapter(platform string) error {
	_, err := d.db.Exec(`DELETE FROM adapters WHERE platform = ?`, platform)
	return err
}

// Close 关闭数据库
func (d *Database) Close() error {
	return d.db.Close()
}

// ParseQQConfig 解析 QQ 配置
func ParseQQConfig(configJSON string) (*QQConfig, error) {
	var config QQConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// ParseWeChatConfig 解析微信配置
func ParseWeChatConfig(configJSON string) (*WeChatConfig, error) {
	var config WeChatConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// ParseTelegramConfig 解析 Telegram 配置
func ParseTelegramConfig(configJSON string) (*TelegramConfig, error) {
	var config TelegramConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return nil, err
	}
	return &config, nil
}
