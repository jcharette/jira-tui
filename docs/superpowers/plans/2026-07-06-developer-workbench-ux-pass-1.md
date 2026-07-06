# Developer Workbench UX Pass 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the selected-ticket developer loop visible from ticket detail without adding new Jira, Git, GitHub, or Claude write paths.

**Architecture:** Reuse the existing Bubble Tea detail model, `detailSections()`, `detailActions()`, action palette, Claude section, comment composer, worklog editor, and start-work workflow. Add one read-only Developer Workbench detail section and shared action grouping/order helpers; do not change confirmation/write behavior.

**Tech Stack:** Go, Bubble Tea v2, Lip Gloss, existing `internal/tui` snapshot harness.

## Global Constraints

- Use JiraTUI as a UX benchmark for discoverability and cockpit-style organization, but do not clone its product.
- Our niche remains Jira plus Git, GitHub, Claude, diagnostics, and safe write previews.
- Do not rebuild the TUI framework or layout engine.
- Do not add provider-neutral AI execution in this pass.
- Do not replace existing worker, cache, Jira metadata, or confirmation paths.
- Existing write gates remain unchanged.
- Existing keyboard shortcuts keep working.

---

### Task 1: Add A Read-Only Developer Workbench Detail Section

**Files:**
- Modify: `internal/tui/detail.go`
- Test: `internal/tui/detail_test.go`

**Interfaces:**
- Consumes: `Model.detailSections() []detailSection`, `Model.renderDetailSection(section detailSection, ctx detailRenderContext, width int) string`, `Model.detailSectionHeader(id, fallback, help string, width int) string`, `Model.detailEmptyState(message string, width int) string`, `Model.claudeAvailable() bool`
- Produces: `Model.renderDeveloperWorkbenchSection(ctx detailRenderContext, width int) string`

- [ ] **Step 1: Write the failing detail-section test**

Add this test near the other detail section rendering tests in `internal/tui/detail_test.go`:

```go
func TestDeveloperWorkbenchSectionShowsDeveloperLoopActions(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{
			Enabled:      true,
			TicketPlan:   true,
			TicketAssist: true,
			DraftComment: true,
			BranchPlan:   true,
		}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude", Version: "test"}),
	)
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 120
	model.height = 34
	model.issues = []jira.Issue{{
		Key:       "ABC-1",
		Summary:   "Tighten deployment review flow",
		Status:    "In Progress",
		Priority:  "High",
		Assignee:  "Jon",
		IssueType: "Story",
	}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       model.issues[0],
			Description: "Make generated text, local edits, and Jira writes easy to distinguish.",
			Reporter:    "Rae",
		},
	}
	model.comments = map[string][]jira.Comment{
		"ABC-1": {{ID: "10001", Author: "Rae", Body: "Please make write gates obvious."}},
	}
	model.worklogs = map[string][]jira.Worklog{
		"ABC-1": {{ID: "20001", Author: "Jon", TimeSpent: "45m", Comment: "UX review"}},
	}

	sections := model.detailSections()
	if len(sections) < 2 || sections[1].ID != "workbench" {
		t.Fatalf("sections = %#v, want workbench immediately after overview", sections)
	}

	model.jumpDetailSection("Workbench")
	view := model.render()
	for _, want := range []string{
		"Developer Workbench",
		"Start Work",
		"Claude Plan",
		"Quality Review",
		"Draft Comment",
		"Add Comment",
		"Log Work",
		"Open Jira",
		"Copy Key",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("workbench missing %q in:\n%s", want, view)
		}
	}
	for _, unwanted := range []string{"Edit Components", "Set Fix Version", "metadata"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("workbench should not show low-frequency metadata action %q in:\n%s", unwanted, view)
		}
	}
}
```

Also update the existing `TestDetailSectionsUseOverviewFirstWithoutStatusTab` expectation:

```go
wantPrefix := []string{"overview", "workbench", "comments", "worklog", "hierarchy"}
```

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `go test ./internal/tui -run TestDeveloperWorkbenchSectionShowsDeveloperLoopActions -count=1`

Expected: FAIL because the workbench section does not exist.

- [ ] **Step 3: Add the workbench section**

In `internal/tui/detail.go`, update `renderDetailSection`:

```go
case "workbench":
	return m.renderDeveloperWorkbenchSection(ctx, width)
```

In `detailSections()`, insert Workbench immediately after Overview:

```go
sections := []detailSection{
	{ID: "overview", Label: "Overview", Short: "Over"},
	{ID: "workbench", Label: "Workbench", Short: "Dev"},
	{ID: "comments", Label: "Comments", Short: "Com"},
	{ID: "worklog", Label: "Worklog", Short: "Work"},
	{ID: "hierarchy", Label: "Hierarchy", Short: "Tree"},
}
```

Then adjust the existing badge indexes in `detailSections()` because Comments, Worklog, and Hierarchy move from indexes `1`, `2`, `3` to `2`, `3`, `4`:

```go
sections[2].Badge = fmt.Sprintf("%d", len(comments))
sections[2].Badge = "..."
sections[2].Badge = "!"
sections[3].Badge = fmt.Sprintf("%d", len(worklogs))
sections[3].Badge = "..."
sections[3].Badge = "!"
sections[4].Badge = fmt.Sprintf("%d", childCount)
sections = append(sections[:5], append([]detailSection{links}, sections[5:]...)...)
```

- [ ] **Step 4: Add the minimal read-only renderer**

Add this helper near `renderOverviewSection` in `internal/tui/detail.go`:

```go
func (m Model) renderDeveloperWorkbenchSection(ctx detailRenderContext, width int) string {
	lines := []string{m.detailSectionHeader("workbench", "Developer Workbench", "next actions", width), ""}
	rows := [][]string{
		{m.theme.FieldLabel.Render("Start Work"), m.theme.Success.Render("ready"), m.theme.Muted.Render("Ticket Actions -> Start Work creates/switches branch after review.")},
		{m.theme.FieldLabel.Render("Claude Plan"), m.claudeWorkbenchState(), m.theme.Muted.Render("Open Claude actions for a read-only implementation plan.")},
		{m.theme.FieldLabel.Render("Quality Review"), m.claudeWorkbenchState(), m.theme.Muted.Render("Ask Claude for readiness gaps before writing Jira.")},
		{m.theme.FieldLabel.Render("Draft Comment"), m.claudeWorkbenchState(), m.theme.Muted.Render("Draft an editable Jira comment before post confirmation.")},
		{m.theme.FieldLabel.Render("Add Comment"), m.theme.Success.Render("ready"), m.theme.Muted.Render("Write or refine a Jira comment, then review before posting.")},
		{m.theme.FieldLabel.Render("Log Work"), m.theme.Success.Render("ready"), m.theme.Muted.Render("Add a Jira worklog entry for this ticket.")},
		{m.theme.FieldLabel.Render("Open Jira"), m.theme.Success.Render("ready"), m.theme.Muted.Render("Open the issue in the browser.")},
		{m.theme.FieldLabel.Render("Copy Key"), m.theme.Success.Render("ready"), m.theme.Muted.Render("Copy the ticket key for branch names, commits, and notes.")},
	}
	lines = append(lines, m.detailTable(0, []string{"ACTION", "STATE", "DETAIL"}, rows, nil))
	lines = append(lines, "", m.detailEmptyState("Use . for Ticket Actions, a for Claude actions, or tab to move through ticket sections.", width))
	return strings.Join(lines, "\n")
}

func (m Model) claudeWorkbenchState() string {
	if m.claudeAvailable() {
		return m.theme.Success.Render("ready")
	}
	return m.theme.Muted.Render("disabled")
}
```

If `detailTable` width is cramped, shorten the detail strings rather than adding wrapping code.

- [ ] **Step 5: Run the focused test to verify it passes**

Run: `go test ./internal/tui -run TestDeveloperWorkbenchSectionShowsDeveloperLoopActions -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/detail.go internal/tui/detail_test.go
git commit -m "feat: add developer workbench detail section"
```

---

### Task 2: Share Developer-Oriented Action Ordering And Groups

**Files:**
- Modify: `internal/tui/detail.go`
- Modify: `internal/tui/action_palette.go`
- Test: `internal/tui/detail_test.go`

**Interfaces:**
- Consumes: `detailAction`, `Model.detailActions() []detailAction`, `actionPaletteGroup(id string) string`
- Produces: `detailActionGroup(id string) string`

- [ ] **Step 1: Write the failing ordering/group test**

Add this test near existing action palette tests in `internal/tui/detail_test.go`:

```go
func TestDetailActionsPrioritizeDeveloperWorkflowGroups(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.mode = modeDetail
	model.width = 120
	model.height = 34
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Status: "To Do", Priority: "Medium"}}
	model.details = map[string]jira.IssueDetail{"ABC-1": {Issue: model.issues[0]}}

	actions := model.detailActions()
	gotIDs := make([]string, 0, 8)
	for index := 0; index < min(8, len(actions)); index++ {
		gotIDs = append(gotIDs, actions[index].ID)
	}
	wantIDs := []string{"start-work", "comment", "log-work", "browser", "copy-key", "copy-url", "transition", "assign"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("first action IDs = %#v, want %#v", gotIDs, wantIDs)
	}

	groups := map[string]string{}
	for _, action := range actions {
		groups[action.ID] = detailActionGroup(action.ID)
	}
	for id, want := range map[string]string{
		"start-work": "Developer",
		"comment":    "Developer",
		"log-work":   "Developer",
		"browser":    "Open/Copy",
		"copy-key":   "Open/Copy",
		"transition": "Jira",
		"summary":    "Jira Field",
		"subtask":    "Create",
	} {
		if got := groups[id]; got != want {
			t.Fatalf("detailActionGroup(%q) = %q, want %q", id, got, want)
		}
	}
}
```

If `reflect` is not already imported in `detail_test.go`, add it to the existing import block.

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `go test ./internal/tui -run TestDetailActionsPrioritizeDeveloperWorkflowGroups -count=1`

Expected: FAIL because `detailActionGroup` does not exist and `log-work` is too low in the ordering.

- [ ] **Step 3: Reorder `detailActions()`**

In `internal/tui/detail.go`, reorder the fixed actions to put developer-loop actions first:

```go
actions := []detailAction{
	{ID: "start-work", Label: "Start Work", Description: "Choose repo, create branch, and apply confirmed Jira updates.", Enabled: true},
	{ID: "comment", Label: "Add Comment", Description: "Write or refine a Jira comment before review and post.", Enabled: true},
	{ID: "log-work", Label: "Log Work", Description: "Add a Jira worklog entry.", Enabled: true},
	{ID: "browser", Label: "Open In Browser", Description: "Open this ticket in Jira.", Enabled: true},
	{ID: "copy-key", Label: "Copy Key", Description: "Copy the ticket key.", Enabled: true},
	{ID: "copy-url", Label: "Copy URL", Description: "Copy the Jira URL.", Enabled: true},
	{ID: "transition", Label: "Transition Status", Description: "Load available Jira transitions and change status.", Enabled: true},
	{ID: "assign", Label: "Assign", Description: "Search assignable Jira users and change assignee.", Enabled: true},
	{ID: "summary", Label: "Edit Summary", Description: "Load Jira edit metadata and update summary.", Enabled: true},
	{ID: "priority", Label: "Change Priority", Description: "Load Jira priority options and update priority.", Enabled: true},
	{ID: "labels", Label: "Edit Labels", Description: "Edit comma-separated Jira labels.", Enabled: true},
	{ID: "components", Label: "Edit Components", Description: "Select Jira components from edit metadata.", Enabled: true},
	{ID: "sprint", Label: "Sprint", Description: "Add this ticket to an active or future Jira sprint.", Enabled: true},
	{ID: "link-issue", Label: "Link Issue", Description: "Create a Jira issue link to another ticket.", Enabled: true},
	{ID: "subtask", Label: "Create Subtask", Description: "Use Jira create metadata for required fields.", Enabled: true},
}
```

- [ ] **Step 4: Add shared grouping helper and use it in the action palette**

In `internal/tui/detail.go`, add:

```go
func detailActionGroup(id string) string {
	if strings.HasPrefix(id, "field:") {
		return "Jira Field"
	}
	switch id {
	case "start-work", "comment", "log-work":
		return "Developer"
	case "browser", "copy-key", "copy-url":
		return "Open/Copy"
	case "transition", "assign", "sprint", "link-issue":
		return "Jira"
	case "summary", "priority", "labels", "components":
		return "Jira Field"
	case "subtask":
		return "Create"
	default:
		return "Action"
	}
}
```

In `internal/tui/action_palette.go`, replace `group := actionPaletteGroup(action.ID)` with:

```go
group := detailActionGroup(action.ID)
```

Then delete the old `actionPaletteGroup` function.

- [ ] **Step 5: Update the Actions section table to show groups**

In `renderActionsSection`, replace the `state` variable with `group := detailActionGroup(action.ID)`, keep disabled actions showing `DisabledState`, and change the headers:

```go
rows = append(rows, []string{
	labelStyle.Render(marker),
	groupStyle.Render(group),
	labelStyle.Render(action.Label),
	descStyle.Render(truncate(action.Description, max(16, width-48))),
})
...
lines = append(lines, m.detailTable(0, []string{"", "GROUP", "ACTION", "DETAIL"}, rows, nil))
```

Use `groupStyle := m.theme.FieldLabel` for enabled rows and `groupStyle = m.theme.Muted` for disabled rows.

- [ ] **Step 6: Run focused action tests**

Run: `go test ./internal/tui -run 'TestDetailActionsPrioritizeDeveloperWorkflowGroups|TestActionPaletteOpensAndFiltersDetailActions|TestDetailActionsIncludeMetadataBackedFields' -count=1`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/tui/detail.go internal/tui/action_palette.go internal/tui/detail_test.go
git commit -m "feat: prioritize developer ticket actions"
```

---

### Task 3: Update Docs And UX Snapshots

**Files:**
- Modify: `internal/tui/ux_snapshot_test.go`
- Modify: `internal/tui/testdata/ux_ticket_detail.golden`
- Modify: `internal/tui/testdata/ux_action_palette.golden`
- Modify: `docs/keyboard.md`
- Modify: `docs/workflows.md`
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

**Interfaces:**
- Consumes: `TestUXSnapshots`, `uxSnapshotDetailModel`, `uxSnapshotActionPaletteModel`
- Produces: updated golden snapshots for the Developer Workbench section and grouped Ticket Actions.

- [ ] **Step 1: Update the UX snapshot fixture to show the Workbench section**

In `internal/tui/ux_snapshot_test.go`, update `uxSnapshotDetailModel`:

```go
func uxSnapshotDetailModel(t *testing.T) Model {
	t.Helper()
	model := uxSnapshotBaseModel(t)
	model.mode = modeDetail
	model.jumpDetailSection("Workbench")
	return model
}
```

Leave `uxSnapshotClaudeSectionModel` as the dedicated Claude section snapshot.

- [ ] **Step 2: Refresh golden snapshots**

Run: `/bin/zsh -lc "UPDATE_GOLDEN=1 go test ./internal/tui -run TestUXSnapshots -count=1"`

Expected: PASS and changes to `ux_ticket_detail.golden` and `ux_action_palette.golden`.

- [ ] **Step 3: Verify the snapshots without update mode**

Run: `go test ./internal/tui -run TestUXSnapshots -count=1`

Expected: PASS.

- [ ] **Step 4: Update keyboard docs**

In `docs/keyboard.md`, update the Ticket Detail section to mention the new Workbench section and grouped action palette:

```markdown
| `tab` / `shift+tab` | Move through fields and sections, including the Developer Workbench |
| `.` | Open grouped Ticket Actions |
```

Add this paragraph after the Ticket Detail table:

```markdown
The Developer Workbench section groups the daily ticket loop: Start Work, Claude planning/review
when enabled, comments, worklogs, Jira open/copy actions, and the safest next Jira workflow actions.
Ticket Actions uses the same developer-first grouping before lower-frequency Jira metadata edits.
```

- [ ] **Step 5: Update workflow docs**

In `docs/workflows.md`, update "Open And Edit A Ticket" with:

```markdown
The Workbench section is the fastest orientation point for developer work. It keeps Start Work,
Claude planning/review, comments, worklogs, Jira open/copy actions, and workflow updates together
without bypassing the existing review prompts.
```

In "Start Work From A Ticket", add:

```markdown
The same entry point appears in the Developer Workbench section and grouped Ticket Actions.
```

- [ ] **Step 6: Update project state and changelog**

In `docs/project-state.md`, add one sentence to the TUI navigation paragraph:

```markdown
Ticket detail now includes a Developer Workbench section that surfaces the Jira/Git/Claude daily
loop before lower-frequency metadata edits.
```

In `docs/releases/CHANGELOG.md`, under `## Unreleased`, add:

```markdown
- Added a Developer Workbench detail section and developer-first Ticket Actions grouping so Start
  Work, Claude planning/review, comments, worklogs, and Jira open/copy actions are easier to find.
```

- [ ] **Step 7: Update task tracking**

In `tasks/todo.md`, mark the Pass 1 plan/implementation items according to actual progress:

```markdown
- [x] Write the implementation plan for Pass 1: Developer Flow Polish.
- [x] Implement Pass 1 in small verified slices.
```

Keep Pass 2/3 reassessment unchecked until this pass is reviewed.

- [ ] **Step 8: Run release-grade local verification**

Run each command:

```bash
go test ./internal/tui -run 'TestDeveloperWorkbenchSectionShowsDeveloperLoopActions|TestDetailActionsPrioritizeDeveloperWorkflowGroups|TestUXSnapshots' -count=1
go test ./... -count=1
make docs-check
make check
make install-user
```

Expected: all commands exit 0.

- [ ] **Step 9: Commit**

```bash
git add internal/tui/ux_snapshot_test.go internal/tui/testdata/ux_ticket_detail.golden internal/tui/testdata/ux_action_palette.golden docs/keyboard.md docs/workflows.md docs/project-state.md docs/releases/CHANGELOG.md tasks/todo.md
git commit -m "docs: document developer workbench UX"
```
