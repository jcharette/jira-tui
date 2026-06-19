# Bounded Agile Loading Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bound Jira Agile board/sprint metadata loading before sprint/board UX expands.

**Architecture:** Keep board discovery single-flight, then schedule returned board sprint reads
through a small TUI-owned queue with a fixed concurrent request cap. Worker pool submissions remain
typed background requests.

**Tech Stack:** Go, Bubble Tea v2, existing TUI model, existing worker pool, Jira Agile client.

## Global Constraints

- Do not change issue JQL, saved views, cache keys, or loaded issues.
- Do not eagerly bypass the worker pool.
- Do not start unbounded board or sprint worker requests.
- Keep Diagnostics on the existing worker/API result path.

---

### Task 1: Sprint Scheduler State

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/results.go`
- Test: `internal/tui/issue_list_test.go`

- [x] Add failing tests that multiple returned boards only start the configured number of sprint
  reads and leave the rest queued.
- [x] Add TUI model state for queued board IDs and active sprint request IDs.
- [x] Start queued sprint work through existing worker commands.

### Task 2: Completion Draining

**Files:**
- Modify: `internal/tui/results.go`
- Test: `internal/tui/issue_list_test.go`

- [x] Add failing tests that a completed sprint read stores the page and starts the next queued
  board without exceeding the limit.
- [x] Teach sprint result handling to drain one queued request per completed active request.

### Task 3: Docs And Verification

**Files:**
- Modify: `docs/backlog.md`
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

- [x] Update docs and review notes.
- [x] Run focused tests, `go test ./... -count=1`, `make check`, and `make install-user`.
