# Developer Workbench UX Design

Date: 2026-07-06

## Goal

Improve `jira-tui` so the primary experience feels like a developer workbench for Jira tickets:
pick a ticket, understand it, start work, update Jira, and finish the pull request without losing
context.

Use JiraTUI as a UX benchmark for discoverability and cockpit-style organization, but do not clone
its product. Our niche remains Jira plus Git, GitHub, Claude, diagnostics, and safe write previews.

## Non-Goals

- Rebuild the TUI framework or layout engine.
- Copy JiraTUI screens, code, styling, or interaction model.
- Add broad Jira administration features.
- Add provider-neutral AI execution in this pass.
- Replace existing worker, cache, Jira metadata, or confirmation paths.

## Pass 1: Developer Flow Polish

Make the daily developer loop obvious from ticket detail and wide layouts.

The first implementation pass should expose a clear "Developer Workbench" action path around the
selected ticket:

- Start Work
- Claude Plan or Quality Review when enabled
- Draft Comment when enabled
- Add Comment
- Log Work
- Open Jira
- Copy Key and Copy URL
- Finish/PR workflow entry if the current Git branch or repo can be detected

This should reuse the existing Ticket Actions, detail sections, Claude actions, worklog, comments,
and git workflow code. The change is mostly presentation and action ordering: the actions users need
for the developer loop should be visible and grouped before lower-frequency Jira metadata edits.

Success criteria:

- A user can open a ticket detail view and immediately see the next developer-oriented action.
- Existing write gates remain unchanged.
- Existing keyboard shortcuts keep working.
- UX snapshots cover the new or changed workbench state.

## Pass 2: Jira Basics Parity

Backfill Jira-first affordances where they support the developer loop and reduce friction compared
with JiraTUI.

Priority order:

1. Search and filtering clarity: make saved views, direct JQL, recent queries, and local filtering
   easier to distinguish in the UI.
2. Related work clarity: make hierarchy, issue links, detected links, and subtasks easier to scan and
   act on from detail.
3. Comment flow clarity: make add/edit/refine/review/post states visually consistent and easy to
   recover from.
4. Optional later item: full-text search if it can be added as a small JQL template surface rather
   than a new search subsystem.

Success criteria:

- The app remains fast for assigned-work scanning.
- Jira basics improve without burying git/Claude actions.
- New Jira writes continue to use metadata-backed existing paths.

## Pass 3: Visual And Chrome Cleanup

Make the app feel more coherent without changing core behavior.

Focus areas:

- Shared header/footer grammar across issue list, detail, query, comments, Claude, diagnostics, and
  config.
- Help that explains the current screen and the next likely action, not every possible command.
- Status indicators for Jira refresh, Claude activity, background workers, stale cache, and errors.
- Stable snapshot coverage for representative states.

Success criteria:

- Help/footer text matches available actions.
- Background activity is visible without becoming noisy.
- UX snapshots catch drift in the main daily-flow screens.

## Implementation Shape

Use three small implementation plans, one per pass. Each plan should land in narrow commits with
focused tests and snapshots.

Recommended first slice:

1. Add a `Developer Workbench` section or grouped action surface to ticket detail using existing
   actions.
2. Reorder Ticket Actions so developer-loop actions come first.
3. Update footer/help/docs for the new grouping.
4. Add or update UX snapshots for the workbench/detail state.

## Verification

Each slice should run:

- Focused package tests for changed TUI behavior.
- `go test ./... -count=1`
- `make docs-check` when docs change.
- `make check`
- `make install-user` before calling the slice complete.

## Decisions

- Start with a ticket-detail section or grouped action surface. Add a wide-layout panel only if the
  detail version feels cramped after implementation.
- Do not add a direct TUI `jira finish` write path in Pass 1. If repo/branch detection is available,
  show a read-only Finish/PR prompt that points to the existing confirmed CLI workflow.
- Defer full-text search until after search/filtering clarity improves. If added later, implement it
  as a small JQL template surface rather than a new search subsystem.
