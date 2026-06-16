package tui

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/jon/jira-tui/internal/claude"
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

	for _, want := range []string{"Ticket Detail", "esc back", "j/k scroll", "tab section", "a ai", "o open"} {
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

func TestDetailTabFocusesEditableFieldsBeforeSections(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Assignee: "Jane Doe", Priority: "Medium", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}

	if target, ok := model.focusedDetailTarget(); !ok || target.ID != "summary" {
		t.Fatalf("initial target = %#v ok=%v", target, ok)
	}
	view := model.render()
	if !strings.Contains(view, "Summary: Story") || !strings.Contains(view, "enter edit") {
		t.Fatalf("initial field focus should expose summary edit affordance: %q", view)
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next := updated.(Model)
	if target, ok := next.focusedDetailTarget(); !ok || target.ID != "assignee" {
		t.Fatalf("after first tab target = %#v ok=%v", target, ok)
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	if target, ok := next.focusedDetailTarget(); !ok || target.ID != "priority" {
		t.Fatalf("after second tab target = %#v ok=%v", target, ok)
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	if section, ok := next.focusedDetailSection(); !ok || section.ID != "description" {
		t.Fatalf("after third tab section = %#v ok=%v", section, ok)
	}
}

func TestDetailEnterOnFocusedSummaryOpensEditor(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Priority: "Medium", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}
	model.editMetadata = map[string]jira.EditMetadata{
		"ABC-1": {Summary: jira.EditField{ID: "summary", Name: "Summary", Editable: true}},
	}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("cached metadata should open summary editor without loading")
	}
	if !next.summaryEditing {
		t.Fatal("expected summary editor")
	}
	if !strings.Contains(next.render(), "Edit Summary") {
		t.Fatalf("missing summary modal in %q", next.render())
	}
}

func TestDetailEnterOnFocusedPriorityOpensPicker(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Priority: "Medium", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}
	model.editMetadata = map[string]jira.EditMetadata{
		"ABC-1": {
			Priority: jira.EditField{
				ID:       "priority",
				Name:     "Priority",
				Editable: true,
				AllowedValues: []jira.FieldOption{
					{ID: "2", Name: "High"},
					{ID: "3", Name: "Medium"},
				},
			},
		},
	}
	model.moveDetailFocus(2)

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("cached metadata should open priority picker without loading")
	}
	if !next.priorityFocus {
		t.Fatal("expected priority picker focus")
	}
	if !strings.Contains(next.render(), "Change Priority") {
		t.Fatalf("missing priority modal in %q", next.render())
	}
}

func TestDetailFooterShowsHierarchyCommandsWhenHierarchySelected(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 140
	model.height = 40
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Parent story", IssueType: "Story"},
		{Key: "ABC-2", Summary: "Child task", IssueType: "Task", ParentKey: "ABC-1"},
	}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: jira.Issue{Key: "ABC-1", Summary: "Parent story", IssueType: "Story"}},
	}
	focusDetailSectionForTest(t, &model, "Hierarchy")

	view := model.render()

	for _, want := range []string{"Ticket Detail", "j/k child", "enter focus"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	if strings.Contains(view, "j/k scroll") {
		t.Fatalf("hierarchy footer should prioritize child movement over detail scroll: %q", view)
	}
}

func TestDetailFooterShowsLinkCommandsWhenLinksSelected(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 140
	model.height = 40
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", URL: "https://example.test/browse/ABC-1"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       jira.Issue{Key: "ABC-1", Summary: "Story"},
			Description: "See https://example.test/run.",
		},
	}
	model.jumpDetailSection("Links")

	view := model.render()

	for _, want := range []string{"Ticket Detail", "j/k link", "enter focus", "y copy"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestDetailFooterShowsActionCommandsWhenActionsSelected(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 140
	model.height = 40
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", URL: "https://example.test/browse/ABC-1"}}
	focusDetailSectionForTest(t, &model, "Actions")

	view := model.render()

	for _, want := range []string{"Ticket Detail", "j/k action", "enter focus"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestSelectedLinksSectionCommandsWorkBeforeActivation(t *testing.T) {
	var opened string
	var copied string
	withLinkActions(t, func(value string) error {
		opened = value
		return nil
	}, func(value string) error {
		copied = value
		return nil
	})
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 40
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       jira.Issue{Key: "ABC-1", Summary: "Story"},
			Description: "One https://example.test/one\nTwo https://example.test/two",
		},
	}
	model.jumpDetailSection("Links")

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "down", Code: tea.KeyDown}))
	next := updated.(Model)
	if next.selectedLink != 1 {
		t.Fatalf("selectedLink = %d", next.selectedLink)
	}
	if next.detailOffset != 0 {
		t.Fatalf("detailOffset = %d", next.detailOffset)
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)
	if !next.linkFocus {
		t.Fatal("expected enter to focus links before opening")
	}

	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected focused link open command")
	}
	updated, _ = next.Update(cmd())
	next = updated.(Model)
	if opened != "https://example.test/two" {
		t.Fatalf("opened = %q", opened)
	}

	next.linkFocus = false
	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "y", Code: 'y'}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected copy link command")
	}
	updated, _ = next.Update(cmd())
	next = updated.(Model)
	if copied != "https://example.test/two" {
		t.Fatalf("copied = %q", copied)
	}
}

func TestSelectedActionsSectionCommandsWorkBeforeActivation(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 40
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", URL: "https://example.test/browse/ABC-1"}}
	focusDetailSectionForTest(t, &model, "Actions")

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "down", Code: tea.KeyDown}))
	next := updated.(Model)
	if next.selectedAction != 1 {
		t.Fatalf("selectedAction = %d", next.selectedAction)
	}
	if next.detailOffset != 0 {
		t.Fatalf("detailOffset = %d", next.detailOffset)
	}

	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)
	if cmd != nil {
		t.Fatal("expected enter to focus actions before running")
	}
	if !next.actionFocus {
		t.Fatal("expected action focus")
	}
}

func TestClaudeSectionRequiresEnabledAvailableTicketPlan(t *testing.T) {
	base := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer base.workers.Stop()
	base.mode = modeDetail
	base.issues = []jira.Issue{{Key: "ABC-1", Summary: "Plan this", Status: "To Do"}}

	if hasDetailSection(base, "claude") {
		t.Fatal("Claude section should be hidden by default")
	}

	enabled := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer enabled.workers.Stop()
	enabled.mode = modeDetail
	enabled.issues = base.issues

	if !hasDetailSection(enabled, "claude") {
		t.Fatal("Claude section should be visible when enabled, available, and ticket_plan is true")
	}
}

func TestClaudeSectionVisibleWithTicketAssistOnly(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Sanitize this", Status: "To Do"}}
	model.jumpDetailSection("Claude")

	if !hasDetailSection(model, "claude") {
		t.Fatal("Claude section should be visible when ticket_assist is enabled")
	}
	view := model.render()
	for _, want := range []string{"Ticket Assist", "Evaluate and rewrite this ticket"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestDetailAKeyJumpsToClaudeWhenAIAvailable(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Sanitize this", Status: "To Do"}}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "a", Code: 'a'}))
	next := updated.(Model)

	assertFocusedDetailSection(t, next, "Claude")
	if next.mode == modeComment {
		t.Fatal("a should jump to AI when Claude is available, not open comment compose")
	}
}

func TestDescriptionFocusShowsInlineAIWhenClaudeTicketAssistAvailable(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Improve this", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{"ABC-1": {Issue: model.issues[0], Description: "Old description"}}
	model.jumpDetailSection("Description")

	view := model.render()
	if !strings.Contains(view, "a AI") {
		t.Fatalf("expected inline AI footer hint in %q", view)
	}
}

func TestDescriptionAKeyOpensInlineAIPicker(t *testing.T) {
	model := newInlineDescriptionAIModel(t)
	model.jumpDetailSection("Description")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "a"}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("picker should not start Claude")
	}
	if !next.inlineAIOpen {
		t.Fatal("expected inline AI picker open")
	}
	view := next.render()
	for _, want := range []string{"AI for Description", "Improve clarity", "Extract acceptance criteria", "Ask Claude a question", "Draft clarifying comment", "enter run", "esc cancel"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestInlineAIPickerCanCancel(t *testing.T) {
	model := newInlineDescriptionAIModel(t)
	model.jumpDetailSection("Description")
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "a"}))
	model = updated.(Model)

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "esc"}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("cancel should not start work")
	}
	if next.inlineAIOpen {
		t.Fatal("expected inline AI picker closed")
	}
}

func TestInlineDescriptionAIImproveClaritySubmitsScopedPrompt(t *testing.T) {
	runner := &fakeClaudeRunner{result: claude.Result{Text: "Review\n- Description is vague\n\nDraft\nClearer description"}}
	model := newInlineDescriptionAIModel(t)
	model.claudeRunner = runner
	model.jumpDetailSection("Description")
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "a"}))
	model = updated.(Model)

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if cmd == nil {
		t.Fatal("expected Claude command")
	}
	if !next.claudeAssistLoading || !next.claudeAssistOpen {
		t.Fatal("expected Claude assist loading modal")
	}

	resultMsg := <-runClaudePlanCommandAsyncForTest(cmd)
	result := resultMsg.(claudeAssistResultMsg)
	if result.err != nil {
		t.Fatalf("Claude result error = %v", result.err)
	}
	for _, want := range []string{"Improve clarity", "Description-scoped", "Do not update Jira", "ABC-1", "Current unclear description", "Please clarify done."} {
		if !strings.Contains(runner.request.Prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, runner.request.Prompt)
		}
	}
}

func TestInlineDescriptionAIAskQuestionUsesInstruction(t *testing.T) {
	runner := &fakeClaudeRunner{result: claude.Result{Text: "Review\n- Answered question\n\nDraft\nThe missing scope is deployment rollback."}}
	model := newInlineDescriptionAIModel(t)
	model.claudeRunner = runner
	model.jumpDetailSection("Description")
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "a"}))
	model = updated.(Model)
	model.selectedInlineAIAction = 2

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	model = updated.(Model)
	if cmd != nil || !model.inlineAIInstructionOpen {
		t.Fatal("expected local instruction editor before Claude")
	}
	model.inlineAIInstruction = "What is missing from this ticket?"
	model.inlineAIInstructionEditor = newClaudeAssistRefineEditor(model.inlineAIInstruction)
	model.inlineAIInstructionReady = true

	updated, cmd = model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	if cmd == nil {
		t.Fatal("expected Claude command")
	}
	<-runClaudePlanCommandAsyncForTest(cmd)
	if !strings.Contains(runner.request.Prompt, "What is missing from this ticket?") {
		t.Fatalf("prompt missing question:\n%s", runner.request.Prompt)
	}
}

func newInlineDescriptionAIModel(t *testing.T) Model {
	t.Helper()
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second, AllowJiraWrites: true, RequireConfirmation: true}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	model.loading = false
	model.mode = modeDetail
	model.width = 130
	model.height = 42
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Improve this", Status: "To Do", Priority: "P2", Assignee: "Jon"}}
	model.details = map[string]jira.IssueDetail{"ABC-1": {Issue: model.issues[0], Description: "Current unclear description", Reporter: "Rae"}}
	model.comments = map[string][]jira.Comment{"ABC-1": {{Author: "Rae", Body: "Please clarify done."}}}
	t.Cleanup(model.workers.Stop)
	return model
}

func TestClaudeSectionSubmitsTicketPlanWithSelectedContext(t *testing.T) {
	runner := &fakeClaudeRunner{result: claude.Result{Text: "Implementation plan"}}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "/usr/local/bin/claude"}),
		WithClaudeRunner(runner),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "To Do", Priority: "High", Assignee: "Jon"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       model.issues[0],
			Description: "The deployment fails during migration.",
		},
	}
	model.comments = map[string][]jira.Comment{
		"ABC-1": {{Author: "Rae", Body: "Please include tests."}},
	}
	model.jumpDetailSection("Claude")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if !next.claudePlanLoading {
		t.Fatal("expected Claude plan loading")
	}
	if cmd == nil {
		t.Fatal("expected Claude plan command")
	}

	resultMsg := <-runClaudePlanCommandAsyncForTest(cmd)
	result, ok := resultMsg.(claudePlanResultMsg)
	if !ok {
		t.Fatalf("message = %#v", resultMsg)
	}
	if result.err != nil {
		t.Fatalf("Claude result error = %v", result.err)
	}
	if !strings.Contains(runner.request.Prompt, "ABC-1") ||
		!strings.Contains(runner.request.Prompt, "Fix production thing") ||
		!strings.Contains(runner.request.Prompt, "The deployment fails during migration.") ||
		!strings.Contains(runner.request.Prompt, "Please include tests.") {
		t.Fatalf("prompt missing ticket context:\n%s", runner.request.Prompt)
	}

	updated, _ = next.Update(result)
	next = updated.(Model)
	view := next.render()
	for _, want := range []string{"Claude Ticket Plan", "Implementation plan", "esc close"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	var sawSubmit bool
	var sawResult bool
	for _, event := range next.diagnosticsEvents {
		if event.Kind != diagnosticKindClaude || event.Label != "ticket_plan" {
			continue
		}
		if event.Status == "submit" && strings.Contains(event.Detail, "ABC-1") {
			sawSubmit = true
		}
		if event.Status == "ok" && strings.Contains(event.Detail, "ABC-1") {
			sawResult = true
		}
	}
	if !sawSubmit || !sawResult {
		t.Fatalf("missing Claude diagnostics submit=%t result=%t events=%#v", sawSubmit, sawResult, next.diagnosticsEvents)
	}
}

func TestClaudeSectionSubmitsTicketAssistWithSelectedContext(t *testing.T) {
	runner := &fakeClaudeRunner{result: claude.Result{Text: "Review\n- Missing acceptance criteria\n\nDraft\nSummary: Better summary\n\nAcceptance Criteria\n- [ ] User can edit this before Jira changes."}}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "/usr/local/bin/claude"}),
		WithClaudeRunner(runner),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix unclear deployment story", Status: "To Do", Priority: "High", Assignee: "Jon"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       model.issues[0],
			Description: "Need to do deploy stuff.",
		},
	}
	model.comments = map[string][]jira.Comment{
		"ABC-1": {{Author: "Rae", Body: "What does done mean?"}},
	}
	model.jumpDetailSection("Claude")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if !next.claudeAssistLoading {
		t.Fatal("expected Claude assist loading")
	}
	if cmd == nil {
		t.Fatal("expected Claude assist command")
	}

	resultMsg := <-runClaudePlanCommandAsyncForTest(cmd)
	result, ok := resultMsg.(claudeAssistResultMsg)
	if !ok {
		t.Fatalf("message = %#v", resultMsg)
	}
	if result.err != nil {
		t.Fatalf("Claude assist result error = %v", result.err)
	}
	for _, want := range []string{
		"Evaluate and sanitize this existing Jira ticket",
		"Do not update Jira",
		"Acceptance Criteria",
		"ABC-1",
		"Need to do deploy stuff.",
		"What does done mean?",
	} {
		if !strings.Contains(runner.request.Prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, runner.request.Prompt)
		}
	}

	updated, _ = next.Update(result)
	next = updated.(Model)
	view := next.render()
	for _, want := range []string{"Claude Ticket Assist", "Review", "Missing acceptance criteria", "Local Draft", "Acceptance Criteria", "r refine"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	if next.claudeAssistDraftValue() == "" {
		t.Fatal("expected editable Claude assist draft")
	}
}

func TestClaudeTicketAssistDraftIsBoundedAndPaged(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Sanitize this", Status: "To Do"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistText = "Review\n" + strings.Repeat("- Review finding with detail.\n", 20) + "\nDraft\n" + strings.Repeat("Acceptance criterion line with enough detail to wrap cleanly.\n", 40)
	model.claudeAssistDraft = claudeAssistDraftFromText(model.claudeAssistText)
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	view := model.render()
	if len(strings.Split(view, "\n")) > model.height {
		t.Fatalf("view height exceeded terminal height:\n%s", view)
	}
	for _, want := range []string{"Local Draft", "Draft Lines 1-", "pgup/pgdn page", "ctrl+y copy"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "pgdown", Code: tea.KeyPgDown}))
	next := updated.(Model)
	if next.claudeAssistEditor.ScrollYOffset() == 0 {
		t.Fatal("expected pgdown to scroll the Claude assist draft editor")
	}
}

func TestClaudeTicketAssistDraftGetsPrimaryModalSpace(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 150
	model.height = 64
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Sanitize this", Status: "To Do"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistText = "Review\n" + strings.Repeat("- Review finding with detail.\n", 12) + "\nDraft\n" + strings.Repeat("Acceptance criterion line with enough detail to wrap cleanly.\n", 40)
	model.claudeAssistDraft = claudeAssistDraftFromText(model.claudeAssistText)
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	if rows := model.claudeAssistEditorRows(); rows <= 10 {
		t.Fatalf("expected draft editor to get more than old 10-row cap, rows=%d", rows)
	}
	view := model.render()
	if !strings.Contains(view, "Local Draft") {
		t.Fatalf("expected local draft label in %q", view)
	}
	if len(strings.Split(view, "\n")) > model.height {
		t.Fatalf("view height exceeded terminal height:\n%s", view)
	}
}

func TestClaudeTicketAssistOutputHasDistinctZonesAndActions(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second, AllowJiraWrites: true, RequireConfirmation: true}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 150
	model.height = 48
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Sanitize this", Status: "To Do"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistText = "Review\n- Claude found stale context.\n\nDraft\nSummary: Better ticket\n\nAcceptance Criteria\n- [ ] Clear and testable"
	model.claudeAssistDraft = claudeAssistDraftFromText(model.claudeAssistText)
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	view := model.render()
	for _, want := range []string{"Claude Review", "Local Draft", "Not Applied", "Available Actions", "ctrl+s apply", "c comment", "r refine", "ctrl+y copy"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestInlineDescriptionAIResultUsesTicketAssistModal(t *testing.T) {
	model := newInlineDescriptionAIModel(t)
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistTarget = claudeAssistTargetDescription
	model.claudeAssistText = "Review\n- Description was unclear\n\nDraft\nA clearer replacement description."
	model.claudeAssistDraft = claudeAssistDraftFromText(model.claudeAssistText)
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	view := model.render()
	for _, want := range []string{"Claude Ticket Assist", "Claude Review", "Local Draft", "Available Actions", "ctrl+s apply", "c comment", "r refine"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestInlineDescriptionAIApplyUpdatesDescriptionOnly(t *testing.T) {
	searcher := &fakeIssueSearcher{}
	model := NewModel(
		searcher,
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second, AllowJiraWrites: true, RequireConfirmation: true}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 130
	model.height = 42
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Original summary", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{"ABC-1": {Issue: model.issues[0], Description: "Old description"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistTarget = claudeAssistTargetDescription
	model.claudeAssistDraft = "Replacement description only."
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("expected confirmation before applying")
	}
	if !next.claudeAssistConfirmApply {
		t.Fatal("expected apply confirmation")
	}
	view := next.render()
	if !strings.Contains(view, "Apply Description Draft") {
		t.Fatalf("missing Description apply confirmation:\n%s", view)
	}
	if strings.Contains(view, "Apply Ticket Assist Draft") {
		t.Fatalf("inline Description apply should not use ticket apply confirmation:\n%s", view)
	}

	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected description update command")
	}
	msg := cmd()
	if _, ok := msg.(workSubmittedMsg); !ok {
		t.Fatalf("submit message = %#v", msg)
	}
	resultMsg := next.waitForWorkerResult()()
	result := resultMsg.(workerResultMsg)
	updated, _ = next.Update(result)
	if searcher.updateSummaryValue != "" {
		t.Fatalf("summary should not be updated, got %q", searcher.updateSummaryValue)
	}
	if searcher.updateDescriptionValue != "Replacement description only." {
		t.Fatalf("description = %q", searcher.updateDescriptionValue)
	}
}

func TestInlineDescriptionAIDraftCanBePostedAsComment(t *testing.T) {
	searcher := &fakeIssueSearcher{comments: []jira.Comment{{ID: "1", Author: "Current User", Body: "posted"}}}
	model := NewModel(
		searcher,
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second, AllowJiraWrites: true, RequireConfirmation: true}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 130
	model.height = 42
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Original summary", Status: "To Do"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistTarget = claudeAssistTargetDescription
	model.claudeAssistDraft = "Could you confirm the rollback requirement?"
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "c"}))
	next := updated.(Model)
	if !next.claudeAssistConfirmComment {
		t.Fatal("expected comment confirmation")
	}
	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected AddComment command")
	}
	_ = cmd()
	resultMsg := next.waitForWorkerResult()()
	result := resultMsg.(workerResultMsg)
	_, _ = next.Update(result)
	if searcher.addedBody != "Could you confirm the rollback requirement?" {
		t.Fatalf("comment body = %q", searcher.addedBody)
	}
}

func TestClaudeTicketAssistDraftCanBePostedAsComment(t *testing.T) {
	searcher := &fakeIssueSearcher{comments: []jira.Comment{{ID: "10001", Author: "Current User", Body: "posted"}}}
	model := NewModel(
		searcher,
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second, AllowJiraWrites: true, RequireConfirmation: true}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 36
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Original summary", Status: "To Do"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistDraft = "Summary: Better ticket\n\nAcceptance Criteria\n- Clear and testable"
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "c"}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("expected confirmation before posting comment")
	}
	if !next.claudeAssistConfirmComment {
		t.Fatal("expected comment confirmation")
	}
	if view := next.render(); !strings.Contains(view, "Post Draft As Comment") || !strings.Contains(view, "ctrl+s post") {
		t.Fatalf("missing comment confirmation in %q", view)
	}

	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected comment post command")
	}
	if !next.claudeAssistPostingComment {
		t.Fatal("expected posting comment state")
	}
	msg := cmd()
	if _, ok := msg.(workSubmittedMsg); !ok {
		t.Fatalf("submit message = %#v", msg)
	}
	resultMsg := next.waitForWorkerResult()()
	result, ok := resultMsg.(workerResultMsg)
	if !ok {
		t.Fatalf("worker message = %#v", resultMsg)
	}
	updated, cmd = next.Update(result)
	next = updated.(Model)
	if searcher.addedBody != model.claudeAssistDraft {
		t.Fatalf("posted body = %q", searcher.addedBody)
	}
	if next.claudeAssistOpen || next.claudeAssistPostingComment {
		t.Fatalf("expected modal closed after comment, open=%v posting=%v", next.claudeAssistOpen, next.claudeAssistPostingComment)
	}
	if !strings.Contains(next.detailNotice, "Ticket assist draft posted as a comment") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
	if cmd == nil {
		t.Fatal("expected comments refresh command")
	}
}

func TestClaudeTicketAssistDraftCanBeCopied(t *testing.T) {
	var copied string
	withLinkActions(t, nil, func(value string) error {
		copied = value
		return nil
	})
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Sanitize this", Status: "To Do"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistDraft = "Summary: Better ticket\n\nAcceptance Criteria\n- [ ] Clear and testable"
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+y"}))
	next := updated.(Model)
	if cmd == nil {
		t.Fatal("expected copy command")
	}
	msg := cmd()
	linkMsg, ok := msg.(linkActionMsg)
	if !ok {
		t.Fatalf("message = %#v", msg)
	}
	updated, _ = next.Update(linkMsg)
	next = updated.(Model)
	if copied != model.claudeAssistDraft {
		t.Fatalf("copied = %q", copied)
	}
	if !strings.Contains(next.detailNotice, "Copied Claude ticket draft") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestClaudeTicketAssistROpensRefinementEditor(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 36
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Original summary", Status: "To Do"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistDraft = "Summary: Better ticket\n\nAcceptance Criteria\n- Clear and testable"
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "r"}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("opening refinement editor should not run Claude")
	}
	if !next.claudeAssistRefining {
		t.Fatal("expected refinement editor state")
	}
	view := next.render()
	for _, want := range []string{"Refine Draft", "Instruction", "ctrl+s send", "esc cancel"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestClaudeTicketAssistRefinementPromptIncludesCurrentEditedDraft(t *testing.T) {
	runner := &fakeClaudeRunner{result: claude.Result{Text: "Review\n- Tightened acceptance criteria\n\nDraft\nSummary: Refined ticket\n\nAcceptance Criteria\n- [ ] Refined criterion"}}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
		WithClaudeRunner(runner),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 36
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Original summary", Status: "To Do", Priority: "High"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0], Description: "Original Jira description."},
	}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistDraft = "Summary: User edited summary\n\nAcceptance Criteria\n- User edited criterion"
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "r"}))
	next := updated.(Model)
	next.claudeAssistRefineInstruction = "make the acceptance criteria measurable"
	next.claudeAssistRefineEditor = newClaudeAssistRefineEditor(next.claudeAssistRefineInstruction)
	next.claudeAssistRefineEditorReady = true
	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected refinement Claude command")
	}
	if !next.claudeAssistLoading {
		t.Fatal("expected loading while refining")
	}

	resultMsg := <-runClaudePlanCommandAsyncForTest(cmd)
	result, ok := resultMsg.(claudeAssistResultMsg)
	if !ok {
		t.Fatalf("message = %#v", resultMsg)
	}
	for _, want := range []string{
		"Refine this Jira ticket draft",
		"make the acceptance criteria measurable",
		"Current user-edited draft",
		"Summary: User edited summary",
		"User edited criterion",
		"Original Jira description.",
	} {
		if !strings.Contains(runner.request.Prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, runner.request.Prompt)
		}
	}

	updated, _ = next.Update(result)
	next = updated.(Model)
	if !strings.Contains(next.claudeAssistDraftValue(), "Refined criterion") {
		t.Fatalf("draft = %q", next.claudeAssistDraftValue())
	}
}

func TestClaudeTicketAssistCtrlSRequiresJiraWriteGate(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second, AllowJiraWrites: false, RequireConfirmation: true}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 36
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Original summary", Status: "To Do"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistDraft = "Summary: Better ticket\n\nAcceptance Criteria\n- Clear and testable"
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("expected no Jira write command when writes are disabled")
	}
	if !next.claudeAssistOpen {
		t.Fatal("draft should stay open when writes are disabled")
	}
	if next.claudeAssistConfirmApply || next.claudeAssistApplying {
		t.Fatalf("unexpected apply state confirm=%v applying=%v", next.claudeAssistConfirmApply, next.claudeAssistApplying)
	}
	if !strings.Contains(next.detailNotice, "Jira writes are disabled") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestClaudeTicketAssistCtrlSOpensApplyConfirmation(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second, AllowJiraWrites: true, RequireConfirmation: true}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 36
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Original summary", Status: "To Do"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistDraft = "Summary: Better ticket\n\nProblem / Goal\nMake it clearer.\n\nAcceptance Criteria\n- Clear and testable"
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("expected confirmation before Jira write command")
	}
	if !next.claudeAssistConfirmApply {
		t.Fatal("expected apply confirmation")
	}
	if next.claudeAssistApplySummary != "Better ticket" {
		t.Fatalf("apply summary = %q", next.claudeAssistApplySummary)
	}
	if !strings.Contains(next.claudeAssistApplyDescription, "Acceptance Criteria") {
		t.Fatalf("apply description = %q", next.claudeAssistApplyDescription)
	}
	view := next.render()
	for _, want := range []string{"Apply Ticket Assist Draft", "Summary", "Description", "ctrl+s apply", "esc cancel"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestClaudeTicketAssistConfirmAppliesSummaryAndDescription(t *testing.T) {
	searcher := &fakeIssueSearcher{}
	model := NewModel(
		searcher,
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second, AllowJiraWrites: true, RequireConfirmation: true}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 36
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Original summary", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: jira.Issue{Key: "ABC-1", Summary: "Original summary"}, Description: "Original description"},
	}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistDraft = "Summary: Better ticket\n\nProblem / Goal\nMake it clearer.\n\nAcceptance Criteria\n- Clear and testable"
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next := updated.(Model)
	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected apply command")
	}
	if !next.claudeAssistApplying {
		t.Fatal("expected applying state")
	}

	submitBatch := cmd()
	batch, ok := submitBatch.(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("submit command = %#v", submitBatch)
	}
	for _, sub := range batch {
		if msg := sub(); msg == nil {
			t.Fatal("expected work submitted message")
		}
	}
	for i := 0; i < 2; i++ {
		msg := next.waitForWorkerResult()()
		result, ok := msg.(workerResultMsg)
		if !ok {
			t.Fatalf("worker message = %#v", msg)
		}
		updated, _ = next.Update(result)
		next = updated.(Model)
	}

	if searcher.updateSummaryKey != "ABC-1" || searcher.updateSummaryValue != "Better ticket" {
		t.Fatalf("summary update = %s/%s", searcher.updateSummaryKey, searcher.updateSummaryValue)
	}
	if searcher.updateDescriptionKey != "ABC-1" || !strings.Contains(searcher.updateDescriptionValue, "Acceptance Criteria") {
		t.Fatalf("description update = %s/%s", searcher.updateDescriptionKey, searcher.updateDescriptionValue)
	}
	if next.claudeAssistOpen || next.claudeAssistApplying {
		t.Fatalf("expected modal closed after apply, open=%v applying=%v", next.claudeAssistOpen, next.claudeAssistApplying)
	}
	if next.issues[0].Summary != "Better ticket" {
		t.Fatalf("issue summary = %q", next.issues[0].Summary)
	}
	if next.details["ABC-1"].Description != searcher.updateDescriptionValue {
		t.Fatalf("detail description = %q", next.details["ABC-1"].Description)
	}
	if !strings.Contains(next.detailNotice, "Ticket assist draft applied") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestClaudeTicketAssistApplyFailureKeepsDraftOpen(t *testing.T) {
	applyErr := errors.New("jira rejected description")
	model := NewModel(
		&fakeIssueSearcher{err: applyErr},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second, AllowJiraWrites: true, RequireConfirmation: true}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 36
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Original summary", Status: "To Do"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistDraft = "Summary: Better ticket\n\nAcceptance Criteria\n- Clear and testable"
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next := updated.(Model)
	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected apply command")
	}
	batch := cmd().(tea.BatchMsg)
	for _, sub := range batch {
		_ = sub()
	}
	msg := next.waitForWorkerResult()()
	result := msg.(workerResultMsg)
	updated, _ = next.Update(result)
	next = updated.(Model)

	if !next.claudeAssistOpen {
		t.Fatal("draft should stay open after apply failure")
	}
	if next.claudeAssistDraftValue() != model.claudeAssistDraft {
		t.Fatalf("draft changed to %q", next.claudeAssistDraftValue())
	}
	if !strings.Contains(next.detailNotice, "Ticket assist apply failed") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestClaudeTicketAssistLoadingSuppressesAssistantPreview(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Minute}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Sanitize this", Status: "To Do"}}
	model.claudeAssistOpen = true
	model.claudeAssistLoading = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistStartedAt = time.Date(2026, 6, 14, 15, 13, 0, 0, time.Local)
	model.claudeAssistProgress = []claude.Event{
		{Kind: "output", Text: "produce the expected precondition/validation errors"},
	}

	view := model.render()
	if !strings.Contains(view, "Output: receiving response") {
		t.Fatalf("missing calm output status in %q", view)
	}
	if strings.Contains(view, "produce the expected precondition") || strings.Contains(view, "Assistant:") {
		t.Fatalf("assist loading modal leaked assistant preview: %q", view)
	}
}

func TestClaudePlanShowsSubprocessActivityAndCancelHintWhileRunning(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Timeout: 2 * time.Minute}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
		WithClaudeRunner(&fakeClaudeRunner{result: claude.Result{Text: "Implementation plan"}}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "To Do"}}
	model.jumpDetailSection("Claude")

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	next.claudePlanStartedAt = time.Date(2026, 6, 14, 15, 13, 0, 0, time.Local)
	now := next.claudePlanStartedAt.Add(3 * time.Second)
	next.now = func() time.Time { return now }

	view := next.render()
	for _, want := range []string{
		"Asking Claude",
		"Activity:",
		"Claude subprocess running",
		"Elapsed: 3s of 2m0s",
		"Output: waiting for first response",
		"esc cancel",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	for _, unwanted := range []string{
		"Request:",
		"Mode:",
		"Command:",
		"Started:",
		"Deadline:",
	} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("debug field %q leaked into loading modal: %q", unwanted, view)
		}
	}
}

func TestClaudePlanTimeoutShowsElapsedDeadlineContext(t *testing.T) {
	started := time.Date(2026, 6, 14, 15, 13, 0, 0, time.Local)
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Command: "/Users/joncha/.local/bin/claude", Timeout: 2 * time.Minute}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "/Users/joncha/.local/bin/claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "To Do"}}
	model.claudePlanOpen = true
	model.claudePlanKey = "ABC-1"
	model.claudePlanStartedAt = started
	model.claudePlanErr = context.DeadlineExceeded
	model.now = func() time.Time { return started.Add(2*time.Minute + 3*time.Second) }

	view := model.render()
	for _, want := range []string{
		"Claude plan timed out after 2m0s",
		"Started: 15:13:00",
		"Deadline: 15:15:00",
		"Elapsed: 2m3s",
		"Command: /Users/joncha/.local/bin/claude -p <prompt>",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestClaudePlanProgressAppearsBeforeFinalResult(t *testing.T) {
	runner := &fakeClaudeRunner{
		events: []claude.Event{
			{Kind: "stderr", Text: "Not logged in - please run /login"},
		},
		waitForCancel: true,
	}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Timeout: time.Minute}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
		WithClaudeRunner(runner),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "To Do"}}
	model.jumpDetailSection("Claude")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	progress := <-runClaudePlanProgressAsyncForTest(cmd)

	updated, _ = next.Update(progress)
	next = updated.(Model)
	view := next.render()
	for _, want := range []string{"Output: receiving response"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	if strings.Contains(view, "Not logged in - please run /login") || strings.Contains(view, "Assistant:") {
		t.Fatalf("loading modal should not stream assistant text: %q", view)
	}

	next = next.cancelClaudeTicketPlan()
}

func TestClaudeProgressModalSuppressesAssistantPreviewWhileLoading(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Timeout: time.Minute}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "To Do"}}
	model.claudePlanOpen = true
	model.claudePlanLoading = true
	model.claudePlanKey = "ABC-1"
	model.claudePlanStartedAt = time.Date(2026, 6, 14, 15, 13, 0, 0, time.Local)
	model.claudePlanProgress = []claude.Event{
		{Kind: "stream_event", Text: `{"type":"stream_event","event":{"type":"message_delta"}}`},
		{Kind: "output", Text: "use ALB target groups"},
	}

	view := model.render()
	if !strings.Contains(view, "Output: receiving response") {
		t.Fatalf("missing calm output status in %q", view)
	}
	if strings.Contains(view, `{"type":"stream_event"`) || strings.Contains(view, "use ALB target groups") || strings.Contains(view, "Assistant:") {
		t.Fatalf("loading modal leaked stream detail: %q", view)
	}
}

func TestClaudeProgressModalIgnoresProtocolNoise(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Timeout: time.Minute}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "To Do"}}
	model.claudePlanOpen = true
	model.claudePlanLoading = true
	model.claudePlanKey = "ABC-1"
	model.claudePlanStartedAt = time.Date(2026, 6, 14, 15, 13, 0, 0, time.Local)
	model.claudePlanProgress = []claude.Event{
		{Kind: "user", Text: "Create a read-only implementation plan"},
		{Kind: "status", Text: "message stop"},
	}

	view := model.render()
	if !strings.Contains(view, "Output: receiving CLI messages") {
		t.Fatalf("missing waiting state in %q", view)
	}
	if strings.Contains(view, "Output: user") || strings.Contains(view, "Create a read-only implementation plan") {
		t.Fatalf("protocol noise leaked into modal: %q", view)
	}
}

func TestClaudeProgressModalDedupesRepeatedOutput(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Timeout: time.Minute}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "To Do"}}
	model.claudePlanOpen = true
	model.claudePlanLoading = true
	model.claudePlanKey = "ABC-1"
	model.claudePlanStartedAt = time.Date(2026, 6, 14, 15, 13, 0, 0, time.Local)
	repeated := "I'll start by exploring the local environment to ground the plan."
	model.claudePlanProgress = []claude.Event{
		{Kind: "output", Text: repeated},
		{Kind: "output", Text: repeated},
	}

	preview := claudeAssistantPreview(model.claudePlanProgress)
	if preview != repeated {
		t.Fatalf("preview = %q", preview)
	}
}

func TestClaudeProgressAssemblesDeltaChunks(t *testing.T) {
	events := []claude.Event{
		{Kind: "output", Text: "I'll start by checking the "},
		{Kind: "output", Text: "Terraform modules and then outline "},
		{Kind: "output", Text: "the verification plan."},
	}

	preview := claudeAssistantPreview(events)
	want := "Assistant: I'll start by checking the Terraform modules"
	if !strings.Contains("Assistant: "+preview, want) {
		t.Fatalf("missing assembled preview %q in %q", want, preview)
	}
	if strings.Contains("Assistant: "+preview, "Assistant: the verification plan") {
		t.Fatalf("preview showed tail fragment instead of assembled beginning: %q", preview)
	}
}

func TestClaudePlanLongResultStaysInsideViewport(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Timeout: time.Minute}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 32
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "To Do"}}
	model.claudePlanOpen = true
	model.claudePlanKey = "ABC-1"
	model.claudePlanText = strings.Repeat("Claude result line with implementation detail.\n", 80)

	view := model.render()
	lines := strings.Split(view, "\n")
	if len(lines) > model.height {
		t.Fatalf("view height = %d, want <= %d\n%s", len(lines), model.height, view)
	}
	if !strings.Contains(view, "Claude Lines 1-") {
		t.Fatalf("missing Claude line range in %q", view)
	}
	if !strings.Contains(view, "j/k scroll") || !strings.Contains(view, "pgup/pgdn page") {
		t.Fatalf("missing scroll footer in %q", view)
	}
}

func TestClaudePlanMarkdownTableRendersAsBoundedTable(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Timeout: time.Minute}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 130
	model.height = 36
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "To Do"}}
	model.claudePlanOpen = true
	model.claudePlanKey = "ABC-1"
	model.claudePlanText = "Findings:\n\n" +
		"| Signal | Value |\n" +
		"|---|---|\n" +
		"| Current repo | terraform-module-eks with only README.md and no Terraform implementation |\n" +
		"| Ticket subject | Internal ALBs for ECS deployments |\n\n" +
		"Next steps."

	view := model.render()
	for _, want := range []string{"Signal", "Current repo", "terraform-module-eks", "Next steps."} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	if strings.Contains(view, "|---|---|") {
		t.Fatalf("raw markdown table separator leaked into Claude modal: %q", view)
	}
	if !strings.Contains(view, "╭") || !strings.Contains(view, "│") {
		t.Fatalf("expected styled table block in %q", view)
	}
}

func TestClaudePlanDialogUsesWideTerminalSpace(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Timeout: time.Minute}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 160
	model.height = 40
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "To Do"}}
	model.claudePlanOpen = true
	model.claudePlanKey = "ABC-1"
	phrase := "This Claude plan sentence should remain readable on a wide terminal without forced narrow wrapping."
	model.claudePlanText = phrase

	view := model.render()
	if !strings.Contains(view, phrase) {
		t.Fatalf("wide Claude modal wrapped phrase unexpectedly:\n%s", view)
	}
}

func TestClaudePlanResultScrollsWithNavigationKeys(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Timeout: time.Minute}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 32
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "To Do"}}
	model.claudePlanOpen = true
	model.claudePlanKey = "ABC-1"
	for i := 1; i <= 40; i++ {
		model.claudePlanText += fmt.Sprintf("Claude result line %02d.\n", i)
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	next := updated.(Model)
	if next.claudePlanOffset != 1 {
		t.Fatalf("offset = %d, want 1", next.claudePlanOffset)
	}
	view := next.render()
	if !strings.Contains(view, "Claude Lines 2-") {
		t.Fatalf("missing scrolled line range in %q", view)
	}
	if strings.Contains(view, "Claude result line 01.") {
		t.Fatalf("first line should be scrolled out of view: %q", view)
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "G", Code: 'G'}))
	next = updated.(Model)
	view = next.render()
	if !strings.Contains(view, "Claude Lines 35-40 of 40") {
		t.Fatalf("missing bottom line range in %q", view)
	}
}

func TestClaudePlanEscCancelsRunningRequest(t *testing.T) {
	runner := &fakeClaudeRunner{waitForCancel: true}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Timeout: time.Minute}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
		WithClaudeRunner(runner),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "To Do"}}
	model.jumpDetailSection("Claude")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if !next.claudePlanLoading {
		t.Fatal("expected Claude plan loading")
	}
	results := runClaudePlanCommandAsyncForTest(cmd)

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "esc", Code: tea.KeyEsc}))
	next = updated.(Model)
	if next.claudePlanLoading {
		t.Fatal("expected Claude plan loading to stop after cancel")
	}
	if next.claudePlanErr == nil || !strings.Contains(next.claudePlanErr.Error(), "cancel") {
		t.Fatalf("expected cancel error, got %v", next.claudePlanErr)
	}

	select {
	case msg := <-results:
		result, ok := msg.(claudePlanResultMsg)
		if !ok {
			t.Fatalf("message = %#v", msg)
		}
		if result.err == nil || !strings.Contains(result.err.Error(), "cancel") {
			t.Fatalf("expected cancelled result error, got %v", result.err)
		}
	case <-time.After(time.Second):
		t.Fatal("Claude command did not unblock after cancel")
	}
}

func TestSelectedHierarchySectionEnterFocusesBeforeOpening(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 40
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Parent story", IssueType: "Story"},
		{Key: "ABC-2", Summary: "Child task", IssueType: "Task", ParentKey: "ABC-1"},
	}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: jira.Issue{Key: "ABC-1", Summary: "Parent story", IssueType: "Story"}},
		"ABC-2": {Issue: jira.Issue{Key: "ABC-2", Summary: "Child task", IssueType: "Task", ParentKey: "ABC-1"}},
	}
	focusDetailSectionForTest(t, &model, "Hierarchy")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("expected enter to focus hierarchy before opening")
	}
	if !next.hierarchyFocus {
		t.Fatal("expected hierarchy focus")
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

func TestFreshCachedDetailSkipsDetailRefresh(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: jira.Issue{Key: "ABC-1"}, Description: "Cached detail"},
	}
	model.comments = map[string][]jira.Comment{"ABC-1": {}}
	model.markIssueDetailFresh("ABC-1")

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
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: jira.Issue{Key: "ABC-1"}, Description: "Stale detail"},
	}
	model.comments = map[string][]jira.Comment{"ABC-1": {}}

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
	focusDetailSectionForTest(t, &model, "Description")

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next := updated.(Model)
	assertFocusedDetailSection(t, next, "Hierarchy")
	if next.detailOffset != 0 {
		t.Fatalf("expected tab to select the next section at its saved scroll, offset=%d", next.detailOffset)
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "h", Code: 'h'}))
	next = updated.(Model)
	assertFocusedDetailSection(t, next, "Hierarchy")
	if next.detailOffset != 0 {
		t.Fatalf("expected h to select hierarchy, offset=%d", next.detailOffset)
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "[", Code: '['}))
	next = updated.(Model)
	if next.focusedDetailTargetID() != "description" || next.detailOffset != 0 {
		t.Fatalf("expected [ to select previous focus target at its saved scroll, target=%q offset=%d", next.focusedDetailTargetID(), next.detailOffset)
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
	focusDetailSectionForTest(t, &model, "Description")

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "pgdown", Code: tea.KeyPgDown}))
	next := updated.(Model)
	descriptionOffset := next.detailOffset
	if descriptionOffset == 0 {
		t.Fatal("expected description scroll offset to advance")
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	assertFocusedDetailSection(t, next, "Hierarchy")
	if next.detailOffset != 0 {
		t.Fatalf("expected hierarchy section at top, offset=%d", next.detailOffset)
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "[", Code: '['}))
	next = updated.(Model)
	if next.focusedDetailTargetID() != "description" || next.detailOffset != descriptionOffset {
		t.Fatalf("expected description offset %d to restore, target=%q offset=%d", descriptionOffset, next.focusedDetailTargetID(), next.detailOffset)
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
	focusDetailSectionForTest(t, &model, "Hierarchy")

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
	model.hierarchyFocus = false
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Parent story", IssueType: "Story"},
		{Key: "ABC-2", Summary: "Child task", IssueType: "Task", ParentKey: "ABC-1"},
	}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: jira.Issue{Key: "ABC-1", Summary: "Parent story", IssueType: "Story"}},
	}
	focusDetailSectionForTest(t, &model, "Hierarchy")

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
	model.hierarchyFocus = false
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Parent story", IssueType: "Story"},
		{Key: "ABC-2", Summary: "First child", IssueType: "Task", ParentKey: "ABC-1"},
		{Key: "ABC-3", Summary: "Second child", IssueType: "Task", ParentKey: "ABC-1"},
	}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: jira.Issue{Key: "ABC-1", Summary: "Parent story", IssueType: "Story"}},
	}
	focusDetailSectionForTest(t, &model, "Hierarchy")

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
	focusDetailSectionForTest(t, &model, "Hierarchy")
	model.hierarchyFocus = true

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

func TestDetailIssueActionsOpenCopyAndIssueURL(t *testing.T) {
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

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "o", Code: 'o'}))
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

func TestDiagnosticsOverlayTogglesWithoutChangingMode(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "One"}}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+d"}))
	next := updated.(Model)

	if !next.diagnosticsOpen {
		t.Fatal("expected diagnostics overlay to open")
	}
	if next.mode != modeDetail {
		t.Fatalf("mode = %v, want detail", next.mode)
	}
	view := next.render()
	for _, want := range []string{"Diagnostics", "Background Activity", "esc close"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "esc", Code: tea.KeyEsc}))
	next = updated.(Model)
	if next.diagnosticsOpen {
		t.Fatal("expected diagnostics overlay to close")
	}
	if next.mode != modeDetail {
		t.Fatalf("mode after close = %v, want detail", next.mode)
	}
}

func TestDiagnosticsRecordsWorkerSubmitAndResult(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()

	updated, _ := model.Update(workSubmittedMsg{
		kind: worker.KindGetIssue,
		id:   42,
		key:  "ABC-1",
	})
	next := updated.(Model)

	updated, _ = next.Update(workerResultMsg{result: worker.Result{
		ID:   42,
		Kind: worker.KindGetIssue,
		GetIssue: &worker.GetIssueResult{
			Key:    "ABC-1",
			Detail: jira.IssueDetail{Issue: jira.Issue{Key: "ABC-1"}},
		},
	}})
	next = updated.(Model)

	if len(next.diagnosticsEvents) != 2 {
		t.Fatalf("diagnostics events = %#v", next.diagnosticsEvents)
	}
	if next.diagnosticsEvents[0].Kind != diagnosticKindWorker || next.diagnosticsEvents[0].Status != "submit" {
		t.Fatalf("submit event = %#v", next.diagnosticsEvents[0])
	}
	if next.diagnosticsEvents[1].Kind != diagnosticKindWorker || next.diagnosticsEvents[1].Status != "ok" {
		t.Fatalf("result event = %#v", next.diagnosticsEvents[1])
	}
}

func TestDiagnosticsRecordsCreateIssueTypeResultCount(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()

	updated, _ := model.Update(workerResultMsg{result: worker.Result{
		ID:   77,
		Kind: worker.KindGetCreateIssueTypes,
		GetCreateIssueTypes: &worker.GetCreateIssueTypesResult{
			ProjectKey: "DEVOPS",
		},
	}})
	next := updated.(Model)

	if len(next.diagnosticsEvents) == 0 {
		t.Fatal("expected diagnostics event")
	}
	got := next.diagnosticsEvents[len(next.diagnosticsEvents)-1].Detail
	for _, want := range []string{"#77", "DEVOPS", "types=0"} {
		if !strings.Contains(got, want) {
			t.Fatalf("diagnostic detail = %q, missing %q", got, want)
		}
	}
}

func TestDiagnosticsRecordsCreateFieldSupportSummary(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()

	updated, _ := model.Update(workerResultMsg{result: worker.Result{
		ID:   78,
		Kind: worker.KindGetCreateFields,
		GetCreateFields: &worker.GetCreateFieldsResult{
			ProjectKey:  "DEVOPS",
			IssueTypeID: "10001",
			Fields: []jira.CreateField{
				{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
				{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
				{ID: "priority", Name: "Priority", Required: true, SchemaSystem: "priority", SchemaType: "priority", AllowedValues: []jira.FieldOption{{ID: "3", Name: "Medium"}}},
				{ID: "customfield_10020", Name: "Asset", Required: true, SchemaType: "array", SchemaCustom: "com.atlassian.assets"},
			},
		},
	}})
	next := updated.(Model)

	if len(next.diagnosticsEvents) == 0 {
		t.Fatal("expected diagnostics event")
	}
	got := next.diagnosticsEvents[len(next.diagnosticsEvents)-1].Detail
	for _, want := range []string{"fields=4", "supported=1", "required_unsupported=1", "priority", "customfield_10020"} {
		if !strings.Contains(got, want) {
			t.Fatalf("diagnostic detail = %q, missing %q", got, want)
		}
	}
}

func TestDiagnosticsEventsAreBounded(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()

	for i := 0; i < maxDiagnosticsEvents+5; i++ {
		model.recordDiagnosticEvent(diagnosticKindWorker, "search_issues", "submit", fmt.Sprintf("request %d", i))
	}

	if len(model.diagnosticsEvents) != maxDiagnosticsEvents {
		t.Fatalf("diagnostics events = %d, want %d", len(model.diagnosticsEvents), maxDiagnosticsEvents)
	}
	if !strings.Contains(model.diagnosticsEvents[0].Detail, "request 5") {
		t.Fatalf("oldest retained event = %#v", model.diagnosticsEvents[0])
	}
}

func TestDiagnosticsRecordsDetailCacheDecisions(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: jira.Issue{Key: "ABC-1"}, Description: "Cached detail"},
	}
	model.comments = map[string][]jira.Comment{"ABC-1": {}}
	model.markIssueDetailFresh("ABC-1")

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	fresh := updated.(Model)

	if len(fresh.diagnosticsEvents) == 0 || fresh.diagnosticsEvents[len(fresh.diagnosticsEvents)-1].Status != "hit" {
		t.Fatalf("fresh cache events = %#v", fresh.diagnosticsEvents)
	}

	model.detailFreshnessCache.Delete("ABC-1")
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	stale := updated.(Model)

	if len(stale.diagnosticsEvents) == 0 || stale.diagnosticsEvents[len(stale.diagnosticsEvents)-1].Status != "stale" {
		t.Fatalf("stale cache events = %#v", stale.diagnosticsEvents)
	}

	delete(model.details, "ABC-1")
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	miss := updated.(Model)

	if len(miss.diagnosticsEvents) == 0 || miss.diagnosticsEvents[len(miss.diagnosticsEvents)-1].Status != "miss" {
		t.Fatalf("miss cache events = %#v", miss.diagnosticsEvents)
	}
}

func TestDiagnosticsOverlayShowsSummaryAndActivityBars(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.diagnosticsOpen = true
	model.width = 100
	model.height = 30
	model.diagnosticsEvents = []diagnosticEvent{
		{At: time.Now(), Kind: diagnosticKindWorker, Label: string(worker.KindSearchIssues), Status: "submit", Detail: "#1"},
		{At: time.Now(), Kind: diagnosticKindWorker, Label: string(worker.KindSearchIssues), Status: "ok", Detail: "#1"},
		{At: time.Now(), Kind: diagnosticKindWorker, Label: string(worker.KindGetIssue), Status: "submit", Detail: "#2 ABC-1"},
		{At: time.Now(), Kind: diagnosticKindCache, Label: "issue_detail", Status: "miss", Detail: "ABC-1"},
		{At: time.Now(), Kind: diagnosticKindCache, Label: "issue_detail", Status: "hit", Detail: "ABC-1"},
		{At: time.Now(), Kind: diagnosticKindWorker, Label: string(worker.KindGetComments), Status: "error", Detail: "#3 ABC-1 failed"},
	}

	view := model.render()

	for _, want := range []string{
		"Workers 4",
		"Cache 2",
		"Errors 1",
		"Active 1",
		"Activity",
		"worker [",
		"cache  [",
		"Last get_comments error",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestDiagnosticsOverlayRowsIncludeOperationLabels(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.diagnosticsOpen = true
	model.width = 100
	model.height = 30
	model.diagnosticsEvents = []diagnosticEvent{
		{At: time.Now(), Kind: diagnosticKindWorker, Label: string(worker.KindGetIssue), Status: "submit", Detail: "#7 ABC-1"},
		{At: time.Now(), Kind: diagnosticKindCache, Label: "issue_detail", Status: "stale", Detail: "ABC-1"},
	}

	view := model.render()

	for _, want := range []string{"get_issue #7 ABC-1", "issue_detail ABC-1"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestDiagnosticsShowsStartupClaudeStatus(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeStatus(ClaudeStatus{
			Enabled:   true,
			Available: true,
			Command:   "/opt/homebrew/bin/claude",
			Version:   "Claude Code 2.0.0",
			Message:   "Claude ready",
		}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.diagnosticsOpen = true
	model.width = 120
	model.height = 30

	view := model.render()

	for _, want := range []string{"claude", "ready", "/opt/homebrew/bin/claude", "Claude Code 2.0.0"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestCreateShortcutLoadsCreateIssueTypes(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.width = 100
	model.height = 30

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "n", Code: 'n'}))
	next := updated.(Model)

	if !next.createOpen {
		t.Fatal("expected create modal to open")
	}
	if next.createProjectKey != "ABC" {
		t.Fatalf("createProjectKey = %q", next.createProjectKey)
	}
	if !next.createIssueTypesLoading {
		t.Fatal("expected issue type metadata loading")
	}
	if cmd == nil {
		t.Fatal("expected metadata request command")
	}
	view := next.render()
	for _, want := range []string{"Create Ticket", "Loading issue types"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestCreateShortcutOpensFromDetailWithoutMovingFocus(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Assignee: "Jane Doe", Priority: "Medium", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}

	if target := model.focusedDetailTargetID(); target != "summary" {
		t.Fatalf("initial focused target = %q", target)
	}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "n", Code: 'n'}))
	next := updated.(Model)

	if !next.createOpen {
		t.Fatal("expected create modal to open")
	}
	if target := next.focusedDetailTargetID(); target != "summary" {
		t.Fatalf("focused target moved to %q", target)
	}
	if cmd == nil {
		t.Fatal("expected metadata request command")
	}
}

func TestCreateEmptyIssueTypesOnlyShowsCancelFooter(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.width = 100
	model.height = 30

	view := model.render()

	for _, want := range []string{"Jira returned 0 creatable issue types for DEVOPS.", "ctrl+d diagnostics", "esc cancel"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	for _, hidden := range []string{"j/k type", "tab field", "ctrl+s submit"} {
		if strings.Contains(view, hidden) {
			t.Fatalf("inactive create command %q should be hidden in %q", hidden, view)
		}
	}
}

func TestCreateEmptyIssueTypesCanOpenDiagnostics(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.width = 100
	model.height = 30
	model.diagnosticsEvents = []diagnosticEvent{
		{At: time.Now(), Kind: diagnosticKindWorker, Label: string(worker.KindGetCreateIssueTypes), Status: "ok", Detail: "#77 DEVOPS types=0"},
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+d"}))
	next := updated.(Model)

	if !next.diagnosticsOpen {
		t.Fatal("expected diagnostics to open from create modal")
	}
	view := next.render()
	if !strings.Contains(view, "get_create_issue_types #77 DEVOPS types=0") {
		t.Fatalf("missing create metadata diagnostics in %q", view)
	}
}

func TestCreateIssueTypePickerLoadsFieldsForSelection(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.createIssueTypes = []jira.CreateIssueType{
		{ID: "10001", Name: "Task"},
		{ID: "10002", Name: "Bug"},
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "down", Code: tea.KeyDown}))
	next := updated.(Model)
	if next.selectedCreateIssueType != 1 {
		t.Fatalf("selectedCreateIssueType = %d", next.selectedCreateIssueType)
	}

	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)
	if next.createIssueType.ID != "10002" {
		t.Fatalf("createIssueType = %#v", next.createIssueType)
	}
	if !next.createFieldsLoading {
		t.Fatal("expected create fields loading")
	}
	if cmd == nil {
		t.Fatal("expected field metadata request command")
	}
}

func TestCreateDraftShortcutNotAvailableInIssueTypeSelection(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.createIssueTypes = []jira.CreateIssueType{
		{ID: "10001", Name: "Task"},
		{ID: "10002", Name: "Bug"},
	}
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "g", Code: 'g'}))
	next := updated.(Model)
	if next.detailNotice != "Select an issue type before generating a ticket draft." {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestCreateIssueTypeSelectionShowsAIGeneratedTab(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, DraftTicket: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.width = 120
	model.height = 40
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.createIssueTypes = []jira.CreateIssueType{
		{ID: "10001", Name: "Task"},
		{ID: "10002", Name: "Bug"},
	}

	view := model.render()
	for _, want := range []string{"Manual", "AI Generated", "ISSUE TYPE", "tab mode"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestCreateIssueModeTabsDoNotTruncateStyledLabels(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, DraftTicket: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.createAIGeneratedMode = true

	tabs := model.renderCreateModeTabs(64)

	for _, want := range []string{"Manual", "AI Generated"} {
		if !strings.Contains(tabs, want) {
			t.Fatalf("missing %q in %q", want, tabs)
		}
	}
	if strings.Contains(tabs, "...") {
		t.Fatalf("tabs should not byte-truncate styled labels: %q", tabs)
	}
	if lipgloss.Width(tabs) < len("Manual AI Generated") {
		t.Fatalf("visible tab width too small: width=%d tabs=%q", lipgloss.Width(tabs), tabs)
	}
	if strings.Contains(tabs, "\x1b[48;") {
		t.Fatalf("create mode tabs should avoid filled backgrounds: %q", tabs)
	}
}

func TestCreateDraftPromptCanRunBeforeIssueTypeSelection(t *testing.T) {
	runner := &fakeClaudeRunner{
		result: claude.Result{Text: strings.Join([]string{
			"Issue Type: Initiative",
			"Summary: Stand up EKS clusters in staging",
			"Description: Create a staging EKS cluster rollout plan with platform controllers.",
		}, "\n")},
	}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, DraftTicket: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
		WithClaudeRunner(runner),
	)
	defer model.workers.Stop()
	model.loading = false
	model.width = 120
	model.height = 40
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.createIssueTypes = []jira.CreateIssueType{
		{ID: "10001", Name: "Initiative"},
		{ID: "10002", Name: "Story"},
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next := updated.(Model)
	if !next.createAIGeneratedMode {
		t.Fatal("expected tab to switch to AI generated mode")
	}
	next.createAIPrompt = "I want to create an epic about building EKS clusters in staging."
	next.createAIPromptEditor = newCreateAIPromptEditor(next.createAIPrompt)
	next.createAIPromptEditorReady = true

	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected create AI prompt command before issue type selection")
	}
	resultMsg := <-runClaudePlanCommandAsyncForTest(cmd)
	if !strings.Contains(runner.request.Prompt, "Issue Type: Not selected yet") {
		t.Fatalf("prompt should allow missing issue type, got %q", runner.request.Prompt)
	}
	for _, want := range []string{"Available Jira Issue Types:", "- Initiative", "- Story"} {
		if !strings.Contains(runner.request.Prompt, want) {
			t.Fatalf("prompt missing Jira issue type %q in %q", want, runner.request.Prompt)
		}
	}
	if strings.Contains(runner.request.Prompt, "- Task") {
		t.Fatalf("prompt should not guess unsupported issue types: %q", runner.request.Prompt)
	}

	updated, cmd = next.Update(resultMsg)
	next = updated.(Model)
	if next.createIssueType.Name != "Initiative" {
		t.Fatalf("issue type should apply Claude's Jira-supported recommendation, got %#v", next.createIssueType)
	}
	if !next.createFieldsLoading {
		t.Fatal("expected recommended issue type to start loading create fields")
	}
	if cmd == nil {
		t.Fatal("expected create fields command after AI issue type recommendation")
	}
	if next.createAIGeneratedMode {
		t.Fatal("expected generated draft to return to manual mode")
	}
	if next.createSummaryDraft != "Stand up EKS clusters in staging" {
		t.Fatalf("createSummaryDraft = %q", next.createSummaryDraft)
	}
	if !strings.Contains(next.detailNotice, "selected Initiative") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestCreateDraftPromptIncludesAvailableComponents(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, DraftTicket: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Story"}
	model.createFields = []jira.CreateField{
		{ID: "components", Name: "Components", SchemaSystem: "components", AllowedValues: []jira.FieldOption{
			{ID: "101", Name: "csp-adapter"},
			{ID: "102", Name: "csp-gateway"},
		}},
	}

	prompt := model.buildCreateIssueDraftPrompt("Audit DNS domain usage.")

	for _, want := range []string{"Available Components:", "- csp-adapter", "- csp-gateway", "Components: <one of the Available Components"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q in:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "- made-up-component") {
		t.Fatalf("prompt should only include Jira-returned components:\n%s", prompt)
	}
}

func TestCreateDraftComponentRecommendationSelectsMatchingJiraComponent(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Story"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
		{ID: "components", Name: "Components", SchemaSystem: "components", AllowedValues: []jira.FieldOption{
			{ID: "101", Name: "csp-adapter"},
			{ID: "102", Name: "csp-gateway"},
		}},
	}
	model.beginCreateForm()
	model.activeCreateAIPromptReqID = 8
	model.createAIPromptLoading = true

	updated, _ := model.Update(createAIPromptResultMsg{
		id: 8,
		text: strings.Join([]string{
			"Issue Type: Story",
			"Summary: Audit DNS domains",
			"Description: Make Terraform modules domain agnostic.",
			"",
			"Components",
			"csp-gateway",
		}, "\n"),
	})
	next := updated.(Model)

	key := createFieldValueKey(model.createFields[2])
	if next.createDynamicSelections[key] != 1 {
		t.Fatalf("component selection = %d", next.createDynamicSelections[key])
	}
}

func TestCreateDraftUnknownComponentDoesNotSelectRandomComponent(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Story"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
		{ID: "components", Name: "Components", SchemaSystem: "components", AllowedValues: []jira.FieldOption{
			{ID: "101", Name: "csp-adapter"},
			{ID: "102", Name: "csp-gateway"},
		}},
	}
	model.beginCreateForm()
	model.activeCreateAIPromptReqID = 9
	model.createAIPromptLoading = true

	updated, _ := model.Update(createAIPromptResultMsg{
		id: 9,
		text: strings.Join([]string{
			"Issue Type: Story",
			"Summary: Audit DNS domains",
			"Description: Make Terraform modules domain agnostic.",
			"",
			"Components",
			"unknown-service",
		}, "\n"),
	})
	next := updated.(Model)

	key := createFieldValueKey(model.createFields[2])
	if next.createDynamicSelections[key] != -1 {
		t.Fatalf("component selection = %d; should remain unselected", next.createDynamicSelections[key])
	}
}

func TestCreateDraftPromptLeavesTypeSelectionWhenRecommendationUnsupported(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.createIssueTypes = []jira.CreateIssueType{
		{ID: "10001", Name: "Initiative"},
		{ID: "10002", Name: "Story"},
	}
	model.activeCreateAIPromptReqID = 7
	model.createAIPromptLoading = true

	updated, cmd := model.Update(createAIPromptResultMsg{
		id: 7,
		text: strings.Join([]string{
			"Issue Type: Task",
			"Summary: Stand up EKS clusters in staging",
			"Description: Create a staging EKS cluster rollout plan with platform controllers.",
		}, "\n"),
	})
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("unsupported issue type recommendation should not submit create fields")
	}
	if next.createIssueType.ID != "" {
		t.Fatalf("issue type should remain unselected, got %#v", next.createIssueType)
	}
	if next.createSummaryDraft != "Stand up EKS clusters in staging" {
		t.Fatalf("createSummaryDraft = %q", next.createSummaryDraft)
	}
	if !strings.Contains(next.detailNotice, "Select an issue type") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestCreateComponentPickerFiltersAndSelectsWithTypeahead(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.width = 120
	model.height = 45
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Story"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
		{ID: "components", Name: "Components", SchemaSystem: "components", AllowedValues: []jira.FieldOption{
			{ID: "101", Name: "csp-adapter"},
			{ID: "102", Name: "csp-report"},
			{ID: "103", Name: "csp-gateway"},
		}},
	}
	model.beginCreateForm()
	model.createFieldFocus = model.createDynamicFieldFocusIndex(0)

	for _, key := range []string{"g", "a", "t"} {
		updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: key, Code: []rune(key)[0]}))
		model = updated.(Model)
	}

	fieldKey := createFieldValueKey(model.createFields[2])
	if model.createDynamicFilters[fieldKey] != "gat" {
		t.Fatalf("filter = %q", model.createDynamicFilters[fieldKey])
	}
	view := model.render()
	if !strings.Contains(view, "Filter: gat") || !strings.Contains(view, "csp-gateway") {
		t.Fatalf("expected filtered gateway view:\n%s", view)
	}
	if strings.Contains(view, "csp-adapter") || strings.Contains(view, "csp-report") {
		t.Fatalf("filter should hide non-matches:\n%s", view)
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	model = updated.(Model)
	if model.createDynamicSelections[fieldKey] != 2 {
		t.Fatalf("component selection = %d", model.createDynamicSelections[fieldKey])
	}
	if model.createDynamicFilters[fieldKey] != "" {
		t.Fatalf("filter should clear after enter, got %q", model.createDynamicFilters[fieldKey])
	}
}

func TestCreateComponentPickerBackspaceAndEscEditFilter(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Story"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
		{ID: "components", Name: "Components", SchemaSystem: "components", AllowedValues: []jira.FieldOption{
			{ID: "101", Name: "csp-adapter"},
			{ID: "102", Name: "csp-gateway"},
		}},
	}
	model.beginCreateForm()
	model.createFieldFocus = model.createDynamicFieldFocusIndex(0)
	fieldKey := createFieldValueKey(model.createFields[2])

	for _, key := range []string{"g", "a"} {
		updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: key, Code: []rune(key)[0]}))
		model = updated.(Model)
	}
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "backspace", Code: tea.KeyBackspace}))
	model = updated.(Model)
	if model.createDynamicFilters[fieldKey] != "g" {
		t.Fatalf("filter after backspace = %q", model.createDynamicFilters[fieldKey])
	}

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "esc", Code: tea.KeyEsc}))
	model = updated.(Model)
	if model.createDynamicFilters[fieldKey] != "" {
		t.Fatalf("filter after esc = %q", model.createDynamicFilters[fieldKey])
	}
	if !model.createOpen {
		t.Fatal("esc should clear filter before closing create modal")
	}
}

func TestCreateDraftPromptCanOpenFromCreateForm(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, DraftTicket: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.width = 120
	model.height = 35
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Task"}
	model.createFields = nil
	model.beginCreateForm()

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next := updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)

	if !next.createAIPromptOpen {
		t.Fatal("expected create AI prompt editor to open")
	}
	if next.createAIPromptErr != nil {
		t.Fatalf("unexpected prompt error: %v", next.createAIPromptErr)
	}
}

func TestCreateDraftPromptCanOpenFromCreateFormWithEnter(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, DraftTicket: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.width = 120
	model.height = 35
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Task"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", Required: true},
		{ID: "description", Name: "Description"},
	}
	model.beginCreateForm()
	model.createSummaryDraft = "internal load balancer"
	model.createDescriptionDraft = "need ecs service-to-service traffic over private alb"
	model.createSummaryEditor = newSummaryEditor(model.createSummaryDraft)
	model.createDescriptionEditor = newCommentEditor(model.createDescriptionDraft)

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next := updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)

	if !next.createAIPromptOpen {
		t.Fatal("expected create AI prompt editor to open via tab selection")
	}
}

func TestCreateDraftPromptVisibleButUnavailableShowsNotice(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, DraftTicket: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: false, Command: "claude", Message: "Claude version check failed", Err: errors.New("some error")}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.width = 120
	model.height = 35
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Task"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", Required: true},
		{ID: "description", Name: "Description"},
	}
	model.beginCreateForm()

	view := model.render()
	if !strings.Contains(view, "enter generate") {
		t.Fatalf("expected create draft key in footer, got %q", view)
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next := updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)
	if next.createAIPromptOpen {
		t.Fatal("create draft prompt should not open while Claude is unavailable")
	}
	if next.detailNotice != "Claude ticket draft generation is currently unavailable." {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestCreateDraftPromptSubmissionParsesSummaryAndDescription(t *testing.T) {
	runner := &fakeClaudeRunner{
		result: claude.Result{
			Text: strings.Join([]string{
				"Summary: Add internal load balancer support",
				"Description: Create a private ALB for ECS internal traffic.",
				"",
				"Implementation notes",
				"- add module wiring",
			}, "\n"),
		},
	}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, DraftTicket: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
		WithClaudeRunner(runner),
	)
	defer model.workers.Stop()
	model.loading = false
	model.width = 120
	model.height = 35
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Task"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", Required: true},
		{ID: "description", Name: "Description"},
	}
	model.beginCreateForm()
	model.createSummaryDraft = "internal load balancer"
	model.createDescriptionDraft = "need ecs service-to-service traffic over private alb"
	model.createSummaryEditor = newSummaryEditor(model.createSummaryDraft)
	model.createDescriptionEditor = newCommentEditor(model.createDescriptionDraft)

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next := updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)

	if !next.createAIPromptOpen {
		t.Fatal("expected create AI prompt editor to open")
	}
	next.createAIPrompt = "Draft a ticket for internal-only load balancers."
	next.createAIPromptEditor = newCreateAIPromptEditor(next.createAIPrompt)
	next.createAIPromptEditorReady = true
	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected create AI prompt command")
	}
	resultMsg := <-runClaudePlanCommandAsyncForTest(cmd)
	result, ok := resultMsg.(createAIPromptResultMsg)
	if !ok {
		t.Fatalf("expected createAIPromptResultMsg, got %T", resultMsg)
	}
	if result.err != nil {
		t.Fatalf("expected no error, got %v", result.err)
	}
	for _, want := range []string{"Project: ABC", "Issue Type: Task", "User request", "Draft a ticket for internal-only load balancers."} {
		if !strings.Contains(runner.request.Prompt, want) {
			t.Fatalf("prompt missing %q in %q", want, runner.request.Prompt)
		}
	}
	for _, want := range []string{"Current draft", "internal load balancer", "need ecs service-to-service traffic over private alb"} {
		if !strings.Contains(runner.request.Prompt, want) {
			t.Fatalf("prompt missing current draft %q in %q", want, runner.request.Prompt)
		}
	}

	updated, _ = next.Update(resultMsg)
	next = updated.(Model)
	if next.createAIPromptOpen {
		t.Fatal("expected prompt modal to close after applying generated draft")
	}
	if next.createSummaryDraft != "Add internal load balancer support" {
		t.Fatalf("createSummaryDraft = %q", next.createSummaryDraft)
	}
	if next.createDescriptionDraft != "Create a private ALB for ECS internal traffic." {
		t.Fatalf("createDescriptionDraft = %q", next.createDescriptionDraft)
	}
	if !strings.Contains(next.detailNotice, "Applied Claude ticket draft") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestCreateDraftPromptKeepsOpenOnMissingSummary(t *testing.T) {
	runner := &fakeClaudeRunner{
		result: claude.Result{
			Text: strings.Join([]string{
				"Description: Create a private ALB for ECS internal traffic.",
				"",
				"Implementation notes",
				"- add module wiring",
			}, "\n"),
		},
	}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, DraftTicket: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
		WithClaudeRunner(runner),
	)
	defer model.workers.Stop()
	model.loading = false
	model.width = 120
	model.height = 35
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Task"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", Required: true},
		{ID: "description", Name: "Description"},
	}
	model.beginCreateForm()

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next := updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)

	if !next.createAIPromptOpen {
		t.Fatal("expected create AI prompt editor to open")
	}
	next.createAIPrompt = "Draft a ticket for internal-only load balancers."
	next.createAIPromptEditor = newCreateAIPromptEditor(next.createAIPrompt)
	next.createAIPromptEditorReady = true
	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected create AI prompt command")
	}
	resultMsg := <-runClaudePlanCommandAsyncForTest(cmd)
	result, ok := resultMsg.(createAIPromptResultMsg)
	if !ok {
		t.Fatalf("expected createAIPromptResultMsg, got %T", resultMsg)
	}
	if result.err != nil {
		t.Fatalf("expected no error, got %v", result.err)
	}

	updated, _ = next.Update(resultMsg)
	next = updated.(Model)
	if !next.createAIPromptOpen {
		t.Fatal("expected create AI prompt to stay open on parse failure")
	}
	if next.createAIPromptErr == nil || !strings.Contains(next.createAIPromptErr.Error(), "missing a summary") {
		t.Fatalf("expected summary parse error, got %v", next.createAIPromptErr)
	}
	if next.createSummaryDraft != "" {
		t.Fatalf("createSummaryDraft = %q", next.createSummaryDraft)
	}
}

func TestParseCreateIssueDraftExtractsSummaryAndDescription(t *testing.T) {
	summary, description := parseCreateIssueDraft(strings.Join([]string{
		"Review",
		"Some quick thoughts.",
		"Summary: Add internal load balancer support",
		"Description:",
		"Create a private ALB for ECS internal traffic.",
		"",
		"Acceptance Criteria",
		"- should expose route53 record",
	}, "\n"))
	if summary != "Add internal load balancer support" {
		t.Fatalf("summary = %q", summary)
	}
	if description != "Create a private ALB for ECS internal traffic." {
		t.Fatalf("description = %q", description)
	}
}

func TestParseCreateIssueDraftFieldsExtractsNamedSections(t *testing.T) {
	fields := parseCreateIssueDraftFields(strings.Join([]string{
		"Summary: Install Kubernetes",
		"Description:",
		"Provision clusters.",
		"",
		"Components",
		"acheron-migrations",
		"active-users-report",
		"",
		"Investment Category",
		"Growth",
		"",
		"Priority",
		"P0 - Critical",
	}, "\n"))

	if fields["components"] != "acheron-migrations\nactive-users-report" {
		t.Fatalf("components = %q", fields["components"])
	}
	if fields["investmentcategory"] != "Growth" {
		t.Fatalf("investment category = %q fields=%#v", fields["investmentcategory"], fields)
	}
	if fields["priority"] != "P0 - Critical" {
		t.Fatalf("priority = %q", fields["priority"])
	}
}

func TestParseCreateIssueOpenQuestionsExtractsActionableQuestions(t *testing.T) {
	questions := parseCreateIssueOpenQuestions(strings.Join([]string{
		"Summary: Build Kubernetes platform",
		"Description:",
		"Provision clusters.",
		"",
		"Open Questions",
		"- Which AWS accounts are in scope?",
		"- Is this for staging only or production too?",
		"",
		"Priority",
		"P2 - Medium",
	}, "\n"))

	if len(questions) != 2 {
		t.Fatalf("questions = %#v", questions)
	}
	if questions[0].Question != "Which AWS accounts are in scope?" {
		t.Fatalf("question[0] = %#v", questions[0])
	}
	if questions[1].Question != "Is this for staging only or production too?" {
		t.Fatalf("question[1] = %#v", questions[1])
	}
}

func TestCreateIssueIgnoresMetadataProjectAndIssueTypeRequiredFields(t *testing.T) {
	fields := []jira.CreateField{
		{ID: "project", Name: "Project", Required: true, SchemaSystem: "project", SchemaType: "project"},
		{ID: "issuetype", Name: "Issue Type", Required: true, SchemaSystem: "issuetype", SchemaType: "issuetype"},
		{ID: "summary", Name: "Summary", Required: true, SchemaSystem: "summary", SchemaType: "string"},
	}

	if unsupported := unsupportedRequiredCreateFields(fields); len(unsupported) != 0 {
		t.Fatalf("unsupported = %#v", unsupported)
	}
	if supported := supportedCreateFields(fields); len(supported) != 0 {
		t.Fatalf("supported = %#v", supported)
	}
}

func TestCreateAIDraftKeepsOpenQuestionsOutOfDescription(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Epic"}
	model.activeCreateAIPromptReqID = 12
	model.createAIPromptLoading = true

	updated, _ := model.Update(createAIPromptResultMsg{
		id: 12,
		text: strings.Join([]string{
			"Summary: Build Kubernetes platform",
			"Description:",
			"Provision clusters.",
			"",
			"Open Questions",
			"- Which AWS accounts are in scope?",
			"- Is this staging only?",
		}, "\n"),
	})
	next := updated.(Model)

	if strings.Contains(next.createDescriptionDraft, "Open Questions") {
		t.Fatalf("description should not include open questions: %q", next.createDescriptionDraft)
	}
	if len(next.createAIQuestions) != 2 {
		t.Fatalf("questions = %#v", next.createAIQuestions)
	}
}

func TestCreateFormRendersOpenQuestionsPanel(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.width = 140
	model.height = 45
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Epic"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
	}
	model.createAIQuestions = []createAIQuestion{
		{Question: "Which AWS accounts are in scope?"},
		{Question: "Is this staging only?", Answer: "Staging first."},
	}
	model.beginCreateForm()
	model.createFieldFocus = model.createQuestionsFieldIndex()

	view := model.render()

	for _, want := range []string{"Open Questions", "> Which AWS accounts are in scope?", "answered Is this staging only?", "enter answer"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in:\n%s", want, view)
		}
	}
}

func TestCreateQuestionAnswerStoresLocalFeedback(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.width = 140
	model.height = 45
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Epic"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
	}
	model.createAIQuestions = []createAIQuestion{{Question: "Which AWS accounts are in scope?"}}
	model.beginCreateForm()
	model.createFieldFocus = model.createQuestionsFieldIndex()

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if !next.createAIQuestionAnswering {
		t.Fatal("expected answer editor to open")
	}
	updated, _ = next.Update(tea.PasteMsg{Content: "All development AWS accounts."})
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)

	if next.createAIQuestionAnswering {
		t.Fatal("expected answer editor to close")
	}
	if next.createAIQuestions[0].Answer != "All development AWS accounts." {
		t.Fatalf("answer = %q", next.createAIQuestions[0].Answer)
	}
}

func TestCreateQuestionAnswerEnterSavesAndMovesToNextQuestion(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.width = 140
	model.height = 45
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Epic"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
	}
	model.createAIQuestions = []createAIQuestion{
		{Question: "Which Kubernetes flavor should we use?"},
		{Question: "Which AWS accounts are in scope?"},
	}
	model.beginCreateForm()
	model.createFieldFocus = model.createQuestionsFieldIndex()

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	updated, _ = next.Update(tea.PasteMsg{Content: "EKS"})
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)

	if next.createAIQuestions[0].Answer != "EKS" {
		t.Fatalf("first answer = %q", next.createAIQuestions[0].Answer)
	}
	if next.selectedCreateAIQuestion != 1 {
		t.Fatalf("selected question = %d, want 1", next.selectedCreateAIQuestion)
	}
	if !next.createAIQuestionAnswering {
		t.Fatal("expected answer editor to stay open on the next question")
	}
	if value := next.createAIQuestionEditor.Value(); value != "" {
		t.Fatalf("next answer editor should start empty, got %q", value)
	}
}

func TestCreateQuestionsPanelShowsAndRunsRefineWithAnswers(t *testing.T) {
	runner := &fakeClaudeRunner{
		result: claude.Result{Text: strings.Join([]string{
			"Summary: Build Kubernetes platform",
			"Description: Refined draft.",
		}, "\n")},
	}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = DEVOPS",
		WithClaudeConfig(ClaudeConfig{Enabled: true, DraftTicket: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
		WithClaudeRunner(runner),
	)
	defer model.workers.Stop()
	model.loading = false
	model.width = 140
	model.height = 45
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Epic"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
	}
	model.createSummaryDraft = "Build Kubernetes platform"
	model.createDescriptionDraft = "Provision clusters."
	model.createAIQuestions = []createAIQuestion{
		{Question: "Which Kubernetes flavor should we use?", Answer: "EKS"},
		{Question: "Which AWS accounts are in scope?", Answer: "All development accounts."},
	}
	model.beginCreateForm()
	model.createFieldFocus = model.createQuestionsFieldIndex()

	view := model.render()
	if !strings.Contains(view, "ctrl+r refine with answers") {
		t.Fatalf("questions panel should expose refine action:\n%s", view)
	}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+r"}))
	next := updated.(Model)
	if cmd == nil {
		t.Fatal("expected ctrl+r to submit Claude refinement")
	}
	if !next.createAIPromptLoading {
		t.Fatal("expected create AI prompt loading after refine")
	}
	resultMsg := <-runClaudePlanCommandAsyncForTest(cmd)
	if _, ok := resultMsg.(createAIPromptResultMsg); !ok {
		t.Fatalf("expected createAIPromptResultMsg, got %T", resultMsg)
	}
	for _, want := range []string{"User answers to Open Questions", "Q: Which Kubernetes flavor should we use?", "A: EKS", "Q: Which AWS accounts are in scope?", "A: All development accounts."} {
		if !strings.Contains(runner.request.Prompt, want) {
			t.Fatalf("prompt missing %q in:\n%s", want, runner.request.Prompt)
		}
	}
}

func TestCreateDraftPromptIncludesOpenQuestionAnswers(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Epic"}
	model.createSummaryDraft = "Build Kubernetes platform"
	model.createDescriptionDraft = "Provision clusters."
	model.createAIQuestions = []createAIQuestion{
		{Question: "Which AWS accounts are in scope?", Answer: "All development AWS accounts."},
		{Question: "Is this staging only?"},
	}

	prompt := model.buildCreateIssueDraftPrompt("Refine with my answers.")

	for _, want := range []string{"User answers to Open Questions", "Q: Which AWS accounts are in scope?", "A: All development AWS accounts.", "Still unanswered Open Questions", "- Is this staging only?"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q in:\n%s", want, prompt)
		}
	}
}

func TestCreateAIDraftAppliesSupportedFieldsAfterMetadataLoads(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.activeCreateAIPromptReqID = 12
	model.createAIPromptLoading = true

	updated, _ := model.Update(createAIPromptResultMsg{
		id: 12,
		text: strings.Join([]string{
			"Summary: Install Kubernetes across all development AWS accounts",
			"Description:",
			"Provision and standardize Kubernetes across development workloads.",
			"",
			"Components",
			"acheron-migrations",
			"active-users-report",
			"",
			"Investment Category",
			"Growth",
			"",
			"Priority",
			"P0 - Critical",
		}, "\n"),
	})
	next := updated.(Model)

	next.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Initiative"}
	next.activeCreateFieldsReqID = 22
	updated, _ = next.Update(workerResultMsg{result: worker.Result{
		ID:   22,
		Kind: worker.KindGetCreateFields,
		GetCreateFields: &worker.GetCreateFieldsResult{
			ProjectKey:  "DEVOPS",
			IssueTypeID: "10001",
			Fields: []jira.CreateField{
				{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
				{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
				{ID: "components", Name: "Components", SchemaSystem: "components", SchemaType: "array", AllowedValues: []jira.FieldOption{
					{ID: "c1", Name: "acheron-migrations"},
					{ID: "c2", Name: "active-users-report"},
				}},
				{ID: "customfield_10010", Name: "Investment Category", SchemaType: "option", AllowedValues: []jira.FieldOption{
					{ID: "i1", Name: "KTLO"},
					{ID: "i2", Name: "Growth"},
				}},
				{ID: "priority", Name: "Priority", SchemaSystem: "priority", SchemaType: "priority", AllowedValues: []jira.FieldOption{
					{ID: "p0", Name: "P0 - Critical"},
					{ID: "p2", Name: "P2 - Medium"},
				}},
			},
		},
	}})
	next = updated.(Model)

	if next.createSummaryDraft != "Install Kubernetes across all development AWS accounts" {
		t.Fatalf("summary = %q", next.createSummaryDraft)
	}
	if got := next.createDynamicSelections["components"]; got != 0 {
		t.Fatalf("components selection = %d", got)
	}
	if got := next.createDynamicSelections["customfield_10010"]; got != 1 {
		t.Fatalf("investment category selection = %d", got)
	}
	if got := next.createDynamicSelections["priority"]; got != 0 {
		t.Fatalf("priority selection = %d", got)
	}
}

func TestCreateFormScrollsToFocusedLaterFields(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.width = 100
	model.height = 24
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Initiative"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
		{ID: "components", Name: "Components", SchemaSystem: "components", SchemaType: "array", AllowedValues: []jira.FieldOption{{ID: "c1", Name: "Component One"}}},
		{ID: "customfield_1", Name: "Release Instructions", SchemaType: "string"},
		{ID: "customfield_2", Name: "Investment Category", SchemaType: "option", AllowedValues: []jira.FieldOption{{ID: "g", Name: "Growth"}}},
		{ID: "labels", Name: "Labels", SchemaSystem: "labels", SchemaType: "string"},
		{ID: "priority", Name: "Priority", SchemaSystem: "priority", SchemaType: "priority", AllowedValues: []jira.FieldOption{{ID: "p0", Name: "P0 - Critical"}}},
	}
	model.beginCreateForm()
	model.createFieldFocus = model.createDynamicFieldFocusIndex(4)

	view := model.render()

	if !strings.Contains(view, "Priority") || !strings.Contains(view, "P0 - Critical") {
		t.Fatalf("focused later field should be visible:\n%s", view)
	}
	if strings.Contains(view, "Components") && strings.Contains(view, "Release Instructions") && strings.Contains(view, "Investment Category") {
		t.Fatalf("create modal did not window long body:\n%s", view)
	}
	if !strings.Contains(view, "Create Lines") {
		t.Fatalf("expected scroll indicator in long create form:\n%s", view)
	}
}

func TestCreateFormRendersCompactMetadataRowsWhenNotFocused(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS", WithClaudeConfig(ClaudeConfig{Enabled: true, DraftTicket: true}), WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true}))
	defer model.workers.Stop()
	model.loading = false
	model.width = 120
	model.height = 40
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Epic"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
		{ID: "components", Name: "Components", SchemaSystem: "components", SchemaType: "array", AllowedValues: []jira.FieldOption{
			{ID: "c1", Name: "acheron-migrations"},
			{ID: "c2", Name: "active-users-report"},
			{ID: "c3", Name: "ad-hoc-projects"},
		}},
		{ID: "priority", Name: "Priority", SchemaSystem: "priority", SchemaType: "priority", AllowedValues: []jira.FieldOption{
			{ID: "p0", Name: "P0 - Critical"},
			{ID: "p1", Name: "P1 - High"},
		}},
	}
	model.beginCreateForm()
	model.createDynamicSelections["components"] = 1
	model.createFieldFocus = createSummaryFieldIndex

	view := model.render()

	for _, want := range []string{"Components: active-users-report", "Priority: P0 - Critical", "Generate Draft"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in:\n%s", want, view)
		}
	}
	for _, unwanted := range []string{"ad-hoc-projects", "P1 - High", "Options 1-"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("unfocused metadata should stay compact, found %q in:\n%s", unwanted, view)
		}
	}
	if strings.Index(view, "Generate Draft") > strings.Index(view, "Components") {
		t.Fatalf("Generate Draft should appear before metadata fields:\n%s", view)
	}
}

func TestCreateFormFocusedPickerExpandsOptions(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.width = 120
	model.height = 35
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Epic"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
		{ID: "components", Name: "Components", SchemaSystem: "components", SchemaType: "array", AllowedValues: []jira.FieldOption{
			{ID: "c1", Name: "acheron-migrations"},
			{ID: "c2", Name: "active-users-report"},
		}},
	}
	model.beginCreateForm()
	model.createFieldFocus = model.createDynamicFieldFocusIndex(0)

	view := model.render()

	for _, want := range []string{"Components", "acheron-migrations", "active-users-report"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing focused picker value %q in:\n%s", want, view)
		}
	}
}

func TestCreateFormCanChangeTypeWithoutClearingDraft(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.width = 120
	model.height = 35
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueTypes = []jira.CreateIssueType{
		{ID: "10001", Name: "Epic"},
		{ID: "10002", Name: "Story"},
	}
	model.selectedCreateIssueType = 0
	model.createIssueType = model.createIssueTypes[0]
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
	}
	model.createSummaryDraft = "Deploy ArgoCD to Kubernetes clusters"
	model.createDescriptionDraft = "Enable GitOps based deployments across clusters."
	model.beginCreateForm()
	model.createFieldFocus = createTypeFieldIndex

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if !next.createChangingType {
		t.Fatal("expected type picker to open from focused Type row")
	}
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "down", Code: tea.KeyDown}))
	next = updated.(Model)
	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)

	if next.createIssueType.Name != "Story" {
		t.Fatalf("issue type = %#v", next.createIssueType)
	}
	if next.createSummaryDraft != "Deploy ArgoCD to Kubernetes clusters" {
		t.Fatalf("summary cleared while changing type: %q", next.createSummaryDraft)
	}
	if next.createDescriptionDraft != "Enable GitOps based deployments across clusters." {
		t.Fatalf("description cleared while changing type: %q", next.createDescriptionDraft)
	}
	if !next.createFieldsLoading {
		t.Fatal("expected changing type to load new create fields")
	}
	if cmd == nil {
		t.Fatal("expected create fields command for new issue type")
	}
}

func TestCreateAIPromptLoadingSuppressesPartialAssistantAndDebugDetails(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = DEVOPS",
		WithClaudeConfig(ClaudeConfig{Enabled: true, DraftTicket: true, Timeout: 2 * time.Minute, Command: "/Users/joncha/.local/bin/claude"}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "/Users/joncha/.local/bin/claude"}),
	)
	defer model.workers.Stop()
	model.createAIPromptLoading = true
	model.createAIPromptStartedAt = time.Date(2026, 6, 15, 12, 2, 30, 0, time.Local)
	model.now = func() time.Time { return model.createAIPromptStartedAt.Add(21 * time.Second) }
	model.createAIPromptProgress = []claude.Event{
		{Kind: "output", Text: "Assistant: activity, IAM/access, logging, baseline add-ons. - Could ask follow-up questions."},
	}

	body := strings.Join(model.renderCreateAIPromptBody(90), "\n")

	if !strings.Contains(body, "receiving response") {
		t.Fatalf("loading body should keep a stable receiving status:\n%s", body)
	}
	for _, noisy := range []string{"Assistant:", "activity, IAM/access", "/Users/joncha", "Deadline:", "Started:"} {
		if strings.Contains(body, noisy) {
			t.Fatalf("loading body should hide noisy/debug detail %q:\n%s", noisy, body)
		}
	}
}

func TestCreateDialogUsesResponsiveWidth(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.width = 150
	model.height = 40
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Epic"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
	}
	model.beginCreateForm()

	width := dialogBorderWidth(model.render())

	if width < 104 {
		t.Fatalf("create dialog width = %d, want wider responsive modal", width)
	}
}

func TestCreateSummaryFocusedUsesMultilineEditor(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.width = 140
	model.height = 40
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Epic"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
	}
	model.createSummaryDraft = "Build Kubernetes clusters across all FloQast development AWS accounts"
	model.beginCreateForm()
	model.createFieldFocus = createSummaryFieldIndex

	value := model.renderCreateSummaryValue(100)

	if got := strings.Count(value, "\n") + 1; got < 3 {
		t.Fatalf("focused summary editor should be multiline, rows=%d value:\n%s", got, value)
	}
}

func TestCreateDescriptionFocusedGetsMoreEditorRows(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.width = 140
	model.height = 50
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Epic"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
	}
	model.beginCreateForm()
	model.createFieldFocus = createDescriptionFieldIndex

	value := model.renderCreateDescriptionValue(64)

	if got := strings.Count(value, "\n") + 1; got < 16 {
		t.Fatalf("focused description editor should have more vertical space, rows=%d value:\n%s", got, value)
	}
}

func TestCreateFormSubmitsSummaryAndDescription(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.width = 100
	model.height = 30
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Task"}
	model.createFields = []jira.CreateField{{ID: "summary", Name: "Summary", Required: true}, {ID: "description", Name: "Description"}}
	model.beginCreateForm()

	updated, _ := model.Update(tea.PasteMsg{Content: "New internal load balancer"})
	next := updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	updated, _ = next.Update(tea.PasteMsg{Content: "Prepare services for internal ECS traffic."})
	next = updated.(Model)
	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)

	if !next.createSubmitting {
		t.Fatal("expected create submission")
	}
	if next.createSubmitSummary != "New internal load balancer" {
		t.Fatalf("createSubmitSummary = %q", next.createSubmitSummary)
	}
	if next.createSubmitDescription != "Prepare services for internal ECS traffic." {
		t.Fatalf("createSubmitDescription = %q", next.createSubmitDescription)
	}
	if cmd == nil {
		t.Fatal("expected create issue command")
	}
}

func TestCreateFormRendersAndSubmitsSupportedDynamicFields(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.width = 120
	model.height = 40
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Task"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", Required: true, SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
		{ID: "priority", Name: "Priority", Required: true, SchemaSystem: "priority", SchemaType: "priority", AllowedValues: []jira.FieldOption{
			{ID: "4", Name: "Low"},
			{ID: "3", Name: "Medium"},
		}},
		{ID: "customfield_10010", Name: "Team", Required: true, SchemaType: "string"},
	}
	model.beginCreateForm()

	view := model.render()
	for _, want := range []string{"Priority", "Low", "Team"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}

	updated, _ := model.Update(tea.PasteMsg{Content: "New internal load balancer"})
	next := updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "down", Code: tea.KeyDown}))
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	updated, _ = next.Update(tea.PasteMsg{Content: "Platform"})
	next = updated.(Model)
	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)

	if !next.createSubmitting {
		t.Fatal("expected create submission")
	}
	if len(next.createSubmitFields) != 2 {
		t.Fatalf("createSubmitFields = %#v", next.createSubmitFields)
	}
	if next.createSubmitFields[0].FieldID != "priority" || next.createSubmitFields[0].Option.Name != "Medium" {
		t.Fatalf("priority field = %#v", next.createSubmitFields[0])
	}
	if next.createSubmitFields[1].FieldID != "customfield_10010" || next.createSubmitFields[1].Text != "Platform" {
		t.Fatalf("team field = %#v", next.createSubmitFields[1])
	}
	if cmd == nil {
		t.Fatal("expected create issue command")
	}
}

func TestCreateFormBoundsLongPickerOptions(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.width = 100
	model.height = 30
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Initiative"}
	options := make([]jira.FieldOption, 0, 30)
	for index := 0; index < 30; index++ {
		options = append(options, jira.FieldOption{ID: strconv.Itoa(index + 1), Name: fmt.Sprintf("component-%02d", index+1)})
	}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", Required: true, SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
		{ID: "components", Name: "Components", SchemaSystem: "components", SchemaType: "array", AllowedValues: options},
	}
	model.beginCreateForm()
	model.createFieldFocus = model.createDynamicFieldFocusIndex(0)

	view := model.render()

	if lines := strings.Count(strings.TrimRight(view, "\n"), "\n") + 1; lines > model.height {
		t.Fatalf("create modal rendered %d lines, terminal height is %d:\n%s", lines, model.height, view)
	}
	if strings.Contains(view, "component-30") {
		t.Fatalf("long picker rendered too many options:\n%s", view)
	}
	for _, want := range []string{"Components", "component-01", "component-06", "Options 1-6 of 30"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in:\n%s", want, view)
		}
	}

	for index := 0; index < 17; index++ {
		updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "down", Code: tea.KeyDown}))
		model = updated.(Model)
	}
	view = model.render()
	for _, want := range []string{"component-17", "Options 14-19 of 30"} {
		if !strings.Contains(view, want) {
			t.Fatalf("moving through long picker did not keep %q visible in:\n%s", want, view)
		}
	}
	if strings.Contains(view, "component-01") {
		t.Fatalf("long picker should scroll away from the first option:\n%s", view)
	}
}

func TestCreateIssueSuccessAddsIssueToList(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Existing"}}
	model.createOpen = true
	model.activeCreateIssueReqID = 22
	model.createSubmitting = true
	model.createSubmitSummary = "New issue"

	updated, _ := model.Update(workerResultMsg{result: worker.Result{
		ID:   22,
		Kind: worker.KindCreateIssue,
		CreateIssue: &worker.CreateIssueResult{
			Issue: jira.Issue{Key: "ABC-2", Summary: "New issue"},
		},
	}})
	next := updated.(Model)

	if next.createOpen {
		t.Fatal("expected create modal to close")
	}
	if len(next.issues) == 0 || next.issues[0].Key != "ABC-2" {
		t.Fatalf("issues = %#v", next.issues)
	}
	if next.selected != 0 {
		t.Fatalf("selected = %d", next.selected)
	}
	if !strings.Contains(next.detailNotice, "Created ABC-2") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
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
	for _, unwanted := range []string{"+--------------------------------------+", "| {\"Sid\":\"DenyS3Deletes\"}", "│"} {
		if strings.Contains(rendered, unwanted) {
			t.Fatalf("expected compact code styling without ASCII borders, rendered = %q", rendered)
		}
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
	if !strings.Contains(rendered, "The failure is:\n\n") || strings.Contains(rendered, "\n\n\n") {
		t.Fatalf("expected one separator before code block, rendered = %q", rendered)
	}
	for _, unwanted := range []string{"+--------------------------------------+", "| Error: missing resource", "│"} {
		if strings.Contains(rendered, unwanted) {
			t.Fatalf("expected compact code styling without ASCII borders, rendered = %q", rendered)
		}
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

func TestRenderDescriptionSeparatesHeaderFromRichBody(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()

	rendered := model.renderDescription("Intro paragraph.\n\n- first item\n\n```\nterraform plan\n```", 80)

	lines := strings.Split(rendered, "\n")
	bodyLine := -1
	for index, line := range lines {
		if strings.Contains(line, "Intro paragraph.") {
			bodyLine = index
			break
		}
	}
	if bodyLine < 2 || strings.TrimSpace(lines[bodyLine-1]) != "" {
		t.Fatalf("expected blank line between description header and body, rendered = %q", rendered)
	}
	for _, want := range []string{"- first item", "terraform plan"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("missing %q in %q", want, rendered)
		}
	}
}

func TestDetailStatesRenderConsistentStatusBlocks(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()

	description := model.renderDescriptionState("Loading issue detail...", 80, false)
	comments := model.renderComments("ABC-1", 80)

	for _, rendered := range []string{description, comments} {
		for _, want := range []string{"Status", "──"} {
			if !strings.Contains(rendered, want) {
				t.Fatalf("missing %q in %q", want, rendered)
			}
		}
	}
	if !strings.Contains(description, "Loading issue detail...") {
		t.Fatalf("missing description state message in %q", description)
	}
	if !strings.Contains(comments, "Comments not loaded.") {
		t.Fatalf("missing comments state message in %q", comments)
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
	focusDetailSectionForTest(t, &model, "Comments")

	content := model.fullDetailContent(80)

	for _, want := range []string{"Comments", "Comment 1/1", "Comment Person", "Please check", "main.tf"} {
		if !strings.Contains(content, want) {
			t.Fatalf("missing %q in %q", want, content)
		}
	}

}

func TestCommentBlockLeadsWithAuthorAndSeparatesBody(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	created := time.Date(2026, 6, 13, 10, 15, 0, 0, time.Local)

	rendered := model.renderCommentBlock(2, 4, "Comment Person", formatTime(created), "Please check `main.tf`.", 80)

	authorIndex := strings.Index(rendered, "Comment Person")
	countIndex := strings.Index(rendered, "Comment 2/4")
	if authorIndex < 0 || countIndex < 0 {
		t.Fatalf("expected author and count in %q", rendered)
	}
	if authorIndex > countIndex {
		t.Fatalf("author should lead comment header, rendered = %q", rendered)
	}
	if !strings.Contains(rendered, "2026-06-13 10:15") {
		t.Fatalf("missing created timestamp in %q", rendered)
	}
	lines := strings.Split(rendered, "\n")
	bodyLine := -1
	for index, line := range lines {
		if strings.Contains(line, "Please check") {
			bodyLine = index
			break
		}
	}
	if bodyLine < 2 || strings.TrimSpace(strings.TrimPrefix(lines[bodyLine-1], "│")) != "" {
		t.Fatalf("expected blank line between comment header and body, rendered = %q", rendered)
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
	focusDetailSectionForTest(t, &model, "Comments")

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
	if !strings.Contains(view, "Summary:") {
		t.Fatalf("expected focused summary field in %q", view)
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next := updated.(Model)
	if next.focusedDetailTargetID() != "assignee" {
		t.Fatalf("focused target = %q", next.focusedDetailTargetID())
	}
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	if next.focusedDetailTargetID() != "priority" {
		t.Fatalf("focused target = %q", next.focusedDetailTargetID())
	}
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	assertFocusedDetailSection(t, next, "Description")
	if view := next.render(); !strings.Contains(view, "Description") {
		t.Fatalf("expected focused description section in %q", view)
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
	focusDetailSectionForTest(t, &model, "Hierarchy")

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
	focusDetailSectionForTest(t, &model, "Hierarchy")

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
	focusDetailSectionForTest(t, &model, "Actions")

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
	focusDetailSectionForTest(t, &model, "Comments")
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

func TestStatusSectionEnterLoadsAvailableTransitions(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}
	model.jumpDetailSection("Status")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("expected transition load command")
	}
	if !next.transitionLoading {
		t.Fatal("transitionLoading should be true")
	}
	if next.transitionRequestKey != "ABC-1" {
		t.Fatalf("transitionRequestKey = %q", next.transitionRequestKey)
	}
	if !strings.Contains(next.detailNotice, "Loading status transitions") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestStatusTransitionPickerRendersTransitionsAndSelection(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}
	model.transitions = map[string][]jira.Transition{
		"ABC-1": {
			{ID: "21", Name: "Start Progress", ToStatus: "In Progress"},
			{ID: "31", Name: "Done", ToStatus: "Done"},
		},
	}
	model.transitionFocus = true
	model.selectedTransition = 1
	model.jumpDetailSection("Status")

	view := model.render()

	for _, want := range []string{"Status", "To Do", "Start Progress", "In Progress", "> Done"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestStatusTransitionPickerRendersAsOverlayDialog(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}
	model.transitions = map[string][]jira.Transition{
		"ABC-1": {
			{ID: "21", Name: "Start Progress", ToStatus: "In Progress"},
			{ID: "31", Name: "Done", ToStatus: "Done"},
		},
	}
	model.transitionFocus = true
	model.selectedTransition = 1
	model.jumpDetailSection("Status")

	view := model.render()

	for _, want := range []string{"Change Status", "ABC-1", "Current: To Do", "j/k select  enter apply  esc cancel", "> Done"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestDetailOverlayDialogStaysWithinVisibleDetailBody(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do", Assignee: "Jane Doe"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       model.issues[0],
			Description: strings.Repeat("detail line\n", 80),
		},
	}
	model.assigneeFocus = true

	view := model.renderFullDetail(model.browserLayout(model.width))
	lines := strings.Split(view, "\n")
	dialogLine := -1
	for index, line := range lines {
		if strings.Contains(line, "Change Assignee") {
			dialogLine = index
			break
		}
	}
	if dialogLine < 0 {
		t.Fatalf("missing assignee dialog in %q", view)
	}
	if dialogLine >= detailHeaderRows+model.fullDetailRows() {
		t.Fatalf("dialog rendered below visible detail body at line %d, max visible body line %d\n%s", dialogLine, detailHeaderRows+model.fullDetailRows()-1, view)
	}
}

func TestStatusTransitionSubmitTransitionsSelectedIssue(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.transitions = map[string][]jira.Transition{
		"ABC-1": {
			{ID: "21", Name: "Start Progress", ToStatus: "In Progress"},
			{ID: "31", Name: "Done", ToStatus: "Done"},
		},
	}
	model.transitionFocus = true
	model.selectedTransition = 1
	model.jumpDetailSection("Status")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("expected transition submit command")
	}
	if !next.transitionSubmitting {
		t.Fatal("transitionSubmitting should be true")
	}
	if next.transitionSubmitKey != "ABC-1" {
		t.Fatalf("transitionSubmitKey = %q", next.transitionSubmitKey)
	}
	if next.transitionSubmitToStatus != "Done" {
		t.Fatalf("transitionSubmitToStatus = %q", next.transitionSubmitToStatus)
	}
}

func TestStatusTransitionSuccessUpdatesIssueAndDetailStatus(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}
	model.activeTransitionReqID = 42
	model.transitionSubmitting = true
	model.transitionSubmitKey = "ABC-1"
	model.transitionFocus = true

	updated, _ := model.Update(workerResultMsg{result: worker.Result{
		ID:   42,
		Kind: worker.KindTransitionIssue,
		TransitionIssue: &worker.TransitionIssueResult{
			Key:      "ABC-1",
			ToStatus: "Done",
			SyncedAt: time.Now(),
		},
	}})
	next := updated.(Model)

	if next.transitionSubmitting {
		t.Fatal("transitionSubmitting should be false")
	}
	if next.transitionFocus {
		t.Fatal("transitionFocus should be false")
	}
	if next.issues[0].Status != "Done" {
		t.Fatalf("issue status = %q", next.issues[0].Status)
	}
	if next.details["ABC-1"].Status != "Done" {
		t.Fatalf("detail status = %q", next.details["ABC-1"].Status)
	}
	if !strings.Contains(next.detailNotice, "Status updated to Done") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestPriorityShortcutLoadsMetadataAndOpensPicker(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do", Priority: "Medium"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "p", Code: 'p'}))
	next := updated.(Model)
	if cmd == nil {
		t.Fatal("expected priority metadata load command")
	}
	if !next.priorityMetadataLoading {
		t.Fatal("priorityMetadataLoading should be true")
	}
	if next.priorityMetadataRequestKey != "ABC-1" {
		t.Fatalf("priorityMetadataRequestKey = %q", next.priorityMetadataRequestKey)
	}

	updated, _ = next.Update(workerResultMsg{result: worker.Result{
		ID:   next.activePriorityMetadataReqID,
		Kind: worker.KindGetEditMetadata,
		GetEditMetadata: &worker.GetEditMetadataResult{
			Key: "ABC-1",
			Metadata: jira.EditMetadata{
				Priority: jira.EditField{
					ID:       "priority",
					Name:     "Priority",
					Editable: true,
					AllowedValues: []jira.FieldOption{
						{ID: "2", Name: "High"},
						{ID: "3", Name: "Medium"},
						{ID: "4", Name: "Low"},
					},
				},
			},
			SyncedAt: time.Now(),
		},
	}})
	next = updated.(Model)
	if !next.priorityFocus {
		t.Fatal("expected priority picker focus")
	}
	if next.selectedPriority != 1 {
		t.Fatalf("selectedPriority = %d", next.selectedPriority)
	}

	view := next.render()
	for _, want := range []string{"Change Priority", "ABC-1", "Current: Medium", "High", "> Medium", "Low", "j/k select  enter apply  esc cancel"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestDetailEnterOnFocusedAssigneeOpensTypeaheadPicker(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Assignee: "Jane Doe", Priority: "Medium", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}
	model.moveDetailFocus(1)

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("enter should open assignee picker without submitting work")
	}
	if !next.assigneeFocus {
		t.Fatal("assigneeFocus should be true")
	}
	if next.assigneeQuery != "" {
		t.Fatalf("assigneeQuery = %q", next.assigneeQuery)
	}
	if !strings.Contains(next.render(), "Change Assignee") {
		t.Fatalf("missing assignee modal in %q", next.render())
	}

	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("typing should submit assignee user search")
	}
	if next.assigneeQuery != "j" {
		t.Fatalf("assigneeQuery = %q", next.assigneeQuery)
	}
	if !next.assigneeSearchLoading {
		t.Fatal("assigneeSearchLoading should be true")
	}

	updated, _ = next.Update(workerResultMsg{result: worker.Result{
		ID:   next.assigneeSearchReqID,
		Kind: worker.KindSearchUsers,
		SearchUsers: &worker.SearchUsersResult{
			Query: "j",
			Users: []jira.User{
				{AccountID: "abc-123", DisplayName: "Jane Doe"},
				{AccountID: "def-456", DisplayName: "Jon Charette"},
			},
		},
	}})
	next = updated.(Model)
	if next.assigneeSearchLoading {
		t.Fatal("assigneeSearchLoading should be false")
	}
	view := next.render()
	for _, want := range []string{"Filter: j", "Jane Doe", "Jon Charette", "> Jane Doe"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestPriorityPickerSubmitsSelectedPriority(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do", Priority: "Medium"}}
	model.editMetadata = map[string]jira.EditMetadata{
		"ABC-1": {
			Priority: jira.EditField{
				ID:       "priority",
				Name:     "Priority",
				Editable: true,
				AllowedValues: []jira.FieldOption{
					{ID: "2", Name: "High"},
					{ID: "3", Name: "Medium"},
					{ID: "4", Name: "Low"},
				},
			},
		},
	}
	model.priorityFocus = true
	model.selectedPriority = 0

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("expected priority update command")
	}
	if !next.prioritySubmitting {
		t.Fatal("prioritySubmitting should be true")
	}
	if next.prioritySubmitKey != "ABC-1" {
		t.Fatalf("prioritySubmitKey = %q", next.prioritySubmitKey)
	}
	if next.prioritySubmitValue.Name != "High" {
		t.Fatalf("prioritySubmitValue = %#v", next.prioritySubmitValue)
	}
}

func TestAssigneePickerSubmitsSelectedUser(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do", Assignee: "Jane Doe"}}
	model.assigneeFocus = true
	model.assigneeUsers = []jira.User{
		{AccountID: "abc-123", DisplayName: "Jane Doe"},
		{AccountID: "def-456", DisplayName: "John Doe"},
	}
	model.selectedAssignee = 1

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("expected assignee update command")
	}
	if !next.assigneeSubmitting {
		t.Fatal("assigneeSubmitting should be true")
	}
	if next.assigneeSubmitKey != "ABC-1" {
		t.Fatalf("assigneeSubmitKey = %q", next.assigneeSubmitKey)
	}
	if next.assigneeSubmitValue.AccountID != "def-456" {
		t.Fatalf("assigneeSubmitValue = %#v", next.assigneeSubmitValue)
	}
}

func TestAssigneePickerUsesCachedUserSearch(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do", Assignee: "Jane Doe"}}
	model.assigneeFocus = true
	model.assigneeQuery = "Jo"
	model.cacheUserSearch("jon", []jira.User{{AccountID: "abc-123", DisplayName: "Jon Charette"}})

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "n", Code: 'n'}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("cached assignee search should not submit worker command")
	}
	if next.assigneeSearchLoading {
		t.Fatal("assigneeSearchLoading should be false")
	}
	if len(next.assigneeUsers) != 1 || next.assigneeUsers[0].DisplayName != "Jon Charette" {
		t.Fatalf("assigneeUsers = %#v", next.assigneeUsers)
	}
	if !strings.Contains(next.render(), "Jon Charette") {
		t.Fatalf("missing cached user in %q", next.render())
	}
}

func TestAssigneeUpdateSuccessUpdatesIssueAndDetailAssignee(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do", Assignee: "Jane Doe"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}
	model.activeAssigneeReqID = 62
	model.assigneeSubmitting = true
	model.assigneeFocus = true
	model.assigneeSubmitKey = "ABC-1"

	updated, _ := model.Update(workerResultMsg{result: worker.Result{
		ID:   62,
		Kind: worker.KindUpdateAssignee,
		UpdateAssignee: &worker.UpdateAssigneeResult{
			Key:      "ABC-1",
			Assignee: jira.User{AccountID: "def-456", DisplayName: "John Doe"},
			SyncedAt: time.Now(),
		},
	}})
	next := updated.(Model)

	if next.assigneeSubmitting {
		t.Fatal("assigneeSubmitting should be false")
	}
	if next.assigneeFocus {
		t.Fatal("assigneeFocus should be false")
	}
	if next.issues[0].Assignee != "John Doe" {
		t.Fatalf("issue assignee = %q", next.issues[0].Assignee)
	}
	if next.details["ABC-1"].Assignee != "John Doe" {
		t.Fatalf("detail assignee = %q", next.details["ABC-1"].Assignee)
	}
	if !strings.Contains(next.detailNotice, "Assignee updated to John Doe") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestPriorityUpdateSuccessUpdatesIssueAndDetailPriority(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do", Priority: "Medium"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}
	model.activePriorityReqID = 61
	model.prioritySubmitting = true
	model.priorityFocus = true
	model.prioritySubmitKey = "ABC-1"

	updated, _ := model.Update(workerResultMsg{result: worker.Result{
		ID:   61,
		Kind: worker.KindUpdatePriority,
		UpdatePriority: &worker.UpdatePriorityResult{
			Key:      "ABC-1",
			Priority: jira.FieldOption{ID: "2", Name: "High"},
			SyncedAt: time.Now(),
		},
	}})
	next := updated.(Model)

	if next.prioritySubmitting {
		t.Fatal("prioritySubmitting should be false")
	}
	if next.priorityFocus {
		t.Fatal("priorityFocus should be false")
	}
	if next.issues[0].Priority != "High" {
		t.Fatalf("issue priority = %q", next.issues[0].Priority)
	}
	if next.details["ABC-1"].Priority != "High" {
		t.Fatalf("detail priority = %q", next.details["ABC-1"].Priority)
	}
	if !strings.Contains(next.detailNotice, "Priority updated to High") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestSummaryShortcutLoadsMetadataAndStartsEditor(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "s", Code: 's'}))
	next := updated.(Model)
	if cmd == nil {
		t.Fatal("expected metadata load command")
	}
	if !next.summaryFocus {
		t.Fatal("expected summary focus")
	}
	if !next.summaryMetadataLoading {
		t.Fatal("summaryMetadataLoading should be true")
	}
	if next.summaryMetadataRequestKey != "ABC-1" {
		t.Fatalf("summaryMetadataRequestKey = %q", next.summaryMetadataRequestKey)
	}

	updated, _ = next.Update(workerResultMsg{result: worker.Result{
		ID:   next.activeSummaryMetadataReqID,
		Kind: worker.KindGetEditMetadata,
		GetEditMetadata: &worker.GetEditMetadataResult{
			Key: "ABC-1",
			Metadata: jira.EditMetadata{
				Summary: jira.EditField{ID: "summary", Name: "Summary", Editable: true},
			},
			SyncedAt: time.Now(),
		},
	}})
	next = updated.(Model)
	if !next.summaryEditing {
		t.Fatal("expected summary editor")
	}
	if !next.summaryEditorReady {
		t.Fatal("expected summary textarea editor")
	}
	if next.summaryDraft != "Story" {
		t.Fatalf("summaryDraft = %q", next.summaryDraft)
	}
	if next.actionFocus || next.transitionFocus || next.hierarchyFocus || next.linkFocus {
		t.Fatalf("unexpected subfocus: action=%v transition=%v hierarchy=%v link=%v", next.actionFocus, next.transitionFocus, next.hierarchyFocus, next.linkFocus)
	}
}

func TestSummaryEditorRendersAsOverlayDialog(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Original story", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}
	model.summaryFocus = true
	model.summaryEditing = true
	model.summaryDraft = "Draft story"

	view := model.render()

	for _, want := range []string{"Edit Summary", "ABC-1", "Summary", "Draft story", "enter save  esc cancel"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	if !strings.Contains(view, "Summary: Original story") {
		t.Fatalf("detail header should keep saved summary behind overlay: %q", view)
	}
	if strings.Contains(view, "Summary: Draft story") {
		t.Fatalf("draft should render inside overlay instead of replacing detail header: %q", view)
	}
}

func TestSummaryShortcutDoesNotRenderInstructionNotice(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "s", Code: 's'}))
	next := updated.(Model)
	view := next.render()

	if !next.summaryFocus {
		t.Fatal("expected summary focus")
	}
	if next.detailNotice != "" {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
	if strings.Contains(view, "Summary selected. Press enter to edit.") {
		t.Fatalf("summary focus instruction should stay out of the body notice: %q", view)
	}
	if !strings.Contains(view, "Loading summary metadata") {
		t.Fatalf("metadata load should be visible while opening summary editor: %q", view)
	}
}

func TestSummaryEditorLongDraftShowsEditedSuffix(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 92
	model.height = 30
	model.issues = []jira.Issue{{
		Key:     "ABC-1",
		Summary: "add ability to create internal load balancers connecting to ECS deployments",
		Status:  "To Do",
	}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}
	model.summaryFocus = true
	model.summaryEditing = true
	model.summaryDraft = model.issues[0].Summary

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "!", Code: '!'}))
	next := updated.(Model)
	view := next.render()

	if next.summaryDraft != model.issues[0].Summary+"!" {
		t.Fatalf("summaryDraft = %q", next.summaryDraft)
	}
	if !strings.Contains(view, "deployments!") {
		t.Fatalf("edited suffix should be visible in summary dialog: %q", view)
	}
}

func TestSummaryEditorDuplicateEnterKeepsUnchangedDraftOpen(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}
	model.summaryFocus = true
	model.summaryEditing = true
	model.summaryDraft = "Story"

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("unchanged summary should not submit")
	}
	if !next.summaryEditing {
		t.Fatal("unchanged summary should keep editor open")
	}
	if !strings.Contains(next.detailNotice, "Edit summary before saving") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestSummaryEditorEnterBeforeTypingDoesNotSubmit(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "List summary", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}
	model.summaryFocus = true
	model.summaryEditing = true
	model.summaryDraft = "Draft from metadata path"

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("summary should not submit before the user edits")
	}
	if !next.summaryEditing {
		t.Fatal("summary editor should stay open before the user edits")
	}
	if !strings.Contains(next.detailNotice, "Edit summary before saving") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestSummaryEditorSubmitsWorkerBackedUpdate(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.summaryFocus = true
	model.summaryEditing = true
	model.summaryDirty = true
	model.summaryDraft = "Updated summary"

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("expected summary submit command")
	}
	if !next.summarySubmitting {
		t.Fatal("summarySubmitting should be true")
	}
	if next.summarySubmitKey != "ABC-1" {
		t.Fatalf("summarySubmitKey = %q", next.summarySubmitKey)
	}
	if next.summarySubmitValue != "Updated summary" {
		t.Fatalf("summarySubmitValue = %q", next.summarySubmitValue)
	}
	if !strings.Contains(next.detailNotice, "Updating summary") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestSummaryUpdateSuccessUpdatesIssueAndDetailSummary(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}
	model.activeSummaryReqID = 51
	model.summarySubmitting = true
	model.summaryEditing = true
	model.summarySubmitKey = "ABC-1"

	updated, _ := model.Update(workerResultMsg{result: worker.Result{
		ID:   51,
		Kind: worker.KindUpdateSummary,
		UpdateSummary: &worker.UpdateSummaryResult{
			Key:      "ABC-1",
			Summary:  "Updated summary",
			SyncedAt: time.Now(),
		},
	}})
	next := updated.(Model)

	if next.summarySubmitting {
		t.Fatal("summarySubmitting should be false")
	}
	if next.summaryEditing {
		t.Fatal("summaryEditing should be false")
	}
	if next.issues[0].Summary != "Updated summary" {
		t.Fatalf("issue summary = %q", next.issues[0].Summary)
	}
	if next.details["ABC-1"].Summary != "Updated summary" {
		t.Fatalf("detail summary = %q", next.details["ABC-1"].Summary)
	}
	if next.details["ABC-1"].Issue.Summary != "Updated summary" {
		t.Fatalf("detail issue summary = %q", next.details["ABC-1"].Issue.Summary)
	}
	if !strings.Contains(next.detailNotice, "Summary updated") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestSummaryEditCancelPreservesDetailNavigation(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}
	focusDetailSectionForTest(t, &model, "Comments")
	model.detailOffset = 4
	model.detailSectionOffset = map[string]int{"comments": 4}
	model.summaryFocus = true
	model.summaryEditing = true
	model.summaryDraft = "Changed"

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "esc", Code: tea.KeyEsc}))
	next := updated.(Model)

	if next.summaryEditing {
		t.Fatal("summaryEditing should be false")
	}
	if next.issues[0].Summary != "Story" {
		t.Fatalf("issue summary = %q", next.issues[0].Summary)
	}
	assertFocusedDetailSection(t, next, "Comments")
	if next.detailOffset != 4 {
		t.Fatalf("detail offset = %d", next.detailOffset)
	}
	if next.detailSectionOffset["comments"] != 4 {
		t.Fatalf("detailSectionOffset = %#v", next.detailSectionOffset)
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
	issues                 []jira.Issue
	detail                 jira.IssueDetail
	comments               []jira.Comment
	addedComment           jira.Comment
	addedBody              string
	addMentions            []jira.Mention
	users                  []jira.User
	transitions            []jira.Transition
	transitionKey          string
	transitionID           string
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

func (f *fakeIssueSearcher) TransitionIssue(_ context.Context, key string, transitionID string) error {
	if f.err != nil {
		return f.err
	}
	f.transitionKey = key
	f.transitionID = transitionID
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

func runClaudePlanProgressAsyncForTest(cmd tea.Cmd) <-chan tea.Msg {
	results := make(chan tea.Msg, 1)
	go func() {
		if cmd == nil {
			close(results)
			return
		}
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, sub := range batch {
				if sub == nil {
					continue
				}
				go func(sub tea.Cmd) {
					if result, ok := sub().(claudePlanProgressMsg); ok {
						results <- result
					}
				}(sub)
			}
			return
		}
		results <- msg
	}()
	return results
}

func runClaudePlanCommandAsyncForTest(cmd tea.Cmd) <-chan tea.Msg {
	results := make(chan tea.Msg, 1)
	go func() {
		if cmd == nil {
			close(results)
			return
		}
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, sub := range batch {
				if sub == nil {
					continue
				}
				go func(sub tea.Cmd) {
					result := sub()
					if _, ok := result.(claudePlanResultMsg); ok {
						results <- result
					}
					if _, ok := result.(claudeAssistResultMsg); ok {
						results <- result
					}
					if _, ok := result.(createAIPromptResultMsg); ok {
						results <- result
					}
				}(sub)
			}
			return
		}
		results <- msg
	}()
	return results
}

func hasDetailSection(model Model, id string) bool {
	for _, section := range model.detailSections() {
		if section.ID == id {
			return true
		}
	}
	return false
}
