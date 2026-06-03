package config

import (
	"database/sql"
	"fmt"
	"time"
)

type KeywordReply struct {
	ID              int64     `json:"id"`
	Keyword         string    `json:"keyword"`
	MatchType       string    `json:"match_type"`
	ReplyType       string    `json:"reply_type"`
	Content         string    `json:"content"`
	Enabled         bool      `json:"enabled"`
	AdminOnly       bool      `json:"admin_only"`
	Pinned          bool      `json:"pinned"`
	Builtin         bool      `json:"builtin"`
	ScheduleEnabled bool      `json:"schedule_enabled"`
	ScheduleCron    string    `json:"schedule_cron"`
	Description     string    `json:"description"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (d *Database) ListKeywordReplies() ([]*KeywordReply, error) {
	rows, err := d.db.Query(`
		SELECT id, keyword, match_type, reply_type, content, enabled, admin_only, pinned, builtin, schedule_enabled, schedule_cron, description, created_at, updated_at
		FROM keyword_replies
		ORDER BY pinned DESC, builtin DESC, id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*KeywordReply, 0)
	for rows.Next() {
		item, err := scanKeywordReply(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (d *Database) SaveKeywordReply(item *KeywordReply) error {
	if item.Keyword == "" {
		return fmt.Errorf("关键字不能为空")
	}
	if item.Builtin {
		return fmt.Errorf("内置指令不能保存")
	}
	if item.MatchType == "" {
		item.MatchType = "regex"
	}
	if item.ReplyType == "" {
		item.ReplyType = "text"
	}
	now := time.Now()
	if item.ID == 0 {
		_, err := d.db.Exec(`
			INSERT INTO keyword_replies (keyword, match_type, reply_type, content, enabled, admin_only, pinned, builtin, schedule_enabled, schedule_cron, description, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?, ?)
		`, item.Keyword, item.MatchType, item.ReplyType, item.Content, boolInt(item.Enabled), boolInt(item.AdminOnly), boolInt(item.Pinned), boolInt(item.ScheduleEnabled), item.ScheduleCron, item.Description, now, now)
		return err
	}

	existing, err := d.GetKeywordReply(item.ID)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("关键字回复不存在: %d", item.ID)
	}
	if existing.Builtin {
		_, err = d.db.Exec(`
			UPDATE keyword_replies
			SET enabled = ?, admin_only = ?, pinned = ?, schedule_enabled = ?, schedule_cron = ?, description = ?, updated_at = ?
			WHERE id = ?
		`, boolInt(item.Enabled), boolInt(item.AdminOnly), boolInt(item.Pinned), boolInt(item.ScheduleEnabled), item.ScheduleCron, item.Description, now, item.ID)
		return err
	}
	_, err = d.db.Exec(`
		UPDATE keyword_replies
		SET keyword = ?, match_type = ?, reply_type = ?, content = ?, enabled = ?, admin_only = ?, pinned = ?, schedule_enabled = ?, schedule_cron = ?, description = ?, updated_at = ?
		WHERE id = ?
	`, item.Keyword, item.MatchType, item.ReplyType, item.Content, boolInt(item.Enabled), boolInt(item.AdminOnly), boolInt(item.Pinned), boolInt(item.ScheduleEnabled), item.ScheduleCron, item.Description, now, item.ID)
	return err
}

func (d *Database) GetKeywordReply(id int64) (*KeywordReply, error) {
	row := d.db.QueryRow(`
		SELECT id, keyword, match_type, reply_type, content, enabled, admin_only, pinned, builtin, schedule_enabled, schedule_cron, description, created_at, updated_at
		FROM keyword_replies WHERE id = ?
	`, id)
	item, err := scanKeywordReply(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (d *Database) DeleteKeywordReply(id int64) error {
	item, err := d.GetKeywordReply(id)
	if err != nil {
		return err
	}
	if item.Builtin {
		return fmt.Errorf("内置指令不能删除")
	}
	_, err = d.db.Exec(`DELETE FROM keyword_replies WHERE id = ?`, id)
	return err
}

type keywordReplyScanner interface {
	Scan(dest ...interface{}) error
}

func scanKeywordReply(scanner keywordReplyScanner) (*KeywordReply, error) {
	var item KeywordReply
	var enabled, adminOnly, pinned, builtin, scheduleEnabled int
	if err := scanner.Scan(&item.ID, &item.Keyword, &item.MatchType, &item.ReplyType, &item.Content, &enabled, &adminOnly, &pinned, &builtin, &scheduleEnabled, &item.ScheduleCron, &item.Description, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return nil, err
	}
	item.Enabled = enabled == 1
	item.AdminOnly = adminOnly == 1
	item.Pinned = pinned == 1
	item.Builtin = builtin == 1
	item.ScheduleEnabled = scheduleEnabled == 1
	return &item, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
