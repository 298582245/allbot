CREATE TABLE IF NOT EXISTS open_api_call_stats (
    endpoint_id TEXT PRIMARY KEY,
    total INTEGER NOT NULL DEFAULT 0,
    success INTEGER NOT NULL DEFAULT 0,
    rejected INTEGER NOT NULL DEFAULT 0,
    failed INTEGER NOT NULL DEFAULT 0,
    last_status_code INTEGER NOT NULL DEFAULT 0,
    last_outcome TEXT NOT NULL DEFAULT '',
    last_called_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS open_api_call_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    endpoint_id TEXT NOT NULL,
    endpoint_name TEXT NOT NULL DEFAULT '',
    method TEXT NOT NULL DEFAULT '',
    request_path TEXT NOT NULL DEFAULT '',
    client_ip TEXT NOT NULL DEFAULT '',
    status_code INTEGER NOT NULL DEFAULT 0,
    outcome TEXT NOT NULL DEFAULT '',
    duration_ms INTEGER NOT NULL DEFAULT 0,
    started_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_open_api_call_logs_endpoint_started
    ON open_api_call_logs(endpoint_id, started_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_open_api_call_logs_endpoint_outcome_started
    ON open_api_call_logs(endpoint_id, outcome, started_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_open_api_call_logs_started
    ON open_api_call_logs(started_at, id);
