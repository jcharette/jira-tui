# ADF Renderer Library Spike Design

## Context

The current ticket detail backlog prioritizes Jira ADF rendering quality before new workflow
features. `internal/adf.Render` already gives the rest of the app one stable API for turning
go-atlassian `CommentNodeScheme` values into terminal-friendly text.

Replacing the renderer now could be worthwhile if a maintained library can handle more of Jira's
real ADF shape than our custom traversal. The risk is wasting effort by extending custom rendering
before proving whether that library path exists.

## Goal

Make the next ADF work library-evaluation-safe: capture realistic renderer expectations first,
evaluate whether a maintained Go dependency can satisfy the ADF conversion problem, and only then
choose between extending the current renderer or wrapping a library.

## Scope

This spike is renderer-only. It does not change ticket detail panel layout, scrolling, colors,
or width fitting in `internal/tui`.

In scope:

- Realistic fixture/golden tests for Jira descriptions and comments with tables, nested lists,
  links, mentions, inline code, code blocks, blockquotes, panels, statuses, and cards.
- A short dependency evaluation for ADF conversion libraries and terminal Markdown renderers.
- A written decision in the task review explaining whether to keep the custom renderer, wrap a
  library, or do a second spike.
- Minimal production changes only after failing tests and the dependency decision.

Out of scope:

- ANSI/styled output from `internal/adf.Render`.
- TUI panel layout changes.
- Jira API changes or live Jira fixture capture.
- A broad Markdown rendering replacement unless the spike proves ADF-to-Markdown conversion is
  already reliable enough.

## Evaluation Model

There are two separate library questions:

1. ADF conversion: can a maintained Go package consume Jira/Atlassian ADF and preserve Jira-specific
   nodes such as mentions, statuses, panels, tables, inline cards, and marks?
2. Terminal Markdown rendering: if we can produce Markdown, can a terminal renderer improve wrapping
   and presentation without making tests or TUI layout brittle?

The first question decides whether replacement is viable. A Markdown renderer alone is not enough
because Jira stores comments and textarea fields as ADF, not Markdown.

## Candidate Direction

Keep `internal/adf.Render(node) string` as the public boundary during the spike. If a library is
chosen, hide it behind this boundary so callers do not care whether output comes from custom
traversal, ADF-to-Markdown conversion, or a terminal Markdown renderer.

The default expected outcome is likely one of:

- Continue custom renderer: if no maintained ADF conversion package covers the Jira nodes we need.
- Hybrid renderer: custom ADF-to-Markdown/plain-text conversion plus an optional Markdown renderer
  later for TUI-specific width handling.
- Library wrapper: only if a maintained dependency handles the fixture corpus clearly better than
  the current renderer with acceptable dependency and output stability.

## Acceptance Criteria

- The fixture tests define expected output for the backlog-required ADF shapes before production
  rendering changes are made.
- The dependency review records at least one terminal Markdown renderer candidate and whether any
  credible Go ADF conversion candidate exists.
- If production code changes, each change starts with a failing test and keeps `adf.Render` callers
  unchanged.
- The review section in `tasks/todo.md` states the decision and why future TUI layout work is or is
  not needed.

## Verification

- Run focused ADF tests with `go test ./internal/adf -count=1`.
- If production code changes, run `go test ./... -count=1`.
- If user-visible rendering changes, update `docs/releases/CHANGELOG.md`.
- If backlog scope changes, update `docs/backlog.md`.
