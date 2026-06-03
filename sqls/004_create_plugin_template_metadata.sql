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
