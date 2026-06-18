# Config Profiles And Default Queries Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make saved TOML profiles durable and selectable with `--profile` while preserving existing default query behavior.

**Architecture:** Extend `internal/config` to retain profile metadata, thread a profile override through `internal/app`, and keep the config editor scoped to the selected active profile. No Jira worker paths change.

**Tech Stack:** Go, BurntSushi TOML, Cobra, Bubble Tea config UI.

## Global Constraints

- Keep the slice bounded to config profile persistence and CLI selection.
- Preserve backward compatibility for existing single-profile config files.
- Do not add a main-TUI profile switcher or per-profile saved views in this slice.
- Write failing tests before production changes.
- Verify with focused tests, `go test ./... -count=1`, `make check`, and `make install-user`.

---

### Task 1: Config Profile Persistence

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Interfaces:**
- Consumes: existing `Load`, `LoadEditable`, `Save`, `Config`, and `LoadOptions`.
- Produces: `type Profile`, `Config.ActiveProfile`, `Config.Profiles`, `LoadOptions.Profile`.

- [x] Write failing config tests for loading `--profile` overrides and preserving multiple profiles.
- [x] Run focused config tests and confirm they fail because profile fields/options are missing.
- [x] Add `Profile`, `ActiveProfile`, `Profiles`, and `LoadOptions.Profile`.
- [x] Make `applyFile` select the requested profile or saved active profile, normalize credentials,
  and retain all loaded profiles.
- [x] Make `Save` preserve all profiles and update the active profile credentials.
- [x] Run focused config tests and confirm they pass.

### Task 2: CLI Profile Selection

**Files:**
- Modify: `internal/app/app.go`
- Modify: `internal/app/app_test.go`

**Interfaces:**
- Consumes: `config.LoadOptions{Profile: profileName}`.
- Produces: root persistent `--profile` flag used by app startup and config editing.

- [x] Write failing app command tests for accepting `--profile` on root and config commands.
- [x] Thread profile selection through `runApp`, `runConfig`, and saved-view writer path.
- [x] Include active profile in the active-view cache namespace.
- [x] Run focused app tests and confirm they pass.

### Task 3: Config UI Active Profile Field

**Files:**
- Modify: `internal/configui/model.go`
- Modify: `internal/configui/model_test.go`

**Interfaces:**
- Consumes: `config.Config.ActiveProfile`.
- Produces: config editor field `Active Profile` that round-trips into `config.Config`.

- [x] Write failing config UI test for active profile field round-trip.
- [x] Add `Active Profile` to the account section and populate `Config.ActiveProfile` from fields.
- [x] Preserve default view/query generation behavior after config edits.
- [x] Run focused config UI tests and confirm they pass.

### Task 4: Docs And Backlog

**Files:**
- Modify: `docs/backlog.md`
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

**Interfaces:**
- Consumes: completed behavior from Tasks 1-3.
- Produces: updated backlog state and task review notes.

- [x] Mark the saved profiles/default queries backlog item complete.
- [x] Update project state with CLI profile selection and profile preservation behavior.
- [x] Add an Unreleased changelog entry.
- [x] Add task review notes and verification evidence.

### Task 5: Verification And Integration

**Files:**
- Modify: `docs/superpowers/plans/2026-06-18-config-profiles-queries.md`

**Interfaces:**
- Consumes: completed Tasks 1-4.
- Produces: pushed `main` with green local and remote checks.

- [x] Run focused config/app/configui tests.
- [x] Run `go test ./... -count=1`.
- [x] Run `make check`.
- [x] Run `make install-user`.
- [ ] Commit, fast-forward merge to `main`, push, watch CI, and clean up the worktree.
