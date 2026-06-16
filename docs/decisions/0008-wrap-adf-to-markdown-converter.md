# 0008: Wrap ADF-To-Markdown Conversion Behind The Renderer Boundary

Date: 2026-06-16

## Status

Accepted

## Context

Ticket detail rendering remains part of the active issue-browser milestone. The previous decision
kept a small terminal ADF renderer in `internal/adf` because no maintained Go renderer had been
found at that time.

A follow-up library spike found `github.com/rgonek/jira-adf-converter`, a maintained Go module and
CLI for converting Jira ADF JSON to GitHub Flavored Markdown. The candidate covers more ADF nodes
than the hand-rolled renderer, including headings, links, mentions, statuses, panels, dates,
inline cards, block cards, expands, and richer marks.

The spike also found that full replacement is not ideal yet: the converter's table output does not
match this app's existing table marker contract for every table shape we already support. The TUI
currently understands fenced code blocks and `[table]` marker blocks in Jira descriptions and
comments.

## Decision

Keep `internal/adf.Render(node) string` as the public boundary and wrap the converter inside that
package.

Use the converter for non-table ADF blocks, normalize the resulting Markdown for terminal/TUI
compatibility, and keep the existing custom table renderer for Jira table nodes. This preserves the
app-specific table path while gaining broader maintained-library coverage for text-oriented ADF
nodes.

## Consequences

- Callers in `internal/jira` and `internal/tui` do not change.
- Jira descriptions and comments now use Markdown-shaped output for links, headings, panels,
  statuses, cards, dates, and expands.
- Tables continue to render through `[table]` blocks so the existing TUI fitted table renderer
  still applies.
- The app now depends on `github.com/rgonek/jira-adf-converter`; future renderer changes should be
  evaluated against `internal/adf` fixture tests before changing TUI layout.
- Media nodes are not claimed as solved by this slice because the converter's default external-media
  fallback is not useful enough for terminal display without additional hooks.
