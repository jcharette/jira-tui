package tui

import (
	"testing"
	"time"
)

func TestJiraCacheRecordFreshness(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	record := newJiraCacheRecord("cached", now, time.Minute)

	if record.Value != "cached" {
		t.Fatalf("Value = %q", record.Value)
	}
	if record.SyncedAt != now {
		t.Fatalf("SyncedAt = %s", record.SyncedAt)
	}
	if record.FreshTill != now.Add(time.Minute) {
		t.Fatalf("FreshTill = %s", record.FreshTill)
	}
	if !record.Fresh(now.Add(59 * time.Second)) {
		t.Fatal("record should be fresh before FreshTill")
	}
	if record.Fresh(now.Add(time.Minute)) {
		t.Fatal("record should be stale at FreshTill")
	}
}
