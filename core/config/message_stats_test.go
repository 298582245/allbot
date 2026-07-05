package config

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

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

func TestGetMessageTotalTrendByDaySplitsPrivateAndGroup(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()
	if _, err := db.db.Exec(`
		INSERT INTO message_stats (stat_date, stat_hour, platform, adapter_id, adapter_name, message_type, count)
		VALUES
		('2026-06-16', 8, 'qq', 'bot-1', '机器人1', 'private', 2),
		('2026-06-17', 10, 'qq', 'bot-1', '机器人1', 'private', 3),
		('2026-06-17', 12, 'telegram', 'bot-2', '机器人2', 'group', 4)
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
	if trend.PrivateTotals[0] != 2 || trend.PrivateTotals[1] != 3 || trend.PrivateTotals[2] != 0 {
		t.Fatalf("unexpected private totals: %+v", trend.PrivateTotals)
	}
	if trend.GroupTotals[0] != 0 || trend.GroupTotals[1] != 4 || trend.GroupTotals[2] != 0 {
		t.Fatalf("unexpected group totals: %+v", trend.GroupTotals)
	}
	if trend.Points[1].Total != 7 || trend.Points[1].Private != 3 || trend.Points[1].Group != 4 {
		t.Fatalf("unexpected point split: %+v", trend.Points[1])
	}
}

func TestRecordMessageStatSplitsPrivateAndGroup(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()

	if err := db.RecordMessageStat(&types.Message{Platform: "qq", AdapterID: "bot-1"}); err != nil {
		t.Fatalf("RecordMessageStat private returned error: %v", err)
	}
	if err := db.RecordMessageStat(&types.Message{Platform: "qq", AdapterID: "bot-1", GroupID: "group-1"}); err != nil {
		t.Fatalf("RecordMessageStat group returned error: %v", err)
	}
	today := time.Now().Format("2006-01-02")
	trend, err := db.GetMessageTotalTrend("day", today, today)
	if err != nil {
		t.Fatalf("GetMessageTotalTrend returned error: %v", err)
	}
	if trend.Totals[0] != 2 || trend.PrivateTotals[0] != 1 || trend.GroupTotals[0] != 1 {
		t.Fatalf("unexpected recorded split: %+v", trend)
	}
}

func TestNormalizeMessageStatType(t *testing.T) {
	cases := []struct {
		name string
		msg  *types.Message
		want string
	}{
		{name: "group id", msg: &types.Message{GroupID: "g"}, want: "group"},
		{name: "group metadata", msg: &types.Message{Metadata: map[string]string{"message_type": "group"}}, want: "group"},
		{name: "channel metadata", msg: &types.Message{Metadata: map[string]string{"message_type": "channel"}}, want: "group"},
		{name: "c2c metadata", msg: &types.Message{Metadata: map[string]string{"message_type": "c2c"}}, want: "private"},
		{name: "dms metadata", msg: &types.Message{Metadata: map[string]string{"message_type": "dms"}}, want: "private"},
		{name: "private metadata", msg: &types.Message{Metadata: map[string]string{"message_type": "private"}}, want: "private"},
		{name: "default", msg: &types.Message{}, want: "private"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeMessageStatType(tc.msg); got != tc.want {
				t.Fatalf("normalizeMessageStatType() = %q, expected %q", got, tc.want)
			}
		})
	}
}

func TestGetMessageTotalTrendLegacyInsertDefaultsToPrivate(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()
	if _, err := db.db.Exec(`
		INSERT INTO message_stats (stat_date, stat_hour, platform, adapter_id, adapter_name, count)
		VALUES ('2026-06-16', 8, 'qq', 'bot-1', '机器人1', 5)
	`); err != nil {
		t.Fatalf("insert legacy message stats returned error: %v", err)
	}

	trend, err := db.GetMessageTotalTrend("day", "2026-06-16", "2026-06-16")
	if err != nil {
		t.Fatalf("GetMessageTotalTrend returned error: %v", err)
	}
	if trend.Totals[0] != 5 || trend.PrivateTotals[0] != 5 || trend.GroupTotals[0] != 0 {
		t.Fatalf("unexpected legacy split: %+v", trend)
	}
}

func TestGetMessageTotalTrendByMonthSplitsPrivateAndGroup(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()
	if _, err := db.db.Exec(`
		INSERT INTO message_stats (stat_date, stat_hour, platform, adapter_id, adapter_name, message_type, count)
		VALUES
		('2026-05-16', 8, 'qq', 'bot-1', '机器人1', 'private', 2),
		('2026-06-17', 10, 'qq', 'bot-1', '机器人1', 'group', 3)
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
	if trend.PrivateTotals[0] != 2 || trend.PrivateTotals[1] != 0 || trend.GroupTotals[0] != 0 || trend.GroupTotals[1] != 3 {
		t.Fatalf("unexpected month split: %+v", trend)
	}
}

func TestMigrateMessageStatsTableRebuildsLegacySchema(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	defer sqlDB.Close()
	if _, err := sqlDB.Exec(`
		CREATE TABLE message_stats (
			stat_date TEXT NOT NULL,
			stat_hour INTEGER NOT NULL,
			platform TEXT NOT NULL,
			adapter_id TEXT NOT NULL DEFAULT '',
			adapter_name TEXT NOT NULL DEFAULT '',
			count INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (stat_date, stat_hour, platform, adapter_id)
		);
		INSERT INTO message_stats (stat_date, stat_hour, platform, adapter_id, adapter_name, count)
		VALUES ('2026-06-16', 8, 'qq', 'bot-1', '机器人1', 5);
	`); err != nil {
		t.Fatalf("create legacy message_stats returned error: %v", err)
	}
	if err := createTables(sqlDB); err != nil {
		t.Fatalf("createTables returned error: %v", err)
	}

	columns, err := tableColumns(sqlDB, "message_stats")
	if err != nil {
		t.Fatalf("tableColumns returned error: %v", err)
	}
	if !columns["message_type"] {
		t.Fatal("message_type column was not created")
	}
	if !messageStatsPrimaryKeyIncludesType(sqlDB) {
		t.Fatal("message_type was not included in primary key")
	}
	db := &Database{db: sqlDB}
	trend, err := db.GetMessageTotalTrend("day", "2026-06-16", "2026-06-16")
	if err != nil {
		t.Fatalf("GetMessageTotalTrend returned error: %v", err)
	}
	if trend.Totals[0] != 5 || trend.PrivateTotals[0] != 5 || trend.GroupTotals[0] != 0 {
		t.Fatalf("unexpected migrated split: %+v", trend)
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
