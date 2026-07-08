# Board Hygiene Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent invisible Jira board work and add `jira ticket check-board` audit/fix tooling.

**Architecture:** Add a tiny shared board-hygiene package over existing `jira.Issue` values, then expose it from the TUI and CLI. Reuse existing search, child expansion, sprint move, assignee update, and metadata-backed edit APIs.

**Tech Stack:** Go, Cobra, Bubble Tea, existing Jira client/worker APIs.

## Global Constraints

- Keep diffs minimal and reuse existing Jira client paths.
- Default fixes must show proposed changes and prompt `Apply these fixes? [y/N]`.
- `--yes` is only for non-interactive/scripted fix mode.
- Do not assume issue-type conversion is impossible; attempt metadata-backed conversion first and fall back to replacement/manual guidance if Jira rejects it.

---

### Task 1: Shared Board Hygiene Checker

**Files:**
- Create: `internal/boardcheck/boardcheck.go`
- Test: `internal/boardcheck/boardcheck_test.go`

**Deliverable:** Pure functions classify bad board shape without Jira IO.

- [ ] Add tests for Epic-owned Sub-task, unassigned in-progress ticket, and clean Story/Task.
- [ ] Implement `Finding`, `CheckIssue`, and helpers.
- [ ] Run `go test ./internal/boardcheck -count=1`.

### Task 2: TUI Prevention And Warnings

**Files:**
- Modify: `internal/tui/detail.go`
- Modify: `internal/tui/detail_test.go`

**Deliverable:** `Create Subtask` is blocked on Epics and selected detail/hierarchy can show board hygiene warnings.

- [ ] Add a failing test that an Epic disables/blocks Create Subtask.
- [ ] Add a focused render test for board hygiene warning text.
- [ ] Implement minimal calls to `boardcheck`.
- [ ] Run focused `go test ./internal/tui -run 'Test.*Subtask|Test.*Board' -count=1`.

### Task 3: CLI Audit

**Files:**
- Create: `internal/app/board_check.go`
- Modify: `internal/app/toil.go`
- Modify: `internal/app/toil_test.go`

**Deliverable:** `jira ticket check-board [KEY]` audits one ticket or current user's in-progress tickets.

- [ ] Add command exposure/help tests.
- [ ] Add audit tests for single key and no-key current-user JQL.
- [ ] Implement the Cobra command and output.
- [ ] Run `go test ./internal/app -run 'Test.*CheckBoard|Test.*Ticket' -count=1`.

### Task 4: Prompted Fixes

**Files:**
- Modify: `internal/app/board_check.go`
- Modify: `internal/app/toil_test.go`

**Deliverable:** `--fix` prompts, `--yes` skips prompt, non-interactive fix without `--yes` refuses.

- [ ] Add tests for confirmation prompt, no response, and `--yes`.
- [ ] Fix assignee via `CurrentUser` + `UpdateAssignee`.
- [ ] Fix active sprint via configured/default active sprint when available.
- [ ] Attempt metadata-backed Sub-task conversion when edit metadata permits; otherwise print guided fallback.
- [ ] Run focused app tests and `go test ./... -count=1`.

### Task 5: Docs And Final Verification

**Files:**
- Modify: `README.md`
- Modify: `docs/keyboard.md`
- Modify: `tasks/todo.md`

**Deliverable:** User-facing docs mention the audit/fix flow.

- [ ] Update docs minimally.
- [ ] Mark this plan complete in `tasks/todo.md`.
- [ ] Run `make docs-check`, `go test ./... -count=1`, and `make check`.
