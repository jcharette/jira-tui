# Actions Field Edits Design

## Scope

Continue focused ticket detail workspace work by replacing the disabled generic `Edit Fields`
Actions row with concrete metadata-backed field edit actions that already exist in ticket detail.

## Design

The Actions section should expose `Edit Summary` and `Change Priority` as enabled rows. Selecting
either row and pressing `enter` should route to the same metadata-backed workflows as the direct
Summary/Priority focus targets and `s`/`p` accelerators:

- `Edit Summary` calls `startSummaryEditor`.
- `Change Priority` calls `startPriorityEditor`.

The existing disabled `Create Subtask` row remains disabled because it still needs create metadata
and a dedicated workflow.

## Non-Goals

Do not add new Jira edit APIs, change Summary/Priority editor behavior, add a command palette, or
implement Subtask creation. Do not alter the existing shortcuts or focused-field `enter` behavior.

## Verification

Use TDD around the Actions menu. Then run focused Actions tests, `go test ./internal/tui -count=1`,
`go test ./... -count=1`, `make check`, and `make install-user`.
