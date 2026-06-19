# Comment Editing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add explicit, worker-backed Jira comment editing from focused ticket detail.

**Architecture:** Reuse the existing comment composer for edit drafts and review confirmation. Add a
worker/client update-comment path and a comments-section focus state for selecting which loaded
comment to edit.

**Tech Stack:** Go, Bubble Tea v2, existing TUI comment composer, worker pool, go-atlassian Jira ADF
comment service.

## Global Constraints

- Keep add-comment behavior available from Comments.
- Keep Jira writes behind explicit review/confirmation.
- Keep comment reads/writes worker-backed.
- Reuse existing comment ADF conversion for links, formatting, and selected mentions.
- Invalidate comment caches after successful updates.

---

### Task 1: Jira Client And Worker Update Comment

**Files:**
- Modify: `internal/jira/client.go`
- Modify: `internal/jira/client_test.go`
- Modify: `internal/worker/pool.go`
- Modify: `internal/worker/pool_test.go`

- [x] Add failing Jira client test for `UpdateComment` using the ADF update payload.
- [x] Add failing worker test for `KindUpdateComment`.
- [x] Add `UpdateComment(ctx, issueKey, commentID, body, mentions)` to the Jira client.
- [x] Add worker request/result structs and handler for comment updates.

### Task 2: TUI Comment Selection And Edit Flow

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/detail.go`
- Modify: `internal/tui/comment.go`
- Modify: `internal/tui/results.go`
- Modify: `internal/tui/commands.go`
- Modify: `internal/tui/comment_test.go`
- Modify: `internal/tui/detail_test.go`

- [x] Add failing tests for focusing/selecting comments and opening edit composer with `e`.
- [x] Add failing tests for confirming and submitting an edited comment.
- [x] Track comments focus, selected comment index, edit issue key, edit comment ID, and original body.
- [x] Render selected comment marker and comments-section key help.
- [x] Route `e` to prefilled edit composer and `enter` to add-comment composer.
- [x] Submit update-comment worker requests from edit mode.
- [x] Handle update result by invalidating caches and refreshing comments.

### Task 3: Docs And Verification

**Files:**
- Modify: `docs/backlog.md`
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

- [x] Remove completed comment-editing backlog item.
- [x] Document comment editing behavior and UX review notes.
- [x] Run focused tests, `go test ./... -count=1`, `make check`, and `make install-user`.
