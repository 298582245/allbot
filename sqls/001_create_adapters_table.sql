-- 创建适配器配置表
CREATE TABLE IF NOT EXISTS adapters (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    platform TEXT NOT NULL UNIQUE,
    enabled INTEGER NOT NULL DEFAULT 0,
    config TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_adapters_platform ON adapters(platform);
CREATE INDEX IF NOT EXISTS idx_adapters_enabled ON adapters(enabled);
