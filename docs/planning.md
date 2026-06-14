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

UX baseline: every TUI view should have deliberate visual structure and discoverable navigation
before it is considered complete. Use the shared styling/theme layer for panes, borders, color, and
status emphasis, and make section/pane movement explicit in the footer or local help.
Ticket detail should keep clear mode boundaries: normal reading/navigation, comment composition,
field editing, transitions, and action menus should each have their own key context instead of
accumulating one-off shortcuts in the detail view.

Jira customization baseline: reads can display Jira data as returned, but writes must be
metadata-driven. Create, edit, transition, assign, board, and sprint workflows should ask Jira for
the allowed fields, required fields, options, transitions, users, and identifiers for the active
site/project/issue before rendering forms or submitting changes. Cache metadata only behind clear
invalidation points, because Jira admins can change these shapes independently of this app.

## Proposed Sequence

1. Issue detail model
   - Done: add Jira API method to fetch one issue by key.
   - Done: add fields needed for a useful detail panel.
   - Done: show detail for selected issue beside or below the list depending on terminal width.
   - Done: load details through the worker pool and ignore stale detail responses.
   - This is milestone M1 in [roadmap.md](roadmap.md).

2. Interaction polish
   - Prioritize rich ticket detail rendering before adding new workflow features.
   - Continue improving Jira ADF rendering for links, mentions, inline code, code blocks, tables,
     panels, statuses, lists, and blockquotes.
   - Add detail-specific navigation for long content, links, and sections.
   - Add help text that adapts to the current screen and pane focus.
   - Continue improving empty/loading/error states.
   - Add `enter` to focus issue detail.
   - Add `esc` to return to list.

3. Query control
   - Add `/` or `s` to edit JQL.
   - Add a small history of recent queries.
   - Persist the last successful query.
   - Add saved views instead of changing the default query semantics in place: assigned to me,
     reported/created by me, project open, watching, and epic-focused drill-down.

4. First write action
   - Add comment creation, because it is lower risk than editing fields or transitions.
   - Require explicit confirmation before posting.
   - Run write actions through request/result messages, not direct UI mutation.
   - Keep the mention picker as a bounded app-owned result list: Jira-specific code populates user
     rows from user-search results and converts selected users into ADF mention nodes.
   - For later field, transition, and assignment writes, add Jira metadata discovery before adding
     the TUI form.

## Non-Goals For Now

- Full Jira administration.
- Workflow configuration.
- Replacing every Jira screen.
- OAuth setup before the core TUI is useful.
