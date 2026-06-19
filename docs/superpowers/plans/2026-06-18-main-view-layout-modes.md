# Main View Layout Modes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add local `Table`, `Workbench`, and `Lanes` layout modes for the starting page without changing Jira queries or saved views.

**Architecture:** Add a small layout-mode enum to the TUI model, render the same loaded issue rows through mode-specific renderers, and keep the polished table/control strip shared. Workbench uses a responsive side panel only when width allows; Lanes groups the same visible issues by status.

**Tech Stack:** Go, Bubble Tea v2, Lip Gloss, existing `internal/tui` issue list renderer and key binding model.

## Global Constraints

- Layout mode is local presentation state only.
- Switching layout mode must not change `jql`, saved view, loaded issues, active filter, caches, or Jira requests.
- Workbench preview must use only already-loaded issue data or retained/cached detail/comment state.
- Lanes groups the current visible loaded issue set; it does not fetch unrelated tickets.
- Keep `enter` as open selected issue.

---

### Task 1: Layout Mode State And Controls

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/keymap.go`
- Modify: `internal/tui/chrome.go`
- Modify: `internal/tui/issue_list.go`
- Modify: `internal/tui/issue_list_test.go`

**Interfaces:**
- Produces: `type issueLayoutMode int`
- Produces: `issueLayoutTable`, `issueLayoutWorkbench`, `issueLayoutLanes`
- Produces: `func (m *Model) cycleIssueLayoutMode()`
- Produces: `func (m Model) issueLayoutModeLabel() string`

- [x] Add failing tests that pressing `L` cycles `Table -> Workbench -> Lanes -> Table` without changing `jql`, `view`, `issues`, or `statusFilter`.
- [x] Add failing render test that the issue-list control strip includes `View`, `Filter`, `Layout`, and `Sort` chips.
- [x] Implement layout mode state, key binding, footer/help label, and compact control strip.
- [x] Run focused tests for layout cycling and control strip rendering.

### Task 2: Workbench Layout

**Files:**
- Modify: `internal/tui/issue_list.go`
- Modify: `internal/tui/issue_list_test.go`

**Interfaces:**
- Produces: `func (m Model) renderIssueWorkbench(layout browserLayout) string`
- Produces: `func (m Model) renderSelectedIssueContextPanel(width int) string`

- [x] Add failing tests that Workbench renders a context panel on wide terminals.
- [x] Add failing tests that Workbench does not repeat selected-row key or summary in the panel.
- [x] Add failing tests that Workbench falls back to the table renderer on normal/narrow widths.
- [x] Implement responsive Workbench rendering using loaded/cached detail and comments only.
- [x] Run focused Workbench rendering tests.

### Task 3: Lanes Layout

**Files:**
- Modify: `internal/tui/issue_list.go`
- Modify: `internal/tui/issue_list_test.go`

**Interfaces:**
- Produces: `func (m Model) renderIssueLanes(layout browserLayout) string`
- Produces: `func (m Model) issueLaneGroups(layout browserLayout) []issueLaneGroup`

- [x] Add failing tests that Lanes groups current visible issues by status and shows counts.
- [x] Add failing tests that Lanes respects the local Active filter.
- [x] Add failing tests that `enter` still opens the selected issue in Lanes mode.
- [x] Implement lane grouping over visible issue indexes without changing selection semantics.
- [x] Run focused Lanes rendering/navigation tests.

### Task 4: Docs And Verification

**Files:**
- Modify: `tasks/todo.md`
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `docs/backlog.md`

- [x] Update docs for local layout modes and the polished starting-page control strip.
- [x] Run `go test ./internal/tui -count=1`.
- [x] Run `go test ./... -count=1`.
- [x] Run `make check`.
- [x] Run `make install-user`.
- [x] Run `git diff --check`.
