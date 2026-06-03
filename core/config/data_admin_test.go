package config

import "testing"

func TestQueryTableRowsSearchesRowValues(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.db.Exec(`CREATE TABLE demo_search (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, note TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.Exec(`INSERT INTO demo_search (name, note) VALUES (?, ?), (?, ?)`, "苹果", "红色水果", "香蕉", "黄色水果"); err != nil {
		t.Fatal(err)
	}

	rows, err := db.QueryTableRows("demo_search", 1, 20, "香蕉")
	if err != nil {
		t.Fatal(err)
	}
	if rows.Total != 1 || len(rows.Rows) != 1 {
		t.Fatalf("expected one matched row, got total=%d len=%d", rows.Total, len(rows.Rows))
	}
	if rows.Rows[0]["name"] != "香蕉" {
		t.Fatalf("unexpected matched row: %#v", rows.Rows[0])
	}
}

func TestQueryTableRowsSearchesTableMetadata(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.db.Exec(`CREATE TABLE demo_meta (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.Exec(`INSERT INTO demo_meta (name) VALUES (?), (?)`, "第一行", "第二行"); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveDataView(DataViewConfig{TableName: "demo_meta", ViewName: "客户资料", GroupName: "业务数据", Description: "客户数据表"}); err != nil {
		t.Fatal(err)
	}

	rows, err := db.QueryTableRows("demo_meta", 1, 20, "客户")
	if err != nil {
		t.Fatal(err)
	}
	if rows.Total != 2 || len(rows.Rows) != 2 {
		t.Fatalf("expected metadata match to return all rows, got total=%d len=%d", rows.Total, len(rows.Rows))
	}
}

func TestQueryTableRowsEscapesSearchWildcards(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.db.Exec(`CREATE TABLE demo_escape (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.Exec(`INSERT INTO demo_escape (name) VALUES (?), (?)`, "100%匹配", "1000匹配"); err != nil {
		t.Fatal(err)
	}

	rows, err := db.QueryTableRows("demo_escape", 1, 20, "100%")
	if err != nil {
		t.Fatal(err)
	}
	if rows.Total != 1 || len(rows.Rows) != 1 || rows.Rows[0]["name"] != "100%匹配" {
		t.Fatalf("expected literal wildcard match, got total=%d rows=%#v", rows.Total, rows.Rows)
	}
}
