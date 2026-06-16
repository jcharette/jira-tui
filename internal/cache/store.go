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

type CreateIssueTypesRecord struct {
	Namespace  string
	ProjectKey string
	IssueTypes []jira.CreateIssueType
	SyncedAt   time.Time
	FreshTill  time.Time
}

type CreateFieldsRecord struct {
	Namespace   string
	ProjectKey  string
	IssueTypeID string
	Fields      []jira.CreateField
	SyncedAt    time.Time
	FreshTill   time.Time
}

type ExpandedChildrenRecord struct {
	Namespace string
	ParentKey string
	Mode      string
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

func (s *Store) PutCreateIssueTypes(ctx context.Context, record CreateIssueTypesRecord) error {
	if s == nil || s.db == nil {
		return errors.New("cache store is closed")
	}
	record.Namespace = strings.TrimSpace(record.Namespace)
	record.ProjectKey = strings.TrimSpace(record.ProjectKey)
	if record.Namespace == "" || record.ProjectKey == "" {
		return errors.New("create issue types namespace and project key are required")
	}
	payload, err := json.Marshal(record.IssueTypes)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO create_issue_types(namespace, project_key, issue_types_json, synced_at_unix_nano, fresh_till_unix_nano, updated_at_unix_nano)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(namespace, project_key) DO UPDATE SET
	issue_types_json = excluded.issue_types_json,
	synced_at_unix_nano = excluded.synced_at_unix_nano,
	fresh_till_unix_nano = excluded.fresh_till_unix_nano,
	updated_at_unix_nano = excluded.updated_at_unix_nano
`, record.Namespace, record.ProjectKey, string(payload), record.SyncedAt.UnixNano(), record.FreshTill.UnixNano(), time.Now().UnixNano())
	return err
}

func (s *Store) GetCreateIssueTypes(ctx context.Context, namespace string, projectKey string) (CreateIssueTypesRecord, bool, error) {
	if s == nil || s.db == nil {
		return CreateIssueTypesRecord{}, false, errors.New("cache store is closed")
	}
	namespace = strings.TrimSpace(namespace)
	projectKey = strings.TrimSpace(projectKey)
	if namespace == "" || projectKey == "" {
		return CreateIssueTypesRecord{}, false, nil
	}
	var payload string
	var syncedAtUnixNano int64
	var freshTillUnixNano int64
	err := s.db.QueryRowContext(ctx, `
SELECT issue_types_json, synced_at_unix_nano, fresh_till_unix_nano
FROM create_issue_types
WHERE namespace = ? AND project_key = ?
`, namespace, projectKey).Scan(&payload, &syncedAtUnixNano, &freshTillUnixNano)
	if errors.Is(err, sql.ErrNoRows) {
		return CreateIssueTypesRecord{}, false, nil
	}
	if err != nil {
		return CreateIssueTypesRecord{}, false, err
	}
	var issueTypes []jira.CreateIssueType
	if err := json.Unmarshal([]byte(payload), &issueTypes); err != nil {
		return CreateIssueTypesRecord{}, false, fmt.Errorf("decode create issue types cache: %w", err)
	}
	return CreateIssueTypesRecord{
		Namespace:  namespace,
		ProjectKey: projectKey,
		IssueTypes: issueTypes,
		SyncedAt:   time.Unix(0, syncedAtUnixNano),
		FreshTill:  time.Unix(0, freshTillUnixNano),
	}, true, nil
}

func (s *Store) PutCreateFields(ctx context.Context, record CreateFieldsRecord) error {
	if s == nil || s.db == nil {
		return errors.New("cache store is closed")
	}
	record.Namespace = strings.TrimSpace(record.Namespace)
	record.ProjectKey = strings.TrimSpace(record.ProjectKey)
	record.IssueTypeID = strings.TrimSpace(record.IssueTypeID)
	if record.Namespace == "" || record.ProjectKey == "" || record.IssueTypeID == "" {
		return errors.New("create fields namespace, project key, and issue type ID are required")
	}
	payload, err := json.Marshal(record.Fields)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO create_fields(namespace, project_key, issue_type_id, fields_json, synced_at_unix_nano, fresh_till_unix_nano, updated_at_unix_nano)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(namespace, project_key, issue_type_id) DO UPDATE SET
	fields_json = excluded.fields_json,
	synced_at_unix_nano = excluded.synced_at_unix_nano,
	fresh_till_unix_nano = excluded.fresh_till_unix_nano,
	updated_at_unix_nano = excluded.updated_at_unix_nano
`, record.Namespace, record.ProjectKey, record.IssueTypeID, string(payload), record.SyncedAt.UnixNano(), record.FreshTill.UnixNano(), time.Now().UnixNano())
	return err
}

func (s *Store) GetCreateFields(ctx context.Context, namespace string, projectKey string, issueTypeID string) (CreateFieldsRecord, bool, error) {
	if s == nil || s.db == nil {
		return CreateFieldsRecord{}, false, errors.New("cache store is closed")
	}
	namespace = strings.TrimSpace(namespace)
	projectKey = strings.TrimSpace(projectKey)
	issueTypeID = strings.TrimSpace(issueTypeID)
	if namespace == "" || projectKey == "" || issueTypeID == "" {
		return CreateFieldsRecord{}, false, nil
	}
	var payload string
	var syncedAtUnixNano int64
	var freshTillUnixNano int64
	err := s.db.QueryRowContext(ctx, `
SELECT fields_json, synced_at_unix_nano, fresh_till_unix_nano
FROM create_fields
WHERE namespace = ? AND project_key = ? AND issue_type_id = ?
`, namespace, projectKey, issueTypeID).Scan(&payload, &syncedAtUnixNano, &freshTillUnixNano)
	if errors.Is(err, sql.ErrNoRows) {
		return CreateFieldsRecord{}, false, nil
	}
	if err != nil {
		return CreateFieldsRecord{}, false, err
	}
	var fields []jira.CreateField
	if err := json.Unmarshal([]byte(payload), &fields); err != nil {
		return CreateFieldsRecord{}, false, fmt.Errorf("decode create fields cache: %w", err)
	}
	return CreateFieldsRecord{
		Namespace:   namespace,
		ProjectKey:  projectKey,
		IssueTypeID: issueTypeID,
		Fields:      fields,
		SyncedAt:    time.Unix(0, syncedAtUnixNano),
		FreshTill:   time.Unix(0, freshTillUnixNano),
	}, true, nil
}

func (s *Store) PutExpandedChildren(ctx context.Context, record ExpandedChildrenRecord) error {
	if s == nil || s.db == nil {
		return errors.New("cache store is closed")
	}
	record.Namespace = strings.TrimSpace(record.Namespace)
	record.ParentKey = strings.TrimSpace(record.ParentKey)
	record.Mode = strings.TrimSpace(record.Mode)
	if record.Namespace == "" || record.ParentKey == "" || record.Mode == "" {
		return errors.New("expanded children namespace, parent key, and mode are required")
	}
	payload, err := json.Marshal(record.Issues)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO expanded_children(namespace, parent_key, mode, issues_json, synced_at_unix_nano, fresh_till_unix_nano, updated_at_unix_nano)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(namespace, parent_key, mode) DO UPDATE SET
	issues_json = excluded.issues_json,
	synced_at_unix_nano = excluded.synced_at_unix_nano,
	fresh_till_unix_nano = excluded.fresh_till_unix_nano,
	updated_at_unix_nano = excluded.updated_at_unix_nano
`, record.Namespace, record.ParentKey, record.Mode, string(payload), record.SyncedAt.UnixNano(), record.FreshTill.UnixNano(), time.Now().UnixNano())
	return err
}

func (s *Store) GetExpandedChildren(ctx context.Context, namespace string, parentKey string, mode string) (ExpandedChildrenRecord, bool, error) {
	if s == nil || s.db == nil {
		return ExpandedChildrenRecord{}, false, errors.New("cache store is closed")
	}
	namespace = strings.TrimSpace(namespace)
	parentKey = strings.TrimSpace(parentKey)
	mode = strings.TrimSpace(mode)
	if namespace == "" || parentKey == "" || mode == "" {
		return ExpandedChildrenRecord{}, false, nil
	}
	var payload string
	var syncedAtUnixNano int64
	var freshTillUnixNano int64
	err := s.db.QueryRowContext(ctx, `
SELECT issues_json, synced_at_unix_nano, fresh_till_unix_nano
FROM expanded_children
WHERE namespace = ? AND parent_key = ? AND mode = ?
`, namespace, parentKey, mode).Scan(&payload, &syncedAtUnixNano, &freshTillUnixNano)
	if errors.Is(err, sql.ErrNoRows) {
		return ExpandedChildrenRecord{}, false, nil
	}
	if err != nil {
		return ExpandedChildrenRecord{}, false, err
	}
	var issues []jira.Issue
	if err := json.Unmarshal([]byte(payload), &issues); err != nil {
		return ExpandedChildrenRecord{}, false, fmt.Errorf("decode expanded children cache: %w", err)
	}
	return ExpandedChildrenRecord{
		Namespace: namespace,
		ParentKey: parentKey,
		Mode:      mode,
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

CREATE TABLE IF NOT EXISTS create_issue_types (
	namespace TEXT NOT NULL,
	project_key TEXT NOT NULL,
	issue_types_json TEXT NOT NULL,
	synced_at_unix_nano INTEGER NOT NULL,
	fresh_till_unix_nano INTEGER NOT NULL,
	updated_at_unix_nano INTEGER NOT NULL,
	PRIMARY KEY(namespace, project_key)
);
CREATE INDEX IF NOT EXISTS create_issue_types_updated_at_idx ON create_issue_types(updated_at_unix_nano);

CREATE TABLE IF NOT EXISTS create_fields (
	namespace TEXT NOT NULL,
	project_key TEXT NOT NULL,
	issue_type_id TEXT NOT NULL,
	fields_json TEXT NOT NULL,
	synced_at_unix_nano INTEGER NOT NULL,
	fresh_till_unix_nano INTEGER NOT NULL,
	updated_at_unix_nano INTEGER NOT NULL,
	PRIMARY KEY(namespace, project_key, issue_type_id)
);
CREATE INDEX IF NOT EXISTS create_fields_updated_at_idx ON create_fields(updated_at_unix_nano);

CREATE TABLE IF NOT EXISTS expanded_children (
	namespace TEXT NOT NULL,
	parent_key TEXT NOT NULL,
	mode TEXT NOT NULL,
	issues_json TEXT NOT NULL,
	synced_at_unix_nano INTEGER NOT NULL,
	fresh_till_unix_nano INTEGER NOT NULL,
	updated_at_unix_nano INTEGER NOT NULL,
	PRIMARY KEY(namespace, parent_key, mode)
);
CREATE INDEX IF NOT EXISTS expanded_children_updated_at_idx ON expanded_children(updated_at_unix_nano);
`)
	return err
}
