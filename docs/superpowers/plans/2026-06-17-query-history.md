# Query History Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist confirmed direct and AI-generated JQL queries between app executions and let users select recent queries from the query modal.

**Architecture:** Add a `query_history` table to the existing SQLite cache store, expose it through the existing active-view store boundary, and add a `Recent` mode to the existing query modal. Confirmed direct and generated JQL runs write history; selecting a recent query loads it into the JQL editor for review or execution.

**Tech Stack:** Go, Bubble Tea v2, existing SQLite cache store, existing query modal.

## Global Constraints

- Do not bypass the worker-backed issue search path.
- Do not write query history for unconfirmed AI previews.
- Do not put query history in config; config remains for intentional saved views.
- Keep history profile/site scoped by namespace.
- Keep the implementation in cohesive existing packages and same-package TUI files.

---

### Task 1: Cache Query History

**Files:**
- Modify: `internal/cache/store.go`
- Modify: `internal/cache/store_test.go`

**Deliverable:** SQLite stores and lists deduped query history records by namespace and normalized JQL cache key.

- [x] Add failing cache tests for persisting direct and AI query history.
- [x] Add query history schema and store methods.
- [x] Run focused cache tests.

### Task 2: TUI Recording And Hydration

**Files:**
- Modify: `internal/tui/view_cache.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/query.go`
- Modify: `internal/tui/query_test.go`
- Modify: `internal/tui/issue_list_test.go`

**Deliverable:** Confirmed direct and generated query runs record history, and opening the query modal loads persisted recents.

- [x] Add failing TUI tests for recording direct/AI confirmed runs and hydrating recents.
- [x] Extend the store interface and fake store.
- [x] Record confirmed query runs with source and prompt metadata.
- [x] Load recent query history when opening the modal.
- [x] Run focused query tests.

### Task 3: Recent Picker UX

**Files:**
- Modify: `internal/tui/query.go`
- Modify: `internal/tui/keymap.go`
- Modify: `internal/tui/query_test.go`

**Deliverable:** Query modal has a `Recent` mode where users can select a persisted query, load it into the JQL editor, or run it explicitly.

- [x] Add failing tests for Recent mode navigation and selection.
- [x] Add `queryModeRecent`, recent selection state, rendering, and key handling.
- [x] Update query modal footer/help copy.
- [x] Run focused query tests.

### Task 4: Docs And Verification

**Files:**
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

**Deliverable:** User-facing docs and task plan describe persisted recent queries.

- [x] Update docs and review notes.
- [x] Run focused tests, `go test ./... -count=1`, `make check`, and `make install-user`.
