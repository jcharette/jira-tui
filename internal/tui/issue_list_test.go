package tui

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/jon/jira-tui/internal/cache"
	"github.com/jon/jira-tui/internal/config"
	"github.com/jon/jira-tui/internal/jira"
	"github.com/jon/jira-tui/internal/ui"
	"github.com/jon/jira-tui/internal/worker"
)

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
	record cache.ActiveViewRecord
	put    cache.ActiveViewRecord
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
