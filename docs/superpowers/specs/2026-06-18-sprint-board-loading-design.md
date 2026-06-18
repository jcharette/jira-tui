# Sprint And Board Incremental Loading

## Goal

Add Jira Agile board and sprint metadata loading behind the existing worker model so sprint-oriented
views can quietly discover available boards and active/future sprints without changing the issue
query path.

## Scope

- Add Jira Agile client support for board pages and board sprint pages.
- Preserve pagination metadata so callers can request the next page later.
- Route board and sprint reads through the worker pool.
- Surface compact loading/error/count state in the TUI for sprint-oriented views.

## Non-Goals

- Do not rewrite JQL or add query generation.
- Do not add saved-view schema changes.
- Do not implement full board/sprint picker UX in this slice.
- Do not eagerly load every sprint for every accessible board.

## UX Shape

Sprint-aware views should keep the current issue list as the primary surface. Board/sprint metadata
loads in the background and appears as compact header status, with errors recorded through the same
worker/diagnostics path as other Jira reads. The first slice loads boards for the configured default
project, then loads active/future sprints for the first returned board. Pagination fields are kept so
follow-up UI can add explicit "load more" controls without changing the Jira layer again.

## Verification

- Jira client tests for board and sprint page parsing.
- Worker tests for successful and invalid board/sprint requests.
- TUI tests for sprint-view planning metadata trigger and compact header state.
- Full verification with focused tests, `go test ./... -count=1`, `make check`, and
  `make install-user`.
