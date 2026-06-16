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

type IssueDetailRecord struct {
	Namespace string
	IssueKey  string
	Detail    jira.IssueDetail
	SyncedAt  time.Time
	FreshTill time.Time
}

type IssueCommentsRecord struct {
	Namespace  string
	IssueKey   string
	MaxResults int
	Comments   []jira.Comment
	SyncedAt   time.Time
	FreshTill  time.Time
}

type IssueTransitionsRecord struct {
	Namespace   string
	IssueKey    string
	Transitions []jira.Transition
	SyncedAt    time.Time
	FreshTill   time.Time
}

type IssueEditMetadataRecord struct {
	Namespace string
	IssueKey  string
	Metadata  jira.EditMetadata
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

func (s *Store) PutIssueDetail(ctx context.Context, record IssueDetailRecord) error {
	if s == nil || s.db == nil {
		return errors.New("cache store is closed")
	}
	record.Namespace = strings.TrimSpace(record.Namespace)
	record.IssueKey = strings.TrimSpace(record.IssueKey)
	if record.Namespace == "" || record.IssueKey == "" {
		return errors.New("issue detail namespace and issue key are required")
	}
	payload, err := json.Marshal(record.Detail)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO issue_details(namespace, issue_key, detail_json, synced_at_unix_nano, fresh_till_unix_nano, updated_at_unix_nano)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(namespace, issue_key) DO UPDATE SET
	detail_json = excluded.detail_json,
	synced_at_unix_nano = excluded.synced_at_unix_nano,
	fresh_till_unix_nano = excluded.fresh_till_unix_nano,
	updated_at_unix_nano = excluded.updated_at_unix_nano
`, record.Namespace, record.IssueKey, string(payload), record.SyncedAt.UnixNano(), record.FreshTill.UnixNano(), time.Now().UnixNano())
	return err
}

func (s *Store) GetIssueDetail(ctx context.Context, namespace string, issueKey string) (IssueDetailRecord, bool, error) {
	if s == nil || s.db == nil {
		return IssueDetailRecord{}, false, errors.New("cache store is closed")
	}
	namespace = strings.TrimSpace(namespace)
	issueKey = strings.TrimSpace(issueKey)
	if namespace == "" || issueKey == "" {
		return IssueDetailRecord{}, false, nil
	}
	var payload string
	var syncedAtUnixNano int64
	var freshTillUnixNano int64
	err := s.db.QueryRowContext(ctx, `
SELECT detail_json, synced_at_unix_nano, fresh_till_unix_nano
FROM issue_details
WHERE namespace = ? AND issue_key = ?
`, namespace, issueKey).Scan(&payload, &syncedAtUnixNano, &freshTillUnixNano)
	if errors.Is(err, sql.ErrNoRows) {
		return IssueDetailRecord{}, false, nil
	}
	if err != nil {
		return IssueDetailRecord{}, false, err
	}
	var detail jira.IssueDetail
	if err := json.Unmarshal([]byte(payload), &detail); err != nil {
		return IssueDetailRecord{}, false, fmt.Errorf("decode issue detail cache: %w", err)
	}
	return IssueDetailRecord{
		Namespace: namespace,
		IssueKey:  issueKey,
		Detail:    detail,
		SyncedAt:  time.Unix(0, syncedAtUnixNano),
		FreshTill: time.Unix(0, freshTillUnixNano),
	}, true, nil
}

func (s *Store) PutIssueComments(ctx context.Context, record IssueCommentsRecord) error {
	if s == nil || s.db == nil {
		return errors.New("cache store is closed")
	}
	record.Namespace = strings.TrimSpace(record.Namespace)
	record.IssueKey = strings.TrimSpace(record.IssueKey)
	if record.Namespace == "" || record.IssueKey == "" || record.MaxResults <= 0 {
		return errors.New("issue comments namespace, issue key, and max results are required")
	}
	payload, err := json.Marshal(record.Comments)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO issue_comments(namespace, issue_key, max_results, comments_json, synced_at_unix_nano, fresh_till_unix_nano, updated_at_unix_nano)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(namespace, issue_key, max_results) DO UPDATE SET
	comments_json = excluded.comments_json,
	synced_at_unix_nano = excluded.synced_at_unix_nano,
	fresh_till_unix_nano = excluded.fresh_till_unix_nano,
	updated_at_unix_nano = excluded.updated_at_unix_nano
`, record.Namespace, record.IssueKey, record.MaxResults, string(payload), record.SyncedAt.UnixNano(), record.FreshTill.UnixNano(), time.Now().UnixNano())
	return err
}

func (s *Store) GetIssueComments(ctx context.Context, namespace string, issueKey string, maxResults int) (IssueCommentsRecord, bool, error) {
	if s == nil || s.db == nil {
		return IssueCommentsRecord{}, false, errors.New("cache store is closed")
	}
	namespace = strings.TrimSpace(namespace)
	issueKey = strings.TrimSpace(issueKey)
	if namespace == "" || issueKey == "" || maxResults <= 0 {
		return IssueCommentsRecord{}, false, nil
	}
	var payload string
	var syncedAtUnixNano int64
	var freshTillUnixNano int64
	err := s.db.QueryRowContext(ctx, `
SELECT comments_json, synced_at_unix_nano, fresh_till_unix_nano
FROM issue_comments
WHERE namespace = ? AND issue_key = ? AND max_results = ?
`, namespace, issueKey, maxResults).Scan(&payload, &syncedAtUnixNano, &freshTillUnixNano)
	if errors.Is(err, sql.ErrNoRows) {
		return IssueCommentsRecord{}, false, nil
	}
	if err != nil {
		return IssueCommentsRecord{}, false, err
	}
	var comments []jira.Comment
	if err := json.Unmarshal([]byte(payload), &comments); err != nil {
		return IssueCommentsRecord{}, false, fmt.Errorf("decode issue comments cache: %w", err)
	}
	return IssueCommentsRecord{
		Namespace:  namespace,
		IssueKey:   issueKey,
		MaxResults: maxResults,
		Comments:   comments,
		SyncedAt:   time.Unix(0, syncedAtUnixNano),
		FreshTill:  time.Unix(0, freshTillUnixNano),
	}, true, nil
}

func (s *Store) DeleteIssueComments(ctx context.Context, namespace string, issueKey string) error {
	if s == nil || s.db == nil {
		return errors.New("cache store is closed")
	}
	namespace = strings.TrimSpace(namespace)
	issueKey = strings.TrimSpace(issueKey)
	if namespace == "" || issueKey == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
DELETE FROM issue_comments
WHERE namespace = ? AND issue_key = ?
`, namespace, issueKey)
	return err
}

func (s *Store) PutIssueTransitions(ctx context.Context, record IssueTransitionsRecord) error {
	if s == nil || s.db == nil {
		return errors.New("cache store is closed")
	}
	record.Namespace = strings.TrimSpace(record.Namespace)
	record.IssueKey = strings.TrimSpace(record.IssueKey)
	if record.Namespace == "" || record.IssueKey == "" {
		return errors.New("issue transitions namespace and issue key are required")
	}
	payload, err := json.Marshal(record.Transitions)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO issue_transitions(namespace, issue_key, transitions_json, synced_at_unix_nano, fresh_till_unix_nano, updated_at_unix_nano)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(namespace, issue_key) DO UPDATE SET
	transitions_json = excluded.transitions_json,
	synced_at_unix_nano = excluded.synced_at_unix_nano,
	fresh_till_unix_nano = excluded.fresh_till_unix_nano,
	updated_at_unix_nano = excluded.updated_at_unix_nano
`, record.Namespace, record.IssueKey, string(payload), record.SyncedAt.UnixNano(), record.FreshTill.UnixNano(), time.Now().UnixNano())
	return err
}

func (s *Store) GetIssueTransitions(ctx context.Context, namespace string, issueKey string) (IssueTransitionsRecord, bool, error) {
	if s == nil || s.db == nil {
		return IssueTransitionsRecord{}, false, errors.New("cache store is closed")
	}
	namespace = strings.TrimSpace(namespace)
	issueKey = strings.TrimSpace(issueKey)
	if namespace == "" || issueKey == "" {
		return IssueTransitionsRecord{}, false, nil
	}
	var payload string
	var syncedAtUnixNano int64
	var freshTillUnixNano int64
	err := s.db.QueryRowContext(ctx, `
SELECT transitions_json, synced_at_unix_nano, fresh_till_unix_nano
FROM issue_transitions
WHERE namespace = ? AND issue_key = ?
`, namespace, issueKey).Scan(&payload, &syncedAtUnixNano, &freshTillUnixNano)
	if errors.Is(err, sql.ErrNoRows) {
		return IssueTransitionsRecord{}, false, nil
	}
	if err != nil {
		return IssueTransitionsRecord{}, false, err
	}
	var transitions []jira.Transition
	if err := json.Unmarshal([]byte(payload), &transitions); err != nil {
		return IssueTransitionsRecord{}, false, fmt.Errorf("decode issue transitions cache: %w", err)
	}
	return IssueTransitionsRecord{
		Namespace:   namespace,
		IssueKey:    issueKey,
		Transitions: transitions,
		SyncedAt:    time.Unix(0, syncedAtUnixNano),
		FreshTill:   time.Unix(0, freshTillUnixNano),
	}, true, nil
}

func (s *Store) PutIssueEditMetadata(ctx context.Context, record IssueEditMetadataRecord) error {
	if s == nil || s.db == nil {
		return errors.New("cache store is closed")
	}
	record.Namespace = strings.TrimSpace(record.Namespace)
	record.IssueKey = strings.TrimSpace(record.IssueKey)
	if record.Namespace == "" || record.IssueKey == "" {
		return errors.New("issue edit metadata namespace and issue key are required")
	}
	payload, err := json.Marshal(record.Metadata)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO issue_edit_metadata(namespace, issue_key, metadata_json, synced_at_unix_nano, fresh_till_unix_nano, updated_at_unix_nano)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(namespace, issue_key) DO UPDATE SET
	metadata_json = excluded.metadata_json,
	synced_at_unix_nano = excluded.synced_at_unix_nano,
	fresh_till_unix_nano = excluded.fresh_till_unix_nano,
	updated_at_unix_nano = excluded.updated_at_unix_nano
`, record.Namespace, record.IssueKey, string(payload), record.SyncedAt.UnixNano(), record.FreshTill.UnixNano(), time.Now().UnixNano())
	return err
}

func (s *Store) GetIssueEditMetadata(ctx context.Context, namespace string, issueKey string) (IssueEditMetadataRecord, bool, error) {
	if s == nil || s.db == nil {
		return IssueEditMetadataRecord{}, false, errors.New("cache store is closed")
	}
	namespace = strings.TrimSpace(namespace)
	issueKey = strings.TrimSpace(issueKey)
	if namespace == "" || issueKey == "" {
		return IssueEditMetadataRecord{}, false, nil
	}
	var payload string
	var syncedAtUnixNano int64
	var freshTillUnixNano int64
	err := s.db.QueryRowContext(ctx, `
SELECT metadata_json, synced_at_unix_nano, fresh_till_unix_nano
FROM issue_edit_metadata
WHERE namespace = ? AND issue_key = ?
`, namespace, issueKey).Scan(&payload, &syncedAtUnixNano, &freshTillUnixNano)
	if errors.Is(err, sql.ErrNoRows) {
		return IssueEditMetadataRecord{}, false, nil
	}
	if err != nil {
		return IssueEditMetadataRecord{}, false, err
	}
	var metadata jira.EditMetadata
	if err := json.Unmarshal([]byte(payload), &metadata); err != nil {
		return IssueEditMetadataRecord{}, false, fmt.Errorf("decode issue edit metadata cache: %w", err)
	}
	return IssueEditMetadataRecord{
		Namespace: namespace,
		IssueKey:  issueKey,
		Metadata:  metadata,
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

CREATE TABLE IF NOT EXISTS issue_details (
	namespace TEXT NOT NULL,
	issue_key TEXT NOT NULL,
	detail_json TEXT NOT NULL,
	synced_at_unix_nano INTEGER NOT NULL,
	fresh_till_unix_nano INTEGER NOT NULL,
	updated_at_unix_nano INTEGER NOT NULL,
	PRIMARY KEY(namespace, issue_key)
);
CREATE INDEX IF NOT EXISTS issue_details_updated_at_idx ON issue_details(updated_at_unix_nano);

CREATE TABLE IF NOT EXISTS issue_comments (
	namespace TEXT NOT NULL,
	issue_key TEXT NOT NULL,
	max_results INTEGER NOT NULL,
	comments_json TEXT NOT NULL,
	synced_at_unix_nano INTEGER NOT NULL,
	fresh_till_unix_nano INTEGER NOT NULL,
	updated_at_unix_nano INTEGER NOT NULL,
	PRIMARY KEY(namespace, issue_key, max_results)
);
CREATE INDEX IF NOT EXISTS issue_comments_updated_at_idx ON issue_comments(updated_at_unix_nano);

CREATE TABLE IF NOT EXISTS issue_transitions (
	namespace TEXT NOT NULL,
	issue_key TEXT NOT NULL,
	transitions_json TEXT NOT NULL,
	synced_at_unix_nano INTEGER NOT NULL,
	fresh_till_unix_nano INTEGER NOT NULL,
	updated_at_unix_nano INTEGER NOT NULL,
	PRIMARY KEY(namespace, issue_key)
);
CREATE INDEX IF NOT EXISTS issue_transitions_updated_at_idx ON issue_transitions(updated_at_unix_nano);

CREATE TABLE IF NOT EXISTS issue_edit_metadata (
	namespace TEXT NOT NULL,
	issue_key TEXT NOT NULL,
	metadata_json TEXT NOT NULL,
	synced_at_unix_nano INTEGER NOT NULL,
	fresh_till_unix_nano INTEGER NOT NULL,
	updated_at_unix_nano INTEGER NOT NULL,
	PRIMARY KEY(namespace, issue_key)
);
CREATE INDEX IF NOT EXISTS issue_edit_metadata_updated_at_idx ON issue_edit_metadata(updated_at_unix_nano);
`)
	return err
}
