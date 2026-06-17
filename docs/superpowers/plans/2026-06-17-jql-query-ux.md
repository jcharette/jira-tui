# JQL Query UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add direct and AI-assisted JQL query changes inside the TUI.

**Architecture:** Implement a table-level query modal in `internal/tui/query.go`, keep Jira reads on
the existing worker-backed search path, and route AI generation through the existing
provider-neutral `submitAIRequest` adapter. Query application changes `m.jql` and starts a normal
foreground search.

**Tech Stack:** Go, Bubble Tea v2, Bubbles textarea, existing worker pool, existing Claude/local AI
adapter, existing event stream.

## Global Constraints

- Do not run generated AI JQL without preview and explicit confirmation.
- Do not bypass the worker pool for Jira searches.
- Do not change saved view definitions or persistent config.
- Keep edits in same-package TUI files unless a cohesive new file is clearer.
- Use TDD for behavior changes.

---

### Task 1: Direct JQL Modal

**Files:**
- Create: `internal/tui/query.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/keymap.go`
- Test: `internal/tui/query_test.go`

**Deliverable:** `/` opens a query modal; direct JQL can be edited and confirmed with `ctrl+s` into
the existing foreground issue search path.

- [ ] Add failing tests for opening the modal, confirming direct JQL, and rejecting empty JQL.
- [ ] Implement query modal state, direct editor setup, render path, key handling, and apply helper.
- [ ] Add `/` table help binding.
- [ ] Run focused query tests.

### Task 2: AI JQL Generation

**Files:**
- Modify: `internal/events/stream.go`
- Modify: `internal/tui/query.go`
- Modify: `internal/tui/model.go`
- Test: `internal/tui/query_test.go`

**Deliverable:** AI mode submits a provider-neutral generate-JQL request and previews the parsed JQL
without running it.

- [ ] Add failing tests for AI request submission and preview-only result handling.
- [ ] Add `AIOperationGenerateJQL`.
- [ ] Implement AI prompt construction, submission, progress/result messages, result parsing, and
  stale-result guards.
- [ ] Run focused query tests.

### Task 3: AI Revision And Confirmation

**Files:**
- Modify: `internal/tui/query.go`
- Test: `internal/tui/query_test.go`

**Deliverable:** Users can revise the AI prompt, resubmit with current preview context, copy preview
to direct JQL with `enter`, or run preview with `ctrl+s`.

- [ ] Add failing tests for preview confirmation and revision prompt context.
- [ ] Implement preview confirmation, prompt revision, and preview-to-direct behavior.
- [ ] Run focused query tests.

### Task 4: Docs And Verification

**Files:**
- Modify: `docs/backlog.md`
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

**Deliverable:** User-facing docs reflect the shipped query workflow and completed task plan.

- [ ] Update backlog, project state, changelog, and task review notes.
- [ ] Run focused query/TUI tests, `go test ./... -count=1`, `make check`, and `make install-user`.
