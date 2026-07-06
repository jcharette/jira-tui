package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jcharette/jira-tui/internal/gitstate"
	"github.com/jcharette/jira-tui/internal/gitworkflow"
	"github.com/jcharette/jira-tui/internal/jira"
)

func TestRunCommitCreatesCommitReportsAndPushes(t *testing.T) {
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
	var out bytes.Buffer

	err := runCommitWithDeps(context.Background(), nil, &out, gitClient, jiraClient, stateStore, alwaysConfirmCommit)

	if err != nil {
		t.Fatalf("runCommitWithDeps() error = %v", err)
	}
	if gitClient.commitMessage != "ABC-123: Prepare release" {
		t.Fatalf("commitMessage = %q", gitClient.commitMessage)
	}
	if !gitClient.pushed {
		t.Fatal("expected branch push")
	}
	if !strings.Contains(jiraClient.commentBody, "Development update:") || !strings.Contains(jiraClient.commentBody, "ABC-123: Prepare release") {
		t.Fatalf("commentBody = %q", jiraClient.commentBody)
	}
	if len(stateStore.marked) != 1 || stateStore.marked[0].SHA != "1111111abcdef" {
		t.Fatalf("marked = %#v", stateStore.marked)
	}
	for _, want := range []string{"Committed 1111111 ABC-123: Prepare release", "Reported progress to ABC-123.", "Pushed feature/ABC-123-work."} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in output %q", want, out.String())
		}
	}
}

func TestRunCommitReportsUnreportedLocalCommits(t *testing.T) {
	gitClient := &fakeCommitGitClient{
		analysis: gitworkflow.Analysis{
			Repo:     gitworkflow.RepoStatus{Path: "/repo", CurrentBranch: "feature/ABC-123-work", Detected: true},
			IssueKey: "ABC-123",
			Commits: []gitworkflow.Commit{
				{SHA: "1111111", Subject: "ABC-123: first change"},
				{SHA: "2222222", Subject: "ABC-123: second change"},
			},
		},
	}
	jiraClient := &fakeCommitJiraClient{issue: jira.Issue{Key: "ABC-123", Summary: "Prepare release"}}
	stateStore := &fakeCommitStateStore{
		reported: []gitstate.ReportedCommit{{RepoPath: "/repo", Branch: "feature/ABC-123-work", IssueKey: "ABC-123", SHA: "1111111"}},
	}
	var out bytes.Buffer

	err := runCommitWithDeps(context.Background(), nil, &out, gitClient, jiraClient, stateStore, alwaysConfirmCommit)

	if err != nil {
		t.Fatalf("runCommitWithDeps() error = %v", err)
	}
	if gitClient.commitMessage != "" {
		t.Fatalf("unexpected commitMessage = %q", gitClient.commitMessage)
	}
	if !strings.Contains(jiraClient.commentBody, "ABC-123: second change") || strings.Contains(jiraClient.commentBody, "ABC-123: first change") {
		t.Fatalf("commentBody = %q", jiraClient.commentBody)
	}
	if len(stateStore.marked) != 1 || stateStore.marked[0].SHA != "2222222" {
		t.Fatalf("marked = %#v", stateStore.marked)
	}
	if !gitClient.pushed {
		t.Fatal("expected branch push")
	}
}

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

	err := runCommitWithDepsAndOptions(context.Background(), nil, &out, gitClient, jiraClient, stateStore, confirmAndWriteCommitReview, commitOptions{
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

func TestRunCommitFallsBackWhenClaudeDraftFails(t *testing.T) {
	gitClient := &fakeCommitGitClient{
		analysis: gitworkflow.Analysis{
			Repo:     gitworkflow.RepoStatus{Path: "/repo", CurrentBranch: "feature/ABC-123-work", Detected: true},
			IssueKey: "ABC-123",
			Commits:  []gitworkflow.Commit{{SHA: "2222222", Subject: "ABC-123: second change"}},
		},
	}
	jiraClient := &fakeCommitJiraClient{issue: jira.Issue{Key: "ABC-123", Summary: "Prepare release"}}
	stateStore := &fakeCommitStateStore{}
	drafter := &fakeCommitNoteDrafter{err: errors.New("claude unavailable")}
	var out bytes.Buffer

	err := runCommitWithDepsAndOptions(context.Background(), nil, &out, gitClient, jiraClient, stateStore, confirmAndWriteCommitReview, commitOptions{
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

func TestRunCommitCancelsBeforeWrites(t *testing.T) {
	gitClient := &fakeCommitGitClient{
		analysis: gitworkflow.Analysis{
			Repo:     gitworkflow.RepoStatus{Path: "/repo", CurrentBranch: "feature/ABC-123-work", Detected: true},
			Changes:  gitworkflow.ChangeSummary{Dirty: true},
			IssueKey: "ABC-123",
		},
		commit: gitworkflow.Commit{SHA: "1111111", Subject: "ABC-123: Prepare release"},
	}
	jiraClient := &fakeCommitJiraClient{issue: jira.Issue{Key: "ABC-123", Summary: "Prepare release"}}
	stateStore := &fakeCommitStateStore{}
	var out bytes.Buffer

	err := runCommitWithDeps(context.Background(), nil, &out, gitClient, jiraClient, stateStore, neverConfirmCommit)

	if err != nil {
		t.Fatalf("runCommitWithDeps() error = %v", err)
	}
	if gitClient.commitMessage != "" || jiraClient.commentBody != "" || gitClient.pushed || len(stateStore.marked) > 0 {
		t.Fatalf("writes happened: commit=%q comment=%q pushed=%v marked=%#v", gitClient.commitMessage, jiraClient.commentBody, gitClient.pushed, stateStore.marked)
	}
	if !strings.Contains(out.String(), "Commit canceled.") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunCommitNoopsWithoutDirtyOrUnreported(t *testing.T) {
	gitClient := &fakeCommitGitClient{
		analysis: gitworkflow.Analysis{
			Repo: gitworkflow.RepoStatus{Path: "/repo", CurrentBranch: "feature/ABC-123-work", Detected: true},
			Commits: []gitworkflow.Commit{
				{SHA: "1111111", Subject: "ABC-123: already reported"},
			},
			IssueKey: "ABC-123",
		},
	}
	jiraClient := &fakeCommitJiraClient{issue: jira.Issue{Key: "ABC-123", Summary: "Prepare release"}}
	stateStore := &fakeCommitStateStore{
		reported: []gitstate.ReportedCommit{{RepoPath: "/repo", Branch: "feature/ABC-123-work", IssueKey: "ABC-123", SHA: "1111111"}},
	}
	var out bytes.Buffer

	err := runCommitWithDeps(context.Background(), nil, &out, gitClient, jiraClient, stateStore, alwaysConfirmCommit)

	if err != nil {
		t.Fatalf("runCommitWithDeps() error = %v", err)
	}
	if gitClient.commitMessage != "" || jiraClient.commentBody != "" || gitClient.pushed {
		t.Fatalf("unexpected writes: commit=%q comment=%q pushed=%v", gitClient.commitMessage, jiraClient.commentBody, gitClient.pushed)
	}
	if !strings.Contains(out.String(), "Nothing to commit or report for ABC-123.") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunCommitRequiresTicketWhenBranchHasNoIssueKey(t *testing.T) {
	gitClient := &fakeCommitGitClient{
		analysis: gitworkflow.Analysis{
			Repo: gitworkflow.RepoStatus{Path: "/repo", CurrentBranch: "feature/work", Detected: true},
		},
	}
	var out bytes.Buffer

	err := runCommitWithDeps(context.Background(), nil, &out, gitClient, &fakeCommitJiraClient{}, &fakeCommitStateStore{}, alwaysConfirmCommit)

	if err == nil || !strings.Contains(err.Error(), "ticket is required") {
		t.Fatalf("err = %v", err)
	}
}

func TestCleanCommitAINoteBoundsOutput(t *testing.T) {
	note := cleanCommitAINote(strings.Repeat("x", maxCommitAINoteBytes+100))
	if len([]byte(note)) > maxCommitAINoteBytes {
		t.Fatalf("note length = %d", len([]byte(note)))
	}
}

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
	for _, want := range []string{"ABC-123", "Prepare release", "internal/app/commit.go", "2222222", "second change", "Do not edit files", "Return only the Jira note"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

type fakeCommitNoteDrafter struct {
	note     string
	err      error
	requests []commitNoteDraftRequest
}

func (f *fakeCommitNoteDrafter) DraftCommitNote(_ context.Context, request commitNoteDraftRequest) (string, error) {
	f.requests = append(f.requests, request)
	return f.note, f.err
}

type fakeCommitGitClient struct {
	analysis      gitworkflow.Analysis
	commit        gitworkflow.Commit
	commitMessage string
	pushed        bool
	err           error
}

func (f *fakeCommitGitClient) DetectRepo(context.Context, string) (gitworkflow.RepoStatus, error) {
	return f.analysis.Repo, f.err
}

func (f *fakeCommitGitClient) Analyze(context.Context, string) (gitworkflow.Analysis, error) {
	return f.analysis, f.err
}

func (f *fakeCommitGitClient) CreateOrSwitchBranch(context.Context, string, string) error {
	return f.err
}

func (f *fakeCommitGitClient) CommitAll(_ context.Context, _ string, message string) (gitworkflow.Commit, error) {
	f.commitMessage = message
	if f.err != nil {
		return gitworkflow.Commit{}, f.err
	}
	return f.commit, nil
}

func (f *fakeCommitGitClient) PushCurrentBranch(context.Context, string) error {
	f.pushed = true
	return f.err
}

type fakeCommitJiraClient struct {
	issue       jira.Issue
	commentBody string
	err         error
}

func (f *fakeCommitJiraClient) GetIssue(context.Context, string) (jira.IssueDetail, error) {
	if f.err != nil {
		return jira.IssueDetail{}, f.err
	}
	return jira.IssueDetail{Issue: f.issue}, nil
}

func (f *fakeCommitJiraClient) AddComment(_ context.Context, _ string, body string, _ []jira.Mention) (jira.Comment, error) {
	f.commentBody = body
	if f.err != nil {
		return jira.Comment{}, f.err
	}
	return jira.Comment{ID: "10001", Body: body}, nil
}

type fakeCommitStateStore struct {
	reported []gitstate.ReportedCommit
	marked   []gitstate.ReportedCommit
	err      error
}

func (f *fakeCommitStateStore) ReportedCommits(context.Context, string, string, string) ([]gitstate.ReportedCommit, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]gitstate.ReportedCommit(nil), f.reported...), nil
}

func (f *fakeCommitStateStore) MarkReported(_ context.Context, records []gitstate.ReportedCommit) error {
	if f.err != nil {
		return f.err
	}
	f.marked = append(f.marked, records...)
	return nil
}

func alwaysConfirmCommit(io.Writer, commitReview) bool {
	return true
}

func confirmAndWriteCommitReview(out io.Writer, review commitReview) bool {
	writeCommitReview(out, review)
	return true
}

func neverConfirmCommit(io.Writer, commitReview) bool {
	return false
}

var _ gitworkflow.Client = (*fakeCommitGitClient)(nil)
var _ commitJiraClient = (*fakeCommitJiraClient)(nil)
var _ commitStateStore = (*fakeCommitStateStore)(nil)
