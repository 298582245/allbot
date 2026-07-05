-- 为消息统计增加消息类型维度，用于区分私聊与群聊趋势。
CREATE TABLE IF NOT EXISTS message_stats_new (
    stat_date TEXT NOT NULL,
    stat_hour INTEGER NOT NULL,
    platform TEXT NOT NULL,
    adapter_id TEXT NOT NULL DEFAULT '',
    adapter_name TEXT NOT NULL DEFAULT '',
    message_type TEXT NOT NULL DEFAULT 'private',
    count INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (stat_date, stat_hour, platform, adapter_id, message_type)
);

INSERT INTO message_stats_new (stat_date, stat_hour, platform, adapter_id, adapter_name, message_type, count, created_at, updated_at)
SELECT
    stat_date,
    stat_hour,
    platform,
    COALESCE(adapter_id, ''),
    COALESCE(MAX(adapter_name), ''),
    'private',
    COALESCE(SUM(count), 0),
    MIN(created_at),
    MAX(updated_at)
FROM message_stats
GROUP BY stat_date, stat_hour, platform, COALESCE(adapter_id, '');

DROP TABLE message_stats;
ALTER TABLE message_stats_new RENAME TO message_stats;

CREATE INDEX IF NOT EXISTS idx_message_stats_date_platform ON message_stats(stat_date, platform);
CREATE INDEX IF NOT EXISTS idx_message_stats_date_adapter ON message_stats(stat_date, adapter_id);
CREATE INDEX IF NOT EXISTS idx_message_stats_date_type ON message_stats(stat_date, message_type);
