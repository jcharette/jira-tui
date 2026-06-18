# Comments And Workflow Actions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve daily Jira usage by adding comment formatting controls and metadata-backed transition field handling.

**Architecture:** Keep Jira IO on the existing worker path. Extend the Jira client domain types for transition fields and transition field values, then let the TUI render a small form only when selected transitions require supported fields. Keep comment formatting as visible text tokens converted to ADF at submit time.

**Tech Stack:** Go, Bubble Tea v2, Bubbles textarea, go-atlassian Jira Cloud v3, existing worker pool and TUI detail workflows.

## Global Constraints

- Do not add Jira IO directly to the Bubble Tea update/render loop.
- Do not add a generic custom-field framework in this slice.
- Preserve existing comments, mentions, link detection, transition list cache, and stale-result behavior.
- Update `docs/backlog.md`, `docs/project-state.md`, `docs/releases/CHANGELOG.md`, and `tasks/todo.md` when behavior lands.
- Verify with focused tests, `go test ./... -count=1`, `make check`, and `make install-user`.

## Task 1: Transition Field Metadata And Submit Values

**Files:**
- Modify: `internal/jira/client.go`
- Modify: `internal/jira/client_test.go`
- Modify: `internal/worker/pool.go`
- Modify: `internal/worker/pool_test.go`
- Modify: `internal/tui/model_test.go`
- Modify: `internal/tui/detail_test.go`

**Interfaces:**
- Produces: `jira.TransitionField`, `jira.TransitionFieldValue`, `jira.TransitionIssueRequest`.
- Produces: `TransitionIssue(ctx, key string, request jira.TransitionIssueRequest) error`.
- Produces: worker `TransitionIssueRequest.Fields []jira.TransitionFieldValue`.

- [ ] Write failing Jira client tests for parsing transition fields and submitting resolution/comment payloads.
- [ ] Run focused Jira tests and confirm the expected failures.
- [ ] Extend Jira transition domain types and lower-level metadata fetch for `expand=transitions.fields`.
- [ ] Extend transition submit payload building for Resolution and Comment fields.
- [ ] Run focused Jira tests and confirm they pass.
- [ ] Write failing worker tests for field value propagation.
- [ ] Extend worker request/result handling and fakes.
- [ ] Run focused worker tests and confirm they pass.
- [ ] Write failing TUI tests for unsupported required fields and required field form submission.
- [ ] Add TUI transition field form state, validation, rendering, commands, and result cleanup.
- [ ] Run focused TUI transition tests and confirm they pass.

## Task 2: Comment Formatting Controls And ADF Marks

**Files:**
- Modify: `internal/jira/client.go`
- Modify: `internal/jira/client_test.go`
- Modify: `internal/tui/comment.go`
- Modify: `internal/tui/comment_test.go`
- Modify: `internal/tui/keymap.go`

**Interfaces:**
- Produces: ADF `strong`, `em`, and `code` marks from visible comment tokens.
- Produces: composer shortcuts for bold, italic, inline code, and bullet insertion.

- [ ] Write failing Jira client tests for bold, italic, and inline-code ADF marks alongside links and mentions.
- [ ] Run focused Jira comment tests and confirm the expected failures.
- [ ] Extend comment ADF inline parsing while preserving link and mention precedence.
- [ ] Run focused Jira comment tests and confirm they pass.
- [ ] Write failing TUI tests for formatting keyboard controls.
- [ ] Add composer keyboard handlers and keymap entries.
- [ ] Run focused TUI comment tests and confirm they pass.

## Task 3: Docs, Backlog, And Verification

**Files:**
- Modify: `docs/backlog.md`
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

- [ ] Remove completed Comments/Workflow backlog items or narrow remaining follow-ups.
- [ ] Document supported transition fields and comment formatting behavior.
- [ ] Record review notes and verification commands in `tasks/todo.md`.
- [ ] Run `go test ./... -count=1`.
- [ ] Run `make check`.
- [ ] Run `make install-user`.
- [ ] Commit, merge to `main`, push, and verify CI.
