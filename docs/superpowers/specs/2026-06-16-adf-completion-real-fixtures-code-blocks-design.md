# ADF Completion With Real Fixtures And Compact Code Blocks

## Context

The ADF renderer now wraps a maintained ADF-to-Markdown converter for text-oriented Jira content
while preserving the existing custom table path behind `internal/adf.Render`. Existing fixtures cover
synthetic and Jira-shaped examples, but the remaining risk is real Jira payload drift: actual
descriptions and comments can combine nodes in ways synthetic fixtures miss.

The ticket detail TUI also renders fenced code blocks with literal ASCII borders. That makes code
recognizable, but it spends extra vertical space and creates visual noise in dense terminal views.
The renderer should keep returning plain text/Markdown-like content; the TUI should own the visual
presentation.

## Goals

- Build a repeatable dev-only workflow for adding sanitized real Jira ADF fixtures.
- Add offline golden tests for real/sanitized Jira description and comment payloads.
- Fix renderer gaps only when the real fixtures expose them.
- Replace ASCII code-block borders in rich description rendering with compact foreground/background
  styling.
- Make the hand-rolled TUI component audit the next major initiative after this ADF completion
  slice.

## Non-Goals

- Do not make normal tests call Jira, require credentials, or depend on mutable issue content.
- Do not change `internal/adf.Render` callers or return styled/ANSI output from `internal/adf`.
- Do not replace table rendering unless a real fixture proves the current fitted table path is
  wrong.
- Do not start the broader hand-rolled TUI component audit in this slice; only promote it in backlog
  and task planning.

## Fixture Capture Design

Add a dev-only fixture capture helper that uses the existing Jira configuration/client path to fetch
selected issue descriptions or comments, sanitize the ADF JSON, and write fixture files under
`internal/adf/testdata`.

The helper should:

- Be explicit and manually invoked, not part of app startup or tests.
- Accept enough input to identify an issue description or a specific comment.
- Write deterministic, formatted `*.adf.json` fixtures.
- Avoid writing credentials, account IDs, raw user identifiers, Jira hostnames, private URLs, or
  organization-specific values when a stable placeholder can preserve structure.
- Preserve ADF node types, marks, nesting, table shape, code text shape, and other rendering-relevant
  structure.

If direct Jira capture takes longer than expected, the fallback is a local sanitizer that reads a raw
ADF JSON file and writes a sanitized fixture. Normal golden tests remain the same either way.

## Rendering Design

Keep `internal/adf.Render(node) string` as the renderer boundary. It should continue to produce
terminal-friendly plain text with Markdown-like fences for code blocks and `[table]` markers for
tables.

For code blocks, change the TUI rich-body renderer rather than the ADF renderer:

- Parse fenced code blocks as it does today.
- Render code lines with the existing code-block theme foreground/background.
- Remove the literal top/bottom `+---+` rules and side `|` borders.
- Keep a small left/right padding so code is visually distinct.
- Do not add extra blank lines beyond the existing single separation before a block.
- Preserve width fitting and truncation behavior for long lines.

This keeps screen usage compact while still making code visually apparent through color and
background styling.

## Tests

Add tests at two levels:

- `internal/adf` fixture tests for any new sanitized real Jira payloads and their golden output.
- `internal/tui` rendering tests that assert code blocks no longer include ASCII border glyphs and
  still render code content with bounded width and no extra blank lines.

Golden fixtures should include at least:

- A real/sanitized description with nested lists, links, mentions, statuses or panels, and code.
- A real/sanitized comment with code-heavy content.
- A real/sanitized table or mixed content payload only if it differs meaningfully from current
  synthetic coverage.

## Documentation And Backlog

Update `docs/backlog.md` so the ADF real-fixture item is removed or narrowed when complete.

Promote `Audit hand-rolled TUI rendering and input components for replacement with maintained Bubble
Tea, Bubbles, Lip Gloss, or compatible public libraries` as the next major initiative after ADF
completion. This initiative exists to keep the codebase simple, reduce custom UI bugs, and align
with the project preference for maintained third-party packages where they fit.

Update `docs/releases/CHANGELOG.md`, `docs/project-state.md`, `tasks/todo.md`, and `tasks/lessons.md`
when behavior or durable workflow changes.

## Verification

- Run focused ADF tests: `go test ./internal/adf -count=1`.
- Run focused TUI rich-rendering tests for code blocks.
- Run `go test ./... -count=1`.
- Run `make check`.
- Run `make install-user` if user-visible rendering or helper behavior changes.
