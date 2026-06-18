# Hierarchy Context Cleanup Design

## Scope

Continue the focused ticket detail workspace cleanup by making the Hierarchy section describe only
parent/child/subtask context. Jira issue links now live in the Links workspace, so Hierarchy should
not render the old linked-issues placeholder.

## Design

Remove the stale `Linked Issues` placeholder from the Hierarchy section. When no related hierarchy
rows are loaded, render a context-specific empty state:

- Root issue with no loaded children: no parent or child issues in the current view.
- Issue with a known parent but no loaded children: parent context is shown in Path and no child or
  subtask issues are loaded in the current view.

Keep grouped Children/Subtasks rows, existing selection behavior, and `enter` open behavior
unchanged.

## Non-Goals

Do not add a new Jira read path, change expanded-child loading, move linked issues back into
Hierarchy, or change Links behavior.

## Verification

Use TDD for the stale placeholder removal and empty-state copy. Then run focused Hierarchy tests,
`go test ./internal/tui -count=1`, `go test ./... -count=1`, `make check`, and
`make install-user`.
