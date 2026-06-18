# Navigation Related Children Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make automatic child issue loading view-scoped, with the default Epics view opting in.

**Architecture:** `config.IssueView` owns whether a saved view wants child issues. The TUI passes the
active view's flag to `worker.SearchIssuesRequest`; the worker gates only the automatic child lookup
on that flag while preserving missing-parent normalization, known subtasks, and explicit expansion.

**Tech Stack:** Go, Bubble Tea, existing worker pool, existing TOML config loader.

## Global Constraints

- Keep Jira I/O worker-backed.
- Do not change saved-view JQL when loading related children.
- Keep explicit `x`/`X` expansion available in every view.
- Add regression tests before production code.

---

### Task 1: Config View Flag

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces: `config.IssueView.IncludeChildren bool`
- Produces TOML field: `[[views.saved]].include_children`

- [ ] Add failing tests that default Epics has `IncludeChildren == true` and config load/save round-trips the field.
- [ ] Run `go test ./internal/config -run 'Test(DefaultViewsIncludeEpicFocusedView|LoadReadsConfigFile|SaveWritesConfigFileWithPrivatePermissions)' -count=1` and verify failure.
- [ ] Add `IncludeChildren bool` to `IssueView` and `viewConfig`.
- [ ] Map the field through `DefaultViews`, `viewConfigs`, and `issueViews`.
- [ ] Re-run the focused config tests and verify pass.

### Task 2: Worker Search Gate

**Files:**
- Modify: `internal/worker/pool.go`
- Test: `internal/worker/pool_test.go`

**Interfaces:**
- Consumes: `SearchIssuesRequest.IncludeChildren bool`
- Produces: automatic child lookup only when `IncludeChildren` is true

- [ ] Add failing tests that ordinary search does not run the `parent in (...)` lookup and opt-in search does.
- [ ] Run `go test ./internal/worker -run 'TestPoolSearchIssues' -count=1` and verify failure.
- [ ] Add `IncludeChildren bool` to `SearchIssuesRequest`.
- [ ] Gate `p.withChildIssues` behind `request.SearchIssues.IncludeChildren`.
- [ ] Re-run focused worker tests and verify pass.

### Task 3: TUI Active View Plumbing

**Files:**
- Modify: `internal/tui/commands.go`
- Modify: `internal/tui/navigation.go`
- Test: `internal/tui/issue_list_test.go`

**Interfaces:**
- Consumes: `Model.activeViewIncludeChildren() bool`
- Produces: `worker.SearchIssuesRequest.IncludeChildren`

- [ ] Add failing tests that submit search uses false for ordinary views and true for an active Epics-style view.
- [ ] Run `go test ./internal/tui -run 'TestSubmitIssueSearch' -count=1` and verify failure.
- [ ] Add `activeViewIncludeChildren`.
- [ ] Pass the flag in `submitIssueSearch`.
- [ ] Re-run focused TUI tests and verify pass.

### Task 4: Docs And Final Verification

**Files:**
- Modify: `docs/backlog.md`
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

- [ ] Move the completed Navigation child-loading item out of the active backlog.
- [ ] Record the behavior in project state and changelog.
- [ ] Run focused config, worker, and TUI tests.
- [ ] Run `go test ./... -count=1`.
- [ ] Run `make check`.
- [ ] Run `make install-user`.
