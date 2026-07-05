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

CREATE INDEX IF NOT EXISTS idx_script_run_stats_date
    ON script_run_stats(stat_date);
CREATE INDEX IF NOT EXISTS idx_script_run_stats_plugin_date
    ON script_run_stats(plugin_id, stat_date);
