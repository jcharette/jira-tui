# Changelog

All notable changes to this project should be recorded here.

## Unreleased

## 1.0.9 - 2026-06-29

- Added quick toil-ticket accounting with a TUI `T` form plus `jira ticket create-toil`,
  `jira ticket update-toil`, `jira ticket close-toil`, and `jira ticket toil` for creation,
  worklog updates, picker-backed selection, and safe terminal transitions.

## 1.0.8 - 2026-06-22

- Added guided Ticket Assist sessions with parsed Open Questions, answer-driven refinement, loaded
  hierarchy context, and first-class subtask recommendation review for keep/add/modify/close child
  work after Summary and Description are applied.
- Expanded ticket Overview descriptions by default and added simple Ticket Assist text selection
  with Shift+Arrow when available plus a `ctrl+space` fallback.

## 1.0.7 - 2026-06-22

- Fixed Ticket Assist draft editing so typed letters stay in the editor; refine and comment actions now use `ctrl+r` and `ctrl+c`.

## 1.0.6 - 2026-06-22

- Fixed parent updates so retained issue detail and active-view caches reflect changed hierarchy state immediately.
- Fixed time tracking detail behavior so Jira estimates are read from issue detail, prefilled in the editor, and patched locally after updates.

## 1.0.5 - 2026-06-21

- Added metadata-backed Ticket Actions for setting or clearing Parent and editing Time Tracking
  estimates when Jira edit metadata exposes those fields as editable.

## 1.0.4 - 2026-06-20

- Added metadata-backed Ticket Actions for Fix Version, Affects Version, and Due Date when Jira edit
  metadata exposes those fields as editable.

## 1.0.3 - 2026-06-20

- Added Sprint ticket actions for adding the selected issue to active or future Jira Agile sprints.

## 1.0.2 - 2026-06-20

- Added curated appearance themes with matching issue-list symbol styles.
- Added quickstart, workflow, and keyboard reference docs.
- Improved Jira rich-text rendering for escaped Markdown punctuation.
- Clarified issue-list footer help for child expansion.

## 1.0.1 - 2026-06-19

- Added the app version to the main TUI and config UI headers.

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
