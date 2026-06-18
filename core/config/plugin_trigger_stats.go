package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/allbot/allbot/core/types"
)

type PluginTriggerTrendPlugin struct {
	PluginID       string  `json:"plugin_id"`
	PluginName     string  `json:"plugin_name"`
	TriggerPattern string  `json:"trigger_pattern"`
	Total          int64   `json:"total"`
	Counts         []int64 `json:"counts"`
}

type PluginTriggerTrendPoint struct {
	Label string `json:"label"`
	Total int64  `json:"total"`
}

type PluginTriggerTrendSummary struct {
	Granularity string                     `json:"granularity"`
	Start       string                     `json:"start"`
	End         string                     `json:"end"`
	Labels      []string                   `json:"labels"`
	Total       int64                      `json:"total"`
	Plugins     []PluginTriggerTrendPlugin `json:"plugins"`
	Points      []PluginTriggerTrendPoint  `json:"points"`
}

func (d *Database) RecordPluginTriggerStat(plugin *types.Plugin, msg *types.Message) error {
	if d == nil || plugin == nil || msg == nil {
		return nil
	}
	pluginID := strings.TrimSpace(plugin.ID)
	if pluginID == "" {
		return nil
	}
	pluginName := strings.TrimSpace(plugin.Name)
	if pluginName == "" {
		pluginName = pluginID
	}
	triggerPattern := strings.TrimSpace(plugin.Trigger)
	platform, adapterID, adapterName := pluginTriggerAdapterInfo(msg)
	now := time.Now()
	_, err := d.db.Exec(`
		INSERT INTO plugin_trigger_stats (stat_date, stat_hour, plugin_id, plugin_name, trigger_pattern, platform, adapter_id, adapter_name, count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(stat_date, stat_hour, plugin_id, platform, adapter_id)
		DO UPDATE SET count = count + 1, plugin_name = excluded.plugin_name, trigger_pattern = excluded.trigger_pattern, adapter_name = excluded.adapter_name, updated_at = CURRENT_TIMESTAMP
	`, now.Format("2006-01-02"), now.Hour(), pluginID, pluginName, triggerPattern, platform, adapterID, adapterName)
	return err
}

func (d *Database) GetPluginTriggerTrend(granularity, start, end string, limit int) (*PluginTriggerTrendSummary, error) {
	if d == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	limit = normalizePluginTriggerTrendLimit(limit)
	granularity = strings.TrimSpace(granularity)
	if granularity == "month" {
		return d.queryMonthlyPluginTriggerTrend(start, end, limit)
	}
	return d.queryDailyPluginTriggerTrend(start, end, limit)
}

func (d *Database) queryDailyPluginTriggerTrend(start, end string, limit int) (*PluginTriggerTrendSummary, error) {
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
	return d.queryPluginTriggerTrend("day", labels, "stat_date", limit)
}

func (d *Database) queryMonthlyPluginTriggerTrend(start, end string, limit int) (*PluginTriggerTrendSummary, error) {
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
	return d.queryPluginTriggerTrend("month", labels, "substr(stat_date, 1, 7)", limit)
}

func (d *Database) queryPluginTriggerTrend(granularity string, labels []string, dateExpr string, limit int) (*PluginTriggerTrendSummary, error) {
	result := &PluginTriggerTrendSummary{Granularity: granularity, Labels: labels, Plugins: []PluginTriggerTrendPlugin{}, Points: make([]PluginTriggerTrendPoint, 0, len(labels))}
	if len(labels) == 0 {
		return result, nil
	}
	result.Start = labels[0]
	result.End = labels[len(labels)-1]
	labelIndex := make(map[string]int, len(labels))
	for index, label := range labels {
		labelIndex[label] = index
	}
	if err := d.fillPluginTriggerPoints(result, dateExpr); err != nil {
		return nil, err
	}
	plugins, err := d.queryTopPluginTriggers(result.Start, result.End, dateExpr, limit, len(labels))
	if err != nil {
		return nil, err
	}
	if len(plugins) == 0 {
		result.Plugins = plugins
		return result, nil
	}
	pluginMap := make(map[string]*PluginTriggerTrendPlugin, len(plugins))
	for index := range plugins {
		pluginMap[plugins[index].PluginID] = &plugins[index]
	}
	rows, err := d.db.Query(`SELECT plugin_id, `+dateExpr+`, COALESCE(SUM(count), 0) FROM plugin_trigger_stats WHERE `+dateExpr+` BETWEEN ? AND ? GROUP BY plugin_id, `+dateExpr, result.Start, result.End)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var pluginID, label string
		var total int64
		if err := rows.Scan(&pluginID, &label, &total); err != nil {
			return nil, err
		}
		plugin, ok := pluginMap[pluginID]
		index, hasLabel := labelIndex[label]
		if ok && hasLabel {
			plugin.Counts[index] = total
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result.Plugins = plugins
	return result, nil
}

func (d *Database) fillPluginTriggerPoints(result *PluginTriggerTrendSummary, dateExpr string) error {
	rows, err := d.db.Query(`SELECT `+dateExpr+`, COALESCE(SUM(count), 0) FROM plugin_trigger_stats WHERE `+dateExpr+` BETWEEN ? AND ? GROUP BY `+dateExpr+` ORDER BY `+dateExpr, result.Start, result.End)
	if err != nil {
		return err
	}
	defer rows.Close()
	counts := map[string]int64{}
	for rows.Next() {
		var label string
		var total int64
		if err := rows.Scan(&label, &total); err != nil {
			return err
		}
		counts[label] = total
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, label := range result.Labels {
		total := counts[label]
		result.Total += total
		result.Points = append(result.Points, PluginTriggerTrendPoint{Label: label, Total: total})
	}
	return nil
}

func (d *Database) queryTopPluginTriggers(start, end, dateExpr string, limit int, labelCount int) ([]PluginTriggerTrendPlugin, error) {
	rows, err := d.db.Query(`
		SELECT plugin_id, COALESCE(MAX(plugin_name), ''), COALESCE(MAX(trigger_pattern), ''), COALESCE(SUM(count), 0) AS total
		FROM plugin_trigger_stats
		WHERE `+dateExpr+` BETWEEN ? AND ?
		GROUP BY plugin_id
		ORDER BY total DESC, plugin_id ASC
		LIMIT ?
	`, start, end, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	plugins := []PluginTriggerTrendPlugin{}
	for rows.Next() {
		item := PluginTriggerTrendPlugin{Counts: make([]int64, labelCount)}
		if err := rows.Scan(&item.PluginID, &item.PluginName, &item.TriggerPattern, &item.Total); err != nil {
			return nil, err
		}
		if item.PluginName == "" {
			item.PluginName = item.PluginID
		}
		plugins = append(plugins, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return plugins, nil
}

func pluginTriggerAdapterInfo(msg *types.Message) (string, string, string) {
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
	return platform, adapterID, adapterName
}

func normalizePluginTriggerTrendLimit(limit int) int {
	if limit <= 0 {
		return 8
	}
	if limit > 12 {
		return 12
	}
	return limit
}
