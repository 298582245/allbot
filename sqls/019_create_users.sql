CREATE TABLE IF NOT EXISTS users (
    union_id TEXT PRIMARY KEY,
    disabled INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_users_disabled_updated ON users(disabled, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_user_accounts_platform_union ON user_accounts(platform, union_id, id);
CREATE INDEX IF NOT EXISTS idx_point_transactions_union_created ON point_transactions(union_id, created_at DESC, id DESC);

INSERT OR IGNORE INTO users (union_id, disabled, created_at, updated_at)
SELECT union_id, 0, MIN(created_at), MAX(updated_at)
FROM (
    SELECT union_id, created_at, updated_at FROM user_accounts
    UNION ALL
    SELECT union_id, created_at, updated_at FROM user_points
)
WHERE TRIM(union_id) <> ''
GROUP BY union_id;

DROP TRIGGER IF EXISTS trg_user_accounts_prevent_union_platform_duplicate_insert;
DROP TRIGGER IF EXISTS trg_user_accounts_prevent_union_platform_duplicate_update;

CREATE TRIGGER trg_user_accounts_prevent_union_platform_duplicate_insert
BEFORE INSERT ON user_accounts
WHEN EXISTS (
    SELECT 1 FROM user_accounts
    WHERE union_id = NEW.union_id AND platform = NEW.platform
)
BEGIN
    SELECT RAISE(ABORT, 'union_id already has an account for this platform');
END;

CREATE TRIGGER trg_user_accounts_prevent_union_platform_duplicate_update
BEFORE UPDATE OF union_id, platform ON user_accounts
WHEN (NEW.union_id <> OLD.union_id OR NEW.platform <> OLD.platform)
AND EXISTS (
    SELECT 1 FROM user_accounts
    WHERE union_id = NEW.union_id AND platform = NEW.platform AND id <> OLD.id
)
BEGIN
    SELECT RAISE(ABORT, 'union_id already has an account for this platform');
END;
