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

func TestStorePersistsIssueDetailRecords(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	syncedAt := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	record := IssueDetailRecord{
		Namespace: "https://example.atlassian.net",
		IssueKey:  "ABC-1",
		Detail:    jira.IssueDetail{Issue: jira.Issue{Key: "ABC-1", Summary: "Cached detail"}, Description: "Stored description"},
		SyncedAt:  syncedAt,
		FreshTill: syncedAt.Add(time.Minute),
	}
	if err := store.PutIssueDetail(ctx, record); err != nil {
		t.Fatalf("PutIssueDetail() error = %v", err)
	}

	got, ok, err := store.GetIssueDetail(ctx, record.Namespace, record.IssueKey)
	if err != nil {
		t.Fatalf("GetIssueDetail() error = %v", err)
	}
	if !ok {
		t.Fatal("expected issue detail record")
	}
	if got.Detail.Key != "ABC-1" || got.Detail.Description != "Stored description" {
		t.Fatalf("Detail = %#v", got.Detail)
	}
	if !got.SyncedAt.Equal(record.SyncedAt) || !got.FreshTill.Equal(record.FreshTill) {
		t.Fatalf("timestamps = %s/%s", got.SyncedAt, got.FreshTill)
	}
}

func TestStorePersistsIssueCommentsRecords(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	syncedAt := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	record := IssueCommentsRecord{
		Namespace:  "https://example.atlassian.net",
		IssueKey:   "ABC-1",
		MaxResults: 10,
		Comments:   []jira.Comment{{ID: "10001", Body: "Stored comment"}},
		SyncedAt:   syncedAt,
		FreshTill:  syncedAt.Add(time.Minute),
	}
	if err := store.PutIssueComments(ctx, record); err != nil {
		t.Fatalf("PutIssueComments() error = %v", err)
	}

	got, ok, err := store.GetIssueComments(ctx, record.Namespace, record.IssueKey, record.MaxResults)
	if err != nil {
		t.Fatalf("GetIssueComments() error = %v", err)
	}
	if !ok {
		t.Fatal("expected issue comments record")
	}
	if len(got.Comments) != 1 || got.Comments[0].ID != "10001" || got.Comments[0].Body != "Stored comment" {
		t.Fatalf("Comments = %#v", got.Comments)
	}
	if !got.SyncedAt.Equal(record.SyncedAt) || !got.FreshTill.Equal(record.FreshTill) {
		t.Fatalf("timestamps = %s/%s", got.SyncedAt, got.FreshTill)
	}
}

func TestStoreInvalidatesIssueComments(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	record := IssueCommentsRecord{
		Namespace:  "https://example.atlassian.net",
		IssueKey:   "ABC-1",
		MaxResults: 10,
		Comments:   []jira.Comment{{ID: "10001"}},
		SyncedAt:   time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC),
		FreshTill:  time.Date(2026, 6, 16, 10, 1, 0, 0, time.UTC),
	}
	if err := store.PutIssueComments(ctx, record); err != nil {
		t.Fatalf("PutIssueComments() error = %v", err)
	}
	if err := store.DeleteIssueComments(ctx, record.Namespace, record.IssueKey); err != nil {
		t.Fatalf("DeleteIssueComments() error = %v", err)
	}
	if _, ok, err := store.GetIssueComments(ctx, record.Namespace, record.IssueKey, record.MaxResults); err != nil || ok {
		t.Fatalf("deleted comments ok=%v err=%v", ok, err)
	}
}

func TestStorePersistsIssueTransitionsRecords(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	syncedAt := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	record := IssueTransitionsRecord{
		Namespace:   "https://example.atlassian.net",
		IssueKey:    "ABC-1",
		Transitions: []jira.Transition{{ID: "21", Name: "Start Progress", ToStatus: "In Progress"}},
		SyncedAt:    syncedAt,
		FreshTill:   syncedAt.Add(time.Minute),
	}
	if err := store.PutIssueTransitions(ctx, record); err != nil {
		t.Fatalf("PutIssueTransitions() error = %v", err)
	}

	got, ok, err := store.GetIssueTransitions(ctx, record.Namespace, record.IssueKey)
	if err != nil {
		t.Fatalf("GetIssueTransitions() error = %v", err)
	}
	if !ok {
		t.Fatal("expected issue transitions record")
	}
	if len(got.Transitions) != 1 || got.Transitions[0].ID != "21" || got.Transitions[0].ToStatus != "In Progress" {
		t.Fatalf("Transitions = %#v", got.Transitions)
	}
	if !got.SyncedAt.Equal(record.SyncedAt) || !got.FreshTill.Equal(record.FreshTill) {
		t.Fatalf("timestamps = %s/%s", got.SyncedAt, got.FreshTill)
	}
}

func TestStoreInvalidatesIssueTransitions(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	syncedAt := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	record := IssueTransitionsRecord{
		Namespace:   "https://example.atlassian.net",
		IssueKey:    "ABC-1",
		Transitions: []jira.Transition{{ID: "21", Name: "Start Progress", ToStatus: "In Progress"}},
		SyncedAt:    syncedAt,
		FreshTill:   syncedAt.Add(time.Minute),
	}
	if err := store.PutIssueTransitions(ctx, record); err != nil {
		t.Fatalf("PutIssueTransitions() error = %v", err)
	}

	if err := store.DeleteIssueTransitions(ctx, record.Namespace, record.IssueKey); err != nil {
		t.Fatalf("DeleteIssueTransitions() error = %v", err)
	}
	if _, ok, err := store.GetIssueTransitions(ctx, record.Namespace, record.IssueKey); err != nil || ok {
		t.Fatalf("deleted transitions ok=%v err=%v", ok, err)
	}
}

func TestStoreDeletesRowsOlderThanCutoffAcrossCacheTables(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	oldSite := "old-site"
	recentSite := "recent-site"
	for _, namespace := range []string{oldSite, recentSite} {
		if err := store.PutActiveView(ctx, ActiveViewRecord{
			Namespace: namespace,
			CacheKey:  "project = ABC",
			Issues:    []jira.Issue{{Key: "ABC-1"}},
			SyncedAt:  now.Add(-2 * time.Hour),
			FreshTill: now.Add(-time.Hour),
		}); err != nil {
			t.Fatalf("PutActiveView(%s) error = %v", namespace, err)
		}
		if err := store.PutIssueDetail(ctx, IssueDetailRecord{
			Namespace: namespace,
			IssueKey:  "ABC-1",
			Detail:    jira.IssueDetail{Issue: jira.Issue{Key: "ABC-1"}},
			SyncedAt:  now.Add(-2 * time.Hour),
			FreshTill: now.Add(-time.Hour),
		}); err != nil {
			t.Fatalf("PutIssueDetail(%s) error = %v", namespace, err)
		}
		if err := store.PutIssueComments(ctx, IssueCommentsRecord{
			Namespace:  namespace,
			IssueKey:   "ABC-1",
			MaxResults: 10,
			Comments:   []jira.Comment{{ID: "10001"}},
			SyncedAt:   now.Add(-2 * time.Hour),
			FreshTill:  now.Add(-time.Hour),
		}); err != nil {
			t.Fatalf("PutIssueComments(%s) error = %v", namespace, err)
		}
		if err := store.PutIssueTransitions(ctx, IssueTransitionsRecord{
			Namespace:   namespace,
			IssueKey:    "ABC-1",
			Transitions: []jira.Transition{{ID: "21"}},
			SyncedAt:    now.Add(-2 * time.Hour),
			FreshTill:   now.Add(-time.Hour),
		}); err != nil {
			t.Fatalf("PutIssueTransitions(%s) error = %v", namespace, err)
		}
		if err := store.PutIssueEditMetadata(ctx, IssueEditMetadataRecord{
			Namespace: namespace,
			IssueKey:  "ABC-1",
			Metadata:  jira.EditMetadata{Summary: jira.EditField{ID: "summary"}},
			SyncedAt:  now.Add(-2 * time.Hour),
			FreshTill: now.Add(-time.Hour),
		}); err != nil {
			t.Fatalf("PutIssueEditMetadata(%s) error = %v", namespace, err)
		}
		if err := store.PutCreateIssueTypes(ctx, CreateIssueTypesRecord{
			Namespace:  namespace,
			ProjectKey: "ABC",
			IssueTypes: []jira.CreateIssueType{{ID: "10001"}},
			SyncedAt:   now.Add(-2 * time.Hour),
			FreshTill:  now.Add(-time.Hour),
		}); err != nil {
			t.Fatalf("PutCreateIssueTypes(%s) error = %v", namespace, err)
		}
		if err := store.PutCreateFields(ctx, CreateFieldsRecord{
			Namespace:   namespace,
			ProjectKey:  "ABC",
			IssueTypeID: "10001",
			Fields:      []jira.CreateField{{ID: "summary"}},
			SyncedAt:    now.Add(-2 * time.Hour),
			FreshTill:   now.Add(-time.Hour),
		}); err != nil {
			t.Fatalf("PutCreateFields(%s) error = %v", namespace, err)
		}
		if err := store.PutExpandedChildren(ctx, ExpandedChildrenRecord{
			Namespace: namespace,
			ParentKey: "ABC-1",
			Mode:      "open",
			Issues:    []jira.Issue{{Key: "ABC-2"}},
			SyncedAt:  now.Add(-2 * time.Hour),
			FreshTill: now.Add(-time.Hour),
		}); err != nil {
			t.Fatalf("PutExpandedChildren(%s) error = %v", namespace, err)
		}
	}

	oldUpdatedAt := now.Add(-8 * 24 * time.Hour).UnixNano()
	for _, table := range []string{
		"active_views",
		"issue_details",
		"issue_comments",
		"issue_transitions",
		"issue_edit_metadata",
		"create_issue_types",
		"create_fields",
		"expanded_children",
	} {
		if _, err := store.db.ExecContext(ctx, "UPDATE "+table+" SET updated_at_unix_nano = ? WHERE namespace = ?", oldUpdatedAt, oldSite); err != nil {
			t.Fatalf("age %s rows: %v", table, err)
		}
	}

	deleted, err := store.DeleteRowsUpdatedBefore(ctx, now.Add(-7*24*time.Hour))
	if err != nil {
		t.Fatalf("DeleteRowsUpdatedBefore() error = %v", err)
	}
	if deleted != 8 {
		t.Fatalf("deleted rows = %d, want 8", deleted)
	}

	assertMissing := func(name string, ok bool, err error) {
		t.Helper()
		if err != nil || ok {
			t.Fatalf("%s old row ok=%v err=%v", name, ok, err)
		}
	}
	assertPresent := func(name string, ok bool, err error) {
		t.Helper()
		if err != nil || !ok {
			t.Fatalf("%s recent row ok=%v err=%v", name, ok, err)
		}
	}

	_, ok, err := store.GetActiveView(ctx, oldSite, "project = ABC")
	assertMissing("active view", ok, err)
	_, ok, err = store.GetIssueDetail(ctx, oldSite, "ABC-1")
	assertMissing("issue detail", ok, err)
	_, ok, err = store.GetIssueComments(ctx, oldSite, "ABC-1", 10)
	assertMissing("issue comments", ok, err)
	_, ok, err = store.GetIssueTransitions(ctx, oldSite, "ABC-1")
	assertMissing("issue transitions", ok, err)
	_, ok, err = store.GetIssueEditMetadata(ctx, oldSite, "ABC-1")
	assertMissing("issue edit metadata", ok, err)
	_, ok, err = store.GetCreateIssueTypes(ctx, oldSite, "ABC")
	assertMissing("create issue types", ok, err)
	_, ok, err = store.GetCreateFields(ctx, oldSite, "ABC", "10001")
	assertMissing("create fields", ok, err)
	_, ok, err = store.GetExpandedChildren(ctx, oldSite, "ABC-1", "open")
	assertMissing("expanded children", ok, err)

	_, ok, err = store.GetActiveView(ctx, recentSite, "project = ABC")
	assertPresent("active view", ok, err)
	_, ok, err = store.GetIssueDetail(ctx, recentSite, "ABC-1")
	assertPresent("issue detail", ok, err)
	_, ok, err = store.GetIssueComments(ctx, recentSite, "ABC-1", 10)
	assertPresent("issue comments", ok, err)
	_, ok, err = store.GetIssueTransitions(ctx, recentSite, "ABC-1")
	assertPresent("issue transitions", ok, err)
	_, ok, err = store.GetIssueEditMetadata(ctx, recentSite, "ABC-1")
	assertPresent("issue edit metadata", ok, err)
	_, ok, err = store.GetCreateIssueTypes(ctx, recentSite, "ABC")
	assertPresent("create issue types", ok, err)
	_, ok, err = store.GetCreateFields(ctx, recentSite, "ABC", "10001")
	assertPresent("create fields", ok, err)
	_, ok, err = store.GetExpandedChildren(ctx, recentSite, "ABC-1", "open")
	assertPresent("expanded children", ok, err)
}

func TestStorePersistsIssueEditMetadataRecords(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	syncedAt := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	record := IssueEditMetadataRecord{
		Namespace: "https://example.atlassian.net",
		IssueKey:  "ABC-1",
		Metadata: jira.EditMetadata{
			Summary: jira.EditField{ID: "summary", Name: "Summary", Editable: true},
			Priority: jira.EditField{
				ID:            "priority",
				Name:          "Priority",
				Editable:      true,
				AllowedValues: []jira.FieldOption{{ID: "2", Name: "High"}},
			},
		},
		SyncedAt:  syncedAt,
		FreshTill: syncedAt.Add(time.Minute),
	}
	if err := store.PutIssueEditMetadata(ctx, record); err != nil {
		t.Fatalf("PutIssueEditMetadata() error = %v", err)
	}

	got, ok, err := store.GetIssueEditMetadata(ctx, record.Namespace, record.IssueKey)
	if err != nil {
		t.Fatalf("GetIssueEditMetadata() error = %v", err)
	}
	if !ok {
		t.Fatal("expected issue edit metadata record")
	}
	if !got.Metadata.Summary.Editable || len(got.Metadata.Priority.AllowedValues) != 1 || got.Metadata.Priority.AllowedValues[0].Name != "High" {
		t.Fatalf("Metadata = %#v", got.Metadata)
	}
	if !got.SyncedAt.Equal(record.SyncedAt) || !got.FreshTill.Equal(record.FreshTill) {
		t.Fatalf("timestamps = %s/%s", got.SyncedAt, got.FreshTill)
	}
}

func TestStorePersistsCreateIssueTypesRecords(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	syncedAt := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	record := CreateIssueTypesRecord{
		Namespace:  "https://example.atlassian.net",
		ProjectKey: "ABC",
		IssueTypes: []jira.CreateIssueType{{ID: "10001", Name: "Task"}},
		SyncedAt:   syncedAt,
		FreshTill:  syncedAt.Add(time.Minute),
	}
	if err := store.PutCreateIssueTypes(ctx, record); err != nil {
		t.Fatalf("PutCreateIssueTypes() error = %v", err)
	}

	got, ok, err := store.GetCreateIssueTypes(ctx, record.Namespace, record.ProjectKey)
	if err != nil {
		t.Fatalf("GetCreateIssueTypes() error = %v", err)
	}
	if !ok {
		t.Fatal("expected create issue types record")
	}
	if len(got.IssueTypes) != 1 || got.IssueTypes[0].ID != "10001" || got.IssueTypes[0].Name != "Task" {
		t.Fatalf("IssueTypes = %#v", got.IssueTypes)
	}
	if !got.SyncedAt.Equal(record.SyncedAt) || !got.FreshTill.Equal(record.FreshTill) {
		t.Fatalf("timestamps = %s/%s", got.SyncedAt, got.FreshTill)
	}
}

func TestStorePersistsCreateFieldsRecords(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	syncedAt := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	record := CreateFieldsRecord{
		Namespace:   "https://example.atlassian.net",
		ProjectKey:  "ABC",
		IssueTypeID: "10001",
		Fields: []jira.CreateField{{
			ID:       "components",
			Name:     "Components",
			Required: true,
			AllowedValues: []jira.FieldOption{
				{ID: "20001", Name: "csp_gateway"},
			},
		}},
		SyncedAt:  syncedAt,
		FreshTill: syncedAt.Add(time.Minute),
	}
	if err := store.PutCreateFields(ctx, record); err != nil {
		t.Fatalf("PutCreateFields() error = %v", err)
	}

	got, ok, err := store.GetCreateFields(ctx, record.Namespace, record.ProjectKey, record.IssueTypeID)
	if err != nil {
		t.Fatalf("GetCreateFields() error = %v", err)
	}
	if !ok {
		t.Fatal("expected create fields record")
	}
	if len(got.Fields) != 1 || got.Fields[0].ID != "components" || len(got.Fields[0].AllowedValues) != 1 || got.Fields[0].AllowedValues[0].Name != "csp_gateway" {
		t.Fatalf("Fields = %#v", got.Fields)
	}
	if !got.SyncedAt.Equal(record.SyncedAt) || !got.FreshTill.Equal(record.FreshTill) {
		t.Fatalf("timestamps = %s/%s", got.SyncedAt, got.FreshTill)
	}
}

func TestStorePersistsExpandedChildrenRecords(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "cache.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	syncedAt := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	record := ExpandedChildrenRecord{
		Namespace: "https://example.atlassian.net",
		ParentKey: "ABC-1",
		Mode:      "open",
		Issues: []jira.Issue{
			{Key: "ABC-2", Summary: "Cached child", ParentKey: "ABC-1"},
		},
		SyncedAt:  syncedAt,
		FreshTill: syncedAt.Add(time.Minute),
	}
	if err := store.PutExpandedChildren(ctx, record); err != nil {
		t.Fatalf("PutExpandedChildren() error = %v", err)
	}

	got, ok, err := store.GetExpandedChildren(ctx, record.Namespace, record.ParentKey, record.Mode)
	if err != nil {
		t.Fatalf("GetExpandedChildren() error = %v", err)
	}
	if !ok {
		t.Fatal("expected expanded children record")
	}
	if len(got.Issues) != 1 || got.Issues[0].Key != "ABC-2" || got.Issues[0].Summary != "Cached child" {
		t.Fatalf("Issues = %#v", got.Issues)
	}
	if !got.SyncedAt.Equal(record.SyncedAt) || !got.FreshTill.Equal(record.FreshTill) {
		t.Fatalf("timestamps = %s/%s", got.SyncedAt, got.FreshTill)
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
