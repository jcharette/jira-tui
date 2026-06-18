# Linked Issue Links Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show Jira issue links in the focused ticket detail Links workspace without adding Jira writes or a new query flow.

**Architecture:** Parse Jira `issuelinks` inside `internal/jira` issue detail conversion, carry them on `jira.IssueDetail`, and adapt `internal/tui/detail.go` to merge Jira issue links with detected description links. Keep all UI behavior inside the existing Links section and keymap.

**Tech Stack:** Go, go-atlassian Jira models, Bubble Tea model tests, existing TUI detail rendering helpers.

## Global Constraints

- Keep Jira IO on the existing issue-detail worker path.
- Keep one clear semantic key path: `l` focuses Links, `j/k` selects rows, `enter`/`o` opens, and `y` copies.
- Keep issue-link support read-only.
- Do not change saved views, JQL, hierarchy expansion, or Actions menu behavior.

---

### Task 1: Jira Issue Link Parsing

**Files:**
- Modify: `internal/jira/client.go`
- Modify: `internal/jira/client_test.go`

**Interfaces:**
- Produces: `jira.IssueLink` and `IssueDetail.IssueLinks []IssueLink`
- Consumes: `model.IssueLinkScheme` from go-atlassian

- [x] Add a failing parser test that builds an issue detail with one outward and one inward issue link.
- [x] Verify the test fails because `IssueDetail` does not expose parsed issue links.
- [x] Add `IssueLink` and parse inward/outward links with relationship, direction, linked issue key, summary, status, issue type, and URL.
- [x] Run the focused Jira parser test and `go test ./internal/jira -count=1`.

### Task 2: Links Workspace Rendering And Actions

**Files:**
- Modify: `internal/tui/detail.go`
- Modify: `internal/tui/detail_test.go`

**Interfaces:**
- Consumes: `jira.IssueDetail.IssueLinks`
- Produces: merged `detailLink` rows where issue links open URLs and copy issue keys.

- [x] Add a failing TUI test that a detail issue link creates a Links tab badge and renders a row with issue key, relationship, status, and summary.
- [x] Add a failing TUI test that `enter` opens a selected Jira issue link URL and `y` copies its issue key.
- [x] Extend `detailLink` conversion so Jira issue links render before detected description links.
- [x] Run focused TUI tests and `go test ./internal/tui -count=1`.

### Task 3: Docs And Verification

**Files:**
- Modify: `docs/backlog.md`
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

**Interfaces:**
- Consumes: completed parser and TUI behavior.
- Produces: docs showing the linked-issue Links slice is complete and the remaining focused-detail work is narrower.

- [x] Update backlog wording so the focused-detail item no longer asks for real linked issue data in Links.
- [x] Document Jira issue links in the Links workspace in project state and changelog.
- [x] Mark `tasks/todo.md` with verification evidence.
- [x] Run `go test ./internal/jira ./internal/tui -count=1`, `go test ./... -count=1`, `make check`, and `make install-user`.
