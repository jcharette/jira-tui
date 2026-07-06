# Claude AI Workflow Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Backburner provider-neutral AI expansion, clean the public/local backlog, and add one useful Claude-assisted `jira commit` note draft path without broadening provider scope.

**Architecture:** Keep the existing Claude runner and `ai.task.*` Diagnostics events. Do not add Codex support or a provider registry in this slice. Add a small app-layer Claude note drafter for `jira commit` that can replace the deterministic Jira progress note only after the existing review gate.

**Tech Stack:** Go, Cobra CLI, existing `internal/claude` local CLI runner, existing `internal/gitworkflow` commit planning, existing docs/check workflow.

## Global Constraints

- Ponytail-full: smallest correct diff, no new dependencies, no speculative provider abstraction.
- Provider neutrality is deferred; Claude is the only AI execution path in this plan.
- Keep Jira writes behind the existing `jira commit` confirmation prompt.
- Keep generated Jira notes bounded and reviewable.
- Keep `ai.task.*` events as Diagnostics breadcrumbs; do not remove the event model just because Codex is deferred.
- Update GitHub Issues and `docs/backlog.md` together when public backlog state changes.

---

### Task 1: Align Backlog And Product Direction

**Files:**
- Modify: `docs/backlog.md`
- Modify: `docs/roadmap.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`
- GitHub: update issue `#6`

**Interfaces:**
- Consumes: Current GitHub issue state for `#6` and closed `#12`.
- Produces: Public/local backlog wording that says Claude workflow cleanup is active and provider-neutral Codex/future-provider support is deferred.

- [ ] **Step 1: Verify GitHub backlog state**

Run:

```bash
gh issue list --repo jcharette/jira-tui --state all --limit 20 --json number,title,state,url
gh issue view 6 --repo jcharette/jira-tui --json number,title,state,body,url
gh issue view 12 --repo jcharette/jira-tui --json number,title,state,url
```

Expected:

```text
#6 is OPEN.
#12 is CLOSED.
```

- [ ] **Step 2: Update `docs/backlog.md`**

Change the public index from:

```markdown
### AI Workflows

- [#6 Expand AI workflows behind provider-neutral ai.task events](https://github.com/jcharette/jira-tui/issues/6)

### Ticket Actions

- [#12 Expand metadata-backed Ticket Actions](https://github.com/jcharette/jira-tui/issues/12)
```

to:

```markdown
### Claude AI Workflows

- [#6 Polish existing Claude AI workflows before provider-neutral expansion](https://github.com/jcharette/jira-tui/issues/6)
```

Keep the local-only Ticket Actions guidance paragraph, but remove closed `#12` from the active public index.

- [ ] **Step 3: Update `docs/roadmap.md`**

Change:

```markdown
### M2: Provider-Neutral AI Workflows

Status: planned.

### AI Provider Expansion

Issue #6 tracks the next significant product direction: make AI workflows provider-neutral behind
`ai.task` events so Claude, Codex, or future providers can share the same request/progress/result
surface.
```

to:

```markdown
### M2: Claude AI Workflow Polish

Status: planned.

### Claude AI Workflow Cleanup

Issue #6 tracks the next product direction: make the existing Claude-backed AI workflows clearer and
more useful before adding provider-neutral execution. Provider-neutral routing, Codex support, and
future providers stay deferred until the Claude-only workflow stack is cohesive.
```

- [ ] **Step 4: Add changelog entry**

Under `## Unreleased` in `docs/releases/CHANGELOG.md`, add:

```markdown
- Reframed the active AI roadmap around polishing existing Claude workflows before provider-neutral
  provider expansion.
```

- [ ] **Step 5: Add active task plan entry**

At the top of `tasks/todo.md`, below `# Task Plan`, add:

```markdown
## Claude AI Workflow Cleanup - 2026-07-06

- [ ] Sync the local backlog and roadmap with the decision to backburner provider-neutral AI.
- [ ] Add optional Claude-assisted Jira note drafting to `jira commit`.
- [ ] Verify focused tests, docs checks, and full local checks.

### Implementation Notes

- Keep Claude as the only execution provider in this slice.
- Keep `ai.task.*` event names for Diagnostics, but do not add Codex execution.
- Keep generated Jira notes reviewable before posting.
```

- [ ] **Step 6: Update GitHub issue #6**

Run:

```bash
gh issue edit 6 --repo jcharette/jira-tui --title "Polish existing Claude AI workflows before provider-neutral expansion" --body-file /tmp/jira-tui-issue-6.md
```

Use this body in `/tmp/jira-tui-issue-6.md`:

```markdown
## Goal

Make the existing Claude-backed AI workflows cohesive and useful before adding provider-neutral
execution.

## Scope

- Keep Claude as the only AI execution provider for this slice.
- Preserve `ai.task.*` events as Diagnostics breadcrumbs.
- Add focused Claude help where the app already has reviewable workflow surfaces, starting with
  compact `jira commit` progress notes.
- Keep generated text bounded, source-aware, and visible in the existing review prompt before any
  Jira write.
- Defer Codex support, provider routing, and workspace-to-ticket mapping until the Claude-only stack
  is clearer.

## Acceptance Criteria

- Local backlog and roadmap no longer present provider-neutral AI as the immediate next slice.
- `jira commit` can use Claude to draft a compact Jira progress note when Claude is enabled and
  available.
- If Claude is disabled, unavailable, times out, or returns invalid output, `jira commit` falls back
  to the existing deterministic note.
- Jira writes still require the existing confirmation prompt.
- Docs and tests cover the Claude-only behavior.
```

- [ ] **Step 7: Verify docs**

Run:

```bash
make docs-check
```

Expected: PASS.

- [ ] **Step 8: Commit Task 1**

Run:

```bash
git add docs/backlog.md docs/roadmap.md docs/releases/CHANGELOG.md tasks/todo.md
git commit -m "docs: reframe AI backlog around Claude cleanup"
```

Expected: commit succeeds.

### Task 2: Add Claude-Assisted `jira commit` Note Drafting

**Files:**
- Modify: `internal/app/commit.go`
- Modify: `internal/app/commit_test.go`
- Modify: `internal/app/app.go`
- Modify: `README.md`
- Modify: `docs/workflows.md`
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

**Interfaces:**
- Consumes: `claude.LocalRunner.Run(ctx, claude.Request) (claude.Result, error)` from `internal/claude`.
- Produces: `type commitNoteDrafter interface { DraftCommitNote(context.Context, commitNoteDraftRequest) (string, error) }` in `internal/app/commit.go`.
- Produces: `runCommitWithDepsAndOptions(ctx, args, out, gitClient, jiraClient, stateStore, confirm, options)` as the testable commit path with optional Claude drafting.

- [ ] **Step 1: Add failing test for Claude note replacement**

In `internal/app/commit_test.go`, add:

```go
func TestRunCommitUsesClaudeDraftedJiraNoteWhenAvailable(t *testing.T) {
	gitClient := &fakeCommitGitClient{
		analysis: gitworkflow.Analysis{
			Repo: gitworkflow.RepoStatus{Path: "/repo", CurrentBranch: "feature/ABC-123-work", Detected: true},
			Changes: gitworkflow.ChangeSummary{
				Dirty: true,
				Files: []gitworkflow.ChangedFile{{Status: "M", Path: "main.go"}},
			},
			IssueKey: "ABC-123",
		},
		commit: gitworkflow.Commit{SHA: "1111111abcdef", Subject: "ABC-123: Prepare release"},
	}
	jiraClient := &fakeCommitJiraClient{issue: jira.Issue{Key: "ABC-123", Summary: "Prepare release"}}
	stateStore := &fakeCommitStateStore{}
	drafter := &fakeCommitNoteDrafter{note: "Development update:\n- Tightened release prep and validation."}
	var out bytes.Buffer

	err := runCommitWithDepsAndOptions(context.Background(), nil, &out, gitClient, jiraClient, stateStore, alwaysConfirmCommit, commitOptions{
		NoteDrafter: drafter,
	})

	if err != nil {
		t.Fatalf("runCommitWithDepsAndOptions() error = %v", err)
	}
	if jiraClient.commentBody != "Development update:\n- Tightened release prep and validation." {
		t.Fatalf("commentBody = %q", jiraClient.commentBody)
	}
	if !strings.Contains(out.String(), "AI drafted Jira note: yes") {
		t.Fatalf("output = %q", out.String())
	}
	if len(drafter.requests) != 1 || drafter.requests[0].Plan.IssueKey != "ABC-123" {
		t.Fatalf("requests = %#v", drafter.requests)
	}
}
```

Add the fake:

```go
type fakeCommitNoteDrafter struct {
	note     string
	err      error
	requests []commitNoteDraftRequest
}

func (f *fakeCommitNoteDrafter) DraftCommitNote(_ context.Context, request commitNoteDraftRequest) (string, error) {
	f.requests = append(f.requests, request)
	return f.note, f.err
}
```

- [ ] **Step 2: Add failing test for fallback**

In `internal/app/commit_test.go`, add:

```go
func TestRunCommitFallsBackWhenClaudeDraftFails(t *testing.T) {
	gitClient := &fakeCommitGitClient{
		analysis: gitworkflow.Analysis{
			Repo:     gitworkflow.RepoStatus{Path: "/repo", CurrentBranch: "feature/ABC-123-work", Detected: true},
			IssueKey: "ABC-123",
			Commits: []gitworkflow.Commit{{SHA: "2222222", Subject: "ABC-123: second change"}},
		},
	}
	jiraClient := &fakeCommitJiraClient{issue: jira.Issue{Key: "ABC-123", Summary: "Prepare release"}}
	stateStore := &fakeCommitStateStore{}
	drafter := &fakeCommitNoteDrafter{err: errors.New("claude unavailable")}
	var out bytes.Buffer

	err := runCommitWithDepsAndOptions(context.Background(), nil, &out, gitClient, jiraClient, stateStore, alwaysConfirmCommit, commitOptions{
		NoteDrafter: drafter,
	})

	if err != nil {
		t.Fatalf("runCommitWithDepsAndOptions() error = %v", err)
	}
	if !strings.Contains(jiraClient.commentBody, "ABC-123: second change") {
		t.Fatalf("commentBody = %q", jiraClient.commentBody)
	}
	if !strings.Contains(out.String(), "AI drafted Jira note: no (claude unavailable)") {
		t.Fatalf("output = %q", out.String())
	}
}
```

Add `errors` to the imports.

- [ ] **Step 3: Run tests to verify failure**

Run:

```bash
go test ./internal/app -run 'TestRunCommitUsesClaudeDraftedJiraNoteWhenAvailable|TestRunCommitFallsBackWhenClaudeDraftFails' -count=1
```

Expected: FAIL because `runCommitWithDepsAndOptions`, `commitOptions`, and `commitNoteDraftRequest` do not exist.

- [ ] **Step 4: Add minimal commit options and draft request**

In `internal/app/commit.go`, add:

```go
type commitOptions struct {
	NoteDrafter commitNoteDrafter
}

type commitNoteDrafter interface {
	DraftCommitNote(context.Context, commitNoteDraftRequest) (string, error)
}

type commitNoteDraftRequest struct {
	Plan  gitworkflow.CommitPlan
	Issue jira.Issue
}
```

Change `runCommitWithDeps` to delegate:

```go
func runCommitWithDeps(ctx context.Context, args []string, out io.Writer, gitClient gitworkflow.Client, jiraClient commitJiraClient, stateStore commitStateStore, confirm commitConfirmFunc) error {
	return runCommitWithDepsAndOptions(ctx, args, out, gitClient, jiraClient, stateStore, confirm, commitOptions{})
}
```

Create `runCommitWithDepsAndOptions` by moving the existing `runCommitWithDeps` body into it.

- [ ] **Step 5: Apply optional draft before review**

In `runCommitWithDepsAndOptions`, after building `plan` and before `review := commitReview{Plan: plan}`, add:

```go
aiDrafted := false
aiDraftErr := ""
if options.NoteDrafter != nil && plan.ShouldReport {
	note, err := options.NoteDrafter.DraftCommitNote(ctx, commitNoteDraftRequest{Plan: plan, Issue: detail.Issue})
	if err != nil {
		aiDraftErr = err.Error()
	} else if cleaned := cleanCommitAINote(note); cleaned != "" {
		plan.JiraNote = cleaned
		aiDrafted = true
	} else {
		aiDraftErr = "empty note"
	}
}
```

Extend `commitReview`:

```go
type commitReview struct {
	Plan       gitworkflow.CommitPlan
	AIDrafted  bool
	AIDraftErr string
}
```

Set:

```go
review := commitReview{Plan: plan, AIDrafted: aiDrafted, AIDraftErr: aiDraftErr}
```

In `writeCommitReview`, before printing `Jira note:`, add:

```go
if plan.ShouldReport {
	if review.AIDrafted {
		_, _ = fmt.Fprintln(out, "AI drafted Jira note: yes")
	} else if strings.TrimSpace(review.AIDraftErr) != "" {
		_, _ = fmt.Fprintf(out, "AI drafted Jira note: no (%s)\n", review.AIDraftErr)
	}
}
```

- [ ] **Step 6: Bound generated note output**

In `internal/app/commit.go`, add:

```go
const maxCommitAINoteBytes = 1600

func cleanCommitAINote(note string) string {
	note = strings.TrimSpace(note)
	if note == "" {
		return ""
	}
	if len([]byte(note)) <= maxCommitAINoteBytes {
		return note
	}
	runes := []rune(note)
	var b strings.Builder
	for _, r := range runes {
		if b.Len()+len(string(r)) > maxCommitAINoteBytes {
			break
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}
```

Add a focused test:

```go
func TestCleanCommitAINoteBoundsOutput(t *testing.T) {
	note := cleanCommitAINote(strings.Repeat("x", maxCommitAINoteBytes+100))
	if len([]byte(note)) > maxCommitAINoteBytes {
		t.Fatalf("note length = %d", len([]byte(note)))
	}
}
```

- [ ] **Step 7: Add Claude implementation**

In `internal/app/commit.go`, add imports:

```go
"github.com/jcharette/jira-tui/internal/claude"
"github.com/jcharette/jira-tui/internal/config"
```

Add:

```go
type claudeCommitNoteDrafter struct {
	Config claude.Config
	Runner claude.LocalRunner
}

func (d claudeCommitNoteDrafter) DraftCommitNote(ctx context.Context, request commitNoteDraftRequest) (string, error) {
	prompt := buildCommitNotePrompt(request)
	result, err := d.Runner.Run(ctx, claude.Request{Config: d.Config, Prompt: prompt})
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

func buildCommitNotePrompt(request commitNoteDraftRequest) string {
	plan := request.Plan
	var b strings.Builder
	fmt.Fprintf(&b, "Draft a compact Jira progress note for %s.\n", plan.IssueKey)
	fmt.Fprintf(&b, "Ticket summary: %s\n", displayValue(plan.IssueSummary, request.Issue.Summary))
	fmt.Fprintf(&b, "Return only the Jira note. Keep it under 6 bullets and under 1200 characters.\n")
	if plan.ShouldCommit {
		fmt.Fprintf(&b, "Pending commit message: %s\n", plan.DefaultCommitMessage)
	}
	if len(plan.Changes.Files) > 0 {
		b.WriteString("Changed files:\n")
		for _, file := range plan.Changes.Files {
			fmt.Fprintf(&b, "- %s %s\n", strings.TrimSpace(file.Status), file.Path)
		}
	}
	if len(plan.UnreportedCommits) > 0 {
		b.WriteString("Unreported commits:\n")
		for _, commit := range plan.UnreportedCommits {
			fmt.Fprintf(&b, "- %s %s\n", shortCommitSHA(commit.SHA), displayValue(commit.Subject, "(no subject)"))
		}
	}
	return strings.TrimSpace(b.String())
}

func commitNoteDrafterFromConfig(cfg config.Config) commitNoteDrafter {
	if !cfg.Claude.Enabled || !cfg.Claude.Features.BranchPlan {
		return nil
	}
	status := claude.LocalRunner{}.Check(context.Background(), claude.Config{
		Enabled: cfg.Claude.Enabled,
		Command: cfg.Claude.Command,
		Timeout: cfg.Claude.Timeout,
	})
	if !status.Enabled || !status.Available {
		return nil
	}
	command := status.Command
	if command == "" {
		command = cfg.Claude.Command
	}
	return claudeCommitNoteDrafter{
		Config: claude.Config{
			Enabled: true,
			Command: command,
			Timeout: cfg.Claude.Timeout,
		},
	}
}
```

Use `BranchPlan` because it is the existing config feature flag that most closely maps to Git-branch/development workflow AI. Do not add a new config field in this slice.

- [ ] **Step 8: Wire `runCommit` to options**

In `runCommit`, replace:

```go
return runCommitWithDeps(ctx, args, out, gitClient, jira.NewClient(cfg), stateStore, defaultCommitConfirm)
```

with:

```go
return runCommitWithDepsAndOptions(ctx, args, out, gitClient, jira.NewClient(cfg), stateStore, defaultCommitConfirm, commitOptions{
	NoteDrafter: commitNoteDrafterFromConfig(cfg),
})
```

- [ ] **Step 9: Add prompt test**

In `internal/app/commit_test.go`, add:

```go
func TestBuildCommitNotePromptIncludesSourceContext(t *testing.T) {
	prompt := buildCommitNotePrompt(commitNoteDraftRequest{
		Plan: gitworkflow.CommitPlan{
			IssueKey:             "ABC-123",
			IssueSummary:         "Prepare release",
			DefaultCommitMessage: "ABC-123: Prepare release",
			ShouldCommit:         true,
			Changes: gitworkflow.ChangeSummary{
				Files: []gitworkflow.ChangedFile{{Status: "M", Path: "internal/app/commit.go"}},
			},
			UnreportedCommits: []gitworkflow.Commit{{SHA: "2222222abcdef", Subject: "ABC-123: second change"}},
		},
	})
	for _, want := range []string{"ABC-123", "Prepare release", "internal/app/commit.go", "2222222", "second change", "Return only the Jira note"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}
```

- [ ] **Step 10: Verify focused app tests**

Run:

```bash
go test ./internal/app -count=1
```

Expected: PASS.

- [ ] **Step 11: Update user docs**

In `README.md`, update the `jira commit` bullet to mention optional Claude drafting:

```markdown
- `jira commit [ABC-123]` reviews the current repo, detects the ticket from the branch when possible,
  commits dirty work, reports unreported local commits to Jira, records reported SHAs locally, can
  use enabled Claude branch-planning assistance to draft the Jira progress note, and offers to push
  the branch after confirmation.
```

In `docs/workflows.md`, under `## Commit And Finish Work`, add:

```markdown
When Claude is enabled and the Branch Plan feature is on, `jira commit` asks Claude for a compact
Jira progress note and shows it in the normal review prompt. If Claude is unavailable or returns an
empty result, the workflow uses the existing deterministic note.
```

In `docs/project-state.md`, update the Current Commands or workflow section to mention the same behavior.

In `docs/releases/CHANGELOG.md`, under `## Unreleased`, add:

```markdown
- Added optional Claude-assisted Jira progress note drafting to `jira commit`, with fallback to the
  existing deterministic note.
```

- [ ] **Step 12: Update task review**

In `tasks/todo.md`, mark the implementation checklist done and add:

```markdown
### Review

- Reframed the active AI backlog around Claude workflow cleanup and removed stale closed Ticket
  Actions issue #12 from the active local index.
- Added optional Claude-assisted Jira progress note drafting to `jira commit` behind the existing
  Claude Branch Plan feature flag.
- Kept provider-neutral execution, Codex support, and workspace mapping deferred.
- Verified focused app tests, docs checks, and full project checks.
```

- [ ] **Step 13: Run full verification**

Run:

```bash
gofmt -w internal/app/commit.go internal/app/commit_test.go
go test ./internal/app -count=1
go test ./... -count=1
make docs-check
make check
```

Expected: all PASS.

- [ ] **Step 14: Commit Task 2**

Run:

```bash
git add internal/app/commit.go internal/app/commit_test.go README.md docs/workflows.md docs/project-state.md docs/releases/CHANGELOG.md tasks/todo.md
git commit -m "feat: draft commit notes with Claude"
```

Expected: commit succeeds.

## Plan Self-Review

- Spec coverage: includes both requested pieces, docs/backlog cleanup and first Claude-assisted `jira commit` code slice.
- Scope check: provider neutrality, Codex support, provider registry, workspace mapping, and `jira finish` generation are explicitly deferred.
- Placeholder scan: no placeholder tasks; each task has files, commands, and expected results.
- Type consistency: `commitOptions`, `commitNoteDrafter`, and `commitNoteDraftRequest` are introduced before use.
