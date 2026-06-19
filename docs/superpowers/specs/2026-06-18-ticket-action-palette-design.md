# Ticket Action Palette Design

## Goal

Make ticket detail actions discoverable and filterable without adding more single-key bindings or
requiring users to tab into the Actions section.

## UX

- Press `.` from focused ticket detail to open `Ticket Actions`.
- The palette is a bounded dialog with a `Filter` input and table-like rows:
  `Action`, `Type`, and `Detail`.
- Typing filters rows by action id, label, type, or detail. `j`/`k` and arrow keys move the
  selection, `enter` runs the selected action, and `esc` closes the palette.
- Existing shortcuts and the inline Actions section remain available for compatibility.

## Architecture

- Add palette state to `Model` and keep the behavior in `internal/tui/action_palette.go`.
- Reuse `detailActions()` as the source of truth so the palette and inline Actions section cannot
  drift.
- Map filtered rows back to their original `detailAction` index, then call
  `runSelectedDetailAction()` to preserve existing worker-backed workflows.

## Non-Goals

- No new Jira reads or queries.
- No generic edit-all-fields flow.
- No removal of existing detail shortcuts.

## Verification

- Focused TUI tests cover opening, filtering, rendering, and dispatching an action through the
  existing workflow.
- Broader verification uses `go test ./... -count=1`, `make check`, and `make install-user`.
