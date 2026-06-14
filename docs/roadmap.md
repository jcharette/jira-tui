# Roadmap

This roadmap groups features by dependency order. The goal is to avoid rewriting architecture while
still delivering useful slices quickly.

## Feature Inventory

Core daily work:

- Issue lists from saved/default JQLs.
- Issue detail view.
- Fast navigation: full-width issue list, focused detail view, jump to key, copy key/URL, open in browser.
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
- Secure credential storage for OAuth/API tokens.
- Cache recent issue lists/details/comments.
- Command mode for fast actions.
- Git integration and AI-assisted ticket/PR workflows.

## Dependency Map

Issue detail depends on:

- Jira issue fetch API in `internal/jira`.
- Worker request/result type for issue detail.
- TUI state for selected issue details.
- Dedicated ADF renderer for descriptions, comments, links, mentions, inline code, code blocks,
  lists, blockquotes, panels/statuses, and tables.
- Detail-specific scrolling and navigation for long content.

Comments depend on:

- Issue detail and selected issue state.
- ADF handling for comment body.
- Write-action confirmation flow.
- Worker request/result type for read/add comment.

Transitions depend on:

- Issue detail and selected issue state.
- Jira transition API.
- Transition metadata for allowed transitions, required fields, and field options.
- Confirmation/error states.
- Required transition field handling.

Create/edit issue depends on:

- Config/default project context.
- Jira create/edit metadata discovery for issue types, required fields, field schemas, field options,
  priorities, statuses, and custom fields.
- Jira issue link metadata/API support for link types and target issue validation.
- Assignable user search for assignee changes.
- Form/input components.
- Confirmation and validation UX.
- A command/action menu so edit, link, subtask, assignment, priority, and transition actions remain
  discoverable without crowding the ticket detail keymap.

Assignment depends on:

- Assignable user search for the selected project/issue.
- Account ID handling and user disambiguation.
- Permission-aware error messages.

Git and AI workflows depend on:

- Stable selected issue/detail state.
- Local git repository detection.
- Configurable branch naming and provider adapters.
- Explicit confirmation before Jira, git provider, or AI-generated write actions.

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

OAuth/security auth depends on:

- Browser or device authorization flow choice for Jira Cloud.
- OAuth scope mapping for read/write workflows.
- Secure credential storage through the OS keychain or equivalent.
- Token refresh and re-auth UX.

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

- Config file under `~/.config/jira/config.toml`.
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
- Edit priority, assignee, labels, components, fix versions, and other editable fields exposed by
  Jira edit metadata.
- Link issues.
- Add/remove issue from epic.
- Move issue between sprint/backlog if API support is clean.

### M7: Power User Workflows

Status: planned.

Goal: make repeated Jira work fast and connect Jira work to local development, pull requests, and
carefully confirmed AI assistance.

- Open branches for assigned or selected tickets.
- Detect ticket keys from the current git branch.
- Draft PR titles and bodies from Jira ticket context and local git state.
- Link branches/PRs back to Jira with explicit confirmation.
- Draft Jira comments, status updates, and implementation summaries with visible source context.
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
   - Render full-width list and focused detail surfaces.

2. Add comments read path.
   - Status: complete.
   - Comments load through the worker pool and render in focused ticket detail with the shared ADF
     formatter.

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
