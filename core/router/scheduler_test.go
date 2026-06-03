package router

import (
	"testing"
	"time"
)

func TestNextCronTimeSupportsMultipleExpressions(t *testing.T) {
	after := time.Date(2026, 5, 20, 8, 30, 14, 0, time.Local)
	next, err := NextCronTime("15 30 8 * * *\n0 8 12 * * *", after)
	if err != nil {
		t.Fatalf("NextCronTime returned error: %v", err)
	}
	expected := time.Date(2026, 5, 20, 8, 30, 15, 0, time.Local)
	if !next.Equal(expected) {
		t.Fatalf("next = %v, expected %v", next, expected)
	}
}

func TestNextCronTimeSupportsOnce(t *testing.T) {
	_, err := NextCronTime("@once", time.Date(2026, 5, 20, 8, 30, 14, 0, time.Local))
	if err == nil || err.Error() != "@once 任务只能手动启动" {
		t.Fatalf("err = %v, expected @once manual start error", err)
	}
}

func TestNextCronTimeRejectsMixedOnce(t *testing.T) {
	_, err := NextCronTime("@once\n0 8 12 * * *", time.Date(2026, 5, 20, 8, 30, 14, 0, time.Local))
	if err == nil {
		t.Fatal("expected mixed @once error")
	}
}
