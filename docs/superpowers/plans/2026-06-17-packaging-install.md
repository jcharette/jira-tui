# Packaging Install Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the install story clear by supporting a versioned `go install` path that installs a `jira` binary and documenting release binary, source, and future Homebrew options.

**Architecture:** Move the existing Cobra command wiring from `cmd/jira-tui` into a shared internal package, then keep both command packages as tiny wrappers. `cmd/jira-tui` preserves existing build targets, while `cmd/jira` exists so `go install github.com/jcharette/jira-tui/cmd/jira@<version>` produces a binary named `jira`. Update the Go module path from the stale non-resolving `github.com/jon/jira-tui` path to the actual remote path `github.com/jcharette/jira-tui`.

**Tech Stack:** Go, Cobra, Bubble Tea, existing Makefile release/docs workflow.

## Global Constraints

- Keep existing `make build`, `make build-local`, `make install-user`, and `make run` behavior.
- Do not add Homebrew automation in this slice; track it as a follow-up.
- Document only install paths that work after a tagged release containing `cmd/jira`.
- Use `github.com/jcharette/jira-tui` as the module and import path because it resolves to the actual GitHub repository.
- Use TDD for the new command package and refactor tests that already cover CLI-adjacent helpers.

---

### Task 1: Shared CLI Package And Go Install Entrypoint

**Files:**
- Modify: `go.mod`
- Modify: every internal import path under `cmd`, `internal`, and tests
- Create: `internal/app/app.go`
- Create: `internal/app/app_test.go`
- Create: `cmd/jira/main.go`
- Modify: `cmd/jira-tui/main.go`
- Delete: `cmd/jira-tui/main_test.go`

**Interfaces:**
- Produces: `app.Execute() error`
- Produces: `app.NewRootCommand() *cobra.Command`

- [ ] Write a failing package check for `cmd/jira`.
- [ ] Verify the old module path fails to resolve and the actual remote module path resolves.
- [ ] Rewrite the module path and internal imports to `github.com/jcharette/jira-tui`.
- [ ] Move existing command wiring to `internal/app`.
- [ ] Keep `cmd/jira-tui/main.go` as a thin wrapper around `app.Execute`.
- [ ] Add `cmd/jira/main.go` as the same thin wrapper.
- [ ] Move the saved-view writer test to `internal/app`.
- [ ] Run `go test ./cmd/jira ./cmd/jira-tui ./internal/app -count=1`.

### Task 2: Install Docs And Backlog Cleanup

**Files:**
- Modify: `README.md`
- Modify: `docs/project-state.md`
- Modify: `docs/backlog.md`
- Modify: `docs/roadmap.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

**Interfaces:**
- Consumes: `cmd/jira` package path from Task 1.
- Produces: documented install commands for release binaries, `go install`, and source installs.

- [ ] Add release binary and `go install` instructions to the README.
- [ ] Note the active install paths in project state.
- [ ] Move the broad packaging backlog item to a Homebrew-only follow-up.
- [ ] Add an Unreleased changelog bullet.
- [ ] Mark the task checklist complete with verification notes after tests pass.

### Task 3: Verification, Merge, Push, Release

**Files:**
- Modify only release metadata if a patch release is cut after merge.

**Interfaces:**
- Consumes: merged implementation and docs.
- Produces: pushed `main` and a tagged release where documented versioned installs work.

- [ ] Run `go build -o /tmp/jira-go-install-check ./cmd/jira`.
- [ ] Run `go list -m -versions github.com/jcharette/jira-tui`.
- [ ] Run `go test ./... -count=1`.
- [ ] Run `make check`.
- [ ] Run `make install-user`.
- [ ] Commit the feature branch.
- [ ] Merge to `main`, re-run `go test ./... -count=1`, and push.
- [ ] Cut a patch release so `go install github.com/jcharette/jira-tui/cmd/jira@<version>` resolves to a tag containing `cmd/jira`.
