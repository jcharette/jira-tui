# 0005: Prefer Maintained Libraries For Non-Core Infrastructure

Date: 2026-06-06

## Status

Accepted

## Context

The goal is to build a useful Jira terminal client, not to minimize binary size or hand-roll common
infrastructure. We want to spend effort on Jira workflows, terminal UX, performance, and reliability.

Repeatedly rebuilding common primitives wastes time and increases maintenance risk.

## Decision

Before implementing non-core infrastructure, check for well-maintained third-party libraries.

Prefer libraries when they:

- Solve a real problem that is not unique to Jira TUI.
- Are actively maintained.
- Have clear APIs and tests.
- Can be wrapped behind an internal boundary.
- Do not leak broad dependency-specific types into app/domain/UI code.

Keep app-specific contracts inside this repo. For example, `ants` can power worker execution, but
`internal/worker` still owns Jira TUI request/result types and Bubble Tea integration.

## Consequences

- We should not default to hand-rolled implementations for mature infrastructure problems.
- Dependency decisions should be recorded when they shape architecture.
- Useful product behavior matters more than making the smallest possible dependency graph.
- Internal wrappers remain important so we can change libraries later without rewriting the UI.

