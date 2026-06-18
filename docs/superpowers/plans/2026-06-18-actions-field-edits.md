# Actions Field Edits Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the Actions section route to existing metadata-backed Summary and Priority edit workflows.

**Architecture:** Keep the change inside `internal/tui/detail.go` and adjacent `internal/tui/detail_test.go`. Reuse existing `startSummaryEditor` and `startPriorityEditor` flows, including metadata cache and worker behavior.

**Tech Stack:** Go, Bubble Tea model tests, existing TUI detail action helpers.

## Global Constraints

- Do not add Jira APIs or worker result types.
- Do not change Summary/Priority modal behavior.
- Keep existing shortcuts and focused-field behavior unchanged.
- Keep `Create Subtask` disabled until its metadata-backed workflow exists.

---

### Task 1: Failing Actions Tests

**Files:**
- Modify: `internal/tui/detail_test.go`

**Interfaces:**
- Consumes: `detailActions`, `runSelectedDetailAction`, `startSummaryEditor`, `startPriorityEditor`.
- Produces: tests proving Actions has concrete Summary/Priority rows and routes to the existing metadata-backed flows.

- [x] Update the Actions rendering test to expect `Edit Summary` and `Change Priority`, and reject generic `Edit Fields`.
- [x] Add a test that selecting `Edit Summary` starts summary metadata loading.
- [x] Add a test that selecting `Change Priority` starts priority metadata loading.
- [x] Run focused tests and confirm they fail before implementation.

### Task 2: Actions Field Edit Routing

**Files:**
- Modify: `internal/tui/detail.go`

**Interfaces:**
- Consumes: failing tests from Task 1.
- Produces: `detailAction` rows with IDs `summary` and `priority` that route to existing edit flows.

- [x] Replace the disabled `edit-fields` row with enabled `summary` and `priority` rows.
- [x] Route `summary` to `startSummaryEditor`.
- [x] Route `priority` to `startPriorityEditor`.
- [x] Run focused Actions tests and `go test ./internal/tui -count=1`.

### Task 3: Docs And Verification

**Files:**
- Modify: `docs/backlog.md`
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

**Interfaces:**
- Consumes: completed Actions routing.
- Produces: docs showing Summary/Priority are metadata-backed Actions entries.

- [x] Update backlog wording to narrow the remaining Actions work.
- [x] Document the Actions section field edit rows in project state and changelog.
- [x] Mark `tasks/todo.md` with verification evidence.
- [x] Run focused tests, `go test ./... -count=1`, `make check`, and `make install-user`.
