package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/jcharette/jira-tui/internal/cache"
	"github.com/jcharette/jira-tui/internal/config"
	"github.com/jcharette/jira-tui/internal/events"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/jcharette/jira-tui/internal/ui"
	"github.com/jcharette/jira-tui/internal/worker"
)

func collectEventsForTest(t *testing.T, received <-chan events.Event, count int) []events.Event {
	t.Helper()
	got := make([]events.Event, 0, count)
	deadline := time.After(time.Second)
	for len(got) < count {
		select {
		case event := <-received:
			got = append(got, event)
		case <-deadline:
			t.Fatalf("timed out waiting for %d events, got %#v", count, got)
		}
	}
	return got
}

func TestTerminalIssueStatusMatchesCommonTerminalStatuses(t *testing.T) {
	for _, status := range []string{"Done", "Resolved", "Closed", "Canceled", "Cancelled", "done - deployed"} {
		if !terminalIssueStatus(status) {
			t.Fatalf("status %q should be terminal", status)
		}
	}
}

func TestTerminalIssueStatusKeepsActiveStatusesVisible(t *testing.T) {
	for _, status := range []string{"", "Unknown", "To Do", "Open", "Ready", "In Progress", "Review", "Blocked", "Waiting", "On Hold"} {
		if terminalIssueStatus(status) {
			t.Fatalf("status %q should remain active", status)
		}
	}
}

func TestLoadedIssuesIgnoreStaleRequest(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithDisplay(config.Display{SymbolMode: "symbols"}))
	defer model.workers.Stop()
	model.loading = false
	model.activeRequestID = 2
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Keep me"}}

	updated, _ := model.Update(workerResultMsg{
		result: worker.Result{
			ID:   1,
			Kind: worker.KindSearchIssues,
			SearchIssues: &worker.SearchIssuesResult{
				Issues:   []jira.Issue{{Key: "ABC-2", Summary: "Stale"}},
				SyncedAt: time.Now(),
			},
		},
	})
	next := updated.(Model)

	if len(next.issues) != 1 || next.issues[0].Key != "ABC-1" {
		t.Fatalf("stale response replaced issues: %#v", next.issues)
	}
}

func TestLoadedIssuesPreserveSelectedIssue(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.refreshing = true
	model.activeRequestID = 2
	model.selected = 1
	model.issues = []jira.Issue{
		{Key: "ABC-1"},
		{Key: "ABC-2"},
	}

	updated, _ := model.Update(workerResultMsg{
		result: worker.Result{
			ID:   2,
			Kind: worker.KindSearchIssues,
			SearchIssues: &worker.SearchIssuesResult{
				Issues: []jira.Issue{
					{Key: "ABC-2"},
					{Key: "ABC-3"},
				},
				SyncedAt: time.Now(),
			},
		},
	})
	next := updated.(Model)

	if next.loading {
		t.Fatal("loading should be false")
	}
	if next.refreshing {
		t.Fatal("refreshing should be false")
	}
	if next.selected != 0 {
		t.Fatalf("selected = %d", next.selected)
	}
}

func TestSearchResultDoesNotPrefetchSelectedIssueComments(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = true
	model.activeRequestID = 7
	model.cacheIssueDetail("ABC-1", jira.IssueDetail{Issue: jira.Issue{Key: "ABC-1"}, Description: "Cached detail"}, now)

	updated, _ := model.Update(workerResultMsg{
		result: worker.Result{
			ID:   7,
			Kind: worker.KindSearchIssues,
			SearchIssues: &worker.SearchIssuesResult{
				Issues:   []jira.Issue{{Key: "ABC-1", Summary: "Loaded issue"}},
				SyncedAt: now,
			},
		},
	})
	next := updated.(Model)

	if next.commentsLoading {
		t.Fatal("commentsLoading should remain false after list refresh")
	}
	if next.commentsRequestKey != "" {
		t.Fatalf("commentsRequestKey = %q", next.commentsRequestKey)
	}
}

func TestSearchResultSkipsMissingDetailPrefetchForLargeView(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = true
	model.activeRequestID = 7
	issues := make([]jira.Issue, 0, maxIssues)
	for index := 0; index < maxIssues; index++ {
		issues = append(issues, jira.Issue{Key: fmt.Sprintf("ABC-%d", index+1)})
	}

	updated, _ := model.Update(workerResultMsg{
		result: worker.Result{
			ID:   7,
			Kind: worker.KindSearchIssues,
			SearchIssues: &worker.SearchIssuesResult{
				Issues:   issues,
				SyncedAt: now,
			},
		},
	})
	next := updated.(Model)

	if next.detailLoading {
		t.Fatal("detailLoading should remain false after large list refresh")
	}
	if next.detailRequestKey != "" {
		t.Fatalf("detailRequestKey = %q", next.detailRequestKey)
	}
	if next.commentsLoading {
		t.Fatal("commentsLoading should remain false after large list refresh")
	}
}

func TestSearchResultPrefetchesMissingSelectedDetailForSmallView(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = true
	model.activeRequestID = 7

	updated, _ := model.Update(workerResultMsg{
		result: worker.Result{
			ID:   7,
			Kind: worker.KindSearchIssues,
			SearchIssues: &worker.SearchIssuesResult{
				Issues: []jira.Issue{
					{Key: "ABC-1", Summary: "Selected issue"},
					{Key: "ABC-2", Summary: "Second issue"},
				},
				SyncedAt: now,
			},
		},
	})
	next := updated.(Model)

	if !next.detailLoading {
		t.Fatal("detailLoading should be true while selected detail prefetches")
	}
	if next.detailRequestKey != "ABC-1" {
		t.Fatalf("detailRequestKey = %q", next.detailRequestKey)
	}
	if next.commentsLoading {
		t.Fatal("commentsLoading should remain false after list refresh")
	}
}

func TestTableNavigationSkipsMissingDetailPrefetchForLargeView(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	for index := 0; index < maxIssues; index++ {
		model.issues = append(model.issues, jira.Issue{Key: fmt.Sprintf("ABC-%d", index+1)})
	}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	next := updated.(Model)

	if next.selected != 1 {
		t.Fatalf("selected = %d", next.selected)
	}
	if cmd != nil {
		t.Fatal("large table navigation should not submit missing detail prefetch")
	}
	if next.detailLoading {
		t.Fatal("detailLoading should remain false after large table navigation")
	}
	if next.commentsLoading {
		t.Fatal("commentsLoading should remain false after large table navigation")
	}
}

func TestSwitchViewUsesFreshCachedIssueViewWithoutRefresh(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.Local)
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithViews([]config.IssueView{
			{Name: "Assigned", JQL: "assignee = currentUser()"},
			{Name: "Sprint", JQL: "sprint in openSprints()"},
		}, "Assigned"),
	)
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = false
	model.cacheActiveIssueView("sprint in openSprints()", []jira.Issue{{Key: "ABC-9", Summary: "Cached sprint"}}, now.Add(-10*time.Second))

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "]", Code: ']'}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("fresh cached view should not submit a refresh command")
	}
	if next.loading || next.refreshing || next.viewStale {
		t.Fatalf("loading=%v refreshing=%v viewStale=%v", next.loading, next.refreshing, next.viewStale)
	}
	if len(next.issues) != 1 || next.issues[0].Key != "ABC-9" {
		t.Fatalf("issues = %#v", next.issues)
	}
}

func TestNewModelHydratesFreshPersistentActiveView(t *testing.T) {
	now := time.Now()
	store := newFakeActiveViewStore()
	store.record = cache.ActiveViewRecord{
		Namespace: "https://example.atlassian.net",
		CacheKey:  activeViewCacheKey("project = ABC"),
		Issues:    []jira.Issue{{Key: "ABC-9", Summary: "Persistent cached issue"}},
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

	if model.loading {
		t.Fatal("hydrated model should render without initial loading")
	}
	if model.viewStale {
		t.Fatal("fresh hydrated model should not be stale")
	}
	if len(model.issues) != 1 || model.issues[0].Key != "ABC-9" {
		t.Fatalf("issues = %#v", model.issues)
	}
	if cmd := model.Init(); cmd == nil {
		t.Fatal("Init should still return worker wait/refresh scheduling")
	}
}

func TestNewModelHydratesStalePersistentActiveViewAndRefreshesInBackground(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.Local)
	store := newFakeActiveViewStore()
	store.record = cache.ActiveViewRecord{
		Namespace: "https://example.atlassian.net",
		CacheKey:  activeViewCacheKey("project = ABC"),
		Issues:    []jira.Issue{{Key: "ABC-9", Summary: "Stale persistent cached issue"}},
		SyncedAt:  now.Add(-2 * time.Hour),
		FreshTill: now.Add(-2*time.Hour + activeViewCacheTTL),
	}

	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithActiveViewStore(store, "https://example.atlassian.net"),
		WithNow(func() time.Time { return now }),
	)
	defer model.workers.Stop()

	if model.loading {
		t.Fatal("stale hydrated model should render cached rows instead of initial loading")
	}
	if !model.viewStale {
		t.Fatal("stale hydrated model should mark the view stale")
	}
	if !model.refreshing {
		t.Fatal("stale hydrated model should mark that a background refresh is pending")
	}
	if len(model.issues) != 1 || model.issues[0].Key != "ABC-9" {
		t.Fatalf("issues = %#v", model.issues)
	}
	if len(model.diagnosticsEvents) == 0 {
		t.Fatal("expected cache hydration diagnostic")
	}
	event := model.diagnosticsEvents[len(model.diagnosticsEvents)-1]
	if event.Kind != diagnosticKindCache || event.Label != "active_view" || event.Status != "hydrate_stale" {
		t.Fatalf("diagnostic event = %#v", event)
	}
	for _, want := range []string{"Default", "issues=1", "age=2h0m0s", "refresh=background"} {
		if !strings.Contains(event.Detail, want) {
			t.Fatalf("diagnostic detail = %q, missing %q", event.Detail, want)
		}
	}
}

func TestSearchResultPersistsActiveView(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.Local)
	store := newFakeActiveViewStore()
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithActiveViewStore(store, "https://example.atlassian.net"),
	)
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.activeRequestID = 7

	updated, _ := model.Update(workerResultMsg{
		result: worker.Result{
			ID:   7,
			Kind: worker.KindSearchIssues,
			SearchIssues: &worker.SearchIssuesResult{
				Issues:   []jira.Issue{{Key: "ABC-1", Summary: "Persist me"}},
				SyncedAt: now,
			},
		},
	})
	next := updated.(Model)

	if next.err != nil {
		t.Fatalf("err = %v", next.err)
	}
	if store.put.Namespace != "https://example.atlassian.net" || store.put.CacheKey != activeViewCacheKey("project = ABC") {
		t.Fatalf("put = %#v", store.put)
	}
	if len(store.put.Issues) != 1 || store.put.Issues[0].Key != "ABC-1" {
		t.Fatalf("persisted issues = %#v", store.put.Issues)
	}
	if !store.put.SyncedAt.Equal(now) || !store.put.FreshTill.Equal(now.Add(activeViewCacheTTL)) {
		t.Fatalf("persisted timestamps = %s/%s", store.put.SyncedAt, store.put.FreshTill)
	}
}

func TestSearchResultPublishesTicketEventsForNewAndUpdatedIssues(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.Local)
	stream := events.NewStream(events.WithNow(func() time.Time { return now }))
	defer stream.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	received, err := stream.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithEventStream(stream),
		WithNow(func() time.Time { return now }),
	)
	defer model.workers.Stop()
	model.loading = false
	model.activeRequestID = 7
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Old summary", Status: "To Do"},
	}

	updated, _ := model.Update(workerResultMsg{
		result: worker.Result{
			ID:   7,
			Kind: worker.KindSearchIssues,
			SearchIssues: &worker.SearchIssuesResult{
				Issues: []jira.Issue{
					{Key: "ABC-1", Summary: "New summary", Status: "In Progress"},
					{Key: "ABC-2", Summary: "New issue", Status: "To Do"},
				},
				SyncedAt: now,
			},
		},
	})
	_ = updated.(Model)

	got := collectEventsForTest(t, received, 2)
	byKey := make(map[string]events.Event, len(got))
	for _, event := range got {
		byKey[string(event.Type)+":"+event.DedupeKey] = event
	}
	updatedEvent, ok := byKey[string(events.TypeJiraTicketUpdated)+":ABC-1"]
	if !ok {
		t.Fatalf("missing updated event, got %#v", got)
	}
	var updatedPayload events.TicketPayload
	if err := json.Unmarshal(updatedEvent.Payload, &updatedPayload); err != nil {
		t.Fatalf("updated payload decode: %v", err)
	}
	if updatedPayload.IssueKey != "ABC-1" || updatedPayload.Previous == nil || updatedPayload.Previous.Summary != "Old summary" || updatedPayload.Current.Summary != "New summary" {
		t.Fatalf("updated payload = %#v", updatedPayload)
	}
	newEvent, ok := byKey[string(events.TypeJiraTicketNew)+":ABC-2"]
	if !ok {
		t.Fatalf("missing new event, got %#v", got)
	}
	var newPayload events.TicketPayload
	if err := json.Unmarshal(newEvent.Payload, &newPayload); err != nil {
		t.Fatalf("new payload decode: %v", err)
	}
	if newPayload.IssueKey != "ABC-2" || newPayload.Previous != nil || newPayload.Current.Summary != "New issue" {
		t.Fatalf("new payload = %#v", newPayload)
	}
}

func TestColdSearchResultDoesNotPublishTicketEvents(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.Local)
	stream := events.NewStream(events.WithNow(func() time.Time { return now }))
	defer stream.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	received, err := stream.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithEventStream(stream),
		WithNow(func() time.Time { return now }),
	)
	defer model.workers.Stop()
	model.activeRequestID = 7

	updated, _ := model.Update(workerResultMsg{
		result: worker.Result{
			ID:   7,
			Kind: worker.KindSearchIssues,
			SearchIssues: &worker.SearchIssuesResult{
				Issues:   []jira.Issue{{Key: "ABC-1", Summary: "Initial cold load"}},
				SyncedAt: now,
			},
		},
	})
	_ = updated.(Model)

	select {
	case event := <-received:
		t.Fatalf("cold load published unexpected event: %#v", event)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestManualRefreshBypassesFreshCachedIssueView(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.Local)
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = false
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.cacheActiveIssueView("project = ABC", []jira.Issue{{Key: "ABC-1"}}, now.Add(-10*time.Second))

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "r", Code: 'r'}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("manual refresh should submit a Jira refresh even when cache is fresh")
	}
	if !next.refreshing {
		t.Fatal("manual refresh should mark the view as refreshing")
	}
	if next.activeRequestID != initialRequestID+1 {
		t.Fatalf("activeRequestID = %d", next.activeRequestID)
	}
}

func TestSwitchViewUsesStaleCachedIssueViewAndRefreshes(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.Local)
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithViews([]config.IssueView{
			{Name: "Assigned", JQL: "assignee = currentUser()"},
			{Name: "Sprint", JQL: "sprint in openSprints()"},
		}, "Assigned"),
	)
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = false
	model.cacheActiveIssueView("sprint in openSprints()", []jira.Issue{{Key: "ABC-9", Summary: "Cached sprint"}}, now.Add(-2*activeViewCacheTTL))

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "]", Code: ']'}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("stale cached view should submit a refresh command")
	}
	if next.loading {
		t.Fatal("stale cached view should render immediately instead of showing initial loading")
	}
	if !next.refreshing || !next.viewStale {
		t.Fatalf("refreshing=%v viewStale=%v", next.refreshing, next.viewStale)
	}
	if len(next.issues) != 1 || next.issues[0].Key != "ABC-9" {
		t.Fatalf("issues = %#v", next.issues)
	}
}

func TestRefreshFailurePreservesStaleCachedIssueView(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.Local)
	refreshErr := errors.New("jira unavailable")
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = false
	model.refreshing = true
	model.viewStale = true
	model.activeRequestID = 2
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Cached"}}
	model.cacheActiveIssueView("project = ABC", model.issues, now.Add(-2*activeViewCacheTTL))

	updated, _ := model.Update(workerResultMsg{result: worker.Result{
		ID:   2,
		Kind: worker.KindSearchIssues,
		Err:  refreshErr,
	}})
	next := updated.(Model)

	if next.loading || next.refreshing {
		t.Fatalf("loading=%v refreshing=%v", next.loading, next.refreshing)
	}
	if !next.viewStale {
		t.Fatal("failed refresh should keep stale state visible")
	}
	if !errors.Is(next.err, refreshErr) {
		t.Fatalf("err = %v", next.err)
	}
	if len(next.issues) != 1 || next.issues[0].Key != "ABC-1" {
		t.Fatalf("issues = %#v", next.issues)
	}
}

func TestOrderIssuesPlacesChildrenAfterParent(t *testing.T) {
	ordered := orderIssues([]jira.Issue{
		{Key: "ABC-2", ParentKey: "ABC-1"},
		{Key: "ABC-3"},
		{Key: "ABC-1"},
	}, sortJira)

	keys := []string{ordered[0].Key, ordered[1].Key, ordered[2].Key}
	want := []string{"ABC-3", "ABC-1", "ABC-2"}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("keys = %#v", keys)
		}
	}
}

func TestPageSelectionMovesByVisibleRows(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.width = 140
	model.height = 30
	for i := 0; i < 20; i++ {
		model.issues = append(model.issues, jira.Issue{Key: fmt.Sprintf("ABC-%d", i)})
	}

	model.pageSelection(1)

	if model.selected != model.currentLayoutRows()-1 {
		t.Fatalf("selected = %d", model.selected)
	}
	model.pageSelection(1)
	if model.offset == 0 {
		t.Fatal("expected offset to advance after second page")
	}
}

func TestBrowserLayoutUsesFullWidthListViewport(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.height = 40

	wide := model.browserLayout(140)
	if wide.rows != 25 {
		t.Fatalf("wide rows = %d", wide.rows)
	}
	if wide.listWidth != wide.contentWidth {
		t.Fatalf("wide list width = %d, want content width %d", wide.listWidth, wide.contentWidth)
	}

	narrow := model.browserLayout(90)
	if narrow.rows != 25 {
		t.Fatalf("narrow rows = %d", narrow.rows)
	}
	if narrow.listWidth != narrow.contentWidth {
		t.Fatalf("narrow list width = %d, want content width %d", narrow.listWidth, narrow.contentWidth)
	}
}

func TestIssueListViewportShowsSelectedWindow(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.height = 20
	model.offset = 6
	model.selected = 8
	layout := model.browserLayout(100)
	for i := 0; i < 20; i++ {
		model.issues = append(model.issues, jira.Issue{
			Key:       fmt.Sprintf("ABC-%d", i),
			Summary:   fmt.Sprintf("Issue number %d", i),
			Status:    "To Do",
			Priority:  "P2",
			IssueType: "Task",
		})
	}

	view := model.renderIssueList(layout)

	if strings.Contains(view, "ABC-0") {
		t.Fatalf("viewport should start at offset, view = %q", view)
	}
	if !strings.Contains(view, "ABC-8") || !strings.Contains(view, "20 issues  rows 7-11") {
		t.Fatalf("viewport did not render selected window, view = %q", view)
	}
}

func TestIssueListViewportUsesRenderedTreeRows(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithDisplay(config.Display{SymbolMode: "symbols"}))
	defer model.workers.Stop()
	model.height = 20
	model.width = 100
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Parent", Status: "To Do", Priority: "P4", IssueType: "Epic"},
	}
	for i := 0; i < 12; i++ {
		model.issues = append(model.issues, jira.Issue{
			Key:       fmt.Sprintf("ABC-%d", i+2),
			Summary:   fmt.Sprintf("Subtask %d", i+1),
			Status:    "To Do",
			Priority:  "P4",
			IssueType: "Sub-task",
			IsSubtask: true,
			ParentKey: "ABC-1",
		})
	}
	model.selected = len(model.issues) - 1
	model.ensureSelectionVisible(model.currentLayoutRows())

	view := model.renderIssueList(model.browserLayout(model.width))

	if !strings.Contains(view, "ABC-13") {
		t.Fatalf("selected subtask should be visible in rendered-row viewport: %q", view)
	}
	if strings.Contains(view, "rows 1-") {
		t.Fatalf("expanded tree should scroll by rendered rows, view = %q", view)
	}
}

func TestIssueListStatusFilterDefaultsToAll(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.height = 30
	model.width = 120
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Build it", Status: "In Progress", IssueType: "Task"},
		{Key: "ABC-2", Summary: "Finished", Status: "Done", IssueType: "Task"},
	}

	view := model.renderIssueList(model.browserLayout(model.width))

	if !strings.Contains(view, "ABC-1") || !strings.Contains(view, "ABC-2") {
		t.Fatalf("all mode should render active and terminal issues: %q", view)
	}
}

func TestIssueListActiveStatusFilterHidesTerminalIssues(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.height = 30
	model.width = 120
	model.statusFilter = issueStatusFilterActive
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Ready", Status: "To Do", IssueType: "Task"},
		{Key: "ABC-2", Summary: "Build it", Status: "In Progress", IssueType: "Task"},
		{Key: "ABC-3", Summary: "Finished", Status: "Done", IssueType: "Task"},
		{Key: "ABC-4", Summary: "Cancelled", Status: "Canceled", IssueType: "Task"},
	}

	view := model.renderIssueList(model.browserLayout(model.width))

	for _, want := range []string{"ABC-1", "ABC-2", "Active", "2 shown"} {
		if !strings.Contains(view, want) {
			t.Fatalf("filtered view missing %q: %q", want, view)
		}
	}
	for _, hidden := range []string{"ABC-3", "ABC-4"} {
		if strings.Contains(view, hidden) {
			t.Fatalf("filtered view should hide %s: %q", hidden, view)
		}
	}
}

func TestIssueListActiveStatusFilterEmptyState(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.height = 30
	model.width = 120
	model.statusFilter = issueStatusFilterActive
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Finished", Status: "Done", IssueType: "Task"},
	}

	view := model.renderIssueList(model.browserLayout(model.width))

	for _, want := range []string{"Active filter hides all loaded issues", "f show all"} {
		if !strings.Contains(view, want) {
			t.Fatalf("filtered empty state missing %q: %q", want, view)
		}
	}
}

func TestIssueListStatusFilterToggleUsesF(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.height = 30
	model.width = 120
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Active", Status: "To Do", IssueType: "Task"},
		{Key: "ABC-2", Summary: "Done", Status: "Done", IssueType: "Task"},
	}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "f", Code: 'f'}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("local status filter toggle should not submit work")
	}
	if next.statusFilter != issueStatusFilterActive {
		t.Fatalf("statusFilter = %v, want active", next.statusFilter)
	}
	if view := next.renderIssueList(next.browserLayout(next.width)); strings.Contains(view, "ABC-2") {
		t.Fatalf("active filter should hide terminal row: %q", view)
	}
}

func TestIssueListStatusFilterRepairsHiddenSelection(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.height = 30
	model.width = 120
	model.selected = 1
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Active", Status: "To Do", IssueType: "Task"},
		{Key: "ABC-2", Summary: "Done", Status: "Done", IssueType: "Task"},
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "f", Code: 'f'}))
	next := updated.(Model)

	if got := next.issues[next.selected].Key; got != "ABC-1" {
		t.Fatalf("selected issue = %s, want ABC-1", got)
	}
}

func TestIssueListStatusFilterNavigationSkipsHiddenRows(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.height = 30
	model.width = 120
	model.statusFilter = issueStatusFilterActive
	model.selected = 0
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Active 1", Status: "To Do", IssueType: "Task"},
		{Key: "ABC-2", Summary: "Done", Status: "Done", IssueType: "Task"},
		{Key: "ABC-3", Summary: "Active 2", Status: "In Progress", IssueType: "Task"},
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	next := updated.(Model)

	if got := next.issues[next.selected].Key; got != "ABC-3" {
		t.Fatalf("selection after j = %s, want ABC-3", got)
	}
}

func TestIssueListStatusFilterHomeEndUseVisibleRows(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.height = 30
	model.width = 120
	model.statusFilter = issueStatusFilterActive
	model.selected = 2
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Done first", Status: "Done", IssueType: "Task"},
		{Key: "ABC-2", Summary: "Active first", Status: "To Do", IssueType: "Task"},
		{Key: "ABC-3", Summary: "Active last", Status: "Review", IssueType: "Task"},
		{Key: "ABC-4", Summary: "Done last", Status: "Closed", IssueType: "Task"},
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "g", Code: 'g'}))
	next := updated.(Model)
	if got := next.issues[next.selected].Key; got != "ABC-2" {
		t.Fatalf("home selected = %s, want ABC-2", got)
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "G", Code: 'G'}))
	next = updated.(Model)
	if got := next.issues[next.selected].Key; got != "ABC-3" {
		t.Fatalf("end selected = %s, want ABC-3", got)
	}
}

func TestIssueListStatusFilterPageIndicatorUsesVisibleRows(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.height = 20
	model.width = 120
	model.statusFilter = issueStatusFilterActive
	for i := 0; i < 12; i++ {
		model.issues = append(model.issues, jira.Issue{
			Key:       fmt.Sprintf("ABC-%d", i+1),
			Summary:   "Active issue",
			Status:    "To Do",
			IssueType: "Task",
		})
	}
	for i := 0; i < 20; i++ {
		model.issues = append(model.issues, jira.Issue{
			Key:       fmt.Sprintf("ABC-DONE-%d", i+1),
			Summary:   "Done issue",
			Status:    "Done",
			IssueType: "Task",
		})
	}
	model.offset = 7

	view := model.renderIssueList(model.browserLayout(model.width))

	if strings.Contains(view, "PgDn next") {
		t.Fatalf("filtered page indicator should not use hidden rows as next page: %q", view)
	}
}

func TestIssueListStatusFilterResetsWhenSwitchingSavedViews(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithViews([]config.IssueView{
		{Name: "Assigned", JQL: "project = ABC"},
		{Name: "Watching", JQL: "watcher = currentUser()"},
	}, "Assigned"))
	defer model.workers.Stop()
	model.statusFilter = issueStatusFilterActive

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next := updated.(Model)

	if next.statusFilter != issueStatusFilterAll {
		t.Fatalf("statusFilter after view switch = %v, want all", next.statusFilter)
	}
}

func TestIssueListRendersCompactHierarchyWithLipglossTree(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithDisplay(config.Display{SymbolMode: "symbols"}))
	defer model.workers.Stop()
	model.height = 30
	model.width = 120
	model.selected = 1
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Platform epic", Status: "In Progress", Priority: "High", IssueType: "Epic", Assignee: "Yogi", SubtaskCount: 2},
		{Key: "ABC-2", Summary: "Child story", Status: "To Do", Priority: "Medium", IssueType: "Story", Assignee: "Jon", ParentKey: "ABC-1"},
		{Key: "ABC-3", Summary: "Standalone task", Status: "Done", Priority: "Low", IssueType: "Task", Assignee: "Rushi"},
	}

	view := model.renderIssueList(model.browserLayout(model.width))

	for _, want := range []string{"T  KEY", "◆  ABC-1", "■  ABC-2", "●  ABC-3", "╰─", "▌  ╰─  ■  ABC-2"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	childLine := lineContaining(view, "ABC-2")
	if !strings.Contains(childLine, "╰─") {
		t.Fatalf("child tree connector should be indented under parent row: %q", childLine)
	}
	if strings.Contains(view, "parent ABC-1") {
		t.Fatalf("nested child should not repeat visible parent metadata: %q", view)
	}
}

func TestIssueListSupportsPlainSymbolMode(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithDisplay(config.Display{SymbolMode: "plain"}))
	defer model.workers.Stop()
	model.height = 30
	model.width = 120
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Platform epic", Status: "In Progress", Priority: "High", IssueType: "Epic", Assignee: "Yogi"},
		{Key: "ABC-2", Summary: "Child story", Status: "To Do", Priority: "Medium", IssueType: "Story", Assignee: "Jon", ParentKey: "ABC-1"},
	}

	view := model.renderIssueList(model.browserLayout(model.width))

	for _, want := range []string{"E  ABC-1", "S  ABC-2"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	if strings.Contains(view, "◆") || strings.Contains(view, "■") {
		t.Fatalf("plain symbol mode should not render unicode issue symbols: %q", view)
	}
}

func TestIssueListHeaderAlignsWithRows(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithDisplay(config.Display{SymbolMode: "symbols"}))
	defer model.workers.Stop()
	model.width = 120
	layout := model.browserLayout(model.width)
	issue := jira.Issue{Key: "ABC-1", Summary: "Summary", Status: "In Progress", Priority: "High", IssueType: "Task", Assignee: "Jon Charette"}

	header := lipgloss.NewStyle().Render(model.issueListHeader(layout))
	row := model.renderIssueDisplayRow(issueDisplayRow{issue: issue}, " ", layout)

	for _, pair := range []struct {
		header string
		row    string
	}{
		{"T", "●"},
		{"KEY", "ABC-1"},
		{"SUMMARY", "Summary"},
		{"STATUS", "In Progress"},
		{"PRI", "!!"},
		{"OWNER", "Jon C."},
	} {
		headerColumn := visibleColumn(header, pair.header)
		rowColumn := visibleColumn(row, pair.row)
		if headerColumn != rowColumn {
			t.Fatalf("%s column = %d, %s column = %d\nheader=%q\nrow=%q", pair.header, headerColumn, pair.row, rowColumn, header, row)
		}
	}
}

func TestIssueListHeaderAlignsWhenOwnerColumnIsHidden(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithDisplay(config.Display{SymbolMode: "symbols"}))
	defer model.workers.Stop()
	layout := browserLayout{listWidth: 90}
	issue := jira.Issue{Key: "ABC-1", Summary: "Summary", Status: "In Progress", Priority: "High", IssueType: "Task", Assignee: "Jon Charette"}

	header := model.issueListHeader(layout)
	row := model.renderIssueDisplayRow(issueDisplayRow{issue: issue}, " ", layout)

	if strings.Contains(header, "OWNER") || strings.Contains(row, "Jon") {
		t.Fatalf("owner column should be hidden below 96 columns\nheader=%q\nrow=%q", header, row)
	}
	for _, pair := range []struct {
		header string
		row    string
	}{
		{"T", "●"},
		{"KEY", "ABC-1"},
		{"SUMMARY", "Summary"},
		{"STATUS", "In Progress"},
		{"PRI", "!!"},
	} {
		headerColumn := visibleColumn(header, pair.header)
		rowColumn := visibleColumn(row, pair.row)
		if headerColumn != rowColumn {
			t.Fatalf("%s column = %d, %s column = %d\nheader=%q\nrow=%q", pair.header, headerColumn, pair.row, rowColumn, header, row)
		}
	}
}

func TestIssueListUsesCompactColumnsBelowNinety(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithDisplay(config.Display{SymbolMode: "symbols"}))
	defer model.workers.Stop()
	layout := browserLayout{listWidth: 88}
	issue := jira.Issue{Key: "ABC-123456789", Summary: "A summary that should still have useful space", Status: "In Progress", Priority: "Highest", IssueType: "Task", Assignee: "Jon Charette"}

	header := model.issueListHeader(layout)
	row := model.renderIssueDisplayRow(issueDisplayRow{issue: issue}, " ", layout)

	if strings.Contains(header, "OWNER") || strings.Contains(row, "Jon") {
		t.Fatalf("owner column should be hidden below 90 columns\nheader=%q\nrow=%q", header, row)
	}
	if !strings.Contains(header, "PRI") || !strings.Contains(row, "!!!") {
		t.Fatalf("priority column should remain visible at medium widths\nheader=%q\nrow=%q", header, row)
	}
	if strings.Contains(row, "ABC-123456789") {
		t.Fatalf("long keys should be clipped in compact layouts: %q", row)
	}
	if width := lipgloss.Width(row); width > layout.listWidth {
		t.Fatalf("row width = %d, want <= %d: %q", width, layout.listWidth, row)
	}
}

func TestIssueListHidesPriorityOnVeryNarrowWidths(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithDisplay(config.Display{SymbolMode: "symbols"}))
	defer model.workers.Stop()
	layout := browserLayout{listWidth: 72}
	issue := jira.Issue{Key: "ABC-1", Summary: "A summary remains visible on narrow terminals", Status: "In Progress", Priority: "Highest", IssueType: "Task", Assignee: "Jon Charette"}

	header := model.issueListHeader(layout)
	row := model.renderIssueDisplayRow(issueDisplayRow{issue: issue}, " ", layout)

	for _, notWant := range []string{"PRI", "OWNER", "!!!", "Jon"} {
		if strings.Contains(header, notWant) || strings.Contains(row, notWant) {
			t.Fatalf("narrow issue list should hide %q\nheader=%q\nrow=%q", notWant, header, row)
		}
	}
	for _, want := range []string{"STATUS", "In Prog...", "A summary"} {
		if !strings.Contains(header, want) && !strings.Contains(row, want) {
			t.Fatalf("narrow issue list should keep %q visible\nheader=%q\nrow=%q", want, header, row)
		}
	}
	if width := lipgloss.Width(row); width > layout.listWidth {
		t.Fatalf("row width = %d, want <= %d: %q", width, layout.listWidth, row)
	}
}

func TestIssueListNestedTreeIndentsChildRows(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithDisplay(config.Display{SymbolMode: "symbols"}))
	defer model.workers.Stop()
	model.height = 30
	model.width = 120
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Epic", IssueType: "Epic"},
		{Key: "ABC-2", Summary: "Story", IssueType: "Story", ParentKey: "ABC-1"},
		{Key: "ABC-3", Summary: "Task", IssueType: "Task", ParentKey: "ABC-2"},
		{Key: "ABC-4", Summary: "Subtask", IssueType: "Sub-task", ParentKey: "ABC-3"},
	}

	view := model.renderIssueList(model.browserLayout(model.width))
	keyColumn := visibleColumn(lineContaining(view, "ABC-1"), "ABC-1")
	for _, key := range []string{"ABC-2", "ABC-3", "ABC-4"} {
		line := lineContaining(view, key)
		if got := visibleColumn(line, key); got <= keyColumn {
			t.Fatalf("%s column = %d, want > parent column %d\nline=%q\nview=%q", key, got, keyColumn, line, view)
		}
	}
}

func TestIssueListNestedTreeKeepsConnectorsNearRows(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithDisplay(config.Display{SymbolMode: "symbols"}))
	defer model.workers.Stop()
	model.height = 30
	model.width = 120
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Epic", IssueType: "Epic"},
		{Key: "ABC-2", Summary: "Story", IssueType: "Story", ParentKey: "ABC-1"},
		{Key: "ABC-3", Summary: "Task", IssueType: "Task", ParentKey: "ABC-2"},
	}

	view := model.renderIssueList(model.browserLayout(model.width))
	childLine := lineContaining(view, "ABC-2")
	grandchildLine := lineContaining(view, "ABC-3")

	for _, line := range []string{childLine, grandchildLine} {
		connectorColumn := minPositiveVisibleColumn(line, "╰─", "├─")
		symbolColumn := visibleColumn(line, "■")
		if symbolColumn < 0 {
			symbolColumn = visibleColumn(line, "●")
		}
		if connectorColumn < 0 || symbolColumn < 0 || symbolColumn-connectorColumn > 6 {
			t.Fatalf("connector should stay visually attached to row\nline=%q\nview=%q", line, view)
		}
	}
}

func TestIssueListCollapseDefaultsExpanded(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithDisplay(config.Display{SymbolMode: "symbols"}))
	defer model.workers.Stop()
	model.height = 30
	model.width = 120
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Parent", IssueType: "Epic"},
		{Key: "ABC-2", Summary: "Child", IssueType: "Story", ParentKey: "ABC-1"},
	}

	view := model.renderIssueList(model.browserLayout(model.width))

	if !strings.Contains(view, "ABC-1") || !strings.Contains(view, "ABC-2") {
		t.Fatalf("default issue tree should remain expanded: %q", view)
	}
	if strings.Contains(view, "hidden") {
		t.Fatalf("default expanded tree should not show hidden count: %q", view)
	}
}

func TestIssueListCollapsedParentHidesLoadedDescendants(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithDisplay(config.Display{SymbolMode: "symbols"}))
	defer model.workers.Stop()
	model.height = 30
	model.width = 120
	model.collapsedIssueKeys = map[string]bool{"ABC-1": true}
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Parent", IssueType: "Epic"},
		{Key: "ABC-2", Summary: "Child", IssueType: "Story", ParentKey: "ABC-1"},
		{Key: "ABC-3", Summary: "Grandchild", IssueType: "Task", ParentKey: "ABC-2"},
		{Key: "ABC-4", Summary: "Peer", IssueType: "Task"},
	}

	view := model.renderIssueList(model.browserLayout(model.width))

	if !strings.Contains(view, "ABC-1") || !strings.Contains(view, "ABC-4") {
		t.Fatalf("collapsed parent and peer should remain visible: %q", view)
	}
	if strings.Contains(view, "ABC-2") || strings.Contains(view, "ABC-3") {
		t.Fatalf("collapsed descendants should be hidden: %q", view)
	}
	if !strings.Contains(lineContaining(view, "ABC-1"), "2 hidden") {
		t.Fatalf("collapsed parent should show hidden descendant count: %q", view)
	}
}

func TestIssueListCollapsedParentKeepsHiddenCountOnNarrowTerminal(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithDisplay(config.Display{SymbolMode: "symbols"}))
	defer model.workers.Stop()
	model.height = 30
	model.width = 76
	model.collapsedIssueKeys = map[string]bool{"ABC-1": true}
	model.issues = []jira.Issue{
		{
			Key:       "ABC-1",
			Summary:   "Parent summary that is intentionally long enough to force truncation before the hidden badge would fit normally",
			IssueType: "Epic",
		},
		{Key: "ABC-2", Summary: "Child", IssueType: "Story", ParentKey: "ABC-1"},
		{Key: "ABC-3", Summary: "Grandchild", IssueType: "Task", ParentKey: "ABC-2"},
	}

	view := model.renderIssueList(model.browserLayout(model.width))
	parentLine := lineContaining(view, "ABC-1")

	if parentLine == "" {
		t.Fatalf("missing collapsed parent row in %q", view)
	}
	if !strings.Contains(parentLine, "2 hidden") {
		t.Fatalf("collapsed parent should keep hidden count when summary truncates: %q", parentLine)
	}
}

func TestVisibleIssueIndexesSkipCollapsedDescendants(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithDisplay(config.Display{SymbolMode: "symbols"}))
	defer model.workers.Stop()
	model.collapsedIssueKeys = map[string]bool{"ABC-1": true}
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Parent", IssueType: "Epic"},
		{Key: "ABC-2", Summary: "Child", IssueType: "Story", ParentKey: "ABC-1"},
		{Key: "ABC-3", Summary: "Grandchild", IssueType: "Task", ParentKey: "ABC-2"},
		{Key: "ABC-4", Summary: "Peer", IssueType: "Task"},
	}

	displayTree := buildIssueDisplayTree(model.issues)
	got := model.visibleIssueIndexes(displayTree)

	want := []int{0, 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("visible issue indexes = %v, want %v", got, want)
	}
}

func TestVisibleIssueIndexesIncludeMissingParentRoots(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithDisplay(config.Display{SymbolMode: "symbols"}))
	defer model.workers.Stop()
	model.collapsedIssueKeys = map[string]bool{"ABC-2": true}
	model.issues = []jira.Issue{
		{Key: "ABC-2", Summary: "Child with missing parent", IssueType: "Story", ParentKey: "MISSING-1", ParentSummary: "Missing parent"},
		{Key: "ABC-3", Summary: "Grandchild", IssueType: "Task", ParentKey: "ABC-2"},
		{Key: "ABC-4", Summary: "Standalone", IssueType: "Task"},
	}

	displayTree := buildIssueDisplayTree(model.issues)
	got := model.visibleIssueIndexes(displayTree)

	want := []int{0, 2}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("visible issue indexes = %v, want %v", got, want)
	}
}

func TestIssueListToggleCollapseFromSelectedNode(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithDisplay(config.Display{SymbolMode: "symbols"}))
	defer model.workers.Stop()
	model.height = 30
	model.width = 120
	model.selected = 0
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Parent", IssueType: "Epic"},
		{Key: "ABC-2", Summary: "Child", IssueType: "Story", ParentKey: "ABC-1"},
	}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "z", Code: 'z'}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("collapse toggle should not submit work")
	}
	if !next.collapsedIssueKeys["ABC-1"] {
		t.Fatalf("expected ABC-1 collapsed, state=%v", next.collapsedIssueKeys)
	}
	if view := next.renderIssueList(next.browserLayout(next.width)); strings.Contains(view, "ABC-2") || !strings.Contains(view, "1 hidden") {
		t.Fatalf("collapsed view = %q", view)
	}
}

func TestIssueListToggleCollapseLeafShowsNotice(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.height = 30
	model.width = 120
	model.selected = 0
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Leaf", IssueType: "Task"}}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "z", Code: 'z'}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("leaf collapse toggle should not submit work")
	}
	if len(next.collapsedIssueKeys) != 0 {
		t.Fatalf("leaf row should not be marked collapsed: %v", next.collapsedIssueKeys)
	}
	if !strings.Contains(next.detailNotice, "No loaded child issues") {
		t.Fatalf("leaf notice = %q", next.detailNotice)
	}
}

func TestIssueListNavigationSkipsCollapsedDescendants(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.height = 30
	model.width = 120
	model.selected = 0
	model.collapsedIssueKeys = map[string]bool{"ABC-1": true}
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Parent", IssueType: "Epic"},
		{Key: "ABC-2", Summary: "Child", IssueType: "Story", ParentKey: "ABC-1"},
		{Key: "ABC-3", Summary: "Peer", IssueType: "Task"},
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	next := updated.(Model)

	if got := next.issues[next.selected].Key; got != "ABC-3" {
		t.Fatalf("selection after j = %s, want ABC-3", got)
	}
}

func TestIssueListRepairSelectionHiddenByCollapsedAncestor(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.height = 30
	model.width = 120
	model.selected = 2
	model.collapsedIssueKeys = map[string]bool{"ABC-1": true}
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Parent", IssueType: "Epic"},
		{Key: "ABC-2", Summary: "Child", IssueType: "Story", ParentKey: "ABC-1"},
		{Key: "ABC-3", Summary: "Grandchild", IssueType: "Task", ParentKey: "ABC-2"},
	}

	model.repairCollapsedSelection()

	if got := model.issues[model.selected].Key; got != "ABC-1" {
		t.Fatalf("selection after repair = %s, want collapsed ancestor ABC-1", got)
	}
}

func TestIssueListPagingUsesVisibleRows(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.height = 30
	model.width = 120
	model.selected = 0
	model.collapsedIssueKeys = map[string]bool{"ABC-1": true}
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Parent", IssueType: "Epic"},
		{Key: "ABC-2", Summary: "Child", IssueType: "Story", ParentKey: "ABC-1"},
		{Key: "ABC-3", Summary: "Peer 1", IssueType: "Task"},
		{Key: "ABC-4", Summary: "Peer 2", IssueType: "Task"},
	}

	model.pageSelection(1)

	if got := model.issues[model.selected].Key; got != "ABC-4" {
		t.Fatalf("page selection = %s, want last visible issue ABC-4", got)
	}
}

func TestIssueListExpandParentPreservesDeeperCollapsedBranch(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithDisplay(config.Display{SymbolMode: "symbols"}))
	defer model.workers.Stop()
	model.height = 30
	model.width = 120
	model.selected = 0
	model.collapsedIssueKeys = map[string]bool{"ABC-1": true, "ABC-2": true}
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Parent", IssueType: "Epic"},
		{Key: "ABC-2", Summary: "Child", IssueType: "Story", ParentKey: "ABC-1"},
		{Key: "ABC-3", Summary: "Grandchild", IssueType: "Task", ParentKey: "ABC-2"},
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "z", Code: 'z'}))
	next := updated.(Model)
	view := next.renderIssueList(next.browserLayout(next.width))

	if next.collapsedIssueKeys["ABC-1"] {
		t.Fatalf("parent should be expanded, state=%v", next.collapsedIssueKeys)
	}
	if !next.collapsedIssueKeys["ABC-2"] {
		t.Fatalf("child collapse state should remain, state=%v", next.collapsedIssueKeys)
	}
	if !strings.Contains(view, "ABC-2") || strings.Contains(view, "ABC-3") {
		t.Fatalf("expanded parent should reveal collapsed child but not grandchild: %q", view)
	}
}

func TestIssueListMergeExpandedIssuesPreservesCollapseState(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.collapsedIssueKeys = map[string]bool{"ABC-1": true}
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Parent", IssueType: "Epic"}}

	added := model.mergeExpandedIssues([]jira.Issue{{Key: "ABC-2", Summary: "Child", ParentKey: "ABC-1"}})

	if added != 1 {
		t.Fatalf("added = %d, want 1", added)
	}
	if !model.collapsedIssueKeys["ABC-1"] {
		t.Fatalf("collapse state should survive explicit expansion: %v", model.collapsedIssueKeys)
	}
}

func TestIssueListReplaceIssuesRepairsSelectionHiddenByPreservedCollapse(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.height = 30
	model.width = 120
	model.selected = 1
	model.collapsedIssueKeys = map[string]bool{"ABC-1": true}
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Parent", IssueType: "Epic"},
		{Key: "ABC-2", Summary: "Child", IssueType: "Story", ParentKey: "ABC-1"},
	}

	model.replaceIssues([]jira.Issue{
		{Key: "ABC-1", Summary: "Parent refreshed", IssueType: "Epic"},
		{Key: "ABC-2", Summary: "Child refreshed", IssueType: "Story", ParentKey: "ABC-1"},
	})

	if got := model.issues[model.selected].Key; got != "ABC-1" {
		t.Fatalf("selection after replace = %s, want collapsed parent ABC-1", got)
	}
}

func TestApplyActiveIssueViewRepairsSelectionHiddenByPreservedCollapse(t *testing.T) {
	now := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.height = 30
	model.width = 120
	model.selected = 1
	model.collapsedIssueKeys = map[string]bool{"ABC-1": true}
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Parent", IssueType: "Epic"},
		{Key: "ABC-2", Summary: "Child", IssueType: "Story", ParentKey: "ABC-1"},
	}

	model.applyActiveIssueView(issueViewCacheRecord{
		Issues: []jira.Issue{
			{Key: "ABC-1", Summary: "Parent cached", IssueType: "Epic"},
			{Key: "ABC-2", Summary: "Child cached", IssueType: "Story", ParentKey: "ABC-1"},
		},
		SyncedAt:  now,
		FreshTill: now.Add(activeViewCacheTTL),
	}, false)

	if got := model.issues[model.selected].Key; got != "ABC-1" {
		t.Fatalf("selection after cached view apply = %s, want collapsed parent ABC-1", got)
	}
}

func TestIssueRenderLinesPreserveIssueIndexForMissingParentChildRow(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithDisplay(config.Display{SymbolMode: "symbols"}))
	defer model.workers.Stop()
	model.height = 30
	model.width = 120
	model.issues = []jira.Issue{
		{Key: "ABC-2", Summary: "Child story", IssueType: "Story", ParentKey: "ABC-1", ParentSummary: "Parent outside filter"},
	}

	lines := model.issueRenderLines(model.browserLayout(model.width))
	if len(lines) < 2 {
		t.Fatalf("expected placeholder and child rows, got %#v", lines)
	}
	if lines[0].issueIndex != -1 {
		t.Fatalf("missing-parent placeholder issueIndex = %d, want -1", lines[0].issueIndex)
	}
	if lines[1].issueIndex != 0 {
		t.Fatalf("missing-parent child row issueIndex = %d, want 0", lines[1].issueIndex)
	}
	if !strings.Contains(lines[1].text, "ABC-2") {
		t.Fatalf("expected child row text to contain issue key, got %q", lines[1].text)
	}
}

func TestIssueListKeepsShallowHierarchyOnNarrowTerminals(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithDisplay(config.Display{SymbolMode: "symbols"}))
	defer model.workers.Stop()
	model.height = 30
	model.width = 72
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Epic", IssueType: "Epic"},
		{Key: "ABC-2", Summary: "Story", IssueType: "Story", ParentKey: "ABC-1"},
		{Key: "ABC-3", Summary: "Task", IssueType: "Task", ParentKey: "ABC-2"},
		{Key: "ABC-4", Summary: "Subtask", IssueType: "Sub-task", ParentKey: "ABC-3"},
	}

	view := model.renderIssueList(model.browserLayout(model.width))
	rootColumn := visibleColumn(lineContaining(view, "ABC-1"), "ABC-1")
	deepColumn := visibleColumn(lineContaining(view, "ABC-4"), "ABC-4")

	if rootColumn < 0 || deepColumn < 0 {
		t.Fatalf("expected root and deep child rows in %q", view)
	}
	if deepColumn-rootColumn > 8 {
		t.Fatalf("narrow hierarchy should stay shallow: root=%d deep=%d view=%q", rootColumn, deepColumn, view)
	}
	for _, line := range strings.Split(view, "\n") {
		maxWidth := model.browserLayout(model.width).listWidth + 2
		if strings.Contains(line, "ABC-") && lipgloss.Width(line) > maxWidth {
			t.Fatalf("issue row width = %d, want <= %d: %q", lipgloss.Width(line), maxWidth, line)
		}
	}
}

func TestIssueListShowsChildMetadataEvenWhenRepeated(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithDisplay(config.Display{SymbolMode: "symbols"}))
	defer model.workers.Stop()
	model.height = 30
	model.width = 120
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Platform epic", Status: "To Do", Priority: "Low", IssueType: "Epic", Assignee: "Yogi Singh"},
		{Key: "ABC-2", Summary: "Child story", Status: "To Do", Priority: "Low", IssueType: "Story", Assignee: "Yogi Singh", ParentKey: "ABC-1"},
	}

	view := model.renderIssueList(model.browserLayout(model.width))
	childLine := lineContaining(view, "ABC-2")
	if childLine == "" {
		t.Fatalf("missing child row in %q", view)
	}
	for _, want := range []string{"To Do", "P4", "Yogi"} {
		if !strings.Contains(childLine, want) {
			t.Fatalf("child row should keep %q metadata: %q", want, childLine)
		}
	}
}

func TestIssueListDoesNotSuppressSubtaskMetadata(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithDisplay(config.Display{SymbolMode: "symbols"}))
	defer model.workers.Stop()
	model.height = 30
	model.width = 120
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Parent", Status: "To Do", Priority: "Low", IssueType: "Story", Assignee: "Yogi Singh"},
		{Key: "ABC-2", Summary: "Subtask", Status: "To Do", Priority: "Low", IssueType: "Sub-task", IsSubtask: true, Assignee: "Yogi Singh", ParentKey: "ABC-1"},
	}

	view := model.renderIssueList(model.browserLayout(model.width))
	childLine := lineContaining(view, "ABC-2")
	for _, want := range []string{"To Do", "P4", "Yogi"} {
		if !strings.Contains(childLine, want) {
			t.Fatalf("subtask row should keep %q metadata: %q", want, childLine)
		}
	}
}

func TestIssueListLabelsMissingParentPlaceholder(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithDisplay(config.Display{SymbolMode: "symbols"}))
	defer model.workers.Stop()
	model.height = 30
	model.width = 120
	model.issues = []jira.Issue{
		{Key: "ABC-2", Summary: "Child story", Status: "To Do", Priority: "Low", IssueType: "Story", Assignee: "Yogi Singh", ParentKey: "ABC-1", ParentSummary: "Parent outside filter"},
	}

	view := model.renderIssueList(model.browserLayout(model.width))
	parentLine := lineContaining(view, "ABC-1")
	if parentLine == "" {
		t.Fatalf("missing parent placeholder in %q", view)
	}
	if !strings.Contains(parentLine, "Parent outside view: ABC-1") {
		t.Fatalf("missing parent row should be explicit, got %q", parentLine)
	}
	if strings.Contains(parentLine, "To Do") || strings.Contains(parentLine, "P4") || strings.Contains(parentLine, "Yogi") {
		t.Fatalf("missing parent placeholder should not look like a loaded issue row: %q", parentLine)
	}
}

func TestSelectedParentDoesNotRepeatVisibleChildCount(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithDisplay(config.Display{SymbolMode: "symbols"}))
	defer model.workers.Stop()
	model.height = 30
	model.width = 120
	model.selected = 0
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Platform epic", Status: "To Do", Priority: "Low", IssueType: "Epic", Assignee: "Yogi Singh"},
		{Key: "ABC-2", Summary: "Child story", Status: "To Do", Priority: "Low", IssueType: "Story", Assignee: "Yogi Singh", ParentKey: "ABC-1"},
	}

	view := model.renderIssueList(model.browserLayout(model.width))

	if strings.Contains(view, "1 children") {
		t.Fatalf("selected parent should not repeat visible child count: %q", view)
	}
	if !strings.Contains(view, "ABC-2") {
		t.Fatalf("child row should still be visible: %q", view)
	}
}

func TestRenderQueryShowsReadableFilterSummary(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS AND (creator = currentUser() OR reporter = currentUser()) AND resolution = Unresolved ORDER BY updated DESC")
	defer model.workers.Stop()
	model.width = 120

	view := model.renderQuery(model.browserLayout(model.width))

	for _, want := range []string{"Filter", "DEVOPS", "created/reported by me", "unresolved", "updated desc"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	if strings.Contains(view, "creator = currentUser") {
		t.Fatalf("filter summary should hide raw JQL: %q", view)
	}
}

func TestHeaderUsesAvailableWidth(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithViews([]config.IssueView{
		{Name: "Assigned", JQL: "project = ABC"},
	}, "Assigned"))
	defer model.workers.Stop()
	model.loading = false
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1"}, {Key: "ABC-2"}}

	header := model.renderHeader(model.browserLayout(model.width))

	if !strings.Contains(header, "Jira") || !strings.Contains(header, "Assigned") || !strings.Contains(header, "2 issues") {
		t.Fatalf("header = %q", header)
	}
	if lipgloss.Width(header) != model.browserLayout(model.width).contentWidth {
		t.Fatalf("header width = %d, want %d, header = %q", lipgloss.Width(header), model.browserLayout(model.width).contentWidth, header)
	}
}

func TestHeaderShowsStaleAndFailedRefreshFreshness(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.width = 110
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.lastSynced = time.Date(2026, 6, 16, 10, 15, 0, 0, time.Local)
	model.viewStale = true

	header := model.renderHeader(model.browserLayout(model.width))
	if !strings.Contains(header, "stale 10:15:00") {
		t.Fatalf("header = %q", header)
	}

	model.err = errors.New("jira unavailable")
	header = model.renderHeader(model.browserLayout(model.width))
	if !strings.Contains(header, "refresh failed 10:15:00") {
		t.Fatalf("header = %q", header)
	}
}

func TestHeaderShowsBackgroundActivityForActiveAI(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.width = 140
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.claudePlanLoading = true

	header := model.renderHeader(model.browserLayout(model.width))

	if !strings.Contains(header, "AI working") {
		t.Fatalf("header = %q", header)
	}
	if strings.Contains(header, "bg ") {
		t.Fatalf("header should use product wording, got %q", header)
	}
}

func TestHeaderShowsBackgroundActivityForActiveJiraWork(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.refreshing = true
	model.width = 140
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1"}}

	header := model.renderHeader(model.browserLayout(model.width))

	if !strings.Contains(header, "syncing") {
		t.Fatalf("header = %q", header)
	}
	if strings.Contains(header, "refreshing") {
		t.Fatalf("header should not duplicate refresh state: %q", header)
	}
}

func TestHeaderShowsRecentTicketUpdates(t *testing.T) {
	now := time.Date(2026, 6, 16, 22, 0, 0, 0, time.UTC)
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = false
	model.width = 140
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.diagnosticsEvents = []diagnosticEvent{
		{At: now.Add(-2 * time.Second), Kind: diagnosticKindEvent, Label: "jira.ticket.new", Status: "published", Detail: "ABC-1"},
		{At: now.Add(-time.Second), Kind: diagnosticKindEvent, Label: "jira.ticket.updated", Status: "published", Detail: "ABC-2"},
	}

	header := model.renderHeader(model.browserLayout(model.width))

	if !strings.Contains(header, "2 ticket updates") {
		t.Fatalf("header = %q", header)
	}
	if strings.Contains(header, "bg ") || strings.Contains(header, "events") {
		t.Fatalf("header should use ticket wording, got %q", header)
	}
}

func TestHeaderFallsBackForRecentNonTicketEvents(t *testing.T) {
	now := time.Date(2026, 6, 16, 22, 0, 0, 0, time.UTC)
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = false
	model.width = 140
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.diagnosticsEvents = []diagnosticEvent{
		{At: now.Add(-time.Second), Kind: diagnosticKindEvent, Label: "ai.task.completed", Status: "published", Detail: "ticket_plan"},
	}

	header := model.renderHeader(model.browserLayout(model.width))

	if !strings.Contains(header, "1 event") {
		t.Fatalf("header = %q", header)
	}
}

func TestHeaderPrioritizesRecentErrorsOverTicketUpdates(t *testing.T) {
	now := time.Date(2026, 6, 16, 22, 0, 0, 0, time.UTC)
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = false
	model.width = 140
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.diagnosticsEvents = []diagnosticEvent{
		{At: now.Add(-2 * time.Second), Kind: diagnosticKindEvent, Label: "jira.ticket.new", Status: "published", Detail: "ABC-1"},
		{At: now.Add(-time.Second), Kind: diagnosticKindWorker, Label: "search_issues", Status: "error", Detail: "jira unavailable"},
	}

	header := model.renderHeader(model.browserLayout(model.width))

	if !strings.Contains(header, "1 error") {
		t.Fatalf("header = %q", header)
	}
	if strings.Contains(header, "ticket updates") {
		t.Fatalf("error should take priority over ticket updates: %q", header)
	}
}

func TestHeaderDropsBackgroundActivityWhenWidthIsTight(t *testing.T) {
	now := time.Date(2026, 6, 16, 22, 0, 0, 0, time.UTC)
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = false
	model.width = 74
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.diagnosticsEvents = []diagnosticEvent{
		{At: now.Add(-time.Second), Kind: diagnosticKindEvent, Label: "jira.ticket.new", Status: "published", Detail: "ABC-1"},
	}

	header := model.renderHeader(model.browserLayout(model.width))

	if strings.Contains(header, "ticket updates") {
		t.Fatalf("tight header should drop recent activity first: %q", header)
	}
	if !strings.Contains(header, "1 issues") || !strings.Contains(header, "not synced") {
		t.Fatalf("tight header should keep issue count and freshness: %q", header)
	}
	if lipgloss.Width(header) != model.browserLayout(model.width).contentWidth {
		t.Fatalf("header width = %d, want %d: %q", lipgloss.Width(header), model.browserLayout(model.width).contentWidth, header)
	}
}

func TestHeaderHidesBackgroundActivityWhenIdle(t *testing.T) {
	now := time.Date(2026, 6, 16, 22, 0, 0, 0, time.UTC)
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = false
	model.width = 140
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.diagnosticsEvents = []diagnosticEvent{
		{At: now.Add(-time.Minute), Kind: diagnosticKindEvent, Label: "jira.ticket.new", Status: "published", Detail: "ABC-1"},
	}

	header := model.renderHeader(model.browserLayout(model.width))

	if strings.Contains(header, "bg ") {
		t.Fatalf("idle header should stay quiet: %q", header)
	}
}

func TestFooterHelpGroupsCommandsAndFitsWidth(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.width = 72
	layout := model.browserLayout(model.width)

	footer := model.renderFooterHelp(keyContextTable, layout)

	for _, want := range []string{"Issue Table", "? help", "j/k move", "enter open", "|"} {
		if !strings.Contains(footer, want) {
			t.Fatalf("missing %q in %q", want, footer)
		}
	}
	if lipgloss.Width(footer) > layout.contentWidth {
		t.Fatalf("footer width = %d, want <= %d: %q", lipgloss.Width(footer), layout.contentWidth, footer)
	}
}

func TestHelpIncludesActiveContextBindings(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.width = 110
	model.height = 40
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Parent", IssueType: "Epic"},
		{Key: "ABC-2", Summary: "Child", IssueType: "Story", ParentKey: "ABC-1"},
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "?", Code: '?'}))
	next := updated.(Model)
	if !next.helpOpen {
		t.Fatal("expected help to open")
	}

	view := next.render()
	for _, want := range []string{"Keyboard Help", "Issue Table", "Collapse or expand the selected issue subtree.", "Load open child issues for the selected parent."} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestKeyBindingsAdaptToBubblesKeyHelp(t *testing.T) {
	binding := keyBinding{
		Keys:        []string{"j", "k", "up", "down"},
		FooterKey:   "j/k",
		Label:       "move",
		Description: "Move the selected issue.",
		Group:       "Navigation",
		Footer:      true,
	}

	adapted := binding.bubbleKeyBinding()

	if got := strings.Join(adapted.Keys(), ","); got != "j,k,up,down" {
		t.Fatalf("adapted keys = %q", got)
	}
	if adapted.Help().Key != "j/k" || adapted.Help().Desc != "move" {
		t.Fatalf("short help = %#v", adapted.Help())
	}

	full := binding.bubbleKeyBindingForFullHelp()
	if full.Help().Key != "j/k" || full.Help().Desc != "Move the selected issue." {
		t.Fatalf("full help = %#v", full.Help())
	}
}

func TestChoiceListUsesSharedBubblesListAdapter(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()

	rendered := model.renderChoiceList([]choiceListOption{
		{Label: "Jane Doe"},
		{Label: "John Doe"},
		{Label: "Jill Doe"},
		{Label: "Joan Doe"},
	}, 2, 24, 2)

	joined := strings.Join(rendered, "\n")
	if !strings.Contains(joined, "> Jill Doe") {
		t.Fatalf("expected selected item from shared choice list, rendered = %q", joined)
	}
	if strings.Contains(joined, "Jane Doe") {
		t.Fatalf("expected paged list to hide first item when selected item is later, rendered = %q", joined)
	}
	if !strings.Contains(joined, "3-4 of 4") {
		t.Fatalf("expected shared range indicator, rendered = %q", joined)
	}
}

func TestFooterHelpDoesNotTruncateMidCommand(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	layout := browserLayout{contentWidth: 10}

	footer := model.renderFooterHelp(keyContextTable, layout)

	if strings.Contains(footer, "...") {
		t.Fatalf("footer should omit overflowing commands instead of truncating: %q", footer)
	}
	if strings.Contains(footer, "e...") || strings.Contains(footer, "enter...") {
		t.Fatalf("footer should not contain partial commands: %q", footer)
	}
}

func TestTableFooterPrioritizesOpenBeforePaging(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	layout := browserLayout{contentWidth: 48}

	footer := model.renderFooterHelp(keyContextTable, layout)

	if !strings.Contains(footer, "enter open") {
		t.Fatalf("table footer should keep open visible before lower-priority paging: %q", footer)
	}
	if strings.Contains(footer, "pgup/pgdn page") {
		t.Fatalf("paging should be omitted before open at constrained widths: %q", footer)
	}
}

func TestDetailOOpensIssueAndNoSortInDetail(t *testing.T) {
	var opened string
	withLinkActions(t, func(value string) error {
		opened = value
		return nil
	}, nil)
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 30
	model.issues = []jira.Issue{
		{Key: "ABC-1", URL: "https://example.test/browse/ABC-1"},
		{Key: "ABC-2", URL: "https://example.test/browse/ABC-2"},
	}
	model.sort = sortPriority

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "o", Code: 'o'}))
	next := updated.(Model)
	if cmd == nil {
		t.Fatal("expected open command in detail mode")
	}
	updated, _ = next.Update(cmd())
	next = updated.(Model)
	if opened != "https://example.test/browse/ABC-1" {
		t.Fatalf("opened = %q", opened)
	}
	if next.sort != sortPriority {
		t.Fatalf("sort changed in detail mode: %v", next.sort)
	}
}

func TestDetailOUpperDoesNotSort(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.sort = sortPriority
	model.issues = []jira.Issue{
		{Key: "ABC-2"},
		{Key: "ABC-1"},
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "O", Code: 'O'}))
	next := updated.(Model)

	if next.sort != sortPriority {
		t.Fatalf("detail uppercase sort key changed sort: %v", next.sort)
	}
	if !reflect.DeepEqual(next.issues, []jira.Issue{{Key: "ABC-2"}, {Key: "ABC-1"}}) {
		t.Fatalf("detail uppercase sort reordered issues: %#v", next.issues)
	}
}

func TestTableSortKeysStillWork(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeTable
	model.issues = []jira.Issue{
		{Key: "ABC-2", Priority: "High"},
		{Key: "ABC-1", Priority: "Low"},
	}
	model.sort = sortPriority

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "o", Code: 'o'}))
	next := updated.(Model)
	if next.sort == sortPriority {
		t.Fatal("lowercase sort key should cycle sort in table")
	}
	firstSort := next.sort

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "O", Code: 'O'}))
	next = updated.(Model)
	if next.sort == firstSort {
		t.Fatal("uppercase sort key should cycle sort in table")
	}
}

func TestSupportedMinimumHeightKeepsUsefulIssueRows(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.width = ui.MinTerminalWidth
	model.height = ui.MinTerminalHeight

	rows := model.currentLayoutRows()

	if rows < minUsefulIssueRows {
		t.Fatalf("rows = %d, want at least %d", rows, minUsefulIssueRows)
	}
}

func TestRenderShowsTerminalSizeWarningBelowMinimum(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.width = 87
	model.height = 23
	model.issues = []jira.Issue{{Key: "ABC-1"}}

	view := model.render()

	for _, want := range []string{"Terminal Size", "Terminal too small: 87x23", "at least 88x24", "120x30 is recommended"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	if strings.Contains(view, "JQL") {
		t.Fatalf("small terminal warning should skip normal layout: %q", view)
	}
}

func TestStatePanelsKeepCommandsInFooter(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.width = 100
	model.height = 30

	view := model.render()

	for _, want := range []string{"No Issues", "No issues matched this query.", "Issue Table", "r refresh"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	for _, notWant := range []string{"Press r", "q to quit"} {
		if strings.Contains(view, notWant) {
			t.Fatalf("state panel should not include inline command hint %q in %q", notWant, view)
		}
	}
}

func TestErrorStatePanelKeepsCommandsInFooter(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.err = errors.New("network unavailable")
	model.width = 100
	model.height = 30

	view := model.render()

	for _, want := range []string{"Error", "network unavailable", "Issue Table", "r refresh"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	for _, notWant := range []string{"Press r", "q to quit"} {
		if strings.Contains(view, notWant) {
			t.Fatalf("error panel should not include inline command hint %q in %q", notWant, view)
		}
	}
}

func TestSwitchSortPreservesSelectedIssue(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.issues = []jira.Issue{
		{Key: "ABC-1", Priority: "Low"},
		{Key: "ABC-2", Priority: "Highest"},
	}
	model.selected = 0

	model.switchSort(1)

	if model.sort != sortPriority {
		t.Fatalf("sort = %v", model.sort)
	}
	if model.issues[0].Key != "ABC-2" {
		t.Fatalf("issues = %#v", model.issues)
	}
	if model.issues[model.selected].Key != "ABC-1" {
		t.Fatalf("selected issue = %#v", model.issues[model.selected])
	}
}

func TestRefreshTickStartsBackgroundRefresh(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithRefreshInterval(time.Minute),
	)
	defer model.workers.Stop()
	model.loading = false
	model.issues = []jira.Issue{{Key: "ABC-1"}}

	updated, cmd := model.Update(refreshTickMsg{})
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("expected refresh command")
	}
	if next.loading {
		t.Fatal("background refresh should not use initial loading state")
	}
	if !next.refreshing {
		t.Fatal("refreshing should be true")
	}
	if next.activeRequestID != initialRequestID+1 {
		t.Fatalf("activeRequestID = %d", next.activeRequestID)
	}
}

func TestSwitchViewUpdatesJQLAndStartsRefresh(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithViews([]config.IssueView{
			{Name: "Assigned", JQL: "assignee = currentUser()"},
			{Name: "Sprint", JQL: "sprint in openSprints()"},
		}, "Assigned"),
	)
	defer model.workers.Stop()
	model.loading = false
	model.issues = []jira.Issue{{Key: "ABC-1"}}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "]", Code: ']'}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("expected refresh command")
	}
	if next.jql != "sprint in openSprints()" {
		t.Fatalf("jql = %q", next.jql)
	}
	if !next.loading {
		t.Fatal("expected loading after switching view")
	}
}

func TestSubmitIssueSearchReturnsLoadedResultFromPool(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{
		issues: []jira.Issue{{Key: "ABC-9"}},
	}, "project = ABC")
	defer model.workers.Stop()

	msg := model.submitIssueSearch(9, worker.PriorityForeground)()
	if _, ok := msg.(workSubmittedMsg); !ok {
		t.Fatalf("msg = %T", msg)
	}

	resultMsg := model.waitForWorkerResult()()
	loaded, ok := resultMsg.(workerResultMsg)
	if !ok {
		t.Fatalf("resultMsg = %T", resultMsg)
	}
	if loaded.result.ID != 9 {
		t.Fatalf("requestID = %d", loaded.result.ID)
	}
	if len(loaded.result.SearchIssues.Issues) != 1 || loaded.result.SearchIssues.Issues[0].Key != "ABC-9" {
		t.Fatalf("issues = %#v", loaded.result.SearchIssues.Issues)
	}
}

func TestSubmitIssueSearchReturnsFailedResultFromPool(t *testing.T) {
	searchErr := errors.New("jira is unavailable")
	model := NewModel(&fakeIssueSearcher{
		err: searchErr,
	}, "project = ABC")
	defer model.workers.Stop()

	msg := model.submitIssueSearch(9, worker.PriorityForeground)()
	if _, ok := msg.(workSubmittedMsg); !ok {
		t.Fatalf("msg = %T", msg)
	}

	resultMsg := model.waitForWorkerResult()()
	failed, ok := resultMsg.(workerResultMsg)
	if !ok {
		t.Fatalf("resultMsg = %T", resultMsg)
	}
	if failed.result.ID != 9 {
		t.Fatalf("requestID = %d", failed.result.ID)
	}
	if !errors.Is(failed.result.Err, searchErr) {
		t.Fatalf("err = %v", failed.result.Err)
	}
}

type fakeActiveViewStore struct {
	record              cache.ActiveViewRecord
	put                 cache.ActiveViewRecord
	queryHistory        []cache.QueryHistoryRecord
	putQueryHistory     cache.QueryHistoryRecord
	detail              cache.IssueDetailRecord
	putDetail           cache.IssueDetailRecord
	comments            cache.IssueCommentsRecord
	putComments         cache.IssueCommentsRecord
	deletedComments     string
	transitions         cache.IssueTransitionsRecord
	putTransitions      cache.IssueTransitionsRecord
	deletedTransitions  string
	editMetadata        cache.IssueEditMetadataRecord
	putEditMetadata     cache.IssueEditMetadataRecord
	createIssueTypes    cache.CreateIssueTypesRecord
	putCreateIssueTypes cache.CreateIssueTypesRecord
	createFields        cache.CreateFieldsRecord
	putCreateFields     cache.CreateFieldsRecord
	expandedChildren    cache.ExpandedChildrenRecord
	putExpandedChildren cache.ExpandedChildrenRecord
}

func newFakeActiveViewStore() *fakeActiveViewStore {
	return &fakeActiveViewStore{}
}

func (f *fakeActiveViewStore) GetActiveView(_ context.Context, namespace string, cacheKey string) (cache.ActiveViewRecord, bool, error) {
	if f.record.Namespace == namespace && f.record.CacheKey == cacheKey {
		return f.record, true, nil
	}
	return cache.ActiveViewRecord{}, false, nil
}

func (f *fakeActiveViewStore) PutActiveView(_ context.Context, record cache.ActiveViewRecord) error {
	f.put = record
	return nil
}

func (f *fakeActiveViewStore) PutQueryHistory(_ context.Context, record cache.QueryHistoryRecord) error {
	f.putQueryHistory = record
	return nil
}

func (f *fakeActiveViewStore) ListQueryHistory(_ context.Context, namespace string, limit int) ([]cache.QueryHistoryRecord, error) {
	if namespace == "" {
		return nil, nil
	}
	if limit <= 0 || limit > len(f.queryHistory) {
		limit = len(f.queryHistory)
	}
	return append([]cache.QueryHistoryRecord(nil), f.queryHistory[:limit]...), nil
}

func (f *fakeActiveViewStore) GetIssueDetail(_ context.Context, namespace string, issueKey string) (cache.IssueDetailRecord, bool, error) {
	if f.detail.Namespace == namespace && f.detail.IssueKey == issueKey {
		return f.detail, true, nil
	}
	return cache.IssueDetailRecord{}, false, nil
}

func (f *fakeActiveViewStore) PutIssueDetail(_ context.Context, record cache.IssueDetailRecord) error {
	f.putDetail = record
	return nil
}

func (f *fakeActiveViewStore) GetIssueComments(_ context.Context, namespace string, issueKey string, maxResults int) (cache.IssueCommentsRecord, bool, error) {
	if f.comments.Namespace == namespace && f.comments.IssueKey == issueKey && f.comments.MaxResults == maxResults {
		return f.comments, true, nil
	}
	return cache.IssueCommentsRecord{}, false, nil
}

func (f *fakeActiveViewStore) PutIssueComments(_ context.Context, record cache.IssueCommentsRecord) error {
	f.putComments = record
	return nil
}

func (f *fakeActiveViewStore) DeleteIssueComments(_ context.Context, _ string, issueKey string) error {
	f.deletedComments = issueKey
	return nil
}

func (f *fakeActiveViewStore) GetIssueTransitions(_ context.Context, namespace string, issueKey string) (cache.IssueTransitionsRecord, bool, error) {
	if f.transitions.Namespace == namespace && f.transitions.IssueKey == issueKey {
		return f.transitions, true, nil
	}
	return cache.IssueTransitionsRecord{}, false, nil
}

func (f *fakeActiveViewStore) PutIssueTransitions(_ context.Context, record cache.IssueTransitionsRecord) error {
	f.putTransitions = record
	return nil
}

func (f *fakeActiveViewStore) DeleteIssueTransitions(_ context.Context, _ string, issueKey string) error {
	f.deletedTransitions = issueKey
	return nil
}

func (f *fakeActiveViewStore) GetIssueEditMetadata(_ context.Context, namespace string, issueKey string) (cache.IssueEditMetadataRecord, bool, error) {
	if f.editMetadata.Namespace == namespace && f.editMetadata.IssueKey == issueKey {
		return f.editMetadata, true, nil
	}
	return cache.IssueEditMetadataRecord{}, false, nil
}

func (f *fakeActiveViewStore) PutIssueEditMetadata(_ context.Context, record cache.IssueEditMetadataRecord) error {
	f.putEditMetadata = record
	return nil
}

func (f *fakeActiveViewStore) GetCreateIssueTypes(_ context.Context, namespace string, projectKey string) (cache.CreateIssueTypesRecord, bool, error) {
	if f.createIssueTypes.Namespace == namespace && f.createIssueTypes.ProjectKey == projectKey {
		return f.createIssueTypes, true, nil
	}
	return cache.CreateIssueTypesRecord{}, false, nil
}

func (f *fakeActiveViewStore) PutCreateIssueTypes(_ context.Context, record cache.CreateIssueTypesRecord) error {
	f.putCreateIssueTypes = record
	return nil
}

func (f *fakeActiveViewStore) GetCreateFields(_ context.Context, namespace string, projectKey string, issueTypeID string) (cache.CreateFieldsRecord, bool, error) {
	if f.createFields.Namespace == namespace && f.createFields.ProjectKey == projectKey && f.createFields.IssueTypeID == issueTypeID {
		return f.createFields, true, nil
	}
	return cache.CreateFieldsRecord{}, false, nil
}

func (f *fakeActiveViewStore) PutCreateFields(_ context.Context, record cache.CreateFieldsRecord) error {
	f.putCreateFields = record
	return nil
}

func (f *fakeActiveViewStore) GetExpandedChildren(_ context.Context, namespace string, parentKey string, mode string) (cache.ExpandedChildrenRecord, bool, error) {
	if f.expandedChildren.Namespace == namespace && f.expandedChildren.ParentKey == parentKey && f.expandedChildren.Mode == mode {
		return f.expandedChildren, true, nil
	}
	return cache.ExpandedChildrenRecord{}, false, nil
}

func (f *fakeActiveViewStore) PutExpandedChildren(_ context.Context, record cache.ExpandedChildrenRecord) error {
	f.putExpandedChildren = record
	return nil
}
