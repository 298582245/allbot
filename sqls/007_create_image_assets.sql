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
