package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jon/jira-tui/internal/jira"
	_ "modernc.org/sqlite"
)

const schemaVersion = 1

type Store struct {
	db *sql.DB
}

type ActiveViewRecord struct {
	Namespace string
	CacheKey  string
	Issues    []jira.Issue
	SyncedAt  time.Time
	FreshTill time.Time
}

func DefaultPath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "jira-tui", "cache.sqlite"), nil
}

func Open(path string) (*Store, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("cache path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func OpenDefault() (*Store, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return Open(path)
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) PutActiveView(ctx context.Context, record ActiveViewRecord) error {
	if s == nil || s.db == nil {
		return errors.New("cache store is closed")
	}
	record.Namespace = strings.TrimSpace(record.Namespace)
	record.CacheKey = strings.TrimSpace(record.CacheKey)
	if record.Namespace == "" || record.CacheKey == "" {
		return errors.New("active view namespace and cache key are required")
	}
	payload, err := json.Marshal(record.Issues)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO active_views(namespace, cache_key, issues_json, synced_at_unix_nano, fresh_till_unix_nano, updated_at_unix_nano)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(namespace, cache_key) DO UPDATE SET
	issues_json = excluded.issues_json,
	synced_at_unix_nano = excluded.synced_at_unix_nano,
	fresh_till_unix_nano = excluded.fresh_till_unix_nano,
	updated_at_unix_nano = excluded.updated_at_unix_nano
`, record.Namespace, record.CacheKey, string(payload), record.SyncedAt.UnixNano(), record.FreshTill.UnixNano(), time.Now().UnixNano())
	return err
}

func (s *Store) GetActiveView(ctx context.Context, namespace string, cacheKey string) (ActiveViewRecord, bool, error) {
	if s == nil || s.db == nil {
		return ActiveViewRecord{}, false, errors.New("cache store is closed")
	}
	namespace = strings.TrimSpace(namespace)
	cacheKey = strings.TrimSpace(cacheKey)
	if namespace == "" || cacheKey == "" {
		return ActiveViewRecord{}, false, nil
	}
	var payload string
	var syncedAtUnixNano int64
	var freshTillUnixNano int64
	err := s.db.QueryRowContext(ctx, `
SELECT issues_json, synced_at_unix_nano, fresh_till_unix_nano
FROM active_views
WHERE namespace = ? AND cache_key = ?
`, namespace, cacheKey).Scan(&payload, &syncedAtUnixNano, &freshTillUnixNano)
	if errors.Is(err, sql.ErrNoRows) {
		return ActiveViewRecord{}, false, nil
	}
	if err != nil {
		return ActiveViewRecord{}, false, err
	}
	var issues []jira.Issue
	if err := json.Unmarshal([]byte(payload), &issues); err != nil {
		return ActiveViewRecord{}, false, fmt.Errorf("decode active view cache: %w", err)
	}
	return ActiveViewRecord{
		Namespace: namespace,
		CacheKey:  cacheKey,
		Issues:    issues,
		SyncedAt:  time.Unix(0, syncedAtUnixNano),
		FreshTill: time.Unix(0, freshTillUnixNano),
	}, true, nil
}

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA user_version = `+fmt.Sprint(schemaVersion)); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS active_views (
	namespace TEXT NOT NULL,
	cache_key TEXT NOT NULL,
	issues_json TEXT NOT NULL,
	synced_at_unix_nano INTEGER NOT NULL,
	fresh_till_unix_nano INTEGER NOT NULL,
	updated_at_unix_nano INTEGER NOT NULL,
	PRIMARY KEY(namespace, cache_key)
);
CREATE INDEX IF NOT EXISTS active_views_updated_at_idx ON active_views(updated_at_unix_nano);
`)
	return err
}
