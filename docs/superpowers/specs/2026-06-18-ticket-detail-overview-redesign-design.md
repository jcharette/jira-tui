# Ticket Detail Overview Redesign Design

## Goal

Redesign focused ticket detail so it opens as an inspectable ticket record instead of a tabbed
document viewer. The selected direction is Proposal B: `Overview + Control Strip`.

## UX

- Opening a ticket detail lands on `Overview`, not `Description`.
- Top-level navigation contains content destinations only:
  - `Overview`
  - `Comments`
  - `Hierarchy`
  - `Links` when links exist
  - `Claude` when AI is enabled
- `Description` is no longer a primary tab. It appears as a preview inside `Overview`, with the
  existing full description reader reachable from `Overview` or a later focused detail affordance.
- `Status` is no longer a primary tab. It becomes a focusable field control in the detail control
  strip, alongside `Priority` and `Assignee`.
- The control strip sits under the title and summary:
  - `Status <value> v`
  - `Priority <value> v`
  - `Assignee <value> v`
  - optional compact metadata such as updated/reporter when space allows
- Pressing `enter` on `Status` opens the existing transition picker flow.
- Pressing `enter` on `Priority` and `Assignee` keeps using the existing metadata-backed pickers.
- The footer reflects the focused control or content area instead of listing every possible command.
- Single-key accelerators remain available for power users, but focus plus `enter` is the primary
  visible interaction model.

## Overview Content

The first screen should answer what the ticket is, what happened recently, and what can be done next:

- A compact comments/activity block:
  - shows latest loaded comment summary when comments are loaded,
  - shows `No comments yet.` when there are no comments,
  - shows loading/error state using the existing detail status block grammar.
- A short description preview, not the full long document.
- A compact hierarchy summary:
  - root/parent context,
  - loaded child count when present,
  - clear empty state when no hierarchy rows are loaded.
- A small action hint that points to the Ticket Actions palette (`.`) instead of rendering an
  always-visible Actions tab.

## Architecture

- Keep the redesign inside the existing `internal/tui` detail rendering model.
- Add an `overview` detail section and make it the first section target.
- Remove `description`, `actions`, and `status` from the visible section list for the first
  implementation slice.
- Keep existing full-description rendering helpers intact so description reading can be restored or
  reintroduced intentionally without losing behavior.
- Promote `status` from section action to field target in `detailTargets()`.
- Reuse the existing `startStatusTransitionPicker()` flow for status field activation.
- Reuse existing summary, priority, assignee, labels, components, transition, comment, hierarchy,
  and link state. This is a layout and interaction redesign, not a Jira API redesign.

## Non-Goals

- No Jira query changes.
- No new Jira writes.
- No generic editable-field framework.
- No removal of existing keyboard accelerators in this slice.
- No final redesign of the issue table view; this spec covers focused ticket detail only.

## Verification

- TUI tests cover new default focus, visible section order, absence of `Status` as a tab, and status
  activation through the focused field control.
- TUI rendering tests cover the compact control strip and Overview content.
- Existing tests for summary, priority, assignee, comments, hierarchy, links, actions palette, and
  transitions continue to pass.
- Full verification uses `go test ./... -count=1`, `make check`, `make install-user`, and
  `git diff --check`.
