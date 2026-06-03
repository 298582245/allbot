package config

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"math/big"
	"strings"
	"time"
)

const bindCodeTTL = 10 * time.Minute

func (d *Database) GetUserAccount(platform, userID string) (*UserAccount, error) {
	platform, userID = normalizeUserKey(platform, userID)
	if platform == "" || userID == "" {
		return nil, sql.ErrNoRows
	}
	return scanUserAccount(d.db.QueryRow(`SELECT ua.id, ua.platform, ua.user_id, ua.union_id, COALESCE(up.points, 0), ua.created_at, ua.updated_at FROM user_accounts ua LEFT JOIN user_points up ON up.union_id = ua.union_id WHERE ua.platform = ? AND ua.user_id = ?`, platform, userID))
}

func (d *Database) ListUserAccountsByUnionID(unionID string) ([]*UserAccount, error) {
	unionID = strings.TrimSpace(unionID)
	if unionID == "" {
		return []*UserAccount{}, nil
	}
	rows, err := d.db.Query(`
		SELECT ua.id, ua.platform, ua.user_id, ua.union_id, COALESCE(up.points, 0), ua.created_at, ua.updated_at
		FROM user_accounts ua
		LEFT JOIN user_points up ON up.union_id = ua.union_id
		WHERE ua.union_id = ?
		ORDER BY ua.updated_at DESC, ua.id DESC
	`, unionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]*UserAccount, 0)
	for rows.Next() {
		account, err := scanUserAccount(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, account)
	}
	return result, rows.Err()
}

func (d *Database) UserUnionExists(unionID string) (bool, error) {
	unionID = strings.TrimSpace(unionID)
	if unionID == "" {
		return false, nil
	}
	var exists int
	err := d.db.QueryRow(`
		SELECT 1
		FROM user_points
		WHERE union_id = ?
		UNION
		SELECT 1
		FROM user_accounts
		WHERE union_id = ?
		LIMIT 1
	`, unionID, unionID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func (d *Database) EnsureUserAccount(platform, userID string) (*UserAccount, error) {
	platform, userID = normalizeUserKey(platform, userID)
	if platform == "" || userID == "" {
		return nil, fmt.Errorf("平台和用户 ID 不能为空")
	}
	if account, err := d.GetUserAccount(platform, userID); err == nil {
		return account, nil
	} else if err != sql.ErrNoRows {
		return nil, err
	}
	unionID := newUnionID(platform, userID)
	if _, err := d.db.Exec(`INSERT INTO user_accounts (platform, user_id, union_id, created_at, updated_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, platform, userID, unionID); err != nil {
		return nil, err
	}
	if err := d.ensureUserPoints(unionID); err != nil {
		return nil, err
	}
	return d.GetUserAccount(platform, userID)
}

func (d *Database) CreateUserBindCode(platform, userID string) (*UserBindCode, error) {
	account, err := d.EnsureUserAccount(platform, userID)
	if err != nil {
		return nil, err
	}
	if _, err := d.db.Exec(`DELETE FROM user_bind_codes WHERE platform = ? AND user_id = ? OR expires_at <= CURRENT_TIMESTAMP`, account.Platform, account.UserID); err != nil {
		return nil, err
	}
	code, err := randomDigits(6)
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(bindCodeTTL)
	if _, err := d.db.Exec(`INSERT INTO user_bind_codes (code, platform, user_id, union_id, expires_at, created_at) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`, code, account.Platform, account.UserID, account.UnionID, expiresAt); err != nil {
		return nil, err
	}
	return &UserBindCode{Code: code, Platform: account.Platform, UserID: account.UserID, UnionID: account.UnionID, ExpiresAt: expiresAt, CreatedAt: time.Now()}, nil
}

func (d *Database) BindUserByCode(platform, userID, code string) (*UserAccount, *UserAccount, error) {
	platform, userID = normalizeUserKey(platform, userID)
	code = strings.TrimSpace(code)
	if platform == "" || userID == "" || code == "" {
		return nil, nil, fmt.Errorf("平台、用户 ID 和绑定码不能为空")
	}
	tx, err := d.db.Begin()
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	var source UserAccount
	var expiresAt time.Time
	err = tx.QueryRow(`SELECT 0, platform, user_id, union_id, 0, created_at, expires_at FROM user_bind_codes WHERE code = ? AND expires_at > CURRENT_TIMESTAMP`, code).Scan(&source.ID, &source.Platform, &source.UserID, &source.UnionID, &source.Points, &source.CreatedAt, &expiresAt)
	if err == sql.ErrNoRows {
		return nil, nil, fmt.Errorf("绑定码不存在或已过期")
	}
	if err != nil {
		return nil, nil, err
	}
	if source.Platform == platform {
		return nil, nil, fmt.Errorf("同平台账号不能互相绑定")
	}

	var target *UserAccount
	target, err = scanUserAccount(tx.QueryRow(`SELECT ua.id, ua.platform, ua.user_id, ua.union_id, COALESCE(up.points, 0), ua.created_at, ua.updated_at FROM user_accounts ua LEFT JOIN user_points up ON up.union_id = ua.union_id WHERE ua.platform = ? AND ua.user_id = ?`, platform, userID))
	if err == sql.ErrNoRows {
		if _, err = tx.Exec(`INSERT INTO user_accounts (platform, user_id, union_id, created_at, updated_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, platform, userID, source.UnionID); err != nil {
			return nil, nil, err
		}
		if _, err = tx.Exec(`INSERT OR IGNORE INTO user_points (union_id, points, created_at, updated_at) VALUES (?, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, source.UnionID); err != nil {
			return nil, nil, err
		}
		target, err = scanUserAccount(tx.QueryRow(`SELECT ua.id, ua.platform, ua.user_id, ua.union_id, COALESCE(up.points, 0), ua.created_at, ua.updated_at FROM user_accounts ua LEFT JOIN user_points up ON up.union_id = ua.union_id WHERE ua.platform = ? AND ua.user_id = ?`, platform, userID))
	}
	if err != nil {
		return nil, nil, err
	}
	if target.UnionID != source.UnionID {
		if _, err = tx.Exec(`
			INSERT INTO user_points (union_id, points, created_at, updated_at)
			VALUES (?, COALESCE((SELECT points FROM user_points WHERE union_id = ?), 0) + COALESCE((SELECT points FROM user_points WHERE union_id = ?), 0), CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			ON CONFLICT(union_id) DO UPDATE SET points = user_points.points + COALESCE((SELECT points FROM user_points WHERE union_id = ?), 0), updated_at = CURRENT_TIMESTAMP
		`, source.UnionID, source.UnionID, target.UnionID, target.UnionID); err != nil {
			return nil, nil, err
		}
		if _, err = tx.Exec(`DELETE FROM user_points WHERE union_id = ?`, target.UnionID); err != nil {
			return nil, nil, err
		}
		if _, err = tx.Exec(`UPDATE user_accounts SET union_id = ?, updated_at = CURRENT_TIMESTAMP WHERE union_id = ?`, source.UnionID, target.UnionID); err != nil {
			return nil, nil, err
		}
		target.UnionID = source.UnionID
	}
	if _, err = tx.Exec(`DELETE FROM user_bind_codes WHERE code = ?`, code); err != nil {
		return nil, nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, nil, err
	}
	boundTarget, err := d.GetUserAccount(platform, userID)
	if err != nil {
		return nil, nil, err
	}
	sourceAccount, err := d.GetUserAccount(source.Platform, source.UserID)
	if err != nil {
		return nil, nil, err
	}
	return boundTarget, sourceAccount, nil
}

func (d *Database) DeleteExpiredUserBindCodes() error {
	_, err := d.db.Exec(`DELETE FROM user_bind_codes WHERE expires_at <= CURRENT_TIMESTAMP`)
	return err
}

func (d *Database) ensureUserPoints(unionID string) error {
	_, err := d.db.Exec(`INSERT OR IGNORE INTO user_points (union_id, points, created_at, updated_at) VALUES (?, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, unionID)
	return err
}

func (d *Database) ConsumeUserPoints(unionID string, amount int64) (int64, error) {
	d.pointsMu.Lock()
	defer d.pointsMu.Unlock()
	return d.changeUserPointsLocked(unionID, -amount)
}

func (d *Database) AddUserPoints(unionID string, amount int64) (int64, error) {
	d.pointsMu.Lock()
	defer d.pointsMu.Unlock()
	return d.changeUserPointsLocked(unionID, amount)
}

func (d *Database) changeUserPointsLocked(unionID string, delta int64) (int64, error) {
	unionID = strings.TrimSpace(unionID)
	if unionID == "" {
		return 0, fmt.Errorf("用户 union_id 不能为空")
	}
	if err := d.ensureUserPoints(unionID); err != nil {
		return 0, err
	}
	tx, err := d.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var current int64
	if err = tx.QueryRow(`SELECT points FROM user_points WHERE union_id = ?`, unionID).Scan(&current); err != nil {
		return 0, err
	}
	if delta < 0 && current < -delta {
		return current, fmt.Errorf("积分不足，当前 %d，需要 %d", current, -delta)
	}
	remaining := current + delta
	if _, err = tx.Exec(`UPDATE user_points SET points = ?, updated_at = CURRENT_TIMESTAMP WHERE union_id = ?`, remaining, unionID); err != nil {
		return current, err
	}
	if err = tx.Commit(); err != nil {
		return current, err
	}
	return remaining, nil
}

func scanUserAccount(row interface {
	Scan(dest ...interface{}) error
}) (*UserAccount, error) {
	var account UserAccount
	if err := row.Scan(&account.ID, &account.Platform, &account.UserID, &account.UnionID, &account.Points, &account.CreatedAt, &account.UpdatedAt); err != nil {
		return nil, err
	}
	return &account, nil
}

func normalizeUserKey(platform, userID string) (string, string) {
	return strings.TrimSpace(platform), strings.TrimSpace(userID)
}

func newUnionID(platform, userID string) string {
	return fmt.Sprintf("U_%s_%s", sanitizeIdentityPart(platform), sanitizeIdentityPart(userID))
}

func sanitizeIdentityPart(value string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	for _, item := range value {
		if item >= 'a' && item <= 'z' || item >= 'A' && item <= 'Z' || item >= '0' && item <= '9' {
			builder.WriteRune(item)
		} else {
			builder.WriteByte('_')
		}
	}
	if builder.Len() == 0 {
		return "unknown"
	}
	return builder.String()
}

func randomDigits(length int) (string, error) {
	var builder strings.Builder
	for builder.Len() < length {
		value, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		builder.WriteByte(byte('0' + value.Int64()))
	}
	return builder.String(), nil
}
