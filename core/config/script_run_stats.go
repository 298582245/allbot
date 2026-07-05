package config

import (
	"database/sql"
	"time"
)

type scriptRunStatsDelta struct {
	PluginID       string
	ScriptPath     string
	Runtime        string
	RuntimeProfile string
	RunMode        string
	StartedAt      time.Time
	RunTotal       int64
	SuccessTotal   int64
	FailedTotal    int64
}

func (d *Database) addScriptRunStats(tx *sql.Tx, item ScriptRunLog, runTotal, successTotal, failedTotal int64) error {
	return addScriptRunStats(tx, scriptRunStatsDelta{
		PluginID:       item.PluginID,
		ScriptPath:     item.ScriptPath,
		Runtime:        item.Runtime,
		RuntimeProfile: item.RuntimeProfile,
		RunMode:        item.RunMode,
		StartedAt:      item.StartedAt,
		RunTotal:       runTotal,
		SuccessTotal:   successTotal,
		FailedTotal:    failedTotal,
	})
}

func addScriptRunStats(tx *sql.Tx, delta scriptRunStatsDelta) error {
	if delta.RunTotal == 0 && delta.SuccessTotal == 0 && delta.FailedTotal == 0 {
		return nil
	}
	statDate, statHour := scriptRunStatBucket(delta.StartedAt)
	_, err := tx.Exec(`
		INSERT INTO script_run_stats (stat_date, stat_hour, plugin_id, script_path, runtime, runtime_profile, run_mode, run_total, success_total, failed_total, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(stat_date, stat_hour, plugin_id, script_path, runtime, runtime_profile, run_mode) DO UPDATE SET
			run_total = script_run_stats.run_total + excluded.run_total,
			success_total = script_run_stats.success_total + excluded.success_total,
			failed_total = script_run_stats.failed_total + excluded.failed_total,
			updated_at = CURRENT_TIMESTAMP
	`, statDate, statHour, delta.PluginID, delta.ScriptPath, delta.Runtime, delta.RuntimeProfile, delta.RunMode, delta.RunTotal, delta.SuccessTotal, delta.FailedTotal)
	return err
}

func scriptRunStatBucket(startedAt time.Time) (string, int) {
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	local := startedAt.Local()
	return local.Format("2006-01-02"), local.Hour()
}

func backfillScriptRunStats(db *sql.DB) error {
	var count int64
	if err := db.QueryRow(`SELECT COUNT(*) FROM script_run_stats`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := db.Exec(`
		INSERT INTO script_run_stats (stat_date, stat_hour, plugin_id, script_path, runtime, runtime_profile, run_mode, run_total, success_total, failed_total, created_at, updated_at)
		SELECT
			substr(started_at, 1, 10),
			CAST(substr(started_at, 12, 2) AS INTEGER),
			plugin_id,
			COALESCE(script_path, ''),
			COALESCE(runtime, ''),
			COALESCE(runtime_profile, ''),
			COALESCE(run_mode, ''),
			SUM(CASE WHEN COALESCE(run_total, 0) > 0 THEN run_total ELSE 1 END),
			SUM(CASE
				WHEN (CASE WHEN COALESCE(run_total, 0) > 0 THEN run_total ELSE 1 END) > COALESCE(failed_total, 0) + CASE WHEN status IN ('running', 'pausing', 'paused') THEN 1 ELSE 0 END
				THEN (CASE WHEN COALESCE(run_total, 0) > 0 THEN run_total ELSE 1 END) - COALESCE(failed_total, 0) - CASE WHEN status IN ('running', 'pausing', 'paused') THEN 1 ELSE 0 END
				ELSE 0
			END),
			SUM(COALESCE(failed_total, 0)),
			CURRENT_TIMESTAMP,
			CURRENT_TIMESTAMP
		FROM script_run_logs
		GROUP BY substr(started_at, 1, 10), CAST(substr(started_at, 12, 2) AS INTEGER), plugin_id, COALESCE(script_path, ''), COALESCE(runtime, ''), COALESCE(runtime_profile, ''), COALESCE(run_mode, '')
	`)
	return err
}

func isScriptRunTerminalStatus(status string) bool {
	return status == "success" || status == "failed" || status == "error" || status == "paused"
}
