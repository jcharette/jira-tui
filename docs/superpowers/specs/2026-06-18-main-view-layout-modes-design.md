# Main View Layout Modes Design

## Goal

Make the starting page support multiple useful ways to inspect the same loaded tickets without
changing Jira queries. The selected direction is to separate ticket scope from visual layout.

## Core Model

- Saved views and filters answer: `which tickets do I care about?`
- Layout modes answer: `how do I want to inspect those tickets?`
- Layout modes are local presentation only. Switching layout modes must not run Jira reads, mutate
  saved views, or change the active JQL.
- The existing loaded issue list remains the source of truth for all layouts.

## Layout Modes

### Table

- A polished version of the current issue table.
- Best for dense scanning and keyboard-first navigation.
- Adds stronger control affordances:
  - `View: <name> v`
  - `Filter: All/Active`
  - `Layout: Table v`
  - `Sort: <mode> v`
- Keeps the selected-row drawer for extra context when useful.

### Workbench

- A split workspace for wider terminals.
- Left side remains the issue table.
- Right side is selected-ticket context only; it must not repeat data already visible in the selected
  table row.
- Useful side context:
  - latest loaded comment or comment-loading state,
  - parent/child summary,
  - detail/comment cache freshness,
  - quick action hints,
  - description preview only if detail is already cached.
- Responsive behavior:
  - wide terminal: table plus context panel,
  - normal terminal: table plus selected-row drawer,
  - narrow terminal: table only.

### Lanes

- Groups the current loaded view by status or workflow state.
- Best for triage after ticket scope has already been narrowed by saved view/filter.
- Examples:
  - `Mine + Lanes`: one engineer's work by status,
  - `Active + Lanes`: workable tickets by status,
  - `Sprint + Lanes`: team sprint triage,
  - `Epics + Lanes`: project/release view.
- Lanes show compact counts and can be collapsed later:
  - `To Do 4`
  - `In Progress 2`
  - `Blocked 1`
- Lanes are presentation only; they do not imply a global dashboard and do not fetch unrelated
  tickets.

## UX

- Add a local layout switch, likely `L`, cycling `Table -> Workbench -> Lanes`.
- Add a visible layout chip in the starting-page control strip.
- Keep `enter` as open selected ticket.
- Keep `/` for query/filter, `v` for saved view workflows, `f` for local All/Active filter.
- Keep secondary operations discoverable in `?` help rather than overloading the footer.
- Prefer compact chips and colored status/priority badges over prose.

## Non-Goals

- No Jira query changes.
- No saved-view schema changes in the first slice.
- No persistence of preferred layout in the first slice.
- No custom column configuration.
- No extra Jira reads for Workbench preview data; use only loaded/cached data.

## Implementation Direction

- First polish the table/control strip so every mode has a better shared top surface.
- Then add local layout mode state and footer/help entry.
- Implement Workbench as a responsive split only when there is enough width.
- Implement Lanes as a renderer over the same visible issue indexes, preserving selection and status
  filtering.
- Persist layout preference later only if the local mode switch proves useful.

## Verification

- TUI tests cover layout cycling without changing `jql`, active saved view, loaded issues, or cache
  state.
- Rendering tests cover the control strip, Workbench responsive fallback, and status-lane grouping.
- Navigation tests cover stable selection and opening selected tickets in every layout mode.
- Full verification uses `go test ./... -count=1`, `make check`, `make install-user`, and
  `git diff --check`.
