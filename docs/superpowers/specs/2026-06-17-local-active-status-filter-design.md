# Local Active Status Filter Design

## Purpose

Saved views can intentionally load tickets in any Jira status. That is useful for complete context,
but day-to-day triage often needs a quick way to focus on tickets that are still workable without
changing the saved view or active JQL. Add a local issue-table filter that hides terminal statuses
inside the TUI only.

## Scope

- Add a table-mode `f` toggle between `All` and `Active`.
- Keep `All` as the default for every loaded view.
- Keep the active JQL, saved views, Jira worker requests, cache keys, cached records, and loaded
  `m.issues` unchanged.
- Apply the filter only to the issue-table presentation and table navigation.
- Treat terminal status names as local text matches after normalization: `done`, `closed`,
  `resolved`, `canceled`, and `cancelled`.
- Treat every other status as active, including custom statuses such as `To Do`, `Ready`, `In
  Progress`, `Review`, `Blocked`, `Waiting`, and `On Hold`.

## Interaction

Pressing `f` in the issue table toggles the local status filter. The footer advertises `f active` or
equivalent short copy. The filter summary/list title shows when the active filter is on and how many
loaded tickets are currently visible, for example `37 issues  Active 24 shown`.

When the filter hides the currently selected issue, selection moves to the nearest visible issue. If
no issue is visible, the table renders an empty filtered state that explains the active filter hid
the loaded tickets and tells the user to press `f` to show all loaded issues.

Normal table navigation, paging, first/last jumps, opening detail, copy/open-browser commands, and
selected-detail prefetch operate on visible issues while the filter is active. Detail mode keeps
showing the ticket already opened; the filter only affects the table when returning to it.

## Data Model

Add small model-local state for the table status filter. This state is not persisted and does not
leave the TUI package. The loaded issue slice remains the source of truth.

Add helpers that answer:

- which issue indexes are visible under the current local filter;
- whether an individual issue is terminal by normalized status text;
- how to repair selection when the selected issue is no longer visible.

These helpers should live in the existing `internal/tui` package, near issue-list or navigation code,
unless implementation shows a small same-package helper file is clearer.

## Rendering And Navigation

Rendering should derive issue rows from the existing display tree plus the current status filter.
Rows hidden by the status filter are omitted from the table. The list title should continue to show
the loaded issue count and add visible-count context only when the filter changes the view.

Navigation should use visible issue indexes instead of raw `m.issues` indexes while the filter is
active. Paging and first/last should land on visible rows. Selection visibility should clamp against
the rendered filtered rows so the cursor does not point at a hidden ticket.

## Error Handling

This feature does not add Jira error states. Unknown or empty status text is treated as active so
the filter does not accidentally hide tickets because Jira returned incomplete data.

## Testing

Add focused TUI tests for:

- default `All` mode still renders terminal and non-terminal tickets;
- `f` toggles Active mode and hides terminal statuses without submitting a command;
- terminal matching handles common variants such as `Done`, `Resolved`, `Closed`, `Canceled`, and
  `Cancelled`;
- active filtering keeps statuses such as `To Do`, `In Progress`, `Review`, `Blocked`, and `Waiting`;
- navigation, paging, and first/last jumps skip hidden rows;
- selection repairs when the active filter hides the selected issue;
- filtered-empty rendering tells the user how to show all loaded issues;
- the footer/help includes the table filter binding.
