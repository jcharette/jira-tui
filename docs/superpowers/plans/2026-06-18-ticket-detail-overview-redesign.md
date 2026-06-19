# Ticket Detail Overview Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Redesign focused ticket detail around an `Overview + Control Strip` layout so tickets open on useful work context instead of the Description tab.

**Architecture:** Keep the work inside the existing Bubble Tea detail model. Add an Overview section, make editable fields visible controls, promote Status to a field target, and reuse existing transition/comment/hierarchy/link/action flows rather than adding Jira behavior.

**Tech Stack:** Go, Bubble Tea v2, Lip Gloss, existing `internal/tui` detail rendering and worker-backed workflows.

## Global Constraints

- Do not change Jira queries or Jira API behavior.
- Keep existing summary, priority, assignee, comment, hierarchy, links, transitions, and Ticket Actions workflows intact.
- Keep keyboard accelerators available; focus plus `enter` becomes the visible primary model.
- Preserve section scroll memory for remaining sections.
- Update docs and changelog for user-visible layout behavior.

---

### Task 1: Detail Targets And Navigation Model

**Files:**
- Modify: `internal/tui/detail.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/chrome.go`
- Modify: `internal/tui/detail_test.go`
- Modify: `internal/tui/issue_list_test.go` if key-context duplicate coverage needs adjustment

**Interfaces:**
- Produces: `detailSection{ID: "overview", Label: "Overview", Short: "Over"}`
- Produces: `detailTarget{ID: "status", Label: "Status", Kind: detailTargetField}`
- Consumes: existing `startStatusTransitionPicker()`

- [x] **Step 1: Add failing tests for section order and default focus.**

Add tests in `internal/tui/detail_test.go` equivalent to:

```go
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
	wantPrefix := []string{"overview", "comments", "hierarchy"}
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
```

- [x] **Step 2: Add failing test for Status as a field target.**

Add a test in `internal/tui/detail_test.go` equivalent to:

```go
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
```

If no helper exists, add a local test helper:

```go
func indexOfDetailTargetForTest(model Model, id string) int {
	for index, target := range model.detailTargets() {
		if target.ID == id {
			return index
		}
	}
	return 0
}
```

- [x] **Step 3: Run focused tests and verify RED.**

Run:

```bash
go test ./internal/tui -run 'TestDetailSectionsUseOverviewFirstWithoutStatusTab|TestTicketDetailDefaultsToOverviewTarget|TestEnterOnStatusFieldStartsTransitionPicker' -count=1
```

Expected: FAIL because `overview` and field-target `status` do not exist yet.

- [x] **Step 4: Implement navigation model changes.**

Change `detailSections()` in `internal/tui/detail.go` to return content destinations:

```go
sections := []detailSection{
	{ID: "overview", Label: "Overview", Short: "Over"},
	{ID: "comments", Label: "Comments", Short: "Com"},
	{ID: "hierarchy", Label: "Hierarchy", Short: "Tree"},
}
```

Keep existing dynamic `links` insertion, but insert it after `hierarchy` or after `comments` based on the rendered order chosen in code. Remove `status`, `description`, and `actions` from this primary list.

Change `detailTargets()` to include Status before Priority:

```go
targets := []detailTarget{
	{ID: "summary", Label: "Summary", Kind: detailTargetField},
	{ID: "status", Label: "Status", Kind: detailTargetField},
	{ID: "priority", Label: "Priority", Kind: detailTargetField},
	{ID: "assignee", Label: "Assignee", Kind: detailTargetField},
}
```

Change `activateFocusedDetailTarget()` field handling:

```go
case "status":
	return m.startStatusTransitionPicker()
```

Keep `runSelectedDetailAction()` and `Ticket Actions` unchanged so the action palette remains available.

- [x] **Step 5: Run focused tests and verify GREEN.**

Run:

```bash
go test ./internal/tui -run 'TestDetailSectionsUseOverviewFirstWithoutStatusTab|TestTicketDetailDefaultsToOverviewTarget|TestEnterOnStatusFieldStartsTransitionPicker' -count=1
```

Expected: PASS.

### Task 2: Overview Section And Compact Control Strip

**Files:**
- Modify: `internal/tui/detail.go`
- Modify: `internal/tui/chrome.go`
- Modify: `internal/tui/detail_test.go`

**Interfaces:**
- Produces: `renderOverviewSection(ctx detailRenderContext, width int) string`
- Produces: `renderDetailControlStrip(width int) string`
- Consumes: existing `renderComments`, `renderHierarchySection`, `renderDescription`, `detailStatusBlock`, `renderDetailNotice`, `statusStyle`

- [x] **Step 1: Add failing render tests.**

Add tests in `internal/tui/detail_test.go` equivalent to:

```go
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

	for _, want := range []string{"Overview", "Status", "To Do", "Priority", "P3 - Low", "Assignee", "Jon C.", "Description preview"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	if strings.Contains(view, "Description   Hierarchy") || strings.Contains(view, "> Status") {
		t.Fatalf("old tab layout still visible in %q", view)
	}
}
```

- [x] **Step 2: Add failing Overview content test for comments and hierarchy summaries.**

Add a test equivalent to:

```go
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
		"ABC-1": {{ID: "10001", Author: "Sam", Body: "Latest update"}},
	}

	view := model.render()

	for _, want := range []string{"Latest", "Sam", "Latest update", "Hierarchy", "1 loaded child"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}
```

- [x] **Step 3: Run focused render tests and verify RED.**

Run:

```bash
go test ./internal/tui -run 'TestRenderFullDetailShowsOverviewControlStrip|TestOverviewSummarizesCommentsAndHierarchy' -count=1
```

Expected: FAIL because Overview rendering does not exist.

- [x] **Step 4: Implement compact header and control strip.**

In `renderFullDetail()`, replace the existing four-line header composition:

```go
header := m.renderDetailTitleLine(headerWidth) + "\n" +
	m.renderDetailSummaryLine(headerWidth) + "\n" +
	m.renderDetailControlStrip(headerWidth) + "\n" +
	m.renderDetailTabs(headerWidth)
```

Add `renderDetailControlStrip(width int) string` near the existing header helpers:

```go
func (m Model) renderDetailControlStrip(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	detail, hasDetail := m.details[selected.Key]
	display := m.displayIssueForDetail(selected, detail, hasDetail)
	parts := []string{
		m.detailMetaPartWithStyle("Status", displayValue(display.Status, "Unknown")+" v", m.focusedDetailTargetID() == "status"),
		m.detailMetaPartWithStyle("Priority", displayValue(display.Priority, "Unknown")+" v", m.focusedDetailTargetID() == "priority"),
		m.detailMetaPartWithStyle("Assignee", shortName(displayValue(display.Assignee, "Unassigned"))+" v", m.focusedDetailTargetID() == "assignee"),
	}
	if hasDetail {
		parts = append(parts, m.detailMetaPart("Updated", formatTime(detail.Updated)))
	}
	if hasDetail && strings.TrimSpace(detail.Reporter) != "" && detail.Reporter != "Unknown" {
		parts = append(parts, m.detailMetaPart("Reporter", shortName(detail.Reporter)))
	}
	separator := m.theme.Muted.Render("   ")
	for len(parts) > 0 {
		line := strings.Join(parts, separator)
		if lipgloss.Width(line) <= width {
			return line
		}
		parts = parts[:len(parts)-1]
	}
	return ""
}
```

Keep `renderDetailHeaderMeta()` until tests are adjusted or all callers are removed; remove it only if unused after the slice is complete.

- [x] **Step 5: Implement Overview section rendering.**

Update `renderDetailSection()`:

```go
case "overview":
	return m.renderOverviewSection(ctx, width)
```

Add a focused Overview renderer:

```go
func (m Model) renderOverviewSection(ctx detailRenderContext, width int) string {
	lines := []string{m.detailSectionHeader("overview", "Overview", "", width)}
	lines = append(lines, "", m.renderOverviewLatest(ctx.display.Key, width))
	lines = append(lines, "", m.renderOverviewDescription(ctx.description, width))
	lines = append(lines, "", m.renderOverviewHierarchy(ctx.display, width))
	lines = append(lines, "", m.detailEmptyState("Press . for Ticket Actions.", width))
	return strings.Join(lines, "\n")
}
```

Add helper functions with bounded output:

```go
func (m Model) renderOverviewLatest(key string, width int) string {
	header := m.theme.Muted.Render("Latest")
	if comments, loaded := m.comments[key]; loaded {
		if len(comments) == 0 {
			return header + "\n" + m.detailEmptyState("No comments yet.", width)
		}
		comment := comments[len(comments)-1]
		body := strings.TrimSpace(comment.Body)
		if body == "" {
			body = "Comment has no text."
		}
		return header + "\n" + m.theme.Text.Render(shortName(comment.Author)+": "+truncate(singleLine(body), max(24, width-4)))
	}
	if m.commentsLoading && m.commentsRequestKey == key {
		return header + "\n" + m.detailStatusBlock("Loading comments...", width, false)
	}
	if m.commentsErr != nil && m.commentsRequestKey == key {
		return header + "\n" + m.detailStatusBlock("Comments failed: "+m.commentsErr.Error(), width, true)
	}
	return header + "\n" + m.detailEmptyState("Comments not loaded.", width)
}
```

```go
func (m Model) renderOverviewDescription(description string, width int) string {
	header := m.theme.Muted.Render("Description preview")
	description = strings.TrimSpace(description)
	if description == "" {
		return header + "\n" + m.detailEmptyState("No description.", width)
	}
	lines := wrapRichText(description, max(24, width))
	if len(lines) > 3 {
		lines = append(lines[:3], m.theme.Muted.Render("..."))
	}
	return header + "\n" + m.renderRichDescriptionBody(lines, width)
}
```

```go
func (m Model) renderOverviewHierarchy(issue jira.Issue, width int) string {
	header := m.theme.Muted.Render("Hierarchy")
	children := m.currentHierarchyChildren()
	if len(children) == 1 {
		return header + "\n" + m.theme.Text.Render("1 loaded child")
	}
	if len(children) > 1 {
		return header + "\n" + m.theme.Text.Render(fmt.Sprintf("%d loaded children", len(children)))
	}
	return header + "\n" + m.renderHierarchyEmptyState(issue, width)
}
```

If `singleLine` does not exist, add:

```go
func singleLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
```

- [x] **Step 6: Run focused render tests and verify GREEN.**

Run:

```bash
go test ./internal/tui -run 'TestRenderFullDetailShowsOverviewControlStrip|TestOverviewSummarizesCommentsAndHierarchy' -count=1
```

Expected: PASS.

### Task 3: Footer, Shortcuts, And Regression Coverage

**Files:**
- Modify: `internal/tui/chrome.go`
- Modify: `internal/tui/detail.go`
- Modify: `internal/tui/detail_test.go`
- Modify: `internal/tui/issue_list_test.go` if footer/key context tests require updated expectations

**Interfaces:**
- Consumes: `focusedDetailTargetID()`
- Consumes: existing section-specific bindings

- [x] **Step 1: Add failing footer test for focused Status control.**

Add a test equivalent to:

```go
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
```

- [x] **Step 2: Run footer test and verify RED.**

Run:

```bash
go test ./internal/tui -run 'TestDetailFooterShowsStatusControlAction' -count=1
```

Expected: FAIL until footer bindings know about field-target status.

- [x] **Step 3: Update contextual footer bindings.**

In `renderContextualFooterBindings()` or the local helper in `internal/tui/chrome.go`, check focused field targets before section bindings:

```go
switch m.focusedDetailTargetID() {
case "status":
	return append(base, keyBinding{Keys: []string{"enter"}, Label: "transition", Group: "Field", Footer: true})
case "priority":
	return append(base, keyBinding{Keys: []string{"enter"}, Label: "priority", Group: "Field", Footer: true})
case "assignee":
	return append(base, keyBinding{Keys: []string{"enter"}, Label: "assignee", Group: "Field", Footer: true})
case "summary":
	return append(base, keyBinding{Keys: []string{"enter"}, Label: "summary", Group: "Field", Footer: true})
}
```

Keep existing single-key shortcuts `s`, `p`, `.`, `n`, `a`, `o`, and `c`.

- [x] **Step 4: Run focused footer test and existing detail tests.**

Run:

```bash
go test ./internal/tui -run 'TestDetailFooterShowsStatusControlAction|TestRenderFullDetail|TestDetailActions|TestStatus|TestTransition' -count=1
```

Expected: PASS.

### Task 4: Docs And Full Verification

**Files:**
- Modify: `tasks/todo.md`
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `docs/backlog.md` if the Deep UX Review item is narrowed

**Interfaces:**
- Consumes: completed Tasks 1-3

- [x] **Step 1: Update task checklist.**

Add a top section to `tasks/todo.md`:

```markdown
## Ticket Detail Overview Redesign

- [x] Approve Proposal B: Overview + Control Strip.
- [x] Add design spec and implementation plan.
- [x] Add tests for Overview-first detail navigation and Status field activation.
- [x] Implement Overview section and compact control strip.
- [x] Update footer/help behavior for focused field controls.
- [x] Update docs and changelog.
- [x] Run focused tests, `go test ./... -count=1`, `make check`, `make install-user`, and `git diff --check`.
```

- [x] **Step 2: Update project state and changelog.**

Add to `docs/project-state.md` that focused ticket detail now opens on Overview and separates content navigation from editable controls.

Add to `docs/releases/CHANGELOG.md`:

```markdown
- Redesigned focused ticket detail around an Overview-first layout with a compact editable control
  strip for Status, Priority, and Assignee, removing one-action Status and passive Description from
  the primary tab row.
```

- [x] **Step 3: Run full verification.**

Run:

```bash
go test ./... -count=1
make check
make install-user
git diff --check
```

Expected: all commands pass.

- [x] **Step 4: Review final diff shape.**

Run:

```bash
git diff --stat
git status --short
```

Expected: changes are limited to TUI detail/chrome tests plus docs for this redesign, alongside already-existing unrelated working-tree changes.
