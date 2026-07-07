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
