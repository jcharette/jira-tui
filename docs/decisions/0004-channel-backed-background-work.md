# 0004: Use Channel-Backed Background Work

Date: 2026-06-06

## Status

Accepted

## Context

Go was chosen partly because goroutines and channels are a good fit for a responsive terminal app
that will eventually manage many concurrent Jira workflows: issue lists, details, comments,
transitions, sprint data, epics, subtasks, and write actions.

Bubble Tea gives us an event loop and command model, but we still want an explicit Go concurrency
methodology so slow Jira work does not leak into rendering or input handling.

## Decision

Run Jira IO through channel-backed background workers at the TUI boundary.

The current issue refresh path uses the typed dispatcher and bounded worker pool powered by
`ants`. Future data fetches should follow the same general shape:

- UI state starts work by creating a request with a request ID.
- The `ants`-backed worker pool performs bounded Jira IO.
- The worker sends one typed result message through a channel.
- The Bubble Tea update loop applies only current request IDs and ignores stale results.
- Existing data remains visible when background refreshes fail.

The dispatcher should stay intentionally small and wrap the third-party worker engine:

- `Request` carries ID, kind, timeout/correlation metadata, and a typed payload.
- `Result` carries ID, kind, typed payload, and error.
- Request queues are bounded.
- Worker count is configurable but has conservative defaults.
- If a queue is full, the caller gets an immediate backpressure result rather than spawning
  unlimited goroutines.
- UI code never receives SDK structs directly; worker results map back to application domain types.
  UI code also does not depend on `ants` directly.

The first worker pool supports issue search. Add issue detail next, then comments, transitions,
sprints, boards, epics, and subtasks after the pattern proves itself across multiple workflows.

## Consequences

- Rendering remains pure and derived from model state.
- Jira IO can scale into multiple independent workers without rewriting the UI contract.
- Future worker pools should use bounded queues and explicit backpressure when concurrency grows.
- We accept a small amount of upfront structure to avoid repeatedly rewriting concurrency plumbing.
- We should not build a broad framework before there are multiple workflows using it.
- Write actions should use the same request/result pattern, with explicit confirmation and failure
  states.
