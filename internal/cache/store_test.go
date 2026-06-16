package cache

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jon/jira-tui/internal/jira"
)

func TestStorePersistsActiveViewRecords(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	syncedAt := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	record := ActiveViewRecord{
		Namespace: "https://example.atlassian.net",
		CacheKey:  "project = ABC",
		Issues:    []jira.Issue{{Key: "ABC-1", Summary: "Cached issue"}},
		SyncedAt:  syncedAt,
		FreshTill: syncedAt.Add(time.Minute),
	}
	if err := store.PutActiveView(ctx, record); err != nil {
		t.Fatalf("PutActiveView() error = %v", err)
	}

	got, ok, err := store.GetActiveView(ctx, record.Namespace, record.CacheKey)
	if err != nil {
		t.Fatalf("GetActiveView() error = %v", err)
	}
	if !ok {
		t.Fatal("expected active view record")
	}
	if !got.SyncedAt.Equal(record.SyncedAt) || !got.FreshTill.Equal(record.FreshTill) {
		t.Fatalf("timestamps = %s/%s", got.SyncedAt, got.FreshTill)
	}
	if len(got.Issues) != 1 || got.Issues[0].Key != "ABC-1" || got.Issues[0].Summary != "Cached issue" {
		t.Fatalf("Issues = %#v", got.Issues)
	}
}

func TestStoreReplacesActiveViewRecordsByNamespaceAndKey(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	base := ActiveViewRecord{
		Namespace: "https://example.atlassian.net",
		CacheKey:  "project = ABC",
		Issues:    []jira.Issue{{Key: "ABC-1"}},
		SyncedAt:  time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC),
		FreshTill: time.Date(2026, 6, 16, 10, 1, 0, 0, time.UTC),
	}
	if err := store.PutActiveView(ctx, base); err != nil {
		t.Fatalf("PutActiveView(base) error = %v", err)
	}
	replacement := base
	replacement.Issues = []jira.Issue{{Key: "ABC-2"}}
	replacement.SyncedAt = base.SyncedAt.Add(time.Minute)
	replacement.FreshTill = base.FreshTill.Add(time.Minute)
	if err := store.PutActiveView(ctx, replacement); err != nil {
		t.Fatalf("PutActiveView(replacement) error = %v", err)
	}

	got, ok, err := store.GetActiveView(ctx, base.Namespace, base.CacheKey)
	if err != nil {
		t.Fatalf("GetActiveView() error = %v", err)
	}
	if !ok {
		t.Fatal("expected replacement record")
	}
	if len(got.Issues) != 1 || got.Issues[0].Key != "ABC-2" {
		t.Fatalf("Issues = %#v", got.Issues)
	}
	if !got.SyncedAt.Equal(replacement.SyncedAt) {
		t.Fatalf("SyncedAt = %s", got.SyncedAt)
	}
}

func TestStoreKeepsActiveViewsIsolatedByNamespace(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	record := ActiveViewRecord{
		Namespace: "https://one.atlassian.net",
		CacheKey:  "project = ABC",
		Issues:    []jira.Issue{{Key: "ONE-1"}},
		SyncedAt:  time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC),
		FreshTill: time.Date(2026, 6, 16, 10, 1, 0, 0, time.UTC),
	}
	if err := store.PutActiveView(ctx, record); err != nil {
		t.Fatalf("PutActiveView() error = %v", err)
	}
	if _, ok, err := store.GetActiveView(ctx, "https://two.atlassian.net", record.CacheKey); err != nil || ok {
		t.Fatalf("other namespace ok=%v err=%v", ok, err)
	}
}

func TestDefaultPathUsesAppCacheFile(t *testing.T) {
	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	if filepath.Base(path) != "cache.sqlite" || filepath.Base(filepath.Dir(path)) != "jira-tui" {
		t.Fatalf("DefaultPath() = %q", path)
	}
}
