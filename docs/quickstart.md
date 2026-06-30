# Quickstart

This is the shortest path from a fresh install to useful daily Jira work.

## 1. Configure Jira

```bash
jira config
```

Set:

- Jira base URL, for example `https://your-domain.atlassian.net`
- Jira account email
- Jira API token
- Default Jira project key

Saved API tokens are stored in the OS keychain. The config file keeps a keyring reference.

## 2. Open Your Work

```bash
jira
```

The default view is assigned, unresolved work for the configured project. The top header shows the
active view, issue count, sync state, and app version.

## 3. Learn The Screen

- Press `j/k` or arrow keys to move.
- Press `enter` to open the selected ticket.
- Press `?` anywhere for context-aware help.
- Press `esc` to go back from detail screens or modals.
- Press `q` to quit.

The footer shows the most useful keys for the current screen. It is intentionally short; use `?`
for the full key list.

## 4. Browse Faster

- Press `tab` / `shift+tab` to switch saved views.
- Press `L` to cycle layouts.
- Press `f` to toggle the local active-ticket filter.
- Press `r` to refresh the current view.
- Press `/` to run JQL or generate JQL with AI when enabled.

## 5. Work With Hierarchy

- Press `x` to load open child issues for the selected ticket.
- Press `X` to load all child issues, including resolved work.
- Press `z` to collapse or expand loaded descendants.

## 6. Act On A Ticket

Open a ticket with `enter`, then:

- Press `tab` / `shift+tab` to move through detail sections.
- Press `.` for Ticket Actions.
- Press `s` to edit Summary.
- Press `p` to edit Priority.
- Select Status, Assignee, or other focused fields and press `enter` to edit.

## 7. Keep Jira Visible

Notifications stay in the app until cleared.

- Press `ctrl+n` to open notifications.
- Press `enter` to open the selected ticket.
- Press `x` to clear the selected notification.
- Press `ctrl+x` to clear all notifications.

## 8. Troubleshoot

- Press `ctrl+d` for Diagnostics.
- Press `B` to open a GitHub bug report composer with optional sanitized Diagnostics.

Diagnostics avoid raw tokens, raw request/response bodies, comments, descriptions, and full JQL.
