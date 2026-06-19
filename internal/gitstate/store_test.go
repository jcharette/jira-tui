package gitstate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreMarksAndReadsReportedCommits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	store, err := Open(path, WithNow(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	err = store.MarkReported(context.Background(), []ReportedCommit{
		{RepoPath: "/tmp/repo", Branch: "abc-123-work", IssueKey: "abc-123", SHA: "111", Subject: "first"},
		{RepoPath: "/tmp/repo", Branch: "abc-123-work", IssueKey: "ABC-123", SHA: "222", Subject: "second"},
	})
	if err != nil {
		t.Fatalf("MarkReported() error = %v", err)
	}

	records, err := store.ReportedCommits(context.Background(), "/tmp/repo", "abc-123-work", "ABC-123")
	if err != nil {
		t.Fatalf("ReportedCommits() error = %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %#v", records)
	}
	if records[0].IssueKey != "ABC-123" || !records[0].ReportedAt.Equal(now) {
		t.Fatalf("record normalization = %#v", records[0])
	}
}

func TestStoreDedupesReportedCommits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	first := ReportedCommit{RepoPath: "/tmp/repo", Branch: "abc-123-work", IssueKey: "ABC-123", SHA: "111", Subject: "old"}
	second := first
	second.Subject = "new"
	if err := store.MarkReported(context.Background(), []ReportedCommit{first, second}); err != nil {
		t.Fatalf("MarkReported() error = %v", err)
	}

	records, err := store.ReportedCommits(context.Background(), "/tmp/repo", "abc-123-work", "ABC-123")
	if err != nil {
		t.Fatalf("ReportedCommits() error = %v", err)
	}
	if len(records) != 1 || records[0].Subject != "new" {
		t.Fatalf("records = %#v", records)
	}
}

func TestStoreCreatesPrivateStateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	if _, err := Open(path); err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat state: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("state mode = %o, want 0600", got)
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("dir mode = %o, want 0700", got)
	}
}
