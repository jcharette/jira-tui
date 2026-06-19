# Jira Start Workflow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `jira start [ticket]` and a selected-ticket TUI launch path that share one reviewed Start workflow for choosing a repo, previewing a branch, and applying confirmed writes.

**Architecture:** Keep command wiring in `internal/app`, local git/repo logic in a focused package, and shared workflow state/rendering in a small workflow package that can be run from CLI or launched from the TUI. Jira reads and writes use the existing `jira.Client` and worker-backed TUI paths where the TUI already owns a running worker pool. Production Git CLI calls must stay inside `internal/gitworkflow`; app, TUI, and workflow packages call that adapter and never shell out to Git directly.

**Tech Stack:** Go, Cobra, Bubble Tea v2, existing Jira client/worker abstractions, local git CLI through `os/exec`, TOML config for branch template defaults.

## Global Constraints

- Do not write to git, Jira, or config before an explicit review/confirm step.
- Cancellation before confirmation has no side effects.
- Store repo choices locally; do not write local repo mappings into Jira.
- Default branch template is `{key}-{summary_slug}` and must be user-editable before branch creation.
- Full Jira development-panel integration is out of scope.
- Use sanitized test data only, such as `PROJ-123`, never real company issue keys or summaries.
- Keep code boundaries focused; do not add unrelated workflows to existing large TUI files.

---

### Task 1: Git And Branch Planning Foundation

**Files:**
- Create: `internal/gitworkflow/git.go`
- Create: `internal/gitworkflow/git_test.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/configui/model.go`
- Modify: `internal/configui/model_test.go`

**Interfaces:**
- Produces `gitworkflow.RepoStatus`, `gitworkflow.Client`, `gitworkflow.NewCLIClient()`, `gitworkflow.RenderBranchName(template string, issue jira.Issue)`, and adapter methods for repo detection and branch create/switch.
- Produces config field `Config.Git.BranchTemplate` with default `{key}-{summary_slug}`.

- [ ] Add the `config.Git` struct and TOML load/save support for `git.branch_template`.
- [ ] Add Config UI field `Branch Template` under a new or existing workflow-oriented section.
- [ ] Implement branch slugging: lowercase summary, ASCII alphanumeric words, dashes, no leading/trailing dash, compact repeated separators.
- [ ] Implement current repo detection with `git rev-parse --show-toplevel`, current branch, and dirty state.
- [ ] Implement branch create/switch using `git switch -c <branch>` when missing and `git switch <branch>` when it already exists.
- [ ] Keep every production `exec.Command("git", ...)` call inside `internal/gitworkflow`; other packages depend on the adapter interface.
- [ ] Add focused tests for config defaults, config round trip, branch rendering, repo detection with a temp git repo, and existing-branch switch behavior.

### Task 2: Shared Start Workflow Model

**Files:**
- Create: `internal/startworkflow/model.go`
- Create: `internal/startworkflow/model_test.go`

**Interfaces:**
- Consumes `jira.Issue`, `config.Config`, and `gitworkflow.RepoStatus`.
- Produces `startworkflow.Model`, `startworkflow.Result`, and `startworkflow.Run(options Options) (Result, error)`.

- [ ] Add workflow steps: ticket, repo, branch, review, applying, done.
- [ ] If a ticket is supplied, start on repo selection; otherwise start on a ticket picker populated by caller-supplied issues.
- [ ] Let users edit repo path and branch name before review.
- [ ] Render review rows for branch create/switch, optional assign, optional transition, and optional comment.
- [ ] Support per-action skip toggles before confirmation.
- [ ] Return a result that describes the confirmed issue key, repo path, branch name, selected actions, and cancellation state.
- [ ] Add model tests for cancel-before-write, branch edit, action skip toggles, and confirm result.

### Task 3: CLI `jira start`

**Files:**
- Modify: `internal/app/app.go`
- Create: `internal/app/start.go`
- Modify: `internal/app/app_test.go`

**Interfaces:**
- Consumes `startworkflow.Run`.
- Produces Cobra command `jira start [ticket]`.

- [ ] Add `jira start [ticket]` to the root command and keep `--profile` inherited.
- [ ] Load config using the same profile path as the main app.
- [ ] With a ticket argument, fetch the issue detail or issue summary before opening the workflow.
- [ ] Without a ticket argument, search the active/default JQL and pass those issues to the focused picker.
- [ ] Detect the current repo and pass it as the preferred repo option.
- [ ] After workflow confirmation, execute the branch action first.
- [ ] Print a clear completed/skipped/failed summary when the workflow exits.
- [ ] Add command tests for registration, argument validation, and injected runner behavior without hitting real Jira or git.

### Task 4: Confirmed Jira Start Writes

**Files:**
- Modify: `internal/jira/client.go`
- Modify: `internal/jira/client_test.go`
- Modify: `internal/worker/pool.go`
- Modify: `internal/worker/pool_test.go`
- Modify: `internal/app/start.go`

**Interfaces:**
- Produces `jira.CurrentUser(ctx) (jira.User, error)` and worker support only if the TUI launch path needs async current-user lookup.
- Consumes existing `GetTransitions`, `TransitionIssue`, `UpdateAssignee`, and `AddComment`.

- [ ] Add Jira current-user lookup through go-atlassian `MySelf.Details`.
- [ ] Rank transitions for Start by available transition to a status containing `in progress`, `doing`, or `started`; skip when required unsupported fields are present.
- [ ] Assign to current user only when current-user lookup succeeds and the user did not skip assignment.
- [ ] Add a compact branch comment only when a branch action succeeded and the user did not skip comment.
- [ ] Apply each Jira write independently and record completed/skipped/failed actions.
- [ ] Add tests for transition ranking, current-user parsing, skipped writes, and partial failure summaries.

### Task 5: TUI Selected-Ticket Start Action

**Files:**
- Create: `internal/tui/start_workflow.go`
- Modify: `internal/tui/action_palette.go`
- Modify: `internal/tui/detail.go`
- Modify: `internal/tui/keymap.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/model_test.go`

**Interfaces:**
- Consumes shared Start workflow model and selected `jira.Issue`.
- Produces a Ticket Actions row named `Start Work`.

- [ ] Add a Ticket Actions row `Start Work` in the Workflow group.
- [ ] Launch the shared Start workflow with the selected ticket and preferred repo state.
- [ ] Render the workflow as a bounded modal using existing TUI dialog helpers.
- [ ] Keep all git/Jira writes behind the workflow review screen.
- [ ] Submit Jira writes through existing worker paths or a narrowly scoped new worker request; do not block Bubble Tea update/render.
- [ ] Refresh selected issue detail after successful Jira status, assignee, or comment writes.
- [ ] Add TUI tests for action discoverability, cancel-before-write, branch review rendering, and non-blocking write submission.

### Task 6: Docs, Verification, And GitHub Sync

**Files:**
- Modify: `README.md`
- Modify: `docs/backlog.md`
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

- [ ] Document `jira start [ticket]` and the in-app Start action.
- [ ] Update changelog under Unreleased.
- [ ] Update GitHub issue #4 with implementation notes and close it when verified.
- [ ] Run focused package tests after each slice.
- [ ] Run final `go test ./... -count=1`, `make check`, `make install-user`, and `git diff --check`.
- [ ] Commit and push the completed work.

## Self-Review

- Spec coverage: ADR 0009 Start requirements map to repo selection, configurable branch template, shared CLI/TUI workflow, review-before-writes, conservative Jira writes, local-only repo memory, cancellation, and partial summaries.
- Placeholder scan: no TBD/TODO placeholders remain.
- Type consistency: the plan names package-level interfaces that are created before consumers use them.
