# Subtask Creation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add metadata-backed subtask creation from the selected ticket.

**Architecture:** Reuse the existing create-ticket modal and worker-backed Jira create flow. Add a
parent key to create requests, filter issue-type selection to Jira subtask types when a parent is
present, and enable the Actions tab `Create Subtask` row.

**Tech Stack:** Go, Bubble Tea v2, existing TUI create form, worker pool, go-atlassian Jira client.

## Global Constraints

- Keep normal `n` ticket creation unchanged.
- Keep Jira writes behind explicit `ctrl+s` submit.
- Do not hard-code subtask issue type names; rely on Jira metadata `Subtask`.
- Keep create metadata and create writes worker-backed.
- Preserve existing supported-field and unsupported-required-field validation.

---

### Task 1: Parent-Aware Create Payload

**Files:**
- Modify: `internal/jira/client.go`
- Modify: `internal/jira/client_test.go`
- Modify: `internal/worker/pool.go`
- Modify: `internal/worker/pool_test.go`

- [x] Add failing Jira client test that `CreateIssueRequest.ParentKey` sends `fields.parent.key`.
- [x] Add failing worker test that create requests preserve `ParentKey`.
- [x] Add `ParentKey` to Jira and worker create request structs.
- [x] Set `model.ParentScheme{Key: parentKey}` only when parent key is provided.

### Task 2: Subtask Create UX

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/detail.go`
- Modify: `internal/tui/create_issue.go`
- Modify: `internal/tui/create_issue_test.go`
- Modify: `internal/tui/detail_test.go`

- [x] Add failing TUI test that Actions `Create Subtask` opens the create modal for the selected
  parent issue.
- [x] Add failing TUI test that subtask creation filters the issue type picker to subtask metadata.
- [x] Track create parent key/summary in model state and reset it with create state.
- [x] Add `startCreateSubtask` and route the Actions row to it.
- [x] Filter selectable issue types to subtask types only when parent key is set.
- [x] Submit `ParentKey` with create issue requests.

### Task 3: Docs And Verification

**Files:**
- Modify: `docs/backlog.md`
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

- [x] Update Creation/Editing backlog and project state.
- [x] Add changelog entry.
- [x] Add task review notes with UX review.
- [x] Run focused tests, `go test ./... -count=1`, `make check`, and `make install-user`.
