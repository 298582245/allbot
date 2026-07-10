package config

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrUserNotFound        = errors.New("用户不存在")
	ErrUserBindingConflict = errors.New("待合并用户存在相同平台账号，无法绑定")
)

func (d *Database) ListUsers(query UserQuery) ([]*UserSummary, int64, error) {
	where, args := buildUserWhere(query, "u")
	var total int64
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM users u`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit, offset := normalizeLimitOffset(query.Limit, query.Offset)
	rows, err := d.db.Query(`
		SELECT u.union_id, COALESCE(up.points, 0), u.disabled,
			(SELECT COUNT(*) FROM user_accounts ua WHERE ua.union_id = u.union_id),
			(SELECT COUNT(DISTINCT ua.platform) FROM user_accounts ua WHERE ua.union_id = u.union_id),
			COALESCE((SELECT GROUP_CONCAT(platform, char(31)) FROM (
				SELECT DISTINCT platform FROM user_accounts WHERE union_id = u.union_id ORDER BY platform
			)), ''),
			COALESCE((SELECT GROUP_CONCAT(platform, char(31)) FROM (
				SELECT platform FROM user_accounts WHERE union_id = u.union_id GROUP BY platform HAVING COUNT(*) > 1 ORDER BY platform
			)), ''),
			u.created_at, u.updated_at
		FROM users u
		LEFT JOIN user_points up ON up.union_id = u.union_id
	`+where+` ORDER BY u.updated_at DESC, u.union_id ASC LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*UserSummary, 0)
	for rows.Next() {
		item, err := scanUserSummary(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (d *Database) GetUserDetail(unionID string) (*UserDetail, error) {
	unionID = strings.TrimSpace(unionID)
	if unionID == "" {
		return nil, ErrUserNotFound
	}
	item, err := scanUserSummary(d.db.QueryRow(`
		SELECT u.union_id, COALESCE(up.points, 0), u.disabled,
			(SELECT COUNT(*) FROM user_accounts ua WHERE ua.union_id = u.union_id),
			(SELECT COUNT(DISTINCT ua.platform) FROM user_accounts ua WHERE ua.union_id = u.union_id),
			COALESCE((SELECT GROUP_CONCAT(platform, char(31)) FROM (
				SELECT DISTINCT platform FROM user_accounts WHERE union_id = u.union_id ORDER BY platform
			)), ''),
			COALESCE((SELECT GROUP_CONCAT(platform, char(31)) FROM (
				SELECT platform FROM user_accounts WHERE union_id = u.union_id GROUP BY platform HAVING COUNT(*) > 1 ORDER BY platform
			)), ''),
			u.created_at, u.updated_at
		FROM users u LEFT JOIN user_points up ON up.union_id = u.union_id
		WHERE u.union_id = ?
	`, unionID))
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	accounts, err := d.listAllUserAccounts(unionID)
	if err != nil {
		return nil, err
	}
	return &UserDetail{UserSummary: *item, Accounts: accounts}, nil
}

func (d *Database) listAllUserAccounts(unionID string) ([]*UserAccountSummary, error) {
	rows, err := d.db.Query(`
		SELECT ua.id, ua.platform, ua.user_id, ua.union_id, COALESCE(up.points, 0), u.disabled,
			CASE WHEN (SELECT COUNT(*) FROM user_accounts duplicate
				WHERE duplicate.union_id = ua.union_id AND duplicate.platform = ua.platform) > 1 THEN 1 ELSE 0 END,
			ua.created_at, ua.updated_at
		FROM user_accounts ua
		JOIN users u ON u.union_id = ua.union_id
		LEFT JOIN user_points up ON up.union_id = ua.union_id
		WHERE ua.union_id = ?
		ORDER BY ua.platform ASC, ua.updated_at DESC, ua.id DESC
	`, unionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]*UserAccountSummary, 0)
	for rows.Next() {
		item, err := scanUserAccountSummary(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (d *Database) ListUserAccounts(query UserQuery, unionID ...string) ([]*UserAccountSummary, int64, error) {
	clauses := make([]string, 0)
	args := make([]interface{}, 0)
	if len(unionID) > 0 && strings.TrimSpace(unionID[0]) != "" {
		clauses = append(clauses, "ua.union_id = ?")
		args = append(args, strings.TrimSpace(unionID[0]))
	}
	if platform := strings.TrimSpace(query.Platform); platform != "" {
		clauses = append(clauses, "ua.platform = ?")
		args = append(args, platform)
	}
	if query.Disabled != nil {
		clauses = append(clauses, "u.disabled = ?")
		args = append(args, boolToInt(*query.Disabled))
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + escapeLike(keyword) + "%"
		clauses = append(clauses, `(ua.user_id LIKE ? ESCAPE '\' OR ua.union_id LIKE ? ESCAPE '\')`)
		args = append(args, like, like)
	}
	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}
	from := ` FROM user_accounts ua JOIN users u ON u.union_id = ua.union_id LEFT JOIN user_points up ON up.union_id = ua.union_id`
	var total int64
	if err := d.db.QueryRow(`SELECT COUNT(*)`+from+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit, offset := normalizeLimitOffset(query.Limit, query.Offset)
	rows, err := d.db.Query(`
		SELECT ua.id, ua.platform, ua.user_id, ua.union_id, COALESCE(up.points, 0), u.disabled,
			CASE WHEN (SELECT COUNT(*) FROM user_accounts duplicate
				WHERE duplicate.union_id = ua.union_id AND duplicate.platform = ua.platform) > 1 THEN 1 ELSE 0 END,
			ua.created_at, ua.updated_at
	`+from+where+` ORDER BY ua.platform ASC, ua.updated_at DESC, ua.id DESC LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*UserAccountSummary, 0)
	for rows.Next() {
		item, err := scanUserAccountSummary(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (d *Database) SetUserDisabled(unionID string, disabled bool) (*UserSummary, error) {
	unionID = strings.TrimSpace(unionID)
	tx, err := d.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE users SET disabled = ?, updated_at = CURRENT_TIMESTAMP WHERE union_id = ?`, boolToInt(disabled), unionID)
	if err != nil {
		return nil, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return nil, ErrUserNotFound
	}
	if disabled {
		if err := clearWebChatSessionsForUnionTx(tx, unionID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	detail, err := d.GetUserDetail(unionID)
	if err != nil {
		return nil, err
	}
	return &detail.UserSummary, nil
}

func (d *Database) IsUserDisabled(platform, userID string) (bool, error) {
	platform, userID = normalizeUserKey(platform, userID)
	if platform == "" || userID == "" {
		return false, nil
	}
	var disabled int
	err := d.db.QueryRow(`
		SELECT u.disabled FROM user_accounts ua JOIN users u ON u.union_id = ua.union_id
		WHERE ua.platform = ? AND ua.user_id = ?
	`, platform, userID).Scan(&disabled)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return disabled == 1, err
}

func (d *Database) IsUnionDisabled(unionID string) (bool, error) {
	var disabled int
	err := d.db.QueryRow(`SELECT disabled FROM users WHERE union_id = ?`, strings.TrimSpace(unionID)).Scan(&disabled)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return disabled == 1, err
}

func (d *Database) AdjustUserPoints(unionID string, delta int64, description string) (*PointAdjustment, error) {
	unionID = strings.TrimSpace(unionID)
	description = strings.TrimSpace(description)
	if unionID == "" {
		return nil, ErrUserNotFound
	}
	if delta == 0 {
		return nil, fmt.Errorf("积分调整值不能为 0")
	}
	d.pointsMu.Lock()
	defer d.pointsMu.Unlock()
	tx, err := d.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var current int64
	if err := tx.QueryRow(`SELECT COALESCE(up.points, 0) FROM users u LEFT JOIN user_points up ON up.union_id = u.union_id WHERE u.union_id = ?`, unionID).Scan(&current); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	balance := current + delta
	if balance < 0 {
		return nil, fmt.Errorf("积分不足，当前 %d，需要 %d", current, -delta)
	}
	if _, err := tx.Exec(`
		INSERT INTO user_points (union_id, points, created_at, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(union_id) DO UPDATE SET points = excluded.points, updated_at = CURRENT_TIMESTAMP
	`, unionID, balance); err != nil {
		return nil, err
	}
	suffix, err := randomHex(8)
	if err != nil {
		return nil, err
	}
	transaction, err := d.RecordPointTransaction(tx, &PointTransaction{
		UnionID: unionID, Delta: delta, BalanceAfter: balance, Source: "admin_adjustment",
		SourceID: "admin_" + suffix, Description: description,
	})
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE union_id = ?`, unionID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &PointAdjustment{UnionID: unionID, Delta: delta, BalanceAfter: balance, Transaction: transaction}, nil
}

func buildUserWhere(query UserQuery, alias string) (string, []interface{}) {
	clauses := make([]string, 0)
	args := make([]interface{}, 0)
	if query.Disabled != nil {
		clauses = append(clauses, alias+".disabled = ?")
		args = append(args, boolToInt(*query.Disabled))
	}
	if platform := strings.TrimSpace(query.Platform); platform != "" {
		clauses = append(clauses, "EXISTS (SELECT 1 FROM user_accounts filter_account WHERE filter_account.union_id = "+alias+".union_id AND filter_account.platform = ?)")
		args = append(args, platform)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + escapeLike(keyword) + "%"
		clauses = append(clauses, `(`+alias+`.union_id LIKE ? ESCAPE '\' OR EXISTS (
			SELECT 1 FROM user_accounts search_account WHERE search_account.union_id = `+alias+`.union_id
			AND search_account.user_id LIKE ? ESCAPE '\'))`)
		args = append(args, like, like)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func scanUserSummary(row interface{ Scan(...interface{}) error }) (*UserSummary, error) {
	var item UserSummary
	var disabled int
	var platforms, duplicates string
	if err := row.Scan(&item.UnionID, &item.Points, &disabled, &item.AccountCount, &item.PlatformCount, &platforms, &duplicates, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return nil, err
	}
	item.Disabled = disabled == 1
	item.Platforms = splitGroupConcat(platforms)
	item.DuplicatePlatforms = splitGroupConcat(duplicates)
	return &item, nil
}

func scanUserAccountSummary(row interface{ Scan(...interface{}) error }) (*UserAccountSummary, error) {
	var item UserAccountSummary
	var disabled, duplicate int
	if err := row.Scan(&item.ID, &item.Platform, &item.UserID, &item.UnionID, &item.Points, &disabled, &duplicate, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return nil, err
	}
	item.Disabled = disabled == 1
	item.DuplicatePlatform = duplicate == 1
	return &item, nil
}

func splitGroupConcat(value string) []string {
	if value == "" {
		return []string{}
	}
	return strings.Split(value, string(rune(31)))
}

func escapeLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func ensureUserTx(tx *sql.Tx, unionID string) error {
	_, err := tx.Exec(`INSERT OR IGNORE INTO users (union_id, disabled, created_at, updated_at) VALUES (?, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, strings.TrimSpace(unionID))
	return err
}

func mergeUserDisabledTx(tx *sql.Tx, keepUnionID, mergedUnionID string) (bool, error) {
	if err := ensureUserTx(tx, keepUnionID); err != nil {
		return false, err
	}
	if err := ensureUserTx(tx, mergedUnionID); err != nil {
		return false, err
	}
	var disabled int
	if err := tx.QueryRow(`SELECT CASE WHEN EXISTS (SELECT 1 FROM users WHERE union_id IN (?, ?) AND disabled = 1) THEN 1 ELSE 0 END`, keepUnionID, mergedUnionID).Scan(&disabled); err != nil {
		return false, err
	}
	if _, err := tx.Exec(`UPDATE users SET disabled = ?, updated_at = CURRENT_TIMESTAMP WHERE union_id = ?`, disabled, keepUnionID); err != nil {
		return false, err
	}
	return disabled == 1, nil
}

func userPlatformConflictTx(tx *sql.Tx, firstUnionID, secondUnionID string) (bool, error) {
	if firstUnionID == secondUnionID {
		return false, nil
	}
	var exists int
	err := tx.QueryRow(`SELECT 1 FROM user_accounts first JOIN user_accounts second ON second.platform = first.platform WHERE first.union_id = ? AND second.union_id = ? LIMIT 1`, firstUnionID, secondUnionID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func unionHasPlatformTx(tx *sql.Tx, unionID, platform string) (bool, error) {
	var exists int
	err := tx.QueryRow(`SELECT 1 FROM user_accounts WHERE union_id = ? AND platform = ? LIMIT 1`, unionID, platform).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func clearWebChatSessionsForUnionTx(tx *sql.Tx, unionID string) error {
	_, err := tx.Exec(`DELETE FROM web_chat_sessions WHERE user_id IN (SELECT user_id FROM user_accounts WHERE platform = ? AND union_id = ?)`, WebChatPlatform, unionID)
	return err
}

func finalizeMergedUserTx(tx *sql.Tx, keepUnionID, mergedUnionID string, disabled bool) error {
	for _, query := range []string{
		`UPDATE plugin_accounts SET union_id = ? WHERE union_id = ?`,
		`UPDATE script_run_logs SET union_id = ? WHERE union_id = ?`,
		`UPDATE payment_orders SET union_id = ? WHERE union_id = ?`,
		`UPDATE point_transactions SET union_id = ? WHERE union_id = ?`,
		`UPDATE user_bind_codes SET union_id = ? WHERE union_id = ?`,
		`UPDATE web_chat_platform_codes SET union_id = ? WHERE union_id = ?`,
	} {
		if _, err := tx.Exec(query, keepUnionID, mergedUnionID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`
		INSERT INTO plugin_authorizations (plugin_id, union_id, status, plan, source, metadata, expires_at, created_at, updated_at)
		SELECT plugin_id, ?, status, plan, source, metadata, expires_at, created_at, updated_at
		FROM plugin_authorizations WHERE union_id = ?
		ON CONFLICT(plugin_id, union_id) DO NOTHING
	`, keepUnionID, mergedUnionID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM plugin_authorizations WHERE union_id = ?`, mergedUnionID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM users WHERE union_id = ?`, mergedUnionID); err != nil {
		return err
	}
	if disabled {
		return clearWebChatSessionsForUnionTx(tx, keepUnionID)
	}
	return nil
}
