package config

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Database struct {
	db       *sql.DB
	pointsMu sync.Mutex
}

func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("初始化数据库参数失败: %w", err)
	}

	if err := createTables(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("创建表失败: %w", err)
	}

	return &Database{db: db}, nil
}

func createTables(db *sql.DB) error {
	if err := migrateAdaptersTable(db); err != nil {
		return err
	}
	if err := migrateScheduledTasksTable(db); err != nil {
		return err
	}

	schema := `
	CREATE TABLE IF NOT EXISTS adapters (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		platform TEXT NOT NULL,
		remark TEXT NOT NULL DEFAULT '',
		description TEXT NOT NULL DEFAULT '',
		enabled INTEGER NOT NULL DEFAULT 0,
		config TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	`
	if _, err := db.Exec(schema); err != nil {
		return err
	}

	settingsSchema := `
	CREATE TABLE IF NOT EXISTS system_settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS data_views (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		plugin_id TEXT NOT NULL DEFAULT '',
		table_name TEXT NOT NULL,
		view_name TEXT NOT NULL,
		group_name TEXT NOT NULL DEFAULT '业务数据',
		description TEXT NOT NULL DEFAULT '',
		columns TEXT NOT NULL DEFAULT '[]',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(plugin_id, table_name)
	);

	CREATE TABLE IF NOT EXISTS keyword_replies (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		keyword TEXT NOT NULL,
		match_type TEXT NOT NULL DEFAULT 'regex',
		reply_type TEXT NOT NULL DEFAULT 'text',
		content TEXT NOT NULL DEFAULT '',
		enabled INTEGER NOT NULL DEFAULT 1,
		admin_only INTEGER NOT NULL DEFAULT 0,
		pinned INTEGER NOT NULL DEFAULT 0,
		builtin INTEGER NOT NULL DEFAULT 0,
		schedule_enabled INTEGER NOT NULL DEFAULT 0,
		schedule_cron TEXT NOT NULL DEFAULT '',
		description TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS user_accounts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		platform TEXT NOT NULL,
		user_id TEXT NOT NULL,
		union_id TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(platform, user_id)
	);

	CREATE INDEX IF NOT EXISTS idx_user_accounts_union_id ON user_accounts(union_id);

	CREATE TABLE IF NOT EXISTS plugin_accounts (
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

	CREATE INDEX IF NOT EXISTS idx_plugin_accounts_plugin_union ON plugin_accounts(plugin_id, union_id);
	CREATE INDEX IF NOT EXISTS idx_plugin_accounts_plugin_env ON plugin_accounts(plugin_id, env_name);
	CREATE INDEX IF NOT EXISTS idx_plugin_accounts_query ON plugin_accounts(plugin_id, union_id, env_name, status);
	CREATE INDEX IF NOT EXISTS idx_plugin_accounts_all_query ON plugin_accounts(plugin_id, env_name, status);

	CREATE TABLE IF NOT EXISTS plugin_authorizations (
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

	CREATE INDEX IF NOT EXISTS idx_plugin_authorizations_plugin_union ON plugin_authorizations(plugin_id, union_id);
	CREATE INDEX IF NOT EXISTS idx_plugin_authorizations_active ON plugin_authorizations(plugin_id, status, expires_at);

	CREATE TABLE IF NOT EXISTS plugin_template_metadata (
		plugin_id TEXT PRIMARY KEY,
		template TEXT NOT NULL,
		template_version TEXT NOT NULL,
		runtime TEXT NOT NULL,
		structure TEXT NOT NULL DEFAULT '',
		metadata TEXT NOT NULL DEFAULT '{}',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS script_run_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		plugin_id TEXT NOT NULL,
		union_id TEXT NOT NULL DEFAULT '',
		script_path TEXT NOT NULL DEFAULT '',
		runtime TEXT NOT NULL DEFAULT '',
		run_mode TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT '',
		output TEXT NOT NULL DEFAULT '',
		error TEXT NOT NULL DEFAULT '',
		started_at DATETIME NOT NULL,
		finished_at DATETIME NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_script_run_logs_plugin_time ON script_run_logs(plugin_id, started_at DESC);
	CREATE INDEX IF NOT EXISTS idx_script_run_logs_union_time ON script_run_logs(union_id, started_at DESC);

	CREATE TABLE IF NOT EXISTS user_points (
		union_id TEXT PRIMARY KEY,
		points INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS user_bind_codes (
		code TEXT PRIMARY KEY,
		platform TEXT NOT NULL,
		user_id TEXT NOT NULL,
		union_id TEXT NOT NULL,
		expires_at DATETIME NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS scheduled_tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		plugin_id TEXT NOT NULL DEFAULT '',
		task_key TEXT NOT NULL DEFAULT '',
		name TEXT NOT NULL DEFAULT '',
		description TEXT NOT NULL DEFAULT '',
		enabled INTEGER NOT NULL DEFAULT 1,
		pinned INTEGER NOT NULL DEFAULT 0,
		cron TEXT NOT NULL,
		platform TEXT NOT NULL,
		adapter_id TEXT NOT NULL DEFAULT '',
		user_id TEXT NOT NULL,
		group_id TEXT NOT NULL DEFAULT '',
		content TEXT NOT NULL,
		source TEXT NOT NULL DEFAULT 'user',
		last_run_at DATETIME,
		next_run_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE UNIQUE INDEX IF NOT EXISTS idx_scheduled_tasks_plugin_key ON scheduled_tasks(plugin_id, task_key) WHERE plugin_id <> '' AND task_key <> '';
	CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_enabled_next_run ON scheduled_tasks(enabled, next_run_at);
	CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_plugin ON scheduled_tasks(plugin_id, created_at);

	CREATE TABLE IF NOT EXISTS message_stats (
		stat_date TEXT NOT NULL,
		stat_hour INTEGER NOT NULL,
		platform TEXT NOT NULL,
		adapter_id TEXT NOT NULL DEFAULT '',
		adapter_name TEXT NOT NULL DEFAULT '',
		count INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (stat_date, stat_hour, platform, adapter_id)
	);

	CREATE INDEX IF NOT EXISTS idx_message_stats_date_platform ON message_stats(stat_date, platform);
	CREATE INDEX IF NOT EXISTS idx_message_stats_date_adapter ON message_stats(stat_date, adapter_id);
	`
	if _, err := db.Exec(settingsSchema); err != nil {
		return err
	}

	if err := backfillUserPoints(db); err != nil {
		return err
	}
	if err := ensureDefaultSystemSettings(db); err != nil {
		return err
	}
	return ensureBuiltinKeywordReplies(db)
}

func backfillUserPoints(db *sql.DB) error {
	columns, err := tableColumns(db, "user_accounts")
	if err != nil {
		return err
	}
	if !columns["points"] {
		_, err = db.Exec(`
			INSERT OR IGNORE INTO user_points (union_id, points, created_at, updated_at)
				SELECT union_id, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
				FROM user_accounts
				GROUP BY union_id
		`)
		return err
	}
	_, err = db.Exec(`
		INSERT OR IGNORE INTO user_points (union_id, points, created_at, updated_at)
			SELECT union_id, COALESCE(MAX(points), 0), CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
			FROM user_accounts
			GROUP BY union_id
	`)
	return err
}

func ensureBuiltinKeywordReplies(db *sql.DB) error {
	items := []struct {
		keyword     string
		description string
		adminOnly   bool
	}{
		{"myid", "返回当前用户身份信息", false},
		{"注册", "注册当前平台用户身份", false},
		{"积分充值", "平台管理员给指定用户充值积分，格式：积分充值 <unionId或平台:userId> <数量>", true},
		{"绑定码", "私聊获取跨平台绑定码", false},
		{"绑定", "私聊使用绑定码绑定其他平台身份", false},
		{"groupId", "返回当前群组 ID，私聊不响应", false},
		{"system", "返回系统运行信息", true},
		{"version", "返回框架版本信息", false},
		{"重启", "平台管理员触发 AllBot 进程重启", true},
	}
	for _, item := range items {
		if _, err := db.Exec(`
			INSERT INTO keyword_replies (keyword, match_type, reply_type, content, enabled, admin_only, pinned, builtin, description, created_at, updated_at)
			SELECT ?, 'exact', 'builtin', ?, 1, ?, 1, 1, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
			WHERE NOT EXISTS (SELECT 1 FROM keyword_replies WHERE builtin = 1 AND keyword = ?)
		`, item.keyword, item.keyword, boolInt(item.adminOnly), item.description, item.keyword); err != nil {
			return err
		}
		if _, err := db.Exec(`
			UPDATE keyword_replies
			SET admin_only = ?, description = ?, updated_at = CURRENT_TIMESTAMP
			WHERE builtin = 1 AND keyword = ?
		`, boolInt(item.adminOnly), item.description, item.keyword); err != nil {
			return err
		}
	}
	return nil
}

func ensureDefaultSystemSettings(db *sql.DB) error {
	if _, err := db.Exec(`DELETE FROM system_settings WHERE key IN ('web.port', 'web_port')`); err != nil {
		return err
	}

	defaults := map[string]string{
		"admin.username":       "admin",
		"admin.platform_users": "[]",
		"web.auto_refresh":     "true",
		"web.refresh_interval": "5",
		"plugin.dir":           "./plugins",
		"plugin.auto_load":     "true",
		"user.points_unit":     "积分",
		"access_control":       "{}",
	}
	descriptions := map[string]string{
		"admin.username":       "管理员用户名",
		"admin.platform_users": "平台管理员用户列表",
		"web.auto_refresh":     "是否自动刷新",
		"web.refresh_interval": "刷新间隔秒数",
		"plugin.dir":           "插件目录",
		"plugin.auto_load":     "启动时自动加载插件",
		"user.points_unit":     "用户积分单位",
		"access_control":       "系统访问控制配置",
	}

	for key, value := range defaults {
		if _, err := db.Exec(`
			INSERT OR IGNORE INTO system_settings (key, value, description, created_at, updated_at)
			VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		`, key, value, descriptions[key]); err != nil {
			return err
		}
	}
	return nil
}

func migrateAdaptersTable(db *sql.DB) error {
	var tableName string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='adapters'`).Scan(&tableName)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}

	columns, err := tableColumns(db, "adapters")
	if err != nil {
		return err
	}

	needsRebuild := columns["platform"] && hasUniqueIndexOnPlatform(db)
	if columns["remark"] && columns["description"] && !needsRebuild {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		CREATE TABLE adapters_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			platform TEXT NOT NULL,
			remark TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 0,
			config TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return err
	}

	remarkExpr := "''"
	if columns["remark"] {
		remarkExpr = "COALESCE(remark, '')"
	}
	descriptionExpr := "''"
	if columns["description"] {
		descriptionExpr = "COALESCE(description, '')"
	}

	copySQL := fmt.Sprintf(`
		INSERT INTO adapters_new (id, platform, remark, description, enabled, config, created_at, updated_at)
		SELECT id, platform, %s, %s, enabled, config, created_at, updated_at FROM adapters
	`, remarkExpr, descriptionExpr)
	if _, err := tx.Exec(copySQL); err != nil {
		return err
	}

	if _, err := tx.Exec(`DROP TABLE adapters`); err != nil {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE adapters_new RENAME TO adapters`); err != nil {
		return err
	}

	return tx.Commit()
}

func migrateScheduledTasksTable(db *sql.DB) error {
	var tableName string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='scheduled_tasks'`).Scan(&tableName)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	columns, err := tableColumns(db, "scheduled_tasks")
	if err != nil {
		return err
	}
	if !columns["adapter_id"] {
		if _, err := db.Exec(`ALTER TABLE scheduled_tasks ADD COLUMN adapter_id TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columns["pinned"] {
		if _, err := db.Exec(`ALTER TABLE scheduled_tasks ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}
	return nil
}

func tableColumns(db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return nil, err
		}
		columns[name] = true
	}
	return columns, rows.Err()
}

func hasUniqueIndexOnPlatform(db *sql.DB) bool {
	rows, err := db.Query(`PRAGMA index_list(adapters)`)
	if err != nil {
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var seq int
		var name string
		var unique int
		var origin string
		var partial int
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil || unique == 0 {
			continue
		}

		indexRows, err := db.Query(`PRAGMA index_info(` + name + `)`)
		if err != nil {
			continue
		}
		columnCount := 0
		platformOnly := false
		for indexRows.Next() {
			var seqno, cid int
			var columnName string
			if err := indexRows.Scan(&seqno, &cid, &columnName); err == nil {
				columnCount++
				platformOnly = columnName == "platform"
			}
		}
		indexRows.Close()
		if columnCount == 1 && platformOnly {
			return true
		}
	}
	return false
}

func (d *Database) GetAllAdapters() ([]*AdapterConfig, error) {
	rows, err := d.db.Query(`
		SELECT id, platform, remark, description, enabled, config, created_at, updated_at
		FROM adapters
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var adapters []*AdapterConfig
	for rows.Next() {
		adapterConfig, err := scanAdapter(rows)
		if err != nil {
			return nil, err
		}
		adapters = append(adapters, adapterConfig)
	}

	return adapters, rows.Err()
}

func (d *Database) GetAdapter(platform string) (*AdapterConfig, error) {
	var adapter AdapterConfig
	var enabled int
	err := d.db.QueryRow(`
		SELECT id, platform, remark, description, enabled, config, created_at, updated_at
		FROM adapters
		WHERE platform = ?
		ORDER BY id
		LIMIT 1
	`, platform).Scan(&adapter.ID, &adapter.Platform, &adapter.Remark, &adapter.Description, &enabled, &adapter.Config, &adapter.CreatedAt, &adapter.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	adapter.Enabled = enabled == 1
	return &adapter, nil
}

func (d *Database) GetAdapterByID(id int64) (*AdapterConfig, error) {
	var adapter AdapterConfig
	var enabled int
	err := d.db.QueryRow(`
		SELECT id, platform, remark, description, enabled, config, created_at, updated_at
		FROM adapters
		WHERE id = ?
	`, id).Scan(&adapter.ID, &adapter.Platform, &adapter.Remark, &adapter.Description, &enabled, &adapter.Config, &adapter.CreatedAt, &adapter.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	adapter.Enabled = enabled == 1
	return &adapter, nil
}

func (d *Database) SaveAdapter(adapter *AdapterConfig) error {
	now := time.Now()
	adapter.UpdatedAt = now

	enabled := 0
	if adapter.Enabled {
		enabled = 1
	}

	if adapter.ID == 0 {
		adapter.CreatedAt = now
		result, err := d.db.Exec(`
			INSERT INTO adapters (platform, remark, description, enabled, config, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, adapter.Platform, adapter.Remark, adapter.Description, enabled, adapter.Config, adapter.CreatedAt, adapter.UpdatedAt)
		if err != nil {
			return err
		}
		id, _ := result.LastInsertId()
		adapter.ID = id
		return nil
	}

	existing, err := d.GetAdapterByID(adapter.ID)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("适配器不存在: %d", adapter.ID)
	}

	_, err = d.db.Exec(`
		UPDATE adapters
		SET platform = ?, remark = ?, description = ?, enabled = ?, config = ?, updated_at = ?
		WHERE id = ?
	`, adapter.Platform, adapter.Remark, adapter.Description, enabled, adapter.Config, adapter.UpdatedAt, adapter.ID)
	if err != nil {
		return err
	}

	adapter.CreatedAt = existing.CreatedAt
	return nil
}

func (d *Database) DeleteAdapter(platform string) error {
	_, err := d.db.Exec(`DELETE FROM adapters WHERE platform = ?`, platform)
	return err
}

func (d *Database) DeleteAdapterByID(id int64) error {
	_, err := d.db.Exec(`DELETE FROM adapters WHERE id = ?`, id)
	return err
}

func scanAdapter(scanner interface {
	Scan(dest ...interface{}) error
}) (*AdapterConfig, error) {
	var adapter AdapterConfig
	var enabled int
	if err := scanner.Scan(&adapter.ID, &adapter.Platform, &adapter.Remark, &adapter.Description, &enabled, &adapter.Config, &adapter.CreatedAt, &adapter.UpdatedAt); err != nil {
		return nil, err
	}
	adapter.Enabled = enabled == 1
	return &adapter, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func ParseQQConfig(configJSON string) (*QQConfig, error) {
	var config QQConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return nil, err
	}
	var raw map[string]interface{}
	_ = json.Unmarshal([]byte(configJSON), &raw)
	if config.ServerURL == "" {
		if value, ok := raw["server_url"].(string); ok {
			config.ServerURL = value
		}
	}
	if config.ServerURL == "" {
		if value, ok := raw["api_url"].(string); ok {
			config.ServerURL = value
		}
	}
	return &config, nil
}

func ParseQQOfficeConfig(configJSON string) (*QQOfficeConfig, error) {
	var config QQOfficeConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return nil, err
	}
	config.AppID = strings.TrimSpace(config.AppID)
	config.ClientSecret = strings.TrimSpace(config.ClientSecret)
	config.APIBaseURL = strings.TrimSpace(config.APIBaseURL)
	config.TokenURL = strings.TrimSpace(config.TokenURL)
	if config.AppID == "" {
		return nil, fmt.Errorf("app_id 不能为空")
	}
	if config.ClientSecret == "" {
		return nil, fmt.Errorf("client_secret 不能为空")
	}
	if config.APIBaseURL == "" {
		config.APIBaseURL = "https://api.sgroup.qq.com"
	}
	if config.TokenURL == "" {
		config.TokenURL = "https://bots.qq.com/app/getAppAccessToken"
	}
	return &config, nil
}

func ParseWeChatConfig(configJSON string) (*WeChatConfig, error) {
	var config WeChatConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func ParseTelegramConfig(configJSON string) (*TelegramConfig, error) {
	var config TelegramConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return nil, err
	}
	return &config, nil
}
