# 0001: Use Go And Bubble Tea

Date: 2026-06-06

## Status

Accepted

## Context

The project goal is a command-line Jira application for someone who strongly prefers terminal
workflows over web interfaces.

We briefly started with a Python scaffold, but the intended direction was Go with Bubble Tea.

## Decision

Use Go as the implementation language and Bubble Tea v2 as the TUI framework.

## Consequences

- The app can ship as a single native binary.
- The TUI can evolve into a rich terminal application without depending on Python packaging.
- Future work should preserve the Go/Bubble Tea direction unless there is a strong reason to revisit it.
- The abandoned Python scaffold should not be restored.

