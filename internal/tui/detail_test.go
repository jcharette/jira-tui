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

func indexOfDetailTargetForTest(model Model, id string) int {
	for index, target := range model.detailTargets() {
		if target.ID == id {
			return index
		}
	}
	return 0
}

func TestDetailSectionsUseOverviewFirstWithoutStatusTab(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}

	sections := model.detailSections()
	got := make([]string, 0, len(sections))
	for _, section := range sections {
		got = append(got, section.ID)
	}
	wantPrefix := []string{"overview", "comments", "worklog", "hierarchy"}
	if len(got) < len(wantPrefix) {
		t.Fatalf("sections = %#v", got)
	}
	for index, want := range wantPrefix {
		if got[index] != want {
			t.Fatalf("sections = %#v, want prefix %#v", got, wantPrefix)
		}
	}
	for _, id := range got {
		if id == "status" || id == "description" || id == "actions" {
			t.Fatalf("section %q should not be a primary tab: %#v", id, got)
		}
	}
}

func TestTicketDetailDefaultsToOverviewTarget(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	target, ok := next.focusedDetailTarget()
	if !ok || target.ID != "overview" {
		t.Fatalf("focused target = %#v ok=%v, want overview", target, ok)
	}
}

func TestEnterOnStatusFieldStartsTransitionPicker(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.detailFocus = indexOfDetailTargetForTest(model, "status")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("expected transition metadata command")
	}
	if !next.transitionLoading || next.transitionRequestKey != "ABC-1" {
		t.Fatalf("transition state loading=%v key=%q", next.transitionLoading, next.transitionRequestKey)
	}
}

func TestRenderFullDetailShowsOverviewControlStrip(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 120
	model.height = 32
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do", Priority: "P3 - Low", Assignee: "Jon C.", IssueType: "Epic"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0], Description: "Long description body that should be previewed.", Reporter: "Jon C."},
	}

	view := model.render()

	for _, want := range []string{"Overview", "Status", "To Do", "Priority", "P3 - Low", "Assignee", "Jon C.", "Description"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	if strings.Contains(view, "Description   Hierarchy") || strings.Contains(view, "> Status") {
		t.Fatalf("old tab layout still visible in %q", view)
	}
}

func TestOverviewSummarizesCommentsAndHierarchy(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 120
	model.height = 32
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Parent", Status: "To Do"},
		{Key: "ABC-2", Summary: "Child", Status: "To Do", ParentKey: "ABC-1"},
	}
	model.comments = map[string][]jira.Comment{
		"ABC-1": {{ID: "10001", Author: "Sam Person", Body: "Latest update"}},
	}

	view := model.render()

	for _, want := range []string{"Latest", "Sam P.", "Latest update", "Hierarchy", "1 loaded child"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestOverviewExpandsDescriptionByDefault(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 140
	model.height = 40
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Parent epic", Status: "To Do", IssueType: "Epic"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue: model.issues[0],
			Description: strings.Join([]string{
				"## Summary",
				"Build a reusable EKS platform baseline.",
				"Install platform controllers.",
				"Automate Helm chart deployment.",
				"Document validation and rollback expectations.",
				"Publish operational runbooks.",
			}, "\n\n"),
		},
	}

	view := model.render()

	for _, want := range []string{"Description", "Build a reusable EKS platform baseline.", "Automate Helm chart deployment.", "Publish operational runbooks."} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing expanded description text %q in:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Description preview") {
		t.Fatalf("overview should render expanded description, got preview header in:\n%s", view)
	}
}

func TestDetailFooterShowsStatusControlAction(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.detailFocus = indexOfDetailTargetForTest(model, "status")

	footer := model.renderModelFooterHelp(model.browserLayout(model.width))

	if !strings.Contains(footer, "enter transition") {
		t.Fatalf("status footer missing transition action: %q", footer)
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

func TestDetailTabStartsAtOverviewThenEditableControls(t *testing.T) {
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

	if target, ok := model.focusedDetailTarget(); !ok || target.ID != "overview" {
		t.Fatalf("initial target = %#v ok=%v", target, ok)
	}
	view := model.render()
	if !strings.Contains(view, "Story") || !strings.Contains(view, "> Overview") {
		t.Fatalf("initial detail focus should expose overview: %q", view)
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next := updated.(Model)
	if target, ok := next.focusedDetailTarget(); !ok || target.ID != "summary" {
		t.Fatalf("after first tab target = %#v ok=%v", target, ok)
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	if target, ok := next.focusedDetailTarget(); !ok || target.ID != "status" {
		t.Fatalf("after second tab target = %#v ok=%v", target, ok)
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	if target, ok := next.focusedDetailTarget(); !ok || target.ID != "priority" {
		t.Fatalf("after third tab target = %#v ok=%v", target, ok)
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
	model.detailFocus = indexOfDetailTargetForTest(model, "summary")

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
	model.detailFocus = indexOfDetailTargetForTest(model, "priority")

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

func TestDetailFooterShowsActionPaletteCommandWithoutActionsTab(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 140
	model.height = 40
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", URL: "https://example.test/browse/ABC-1"}}

	view := model.render()

	for _, want := range []string{"Ticket Detail", ". actions"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	if strings.Contains(view, "> Actions") {
		t.Fatalf("Actions should not be a primary tab: %q", view)
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

func TestActionsPaletteOpensWithoutActionsSection(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 40
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", URL: "https://example.test/browse/ABC-1"}}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: ".", Code: '.'}))
	next := updated.(Model)
	if !next.actionPaletteOpen {
		t.Fatal("expected action palette to open")
	}
	if strings.Contains(next.render(), "> Actions") {
		t.Fatalf("Actions should not be a primary tab: %q", next.render())
	}
}

func TestDetailAKeyOpensOverviewInlineAIWhenAvailable(t *testing.T) {
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
	model.details = map[string]jira.IssueDetail{"ABC-1": {Issue: model.issues[0], Description: "Current description"}}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "a", Code: 'a'}))
	next := updated.(Model)

	if !next.inlineAIOpen {
		t.Fatal("expected overview inline AI picker to open")
	}
	if next.mode == modeComment {
		t.Fatal("a should jump to AI when Claude is available, not open comment compose")
	}
}

func TestDetailAKeyDoesNothingWhenAIUnavailable(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.comments = map[string][]jira.Comment{"ABC-1": {}}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "a", Code: 'a'}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("a without AI should not submit work")
	}
	if next.mode != modeDetail {
		t.Fatalf("mode = %v, want detail", next.mode)
	}
	if next.inlineAIOpen {
		t.Fatal("inline AI picker should remain closed")
	}
	if section, ok := next.focusedDetailSection(); ok && strings.EqualFold(section.Label, "Claude") {
		t.Fatalf("focused section = %#v, want non-Claude when AI is unavailable", section)
	}
}

func TestOverviewFocusShowsInlineAIWhenClaudeTicketAssistAvailable(t *testing.T) {
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

	view := model.render()
	if !strings.Contains(view, "a ai") {
		t.Fatalf("expected inline AI footer hint in %q", view)
	}
}

func TestOverviewAKeyOpensInlineAIPicker(t *testing.T) {
	model := newInlineDescriptionAIModel(t)

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "a"}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("picker should not start Claude")
	}
	if !next.inlineAIOpen {
		t.Fatal("expected inline AI picker open")
	}
	view := next.render()
	for _, want := range []string{"Ticket Assist", "Improve ticket", "subtask recommendations", "Extract acceptance criteria", "Ask Claude a question", "Draft clarifying comment", "enter run", "esc cancel"} {
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
	if !strings.Contains(view, "Issue Lanes") || !strings.Contains(view, "ABC-2") {
		t.Fatalf("issue list should render selected issue after esc: %q", view)
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
	focusDetailSectionForTest(t, &model, "Overview")

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next := updated.(Model)
	if next.focusedDetailTargetID() != "summary" {
		t.Fatalf("expected tab to select summary control, target=%q", next.focusedDetailTargetID())
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "h", Code: 'h'}))
	next = updated.(Model)
	assertFocusedDetailSection(t, next, "Hierarchy")
	if next.detailOffset != 0 {
		t.Fatalf("expected h to select hierarchy, offset=%d", next.detailOffset)
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "[", Code: '['}))
	next = updated.(Model)
	if next.focusedDetailTargetID() != "worklog" || next.detailOffset != 0 {
		t.Fatalf("expected [ to select previous section target at its saved scroll, target=%q offset=%d", next.focusedDetailTargetID(), next.detailOffset)
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
	focusDetailSectionForTest(t, &model, "Overview")

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
	focusDetailSectionForTest(t, &model, "Overview")

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "pgdown", Code: tea.KeyPgDown}))
	next := updated.(Model)
	overviewOffset := next.detailOffset
	if overviewOffset == 0 {
		t.Fatal("expected overview scroll offset to advance")
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "h", Code: 'h'}))
	next = updated.(Model)
	assertFocusedDetailSection(t, next, "Hierarchy")
	if next.detailOffset != 0 {
		t.Fatalf("expected hierarchy section at top, offset=%d", next.detailOffset)
	}

	next.jumpDetailSection("Overview")
	if next.focusedDetailTargetID() != "overview" || next.detailOffset != overviewOffset {
		t.Fatalf("expected overview offset %d to restore, target=%q offset=%d", overviewOffset, next.focusedDetailTargetID(), next.detailOffset)
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
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	for _, notWant := range []string{"Linked Issues", "Linked issue data is not loaded yet."} {
		if strings.Contains(view, notWant) {
			t.Fatalf("stale linked issue placeholder rendered %q in %q", notWant, view)
		}
	}
	if strings.Contains(view, "No parent or child issues in the current result.") {
		t.Fatalf("old hierarchy empty state should not render when grouped rows exist: %q", view)
	}
}

func TestHierarchySectionShowsKnownParentEmptyState(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 40
	model.issues = []jira.Issue{
		{Key: "ABC-2", Summary: "Child task", IssueType: "Task", ParentKey: "ABC-1", ParentSummary: "Parent story"},
	}
	model.details = map[string]jira.IssueDetail{
		"ABC-2": {Issue: model.issues[0], Description: "Detail"},
	}
	focusDetailSectionForTest(t, &model, "Hierarchy")

	view := model.render()

	for _, want := range []string{"Path", "Parent", "ABC-1", "Parent story", "No child or subtask issues loaded in the current view."} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	for _, notWant := range []string{"Linked Issues", "Linked issue data is not loaded yet.", "No parent or child issues in the current result."} {
		if strings.Contains(view, notWant) {
			t.Fatalf("unexpected stale hierarchy copy %q in %q", notWant, view)
		}
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

func TestFullDetailContentRendersJiraIssueLinksSection(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "In Progress", Priority: "High", IssueType: "Story", Assignee: "A Developer", URL: "https://example.atlassian.net/browse/ABC-1"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       model.issues[0],
			Description: "No external links here.",
			IssueLinks: []jira.IssueLink{
				{
					Direction:    "outward",
					Relationship: "blocks",
					Key:          "ABC-2",
					Summary:      "Blocked downstream task",
					Status:       "To Do",
					IssueType:    "Task",
					URL:          "https://example.atlassian.net/browse/ABC-2",
				},
			},
		},
	}
	model.jumpDetailSection("Links")

	content := model.fullDetailContent(100)

	for _, want := range []string{"Links", "ABC-2", "blocks", "Blocked downstream task", "To Do"} {
		if !strings.Contains(content, want) {
			t.Fatalf("missing %q in %q", want, content)
		}
	}
	tabs := model.detailTabsLine(model.detailSections(), 100, false)
	if !strings.Contains(tabs, "Links 1") {
		t.Fatalf("tabs = %q, want Links badge", tabs)
	}
}

func TestDetailIssueLinksOpenURLAndCopyKey(t *testing.T) {
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
	model.width = 100
	model.height = 60
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       jira.Issue{Key: "ABC-1"},
			Description: "No external links here.",
			IssueLinks: []jira.IssueLink{
				{
					Relationship: "blocks",
					Key:          "ABC-2",
					Summary:      "Blocked downstream task",
					URL:          "https://example.atlassian.net/browse/ABC-2",
				},
			},
		},
	}
	model.focusDetailLinks()

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if cmd == nil {
		t.Fatal("expected open command")
	}
	updated, _ = next.Update(cmd())
	next = updated.(Model)
	if opened != "https://example.atlassian.net/browse/ABC-2" {
		t.Fatalf("opened = %q", opened)
	}

	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "y", Code: 'y'}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected copy command")
	}
	updated, _ = next.Update(cmd())
	next = updated.(Model)
	if copied != "ABC-2" {
		t.Fatalf("copied = %q", copied)
	}
}

func TestDetailIssueLinkDeleteRequiresConfirmation(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 60
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue: jira.Issue{Key: "ABC-1"},
			IssueLinks: []jira.IssueLink{{
				LinkID:       "20001",
				Relationship: "blocks",
				Key:          "ABC-2",
				URL:          "https://example.atlassian.net/browse/ABC-2",
			}},
		},
	}
	model.focusDetailLinks()

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "d", Code: 'd'}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("delete confirmation should open locally")
	}
	if !next.issueLinkDeleteConfirm || next.issueLinkDeleteID != "20001" {
		t.Fatalf("delete confirm = %v id=%q", next.issueLinkDeleteConfirm, next.issueLinkDeleteID)
	}
	if !strings.Contains(next.render(), "Remove Link") {
		t.Fatalf("missing remove dialog:\n%s", next.render())
	}

	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("confirming delete should enqueue worker request")
	}
	if !next.issueLinkDeleteSubmitting || next.activeDeleteIssueLinkReqID == 0 {
		t.Fatalf("delete submit state submitting=%v req=%d", next.issueLinkDeleteSubmitting, next.activeDeleteIssueLinkReqID)
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
	for _, want := range []string{"Overview", "Comments", "Worklog", "Hierarchy"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing detail tab %q in %q", want, view)
		}
	}
	for _, notWant := range []string{"> Description", "> Actions", "> Status"} {
		if strings.Contains(view, notWant) {
			t.Fatalf("legacy tab %q should not be focused in %q", notWant, view)
		}
	}
	if !strings.Contains(view, "Fix production thing") {
		t.Fatalf("expected ticket summary in %q", view)
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next := updated.(Model)
	if next.focusedDetailTargetID() != "summary" {
		t.Fatalf("focused target = %q", next.focusedDetailTargetID())
	}
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	if next.focusedDetailTargetID() != "status" {
		t.Fatalf("focused target = %q", next.focusedDetailTargetID())
	}
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "m", Code: 'm'}))
	next = updated.(Model)
	assertFocusedDetailSection(t, next, "Comments")
	if view := next.render(); !strings.Contains(view, "Comments") {
		t.Fatalf("expected focused comments section in %q", view)
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
				if cmd != nil || !next.commentFocus {
					t.Fatalf("comments enter did not focus comments: cmd=%v commentFocus=%v", cmd != nil, next.commentFocus)
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
		{target: "summary", want: []string{"Ticket Detail", "enter summary"}},
		{target: "assignee", want: []string{"Ticket Detail", "enter assignee"}},
		{target: "priority", want: []string{"Ticket Detail", "enter priority"}},
		{target: "links", want: []string{"Ticket Detail", "j/k link", "enter focus", "y copy"}},
		{target: "hierarchy", want: []string{"Ticket Detail", "j/k child", "enter focus"}},
		{target: "comments", want: []string{"Ticket Detail", "enter add"}},
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

	for _, want := range []string{"Overview", "Description", "First description line."} {
		if !strings.Contains(content, want) {
			t.Fatalf("missing focused detail workspace text %q in %q", want, content)
		}
	}
	for _, notWant := range []string{"Collapsed:", "Hierarchy 1", "Comments 1", "> Actions"} {
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
	focusDetailSectionForTest(t, &model, "Overview")

	tabs := model.renderDetailTabs(100)

	if !strings.Contains(tabs, "> Overview") {
		t.Fatalf("active tab should use a plain selected marker: %q", tabs)
	}
	for _, want := range []string{"Comments", "Hierarchy"} {
		if !strings.Contains(tabs, want) {
			t.Fatalf("missing inactive tab %q in %q", want, tabs)
		}
	}
	for _, notWant := range []string{"Description", "Actions", "Status"} {
		if strings.Contains(tabs, notWant) {
			t.Fatalf("legacy tab %q should not render in %q", notWant, tabs)
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
	for _, want := range []string{"Over", "Com", "Work", "Tree"} {
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

func TestDetailActionsPaletteListsSafeActionsAndGenericEditFields(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Parent epic", Status: "In Progress", Priority: "High", IssueType: "Epic", Assignee: "A Developer", URL: "https://example.test/browse/ABC-1"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0], Description: "Parent description."},
	}
	model.editMetadata = map[string]jira.EditMetadata{
		"ABC-1": {
			Fields: []jira.EditField{
				{ID: "summary", Name: "Summary", Editable: true},
				{ID: "customfield_10016", Name: "Story Points", Editable: true, SchemaType: "number"},
				{ID: "fixVersions", Name: "Fix Version/s", Editable: true, SchemaType: "array", SchemaItems: "version", AllowedValues: []jira.FieldOption{{ID: "10001", Name: "1.0.0"}}},
				{ID: "versions", Name: "Affects Version/s", Editable: true, SchemaType: "array", SchemaItems: "version", AllowedValues: []jira.FieldOption{{ID: "10002", Name: "1.1.0"}}},
				{ID: "duedate", Name: "Due date", Editable: true, SchemaType: "date"},
				{ID: "parent", Name: "Parent", Editable: true, SchemaType: "issuelink"},
				{ID: "timetracking", Name: "Time tracking", Editable: true, SchemaType: "timetracking"},
				{ID: "customfield_10017", Name: "Team", Editable: false},
			},
		},
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: ".", Code: '.'}))
	next := updated.(Model)
	if !next.actionPaletteOpen {
		t.Fatal("expected action palette")
	}
	view := next.render()
	for _, want := range []string{"Ticket Actions", "Add Comment", "Edit Summary", "Change Priority"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	if !strings.Contains(view, "Edit Story Points") {
		t.Fatalf("metadata-backed generic field should render:\n%s", view)
	}
	for _, want := range []string{"Set Fix Version", "Set Affects Version", "Set Due Date"} {
		if !strings.Contains(view, want) {
			t.Fatalf("metadata-backed standard field should render %q:\n%s", want, view)
		}
	}
	for _, want := range []string{"Set Parent", "Edit Estimates"} {
		if !strings.Contains(view, want) {
			t.Fatalf("metadata-backed workflow field should render %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Edit Team") {
		t.Fatalf("non-editable metadata field should not render:\n%s", view)
	}
	if strings.Count(view, "Edit Summary") != 1 {
		t.Fatalf("supported metadata field should not duplicate Edit Summary:\n%s", view)
	}
	if strings.Contains(view, "Edit Fields") {
		t.Fatalf("generic Edit Fields action should not render: %q", view)
	}
	actionLabels := map[string]detailAction{}
	for _, action := range next.detailActions() {
		actionLabels[action.ID] = action
	}
	if actionLabels["summary"].Label != "Edit Summary" || !actionLabels["summary"].Enabled {
		t.Fatalf("summary action = %#v", actionLabels["summary"])
	}
	if actionLabels["priority"].Label != "Change Priority" || !actionLabels["priority"].Enabled {
		t.Fatalf("priority action = %#v", actionLabels["priority"])
	}
	if actionLabels["labels"].Label != "Edit Labels" || !actionLabels["labels"].Enabled {
		t.Fatalf("labels action = %#v", actionLabels["labels"])
	}
	if actionLabels["components"].Label != "Edit Components" || !actionLabels["components"].Enabled {
		t.Fatalf("components action = %#v", actionLabels["components"])
	}
	if _, ok := actionLabels["edit-fields"]; ok {
		t.Fatalf("generic edit-fields action should be removed: %#v", actionLabels["edit-fields"])
	}
	storyPoints := actionLabels["field:customfield_10016"]
	if storyPoints.Label != "Edit Story Points" || !storyPoints.Enabled {
		t.Fatalf("story points action = %#v", storyPoints)
	}
	if !strings.Contains(storyPoints.Description, "number") {
		t.Fatalf("story points description should include schema context: %#v", storyPoints)
	}
	if !strings.Contains(storyPoints.Description, "generic editor") {
		t.Fatalf("story points description should include generic editor context: %#v", storyPoints)
	}
	if actionLabels["field:fixVersions"].Label != "Set Fix Version" || !actionLabels["field:fixVersions"].Enabled {
		t.Fatalf("fix version action = %#v", actionLabels["field:fixVersions"])
	}
	if actionLabels["field:versions"].Label != "Set Affects Version" || !actionLabels["field:versions"].Enabled {
		t.Fatalf("affects version action = %#v", actionLabels["field:versions"])
	}
	if actionLabels["field:duedate"].Label != "Set Due Date" || !actionLabels["field:duedate"].Enabled {
		t.Fatalf("due date action = %#v", actionLabels["field:duedate"])
	}
	if actionLabels["field:parent"].Label != "Set Parent" || !actionLabels["field:parent"].Enabled {
		t.Fatalf("parent action = %#v", actionLabels["field:parent"])
	}
	if actionLabels["field:timetracking"].Label != "Edit Estimates" || !actionLabels["field:timetracking"].Enabled {
		t.Fatalf("time tracking action = %#v", actionLabels["field:timetracking"])
	}
	if actionLabels["subtask"].Label != "Create Subtask" || !actionLabels["subtask"].Enabled {
		t.Fatalf("subtask action = %#v", actionLabels["subtask"])
	}
	if actionLabels["start-work"].Label != "Start Work" || !actionLabels["start-work"].Enabled {
		t.Fatalf("start work action = %#v", actionLabels["start-work"])
	}
	if activeKeyContext(next) != keyContextActionPalette {
		t.Fatalf("activeKeyContext = %q", activeKeyContext(next))
	}

	next.selectedActionPalette = actionPaletteIndexForTest(t, next.filteredActionPaletteActions(), "comment")
	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)
	if cmd != nil {
		t.Fatal("add comment action should not submit work immediately")
	}
	if next.mode != modeComment {
		t.Fatalf("mode = %v, want comment", next.mode)
	}

	model.actionFocus = true
	model.selectedAction = detailActionIndexForTest(t, model.detailActions(), "subtask")
	updated, cmd = model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)
	if !next.createOpen {
		t.Fatal("subtask action should open create modal")
	}
	if next.createParentKey != "ABC-1" {
		t.Fatalf("createParentKey = %q", next.createParentKey)
	}
	if cmd == nil {
		t.Fatal("subtask action should request create metadata")
	}
}

func TestMetadataBackedStandardActionOpensGenericFieldEditor(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do", Priority: "Medium"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}
	model.editMetadata = map[string]jira.EditMetadata{
		"ABC-1": {
			Fields: []jira.EditField{
				{ID: "fixVersions", Name: "Fix Version/s", Editable: true, SchemaType: "array", SchemaItems: "version", AllowedValues: []jira.FieldOption{{ID: "10001", Name: "1.0.0"}}},
			},
		},
	}
	model.actionFocus = true
	model.selectedAction = detailActionIndexForTest(t, model.detailActions(), "field:fixVersions")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("cached metadata should open generic field editor without loading")
	}
	if !next.genericFieldFocus {
		t.Fatal("expected generic field focus")
	}
	if next.genericField.ID != "fixVersions" {
		t.Fatalf("genericField = %#v", next.genericField)
	}
	if !strings.Contains(next.render(), "Set Fix Version") {
		t.Fatalf("missing standard field modal title in %q", next.render())
	}
}

func TestMetadataBackedParentActionOpensParentEditor(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do", ParentKey: "ABC-100"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}
	model.editMetadata = map[string]jira.EditMetadata{
		"ABC-1": {
			Fields: []jira.EditField{
				{ID: "parent", Name: "Parent", Editable: true, SchemaType: "issuelink"},
			},
		},
	}
	model.actionFocus = true
	model.selectedAction = detailActionIndexForTest(t, model.detailActions(), "field:parent")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("cached metadata should open parent editor without loading")
	}
	if !next.parentFocus {
		t.Fatal("expected parent focus")
	}
	if !strings.Contains(next.render(), "Set Parent") {
		t.Fatalf("missing parent modal in %q", next.render())
	}
}

func TestMetadataBackedTimeTrackingActionOpensEstimateEditor(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0], OriginalEstimate: "2d", RemainingEstimate: "3h"},
	}
	model.editMetadata = map[string]jira.EditMetadata{
		"ABC-1": {
			Fields: []jira.EditField{
				{ID: "timetracking", Name: "Time tracking", Editable: true, SchemaType: "timetracking"},
			},
		},
	}
	model.actionFocus = true
	model.selectedAction = detailActionIndexForTest(t, model.detailActions(), "field:timetracking")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("cached metadata should open estimate editor without loading")
	}
	if !next.timeTrackingFocus {
		t.Fatal("expected time tracking focus")
	}
	if next.timeTrackingOriginalEditorValue() != "2d" || next.timeTrackingRemainingEditorValue() != "3h" {
		t.Fatalf("estimate drafts = %q/%q", next.timeTrackingOriginalEditorValue(), next.timeTrackingRemainingEditorValue())
	}
	if !strings.Contains(next.render(), "Edit Estimates") {
		t.Fatalf("missing estimates modal in %q", next.render())
	}
}

func TestActionPaletteOpensAndFiltersDetailActions(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 110
	model.height = 34
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do", Priority: "Medium"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: ".", Code: '.'}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("opening the action palette should not submit work")
	}
	if !next.actionPaletteOpen {
		t.Fatal("expected action palette to open")
	}
	if activeKeyContext(next) != keyContextActionPalette {
		t.Fatalf("activeKeyContext = %q", activeKeyContext(next))
	}

	view := next.render()
	for _, want := range []string{"Ticket Actions", "ABC-1", "Filter", "Edit Summary", "Change Priority", "Transition Status"} {
		if !strings.Contains(view, want) {
			t.Fatalf("palette missing %q in:\n%s", want, view)
		}
	}

	for _, key := range []tea.KeyMsg{
		tea.KeyPressMsg(tea.Key{Text: "p", Code: 'p'}),
		tea.KeyPressMsg(tea.Key{Text: "r", Code: 'r'}),
		tea.KeyPressMsg(tea.Key{Text: "i", Code: 'i'}),
	} {
		updated, _ = next.Update(key)
		next = updated.(Model)
	}
	if next.actionPaletteFilter != "pri" {
		t.Fatalf("actionPaletteFilter = %q", next.actionPaletteFilter)
	}
	view = next.render()
	if !strings.Contains(view, "Change Priority") {
		t.Fatalf("filtered palette missing priority action:\n%s", view)
	}
	if strings.Contains(view, "Edit Summary") {
		t.Fatalf("filtered palette should hide non-matching summary action:\n%s", view)
	}
}

func TestActionPaletteRunsFilteredActionThroughExistingWorkflow(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do", Priority: "Medium"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}
	model.openActionPalette()
	for _, key := range []tea.KeyMsg{
		tea.KeyPressMsg(tea.Key{Text: "p", Code: 'p'}),
		tea.KeyPressMsg(tea.Key{Text: "r", Code: 'r'}),
		tea.KeyPressMsg(tea.Key{Text: "i", Code: 'i'}),
	} {
		updated, _ := model.Update(key)
		model = updated.(Model)
	}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("expected priority metadata command from action palette")
	}
	if next.actionPaletteOpen {
		t.Fatal("action palette should close after running an action")
	}
	if !next.priorityFocus {
		t.Fatal("expected priority focus")
	}
	if !next.priorityMetadataLoading {
		t.Fatal("priorityMetadataLoading should be true")
	}
	if next.priorityMetadataRequestKey != "ABC-1" {
		t.Fatalf("priorityMetadataRequestKey = %q", next.priorityMetadataRequestKey)
	}
}

func TestActionPaletteFindsUnsupportedEditFieldWithoutSubmitting(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do", Priority: "Medium"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}
	model.editMetadata = map[string]jira.EditMetadata{
		"ABC-1": {
			Fields: []jira.EditField{{
				ID:              "customfield_10016",
				Name:            "Story Points",
				Editable:        true,
				SchemaType:      "array",
				SchemaItems:     "group",
				AutoCompleteURL: "https://example.atlassian.net/rest/api/3/customFieldOption/suggest",
			}},
		},
	}
	model.openActionPalette()
	for _, key := range []tea.KeyMsg{
		tea.KeyPressMsg(tea.Key{Text: "s", Code: 's'}),
		tea.KeyPressMsg(tea.Key{Text: "t", Code: 't'}),
		tea.KeyPressMsg(tea.Key{Text: "o", Code: 'o'}),
		tea.KeyPressMsg(tea.Key{Text: "r", Code: 'r'}),
		tea.KeyPressMsg(tea.Key{Text: "y", Code: 'y'}),
	} {
		updated, _ := model.Update(key)
		model = updated.(Model)
	}

	view := model.render()
	if !strings.Contains(view, "Edit Story Points") {
		t.Fatalf("filtered palette missing unsupported field:\n%s", view)
	}
	if !strings.Contains(view, "autocomplete") {
		t.Fatalf("filtered palette should describe option source:\n%s", view)
	}
	if strings.Contains(view, "Edit Summary") {
		t.Fatalf("filtered palette should hide non-matching summary action:\n%s", view)
	}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("unsupported field should not submit work")
	}
	if next.actionPaletteOpen {
		t.Fatal("action palette should close after selecting unsupported field")
	}
	if !strings.Contains(next.detailNotice, "Story Points") || !strings.Contains(next.detailNotice, "field-specific workflow") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestDetailActionsMenuStartsGenericEditFieldEditor(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do", Priority: "Medium"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}
	model.editMetadata = map[string]jira.EditMetadata{
		"ABC-1": {
			Fields: []jira.EditField{{
				ID:         "customfield_10016",
				Name:       "Story Points",
				Editable:   true,
				SchemaType: "number",
				Operations: []string{"set"},
			}},
		},
	}
	model.actionFocus = true
	model.selectedAction = detailActionIndexForTest(t, model.detailActions(), "field:customfield_10016")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("cached generic field metadata should not submit work")
	}
	if !next.genericFieldFocus {
		t.Fatal("expected generic field focus")
	}
	if activeKeyContext(next) != keyContextGenericField {
		t.Fatalf("activeKeyContext = %q", activeKeyContext(next))
	}
	view := next.render()
	if !strings.Contains(view, "Edit Story Points") || !strings.Contains(view, "enter save") {
		t.Fatalf("generic field editor did not render:\n%s", view)
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "8", Code: '8'}))
	next = updated.(Model)
	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)

	if cmd == nil {
		t.Fatal("expected generic field update command")
	}
	if !next.genericFieldSubmitting {
		t.Fatal("genericFieldSubmitting should be true")
	}
	if next.genericFieldSubmitKey != "ABC-1" || next.genericFieldSubmitValue.FieldID != "customfield_10016" || next.genericFieldSubmitValue.Text != "8" {
		t.Fatalf("generic submit = %s/%#v", next.genericFieldSubmitKey, next.genericFieldSubmitValue)
	}
}

func TestGenericEditAutocompleteUserFieldSearchesAndSubmitsSelection(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{"ABC-1": {Issue: model.issues[0]}}
	model.editMetadata = map[string]jira.EditMetadata{
		"ABC-1": {
			Fields: []jira.EditField{{
				ID:              "customfield_10030",
				Name:            "Reviewer",
				Editable:        true,
				SchemaType:      "user",
				AutoCompleteURL: "https://example.atlassian.net/rest/api/3/user/picker",
			}},
		},
	}
	model.actionFocus = true
	model.selectedAction = detailActionIndexForTest(t, model.detailActions(), "field:customfield_10030")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("cached generic metadata should open locally")
	}
	if !next.genericFieldFocus || !genericEditFieldUsesAutocomplete(next.genericField) {
		t.Fatalf("generic field focus=%v field=%#v", next.genericFieldFocus, next.genericField)
	}

	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "a", Code: 'a'}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("typing in autocomplete field should request options")
	}
	if !next.genericFieldOptionsLoading || next.genericFieldOptionsQuery != "a" {
		t.Fatalf("options state loading=%v query=%q", next.genericFieldOptionsLoading, next.genericFieldOptionsQuery)
	}

	updated, _ = next.Update(workerResultMsg{result: worker.Result{
		ID:   next.activeGenericFieldOptionsReqID,
		Kind: worker.KindSearchFieldOptions,
		SearchFieldOptions: &worker.SearchFieldOptionsResult{
			FieldID: "customfield_10030",
			Query:   "a",
			Options: []jira.FieldOption{{ID: "account-1", Name: "A Developer"}},
		},
	}})
	next = updated.(Model)
	if next.genericFieldOptionsLoading || len(next.genericField.AllowedValues) != 1 {
		t.Fatalf("options result loading=%v values=%#v", next.genericFieldOptionsLoading, next.genericField.AllowedValues)
	}

	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("saving selected autocomplete value should enqueue update")
	}
	if !next.genericFieldSubmitting || next.genericFieldSubmitValue.Option.ID != "account-1" || next.genericFieldSubmitValue.SchemaType != "user" {
		t.Fatalf("generic submit = submitting=%v value=%#v", next.genericFieldSubmitting, next.genericFieldSubmitValue)
	}
}

func TestDetailActionsMenuStartsSummaryEditor(t *testing.T) {
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
	model.selectedAction = detailActionIndexForTest(t, model.detailActions(), "summary")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("expected summary metadata command from Actions menu")
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
}

func TestDetailActionsMenuStartsPriorityEditor(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do", Priority: "Medium"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}
	model.actionFocus = true
	model.selectedAction = detailActionIndexForTest(t, model.detailActions(), "priority")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("expected priority metadata command from Actions menu")
	}
	if !next.priorityFocus {
		t.Fatal("expected priority focus")
	}
	if !next.priorityMetadataLoading {
		t.Fatal("priorityMetadataLoading should be true")
	}
	if next.priorityMetadataRequestKey != "ABC-1" {
		t.Fatalf("priorityMetadataRequestKey = %q", next.priorityMetadataRequestKey)
	}
}

func TestDetailActionsMenuStartsLabelsEditor(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0], Labels: []string{"platform", "backend"}},
	}
	model.actionFocus = true
	model.selectedAction = detailActionIndexForTest(t, model.detailActions(), "labels")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("expected labels metadata command from Actions menu")
	}
	if !next.labelsFocus {
		t.Fatal("expected labels focus")
	}
	if !next.labelsMetadataLoading {
		t.Fatal("labelsMetadataLoading should be true")
	}
	if next.labelsMetadataRequestKey != "ABC-1" {
		t.Fatalf("labelsMetadataRequestKey = %q", next.labelsMetadataRequestKey)
	}
}

func TestDetailActionsMenuStartsComponentsEditor(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0], Components: []string{"API"}},
	}
	model.actionFocus = true
	model.selectedAction = detailActionIndexForTest(t, model.detailActions(), "components")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("expected components metadata command from Actions menu")
	}
	if !next.componentsFocus {
		t.Fatal("expected components focus")
	}
	if !next.componentsMetadataLoading {
		t.Fatal("componentsMetadataLoading should be true")
	}
	if next.componentsMetadataRequestKey != "ABC-1" {
		t.Fatalf("componentsMetadataRequestKey = %q", next.componentsMetadataRequestKey)
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

func TestDetailActionsMenuStartsIssueLinkEditor(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}
	model.issueLinkTypes = []jira.IssueLinkType{{ID: "10000", Name: "Blocks", Inward: "is blocked by", Outward: "blocks"}}
	model.actionFocus = true
	model.selectedAction = detailActionIndexForTest(t, model.detailActions(), "link-issue")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("issue link editor should use cached link types without submitting work")
	}
	if !next.issueLinkFocus {
		t.Fatal("issueLinkFocus should be true")
	}
	if activeKeyContext(next) != keyContextIssueLink {
		t.Fatalf("activeKeyContext = %q", activeKeyContext(next))
	}
	if !strings.Contains(next.render(), "Link Issue") || !strings.Contains(next.render(), "ABC-1 blocks target") {
		t.Fatalf("missing issue link modal in %q", next.render())
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "J", Code: 'J'}))
	next = updated.(Model)
	if next.issueLinkTargetDraft != "J" {
		t.Fatalf("issueLinkTargetDraft = %q", next.issueLinkTargetDraft)
	}
	if next.selectedIssueLinkRelation != 0 {
		t.Fatalf("typing should not move relation selection: %d", next.selectedIssueLinkRelation)
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "down", Code: tea.KeyDown}))
	next = updated.(Model)
	if next.selectedIssueLinkRelation != 1 {
		t.Fatalf("selectedIssueLinkRelation = %d", next.selectedIssueLinkRelation)
	}

	next.issueLinkTargetDraft = "ABC-2"
	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("issue link submit should enqueue worker request")
	}
	if !next.issueLinkSubmitting {
		t.Fatal("issueLinkSubmitting should be true")
	}
	if next.issueLinkSubmitRequest.SourceKey != "ABC-1" || next.issueLinkSubmitRequest.TargetKey != "ABC-2" || next.issueLinkSubmitRequest.Direction != "inward" {
		t.Fatalf("issueLinkSubmitRequest = %#v", next.issueLinkSubmitRequest)
	}
}

func TestDetailActionsMenuStartsWorklogEditor(t *testing.T) {
	now := time.Date(2026, 6, 19, 9, 30, 0, 0, time.UTC)
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}
	model.actionFocus = true
	model.selectedAction = detailActionIndexForTest(t, model.detailActions(), "log-work")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("worklog editor should open locally")
	}
	if !next.worklogFocus {
		t.Fatal("worklogFocus should be true")
	}
	if activeKeyContext(next) != keyContextWorklog {
		t.Fatalf("activeKeyContext = %q", activeKeyContext(next))
	}
	if !strings.Contains(next.render(), "Log Work") || !strings.Contains(next.render(), "Duration") {
		t.Fatalf("missing worklog modal in %q", next.render())
	}

	next.worklogTimeDraft = "45m"
	next.worklogCommentDraft = "Reviewed ABC-2"
	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("worklog submit should enqueue worker request")
	}
	if !next.worklogSubmitting {
		t.Fatal("worklogSubmitting should be true")
	}
	if next.worklogSubmitKey != "ABC-1" || next.worklogSubmitRequest.TimeSpent != "45m" || next.worklogSubmitRequest.Comment != "Reviewed ABC-2" {
		t.Fatalf("worklog submit = %s/%#v", next.worklogSubmitKey, next.worklogSubmitRequest)
	}
	if !next.worklogSubmitRequest.Started.Equal(now) {
		t.Fatalf("Started = %v", next.worklogSubmitRequest.Started)
	}
}

func TestWorklogSectionSelectsEditsAndDeletesRows(t *testing.T) {
	started := time.Date(2026, 6, 19, 8, 0, 0, 0, time.UTC)
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{"ABC-1": {Issue: model.issues[0]}}
	model.worklogs = map[string][]jira.Worklog{
		"ABC-1": {
			{ID: "10001", TimeSpent: "30m", Comment: "Read context", Started: started},
			{ID: "10002", TimeSpent: "1h", Comment: "Implemented change", Started: started.Add(time.Hour)},
		},
	}
	focusDetailSectionForTest(t, &model, "Worklog")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("worklog row focus should be local")
	}
	if !next.worklogListFocus {
		t.Fatal("worklogListFocus should be true")
	}
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	next = updated.(Model)
	if next.selectedWorklog != 1 {
		t.Fatalf("selectedWorklog = %d", next.selectedWorklog)
	}

	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "e", Code: 'e'}))
	next = updated.(Model)
	if cmd != nil {
		t.Fatal("worklog edit dialog should open locally")
	}
	if !next.worklogFocus || !next.worklogEditing || next.worklogUpdateRequest.ID != "10002" {
		t.Fatalf("edit state focus=%v editing=%v request=%#v", next.worklogFocus, next.worklogEditing, next.worklogUpdateRequest)
	}
	next.worklogTimeDraft = "2h"
	next.worklogCommentDraft = "Updated implementation"
	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("worklog update should enqueue worker request")
	}
	if !next.worklogSubmitting || next.activeUpdateWorklogReqID == 0 || next.worklogUpdateRequest.TimeSpent != "2h" {
		t.Fatalf("update state submitting=%v req=%d request=%#v", next.worklogSubmitting, next.activeUpdateWorklogReqID, next.worklogUpdateRequest)
	}

	model.worklogListFocus = true
	model.selectedWorklog = 1
	updated, cmd = model.Update(tea.KeyPressMsg(tea.Key{Text: "d", Code: 'd'}))
	next = updated.(Model)
	if cmd != nil {
		t.Fatal("worklog delete confirmation should open locally")
	}
	if !next.worklogDeleteConfirm || next.worklogDeleteID != "10002" {
		t.Fatalf("delete state confirm=%v id=%q", next.worklogDeleteConfirm, next.worklogDeleteID)
	}
	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("worklog delete should enqueue worker request")
	}
	if !next.worklogDeleteSubmitting || next.activeDeleteWorklogReqID == 0 {
		t.Fatalf("delete submit state submitting=%v req=%d", next.worklogDeleteSubmitting, next.activeDeleteWorklogReqID)
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

func actionPaletteIndexForTest(t *testing.T, actions []actionPaletteAction, id string) int {
	t.Helper()
	for index, action := range actions {
		if action.Action.ID == id {
			return index
		}
	}
	t.Fatalf("missing action palette action %q in %#v", id, actions)
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
	model.detailFocus = indexOfDetailTargetForTest(model, "status")

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
	model.detailFocus = indexOfDetailTargetForTest(model, "status")

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
	model.detailFocus = indexOfDetailTargetForTest(model, "status")

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
	model.detailFocus = indexOfDetailTargetForTest(model, "status")

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
	model.detailFocus = indexOfDetailTargetForTest(model, "status")

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

func TestStatusTransitionBlocksUnsupportedRequiredFields(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.transitions = map[string][]jira.Transition{
		"ABC-1": {
			{
				ID:       "31",
				Name:     "Done",
				ToStatus: "Done",
				Fields: []jira.TransitionField{
					{ID: "customfield_10010", Name: "Asset", Required: true, SchemaType: "object"},
				},
			},
		},
	}
	model.transitionFocus = true
	model.detailFocus = indexOfDetailTargetForTest(model, "status")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("unsupported required fields should not submit work")
	}
	if next.transitionSubmitting {
		t.Fatal("transitionSubmitting should be false")
	}
	if !strings.Contains(next.detailNotice, "Asset") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestStatusTransitionFieldFormSubmitsCustomOptionField(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.transitions = map[string][]jira.Transition{
		"ABC-1": {
			{
				ID:       "31",
				Name:     "Done",
				ToStatus: "Done",
				Fields: []jira.TransitionField{
					{
						ID:         "customfield_10010",
						Name:       "Deployment Environment",
						Required:   true,
						SchemaType: "option",
						AllowedValues: []jira.FieldOption{
							{ID: "20001", Name: "Staging"},
							{ID: "20002", Name: "Production"},
						},
					},
				},
			},
		},
	}
	model.transitionFocus = true
	model.detailFocus = indexOfDetailTargetForTest(model, "status")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("required option field should open the field form before submitting")
	}
	if !next.transitionFieldEditing {
		t.Fatal("transition field form should open")
	}
	if view := next.render(); !strings.Contains(view, "Deployment Environment") || !strings.Contains(view, "not selected") {
		t.Fatalf("missing custom option field in form:\n%s", view)
	}

	next.transitionFieldSelections["customfield_10010"] = 1
	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)

	if cmd == nil {
		t.Fatal("expected transition submit command")
	}
	if len(next.transitionSubmitFields) != 1 {
		t.Fatalf("transitionSubmitFields = %#v", next.transitionSubmitFields)
	}
	field := next.transitionSubmitFields[0]
	if field.FieldID != "customfield_10010" || field.SchemaType != "option" || field.Option.Name != "Production" {
		t.Fatalf("transition field = %#v", field)
	}
}

func TestStatusTransitionFieldFormSubmitsTextDateUserAndMultiSelectFields(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 110
	model.height = 36
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.transitions = map[string][]jira.Transition{
		"ABC-1": {
			{
				ID:       "31",
				Name:     "Done",
				ToStatus: "Done",
				Fields: []jira.TransitionField{
					{ID: "customfield_10020", Name: "Root Cause", Required: true, SchemaType: "string"},
					{ID: "customfield_10021", Name: "Target Date", Required: true, SchemaType: "date"},
					{
						ID:         "customfield_10022",
						Name:       "Reviewer",
						Required:   true,
						SchemaType: "user",
						AllowedValues: []jira.FieldOption{
							{ID: "abc-123", Name: "Jane Doe"},
						},
					},
					{
						ID:          "customfield_10023",
						Name:        "Impacted Areas",
						Required:    true,
						SchemaType:  "array",
						SchemaItems: "option",
						AllowedValues: []jira.FieldOption{
							{ID: "1", Name: "Backend"},
							{ID: "2", Name: "Frontend"},
						},
					},
				},
			},
		},
	}
	model.transitionFocus = true
	model.detailFocus = indexOfDetailTargetForTest(model, "status")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("required fields should open the field form before submitting")
	}
	if !next.transitionFieldEditing {
		t.Fatal("transition field form should open")
	}
	view := next.render()
	for _, want := range []string{"Root Cause", "Target Date", "Reviewer", "Impacted Areas"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in transition form:\n%s", want, view)
		}
	}

	next.transitionFieldDrafts["customfield_10020"] = "Root cause text"
	next.transitionFieldDrafts["customfield_10021"] = "2026-06-20"
	next.transitionFieldSelections["customfield_10022"] = 0
	next.transitionFieldMultiSelections["customfield_10023"] = map[int]bool{0: true, 1: true}
	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)

	if cmd == nil {
		t.Fatal("expected transition submit command")
	}
	if len(next.transitionSubmitFields) != 4 {
		t.Fatalf("transitionSubmitFields = %#v", next.transitionSubmitFields)
	}
	gotByField := make(map[string]jira.TransitionFieldValue, len(next.transitionSubmitFields))
	for _, field := range next.transitionSubmitFields {
		gotByField[field.FieldID] = field
	}
	if gotByField["customfield_10020"].Text != "Root cause text" {
		t.Fatalf("text field = %#v", gotByField["customfield_10020"])
	}
	if gotByField["customfield_10021"].Text != "2026-06-20" {
		t.Fatalf("date field = %#v", gotByField["customfield_10021"])
	}
	if gotByField["customfield_10022"].Option.ID != "abc-123" {
		t.Fatalf("user field = %#v", gotByField["customfield_10022"])
	}
	if len(gotByField["customfield_10023"].Options) != 2 {
		t.Fatalf("multi-select field = %#v", gotByField["customfield_10023"])
	}
}

func TestStatusTransitionFieldAutocompleteLoadsOptions(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 110
	model.height = 36
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.transitions = map[string][]jira.Transition{
		"ABC-1": {
			{
				ID:       "31",
				Name:     "Done",
				ToStatus: "Done",
				Fields: []jira.TransitionField{
					{
						ID:              "customfield_10022",
						Name:            "Reviewer",
						Required:        true,
						SchemaType:      "user",
						AutoCompleteURL: "https://example.atlassian.net/rest/api/3/user/picker?fieldName=customfield_10022",
					},
				},
			},
		},
	}
	model.transitionFocus = true
	model.detailFocus = indexOfDetailTargetForTest(model, "status")

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	next = updated.(Model)

	if cmd == nil {
		t.Fatal("expected transition autocomplete option request command")
	}
	if !next.transitionFieldOptionsLoading["customfield_10022"] {
		t.Fatalf("transitionFieldOptionsLoading = %#v", next.transitionFieldOptionsLoading)
	}
	if next.transitionFieldOptionsQuery["customfield_10022"] != "j" {
		t.Fatalf("transitionFieldOptionsQuery = %#v", next.transitionFieldOptionsQuery)
	}
	if !strings.Contains(next.render(), "Loading Jira options") {
		t.Fatalf("expected loading state:\n%s", next.render())
	}

	updated, _ = next.Update(workerResultMsg{result: worker.Result{
		ID:   next.activeTransitionFieldOptionsReqID,
		Kind: worker.KindSearchFieldOptions,
		SearchFieldOptions: &worker.SearchFieldOptionsResult{
			FieldID: "customfield_10022",
			Query:   "j",
			Options: []jira.FieldOption{{ID: "abc-123", Name: "Jane Doe"}, {ID: "def-456", Name: "John Doe"}},
		},
	}})
	next = updated.(Model)
	if next.transitionFieldOptionsLoading["customfield_10022"] {
		t.Fatalf("transitionFieldOptionsLoading = %#v", next.transitionFieldOptionsLoading)
	}
	transition := next.transitions["ABC-1"][0]
	if len(transition.Fields[0].AllowedValues) != 2 {
		t.Fatalf("AllowedValues = %#v", transition.Fields[0].AllowedValues)
	}
	if next.transitionFieldSelections["customfield_10022"] != 0 {
		t.Fatalf("selection = %d", next.transitionFieldSelections["customfield_10022"])
	}
	view := next.render()
	if !strings.Contains(view, "Jane Doe") || !strings.Contains(view, "John Doe") {
		t.Fatalf("expected autocomplete options in transition picker:\n%s", view)
	}
}

func TestStatusTransitionFieldFormSubmitsResolutionAndComment(t *testing.T) {
	searcher := &fakeIssueSearcher{}
	model := NewModel(searcher, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.transitions = map[string][]jira.Transition{
		"ABC-1": {
			{
				ID:       "31",
				Name:     "Done",
				ToStatus: "Done",
				Fields: []jira.TransitionField{
					{
						ID:       "resolution",
						Name:     "Resolution",
						Required: true,
						AllowedValues: []jira.FieldOption{
							{ID: "10000", Name: "Done"},
							{ID: "10001", Name: "Won't Do"},
						},
					},
					{ID: "comment", Name: "Comment", Required: true},
				},
			},
		},
	}
	model.transitionFocus = true
	model.detailFocus = indexOfDetailTargetForTest(model, "status")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("required fields should open the local transition field form before submitting")
	}
	if !next.transitionFieldEditing {
		t.Fatal("transition field form should open")
	}
	if view := next.render(); !strings.Contains(view, "Required Fields") || !strings.Contains(view, "Resolution") || !strings.Contains(view, "Comment") {
		t.Fatalf("missing field form in %q", view)
	}

	next.transitionFieldSelections["resolution"] = 0
	next.transitionFieldDrafts["comment"] = "Ship **this** now"
	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)

	if cmd == nil {
		t.Fatal("expected transition submit command")
	}
	if !next.transitionSubmitting {
		t.Fatal("transitionSubmitting should be true")
	}
	if len(next.transitionSubmitFields) != 2 {
		t.Fatalf("transitionSubmitFields = %#v", next.transitionSubmitFields)
	}
	_ = cmd()
	select {
	case result := <-next.workers.Results():
		updated, _ = next.Update(workerResultMsg{result: result})
		next = updated.(Model)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for transition worker result")
	}
	if searcher.transitionRequest.TransitionID != "31" {
		t.Fatalf("transition request = %#v", searcher.transitionRequest)
	}
	if len(searcher.transitionRequest.Fields) != 2 {
		t.Fatalf("transition request fields = %#v", searcher.transitionRequest.Fields)
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
	model.detailFocus = indexOfDetailTargetForTest(model, "assignee")

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
			Query:    "j",
			IssueKey: "ABC-1",
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
	model.cacheAssignableUserSearch("ABC-1", "jon", []jira.User{{AccountID: "abc-123", DisplayName: "Jon Charette"}})

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

func TestAssigneePickerDoesNotUseGlobalUserSearchCache(t *testing.T) {
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

	if cmd == nil {
		t.Fatal("global user cache should not satisfy issue-scoped assignee search")
	}
	if !next.assigneeSearchLoading {
		t.Fatal("assigneeSearchLoading should be true")
	}
}

func TestAssigneePickerUsesIssueScopedAssignableSearch(t *testing.T) {
	searcher := &fakeIssueSearcher{
		assignableUsers: []jira.User{{AccountID: "def-456", DisplayName: "John Doe"}},
	}
	model := NewModel(searcher, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do", Assignee: "Jane Doe"}}
	model.assigneeFocus = true
	model.assigneeQuery = "Jo"

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "h", Code: 'h'}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("expected assignable user search command")
	}
	if msg := cmd(); msg == nil {
		t.Fatal("expected work submitted message")
	}
	resultMsg := next.waitForWorkerResult()()
	result, ok := resultMsg.(workerResultMsg)
	if !ok {
		t.Fatalf("worker result = %#v", resultMsg)
	}
	updated, _ = next.Update(result)
	next = updated.(Model)

	if searcher.assignableIssueKey != "ABC-1" || searcher.assignableQuery != "Joh" {
		t.Fatalf("assignable search = issue %q query %q", searcher.assignableIssueKey, searcher.assignableQuery)
	}
	if len(next.assigneeUsers) != 1 || next.assigneeUsers[0].AccountID != "def-456" {
		t.Fatalf("assigneeUsers = %#v", next.assigneeUsers)
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

func TestLabelsEditorSubmitsWorkerBackedUpdate(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0], Labels: []string{"backend"}},
	}
	model.labelsFocus = true
	model.labelsEditing = true
	model.labelsDirty = true
	model.labelsDraft = "platform, backend, needs-review"

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("expected labels submit command")
	}
	if !next.labelsSubmitting {
		t.Fatal("labelsSubmitting should be true")
	}
	if next.labelsSubmitKey != "ABC-1" {
		t.Fatalf("labelsSubmitKey = %q", next.labelsSubmitKey)
	}
	want := []string{"platform", "backend", "needs-review"}
	if !labelsEqual(next.labelsSubmitValue, want) {
		t.Fatalf("labelsSubmitValue = %#v", next.labelsSubmitValue)
	}
	if !strings.Contains(next.detailNotice, "Updating labels") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestLabelsEditorUnchangedDoesNotSubmit(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0], Labels: []string{"backend", "platform"}},
	}
	model.labelsFocus = true
	model.labelsEditing = true
	model.labelsDirty = true
	model.labelsDraft = "platform, backend"

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("unchanged labels should not submit")
	}
	if !next.labelsEditing {
		t.Fatal("unchanged labels should keep editor open")
	}
	if !strings.Contains(next.detailNotice, "Labels unchanged") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestLabelsUpdateSuccessUpdatesIssueDetailLabels(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0], Labels: []string{"backend"}},
	}
	model.activeLabelsReqID = 71
	model.labelsSubmitting = true
	model.labelsEditing = true
	model.labelsSubmitKey = "ABC-1"

	updated, _ := model.Update(workerResultMsg{result: worker.Result{
		ID:   71,
		Kind: worker.KindUpdateLabels,
		UpdateLabels: &worker.UpdateLabelsResult{
			Key:      "ABC-1",
			Labels:   []string{"platform", "backend"},
			SyncedAt: time.Now(),
		},
	}})
	next := updated.(Model)

	if next.labelsSubmitting {
		t.Fatal("labelsSubmitting should be false")
	}
	if next.labelsEditing {
		t.Fatal("labelsEditing should be false")
	}
	if !labelsEqual(next.details["ABC-1"].Labels, []string{"platform", "backend"}) {
		t.Fatalf("detail labels = %#v", next.details["ABC-1"].Labels)
	}
	if !strings.Contains(next.detailNotice, "Labels updated") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestComponentsPickerTogglesAndSubmitsWorkerBackedUpdate(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0], Components: []string{"API"}},
	}
	model.editMetadata = map[string]jira.EditMetadata{
		"ABC-1": {
			Components: jira.EditField{
				ID:       "components",
				Name:     "Components",
				Editable: true,
				AllowedValues: []jira.FieldOption{
					{ID: "101", Name: "Platform"},
					{ID: "102", Name: "API"},
				},
			},
		},
	}
	model = model.beginComponentsEditing(model.editMetadata["ABC-1"])

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: " ", Code: tea.KeySpace}))
	next := updated.(Model)
	if !next.componentsDirty {
		t.Fatal("componentsDirty should be true after toggling")
	}
	if !next.selectedComponents["101"] {
		t.Fatalf("selectedComponents = %#v", next.selectedComponents)
	}
	view := next.render()
	if !strings.Contains(view, "Platform") || !strings.Contains(view, "Selected: Platform, API") {
		t.Fatalf("components picker view missing selection:\n%s", view)
	}

	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected components submit command")
	}
	if !next.componentsSubmitting {
		t.Fatal("componentsSubmitting should be true")
	}
	if next.componentsSubmitKey != "ABC-1" {
		t.Fatalf("componentsSubmitKey = %q", next.componentsSubmitKey)
	}
	if len(next.componentsSubmitValue) != 2 {
		t.Fatalf("componentsSubmitValue = %#v", next.componentsSubmitValue)
	}
}

func TestComponentsUpdateSuccessUpdatesIssueDetailComponents(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0], Components: []string{"API"}},
	}
	model.activeComponentsReqID = 81
	model.componentsSubmitting = true
	model.componentsFocus = true
	model.componentsSubmitKey = "ABC-1"

	updated, _ := model.Update(workerResultMsg{result: worker.Result{
		ID:   81,
		Kind: worker.KindUpdateComponents,
		UpdateComponents: &worker.UpdateComponentsResult{
			Key: "ABC-1",
			Components: []jira.FieldOption{
				{ID: "101", Name: "Platform"},
				{ID: "102", Name: "API"},
			},
			SyncedAt: time.Now(),
		},
	}})
	next := updated.(Model)

	if next.componentsSubmitting {
		t.Fatal("componentsSubmitting should be false")
	}
	if next.componentsFocus {
		t.Fatal("componentsFocus should be false")
	}
	if !labelsEqual(next.details["ABC-1"].Components, []string{"Platform", "API"}) {
		t.Fatalf("detail components = %#v", next.details["ABC-1"].Components)
	}
	if !strings.Contains(next.detailNotice, "Components updated") {
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

func TestSprintActionListsActiveAndFutureSprints(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithDefaultBoardID(100))
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do", Priority: "Medium"}}
	model.details = map[string]jira.IssueDetail{"ABC-1": {Issue: model.issues[0]}}
	model.planningBoards = []jira.Board{{ID: 100, Name: "ABC Scrum", Type: "scrum"}}
	model.planningBoardID = 100
	model.planningSprints = map[int][]jira.Sprint{
		100: {
			{ID: 300, BoardID: 100, Name: "Platform Sprint 24", State: "active"},
			{ID: 301, BoardID: 100, Name: "Platform Sprint 25", State: "future"},
		},
	}
	model.actionFocus = true
	model.selectedAction = detailActionIndexForTest(t, model.detailActions(), "sprint")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("opening sprint picker should not submit work")
	}
	if !next.sprintFocus {
		t.Fatal("expected sprint picker focus")
	}
	view := next.render()
	for _, want := range []string{"Sprint", "Platform Sprint 24", "active", "Platform Sprint 25", "future"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in:\n%s", want, view)
		}
	}
	if activeKeyContext(next) != keyContextSprint {
		t.Fatalf("activeKeyContext = %q", activeKeyContext(next))
	}
}

func TestSprintActionSubmitsSelectedSprintMove(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithDefaultBoardID(100))
	defer model.workers.Stop()
	model.mode = modeDetail
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story"}}
	model.planningBoardID = 100
	model.planningSprints = map[int][]jira.Sprint{
		100: {{ID: 300, BoardID: 100, Name: "Platform Sprint 24", State: "active"}},
	}
	model.sprintFocus = true

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if cmd == nil {
		t.Fatal("expected sprint move worker command")
	}
	if !next.sprintSubmitting || next.sprintSubmitKey != "ABC-1" || next.sprintSubmit.ID != 300 {
		t.Fatalf("sprint submit state submitting=%v key=%q sprint=%#v", next.sprintSubmitting, next.sprintSubmitKey, next.sprintSubmit)
	}
}
