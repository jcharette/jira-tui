# Hierarchy Context Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the focused detail Hierarchy workspace show only hierarchy context now that Jira issue links live in Links.

**Architecture:** Keep changes inside existing `internal/tui/detail.go` hierarchy rendering helpers and adjacent tests in `internal/tui/detail_test.go`. Update docs and task records after behavior is verified.

**Tech Stack:** Go, Bubble Tea model tests, existing TUI detail rendering helpers.

## Global Constraints

- Do not add a Jira read path or worker family.
- Do not change Links workspace behavior.
- Keep hierarchy grouped rows and open-selected-child behavior unchanged.
- Preserve current same-package TUI boundaries.

---

### Task 1: Failing Hierarchy Context Tests

**Files:**
- Modify: `internal/tui/detail_test.go`

**Interfaces:**
- Consumes: `Model.render`, `Model.fullDetailContent`, existing `focusDetailSectionForTest`.
- Produces: tests proving Hierarchy no longer renders the linked-issues placeholder and uses clearer empty-state copy.

- [x] Update grouped hierarchy rendering test to reject `Linked Issues` and `Linked issue data is not loaded yet.`
- [x] Add an empty parent-context test for a child issue whose parent is known but no children are loaded.
- [x] Run focused hierarchy tests and confirm they fail before implementation.

### Task 2: Hierarchy Rendering Cleanup

**Files:**
- Modify: `internal/tui/detail.go`

**Interfaces:**
- Consumes: failing tests from Task 1.
- Produces: `renderHierarchySection` without the stale linked-issues placeholder.

- [x] Remove `renderLinkedIssuesPlaceholder` calls from Hierarchy rendering.
- [x] Replace the no-row empty state with copy that distinguishes root issues from known-parent issues.
- [x] Remove the unused placeholder helper.
- [x] Run focused hierarchy tests and `go test ./internal/tui -count=1`.

### Task 3: Docs And Verification

**Files:**
- Modify: `docs/backlog.md`
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

**Interfaces:**
- Consumes: completed hierarchy cleanup.
- Produces: docs showing the focused-detail hierarchy cleanup is complete.

- [x] Update backlog wording to narrow the remaining focused-detail workspace item.
- [x] Document the Hierarchy/Links separation in project state and changelog.
- [x] Mark `tasks/todo.md` with verification evidence.
- [x] Run full verification: focused tests, `go test ./... -count=1`, `make check`, and `make install-user`.
