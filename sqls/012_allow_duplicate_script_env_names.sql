CREATE TABLE script_env_vars_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    value TEXT NOT NULL DEFAULT '',
    remark TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    pinned INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO script_env_vars_new (id, name, value, remark, enabled, pinned, created_at, updated_at)
SELECT id, name, COALESCE(value, ''), COALESCE(remark, ''), COALESCE(enabled, 1), COALESCE(pinned, 0), created_at, updated_at
FROM script_env_vars;

DROP TABLE script_env_vars;
ALTER TABLE script_env_vars_new RENAME TO script_env_vars;

CREATE INDEX IF NOT EXISTS idx_script_env_vars_enabled ON script_env_vars(enabled, name);
CREATE INDEX IF NOT EXISTS idx_script_env_vars_pinned ON script_env_vars(pinned DESC, name ASC, id ASC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_script_env_vars_name_value_unique ON script_env_vars(name, value);
