CREATE TABLE IF NOT EXISTS plugin_trigger_stats (
    stat_date TEXT NOT NULL,
    stat_hour INTEGER NOT NULL,
    plugin_id TEXT NOT NULL,
    plugin_name TEXT NOT NULL DEFAULT '',
    trigger_pattern TEXT NOT NULL DEFAULT '',
    platform TEXT NOT NULL DEFAULT '',
    adapter_id TEXT NOT NULL DEFAULT '',
    adapter_name TEXT NOT NULL DEFAULT '',
    count INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (stat_date, stat_hour, plugin_id, platform, adapter_id)
);

CREATE INDEX IF NOT EXISTS idx_plugin_trigger_stats_date_plugin
    ON plugin_trigger_stats(stat_date, plugin_id);
CREATE INDEX IF NOT EXISTS idx_plugin_trigger_stats_plugin_date
    ON plugin_trigger_stats(plugin_id, stat_date);
CREATE INDEX IF NOT EXISTS idx_plugin_trigger_stats_date_platform
    ON plugin_trigger_stats(stat_date, platform);
