# Task Plan

## Hierarchy Detail Polish

- [x] Explore project context and backlog.
- [x] Confirm hierarchy detail design direction.
- [x] Write implementation plan after design approval.
- [x] Add failing TUI tests for grouped hierarchy rendering and selection.
- [x] Implement grouped hierarchy rows in `internal/tui/model.go`.
- [x] Keep hierarchy activation/back-stack behavior working across grouped rows.
- [x] Run focused TUI tests.
- [x] Update docs and changelog for the hierarchy tab behavior.
- [x] Run `make check`.

## Review

- Implemented grouped Hierarchy tab rendering for Path, Children, Subtasks, and a linked-issues placeholder.
- Added focused TUI coverage for grouped hierarchy rendering and opening a selected grouped subtask.
- Verified with `go test ./internal/tui -count=1`, `make check`, and `make install-user`.

## Implementation Plan

### Scope

Improve the focused ticket detail Hierarchy tab so it renders a grouped tree workspace:

- `Path` section for known parent context.
- `Children` section for visible non-subtask children.
- `Subtasks` section for visible subtasks.
- `Linked Issues` placeholder section only, because linked issue data is not currently exposed through the Jira client.

No new Jira API calls are part of this slice. Existing table expansion through `x` and `X` remains table-mode behavior.

### Files

- Modify `internal/tui/model.go`
  - Add a small internal hierarchy row model.
  - Split visible related issues into children and subtasks.
  - Render grouped sections with the existing section header, empty-state, and detail table helpers.
  - Map `selectedHierarchy` to selectable issue rows across groups.
- Modify `internal/tui/model_test.go`
  - Add RED tests for grouped Path/Children/Subtasks rendering.
  - Add RED tests that `enter` opens the selected grouped subtask/child and preserves the detail back stack.
- Modify docs after code is green:
  - `docs/project-state.md`
  - `docs/backlog.md`
  - `docs/releases/CHANGELOG.md`

### TDD Steps

- [ ] Write `TestHierarchySectionRendersGroupedTree`.
  - Build a detail-mode model with selected parent `ABC-1`, one child story, and one subtask.
  - Assert the rendered hierarchy contains `Path`, `Children 1`, `Subtasks 1`, the child key, and the subtask key.
  - Assert it does not use the old flat empty-state wording when grouped rows exist.
- [ ] Run `go test ./internal/tui -run TestHierarchySectionRendersGroupedTree -count=1`.
  - Expected: FAIL because the current renderer has no grouped section labels/counts.
- [ ] Implement the smallest grouped renderer in `internal/tui/model.go`.
  - Add `hierarchyRow` and grouping helpers near the hierarchy renderer.
  - Render parent path first, then child and subtask tables.
  - Keep selectable rows flat internally so existing `selectedHierarchy` behavior stays simple.
- [ ] Run `go test ./internal/tui -run TestHierarchySectionRendersGroupedTree -count=1`.
  - Expected: PASS.
- [ ] Write `TestHierarchyEnterOpensSelectedGroupedSubtask`.
  - Focus Hierarchy, select the subtask row after the child row, press `enter`.
  - Assert selected issue changes to the subtask and previous selection is pushed to the back stack.
- [ ] Run `go test ./internal/tui -run TestHierarchyEnterOpensSelectedGroupedSubtask -count=1`.
  - Expected: PASS if row mapping was implemented cleanly; otherwise fix mapping only.
- [ ] Run `go test ./internal/tui -count=1`.
- [ ] Update docs and changelog.
- [ ] Run `make check`.
