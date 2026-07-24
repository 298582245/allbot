package config

import (
	"path/filepath"
	"testing"
	"time"
)

func TestReplaceWithWaitsForActiveDatabaseOperation(t *testing.T) {
	dir := t.TempDir()
	targetPath := filepath.Join(dir, "target.db")
	sourcePath := filepath.Join(dir, "source.db")
	database, err := NewDatabase(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	source, err := NewDatabase(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}

	done := database.db.beginOperation()
	replaced := make(chan error, 1)
	go func() { replaced <- database.ReplaceWith(sourcePath) }()
	select {
	case err := <-replaced:
		t.Fatalf("数据库替换不应越过活动操作: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	done()
	select {
	case err := <-replaced:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("活动操作结束后数据库替换未完成")
	}
}
