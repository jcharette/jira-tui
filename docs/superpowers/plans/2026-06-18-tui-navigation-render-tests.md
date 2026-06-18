# TUI Navigation And Rendering Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the broad backlog item for TUI navigation and rendering tests with focused regression coverage for existing table and detail behavior.

**Architecture:** Keep coverage in the existing `internal/tui` package and workflow test files. Add issue-list tests to `issue_list_test.go`, detail tests to `detail_test.go`, and update docs/backlog once coverage lands.

**Tech Stack:** Go test, Bubble Tea key messages, existing `internal/tui` model test helpers.

## Global Constraints

- This is test hardening only; do not change user-visible behavior unless a test exposes a real bug.
- Keep Jira IO and worker behavior unchanged.
- Use same-package workflow files per `docs/package-boundary-audit.md`.
- Verify with `go test ./internal/tui -count=1`, `go test ./... -count=1`, `make check`, and `make install-user`.

---

### Task 1: Issue-List Navigation Rendering Tests

**Files:**
- Modify: `internal/tui/issue_list_test.go`

**Interfaces:**
- Consumes: `Model.Update`, `renderIssueList`, `browserLayout`, and existing issue-list model fields.
- Produces: focused tests that lock first/last navigation and viewport rendering behavior.

- [ ] Add a regression test for `g`/`G` table navigation moving to first/last visible rows and rendering the selected row.
- [ ] Run `go test ./internal/tui -run 'TestIssueList.*Navigation.*Render|TestIssueList.*FirstLast' -count=1`.

### Task 2: Detail Navigation Rendering Tests

**Files:**
- Modify: `internal/tui/detail_test.go`

**Interfaces:**
- Consumes: `Model.Update`, `render`, `focusedDetailSection`, and existing detail helpers.
- Produces: focused tests for exiting detail mode and preserving detail footer/render contracts.

- [ ] Add a regression test for `esc` returning from detail to table mode without changing selected issue.
- [ ] Add a regression test for detail section navigation rendering section-specific footer hints after moving between sections.
- [ ] Run `go test ./internal/tui -run 'TestDetail.*(Esc|Footer|Section)' -count=1`.

### Task 3: Docs And Verification

**Files:**
- Modify: `docs/backlog.md`
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

**Interfaces:**
- Consumes: completed focused tests.
- Produces: docs showing the broad coverage item is no longer an open backlog task.

- [ ] Update `docs/backlog.md` to replace the completed broad test item with a narrower ongoing coverage note.
- [ ] Add a changelog bullet for the test coverage.
- [ ] Add project-state coverage note.
- [ ] Mark `tasks/todo.md` checklist complete with verification evidence.
- [ ] Run focused tests, full tests, `make check`, and `make install-user`.
