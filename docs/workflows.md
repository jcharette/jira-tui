# Workflows

Use these as short recipes for common tasks.

## Browse Assigned Work

1. Run `jira`.
2. Use `j/k` to move.
3. Press `enter` to open detail.
4. Press `esc` to return to the issue list.
5. Press `?` if the current screen is unclear.

The selected-ticket strip under the view controls summarizes the focused ticket before you open it.

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

The Ticket Dashboard section is the fastest orientation point for developer work. It summarizes the
ticket owner, status, priority, latest comment, Start Work, Claude planning/review, comments,
worklogs, hierarchy, links, Jira open/copy actions, and workflow updates together without bypassing
the existing review prompts.

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

## Set Parent Or Estimates

1. Open ticket detail.
2. Press `.` for Ticket Actions.
3. Choose Set Parent or Edit Estimates.
4. For Parent, enter the parent issue key, or leave it blank to clear the parent.
5. For Estimates, enter Original and/or Remaining estimate values such as `2d`, `3h`, or `30m`.
6. Press `enter` to save.

Parent and Estimates actions only appear when Jira edit metadata says the selected issue can edit
those fields. Estimate edits update Jira time tracking estimates; they do not create worklogs.

## Comment On A Ticket

1. Open ticket detail.
2. Move to Comments.
3. Press `enter` to add a comment.
4. Use `@` mentions from the mention picker when needed.
5. Use `ctrl+r` to refine the local draft with Claude when Ticket Assist is enabled.
6. Review before submitting.

Comment composition supports basic formatting, detected links, and Claude refinement before the
normal Jira post/update confirmation.

## Create A Ticket

1. Press `n`.
2. Choose an issue type.
3. Fill Summary, Description, and supported Jira metadata fields.
4. Press `ctrl+s` to create.

Subtask creation is available from focused ticket actions and reuses the same metadata-backed form
when the focused ticket is not an Epic. For Epic work, create a Story or Task under the Epic so the
work appears on sprint boards.

## Check Board Hygiene

Audit your current in-progress tickets and their children for board visibility problems:

```bash
jira ticket check-board
```

Check one ticket:

```bash
jira ticket check-board ABC-123
```

Use `--fix` to print proposed fixes and confirm them in one prompt. Safe fixes assign unassigned
work to the current user, add tickets to the active sprint when configured, and attempt Story/Task
conversion for Epic-owned Sub-tasks before reporting manual follow-up.

## Account For Toil

From the TUI:

1. Press `T`.
2. Fill Summary, Duration, and an optional Note.
3. Toggle Close after create with `space` when the checkbox is focused.
4. Press `ctrl+s` to create the ticket, log the work, and optionally close it safely.

Create and log a quick toil ticket from the shell:

```bash
jira ticket create-toil --summary "rotate certs" --time 45m --note "prod cert cleanup"
```

New toil tickets get label `toil`. If Jira exposes an issue type named `Toil`, the command uses it;
otherwise it uses the first safe non-subtask issue type in the configured project.

Update an existing toil ticket:

```bash
jira ticket update-toil DEVOPS-123 --time 30m --note "follow-up validation"
```

Close a toil ticket after logging final time:

```bash
jira ticket close-toil DEVOPS-123 --time 15m --note "done"
```

Omit the ticket key on `update-toil` or `close-toil` to pick from open assigned toil tickets. The
picker searches `labels = toil OR issuetype = Toil` and keeps close transitions safe: if Jira
requires extra fields to close the ticket, the command logs time but leaves the ticket open and
reports the skipped close.

## Create Tickets With Claude Drafts

In the Create Ticket form, enable AI Generated mode to ask Claude for a local ticket draft. After a
draft is applied to the editable form, `ctrl+r` refines the current Summary and Description with
Claude before any Jira write. If Claude returned Open Questions, answer them locally and use
`ctrl+r` from the Open Questions panel to refine with those saved answers.

## Improve An Epic With Ticket Assist

1. Open the epic detail view.
2. Press `a`, choose the whole-ticket Ticket Assist action, and review the editable draft.
3. Answer any Open Questions locally and use `ctrl+r` to refine with those answers.
4. Press `ctrl+s`, confirm with `ctrl+s`, then review any Subtask Recommendations.
5. In Review Subtask Changes, use `enter` to apply one recommendation, `s` to skip, and `esc` when done.

Add recommendations open the normal child-ticket create flow. Modify recommendations post a review
comment to the existing child. Remove or defer recommendations close the child only when Jira exposes
a safe no-extra-fields close-as-invalid style transition; otherwise the app comments on the child for
manual review.

The Claude detail section also has Quality Review for read-only ticket readiness feedback and Draft
Comment for an editable Jira comment draft. Drafted comments still require the normal post
confirmation.

## Start Work From A Ticket

From the TUI:

1. Open or focus a ticket.
2. Press `.` for Ticket Actions.
3. Choose Start Work.
4. Review the repo, branch, optional Claude Branch Plan, assignment, status, and comment actions.
5. Confirm only when the preview looks correct.

From the shell:

```bash
jira start ABC-123
jira start
```

The same entry point appears in the Ticket Dashboard section and grouped Ticket Actions.

`jira start` without a ticket opens a picker from your default query.

When Claude is enabled and the Branch Plan feature is on, Start Work asks Claude for a read-only
implementation plan before the review screen. The generated plan is advisory only; branch and Jira
writes still require the normal confirmation, and the workflow falls back to the deterministic
review when Claude is unavailable.

## Commit And Finish Work

```bash
jira commit ABC-123
jira finish ABC-123
```

These workflows inspect local Git state, detect the Jira ticket when possible, preview writes, and
avoid duplicate Jira progress notes for already-reported commits.

When Claude is enabled and the Branch Plan feature is on, `jira commit` asks Claude for a compact
Jira progress note and shows it in the normal review prompt. If Claude is unavailable or returns an
empty result, the workflow uses the existing deterministic note.

When Claude is enabled and the PR Creation feature is on, `jira finish` asks Claude for a pull
request title/body and final Jira note. The generated text is shown in the normal review prompt
before any push, pull request, Jira comment, or transition runs.

`jira finish` can push and create or reuse a GitHub draft pull request through `gh`.

## Use Notifications

- Press `ctrl+n` to open the notification center.
- Press `enter` to open the selected ticket.
- Press `x` to clear one notification.
- Press `ctrl+x` to clear all notifications.

When configured, incoming ticket events can auto-open the notification center and keep it visible
until cleared. Optional system notifications use `beeep`.

## Use Diagnostics And Bug Reports

- Press `ctrl+d` to inspect recent background activity.
- Press `B` to open the bug report composer.
- Use `ctrl+r` to polish the local bug report title/body with Claude when Draft Ticket is enabled.

Bug reports can include a sanitized Diagnostics excerpt only when you opt in. GitHub opens only
after the normal `ctrl+s` submit.

## Change Appearance

Run:

```bash
jira config
```

Go to Appearance and use `j/k` to choose a theme. Use Advanced Colors only when you want manual color
overrides. Display > Symbol Mode can override the theme icon style.
