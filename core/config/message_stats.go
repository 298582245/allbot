package config

import (
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
	_, err := d.db.Exec(`
		INSERT INTO message_stats (stat_date, stat_hour, platform, adapter_id, adapter_name, count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(stat_date, stat_hour, platform, adapter_id)
		DO UPDATE SET count = count + 1, adapter_name = excluded.adapter_name, updated_at = CURRENT_TIMESTAMP
	`, now.Format("2006-01-02"), now.Hour(), platform, adapterID, adapterName)
	return err
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
			INSERT INTO message_stats (stat_date, stat_hour, platform, adapter_id, adapter_name, count, created_at, updated_at)
			SELECT stat_date, stat_hour, platform, ?, ?, count, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
			FROM message_stats
			WHERE platform = ? AND adapter_id = ?
			ON CONFLICT(stat_date, stat_hour, platform, adapter_id)
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
