# View Creation UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make saved issue views easy to create and manage from the existing query modal.

**Architecture:** Generalize the current recent-query save prompt so it can save any query source.
Add template and view-management modes to the query modal, and persist full view-list edits through
the existing config save path.

**Tech Stack:** Go, Bubble Tea v2, existing config TOML helpers, existing TUI query modal.

## Global Constraints

- Do not add Jira requests for create/manage actions.
- Do not change active JQL, selected view, loaded issues, or caches when saving/managing views.
- Keep view persistence in config; recent query history remains cache-backed.

---

### Task 1: Save Prompt Sources

**Files:**
- Modify: `internal/tui/query_test.go`
- Modify: `internal/tui/query.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/keymap.go`

- [ ] Write failing tests for current-query, direct-draft, and AI-preview saved-view creation.
- [ ] Implement generalized save prompt state with JQL and include-children metadata.
- [ ] Add `v` table entry and `ctrl+v` query entry without changing query execution behavior.

### Task 2: Templates

**Files:**
- Modify: `internal/tui/query_test.go`
- Modify: `internal/tui/query.go`

- [ ] Write failing tests for template selection and save behavior.
- [ ] Add a `Templates` query mode with project-scoped starter views.
- [ ] Save selected templates through the generalized save prompt.

### Task 3: Manage Views

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/tui/query_test.go`
- Modify: `internal/tui/query.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/app/app.go`

- [ ] Write failing tests for replacing the saved-view list and TUI rename/reorder/delete/toggle.
- [ ] Add a config helper and TUI writer for full saved-view list persistence.
- [ ] Add a `Views` query mode for rename, reorder, delete, and include-children toggles.

### Task 4: Docs And Verification

**Files:**
- Modify: `tasks/todo.md`
- Modify: `docs/project-state.md`
- Modify: `docs/backlog.md`
- Modify: `docs/releases/CHANGELOG.md`

- [ ] Update docs and changelog.
- [ ] Run focused tests, `go test ./... -count=1`, `make check`, and `make install-user`.
