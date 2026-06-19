# Task Plan

## 1.0.0 Initial Release Baseline

- [x] Consolidate the app into a clean initial release baseline.
- [x] Keep README, project docs, security overview, backlog, and changelog aligned with the current
  product state.
- [x] Publish `v1.0.0` as the first visible release after history reset.

### 1.0.0 Scope

This release is the clean baseline for the full Jira TUI app: Jira browsing, ticket detail,
metadata-backed Jira writes, Git/GitHub developer workflows, local Claude assistance, notifications,
cache/Diagnostics infrastructure, and the current security model.

### Backlog

- GitHub issue #6 remains the active public backlog item for provider-neutral AI workflow expansion.

## Show App Version In Header

- [x] Add a shared app version source.
- [x] Show the version in the main app header.
- [x] Show the version in the config header.
- [x] Verify focused header tests.
- [x] Prepare the `v1.0.1` release notes and install docs.

### Review

- Added `internal/version` as the single source for the visible app version.
- Main TUI header now shows the version with the right-side runtime metadata.
- Config UI header now shows the same version next to its status.
- Updated the config UI golden snapshot for the new versioned header.
- Verified with `go test ./internal/tui ./internal/configui -run 'TestHeaderUsesAvailableWidth' -count=1`.
- Verified full suite with `go test ./... -count=1`, `make check`, and `make install-user`.
