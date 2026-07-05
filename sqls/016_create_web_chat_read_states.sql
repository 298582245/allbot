CREATE TABLE IF NOT EXISTS web_chat_read_states (
    user_id TEXT NOT NULL,
    plugin_id TEXT NOT NULL DEFAULT '',
    last_read_message_id INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, plugin_id),
    FOREIGN KEY(user_id) REFERENCES web_chat_users(user_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_web_chat_read_states_user
ON web_chat_read_states(user_id, plugin_id);
