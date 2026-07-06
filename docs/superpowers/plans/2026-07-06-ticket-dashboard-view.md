# Ticket Dashboard View Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the existing Developer Workbench detail section feel like a ticket dashboard that quickly answers what the ticket is, what changed, and what to do next.

**Architecture:** Reuse the existing detail section renderer, Workbench section, comments, worklog, hierarchy, links, Claude state, and Ticket Actions. Do not add a new dashboard mode, persistence layer, or Jira reads in this slice.

**Tech Stack:** Go, Bubble Tea model rendering, Lip Gloss styling, existing golden UX snapshots.

## Global Constraints

- Keep Jira/Git/Claude write gates unchanged.
- Reuse existing detail/workbench helpers before adding new abstractions.
- Keep the diff narrow and cover visible UX with focused tests and snapshots.

---

### Task 1: Reframe Developer Workbench As Ticket Dashboard

**Files:**
- Modify: `internal/tui/detail.go`
- Modify: `internal/tui/detail_test.go`
- Modify: `internal/tui/testdata/ux_ticket_detail.golden`
- Modify: `internal/tui/testdata/ux_claude_section.golden`
- Modify: `internal/tui/testdata/ux_worklog_dialog.golden`
- Modify: `docs/keyboard.md`
- Modify: `docs/workflows.md`
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

**Interfaces:**
- Consumes: `renderDeveloperWorkbenchSection(ctx detailRenderContext, width int) string`
- Produces: a dashboard-style Workbench render using existing comments, worklog, hierarchy, links, and Claude state helpers.

- [x] **Step 1: Write the failing test**

Update `TestDeveloperWorkbenchSectionShowsDeveloperLoopActions` to expect:
- `Ticket Dashboard`
- `Issue ABC-1`
- `Owner Jon`
- `Recent Rae: Please make write gates obvious.`
- `NEXT ACTION`
- existing action names: `Start Work`, `Claude Plan`, `Quality Review`, `Draft Comment`, `Add Comment`, `Log Work`

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui -run TestDeveloperWorkbenchSectionShowsDeveloperLoopActions -count=1`

Expected: FAIL because the Workbench still renders as `Developer Workbench` with the old `SURFACE/STATE/NEXT` table.

- [x] **Step 3: Write minimal implementation**

Change `renderDeveloperWorkbenchSection` so it renders:
- header `Ticket Dashboard`
- one identity line for issue key, owner, priority, status
- one recent-comment line when loaded
- a smaller table headed `NEXT ACTION`, `STATE`, `WHY`
- the same existing rows/actions, preserving write gates

- [x] **Step 4: Run focused tests and update snapshots**

Run:

```bash
go test ./internal/tui -run TestDeveloperWorkbenchSectionShowsDeveloperLoopActions -count=1
UPDATE_GOLDEN=1 go test ./internal/tui -run TestUXSnapshots -count=1
go test ./internal/tui -run 'TestDeveloperWorkbenchSectionShowsDeveloperLoopActions|TestUXSnapshots' -count=1
```

- [x] **Step 5: Update docs and task notes**

Document the Workbench as a ticket dashboard in keyboard/workflow/project-state/changelog/task notes.

- [x] **Step 6: Verify and commit**

Run:

```bash
go test ./... -count=1
make docs-check
make check
make install-user
```

Commit:

```bash
git add internal/tui/detail.go internal/tui/detail_test.go internal/tui/testdata docs tasks/todo.md
git commit -m "feat: make workbench a ticket dashboard"
```
