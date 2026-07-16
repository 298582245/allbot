package config

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	openAPISettingsKey         = "open_api.config"
	openAPISettingsDescription = "开放接口全局配置"
	defaultOpenAPIRetention    = 30
	defaultOpenAPICleanupBatch = 500
)

const (
	OpenAPICallOutcomeSuccess     = "success"
	OpenAPICallOutcomeIPDenied    = "ip_denied"
	OpenAPICallOutcomeTokenDenied = "token_denied"
	OpenAPICallOutcomeFailed      = "failed"
)

// OpenAPISettings 仅保存开放接口的全局配置，避免与通用系统设置互相覆盖。
type OpenAPISettings struct {
	IPWhitelist    []string `json:"ip_whitelist"`
	TrustedProxies []string `json:"trusted_proxies"`
	RetentionDays  int      `json:"retention_days"`
}

type OpenAPICallStat struct {
	EndpointID     string     `json:"endpoint_id"`
	Total          int64      `json:"total"`
	Success        int64      `json:"success"`
	Rejected       int64      `json:"rejected"`
	Failed         int64      `json:"failed"`
	LastStatusCode int        `json:"last_status_code"`
	LastOutcome    string     `json:"last_outcome"`
	LastCalledAt   *time.Time `json:"last_called_at,omitempty"`
}

type OpenAPICallStatDelta struct {
	EndpointID     string
	Total          int64
	Success        int64
	Rejected       int64
	Failed         int64
	LastStatusCode int
	LastOutcome    string
	LastCalledAt   time.Time
}

type OpenAPICallLog struct {
	ID           int64     `json:"id"`
	EndpointID   string    `json:"endpoint_id"`
	EndpointName string    `json:"endpoint_name"`
	Method       string    `json:"method"`
	RequestPath  string    `json:"request_path"`
	ClientIP     string    `json:"client_ip"`
	StatusCode   int       `json:"status_code"`
	Outcome      string    `json:"outcome"`
	DurationMS   int64     `json:"duration_ms"`
	StartedAt    time.Time `json:"started_at"`
}

type OpenAPICallLogFilter struct {
	EndpointID  string
	Outcome     string
	ClientIP    string
	StatusCode  int
	StartedFrom *time.Time
	StartedTo   *time.Time
	Limit       int
	Offset      int
}

func DefaultOpenAPISettings() OpenAPISettings {
	return OpenAPISettings{
		IPWhitelist:    []string{"*"},
		TrustedProxies: []string{"127.0.0.1/32", "::1/128"},
		RetentionDays:  defaultOpenAPIRetention,
	}
}

// GetOpenAPISettings 对缺失配置和旧版缺失字段补齐兼容默认值；规则合法性由 Web 校验层负责。
func (d *Database) GetOpenAPISettings() (OpenAPISettings, error) {
	settings := DefaultOpenAPISettings()
	value, err := d.GetSetting(openAPISettingsKey)
	if err == sql.ErrNoRows || strings.TrimSpace(value) == "" {
		return settings, nil
	}
	if err != nil {
		return settings, err
	}
	data := []byte(value)
	if err := json.Unmarshal(data, &settings); err != nil {
		return DefaultOpenAPISettings(), fmt.Errorf("解析开放接口全局配置失败: %w", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err == nil {
		defaults := DefaultOpenAPISettings()
		if _, ok := raw["ip_whitelist"]; !ok {
			settings.IPWhitelist = defaults.IPWhitelist
		}
		if _, ok := raw["trusted_proxies"]; !ok {
			settings.TrustedProxies = defaults.TrustedProxies
		}
		if _, ok := raw["retention_days"]; !ok {
			settings.RetentionDays = defaults.RetentionDays
		}
	}
	return settings, nil
}

// SaveOpenAPISettings 按原值持久化，边界校验和地址规范化由调用方在保存前完成。
func (d *Database) SaveOpenAPISettings(settings OpenAPISettings) error {
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return d.SetSetting(openAPISettingsKey, string(data), openAPISettingsDescription)
}

// WriteOpenAPICallBatch 在一个事务内同时写入累计统计和明细，任一步失败都不会产生半批数据。
func (d *Database) WriteOpenAPICallBatch(stats []OpenAPICallStatDelta, logs []OpenAPICallLog) error {
	if len(stats) == 0 && len(logs) == 0 {
		return nil
	}
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, stat := range stats {
		if err := upsertOpenAPICallStat(tx, stat); err != nil {
			return err
		}
	}
	for _, item := range logs {
		if err := insertOpenAPICallLog(tx, item); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func upsertOpenAPICallStat(tx *sql.Tx, stat OpenAPICallStatDelta) error {
	endpointID := strings.TrimSpace(stat.EndpointID)
	if endpointID == "" {
		return fmt.Errorf("开放接口 ID 不能为空")
	}
	if stat.Total < 0 || stat.Success < 0 || stat.Rejected < 0 || stat.Failed < 0 {
		return fmt.Errorf("开放接口统计增量不能为负数")
	}
	if stat.Total == 0 && stat.Success == 0 && stat.Rejected == 0 && stat.Failed == 0 {
		return nil
	}
	if stat.Total != stat.Success+stat.Rejected+stat.Failed {
		return fmt.Errorf("开放接口统计总数必须等于成功、拒绝和失败数量之和")
	}
	calledAt := stat.LastCalledAt
	if calledAt.IsZero() {
		calledAt = time.Now()
	}
	_, err := tx.Exec(`
		INSERT INTO open_api_call_stats (
			endpoint_id, total, success, rejected, failed, last_status_code,
			last_outcome, last_called_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(endpoint_id) DO UPDATE SET
			total = open_api_call_stats.total + excluded.total,
			success = open_api_call_stats.success + excluded.success,
			rejected = open_api_call_stats.rejected + excluded.rejected,
			failed = open_api_call_stats.failed + excluded.failed,
			last_status_code = CASE WHEN excluded.last_called_at >= open_api_call_stats.last_called_at OR open_api_call_stats.last_called_at IS NULL THEN excluded.last_status_code ELSE open_api_call_stats.last_status_code END,
			last_outcome = CASE WHEN excluded.last_called_at >= open_api_call_stats.last_called_at OR open_api_call_stats.last_called_at IS NULL THEN excluded.last_outcome ELSE open_api_call_stats.last_outcome END,
			last_called_at = CASE WHEN open_api_call_stats.last_called_at IS NULL OR excluded.last_called_at >= open_api_call_stats.last_called_at THEN excluded.last_called_at ELSE open_api_call_stats.last_called_at END,
			updated_at = CURRENT_TIMESTAMP
	`, endpointID, stat.Total, stat.Success, stat.Rejected, stat.Failed, stat.LastStatusCode, strings.TrimSpace(stat.LastOutcome), calledAt)
	return err
}

func insertOpenAPICallLog(tx *sql.Tx, item OpenAPICallLog) error {
	endpointID := strings.TrimSpace(item.EndpointID)
	if endpointID == "" {
		return fmt.Errorf("开放接口 ID 不能为空")
	}
	startedAt := item.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	durationMS := item.DurationMS
	if durationMS < 0 {
		durationMS = 0
	}
	_, err := tx.Exec(`
		INSERT INTO open_api_call_logs (
			endpoint_id, endpoint_name, method, request_path, client_ip,
			status_code, outcome, duration_ms, started_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, endpointID, strings.TrimSpace(item.EndpointName), strings.ToUpper(strings.TrimSpace(item.Method)), item.RequestPath, strings.TrimSpace(item.ClientIP), item.StatusCode, strings.TrimSpace(item.Outcome), durationMS, startedAt)
	return err
}

// GetOpenAPICallStats 批量读取指定接口的累计统计；空 ID 列表表示读取全部。
func (d *Database) GetOpenAPICallStats(endpointIDs []string) (map[string]OpenAPICallStat, error) {
	ids := normalizeOpenAPIEndpointIDs(endpointIDs)
	query := `SELECT endpoint_id, total, success, rejected, failed, last_status_code, last_outcome, last_called_at FROM open_api_call_stats`
	args := make([]interface{}, 0, len(ids))
	if len(ids) > 0 {
		placeholders := make([]string, len(ids))
		for index, id := range ids {
			placeholders[index] = "?"
			args = append(args, id)
		}
		query += ` WHERE endpoint_id IN (` + strings.Join(placeholders, ",") + `)`
	}
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]OpenAPICallStat, len(ids))
	for rows.Next() {
		var item OpenAPICallStat
		var calledAt sql.NullTime
		if err := rows.Scan(&item.EndpointID, &item.Total, &item.Success, &item.Rejected, &item.Failed, &item.LastStatusCode, &item.LastOutcome, &calledAt); err != nil {
			return nil, err
		}
		if calledAt.Valid {
			value := calledAt.Time
			item.LastCalledAt = &value
		}
		result[item.EndpointID] = item
	}
	return result, rows.Err()
}

func normalizeOpenAPIEndpointIDs(endpointIDs []string) []string {
	result := make([]string, 0, len(endpointIDs))
	seen := make(map[string]struct{}, len(endpointIDs))
	for _, item := range endpointIDs {
		id := strings.TrimSpace(item)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func (filter OpenAPICallLogFilter) buildWhere() (string, []interface{}) {
	clauses := make([]string, 0, 6)
	args := make([]interface{}, 0, 6)
	if endpointID := strings.TrimSpace(filter.EndpointID); endpointID != "" {
		clauses = append(clauses, "endpoint_id = ?")
		args = append(args, endpointID)
	}
	if outcome := strings.TrimSpace(filter.Outcome); outcome != "" {
		clauses = append(clauses, "outcome = ?")
		args = append(args, outcome)
	}
	if clientIP := strings.TrimSpace(filter.ClientIP); clientIP != "" {
		clauses = append(clauses, "client_ip = ?")
		args = append(args, clientIP)
	}
	if filter.StatusCode > 0 {
		clauses = append(clauses, "status_code = ?")
		args = append(args, filter.StatusCode)
	}
	if filter.StartedFrom != nil && !filter.StartedFrom.IsZero() {
		clauses = append(clauses, "started_at >= ?")
		args = append(args, *filter.StartedFrom)
	}
	if filter.StartedTo != nil && !filter.StartedTo.IsZero() {
		clauses = append(clauses, "started_at <= ?")
		args = append(args, *filter.StartedTo)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

// ListOpenAPICallLogs 返回筛选后的服务端分页结果和筛选总数。
func (d *Database) ListOpenAPICallLogs(filter OpenAPICallLogFilter) ([]OpenAPICallLog, int64, error) {
	where, args := filter.buildWhere()
	var total int64
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM open_api_call_logs`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	} else if limit > 200 {
		limit = 200
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	rows, err := d.db.Query(`
		SELECT id, endpoint_id, endpoint_name, method, request_path, client_ip,
			status_code, outcome, duration_ms, started_at
		FROM open_api_call_logs`+where+`
		ORDER BY started_at DESC, id DESC LIMIT ? OFFSET ?
	`, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]OpenAPICallLog, 0)
	for rows.Next() {
		var item OpenAPICallLog
		if err := rows.Scan(&item.ID, &item.EndpointID, &item.EndpointName, &item.Method, &item.RequestPath, &item.ClientIP, &item.StatusCode, &item.Outcome, &item.DurationMS, &item.StartedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

// CleanupOpenAPICallLogsBatch 每次只删除固定小批明细，避免长事务占用 SQLite 唯一连接。
func (d *Database) CleanupOpenAPICallLogsBatch(retentionDays, batchSize int, now time.Time) (int64, error) {
	if retentionDays <= 0 {
		return 0, nil
	}
	if batchSize <= 0 {
		batchSize = defaultOpenAPICleanupBatch
	} else if batchSize > 5000 {
		batchSize = 5000
	}
	if now.IsZero() {
		now = time.Now()
	}
	cutoff := now.AddDate(0, 0, -retentionDays)
	result, err := d.db.Exec(`
		DELETE FROM open_api_call_logs
		WHERE id IN (
			SELECT id FROM open_api_call_logs
			WHERE started_at < ?
			ORDER BY started_at ASC, id ASC
			LIMIT ?
		)
	`, cutoff, batchSize)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// DeleteOpenAPICallData 删除接口明细及累计总数，供永久删除接口时调用。
func (d *Database) DeleteOpenAPICallData(endpointID string) error {
	endpointID = strings.TrimSpace(endpointID)
	if endpointID == "" {
		return fmt.Errorf("开放接口 ID 不能为空")
	}
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM open_api_call_logs WHERE endpoint_id = ?`, endpointID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM open_api_call_stats WHERE endpoint_id = ?`, endpointID); err != nil {
		return err
	}
	return tx.Commit()
}
