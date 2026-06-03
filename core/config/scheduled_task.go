package config

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type ScheduledTask struct {
	ID          int64      `json:"id"`
	PluginID    string     `json:"plugin_id"`
	TaskKey     string     `json:"task_key"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Enabled     bool       `json:"enabled"`
	Pinned      bool       `json:"pinned"`
	Cron        string     `json:"cron"`
	Platform    string     `json:"platform"`
	AdapterID   string     `json:"adapter_id"`
	UserID      string     `json:"user_id"`
	GroupID     string     `json:"group_id"`
	Content     string     `json:"content"`
	Source      string     `json:"source"`
	LastRunAt   *time.Time `json:"last_run_at,omitempty"`
	NextRunAt   *time.Time `json:"next_run_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (d *Database) ListScheduledTasks() ([]*ScheduledTask, error) {
	rows, err := d.db.Query(`
		SELECT id, plugin_id, task_key, name, description, enabled, pinned, cron, platform, adapter_id, user_id, group_id, content, source, last_run_at, next_run_at, created_at, updated_at
		FROM scheduled_tasks
		ORDER BY pinned DESC, enabled DESC, plugin_id ASC, id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*ScheduledTask, 0)
	for rows.Next() {
		item, err := scanScheduledTask(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (d *Database) ListDueScheduledTasks(now time.Time) ([]*ScheduledTask, error) {
	rows, err := d.db.Query(`
		SELECT id, plugin_id, task_key, name, description, enabled, pinned, cron, platform, adapter_id, user_id, group_id, content, source, last_run_at, next_run_at, created_at, updated_at
		FROM scheduled_tasks
		WHERE enabled = 1 AND LOWER(TRIM(cron)) <> '@once' AND next_run_at IS NOT NULL AND next_run_at <= ?
		ORDER BY next_run_at ASC, id ASC
	`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*ScheduledTask, 0)
	for rows.Next() {
		item, err := scanScheduledTask(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (d *Database) GetScheduledTask(id int64) (*ScheduledTask, error) {
	row := d.db.QueryRow(`
		SELECT id, plugin_id, task_key, name, description, enabled, pinned, cron, platform, adapter_id, user_id, group_id, content, source, last_run_at, next_run_at, created_at, updated_at
		FROM scheduled_tasks WHERE id = ?
	`, id)
	item, err := scanScheduledTask(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return item, err
}

func (d *Database) SaveScheduledTask(item *ScheduledTask) error {
	if err := normalizeScheduledTask(item); err != nil {
		return err
	}
	now := time.Now()
	if item.ID == 0 {
		_, err := d.db.Exec(`
			INSERT INTO scheduled_tasks (plugin_id, task_key, name, description, enabled, pinned, cron, platform, adapter_id, user_id, group_id, content, source, next_run_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, item.PluginID, item.TaskKey, item.Name, item.Description, boolInt(item.Enabled), boolInt(item.Pinned), item.Cron, item.Platform, item.AdapterID, item.UserID, item.GroupID, item.Content, item.Source, item.NextRunAt, now, now)
		return err
	}
	_, err := d.db.Exec(`
		UPDATE scheduled_tasks
		SET plugin_id = ?, task_key = ?, name = ?, description = ?, enabled = ?, pinned = ?, cron = ?, platform = ?, adapter_id = ?, user_id = ?, group_id = ?, content = ?, source = ?, next_run_at = ?, updated_at = ?
		WHERE id = ?
	`, item.PluginID, item.TaskKey, item.Name, item.Description, boolInt(item.Enabled), boolInt(item.Pinned), item.Cron, item.Platform, item.AdapterID, item.UserID, item.GroupID, item.Content, item.Source, item.NextRunAt, now, item.ID)
	return err
}

func (d *Database) UpsertPluginScheduledTask(pluginID string, item *ScheduledTask, maxCount int) (*ScheduledTask, error) {
	if strings.TrimSpace(pluginID) == "" {
		return nil, fmt.Errorf("插件 ID 不能为空")
	}
	item.PluginID = strings.TrimSpace(pluginID)
	item.Source = "plugin"
	if strings.TrimSpace(item.TaskKey) == "" {
		item.TaskKey = strings.TrimSpace(item.Name)
	}
	if strings.TrimSpace(item.TaskKey) == "" {
		return nil, fmt.Errorf("任务 key 不能为空")
	}
	if err := normalizeScheduledTask(item); err != nil {
		return nil, err
	}
	now := time.Now()
	var existingID int64
	err := d.db.QueryRow(`SELECT id FROM scheduled_tasks WHERE plugin_id = ? AND task_key = ?`, item.PluginID, item.TaskKey).Scan(&existingID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if err == nil {
		item.ID = existingID
		if item.Enabled && item.NextRunAt == nil && !strings.EqualFold(strings.TrimSpace(item.Cron), "@once") {
			var existingNextRunAt sql.NullTime
			if err := d.db.QueryRow(`SELECT next_run_at FROM scheduled_tasks WHERE id = ?`, item.ID).Scan(&existingNextRunAt); err != nil {
				return nil, err
			}
			if existingNextRunAt.Valid && existingNextRunAt.Time.After(now) {
				item.NextRunAt = &existingNextRunAt.Time
			}
		}
		_, err = d.db.Exec(`
			UPDATE scheduled_tasks
			SET name = ?, description = ?, enabled = ?, pinned = ?, cron = ?, platform = ?, adapter_id = ?, user_id = ?, group_id = ?, content = ?, source = 'plugin', next_run_at = ?, updated_at = ?
			WHERE id = ?
		`, item.Name, item.Description, boolInt(item.Enabled), boolInt(item.Pinned), item.Cron, item.Platform, item.AdapterID, item.UserID, item.GroupID, item.Content, item.NextRunAt, now, item.ID)
	} else {
		result, insertErr := d.db.Exec(`
			INSERT INTO scheduled_tasks (plugin_id, task_key, name, description, enabled, pinned, cron, platform, adapter_id, user_id, group_id, content, source, next_run_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'plugin', ?, ?, ?)
		`, item.PluginID, item.TaskKey, item.Name, item.Description, boolInt(item.Enabled), boolInt(item.Pinned), item.Cron, item.Platform, item.AdapterID, item.UserID, item.GroupID, item.Content, item.NextRunAt, now, now)
		err = insertErr
		if err == nil {
			item.ID, _ = result.LastInsertId()
		}
	}
	if err != nil {
		return nil, err
	}
	if maxCount > 0 {
		if err := d.TrimPluginScheduledTasks(item.PluginID, maxCount); err != nil {
			return nil, err
		}
	}
	return d.GetScheduledTask(item.ID)
}

func (d *Database) TrimPluginScheduledTasks(pluginID string, maxCount int) error {
	if maxCount <= 0 {
		return nil
	}
	_, err := d.db.Exec(`
		DELETE FROM scheduled_tasks
		WHERE id IN (
			SELECT id FROM scheduled_tasks
			WHERE plugin_id = ? AND source = 'plugin'
			ORDER BY created_at ASC, id ASC
			LIMIT max((SELECT COUNT(*) FROM scheduled_tasks WHERE plugin_id = ? AND source = 'plugin') - ?, 0)
		)
	`, pluginID, pluginID, maxCount)
	return err
}

func (d *Database) MarkScheduledTaskRun(id int64, lastRunAt time.Time, nextRunAt *time.Time) error {
	_, err := d.db.Exec(`
		UPDATE scheduled_tasks
		SET last_run_at = ?, next_run_at = ?, updated_at = ?
		WHERE id = ?
	`, lastRunAt, nextRunAt, time.Now(), id)
	return err
}

func (d *Database) UpdateScheduledTaskNextRun(id int64, nextRunAt *time.Time) error {
	_, err := d.db.Exec(`
		UPDATE scheduled_tasks
		SET next_run_at = ?, updated_at = ?
		WHERE id = ?
	`, nextRunAt, time.Now(), id)
	return err
}

func (d *Database) DeleteScheduledTask(id int64) error {
	_, err := d.db.Exec(`DELETE FROM scheduled_tasks WHERE id = ?`, id)
	return err
}

func (d *Database) DisablePluginScheduledTasks(pluginID string) (int64, error) {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return 0, fmt.Errorf("插件 ID 不能为空")
	}
	result, err := d.db.Exec(`
		UPDATE scheduled_tasks
		SET enabled = 0, next_run_at = NULL, updated_at = ?
		WHERE plugin_id = ? AND source = 'plugin' AND (enabled <> 0 OR next_run_at IS NOT NULL)
	`, time.Now(), pluginID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func normalizeScheduledTask(item *ScheduledTask) error {
	if item == nil {
		return fmt.Errorf("定时任务不能为空")
	}
	item.PluginID = strings.TrimSpace(item.PluginID)
	item.TaskKey = strings.TrimSpace(item.TaskKey)
	item.Name = strings.TrimSpace(item.Name)
	item.Description = strings.TrimSpace(item.Description)
	item.Cron = strings.TrimSpace(item.Cron)
	item.Platform = strings.TrimSpace(item.Platform)
	item.AdapterID = strings.TrimSpace(item.AdapterID)
	item.UserID = strings.TrimSpace(item.UserID)
	item.GroupID = strings.TrimSpace(item.GroupID)
	item.Content = strings.TrimSpace(item.Content)
	item.Source = strings.TrimSpace(item.Source)
	if item.Source == "" {
		item.Source = "user"
	}
	if item.Name == "" {
		item.Name = item.TaskKey
	}
	if item.Cron == "" {
		return fmt.Errorf("定时表达式不能为空")
	}
	if item.Platform == "" {
		return fmt.Errorf("平台不能为空")
	}
	if item.UserID == "" {
		return fmt.Errorf("用户 ID 不能为空")
	}
	if item.Content == "" {
		return fmt.Errorf("消息内容不能为空")
	}
	return nil
}

type scheduledTaskScanner interface {
	Scan(dest ...interface{}) error
}

func scanScheduledTask(scanner scheduledTaskScanner) (*ScheduledTask, error) {
	var item ScheduledTask
	var enabled, pinned int
	var lastRunAt, nextRunAt sql.NullTime
	if err := scanner.Scan(&item.ID, &item.PluginID, &item.TaskKey, &item.Name, &item.Description, &enabled, &pinned, &item.Cron, &item.Platform, &item.AdapterID, &item.UserID, &item.GroupID, &item.Content, &item.Source, &lastRunAt, &nextRunAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return nil, err
	}
	item.Enabled = enabled == 1
	item.Pinned = pinned == 1
	if lastRunAt.Valid {
		item.LastRunAt = &lastRunAt.Time
	}
	if nextRunAt.Valid {
		item.NextRunAt = &nextRunAt.Time
	}
	return &item, nil
}
