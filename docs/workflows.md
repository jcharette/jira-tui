# Workflows

Use these as short recipes for common tasks.

## Browse Assigned Work

1. Run `jira`.
2. Use `j/k` to move.
3. Press `enter` to open detail.
4. Press `esc` to return to the issue list.
5. Press `?` if the current screen is unclear.

## Switch Views

- Press `tab` for the next saved view.
- Press `shift+tab` for the previous saved view.
- Press `/` to run direct JQL.
- Press `v` to save the current query as a view.

Saved views can include child loading, and the app preserves useful cached results while refreshing
in the background.

## Use Layouts

Press `L` to cycle:

- `Lanes`: default status-grouped view.
- `Table`: dense table view.
- `Workbench`: issue list plus focused context panel on wide terminals.
- `Planning`: sprint-aware planning layout when Agile metadata is useful.

## Load Children And Epics

- Select a parent issue.
- Press `x` to load open children.
- Press `X` to load all children, including resolved issues.
- Press `z` to collapse or expand loaded descendants.

This does not rewrite your JQL. It loads hierarchy around the selected issue.

## Open And Edit A Ticket

1. Select a ticket and press `enter`.
2. Use `tab` / `shift+tab` to move through detail fields and sections.
3. Press `enter` on editable fields such as Status or Assignee.
4. Use direct shortcuts where available:
   - `s` edits Summary.
   - `p` edits Priority.
   - `.` opens Ticket Actions.

Jira writes use Jira metadata before submitting, so required transition fields and supported edit
fields are discovered from the configured site/project/issue.

## Add A Ticket To A Sprint

1. Open ticket detail.
2. Press `.` for Ticket Actions.
3. Choose Sprint.
4. Select the active sprint or one of the future sprints.
5. Press `enter` to add the ticket.

Sprint Actions use Jira Agile APIs instead of editing the raw Sprint custom field. The active sprint
is listed first, future sprints follow, and closed sprints are hidden. Set `Default Board ID` in
`jira config` when a project has multiple boards and you want sprint actions scoped to one board.

## Set Versions Or Due Date

1. Open ticket detail.
2. Press `.` for Ticket Actions.
3. Choose Set Fix Version, Set Affects Version, or Set Due Date.
4. Pick Jira-provided version values, or type the due date as `YYYY-MM-DD`.
5. Press `enter` to save.

These actions only appear when Jira edit metadata says the selected issue can edit the field. If a
workflow hides versions or due date from the edit screen, the action stays hidden or disabled.

## Comment On A Ticket

1. Open ticket detail.
2. Move to Comments.
3. Press `enter` to add a comment.
4. Use `@` mentions from the mention picker when needed.
5. Review before submitting.

Comment composition supports basic formatting and detected links.

## Create A Ticket

1. Press `n`.
2. Choose an issue type.
3. Fill Summary, Description, and supported Jira metadata fields.
4. Press `ctrl+s` to create.

Subtask creation is available from focused ticket actions and reuses the same metadata-backed form.

## Start Work From A Ticket

From the TUI:

1. Open or focus a ticket.
2. Press `.` for Ticket Actions.
3. Choose Start Work.
4. Review the repo, branch, assignment, status, and comment actions.
5. Confirm only when the preview looks correct.

From the shell:

```bash
jira start ABC-123
jira start
```

`jira start` without a ticket opens a picker from your default query.

## Commit And Finish Work

```bash
jira commit ABC-123
jira finish ABC-123
```

These workflows inspect local Git state, detect the Jira ticket when possible, preview writes, and
avoid duplicate Jira progress notes for already-reported commits.

`jira finish` can push and create or reuse a GitHub draft pull request through `gh`.

## Use Notifications

- Press `ctrl+n` to open the notification center.
- Press `x` to clear one notification.
- Press `ctrl+x` to clear all notifications.

When configured, incoming ticket events can auto-open the notification center and keep it visible
until cleared. Optional system notifications use `beeep`.

## Use Diagnostics And Bug Reports

- Press `ctrl+d` to inspect recent background activity.
- Press `B` to open the bug report composer.

Bug reports can include a sanitized Diagnostics excerpt only when you opt in.

## Change Appearance

Run:

```bash
jira config
```

Go to Appearance and use `j/k` to choose a theme. Use Advanced Colors only when you want manual color
overrides. Display > Symbol Mode can override the theme icon style.
