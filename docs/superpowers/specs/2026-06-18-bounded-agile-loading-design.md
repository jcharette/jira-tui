# Bounded Agile Loading Design

Sprint-oriented views already load Jira Agile metadata through the worker pool. The next step is to
make that path safe before expanding beyond the first board.

## UX

The issue list remains the primary surface. Sprint metadata continues to appear as compact header
state: loading, error, or loaded sprint count. There is no new user-facing query behavior.

## Architecture

Board discovery remains single-flight per sprint-oriented view. When board results arrive, the TUI
queues sprint reads for returned boards and starts only a small bounded number at a time. Completed
sprint reads store their board page, reduce the active count, and start the next queued board if
one exists. The generic worker pool remains the execution backend.

## Constraints

- Do not change issue JQL, saved views, cache keys, or loaded issues.
- Do not eagerly bypass the worker pool.
- Do not start unbounded board or sprint worker requests.
- Keep Diagnostics on the existing worker/API result path.
