# Ticket Action Palette Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a searchable ticket action palette in focused ticket detail.

**Architecture:** Keep the palette as a same-package TUI file that reuses `detailActions()` and
`runSelectedDetailAction()`. The feature changes only local UX state and key handling; it does not
add Jira IO.

**Tech Stack:** Go, Bubble Tea v2, Bubbles `textinput`, existing TUI render helpers.

## Global Constraints

- Keep existing detail shortcuts and inline Actions behavior working.
- Do not add Jira query/API paths for this UX-only slice.
- Keep the diff scoped to TUI state, keymap, tests, and docs.

---

### Task 1: Palette Behavior

**Files:**
- Modify: `internal/tui/detail_test.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/keymap.go`
- Create: `internal/tui/action_palette.go`
- Modify: `internal/tui/issue_list_test.go`

**Interfaces:**
- Consumes: `detailActions() []detailAction`, `runSelectedDetailAction() (Model, tea.Cmd)`
- Produces: `openActionPalette()`, `updateActionPalette(tea.KeyMsg) (Model, tea.Cmd)`,
  `renderActionPalette(browserLayout) string`

- [x] **Step 1: Write failing tests**

Add TUI tests that press `.`, assert `Ticket Actions` renders, type a filter, and run the filtered
Priority action.

- [x] **Step 2: Verify RED**

Run: `go test ./internal/tui -run 'TestActionPalette' -count=1`
Expected: build failure for missing palette fields and methods.

- [x] **Step 3: Implement minimal palette**

Add model state, a key context, filter input, renderer, update handler, and filtered-row dispatch
through the existing detail action runner.

- [x] **Step 4: Verify focused tests**

Run: `go test ./internal/tui -run 'TestActionPalette' -count=1`
Expected: PASS.

- [x] **Step 5: Verify full repo**

Run: `go test ./... -count=1`, `make check`, and `make install-user`.

### Task 2: Docs

**Files:**
- Modify: `tasks/todo.md`
- Modify: `docs/backlog.md`
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`

**Interfaces:**
- Consumes: verified UX behavior from Task 1.
- Produces: updated backlog and release notes.

- [x] **Step 1: Record task review**

Add the Ticket Action Palette scope and UX notes to `tasks/todo.md`.

- [x] **Step 2: Move completed backlog item**

Remove the command-palette line from `docs/backlog.md` and add a matching changelog entry.

- [x] **Step 3: Record verification evidence**

After full verification, mark the remaining checkbox complete in `tasks/todo.md`.
