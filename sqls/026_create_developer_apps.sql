CREATE TABLE IF NOT EXISTS developer_apps (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    client_id TEXT NOT NULL UNIQUE,
    client_secret_hash TEXT NOT NULL,
    permissions TEXT NOT NULL DEFAULT '[]',
    credential_version INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at DATETIME,
    deleted_at DATETIME
);

CREATE TABLE IF NOT EXISTS developer_app_audit (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    app_id TEXT,
    action TEXT NOT NULL,
    actor TEXT NOT NULL DEFAULT 'admin',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_developer_apps_client_id ON developer_apps(client_id);
CREATE INDEX IF NOT EXISTS idx_developer_app_audit_app_id ON developer_app_audit(app_id, created_at);
