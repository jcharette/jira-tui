# Planning

See [roadmap.md](roadmap.md) for the full feature inventory, dependency map, and milestone plan.

## Current Milestone: Useful Daily Issue Browser

Objective: make Jira TUI good enough to use instead of opening Jira for assigned issue review.

Every implementation step should finish by updating the relevant docs listed in
[working-agreement.md](working-agreement.md).

Library baseline: before building non-core infrastructure, check for well-maintained libraries and
wrap good fits behind internal boundaries.

Performance baseline: Jira work should run through Bubble Tea commands, use request IDs where
responses can overlap, preserve useful existing state during refreshes, and avoid blocking the UI.
Jira IO should use the typed dispatcher and bounded worker pool so issue search, issue details,
comments, and sprint data do not each invent their own concurrency shape.

## Proposed Sequence

1. Issue detail model
   - Add Jira API method to fetch one issue by key.
   - Add fields needed for a useful detail panel.
   - Show detail for selected issue beside or below the list depending on terminal width.
   - Load details through the worker pool and ignore stale detail responses.
   - This is milestone M1 in [roadmap.md](roadmap.md).

2. Interaction polish
   - Add help text that adapts to the current screen.
   - Continue improving empty/loading/error states.
   - Add `enter` to focus issue detail.
   - Add `esc` to return to list.

3. Query control
   - Add `/` or `s` to edit JQL.
   - Add a small history of recent queries.
   - Persist the last successful query.

4. First write action
   - Add comment creation, because it is lower risk than editing fields or transitions.
   - Require explicit confirmation before posting.
   - Run write actions through request/result messages, not direct UI mutation.

## Non-Goals For Now

- Full Jira administration.
- Workflow configuration.
- Replacing every Jira screen.
- OAuth setup before the core TUI is useful.
