package config

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type ScriptRunLog struct {
	ID         int64     `json:"id"`
	PluginID   string    `json:"plugin_id"`
	UnionID    string    `json:"union_id"`
	ScriptPath string    `json:"script_path"`
	Runtime    string    `json:"runtime"`
	RunMode    string    `json:"run_mode"`
	Status     string    `json:"status"`
	Output     string    `json:"output"`
	Error      string    `json:"error"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	CreatedAt  time.Time `json:"created_at"`
}

type ScriptRunLogFilter struct {
	Keyword    string
	UnionID    string
	PluginID   string
	ScriptPath string
	RunMode    string
	Status     string
	Limit      int
	Offset     int
}

func (filter ScriptRunLogFilter) buildWhere() (string, []interface{}) {
	where := []string{"1 = 1"}
	args := make([]interface{}, 0)
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		where = append(where, `(union_id LIKE ? OR plugin_id LIKE ? OR script_path LIKE ? OR run_mode LIKE ? OR status LIKE ?)`)
		args = append(args, like, like, like, like, like)
	}
	if unionID := strings.TrimSpace(filter.UnionID); unionID != "" {
		where = append(where, `union_id LIKE ?`)
		args = append(args, "%"+unionID+"%")
	}
	if pluginID := strings.TrimSpace(filter.PluginID); pluginID != "" {
		where = append(where, `plugin_id LIKE ?`)
		args = append(args, "%"+pluginID+"%")
	}
	if scriptPath := strings.TrimSpace(filter.ScriptPath); scriptPath != "" {
		where = append(where, `script_path LIKE ?`)
		args = append(args, "%"+scriptPath+"%")
	}
	if runMode := strings.TrimSpace(filter.RunMode); runMode != "" {
		where = append(where, `run_mode = ?`)
		args = append(args, runMode)
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		where = append(where, `status = ?`)
		args = append(args, status)
	}
	return strings.Join(where, " AND "), args
}

func (d *Database) SaveScriptRunLog(item ScriptRunLog) (int64, error) {
	result, err := d.db.Exec(`
		INSERT INTO script_run_logs (plugin_id, union_id, script_path, runtime, run_mode, status, output, error, started_at, finished_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.PluginID, item.UnionID, item.ScriptPath, item.Runtime, item.RunMode, item.Status, item.Output, item.Error, item.StartedAt, item.FinishedAt, time.Now())
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (d *Database) UpsertScriptRunLog(item ScriptRunLog) (int64, bool, error) {
	existing, err := d.FindLatestScriptRunLog(item.PluginID, item.ScriptPath, item.RunMode, item.UnionID)
	if err != nil {
		return 0, false, err
	}
	if existing == nil {
		id, err := d.SaveScriptRunLog(item)
		return id, false, err
	}
	_, err = d.db.Exec(`
		UPDATE script_run_logs
		SET union_id = ?, runtime = ?, status = ?, output = '', error = '', started_at = ?, finished_at = ?, created_at = ?
		WHERE id = ?
	`, item.UnionID, item.Runtime, item.Status, item.StartedAt, item.FinishedAt, time.Now(), existing.ID)
	return existing.ID, true, err
}

func (d *Database) FindLatestScriptRunLog(pluginID, scriptPath, runMode, unionID string) (*ScriptRunLog, error) {
	query := `
		SELECT id, plugin_id, union_id, script_path, runtime, run_mode, status, output, error, started_at, finished_at, created_at
		FROM script_run_logs
		WHERE plugin_id = ? AND script_path = ? AND run_mode = ?
	`
	args := []interface{}{pluginID, scriptPath, runMode}
	if runMode == "single_account" {
		query += ` AND union_id = ?`
		args = append(args, unionID)
	}
	query += ` ORDER BY id DESC LIMIT 1`
	item, err := scanScriptRunLog(d.db.QueryRow(query, args...))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return item, err
}

func (d *Database) FindRunningScriptRunLog(pluginID, scriptPath, runMode, unionID string) (*ScriptRunLog, error) {
	query := `
		SELECT id, plugin_id, union_id, script_path, runtime, run_mode, status, output, error, started_at, finished_at, created_at
		FROM script_run_logs
		WHERE plugin_id = ? AND script_path = ? AND run_mode = ? AND status IN ('running', 'pausing')
	`
	args := []interface{}{pluginID, scriptPath, runMode}
	if runMode == "single_account" {
		query += ` AND union_id = ?`
		args = append(args, unionID)
	}
	query += ` ORDER BY id DESC LIMIT 1`
	item, err := scanScriptRunLog(d.db.QueryRow(query, args...))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return item, err
}

func (d *Database) UpdateScriptRunLog(id int64, status, output, errorText string, finishedAt time.Time) error {
	if id <= 0 {
		return fmt.Errorf("脚本任务 ID 无效")
	}
	if finishedAt.IsZero() {
		_, err := d.db.Exec(`UPDATE script_run_logs SET status = ?, output = ?, error = ? WHERE id = ?`, status, output, errorText, id)
		return err
	}
	_, err := d.db.Exec(`UPDATE script_run_logs SET status = ?, output = ?, error = ?, finished_at = ? WHERE id = ?`, status, output, errorText, finishedAt, id)
	return err
}

func (d *Database) ListScriptRunLogs(filter ScriptRunLogFilter) ([]*ScriptRunLog, error) {
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
		SELECT id, plugin_id, union_id, script_path, runtime, run_mode, status, '', error, started_at, finished_at, created_at
		FROM script_run_logs
		WHERE `+where+`
		ORDER BY started_at DESC, finished_at DESC, id DESC
		LIMIT ? OFFSET ?
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]*ScriptRunLog, 0)
	for rows.Next() {
		item, err := scanScriptRunLog(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (d *Database) CountScriptRunLogs(filter ScriptRunLogFilter) (int, error) {
	where, args := filter.buildWhere()
	var total int
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM script_run_logs WHERE `+where, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (d *Database) GetScriptRunLog(id int64) (*ScriptRunLog, error) {
	item, err := scanScriptRunLog(d.db.QueryRow(`
		SELECT id, plugin_id, union_id, script_path, runtime, run_mode, status, output, error, started_at, finished_at, created_at
		FROM script_run_logs
		WHERE id = ?
	`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return item, err
}

func (d *Database) DeleteScriptRunLog(id int64) error {
	_, err := d.db.Exec(`DELETE FROM script_run_logs WHERE id = ?`, id)
	return err
}

func (d *Database) CleanupScriptRunLogs(retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, nil
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	result, err := d.db.Exec(`
		DELETE FROM script_run_logs
		WHERE status NOT IN ('running', 'pausing') AND finished_at < ?
	`, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

type scriptRunLogScanner interface {
	Scan(dest ...interface{}) error
}

func scanScriptRunLog(scanner scriptRunLogScanner) (*ScriptRunLog, error) {
	var item ScriptRunLog
	if err := scanner.Scan(&item.ID, &item.PluginID, &item.UnionID, &item.ScriptPath, &item.Runtime, &item.RunMode, &item.Status, &item.Output, &item.Error, &item.StartedAt, &item.FinishedAt, &item.CreatedAt); err != nil {
		return nil, err
	}
	return &item, nil
}
