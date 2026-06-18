package config

import (
	"testing"

	"github.com/allbot/allbot/core/types"
)

func TestGetMessageCountSummary(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()

	if err := db.RecordMessageStat(&types.Message{Platform: "qq", AdapterID: "bot-1"}); err != nil {
		t.Fatalf("RecordMessageStat returned error: %v", err)
	}
	if err := db.RecordMessageStat(&types.Message{Platform: "telegram", AdapterID: "bot-2"}); err != nil {
		t.Fatalf("RecordMessageStat returned error: %v", err)
	}
	if _, err := db.db.Exec(`
		INSERT INTO message_stats (stat_date, stat_hour, platform, adapter_id, adapter_name, count)
		VALUES ('2000-01-01', 0, 'qq', 'old-bot', 'old', 5)
	`); err != nil {
		t.Fatalf("insert legacy stats returned error: %v", err)
	}

	summary, err := db.GetMessageCountSummary()
	if err != nil {
		t.Fatalf("GetMessageCountSummary returned error: %v", err)
	}
	if summary.Total != 7 {
		t.Fatalf("total = %d, expected 7", summary.Total)
	}
	if summary.Today != 2 {
		t.Fatalf("today = %d, expected 2", summary.Today)
	}
}

func TestGetMessageTotalTrendByDay(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()
	if _, err := db.db.Exec(`
		INSERT INTO message_stats (stat_date, stat_hour, platform, adapter_id, adapter_name, count)
		VALUES
		('2026-06-16', 8, 'qq', 'bot-1', '机器人1', 2),
		('2026-06-17', 10, 'qq', 'bot-1', '机器人1', 3),
		('2026-06-17', 12, 'telegram', 'bot-2', '机器人2', 4)
	`); err != nil {
		t.Fatalf("insert message stats returned error: %v", err)
	}

	trend, err := db.GetMessageTotalTrend("day", "2026-06-16", "2026-06-18")
	if err != nil {
		t.Fatalf("GetMessageTotalTrend returned error: %v", err)
	}
	if len(trend.Totals) != 3 || trend.Totals[0] != 2 || trend.Totals[1] != 7 || trend.Totals[2] != 0 {
		t.Fatalf("unexpected day totals: %+v", trend)
	}
}

func TestGetMessageTotalTrendByMonth(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()
	if _, err := db.db.Exec(`
		INSERT INTO message_stats (stat_date, stat_hour, platform, adapter_id, adapter_name, count)
		VALUES
		('2026-05-16', 8, 'qq', 'bot-1', '机器人1', 2),
		('2026-06-17', 10, 'qq', 'bot-1', '机器人1', 3)
	`); err != nil {
		t.Fatalf("insert message stats returned error: %v", err)
	}

	trend, err := db.GetMessageTotalTrend("month", "2026-05", "2026-06")
	if err != nil {
		t.Fatalf("GetMessageTotalTrend returned error: %v", err)
	}
	if len(trend.Totals) != 2 || trend.Totals[0] != 2 || trend.Totals[1] != 3 {
		t.Fatalf("unexpected month totals: %+v", trend)
	}
}

func TestGetMessageTotalTrendRejectsLongRanges(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()
	if _, err := db.GetMessageTotalTrend("day", "2026-06-01", "2026-06-30"); err == nil {
		t.Fatal("expected day range error")
	}
	if _, err := db.GetMessageTotalTrend("month", "2025-01", "2026-06"); err == nil {
		t.Fatal("expected month range error")
	}
}
