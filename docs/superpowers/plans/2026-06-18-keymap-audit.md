# Keymap Audit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the active-context key binding audit by removing stale detail shortcuts and documenting the verified keymap rules.

**Architecture:** Keep key metadata in `internal/tui/keymap.go` and key handling in `internal/tui/model.go`. Add focused regression tests in existing TUI test files and document the audit in project docs.

**Tech Stack:** Go, Bubble Tea key messages, existing `internal/tui` model tests.

## Global Constraints

- Keep conventional navigation aliases: `j/k`, arrows, page keys, home/end, and text-editing aliases.
- Keep one clear semantic path per workflow: `tab` moves focus, `enter` acts on focus, single-letter keys are distinct accelerators.
- Do not add new packages or worker/Jira behavior.
- Verify with focused tests, `go test ./internal/tui -count=1`, `go test ./... -count=1`, `make check`, and `make install-user`.

---

### Task 1: Failing Keymap Regression Tests

**Files:**
- Modify: `internal/tui/detail_test.go`
- Modify: `internal/tui/issue_list_test.go`

**Interfaces:**
- Consumes: `Model.Update`, `keyBindings`, `activeKeyContext`, and existing test helpers.
- Produces: tests proving hidden detail `b` and AI fallback `a` are gone, plus a context key uniqueness invariant.

- [x] Add a detail test that pressing `b` no longer opens the selected issue URL.
- [x] Add a detail test that pressing `a` with AI unavailable does not start comment composition.
- [x] Add a keymap metadata test that fails on duplicate non-navigation keys inside each context.
- [x] Run focused tests and confirm they fail before implementation.

### Task 2: Remove Stale Detail Handlers

**Files:**
- Modify: `internal/tui/model.go`

**Interfaces:**
- Consumes: failing tests from Task 1.
- Produces: detail key behavior that matches `internal/tui/keymap.go`.

- [x] Remove the `b` detail-mode issue-open handler.
- [x] Remove the `a` no-AI comment-composer fallback; leave AI section jump and inline description AI behavior intact.
- [x] Run focused tests and `go test ./internal/tui -count=1`.

### Task 3: Audit Docs And Verification

**Files:**
- Modify: `docs/backlog.md`
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

**Interfaces:**
- Consumes: completed keymap cleanup.
- Produces: docs showing the key binding audit item is complete.

- [x] Replace the broad key binding audit backlog item with an ongoing rule for new contexts.
- [x] Document the audit result and stale shortcut removal in project state/changelog.
- [x] Mark `tasks/todo.md` complete with verification evidence.
- [x] Run the full verification loop.
