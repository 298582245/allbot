CREATE TABLE IF NOT EXISTS unregistered_prompt_stats (
    platform TEXT NOT NULL,
    user_id TEXT NOT NULL,
    prompt_count INTEGER NOT NULL DEFAULT 0,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY(platform, user_id)
);
