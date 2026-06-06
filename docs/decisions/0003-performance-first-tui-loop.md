# 0003: Use A Performance-First TUI Update Loop

Date: 2026-06-06

## Status

Accepted

## Context

Jira TUI is intended to become a full daily-use Jira client with issue details, comments, edits,
transitions, sprint views, epics, subtasks, and background data updates.

As the app grows, Jira requests can overlap. Slow responses must not block interaction or overwrite
newer state. The TUI should remain responsive while refreshes happen in the background.

## Decision

Use Bubble Tea commands for asynchronous Jira work and keep the model protected against stale
responses.

The TUI loop should:

- Use background refresh commands instead of blocking UI updates.
- Assign request IDs to refreshes and ignore stale responses.
- Preserve selection across refreshed issue lists when the selected issue still exists.
- Bound Jira calls with request timeouts.
- Keep rendering derived from model state only.

## Consequences

- Future Jira fetches should follow the same command/message/request-ID pattern.
- Details, comments, sprint data, and write actions should avoid blocking the UI loop.
- Larger views should favor incremental loading and cached state over repeated full reloads.
- Background refresh failures should not discard currently displayed data.

