package config

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/mail"
	"regexp"
	"strings"
	"time"
)

const (
	WebChatPlatform                  = "web"
	WebChatEmailPurposeRegister      = "register"
	WebChatEmailPurposeResetPassword = "reset_password"
	WebChatEmailPurposeLogin         = "login"
	webChatCodeTTL                   = 10 * time.Minute
	webChatSessionTTL                = 7 * 24 * time.Hour
	webChatMaxCodeAttempts           = 5
)

var webChatUsernamePattern = regexp.MustCompile(`^[A-Za-z0-9_]{3,32}$`)

type WebChatUser struct {
	UserID        string     `json:"user_id"`
	Username      string     `json:"username"`
	Email         string     `json:"email"`
	DisplayName   string     `json:"display_name"`
	EmailVerified bool       `json:"email_verified"`
	Disabled      bool       `json:"disabled"`
	UnionID       string     `json:"union_id,omitempty"`
	LastLoginAt   *time.Time `json:"last_login_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type WebChatRegisterInput struct {
	Email       string
	Code        string
	Username    string
	Password    string
	DisplayName string
	BindCode    string
}

type WebChatSession struct {
	Token     string       `json:"-"`
	TokenHash string       `json:"-"`
	CSRFToken string       `json:"csrf_token"`
	User      *WebChatUser `json:"user"`
	ExpiresAt time.Time    `json:"expires_at"`
}

type WebChatMessage struct {
	MessageID   int64     `json:"message_id"`
	UserID      string    `json:"user_id"`
	Direction   string    `json:"direction"`
	MessageType string    `json:"message_type"`
	Content     string    `json:"content"`
	ImageURL    string    `json:"image_url,omitempty"`
	RichJSON    string    `json:"rich_json,omitempty"`
	Target      string    `json:"target,omitempty"`
	PluginID    string    `json:"plugin_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type WebChatMessageCount struct {
	PluginID          string `json:"plugin_id"`
	Count             int64  `json:"count"`
	LastMessageID     int64  `json:"last_message_id"`
	LastReadMessageID int64  `json:"last_read_message_id"`
	UnreadCount       int64  `json:"unread_count"`
}

func (d *Database) CreateWebChatEmailCode(email, code, purpose, sentIP string) error {
	email = normalizeWebChatEmail(email)
	purpose, err := normalizeWebChatPurpose(purpose)
	if err != nil {
		return err
	}
	if email == "" {
		return fmt.Errorf("邮箱格式无效")
	}
	if code = strings.TrimSpace(code); code == "" {
		return fmt.Errorf("验证码不能为空")
	}
	if err := d.ensureWebChatEmailRate(email, purpose, sentIP); err != nil {
		return err
	}
	hash, err := HashAdminPassword(code)
	if err != nil {
		return err
	}
	_, err = d.db.Exec(`
		INSERT INTO web_chat_email_codes (email, code_hash, purpose, expires_at, sent_ip, created_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, email, hash, purpose, time.Now().Add(webChatCodeTTL), strings.TrimSpace(sentIP))
	return err
}

func (d *Database) RegisterWebChatUser(input WebChatRegisterInput) (*WebChatUser, error) {
	email := normalizeWebChatEmail(input.Email)
	username := strings.TrimSpace(input.Username)
	displayName := strings.TrimSpace(input.DisplayName)
	password := strings.TrimSpace(input.Password)
	code := strings.TrimSpace(input.Code)
	bindCode := strings.TrimSpace(input.BindCode)
	if email == "" {
		return nil, fmt.Errorf("邮箱格式无效")
	}
	if !webChatUsernamePattern.MatchString(username) {
		return nil, fmt.Errorf("账号只能包含字母、数字和下划线，长度 3 到 32 位")
	}
	if len(password) < 8 || len(password) > 128 {
		return nil, fmt.Errorf("密码长度必须为 8 到 128 位")
	}
	if code == "" {
		return nil, fmt.Errorf("邮箱验证码不能为空")
	}
	passwordHash, err := HashAdminPassword(password)
	if err != nil {
		return nil, err
	}
	userID, err := newWebChatUserID()
	if err != nil {
		return nil, err
	}
	if displayName == "" {
		displayName = username
	}

	tx, err := d.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := consumeWebChatEmailCodeTx(tx, email, code, WebChatEmailPurposeRegister); err != nil {
		return nil, err
	}
	unionID := newUnionID(WebChatPlatform, userID)
	if bindCode != "" {
		source, err := userBindCodeByCodeTx(tx, bindCode)
		if err != nil {
			return nil, err
		}
		if source.Platform == WebChatPlatform {
			return nil, fmt.Errorf("不能绑定另一个 web 账号")
		}
		unionID = source.UnionID
	}
	if _, err = tx.Exec(`
		INSERT INTO web_chat_users (user_id, username, email, password_hash, display_name, email_verified, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, userID, username, email, passwordHash, displayName); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`INSERT INTO user_accounts (platform, user_id, union_id, created_at, updated_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, WebChatPlatform, userID, unionID); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`INSERT OR IGNORE INTO user_points (union_id, points, created_at, updated_at) VALUES (?, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, unionID); err != nil {
		return nil, err
	}
	if bindCode != "" {
		if _, err = tx.Exec(`DELETE FROM user_bind_codes WHERE code = ?`, bindCode); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return d.GetWebChatUser(userID)
}

func (d *Database) WebChatUserExistsByEmail(email string) (bool, error) {
	email = normalizeWebChatEmail(email)
	if email == "" {
		return false, fmt.Errorf("邮箱格式无效")
	}
	var count int
	if err := d.db.QueryRow(`SELECT COUNT(1) FROM web_chat_users WHERE email = ? AND disabled = 0`, email).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (d *Database) VerifyWebChatEmailLogin(email, code string) (*WebChatUser, error) {
	email = normalizeWebChatEmail(email)
	code = strings.TrimSpace(code)
	if email == "" {
		return nil, fmt.Errorf("邮箱格式无效")
	}
	if code == "" {
		return nil, fmt.Errorf("邮箱验证码不能为空")
	}
	tx, err := d.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var userID string
	if err = tx.QueryRow(`SELECT user_id FROM web_chat_users WHERE email = ? AND disabled = 0`, email).Scan(&userID); err == sql.ErrNoRows {
		return nil, fmt.Errorf("邮箱验证码错误")
	} else if err != nil {
		return nil, err
	}
	if err = consumeWebChatEmailCodeTx(tx, email, code, WebChatEmailPurposeLogin); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`UPDATE web_chat_users SET last_login_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE user_id = ?`, userID); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return d.GetWebChatUser(userID)
}

func (d *Database) ResetWebChatUserPassword(email, code, password string) error {
	email = normalizeWebChatEmail(email)
	code = strings.TrimSpace(code)
	password = strings.TrimSpace(password)
	if email == "" {
		return fmt.Errorf("邮箱格式无效")
	}
	if code == "" {
		return fmt.Errorf("邮箱验证码不能为空")
	}
	if len(password) < 8 || len(password) > 128 {
		return fmt.Errorf("密码长度必须为 8 到 128 位")
	}
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var userID string
	if err = tx.QueryRow(`SELECT user_id FROM web_chat_users WHERE email = ? AND disabled = 0`, email).Scan(&userID); err == sql.ErrNoRows {
		return fmt.Errorf("邮箱账号不存在")
	} else if err != nil {
		return err
	}
	if err = consumeWebChatEmailCodeTx(tx, email, code, WebChatEmailPurposeResetPassword); err != nil {
		return err
	}
	passwordHash, err := HashAdminPassword(password)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE web_chat_users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE user_id = ?`, passwordHash, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (d *Database) VerifyWebChatLogin(login, password string) (*WebChatUser, error) {
	login = strings.ToLower(strings.TrimSpace(login))
	password = strings.TrimSpace(password)
	if login == "" || password == "" {
		return nil, fmt.Errorf("账号或密码错误")
	}
	var userID, hash string
	err := d.db.QueryRow(`
		SELECT user_id, password_hash FROM web_chat_users
		WHERE disabled = 0 AND (LOWER(username) = ? OR LOWER(email) = ?)
	`, login, login).Scan(&userID, &hash)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("账号或密码错误")
	}
	if err != nil {
		return nil, err
	}
	if !VerifyAdminPasswordHash(password, hash) {
		return nil, fmt.Errorf("账号或密码错误")
	}
	_, _ = d.db.Exec(`UPDATE web_chat_users SET last_login_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE user_id = ?`, userID)
	return d.GetWebChatUser(userID)
}

func (d *Database) CreateWebChatSession(userID, userAgent, clientIP string) (*WebChatSession, error) {
	user, err := d.GetWebChatUser(userID)
	if err != nil {
		return nil, err
	}
	token, err := randomHex(32)
	if err != nil {
		return nil, err
	}
	csrf, err := randomHex(24)
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(webChatSessionTTL)
	tokenHash := WebChatTokenHash(token)
	_, err = d.db.Exec(`
		INSERT INTO web_chat_sessions (token_hash, user_id, csrf_token, expires_at, user_agent, client_ip, created_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, tokenHash, userID, csrf, expiresAt, truncateWebChatText(userAgent, 300), truncateWebChatText(clientIP, 100))
	if err != nil {
		return nil, err
	}
	return &WebChatSession{Token: token, TokenHash: tokenHash, CSRFToken: csrf, User: user, ExpiresAt: expiresAt}, nil
}

func (d *Database) GetWebChatSession(token string) (*WebChatSession, error) {
	tokenHash := WebChatTokenHash(token)
	if tokenHash == "" {
		return nil, sql.ErrNoRows
	}
	var userID, csrf string
	var expiresAt time.Time
	err := d.db.QueryRow(`SELECT user_id, csrf_token, expires_at FROM web_chat_sessions WHERE token_hash = ? AND expires_at > CURRENT_TIMESTAMP`, tokenHash).Scan(&userID, &csrf, &expiresAt)
	if err != nil {
		return nil, err
	}
	user, err := d.GetWebChatUser(userID)
	if err != nil {
		return nil, err
	}
	return &WebChatSession{Token: token, TokenHash: tokenHash, CSRFToken: csrf, User: user, ExpiresAt: expiresAt}, nil
}

func (d *Database) DeleteWebChatSession(token string) error {
	tokenHash := WebChatTokenHash(token)
	if tokenHash == "" {
		return nil
	}
	_, err := d.db.Exec(`DELETE FROM web_chat_sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (d *Database) BindWebChatUserByCode(userID, code string) (*UserAccount, *UserAccount, error) {
	userID, code = strings.TrimSpace(userID), strings.TrimSpace(code)
	if userID == "" || code == "" {
		return nil, nil, fmt.Errorf("用户和绑定码不能为空")
	}
	tx, err := d.db.Begin()
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()
	webAccount, err := scanUserAccount(tx.QueryRow(`SELECT ua.id, ua.platform, ua.user_id, ua.union_id, COALESCE(up.points, 0), ua.created_at, ua.updated_at FROM user_accounts ua LEFT JOIN user_points up ON up.union_id = ua.union_id WHERE ua.platform = ? AND ua.user_id = ?`, WebChatPlatform, userID))
	if err != nil {
		return nil, nil, err
	}
	var boundCount int
	if err = tx.QueryRow(`SELECT COUNT(1) FROM user_accounts WHERE union_id = ? AND NOT (platform = ? AND user_id = ?)`, webAccount.UnionID, WebChatPlatform, userID).Scan(&boundCount); err != nil {
		return nil, nil, err
	}
	if boundCount > 0 {
		return nil, nil, fmt.Errorf("当前账号已绑定其他平台，不能重复绑定")
	}
	source, err := userBindCodeByCodeTx(tx, code)
	if err != nil {
		return nil, nil, err
	}
	if source.Platform == WebChatPlatform {
		return nil, nil, fmt.Errorf("不能绑定另一个 web 账号")
	}
	if source.UnionID != webAccount.UnionID {
		if _, err = tx.Exec(`
			INSERT INTO user_points (union_id, points, created_at, updated_at)
			VALUES (?, COALESCE((SELECT points FROM user_points WHERE union_id = ?), 0) + COALESCE((SELECT points FROM user_points WHERE union_id = ?), 0), CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			ON CONFLICT(union_id) DO UPDATE SET points = user_points.points + COALESCE((SELECT points FROM user_points WHERE union_id = ?), 0), updated_at = CURRENT_TIMESTAMP
		`, webAccount.UnionID, webAccount.UnionID, source.UnionID, source.UnionID); err != nil {
			return nil, nil, err
		}
		if _, err = tx.Exec(`UPDATE user_accounts SET union_id = ?, updated_at = CURRENT_TIMESTAMP WHERE union_id = ?`, webAccount.UnionID, source.UnionID); err != nil {
			return nil, nil, err
		}
		if _, err = tx.Exec(`DELETE FROM user_points WHERE union_id = ?`, source.UnionID); err != nil {
			return nil, nil, err
		}
	}
	if _, err = tx.Exec(`DELETE FROM user_bind_codes WHERE code = ?`, code); err != nil {
		return nil, nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, nil, err
	}
	boundWeb, err := d.GetUserAccount(WebChatPlatform, userID)
	if err != nil {
		return nil, nil, err
	}
	boundSource, err := d.GetUserAccount(source.Platform, source.UserID)
	if err != nil {
		return nil, nil, err
	}
	return boundWeb, boundSource, nil
}

func (d *Database) GetWebChatUser(userID string) (*WebChatUser, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, sql.ErrNoRows
	}
	return scanWebChatUser(d.db.QueryRow(webChatUserSelectSQL()+` WHERE u.user_id = ?`, userID))
}

func (d *Database) SaveWebChatMessage(message *WebChatMessage) (*WebChatMessage, error) {
	if message == nil {
		return nil, fmt.Errorf("消息不能为空")
	}
	message.UserID = strings.TrimSpace(message.UserID)
	message.Direction = normalizeWebChatMessageDirection(message.Direction)
	message.MessageType = normalizeWebChatMessageType(message.MessageType)
	if message.UserID == "" {
		return nil, fmt.Errorf("用户不能为空")
	}
	result, err := d.db.Exec(`
		INSERT INTO web_chat_messages (user_id, direction, message_type, content, image_url, rich_json, target, plugin_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, message.UserID, message.Direction, message.MessageType, truncateWebChatText(message.Content, 20000), truncateWebChatText(message.ImageURL, 2000), truncateWebChatText(message.RichJSON, 50000), truncateWebChatText(message.Target, 300), truncateWebChatText(message.PluginID, 200))
	if err != nil {
		return nil, err
	}
	id, _ := result.LastInsertId()
	return d.GetWebChatMessage(id)
}

func (d *Database) GetWebChatMessage(messageID int64) (*WebChatMessage, error) {
	return scanWebChatMessage(d.db.QueryRow(webChatMessageSelectSQL()+` WHERE message_id = ?`, messageID))
}

func (d *Database) ListWebChatMessages(userID string, afterID int64, limit int) ([]*WebChatMessage, error) {
	return d.ListWebChatMessagesByPlugin(userID, "", afterID, limit)
}

func (d *Database) ListWebChatMessagesByPlugin(userID string, pluginID string, afterID int64, limit int) ([]*WebChatMessage, error) {
	userID = strings.TrimSpace(userID)
	pluginID = strings.TrimSpace(pluginID)
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := webChatMessageSelectSQL() + ` WHERE user_id = ? AND plugin_id = ? AND message_id > ? ORDER BY message_id ASC LIMIT ?`
	if afterID <= 0 {
		query = `SELECT * FROM (` + webChatMessageSelectSQL() + ` WHERE user_id = ? AND plugin_id = ? ORDER BY message_id DESC LIMIT ?) ORDER BY message_id ASC`
	}
	var rows *sql.Rows
	var err error
	if afterID <= 0 {
		rows, err = d.db.Query(query, userID, pluginID, limit)
	} else {
		rows, err = d.db.Query(query, userID, pluginID, afterID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]*WebChatMessage, 0)
	for rows.Next() {
		item, err := scanWebChatMessage(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (d *Database) CountWebChatMessagesByPlugin(userID string) ([]*WebChatMessageCount, error) {
	userID = strings.TrimSpace(userID)
	rows, err := d.db.Query(`
		SELECT m.plugin_id, COUNT(*), MAX(m.message_id), COALESCE(r.last_read_message_id, 0),
			SUM(CASE WHEN m.message_id > COALESCE(r.last_read_message_id, 0) THEN 1 ELSE 0 END)
		FROM web_chat_messages m
		LEFT JOIN web_chat_read_states r ON r.user_id = m.user_id AND r.plugin_id = m.plugin_id
		WHERE m.user_id = ?
		GROUP BY m.plugin_id, COALESCE(r.last_read_message_id, 0)
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]*WebChatMessageCount, 0)
	for rows.Next() {
		item := &WebChatMessageCount{}
		if err := rows.Scan(&item.PluginID, &item.Count, &item.LastMessageID, &item.LastReadMessageID, &item.UnreadCount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (d *Database) MarkWebChatMessagesRead(userID, pluginID string) (*WebChatMessageCount, error) {
	userID = strings.TrimSpace(userID)
	pluginID = strings.TrimSpace(pluginID)
	if userID == "" {
		return nil, fmt.Errorf("用户不能为空")
	}
	var lastMessageID int64
	if err := d.db.QueryRow(`SELECT COALESCE(MAX(message_id), 0) FROM web_chat_messages WHERE user_id = ? AND plugin_id = ?`, userID, pluginID).Scan(&lastMessageID); err != nil {
		return nil, err
	}
	_, err := d.db.Exec(`
		INSERT INTO web_chat_read_states (user_id, plugin_id, last_read_message_id, created_at, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id, plugin_id) DO UPDATE SET last_read_message_id = excluded.last_read_message_id, updated_at = CURRENT_TIMESTAMP
	`, userID, pluginID, lastMessageID)
	if err != nil {
		return nil, err
	}
	return d.GetWebChatMessageCount(userID, pluginID)
}

func (d *Database) GetWebChatMessageCount(userID, pluginID string) (*WebChatMessageCount, error) {
	userID = strings.TrimSpace(userID)
	pluginID = strings.TrimSpace(pluginID)
	item := &WebChatMessageCount{PluginID: pluginID}
	err := d.db.QueryRow(`
		SELECT COUNT(m.message_id), COALESCE(MAX(m.message_id), 0), COALESCE(r.last_read_message_id, 0),
			COALESCE(SUM(CASE WHEN m.message_id > COALESCE(r.last_read_message_id, 0) THEN 1 ELSE 0 END), 0)
		FROM web_chat_messages m
		LEFT JOIN web_chat_read_states r ON r.user_id = m.user_id AND r.plugin_id = m.plugin_id
		WHERE m.user_id = ? AND m.plugin_id = ?
		GROUP BY COALESCE(r.last_read_message_id, 0)
	`, userID, pluginID).Scan(&item.Count, &item.LastMessageID, &item.LastReadMessageID, &item.UnreadCount)
	if err == sql.ErrNoRows {
		var lastRead int64
		if readErr := d.db.QueryRow(`SELECT last_read_message_id FROM web_chat_read_states WHERE user_id = ? AND plugin_id = ?`, userID, pluginID).Scan(&lastRead); readErr != nil && readErr != sql.ErrNoRows {
			return nil, readErr
		} else if readErr == nil {
			item.LastReadMessageID = lastRead
		}
		return item, nil
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

func WebChatTokenHash(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func RandomWebChatEmailCode() (string, error) {
	max := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func normalizeWebChatEmail(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	addr, err := mail.ParseAddress(value)
	if err != nil || strings.ToLower(addr.Address) != value {
		return ""
	}
	return value
}

func normalizeWebChatPurpose(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return WebChatEmailPurposeRegister, nil
	}
	switch value {
	case WebChatEmailPurposeRegister, WebChatEmailPurposeResetPassword, WebChatEmailPurposeLogin:
		return value, nil
	default:
		return "", fmt.Errorf("验证码用途无效")
	}
}

func (d *Database) ensureWebChatEmailRate(email, purpose, sentIP string) error {
	var recent int
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM web_chat_email_codes WHERE email = ? AND purpose = ? AND created_at > datetime('now', '-60 seconds')`, email, purpose).Scan(&recent); err != nil {
		return err
	}
	if recent > 0 {
		return fmt.Errorf("验证码发送过于频繁，请稍后再试")
	}
	var emailCount int
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM web_chat_email_codes WHERE email = ? AND purpose = ? AND created_at > datetime('now', '-10 minutes')`, email, purpose).Scan(&emailCount); err != nil {
		return err
	}
	if emailCount >= 5 {
		return fmt.Errorf("验证码发送次数过多，请稍后再试")
	}
	if strings.TrimSpace(sentIP) != "" {
		var ipCount int
		if err := d.db.QueryRow(`SELECT COUNT(*) FROM web_chat_email_codes WHERE sent_ip = ? AND created_at > datetime('now', '-10 minutes')`, strings.TrimSpace(sentIP)).Scan(&ipCount); err != nil {
			return err
		}
		if ipCount >= 5 {
			return fmt.Errorf("验证码发送次数过多，请稍后再试")
		}
	}
	return nil
}

func consumeWebChatEmailCodeTx(tx *sql.Tx, email, code, purpose string) error {
	var id, attempts int64
	var hash string
	err := tx.QueryRow(`
		SELECT id, code_hash, attempts FROM web_chat_email_codes
		WHERE email = ? AND purpose = ? AND used_at IS NULL AND expires_at > CURRENT_TIMESTAMP
		ORDER BY created_at DESC, id DESC LIMIT 1
	`, email, purpose).Scan(&id, &hash, &attempts)
	if err == sql.ErrNoRows {
		return fmt.Errorf("验证码不存在或已过期")
	}
	if err != nil {
		return err
	}
	if attempts >= webChatMaxCodeAttempts {
		return fmt.Errorf("验证码错误次数过多，请重新获取")
	}
	if !VerifyAdminPasswordHash(code, hash) {
		_, _ = tx.Exec(`UPDATE web_chat_email_codes SET attempts = attempts + 1 WHERE id = ?`, id)
		return fmt.Errorf("验证码错误")
	}
	_, err = tx.Exec(`UPDATE web_chat_email_codes SET used_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

func userBindCodeByCodeTx(tx *sql.Tx, code string) (*UserAccount, error) {
	var source UserAccount
	var expiresAt time.Time
	err := tx.QueryRow(`SELECT 0, platform, user_id, union_id, 0, created_at, expires_at FROM user_bind_codes WHERE code = ? AND expires_at > CURRENT_TIMESTAMP`, strings.TrimSpace(code)).Scan(&source.ID, &source.Platform, &source.UserID, &source.UnionID, &source.Points, &source.CreatedAt, &expiresAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("绑定码不存在或已过期")
	}
	if err != nil {
		return nil, err
	}
	return &source, nil
}

func scanWebChatUser(row interface {
	Scan(dest ...interface{}) error
}) (*WebChatUser, error) {
	var item WebChatUser
	var verified, disabled int
	var lastLogin sql.NullTime
	var unionID sql.NullString
	if err := row.Scan(&item.UserID, &item.Username, &item.Email, &item.DisplayName, &verified, &disabled, &lastLogin, &item.CreatedAt, &item.UpdatedAt, &unionID); err != nil {
		return nil, err
	}
	item.EmailVerified = verified == 1
	item.Disabled = disabled == 1
	if lastLogin.Valid {
		item.LastLoginAt = &lastLogin.Time
	}
	item.UnionID = unionID.String
	return &item, nil
}

func webChatUserSelectSQL() string {
	return `SELECT u.user_id, u.username, u.email, u.display_name, u.email_verified, u.disabled, u.last_login_at, u.created_at, u.updated_at, ua.union_id FROM web_chat_users u LEFT JOIN user_accounts ua ON ua.platform = 'web' AND ua.user_id = u.user_id`
}

func scanWebChatMessage(row interface {
	Scan(dest ...interface{}) error
}) (*WebChatMessage, error) {
	var item WebChatMessage
	if err := row.Scan(&item.MessageID, &item.UserID, &item.Direction, &item.MessageType, &item.Content, &item.ImageURL, &item.RichJSON, &item.Target, &item.PluginID, &item.CreatedAt); err != nil {
		return nil, err
	}
	return &item, nil
}

func webChatMessageSelectSQL() string {
	return `SELECT message_id, user_id, direction, message_type, content, image_url, rich_json, target, plugin_id, created_at FROM web_chat_messages`
}

func normalizeWebChatMessageDirection(value string) string {
	if strings.TrimSpace(value) == "out" {
		return "out"
	}
	return "in"
}

func normalizeWebChatMessageType(value string) string {
	switch strings.TrimSpace(value) {
	case "markdown", "image", "rich", "buttons":
		return strings.TrimSpace(value)
	default:
		return "text"
	}
}

func truncateWebChatText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len([]rune(value)) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit])
}

func newWebChatUserID() (string, error) {
	value, err := randomHex(12)
	if err != nil {
		return "", err
	}
	return "web_" + value, nil
}

func randomHex(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
