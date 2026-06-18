package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jcharette/jira-tui/internal/cache"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/jcharette/jira-tui/internal/worker"
)

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
	if !strings.Contains(view, "Story") || !strings.Contains(view, "enter edit") {
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

func TestDetailEscReturnsToTablePreservingSelection(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.selected = 1
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "First issue"},
		{Key: "ABC-2", Summary: "Selected issue"},
	}
	model.details = map[string]jira.IssueDetail{
		"ABC-2": {Issue: jira.Issue{Key: "ABC-2", Summary: "Selected issue"}},
	}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "esc", Code: tea.KeyEsc}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("esc from detail should not submit work")
	}
	if next.mode != modeTable {
		t.Fatalf("mode = %v, want table", next.mode)
	}
	if next.selected != 1 || next.issues[next.selected].Key != "ABC-2" {
		t.Fatalf("selected issue changed: selected=%d issues=%#v", next.selected, next.issues)
	}
	view := next.render()
	if !strings.Contains(view, "Issue Table") || !strings.Contains(view, "ABC-2") {
		t.Fatalf("table view should render selected issue after esc: %q", view)
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

func TestDetailSectionNavigationRendersContextFooter(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 140
	model.height = 36
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", URL: "https://example.test/browse/ABC-1"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       jira.Issue{Key: "ABC-1", Summary: "Story", URL: "https://example.test/browse/ABC-1"},
			Description: "See https://example.test/build.",
		},
	}
	focusDetailSectionForTest(t, &model, "Description")

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "l", Code: 'l'}))
	next := updated.(Model)

	view := next.render()
	for _, want := range []string{"Links", "j/k link", "o/enter open", "y copy"} {
		if !strings.Contains(view, want) {
			t.Fatalf("links footer missing %q in %q", want, view)
		}
	}
	if strings.Contains(view, "j/k scroll") {
		t.Fatalf("links footer should replace generic detail scrolling hints: %q", view)
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
	if !strings.Contains(view, "Fix production thing") {
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

func TestTicketDetailFocusedEnterActionsStayConsistent(t *testing.T) {
	cases := []struct {
		target string
		assert func(t *testing.T, next Model, cmd tea.Cmd)
	}{
		{
			target: "summary",
			assert: func(t *testing.T, next Model, cmd tea.Cmd) {
				t.Helper()
				if cmd == nil || !next.summaryMetadataLoading || next.summaryMetadataRequestKey != "ABC-1" {
					t.Fatalf("summary enter did not load metadata: cmd=%v loading=%v key=%q", cmd != nil, next.summaryMetadataLoading, next.summaryMetadataRequestKey)
				}
			},
		},
		{
			target: "assignee",
			assert: func(t *testing.T, next Model, cmd tea.Cmd) {
				t.Helper()
				if cmd != nil || !next.assigneeFocus {
					t.Fatalf("assignee enter did not open picker: cmd=%v assigneeFocus=%v", cmd != nil, next.assigneeFocus)
				}
			},
		},
		{
			target: "priority",
			assert: func(t *testing.T, next Model, cmd tea.Cmd) {
				t.Helper()
				if cmd == nil || !next.priorityMetadataLoading || next.priorityMetadataRequestKey != "ABC-1" {
					t.Fatalf("priority enter did not load metadata: cmd=%v loading=%v key=%q", cmd != nil, next.priorityMetadataLoading, next.priorityMetadataRequestKey)
				}
			},
		},
		{
			target: "links",
			assert: func(t *testing.T, next Model, cmd tea.Cmd) {
				t.Helper()
				if cmd != nil || !next.linkFocus {
					t.Fatalf("links enter did not focus links: cmd=%v linkFocus=%v", cmd != nil, next.linkFocus)
				}
			},
		},
		{
			target: "hierarchy",
			assert: func(t *testing.T, next Model, cmd tea.Cmd) {
				t.Helper()
				if cmd != nil || !next.hierarchyFocus {
					t.Fatalf("hierarchy enter did not focus hierarchy: cmd=%v hierarchyFocus=%v", cmd != nil, next.hierarchyFocus)
				}
			},
		},
		{
			target: "comments",
			assert: func(t *testing.T, next Model, cmd tea.Cmd) {
				t.Helper()
				if cmd != nil || next.mode != modeComment {
					t.Fatalf("comments enter did not open composer: cmd=%v mode=%v", cmd != nil, next.mode)
				}
			},
		},
		{
			target: "actions",
			assert: func(t *testing.T, next Model, cmd tea.Cmd) {
				t.Helper()
				if cmd != nil || !next.actionFocus {
					t.Fatalf("actions enter did not focus menu: cmd=%v actionFocus=%v", cmd != nil, next.actionFocus)
				}
			},
		},
		{
			target: "status",
			assert: func(t *testing.T, next Model, cmd tea.Cmd) {
				t.Helper()
				if cmd == nil || !next.transitionLoading || next.transitionRequestKey != "ABC-1" {
					t.Fatalf("status enter did not load transitions: cmd=%v loading=%v key=%q", cmd != nil, next.transitionLoading, next.transitionRequestKey)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			model := newTicketDetailActionContractModel(t)
			model.detailFocus = detailTargetIndexForTest(t, model.detailTargets(), tc.target)

			updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
			tc.assert(t, updated.(Model), cmd)
		})
	}
}

func TestTicketDetailFocusedSectionFootersAdvertiseActions(t *testing.T) {
	cases := []struct {
		target string
		want   []string
	}{
		{target: "summary", want: []string{"Ticket Detail", "enter edit"}},
		{target: "assignee", want: []string{"Ticket Detail", "enter edit"}},
		{target: "priority", want: []string{"Ticket Detail", "enter edit"}},
		{target: "links", want: []string{"Ticket Detail", "j/k link", "enter focus", "y copy"}},
		{target: "hierarchy", want: []string{"Ticket Detail", "j/k child", "enter focus"}},
		{target: "comments", want: []string{"Ticket Detail", "enter add"}},
		{target: "actions", want: []string{"Ticket Detail", "j/k action", "enter focus"}},
		{target: "status", want: []string{"Ticket Detail", "enter transition"}},
	}

	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			model := newTicketDetailActionContractModel(t)
			model.detailFocus = detailTargetIndexForTest(t, model.detailTargets(), tc.target)

			footer := model.renderModelFooterHelp(model.browserLayout(model.width))

			for _, want := range tc.want {
				if !strings.Contains(footer, want) {
					t.Fatalf("missing %q in %q", want, footer)
				}
			}
		})
	}
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

func TestDetailHeaderPromotesSummaryAsTitle(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 120
	model.height = 32
	model.issues = []jira.Issue{{
		Key:       "ABC-1",
		Summary:   "Create slack alert on data provisioner migration fail",
		Status:    "In Progress",
		Priority:  "P4",
		IssueType: "Story",
		Assignee:  "Mike Person",
	}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:    model.issues[0],
			Reporter: "Jane Reporter",
			Updated:  time.Date(2026, 6, 16, 21, 58, 0, 0, time.Local),
		},
	}

	view := model.renderFullDetail(model.browserLayout(model.width))

	if strings.Contains(view, "Summary:") {
		t.Fatalf("summary label should not compete with the title line: %q", view)
	}
	identityIndex := strings.Index(view, "ABC-1")
	summaryIndex := strings.Index(view, "Create slack alert on data provisioner migration fail")
	metaIndex := strings.Index(view, "Assignee")
	if identityIndex < 0 || summaryIndex < 0 || metaIndex < 0 {
		t.Fatalf("missing expected header text in %q", view)
	}
	if !(identityIndex < summaryIndex && summaryIndex < metaIndex) {
		t.Fatalf("header should render identity, then summary title, then metadata: %q", view)
	}
	if strings.Contains(view, "  |  ") {
		t.Fatalf("metadata should use compact spacing instead of pipe dividers: %q", view)
	}
}

func TestDetailTabsUsePlainActiveMarker(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 120
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "In Progress", Priority: "High", IssueType: "Story", Assignee: "A Developer"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       model.issues[0],
			Description: "First line.",
		},
	}
	focusDetailSectionForTest(t, &model, "Description")

	tabs := model.renderDetailTabs(100)

	if !strings.Contains(tabs, "> Description") {
		t.Fatalf("active tab should use a plain selected marker: %q", tabs)
	}
	for _, want := range []string{"Hierarchy", "Comments", "Actions"} {
		if !strings.Contains(tabs, want) {
			t.Fatalf("missing inactive tab %q in %q", want, tabs)
		}
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

func TestDetailActionsMenuStartsStatusTransition(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}
	model.actionFocus = true
	model.selectedAction = detailActionIndexForTest(t, model.detailActions(), "transition")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("expected transition metadata command from Actions menu")
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

func TestDetailActionsMenuStartsAssigneePicker(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do", Assignee: "Jane Doe"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}
	model.actionFocus = true
	model.selectedAction = detailActionIndexForTest(t, model.detailActions(), "assign")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("assignee picker should open locally before searching")
	}
	if !next.assigneeFocus {
		t.Fatal("assigneeFocus should be true")
	}
	if !strings.Contains(next.render(), "Change Assignee") {
		t.Fatalf("missing assignee modal in %q", next.render())
	}
}

func detailActionIndexForTest(t *testing.T, actions []detailAction, id string) int {
	t.Helper()
	for index, action := range actions {
		if action.ID == id {
			return index
		}
	}
	t.Fatalf("missing detail action %q in %#v", id, actions)
	return 0
}

func newTicketDetailActionContractModel(t *testing.T) Model {
	t.Helper()
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	t.Cleanup(func() { model.workers.Stop() })
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Parent story", Status: "To Do", Priority: "Medium", IssueType: "Story", Assignee: "Jane Doe", URL: "https://example.test/browse/ABC-1"},
		{Key: "ABC-2", Summary: "Child task", Status: "To Do", Priority: "Low", IssueType: "Task", Assignee: "John Doe", ParentKey: "ABC-1"},
	}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       model.issues[0],
			Description: "Review https://example.test/runbook before changing this ticket.",
		},
	}
	model.comments = map[string][]jira.Comment{
		"ABC-1": {{ID: "10001", Author: "Comment Person", Body: "Existing comment."}},
	}
	return model
}

func detailTargetIndexForTest(t *testing.T, targets []detailTarget, id string) int {
	t.Helper()
	for index, target := range targets {
		if target.ID == id {
			return index
		}
	}
	t.Fatalf("missing detail target %q in %#v", id, targets)
	return 0
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

func TestStatusSectionUsesFreshPersistentTransitions(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	store := newFakeActiveViewStore()
	store.transitions = cache.IssueTransitionsRecord{
		Namespace:   "https://example.atlassian.net",
		IssueKey:    "ABC-1",
		Transitions: []jira.Transition{{ID: "21", Name: "Start Progress", ToStatus: "In Progress"}},
		SyncedAt:    now.Add(-10 * time.Second),
		FreshTill:   now.Add(time.Minute),
	}
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithActiveViewStore(store, "https://example.atlassian.net"))
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{"ABC-1": {Issue: model.issues[0]}}
	model.jumpDetailSection("Status")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("fresh persistent transitions should not submit background work")
	}
	if !next.transitionFocus {
		t.Fatal("transitionFocus should be true")
	}
	if len(next.transitions["ABC-1"]) != 1 || next.transitions["ABC-1"][0].Name != "Start Progress" {
		t.Fatalf("transitions = %#v", next.transitions["ABC-1"])
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

func TestAssigneePickerUsesAssigneeHelpContext(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Assignee: "Jane Doe", Priority: "Medium", Status: "To Do"}}
	model.assigneeFocus = true

	if activeKeyContext(model) != keyContextAssignee {
		t.Fatalf("activeKeyContext = %q", activeKeyContext(model))
	}
	footer := model.renderModelFooterHelp(model.browserLayout(model.width))
	for _, want := range []string{"Assignee", "type filter", "up/down select", "enter apply", "esc cancel"} {
		if !strings.Contains(footer, want) {
			t.Fatalf("missing %q in %q", want, footer)
		}
	}
}

func TestAssigneePickerFilterUsesCursorAwareTextInput(t *testing.T) {
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
	model.assigneeFocus = true
	model.assigneeQueryEditor = newUserSearchInput("")
	model.assigneeQueryEditorReady = true

	for _, key := range []tea.KeyMsg{
		tea.KeyPressMsg(tea.Key{Text: "J", Code: 'J'}),
		tea.KeyPressMsg(tea.Key{Text: "o", Code: 'o'}),
		tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}),
		tea.KeyPressMsg(tea.Key{Text: "h", Code: 'h'}),
	} {
		updated, _ := model.Update(key)
		model = updated.(Model)
	}

	if model.assigneeQuery != "Jho" {
		t.Fatalf("assigneeQuery = %q", model.assigneeQuery)
	}
}

func TestAssigneePickerUsesSharedChoiceListRendering(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Assignee: "Jane Doe", Priority: "Medium", Status: "To Do"}}
	model.assigneeFocus = true
	model.assigneeQuery = "user"
	model.assigneeUsers = []jira.User{
		{AccountID: "user-1", DisplayName: "User 01"},
		{AccountID: "user-2", DisplayName: "User 02"},
		{AccountID: "user-3", DisplayName: "User 03"},
		{AccountID: "user-4", DisplayName: "User 04"},
		{AccountID: "user-5", DisplayName: "User 05"},
		{AccountID: "user-6", DisplayName: "User 06"},
		{AccountID: "user-7", DisplayName: "User 07"},
		{AccountID: "user-8", DisplayName: "User 08"},
	}
	model.selectedAssignee = 6

	view := model.render()

	if !strings.Contains(view, "> User 07") {
		t.Fatalf("expected selected assignee from shared choice list, got %q", view)
	}
	if strings.Contains(view, "User 01") {
		t.Fatalf("expected shared choice list to page long assignee results, got %q", view)
	}
	if !strings.Contains(view, "of 8") {
		t.Fatalf("expected shared range indicator for assignee results, got %q", view)
	}
	if strings.Contains(view, "USER") {
		t.Fatalf("assignee picker should not render the old table header, got %q", view)
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

func TestPriorityEditorUsesFreshPersistentEditMetadata(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	store := newFakeActiveViewStore()
	store.editMetadata = cache.IssueEditMetadataRecord{
		Namespace: "https://example.atlassian.net",
		IssueKey:  "ABC-1",
		Metadata: jira.EditMetadata{
			Priority: jira.EditField{
				ID:            "priority",
				Name:          "Priority",
				Editable:      true,
				AllowedValues: []jira.FieldOption{{ID: "2", Name: "High"}},
			},
		},
		SyncedAt:  now.Add(-10 * time.Second),
		FreshTill: now.Add(time.Minute),
	}
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithActiveViewStore(store, "https://example.atlassian.net"))
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do", Priority: "Medium"}}

	updated, cmd := model.startPriorityEditor()
	next := updated

	if cmd != nil {
		t.Fatal("fresh persistent edit metadata should not submit background work")
	}
	if !next.priorityFocus {
		t.Fatal("priorityFocus should be true")
	}
	options := next.priorityOptions("ABC-1")
	if len(options) != 1 || options[0].Name != "High" {
		t.Fatalf("priority options = %#v", options)
	}
}

func TestTransitionAndEditMetadataResultsPersistToStore(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	store := newFakeActiveViewStore()
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithActiveViewStore(store, "https://example.atlassian.net"))
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = false
	model.mode = modeDetail
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do", Priority: "Medium"}}
	model.activeTransitionsReqID = 7
	model.transitionRequestKey = "ABC-1"
	model.activePriorityMetadataReqID = 8
	model.priorityMetadataRequestKey = "ABC-1"

	updated, _ := model.Update(workerResultMsg{result: worker.Result{
		ID:   7,
		Kind: worker.KindGetTransitions,
		GetTransitions: &worker.GetTransitionsResult{
			Key:         "ABC-1",
			Transitions: []jira.Transition{{ID: "21", Name: "Start Progress", ToStatus: "In Progress"}},
			SyncedAt:    now,
		},
	}})
	next := updated.(Model)
	updated, _ = next.Update(workerResultMsg{result: worker.Result{
		ID:   8,
		Kind: worker.KindGetEditMetadata,
		GetEditMetadata: &worker.GetEditMetadataResult{
			Key: "ABC-1",
			Metadata: jira.EditMetadata{
				Priority: jira.EditField{
					ID:            "priority",
					Name:          "Priority",
					Editable:      true,
					AllowedValues: []jira.FieldOption{{ID: "2", Name: "High"}},
				},
			},
			SyncedAt: now,
		},
	}})
	next = updated.(Model)

	if store.putTransitions.IssueKey != "ABC-1" || len(store.putTransitions.Transitions) != 1 || store.putTransitions.Transitions[0].ID != "21" {
		t.Fatalf("putTransitions = %#v", store.putTransitions)
	}
	if store.putEditMetadata.IssueKey != "ABC-1" || !store.putEditMetadata.Metadata.Priority.Editable {
		t.Fatalf("putEditMetadata = %#v", store.putEditMetadata)
	}
	if !store.putTransitions.FreshTill.Equal(now.Add(issueTransitionsCacheTTL)) {
		t.Fatalf("transitions FreshTill = %s", store.putTransitions.FreshTill)
	}
	if !store.putEditMetadata.FreshTill.Equal(now.Add(issueEditMetadataCacheTTL)) {
		t.Fatalf("metadata FreshTill = %s", store.putEditMetadata.FreshTill)
	}
}

func TestPriorityPickerUsesSharedChoiceListRendering(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do", Priority: "P2"}}
	model.editMetadata = map[string]jira.EditMetadata{
		"ABC-1": {
			Priority: jira.EditField{
				ID:       "priority",
				Name:     "Priority",
				Editable: true,
				AllowedValues: []jira.FieldOption{
					{ID: "p0", Name: "P0"},
					{ID: "p1", Name: "P1"},
					{ID: "p2", Name: "P2"},
					{ID: "p3", Name: "P3"},
					{ID: "p4", Name: "P4"},
					{ID: "p5", Name: "P5"},
					{ID: "p6", Name: "P6"},
					{ID: "p7", Name: "P7"},
				},
			},
		},
	}
	model.priorityFocus = true
	model.selectedPriority = 6

	view := model.render()

	if !strings.Contains(view, "> P6") {
		t.Fatalf("expected selected priority from shared choice list:\n%s", view)
	}
	if strings.Contains(view, "P0") {
		t.Fatalf("expected shared choice list to page long priority results:\n%s", view)
	}
	if !strings.Contains(view, "of 8") {
		t.Fatalf("expected shared range indicator for priority results:\n%s", view)
	}
	if strings.Contains(view, "PRIORITY") {
		t.Fatalf("priority picker should not render the old table header:\n%s", view)
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
	headerIndex := strings.Index(view, "Original story")
	dialogIndex := strings.Index(view, "Edit Summary")
	draftIndex := strings.Index(view, "Draft story")
	if headerIndex < 0 {
		t.Fatalf("detail header should keep saved summary behind overlay: %q", view)
	}
	if dialogIndex < 0 || draftIndex < 0 || !(headerIndex < dialogIndex && dialogIndex < draftIndex) {
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

func hasDetailSection(model Model, id string) bool {
	for _, section := range model.detailSections() {
		if section.ID == id {
			return true
		}
	}
	return false
}
