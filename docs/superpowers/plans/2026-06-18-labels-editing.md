# Labels Editing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add metadata-backed Jira Labels editing from focused ticket detail.

**Architecture:** Extend the existing Summary/Priority edit flow pattern with a labels-specific
editor and worker request. The implementation remains metadata-gated and action-surface-only.

**Tech Stack:** Go, Bubble Tea v2, Bubbles `textarea`, existing Jira client/worker/TUI patterns.

## Global Constraints

- Keep Jira writes worker-backed.
- Do not add a generic edit-all-fields framework.
- Do not add label autocomplete or new Jira reads beyond edit metadata.

---

### Task 1: Client And Worker Labels Update

**Files:**
- Modify: `internal/jira/client.go`
- Modify: `internal/jira/client_test.go`
- Modify: `internal/worker/pool.go`
- Modify: `internal/worker/pool_test.go`

**Interfaces:**
- Produces: `jira.EditMetadata.Labels`
- Produces: `UpdateLabels(ctx context.Context, key string, labels []string) error`
- Produces: `worker.KindUpdateLabels`, `UpdateLabelsRequest`, `UpdateLabelsResult`

- [x] **Step 1: Write failing client and worker tests.**
- [x] **Step 2: Run focused tests and verify RED.**
- [x] **Step 3: Implement metadata parsing, Jira update payload, and worker routing.**
- [x] **Step 4: Run focused tests and verify GREEN.**

### Task 2: TUI Labels Editor

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/keymap.go`
- Modify: `internal/tui/detail.go`
- Create: `internal/tui/labels.go`
- Modify: `internal/tui/results.go`
- Modify: `internal/tui/commands.go`
- Modify: `internal/tui/navigation.go`
- Modify: `internal/tui/detail_test.go`

**Interfaces:**
- Consumes: `worker.KindUpdateLabels`
- Produces: `startLabelsEditor()`, `updateLabelsEditor(tea.KeyMsg)`, `submitSelectedLabels()`

- [x] **Step 1: Write failing TUI tests.**
- [x] **Step 2: Run focused TUI tests and verify RED.**
- [x] **Step 3: Implement action row, modal, update handling, submit command, and success patching.**
- [x] **Step 4: Run focused TUI tests and verify GREEN.**

### Task 3: Docs And Full Verification

**Files:**
- Modify: `tasks/todo.md`
- Modify: `docs/backlog.md`
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`

- [x] **Step 1: Update docs for Labels editing behavior.**
- [x] **Step 2: Run `go test ./... -count=1`.**
- [x] **Step 3: Run `make check`.**
- [x] **Step 4: Run `make install-user`.**
- [x] **Step 5: Record verification evidence.**
