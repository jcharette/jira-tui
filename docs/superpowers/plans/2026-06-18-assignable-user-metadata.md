# Assignable User Metadata Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make assignee editing search only users assignable to the selected Jira issue.

**Architecture:** Add an issue-scoped Jira client method, route it through the existing worker
`SearchUsers` request via an optional issue key, and update the TUI assignee picker to use
issue-scoped cache keys.

**Tech Stack:** Go, Bubble Tea v2, existing worker pool, Jira Cloud REST API.

## Global Constraints

- Do not change mention search behavior.
- Do not submit Jira writes until the user selects a user and confirms with enter.
- Do not share global user search cache entries with issue-scoped assignee search entries.
- Keep assignee picker rendering and key bindings unchanged in this slice.

---

### Task 1: Jira Client

**Files:**
- Modify: `internal/jira/client.go`
- Modify: `internal/jira/client_test.go`

- [x] Add failing tests for issue-scoped assignable-user search endpoint and parsing.
- [x] Implement `SearchAssignableUsers(ctx, issueKey, query, maxResults)`.

### Task 2: Worker Routing

**Files:**
- Modify: `internal/worker/pool.go`
- Modify: `internal/worker/pool_test.go`

- [x] Add failing tests that `SearchUsersRequest.IssueKey` routes to assignable search.
- [x] Extend the worker client interface and search handler.

### Task 3: TUI Assignee Picker

**Files:**
- Modify: `internal/tui/detail.go`
- Modify: `internal/tui/results.go`
- Modify: `internal/tui/commands.go`
- Modify: `internal/tui/detail_test.go`

- [x] Add failing tests that assignee picker searches with the selected issue key.
- [x] Add failing tests that global user-search cache entries are not reused for assignee search.
- [x] Implement issue-scoped assignee cache helpers and route picker searches through assignable
  worker requests.

### Task 4: Docs And Verification

**Files:**
- Modify: `docs/backlog.md`
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

- [x] Update docs and review notes.
- [x] Run focused tests, `go test ./... -count=1`, `make check`, and `make install-user`.
