# Changelog

All notable changes to this project should be recorded here.

## 1.0.0 - 2026-06-19

Initial public baseline release of `jira-tui`.

- Added a terminal-first Jira Cloud client with saved views, direct JQL, recent query history,
  status-oriented layouts, hierarchy expansion/collapse, sorting, filtering, and responsive table,
  lane, workbench, and planning views.
- Added focused ticket detail with rich Jira ADF description/comment rendering, links, hierarchy,
  comments, worklogs, issue links, and metadata-backed Jira actions.
- Added Jira write workflows for comments, status transitions, summary/priority/assignee/label/
  component edits, create/subtask flows, issue links, and worklog add/edit/delete, all routed through
  bounded background workers.
- Added `jira start`, `jira commit`, and `jira finish` developer workflows with reviewed Git, Jira,
  and GitHub writes behind local adapter boundaries.
- Added optional local Claude CLI assistance for JQL generation, ticket planning, ticket drafting,
  inline description help, and gated Jira write/apply flows.
- Added persistent in-app ticket notifications with optional cross-platform system notifications
  through `github.com/gen2brain/beeep`.
- Added local performance infrastructure: worker prioritization/coalescing, stale-while-refresh
  caches, SQLite-backed Jira read caches, conservative cache cleanup, and Diagnostics visibility.
- Added the security baseline: HTTPS-only Jira URLs, OS keychain storage for Jira API tokens,
  owner-only config/cache/diagnostics files, sanitized bounded Diagnostics, explicit opt-in bug
  report excerpts, and a dedicated security overview.
- Added release artifacts, Go install support, source install support, and a repo-managed Homebrew
  formula.
