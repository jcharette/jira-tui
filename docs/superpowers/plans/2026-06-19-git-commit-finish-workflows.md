# Git Commit And Finish Workflows Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the CLI-first `jira commit` and `jira finish` workflows so ticket branches can be committed, reported to Jira, pushed, and turned into GitHub pull requests with reviewed Jira completion updates.

**Architecture:** Keep production Git shell calls inside `internal/gitworkflow`. Add local reported-commit state in a focused package, add GitHub PR operations behind a provider interface, and keep app command orchestration in `internal/app`. CLI workflows use explicit review/confirmation before any Git, Jira, or GitHub write.

**Tech Stack:** Go, Cobra, existing Jira client, Git CLI through `internal/gitworkflow`, GitHub CLI behind `internal/prprovider`, local JSON state under the user cache directory.

## Global Constraints

- Work happens on one feature branch: `feat/git-commit-finish-workflows`.
- Do not write to Git, Jira, GitHub, or local reported state before an explicit confirmation step.
- Production `git` commands stay in `internal/gitworkflow`; no scattered `exec.Command("git", ...)`.
- Production GitHub provider commands stay behind `internal/prprovider`; app code depends on an interface.
- Reported commit state is local-only and keyed by repo root, branch, issue key, and commit SHA.
- Jira notes stay compact and reviewable; no long generated narratives.
- AI assistance is deferred unless an existing provider-neutral hook can be reused without expanding scope.
- Test data must stay generic, such as `ABC-123` and `PROJ-456`.
- GitHub issues close only after merge to `main`, not after branch push.

---

### Task 1: Git Analysis And Local Commit State

**Files:**
- Modify: `internal/gitworkflow/git.go`
- Modify: `internal/gitworkflow/git_test.go`
- Create: `internal/gitstate/store.go`
- Create: `internal/gitstate/store_test.go`

**Interfaces:**
- Produces `gitworkflow.Analysis`, `gitworkflow.Commit`, `gitworkflow.ChangeSummary`.
- Produces Git client methods for branch analysis, commit creation, and push.
- Produces `gitstate.Store` with `ReportedCommits`, `MarkReported`, and `DefaultPath`.

- [ ] Add `Analysis{Repo RepoStatus, BaseBranch string, UpstreamBranch string, IssueKey string, Changes ChangeSummary, Commits []Commit}`.
- [ ] Add `ChangeSummary{Dirty bool, Files []ChangedFile}` and `ChangedFile{Path, Status string}`.
- [ ] Add `Commit{SHA, Subject, Body string}`.
- [ ] Extend `gitworkflow.Client` with `Analyze(ctx, path string) (Analysis, error)`, `CommitAll(ctx, repoPath, message string) (Commit, error)`, and `PushCurrentBranch(ctx, repoPath string) error`.
- [ ] Implement issue-key detection from current branch using an uppercase project-key regex.
- [ ] Implement base branch detection in order: upstream merge-base target, `origin/main`, `main`, `origin/master`, `master`.
- [ ] Implement local commit listing with `git log <base>..HEAD --format=%H%x00%s%x00%b%x1e`.
- [ ] Implement dirty file listing with `git status --porcelain`.
- [ ] Implement commit creation using `git add -A` then `git commit -m <message>`.
- [ ] Implement push using `git push -u origin HEAD`.
- [ ] Store reported commit records in JSON with `0600` file permissions and parent directory `0700`.
- [ ] Add tests using temporary Git repositories and temporary state files.

### Task 2: Shared Git Workflow Planning Helpers

**Files:**
- Create: `internal/gitworkflow/plan.go`
- Create: `internal/gitworkflow/plan_test.go`

**Interfaces:**
- Produces `CommitPlan`, `FinishPlan`, and compact note builders consumed by app commands.

- [ ] Add `CommitPlan` with issue key, branch, dirty flag, unreported commits, default commit message, compact Jira note, and recommended push flag.
- [ ] Add `FinishPlan` with commit plan, PR title/body, final Jira note, and recommended terminal transition label.
- [ ] Build default commit messages as `<ISSUE>: <summary>` with summary trimmed to one line.
- [ ] Build compact Jira commit notes from commit subjects only, capped to a small fixed number of lines.
- [ ] Build final Jira notes as a short synopsis from PR title plus commit subjects.
- [ ] Add tests for dirty-only, unreported-only, mixed, and no-op planning.

### Task 3: `jira commit`

**Files:**
- Modify: `internal/app/app.go`
- Create: `internal/app/commit.go`
- Modify: `internal/app/app_test.go`

**Interfaces:**
- Produces Cobra command `jira commit [ticket]`.
- Consumes `gitworkflow.Client`, `gitstate.Store`, and `jira.Client`.

- [ ] Register `jira commit [ticket]` and keep `--profile` inherited.
- [ ] Load config through the same profile path as the main app.
- [ ] Analyze current repo through `gitworkflow.Client`.
- [ ] Resolve issue key from explicit argument first, then branch-derived key.
- [ ] Fetch issue detail for summary context when possible.
- [ ] Load locally reported commits for repo, branch, and issue key.
- [ ] Present a review summary before writes: dirty files, unreported commits, Jira note, push action.
- [ ] Support cancellation with no writes.
- [ ] On confirmation, create a commit when dirty work exists.
- [ ] Post a compact Jira comment for newly created and unreported commits when confirmed.
- [ ] Mark reported commit SHAs only after the Jira comment succeeds or when the user explicitly skips Jira reporting.
- [ ] Offer branch push after commit/report writes.
- [ ] Print completed/skipped/failed outcomes.
- [ ] Add app tests with fake Git, fake state, fake Jira, and injected confirmation.

### Task 4: GitHub PR Provider

**Files:**
- Create: `internal/prprovider/provider.go`
- Create: `internal/prprovider/github.go`
- Create: `internal/prprovider/github_test.go`

**Interfaces:**
- Produces `Provider` with `CurrentPR`, `CreateOrUpdatePR`, and `PushURL`-independent request types.

- [ ] Define `Provider` interface independent of GitHub.
- [ ] Implement `GitHubCLIProvider` using `gh pr view` and `gh pr create`.
- [ ] Keep every production `gh` shell call inside `internal/prprovider`.
- [ ] Parse existing PR URLs from `gh pr view --json url,title,state`.
- [ ] Create draft PRs by default with reviewed title/body.
- [ ] Return clear errors when `gh` is missing, unauthenticated, or unsupported.
- [ ] Add tests with an injected command runner rather than real GitHub.

### Task 5: `jira finish`

**Files:**
- Modify: `internal/app/app.go`
- Create: `internal/app/finish.go`
- Modify: `internal/app/app_test.go`

**Interfaces:**
- Produces Cobra command `jira finish [ticket]`.
- Consumes `gitworkflow.Client`, `gitstate.Store`, `prprovider.Provider`, and `jira.Client`.

- [ ] Register `jira finish [ticket]`.
- [ ] Analyze repo and resolve ticket the same way `jira commit` does.
- [ ] Reuse commit planning to handle dirty work and unreported commits.
- [ ] Fetch Jira transitions and choose the best terminal transition by metadata (`done`, `closed`, `resolved`, `complete`) when no required unsupported fields are present.
- [ ] Present a review summary before writes: commit, Jira commit note, push, PR create/update, final Jira note, transition.
- [ ] On confirmation, commit dirty work if present.
- [ ] Report unreported commits and mark them locally after successful Jira reporting.
- [ ] Push the branch before PR creation/update.
- [ ] Create or report an existing GitHub draft PR through the provider interface.
- [ ] Post a final compact Jira comment with the PR URL when confirmed.
- [ ] Transition Jira only when a safe terminal transition is available and confirmed.
- [ ] Print completed/skipped/failed outcomes.
- [ ] Add app tests for existing PR, new PR, transition skip with required fields, and partial failure summaries.

### Task 6: Docs, GitHub Sync, And Verification

**Files:**
- Modify: `README.md`
- Modify: `docs/backlog.md`
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

- [ ] Document `jira commit` and `jira finish`.
- [ ] Update changelog under Unreleased.
- [ ] Update GitHub issues #5 and #8 with implementation notes; close them only after merge to `main`.
- [ ] Run focused package tests after each implementation slice.
- [ ] Run final `go test ./... -count=1`, `make check`, `make install-user`, `make docs-status`, and `git diff --check`.
- [ ] Commit and push the feature branch for review or merge.

## Self-Review

- Spec coverage: ADR 0009, GitHub issue #5, and GitHub issue #8 map to Git adapter expansion, local reported commit state, commit workflow, finish workflow, GitHub provider boundary, reviewed writes, docs, and GitHub issue sync.
- Placeholder scan: no TBD/TODO placeholders remain; deferred AI is explicitly out of this slice unless existing hooks fit without expanding scope.
- Type consistency: package names and interfaces are defined before app command consumers use them.
