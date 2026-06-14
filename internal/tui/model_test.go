package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/jon/jira-tui/internal/config"
	"github.com/jon/jira-tui/internal/jira"
	"github.com/jon/jira-tui/internal/ui"
	"github.com/jon/jira-tui/internal/worker"
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

func lineContaining(value string, needle string) string {
	for _, line := range strings.Split(value, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
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

func TestDetailFooterKeepsSecondaryCopyActionsInHelp(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.width = 120
	layout := model.browserLayout(model.width)

	footer := model.renderFooterHelp(keyContextDetail, layout)

	for _, want := range []string{"Ticket Detail", "esc back", "j/k scroll", "tab section", "a comment", "b browser"} {
		if !strings.Contains(footer, want) {
			t.Fatalf("missing %q in %q", want, footer)
		}
	}
	for _, hidden := range []string{"enter select", "n/p section", "c key", "y url"} {
		if strings.Contains(footer, hidden) {
			t.Fatalf("secondary action %q should stay in full help, footer = %q", hidden, footer)
		}
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

func TestDetailPageDownScrollsDetailInsteadOfChangingIssue(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 24
	model.issues = []jira.Issue{{Key: "ABC-1"}, {Key: "ABC-2"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue: jira.Issue{Key: "ABC-1", Summary: "One"},
			Description: strings.Join([]string{
				"line 1",
				"line 2",
				"line 3",
				"line 4",
				"line 5",
				"line 6",
				"line 7",
				"line 8",
				"line 9",
				"line 10",
			}, "\n"),
		},
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "pgdown", Code: tea.KeyPgDown}))
	next := updated.(Model)

	if next.selected != 0 {
		t.Fatalf("selected = %d", next.selected)
	}
	if next.detailOffset == 0 {
		t.Fatal("expected detailOffset to advance")
	}
}

func TestDetailLinksCanBeFocusedSelectedAndCopied(t *testing.T) {
	var copied string
	withLinkActions(t, func(string) error { return nil }, func(value string) error {
		copied = value
		return nil
	})
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 60
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       jira.Issue{Key: "ABC-1"},
			Description: "Run https://example.test/build then email ops@example.test.",
		},
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "l", Code: 'l'}))
	next := updated.(Model)
	if !next.linkFocus {
		t.Fatal("expected links to be focused")
	}
	if next.selectedLink != 0 {
		t.Fatalf("selectedLink = %d", next.selectedLink)
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	next = updated.(Model)
	if next.selectedLink != 1 {
		t.Fatalf("selectedLink after j = %d", next.selectedLink)
	}

	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "y", Code: 'y'}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected copy command")
	}
	msg := cmd()
	updated, _ = next.Update(msg)
	next = updated.(Model)

	if copied != "ops@example.test" {
		t.Fatalf("copied = %q", copied)
	}
	if !strings.Contains(next.detailNotice, "Copied ops@example.test") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestDetailLinksOpenSelectedTarget(t *testing.T) {
	var opened string
	withLinkActions(t, func(value string) error {
		opened = value
		return nil
	}, func(string) error { return nil })
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 60
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       jira.Issue{Key: "ABC-1"},
			Description: "Run https://example.test/build.",
		},
	}
	model.focusDetailLinks()

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if cmd == nil {
		t.Fatal("expected open command")
	}
	msg := cmd()
	updated, _ = next.Update(msg)
	next = updated.(Model)

	if opened != "https://example.test/build" {
		t.Fatalf("opened = %q", opened)
	}
	if !strings.Contains(next.detailNotice, "Opened https://example.test/build") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestDetailSectionNavigationJumpsBetweenSections(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 18
	model.issues = []jira.Issue{{Key: "ABC-1", URL: "https://example.test/browse/ABC-1"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       jira.Issue{Key: "ABC-1", URL: "https://example.test/browse/ABC-1"},
			Description: "Run https://example.test/build.\n\n" + strings.Repeat("detail line\n", 20),
		},
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "n", Code: 'n'}))
	next := updated.(Model)
	if next.detailFocus != 1 || next.detailOffset != 0 {
		t.Fatalf("expected n to select the next section at its saved scroll, focus=%d offset=%d", next.detailFocus, next.detailOffset)
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "h", Code: 'h'}))
	next = updated.(Model)
	if next.detailFocus != 1 || next.detailOffset != 0 {
		t.Fatalf("expected h to select hierarchy, focus=%d offset=%d", next.detailFocus, next.detailOffset)
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "p", Code: 'p'}))
	next = updated.(Model)
	if next.detailFocus != 0 || next.detailOffset != 0 {
		t.Fatalf("expected p to select previous section at its saved scroll, focus=%d offset=%d", next.detailFocus, next.detailOffset)
	}
}

func TestDetailSectionNavigationRestoresSectionScrollOffsets(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 18
	model.issues = []jira.Issue{
		{Key: "ABC-1", URL: "https://example.test/browse/ABC-1"},
		{Key: "ABC-2", URL: "https://example.test/browse/ABC-2"},
	}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       jira.Issue{Key: "ABC-1", URL: "https://example.test/browse/ABC-1"},
			Description: "Run https://example.test/build.\n\n" + strings.Repeat("detail line\n", 30),
		},
		"ABC-2": {
			Issue:       jira.Issue{Key: "ABC-2", URL: "https://example.test/browse/ABC-2"},
			Description: strings.Repeat("other detail line\n", 30),
		},
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "pgdown", Code: tea.KeyPgDown}))
	next := updated.(Model)
	descriptionOffset := next.detailOffset
	if descriptionOffset == 0 {
		t.Fatal("expected description scroll offset to advance")
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "n", Code: 'n'}))
	next = updated.(Model)
	if next.detailFocus != 1 || next.detailOffset != 0 {
		t.Fatalf("expected links section at top, focus=%d offset=%d", next.detailFocus, next.detailOffset)
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "p", Code: 'p'}))
	next = updated.(Model)
	if next.detailFocus != 0 || next.detailOffset != descriptionOffset {
		t.Fatalf("expected description offset %d to restore, focus=%d offset=%d", descriptionOffset, next.detailFocus, next.detailOffset)
	}

	next.mode = modeTable
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "down", Code: tea.KeyDown}))
	next = updated.(Model)
	if next.selected != 1 {
		t.Fatalf("selected = %d", next.selected)
	}
	if next.detailOffset != 0 || len(next.detailSectionOffset) != 0 {
		t.Fatalf("issue change should reset detail section offsets, offset=%d offsets=%#v", next.detailOffset, next.detailSectionOffset)
	}
}

func TestHierarchySectionRendersGroupedTree(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 80
	model.detailFocus = 1
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Parent story", IssueType: "Story", ParentKey: "ABC-0", ParentSummary: "Platform epic"},
		{Key: "ABC-2", Summary: "Child task", IssueType: "Task", Status: "To Do", Priority: "High", Assignee: "Rae", ParentKey: "ABC-1"},
		{Key: "ABC-3", Summary: "Regression tests", IssueType: "Subtask", Status: "Review", Priority: "Medium", Assignee: "Jon", ParentKey: "ABC-1"},
	}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue: jira.Issue{
				Key:           "ABC-1",
				Summary:       "Parent story",
				IssueType:     "Story",
				ParentKey:     "ABC-0",
				ParentSummary: "Platform epic",
			},
			Description: "Detail",
		},
	}

	view := model.render()

	for _, want := range []string{
		"Path",
		"ABC-0",
		"Platform epic",
		"Children 1",
		"ABC-2",
		"Child task",
		"Subtasks 1",
		"ABC-3",
		"Regression tests",
		"Linked Issues",
		"Linked issue data is not loaded yet.",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	if strings.Contains(view, "No parent or child issues in the current result.") {
		t.Fatalf("old hierarchy empty state should not render when grouped rows exist: %q", view)
	}
}

func TestHierarchySectionShowsCursorBeforeActivation(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 40
	model.detailFocus = 1
	model.hierarchyFocus = false
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Parent story", IssueType: "Story"},
		{Key: "ABC-2", Summary: "Child task", IssueType: "Task", ParentKey: "ABC-1"},
	}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: jira.Issue{Key: "ABC-1", Summary: "Parent story", IssueType: "Story"}},
	}

	view := model.render()

	for _, want := range []string{"Path", "Current", "ABC-1", "Parent story", "enter focus", "> ABC-2"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	if activeKeyContext(model) != keyContextDetail {
		t.Fatalf("activeKeyContext = %q", activeKeyContext(model))
	}
}

func TestHierarchySectionMovesCursorBeforeActivation(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 40
	model.detailFocus = 1
	model.hierarchyFocus = false
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Parent story", IssueType: "Story"},
		{Key: "ABC-2", Summary: "First child", IssueType: "Task", ParentKey: "ABC-1"},
		{Key: "ABC-3", Summary: "Second child", IssueType: "Task", ParentKey: "ABC-1"},
	}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: jira.Issue{Key: "ABC-1", Summary: "Parent story", IssueType: "Story"}},
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "down", Code: tea.KeyDown}))
	next := updated.(Model)

	if next.selectedHierarchy != 1 {
		t.Fatalf("selectedHierarchy = %d", next.selectedHierarchy)
	}
	if next.detailOffset != 0 {
		t.Fatalf("detailOffset = %d", next.detailOffset)
	}
	view := next.render()
	if !strings.Contains(view, "> ABC-3") {
		t.Fatalf("expected second child selected in %q", view)
	}
}

func TestHierarchyEnterOpensSelectedGroupedSubtask(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 40
	model.detailFocus = 1
	model.hierarchyFocus = true
	model.selectedHierarchy = 1
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Parent story", IssueType: "Story"},
		{Key: "ABC-2", Summary: "Child task", IssueType: "Task", ParentKey: "ABC-1"},
		{Key: "ABC-3", Summary: "Regression tests", IssueType: "Subtask", ParentKey: "ABC-1"},
	}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: jira.Issue{Key: "ABC-1", Summary: "Parent story", IssueType: "Story"}},
		"ABC-3": {Issue: jira.Issue{Key: "ABC-3", Summary: "Regression tests", IssueType: "Subtask", ParentKey: "ABC-1"}},
	}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("expected detail request command")
	}
	if next.selected != 2 || next.issues[next.selected].Key != "ABC-3" {
		t.Fatalf("selected issue = %d %#v", next.selected, next.issues)
	}
	if len(next.detailBackStack) != 1 || next.detailBackStack[0] != 0 {
		t.Fatalf("detailBackStack = %#v", next.detailBackStack)
	}
	if next.hierarchyFocus {
		t.Fatal("expected hierarchy focus to clear after opening selected issue")
	}
}

func TestDetailIssueActionsOpenAndCopySelectedIssue(t *testing.T) {
	var opened string
	var copied []string
	withLinkActions(t, func(value string) error {
		opened = value
		return nil
	}, func(value string) error {
		copied = append(copied, value)
		return nil
	})
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", URL: "https://example.test/browse/ABC-1"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       jira.Issue{Key: "ABC-1", URL: "https://example.test/browse/ABC-1"},
			Description: "No links here.",
		},
	}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "b", Code: 'b'}))
	next := updated.(Model)
	if cmd == nil {
		t.Fatal("expected browser command")
	}
	updated, _ = next.Update(cmd())
	next = updated.(Model)
	if opened != "https://example.test/browse/ABC-1" {
		t.Fatalf("opened = %q", opened)
	}

	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "c", Code: 'c'}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected copy key command")
	}
	updated, _ = next.Update(cmd())
	next = updated.(Model)

	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "y", Code: 'y'}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected copy url command")
	}
	updated, _ = next.Update(cmd())
	next = updated.(Model)

	want := []string{"ABC-1", "https://example.test/browse/ABC-1"}
	if fmt.Sprint(copied) != fmt.Sprint(want) {
		t.Fatalf("copied = %#v", copied)
	}
	if !strings.Contains(next.detailNotice, "Copied https://example.test/browse/ABC-1") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestCommentComposerConfirmsAndPostsComment(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{
		addedComment: jira.Comment{ID: "10002", Author: "Current User", Body: "Please review"},
		comments:     []jira.Comment{{ID: "10002", Author: "Current User", Body: "Please review"}},
	}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", URL: "https://example.test/browse/ABC-1"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       jira.Issue{Key: "ABC-1", URL: "https://example.test/browse/ABC-1"},
			Description: "No links here.",
		},
	}
	model.comments = map[string][]jira.Comment{
		"ABC-1": {},
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "a", Code: 'a'}))
	next := updated.(Model)
	if next.mode != modeComment {
		t.Fatalf("mode = %v", next.mode)
	}

	for _, key := range []string{"P", "l", "e", "a", "s", "e", "-", "r", "e", "v", "i", "e", "w"} {
		updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: key, Code: []rune(key)[0]}))
		next = updated.(Model)
	}
	if next.commentDraft != "Please-review" {
		t.Fatalf("commentDraft = %q", next.commentDraft)
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	if !next.commentConfirm {
		t.Fatal("expected confirmation state")
	}

	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "y", Code: 'y'}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected add comment command")
	}
	updated, _ = next.Update(cmd())
	next = updated.(Model)

	resultMsg := next.waitForWorkerResult()()
	updated, cmd = next.Update(resultMsg)
	next = updated.(Model)
	if next.mode != modeDetail {
		t.Fatalf("mode after add = %v", next.mode)
	}
	if next.commentDraft != "" {
		t.Fatalf("commentDraft after add = %q", next.commentDraft)
	}
	if !strings.Contains(next.detailNotice, "Comment posted") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
	if cmd == nil {
		t.Fatal("expected comments refresh command")
	}
	if !next.commentsLoading {
		t.Fatal("expected comments refresh to start")
	}
	if _, ok := next.comments["ABC-1"]; ok {
		t.Fatalf("expected stale comment cache to be cleared: %#v", next.comments["ABC-1"])
	}
}

func TestCommentComposerCancelReturnsToDetail(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.commentDraft = "Never mind"

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "esc", Code: tea.KeyEsc}))
	next := updated.(Model)

	if next.mode != modeDetail {
		t.Fatalf("mode = %v", next.mode)
	}
	if next.commentDraft != "" {
		t.Fatalf("commentDraft = %q", next.commentDraft)
	}
}

func TestCommentComposerAcceptsSpaceNewlineAndCursorEditing(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.issues = []jira.Issue{{Key: "ABC-1"}}

	for _, key := range []tea.KeyMsg{
		tea.KeyPressMsg(tea.Key{Text: "H", Code: 'H'}),
		tea.KeyPressMsg(tea.Key{Text: "i", Code: 'i'}),
		tea.KeyPressMsg(tea.Key{Text: " ", Code: tea.KeySpace}),
		tea.KeyPressMsg(tea.Key{Text: "t", Code: 't'}),
		tea.KeyPressMsg(tea.Key{Text: "h", Code: 'h'}),
		tea.KeyPressMsg(tea.Key{Text: "e", Code: 'e'}),
		tea.KeyPressMsg(tea.Key{Text: "r", Code: 'r'}),
		tea.KeyPressMsg(tea.Key{Text: "e", Code: 'e'}),
		tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}),
		tea.KeyPressMsg(tea.Key{Text: "N", Code: 'N'}),
		tea.KeyPressMsg(tea.Key{Text: "e", Code: 'e'}),
		tea.KeyPressMsg(tea.Key{Text: "x", Code: 'x'}),
		tea.KeyPressMsg(tea.Key{Text: "t", Code: 't'}),
		tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}),
		tea.KeyPressMsg(tea.Key{Text: "K", Code: 'K'}),
		tea.KeyPressMsg(tea.Key{Text: "p", Code: 'p'}),
		tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}),
		tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}),
		tea.KeyPressMsg(tea.Key{Text: "!", Code: '!'}),
	} {
		updated, _ := model.Update(key)
		model = updated.(Model)
	}

	want := "Hi there\nNext\n!Kp"
	if model.commentDraft != want {
		t.Fatalf("commentDraft = %q, want %q", model.commentDraft, want)
	}
}

func TestCommentComposerAcceptsPaste(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.issues = []jira.Issue{{Key: "ABC-1"}}

	updated, _ := model.Update(tea.PasteMsg{Content: "first line\nsecond line"})
	next := updated.(Model)

	if next.commentDraft != "first line\nsecond line" {
		t.Fatalf("commentDraft = %q", next.commentDraft)
	}
}

func TestCommentComposerTreatsQAsText(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.issues = []jira.Issue{{Key: "ABC-1"}}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "q", Code: 'q'}))
	next := updated.(Model)

	if next.mode != modeComment {
		t.Fatalf("mode = %v", next.mode)
	}
	if next.commentDraft != "q" {
		t.Fatalf("commentDraft = %q", next.commentDraft)
	}
}

func TestCommentComposerTreatsGlobalActionKeysAsText(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.issues = []jira.Issue{{Key: "ABC-1"}}

	for _, key := range []string{"r", "e", "q", "u", "i", "r", "e", "d", " ", "b", "y", "m", "l", "o", "c"} {
		msg := tea.KeyPressMsg(tea.Key{Text: key, Code: []rune(key)[0]})
		if key == " " {
			msg = tea.KeyPressMsg(tea.Key{Text: " ", Code: tea.KeySpace})
		}
		updated, _ := model.Update(msg)
		model = updated.(Model)
		if model.mode != modeComment {
			t.Fatalf("mode after %q = %v", key, model.mode)
		}
	}

	if model.commentDraft != "required bymloc" {
		t.Fatalf("commentDraft = %q", model.commentDraft)
	}
}

func TestCommentComposerTabReviewsDraft(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.commentDraft = "Ready to post"

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("tab review should not produce a command before confirmation")
	}
	if !next.commentConfirm {
		t.Fatal("expected confirmation state")
	}
	if next.commentEditor.Focused() {
		t.Fatal("expected editor to blur while reviewing")
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "n", Code: 'n'}))
	next = updated.(Model)
	if next.commentConfirm {
		t.Fatal("expected confirmation state to close")
	}
	if !next.commentEditor.Focused() {
		t.Fatal("expected editor to refocus after returning to edit")
	}
}

func TestCommentComposerRendersEditor(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.width = 90
	model.height = 24
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "One"}}

	view := model.render()
	for _, want := range []string{"Add Comment", "──", "rite a Jira comment"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}

	model.commentDraft = "line one\n"
	view = model.render()
	if !strings.Contains(view, "line one") {
		t.Fatalf("expected draft in %q", view)
	}
}

func TestCommentComposerRendersNoticeBlock(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "One"}}
	model.detailNotice = "Write a comment before posting."

	view := model.render()

	for _, want := range []string{"Notice", "Write a comment before posting."} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestCommentComposerSubmittingUsesSectionHeader(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "One"}}
	model.commentSubmitting = true

	view := model.render()

	for _, want := range []string{"Posting Comment", "──", "Posting comment..."} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestCommentComposerIsBoundedAndScrollsDraft(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.width = 90
	model.height = 24
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "One"}}
	model.commentDraft = strings.Join([]string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"line 5",
		"line 6",
		"line 7",
		"line 8",
	}, "\n")

	view := model.render()
	if lines := strings.Count(view, "\n") + 1; lines > model.height {
		t.Fatalf("composer lines = %d, height = %d\n%s", lines, model.height, view)
	}
	if !strings.Contains(view, "Lines 1-") {
		t.Fatalf("expected draft pagination indicator in %q", view)
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "pgdown", Code: tea.KeyPgDown}))
	next := updated.(Model)
	if next.commentEditor.ScrollYOffset() == 0 {
		t.Fatal("expected editor viewport to advance")
	}
	view = next.render()
	if lines := strings.Count(view, "\n") + 1; lines > model.height {
		t.Fatalf("composer lines after scroll = %d, height = %d\n%s", lines, model.height, view)
	}
}

func TestCommentComposerShowsDetectedLinks(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.width = 110
	model.height = 24
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "One"}}
	model.commentDraft = "See https://example.test/run/1 and email ops@example.test"

	view := model.render()

	for _, want := range []string{"Detected Links", "──", "https://example.test/run/1", "ops@example.test"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	if lines := strings.Count(view, "\n") + 1; lines > model.height {
		t.Fatalf("composer lines = %d, height = %d\n%s", lines, model.height, view)
	}
}

func TestCommentComposerShowsDetectedLinksInReview(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.width = 110
	model.height = 24
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "One"}}
	model.commentDraft = "See https://example.test/run/1"

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next := updated.(Model)
	view := next.render()

	for _, want := range []string{"Review Comment", "Detected Links", "https://example.test/run/1", "Post this comment"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestCommentComposerShowsUnresolvedMentions(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.width = 110
	model.height = 24
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "One"}}
	model.commentDraft = "Please check with @jane.doe"

	view := model.render()

	for _, want := range []string{"Unresolved Mentions", "──", "@jane.doe", "Use @ to select Jira users"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	if lines := strings.Count(view, "\n") + 1; lines > model.height {
		t.Fatalf("composer lines = %d, height = %d\n%s", lines, model.height, view)
	}
}

func TestCommentComposerMentionPickerSearchesAndInsertsUser(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.width = 110
	model.height = 28
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "One"}}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "@", Code: '@'}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("@ should open mention picker without submitting work")
	}
	if !next.mentionPickerOpen {
		t.Fatal("expected mention picker to open")
	}
	if activeKeyContext(next) != keyContextMentionPicker {
		t.Fatalf("activeKeyContext = %q", activeKeyContext(next))
	}
	view := next.render()
	for _, want := range []string{"Mention User", "──", "type to search Jira", "users"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected mention picker text %q in %q", want, view)
		}
	}

	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "J", Code: 'J'}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected user search command")
	}
	if next.mentionQuery != "J" {
		t.Fatalf("mentionQuery = %q", next.mentionQuery)
	}
	if !next.mentionSearchLoading {
		t.Fatal("expected user search loading state")
	}

	updated, _ = next.Update(workerResultMsg{result: worker.Result{
		ID:   next.mentionSearchReqID,
		Kind: worker.KindSearchUsers,
		SearchUsers: &worker.SearchUsersResult{
			Query: "J",
			Users: []jira.User{
				{AccountID: "abc-123", DisplayName: "Jane Doe", Email: "jane@example.test", Active: true},
				{AccountID: "def-456", DisplayName: "John Doe", Email: "john@example.test", Active: true},
				{AccountID: "ghi-789", DisplayName: "Jill Doe", Email: "jill@example.test", Active: true},
			},
		},
	}})
	next = updated.(Model)
	if next.mentionSearchLoading {
		t.Fatal("expected user search loading state to clear")
	}
	view = next.render()
	for _, want := range []string{"Jane Doe", "John Doe", "Jill Doe", "Filter: J"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	if !strings.Contains(view, "> Jane Doe") {
		t.Fatalf("expected first user to be visibly selected in %q", view)
	}
	if strings.Contains(view, "jane@example.test") {
		t.Fatalf("expected mention picker to hide email by default: %q", view)
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	next = updated.(Model)
	if next.mentionCursor != 1 {
		t.Fatalf("mentionCursor = %d", next.mentionCursor)
	}
	view = next.render()
	if !strings.Contains(view, "> John Doe") {
		t.Fatalf("expected second user to be visibly selected in %q", view)
	}
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	next = updated.(Model)
	if next.mentionPickerOpen {
		t.Fatal("expected mention picker to close")
	}
	if next.commentDraft != "@John Doe" {
		t.Fatalf("commentDraft = %q", next.commentDraft)
	}
	if !next.commentEditor.Focused() {
		t.Fatal("expected comment editor to regain focus")
	}
	if len(next.commentMentions) != 1 {
		t.Fatalf("commentMentions = %#v", next.commentMentions)
	}
	if next.commentMentions[0].AccountID != "def-456" || next.commentMentions[0].Text != "@John Doe" {
		t.Fatalf("commentMentions[0] = %#v", next.commentMentions[0])
	}
	view = next.render()
	if strings.Contains(view, "Unresolved Mentions") {
		t.Fatalf("selected mention should not render unresolved warning: %q", view)
	}
}

func TestCommentComposerMentionPickerCancelKeepsLiteralText(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.width = 110
	model.height = 28
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "One"}}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "@", Code: '@'}))
	next := updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "o", Code: 'o'}))
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "p", Code: 'p'}))
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "s", Code: 's'}))
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	next = updated.(Model)

	if next.mentionPickerOpen {
		t.Fatal("expected mention picker to close")
	}
	if next.commentDraft != "@ops" {
		t.Fatalf("commentDraft = %q", next.commentDraft)
	}
}

func TestCommentComposerMentionPickerTreatsJAsFilterText(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeComment
	model.width = 110
	model.height = 28
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "One"}}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "@", Code: '@'}))
	next := updated.(Model)
	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	next = updated.(Model)

	if cmd == nil {
		t.Fatal("expected user search command")
	}
	if next.mentionQuery != "j" {
		t.Fatalf("mentionQuery = %q", next.mentionQuery)
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
	for _, want := range []string{"Keyboard Help", "Ticket Detail", "Add a plain-text Jira comment", "Open the selected Jira issue"} {
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

func TestWrapRichTextPreservesParagraphsAndListIndent(t *testing.T) {
	got := wrapRichText("Need: this is a longer sentence that should wrap cleanly.\n\n- first item has enough words to wrap onto a continuation line", 28)
	want := "Need: this is a longer\nsentence that should wrap\ncleanly.\n\n- first item has enough\n  words to wrap onto a\n  continuation line"
	if got != want {
		t.Fatalf("wrapped = %q", got)
	}
}

func TestRenderRichDescriptionStylesInlineCode(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()

	rendered := model.renderRichDescriptionBody("Use `locals.tf` and `main.tf`.", 80)

	if strings.Contains(rendered, "`locals.tf`") {
		t.Fatalf("expected inline code markers to be styled away: %q", rendered)
	}
	if !strings.Contains(rendered, "locals.tf") || !strings.Contains(rendered, "main.tf") {
		t.Fatalf("rendered = %q", rendered)
	}
	if strings.Contains(rendered, "main.tf .") {
		t.Fatalf("inline code styling should not add padding before punctuation: %q", rendered)
	}
}

func TestRenderRichDescriptionFormatsCodeBlock(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()

	rendered := model.renderRichDescriptionBody("```\n{\"Sid\":\"DenyS3Deletes\"}\n```", 40)

	if strings.Contains(rendered, "```") {
		t.Fatalf("expected code fences to be styled away: %q", rendered)
	}
	if !strings.Contains(rendered, "{\"Sid\":\"DenyS3Deletes\"}") {
		t.Fatalf("rendered = %q", rendered)
	}
	if !strings.Contains(rendered, "+--------------------------------------+") {
		t.Fatalf("expected block styling, rendered = %q", rendered)
	}
}

func TestRenderRichDescriptionDoesNotLeaveExtraBlankLinesBeforeCodeBlock(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()

	rendered := model.renderRichDescriptionBody("The failure is:\n\n```\n\nError: missing resource\n\n```", 40)

	if strings.Contains(rendered, "\n\n\n") {
		t.Fatalf("expected code block spacing to be collapsed, rendered = %q", rendered)
	}
	if strings.Contains(rendered, "|                                      |") {
		t.Fatalf("expected leading/trailing blank code lines to be trimmed, rendered = %q", rendered)
	}
	if !strings.Contains(rendered, "The failure is:\n\n+--------------------------------------+") {
		t.Fatalf("expected one separator before code block, rendered = %q", rendered)
	}
}

func TestRenderRichDescriptionUsesLipglossTable(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()

	rendered := model.renderRichDescriptionBody("[table]\n| Field | Value |\n|-------|-------|\n| Status | Ready |\n[/table]", 60)

	if strings.Contains(rendered, "[table]") || strings.Contains(rendered, "|-------|") {
		t.Fatalf("expected semantic table markers and ASCII separator to be styled away: %q", rendered)
	}
	for _, want := range []string{"Field", "Value", "Status", "Ready"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("missing %q in %q", want, rendered)
		}
	}
	if !strings.Contains(rendered, "╭") || !strings.Contains(rendered, "│") {
		t.Fatalf("expected lipgloss rounded table border, rendered = %q", rendered)
	}
}

func TestCollectDetailLinksFindsURLsMailtoAndEmails(t *testing.T) {
	links := collectDetailLinks("Failed run: https://example.test/run/1. Contact ops@example.test or mailto:oncall@example.test. Read https://example.test/run/1")

	want := []detailLink{
		{Kind: "URL", Label: "https://example.test/run/1", Target: "https://example.test/run/1", Start: 12, End: 38},
		{Kind: "Email", Label: "ops@example.test", Target: "mailto:ops@example.test", Start: 48, End: 64},
		{Kind: "Email", Label: "oncall@example.test", Target: "mailto:oncall@example.test", Start: 68, End: 94},
	}
	if len(links) != len(want) {
		t.Fatalf("links = %#v", links)
	}
	for index := range want {
		if links[index] != want[index] {
			t.Fatalf("links[%d] = %#v, want %#v", index, links[index], want[index])
		}
	}
}

func TestFullDetailContentRendersLinksSection(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "In Progress", Priority: "High", IssueType: "Story", Assignee: "A Developer", URL: "https://example.atlassian.net/browse/ABC-1"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       model.issues[0],
			Description: "Failed run: https://github.example.test/actions/1\nEmail ops@example.test.",
		},
	}
	model.jumpDetailSection("Links")

	content := model.fullDetailContent(90)

	for _, want := range []string{"Links", "https://github.example.test/actions/1", "ops@example.test"} {
		if !strings.Contains(content, want) {
			t.Fatalf("missing %q in %q", want, content)
		}
	}
	if strings.Contains(content, "mailto:ops@example.test") {
		t.Fatalf("email rows should hide mailto noise in rendered detail: %q", content)
	}
}

func TestFullDetailContentRendersCommentsSection(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	created := time.Date(2026, 6, 13, 10, 15, 0, 0, time.Local)
	model.width = 100
	model.height = 20
	model.mode = modeDetail
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "In Progress", Priority: "High", IssueType: "Story", Assignee: "A Developer", URL: "https://example.atlassian.net/browse/ABC-1"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       model.issues[0],
			Description: "Use `main.tf`.",
		},
	}
	model.comments = map[string][]jira.Comment{
		"ABC-1": {
			{ID: "10001", Author: "Comment Person", Body: "Please check `main.tf`.", Created: created},
		},
	}
	model.detailFocus = 3

	content := model.fullDetailContent(80)

	for _, want := range []string{"Comments", "Comment 1/1", "Comment Person", "Please check", "main.tf"} {
		if !strings.Contains(content, want) {
			t.Fatalf("missing %q in %q", want, content)
		}
	}

}

func TestFullDetailContentShowsCommentLimitHint(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "In Progress", Priority: "High", IssueType: "Story", Assignee: "A Developer"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       model.issues[0],
			Description: "Use `main.tf`.",
		},
	}
	comments := make([]jira.Comment, 0, maxComments)
	for index := 0; index < maxComments; index++ {
		comments = append(comments, jira.Comment{ID: fmt.Sprintf("%d", index), Author: "Comment Person", Body: "Comment body."})
	}
	model.comments = map[string][]jira.Comment{"ABC-1": comments}
	model.detailFocus = 3

	content := model.fullDetailContent(80)

	if !strings.Contains(content, fmt.Sprintf("Showing latest %d comments.", maxComments)) {
		t.Fatalf("missing comment limit hint in %q", content)
	}
}

func TestDetailTabsMoveFocusAndActivateSection(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "In Progress", Priority: "High", IssueType: "Story", Assignee: "A Developer"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       model.issues[0],
			Description: "First line.\n\nSecond line.",
		},
	}

	view := model.render()
	for _, want := range []string{"Description", "Hierarchy", "Comments", "Actions"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing detail tab %q in %q", want, view)
		}
	}
	if strings.Contains(view, " Summary ") {
		t.Fatalf("summary should not be a detail section tab anymore: %q", view)
	}
	if !strings.Contains(view, "Description") {
		t.Fatalf("expected focused description section in %q", view)
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next := updated.(Model)
	if next.detailFocus != 1 {
		t.Fatalf("detailFocus = %d", next.detailFocus)
	}
	if view := next.render(); !strings.Contains(view, "Hierarchy") {
		t.Fatalf("expected focused hierarchy section in %q", view)
	}
	if next.jql != model.jql {
		t.Fatalf("detail tab should not switch saved view, jql = %q", next.jql)
	}
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)
}

func TestFullDetailContentRendersFocusedSectionWithPreviews(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 110
	model.height = 32
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Parent epic", Status: "In Progress", Priority: "High", IssueType: "Epic", Assignee: "A Developer"},
		{Key: "ABC-2", Summary: "Child task", Status: "To Do", Priority: "Low", IssueType: "Task", Assignee: "B Developer", ParentKey: "ABC-1"},
	}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       model.issues[0],
			Description: "First description line.\n\nSecond line.",
		},
	}
	model.comments = map[string][]jira.Comment{
		"ABC-1": {{ID: "10001", Author: "Comment Person", Body: "Latest comment body."}},
	}

	content := model.fullDetailContent(90)

	for _, want := range []string{"Description", "First description line."} {
		if !strings.Contains(content, want) {
			t.Fatalf("missing focused detail workspace text %q in %q", want, content)
		}
	}
	for _, notWant := range []string{"Collapsed:", "Hierarchy 1", "Comments 1", "Actions"} {
		if strings.Contains(content, notWant) {
			t.Fatalf("inactive sections should stay in the tab bar, found %q in %q", notWant, content)
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

	for _, want := range []string{"ABC-1", "In Progress", "Story", "Fix production thing", "Assignee: A D.", "Priority: High"} {
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
	metaIndex := strings.Index(view, "Priority: High")
	dividerIndex := -1
	if metaIndex >= 0 {
		dividerOffset := strings.Index(view[metaIndex:], "────")
		if dividerOffset >= 0 {
			dividerIndex = metaIndex + dividerOffset
		}
	}
	tabsIndex := strings.Index(view, "Description")
	if summaryIndex < 0 || metaIndex < 0 || dividerIndex < 0 || tabsIndex < 0 {
		t.Fatalf("expected summary, metadata, divider, and tabs in %q", view)
	}
	lines := strings.Split(view, "\n")
	summaryLine, metaLine := -1, -1
	for index, line := range lines {
		if strings.Contains(line, "Fix production thing") {
			summaryLine = index
		}
		if strings.Contains(line, "Priority: High") {
			metaLine = index
		}
	}
	if summaryLine < 0 || metaLine < 0 || metaLine-summaryLine != 2 {
		t.Fatalf("expected spacer between summary and metadata in %q", view)
	}
	if !(metaIndex < dividerIndex && dividerIndex < tabsIndex) {
		t.Fatalf("expected divider between metadata and tabs: meta=%d divider=%d tabs=%d view=%q", metaIndex, dividerIndex, tabsIndex, view)
	}
}

func TestDetailTabsDoNotTruncateWithEllipses(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 90
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "In Progress", Priority: "High", IssueType: "Story", Assignee: "A Developer"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       model.issues[0],
			Description: "First line.",
		},
	}

	tabs := model.renderDetailTabs(42)

	if strings.Contains(tabs, "...") {
		t.Fatalf("detail tabs should not truncate with ellipses: %q", tabs)
	}
	for _, want := range []string{"Desc", "Tree", "Com", "Act"} {
		if !strings.Contains(tabs, want) {
			t.Fatalf("missing compact detail tab %q in %q", want, tabs)
		}
	}
}

func TestDetailTabsShowSectionBadges(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 120
	model.height = 30
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Parent epic", Status: "In Progress", Priority: "High", IssueType: "Epic", Assignee: "A Developer"},
		{Key: "ABC-2", Summary: "Child task", Status: "To Do", Priority: "Low", IssueType: "Task", Assignee: "B Developer", ParentKey: "ABC-1"},
		{Key: "ABC-3", Summary: "Second child", Status: "To Do", Priority: "Low", IssueType: "Task", Assignee: "C Developer", ParentKey: "ABC-1"},
	}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       model.issues[0],
			Description: "Failed run: https://example.test/run/1",
		},
	}
	model.comments = map[string][]jira.Comment{
		"ABC-1": {
			{ID: "10001", Author: "A Developer", Body: "First"},
			{ID: "10002", Author: "B Developer", Body: "Second"},
		},
	}

	tabs := model.renderDetailTabs(100)

	for _, want := range []string{"Hierarchy 2", "Links 1", "Comments 2"} {
		if !strings.Contains(tabs, want) {
			t.Fatalf("missing section badge %q in %q", want, tabs)
		}
	}
}

func TestFullDetailContentRendersHierarchyChildren(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Parent epic", Status: "In Progress", Priority: "High", IssueType: "Epic", Assignee: "A Developer"},
		{Key: "ABC-2", Summary: "Child task", Status: "To Do", Priority: "Low", IssueType: "Task", Assignee: "B Developer", ParentKey: "ABC-1"},
	}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       model.issues[0],
			Description: "Parent description.",
		},
	}
	model.detailFocus = 1

	content := model.fullDetailContent(90)

	for _, want := range []string{"Hierarchy", "ABC-2", "Child task", "To Do", "B D."} {
		if !strings.Contains(content, want) {
			t.Fatalf("missing %q in %q", want, content)
		}
	}
}

func TestDetailHierarchyFocusSelectsAndOpensChild(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Parent epic", Status: "In Progress", Priority: "High", IssueType: "Epic", Assignee: "A Developer"},
		{Key: "ABC-2", Summary: "First child", Status: "To Do", Priority: "Low", IssueType: "Task", Assignee: "B Developer", ParentKey: "ABC-1"},
		{Key: "ABC-3", Summary: "Second child", Status: "To Do", Priority: "Low", IssueType: "Task", Assignee: "C Developer", ParentKey: "ABC-1"},
	}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0], Description: "Parent description."},
	}
	model.detailFocus = 1

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if !next.hierarchyFocus {
		t.Fatal("expected hierarchy focus")
	}
	view := next.render()
	if !strings.Contains(view, "> ABC-2") {
		t.Fatalf("expected first child selected in hierarchy: %q", view)
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	next = updated.(Model)
	if next.selectedHierarchy != 1 {
		t.Fatalf("selectedHierarchy = %d", next.selectedHierarchy)
	}
	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)
	if next.selected != 2 || next.issues[next.selected].Key != "ABC-3" {
		t.Fatalf("selected issue = %d %#v", next.selected, next.issues[next.selected])
	}
	if next.hierarchyFocus {
		t.Fatal("expected hierarchy focus to clear after opening child")
	}
	if cmd == nil {
		t.Fatal("expected opening child to request detail")
	}
	if len(next.detailBackStack) != 1 || next.detailBackStack[0] != 0 {
		t.Fatalf("detailBackStack = %#v", next.detailBackStack)
	}

	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "esc", Code: tea.KeyEsc}))
	next = updated.(Model)
	if next.mode != modeDetail {
		t.Fatalf("mode = %v, want detail", next.mode)
	}
	if next.selected != 0 || next.issues[next.selected].Key != "ABC-1" {
		t.Fatalf("selected issue after back = %d %#v", next.selected, next.issues[next.selected])
	}
	if len(next.detailBackStack) != 0 {
		t.Fatalf("detailBackStack after back = %#v", next.detailBackStack)
	}
	if cmd == nil {
		t.Fatal("expected returning to parent detail to request detail")
	}
}

func TestDetailActionsFocusRunsSafeActionsAndBlocksMetadataActions(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Parent epic", Status: "In Progress", Priority: "High", IssueType: "Epic", Assignee: "A Developer", URL: "https://example.test/browse/ABC-1"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0], Description: "Parent description."},
	}
	model.detailFocus = 3

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if !next.actionFocus {
		t.Fatal("expected action focus")
	}
	view := next.render()
	for _, want := range []string{"ACTION", "STATE", "Add Comment", "ready", "Edit Fields", "metadata"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	if activeKeyContext(next) != keyContextActions {
		t.Fatalf("activeKeyContext = %q", activeKeyContext(next))
	}

	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)
	if cmd != nil {
		t.Fatal("add comment action should not submit work immediately")
	}
	if next.mode != modeComment {
		t.Fatalf("mode = %v, want comment", next.mode)
	}

	model.actionFocus = true
	model.selectedAction = 4
	updated, cmd = model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)
	if cmd != nil {
		t.Fatal("disabled metadata action should not produce command")
	}
	if !strings.Contains(next.detailNotice, "needs Jira metadata") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestFullDetailContentRendersNoticeBlock(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "In Progress", Priority: "High", IssueType: "Story", Assignee: "A Developer"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       model.issues[0],
			Description: "Use `main.tf`.",
		},
	}
	model.detailNotice = "Copy URL failed: clipboard unavailable."

	content := model.fullDetailContent(80)

	for _, want := range []string{"Notice", "Copy URL failed"} {
		if !strings.Contains(content, want) {
			t.Fatalf("missing %q in %q", want, content)
		}
	}
}

func TestFullDetailContentKeepsDescriptionSectionWhileLoading(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "In Progress", Priority: "High", IssueType: "Story", Assignee: "A Developer"}}
	model.detailLoading = true
	model.detailRequestKey = "ABC-1"
	model.detailNotice = "Copy URL failed: clipboard unavailable."

	content := model.fullDetailContent(80)

	for _, want := range []string{"Description", "Loading issue detail", "Notice", "Copy URL failed"} {
		if !strings.Contains(content, want) {
			t.Fatalf("missing %q in %q", want, content)
		}
	}
}

func TestFullDetailContentDoesNotAppendDuplicateHierarchyFooter(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "In Progress", Priority: "High", IssueType: "Story", Assignee: "A Developer", URL: "https://example.atlassian.net/browse/ABC-1"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       model.issues[0],
			Reporter:    "Reporter",
			Creator:     "Creator",
			Description: "Use `main.tf`.",
		},
	}

	content := model.fullDetailContent(80)

	if strings.Contains(content, "Details\n") {
		t.Fatalf("metadata should be compact, got %q", content)
	}
	for _, want := range []string{"Description"} {
		if !strings.Contains(content, want) {
			t.Fatalf("missing %q in %q", want, content)
		}
	}
	for _, notWant := range []string{"Summary", "Assignee A Developer", "Collapsed:", "Parent No parent", "URL https://example.atlassian.net/browse/ABC-1"} {
		if strings.Contains(content, notWant) {
			t.Fatalf("duplicate detail footer or metadata leaked into the body, found %q in %q", notWant, content)
		}
	}
}

func TestScrollableDetailStatusNamesFocusedSection(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.height = 12
	model.detailFocus = 2
	content := strings.Join([]string{
		"Comments",
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"line 5",
		"line 6",
		"line 7",
		"line 8",
		"line 9",
	}, "\n")

	rendered := model.renderScrollableDetailBody(content, 60)

	if !strings.Contains(rendered, "Comments") {
		t.Fatalf("expected focused section label in scroll status: %q", rendered)
	}
	if !strings.Contains(rendered, "Lines 1-") {
		t.Fatalf("expected line range in scroll status: %q", rendered)
	}
}

func TestFullDetailContentKeepsBetterSelectedAssigneeOverPrivacyAlias(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "In Progress", Priority: "High", IssueType: "Story", Assignee: "Jon Charette"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       jira.Issue{Key: "ABC-1", Summary: "Fix production thing", Status: "In Progress", Priority: "High", IssueType: "Story", Assignee: "User e31ec"},
			Description: "Use `main.tf`.",
		},
	}

	content := model.render()

	if !strings.Contains(content, "Jon C.") {
		t.Fatalf("expected selected issue assignee to be preserved in %q", content)
	}
	if strings.Contains(content, "User e31ec") {
		t.Fatalf("generic Jira privacy alias leaked into detail content: %q", content)
	}
}

func TestWrapRichTextPreservesTableRows(t *testing.T) {
	got := wrapRichText("[table]\n| Module block | Workspace name pattern | Affected? |\n|--------------|------------------------|-----------|\n| stream_processing_1 | ${env}-dpmetadata-stream-instance | Yes |\n[/table]", 46)
	want := "[table]\n| Module block | Workspace name  | Affected? |\n|              | pattern         |           |\n|--------------|-----------------|-----------|\n| stream_pr... | ${env}-dpmet... | Yes       |\n[/table]"
	if got != want {
		t.Fatalf("wrapped table = %q", got)
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

	msg := model.submitIssueSearch(9)()
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

	msg := model.submitIssueSearch(9)()
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

type fakeIssueSearcher struct {
	issues       []jira.Issue
	detail       jira.IssueDetail
	comments     []jira.Comment
	addedComment jira.Comment
	addMentions  []jira.Mention
	users        []jira.User
	err          error
}

func (f *fakeIssueSearcher) SearchIssues(context.Context, string, int) ([]jira.Issue, error) {
	if f.err != nil {
		return nil, f.err
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
