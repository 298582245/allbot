package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/allbot/allbot/core/types"
)

type MessageStatPoint struct {
	Name   string `json:"name"`
	Counts []int  `json:"counts"`
	Total  int    `json:"total"`
}

type MessageStatsSummary struct {
	Date       string             `json:"date"`
	Mode       string             `json:"mode"`
	Hours      []int              `json:"hours"`
	ByPlatform []MessageStatPoint `json:"by_platform"`
	ByAdapter  []MessageStatPoint `json:"by_adapter"`
}

type MessageCountSummary struct {
	Total int64 `json:"total"`
	Today int64 `json:"today"`
}

type MessageTotalTrendPoint struct {
	Label   string `json:"label"`
	Total   int64  `json:"total"`
	Private int64  `json:"private"`
	Group   int64  `json:"group"`
}

type MessageTotalTrendSummary struct {
	Granularity   string                   `json:"granularity"`
	Start         string                   `json:"start"`
	End           string                   `json:"end"`
	Labels        []string                 `json:"labels"`
	Totals        []int64                  `json:"totals"`
	PrivateTotals []int64                  `json:"private_totals"`
	GroupTotals   []int64                  `json:"group_totals"`
	Points        []MessageTotalTrendPoint `json:"points"`
}

func (d *Database) RecordMessageStat(msg *types.Message) error {
	if msg == nil {
		return nil
	}
	now := time.Now()
	platform := strings.TrimSpace(msg.Platform)
	if platform == "" {
		platform = "unknown"
	}
	adapterID := strings.TrimSpace(msg.AdapterID)
	adapterName := ""
	if msg.Metadata != nil {
		if adapterID == "" {
			adapterID = strings.TrimSpace(msg.Metadata["adapter_id"])
		}
		adapterName = strings.TrimSpace(msg.Metadata["adapter_remark"])
		if adapterName == "" {
			adapterName = strings.TrimSpace(msg.Metadata["adapter_description"])
		}
	}
	if adapterID == "" {
		adapterID = platform
	}
	if adapterName == "" {
		adapterName = platform + "#" + adapterID
	}
	messageType := normalizeMessageStatType(msg)
	_, err := d.db.Exec(`
		INSERT INTO message_stats (stat_date, stat_hour, platform, adapter_id, adapter_name, message_type, count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(stat_date, stat_hour, platform, adapter_id, message_type)
		DO UPDATE SET count = count + 1, adapter_name = excluded.adapter_name, updated_at = CURRENT_TIMESTAMP
	`, now.Format("2006-01-02"), now.Hour(), platform, adapterID, adapterName, messageType)
	return err
}

func normalizeMessageStatType(msg *types.Message) string {
	if msg == nil {
		return "private"
	}
	if strings.TrimSpace(msg.GroupID) != "" {
		return "group"
	}
	messageType := ""
	if msg.Metadata != nil {
		messageType = strings.ToLower(strings.TrimSpace(msg.Metadata["message_type"]))
	}
	switch messageType {
	case "group", "supergroup", "channel", "guild", "room", "conversation":
		return "group"
	case "private", "c2c", "dms", "direct", "user", "event":
		return "private"
	default:
		return "private"
	}
}

func (d *Database) GetMessageCountSummary() (MessageCountSummary, error) {
	if err := d.normalizeLegacyMessageStats(); err != nil {
		return MessageCountSummary{}, err
	}
	today := time.Now().Format("2006-01-02")
	var summary MessageCountSummary
	if err := d.db.QueryRow(`SELECT COALESCE(SUM(count), 0) FROM message_stats`).Scan(&summary.Total); err != nil {
		return MessageCountSummary{}, err
	}
	if err := d.db.QueryRow(`SELECT COALESCE(SUM(count), 0) FROM message_stats WHERE stat_date = ?`, today).Scan(&summary.Today); err != nil {
		return MessageCountSummary{}, err
	}
	return summary, nil
}

func (d *Database) GetMessageStats(date, mode string) (*MessageStatsSummary, error) {
	if err := d.normalizeLegacyMessageStats(); err != nil {
		return nil, err
	}
	mode = strings.TrimSpace(mode)
	if mode != "total" {
		mode = "date"
	}
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	hours := []int{0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 22}
	summary := &MessageStatsSummary{Date: date, Mode: mode, Hours: hours, ByPlatform: []MessageStatPoint{}, ByAdapter: []MessageStatPoint{}}
	byPlatform, err := d.queryMessageStats(mode, date, "platform", "platform")
	if err != nil {
		return nil, err
	}
	byAdapter, err := d.queryMessageStats(mode, date, "adapter_id", "adapter_name")
	if err != nil {
		return nil, err
	}
	summary.ByPlatform = byPlatform
	summary.ByAdapter = byAdapter
	return summary, nil
}

func (d *Database) GetMessageTotalTrend(granularity, start, end string) (*MessageTotalTrendSummary, error) {
	if err := d.normalizeLegacyMessageStats(); err != nil {
		return nil, err
	}
	granularity = strings.TrimSpace(granularity)
	if granularity == "month" {
		return d.queryMonthlyMessageTotalTrend(start, end)
	}
	return d.queryDailyMessageTotalTrend(start, end)
}

func (d *Database) queryDailyMessageTotalTrend(start, end string) (*MessageTotalTrendSummary, error) {
	endDate, err := parseDateOrDefault(end, time.Now())
	if err != nil {
		return nil, err
	}
	startDate, err := parseDateOrDefault(start, endDate.AddDate(0, 0, -6))
	if err != nil {
		return nil, err
	}
	if startDate.After(endDate) {
		return nil, fmt.Errorf("开始日期不能晚于结束日期")
	}
	if endDate.Sub(startDate).Hours()/24 > 14 {
		return nil, fmt.Errorf("按日统计最多只能选择 15 天")
	}
	labels := make([]string, 0)
	for day := startDate; !day.After(endDate); day = day.AddDate(0, 0, 1) {
		labels = append(labels, day.Format("2006-01-02"))
	}
	return d.queryMessageTotalTrend("day", labels, "stat_date")
}

func (d *Database) queryMonthlyMessageTotalTrend(start, end string) (*MessageTotalTrendSummary, error) {
	endMonth, err := parseMonthOrDefault(end, firstDayOfMonth(time.Now()))
	if err != nil {
		return nil, err
	}
	startMonth, err := parseMonthOrDefault(start, endMonth.AddDate(0, -5, 0))
	if err != nil {
		return nil, err
	}
	if startMonth.After(endMonth) {
		return nil, fmt.Errorf("开始月份不能晚于结束月份")
	}
	labels := make([]string, 0)
	for month := startMonth; !month.After(endMonth); month = month.AddDate(0, 1, 0) {
		labels = append(labels, month.Format("2006-01"))
	}
	if len(labels) > 12 {
		return nil, fmt.Errorf("按月统计最多只能选择 12 个月")
	}
	return d.queryMessageTotalTrend("month", labels, "substr(stat_date, 1, 7)")
}

func (d *Database) queryMessageTotalTrend(granularity string, labels []string, dateExpr string) (*MessageTotalTrendSummary, error) {
	result := &MessageTotalTrendSummary{
		Granularity:   granularity,
		Labels:        labels,
		Totals:        make([]int64, len(labels)),
		PrivateTotals: make([]int64, len(labels)),
		GroupTotals:   make([]int64, len(labels)),
		Points:        make([]MessageTotalTrendPoint, 0, len(labels)),
	}
	if len(labels) == 0 {
		return result, nil
	}
	result.Start = labels[0]
	result.End = labels[len(labels)-1]
	rows, err := d.db.Query(`SELECT `+dateExpr+`, COALESCE(NULLIF(message_type, ''), 'private'), COALESCE(SUM(count), 0) FROM message_stats WHERE `+dateExpr+` BETWEEN ? AND ? GROUP BY `+dateExpr+`, message_type ORDER BY `+dateExpr, result.Start, result.End)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	privateCounts := map[string]int64{}
	groupCounts := map[string]int64{}
	for rows.Next() {
		var label string
		var messageType string
		var total int64
		if err := rows.Scan(&label, &messageType, &total); err != nil {
			return nil, err
		}
		if messageType == "group" {
			groupCounts[label] += total
		} else {
			privateCounts[label] += total
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index, label := range labels {
		privateTotal := privateCounts[label]
		groupTotal := groupCounts[label]
		total := privateTotal + groupTotal
		result.PrivateTotals[index] = privateTotal
		result.GroupTotals[index] = groupTotal
		result.Totals[index] = total
		result.Points = append(result.Points, MessageTotalTrendPoint{Label: label, Total: total, Private: privateTotal, Group: groupTotal})
	}
	return result, nil
}

func parseDateOrDefault(value string, fallback time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return truncateDate(fallback), nil
	}
	date, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("日期格式必须为 YYYY-MM-DD")
	}
	return truncateDate(date), nil
}

func parseMonthOrDefault(value string, fallback time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return firstDayOfMonth(fallback), nil
	}
	month, err := time.ParseInLocation("2006-01", value, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("月份格式必须为 YYYY-MM")
	}
	return firstDayOfMonth(month), nil
}

func truncateDate(date time.Time) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.Local)
}

func firstDayOfMonth(date time.Time) time.Time {
	return time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.Local)
}

func (d *Database) normalizeLegacyMessageStats() error {
	adapters, err := d.GetAllAdapters()
	if err != nil {
		return err
	}
	for _, item := range adapters {
		if item == nil || !item.Enabled {
			continue
		}
		platform := strings.TrimSpace(item.Platform)
		if platform == "" {
			continue
		}
		adapterID := strconv.FormatInt(item.ID, 10)
		adapterName := strings.TrimSpace(item.Remark)
		if adapterName == "" {
			adapterName = strings.TrimSpace(item.Description)
		}
		if adapterName == "" {
			adapterName = platform + "#" + adapterID
		}
		if _, err := d.db.Exec(`
			INSERT INTO message_stats (stat_date, stat_hour, platform, adapter_id, adapter_name, message_type, count, created_at, updated_at)
			SELECT stat_date, stat_hour, platform, ?, ?, message_type, count, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
			FROM message_stats
			WHERE platform = ? AND adapter_id = ?
			ON CONFLICT(stat_date, stat_hour, platform, adapter_id, message_type)
			DO UPDATE SET count = count + excluded.count, adapter_name = excluded.adapter_name, updated_at = CURRENT_TIMESTAMP
		`, adapterID, adapterName, platform, platform); err != nil {
			return err
		}
		if _, err := d.db.Exec(`
			DELETE FROM message_stats
			WHERE platform = ? AND adapter_id = ?
		`, platform, platform); err != nil {
			return err
		}
	}
	return nil
}

func (d *Database) queryMessageStats(mode, date, keyColumn, nameColumn string) ([]MessageStatPoint, error) {
	where := "WHERE stat_date = ?"
	args := []interface{}{date}
	if mode == "total" {
		where = ""
		args = nil
	}
	query := `
		SELECT ` + keyColumn + `, MAX(` + nameColumn + `), stat_hour / 2 AS bucket, SUM(count)
		FROM message_stats
		` + where + `
		GROUP BY ` + keyColumn + `, bucket
		ORDER BY ` + keyColumn + `, bucket
	`
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := map[string]*MessageStatPoint{}
	order := []string{}
	for rows.Next() {
		var key, name string
		var bucket, count int
		if err := rows.Scan(&key, &name, &bucket, &count); err != nil {
			return nil, err
		}
		if key == "" {
			key = "unknown"
		}
		if name == "" {
			name = key
		}
		point, ok := points[key]
		if !ok {
			point = &MessageStatPoint{Name: name, Counts: make([]int, 12)}
			points[key] = point
			order = append(order, key)
		}
		if bucket >= 0 && bucket < len(point.Counts) {
			point.Counts[bucket] += count
			point.Total += count
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := make([]MessageStatPoint, 0, len(order))
	for _, key := range order {
		result = append(result, *points[key])
	}
	return result, nil
}
