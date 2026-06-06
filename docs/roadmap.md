# Roadmap

This roadmap groups features by dependency order. The goal is to avoid rewriting architecture while
still delivering useful slices quickly.

## Feature Inventory

Core daily work:

- Issue lists from saved/default JQLs.
- Issue detail view.
- Fast navigation: list/detail split, jump to key, copy key/URL, open in browser.
- Comments: read and add.
- Transitions: view available transitions and move status.
- Create and edit issues.

Planning and team work:

- Sprint and board views.
- Epic views and epic children.
- Subtasks.
- Backlog and current sprint navigation.
- Bulk operations with confirmations.

App quality:

- Config file and profiles.
- Multiple Jira sites.
- Saved queries.
- API token auth check.
- OAuth later if needed.
- Cache recent issue lists/details/comments.
- Command mode for fast actions.

## Dependency Map

Issue detail depends on:

- Jira issue fetch API in `internal/jira`.
- Worker request/result type for issue detail.
- TUI state for selected issue details.
- ADF/plain-text rendering strategy for descriptions.

Comments depend on:

- Issue detail and selected issue state.
- ADF handling for comment body.
- Write-action confirmation flow.
- Worker request/result type for read/add comment.

Transitions depend on:

- Issue detail and selected issue state.
- Jira transition API.
- Confirmation/error states.
- Optional transition field handling.

Create/edit issue depends on:

- Config/default project context.
- Jira create/edit metadata discovery.
- Form/input components.
- Confirmation and validation UX.

Sprint/board views depend on:

- Jira Agile API support through `go-atlassian`.
- Config/default board or project.
- A second top-level view mode beyond issue list/detail.
- Potential caching because board/sprint data can expand quickly.

Config/profiles depend on:

- Choosing config format and path.
- Merge precedence: flags > env vars > config file > defaults.
- Saved query schema.
- Multi-site profile schema.

Command mode depends on:

- Stable app state model.
- Typed actions for search, open, comment, transition, assign, and config commands.
- Good validation/error messaging.

Caching depends on:

- Stable domain types.
- Clear invalidation points after write actions.
- Background refresh semantics.

## Milestones

### M0: Foundation

Status: complete (2026-06-06).

- Go + Bubble Tea app shell.
- Jira search through `go-atlassian`.
- Typed worker dispatcher.
- `ants`-powered bounded worker pool.
- Request IDs and stale-response protection.
- Background refresh.
- Makefile and project docs.

### M1: Useful Daily Issue Browser

Status: active.

Goal: use Jira TUI instead of opening Jira for assigned issue review.

- Issue detail fetch and display.
- Richer issue fields: description, priority, labels, components, fix versions, created, updated.
- List/detail layout with responsive terminal behavior.
- Open selected issue in browser.
- Copy selected issue key/URL.
- TUI navigation/render tests.

### M2: Communication

Status: planned.

Goal: handle the common "read context and respond" workflow.

- Issue comments view.
- Add comment.
- ADF/plain-text conversion for comments.
- Confirmation before posting.
- Background refresh keeps existing detail/comment data visible on failure.

### M3: Workflow Actions

Status: planned.

Goal: move tickets through normal work states without Jira web.

- View available transitions.
- Transition selected issue.
- Assign issue.
- Edit lightweight fields: summary, labels, priority, components.
- Clear success/failure messages.

### M4: Configuration And Saved Workspaces

Status: planned.

Goal: stop relying on exported env vars for daily use.

- Config file under `~/.config/jira-tui/config.toml` unless a better library/path choice emerges.
- Saved queries.
- Default project and board.
- Multiple Jira site profiles.
- `auth check` command or equivalent startup diagnostics.

### M5: Planning Views

Status: planned.

Goal: support team planning and sprint navigation.

- Current sprint view.
- Board/backlog view.
- Epic list and epic children.
- Subtasks in issue detail and planning views.
- Lightweight caching for board/sprint data.

### M6: Creation And Editing

Status: planned.

Goal: create and reshape Jira work from the terminal.

- Create issue.
- Create subtask.
- Edit description.
- Link issues.
- Add/remove issue from epic.
- Move issue between sprint/backlog if API support is clean.

### M7: Power User Workflows

Status: planned.

Goal: make repeated Jira work fast.

- Command mode.
- Bulk transitions/assignments/labels with confirmations.
- Worklog support.
- More saved filters.
- Packaging/install story.

## Near-Term Implementation Plan

1. Add issue detail support.
   - Add `internal/jira.GetIssue`.
   - Add `worker.KindGetIssue`.
   - Add TUI detail state and selected issue request IDs.
   - Render list/detail split.

2. Add comments read path.
   - Add comments to issue detail or separate request.
   - Decide ADF/plain-text rendering helper.
   - Add tests for refresh/error behavior.

3. Add first write path: add comment.
   - Add confirmation UI.
   - Add worker request/result for comment creation.
   - Refresh detail/comments after successful write.

4. Add transition support.
   - Fetch transitions for selected issue.
   - Render transition chooser.
   - Apply selected transition with confirmation.

5. Add config file and saved queries.
   - Check maintained config libraries before implementation.
   - Keep typed config struct and explicit merge precedence.
