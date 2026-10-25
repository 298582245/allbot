package config

import (
	"database/sql"
	"fmt"
	"strings"
)

// ConsumeUnregisteredPrompt records one registration guide reply when the
// configured per-user limit has not been reached.
func (d *Database) ConsumeUnregisteredPrompt(platform, userID string) (bool, error) {
	platform = strings.TrimSpace(platform)
	userID = strings.TrimSpace(userID)
	if platform == "" || userID == "" {
		return false, nil
	}
	limit, err := d.GetUnregisteredPromptLimit()
	if err != nil {
		return false, err
	}
	if limit <= 0 {
		return false, nil
	}
	result, err := d.db.Exec(`
		INSERT INTO unregistered_prompt_stats (platform, user_id, prompt_count, updated_at)
		VALUES (?, ?, 1, CURRENT_TIMESTAMP)
		ON CONFLICT(platform, user_id) DO UPDATE SET
			prompt_count = prompt_count + 1,
			updated_at = CURRENT_TIMESTAMP
		WHERE prompt_count < ?
	`, platform, userID, limit)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (d *Database) GetUnregisteredPromptLimit() (int, error) {
	value, err := d.GetSetting(unregisteredPromptLimitKey)
	if err == sql.ErrNoRows || strings.TrimSpace(value) == "" {
		return 10, nil
	}
	if err != nil {
		return 0, err
	}
	limit := intValueAllowZeroOrDefault(map[string]string{unregisteredPromptLimitKey: value}, unregisteredPromptLimitKey, 10)
	if limit < 0 || limit > 1000000 {
		return 0, fmt.Errorf("未注册用户提示次数上限必须在 0 到 1000000 之间")
	}
	return limit, nil
}
