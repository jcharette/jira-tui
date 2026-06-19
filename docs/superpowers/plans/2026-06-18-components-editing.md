# Components Editing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add metadata-backed Jira Components editing from focused ticket detail.

**Architecture:** Extend the existing edit metadata and worker update pattern with a components-specific
multi-select picker. The implementation remains metadata-gated and action-surface-only.

**Tech Stack:** Go, Bubble Tea v2, Bubbles `textinput`, existing Jira client/worker/TUI patterns.

## Global Constraints

- Keep Jira writes worker-backed.
- Do not add a generic edit-all-fields framework.
- Use only Jira edit metadata allowed values for component options.

---

### Task 1: Client And Worker Components Update

**Files:**
- Modify: `internal/jira/client.go`
- Modify: `internal/jira/client_test.go`
- Modify: `internal/worker/pool.go`
- Modify: `internal/worker/pool_test.go`

**Interfaces:**
- Produces: `jira.EditMetadata.Components`
- Produces: `UpdateComponents(ctx context.Context, key string, components []jira.FieldOption) error`
- Produces: `worker.KindUpdateComponents`, `UpdateComponentsRequest`, `UpdateComponentsResult`

- [x] **Step 1: Write failing client and worker tests.**
- [x] **Step 2: Run focused tests and verify RED.**
- [x] **Step 3: Implement metadata parsing, Jira update payload, and worker routing.**
- [x] **Step 4: Run focused tests and verify GREEN.**

### Task 2: TUI Components Picker

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/keymap.go`
- Modify: `internal/tui/detail.go`
- Create: `internal/tui/components.go`
- Modify: `internal/tui/results.go`
- Modify: `internal/tui/commands.go`
- Modify: `internal/tui/navigation.go`
- Modify: `internal/tui/detail_test.go`

**Interfaces:**
- Consumes: `worker.KindUpdateComponents`
- Produces: `startComponentsEditor()`, `updateComponentsEditor(tea.KeyMsg)`, `submitSelectedComponents()`

- [x] **Step 1: Write failing TUI tests.**
- [x] **Step 2: Run focused TUI tests and verify RED.**
- [x] **Step 3: Implement action row, modal, filtering, toggling, submit command, and success patching.**
- [x] **Step 4: Run focused TUI tests and verify GREEN.**

### Task 3: Docs And Full Verification

**Files:**
- Modify: `tasks/todo.md`
- Modify: `docs/backlog.md`
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`

- [x] **Step 1: Update docs for Components editing behavior.**
- [x] **Step 2: Run `go test ./... -count=1`.**
- [x] **Step 3: Run `make check`.**
- [x] **Step 4: Run `make install-user`.**
- [x] **Step 5: Record verification evidence.**
