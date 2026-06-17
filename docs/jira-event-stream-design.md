# Jira Event Stream Design

## Goal

Add a background event stream that can carry both Jira domain events and internal app command
events. Start with active-view ticket `new` and `updated` events, plus enough structure to later
drive scheduled cache refreshes. The first consumers should be diagnostics and tests. Later
consumers can add macOS notifications, terminal bells, in-app dialogs, or internal refresh
workflows without changing Jira result handlers.

## Library Recommendation

Use Watermill with its GoChannel Pub/Sub for the first implementation.

- Watermill is actively maintained, has a stable v1 API, and is designed for event-driven Go
  applications.
- GoChannel is in-process and channel-backed, which fits the current TUI architecture.
- Watermill already has Publisher/Subscriber interfaces, message acknowledgement, routers,
  middleware, and optional future transports if we ever need durable or cross-process event
  delivery.
- Avoid lighter callback-style event bus packages for now. They are smaller, but they give us less
  structure for future notification fan-out, observability, and transport replacement.

Keep an app-specific adapter around Watermill so the rest of the codebase publishes typed Jira
events, not raw Watermill messages.

## Event Model

Create a small same-package or `internal/events` boundary depending on implementation size:

- `Event` contains ID, timestamp, source, type, dedupe key, and JSON payload.
- `TicketEvent` payload contains issue key, previous issue snapshot, current issue snapshot, changed
  fields, active view name/JQL, and sync timestamp.
- Event types start with:
  - `jira.ticket.new`
  - `jira.ticket.updated`
  - `jira.view.hydrated`
  - `jira.view.refresh.started`
  - `jira.view.refresh.completed`
  - `jira.view.refresh.failed`
  - `jira.cache.refresh.requested`
  - `jira.cache.refresh.completed`
  - `jira.cache.refresh.failed`
  - `ai.task.requested`
  - `ai.task.progress`
  - `ai.task.completed`
  - `ai.task.failed`

Separate event intent:

- Domain events describe something that happened, such as `jira.ticket.updated`.
- Command events request work, such as `jira.cache.refresh.requested`.

Command event payloads should include target type (`active_view`, `issue_detail`, `comments`,
`transitions`, `create_metadata`, `expanded_children`), cache key, priority, reason, and whether the
request may run silently in the background.

AI command payloads should include operation type (`ticket_plan`, `ticket_assist`, `create_draft`,
`refine_draft`, `code_review`, `implementation_plan`), preferred provider (`claude`, `codex`,
`auto`), issue key or draft ID, prompt/context references, priority, cancellation key, and whether
the result is allowed to update Jira or only return a draft.

Do not emit notifications or run refresh work directly from Jira result handlers. Result handlers
should publish domain events; consumers decide whether to show notifications, update diagnostics, or
schedule follow-up commands.

## Active View Cache Policy

Split active-view cache policy into two windows:

- Freshness window: keep the current short refresh cadence around 90 seconds.
- Display window: allow recently persisted rows, initially 24 hours, to render immediately on
  startup.

Startup behavior should be stale-while-revalidate:

- If SQLite has usable rows, render them immediately.
- If the rows are outside the freshness window, mark the view stale/refreshing and start a
  background Jira refresh.
- The background refresh must preserve selection and avoid blanking the issue list.
- When fresh Jira rows arrive, diff against the displayed rows and publish ticket events.

## Ticket Diff Rules

Diff active-view rows by issue key:

- If a key exists in refreshed Jira rows but not in the currently displayed rows, publish
  `jira.ticket.new`.
- If a key exists in both but display-relevant fields changed, publish `jira.ticket.updated`.
- Ignore pure ordering changes.
- Do not publish `new` events for the first cold load with no previously displayed or hydrated
  rows.
- Include enough previous/current data for future notification text without requiring another Jira
  lookup.

## First Implementation Slices

1. Add Watermill and a thin event stream adapter.
   - Keep the adapter small: `Publish(ctx, Event) error`, `Subscribe(ctx)`.
   - Use GoChannel in-process.
   - Add tests proving subscribers receive published events without blocking the TUI.

2. Add active-view stale-while-revalidate.
   - Add active-view display TTL, initially 24 hours.
   - Hydrate stale-but-usable SQLite rows on startup.
   - Submit startup refresh as background work when rows are already visible.
   - Add diagnostics for hydrate hit/stale/expired and refresh scheduling.

3. Publish ticket events from active-view refreshes.
   - Diff current visible rows against refreshed Jira rows before replacing them.
   - Publish `jira.ticket.new` and `jira.ticket.updated`.
   - Add diagnostics as the first event consumer.

4. Add notification consumers later.
   - macOS notification adapter.
   - TUI modal/toast adapter.
   - Configurable filters for projects, priorities, watched keys, or assigned-to-me.

5. Add scheduler publishers after the stream is proven.
   - A timer publishes `jira.cache.refresh.requested` for active views or watched cache keys.
   - A refresh consumer translates those command events into existing worker requests.
   - Foreground user actions still bypass or outrank background command events.
   - Diagnostics record command emission, worker admission, coalescing, and completion.

6. Add provider-agnostic AI workers after the Jira event path is stable.
   - Publish `ai.task.requested` instead of invoking Claude directly from TUI actions.
   - Start with the current Claude subprocess runner behind a provider interface.
   - Add Codex or other providers as separate workers when there is a clear task fit.
   - Use `preferred_provider=auto` for tasks where the app can route by capability, cost, latency,
     or user configuration.
   - Add persistent provider sessions only when the provider exposes a stable request/response
     protocol that supports cancellation, timeouts, progress, and clean recovery.
   - Keep bounded active AI jobs by default; queue or coalesce lower-priority AI requests so the TUI
     stays responsive.
   - Publish `ai.task.progress`, `ai.task.completed`, and `ai.task.failed` for diagnostics, UI
     progress, and future notifications.

## Testing

- Unit-test event publish/subscribe behavior with context cancellation.
- Unit-test active-view stale hydration from SQLite without a blocking initial load.
- Unit-test background refresh preserving visible rows and selection.
- Unit-test ticket diffing for new, updated, unchanged, and reordered issues.
- Unit-test diagnostics receiving event-stream messages.
- Unit-test timer/scheduler command events without real sleeping by injecting a clock or tick
  source.
- Unit-test AI command events with fake provider consumers before changing real provider runners.
- Run `go test ./internal/tui -count=1`, `go test ./... -count=1`, and `make check`.

## Open Decisions

- Whether the first event stream should live in `internal/tui` or a new `internal/events` package.
  Prefer `internal/events` if Watermill types would otherwise leak into TUI model code.
- Whether event history should stay in memory only, or later be persisted in SQLite for debugging.
- Whether notifications should be opt-in globally or per event type.
- Whether command events should be persisted later, or remain in-memory only to avoid replaying old
  refresh requests on startup.
- Which AI tasks should default to Claude, Codex, or `auto` provider routing.
- Whether persistent provider sessions are supported well enough to justify replacing
  subprocess-per-request. If not, keep subprocess execution behind the event-stream worker boundary.
