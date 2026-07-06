# Changelog

All notable changes to this project should be recorded here.

## Unreleased

- Added a Developer Workbench detail section and developer-first Ticket Actions grouping so Start
  Work, Claude planning/review, comments, worklogs, and Jira open/copy actions are easier to find.
- Tightened Developer Workbench into a compact cockpit summary for developer actions plus Comments,
  Worklog, Hierarchy, and Links state.
- Reframed the ticket detail Workbench as a Ticket Dashboard with ownership, recent comment signal,
  and next-action rows.
- Added a selected-ticket strip to the issue list so the initial screen shows the focused key,
  status, priority, owner, and next action.
- Renamed the issue-list local filter chip to `Status All/Active` so it is not confused with the
  active Jira query filter.

## 1.0.11 - 2026-07-06

- Reframed the active AI roadmap around polishing existing Claude workflows before provider-neutral
  provider expansion.
- Added optional Claude-assisted Jira progress note drafting to `jira commit`, with fallback to the
  existing deterministic note.
- Added optional Claude-assisted pull request text and final Jira note drafting to `jira finish`.
- Added Claude ticket-detail Quality Review and Draft Comment actions.
- Added optional read-only Claude Branch Plans to Start Work review before branch or Jira writes.
- Added `ctrl+r` Claude refinement for local Create Ticket drafts before Jira creation.
- Added `ctrl+r` Claude polish for local bug report title/body drafts before opening GitHub.
- Added `ctrl+r` Claude refinement for local Add/Edit Comment drafts before Jira post/update.
- Fixed reviewed UX gaps: Start Work now stops Jira writes after a required branch failure, Ticket
  Assist comment posting honors `allow_jira_writes`, Draft Comment uses its own feature flag, and
  keyboard/command docs match current shortcuts.
- Added UX render snapshots for representative TUI states to catch help, footer, modal, and flow
  drift.

## 1.0.10 - 2026-06-30

- Fixed notification-center `enter` so it opens the selected ticket even when that ticket is not in
  the current loaded issue list.

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
