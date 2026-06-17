# Saved Query Promotion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let users save a recent direct or AI-generated JQL query as a durable named saved view.

**Architecture:** Add a config helper for appending saved views, inject a saved-view writer into the TUI model, and extend the query modal's `Recent` mode with a compact naming prompt. The writer is wired in `main` and persists through the existing `config.Save` path.

**Tech Stack:** Go, Bubble Tea v2, Bubbles textinput, existing config TOML persistence.

## Global Constraints

- Do not run Jira or change the active query when saving a recent query as a named view.
- Preserve existing direct JQL, AI JQL, and recent-query run behavior.
- Persist durable named views through the existing config file, not the SQLite query history cache.
- Keep implementation inside existing config/TUI package boundaries.
- Use TDD: add focused failing tests before production code.

---

### Task 1: Config Saved View Helper

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Interfaces:**
- Produces: `func AddSavedView(cfg Config, view IssueView) (Config, error)`

- [x] Add failing tests for appending a saved view, trimming name/JQL, preserving `ActiveView`, rejecting blank names/JQL, and rejecting duplicate names case-insensitively.
- [x] Run `go test ./internal/config -run 'TestAddSavedView' -count=1` and verify it fails with undefined `AddSavedView`.
- [x] Implement `AddSavedView` in `internal/config/config.go`.
- [x] Run `go test ./internal/config -run 'TestAddSavedView' -count=1` and verify it passes.

### Task 2: TUI Recent Save Prompt

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/query.go`
- Modify: `internal/tui/keymap.go`
- Modify: `internal/tui/query_test.go`

**Interfaces:**
- Consumes: `config.AddSavedView`
- Produces: `type SavedViewWriter func(config.IssueView) error`
- Produces: `func WithSavedViewWriter(writer SavedViewWriter) Option`

- [x] Add failing TUI tests for opening the prompt with `s`, saving with `ctrl+s`, rejecting duplicates, and not submitting Jira search work.
- [x] Run focused TUI tests and verify they fail for missing save prompt/writer behavior.
- [x] Add model fields for save prompt state and a textinput editor.
- [x] Add `WithSavedViewWriter`.
- [x] Add Recent-mode `s` handling, prompt rendering, validation, and successful in-memory append.
- [x] Update query modal footer/help copy.
- [x] Run focused TUI tests and verify they pass.

### Task 3: Main Wiring

**Files:**
- Modify: `cmd/jira-tui/main.go`
- Test: existing config/TUI tests plus full package build

**Interfaces:**
- Consumes: `jiratui.WithSavedViewWriter`
- Consumes: `config.AddSavedView`

- [x] Resolve the config path in `runApp`.
- [x] Pass `WithSavedViewWriter` that appends the view to captured config and calls `config.Save`.
- [x] Run `go test ./cmd/jira-tui ./internal/config ./internal/tui -count=1`.

### Task 4: Docs And Verification

**Files:**
- Modify: `docs/backlog.md`
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `docs/roadmap.md`
- Modify: `tasks/todo.md`

- [x] Update task plan, project state, changelog, backlog, and roadmap.
- [x] Run focused tests.
- [x] Run `go test ./... -count=1`.
- [x] Run `make check`.
- [x] Run `make install-user`.
- [x] Commit, fast-forward merge to `main`, and push.
