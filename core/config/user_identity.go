package config

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	bindCodeTTL         = 10 * time.Minute
	bindMaxAttempts     = 5
	bindAttemptWindow   = 10 * time.Minute
	bindAttemptLockTime = 15 * time.Minute
)

var ErrUserBindLocked = errors.New("绑定尝试过于频繁，请稍后再试")

type bindAttempt struct {
	Count       int
	WindowStart time.Time
	LockedUntil time.Time
}

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
	err := d.db.QueryRow(`SELECT 1 FROM users WHERE union_id = ?`, unionID).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if err != sql.ErrNoRows {
		return false, err
	}
	result, err := d.db.Exec(`
		INSERT OR IGNORE INTO users (union_id, disabled, created_at, updated_at)
		SELECT ?, 0,
			COALESCE(MIN(created_at), CURRENT_TIMESTAMP),
			COALESCE(MAX(updated_at), CURRENT_TIMESTAMP)
		FROM (
			SELECT created_at, updated_at FROM user_points WHERE union_id = ?
			UNION ALL
			SELECT created_at, updated_at FROM user_accounts WHERE union_id = ?
		) legacy
		HAVING COUNT(*) > 0
	`, unionID, unionID, unionID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
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
	tx, err := d.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := ensureUserTx(tx, unionID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`INSERT INTO user_accounts (platform, user_id, union_id, created_at, updated_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, platform, userID, unionID); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO user_points (union_id, points, created_at, updated_at) VALUES (?, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, unionID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return d.GetUserAccount(platform, userID)
}

func (d *Database) CreateUserBindCode(platform, userID string) (*UserBindCode, error) {
	d.bindCodeMu.Lock()
	defer d.bindCodeMu.Unlock()

	account, err := d.EnsureUserAccount(platform, userID)
	if err != nil {
		return nil, err
	}

	var existing UserBindCode
	err = d.db.QueryRow(`SELECT code, platform, user_id, union_id, expires_at, created_at FROM user_bind_codes WHERE platform = ? AND user_id = ? AND expires_at > CURRENT_TIMESTAMP ORDER BY expires_at DESC LIMIT 1`, account.Platform, account.UserID).Scan(&existing.Code, &existing.Platform, &existing.UserID, &existing.UnionID, &existing.ExpiresAt, &existing.CreatedAt)
	if err == nil {
		return &existing, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	code, err := randomBindToken()
	if err != nil {
		return nil, err
	}
	createdAt := time.Now()
	expiresAt := createdAt.Add(bindCodeTTL)
	if _, err := d.db.Exec(`INSERT INTO user_bind_codes (code, platform, user_id, union_id, expires_at, created_at) VALUES (?, ?, ?, ?, ?, ?)`, code, account.Platform, account.UserID, account.UnionID, expiresAt, createdAt); err != nil {
		return nil, err
	}
	return &UserBindCode{Code: code, Platform: account.Platform, UserID: account.UserID, UnionID: account.UnionID, ExpiresAt: expiresAt, CreatedAt: createdAt}, nil
}

func (d *Database) BindUserByCode(platform, userID, code string) (*UserAccount, *UserAccount, error) {
	platform, userID = normalizeUserKey(platform, userID)
	code = strings.TrimSpace(code)
	if platform == "" || userID == "" || code == "" {
		return nil, nil, fmt.Errorf("平台、用户 ID 和绑定码不能为空")
	}
	attemptKey := platform + "\x00" + userID
	if err := d.checkBindAttempt(attemptKey); err != nil {
		return nil, nil, err
	}
	tx, err := d.db.Begin()
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	sourceAccount, err := d.validateBindCodeTx(tx, attemptKey, code)
	if err != nil {
		return nil, nil, err
	}
	source := *sourceAccount
	if source.Platform == platform {
		return nil, nil, fmt.Errorf("同平台账号不能互相绑定")
	}
	if err := ensureUserTx(tx, source.UnionID); err != nil {
		return nil, nil, err
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
		conflict, err := userPlatformConflictTx(tx, source.UnionID, target.UnionID)
		if err != nil {
			return nil, nil, err
		}
		if conflict {
			return nil, nil, ErrUserBindingConflict
		}
		disabled, err := mergeUserDisabledTx(tx, source.UnionID, target.UnionID)
		if err != nil {
			return nil, nil, err
		}
		var sourcePoints, targetPoints int64
		if err = tx.QueryRow(`SELECT COALESCE(points, 0) FROM user_points WHERE union_id = ?`, source.UnionID).Scan(&sourcePoints); err != nil && err != sql.ErrNoRows {
			return nil, nil, err
		}
		if err = tx.QueryRow(`SELECT COALESCE(points, 0) FROM user_points WHERE union_id = ?`, target.UnionID).Scan(&targetPoints); err != nil && err != sql.ErrNoRows {
			return nil, nil, err
		}
		mergedPoints, err := checkedAddInt64(sourcePoints, targetPoints)
		if err != nil {
			return nil, nil, fmt.Errorf("积分余额溢出")
		}
		if _, err = tx.Exec(`
			INSERT INTO user_points (union_id, points, created_at, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			ON CONFLICT(union_id) DO UPDATE SET points = excluded.points, updated_at = CURRENT_TIMESTAMP
		`, source.UnionID, mergedPoints); err != nil {
			return nil, nil, err
		}
		if _, err = tx.Exec(`DELETE FROM user_points WHERE union_id = ?`, target.UnionID); err != nil {
			return nil, nil, err
		}
		if _, err = tx.Exec(`UPDATE user_accounts SET union_id = ?, updated_at = CURRENT_TIMESTAMP WHERE union_id = ?`, source.UnionID, target.UnionID); err != nil {
			return nil, nil, err
		}
		if err := finalizeMergedUserTx(tx, source.UnionID, target.UnionID, disabled); err != nil {
			return nil, nil, err
		}
		target.UnionID = source.UnionID
	}
	if err = consumeUserBindCodeTx(tx, code); err != nil {
		return nil, nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, nil, err
	}
	d.clearBindAttempts(attemptKey)
	boundTarget, err := d.GetUserAccount(platform, userID)
	if err != nil {
		return nil, nil, err
	}
	boundSource, err := d.GetUserAccount(source.Platform, source.UserID)
	if err != nil {
		return nil, nil, err
	}
	return boundTarget, boundSource, nil
}

func (d *Database) DeleteExpiredUserBindCodes() error {
	_, err := d.db.Exec(`DELETE FROM user_bind_codes WHERE expires_at <= CURRENT_TIMESTAMP`)
	return err
}

func (d *Database) ensureUserPoints(unionID string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := ensureUserTx(tx, unionID); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO user_points (union_id, points, created_at, updated_at) VALUES (?, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, unionID); err != nil {
		return err
	}
	return tx.Commit()
}

func (d *Database) GetUserPoints(unionID string) (int64, error) {
	unionID = strings.TrimSpace(unionID)
	if unionID == "" {
		return 0, fmt.Errorf("用户 union_id 不能为空")
	}
	if err := d.ensureUserPoints(unionID); err != nil {
		return 0, err
	}
	var points int64
	if err := d.db.QueryRow(`SELECT points FROM user_points WHERE union_id = ?`, unionID).Scan(&points); err != nil {
		return 0, err
	}
	return points, nil
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
	remaining, err := checkedAddInt64(current, delta)
	if err != nil {
		return current, fmt.Errorf("积分余额溢出")
	}
	if remaining < 0 {
		return current, fmt.Errorf("积分不足，当前 %d，需要 %d", current, -delta)
	}
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

func randomBindToken() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func (d *Database) validateBindCodeTx(tx *sql.Tx, attemptKey, code string) (*UserAccount, error) {
	if err := d.checkBindAttempt(attemptKey); err != nil {
		return nil, err
	}
	var source UserAccount
	var expiresAt time.Time
	err := tx.QueryRow(`SELECT 0, platform, user_id, union_id, 0, created_at, expires_at FROM user_bind_codes WHERE code = ? AND expires_at > CURRENT_TIMESTAMP`, strings.TrimSpace(code)).Scan(&source.ID, &source.Platform, &source.UserID, &source.UnionID, &source.Points, &source.CreatedAt, &expiresAt)
	if err == sql.ErrNoRows {
		d.recordBindFailure(attemptKey)
		return nil, fmt.Errorf("绑定码不存在或已过期")
	}
	if err != nil {
		return nil, err
	}
	return &source, nil
}

func consumeUserBindCodeTx(tx *sql.Tx, code string) error {
	result, err := tx.Exec(`DELETE FROM user_bind_codes WHERE code = ? AND expires_at > CURRENT_TIMESTAMP`, strings.TrimSpace(code))
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return fmt.Errorf("绑定码不存在、已过期或已使用")
	}
	return nil
}

func (d *Database) checkBindAttempt(key string) error {
	now := time.Now()
	d.bindRateMu.Lock()
	defer d.bindRateMu.Unlock()
	if d.bindAttempts == nil {
		d.bindAttempts = make(map[string]bindAttempt)
	}
	attempt := d.bindAttempts[key]
	if attempt.LockedUntil.After(now) {
		return ErrUserBindLocked
	}
	if !attempt.WindowStart.IsZero() && now.Sub(attempt.WindowStart) >= bindAttemptWindow {
		delete(d.bindAttempts, key)
	}
	return nil
}

func (d *Database) recordBindFailure(key string) {
	now := time.Now()
	d.bindRateMu.Lock()
	defer d.bindRateMu.Unlock()
	if d.bindAttempts == nil {
		d.bindAttempts = make(map[string]bindAttempt)
	}
	attempt := d.bindAttempts[key]
	if attempt.WindowStart.IsZero() || now.Sub(attempt.WindowStart) >= bindAttemptWindow {
		attempt = bindAttempt{WindowStart: now}
	}
	attempt.Count++
	if attempt.Count >= bindMaxAttempts {
		attempt.LockedUntil = now.Add(bindAttemptLockTime)
	}
	d.bindAttempts[key] = attempt
}

func (d *Database) clearBindAttempts(key string) {
	d.bindRateMu.Lock()
	delete(d.bindAttempts, key)
	d.bindRateMu.Unlock()
}
