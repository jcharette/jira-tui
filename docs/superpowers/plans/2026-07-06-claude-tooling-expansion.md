# Claude Tooling Expansion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add practical Claude assistance to the existing Jira/Git tooling surfaces without adding provider-neutral routing or unreviewed writes.

**Architecture:** Reuse the existing local Claude CLI runner, TUI `submitAIRequest` path, feature flags, progress handling, and review gates. Add small Claude hooks only where a user already reviews generated text before posting, creating, or opening external URLs. Keep provider-neutral/Codex work deferred.

**Tech Stack:** Go, Cobra CLI, Bubble Tea v2, existing `internal/claude`, existing `internal/app` git workflows, existing `internal/tui` Claude Assist/create/bug-report surfaces.

## Global Constraints

- Ponytail-full: smallest correct diff, no new dependencies, no provider registry.
- Claude remains the only AI execution provider in this batch.
- No Jira, Git, GitHub, code, or browser write may happen without the existing user confirmation path.
- Generated text must be bounded, source-aware, and editable or reviewable before use.
- If Claude is disabled, unavailable, times out, or returns invalid output, every workflow must fall back to the current non-Claude behavior.
- Shortcut or behavior changes must update visible footer/help text and durable docs in the same change.
- Keep `ai.task.*` events as Diagnostics breadcrumbs; do not remove provider-neutral event names.

---

### Task 1: Claude Drafts `jira finish` PR Text And Final Note

**Files:**
- Modify: `internal/app/finish.go`
- Modify: `internal/app/finish_test.go`
- Modify: `internal/app/commit.go`
- Modify: `README.md`
- Modify: `docs/workflows.md`
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

**Interfaces:**
- Consumes: `claudeCommitNoteDrafter` pattern from `internal/app/commit.go`.
- Produces:
  - `type finishDrafter interface { DraftFinishText(context.Context, finishDraftRequest) (finishDraft, error) }`
  - `type finishDraft struct { PRTitle string; PRBody string; FinalJiraNote string }`
  - `runFinishWithDepsAndOptions(..., options finishOptions) error`

- [ ] **Step 1: Write RED tests for Claude finish drafting**

Add to `internal/app/finish_test.go`:

```go
func TestRunFinishUsesClaudeDraftedPRAndFinalNote(t *testing.T) {
	gitClient := &fakeCommitGitClient{
		analysis: gitworkflow.Analysis{
			Repo:       gitworkflow.RepoStatus{Path: "/repo", CurrentBranch: "feature/ABC-123-work", Detected: true},
			BaseBranch: "main",
			IssueKey:   "ABC-123",
			Commits:    []gitworkflow.Commit{{SHA: "2222222", Subject: "ABC-123: tighten checks"}},
		},
	}
	jiraClient := &fakeFinishJiraClient{issue: jira.Issue{Key: "ABC-123", Summary: "Prepare release"}}
	stateStore := &fakeCommitStateStore{}
	prProvider := &fakePRProvider{pr: prprovider.PullRequest{URL: "https://github.com/acme/repo/pull/13", Created: true}}
	drafter := &fakeFinishDrafter{draft: finishDraft{
		PRTitle:       "ABC-123: Prepare release validation",
		PRBody:        "Summary:\n- Tightened release checks.\n\nVerification:\n- go test ./...",
		FinalJiraNote: "Ready for review:\n- Release validation tightened.",
	}}
	var out bytes.Buffer

	err := runFinishWithDepsAndOptions(context.Background(), nil, &out, gitClient, jiraClient, stateStore, prProvider, confirmAndWriteFinishReview, finishOptions{Drafter: drafter})

	if err != nil {
		t.Fatalf("runFinishWithDepsAndOptions() error = %v", err)
	}
	if len(prProvider.requests) != 1 {
		t.Fatalf("requests = %#v", prProvider.requests)
	}
	if prProvider.requests[0].Title != "ABC-123: Prepare release validation" {
		t.Fatalf("PR title = %q", prProvider.requests[0].Title)
	}
	if !strings.Contains(prProvider.requests[0].Body, "Verification") {
		t.Fatalf("PR body = %q", prProvider.requests[0].Body)
	}
	if !strings.Contains(jiraClient.comments[0], "Release validation tightened") {
		t.Fatalf("final note = %#v", jiraClient.comments)
	}
	if !strings.Contains(out.String(), "AI drafted finish text: yes") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunFinishFallsBackWhenClaudeDraftFails(t *testing.T) {
	gitClient := &fakeCommitGitClient{
		analysis: gitworkflow.Analysis{
			Repo:     gitworkflow.RepoStatus{Path: "/repo", CurrentBranch: "feature/ABC-123-work", Detected: true},
			IssueKey: "ABC-123",
		},
	}
	jiraClient := &fakeFinishJiraClient{issue: jira.Issue{Key: "ABC-123", Summary: "Prepare release"}}
	prProvider := &fakePRProvider{pr: prprovider.PullRequest{URL: "https://github.com/acme/repo/pull/13", Created: true}}
	drafter := &fakeFinishDrafter{err: errors.New("claude unavailable")}
	var out bytes.Buffer

	err := runFinishWithDepsAndOptions(context.Background(), nil, &out, gitClient, jiraClient, &fakeCommitStateStore{}, prProvider, confirmAndWriteFinishReview, finishOptions{Drafter: drafter})

	if err != nil {
		t.Fatalf("runFinishWithDepsAndOptions() error = %v", err)
	}
	if prProvider.requests[0].Title != "ABC-123: Prepare release" {
		t.Fatalf("PR title = %q", prProvider.requests[0].Title)
	}
	if !strings.Contains(out.String(), "AI drafted finish text: no (claude unavailable)") {
		t.Fatalf("output = %q", out.String())
	}
}
```

Add helpers:

```go
type fakeFinishDrafter struct {
	draft    finishDraft
	err      error
	requests []finishDraftRequest
}

func (f *fakeFinishDrafter) DraftFinishText(_ context.Context, request finishDraftRequest) (finishDraft, error) {
	f.requests = append(f.requests, request)
	return f.draft, f.err
}

func confirmAndWriteFinishReview(out io.Writer, review finishReview) bool {
	writeFinishReview(out, review)
	return true
}
```

- [ ] **Step 2: Run RED tests**

Run:

```bash
go test ./internal/app -run 'TestRunFinishUsesClaudeDraftedPRAndFinalNote|TestRunFinishFallsBackWhenClaudeDraftFails' -count=1
```

Expected: FAIL because `runFinishWithDepsAndOptions`, `finishOptions`, `finishDraftRequest`, and `finishDraft` do not exist.

- [ ] **Step 3: Implement minimal finish drafter options**

In `internal/app/finish.go`, add:

```go
type finishOptions struct {
	Drafter finishDrafter
}

type finishDrafter interface {
	DraftFinishText(context.Context, finishDraftRequest) (finishDraft, error)
}

type finishDraftRequest struct {
	Plan  gitworkflow.FinishPlan
	Issue jira.Issue
}

type finishDraft struct {
	PRTitle       string
	PRBody        string
	FinalJiraNote string
}
```

Change `runFinishWithDeps` to call:

```go
return runFinishWithDepsAndOptions(ctx, args, out, gitClient, jiraClient, stateStore, prProvider, confirm, finishOptions{})
```

Move the existing body into `runFinishWithDepsAndOptions`.

- [ ] **Step 4: Apply cleaned finish draft before review**

In `runFinishWithDepsAndOptions`, after `plan := gitworkflow.BuildFinishPlan(...)`, add:

```go
aiDrafted := false
aiDraftErr := ""
if options.Drafter != nil {
	draft, err := options.Drafter.DraftFinishText(ctx, finishDraftRequest{Plan: plan, Issue: detail.Issue})
	if err != nil {
		aiDraftErr = err.Error()
	} else if cleaned, ok := cleanFinishDraft(draft); ok {
		plan.PRTitle = cleaned.PRTitle
		plan.PRBody = cleaned.PRBody
		plan.FinalJiraNote = cleaned.FinalJiraNote
		aiDrafted = true
	} else {
		aiDraftErr = "empty finish draft"
	}
}
```

Extend `finishReview`:

```go
AIDrafted  bool
AIDraftErr string
```

Print in `writeFinishReview` before PR title:

```go
if review.AIDrafted {
	_, _ = fmt.Fprintln(out, "AI drafted finish text: yes")
} else if strings.TrimSpace(review.AIDraftErr) != "" {
	_, _ = fmt.Fprintf(out, "AI drafted finish text: no (%s)\n", review.AIDraftErr)
}
```

- [ ] **Step 5: Add prompt and cleaning helpers**

In `internal/app/finish.go`, add:

```go
const maxFinishAIBodyBytes = 4000

func cleanFinishDraft(draft finishDraft) (finishDraft, bool) {
	draft.PRTitle = oneLineBounded(draft.PRTitle, 180)
	draft.PRBody = cleanBoundedText(draft.PRBody, maxFinishAIBodyBytes)
	draft.FinalJiraNote = cleanBoundedText(draft.FinalJiraNote, maxCommitAINoteBytes)
	if draft.PRTitle == "" || draft.PRBody == "" || draft.FinalJiraNote == "" {
		return finishDraft{}, false
	}
	return draft, true
}
```

Add shared helpers to `internal/app/commit.go`:

```go
func oneLineBounded(value string, maxBytes int) string {
	return cleanBoundedText(strings.Join(strings.Fields(value), " "), maxBytes)
}

func cleanBoundedText(value string, maxBytes int) string {
	value = strings.TrimSpace(value)
	if value == "" || maxBytes <= 0 {
		return ""
	}
	if len([]byte(value)) <= maxBytes {
		return value
	}
	var b strings.Builder
	for _, r := range value {
		if b.Len()+len(string(r)) > maxBytes {
			break
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}
```

Then simplify `cleanCommitAINote` to `return cleanBoundedText(note, maxCommitAINoteBytes)`.

- [ ] **Step 6: Add Claude finish drafter**

In `internal/app/finish.go`, add:

```go
type claudeFinishDrafter struct {
	Config claude.Config
	Runner claude.LocalRunner
}

func (d claudeFinishDrafter) DraftFinishText(ctx context.Context, request finishDraftRequest) (finishDraft, error) {
	result, err := d.Runner.Run(ctx, claude.Request{Config: d.Config, Prompt: buildFinishDraftPrompt(request)})
	if err != nil {
		return finishDraft{}, err
	}
	return parseFinishDraft(result.Text)
}
```

Use a simple parse format:

```text
PR Title: ...
PR Body:
...
Final Jira Note:
...
```

- [ ] **Step 7: Wire from config**

Add:

```go
func finishDrafterFromConfig(cfg config.Config) finishDrafter {
	if !cfg.Claude.Enabled || !cfg.Claude.Features.PRCreation {
		return nil
	}
	claudeConfig, ok := checkedClaudeConfig(cfg)
	if !ok {
		return nil
	}
	return claudeFinishDrafter{Config: claudeConfig}
}
```

Extract the existing `commitNoteDrafterFromConfig` status logic into `checkedClaudeConfig(cfg config.Config) (claude.Config, bool)`.

- [ ] **Step 8: Verify and commit Task 1**

Run:

```bash
gofmt -w internal/app/commit.go internal/app/finish.go internal/app/finish_test.go
go test ./internal/app -count=1
go test ./... -count=1
make docs-check
make check
```

Expected: all PASS.

Commit:

```bash
git add internal/app/commit.go internal/app/finish.go internal/app/finish_test.go README.md docs/workflows.md docs/project-state.md docs/releases/CHANGELOG.md tasks/todo.md
git commit -m "feat: draft finish text with Claude"
```

### Task 2: Add Ticket Detail Quality Review And Draft Comment Actions

**Files:**
- Modify: `internal/tui/claude_assist.go`
- Modify: `internal/tui/claude_assist_test.go`
- Modify: `internal/tui/keymap.go`
- Modify: `docs/keyboard.md`
- Modify: `docs/workflows.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

**Interfaces:**
- Consumes: existing `claudeAssist` modal, `submitClaudeTicketAssistOperation`, and parsed draft/comment apply paths.
- Produces: two new Claude section actions:
  - `quality_review`: read-only ticket quality output.
  - `draft_comment`: editable comment draft from ticket context and recent comments.

- [ ] **Step 1: Write RED tests for visible actions**

Add tests to `internal/tui/claude_assist_test.go`:

```go
func TestClaudeSectionShowsQualityReviewAndDraftComment(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	model.mode = modeDetail
	model.detail = jira.IssueDetail{Issue: jira.Issue{Key: "ABC-1", Summary: "Unclear rollout"}}

	view := model.renderClaudeSection(detailRenderContext{selected: model.detail.Issue, display: model.detail.Issue}, 100)

	for _, want := range []string{"Quality Review", "Draft Comment"} {
		if !strings.Contains(view, want) {
			t.Fatalf("Claude section missing %q:\n%s", want, view)
		}
	}
}
```

- [ ] **Step 2: Add action definitions**

Change `claudeActions()` to include:

```go
{ID: "quality_review", Label: "Quality Review", Description: "Find missing acceptance criteria, assumptions, and questions.", Enabled: m.claudeTicketAssistAvailable()},
{ID: "draft_comment", Label: "Draft Comment", Description: "Draft a clarifying Jira comment for review before posting.", Enabled: m.claudeTicketAssistAvailable()},
```

- [ ] **Step 3: Route selected actions**

In `runSelectedClaudeAction`, add cases:

```go
case "quality_review":
	return m.startClaudeQualityReview()
case "draft_comment":
	return m.startClaudeDraftComment()
```

Implement `startClaudeQualityReview()` as a read-only `claudePlan` operation with a quality prompt and `events.AIOperationTicketAssist`.

Implement `startClaudeDraftComment()` as a `claudeAssist` operation with `m.claudeAssistTarget = claudeAssistTargetComment`.

- [ ] **Step 4: Add prompt tests**

Add tests that call `buildClaudeQualityReviewPrompt(ctx)` and `buildClaudeDraftCommentPrompt(ctx)` and assert they include:

```text
Acceptance Criteria
Open Questions
Recent Comments
Return only
```

- [ ] **Step 5: Verify and commit Task 2**

Run:

```bash
gofmt -w internal/tui/claude_assist.go internal/tui/claude_assist_test.go internal/tui/keymap.go
go test ./internal/tui -run 'TestClaude.*Quality|TestClaude.*DraftComment|TestClaudeSectionShowsQualityReviewAndDraftComment' -count=1
go test ./internal/tui -count=1
go test ./... -count=1
make docs-check
make check
```

Expected: all PASS.

Commit:

```bash
git add internal/tui/claude_assist.go internal/tui/claude_assist_test.go internal/tui/keymap.go docs/keyboard.md docs/workflows.md docs/releases/CHANGELOG.md tasks/todo.md
git commit -m "feat: add Claude ticket quality and comment drafts"
```

### Task 3: Add Claude Read-Only Plan To Start Work

**Files:**
- Modify: `internal/app/start.go`
- Modify: `internal/app/start_test.go`
- Modify: `internal/tui/start_workflow.go`
- Modify: `internal/tui/start_workflow_test.go`
- Modify: `README.md`
- Modify: `docs/workflows.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

**Interfaces:**
- Consumes: existing `jira start` flow and `ClaudeFeatures.BranchPlan`.
- Produces: optional read-only implementation plan text shown in the start review before branch/Jira writes.

- [ ] **Step 1: Write RED CLI start test**

Add to `internal/app/start_test.go`:

```go
func TestRunStartShowsClaudePlanBeforeWrites(t *testing.T) {
	// Use the existing fake start dependencies in this file.
	// Assert the review output contains "Claude plan:" and the supplied fake plan.
}
```

Use existing fakes; if no injection seam exists, add `startOptions{PlanDrafter startPlanDrafter}` matching the commit/finish pattern.

- [ ] **Step 2: Implement minimal start plan drafter**

Add:

```go
type startPlanDrafter interface {
	DraftStartPlan(context.Context, startPlanDraftRequest) (string, error)
}
```

Prompt includes ticket key, summary, description excerpt, selected repo, branch, and requested writes. The plan is read-only and never changes branch/comment actions.

- [ ] **Step 3: Wire config**

Use `cfg.Claude.Features.BranchPlan`. If disabled/unavailable, start behaves exactly as today.

- [ ] **Step 4: Add TUI review display**

In the shared start workflow review screen, render:

```text
Claude plan:
<bounded plan text>
```

If drafting fails, show:

```text
Claude plan: unavailable (<error>)
```

Do not block branch creation.

- [ ] **Step 5: Verify and commit Task 3**

Run:

```bash
gofmt -w internal/app/start.go internal/app/start_test.go internal/tui/start_workflow.go internal/tui/start_workflow_test.go
go test ./internal/app ./internal/tui -run 'Start.*Claude|Claude.*Start' -count=1
go test ./... -count=1
make docs-check
make check
```

Expected: all PASS.

Commit:

```bash
git add internal/app/start.go internal/app/start_test.go internal/tui/start_workflow.go internal/tui/start_workflow_test.go README.md docs/workflows.md docs/releases/CHANGELOG.md tasks/todo.md
git commit -m "feat: add Claude start-work plans"
```

### Task 4: Add Create-Ticket Draft Refinement Action

**Files:**
- Modify: `internal/tui/create_issue.go`
- Modify: `internal/tui/create_issue_test.go`
- Modify: `docs/keyboard.md`
- Modify: `docs/workflows.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

**Interfaces:**
- Consumes: existing `createAIPrompt` flow and Open Questions answer loop.
- Produces: visible “Refine Draft” action after a Claude-created or manually edited draft exists.

- [ ] **Step 1: Write RED test for refine prompt**

Add to `internal/tui/create_issue_test.go`:

```go
func TestCreateIssueRefineDraftPromptIncludesCurrentDraft(t *testing.T) {
	model := newCreateIssueTestModel()
	model.createSummaryDraft = "Initial summary"
	model.createDescriptionDraft = "Initial description"
	model.createAIPrompt = "Make this clearer and add acceptance criteria."

	prompt := model.buildCreateIssueRefinePrompt(model.createAIPrompt)

	for _, want := range []string{"Initial summary", "Initial description", "acceptance criteria", "Return plain text in this exact format"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}
```

- [ ] **Step 2: Implement by reusing create prompt path**

Add:

```go
func (m Model) buildCreateIssueRefinePrompt(request string) string {
	return m.buildCreateIssueDraftPrompt("Refine the current draft. "+strings.TrimSpace(request))
}
```

Add a visible button/action in create mode footer when summary or description is non-empty:

```text
ctrl+r refine with Claude
```

Route `ctrl+r` to open `createAIPrompt` with placeholder `Refine the current ticket draft.`

- [ ] **Step 3: Verify and commit Task 4**

Run:

```bash
gofmt -w internal/tui/create_issue.go internal/tui/create_issue_test.go
go test ./internal/tui -run 'TestCreateIssue.*Refine|TestCreateIssue.*AIPrompt' -count=1
go test ./internal/tui -count=1
go test ./... -count=1
make docs-check
make check
```

Expected: all PASS.

Commit:

```bash
git add internal/tui/create_issue.go internal/tui/create_issue_test.go docs/keyboard.md docs/workflows.md docs/releases/CHANGELOG.md tasks/todo.md
git commit -m "feat: refine create-ticket drafts with Claude"
```

### Task 5: Add Claude Cleanup To Bug Report Composer

**Files:**
- Modify: `internal/tui/bug_report.go`
- Modify: `internal/tui/bug_report_test.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/keymap.go`
- Modify: `docs/keyboard.md`
- Modify: `docs/workflows.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

**Interfaces:**
- Consumes: existing bug report title/body fields and sanitized diagnostics excerpt.
- Produces: optional Claude cleanup action that rewrites the local title/body only; it does not open GitHub or upload logs.

- [ ] **Step 1: Write RED test for bug report cleanup prompt**

Add to `internal/tui/bug_report_test.go`:

```go
func TestBugReportClaudeCleanupPromptUsesSanitizedDiagnostics(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	model = model.startBugReport()
	model.bugReportTitleEditor.SetValue("Refresh freezes")
	model.bugReportBodyEditor.SetValue("It stopped after I pressed r.")
	model.bugReportIncludeDiagnostics = true
	model.recordDiagnosticEvent(diagnosticKindState, "unsafe", "error", "token=secret safe=value")

	prompt := model.buildBugReportCleanupPrompt()

	for _, want := range []string{"Refresh freezes", "It stopped", "safe=value", "Return plain text"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "token=secret") {
		t.Fatalf("prompt leaked sensitive diagnostics:\n%s", prompt)
	}
}
```

- [ ] **Step 2: Add cleanup action**

Add key binding:

```text
ctrl+r polish with Claude
```

When pressed, submit a Claude request using existing `submitAIRequest`. Parse:

```text
Title: ...
Body:
...
```

Apply parsed title/body back into local editors. Do not call `openURL`.

- [ ] **Step 3: Verify and commit Task 5**

Run:

```bash
gofmt -w internal/tui/bug_report.go internal/tui/bug_report_test.go internal/tui/model.go internal/tui/keymap.go
go test ./internal/tui -run 'TestBugReport.*Claude|TestBugReport.*Diagnostics' -count=1
go test ./internal/tui -count=1
go test ./... -count=1
make docs-check
make check
```

Expected: all PASS.

Commit:

```bash
git add internal/tui/bug_report.go internal/tui/bug_report_test.go internal/tui/model.go internal/tui/keymap.go docs/keyboard.md docs/workflows.md docs/releases/CHANGELOG.md tasks/todo.md
git commit -m "feat: polish bug reports with Claude"
```

### Task 6: Final Docs Audit And Release Readiness

**Files:**
- Modify: `README.md`
- Modify: `docs/quickstart.md`
- Modify: `docs/workflows.md`
- Modify: `docs/keyboard.md`
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

**Interfaces:**
- Consumes: all implemented Claude workflow changes from Tasks 1-5.
- Produces: complete user-facing documentation and final verification record.

- [ ] **Step 1: Audit docs for every new Claude surface**

Run:

```bash
rg -n "Claude|AI|jira finish|jira start|bug report|create.*draft|Draft Comment|Quality Review" README.md docs
```

Expected: each new behavior appears in at least one user-facing doc and the changelog.

- [ ] **Step 2: Update `tasks/todo.md` review**

Add:

```markdown
### Review

- Added Claude drafting to `jira finish` PR title/body and final Jira note.
- Added ticket-detail Claude Quality Review and Draft Comment actions.
- Added read-only Claude start-work plans before branch/Jira writes.
- Added create-ticket draft refinement.
- Added Claude cleanup for bug report title/body without changing the explicit GitHub-open action.
- Kept provider-neutral execution, Codex support, workspace mapping, and code edits deferred.
- Verified focused tests, `go test ./... -count=1`, `make docs-check`, `make check`, and `make install-user`.
```

- [ ] **Step 3: Run final verification**

Run:

```bash
go test ./... -count=1
make docs-check
make check
make install-user
```

Expected: all PASS.

- [ ] **Step 4: Commit final docs audit**

Run:

```bash
git add README.md docs/quickstart.md docs/workflows.md docs/keyboard.md docs/project-state.md docs/releases/CHANGELOG.md tasks/todo.md
git commit -m "docs: document Claude tooling expansion"
```

Expected: commit succeeds, or no-op if all docs were already committed in earlier tasks.

## Plan Self-Review

- Spec coverage: covers all six requested Claude uses: `jira finish`, draft comments, ticket quality check, start-work plan, create-ticket refinement, and bug report cleanup.
- Scope check: explicitly defers provider-neutral routing, Codex support, workspace mapping, and code edits.
- Placeholder scan: no placeholder markers; each task has files, test targets, verification, and commit boundary.
- Type consistency: app-layer drafter patterns follow the existing `jira commit` Claude note drafter; TUI tasks reuse existing `submitAIRequest`, progress, and modal patterns.
