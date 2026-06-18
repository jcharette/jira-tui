package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/jcharette/jira-tui/internal/cache"
	"github.com/jcharette/jira-tui/internal/claude"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/jcharette/jira-tui/internal/worker"
)

func withLinkActions(t *testing.T, open func(string) error, copy func(string) error) {
	t.Helper()
	previousOpen := openExternal
	previousCopy := copyToClipboard
	openExternal = open
	copyToClipboard = copy
	t.Cleanup(func() {
		openExternal = previousOpen
		copyToClipboard = previousCopy
	})
}

func focusDetailSectionForTest(t *testing.T, model *Model, title string) {
	t.Helper()
	model.jumpDetailSection(title)
	section, ok := model.focusedDetailSection()
	if !ok || !strings.EqualFold(section.Label, title) {
		t.Fatalf("focused detail section = %#v, ok=%v, want %q", section, ok, title)
	}
}

func assertFocusedDetailSection(t *testing.T, model Model, title string) {
	t.Helper()
	section, ok := model.focusedDetailSection()
	if !ok || !strings.EqualFold(section.Label, title) {
		t.Fatalf("focused detail section = %#v, ok=%v, want %q", section, ok, title)
	}
}

func lineContaining(value string, needle string) string {
	for _, line := range strings.Split(value, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

func dialogBorderWidth(value string) int {
	for _, line := range strings.Split(value, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "╭") && strings.Contains(trimmed, "╮") {
			return lipgloss.Width(trimmed)
		}
	}
	return 0
}

func visibleColumn(value string, needle string) int {
	index := strings.Index(value, needle)
	if index < 0 {
		return -1
	}
	return lipgloss.Width(value[:index])
}

func minPositiveVisibleColumn(value string, needles ...string) int {
	best := -1
	for _, needle := range needles {
		column := visibleColumn(value, needle)
		if column >= 0 && (best < 0 || column < best) {
			best = column
		}
	}
	return best
}

func TestCommentsSectionFooterShowsAddCommentAction(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story"}}
	model.comments = map[string][]jira.Comment{"ABC-1": {}}
	focusDetailSectionForTest(t, &model, "Comments")

	footer := model.renderModelFooterHelp(model.browserLayout(model.width))

	if !strings.Contains(footer, "enter add") {
		t.Fatalf("comments footer should show add action, got %q", footer)
	}
}

func TestCommentsSectionEnterStartsCommentComposer(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story"}}
	model.comments = map[string][]jira.Comment{"ABC-1": {}}
	focusDetailSectionForTest(t, &model, "Comments")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("comment composer should open locally without submitting work")
	}
	if next.mode != modeComment {
		t.Fatalf("mode = %v, want comment", next.mode)
	}
	if activeKeyContext(next) != keyContextComment {
		t.Fatalf("activeKeyContext = %q", activeKeyContext(next))
	}
}

func TestExpandSelectedIssueLoadsOpenChildren(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.width = 120
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Parent", IssueType: "Story"}}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "x", Code: 'x'}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("expected expand command")
	}
	if !next.expandLoading || next.expandRequestKey != "ABC-1" || next.expandMode != worker.ExpandModeOpen {
		t.Fatalf("expand state loading=%v key=%q mode=%q", next.expandLoading, next.expandRequestKey, next.expandMode)
	}
	if !strings.Contains(next.detailNotice, "Loading open children") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}

	updated, _ = next.Update(workerResultMsg{result: worker.Result{
		ID:   next.activeExpandReqID,
		Kind: worker.KindExpandIssues,
		ExpandIssues: &worker.ExpandIssuesResult{
			ParentKey: "ABC-1",
			Mode:      worker.ExpandModeOpen,
			Issues: []jira.Issue{
				{Key: "ABC-2", Summary: "Child", Status: "To Do", IssueType: "Task", ParentKey: "ABC-1"},
			},
		},
	}})
	next = updated.(Model)

	if next.expandLoading {
		t.Fatal("expected expand loading to clear")
	}
	if len(next.issues) != 2 || next.issues[1].Key != "ABC-2" {
		t.Fatalf("issues = %#v", next.issues)
	}
	if next.selected != 0 || next.issues[next.selected].Key != "ABC-1" {
		t.Fatalf("selected issue = %d %#v", next.selected, next.issues)
	}
	if !strings.Contains(next.detailNotice, "Loaded 1 open children") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
	view := next.render()
	for _, want := range []string{"ABC-2", "Notice", "Loaded 1 open children"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestExpandSelectedIssueUsesFreshPersistentChildren(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.Local)
	store := newFakeActiveViewStore()
	store.expandedChildren = cache.ExpandedChildrenRecord{
		Namespace: "https://example.atlassian.net",
		ParentKey: "ABC-1",
		Mode:      string(worker.ExpandModeOpen),
		Issues: []jira.Issue{
			{Key: "ABC-2", Summary: "Cached child", Status: "To Do", IssueType: "Task", ParentKey: "ABC-1"},
		},
		SyncedAt:  now.Add(-10 * time.Second),
		FreshTill: now.Add(time.Minute),
	}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithActiveViewStore(store, "https://example.atlassian.net"),
	)
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = false
	model.width = 120
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Parent", IssueType: "Story"}}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "x", Code: 'x'}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("fresh persistent expanded children should not submit Jira work")
	}
	if next.expandLoading {
		t.Fatal("fresh persistent expanded children should not show loading")
	}
	if len(next.issues) != 2 || next.issues[1].Key != "ABC-2" {
		t.Fatalf("issues = %#v", next.issues)
	}
	if !strings.Contains(next.detailNotice, "Loaded 1 open children") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestExpandIssuesResultPersistsChildren(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.Local)
	store := newFakeActiveViewStore()
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithActiveViewStore(store, "https://example.atlassian.net"),
	)
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = false
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Parent", IssueType: "Story"}}
	model.activeExpandReqID = 12
	model.expandRequestKey = "ABC-1"
	model.expandMode = worker.ExpandModeOpen
	model.expandLoading = true

	updated, _ := model.Update(workerResultMsg{result: worker.Result{
		ID:   12,
		Kind: worker.KindExpandIssues,
		ExpandIssues: &worker.ExpandIssuesResult{
			ParentKey: "ABC-1",
			Mode:      worker.ExpandModeOpen,
			Issues: []jira.Issue{
				{Key: "ABC-2", Summary: "Child", Status: "To Do", IssueType: "Task", ParentKey: "ABC-1"},
			},
			SyncedAt: now,
		},
	}})
	next := updated.(Model)

	if next.expandLoading {
		t.Fatal("expected expand loading to clear")
	}
	if store.putExpandedChildren.Namespace != "https://example.atlassian.net" || store.putExpandedChildren.ParentKey != "ABC-1" || store.putExpandedChildren.Mode != string(worker.ExpandModeOpen) {
		t.Fatalf("putExpandedChildren = %#v", store.putExpandedChildren)
	}
	if len(store.putExpandedChildren.Issues) != 1 || store.putExpandedChildren.Issues[0].Key != "ABC-2" {
		t.Fatalf("persisted children = %#v", store.putExpandedChildren.Issues)
	}
	if !store.putExpandedChildren.SyncedAt.Equal(now) || !store.putExpandedChildren.FreshTill.Equal(now.Add(expandedChildrenCacheTTL)) {
		t.Fatalf("timestamps = %s/%s", store.putExpandedChildren.SyncedAt, store.putExpandedChildren.FreshTill)
	}
}

func TestExpandSelectedIssueLoadsAllChildren(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Parent", IssueType: "Story"}}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "X", Code: 'X'}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("expected expand command")
	}
	if next.expandMode != worker.ExpandModeAll {
		t.Fatalf("expandMode = %q", next.expandMode)
	}
	if !strings.Contains(next.detailNotice, "Loading all children") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestEnterStartsSelectedIssueDetailRequest(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.issues = []jira.Issue{{Key: "ABC-1"}}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("expected detail request command")
	}
	if next.mode != modeDetail {
		t.Fatalf("mode = %v", next.mode)
	}
	if !next.detailLoading {
		t.Fatal("detailLoading should be true")
	}
	if next.detailRequestKey != "ABC-1" {
		t.Fatalf("detailRequestKey = %q", next.detailRequestKey)
	}
}

func TestLoadedDetailIgnoresStaleSelection(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.issues = []jira.Issue{{Key: "ABC-1"}, {Key: "ABC-2"}}
	model.selected = 1
	model.activeDetailRequestID = 4
	model.detailRequestKey = "ABC-1"
	model.detailLoading = true

	updated, _ := model.Update(workerResultMsg{
		result: worker.Result{
			ID:   4,
			Kind: worker.KindGetIssue,
			GetIssue: &worker.GetIssueResult{
				Key:    "ABC-1",
				Detail: jira.IssueDetail{Issue: jira.Issue{Key: "ABC-1"}},
			},
		},
	})
	next := updated.(Model)

	if _, ok := next.details["ABC-1"]; ok {
		t.Fatal("stale detail should not be cached")
	}
	if !next.detailLoading {
		t.Fatal("stale detail should not clear loading for current request")
	}
}

func TestFreshCachedDetailSkipsDetailRefresh(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = false
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.cacheIssueDetail("ABC-1", jira.IssueDetail{Issue: jira.Issue{Key: "ABC-1"}, Description: "Cached detail"}, now)
	model.cacheIssueComments("ABC-1", nil, now)

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("fresh cached detail should not submit background work")
	}
	if next.detailLoading {
		t.Fatal("detailLoading should be false")
	}
	if next.detailRequestKey != "" {
		t.Fatalf("detailRequestKey = %q", next.detailRequestKey)
	}
}

func TestStaleCachedDetailStartsBackgroundRefresh(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = false
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.cacheIssueDetail("ABC-1", jira.IssueDetail{Issue: jira.Issue{Key: "ABC-1"}, Description: "Stale detail"}, now.Add(-2*issueDetailCacheTTL))
	model.cacheIssueComments("ABC-1", nil, now)

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("stale cached detail should submit background refresh")
	}
	if !next.detailLoading {
		t.Fatal("detailLoading should be true while stale detail refreshes")
	}
	if next.detailRequestKey != "ABC-1" {
		t.Fatalf("detailRequestKey = %q", next.detailRequestKey)
	}
	if next.details["ABC-1"].Description != "Stale detail" {
		t.Fatalf("stale detail should remain visible, details = %#v", next.details["ABC-1"])
	}
}

func TestFreshCachedCommentsSkipCommentsRefresh(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = false
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.cacheIssueDetail("ABC-1", jira.IssueDetail{Issue: jira.Issue{Key: "ABC-1"}, Description: "Cached detail"}, now)
	model.cacheIssueComments("ABC-1", []jira.Comment{{ID: "10001", Body: "Cached comment"}}, now)

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("fresh cached comments should not submit background work")
	}
	if next.commentsLoading {
		t.Fatal("commentsLoading should be false")
	}
	if next.commentsRequestKey != "" {
		t.Fatalf("commentsRequestKey = %q", next.commentsRequestKey)
	}
}

func TestStaleCachedCommentsStayVisibleWhileRefreshing(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = false
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.cacheIssueDetail("ABC-1", jira.IssueDetail{Issue: jira.Issue{Key: "ABC-1"}, Description: "Cached detail"}, now)
	model.cacheIssueComments("ABC-1", []jira.Comment{{ID: "10001", Body: "Stale comment"}}, now.Add(-2*issueCommentsCacheTTL))

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("stale cached comments should submit background refresh")
	}
	if !next.commentsLoading {
		t.Fatal("commentsLoading should be true while stale comments refresh")
	}
	if next.commentsRequestKey != "ABC-1" {
		t.Fatalf("commentsRequestKey = %q", next.commentsRequestKey)
	}
	if next.comments["ABC-1"][0].Body != "Stale comment" {
		t.Fatalf("stale comments should remain visible, comments = %#v", next.comments["ABC-1"])
	}
}

func TestPersistentDetailAndCommentsHydrateOnDetailOpen(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	store := newFakeActiveViewStore()
	store.detail = cache.IssueDetailRecord{
		Namespace: "https://example.atlassian.net",
		IssueKey:  "ABC-1",
		Detail:    jira.IssueDetail{Issue: jira.Issue{Key: "ABC-1"}, Description: "Persistent detail"},
		SyncedAt:  now.Add(-10 * time.Second),
		FreshTill: now.Add(time.Minute),
	}
	store.comments = cache.IssueCommentsRecord{
		Namespace:  "https://example.atlassian.net",
		IssueKey:   "ABC-1",
		MaxResults: maxComments,
		Comments:   []jira.Comment{{ID: "10001", Body: "Persistent comment"}},
		SyncedAt:   now.Add(-10 * time.Second),
		FreshTill:  now.Add(time.Minute),
	}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithActiveViewStore(store, "https://example.atlassian.net"),
	)
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = false
	model.issues = []jira.Issue{{Key: "ABC-1"}}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("fresh persistent detail/comments should not submit background work")
	}
	if next.detailLoading || next.commentsLoading {
		t.Fatalf("detailLoading=%v commentsLoading=%v", next.detailLoading, next.commentsLoading)
	}
	if next.details["ABC-1"].Description != "Persistent detail" {
		t.Fatalf("detail = %#v", next.details["ABC-1"])
	}
	if len(next.comments["ABC-1"]) != 1 || next.comments["ABC-1"][0].Body != "Persistent comment" {
		t.Fatalf("comments = %#v", next.comments["ABC-1"])
	}
}

func TestSearchDetailAndCommentsPersistToStore(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	store := newFakeActiveViewStore()
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithActiveViewStore(store, "https://example.atlassian.net"),
	)
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = false
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.activeDetailRequestID = 7
	model.detailRequestKey = "ABC-1"
	model.activeCommentsReqID = 8
	model.commentsRequestKey = "ABC-1"

	updated, _ := model.Update(workerResultMsg{result: worker.Result{
		ID:   7,
		Kind: worker.KindGetIssue,
		GetIssue: &worker.GetIssueResult{
			Key:      "ABC-1",
			Detail:   jira.IssueDetail{Issue: jira.Issue{Key: "ABC-1"}, Description: "Store detail"},
			SyncedAt: now,
		},
	}})
	next := updated.(Model)
	updated, _ = next.Update(workerResultMsg{result: worker.Result{
		ID:   8,
		Kind: worker.KindGetComments,
		GetComments: &worker.GetCommentsResult{
			Key:      "ABC-1",
			Comments: []jira.Comment{{ID: "10001", Body: "Store comment"}},
			SyncedAt: now,
		},
	}})
	next = updated.(Model)

	if store.putDetail.IssueKey != "ABC-1" || store.putDetail.Detail.Description != "Store detail" {
		t.Fatalf("putDetail = %#v", store.putDetail)
	}
	if store.putComments.IssueKey != "ABC-1" || len(store.putComments.Comments) != 1 || store.putComments.Comments[0].Body != "Store comment" {
		t.Fatalf("putComments = %#v", store.putComments)
	}
	if !store.putDetail.FreshTill.Equal(now.Add(issueDetailCacheTTL)) {
		t.Fatalf("detail FreshTill = %s", store.putDetail.FreshTill)
	}
	if !store.putComments.FreshTill.Equal(now.Add(issueCommentsCacheTTL)) {
		t.Fatalf("comments FreshTill = %s", store.putComments.FreshTill)
	}
}

func TestIssueDetailCacheRecordStoresFreshness(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.now = func() time.Time { return now }

	model.cacheIssueDetail("ABC-1", jira.IssueDetail{Issue: jira.Issue{Key: "ABC-1"}, Description: "Cached detail"}, now)

	if !model.isIssueDetailFresh("ABC-1") {
		t.Fatal("detail should be fresh immediately after caching")
	}
	if model.details["ABC-1"].Description != "Cached detail" {
		t.Fatalf("details map = %#v", model.details["ABC-1"])
	}
	model.now = func() time.Time { return now.Add(issueDetailCacheTTL) }
	if model.isIssueDetailFresh("ABC-1") {
		t.Fatal("detail should be stale at the freshness boundary")
	}
}

func TestIssueWritePatchesRetainedIssueCaches(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	store := newFakeActiveViewStore()
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithActiveViewStore(store, "https://example.atlassian.net"),
	)
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.issues = []jira.Issue{{
		Key:       "ABC-1",
		Summary:   "Old summary",
		Status:    "To Do",
		Priority:  "Medium",
		Assignee:  "Old Person",
		IssueType: "Story",
	}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       model.issues[0],
			Description: "Old description",
		},
	}
	model.cacheActiveIssueView(model.jql, model.issues, now)
	model.cacheIssueDetail("ABC-1", model.details["ABC-1"], now)

	model.updateIssueSummary("ABC-1", "New summary")
	model.updateIssuePriority("ABC-1", "High")
	model.updateIssueAssignee("ABC-1", "New Person")
	model.updateIssueStatus("ABC-1", "In Progress")
	model.updateIssueDescription("ABC-1", "New description")

	detailRecord, ok := model.cachedIssueDetail("ABC-1")
	if !ok {
		t.Fatal("expected retained detail cache")
	}
	if detailRecord.Value.Summary != "New summary" || detailRecord.Value.Issue.Summary != "New summary" {
		t.Fatalf("detail summary was not patched: %#v", detailRecord.Value)
	}
	if detailRecord.Value.Priority != "High" || detailRecord.Value.Issue.Priority != "High" {
		t.Fatalf("detail priority was not patched: %#v", detailRecord.Value)
	}
	if detailRecord.Value.Assignee != "New Person" || detailRecord.Value.Issue.Assignee != "New Person" {
		t.Fatalf("detail assignee was not patched: %#v", detailRecord.Value)
	}
	if detailRecord.Value.Status != "In Progress" || detailRecord.Value.Issue.Status != "In Progress" {
		t.Fatalf("detail status was not patched: %#v", detailRecord.Value)
	}
	if detailRecord.Value.Description != "New description" {
		t.Fatalf("detail description was not patched: %#v", detailRecord.Value)
	}

	viewRecord, ok := model.cachedActiveIssueView(model.jql)
	if !ok || len(viewRecord.Issues) != 1 {
		t.Fatalf("active view record ok=%v record=%#v", ok, viewRecord)
	}
	got := viewRecord.Issues[0]
	if got.Summary != "New summary" || got.Priority != "High" || got.Assignee != "New Person" || got.Status != "In Progress" {
		t.Fatalf("active view issue was not patched: %#v", got)
	}
	if store.putDetail.Detail.Description != "New description" || store.putDetail.Detail.Issue.Status != "In Progress" {
		t.Fatalf("persistent detail was not patched: %#v", store.putDetail)
	}
	if len(store.put.Issues) != 1 || store.put.Issues[0].Summary != "New summary" || store.put.Issues[0].Status != "In Progress" {
		t.Fatalf("persistent active view was not patched: %#v", store.put)
	}
}

func TestStatusWriteInvalidatesTransitionCache(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	store := newFakeActiveViewStore()
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithActiveViewStore(store, "https://example.atlassian.net"),
	)
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.cacheIssueTransitions("ABC-1", []jira.Transition{{ID: "21", Name: "Start Progress"}}, now)

	model.updateIssueStatus("ABC-1", "In Progress")

	if _, ok := model.cachedIssueTransitions("ABC-1"); ok {
		t.Fatal("status change should invalidate retained transition options")
	}
	if store.deletedTransitions != "ABC-1" {
		t.Fatalf("deleted persistent transitions = %q", store.deletedTransitions)
	}
}

func TestLoadedDetailMarksDetailCacheFresh(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.activeDetailRequestID = 4
	model.detailRequestKey = "ABC-1"
	model.detailLoading = true

	updated, _ := model.Update(workerResultMsg{
		result: worker.Result{
			ID:   4,
			Kind: worker.KindGetIssue,
			GetIssue: &worker.GetIssueResult{
				Key:    "ABC-1",
				Detail: jira.IssueDetail{Issue: jira.Issue{Key: "ABC-1"}, Description: "Fresh detail"},
			},
		},
	})
	next := updated.(Model)

	if !next.isIssueDetailFresh("ABC-1") {
		t.Fatal("loaded detail should mark cache fresh")
	}
}

func TestHelpOverlayShowsActiveContextBindings(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 60
	model.issues = []jira.Issue{{Key: "ABC-1"}}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "?", Code: '?'}))
	next := updated.(Model)
	if !next.helpOpen {
		t.Fatal("expected help to open")
	}

	view := next.render()
	for _, want := range []string{"Keyboard Help", "Ticket Detail", "Open contextual AI for supported sections", "Open the selected Jira issue"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "esc", Code: tea.KeyEsc}))
	next = updated.(Model)
	if next.helpOpen {
		t.Fatal("expected help to close")
	}
}

func TestHelpOverlayFitsTerminalAndScrolls(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 90
	model.height = 24
	model.issues = []jira.Issue{{Key: "ABC-1"}}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "?", Code: '?'}))
	next := updated.(Model)
	view := next.render()
	if lines := strings.Count(view, "\n") + 1; lines > model.height {
		t.Fatalf("help lines = %d, height = %d\n%s", lines, model.height, view)
	}
	if !strings.Contains(view, "Lines 1-") {
		t.Fatalf("expected pagination indicator in %q", view)
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "pgdown", Code: tea.KeyPgDown}))
	next = updated.(Model)
	if next.helpOffset == 0 {
		t.Fatal("expected helpOffset to advance")
	}
	view = next.render()
	if lines := strings.Count(view, "\n") + 1; lines > model.height {
		t.Fatalf("help lines after scroll = %d, height = %d\n%s", lines, model.height, view)
	}
	for _, want := range []string{"Keyboard Help", "Ticket Detail"} {
		if !strings.Contains(view, want) {
			t.Fatalf("sticky help header missing %q after scroll in %q", want, view)
		}
	}
}

func TestRenderFullDetailShowsPersistentIssueIdentity(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing with a long summary", Status: "In Progress", Priority: "High", IssueType: "Story", Assignee: "A Developer"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       model.issues[0],
			Description: "Use `main.tf`.",
		},
	}

	view := model.render()

	for _, want := range []string{"ABC-1", "In Progress", "Story", "Fix production thing", "Assignee A D.", "Priority High"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing persistent detail identity %q in %q", want, view)
		}
	}
}

func TestRenderFullDetailSeparatesMetadataFromTabs(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "In Progress", Priority: "High", IssueType: "Story", Assignee: "A Developer"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       model.issues[0],
			Description: "Use `main.tf`.",
		},
	}

	view := model.render()
	summaryIndex := strings.Index(view, "Fix production thing")
	metaIndex := strings.Index(view, "Priority High")
	tabsIndex := strings.Index(view, "Description")
	if summaryIndex < 0 || metaIndex < 0 || tabsIndex < 0 {
		t.Fatalf("expected summary, metadata, and tabs in %q", view)
	}
	lines := strings.Split(view, "\n")
	summaryLine, metaLine := -1, -1
	for index, line := range lines {
		if strings.Contains(line, "Fix production thing") {
			summaryLine = index
		}
		if strings.Contains(line, "Priority High") {
			metaLine = index
		}
	}
	if summaryLine < 0 || metaLine < 0 || metaLine-summaryLine != 1 {
		t.Fatalf("expected metadata directly after summary in %q", view)
	}
	if !(metaIndex < tabsIndex) {
		t.Fatalf("expected tabs after metadata: meta=%d tabs=%d view=%q", metaIndex, tabsIndex, view)
	}
}

type fakeIssueSearcher struct {
	issues                 []jira.Issue
	searchResults          map[string][]jira.Issue
	searches               []string
	detail                 jira.IssueDetail
	comments               []jira.Comment
	addedComment           jira.Comment
	addedBody              string
	addMentions            []jira.Mention
	users                  []jira.User
	transitions            []jira.Transition
	transitionKey          string
	transitionID           string
	transitionRequest      jira.TransitionIssueRequest
	editMetadata           jira.EditMetadata
	createIssueTypes       []jira.CreateIssueType
	createFields           []jira.CreateField
	createIssueRequest     jira.CreateIssueRequest
	createdIssue           jira.Issue
	updateSummaryKey       string
	updateSummaryValue     string
	updateDescriptionKey   string
	updateDescriptionValue string
	updatePriorityKey      string
	updatePriorityValue    jira.FieldOption
	updateAssigneeKey      string
	updateAssigneeValue    jira.User
	err                    error
}

func (f *fakeIssueSearcher) SearchIssues(_ context.Context, jql string, _ int) ([]jira.Issue, error) {
	f.searches = append(f.searches, jql)
	if f.err != nil {
		return nil, f.err
	}
	if f.searchResults != nil {
		if issues, ok := f.searchResults[jql]; ok {
			return issues, nil
		}
	}
	if f.issues != nil {
		return f.issues, nil
	}
	return []jira.Issue{{Key: "ABC-1"}}, nil
}

func (f *fakeIssueSearcher) GetIssue(_ context.Context, key string) (jira.IssueDetail, error) {
	if f.err != nil {
		return jira.IssueDetail{}, f.err
	}
	if f.detail.Key != "" {
		return f.detail, nil
	}
	return jira.IssueDetail{Issue: jira.Issue{Key: key, Summary: "Loaded detail"}}, nil
}

func (f *fakeIssueSearcher) GetComments(_ context.Context, key string, _ int) ([]jira.Comment, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.comments != nil {
		return f.comments, nil
	}
	return []jira.Comment{{ID: "10001", Author: "Comment Person", Body: key}}, nil
}

func (f *fakeIssueSearcher) AddComment(_ context.Context, key string, body string, mentions []jira.Mention) (jira.Comment, error) {
	if f.err != nil {
		return jira.Comment{}, f.err
	}
	f.addedBody = body
	f.addMentions = mentions
	if f.addedComment.ID != "" {
		return f.addedComment, nil
	}
	return jira.Comment{ID: "10002", Author: "Current User", Body: body}, nil
}

func (f *fakeIssueSearcher) SearchUsers(_ context.Context, query string, _ int) ([]jira.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.users != nil {
		return f.users, nil
	}
	return []jira.User{{AccountID: "abc-123", DisplayName: query}}, nil
}

func (f *fakeIssueSearcher) GetTransitions(_ context.Context, key string) ([]jira.Transition, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.transitionKey = key
	if f.transitions != nil {
		return f.transitions, nil
	}
	return []jira.Transition{{ID: "21", Name: "Start Progress", ToStatus: "In Progress"}}, nil
}

func (f *fakeIssueSearcher) TransitionIssue(_ context.Context, key string, request jira.TransitionIssueRequest) error {
	if f.err != nil {
		return f.err
	}
	f.transitionKey = key
	f.transitionID = request.TransitionID
	f.transitionRequest = request
	return nil
}

func (f *fakeIssueSearcher) GetEditMetadata(_ context.Context, key string) (jira.EditMetadata, error) {
	if f.err != nil {
		return jira.EditMetadata{}, f.err
	}
	f.transitionKey = key
	if f.editMetadata.Summary.ID != "" || f.editMetadata.Summary.Editable || f.editMetadata.Priority.ID != "" || f.editMetadata.Priority.Editable {
		return f.editMetadata, nil
	}
	return jira.EditMetadata{Summary: jira.EditField{ID: "summary", Name: "Summary", Editable: true}}, nil
}

func (f *fakeIssueSearcher) GetCreateIssueTypes(_ context.Context, projectKey string) ([]jira.CreateIssueType, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.transitionKey = projectKey
	return f.createIssueTypes, nil
}

func (f *fakeIssueSearcher) GetCreateFields(_ context.Context, projectKey string, issueTypeID string) ([]jira.CreateField, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.transitionKey = projectKey
	f.transitionID = issueTypeID
	return f.createFields, nil
}

func (f *fakeIssueSearcher) CreateIssue(_ context.Context, request jira.CreateIssueRequest) (jira.Issue, error) {
	if f.err != nil {
		return jira.Issue{}, f.err
	}
	f.createIssueRequest = request
	if f.createdIssue.Key != "" {
		return f.createdIssue, nil
	}
	return jira.Issue{Key: "ABC-123", Summary: request.Summary}, nil
}

func (f *fakeIssueSearcher) UpdateSummary(_ context.Context, key string, summary string) error {
	if f.err != nil {
		return f.err
	}
	f.updateSummaryKey = key
	f.updateSummaryValue = summary
	return nil
}

func (f *fakeIssueSearcher) UpdateDescription(_ context.Context, key string, description string) error {
	if f.err != nil {
		return f.err
	}
	f.updateDescriptionKey = key
	f.updateDescriptionValue = description
	return nil
}

func (f *fakeIssueSearcher) UpdatePriority(_ context.Context, key string, priority jira.FieldOption) error {
	if f.err != nil {
		return f.err
	}
	f.updatePriorityKey = key
	f.updatePriorityValue = priority
	return nil
}

func (f *fakeIssueSearcher) UpdateAssignee(_ context.Context, key string, assignee jira.User) error {
	if f.err != nil {
		return f.err
	}
	f.updateAssigneeKey = key
	f.updateAssigneeValue = assignee
	return nil
}

type fakeClaudeRunner struct {
	request       claude.Request
	result        claude.Result
	err           error
	waitForCancel bool
	events        []claude.Event
}

func (f *fakeClaudeRunner) Run(ctx context.Context, request claude.Request) (claude.Result, error) {
	f.request = request
	for _, event := range f.events {
		if request.Progress != nil {
			request.Progress(event)
		}
	}
	if f.waitForCancel {
		<-ctx.Done()
		return claude.Result{}, ctx.Err()
	}
	if f.err != nil {
		return claude.Result{}, f.err
	}
	return f.result, nil
}
