package config

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type ScriptRunLog struct {
	ID             int64     `json:"id"`
	PluginID       string    `json:"plugin_id"`
	UnionID        string    `json:"union_id"`
	ScriptPath     string    `json:"script_path"`
	Runtime        string    `json:"runtime"`
	RuntimeProfile string    `json:"runtime_profile"`
	RunMode        string    `json:"run_mode"`
	Status         string    `json:"status"`
	RunTotal       int64     `json:"run_total"`
	FailedTotal    int64     `json:"failed_total"`
	Output         string    `json:"output"`
	Error          string    `json:"error"`
	StartedAt      time.Time `json:"started_at"`
	FinishedAt     time.Time `json:"finished_at"`
	CreatedAt      time.Time `json:"created_at"`
}

type ScriptRunLogFilter struct {
	Keyword        string
	UnionID        string
	PluginID       string
	ScriptPath     string
	RuntimeProfile string
	RunMode        string
	Status         string
	Limit          int
	Offset         int
}

type ScriptRunStatsSummary struct {
	Total   int64 `json:"total"`
	Today   int64 `json:"today"`
	Running int64 `json:"running"`
	Pausing int64 `json:"pausing"`
	Queued  int64 `json:"queued"`
	Success int64 `json:"success"`
	Failed  int64 `json:"failed"`
}

const ScriptRunStatusQueued = "queued"

func (filter ScriptRunLogFilter) buildWhere() (string, []interface{}) {
	where := []string{"1 = 1"}
	args := make([]interface{}, 0)
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		where = append(where, `(union_id LIKE ? OR plugin_id LIKE ? OR script_path LIKE ? OR runtime_profile LIKE ? OR run_mode LIKE ? OR status LIKE ?)`)
		args = append(args, like, like, like, like, like, like)
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
	if runtimeProfile := strings.TrimSpace(filter.RuntimeProfile); runtimeProfile != "" {
		where = append(where, `runtime_profile = ?`)
		args = append(args, runtimeProfile)
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
	item.RuntimeProfile = strings.TrimSpace(item.RuntimeProfile)
	tx, err := d.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`
		INSERT INTO script_run_logs (plugin_id, union_id, script_path, runtime, runtime_profile, run_mode, status, run_total, failed_total, output, error, started_at, finished_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.PluginID, item.UnionID, item.ScriptPath, item.Runtime, item.RuntimeProfile, item.RunMode, item.Status, 1, 0, item.Output, item.Error, item.StartedAt, item.FinishedAt, time.Now())
	if err != nil {
		return 0, err
	}
	if err := d.addScriptRunStats(tx, item, 1, 0, 0); err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func (d *Database) UpsertScriptRunLog(item ScriptRunLog) (int64, bool, error) {
	item.RuntimeProfile = strings.TrimSpace(item.RuntimeProfile)
	existing, err := d.FindLatestScriptRunLog(item.PluginID, item.ScriptPath, item.RunMode, item.UnionID, item.RuntimeProfile)
	if err != nil {
		return 0, false, err
	}
	if existing == nil {
		id, err := d.SaveScriptRunLog(item)
		return id, false, err
	}
	tx, err := d.db.Begin()
	if err != nil {
		return 0, true, err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`
		UPDATE script_run_logs
		SET union_id = ?, runtime = ?, runtime_profile = ?, status = ?, run_total = COALESCE(run_total, 0) + 1, output = '', error = '', started_at = ?, finished_at = ?, created_at = ?
		WHERE id = ?
	`, item.UnionID, item.Runtime, item.RuntimeProfile, item.Status, item.StartedAt, item.FinishedAt, time.Now(), existing.ID)
	if err != nil {
		return 0, true, err
	}
	if err := d.addScriptRunStats(tx, item, 1, 0, 0); err != nil {
		return 0, true, err
	}
	if err := tx.Commit(); err != nil {
		return 0, true, err
	}
	return existing.ID, true, nil
}

func (d *Database) FindLatestScriptRunLog(pluginID, scriptPath, runMode, unionID, runtimeProfile string) (*ScriptRunLog, error) {
	runtimeProfile = strings.TrimSpace(runtimeProfile)
	item, err := scanScriptRunLog(d.db.QueryRow(`
		SELECT id, plugin_id, union_id, script_path, runtime, runtime_profile, run_mode, status, run_total, failed_total, output, error, started_at, finished_at, created_at
		FROM script_run_logs
		WHERE plugin_id = ? AND script_path = ? AND run_mode = ? AND union_id = ? AND runtime_profile = ?
		ORDER BY id DESC LIMIT 1
	`, pluginID, scriptPath, runMode, unionID, runtimeProfile))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return item, err
}

func (d *Database) FindRunningScriptRunLog(pluginID, scriptPath, runMode, unionID, runtimeProfile string) (*ScriptRunLog, error) {
	runtimeProfile = strings.TrimSpace(runtimeProfile)
	item, err := scanScriptRunLog(d.db.QueryRow(`
		SELECT id, plugin_id, union_id, script_path, runtime, runtime_profile, run_mode, status, run_total, failed_total, output, error, started_at, finished_at, created_at
		FROM script_run_logs
		WHERE plugin_id = ? AND script_path = ? AND run_mode = ? AND union_id = ? AND runtime_profile = ? AND status IN ('running', 'pausing')
		ORDER BY id DESC LIMIT 1
	`, pluginID, scriptPath, runMode, unionID, runtimeProfile))
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
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	oldItem, err := scanScriptRunLog(tx.QueryRow(`
		SELECT id, plugin_id, union_id, script_path, runtime, runtime_profile, run_mode, status, run_total, failed_total, output, error, started_at, finished_at, created_at
		FROM script_run_logs
		WHERE id = ?
	`, id))
	if err != nil {
		return err
	}
	failedIncrement := 0
	if !isScriptRunTerminalStatus(oldItem.Status) && (status == "failed" || status == "error") {
		failedIncrement = 1
	}
	_, err = tx.Exec(`UPDATE script_run_logs SET status = ?, output = ?, error = ?, finished_at = ?, failed_total = COALESCE(failed_total, 0) + ? WHERE id = ?`, status, output, errorText, finishedAt, failedIncrement, id)
	if err != nil {
		return err
	}
	if !isScriptRunTerminalStatus(oldItem.Status) {
		if status == "success" {
			if err := d.addScriptRunStats(tx, *oldItem, 0, 1, 0); err != nil {
				return err
			}
		} else if status == "failed" || status == "error" {
			if err := d.addScriptRunStats(tx, *oldItem, 0, 0, 1); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
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
		SELECT id, plugin_id, union_id, script_path, runtime, runtime_profile, run_mode, status, run_total, failed_total, '', error, started_at, finished_at, created_at
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

func (d *Database) GetScriptRunStatsSummary() (ScriptRunStatsSummary, error) {
	var summary ScriptRunStatsSummary
	today := time.Now().Format("2006-01-02")
	if err := d.db.QueryRow(`
		SELECT
			COALESCE(SUM(run_total), 0),
			COALESCE(SUM(CASE WHEN stat_date = ? THEN run_total ELSE 0 END), 0),
			COALESCE(SUM(success_total), 0),
			COALESCE(SUM(failed_total), 0)
		FROM script_run_stats
	`, today).Scan(&summary.Total, &summary.Today, &summary.Success, &summary.Failed); err != nil {
		return summary, err
	}
	err := d.db.QueryRow(`
		SELECT
			COALESCE(SUM(CASE WHEN status = 'running' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'pausing' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0)
		FROM script_run_logs
	`, ScriptRunStatusQueued).Scan(&summary.Running, &summary.Pausing, &summary.Queued)
	return summary, err
}

func (d *Database) ListQueuedScriptRunLogs(limit int) ([]*ScriptRunLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := d.db.Query(`
		SELECT id, plugin_id, union_id, script_path, runtime, runtime_profile, run_mode, status, run_total, failed_total, output, error, started_at, finished_at, created_at
		FROM script_run_logs
		WHERE status = ?
		ORDER BY created_at ASC, id ASC
		LIMIT ?
	`, ScriptRunStatusQueued, limit)
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

func (d *Database) GetScriptRunLog(id int64) (*ScriptRunLog, error) {
	item, err := scanScriptRunLog(d.db.QueryRow(`
		SELECT id, plugin_id, union_id, script_path, runtime, runtime_profile, run_mode, status, run_total, failed_total, output, error, started_at, finished_at, created_at
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
		WHERE status NOT IN ('running', 'pausing', ?) AND finished_at < ?
	`, ScriptRunStatusQueued, cutoff)
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
	if err := scanner.Scan(&item.ID, &item.PluginID, &item.UnionID, &item.ScriptPath, &item.Runtime, &item.RuntimeProfile, &item.RunMode, &item.Status, &item.RunTotal, &item.FailedTotal, &item.Output, &item.Error, &item.StartedAt, &item.FinishedAt, &item.CreatedAt); err != nil {
		return nil, err
	}
	return &item, nil
}
