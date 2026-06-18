# Keymap Audit Design

## Scope

Complete the `Now: Read And View` key binding audit by checking every active key context for stale,
hidden, or misleading commands. Keep conventional navigation aliases such as `j/k`, arrows, page
keys, and home/end. Remove shortcut paths that duplicate a workflow without being advertised or that
make one key mean an unrelated fallback action.

## Findings

The keymap is already centralized in `internal/tui/keymap.go`, and most contexts are clean. Two
detail-mode paths need cleanup:

- `b` still opens the selected issue in the browser, but it is no longer advertised after detail
  open moved to the visible `o` binding.
- `a` is advertised as contextual AI, but when AI is unavailable it starts the comment composer.
  Comments already have their own section/focused action path, so `a` should not silently mean
  comment.

## Approach

Add regression tests first for the desired behavior and a metadata-level test that prevents
duplicate keys inside each active context except for documented navigation/editing aliases. Then
remove the stale detail handlers and update docs/backlog/changelog with the audit result.

## Non-Goals

Do not redesign the whole key system, add a command palette, or remove conventional navigation
aliases. Do not change Jira reads, worker flows, or modal behavior unrelated to the stale detail
shortcuts.

## Verification

Run focused keymap/detail tests, then `go test ./internal/tui -count=1`, `go test ./... -count=1`,
`make check`, and `make install-user`.
