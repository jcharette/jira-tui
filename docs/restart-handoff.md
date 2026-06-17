# Restart Handoff

Last updated: 2026-06-13

## Current State

- Working directory: `/Users/joncha/personal-dev/jira-tui`
- Installed binary: `/Users/joncha/bin/jira`
- Main verification command: `make check`
- Install/update command: `make install-user`
- Latest verified state: `make check` passed and `make install-user` rebuilt the user binary.

## What Landed In This Session

- Added persistent TOML config and config TUI, with validation and live Jira connection testing.
- Renamed the installed binary to `jira`.
- Added `make install-user`.
- Added saved views, default project config, configurable colors, and symbol modes.
- Reworked the issue list into a viewport-backed tree table with parent/subtask enrichment.
- Added focused ticket detail with tabs: Description, Links when present, Hierarchy, Comments, Actions.
- Added Jira ADF-aware rendering for descriptions/comments, including links, inline code, code blocks, lists, tables, panels/statuses, and mentions.
- Added link detection and link actions.
- Added read-only comments plus multi-line comment composer.
- Added Jira user search mention picker for comment mentions.
- Centralized key bindings and added contextual help.
- Ran a UX review pass and applied multiple ticket-detail layout cleanups.

## Current Ticket Detail UX Model

- Header bands:
  - issue identity: key, status, type
  - title-style summary
  - compact metadata row: assignee, priority, updated, reporter when available
  - plain-marker tab bar
- The tab bar uses a plain `>` marker as the single strong section-focus indicator.
- Body section headers are content headings, not selected controls.
- The body renders only the selected section.
- Inactive section names and badges live only in the tab bar.
- `tab` / `shift+tab` and `n` / `p` move selected detail section.
- `d`, `h`, `m`, `l` jump to Description, Hierarchy, Comments, Links.
- `enter` activates interactive sections such as Links, Hierarchy, and Actions.
- The footer starts with the active key context, such as `Ticket Detail`, `Links`, or `Add Comment`.
- Long detail views show a pager with active section name on the left and line range on the right.

## Important Design Decisions

- Do not add one-off edit shortcuts to ticket detail.
- Future writes must use explicit modes:
  - view mode
  - comment compose mode
  - edit-field mode
  - transition mode
  - action menu or command palette
- Jira writes must be metadata-driven. Ask Jira for edit/create/transition metadata, allowed values, required fields, users, statuses, issue types, boards, and sprints before rendering forms or submitting writes.
- Keep Jira work on the worker/command path so the TUI stays responsive.
- Prefer maintained Bubble Tea, Bubbles, Lip Gloss, or public packages before hand-rolling UI or parsing behavior.

## Files Most Relevant To Resume

- `internal/tui/model.go`: core Bubble Tea model, constructor/options, update loop, and top-level
  view dispatch.
- `internal/tui/detail.go`, `comment.go`, `create_issue.go`, `claude_assist.go`, `issue_list.go`,
  `rich_text.go`, `chrome.go`, `navigation.go`, and `results.go`: same-package TUI workflow files.
- `internal/tui/keymap.go`: centralized key contexts and footer/help bindings.
- `internal/tui/*_test.go`: workflow regression tests, with shared fakes/helpers in
  `model_test.go`.
- `internal/jira/client.go`: Jira API wrapper.
- `internal/worker/pool.go`: background Jira work dispatcher.
- `internal/adf/`: Jira ADF terminal renderer.
- `internal/linkdetect/`: URL/email detection.
- `internal/mentiondetect/`: comment mention detection.
- `internal/config/` and `internal/configui/`: config persistence and TUI.
- `docs/backlog.md`: current task queue.
- `docs/project-state.md`: current behavior and architecture.
- `docs/roadmap.md`: milestone-level plan.

## Good Resume Prompt

```text
We are in /Users/joncha/personal-dev/jira-tui. Read docs/restart-handoff.md,
docs/project-state.md, docs/backlog.md, and docs/working-agreement.md first.
Continue the Jira TUI UX work from the backlog. Use existing Bubble Tea/Bubbles/Lip Gloss
patterns or public packages before hand-rolling. Keep Jira work on the worker model.
Run make check and make install-user before stopping.
```

## Suggested Next Work

1. Continue ticket detail UX polish.
   - Improve the visual rhythm inside selected sections, especially Description and Comments.
   - Consider whether the tab bar needs a clearer active style or better spacing.
   - Keep detail body as one selected section only.

2. Improve Hierarchy detail.
   - Make parent/children/subtasks more useful inside the Hierarchy tab.
   - Consider linked issues/dependencies as a separate future section.

3. Plan edit workflows before implementing writes.
   - Define command palette/action menu behavior.
   - Add metadata discovery adapters before field edit, transition, assignment, create, or subtask flows.

4. Keep testing with realistic Jira tickets.
   - Long descriptions.
   - Tables.
   - Code-heavy descriptions.
   - Epics with children/subtasks.
   - Tickets with links and comments.
