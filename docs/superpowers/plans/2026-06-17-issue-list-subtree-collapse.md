# Issue List Subtree Collapse Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add local issue-table subtree collapse/expand so users can focus dense ticket trees without changing Jira reads or loaded issue data.

**Architecture:** Keep `m.issues` as the loaded source of truth and add model-local collapse state keyed by issue key. Build a visible issue-row projection from the existing issue tree plus collapse state, then route rendering, selection movement, paging, and selection repair through that projection. Keep Jira-backed `x`/`X` expansion unchanged.

**Tech Stack:** Go, Bubble Tea v2, Lip Gloss, existing `internal/tui` model/update/rendering tests.

## Global Constraints

- Collapse is presentation state only; do not change Jira reads, worker requests, saved views, persistent cache records, issue ordering, or `m.issues` contents.
- Default behavior remains expanded until the user collapses a node.
- `x` and `X` keep loading open/all children through the existing worker path.
- Expanding a node reveals that node's direct subtree level while preserving deeper collapsed branches.
- Use `z` as the first table collapse/expand toggle.
- Keep implementation in the existing `tui` package; create a small same-package helper file if it keeps `issue_list.go` focused.
- Do not commit unless the user asks for a commit.

---

## File Structure

- Modify `internal/tui/model.go`: add `collapsedIssueKeys map[string]bool` to `Model`; add `z` key handling in table mode.
- Modify `internal/tui/keymap.go`: document the `z collapse` table binding.
- Modify `internal/tui/issue_list.go`: introduce visible row projection types and render collapsed subtree rows with hidden counts.
- Modify `internal/tui/navigation.go`: add collapse toggle, selection repair, and visible-row navigation helpers.
- Modify `internal/tui/issue_list_test.go`: add focused rendering/navigation regression tests.
- Modify `docs/project-state.md`, `docs/releases/CHANGELOG.md`, and `tasks/todo.md`: document the user-visible behavior and mark the checklist complete after verification.

---

### Task 1: Collapse State And Rendered Projection

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/issue_list.go`
- Test: `internal/tui/issue_list_test.go`

**Interfaces:**
- Consumes: `buildIssueDisplayTree(issues []jira.Issue) issueDisplayTree`
- Produces: `func (m Model) visibleIssueIndexes(displayTree issueDisplayTree) []int`
- Produces: `func (m Model) loadedDescendantCount(displayTree issueDisplayTree, issueKey string) int`
- Produces: `func (m Model) issueRenderLines(layout browserLayout) []issueRenderLine`

- [ ] **Step 1: Add failing default-expanded and collapsed-render tests**

Add these tests near the existing hierarchy tests in `internal/tui/issue_list_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/tui -run 'TestIssueListCollapseDefaultsExpanded|TestIssueListCollapsedParentHidesLoadedDescendants' -count=1
```

Expected: first test passes or remains unaffected; second test fails because `collapsedIssueKeys` does not exist or children still render.

- [ ] **Step 3: Add model field and render projection**

In `internal/tui/model.go`, add this field near `issues`, `selected`, and `offset`:

```go
	collapsedIssueKeys                  map[string]bool
```

In `internal/tui/issue_list.go`, add this type near `issueDisplayTree`:

```go
type issueRenderLine struct {
	text       string
	issueIndex int
}
```

Replace `issueRows` with a projection wrapper:

```go
func (m Model) issueRows(layout browserLayout) []string {
	lines := m.issueRenderLines(layout)
	rows := make([]string, 0, len(lines))
	for _, line := range lines {
		rows = append(rows, line.text)
	}
	return rows
}

func (m Model) issueRenderLines(layout browserLayout) []issueRenderLine {
	displayTree := buildIssueDisplayTree(m.issues)
	var lines []issueRenderLine
	for _, root := range displayTree.roots {
		if root.missingParentKey != "" {
			for _, row := range m.missingParentRows(displayTree, root.missingParentKey, layout) {
				lines = append(lines, issueRenderLine{text: row, issueIndex: -1})
			}
			continue
		}
		lines = append(lines, m.issueTreeRenderLines(displayTree, root.issueIndex, "", true, layout)...)
	}
	return lines
}
```

Add visible-index and descendant helpers:

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
	indexes = append(indexes, index)
	if m.issueCollapsed(displayTree.issues[index].Key) {
		return indexes
	}
	for _, child := range displayTree.children[displayTree.issues[index].Key] {
		indexes = m.appendVisibleIssueIndexes(indexes, displayTree, child)
	}
	return indexes
}

func (m Model) issueCollapsed(key string) bool {
	return key != "" && m.collapsedIssueKeys != nil && m.collapsedIssueKeys[key]
}

func (m Model) loadedDescendantCount(displayTree issueDisplayTree, issueKey string) int {
	count := 0
	var walk func(string)
	walk = func(key string) {
		for _, child := range displayTree.children[key] {
			count++
			walk(displayTree.issues[child].Key)
		}
	}
	walk(issueKey)
	return count
}
```

Replace `issueTreeRows` with a wrapper and render-line implementation:

```go
func (m Model) issueTreeRows(displayTree issueDisplayTree, index int, prefix string, last bool, layout browserLayout) []string {
	lines := m.issueTreeRenderLines(displayTree, index, prefix, last, layout)
	rows := make([]string, 0, len(lines))
	for _, line := range lines {
		rows = append(rows, line.text)
	}
	return rows
}

func (m Model) issueTreeRenderLines(displayTree issueDisplayTree, index int, prefix string, last bool, layout browserLayout) []issueRenderLine {
	row := displayTree.issueRow(index)
	hiddenDescendants := 0
	if m.issueCollapsed(row.issue.Key) {
		hiddenDescendants = m.loadedDescendantCount(displayTree, row.issue.Key)
	}
	gutter := issueTreeGutter(prefix, last, index == m.selected, layout)
	label := m.renderIssueDisplayRowWithHidden(row, gutter, hiddenDescendants, layout)
	if index == m.selected {
		label = m.theme.Selected.Render(label)
		if detail := m.selectedIssueListDetail(row, layout); detail != "" {
			label += "\n" + m.theme.Muted.Render(padRight("", issueTreeGutterWidth(layout)+issueTypeColumnWidth+1)+detail)
		}
	}
	rawRows := strings.Split(label, "\n")
	lines := make([]issueRenderLine, 0, len(rawRows))
	for lineIndex, raw := range rawRows {
		issueIndex := index
		if lineIndex > 0 {
			issueIndex = -1
		}
		lines = append(lines, issueRenderLine{text: raw, issueIndex: issueIndex})
	}
	if hiddenDescendants > 0 {
		return lines
	}
	children := displayTree.children[row.issue.Key]
	nextPrefix := prefix
	if prefix != "" || len(children) > 0 {
		if last {
			nextPrefix += "  "
		} else {
			nextPrefix += "│ "
		}
	}
	for childPosition, childIndex := range children {
		lines = append(lines, m.issueTreeRenderLines(displayTree, childIndex, nextPrefix, childPosition == len(children)-1, layout)...)
	}
	return lines
}
```

Add this render helper near `renderIssueDisplayRow`:

```go
func (m Model) renderIssueDisplayRowWithHidden(row issueDisplayRow, gutter string, hiddenDescendants int, layout browserLayout) string {
	if hiddenDescendants <= 0 {
		return m.renderIssueDisplayRow(row, gutter, layout)
	}
	row.issue.Summary = strings.TrimSpace(row.issue.Summary + "  " + fmt.Sprintf("%d hidden", hiddenDescendants))
	return m.renderIssueDisplayRow(row, gutter, layout)
}
```

- [ ] **Step 4: Run focused render tests**

Run:

```bash
go test ./internal/tui -run 'TestIssueListCollapseDefaultsExpanded|TestIssueListCollapsedParentHidesLoadedDescendants|TestIssueListRendersCompactHierarchyWithLipglossTree|TestIssueListNestedTree' -count=1
```

Expected: PASS.

---

### Task 2: Toggle Command And Key Help

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/navigation.go`
- Modify: `internal/tui/keymap.go`
- Test: `internal/tui/issue_list_test.go`

**Interfaces:**
- Consumes: `func (m Model) loadedDescendantCount(displayTree issueDisplayTree, issueKey string) int`
- Produces: `func (m *Model) toggleSelectedIssueCollapse()`

- [ ] **Step 1: Add failing toggle tests**

Add these tests to `internal/tui/issue_list_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/tui -run 'TestIssueListToggleCollapseFromSelectedNode|TestIssueListToggleCollapseLeafShowsNotice' -count=1
```

Expected: FAIL because `z` is not handled and `toggleSelectedIssueCollapse` does not exist.

- [ ] **Step 3: Implement toggle helper**

Add this helper to `internal/tui/navigation.go` near `startExpandSelectedIssue`:

```go
func (m *Model) toggleSelectedIssueCollapse() {
	issue, ok := m.selectedIssue()
	if !ok || issue.Key == "" {
		m.detailNotice = "No issue selected."
		return
	}
	displayTree := buildIssueDisplayTree(m.issues)
	descendants := m.loadedDescendantCount(displayTree, issue.Key)
	if descendants == 0 {
		m.detailNotice = "No loaded child issues for " + issue.Key + "."
		return
	}
	if m.collapsedIssueKeys == nil {
		m.collapsedIssueKeys = make(map[string]bool)
	}
	if m.collapsedIssueKeys[issue.Key] {
		delete(m.collapsedIssueKeys, issue.Key)
		m.detailNotice = "Expanded " + issue.Key + "."
	} else {
		m.collapsedIssueKeys[issue.Key] = true
		m.detailNotice = "Collapsed " + issue.Key + "."
	}
	m.repairCollapsedSelection()
	m.ensureSelectionVisible(m.currentLayoutRows())
}
```

Add a `z` case in `internal/tui/model.go` inside the main key switch:

```go
		case "z":
			if m.mode == modeTable {
				m.toggleSelectedIssueCollapse()
				return m, nil
			}
```

Add the key binding to `tableBindings()` in `internal/tui/keymap.go`:

```go
		{Keys: []string{"z"}, Label: "collapse", Description: "Collapse or expand the selected issue subtree.", Group: "Issue", Footer: true},
```

- [ ] **Step 4: Run focused toggle tests**

Run:

```bash
go test ./internal/tui -run 'TestIssueListToggleCollapseFromSelectedNode|TestIssueListToggleCollapseLeafShowsNotice|TestHelpIncludesActiveContextBindings' -count=1
```

Expected: PASS.

---

### Task 3: Visible-Row Navigation And Selection Repair

**Files:**
- Modify: `internal/tui/navigation.go`
- Modify: `internal/tui/issue_list.go`
- Test: `internal/tui/issue_list_test.go`

**Interfaces:**
- Consumes: `func (m Model) visibleIssueIndexes(displayTree issueDisplayTree) []int`
- Produces: `func (m *Model) repairCollapsedSelection()`
- Produces: `func (m Model) visibleSelectionPosition(visible []int) int`
- Produces: `func (m Model) selectedRenderedLineIndex(lines []issueRenderLine) int`

- [ ] **Step 1: Add failing navigation tests**

Add these tests to `internal/tui/issue_list_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/tui -run 'TestIssueListNavigationSkipsCollapsedDescendants|TestIssueListRepairSelectionHiddenByCollapsedAncestor|TestIssueListPagingUsesVisibleRows' -count=1
```

Expected: FAIL because movement and repair still use raw `m.issues` indexes.

- [ ] **Step 3: Route movement through visible issue indexes**

Replace `moveSelection` and `pageSelection` in `internal/tui/navigation.go` with:

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

Add helpers:

```go
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

func (m *Model) repairCollapsedSelection() {
	if len(m.issues) == 0 {
		m.selected = 0
		m.offset = 0
		return
	}
	displayTree := buildIssueDisplayTree(m.issues)
	visible := m.visibleIssueIndexes(displayTree)
	for _, index := range visible {
		if index == m.selected {
			return
		}
	}
	if m.selected >= 0 && m.selected < len(m.issues) {
		parentKey := m.issues[m.selected].ParentKey
		for parentKey != "" {
			parentIndex, ok := displayTree.indexByKey[parentKey]
			if !ok {
				break
			}
			if m.issueCollapsed(parentKey) {
				m.selected = parentIndex
				return
			}
			parentKey = displayTree.issues[parentIndex].ParentKey
		}
	}
	if len(visible) > 0 {
		m.selected = visible[0]
		return
	}
	m.selected = 0
}
```

Replace `ensureSelectionVisible` and `selectedRenderedRowIndex` with line-aware versions:

```go
func (m *Model) ensureSelectionVisible(rows int) {
	rows = max(1, rows)
	lines := m.issueRenderLines(m.browserLayout(m.width))
	selectedRow := m.selectedRenderedLineIndex(lines)
	maxOffset := max(0, len(lines)-rows)
	if selectedRow < m.offset {
		m.offset = selectedRow
	}
	if selectedRow >= m.offset+rows {
		m.offset = selectedRow - rows + 1
	}
	m.offset = clamp(m.offset, 0, maxOffset)
}

func (m Model) selectedRenderedLineIndex(lines []issueRenderLine) int {
	if len(m.issues) == 0 || m.selected < 0 || m.selected >= len(m.issues) {
		return 0
	}
	for index, line := range lines {
		if line.issueIndex == m.selected {
			return index
		}
	}
	return clamp(m.selected, 0, max(0, len(lines)-1))
}

Delete the old `selectedRenderedRowIndex(rows []string) int` helper after `ensureSelectionVisible`
uses `selectedRenderedLineIndex`.
```

- [ ] **Step 4: Update home/end table behavior**

In `internal/tui/model.go`, replace the table-mode `g/home` and `G/end` raw index assignments with visible-index logic:

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

- [ ] **Step 5: Run focused navigation tests**

Run:

```bash
go test ./internal/tui -run 'TestIssueListNavigationSkipsCollapsedDescendants|TestIssueListRepairSelectionHiddenByCollapsedAncestor|TestIssueListPagingUsesVisibleRows|TestIssueListViewportUsesRenderedTreeRows' -count=1
```

Expected: PASS.

---

### Task 4: Preserve Deeper Collapse State And Expansion Compatibility

**Files:**
- Modify: `internal/tui/navigation.go`
- Test: `internal/tui/issue_list_test.go`

**Interfaces:**
- Consumes: `collapsedIssueKeys map[string]bool`
- Consumes: `func (m *Model) repairCollapsedSelection()`
- Produces: refresh and explicit expansion paths that preserve collapse state.

- [ ] **Step 1: Add failing deeper-state and expansion compatibility tests**

Add this test to `internal/tui/issue_list_test.go`:

```go
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
```

Add this focused model-level test to `internal/tui/issue_list_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify behavior**

Run:

```bash
go test ./internal/tui -run 'TestIssueListExpandParentPreservesDeeperCollapsedBranch|TestIssueListMergeExpandedIssuesPreservesCollapseState' -count=1
```

Expected: PASS once Tasks 1-3 are complete; if either fails, fix only the state mutation path causing collapse state to be cleared.

- [ ] **Step 3: Repair selection after issue replacement**

In `internal/tui/navigation.go`, update `replaceIssues` so any selected key restored from a refresh is repaired if hidden by a preserved collapsed ancestor:

```go
	if selectedKey != "" {
		for index, issue := range m.issues {
			if issue.Key == selectedKey {
				m.selected = index
				m.repairCollapsedSelection()
				m.ensureSelectionVisible(m.currentLayoutRows())
				return
			}
		}
	}
	m.selected = clamp(m.selected, 0, len(m.issues)-1)
	m.repairCollapsedSelection()
	m.ensureSelectionVisible(m.currentLayoutRows())
```

- [ ] **Step 4: Run broader focused issue-list tests**

Run:

```bash
go test ./internal/tui -run 'TestIssueList|TestSelectedParentDoesNotRepeatVisibleChildCount' -count=1
```

Expected: PASS.

---

### Task 5: Documentation And Final Verification

**Files:**
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

**Interfaces:**
- Consumes: implemented `z` issue-table collapse behavior.
- Produces: updated repo docs and completed task review notes.

- [ ] **Step 1: Update project state**

In `docs/project-state.md`, extend the issue-table bullet around explicit parent expansion with:

```markdown
- Issue tables support local subtree collapse with `z`: the selected node can hide or reveal its
  loaded descendants without changing Jira reads, saved views, cache records, issue ordering, or the
  loaded issue data. Collapsed rows show a compact hidden-descendant count, and navigation skips
  hidden rows until the ancestor is expanded.
```

- [ ] **Step 2: Update changelog**

Add this bullet under `## Unreleased` in `docs/releases/CHANGELOG.md`:

```markdown
- Added local issue-table subtree collapse with `z`, so dense loaded ticket branches can be hidden
  or revealed without changing Jira reads or the loaded issue data.
```

- [ ] **Step 3: Update task checklist and review**

In `tasks/todo.md`, mark the `Issue List Subtree Collapse` checklist complete and replace the pending review line with:

```markdown
- Added `z` as a local issue-table subtree collapse/expand toggle.
- Kept Jira reads, explicit `x`/`X` child loading, saved views, caches, issue ordering, and
  `m.issues` unchanged.
- Added visible-row projection so rendering, navigation, paging, and selection visibility honor
  collapsed branches.
- Preserved deeper collapsed branches when expanding an ancestor.
- Rendered compact hidden-descendant counts on collapsed rows.
- Verified with focused issue-list/navigation tests, full Go tests, `make check`, and
  `make install-user`.
```

- [ ] **Step 4: Format touched Go files**

Run:

```bash
gofmt -w internal/tui/model.go internal/tui/keymap.go internal/tui/issue_list.go internal/tui/navigation.go internal/tui/issue_list_test.go
```

Expected: command exits 0 and only touched Go files are reformatted.

- [ ] **Step 5: Run focused tests**

Run:

```bash
go test ./internal/tui -run 'TestIssueList|TestSelectedParentDoesNotRepeatVisibleChildCount|TestHelpIncludesActiveContextBindings' -count=1
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
git diff -- docs/superpowers/specs/2026-06-17-issue-list-subtree-collapse-design.md docs/superpowers/plans/2026-06-17-issue-list-subtree-collapse.md tasks/todo.md docs/project-state.md docs/releases/CHANGELOG.md internal/tui/model.go internal/tui/keymap.go internal/tui/issue_list.go internal/tui/navigation.go internal/tui/issue_list_test.go
```

Expected: diff is limited to the planned files and contains no Jira worker/cache/data-fetch changes.
