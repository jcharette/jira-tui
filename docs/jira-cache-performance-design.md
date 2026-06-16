# Jira Cache And Background Refresh Design

Date: 2026-06-16

## Goal

Large Jira views should become responsive without showing silently stale data. The app should be
able to render useful local results quickly, refresh Jira data in the background, show freshness
state, and always prioritize explicit user actions over prefetch or periodic sync work.

## Current State

- Jira IO already runs through `internal/worker.Pool` with a bounded `ants` pool, a bounded
  admission queue, request timeouts, typed requests, and typed results.
- The TUI periodically refreshes the active JQL view. Refresh keeps the UI interactive but still
  waits for a full Jira search before replacing the active issue list.
- Search results are not cached as a reusable read model. A large active JQL view therefore pays the
  Jira search/enrichment cost at startup, view changes, and refresh.
- Issue details have an in-memory freshness TTL, but only as a key marker around `m.details`.
  Comments, transitions, edit metadata, create metadata, and expanded children are map-backed but do
  not share a consistent freshness model.
- User search already uses `ttlcache`.
- Worker search enrichment currently performs the primary JQL search, missing-parent enrichment,
  known-subtask merge, and child lookup before returning one result. That is convenient, but it makes
  initial large-view load wait for all enrichment.

## Principles

- Foreground actions win. Opening a ticket, posting a comment, applying a transition, editing
  fields, changing views, and explicit refresh must outrank background sync or prefetch.
- Cached data must be labeled. The UI should expose `synced`, `refreshing`, `stale`, and `failed`
  states with timestamps where useful.
- Writes invalidate or patch immediately. A successful write updates visible state and marks
  affected cached records dirty until Jira confirms a fresh read.
- Background work should coalesce. Multiple refresh requests for the same view/key should collapse
  into one in-flight job.
- Keep cache storage app-local and private. If persistent cache is added, store it under the user's
  cache directory, not the config file, and do not store tokens or raw request bodies.
- Prefer maintained primitives. Keep `ants` for bounded workers unless it becomes a bad fit. Use
  `ttlcache` for TTL data. For priority scheduling, evaluate a maintained priority queue before
  adding a custom heap; if the standard library `container/heap` is chosen, wrap it behind a small
  tested scheduler boundary.

## Read Model

Add a cache/read-model layer behind the TUI and in front of worker submission. The first shape can be
in-memory only:

| Data | Cache Key | Suggested Fresh TTL | Notes |
| --- | --- | --- | --- |
| Active JQL issue list | normalized JQL + max results + enrichment flags | 30-90 seconds | Render immediately on hit, refresh in background when stale. |
| Issue detail | issue key | 2-5 minutes | Existing map + freshness marker can move behind the same cache record model. |
| Comments | issue key + max results | 30-90 seconds | Invalidate after add/edit/delete comment. |
| User search | normalized query | 5-15 minutes | Existing `ttlcache` path is fine. |
| Transitions | issue key | 30-90 seconds | Invalidate after transition. |
| Edit metadata | issue key | 5-15 minutes | Usually stable but permission/status can change; stale refresh is acceptable. |
| Create metadata | project + issue type | 15-60 minutes | Stable enough to cache longer; refresh on project/issue-type change if missing. |
| Expanded children | parent key + mode | 30-90 seconds | Invalidate when parent/subtask creation lands. |

Each cache record should include:

- `Value`
- `SyncedAt`
- `ExpiresAt`
- `RefreshStartedAt`
- `Err`
- `Dirty` or `InvalidatedAt`
- `InFlightRequestID`

## Freshness Behavior

For foreground reads:

1. If a fresh record exists, render it immediately and skip Jira.
2. If a stale record exists, render it immediately with a visible stale/refreshing label, then enqueue
   a foreground refresh unless an equivalent request is already in flight.
3. If no record exists, show the current loading state and enqueue a foreground request.
4. If refresh fails and a stale record exists, keep stale data visible and surface the failure in the
   header/status/diagnostics instead of clearing the view.

For background reads:

1. Run only when the equivalent foreground job is not queued or running.
2. Do not replace a user's active selection or scroll position unless the user is still on the same
   view/key.
3. Merge refreshed issue rows by key so active selection survives reordering and enrichment.
4. Record cache hit/miss/stale/refresh/failure diagnostics.

## Priority Classes

Introduce request priority at the scheduler boundary, not inside individual TUI screens.

| Priority | Examples |
| --- | --- |
| P0 write | add comment, transition issue, update summary/description/priority/assignee, create issue |
| P1 foreground read | active view search, selected issue detail, selected comments, metadata needed for a visible editor |
| P2 explicit refresh | user pressed refresh, active view stale-while-refresh |
| P3 foreground prefetch | selected issue adjacent detail/comments, expanded visible parent children |
| P4 background sync | periodic active view refresh, stale cache refresh, saved-view warming |

Queue rules:

- P0 and P1 should bypass or outrank background jobs.
- Coalesce duplicate reads by cache key and request kind.
- Drop background jobs when the queue is full before rejecting foreground jobs.
- Keep write requests non-coalesced.
- Preserve stale-response guards already used by active request IDs.

## Persistent Cache Option

Start in-memory. Add persistent storage only after the in-memory model proves useful.

If persistent cache is added:

- Store under `os.UserCacheDir()/jira/`.
- Use a small embedded store with clear ownership, such as SQLite via a maintained driver, if we need
  indexed issue rows or cross-session saved-view warming.
- Store normalized issue/detail/comment records and sync metadata, not credentials.
- Version the schema and tolerate cache deletion at any time.
- Use an app-level cache namespace including Jira base URL and account identity hash so separate Jira
  sites do not mix data.

## Diagnostics And UI

Expose enough state to make freshness understandable:

- Header: active view freshness label such as `synced 10:42`, `refreshing`, `stale 4m`, or
  `refresh failed`.
- Diagnostics overlay: queue depth by priority, active jobs, cache hits/misses/stale hits, refresh
  failures, last sync per active view/key, and dropped background jobs.
- Detail sections: comments/detail metadata should show stale or refreshing states locally when
  cached data is visible during refresh.

## Implementation Plan

### Slice 1: In-memory active view cache

- Add a small cache record type for active JQL search results.
- Use normalized JQL + max results as the cache key.
- On initial load/view switch/refresh, render a cached stale result immediately when available, then
  submit a Jira refresh.
- Track view freshness state and show it in the header or filter summary.
- Add tests for fresh hit, stale hit with background refresh, failed refresh preserving stale data,
  and selection preservation after refreshed rows arrive.

Status: implemented in `internal/tui/view_cache.go` using `ttlcache` for retained active-view
records. Freshness is visible in the header, stale cached rows render before refresh, failed
refreshes preserve stale rows, and explicit user refresh still submits a foreground Jira refresh
even when cached data is fresh.

### Slice 2: Scheduler priority and coalescing

- Add priority metadata to worker requests or introduce a scheduler in front of `worker.Pool`.
- Coalesce duplicate read jobs by kind/cache key.
- Reject/drop background jobs before foreground jobs when full.
- Add diagnostics for queue depth and dropped/coalesced jobs.

Status: implemented in `internal/worker.Pool` as an admission scheduler around the existing
maintained `ants` execution pool. Worker requests now carry priority and coalesce-key metadata;
duplicate reads fan out cloned results, and queued lower-priority work can be dropped before
foreground work is rejected.

Queue running, pending, coalesced, and capacity counts are exposed through a scheduler stats
snapshot and rendered in Diagnostics. Dropped-job counters and per-priority depth remain future
diagnostics enhancements if the compact snapshot is not enough in practice.

### Slice 3: Detail/comment metadata cache unification and diagnostics

- Move detail freshness, comments, transitions, edit metadata, create metadata, and expanded
  children behind the same cache record semantics.
- Patch or invalidate affected records after writes.
- Add stale-while-refresh UI labels for detail/comment sections.
- Add diagnostics for queue depth, coalesced reads, dropped background jobs, and cache refresh
  failures.

Status: partially implemented for issue detail and comments. Detail freshness and comments now use
retained cache records with value, sync time, and freshness boundary backed by `ttlcache`, while the
existing detail/comment maps continue to serve current rendering paths. Transitions, edit metadata,
create metadata, and expanded children remain pending.

### Slice 4: Optional persistent cache

- Decide whether startup and cross-session behavior justify disk cache.
- If yes, add a private versioned cache store under the user cache directory.
- Persist active view issue rows and selected detail/comment records with schema versioning and
  per-site namespacing.

Status: partially implemented for active Jira views. The app now opens a private SQLite cache under
the user's cache directory with `modernc.org/sqlite`, hydrates active view rows by Jira base URL and
normalized JQL, and writes successful active view searches back to disk. Detail/comment persistence,
metadata persistence, and cache cleanup policies remain pending.

## Non-Goals For The First Slice

- Do not add disk persistence.
- Do not rewrite `internal/worker.Pool`.
- Do not change Jira write semantics.
- Do not prefetch every saved view.
- Do not silently serve stale data without a freshness indicator.
