package config

import (
	"database/sql"
	"strings"
	"time"
)

const (
	PluginTaskStatusRunning  = "running"
	PluginTaskStatusStopping = "stopping"
	PluginTaskStatusSuccess  = "success"
	PluginTaskStatusFailed   = "failed"
	PluginTaskStatusStopped  = "stopped"
)

type PluginTaskLog struct {
	ID         int64     `json:"id"`
	PluginID   string    `json:"plugin_id"`
	Platform   string    `json:"platform"`
	AdapterID  string    `json:"adapter_id"`
	UserID     string    `json:"user_id"`
	GroupID    string    `json:"group_id"`
	UnionID    string    `json:"union_id"`
	MessageID  string    `json:"message_id"`
	Content    string    `json:"content"`
	Status     string    `json:"status"`
	RunTotal   int64     `json:"run_total"`
	Output     string    `json:"output"`
	Error      string    `json:"error"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	CreatedAt  time.Time `json:"created_at"`
}

type PluginTaskLogFilter struct {
	Keyword  string
	PluginID string
	Platform string
	Status   string
	Limit    int
	Offset   int
}

func (filter PluginTaskLogFilter) buildWhere() (string, []interface{}) {
	where := []string{"1 = 1"}
	args := make([]interface{}, 0)
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		where = append(where, `(plugin_id LIKE ? OR platform LIKE ? OR adapter_id LIKE ? OR user_id LIKE ? OR group_id LIKE ? OR union_id LIKE ? OR message_id LIKE ? OR content LIKE ? OR status LIKE ?)`)
		for i := 0; i < 9; i++ {
			args = append(args, like)
		}
	}
	if value := strings.TrimSpace(filter.PluginID); value != "" {
		where = append(where, `plugin_id LIKE ?`)
		args = append(args, "%"+value+"%")
	}
	if value := strings.TrimSpace(filter.Platform); value != "" {
		where = append(where, `platform = ?`)
		args = append(args, value)
	}
	if value := strings.TrimSpace(filter.Status); value != "" {
		where = append(where, `status = ?`)
		args = append(args, value)
	}
	return strings.Join(where, " AND "), args
}

func (d *Database) SavePluginTaskLog(item PluginTaskLog) (int64, error) {
	if item.Status == "" {
		item.Status = PluginTaskStatusRunning
	}
	if item.StartedAt.IsZero() {
		item.StartedAt = time.Now()
	}
	if item.FinishedAt.IsZero() {
		item.FinishedAt = item.StartedAt
	}
	if item.RunTotal <= 0 {
		item.RunTotal = 1
	}
	result, err := d.db.Exec(`
		INSERT INTO plugin_task_logs (plugin_id, platform, adapter_id, user_id, group_id, union_id, message_id, content, status, run_total, output, error, started_at, finished_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, item.PluginID, item.Platform, item.AdapterID, item.UserID, item.GroupID, item.UnionID, item.MessageID, item.Content, item.Status, item.RunTotal, item.Output, item.Error, item.StartedAt, item.FinishedAt)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (d *Database) FindLatestPluginTaskLog(item PluginTaskLog) (*PluginTaskLog, error) {
	return d.findPluginTaskLog(`plugin_id = ? AND platform = ? AND adapter_id = ? AND user_id = ? AND group_id = ? AND union_id = ? AND content = ?`, item.PluginID, item.Platform, item.AdapterID, item.UserID, item.GroupID, item.UnionID, item.Content)
}

func (d *Database) ReusePluginTaskLog(id int64, item PluginTaskLog) (bool, error) {
	result, err := d.db.Exec(`
		UPDATE plugin_task_logs
		SET platform = ?, adapter_id = ?, user_id = ?, group_id = ?, union_id = ?, message_id = ?, content = ?, status = ?, run_total = COALESCE(run_total, 0) + 1, output = '', error = '', started_at = ?, finished_at = ?
		WHERE id = ? AND status IN (?, ?)
	`, item.Platform, item.AdapterID, item.UserID, item.GroupID, item.UnionID, item.MessageID, item.Content, PluginTaskStatusRunning, item.StartedAt, item.StartedAt, id, PluginTaskStatusSuccess, PluginTaskStatusFailed)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows > 0, err
}

func (d *Database) UpdatePluginTaskLog(id int64, status, output, errorText string, finishedAt time.Time) error {
	if finishedAt.IsZero() {
		_, err := d.db.Exec(`UPDATE plugin_task_logs SET status = ?, output = ?, error = ? WHERE id = ?`, status, output, errorText, id)
		return err
	}
	_, err := d.db.Exec(`UPDATE plugin_task_logs SET status = ?, output = ?, error = ?, finished_at = ? WHERE id = ?`, status, output, errorText, finishedAt, id)
	return err
}

func (d *Database) GetPluginTaskLog(id int64) (*PluginTaskLog, error) {
	item, err := scanPluginTaskLog(d.db.QueryRow(`
		SELECT id, plugin_id, platform, adapter_id, user_id, group_id, union_id, message_id, content, status, run_total, output, error, started_at, finished_at, created_at
		FROM plugin_task_logs WHERE id = ?
	`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return item, err
}

func (d *Database) ListPluginTaskLogs(filter PluginTaskLogFilter) ([]*PluginTaskLog, error) {
	where, args := filter.buildWhere()
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	args = append(args, limit, offset)
	rows, err := d.db.Query(`
		SELECT id, plugin_id, platform, adapter_id, user_id, group_id, union_id, message_id, content, status, run_total, '', error, started_at, finished_at, created_at
		FROM plugin_task_logs WHERE `+where+` ORDER BY started_at DESC, id DESC LIMIT ? OFFSET ?
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]*PluginTaskLog, 0)
	for rows.Next() {
		item, err := scanPluginTaskLog(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (d *Database) CountPluginTaskLogs(filter PluginTaskLogFilter) (int, error) {
	where, args := filter.buildWhere()
	var total int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM plugin_task_logs WHERE `+where, args...).Scan(&total)
	return total, err
}

func (d *Database) DeletePluginTaskLog(id int64) error {
	_, err := d.db.Exec(`DELETE FROM plugin_task_logs WHERE id = ?`, id)
	return err
}

func (d *Database) CleanupPluginTaskLogs(retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, nil
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	result, err := d.db.Exec(`DELETE FROM plugin_task_logs WHERE status NOT IN (?, ?) AND finished_at < ?`, PluginTaskStatusRunning, PluginTaskStatusStopping, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (d *Database) MarkRunningPluginTasksStopped() error {
	_, err := d.db.Exec(`
		UPDATE plugin_task_logs
		SET status = ?, error = ?, finished_at = CURRENT_TIMESTAMP
		WHERE status IN (?, ?)
	`, PluginTaskStatusStopped, "AllBot 重启后插件任务已停止", PluginTaskStatusRunning, PluginTaskStatusStopping)
	return err
}

type pluginTaskLogScanner interface {
	Scan(dest ...interface{}) error
}

func scanPluginTaskLog(scanner pluginTaskLogScanner) (*PluginTaskLog, error) {
	var item PluginTaskLog
	if err := scanner.Scan(&item.ID, &item.PluginID, &item.Platform, &item.AdapterID, &item.UserID, &item.GroupID, &item.UnionID, &item.MessageID, &item.Content, &item.Status, &item.RunTotal, &item.Output, &item.Error, &item.StartedAt, &item.FinishedAt, &item.CreatedAt); err != nil {
		return nil, err
	}
	return &item, nil
}

func (d *Database) findPluginTaskLog(where string, args ...interface{}) (*PluginTaskLog, error) {
	item, err := scanPluginTaskLog(d.db.QueryRow(`
		SELECT id, plugin_id, platform, adapter_id, user_id, group_id, union_id, message_id, content, status, run_total, output, error, started_at, finished_at, created_at
		FROM plugin_task_logs WHERE `+where+` ORDER BY id DESC LIMIT 1
	`, args...))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return item, err
}
