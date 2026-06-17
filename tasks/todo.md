# Task Plan

## Saved Query Promotion

- [x] Add a config helper to append a named saved view safely.
- [x] Add query-modal UX to save a selected recent query as a named view.
- [x] Wire app startup so saved-query promotion writes through the existing config file.
- [x] Update docs/changelog/backlog for the user-visible saved-query workflow.
- [x] Run focused tests, full Go tests, `make check`, and `make install-user`.

### Saved Query Promotion Scope

Users can promote a selected recent direct or AI-generated JQL query into a durable named saved view.
Saving a recent query should not run Jira, change the active query, or switch the current view. The
new view should be persisted through the config file and become available in normal saved-view
rotation immediately.

### Saved Query Promotion Review

- Added `config.AddSavedView` to trim, append, and reject invalid or duplicate saved-view names.
- Added `s save view` from query modal `Recent` mode with a compact name prompt.
- Saved selected recent queries through the existing config file via a TUI writer injected by
  `cmd/jira-tui`.
- Kept saving separate from query execution: no Jira search command, active query change, or active
  view switch.
- Updated project state, changelog, roadmap, and backlog to reflect the shipped workflow.
- Verified with focused config/query/main tests, `go test ./... -count=1`, `make check`, and
  `make install-user`.

## Persisted Query History

- [x] Persist confirmed direct JQL queries between application executions.
- [x] Persist confirmed AI-generated JQL queries with their source prompt.
- [x] Add a Recent mode in the query modal so users can select previous queries.
- [x] Keep query history in the SQLite app cache, scoped by active cache namespace/site.
- [x] Keep named saved-view promotion as a future follow-up unless it stays small after history lands.
- [x] Update docs/changelog for the user-visible query history workflow.
- [x] Run focused query/cache tests, full Go tests, `make check`, and `make install-user`.

### Persisted Query History Scope

Confirmed query runs should be recorded automatically. AI previews must not be recorded until the
user explicitly runs the generated JQL. Recent entries should dedupe by normalized JQL, preserve the
AI prompt when available, track source (`direct` or `ai`), and be selectable in the query modal
without changing the active query until the user confirms.

### Persisted Query History Review

- Added SQLite-backed query history records for confirmed direct JQL and confirmed AI-generated JQL.
- Kept AI previews out of history until the user explicitly runs the generated query.
- Added `Recent` mode to the query modal with selection, load-for-review, and run-selected flows.
- Kept query history scoped by active cache namespace and outside transient cache cleanup.
- Left named saved-view promotion as a documented follow-up because it needs a separate naming and
  config/profile write UX.
- Verified with focused cache/query tests, `go test ./... -count=1`, `make check`, and
  `make install-user`.

## JQL Query UX

- [x] Add direct raw JQL entry in the TUI without restarting.
- [x] Add AI-assisted JQL generation through the existing provider-neutral AI path.
- [x] Show generated JQL as a preview before it can run.
- [x] Support user feedback/revision by resubmitting the AI prompt with current preview context.
- [x] Require explicit confirmation before direct or generated JQL changes the active query.
- [x] Update docs/changelog/backlog for the shipped query workflow.
- [x] Run focused query tests, full Go tests, `make check`, and `make install-user`.

### JQL Query UX Scope

Build a table-level query modal opened with `/`. The modal supports direct JQL editing for power
users and AI-assisted JQL generation for natural-language requests. AI-generated JQL must be
previewed and confirmed before running. Query execution must reuse the existing worker-backed issue
search path, cache behavior, and stale-result guards.

### JQL Query UX Review

- Added `/` in the issue table to open a query modal.
- Added direct raw JQL editing with explicit `ctrl+s` confirmation.
- Added AI-assisted JQL generation through the existing provider-neutral AI request adapter and a
  new `generate_jql` event operation.
- Kept generated JQL preview-only until the user confirms it, and treated edited AI prompts as
  revision requests instead of accidental runs.
- Kept Jira reads on the existing worker-backed issue search path.
- Updated backlog, project state, and changelog notes.
- Verified with focused query/help tests, `go test ./... -count=1`, `make check`, and
  `make install-user`.

## JQL Query UX Backlog Note

- [x] Record direct raw JQL entry as a required query workflow.
- [x] Record AI-assisted JQL generation as a separate query workflow with preview, revision
  feedback, and explicit confirmation before running.

### JQL Query UX Scope

Future query work should support both power-user direct JQL entry and AI-assisted generation in the
same query UX area. AI output should be inspectable and revisable before any generated JQL changes
the active view or triggers Jira reads.

### JQL Query UX Review

- Updated the backlog to replace the generic JQL input item with direct JQL plus AI-assisted JQL
  generation.
- Captured the required AI feedback loop: generated JQL preview, user revision feedback, and
  explicit confirmation before running.

## Local Active Status Filter

- [x] Confirm UX design for a local active-status issue-table filter.
- [x] Add focused tests for terminal status classification and active status preservation.
- [x] Add model-local table filter state without changing JQL, saved views, Jira requests, cache
  keys, cache records, or loaded issue data.
- [x] Render `All` by default and toggle `Active` with `f`.
- [x] Hide only terminal-looking status text in Active mode: done, closed, resolved, canceled, and
  cancelled.
- [x] Show active-filter count/empty-state copy in the issue table.
- [x] Route table navigation, paging, first/last jumps, and selection repair through visible loaded
  issues.
- [x] Update project docs/changelog for the user-visible table filter.
- [x] Run focused issue-list/status-filter tests, full Go tests, `make check`, and
  `make install-user`.

### Local Active Status Filter Scope

Add a UX-only issue-table filter that toggles between all loaded issues and active loaded issues.
The active filter hides tickets whose status text looks terminal while preserving the active saved
view, JQL, Jira reads, caches, issue ordering, and `m.issues`.

### Local Active Status Filter Review

- Added `f` as a table-mode toggle between All and Active loaded issues.
- Kept JQL, saved views, Jira requests, cache keys, cache records, and `m.issues` unchanged.
- Hid only locally terminal status text: done, closed, resolved, canceled, and cancelled.
- Routed rendering, movement, paging, first/last jumps, selection repair, and page indicators
  through visible loaded issues while the filter is active.
- Reset the local filter to All when switching saved views so every loaded view starts unfiltered.
- Added filtered-empty copy that tells users how to return to all loaded issues.
- Verified with focused issue-list/status-filter tests, `go test ./... -count=1`, `make check`, and
  `make install-user`.

## Issue List Subtree Collapse

- [x] Confirm UX design/spec for issue-list subtree collapse.
- [x] Audit issue-table key bindings and choose one low-conflict collapse toggle.
- [x] Add focused tests for default expanded rendering and subtree collapse projection.
- [x] Add focused tests for subtree re-expand, preserved deeper collapse state, and explicit child
  expansion compatibility.
- [x] Add model-local collapse state keyed by issue key without changing Jira reads, caches, saved
  views, issue ordering, or `m.issues`.
- [x] Derive visible issue rows from the existing tree plus collapse state.
- [x] Render a compact hidden-descendant count on collapsed nodes.
- [x] Route issue-list navigation and selection visibility through visible rows.
- [x] Update project docs/changelog for the user-visible issue-list behavior.
- [x] Run focused issue-list/navigation tests, broader Go verification, `make check`, and
  `make install-user`.

### Issue List Subtree Collapse Scope

Add purely local issue-table collapse/expand behavior so a selected node can hide or reveal its
loaded descendant subtree. The default view remains expanded, explicit `x`/`X` Jira child loading
keeps using the existing worker path, and the loaded ticket data stays unchanged. Expanding a node
reveals that node while preserving any deeper collapsed branches.

### Issue List Subtree Collapse Review

- Added `z` as a local issue-table subtree collapse/expand toggle.
- Kept Jira reads, explicit `x`/`X` child loading, saved views, caches, issue ordering, and
  `m.issues` unchanged.
- Added visible-row projection so rendering, navigation, paging, and selection visibility honor
  collapsed branches.
- Preserved deeper collapsed branches when expanding an ancestor.
- Rendered compact hidden-descendant counts on collapsed rows.
- Updated project-state and changelog docs for the user-visible behavior.
- Verified with focused issue-list/navigation tests, `go test ./... -count=1`, `make check`, and
  `make install-user`.

### Task 1: Collapse State And Rendered Projection

- [x] Add the Task 1 default-expanded and collapsed-render regression tests.
- [x] Capture focused RED output for the Task 1 collapse render tests.
- [x] Add model-local collapse state and rendered projection helpers without changing issue data.
- [x] Run the Task 1 focused GREEN issue-list render tests.
- [x] Run the broader Task 1 hierarchy regression command.
- [x] Record Task 1 review notes and verification results.

### Task 1 Review Fixes

- [x] Add failing focused tests for narrow collapsed-row hidden counts and direct visible-index projection.
- [x] Capture RED output for the Task 1 review-fix tests.
- [x] Fix collapsed-row rendering so the hidden-descendant cue survives summary truncation.
- [x] Add direct `visibleIssueIndexes` assertions for collapsed branches, including missing-parent roots.
- [x] Re-run focused GREEN tests and the broader Task 1 hierarchy command.
- [x] Append review-fix evidence to the Task 1 report and record the verification package.

### Task 2: Toggle Command And Key Help

- [x] Add the Task 2 collapse-toggle and leaf-notice failing tests.
- [x] Record that the stalled Task 2 worker did not preserve RED output.
- [x] Add the table `z` collapse toggle handler and key help binding.
- [x] Re-run the focused GREEN toggle/help tests.
- [x] Run the broader Task 2 verification command from the brief.
- [x] Record Task 2 review notes and verification results.

### Task 2 Review

- Added focused table-mode regression coverage for collapsing a selected parent, rejecting collapse on
  leaf rows with a notice, and exposing the new `z` binding in the table help overlay.
- Added a model-local `toggleSelectedIssueCollapse` helper that only mutates presentation state and
  reuses the existing rendered-tree descendant counting.
- Wired `z` into the existing table key switch and footer/full-help key bindings without touching
  Jira reads, worker expansion flows, saved views, cache records, or `m.issues`.
- Verified the focused Task 2 toggle/help command and the broader Task 2 command after recovering
  from the stalled worker; the original RED output was not preserved.

### Task 3: Visible-Row Navigation And Selection Repair

- [x] Add focused tests for skipping collapsed descendants during movement.
- [x] Add focused tests for repairing selection hidden by a collapsed ancestor.
- [x] Add focused tests for paging over visible rows.
- [x] Route movement, paging, selection visibility, and table first/last commands through visible
  issue rows.
- [x] Run the focused Task 3 navigation test command.

### Task 3 Review

- Added visible-row navigation coverage for collapsed branches.
- Routed table `j/k`, paging, `g/G`, and viewport offset calculations through the collapse-aware
  rendered projection.
- Preserved the raw loaded issue slice while changing only selection and offset behavior.
- Verified with the focused Task 3 navigation command.

### Task 4: Preserve Deeper Collapse State And Expansion Compatibility

- [x] Add focused tests for expanding a parent while preserving deeper collapsed branches.
- [x] Add focused tests proving explicit child merge keeps collapse state.
- [x] Add refresh/replace regression coverage for repairing selection hidden by a preserved
  collapsed ancestor.
- [x] Repair selection after refreshed issue replacement restores a now-hidden selected issue.
- [x] Run focused Task 4 tests and the broader issue-list regression command.

### Task 4 Review

- Preserved child collapse state when expanding an ancestor.
- Kept explicit `x`/`X` merge behavior compatible with existing collapse state.
- Repaired refresh replacement so preserved collapse state cannot leave selection on a hidden row.
- Verified with focused Task 4 tests and the broader issue-list command.

## Assignee And Mention Filter Textinput

- [x] Add focused tests proving Assignee and mention picker filters support cursor-aware editing.
- [x] Add shared Bubbles `textinput` setup for Jira user-search filters.
- [x] Route Assignee and mention picker filter updates through `textinput` while keeping existing
  query strings synchronized for worker requests/results.
- [x] Remove the old hand-rolled mention query editor helper.
- [x] Update TUI component audit/changelog notes.
- [x] Run focused TUI tests, full Go tests, `make check`, and install the updated binary.

### Assignee And Mention Filter Textinput Scope

Finish the next low-risk TUI consistency item from the component audit. Mention and Assignee result
lists already share the Bubbles choice-list adapter; this slice only moves their filter editing from
manual rune/backspace handling to Bubbles `textinput`. Keep Jira search behavior, caching,
selection, footer/help text, and picker rendering semantics unchanged.

### Assignee And Mention Filter Textinput Review

- Added focused regression coverage proving Assignee and mention filters support cursor-aware
  insertion through Bubbles `textinput`.
- Added a shared `newUserSearchInput` helper for Jira user-search picker filters.
- Kept `assigneeQuery` and `mentionQuery` synchronized as the existing source of truth for worker
  requests, cached user search, and stale-result guards.
- Removed the old hand-rolled `nextMentionQuery` rune/backspace editor helper.
- Preserved existing behavior where cached Assignee user searches do not submit worker commands.
- Updated the TUI component audit and changelog.
- Verified with focused picker tests, `go test ./... -count=1`, `make check`, and
  `make install-user`.

## Default Epic Saved View

- [x] Add focused config test coverage for the default Epics saved view.
- [x] Add an `Epics` saved view scoped to the default project.
- [x] Update docs/backlog/changelog notes.
- [x] Run focused config tests, full Go tests, `make check`, and install the updated binary.

### Default Epic Saved View Scope

Finish the existing saved-view backlog item without adding a new epic workspace. This slice only
adds the missing default Epics JQL view alongside the existing assigned, created/reported, project
open, current sprint, and watching views.

### Default Epic Saved View Review

- Added regression coverage that default project views include `Epics`.
- Added `project = <key> AND issuetype = Epic AND resolution = Unresolved ORDER BY updated DESC`
  as the generated default Epics view.
- Removed the completed saved-view backlog item and updated current-state/changelog docs.
- Verified with focused config tests, `go test ./... -count=1`, `make check`, and
  `make install-user`.

## Sanitized API Debug Diagnostics

- [x] Add focused Diagnostics tests for sanitized Jira API debug rows.
- [x] Track worker request start times without storing raw request bodies or credentials.
- [x] Render sanitized API rows with operation, endpoint family, request ID, key/project scope,
  result class, result counts, elapsed time, and short error summaries.
- [x] Keep the log bounded and in-memory by reusing the existing Diagnostics event buffer.
- [x] Reconcile completed cache and AI event backlog notes.
- [x] Run focused Diagnostics tests, full Go tests, `make check`, and install the updated binary.

### Sanitized API Debug Diagnostics Scope

Build the opt-in sanitized API debug log on the existing Diagnostics overlay. This slice should not
add persistence, raw request/response logging, token capture, a new debug UI, or extra Jira calls.
Diagnostics may record Jira operation families, request IDs, issue/project keys, result classes,
counts, elapsed time, and truncated error summaries only.

### Sanitized API Debug Diagnostics Review

- Added focused Diagnostics coverage for sanitized successful search rows and sanitized write-error
  rows.
- Added a bounded `api` Diagnostics event kind backed by the existing in-memory event buffer.
- Kept API-specific sanitization and formatting in `internal/tui/diagnostics_api.go` so
  `diagnostics.go` does not absorb another workflow.
- Tracked worker request start times in model-local state so result rows can include elapsed time.
- Recorded sanitized API row details from typed worker results: endpoint family, request ID, safe
  scope, result class, counts/empty state where available, elapsed time, and categorized error.
- Avoided raw JQL strings, raw request bodies, raw response bodies, tokens, and free-form error text
  in API diagnostic rows.
- Reconciled stale backlog notes for completed cache persistence/unification and provider-neutral
  AI event work.
- Verified with focused Diagnostics tests, `go test ./... -count=1`, `make check`, and
  `make install-user`.

## Cache Refresh Failure Diagnostics

- [x] Add focused tests proving failed active-view, issue-detail, and comments refreshes attach
  errors to retained cache records and render them in Diagnostics.
- [x] Store refresh errors on retained active-view cache records.
- [x] Store refresh errors on retained Jira cache records without changing the cached value or
  freshness boundary.
- [x] Render per-cache-family error counts in the Diagnostics cache summary.
- [x] Update cache performance docs/changelog/backlog.
- [x] Run focused diagnostics tests, full Go tests, `make check`, and install the updated binary.

### Cache Refresh Failure Diagnostics Scope

Improve observability only. When a refresh fails while a retained record still exists, keep the
stale value visible and attach the latest refresh error to that retained record for Diagnostics.
Do not change worker retry behavior, TTL values, cache persistence schema, or user-facing stale data
rendering in this slice.

### Cache Refresh Failure Diagnostics Review

- Added Diagnostics regression coverage for failed active-view, issue-detail, comments,
  transitions, edit metadata, create issue type, create field, and expanded-child refreshes.
- Stored the latest refresh error on retained cache records without changing the cached value,
  synced timestamp, freshness boundary, retry behavior, or persistence schema.
- Extended the Diagnostics cache-family summary from fresh/stale counts to fresh/stale/error counts.
- Updated cache performance docs, backlog, and changelog to remove the remaining refresh-failure
  diagnostics backlog item.
- Verified with focused Diagnostics tests, `go test ./... -count=1`, `make check`, and
  `make install-user`.

## Persistent Cache Cleanup

- [x] Add focused SQLite store tests for conservative age-based cache cleanup.
- [x] Add a cleanup method that deletes rows by `updated_at_unix_nano` cutoff across every cache
  table.
- [x] Run cleanup after opening the default cache store without blocking the Bubble Tea render loop.
- [x] Keep cleanup based on disk retention age, not record freshness, so stale-while-revalidate
  behavior remains intact.
- [x] Update cache performance docs/changelog/backlog.
- [x] Run focused cache tests, full Go tests, `make check`, and install the updated binary.

### Persistent Cache Cleanup Scope

Prevent the private SQLite cache from growing forever. Cleanup should be conservative and should
only remove records that have not been updated for the configured persistent retention window. Do
not delete records merely because `FreshTill` has passed; stale active views and detail data are
still useful while background refresh runs.

### Persistent Cache Cleanup Review

- Added `Store.DeleteRowsUpdatedBefore` to delete old rows from every persistent cache table using
  the existing `updated_at_unix_nano` indexes.
- Kept cleanup conservative: records are removed only when their disk update time is older than the
  seven-day retention window, not when their freshness TTL expires.
- Started cleanup as a short best-effort background task after opening the default cache store so it
  does not block Bubble Tea startup/rendering.
- Added store coverage that proves old rows across every cache table are deleted while recent stale
  rows remain available.
- Verified with focused cache cleanup tests, `go test ./... -count=1`, `make check`, and
  `make install-user`.

## Write-Side Cache Consistency

- [x] Add focused tests proving summary/priority/assignee/description/status writes patch retained
  detail and active-view cache records.
- [x] Patch retained issue-detail cache records when shared `updateIssue*` helpers mutate visible
  issue data.
- [x] Patch retained active-view cache records for the current view when visible issue rows change.
- [x] Invalidate retained transition options after a successful status transition.
- [x] Update cache performance docs/changelog/backlog.
- [x] Run focused TUI tests, full TUI tests, `make check`, and install the updated binary.

### Write-Side Cache Consistency Scope

Keep write behavior and Jira requests unchanged. After Jira confirms a write, the local read caches
must not later rehydrate stale values over the visible mutation. Patch only app-owned retained cache
records already in memory, persist the patched current active view/detail through existing store
methods when available, and invalidate transition options after a status change because allowed
transitions are status-dependent.

### Write-Side Cache Consistency Review

- Added regression coverage for summary, priority, assignee, status, and description mutations
  patching retained detail/current-view cache records.
- Added regression coverage and SQLite store support for deleting persisted transition records.
- Routed shared `updateIssue*` helpers through retained detail/current-view cache patch helpers so
  direct ticket-detail edits and AI-assisted summary/description applies share the same behavior.
- Invalidated retained and persisted transition options after status changes because transition
  availability depends on the current status.
- Kept Jira write behavior, TTL values, worker scheduling, and background refresh behavior
  unchanged.
- Verified with focused write-cache tests, `go test ./internal/tui ./internal/cache -count=1`,
  `make check`, and `make install-user`.

## Cache Family Diagnostics Summary

- [x] Add a focused Diagnostics test for per-cache-family record counts and freshness totals.
- [x] Render a compact cache summary derived from retained TTL cache records.
- [x] Include active view, issue detail, comments, transitions, edit metadata, create metadata, and
  expanded children without changing cache policy.
- [x] Update docs/changelog for the Diagnostics visibility improvement.
- [x] Run focused diagnostics tests, full TUI tests, `make check`, and install the updated binary.

### Cache Family Diagnostics Summary Scope

Improve cache observability before changing cache invalidation or cleanup behavior. Diagnostics
should show how many retained records each Jira cache family currently holds and whether those
records are fresh or stale. This slice must not change worker scheduling, cache TTL values, Jira
request behavior, or write invalidation policies.

### Cache Family Diagnostics Summary Review

- Added a Diagnostics cache summary that reports retained cache records by family as fresh/stale
  counts.
- Covered active view, issue detail, comments, transitions, edit metadata, create issue types,
  create fields, and expanded children.
- Kept the change read-only: no TTLs, worker scheduling, Jira requests, persistence writes, or
  invalidation policies changed.
- Updated cache performance docs, backlog, and changelog to record that family-level diagnostics
  landed while per-record refresh failures, cleanup, and remaining write invalidation policies stay
  queued.
- Verified with `go test ./internal/tui -run 'TestDiagnostics' -count=1`,
  `go test ./internal/tui -count=1`, `make check`, and `make install-user`.

## Ticket Detail Visual Hierarchy Polish

- [x] Add focused tests for summary-as-title header behavior and plain active tab markers.
- [x] Promote the ticket summary to standalone title text without a redundant `Summary:` label.
- [x] Render header metadata as compact label/value segments without heavy pipe dividers.
- [x] Replace filled active tab styling with a plain selected marker while preserving badges.
- [x] Update changelog/project notes for the user-visible detail polish.
- [x] Run focused detail tests, full TUI tests, `make check`, and install the updated binary.

### Ticket Detail Visual Hierarchy Polish Scope

Make the ticket detail panel easier to scan without changing navigation, key handling, worker-backed
Jira reads, or section activation behavior. This pass focuses on the header and section tabs first:
summary should read as the issue title, metadata should stay compact, and tabs should use the same
plain selected-marker pattern as other compact TUI controls.

### Ticket Detail Visual Hierarchy Polish Review

- Promoted the ticket summary to a standalone title line and removed the redundant `Summary:` label
  from the normal detail header.
- Tightened header spacing by placing compact metadata directly under the summary and removing the
  divider row before tabs.
- Replaced filled active detail tab styling with a plain `>` marker while preserving full/compact
  labels and section badges.
- Updated focused and existing regression tests for the new header/tab contract while preserving
  detail focus, overlay, and action behavior.
- Updated project docs and changelog to reflect the new ticket-detail header grammar.
- Verified with focused detail tests, `go test ./internal/tui -count=1`, `make check`, and
  `make install-user`.

## Header Background Activity Indicator

- [x] Add focused header tests for active AI, active Jira work, recent event bursts, and idle state.
- [x] Implement a compact background activity label derived from existing worker stats,
  diagnostics events, refresh state, and AI loading flags.
- [x] Render the label in the header only when background activity is active or recent.
- [x] Update docs/changelog if the user-visible header changes.
- [x] Run focused TUI tests, full TUI tests, `make check`, and install the updated binary.

### Header Background Activity Indicator Scope

Add a quiet always-available activity hint to the header without adding timers, animation, new
event infrastructure, or footer noise. Diagnostics remains the detailed background activity view.

### Header Background Activity Indicator Review

- Added compact header labels for active AI work, active Jira sync/work, recent ticket updates,
  recent generic events, and recent background errors.
- Kept idle headers quiet; the label only appears for active or recent activity and drops first on
  narrow widths.
- Derived the label from existing model flags, worker stats, and diagnostics events without adding
  new infrastructure.
- Removed `bg` implementation language from user-facing labels and avoided duplicate
  `refreshing`/`syncing` wording.
- Added focused header tests for active AI, active Jira work, recent ticket updates, generic events,
  error priority, width priority, and idle state.
- Updated the changelog and installed the updated user binary.
- Saved the UX rule as a memory note: compact feedback for background work matters.
- Verified with focused header tests, `go test ./internal/tui -count=1`, `make check`, and
  `make install-user`.

## Missing Parent Placeholder Clarity

- [x] Add a focused issue-list test for missing parent placeholder wording.
- [x] Change missing parent rows so they are explicitly labeled as parent context outside the
  current view.
- [x] Update the lesson from this correction.
- [x] Run focused TUI tests and install the updated binary.

### Missing Parent Placeholder Clarity Scope

Keep the hierarchy behavior unchanged. Only adjust the placeholder row text so users can distinguish
an unloaded parent context row from a real greyed-out ticket.

### Missing Parent Placeholder Clarity Review

- Missing parent rows now render as `Parent outside view: KEY Summary` instead of a muted
  key/summary-only row.
- Added a regression test that ensures the placeholder label is explicit and does not include normal
  loaded issue metadata.
- Recorded the correction in `tasks/lessons.md`.
- Verified with `go test ./internal/tui -run TestIssueListLabelsMissingParentPlaceholder -count=1`,
  `go test ./internal/tui -count=1`, and `make install-user`.

## Large View Prefetch Throttling

- [x] Add focused TUI tests for search-result prefetch behavior.
- [x] Keep list refreshes from loading comments automatically.
- [x] Bound missing selected-detail prefetch on large views while keeping explicit detail opens
  unchanged.
- [x] Use existing worker priorities for prefetch requests.
- [x] Update docs/backlog/changelog notes if the behavior changes user-visible diagnostics.
- [x] Run focused TUI tests, full Go tests, `make check`, and install the updated binary.

### Large View Prefetch Throttling Scope

Reduce Jira API work triggered by active-view refreshes without changing foreground detail behavior.
Search results may warm the selected issue detail only as low-priority bounded prefetch; comments
should load when the user opens detail or explicitly navigates detail content, not as part of every
list refresh.

### Large View Prefetch Throttling Review

- Added a table/list prefetch path separate from the foreground detail loader.
- Active-view search results and table navigation now skip missing selected-detail prefetch when the
  visible issue list is larger than the prefetch limit.
- Comments no longer prefetch from list refreshes or table navigation; explicit detail opens still
  load detail and comments through foreground worker requests.
- Updated cache performance docs, backlog, and changelog to record the new behavior.
- Fixed the ticket-event stream test to assert by event type/key instead of relying on asynchronous
  event delivery order.
- Verified with focused TUI regression tests, `go test ./... -count=1`, `make check`, and
  `make install-user`.

## Jira Event Stream And Active View Refresh Plan

- [x] Research maintained Go event/pubsub libraries.
- [x] Choose a recommended event-stream foundation.
- [x] Define initial Jira ticket event types and payload boundaries.
- [x] Capture internal command/scheduler events as a first-class future use case.
- [x] Capture provider-agnostic AI command events as a future stream consumer.
- [x] Define active-view stale-while-revalidate cache behavior.
- [x] Define implementation slices for event stream, active-view refresh, ticket diffing, and
  future notifications.
- [x] Implement Watermill-backed event stream adapter.
- [x] Implement active-view stale-while-revalidate startup behavior.
- [x] Publish ticket new/updated events from refreshed active views.
- [x] Add diagnostics as the first event consumer.
- [x] Move Claude requests behind provider-agnostic event-stream AI command handling in a later
  slice.
- [ ] Add notification consumers in a later slice.

### Jira Event Stream And Active View Refresh Plan Review

- Recommended Watermill with GoChannel for the first in-process background stream.
- Deferred macOS notification and in-app dialog delivery behind future consumers.
- Captured the full design in `docs/jira-event-stream-design.md`.
- Added `internal/events` as the app-owned adapter around Watermill GoChannel.
- Hydrated displayable stale active-view rows immediately and refreshed them in the background.
- Published active-view `jira.ticket.new` and `jira.ticket.updated` events from refreshed rows.
- Routed app events into Diagnostics as the first non-blocking consumer.

## Provider-Agnostic AI Event Command Foundation

- [x] Add provider-neutral AI task payload types for operation, provider, key, prompt, and result
  metadata.
- [x] Route existing Claude subprocess requests through provider-neutral request metadata while
  keeping the current Claude runner as the only concrete provider.
- [x] Publish `ai.task.requested`, `ai.task.progress`, `ai.task.completed`, and `ai.task.failed`
  events around AI work.
- [x] Keep existing Claude/TUI result messages and modals unchanged in this slice.
- [x] Add focused stream tests for ticket plan success, progress, and failure events.
- [x] Update docs/changelog/backlog notes and run focused tests plus `make check`.

### Provider-Agnostic AI Event Command Foundation Scope

Introduce the provider boundary without changing user-facing AI behavior. The current Claude
subprocess runner remains the concrete implementation. This slice should only add typed metadata
and event publication so future Codex or `auto` routing can plug in without another direct TUI to
Claude command path.

### Provider-Agnostic AI Event Command Foundation Review

- Added `events.AITaskPayload`, provider constants, and operation constants for ticket plan, ticket
  assist, inline assist, create draft, refine draft, code review, and implementation plan tasks.
- Added a focused TUI AI request adapter that publishes `ai.task.requested`, `ai.task.progress`,
  `ai.task.completed`, and `ai.task.failed` around the current Claude runner.
- Routed ticket plan, ticket assist, inline assist, refine assist, and create-ticket draft calls
  through the adapter while preserving existing result messages and modals.
- Kept prompt/result text out of event payloads; progress events carry kind and byte count only.
- Added stream tests proving ticket plan success, progress, and failure events carry provider-neutral
  metadata.

## Persistent Expanded Children Cache

- [x] Add retained hot-cache records for explicit expanded children by parent key and expand mode.
- [x] Add SQLite records for expanded children keyed by Jira namespace, parent key, and expand mode.
- [x] Use fresh cached expanded children to merge hierarchy rows without submitting Jira work.
- [x] Treat stale or missing expanded children as a normal Jira refresh.
- [x] Persist successful expanded-child Jira reads back to SQLite.
- [x] Add focused store and TUI persistence tests.
- [x] Update cache design/backlog/changelog notes.
- [x] Run final verification with `go test ./internal/cache -count=1`,
  `go test ./internal/tui -count=1`, `go test ./... -count=1`, and `make check`.

### Persistent Expanded Children Cache Scope

Extend the existing hot-cache plus SQLite pattern to explicit hierarchy expansion only: `open
children` and `all children` requests for a selected parent. Do not change broad issue search child
lookup or prefetch behavior in this slice. Fresh cached child rows can merge immediately; stale
records should submit Jira work rather than silently changing the visible hierarchy.

### Persistent Expanded Children Cache Review

- Added retained hot-cache records for explicit expanded children keyed by parent key and expand
  mode.
- Added SQLite tables and store methods for expanded child issue rows.
- Hydrated fresh persisted expanded children before submitting explicit hierarchy expansion work.
- Persisted successful expanded-child worker reads back to SQLite.
- Verified focused red/green coverage for store round-trips, fresh expanded-child hydration, and
  expanded-child result persistence.
- Verified with `go test ./internal/cache -count=1`, `go test ./internal/tui -count=1`,
  `go test ./... -count=1`, and `make check`.

## Persistent Create Metadata Cache

- [x] Add retained hot-cache records for create issue types and create fields.
- [x] Add SQLite records for create issue types keyed by Jira namespace and project key.
- [x] Add SQLite records for create fields keyed by Jira namespace, project key, and issue type ID.
- [x] Hydrate fresh persisted issue types before loading the create issue type picker.
- [x] Hydrate fresh persisted create fields before rendering the create form.
- [x] Persist successful create metadata Jira reads back to SQLite.
- [x] Add focused store and TUI persistence tests.
- [x] Update cache design/backlog/changelog notes.
- [x] Run final verification with `go test ./internal/cache -count=1`,
  `go test ./internal/tui -count=1`, `go test ./... -count=1`, and `make check`.

### Persistent Create Metadata Cache Scope

Extend the existing hot-cache plus SQLite pattern to create issue type metadata and create field
metadata only. Keep expanded children, cache cleanup, and detailed cache diagnostics for later
slices. Treat stale create metadata as needing refresh before rendering dependent create screens,
because Jira required fields and allowed values can change by project, issue type, or permission.

### Persistent Create Metadata Cache Review

- Added retained hot-cache records for create issue types and create fields.
- Added SQLite tables and store methods for create issue type and create field metadata.
- Hydrated fresh persisted issue types before rendering the create type picker.
- Hydrated fresh persisted fields before rendering the create form.
- Persisted successful create metadata worker reads back to SQLite.
- Verified focused red/green coverage for store round-trips, fresh create metadata hydration, and
  create metadata result persistence.
- Verified with `go test ./internal/cache -count=1`, `go test ./internal/tui -count=1`,
  `go test ./... -count=1`, and `make check`.

## Persistent Transition And Edit Metadata Cache

- [x] Add retained hot-cache records for transitions and edit metadata.
- [x] Add SQLite records for transitions and edit metadata keyed by Jira namespace and issue key.
- [x] Hydrate fresh persisted transitions before loading the status picker.
- [x] Hydrate fresh persisted edit metadata before summary/priority editors.
- [x] Persist successful transition and edit metadata Jira reads back to SQLite.
- [x] Add focused store and TUI persistence tests.
- [x] Update cache design/backlog/changelog notes.
- [x] Run final verification with `go test ./internal/cache -count=1`,
  `go test ./internal/tui -count=1`, `go test ./... -count=1`, and `make check`.

### Persistent Transition And Edit Metadata Cache Scope

Extend the existing hot-cache plus SQLite pattern to transition options and edit metadata only.
Keep create metadata, expanded children, cache cleanup, and detailed cache diagnostics for later
slices. Treat stale edit metadata as needing refresh before rendering editors, because Jira field
constraints can change by status or permissions.

### Persistent Transition And Edit Metadata Cache Review

- Added retained hot-cache records for status transitions and edit metadata.
- Added SQLite tables and store methods for transitions and edit metadata.
- Hydrated fresh persisted transitions before rendering the status picker.
- Hydrated fresh persisted edit metadata before summary and priority editors.
- Persisted successful transition and edit metadata worker reads back to SQLite.
- Preserved existing plain-map test/setup behavior when no hot cache record exists.
- Verified with `go test ./internal/cache -count=1`, `go test ./internal/tui -count=1`,
  `go test ./... -count=1`, and `make check`.

## Persistent Detail And Comment Cache

- [x] Add SQLite records for issue detail and comments using `modernc.org/sqlite`.
- [x] Key records by Jira namespace and issue key, with comment records also storing max results.
- [x] Store payload JSON plus `SyncedAt` and `FreshTill` timestamps.
- [x] Hydrate detail/comments from the persistent store when opening a selected issue.
- [x] Persist successful detail/comment Jira reads back to SQLite.
- [x] Invalidate persistent comments after comment writes.
- [x] Keep `ttlcache` as the hot cache layer and existing maps as render sources.
- [x] Add focused store and TUI persistence tests.
- [x] Update cache design/backlog/changelog notes.
- [x] Run final verification with `go test ./internal/cache -count=1`,
  `go test ./internal/tui -count=1`, `go test ./... -count=1`, and `make check`.

### Persistent Detail And Comment Cache Scope

Extend the SQLite persistent cache to issue detail and comments only. Keep transitions, edit/create
metadata, expanded children, and cleanup policy for later slices. Continue using the existing hot
`ttlcache` records and render maps; SQLite is only the durable read model behind them.

### Persistent Detail And Comment Cache Review

- Added SQLite tables and store methods for issue detail and comments.
- Keyed detail by namespace and issue key; keyed comments by namespace, issue key, and max result
  count.
- Hydrated persistent detail/comments into the existing hot cache records before opening detail.
- Persisted successful detail/comment worker reads back to SQLite.
- Invalidated persistent comments after user comment creation and shared retained invalidation path.
- Kept `ttlcache` and current render maps as the hot/read path.
- Verified with `go test ./internal/cache -count=1`, `go test ./internal/tui -count=1`,
  `go test ./... -count=1`, and `make check`.

## Persistent Active View Cache

- [x] Add a SQLite-backed cache store using `modernc.org/sqlite`.
- [x] Store active-view issue rows as app-local JSON payloads keyed by namespace and normalized JQL.
- [x] Store sync and freshness timestamps so cached rows can hydrate fresh or stale views.
- [x] Keep persistence private to `os.UserCacheDir()` for the real app and injectable for tests.
- [x] Wire active-view cache writes to the store after successful Jira search results.
- [x] Hydrate active views from the store before submitting Jira refreshes.
- [x] Keep in-memory `ttlcache` as the hot cache layer.
- [x] Add focused store and TUI hydration tests.
- [x] Update cache design/backlog/changelog notes.
- [x] Run final verification with `go test ./internal/cache -count=1`,
  `go test ./internal/tui -count=1`, `go test ./... -count=1`, and `make check`.

### Persistent Active View Cache Scope

Implement the first persistent cache slice only for active Jira views. Use maintained
`modernc.org/sqlite` through `database/sql`; do not build a custom storage engine. Keep detail,
comments, transitions, metadata, and expanded children persistence for later slices. The store must
be disposable, credential-free, namespace-aware, and safe to ignore if unavailable.

### Persistent Active View Cache Review

- Added `internal/cache.Store` backed by SQLite through `modernc.org/sqlite`.
- Stored active-view issue rows as JSON payloads keyed by Jira namespace and normalized JQL.
- Stored `SyncedAt` and `FreshTill` timestamps for cross-session freshness decisions.
- Opened the real app cache under `os.UserCacheDir()/jira-tui/cache.sqlite`; cache open failures do
  not block app startup.
- Hydrated active views from the persistent store into the existing hot `ttlcache` layer.
- Persisted successful active-view Jira search results back to SQLite.
- Left detail/comment persistence, metadata persistence, and cache cleanup for later slices.
- Verified with `go test ./internal/cache -count=1`, `go test ./internal/tui -count=1`,
  `go test ./... -count=1`, and `make check`.

## Comments Cache Record Unification

- [x] Add retained comment cache records using the shared Jira cache record helper.
- [x] Preserve existing `comments` map rendering while moving freshness checks to cache records.
- [x] Keep fresh cached comments from submitting Jira work when opening detail.
- [x] Keep stale comments visible while a refresh is submitted.
- [x] Invalidate retained comments after adding a comment so the follow-up refresh is real.
- [x] Add focused fresh/stale/invalidation tests.
- [x] Update cache design/backlog/changelog notes.
- [x] Run final verification with `go test ./internal/tui -count=1`, `go test ./... -count=1`,
  and `make check`.

### Comments Cache Record Unification Scope

Migrate comments to the same retained cache-record semantics now used by issue detail. Do not
migrate transitions, edit metadata, create metadata, or expanded children in this commit. Keep the
current `comments` map as the render source for this slice so UI rendering and Claude prompt
assembly continue to use the same data path.

### Comments Cache Record Unification Review

- Added retained `ttlcache` records for issue comments using the shared `jiraCacheRecord[T]` helper.
- Preserved the existing `comments` map as the render/Claude prompt source for this slice.
- Made fresh cached comments skip Jira work when opening detail.
- Made stale cached comments remain visible while submitting a refresh.
- Invalidated both the visible comments map and retained comment record after comment creation and
  Claude-assisted comment posting.
- Added focused tests for fresh comments, stale comments, and add-comment invalidation.
- Verified with `go test ./internal/tui -count=1`, `go test ./... -count=1`, and `make check`.

## Detail Cache Record Unification

- [x] Add a generic Jira cache record helper for value, sync time, and freshness window.
- [x] Replace the marker-only issue detail freshness cache with a retained detail cache record.
- [x] Keep the existing `details` map as the current render/read path while detail freshness moves
  to the shared record semantics.
- [x] Preserve fresh-hit, stale-while-refresh, and stale-visible behavior for issue detail.
- [x] Add focused cache-record and detail-cache tests.
- [x] Update cache design/backlog/changelog notes.
- [x] Run final verification with `go test ./internal/tui -count=1`, `go test ./... -count=1`,
  and `make check`.

### Detail Cache Record Unification Scope

Implement the first cache-unification slice behind ticket detail only. Use the existing maintained
`ttlcache` dependency for retained records; do not migrate comments, transitions, edit metadata, or
create metadata in this commit. The goal is to replace a marker-only freshness cache with a reusable
record shape that later read caches can share.

### Detail Cache Record Unification Review

- Added generic `jiraCacheRecord[T]` freshness semantics for retained Jira read records.
- Replaced the marker-only issue detail freshness cache with a `ttlcache`-retained detail record.
- Kept `details` as the current render path while storing the same detail value in the retained
  record for freshness checks.
- Preserved fresh detail hits, stale-while-refresh behavior, and stale detail visibility.
- Left comments, transitions, edit metadata, create metadata, and expanded children for follow-up
  cache-unification slices.
- Verified with `go test ./internal/tui -count=1`, `go test ./... -count=1`, and `make check`.

## Worker Diagnostics Queue Summary

- [x] Expose worker scheduler stats from the existing worker pool.
- [x] Show a compact queue summary in the Diagnostics overlay.
- [x] Keep the Diagnostics overlay useful even when no activity events have been recorded yet.
- [x] Add focused worker and TUI rendering tests.
- [x] Update cache design/backlog/changelog notes.
- [x] Run final verification with `go test ./internal/worker -count=1`,
  `go test ./internal/tui -count=1`, `go test ./... -count=1`, and `make check`.

### Worker Diagnostics Queue Summary Scope

Continue the cache/performance work by making the scheduler observable without adding a new
queueing dependency. Keep `ants` as the maintained execution primitive and expose only a small,
read-only stats snapshot from the app-specific admission layer.

### Worker Diagnostics Queue Summary Review

- Added `worker.Stats` and `Pool.Stats()` as a lock-protected read-only snapshot of the existing
  scheduler state.
- Added a compact Diagnostics queue line for running, pending, coalesced, and capacity counts.
- Kept the Diagnostics overlay informative before any activity events are recorded by showing the
  queue summary above the empty-state message.
- Added focused worker and TUI tests for scheduler stats and queue summary rendering.
- Verified with `go test ./internal/worker -count=1`, `go test ./internal/tui -count=1`,
  `go test ./... -count=1`, and `make check`.

## Worker Priority And Coalescing

- [x] Add worker request priority and coalesce-key metadata.
- [x] Keep `ants` as the maintained worker execution primitive.
- [x] Add bounded scheduler admission around the worker pool.
- [x] Coalesce duplicate read requests and fan out cloned results to duplicate request IDs.
- [x] Drop queued lower-priority requests to admit foreground work when capacity is full.
- [x] Mark active-view searches with explicit foreground/refresh/background priorities.
- [x] Add regression tests for duplicate read coalescing and foreground admission over background.
- [x] Run final verification with `go test ./internal/worker -count=1`,
  `go test ./internal/tui -count=1`, `go test ./... -count=1`, and `make check`.

### Worker Priority And Coalescing Scope

Implement slice 2 from the cache design without replacing the maintained `ants` execution pool.
The local scheduler layer owns only app-specific policy: request priority, duplicate read
coalescing, and dropping queued lower-priority work before rejecting foreground work.

### Worker Priority And Coalescing Review

- Added `worker.Priority` levels for writes, foreground reads, explicit refresh, prefetch, and
  background work.
- Added request `CoalesceKey` support so duplicate reads share one Jira execution and each request
  ID still receives a result.
- Added scheduler admission inside `internal/worker.Pool` while preserving `ants` for execution.
- Updated active-view search submissions so startup/view-switch work is foreground, manual refresh
  is explicit refresh, and timer refresh is background.
- Verified with `go test ./internal/worker -count=1`, `go test ./internal/tui -count=1`,
  `go test ./... -count=1`, and `make check`.

## Active View Cache Implementation

- [x] Add `ttlcache`-backed active-view cache records to the TUI model.
- [x] Render fresh cached views without submitting a Jira search.
- [x] Render stale cached views immediately and refresh in the background.
- [x] Preserve stale cached rows when refresh fails.
- [x] Show active-view freshness in the header.
- [x] Keep manual refresh as a real Jira refresh even when a fresh cached view exists.
- [x] Add focused tests for fresh hit, stale hit, failed-refresh preservation, selection behavior,
  manual refresh bypass, and visible freshness labels.
- [x] Run final verification with `go test ./internal/tui -count=1`, `go test ./... -count=1`, and
  `make check`.

### Active View Cache Implementation Scope

Implement slice 1 from the cache design: an in-memory active-view read model only. Do not introduce
disk persistence or rewrite the worker pool yet. Use the existing maintained `ttlcache` dependency
for cache retention, with app policy deciding whether a retained record is fresh or stale.

### Active View Cache Implementation Review

- Added `internal/tui/view_cache.go` as the active-view cache adapter.
- Cached successful active JQL search results by normalized JQL.
- Added stale-while-refresh behavior for cached active views.
- Preserved stale rows and freshness labels when refresh fails.
- Added header labels for `synced`, `stale`, and `refresh failed`.
- Kept explicit user refresh on the foreground Jira path instead of letting a fresh cache hit turn it
  into a no-op.
- Verified with `go test ./internal/tui -count=1`, `go test ./... -count=1`, and `make check`.

## Jira Cache And Background Refresh Design

- [x] Inspect current Jira read/write flow, worker pool, refresh behavior, and existing caches.
- [x] Document freshness, stale-while-refresh, write invalidation, priority, diagnostics, and
  persistence rules.
- [x] Define the first implementation slice around an in-memory active-view cache.
- [x] Verify docs/code health with `make check`.

### Jira Cache And Background Refresh Design Scope

Plan the performance work before implementation so large Jira views become responsive without
silently stale data. Keep the current worker-backed Jira IO model, prioritize foreground user
actions over background refresh, and avoid persistent storage until the in-memory read model proves
useful.

### Jira Cache And Background Refresh Design Review

- Added `docs/jira-cache-performance-design.md`.
- Identified that active JQL search results are not currently cached as a reusable read model; large
  views still wait on Jira search plus enrichment before replacing the issue list.
- Kept existing `ants` worker pool and `ttlcache` usage as near-term primitives.
- Defined priority classes for writes, foreground reads, explicit refresh, foreground prefetch, and
  background sync.
- Chose the next implementation slice: in-memory active-view cache with stale-while-refresh,
  freshness labels, failed-refresh stale preservation, and selection-preserving merge tests.
- Verified with `make check`.

## Ticket Detail Rich Rendering Polish

- [x] Inspect current rich ticket detail rendering gaps after the package split.
- [x] Add focused coverage for Jira ADF panel/status markers.
- [x] Style panel/status markers in the TUI rich text renderer without changing `internal/adf.Render`.
- [x] Add focused coverage for blockquotes, mentions, and links in rich ticket detail text.
- [x] Style blockquotes, mentions, URLs, and emails in the same TUI rich text presentation path.
- [x] Run focused rich-rendering tests.
- [x] Run final verification with `go test ./internal/tui -count=1`, `go test ./... -count=1`, and `make check`.

### Ticket Detail Rich Rendering Polish Scope

Continue the Read/View backlog by improving how Jira ADF-derived rich text appears in ticket detail
without changing Jira API conversion or broad layout behavior. Keep the slice small: preserve the
plain-text `internal/adf.Render` boundary and make the TUI presentation layer style markers it
already receives.

### Ticket Detail Rich Rendering Polish Review

- Added regression coverage for ADF panel/status marker rendering.
- Converted `[panel]` prefixes into the existing compact notice-block style in rich ticket text.
- Converted narrow all-caps Jira status tokens such as `[BLOCKED]` into styled status text so raw
  bracket markers do not leak into ticket detail.
- Converted `> ` blockquote lines into compact left-rule quote blocks.
- Styled inline URLs, email addresses, and `@mentions` in the shared rich text renderer while
  preserving existing inline code behavior.
- Verified with focused rich-rendering tests, `go test ./internal/tui -count=1`,
  `go test ./... -count=1`, and `make check`.

## Same-Package TUI File Split

- [x] Record execution scope and split order.
- [x] Move worker command constructors into a same-package file and verify `internal/tui`.
- [x] Move diagnostics types and rendering helpers into a same-package file and verify `internal/tui`.
- [x] Move issue-list types and rendering helpers into a same-package file if the range is clean.
- [x] Move remaining clean workflow clusters into same-package files.
- [x] Split workflow-specific tests so coverage follows the new file boundaries.
- [x] Audit docs, comments, imports, and misplaced tests before calling the split done.
- [x] Update audit/backlog/changelog/task review notes.
- [x] Run final verification with `go test ./internal/tui -count=1`, `go test ./... -count=1`, and `make check`.

### Same-Package TUI File Split Scope

Execute the package-boundary audit recommendation with mechanical same-package file moves only.
Keep `package tui`, the Bubble Tea `Model` boundary, all worker-backed Jira IO, and current behavior
unchanged. Stop and re-plan if a candidate split needs non-mechanical rewrites or creates import
cycles.

### Same-Package TUI File Split Review

- Added same-package workflow files for worker command submission, diagnostics, issue-list
  rendering, comments, create issue/create AI, Claude assist, detail views/actions, rich text,
  browser chrome, worker results, navigation, formatting, summary editing, and external actions.
- Kept `model.go` focused on constants, core types, `Model`, options, `Init`, `Update`, and top-level
  `View`/`render` dispatch.
- Split the broad TUI test file into workflow test files for create issue, Claude assist, comments,
  diagnostics, issue list, rich text, and detail behavior while keeping shared fakes/helpers in
  `model_test.go`.
- Moved a detail overlay regression out of `issue_list_test.go` into `detail_test.go` during the
  final boundary audit.
- Reduced `internal/tui/model.go` from 10,540 lines to 1,105 lines and `internal/tui/model_test.go`
  from 8,057 lines to 658 lines without changing `package tui` or the Bubble Tea `Model` boundary.
- Saved the proactive package-boundary lesson to project memory and `tasks/lessons.md`: in Go,
  same-package file splits should happen before a single file absorbs unrelated workflows.
- Reviewed split-file comments/imports/docs for stale intermediate-state wording before final
  verification.
- Verified with `go test ./internal/tui -count=1`, `go test ./... -count=1`, and `make check`.

## Package And File Boundary Audit

- [x] Record audit criteria and scope.
- [x] Collect codebase shape metrics: large files, package sizes, test concentration, and dependency direction.
- [x] Inspect high-risk files/packages for mixed responsibilities and repeated local helpers.
- [x] Write split/no-split recommendations with staged next steps.
- [x] Update backlog/project docs as needed.
- [x] Verify the audit artifacts and run an appropriate docs/code health check.

### Package And File Boundary Audit Scope

Evaluate whether the codebase is drifting toward monolithic files or unclear internal libraries
after the TUI consistency work. Use evidence rather than file length alone: package cohesion,
change concentration, repeated helper patterns, dependency direction, test locality, and whether a
small internal package would reduce future bug risk without creating unnecessary abstraction.

### Package And File Boundary Audit Review

- Added `docs/package-boundary-audit.md` with size, cohesion, dependency-direction, test-locality,
  and package-boundary findings.
- Identified `internal/tui/model.go` and `internal/tui/model_test.go` as the urgent monolith risk at
  audit time. The issue was file-level responsibility concentration, not package import direction.
- Recommended same-package splits first so the current Bubble Tea `Model` boundary stays intact:
  detail, detail rendering, comments, create issue, create AI, Claude assist, worker commands,
  issue list, diagnostics, and shared model state.
- Recommended deferring new `internal/tui` subpackages until reusable adapters have multiple
  independent callers and no dependency on app-specific `Model` state.
- Recommended keeping `internal/jira` and `internal/worker` as current package boundaries, with
  later file-level splits by endpoint/request family when churn or cache/scheduler work justifies it.
- Updated the backlog so the next structural work was a mechanical same-package TUI file split, not
  a broad architectural rewrite.
- Verified with `make check`.

## Ticket Detail Menus And Actions Regression Pass

- [x] Reproduce the missing Add Comment path from the focused Comments section.
- [x] Inventory ticket-detail keys, section activations, field editors, and action menu behavior.
- [x] Add regression tests for every broken or under-covered visible ticket-detail action.
- [x] Fix root causes with minimal section/keymap changes.
- [x] Run focused ticket-detail tests, `go test ./internal/tui -count=1`, `go test ./... -count=1`,
  and `make check`.
- [x] Update docs/changelog/review notes before returning to TUI consistency work.

### Ticket Detail Menus And Actions Regression Pass Scope

Before resuming the hand-rolled TUI component audit, verify ticket-detail interactions end to end:
section tabs, section-specific footer hints, `enter` activation, Actions menu entries, Summary,
Priority, Assignee, Status, Links, Hierarchy, Comments, browser/copy commands, create shortcut,
help context, and cancellation behavior. Prefer one consistent activation pattern: visible section
or menu actions must have a matching key binding, footer/help entry, and focused regression test.

### Ticket Detail Menus And Actions Regression Pass Review

- Restored the focused Comments section action: `enter` opens the existing comment composer and the
  footer advertises `enter add`.
- Enabled implemented Actions menu routes for Status transitions and Assignee changes, delegating to
  the same worker-backed flows used by direct section/field activation.
- Added an Assignee key context so the assignee picker owns its footer/help instead of inheriting
  generic Ticket Detail commands.
- Added contract tests for focused ticket-detail `enter` behavior and section footer hints across
  Summary, Assignee, Priority, Links, Hierarchy, Comments, Actions, and Status.
- Captured the consistency lesson: visible targets, footer/help bindings, key handling, and resulting
  mode/command need to be tested together.
- Verified with focused ticket-detail tests, `go test ./internal/tui -count=1`, `go test ./... -count=1`,
  and `make check`.

## Hand-Rolled TUI Component Audit

- [x] Inspect current backlog, working agreement, dependency baseline, and recurring TUI lessons.
- [x] Write a design spec for the audit and first implementation slice.
- [x] Write an implementation plan for the audit matrix and first maintained-primitive migration.
- [x] Create `docs/tui-component-audit.md` with custom TUI surfaces, candidates, recommendations, and priorities.
- [x] Add or confirm focused tests around the first low-risk migration target.
- [x] Implement one small maintained-primitive migration: config scalar text input now uses Bubbles `textinput`.
- [x] Update docs/changelog/backlog as needed.
- [x] Run focused tests, `go test ./... -count=1`, and `make check`.

### Hand-Rolled TUI Component Audit Scope

Audit custom TUI rendering/input surfaces before adding more workflows. Prefer maintained Bubble
Tea/Bubbles/Lip Gloss primitives where they fit, but keep Jira-specific behavior and current worker
flows stable. The audit found footer/help rendering and config scalar text input are lower-risk first
migrations than create metadata pickers. Create pickers remain a near-term target because long
component lists have already caused usability friction.

### Hand-Rolled TUI Component Audit Review

- Added `docs/tui-component-audit.md` with ranked custom TUI surfaces and maintained Bubble
  Tea/Bubbles/Lip Gloss candidates.
- Chose config scalar text input as the first small migration because it is isolated from Jira
  workflows and easy to regression test.
- Replaced the config editor's manual scalar edit buffer with Bubbles `textinput`, preserving
  custom boolean/color field behavior.
- Added cursor-aware scalar edit coverage proving left-arrow movement inserts text at the cursor
  instead of appending or treating the arrow as literal text.
- Replaced local footer command rendering with a Bubbles `key`/`help` adapter while preserving
  existing key contexts, footer labels, grouping, and full help text.
- Added a shared Bubbles list-backed choice-list adapter and migrated comment mention results onto
  it as the first bounded picker/list surface.
- Migrated Assignee search results onto the same shared Bubbles list-backed choice-list adapter,
  preserving Jira user search and assignment behavior while sharing pagination/range rendering.
- Migrated focused dynamic create option fields onto the same shared adapter, preserving existing
  typeahead filter and selection behavior for Jira-provided options.
- Migrated create issue type selection onto the same shared adapter, preserving keyboard movement
  and create-field metadata loading behavior.
- Replaced manual create option filter input with Bubbles `textinput`, keeping existing filter
  strings synchronized for matching and submit behavior.
- Migrated the Priority picker onto the same shared adapter, preserving metadata-backed selection
  and submit behavior.
- Captured the consistency lesson: component migrations should add one reusable adapter path and
  move surfaces onto it incrementally instead of creating one-off wrappers.
- Left richer multi-column/action-state lists for the package/file boundary audit rather than
  forcing them into the simple choice-list adapter.
- Added a design-first backlog item for responsive large Jira view loading so cache persistence,
  freshness semantics, and priority scheduling are planned before implementation starts.
- Added a post-TUI-consistency backlog item to audit package/file boundaries and recommend
  split/no-split changes before the next large feature.
- Verified with `go test ./internal/configui -count=1`,
  `go test ./internal/tui -run 'Test(KeyBindingsAdaptToBubblesKeyHelp|FooterHelp|HelpOverlay|DetailFooter)' -count=1`,
  `go test ./internal/tui -run 'Test(ChoiceListUsesSharedBubblesListAdapter|CommentComposerMentionPicker|CommentComposerShowsUnresolvedMentions)' -count=1`,
  `go test ./... -count=1`, and `make check`.

## ADF Completion With Real Fixtures And Compact Code Blocks

- [x] Add tests that rich code blocks render without ASCII border rows or side borders.
- [x] Add tests that code block blank-line trimming preserves one separator before a block.
- [x] Implement compact styled code-block rendering in the TUI rich-body renderer.
- [x] Add sanitized ADF fixture helper tests for mentions, links, cards, account IDs, and deterministic output.
- [x] Implement sanitized ADF fixture helper.
- [x] Add raw ADF description/comment Jira client access for dev fixture capture.
- [x] Add a dev-only ADF fixture capture/sanitize command.
- [x] Add real/sanitized description and comment JSON/golden fixtures.
- [x] Update docs/changelog/lessons/backlog, including promoting hand-rolled TUI component audit as the next major initiative.
- [x] Run focused tests, `go test ./... -count=1`, `make check`, and `make install-user`.

### ADF Completion With Real Fixtures And Compact Code Blocks Scope

Complete the ADF renderer work by adding a repeatable dev-only sanitized fixture workflow, offline
golden tests for real-shaped Jira payloads, and compact TUI rendering for code blocks. Keep
`internal/adf.Render` as plain text/Markdown-like output and keep normal tests free of live Jira
dependencies.

### ADF Completion With Real Fixtures And Compact Code Blocks Review

- Added sanitized real-shaped Jira ADF description and comment fixtures under `internal/adf/testdata`
  and included them in the offline golden fixture suite.
- Added `internal/adf/fixture` to sanitize raw ADF nodes by replacing private account identifiers,
  mention names/emails, and URLs with stable placeholders while preserving rendering-relevant node
  structure.
- Added `cmd/adf-fixture` as a dev-only helper that can sanitize a local raw ADF JSON file or fetch a
  selected issue description/comment ADF through the existing Jira config, then write a sanitized
  fixture.
- Added raw ADF Jira client accessors for descriptions and selected comments so fixture capture does
  not depend on already-rendered display strings.
- Replaced ASCII bordered code blocks in rich TUI text with compact foreground/background-styled
  code lines, preserving trimming and width fitting while using less screen space.
- Promoted the hand-rolled TUI rendering/input component audit as the next major backlog initiative.

## Create Ticket Component Typeahead And AI Guessing

- [x] Add tests that create-ticket AI prompts include only Jira-returned Components.
- [x] Add tests that an AI `Components:` recommendation selects a matching Jira component.
- [x] Add tests that unknown AI component recommendations do not select a random component.
- [x] Add tests that typing in a focused Components picker filters options and `enter` selects from filtered results.
- [x] Add tests that `backspace` edits and `esc` clears a picker filter.
- [x] Add tests that Jira metadata `Project` and `Issue Type` required fields do not block create.
- [x] Implement create-picker filter state, rendering, movement, selection, and clearing.
- [x] Add Components to the create-ticket AI prompt and reuse existing AI field application.
- [x] Update docs/changelog/lessons as needed.
- [x] Run focused tests, `go test ./... -count=1`, `make check`, and `make install-user`.

### Create Ticket Component Typeahead And AI Guessing Scope

Use Jira create metadata as the source of truth. Add generic typeahead behavior for option-backed
create fields, but keep AI guessing scoped to Components by listing Available Components in the
create-ticket prompt and applying only exact/fuzzy matches against Jira-returned options.

### Create Ticket Component Typeahead And AI Guessing Review

- Create-field pickers now support inline typeahead filtering while focused; typed characters narrow
  Jira-provided options, `backspace` edits the filter, `esc` clears the filter before closing, and
  `enter` selects the current filtered option.
- Optional picker-backed fields, including Components, now start unselected so Jira does not receive
  a random first option. Required picker fields and Priority still keep a safe default selection.
- Claude create-ticket prompts now include an `Available Components` section from Jira create
  metadata and ask for `Components` only from that list.
- Matching AI `Components` recommendations are applied through the existing Jira option matching;
  unknown component names leave the field unselected instead of picking a fallback.
- Jira metadata-required `Project` and `Issue Type` fields are treated as built-in create context,
  so they no longer show as unsupported required fields after the user has already selected the
  project and issue type.

## ADF Renderer Library Spike

- [x] Add fixture/golden tests that capture realistic Jira ADF descriptions/comments before changing production rendering.
- [x] Evaluate maintained Go ADF conversion options and terminal Markdown renderer options against the fixture needs.
- [x] Decide whether to extend the current renderer, wrap a library, or split off a second spike.
- [x] If keeping the current renderer, add the smallest production changes needed to pass the new fixture tests.
- [x] Update docs/changelog/backlog as needed.
- [x] Run focused ADF tests and broader verification if production code changes.

### ADF Renderer Library Spike Scope

Keep this slice renderer-only. Preserve `internal/adf.Render(node) string` as the app boundary and
avoid ticket-detail TUI layout changes. The first deliverable is evidence: realistic fixture
coverage plus a dependency decision. A terminal Markdown renderer is useful only after ADF has been
converted; it does not replace ADF traversal by itself.

### ADF Renderer Library Spike Review

- Found `github.com/rgonek/jira-adf-converter` as a viable Go ADF-to-Markdown converter.
- Kept `internal/adf.Render(node) string` as the stable app boundary.
- Switched non-table ADF blocks to the maintained converter and normalized Markdown for current TUI
  compatibility.
- Kept the existing custom table renderer for Jira tables because the candidate converter mishandled
  some all-`tableCell` and rich-cell table shapes already covered by local tests.
- Added realistic fixtures for Jira descriptions and extended ADF nodes including headings, links,
  mentions, code blocks, panels/statuses, dates, cards, expands, and app-compatible table output.
- Added JSON fixture/golden files under `internal/adf/testdata` so future renderer changes can be
  checked against Jira-shaped payloads instead of only Go constructor fixtures.
- Added long-table and nested-list fixture coverage for code-heavy rollout notes, multiline table
  cells, links, mentions, and long acceptance-criteria text.
- Left media nodes out of the solved scope because the converter's default external-media fallback
  is not useful enough without a dedicated media hook.

## Create Ticket Open Questions Loop

- [x] Add tests that AI create drafts parse `Open Questions` separately from Description.
- [x] Add tests that create form renders Open Questions as a selectable local feedback panel.
- [x] Add tests that answering an Open Question stores local feedback without writing Jira.
- [x] Add tests that refinement prompts include current draft plus question answers.
- [x] Implement Open Questions parsing/state/rendering.
- [x] Implement question answer editor and refine prompt context.
- [x] Update lessons/review notes.
- [x] Run focused tests, `go test ./... -count=1`, `make check`, and `make install-user`.

### Create Ticket Open Questions Loop Scope

Use the inline Questions panel approach. When Claude returns `Open Questions`, the create-ticket
form should extract them from the generated draft and present them as actionable local feedback
items. The user can focus the Questions panel, select a question, answer it locally, then run
Generate Draft again so Claude receives the current draft plus those answers. Jira writes remain
gated behind the normal explicit create action.

### Create Ticket Open Questions Loop Review

- AI create drafts now parse `Open Questions` into local actionable questions instead of leaving
  them buried in Description.
- The create form renders an `Open Questions` panel when Claude returned questions.
- Users can focus the panel, select a question, press `enter`, type an answer, and save it locally
  with `ctrl+s`.
- While answering, `enter` saves the current answer and advances to the next question so users can
  answer several questions before spending another Claude request.
- The focused Questions panel exposes `ctrl+r refine with answers` so users can resync Claude
  without tabbing away to find Generate Draft.
- The next Generate Draft prompt includes answered questions and still-unanswered questions so
  Claude can refine the existing draft from user feedback.
- Existing Jira create remains the only write path; question answers are local drafting context.

## Create Ticket Modal Readability

- [x] Add tests that create-ticket AI loading stays calm and hides stream/debug detail.
- [x] Add tests that create-ticket dialogs use responsive width instead of the old narrow cap.
- [x] Add tests that focused Summary and Description editors get more usable space.
- [x] Implement calmer create-ticket AI loading copy.
- [x] Implement responsive create-ticket modal width and larger editors.
- [x] Update lessons/review notes.
- [x] Run focused tests, `go test ./... -count=1`, `make check`, and `make install-user`.

### Create Ticket Modal Readability Scope

Clean up the create-ticket modal after AI generation. The normal Claude waiting state should show
that the subprocess is active without streaming noisy partial assistant snippets or command/debug
details. The create form should use more of the terminal width, Summary should behave like a small
multi-line editor when focused, and Description should get primary vertical space while focused.

### Create Ticket Modal Readability Review

- Create-ticket AI loading now shows stable response/elapsed state and hides partial assistant text,
  command path, start time, and deadline from the normal modal.
- Create-ticket dialogs use a responsive max width based on terminal size instead of the narrow
  default dialog cap.
- Focused Summary now uses a small multiline editor.
- Focused Description now gets substantially more vertical space while staying inside the bounded
  create body.

## Create Ticket AI Issue Type Selection

- [x] Add tests that Claude create-ticket prompts include only Jira-returned issue types.
- [x] Add tests that Claude can recommend a Jira-supported issue type and preserve the generated draft.
- [x] Add tests that users can change create-ticket type without restarting or losing Summary/Description.
- [x] Implement Jira issue-type-aware prompt building and result handling.
- [x] Implement a focusable Type row in the create form.
- [x] Update lessons and review notes.
- [x] Run focused tests, `go test ./... -count=1`, `make check`, and `make install-user`.

### Create Ticket AI Issue Type Selection Scope

Claude-assisted ticket creation must use the issue types Jira returned for the selected project. Do
not guess common Jira types or hard-code presets into the prompt. When Claude has enough context, it
can recommend one of those returned types; the TUI should apply that match and load the correct Jira
create metadata while keeping the generated Summary/Description draft editable. Users must also be
able to change the type from the create form without throwing away the draft.

### Create Ticket AI Issue Type Selection Review

- Claude create-ticket prompts now include an `Available Jira Issue Types` section built only from
  Jira-returned project metadata.
- Claude can recommend `Issue Type`; the TUI applies it only when it matches a returned Jira type by
  name or ID, then loads create-field metadata through the existing worker path.
- Unsupported or unknown AI type recommendations keep the local Summary/Description draft and return
  the user to manual type selection.
- The manual create form now has a focusable Type row. Press `enter` on Type to change issue type
  without clearing Summary or Description.
- Focus indexes are named helpers now, so tests and editor routing are less brittle after adding
  Type to the focus order.

## Inline Description AI

- [x] Add TUI tests that Description focus exposes `a AI` only when Claude Ticket Assist is available.
- [x] Add TUI tests that `a` on Description opens the `AI for Description` picker and can cancel without running Claude.
- [x] Add TUI tests that inline Description AI actions submit scoped Claude prompts with ticket, Description, comments, and user instruction context.
- [x] Add TUI tests that inline Description AI results reuse the Ticket Assist modal zones.
- [x] Add TUI tests that inline Description AI apply updates Description only and never Summary.
- [x] Add TUI tests that inline Description AI drafts can still post as Jira comments through confirmation.
- [x] Implement inline Description AI state, picker, instruction editor, and prompt builder.
- [x] Implement Description-only Ticket Assist apply target while preserving normal Ticket Assist Summary+Description apply.
- [x] Update docs, changelog, and review notes.
- [x] Run focused tests, `go test ./internal/tui`, `make check`, and `make install-user`.

### Inline Description AI Scope

Implement the approved A+ hybrid pattern from
`docs/superpowers/specs/2026-06-15-inline-description-ai-design.md`. Normal ticket detail stays
simple; when Description is focused and Claude Ticket Assist is available, `a` opens a contextual
`AI for Description` picker. The picker supports improve clarity, extract acceptance criteria, ask
Claude a question, and draft clarifying comment. Results reuse the existing Ticket Assist modal and
draft/refine/copy/comment mechanics, but inline Description apply must update Description only.

### Inline Description AI Design

Follow `docs/superpowers/plans/2026-06-15-inline-description-ai-implementation.md`. Keep Claude IO
on the existing background runner and diagnostics path. Add only small scoped TUI state for the
inline picker/instruction flow and a draft target that distinguishes normal Ticket Assist from
Description-scoped output. If Description detail is not loaded, do not call Claude; show
`Description is not loaded yet.`

### Inline Description AI Review

- Added Description-scoped inline AI entry with `a` from the Description section.
- Added an `AI for Description` picker with improve clarity, extract acceptance criteria, ask
  Claude a question, and draft clarifying comment actions.
- Reused the existing Claude runner, Ticket Assist modal, refine, copy, comment, progress, and
  Diagnostics flows.
- Added a Description-only Ticket Assist draft target so inline AI apply updates Description and
  cannot change Summary.
- Preserved normal Ticket Assist Summary plus Description apply behavior behind the existing target.
- Final verification passed with focused inline AI/Ticket Assist tests, `go test ./internal/tui`,
  `make check`, and `make install-user`.

## Ticket Assist Output Clarity And Comment Path

- [x] Add TUI tests that Ticket Assist output renders distinct Claude Review, Local Draft, and action zones.
- [x] Add TUI tests that the action zone explains apply vs comment vs refine vs copy.
- [x] Add TUI tests that a Ticket Assist draft can be posted as a Jira comment without editing fields.
- [x] Implement the clearer Ticket Assist modal layout.
- [x] Implement the Ticket Assist comment confirmation and worker-backed post flow.
- [x] Update docs, changelog, lessons, and review notes.
- [x] Run focused tests, `make check`, and `make install-user`.

### Ticket Assist Output Clarity And Comment Path Scope

Ticket Assist results must clearly separate Claude's review from the user's local editable draft and
from available actions. The modal should make it obvious what came from Claude, what is local and
not yet applied, and what the user can do next. Add a comment path for tickets the user should not
directly edit: from the draft modal, `c` opens a confirmation to post the current draft as a Jira
comment, while the existing gated `ctrl+s` path remains the direct Summary/Description edit flow.

### Ticket Assist Output Clarity And Comment Path Design

Render the modal as three zones: `Claude Review`, `Local Draft`, and `Available Actions`. `Claude
Review` is a bounded read-only preview. `Local Draft` is the large editable region and shows a `Not
Applied` state label. `Available Actions` is a short body-level action hint that explains `ctrl+s`
apply, `c` comment, `r` refine, `ctrl+y` copy, and `esc` close; if Jira writes are disabled, it
should say so. Comment posting uses the existing worker AddComment request/result path with
Ticket-Assist-specific state and confirmation, then refreshes comments on success.

### Ticket Assist Output Clarity And Comment Path Review

- Renamed the review area to `Claude Review` and the editable area to `Local Draft Not Applied`.
- Added an `Available Actions` zone on normal-height terminals so apply/comment/refine/copy choices
  are visible in the body, not only the footer.
- Kept cramped terminals within bounds by hiding the body action zone and relying on the footer.
- Added `c` from the Ticket Assist draft modal to open a `Post Draft As Comment` confirmation.
- Posting a comment uses the existing worker `AddComment` path with Ticket-Assist-specific state,
  closes the modal on success, and refreshes comments.
- Captured inline detail-section AI actions as the next design slice: field/comment scoped AI
  should reuse the same local draft, refinement, apply, and comment machinery.
- Final verification passed with `go test ./internal/tui -run 'TestClaudeTicketAssist'`,
  `go test ./internal/tui`, `make check`, and `make install-user`.

## Ticket Assist Refinement Loop

- [x] Add TUI tests that `r` opens a refinement instruction editor from Ticket Assist.
- [x] Add TUI tests that submitting refinement sends Claude the current user-edited draft plus instruction.
- [x] Add TUI tests that the refinement result replaces the editable draft and remains local until apply.
- [x] Implement the refinement instruction editor, loading state, prompt builder, and result handling.
- [x] Update docs, changelog, lessons, and review notes.
- [x] Run focused tests, `make check`, and `make install-user`.

### Ticket Assist Refinement Scope

Make Ticket Assist iterative without turning it into a general chat app. From an existing Ticket
Assist draft, `r` opens a small instruction editor. The user can tell Claude how to adjust the
current draft, then `ctrl+s` sends the refinement request. The prompt must include the current
user-edited draft, not only the original Claude output, so Claude can revise instead of
reinventing. The returned draft replaces the editable draft and still requires explicit user review
and gated Jira apply.

### Ticket Assist Refinement Design

Refinement reuses the existing Claude Ticket Assist modal and background runner. The modal has
three non-write states: editable draft, instruction editor, and Claude loading. The instruction
editor is local and cancellable with `esc`; `ctrl+s` sends a read-only refinement prompt containing
the original ticket context, the current edited draft, and the user instruction. While running, the
normal calm Claude loading state is shown. On success, the returned draft replaces the editable
draft and the review text updates if Claude returned a Review section. On failure, the current draft
and instruction remain available.

### Ticket Assist Refinement Review

- Added a refinement instruction editor opened with `r` from the Ticket Assist draft modal.
- Submitting refinement with `ctrl+s` runs Claude through the existing background Ticket Assist
  request/progress path and calm loading modal.
- The refinement prompt includes original ticket context, the user's instruction, and the current
  user-edited draft so Claude revises from the actual draft state.
- Successful refinement replaces the editable draft and updates review text if Claude returns a
  Review section; Jira writes remain gated behind the existing apply flow.
- Verified so far with `go test ./internal/tui -run 'TestClaudeTicketAssistR|TestClaudeTicketAssistRefinement'`
  and `go test ./internal/tui -run 'TestClaudeTicketAssist'`.
- Final verification passed with `go test ./internal/tui`, `make check`, and `make install-user`.

## Ticket Assist Apply To Jira

## Ticket Assist Apply To Jira

- [x] Add Jira client tests and implementation for updating Description with ADF text.
- [x] Add worker request/result tests and implementation for Description updates.
- [x] Add TUI tests that Ticket Assist `ctrl+s` opens an apply confirmation when Jira writes are enabled.
- [x] Add TUI tests that disabled Jira write gates keep Ticket Assist local-only.
- [x] Add TUI tests that confirming applies parsed Summary and Description while preserving draft on failure.
- [x] Fix Ticket Assist draft modal sizing so the editable draft is visually distinct and gets primary space.
- [x] Implement Ticket Assist apply confirmation and worker-backed save flow.
- [x] Update docs, changelog, lessons, and review notes.
- [x] Run focused tests, `make check`, and `make install-user`.

### Ticket Assist Apply Scope

Make Ticket Assist actionable for existing tickets without adding a pile of new keys. When the
edited draft is ready, `ctrl+s` is the single save/apply action if Jira writes are enabled; `esc`
remains the non-writing close/cancel path. The first apply slice updates Jira Summary and
Description only. Acceptance Criteria stay first-class in the editable draft and are written into
Description under their own heading for now.

### Ticket Assist Apply Design

Ticket Assist keeps the editable draft modal. `ctrl+s` parses the edited draft into a Summary value
and a Description body, then opens a confirmation state that names the issue and fields to update.
Confirming submits worker-backed Jira updates for Summary and Description. The modal remains open
while saving. Success refreshes the visible issue summary and cached detail description, records
Diagnostics, and closes the modal with a notice. Failure leaves the edited draft intact and shows
the Jira error. If `allow_jira_writes` is false, `ctrl+s` does not write; the footer and notice
explain that Jira writes are disabled and `ctrl+y` remains the copy path.

### Ticket Assist Apply Review

- Added Jira client support for updating Description using the same ADF text payload style as issue
  creation.
- Added worker request/result support for Description updates.
- Extended TUI Claude config with `require_confirmation` and `allow_jira_writes` gates from the
  existing app config.
- Changed Ticket Assist `ctrl+s` from close-only to the gated save/apply path; `esc` remains the
  non-writing close/cancel path.
- Added an apply confirmation that previews Summary and Description before Jira writes.
- Confirming applies Summary and Description through the worker pool, updates the visible issue
  summary and cached detail description, and closes with a notice after both writes complete.
- With Jira writes disabled, Ticket Assist stays local-only and keeps `ctrl+y` as the copy/export
  path.
- Enlarged and visually separated the editable draft block, and hides the review preview on cramped
  terminals so draft editing remains usable.
- Final verification passed with focused Jira, worker, and TUI tests,
  `go test ./internal/jira ./internal/worker ./internal/tui`, `make check`, and
  `make install-user`.

## Existing Ticket Assistance

- [x] Add config/config UI coverage for a `ticket_assist` Claude feature flag.
- [x] Add TUI regression coverage that the Claude section exposes ticket assistance for existing tickets.
- [x] Add TUI regression coverage that ticket assistance sends a read-only evaluation/sanitization prompt.
- [x] Add TUI regression coverage that Claude's returned draft opens in an editable review modal.
- [x] Add TUI regression coverage that long Ticket Assist drafts are bounded, paged, and copyable.
- [x] Add TUI regression coverage that Ticket Assist loading suppresses partial assistant text.
- [x] Add TUI regression coverage that `a` jumps to the AI/Claude section when available.
- [x] Implement the gated ticket-assist workflow without Jira writes.
- [x] Calm Claude loading modals so they show stable subprocess status instead of changing partial text.
- [x] Update docs, changelog, lessons, and review notes.
- [x] Run focused tests, `make check`, and `make install-user`.

### Existing Ticket Assistance Scope

Add the first existing-ticket assistance workflow. From a focused ticket detail Claude section, users
can ask Claude to evaluate the current Jira ticket and produce a structured improved draft. The
workflow must be read-only: no Jira writes, no git/GitHub/code changes, and no external side effects
beyond running the user's local Claude CLI. Claude output should be editable before use because the
model can overreach. Acceptance Criteria must be a first-class draft section even if Jira stores it
inside the Description for now.

### Existing Ticket Assistance Design

The Claude section should expose two distinct actions when enabled: ticket plan and ticket assist.
Ticket plan keeps the current implementation/verification plan behavior. Ticket assist sends the
selected ticket metadata, description, and loaded comments to Claude with instructions to return:
review findings, a rewritten summary, a problem/goal section, acceptance criteria, test/verification
notes, implementation notes, and open questions. When the request finishes, the TUI opens a
bounded modal with the review and an editable draft editor. The editor is local only in this slice;
later work can add copy/apply-to-Jira behind confirmation and write gates.

### Existing Ticket Assistance Review

- Added a `ticket_assist` Claude feature flag to config load/save and the config editor.
- Added Ticket Assist as a selectable Claude section action alongside Ticket Plan.
- Added a read-only Claude prompt for evaluating existing tickets and drafting Summary, Problem /
  Goal, Acceptance Criteria, Test / Verification, Implementation Notes, and Open Questions.
- Rendered Claude ticket-assist results in a modal with read-only review findings plus an editable
  local draft textarea; closing the modal does not write to Jira.
- Bounded long Ticket Assist review/draft output, added draft line-range/page hints, routed
  `pgup`/`pgdn` through the editor, and added `ctrl+y` to copy the edited draft.
- Calmed the Ticket Assist and Ticket Plan loading modals so running AI calls show stable output
  states instead of constantly changing partial assistant text; detailed stream output remains in
  Diagnostics and final result/error views.
- Made `a` jump to the Claude/AI section when Claude actions are available, while preserving
  add-comment as the fallback when Claude is unavailable and through the Actions workflow.
- Verified so far with focused config, config UI, and TUI tests.
- Final verification passed with `go test ./internal/claude ./internal/config ./internal/configui ./internal/tui`,
  `go test ./internal/tui`, `make check`, and `make install-user`.

## Claude Loading Modal Cleanup

- [x] Add TUI regression coverage for the concise Claude loading modal.
- [x] Add TUI regression coverage for rendering Claude Markdown tables as bounded table blocks.
- [x] Replace debug-heavy loading copy with subprocess activity, elapsed progress, and stable output status.
- [x] Render final Claude Markdown pipe tables through the existing fitted table renderer.
- [x] Update docs, changelog, lessons, and review notes.
- [x] Run focused tests, `make check`, and `make install-user`.

### Claude Loading Modal Cleanup Scope

The normal Claude ticket-plan loading modal should feel like a running product workflow, not a
debug pane. Keep clear feedback that the local Claude subprocess is alive, keep elapsed progress,
show assistant preview/status when stream output arrives, and keep `esc` cancellation. Move command,
request, start, and deadline detail out of the normal waiting modal; timeout/error states and
Diagnostics can still carry deeper troubleshooting context.

### Claude Loading Modal Cleanup Review

- Replaced normal Claude loading modal debug rows with an activity spinner-style subprocess line,
  elapsed progress, and stable output state.
- Kept timeout/error handling rich enough to show start/deadline/command evidence when something
  actually fails.
- Added Markdown pipe-table normalization for final Claude plans so existing fitted table rendering
  handles wide Claude tables inside the bounded modal.
- Captured ticket-to-local-workspace mapping as the next planned feature slice for future Claude and
  code-workflow context.
- Verified so far with `go test ./internal/tui -run 'TestClaudePlanShowsSubprocessActivityAndCancelHintWhileRunning'`
  and focused Claude modal/table tests.
- Final verification passed with `go test ./internal/claude`, `go test ./internal/tui`,
  `make check`, and `make install-user`.

## Claude Plan Interaction Fix

- [x] Add TUI regression coverage for long final Claude results staying inside the visible panel.
- [x] Bound final Claude result rendering and show a line-range/scroll hint.
- [x] Add TUI regression coverage for assembling assistant delta chunks into one preview.
- [x] Update rolling assistant preview logic to append deltas while still deduping repeated cumulative partials.
- [x] Add Claude runner tests for stream-json CLI progress and stderr events.
- [x] Add TUI tests that Claude progress/stderr appears in the modal before final completion.
- [x] Run Claude with stream-json partial output when progress reporting is requested.
- [x] Feed Claude progress events through Bubble Tea commands while the request is running.
- [x] Add TUI regression tests for elapsed loading state and cancel behavior.
- [x] Add a cancellable context for in-flight Claude ticket-plan requests.
- [x] Render elapsed time, configured timeout, and cancel guidance while Claude is running.
- [x] Ignore stale/cancelled results while still recording Diagnostics.
- [x] Add absolute start/deadline and command context to the Claude modal.
- [x] Make timeout failures say how long Claude ran before the deadline.
- [x] Update docs, changelog, lessons, and review notes.
- [x] Run focused tests, `make check`, and `make install-user`.

### Claude Plan Interaction Fix Scope

The first Claude ticket-plan workflow can legitimately take time or fail inside the local Claude
CLI. The TUI must not look frozen while this happens. Keep the request backgrounded, show elapsed
time and timeout context in the modal, allow `esc` to cancel the running process, and make the final
state visible through the modal and Diagnostics. Do not add Jira/git/GitHub/code write behavior.

### Claude Plan Interaction Fix Review

- Added a Claude runner streaming path that runs `claude --verbose --output-format stream-json
  --include-partial-messages -p <prompt>` when progress reporting is requested.
- Parsed stream-json stdout and stderr into progress events while preserving the final result.
- Parsed nested Claude `stream_event` content deltas into readable output events and suppressed raw
  JSON envelopes in the modal.
- Filtered protocol/user events out of visible progress and changed partial output rendering to a
  single rolling assistant preview to avoid repeated overlapping chunks.
- Assembled true assistant delta chunks into one readable preview while still deduping repeated
  cumulative partials.
- Bounded long final Claude plans inside the visible ticket-detail panel and added a Claude line
  range hint.
- Added independent scroll state for final Claude plan results with `j`/`k`, arrows, page keys, and
  top/bottom jumps.
- Made the Claude plan dialog use a responsive percentage of available width so final plans are
  readable on wide terminals.
- Added a Bubble Tea progress channel/message path so the modal can repaint before final completion.
- Rendered recent Claude progress/stderr/output events in the ticket-plan modal.
- Added regression coverage for elapsed/timeout/cancel modal text while Claude is running.
- Added regression coverage that `esc` cancels the in-flight Claude runner context.
- Added a Claude runner regression proving `context.DeadlineExceeded` does not fire before the
  configured timeout.
- Added cancellable request context storage for active Claude ticket-plan requests.
- Added a one-second redraw tick while Claude is running so elapsed time can update.
- Changed loading modal footer from `esc close` to `esc cancel`; after cancel, the modal shows the
  cancelled state and stale runner results are ignored while Diagnostics still records them.
- Added request type, read-only mode, sanitized command, output wait state, start time, deadline,
  and timeout-specific failure detail to the Claude modal.

## Claude Ticket Plan Workflow

- [x] Add Claude runner tests for executing a read-only prompt through the local CLI.
- [x] Implement `claude.LocalRunner.Run` with bounded output and timeout support.
- [x] Add TUI tests that the Claude section is hidden unless enabled, available, and `ticket_plan` is flagged on.
- [x] Add TUI tests that activating the Claude section submits a ticket-plan request with selected ticket context.
- [x] Add TUI tests that successful results render in a modal and diagnostics record submit/result.
- [x] Implement gated Claude detail section, request command, prompt builder, result state, and modal rendering.
- [x] Update docs, changelog, lessons, and review notes.
- [x] Run focused tests, `make check`, and `make install-user`.

### Claude Ticket Plan Workflow Scope

First read-only Claude workflow. From focused ticket detail, show a Claude section only when Claude
is enabled, preflight is available, and the `ticket_plan` feature flag is true. Activating it sends
selected ticket context to local Claude Code/CLI in the background and renders the returned plan in
a modal. No Jira, git, GitHub, or code writes are allowed in this slice.

### Claude Ticket Plan Workflow Review

- Added `claude.LocalRunner.Run` for bounded local CLI prompt execution in print mode.
- Added a gated Claude ticket detail section that appears only when Claude is enabled, available,
  and `ticket_plan` is true.
- Built a read-only ticket-plan prompt from selected ticket metadata, description, and loaded
  comments.
- Submitted Claude plan requests asynchronously and rendered success/error output in the shared
  modal pattern.
- Recorded Diagnostics submit/result events for Claude ticket-plan requests.

## Config Boolean Pickers

- [x] Add config UI tests that boolean fields toggle without entering free-text edit mode.
- [x] Mark config fields with boolean value kind.
- [x] Render boolean fields as picker-style true/false values.
- [x] Route enter/space/left/right on boolean fields to toggle values directly.
- [x] Update docs, changelog, lessons, and review notes.
- [x] Run focused tests, `make check`, and `make install-user`.

### Config Boolean Pickers Scope

Boolean config fields, especially Claude feature flags and write gates, should not require typing
`true` or `false`. Keep the config editor simple, but treat booleans as picker/toggle fields:
pressing enter or space toggles the selected value, and left/right chooses false/true while staying
in the config menu. Do not change text fields or the TOML representation.

### Config Boolean Pickers Review

- Added config UI regression coverage that boolean fields toggle without entering free-text edit
  mode.
- Marked Claude boolean fields as boolean value-kind fields in the config UI model.
- Rendered boolean values as `false / true` picker choices instead of plain text.
- Routed `enter` and `space` to toggle, `left` to false, and `right` to true while keeping focus in
  the config menu.
- Verified with focused config/config UI tests, `make check`, and `make install-user`.

## Claude Local Setup Foundation

- [x] Add config tests for Claude defaults, TOML load/save, feature flags, gates, and timeout validation.
- [x] Implement Claude config structs under the existing Jira config schema.
- [x] Add config UI tests that the Jira config menu exposes Claude settings.
- [x] Implement a Claude config section with enable/path/timeout/features/gates fields.
- [x] Add a local Claude runner/preflight package that uses `exec.LookPath("claude")` when no command override is set.
- [x] Add tests for Claude auto-detect, explicit command, version check, timeout, and not-found status.
- [x] Wire startup preflight status into the TUI model/diagnostics without blocking Jira startup when Claude is optional.
- [x] Update docs, changelog, lessons, and review notes.
- [x] Run focused tests, `make check`, and `make install-user`.

### Claude Local Setup Foundation Scope

First infrastructure slice only: configure and verify local Claude Code/CLI availability. Default
Claude to disabled, default command to auto-detect `claude`, allow manual command/path override, and
record runtime status for diagnostics. Do not add Jira AI workflows yet. Claude workflows must later
check config feature flags and write gates before they are shown, before work is enqueued, and
before any generated action is applied.

### Claude Local Setup Foundation Review

- Added `[claude]`, `[claude.features]`, and `[claude.gates]` to the existing TOML config schema.
- Defaulted Claude to disabled, command auto-detect, a two-minute timeout, confirmation required,
  and all write gates closed.
- Added Claude fields to the existing `jira config` editor so users can enable Claude, override the
  command/path, set timeout, turn features on/off, and control write gates.
- Added `internal/claude.LocalRunner` for local Claude Code/CLI preflight using `exec.LookPath` when
  command is empty and `claude --version` with a bounded timeout.
- Wired startup preflight status into TUI Diagnostics without blocking Jira startup when optional
  Claude is disabled or unavailable.
- Preserved safe default confirmation while still honoring an explicit
  `require_confirmation = false` in TOML.
- Verified with focused config/config UI/Claude/TUI tests, `make check`, and `make install-user`.

## Bounded Create Modal

- [x] Add TUI regression tests for large create metadata lists staying inside a bounded modal.
- [x] Implement a scroll-window renderer for create dynamic fields and long picker option lists.
- [x] Keep focused create field and selected picker option visible while navigating.
- [x] Update docs, changelog, lessons, and review notes.
- [x] Run focused tests, `make check`, and `make install-user`.

### Bounded Create Modal Scope

The create-ticket modal must not grow past the visible terminal when Jira returns many fields or a
long allowed-values list such as Components. Keep Summary and Description at the top, render Jira
metadata fields in a bounded area, and window long picker option lists around the selected option.
Do not introduce wizard pages yet and do not hard-code Jira fields.

### Bounded Create Modal Review

- Added a TUI regression test for a Components field with 30 Jira-provided options.
- Windowed long picker fields to six visible options plus an `Options x-y of n` range line.
- Kept the selected picker option visible as users move through long Jira option lists.
- Rendered inactive Summary and Description fields as compact one-line previews so large metadata
  forms fit the terminal while the focused text field still uses the editor surface.
- Verified with focused create modal tests, `make check`, and `make install-user`.

## Create Field Metadata Fallback

- [x] Add a Jira client regression test for empty field mappings falling back to expanded create metadata.
- [x] Implement fallback parsing for fields under `projects[].issuetypes[].fields`.
- [x] Update docs, changelog, and review notes.
- [x] Run focused tests, `make check`, and `make install-user`.

### Create Field Metadata Fallback Scope

Jira is returning issue types from create metadata, but selected issue-type field mapping requests
are returning zero fields. Match the issue-type discovery pattern: keep the paged field-mapping
endpoint first, then fall back to expanded create metadata for the selected project and issue type
when the primary response is empty. Do not add UI presets or hard-code fields.

### Create Field Metadata Fallback Review

- Added a regression test proving empty `FetchFieldMappings` responses retry expanded create
  metadata with the selected project key and issue type ID.
- Added fallback parsing for `projects[].issuetypes[].fields`, reusing the same field parser as the
  paged field-mapping response.
- Sorted expanded field IDs before parsing so object-map response ordering stays deterministic.
- Verified with focused Jira/TUI create metadata tests, `make check`, and `make install-user`.

## Dynamic Create Fields

- [x] Add Jira client tests for create requests carrying priority and custom field values.
- [x] Extend typed create requests through Jira client and worker without raw body logging.
- [x] Add TUI tests for rendering Jira create metadata fields beyond Summary/Description.
- [x] Add TUI tests for picker/text field editing and submission payloads.
- [x] Implement dynamic create field state for supported text, labels, priority, and single-option fields.
- [x] Keep unsupported required Jira fields visible and block submit before Jira returns validation errors.
- [x] Update docs, changelog, and lessons.
- [x] Run focused tests, `make check`, and `make install-user`.

### Dynamic Create Fields Scope

First slice: keep Summary and Description as dedicated editors, then render additional supported
fields returned by Jira create metadata. Supported fields are Priority, Labels, simple string/text
fields, and single-select option fields with Jira-provided allowed values. Unsupported required
fields are shown as blocking notices. Multi-value fields, cascading fields, rich user pickers, parent
issue pickers, components, fix versions, sprint, and asset fields remain follow-ups unless Jira
exposes them as simple allowed-value single-selects we can encode safely.

### Dynamic Create Fields Review

- Added typed `CreateIssueFieldValue` support to the Jira client and worker create request path.
- Encoded supported standard fields through go-atlassian issue fields: Priority, Labels, and
  Components.
- Encoded supported custom fields through go-atlassian `CustomFields.Raw`: simple text/number values
  and single-select option payloads.
- Added TUI state/rendering for supported create fields returned by Jira metadata, with `tab`
  navigation, `j`/`k` or arrow selection for pickers, and text input for simple fields.
- Unsupported required Jira fields now block submit with a clear local message instead of relying
  only on Jira validation errors.
- Added sanitized create-field diagnostics so `ctrl+d` shows total fields, supported fields,
  unsupported required fields, and a short field ID/name/schema sample for the selected issue type.
- Verified with focused Jira client, worker, and TUI create tests, then `make check`, then
  `make install-user`.

## Create Ticket Flow

- [x] Make `n` consistently create tickets from table and detail contexts while `tab` owns detail focus movement.
- [x] Add Jira client tests for creating an issue with project, issue type, summary, and ADF description.
- [x] Implement typed Jira create issue request/response support.
- [x] Add worker tests for background issue creation.
- [x] Implement worker create issue request/result support.
- [x] Add TUI tests for `n` opening create metadata, picking issue type, editing fields, and submitting.
- [x] Implement the first create-ticket modal flow.
- [x] Update docs, changelog, and lessons.
- [x] Run focused tests, `make check`, and `make install-user`.

### Create Ticket Flow Scope

First usable ticket creation slice: `n` opens a modal, loads create issue types for the active
project, lets the user choose a Jira-provided issue type, loads fields for that type, and renders
Summary plus Description editors. Submitting creates the ticket through the worker pool. Additional
custom/required fields are not rendered yet; Jira errors should stay visible in the modal.

### Create Ticket Flow Review

- Added typed Jira issue creation support that builds project, issue type, summary, and ADF
  description payloads through the existing Jira client boundary.
- Added worker-backed create requests/results so ticket creation stays off the Bubble Tea
  update/render loop.
- Added a go-atlassian create metadata fallback: if `FetchIssueMappings` succeeds with zero issue
  types, the client retries `Issue.Metadata.Create` with `projects.issuetypes` expansion.
- Added a first create-ticket modal opened with `n` from the issue table or focused ticket detail,
  with Jira-provided issue type selection, Summary and Description editors, and worker-backed
  submission.
- Updated key behavior so `tab`/`shift+tab` own ticket-detail focus movement and `n` consistently
  means new ticket instead of also acting as section navigation.
- Added the keymap clarity review as its own follow-up so future shortcuts keep one clear semantic
  path per workflow.
- Added clearer empty create metadata handling: if Jira returns zero creatable issue types, the
  modal reports that result, hides inactive field/submit commands, and leaves `ctrl+d` diagnostics
  available with sanitized result counts.
- Verified with focused Jira client, worker, and TUI tests, then `make check`, then
  `make install-user`.

### Create Ticket Draft Assistance

- [x] Add a create-form shortcut to open a "Generate Ticket Draft" prompt (`g`) when Claude draft
  is enabled.
- [x] Send contextual prompt with project and issue type, plus user instruction, to Claude.
- [x] Parse returned summary and description into the create form editors.
- [x] Keep modal open with a clear error if the draft result omits a parsable summary.
- [x] Add TUI tests for draft modal open/submit/fail-fast parse behavior and parser extraction.

### Create Ticket Draft Assistance Review

- Added create-form draft prompt support (`g`) for create ticket flow when Draft Ticket feature is
  enabled.
- Wired prompt execution through the existing Claude async path and recorded request fields, progress,
  and result handling.
- Added draft parsing into Summary and Description editors with parse-failure guardrails.
- Added focused create-draft tests for modal open, prompt result application, parse failure behavior,
  and parser extraction.

## Keymap Clarity Review

- [x] Audit every active key context for redundant commands that do the same thing with different
      semantics.
- [x] Keep conventional navigation aliases only where they reduce friction without changing meaning
      (`j`/`k` with arrows, page keys, home/end).
- [x] Prefer one primary action path per workflow: `tab` moves focus, `enter` acts on focus, and
      single-letter commands are stable accelerators for distinct actions.
- [x] Add tests for any removed or reassigned binding so help text and behavior stay aligned.
- [x] Update docs/changelog with the keymap clarity rules and the resulting command changes.

### Keymap Clarity Review Scope

The first pass of this review focused on detail-mode `o` behavior, which previously triggered table
sorting while not in the issue table. In table mode, `o/O` still cycles issue sort. In detail mode,
`o` now opens the selected issue in the browser, keeping the key's meaning aligned with context.

### Keymap Clarity Review Design

This pass keeps table `o/O` as sort controls where they are meaningful, while detail-mode
`o` is aligned with `b`-style issue-open behavior via footer/help visibility. `O` in detail mode
does not alter sorting.

### Keymap Clarity Review Result

- Added `o` as the detail-mode issue-open accelerator in key bindings and updated footer help to
  show `o open` in detail context.
- Preserved table sort semantics so `o/O` cycle sorting only while in issue-table mode.
- Added regression coverage that detail `o` opens the selected issue URL and does not affect sorting,
  that uppercase detail `O` does not sort, and table `o/O` still sort in table mode.
- Updated docs/project-state and changelog to reflect detail-mode open key semantics.

## Create Ticket AI Tabs And Cleanup

- [x] Add regression tests for `Manual` / `AI Generated` tabs on the first create-ticket modal.
- [x] Add regression tests that `tab` enters AI-generated mode before issue type selection and `ctrl+s` asks Claude for a draft.
- [x] Add regression tests that a generated draft returns to manual issue-type selection with Summary/Description prefilled.
- [x] Add regression tests that manual-form AI cleanup includes the current user-entered Summary and Description in the Claude prompt.
- [x] Implement tabbed create mode without adding one-letter shortcuts inside text entry.
- [x] Keep Jira create metadata as the final authority: AI drafts populate local fields, but users still select issue type and review before `ctrl+s create`.
- [x] Run focused create tests, `go test ./internal/tui`, and `go test ./...`.

### Create Ticket AI Tabs And Cleanup Scope

The first create-ticket modal should offer two tabbed modes: `Manual` for the Jira metadata-driven
issue-type picker, and `AI Generated` for describing the ticket to Claude before choosing a Jira
issue type. After Claude returns, the draft fills local Summary and Description and the flow returns
to manual issue-type selection so Jira still controls available fields. The manual create form keeps
its focused `Generate Draft` action, but its prompt must include the user's current draft so Claude
can clean up or refine entered text instead of starting over.

### Create Ticket AI Tabs And Cleanup Design

Use a small create-mode boolean to switch the initial modal body. In AI mode, reuse the existing
create AI prompt editor, Claude runner, progress, parser, and draft application code. `tab` changes
between `Manual` and `AI Generated` only before issue type selection; after issue type selection,
`tab` continues to move fields and `enter` activates the focused `Generate Draft` row. No AI
generation is triggered by a printable letter while a text editor owns input.

### Create Ticket AI Tabs And Cleanup Review

- Added a first-screen `Manual` / `AI Generated` tab mode to create-ticket when Claude draft
  generation is enabled.
- `tab` switches between manual issue-type selection and AI generated mode before issue type
  selection; after selecting an issue type, `tab` remains field navigation.
- AI generated mode can ask Claude for a draft before issue type selection. Returned Summary and
  Description are applied locally, then the flow returns to manual issue-type selection.
- Manual-form `Generate Draft` still works as a focused row, and its prompt now includes the user's
  current Summary and Description so Claude can clean up existing draft text.
- Jira create metadata remains the final gate: users still pick issue type, review populated fields,
  and explicitly create with `ctrl+s`.
- Verification passed with focused create tests, `go test ./internal/tui`, `go test ./...`, `make
  check`, and `make install-user`.

## Create Ticket Progressive Layout

- [x] Add regression tests that non-focused picker fields render as compact value rows instead of full option lists.
- [x] Add regression tests that Description gets expanded editor space when focused.
- [x] Add regression tests that the AI draft action stays near Summary/Description.
- [x] Implement compact metadata field rows with focused picker expansion only.
- [x] Increase focused Description editor height while keeping the modal bounded.
- [x] Move `Generate Draft` before Jira metadata fields so AI cleanup is part of drafting, not buried after dropdowns.
- [x] Run focused create tests, `go test ./internal/tui`, `make check`, and `make install-user`.

### Create Ticket Progressive Layout Scope

Make the manual create form usable for AI-assisted drafting. Summary and Description should be the
primary authoring surface. Jira metadata should remain visible, but dropdown-backed fields should
collapse to one-line selected values until focused. The AI draft action should sit near Summary and
Description so users can improve the current draft without tabbing through every Jira field.

### Create Ticket Progressive Layout Design

Render the create form in this order: Type, Summary, Description, Generate Draft, then metadata
fields. Description gets more visible rows while focused. Picker fields show `Field: selected value`
when not focused and expand to the bounded option picker only when focused. Free-text dynamic fields
show a compact `Field: value` row unless focused. Existing `tab` focus movement and `ctrl+s create`
semantics remain unchanged.

### Create Ticket Progressive Layout Review

- Moved `Generate Draft` directly after Description so AI cleanup is part of the drafting surface.
- Expanded the focused Description editor to use more modal space while keeping the create modal bounded.
- Collapsed unfocused Jira metadata fields to one-line `Field: value` rows.
- Kept focused picker fields expandable with the existing bounded option list.
- Preserved `tab` focus movement, `enter` generate behavior, and `ctrl+s create` semantics.

## Sanitized API Debug Log

- [ ] Extend Diagnostics into an opt-in API debug/event log for Jira operations.
- [ ] Record request IDs, operation names, endpoint families, project/issue keys, status/error
      classes, timing, and result counts without raw response bodies or credentials.
- [ ] Surface empty metadata responses clearly, such as create issue types returning zero values.
- [ ] Add a future export path that can prefill a GitHub issue for this app with sanitized logs and
      app/version/context data.

## Create Metadata Discovery

- [x] Add Jira client tests for project create issue type discovery.
- [x] Add Jira client tests for issue-type create field metadata discovery.
- [x] Implement typed create metadata structs and Jira client methods.
- [x] Add worker request/result tests for create issue types and create fields.
- [x] Implement worker request/result support for create metadata discovery.
- [x] Update docs, changelog, and lessons.
- [x] Run focused tests, `make check`, and `make install-user`.

### Create Metadata Discovery Scope

First slice for ticket creation: discover Jira create metadata without rendering a create form yet.
Fetch create issue types for a project, then fetch create fields for a selected issue type. Keep
these calls worker-backed for future TUI use, parse Jira-provided required/allowed/schema data, and
avoid hard-coded presets.

### Create Metadata Discovery Review

- Added typed `CreateIssueType` and `CreateField` domain structs in `internal/jira`.
- Added `GetCreateIssueTypes(projectKey)` using Jira create metadata issue-type mappings.
- Added `GetCreateFields(projectKey, issueTypeID)` using Jira create metadata field mappings.
- Parsed required/default flags, schema type/system/items/custom data, operations, allowed values,
  and autocomplete URLs for future create-form rendering.
- Added worker request/result support for project issue types and selected issue-type fields so the
  future TUI create flow can stay backgrounded.
- Verified so far with `go test ./internal/jira -run 'TestGetCreate' -count=1`,
  `go test ./internal/worker -run 'TestPoolGetCreate|TestPoolReturnsInvalidSummaryEditRequestResults' -count=1`,
  and `go test ./...`.
- Final verification passed with `make check` and `make install-user`.

## Diagnostics Overlay Polish

- [x] Add TUI tests for diagnostics summary counts and activity bars.
- [x] Add TUI tests that diagnostics rows include scan-friendly operation labels.
- [x] Implement a compact diagnostics summary strip without adding Jira IO or render-blocking work.
- [x] Update docs, changelog, and task review.
- [x] Run focused tests, `make check`, and `make install-user`.

### Diagnostics Overlay Polish Scope

Polish the existing read-only Diagnostics overlay so background activity scans faster. Add a compact
summary strip, simple activity bars, and clearer row detail labels. Do not add a timer, persistence,
new Jira requests, or animated state that is not tied to recorded worker/cache events.

### Diagnostics Overlay Polish Review

- Added a compact summary strip with worker/cache/error/active counts and the last recorded event.
- Added simple worker/cache activity bars computed from the retained diagnostics buffer.
- Added operation labels to event details so rows read as `get_issue #7 ABC-1` or
  `issue_detail ABC-1` instead of bare IDs.
- Kept the overlay read-only and render-local: no timers, persistence, new Jira calls, or background
  work were added for this visual polish.
- Verified so far with `go test ./internal/tui -run 'TestDiagnostics' -count=1` and
  `go test ./internal/tui -count=1`.
- Final verification passed with `make check` and `make install-user`.

## Diagnostics Overlay

- [x] Add TUI tests that the diagnostics overlay toggles without changing the active workflow.
- [x] Add TUI tests that worker submit/result activity is recorded in a bounded in-memory buffer.
- [x] Add TUI tests that detail-cache hit/miss/stale-refresh decisions are recorded.
- [x] Implement the read-only diagnostics overlay with `ctrl+d` and `esc` close behavior.
- [x] Wire diagnostics events to existing worker messages and detail TTL cache decisions.
- [x] Update docs, changelog, and lessons.
- [x] Run focused tests, `make check`, and `make install-user`.

### Diagnostics Overlay Scope

First slice: add a toggleable, read-only overlay for recent background activity. It should show
worker submit/result events and detail-cache hit/miss/stale refresh decisions from an in-memory
bounded buffer. It must not make Jira calls, block rendering, persist logs, or interfere with
active text-editing flows.

### Diagnostics Overlay Review

- Added a bounded in-memory diagnostics event buffer to the TUI model.
- Added `ctrl+d` to open a read-only Diagnostics overlay and `esc`/`ctrl+d` to close it.
- Recorded worker submit/result events from the existing worker message path.
- Recorded issue-detail cache hit, miss, and stale-refresh decisions from the existing TTL cache
  decision path.
- Verified so far with `go test ./internal/tui -run 'TestDiagnostics' -count=1` and
  `go test ./internal/tui -count=1`.
- Final verification passed with `make check` and `make install-user`.

## Detail TTL Cache Refresh

- [x] Add TUI tests that fresh cached issue detail avoids duplicate Jira detail requests.
- [x] Add TUI tests that stale cached issue detail stays visible while a background refresh starts.
- [x] Add TUI tests that loaded detail marks the cache entry fresh.
- [x] Implement issue-detail freshness tracking with `github.com/jellydator/ttlcache/v3`.
- [x] Keep stale detail display non-blocking and refresh only through the worker pool.
- [x] Update docs, changelog, and lessons.
- [x] Run focused tests, `make check`, and `make install-user`.

### Detail TTL Cache Scope

First slice: keep the existing in-memory issue detail map as the stale display cache, and add a
short-lived TTL freshness marker using `ttlcache`. Fresh detail avoids duplicate Jira reads. Stale
detail remains visible immediately, but the worker pool refreshes it in the background. This keeps
the TUI responsive while making detail data less permanently stale.

### Detail TTL Cache Review

- Added issue-detail freshness tracking with `github.com/jellydator/ttlcache/v3`.
- Kept the existing detail map as the stale display cache so already-loaded details remain visible.
- Fresh details skip duplicate Jira detail requests.
- Stale details start a worker-backed background refresh without blocking the detail view.
- Loaded detail responses mark the selected issue detail fresh for a short TTL.
- Verified with `go test ./internal/tui -run 'Test(FreshCachedDetailSkipsDetailRefresh|StaleCachedDetailStartsBackgroundRefresh|LoadedDetailMarksDetailCacheFresh)' -count=1`,
  `make check`, and `make install-user`.

## Assignee Edit Workflow

- [x] Add Jira client tests for assigning an issue by Jira account ID.
- [x] Implement minimal Jira assignee update support through `Issue.Assign`.
- [x] Add worker request/result tests for assignee updates.
- [x] Implement background worker assignee update flow.
- [x] Add TUI tests for Assignee as a focusable detail field.
- [x] Add TUI tests for opening a Jira-user picker and applying the selected assignee.
- [x] Implement the Assignee picker without relying on the Actions menu.
- [x] Update docs, changelog, and lessons.
- [x] Run focused tests, `make check`, and `make install-user`.

### Assignee Edit Scope

First slice: make Assignee a first-class ticket-detail focus target after Summary and before
Priority. `tab`/`shift+tab` moves to it, and `enter` opens a modal picker with type-to-filter Jira
user search. Typing characters such as `jon` updates the query and refreshes Jira user results
through the worker pool. Selecting a user submits an assignee update by Jira account ID through the
worker pool and updates the visible issue row plus cached detail assignee on success.

This intentionally does not build the full generic edit-fields framework yet. It proves the next
metadata-backed field shape with the smallest useful workflow: a user picker populated from Jira,
keyboard selection, submit, cancel, and worker result handling.

### Assignee Edit Review

- Added Jira client support for assigning an issue by Jira account ID through `Issue.Assign`.
- Added worker request/result support for assignee updates so assignment stays backgrounded.
- Added Assignee as a first-class ticket-detail focus target between Summary and Priority.
- Added a `Change Assignee` modal with type-to-filter Jira user search, arrow-key selection,
  worker-submitted assignment, and cancel behavior.
- Added a short-lived in-memory TTL cache for Jira user-search results using
  `github.com/jellydator/ttlcache/v3`, with case-insensitive query keys.
- Deferred broader background cache refresh tooling to the next cache-framework slice: use
  `ttlcache` eviction/loader hooks where they fit, and keep ticket refresh async/merge-based rather
  than blocking the UI.
- Verified with `go test ./internal/jira ./internal/worker ./internal/tui -run 'Test(UpdateAssignee|PoolUpdateAssignee|PoolReturnsInvalidSummaryEditRequestResults|DetailTabFocusesEditableFieldsBeforeSections|DetailEnterOnFocusedAssigneeOpensTypeaheadPicker|AssigneePickerSubmitsSelectedUser|AssigneePickerUsesCachedUserSearch|AssigneeUpdateSuccessUpdatesIssueAndDetailAssignee)' -count=1`,
  `make check`, and `make install-user`.

## Detail Field Focus Workflow

- [x] Add TUI tests for tabbing through Summary and Priority fields before section tabs.
- [x] Add TUI tests that `enter` on focused Summary opens the Summary editor modal.
- [x] Add TUI tests that `enter` on focused Priority opens the Priority picker modal.
- [x] Implement unified detail focus targets for header fields plus existing sections.
- [x] Keep `s` and `p` as temporary accelerators that use the same activation path.
- [x] Update docs, changelog, and lessons.
- [x] Run focused tests, `make check`, and `make install-user`.

### Detail Field Focus Scope

First slice: make Summary and Priority first-class detail focus targets before the existing
Description/Hierarchy/Comments/Actions/Status section targets. `tab` and `shift+tab` move through
fields and sections; `enter` activates the focused target. Text fields still open editor modals,
set-value fields still open picker modals, and existing single-letter shortcuts remain aliases
rather than the primary workflow.

### Detail Field Focus Review

- Added Summary and Priority as first-class ticket-detail focus targets before the section tabs.
- `tab` and `shift+tab` now move through editable fields and sections; `enter` activates the
  focused target.
- `s` and `p` remain accelerators into the same Summary and Priority modal flows.
- Status activation now follows the same direct-edit rule: selecting Status and pressing `enter`
  starts the Jira-backed transition picker/load instead of requiring an intermediate focus step.
- Verified with `go test ./internal/tui -count=1`, `make check`, and `make install-user`.

## Priority Edit Workflow

- [x] Add Jira client tests for parsing priority edit metadata and allowed values.
- [x] Add Jira client tests for updating the selected issue priority.
- [x] Implement minimal Jira priority metadata and update support.
- [x] Add worker request/result tests for priority update.
- [x] Implement background worker priority update flow.
- [x] Add TUI tests for direct Priority picker modal editing.
- [x] Implement the Priority picker without relying on the Actions menu.
- [x] Update docs and changelog.
- [x] Run focused tests, `make check`, and `make install-user`.

### Priority Edit Scope

First slice: pressing `p` in ticket detail starts a metadata-backed Priority edit flow for the
selected issue. The TUI reuses cached edit metadata or loads it through the worker pool; if Jira
reports priority as editable and supplies allowed values, the modal renders a picker/dropdown-style
list. `j`/`k` or arrow keys select the new priority, `enter` submits the selected value through the
worker pool, and `esc` cancels.

This follows the modal split: Priority is an enumerated field, so it uses a picker rather than a
text editor. Jira allowed values must come from edit metadata; do not hard-code a preset list.

### Priority Edit Review

- Extended Jira edit metadata parsing with Priority and Jira-provided allowed values.
- Added Jira and worker support for priority updates using Jira priority ID/name values.
- Added `p` as the direct Priority edit shortcut in ticket detail; previous-section movement remains
  available through `shift+tab` and `[`.
- Added a `Change Priority` picker modal with current priority, Jira-loaded options, selection
  movement, apply, and cancel behavior.
- Successful priority updates refresh the visible issue row and cached detail priority immediately.
- Verified with `go test ./internal/jira ./internal/worker ./internal/tui -run 'Test(GetEditMetadataParsesPriorityAllowedValues|UpdatePriority|PoolUpdatePriority|PoolReturnsInvalidSummaryEditRequestResults|Priority)' -count=1`,
  `go test ./internal/tui -count=1`, `make check`, and `make install-user`.

## Detail Overlay Dialogs

- [x] Add failing TUI tests for Summary edit rendering as a centered overlay dialog.
- [x] Add failing TUI tests for Status transition rendering as the same overlay dialog pattern.
- [x] Implement a small reusable detail overlay renderer in `internal/tui/model.go`.
- [x] Move Summary edit draft rendering out of the header row and into the overlay.
- [x] Move Status transition picker rendering out of the Status section body and into the overlay.
- [x] Update docs and changelog.
- [x] Run focused tests, `make check`, and `make install-user`.

### Detail Overlay Scope

First slice: keep the current detail screen as the background and render focused edit/selection
workflows in a layered dialog. Summary edit owns the draft text, loading/saving/error notice, and
`enter`/`esc` hints inside the overlay. Status transitions reuse the same shell for Jira-loaded
transition choices and preserve `j`/`k`, `enter`, and `esc` behavior.

This is a rendering and interaction cleanup only. Jira metadata, summary updates, transition loads,
and transition submissions stay worker-backed and unchanged.

### Detail Overlay Review

- Added a shared ticket-detail dialog shell for focused mutation workflows.
- Summary editing now keeps the saved summary in the detail header and renders the unsaved draft in
  an `Edit Summary` dialog with explicit save/cancel hints.
- Summary focus no longer renders an instructional Notice block before the dialog opens; the footer
  owns the `enter`/`esc` guidance in that intermediate state.
- Long Summary drafts now render as an input-style line biased toward the edit cursor so typing at
  the end of a long value is visible.
- Pressing `s` now starts the metadata-backed Summary edit flow immediately and renders loading/edit
  states in the modal shell instead of stopping in a footer-only focus state.
- Text-backed modal fields use editor surfaces; enumerated modal fields keep picker/list selection.
- Status transition selection now renders in a `Change Status` dialog with current status,
  Jira-loaded transition choices, selection cursor, and apply/cancel hints.
- Verified with `go test ./internal/tui -run 'Test(SummaryEditorRendersAsOverlayDialog|StatusTransitionPickerRendersAsOverlayDialog)' -count=1`,
  `make check`, and `make install-user`.

## Summary Edit Workflow

- [x] Add Jira client tests for fetching edit metadata for the selected issue.
- [x] Add Jira client tests for updating the issue summary.
- [x] Implement minimal Jira edit metadata and summary update client methods.
- [x] Add worker request/result tests for metadata loading and summary update.
- [x] Implement background worker edit metadata and summary update flows.
- [x] Add TUI tests for direct Summary section editing.
- [x] Implement Summary section editor without relying on the Actions menu.
- [x] Update docs and changelog.
- [x] Run focused tests, `make check`, and `make install-user`.

### Summary Edit Review

- Added Jira client methods and tests for issue edit metadata and summary updates.
- Added worker request/result handlers so metadata loading and summary updates stay backgrounded.
- Added direct Summary field focus in ticket detail with `s`; focused `enter` loads metadata and
  opens a prefilled one-line editor, then `enter` submits the update.
- Fixed duplicate/queued `enter` handling so Summary drafts cannot submit until the user actually
  edits the opened draft.
- Successful summary updates refresh the visible issue row and cached detail summary immediately.
- Verified with `go test ./internal/jira ./internal/worker ./internal/tui -count=1`,
  `make check`, and `make install-user`.

### Summary Edit Scope

First slice: add a direct Summary section in ticket detail. Selecting Summary and pressing `enter`
loads Jira edit metadata through the worker pool; if Jira reports `summary` as editable, the TUI
opens a small prefilled editor. Submitting updates only the summary through Jira, then updates the
visible issue row and cached detail.

The first slice intentionally edits only Summary. Priority, assignee, labels, and custom fields
should reuse this metadata/edit framework in later slices instead of sharing a generic Actions menu.

## Status Transition Workflow

- [x] Add Jira client tests for listing available issue transitions.
- [x] Add Jira client tests for applying a selected transition.
- [x] Implement minimal Jira transition client methods with local DTOs.
- [x] Add worker request/result tests for transition loading and submission.
- [x] Implement background worker transition flows.
- [x] Add TUI tests for direct status-field transition workflow.
- [x] Implement the status transition picker without relying on the Actions menu.
- [x] Update docs and changelog.
- [x] Run focused tests, `make check`, and `make install-user`.

### Status Transition Review

- Added Jira client methods and tests for listing and applying issue transitions.
- Added worker request/result handlers so transition loading and submission stay backgrounded.
- Added a direct ticket detail Status section with a Jira-populated transition picker; `enter`
  loads transitions, `j`/`k` selects, and focused `enter` applies the transition.
- Successful transitions update the visible issue row and cached detail status immediately.
- Verified with `go test ./internal/jira ./internal/worker ./internal/tui -count=1`,
  `make check`, and `make install-user`.

### Status Transition Scope

First slice: from a focused ticket detail status field/section, `enter` loads available Jira
transitions in the background, renders a small picker populated from Jira, and submits the selected
transition through the worker pool. On success, the visible issue status is updated or refreshed.

The pinned go-atlassian client can list and apply transitions, but does not expose
`expand=transitions.fields`; transition-screen field metadata remains a follow-up lower-level Jira
request rather than a hard-coded preset list.

## Detail Footer Section Commands

- [x] Confirm UX scope for selected-but-not-activated section footer commands.
- [x] Add failing footer tests for Hierarchy, Links, and Actions selected sections.
- [x] Add section-aware footer bindings while preserving active sub-mode footers.
- [x] Update docs and changelog.
- [x] Run focused tests, `make check`, and `make install-user`.

## Description And Comments UX Rhythm

- [x] Confirm ordered UX scope: comments, description, then states.
- [x] Add failing tests for lighter comment headers and body spacing.
- [x] Improve comment block rendering.
- [x] Add failing tests for description section spacing around rich content.
- [x] Improve description rendering rhythm.
- [x] Add failing tests for aligned detail state blocks.
- [x] Align description/comment loading, empty, and error states.
- [x] Update docs and changelog.
- [x] Run focused tests, `make check`, and `make install-user`.

## Hierarchy Detail Polish

- [x] Explore project context and backlog.
- [x] Confirm hierarchy detail design direction.
- [x] Write implementation plan after design approval.
- [x] Add failing TUI tests for grouped hierarchy rendering and selection.
- [x] Implement grouped hierarchy rows in `internal/tui/model.go`.
- [x] Keep hierarchy activation/back-stack behavior working across grouped rows.
- [x] Run focused TUI tests.
- [x] Update docs and changelog for the hierarchy tab behavior.
- [x] Run `make check`.

## Review

- Implemented grouped Hierarchy tab rendering for Path, Children, Subtasks, and a linked-issues placeholder.
- Added focused TUI coverage for grouped hierarchy rendering and opening a selected grouped subtask.
- Verified with `go test ./internal/tui -count=1`, `make check`, and `make install-user`.

## Implementation Plan

### Scope

Improve the focused ticket detail Hierarchy tab so it renders a grouped tree workspace:

- `Path` section for known parent context.
- `Children` section for visible non-subtask children.
- `Subtasks` section for visible subtasks.
- `Linked Issues` placeholder section only, because linked issue data is not currently exposed through the Jira client.

No new Jira API calls are part of this slice. Existing table expansion through `x` and `X` remains table-mode behavior.

### Files

- Modify `internal/tui/model.go`
  - Add a small internal hierarchy row model.
  - Split visible related issues into children and subtasks.
  - Render grouped sections with the existing section header, empty-state, and detail table helpers.
  - Map `selectedHierarchy` to selectable issue rows across groups.
- Modify `internal/tui/model_test.go`
  - Add RED tests for grouped Path/Children/Subtasks rendering.
  - Add RED tests that `enter` opens the selected grouped subtask/child and preserves the detail back stack.
- Modify docs after code is green:
  - `docs/project-state.md`
  - `docs/backlog.md`
  - `docs/releases/CHANGELOG.md`

### TDD Steps

- [ ] Write `TestHierarchySectionRendersGroupedTree`.
  - Build a detail-mode model with selected parent `ABC-1`, one child story, and one subtask.
  - Assert the rendered hierarchy contains `Path`, `Children 1`, `Subtasks 1`, the child key, and the subtask key.
  - Assert it does not use the old flat empty-state wording when grouped rows exist.
- [ ] Run `go test ./internal/tui -run TestHierarchySectionRendersGroupedTree -count=1`.
  - Expected: FAIL because the current renderer has no grouped section labels/counts.
- [ ] Implement the smallest grouped renderer in `internal/tui/model.go`.
  - Add `hierarchyRow` and grouping helpers near the hierarchy renderer.
  - Render parent path first, then child and subtask tables.
  - Keep selectable rows flat internally so existing `selectedHierarchy` behavior stays simple.
- [ ] Run `go test ./internal/tui -run TestHierarchySectionRendersGroupedTree -count=1`.
  - Expected: PASS.
- [ ] Write `TestHierarchyEnterOpensSelectedGroupedSubtask`.
  - Focus Hierarchy, select the subtask row after the child row, press `enter`.
  - Assert selected issue changes to the subtask and previous selection is pushed to the back stack.
- [ ] Run `go test ./internal/tui -run TestHierarchyEnterOpensSelectedGroupedSubtask -count=1`.
  - Expected: PASS if row mapping was implemented cleanly; otherwise fix mapping only.
- [ ] Run `go test ./internal/tui -count=1`.
- [ ] Update docs and changelog.
- [ ] Run `make check`.
