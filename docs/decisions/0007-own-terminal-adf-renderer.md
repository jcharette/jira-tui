# 0007: Own A Terminal ADF Renderer Behind An Internal Boundary

Date: 2026-06-13

## Status

Accepted

## Context

Jira Cloud stores rich text fields such as descriptions and comments as Atlassian Document Format
(ADF). Ticket detail is a core view for this app, so descriptions, comments, links, mentions, inline
code, code blocks, lists, blockquotes, panels/statuses, and tables must be readable in a terminal.

We checked for a maintained Go formatter before expanding custom rendering:

- `go-atlassian` exposes ADF models and Jira ADF endpoints, but no terminal/plain-text/Markdown
  renderer.
- Local Go module search did not reveal an installed ADF renderer.
- A Go module check against common Jira client packages did not reveal an ADF formatter direction.
- Atlassian documents the ADF schema, nodes, marks, and client libraries, but the richer rendering
  ecosystem is Atlaskit/JavaScript-oriented, which is not a clean dependency for this Go TUI.
- npm metadata checks for Atlaskit packages could not complete in the sandbox because registry DNS
  lookup failed.

## Decision

Keep a small purpose-built renderer in `internal/adf` for terminal output.

The renderer should remain isolated behind this package boundary, covered by fixtures/tests, and
focused on readable terminal output rather than exact Jira web parity. The TUI may apply final
styling after `internal/adf` preserves semantic markers such as code spans and tables.

If a maintained Go ADF renderer becomes available later, replace or wrap it behind `internal/adf`
instead of changing Jira client or TUI call sites directly.

## Consequences

- We can make ticket detail usable now without embedding a JavaScript renderer.
- We need tests for every supported ADF construct, especially tables, links, mentions, and code.
- Unsupported ADF nodes should degrade visibly and predictably instead of disappearing silently.
