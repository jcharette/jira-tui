# Local Active Status Filter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a local issue-table Active filter that hides terminal-status tickets without changing Jira reads, saved views, caches, or loaded issue data.

**Architecture:** Keep `m.issues` as the loaded source of truth and add model-local table filter state. Build filtered issue rows and visible issue indexes from the existing issue display tree, then route table rendering, movement, paging, selection repair, and first/last jumps through those visible indexes. The filter is presentation-only and does not touch JQL, workers, or cache keys.

**Tech Stack:** Go, Bubble Tea v2, Lip Gloss, existing `internal/tui` model/update/rendering tests.

## Global Constraints

- This is UX-only: do not change active JQL, saved views, Jira worker requests, cache keys, cached records, or `m.issues`.
- `All` is the default mode for every loaded view.
- `Active` hides only terminal statuses matched by normalized status text: `done`, `closed`, `resolved`, `canceled`, and `cancelled`.
- Empty or unknown status text remains visible.
- Detail mode keeps showing any ticket that was already opened; the filter only affects table presentation and table navigation.
- Use `f` as the table filter toggle.
- Keep implementation inside `internal/tui`; create a small same-package helper file only if it keeps `issue_list.go` and `navigation.go` focused.
- Do not commit unless the user asks for a commit.

---

## File Structure

- Modify `internal/tui/model.go`: add local status filter state and handle `f` in table mode.
- Modify `internal/tui/keymap.go`: document the table filter binding.
- Modify `internal/tui/issue_list.go`: filter rendered issue rows and render filtered-empty copy/title context.
- Modify `internal/tui/navigation.go`: add visible-index helpers, selection repair, and filtered movement/paging.
- Modify `internal/tui/issue_list_test.go`: add focused filter rendering/navigation tests.
- Modify `docs/project-state.md`, `docs/releases/CHANGELOG.md`, and `tasks/todo.md`: document the user-visible behavior after verification.

---

### Task 1: Filter State And Status Classification

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/issue_list.go`
- Test: `internal/tui/issue_list_test.go`

**Interfaces:**
- Produces: `type issueStatusFilter int`
- Produces: `const issueStatusFilterAll issueStatusFilter`
- Produces: `const issueStatusFilterActive issueStatusFilter`
- Produces: `func terminalIssueStatus(status string) bool`
- Produces: `func (m Model) issueVisibleByStatus(issue jira.Issue) bool`
- Produces: `func (m Model) activeStatusFilterLabel() string`

- [ ] **Step 1: Add failing status classification tests**

Add these tests to `internal/tui/issue_list_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/tui -run 'TestTerminalIssueStatusMatchesCommonTerminalStatuses|TestTerminalIssueStatusKeepsActiveStatusesVisible' -count=1
```

Expected: FAIL because `terminalIssueStatus` does not exist.

- [ ] **Step 3: Add filter state and helpers**

In `internal/tui/model.go`, add this type near the existing mode/sort types if present, or near the model constants:

```go
type issueStatusFilter int

const (
	issueStatusFilterAll issueStatusFilter = iota
	issueStatusFilterActive
)
```

Add this field near `sort sortMode`:

```go
	statusFilter issueStatusFilter
```

In `internal/tui/issue_list.go`, add:

```go
func terminalIssueStatus(status string) bool {
	normalized := strings.Join(strings.Fields(strings.ToLower(status)), " ")
	if normalized == "" || normalized == "unknown" {
		return false
	}
	for _, terminal := range []string{"done", "closed", "resolved", "canceled", "cancelled"} {
		if strings.Contains(normalized, terminal) {
			return true
		}
	}
	return false
}

func (m Model) issueVisibleByStatus(issue jira.Issue) bool {
	if m.statusFilter != issueStatusFilterActive {
		return true
	}
	return !terminalIssueStatus(issue.Status)
}

func (m Model) activeStatusFilterLabel() string {
	if m.statusFilter == issueStatusFilterActive {
		return "Active"
	}
	return "All"
}
```

- [ ] **Step 4: Run focused classification tests**

Run:

```bash
go test ./internal/tui -run 'TestTerminalIssueStatusMatchesCommonTerminalStatuses|TestTerminalIssueStatusKeepsActiveStatusesVisible' -count=1
```

Expected: PASS.

---

### Task 2: Filtered Rendering And Empty State

**Files:**
- Modify: `internal/tui/issue_list.go`
- Test: `internal/tui/issue_list_test.go`

**Interfaces:**
- Consumes: `func (m Model) issueVisibleByStatus(issue jira.Issue) bool`
- Produces: `func (m Model) visibleIssueIndexes(displayTree issueDisplayTree) []int`
- Produces: `func (m Model) filteredIssueRows(layout browserLayout) []string`

- [ ] **Step 1: Add failing rendering tests**

Add these tests to `internal/tui/issue_list_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/tui -run 'TestIssueListStatusFilterDefaultsToAll|TestIssueListActiveStatusFilterHidesTerminalIssues|TestIssueListActiveStatusFilterEmptyState' -count=1
```

Expected: FAIL because rendering does not apply the status filter or empty-state copy.

- [ ] **Step 3: Add visible issue indexes and filtered rows**

In `internal/tui/issue_list.go`, add:

```go
func (m Model) visibleIssueIndexes(displayTree issueDisplayTree) []int {
	indexes := make([]int, 0, len(displayTree.issues))
	for _, root := range displayTree.roots {
		if root.missingParentKey != "" {
			for _, child := range displayTree.missingParents[root.missingParentKey].children {
				indexes = m.appendVisibleIssueIndexes(indexes, displayTree, child)
			}
			continue
		}
		indexes = m.appendVisibleIssueIndexes(indexes, displayTree, root.issueIndex)
	}
	return indexes
}

func (m Model) appendVisibleIssueIndexes(indexes []int, displayTree issueDisplayTree, index int) []int {
	issue := displayTree.issues[index]
	if m.issueVisibleByStatus(issue) {
		indexes = append(indexes, index)
	}
	for _, child := range displayTree.children[issue.Key] {
		indexes = m.appendVisibleIssueIndexes(indexes, displayTree, child)
	}
	return indexes
}

func (m Model) filteredIssueRows(layout browserLayout) []string {
	displayTree := buildIssueDisplayTree(m.issues)
	visible := make(map[int]bool, len(m.issues))
	for _, index := range m.visibleIssueIndexes(displayTree) {
		visible[index] = true
	}
	var rows []string
	for _, root := range displayTree.roots {
		if root.missingParentKey != "" {
			rows = append(rows, m.filteredMissingParentRows(displayTree, root.missingParentKey, visible, layout)...)
			continue
		}
		rows = append(rows, m.filteredIssueTreeRows(displayTree, root.issueIndex, "", true, visible, layout)...)
	}
	return rows
}
```

Add filtered wrappers that preserve existing row rendering:

```go
func (m Model) filteredMissingParentRows(displayTree issueDisplayTree, parentKey string, visible map[int]bool, layout browserLayout) []string {
	group := displayTree.missingParents[parentKey]
	var childRows []string
	for index, child := range group.children {
		childRows = append(childRows, m.filteredIssueTreeRows(displayTree, child, "  ", index == len(group.children)-1, visible, layout)...)
	}
	if len(childRows) == 0 {
		return nil
	}
	label := "Parent outside view: " + parentKey
	if group.summary != "" {
		label += "  " + group.summary
	}
	gutterWidth := issueTreeGutterWidth(layout)
	rows := []string{m.theme.Muted.Render(padRight("  ◇", gutterWidth) + truncate(label, max(20, layout.listWidth-gutterWidth-2)))}
	rows = append(rows, childRows...)
	return rows
}

func (m Model) filteredIssueTreeRows(displayTree issueDisplayTree, index int, prefix string, last bool, visible map[int]bool, layout browserLayout) []string {
	var rows []string
	if visible[index] {
		rows = append(rows, m.issueTreeRows(displayTree, index, prefix, last, layout)...)
		return rows
	}
	children := displayTree.children[displayTree.issues[index].Key]
	nextPrefix := prefix
	if prefix != "" || len(children) > 0 {
		if last {
			nextPrefix += "  "
		} else {
			nextPrefix += "│ "
		}
	}
	for childPosition, childIndex := range children {
		rows = append(rows, m.filteredIssueTreeRows(displayTree, childIndex, nextPrefix, childPosition == len(children)-1, visible, layout)...)
	}
	return rows
}
```

Change `issueRows` to use the filtered path:

```go
func (m Model) issueRows(layout browserLayout) []string {
	if m.statusFilter == issueStatusFilterAll {
		displayTree := buildIssueDisplayTree(m.issues)
		var rows []string
		for _, root := range displayTree.roots {
			if root.missingParentKey != "" {
				rows = append(rows, m.missingParentRows(displayTree, root.missingParentKey, layout)...)
				continue
			}
			rows = append(rows, m.issueTreeRows(displayTree, root.issueIndex, "", true, layout)...)
		}
		return rows
	}
	return m.filteredIssueRows(layout)
}
```

Update `renderIssueList` empty rendering:

```go
	} else if len(m.issues) > 0 && m.statusFilter == issueStatusFilterActive {
		b.WriteString(m.theme.Muted.Render("Active filter hides all loaded issues. Press f show all."))
	} else {
		b.WriteString(m.theme.Muted.Render("No issues found for this view."))
	}
```

Update `issueListTitle`:

```go
func (m Model) issueListTitle(rowTotal int, rowCount int, start int, end int) string {
	title := "0 issues"
	if len(m.issues) > 0 {
		title = fmt.Sprintf("%d issues", len(m.issues))
		if m.statusFilter == issueStatusFilterActive {
			title += fmt.Sprintf("  Active %d shown", rowTotal)
		}
		if rowTotal > rowCount {
			title += fmt.Sprintf("  rows %d-%d", start+1, end)
		}
	}
	return title
}
```

- [ ] **Step 4: Run focused rendering tests**

Run:

```bash
go test ./internal/tui -run 'TestIssueListStatusFilterDefaultsToAll|TestIssueListActiveStatusFilterHidesTerminalIssues|TestIssueListActiveStatusFilterEmptyState|TestIssueListRendersCompactHierarchyWithLipglossTree' -count=1
```

Expected: PASS.

---

### Task 3: Toggle Binding And Navigation Through Visible Issues

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/navigation.go`
- Modify: `internal/tui/keymap.go`
- Test: `internal/tui/issue_list_test.go`

**Interfaces:**
- Consumes: `func (m Model) visibleIssueIndexes(displayTree issueDisplayTree) []int`
- Produces: `func (m *Model) toggleStatusFilter()`
- Produces: `func (m *Model) repairStatusFilterSelection()`
- Produces: `func (m Model) visibleSelectionPosition(visible []int) int`

- [ ] **Step 1: Add failing toggle and navigation tests**

Add these tests to `internal/tui/issue_list_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/tui -run 'TestIssueListStatusFilterToggleUsesF|TestIssueListStatusFilterRepairsHiddenSelection|TestIssueListStatusFilterNavigationSkipsHiddenRows|TestIssueListStatusFilterHomeEndUseVisibleRows' -count=1
```

Expected: FAIL because `f` is not handled and navigation uses raw indexes.

- [ ] **Step 3: Add toggle and selection helpers**

In `internal/tui/navigation.go`, add:

```go
func (m *Model) toggleStatusFilter() {
	if m.statusFilter == issueStatusFilterActive {
		m.statusFilter = issueStatusFilterAll
		m.detailNotice = "Showing all loaded issues."
	} else {
		m.statusFilter = issueStatusFilterActive
		m.detailNotice = "Showing active loaded issues."
	}
	m.repairStatusFilterSelection()
	m.ensureSelectionVisible(m.currentLayoutRows())
}

func (m *Model) repairStatusFilterSelection() {
	if len(m.issues) == 0 {
		m.selected = 0
		m.offset = 0
		return
	}
	displayTree := buildIssueDisplayTree(m.issues)
	visible := m.visibleIssueIndexes(displayTree)
	if len(visible) == 0 {
		m.selected = 0
		m.offset = 0
		return
	}
	for _, index := range visible {
		if index == m.selected {
			return
		}
	}
	for _, index := range visible {
		if index >= m.selected {
			m.selected = index
			return
		}
	}
	m.selected = visible[len(visible)-1]
}

func (m Model) visibleSelectionPosition(visible []int) int {
	if len(visible) == 0 {
		return 0
	}
	for position, index := range visible {
		if index == m.selected {
			return position
		}
	}
	return 0
}
```

Replace `moveSelection` and `pageSelection` in `internal/tui/navigation.go` with visible-index versions:

```go
func (m *Model) moveSelection(delta int) {
	if len(m.issues) == 0 {
		m.selected = 0
		m.offset = 0
		return
	}
	displayTree := buildIssueDisplayTree(m.issues)
	visible := m.visibleIssueIndexes(displayTree)
	if len(visible) == 0 {
		m.selected = 0
		m.offset = 0
		return
	}
	position := m.visibleSelectionPosition(visible)
	position = clamp(position+delta, 0, len(visible)-1)
	m.selected = visible[position]
	m.resetDetailScroll()
	m.ensureSelectionVisible(m.currentLayoutRows())
}

func (m *Model) pageSelection(delta int) {
	if len(m.issues) == 0 {
		m.selected = 0
		m.offset = 0
		return
	}
	displayTree := buildIssueDisplayTree(m.issues)
	visible := m.visibleIssueIndexes(displayTree)
	if len(visible) == 0 {
		m.selected = 0
		m.offset = 0
		return
	}
	rows := m.currentLayoutRows()
	step := max(1, rows-1)
	position := m.visibleSelectionPosition(visible)
	position = clamp(position+(delta*step), 0, len(visible)-1)
	m.selected = visible[position]
	m.resetDetailScroll()
	m.ensureSelectionVisible(rows)
}
```

- [ ] **Step 4: Wire key handling and footer/help**

In `internal/tui/model.go`, add a table-mode key case:

```go
		case "f":
			if m.mode == modeTable {
				m.toggleStatusFilter()
				return m, nil
			}
```

Update table-mode home/end behavior to use visible indexes:

```go
			displayTree := buildIssueDisplayTree(m.issues)
			visible := m.visibleIssueIndexes(displayTree)
			if len(visible) > 0 {
				m.selected = visible[0]
			} else {
				m.selected = 0
			}
			m.offset = 0
			return m.startSelectedIssuePrefetch()
```

and:

```go
			displayTree := buildIssueDisplayTree(m.issues)
			visible := m.visibleIssueIndexes(displayTree)
			if len(visible) > 0 {
				m.selected = visible[len(visible)-1]
			} else {
				m.selected = max(0, len(m.issues)-1)
			}
			m.ensureSelectionVisible(m.currentLayoutRows())
			return m.startSelectedIssuePrefetch()
```

In `internal/tui/keymap.go`, add to `tableBindings()`:

```go
		{Keys: []string{"f"}, Label: "active", Description: "Toggle local active-status filtering for loaded issues.", Group: "Views", Footer: true},
```

- [ ] **Step 5: Run focused toggle/navigation tests**

Run:

```bash
go test ./internal/tui -run 'TestIssueListStatusFilterToggleUsesF|TestIssueListStatusFilterRepairsHiddenSelection|TestIssueListStatusFilterNavigationSkipsHiddenRows|TestIssueListStatusFilterHomeEndUseVisibleRows|TestHelpIncludesActiveContextBindings' -count=1
```

Expected: PASS.

---

### Task 4: Docs, Formatting, And Verification

**Files:**
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`
- Format: `internal/tui/model.go`
- Format: `internal/tui/keymap.go`
- Format: `internal/tui/issue_list.go`
- Format: `internal/tui/navigation.go`
- Format: `internal/tui/issue_list_test.go`

**Interfaces:**
- Consumes: implemented `f` local active-status filter.
- Produces: updated repo docs and completed task review notes.

- [ ] **Step 1: Update project state**

Add this behavior note to the issue table behavior area in `docs/project-state.md`:

```markdown
- Issue tables support a local `f` Active filter that hides loaded tickets whose status text looks
  terminal (`done`, `closed`, `resolved`, `canceled`, or `cancelled`) without changing Jira reads,
  saved views, cache records, or loaded issue data.
```

- [ ] **Step 2: Update changelog**

Add this bullet under `## Unreleased` in `docs/releases/CHANGELOG.md`:

```markdown
- Added a local issue-table Active filter with `f`, hiding loaded terminal-status tickets without
  changing saved views, JQL, Jira reads, or cached issue data.
```

- [ ] **Step 3: Update task checklist and review**

In `tasks/todo.md`, mark the `Local Active Status Filter` checklist complete and add review notes:

```markdown
- Added `f` as a table-mode toggle between All and Active loaded issues.
- Kept JQL, saved views, Jira requests, cache keys, cache records, and `m.issues` unchanged.
- Hid only locally terminal status text: done, closed, resolved, canceled, and cancelled.
- Routed rendering, movement, paging, first/last jumps, and selection repair through visible loaded
  issues while the filter is active.
- Added filtered-empty copy that tells users how to return to all loaded issues.
- Verified with focused issue-list/status-filter tests, full Go tests, `make check`, and
  `make install-user`.
```

- [ ] **Step 4: Format touched Go files**

Run:

```bash
gofmt -w internal/tui/model.go internal/tui/keymap.go internal/tui/issue_list.go internal/tui/navigation.go internal/tui/issue_list_test.go
```

Expected: command exits 0.

- [ ] **Step 5: Run focused tests**

Run:

```bash
go test ./internal/tui -run 'TestIssueListStatusFilter|TestTerminalIssueStatus|TestHelpIncludesActiveContextBindings' -count=1
```

Expected: PASS.

- [ ] **Step 6: Run full Go tests**

Run:

```bash
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 7: Run standard verification**

Run:

```bash
make check
```

Expected: PASS.

- [ ] **Step 8: Install updated binary**

Run:

```bash
make install-user
```

Expected: PASS.

- [ ] **Step 9: Inspect final diff**

Run:

```bash
git diff --stat
git diff -- docs/superpowers/specs/2026-06-17-local-active-status-filter-design.md docs/superpowers/plans/2026-06-17-local-active-status-filter.md tasks/todo.md docs/project-state.md docs/releases/CHANGELOG.md internal/tui/model.go internal/tui/keymap.go internal/tui/issue_list.go internal/tui/navigation.go internal/tui/issue_list_test.go
```

Expected: diff is limited to the planned files and contains no Jira worker/cache/query changes.

## Self-Review

- Spec coverage: the plan covers local-only filtering, default All mode, terminal status matching,
  visible rendering, navigation repair, empty state, docs, and verification.
- Placeholder scan: no placeholder tasks remain.
- Type consistency: `issueStatusFilter`, `terminalIssueStatus`, `issueVisibleByStatus`,
  `visibleIssueIndexes`, `toggleStatusFilter`, and `repairStatusFilterSelection` are introduced
  before later tasks consume them.
