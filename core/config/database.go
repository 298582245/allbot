package config

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Database struct {
	db       *sql.DB
	path     string
	pointsMu sync.Mutex
}

func NewDatabase(dbPath string) (*Database, error) {
	storedPath := dbPath
	if strings.TrimSpace(dbPath) != ":memory:" {
		absPath, err := filepath.Abs(dbPath)
		if err != nil {
			return nil, fmt.Errorf("解析数据库路径失败: %w", err)
		}
		storedPath = absPath
	}
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

	return &Database{db: db, path: storedPath}, nil
}

func (d *Database) Path() string {
	if d == nil {
		return ""
	}
	return d.path
}

func (d *Database) ReplaceWith(sourcePath string) error {
	if d == nil || d.db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if d.path == "" || d.path == ":memory:" {
		return fmt.Errorf("当前数据库路径不支持恢复")
	}
	if err := d.db.Close(); err != nil {
		return err
	}
	if err := copyFile(sourcePath, d.path); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", d.path)
	if err != nil {
		return fmt.Errorf("打开恢复后的数据库失败: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON;`); err != nil {
		_ = db.Close()
		return fmt.Errorf("初始化恢复后的数据库参数失败: %w", err)
	}
	if err := createTables(db); err != nil {
		_ = db.Close()
		return fmt.Errorf("迁移恢复后的数据库失败: %w", err)
	}
	d.db = db
	return nil
}

func copyFile(sourcePath, targetPath string) error {
	input, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return err
	}
	tmpPath := targetPath + ".restore-tmp"
	output, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return closeErr
	}
	for _, stalePath := range []string{targetPath, targetPath + "-wal", targetPath + "-shm"} {
		if err := os.Remove(stalePath); err != nil && !os.IsNotExist(err) {
			_ = os.Remove(tmpPath)
			return err
		}
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func createTables(db *sql.DB) error {
	if err := migrateAdaptersTable(db); err != nil {
		return err
	}
	if err := migrateScheduledTasksTable(db); err != nil {
		return err
	}
	if err := migrateScriptRunLogsTable(db); err != nil {
		return err
	}
	if err := migrateScriptEnvVarsTable(db); err != nil {
		return err
	}
	if err := migrateMessageStatsTable(db); err != nil {
		return err
	}
	if err := migrateWebChatMessagesTable(db); err != nil {
		return err
	}
	schema := `
	CREATE TABLE IF NOT EXISTS adapters (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		platform TEXT NOT NULL,
		remark TEXT NOT NULL DEFAULT '',
		description TEXT NOT NULL DEFAULT '',
		enabled INTEGER NOT NULL DEFAULT 0,
		pinned INTEGER NOT NULL DEFAULT 0,
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

	CREATE TABLE IF NOT EXISTS script_env_vars (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		value TEXT NOT NULL DEFAULT '',
		remark TEXT NOT NULL DEFAULT '',
		enabled INTEGER NOT NULL DEFAULT 1,
		pinned INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_script_env_vars_enabled ON script_env_vars(enabled, name);
	CREATE INDEX IF NOT EXISTS idx_script_env_vars_pinned ON script_env_vars(pinned DESC, name ASC, id ASC);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_script_env_vars_name_value_unique ON script_env_vars(name, value);

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
		runtime_profile TEXT NOT NULL DEFAULT '',
		run_mode TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT '',
		run_total INTEGER NOT NULL DEFAULT 0,
		failed_total INTEGER NOT NULL DEFAULT 0,
		output TEXT NOT NULL DEFAULT '',
		error TEXT NOT NULL DEFAULT '',
		started_at DATETIME NOT NULL,
		finished_at DATETIME NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_script_run_logs_plugin_time ON script_run_logs(plugin_id, started_at DESC);
	CREATE INDEX IF NOT EXISTS idx_script_run_logs_union_time ON script_run_logs(union_id, started_at DESC);
	CREATE INDEX IF NOT EXISTS idx_script_run_logs_status_created_at ON script_run_logs(status, created_at, id);
	CREATE INDEX IF NOT EXISTS idx_script_run_logs_queue ON script_run_logs(status, created_at, id) WHERE status = 'queued';

	CREATE TABLE IF NOT EXISTS script_run_stats (
		stat_date TEXT NOT NULL,
		stat_hour INTEGER NOT NULL,
		plugin_id TEXT NOT NULL,
		script_path TEXT NOT NULL DEFAULT '',
		runtime TEXT NOT NULL DEFAULT '',
		runtime_profile TEXT NOT NULL DEFAULT '',
		run_mode TEXT NOT NULL DEFAULT '',
		run_total INTEGER NOT NULL DEFAULT 0,
		success_total INTEGER NOT NULL DEFAULT 0,
		failed_total INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (stat_date, stat_hour, plugin_id, script_path, runtime, runtime_profile, run_mode)
	);

	CREATE INDEX IF NOT EXISTS idx_script_run_stats_date ON script_run_stats(stat_date);
	CREATE INDEX IF NOT EXISTS idx_script_run_stats_plugin_date ON script_run_stats(plugin_id, stat_date);

	CREATE TABLE IF NOT EXISTS user_points (
		union_id TEXT PRIMARY KEY,
		points INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS payment_orders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		order_no TEXT NOT NULL UNIQUE,
		plugin_id TEXT NOT NULL DEFAULT '',
		union_id TEXT NOT NULL,
		platform TEXT NOT NULL DEFAULT '',
		adapter_id TEXT NOT NULL DEFAULT '',
		user_id TEXT NOT NULL DEFAULT '',
		group_id TEXT NOT NULL DEFAULT '',
		subject TEXT NOT NULL,
		amount_cents INTEGER NOT NULL,
		points_amount INTEGER NOT NULL,
		provider TEXT NOT NULL,
		method TEXT NOT NULL,
		status TEXT NOT NULL,
		provider_order_no TEXT NOT NULL DEFAULT '',
		pay_url TEXT NOT NULL DEFAULT '',
		qrcode TEXT NOT NULL DEFAULT '',
		notify_raw TEXT NOT NULL DEFAULT '',
		metadata TEXT NOT NULL DEFAULT '{}',
		expired_at DATETIME NOT NULL,
		paid_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_payment_orders_union_id ON payment_orders(union_id);
	CREATE INDEX IF NOT EXISTS idx_payment_orders_status ON payment_orders(status);
	CREATE INDEX IF NOT EXISTS idx_payment_orders_provider_order_no ON payment_orders(provider, provider_order_no);
	CREATE INDEX IF NOT EXISTS idx_payment_orders_created_at ON payment_orders(created_at);

	CREATE TABLE IF NOT EXISTS payment_alipay_bill_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		provider_order_no TEXT NOT NULL,
		account_log_id TEXT NOT NULL DEFAULT '',
		order_no TEXT NOT NULL DEFAULT '',
		amount_cents INTEGER NOT NULL,
		direction TEXT NOT NULL,
		remark TEXT NOT NULL DEFAULT '',
		summary TEXT NOT NULL DEFAULT '',
		opposite_account TEXT NOT NULL DEFAULT '',
		paid_at DATETIME NOT NULL,
		raw TEXT NOT NULL DEFAULT '{}',
		matched_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_alipay_bill_records_provider_order_no ON payment_alipay_bill_records(provider_order_no);
	CREATE INDEX IF NOT EXISTS idx_payment_alipay_bill_records_order_no ON payment_alipay_bill_records(order_no);
	CREATE INDEX IF NOT EXISTS idx_payment_alipay_bill_records_paid_at ON payment_alipay_bill_records(paid_at);

	CREATE TABLE IF NOT EXISTS payment_provider_states (
		provider TEXT NOT NULL,
		state_key TEXT NOT NULL,
		state_value TEXT NOT NULL DEFAULT '',
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (provider, state_key)
	);

	CREATE TABLE IF NOT EXISTS payment_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		order_no TEXT NOT NULL,
		event_type TEXT NOT NULL,
		message TEXT NOT NULL DEFAULT '',
		payload TEXT NOT NULL DEFAULT '{}',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_payment_events_order_no ON payment_events(order_no);
	CREATE INDEX IF NOT EXISTS idx_payment_events_created_at ON payment_events(created_at);

	CREATE TABLE IF NOT EXISTS point_transactions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		union_id TEXT NOT NULL,
		delta INTEGER NOT NULL,
		balance_after INTEGER NOT NULL,
		source TEXT NOT NULL,
		source_id TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_point_transactions_union_id ON point_transactions(union_id);
	CREATE INDEX IF NOT EXISTS idx_point_transactions_source ON point_transactions(source, source_id);
	CREATE INDEX IF NOT EXISTS idx_point_transactions_created_at ON point_transactions(created_at);

	CREATE TABLE IF NOT EXISTS user_bind_codes (
		code TEXT PRIMARY KEY,
		platform TEXT NOT NULL,
		user_id TEXT NOT NULL,
		union_id TEXT NOT NULL,
		expires_at DATETIME NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS web_chat_users (
		user_id TEXT PRIMARY KEY,
		username TEXT NOT NULL UNIQUE,
		email TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		display_name TEXT NOT NULL DEFAULT '',
		email_verified INTEGER NOT NULL DEFAULT 0,
		disabled INTEGER NOT NULL DEFAULT 0,
		last_login_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS web_chat_email_codes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT NOT NULL,
		code_hash TEXT NOT NULL,
		purpose TEXT NOT NULL,
		expires_at DATETIME NOT NULL,
		attempts INTEGER NOT NULL DEFAULT 0,
		sent_ip TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		used_at DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_web_chat_email_codes_email ON web_chat_email_codes(email, purpose, created_at);
	CREATE INDEX IF NOT EXISTS idx_web_chat_email_codes_ip ON web_chat_email_codes(sent_ip, created_at);

	CREATE TABLE IF NOT EXISTS web_chat_platform_codes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		platform TEXT NOT NULL,
		adapter_id TEXT NOT NULL DEFAULT '',
		user_id TEXT NOT NULL,
		union_id TEXT NOT NULL,
		code_hash TEXT NOT NULL,
		purpose TEXT NOT NULL DEFAULT 'login',
		expires_at DATETIME NOT NULL,
		attempts INTEGER NOT NULL DEFAULT 0,
		sent_ip TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		used_at DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_web_chat_platform_codes_target ON web_chat_platform_codes(platform, adapter_id, user_id, purpose, created_at);
	CREATE INDEX IF NOT EXISTS idx_web_chat_platform_codes_union ON web_chat_platform_codes(union_id, purpose, created_at);
	CREATE INDEX IF NOT EXISTS idx_web_chat_platform_codes_ip ON web_chat_platform_codes(sent_ip, created_at);

	CREATE TABLE IF NOT EXISTS web_chat_sessions (
		token_hash TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		csrf_token TEXT NOT NULL,
		expires_at DATETIME NOT NULL,
		user_agent TEXT NOT NULL DEFAULT '',
		client_ip TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id) REFERENCES web_chat_users(user_id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_web_chat_sessions_user ON web_chat_sessions(user_id, expires_at);

	CREATE TABLE IF NOT EXISTS web_chat_messages (
		message_id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id TEXT NOT NULL,
		direction TEXT NOT NULL,
		message_type TEXT NOT NULL,
		content TEXT NOT NULL DEFAULT '',
		image_url TEXT NOT NULL DEFAULT '',
		rich_json TEXT NOT NULL DEFAULT '',
		target TEXT NOT NULL DEFAULT '',
		plugin_id TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id) REFERENCES web_chat_users(user_id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_web_chat_messages_user ON web_chat_messages(user_id, message_id);
	CREATE INDEX IF NOT EXISTS idx_web_chat_messages_user_plugin_message ON web_chat_messages(user_id, plugin_id, message_id);

	CREATE TABLE IF NOT EXISTS web_chat_read_states (
		user_id TEXT NOT NULL,
		plugin_id TEXT NOT NULL DEFAULT '',
		last_read_message_id INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, plugin_id),
		FOREIGN KEY(user_id) REFERENCES web_chat_users(user_id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_web_chat_read_states_user ON web_chat_read_states(user_id, plugin_id);

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
		message_type TEXT NOT NULL DEFAULT 'private',
		count INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (stat_date, stat_hour, platform, adapter_id, message_type)
	);

	CREATE INDEX IF NOT EXISTS idx_message_stats_date_platform ON message_stats(stat_date, platform);
	CREATE INDEX IF NOT EXISTS idx_message_stats_date_adapter ON message_stats(stat_date, adapter_id);
	CREATE INDEX IF NOT EXISTS idx_message_stats_date_type ON message_stats(stat_date, message_type);

	CREATE TABLE IF NOT EXISTS plugin_trigger_stats (
		stat_date TEXT NOT NULL,
		stat_hour INTEGER NOT NULL,
		plugin_id TEXT NOT NULL,
		plugin_name TEXT NOT NULL DEFAULT '',
		trigger_pattern TEXT NOT NULL DEFAULT '',
		platform TEXT NOT NULL DEFAULT '',
		adapter_id TEXT NOT NULL DEFAULT '',
		adapter_name TEXT NOT NULL DEFAULT '',
		count INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (stat_date, stat_hour, plugin_id, platform, adapter_id)
	);

	CREATE INDEX IF NOT EXISTS idx_plugin_trigger_stats_date_plugin ON plugin_trigger_stats(stat_date, plugin_id);
	CREATE INDEX IF NOT EXISTS idx_plugin_trigger_stats_plugin_date ON plugin_trigger_stats(plugin_id, stat_date);
	CREATE INDEX IF NOT EXISTS idx_plugin_trigger_stats_date_platform ON plugin_trigger_stats(stat_date, platform);

	CREATE TABLE IF NOT EXISTS image_assets (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		public_id TEXT NOT NULL UNIQUE,
		original_name TEXT NOT NULL DEFAULT '',
		storage_key TEXT NOT NULL,
		ext TEXT NOT NULL,
		content_type TEXT NOT NULL,
		size_bytes INTEGER NOT NULL,
		width INTEGER NOT NULL DEFAULT 0,
		height INTEGER NOT NULL DEFAULT 0,
		sha256 TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_image_assets_created_at ON image_assets(created_at);
	CREATE INDEX IF NOT EXISTS idx_image_assets_content_type ON image_assets(content_type);
	CREATE INDEX IF NOT EXISTS idx_image_assets_sha256 ON image_assets(sha256);
	`
	if _, err := db.Exec(settingsSchema); err != nil {
		return err
	}

	if err := migrateUsersTable(db); err != nil {
		return err
	}
	if err := backfillUserPoints(db); err != nil {
		return err
	}
	if err := ensureDefaultSystemSettings(db); err != nil {
		return err
	}
	if err := backfillScriptRunStats(db); err != nil {
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
		keyword       string
		lookupKeyword string
		content       string
		matchType     string
		description   string
		adminOnly     bool
	}{
		{keyword: "myid", content: "myid", matchType: "exact", description: "返回当前用户身份信息", adminOnly: false},
		{keyword: "注册", content: "注册", matchType: "exact", description: "注册当前平台用户身份", adminOnly: false},
		{keyword: "积分充值", content: "积分充值", matchType: "exact", description: "用户自助充值积分；平台管理员可给指定用户加积分，格式：积分充值 <unionId或平台:userId> <数量>", adminOnly: false},
		{keyword: "用户搜索", content: "用户搜索", matchType: "exact", description: "平台管理员按 union_id 或平台用户号查询关联平台账号和积分，格式：用户搜索 <unionId> / 用户搜索 <平台>:<用户号> / 用户搜索 <平台> <用户号>", adminOnly: true},
		{keyword: "绑定码", content: "绑定码", matchType: "exact", description: "私聊获取跨平台绑定码", adminOnly: false},
		{keyword: "绑定", content: "绑定", matchType: "exact", description: "私聊使用绑定码绑定其他平台身份", adminOnly: false},
		{keyword: "我的平台", content: "我的平台", matchType: "exact", description: "私聊查看当前 union_id 已绑定的平台和用户 ID", adminOnly: false},
		{keyword: "groupId", content: "groupId", matchType: "exact", description: "返回当前群组 ID，私聊不响应", adminOnly: false},
		{keyword: "插件列表", content: "插件列表", matchType: "exact", description: "平台管理员交互式管理插件启停和访问控制", adminOnly: true},
		{keyword: "system", content: "system", matchType: "exact", description: "返回系统运行信息", adminOnly: true},
		{keyword: `(?i)^v(ersion)?$`, lookupKeyword: "version", content: "version", matchType: "regex", description: "返回框架版本信息", adminOnly: false},
		{keyword: "更新", content: "更新", matchType: "exact", description: "平台管理员触发 AllBot 一键升级", adminOnly: true},
		{keyword: "重启", content: "重启", matchType: "exact", description: "平台管理员触发 AllBot 进程重启", adminOnly: true},
	}
	for _, item := range items {
		lookupKeyword := item.lookupKeyword
		if lookupKeyword == "" {
			lookupKeyword = item.keyword
		}
		content := item.content
		if content == "" {
			content = item.keyword
		}
		matchType := item.matchType
		if matchType == "" {
			matchType = "exact"
		}
		if _, err := db.Exec(`
			INSERT INTO keyword_replies (keyword, match_type, reply_type, content, enabled, admin_only, pinned, builtin, description, created_at, updated_at)
			SELECT ?, ?, 'builtin', ?, 1, ?, 1, 1, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
			WHERE NOT EXISTS (SELECT 1 FROM keyword_replies WHERE builtin = 1 AND (keyword = ? OR content = ?))
		`, item.keyword, matchType, content, boolInt(item.adminOnly), item.description, lookupKeyword, lookupKeyword); err != nil {
			return err
		}
		if _, err := db.Exec(`
			UPDATE keyword_replies
			SET keyword = ?, match_type = ?, content = ?, admin_only = ?, description = ?, updated_at = CURRENT_TIMESTAMP
			WHERE builtin = 1 AND (keyword = ? OR content = ?)
		`, item.keyword, matchType, content, boolInt(item.adminOnly), item.description, lookupKeyword, lookupKeyword); err != nil {
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
		"admin.username":                            "admin",
		"admin.platform_users":                      "[]",
		"web.auto_refresh":                          "true",
		"web.refresh_interval":                      "5",
		"plugin.dir":                                "./plugins",
		"plugin.auto_load":                          "true",
		"script_tasks.concurrent_limit":             "1",
		"script_tasks.retention_days":               "0",
		"script_tasks.run_timeout_seconds":          "0",
		"script_tasks.timeout_notify_admin_enabled": "false",
		"user.points_unit":                          "积分",
		"access_control":                            "{}",
		"payment.points_per_rmb":                    "100",
		"payment.config":                            defaultPaymentConfigJSON(),
		"imagehost.config":                          defaultImageHostConfigJSON(),
	}
	descriptions := map[string]string{
		"admin.username":                            "管理员用户名",
		"admin.platform_users":                      "平台管理员用户列表",
		"web.auto_refresh":                          "是否自动刷新",
		"web.refresh_interval":                      "刷新间隔秒数",
		"plugin.dir":                                "插件目录",
		"plugin.auto_load":                          "启动时自动加载插件",
		"script_tasks.concurrent_limit":             "脚本任务全局并发上限",
		"script_tasks.retention_days":               "脚本任务日志自动清理天数，0 表示不自动清理",
		"script_tasks.run_timeout_seconds":          "脚本任务运行超时时间，0 表示不限制",
		"script_tasks.timeout_notify_admin_enabled": "脚本任务超时自动停止后是否通知平台管理员",
		"user.points_unit":                          "用户积分单位",
		"access_control":                            "系统访问控制配置",
		"payment.points_per_rmb":                    "积分兑换比例",
		"payment.config":                            "支付配置",
		"imagehost.config":                          "图床配置",
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

func migrateUsersTable(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			union_id TEXT PRIMARY KEY,
			disabled INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_users_disabled_updated ON users(disabled, updated_at DESC);
		CREATE INDEX IF NOT EXISTS idx_user_accounts_platform_union ON user_accounts(platform, union_id, id);
		CREATE INDEX IF NOT EXISTS idx_point_transactions_union_created ON point_transactions(union_id, created_at DESC, id DESC);
	`); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT OR IGNORE INTO users (union_id, disabled, created_at, updated_at)
		SELECT union_id, 0, MIN(created_at), MAX(updated_at)
		FROM (
			SELECT union_id, created_at, updated_at FROM user_accounts
			UNION ALL
			SELECT union_id, created_at, updated_at FROM user_points
		)
		WHERE TRIM(union_id) <> '' GROUP BY union_id
	`); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		DROP TRIGGER IF EXISTS trg_user_accounts_prevent_union_platform_duplicate_insert;
		DROP TRIGGER IF EXISTS trg_user_accounts_prevent_union_platform_duplicate_update;

		CREATE TRIGGER trg_user_accounts_prevent_union_platform_duplicate_insert
		BEFORE INSERT ON user_accounts
		WHEN EXISTS (
			SELECT 1 FROM user_accounts
			WHERE union_id = NEW.union_id AND platform = NEW.platform
		)
		BEGIN
			SELECT RAISE(ABORT, 'union_id already has an account for this platform');
		END;

		CREATE TRIGGER trg_user_accounts_prevent_union_platform_duplicate_update
		BEFORE UPDATE OF union_id, platform ON user_accounts
		WHEN (NEW.union_id <> OLD.union_id OR NEW.platform <> OLD.platform)
		AND EXISTS (
			SELECT 1 FROM user_accounts
			WHERE union_id = NEW.union_id AND platform = NEW.platform AND id <> OLD.id
		)
		BEGIN
			SELECT RAISE(ABORT, 'union_id already has an account for this platform');
		END;
	`); err != nil {
		return err
	}
	return tx.Commit()
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

	if !columns["pinned"] {
		if _, err := db.Exec(`ALTER TABLE adapters ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
		columns["pinned"] = true
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
			pinned INTEGER NOT NULL DEFAULT 0,
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

	pinnedExpr := "0"
	if columns["pinned"] {
		pinnedExpr = "COALESCE(pinned, 0)"
	}

	copySQL := fmt.Sprintf(`
		INSERT INTO adapters_new (id, platform, remark, description, enabled, pinned, config, created_at, updated_at)
		SELECT id, platform, %s, %s, enabled, %s, config, created_at, updated_at FROM adapters
	`, remarkExpr, descriptionExpr, pinnedExpr)
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

func migrateScriptRunLogsTable(db *sql.DB) error {
	var tableName string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='script_run_logs'`).Scan(&tableName)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	columns, err := tableColumns(db, "script_run_logs")
	if err != nil {
		return err
	}
	if !columns["runtime_profile"] {
		if _, err := db.Exec(`ALTER TABLE script_run_logs ADD COLUMN runtime_profile TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if !columns["run_total"] {
		if _, err := db.Exec(`ALTER TABLE script_run_logs ADD COLUMN run_total INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
		if _, err := db.Exec(`UPDATE script_run_logs SET run_total = 1 WHERE run_total = 0`); err != nil {
			return err
		}
	}
	if !columns["failed_total"] {
		if _, err := db.Exec(`ALTER TABLE script_run_logs ADD COLUMN failed_total INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
		if _, err := db.Exec(`UPDATE script_run_logs SET failed_total = 1 WHERE status IN ('failed', 'error') AND failed_total = 0`); err != nil {
			return err
		}
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_script_run_logs_status_created_at ON script_run_logs(status, created_at, id)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_script_run_logs_queue ON script_run_logs(status, created_at, id) WHERE status = 'queued'`); err != nil {
		return err
	}
	return nil
}

func migrateWebChatMessagesTable(db *sql.DB) error {
	var tableName string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='web_chat_messages'`).Scan(&tableName)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err != sql.ErrNoRows {
		if _, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_web_chat_messages_user_plugin_message ON web_chat_messages(user_id, plugin_id, message_id)`); err != nil {
			return err
		}
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS web_chat_read_states (
			user_id TEXT NOT NULL,
			plugin_id TEXT NOT NULL DEFAULT '',
			last_read_message_id INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (user_id, plugin_id),
			FOREIGN KEY(user_id) REFERENCES web_chat_users(user_id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_web_chat_read_states_user ON web_chat_read_states(user_id, plugin_id);
	`)
	return err
}

func migrateScriptEnvVarsTable(db *sql.DB) error {
	var tableName string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='script_env_vars'`).Scan(&tableName)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	columns, err := tableColumns(db, "script_env_vars")
	if err != nil {
		return err
	}
	if !columns["pinned"] {
		if _, err := db.Exec(`ALTER TABLE script_env_vars ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
		columns["pinned"] = true
	}
	if scriptEnvVarsHasNameOnlyUnique(db) {
		if err := rebuildScriptEnvVarsTable(db, columns); err != nil {
			return err
		}
	}
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_script_env_vars_enabled ON script_env_vars(enabled, name);
		CREATE INDEX IF NOT EXISTS idx_script_env_vars_pinned ON script_env_vars(pinned DESC, name ASC, id ASC);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_script_env_vars_name_value_unique ON script_env_vars(name, value);
	`)
	return err
}

func rebuildScriptEnvVarsTable(db *sql.DB, columns map[string]bool) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`
		CREATE TABLE script_env_vars_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			value TEXT NOT NULL DEFAULT '',
			remark TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			pinned INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return err
	}
	pinnedExpr := "0"
	if columns["pinned"] {
		pinnedExpr = "COALESCE(pinned, 0)"
	}
	copySQL := fmt.Sprintf(`
		INSERT INTO script_env_vars_new (id, name, value, remark, enabled, pinned, created_at, updated_at)
		SELECT id, name, COALESCE(value, ''), COALESCE(remark, ''), COALESCE(enabled, 1), %s, created_at, updated_at
		FROM script_env_vars
	`, pinnedExpr)
	if _, err := tx.Exec(copySQL); err != nil {
		return err
	}
	if _, err := tx.Exec(`DROP TABLE script_env_vars`); err != nil {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE script_env_vars_new RENAME TO script_env_vars`); err != nil {
		return err
	}
	return tx.Commit()
}

func scriptEnvVarsHasNameOnlyUnique(db *sql.DB) bool {
	rows, err := db.Query(`PRAGMA index_list(script_env_vars)`)
	if err != nil {
		return false
	}
	uniqueIndexes := make([]string, 0)
	for rows.Next() {
		var seq int
		var name string
		var unique int
		var origin string
		var partial int
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err == nil && unique == 1 {
			uniqueIndexes = append(uniqueIndexes, name)
		}
	}
	rows.Close()
	for _, name := range uniqueIndexes {
		indexRows, err := db.Query(`PRAGMA index_info(` + name + `)`)
		if err != nil {
			continue
		}
		columnCount := 0
		nameOnly := false
		for indexRows.Next() {
			var seqno, cid int
			var columnName string
			if err := indexRows.Scan(&seqno, &cid, &columnName); err == nil {
				columnCount++
				nameOnly = columnName == "name"
			}
		}
		indexRows.Close()
		if columnCount == 1 && nameOnly {
			return true
		}
	}
	return false
}

func migrateMessageStatsTable(db *sql.DB) error {
	var tableName string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='message_stats'`).Scan(&tableName)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}

	columns, err := tableColumns(db, "message_stats")
	if err != nil {
		return err
	}
	if columns["message_type"] && messageStatsPrimaryKeyIncludesType(db) {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		CREATE TABLE message_stats_new (
			stat_date TEXT NOT NULL,
			stat_hour INTEGER NOT NULL,
			platform TEXT NOT NULL,
			adapter_id TEXT NOT NULL DEFAULT '',
			adapter_name TEXT NOT NULL DEFAULT '',
			message_type TEXT NOT NULL DEFAULT 'private',
			count INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (stat_date, stat_hour, platform, adapter_id, message_type)
		)
	`); err != nil {
		return err
	}

	messageTypeExpr := "'private'"
	if columns["message_type"] {
		messageTypeExpr = "COALESCE(NULLIF(message_type, ''), 'private')"
	}
	copySQL := fmt.Sprintf(`
		INSERT INTO message_stats_new (stat_date, stat_hour, platform, adapter_id, adapter_name, message_type, count, created_at, updated_at)
		SELECT stat_date, stat_hour, platform, COALESCE(adapter_id, ''), COALESCE(MAX(adapter_name), ''), %s, COALESCE(SUM(count), 0), MIN(created_at), MAX(updated_at)
		FROM message_stats
		GROUP BY stat_date, stat_hour, platform, COALESCE(adapter_id, ''), %s
	`, messageTypeExpr, messageTypeExpr)
	if _, err := tx.Exec(copySQL); err != nil {
		return err
	}
	if _, err := tx.Exec(`DROP TABLE message_stats`); err != nil {
		return err
	}
	if _, err := tx.Exec(`ALTER TABLE message_stats_new RENAME TO message_stats`); err != nil {
		return err
	}
	return tx.Commit()
}

func messageStatsPrimaryKeyIncludesType(db *sql.DB) bool {
	rows, err := db.Query(`PRAGMA table_info(message_stats)`)
	if err != nil {
		return false
	}
	defer rows.Close()

	primaryKeys := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return false
		}
		if pk > 0 {
			primaryKeys[name] = true
		}
	}
	return primaryKeys["stat_date"] && primaryKeys["stat_hour"] && primaryKeys["platform"] && primaryKeys["adapter_id"] && primaryKeys["message_type"]
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
		SELECT id, platform, remark, description, enabled, pinned, config, created_at, updated_at
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
	var pinned int
	err := d.db.QueryRow(`
		SELECT id, platform, remark, description, enabled, pinned, config, created_at, updated_at
		FROM adapters
		WHERE platform = ?
		ORDER BY id
		LIMIT 1
	`, platform).Scan(&adapter.ID, &adapter.Platform, &adapter.Remark, &adapter.Description, &enabled, &pinned, &adapter.Config, &adapter.CreatedAt, &adapter.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	adapter.Enabled = enabled == 1
	adapter.Pinned = pinned == 1
	return &adapter, nil
}

func (d *Database) GetAdapterByID(id int64) (*AdapterConfig, error) {
	var adapter AdapterConfig
	var enabled int
	var pinned int
	err := d.db.QueryRow(`
		SELECT id, platform, remark, description, enabled, pinned, config, created_at, updated_at
		FROM adapters
		WHERE id = ?
	`, id).Scan(&adapter.ID, &adapter.Platform, &adapter.Remark, &adapter.Description, &enabled, &pinned, &adapter.Config, &adapter.CreatedAt, &adapter.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	adapter.Enabled = enabled == 1
	adapter.Pinned = pinned == 1
	return &adapter, nil
}

func (d *Database) SaveAdapter(adapter *AdapterConfig) error {
	now := time.Now()
	adapter.UpdatedAt = now

	enabled := 0
	if adapter.Enabled {
		enabled = 1
	}
	pinned := 0
	if adapter.Pinned {
		pinned = 1
	}

	if adapter.ID == 0 {
		adapter.CreatedAt = now
		result, err := d.db.Exec(`
			INSERT INTO adapters (platform, remark, description, enabled, pinned, config, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, adapter.Platform, adapter.Remark, adapter.Description, enabled, pinned, adapter.Config, adapter.CreatedAt, adapter.UpdatedAt)
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
		SET platform = ?, remark = ?, description = ?, enabled = ?, pinned = ?, config = ?, updated_at = ?
		WHERE id = ?
	`, adapter.Platform, adapter.Remark, adapter.Description, enabled, pinned, adapter.Config, adapter.UpdatedAt, adapter.ID)
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
	var pinned int
	if err := scanner.Scan(&adapter.ID, &adapter.Platform, &adapter.Remark, &adapter.Description, &enabled, &pinned, &adapter.Config, &adapter.CreatedAt, &adapter.UpdatedAt); err != nil {
		return nil, err
	}
	adapter.Enabled = enabled == 1
	adapter.Pinned = pinned == 1
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
