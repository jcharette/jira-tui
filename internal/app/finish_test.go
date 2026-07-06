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
	"github.com/jcharette/jira-tui/internal/prprovider"
)

func TestRunFinishCommitsPushesCreatesPRCommentsAndTransitions(t *testing.T) {
	gitClient := &fakeCommitGitClient{
		analysis: gitworkflow.Analysis{
			Repo:       gitworkflow.RepoStatus{Path: "/repo", CurrentBranch: "feature/ABC-123-work", Detected: true},
			BaseBranch: "main",
			Changes: gitworkflow.ChangeSummary{
				Dirty: true,
				Files: []gitworkflow.ChangedFile{{Status: "M", Path: "main.go"}},
			},
			IssueKey: "ABC-123",
		},
		commit: gitworkflow.Commit{SHA: "1111111abcdef", Subject: "ABC-123: Prepare release"},
	}
	jiraClient := &fakeFinishJiraClient{
		issue: jira.Issue{Key: "ABC-123", Summary: "Prepare release"},
		transitions: []jira.Transition{
			{ID: "31", Name: "Done", ToStatus: "Done", IsAvailable: true},
		},
	}
	stateStore := &fakeCommitStateStore{}
	prProvider := &fakePRProvider{pr: prprovider.PullRequest{URL: "https://github.com/acme/repo/pull/13", Created: true}}
	var out bytes.Buffer

	err := runFinishWithDeps(context.Background(), nil, &out, gitClient, jiraClient, stateStore, prProvider, alwaysConfirmFinish)

	if err != nil {
		t.Fatalf("runFinishWithDeps() error = %v", err)
	}
	if gitClient.commitMessage != "ABC-123: Prepare release" || !gitClient.pushed {
		t.Fatalf("git state commit=%q pushed=%v", gitClient.commitMessage, gitClient.pushed)
	}
	if len(prProvider.requests) != 1 || prProvider.requests[0].Title != "ABC-123: Prepare release" || !prProvider.requests[0].Draft {
		t.Fatalf("requests = %#v", prProvider.requests)
	}
	if len(jiraClient.comments) != 2 {
		t.Fatalf("comments = %#v", jiraClient.comments)
	}
	if !strings.Contains(jiraClient.comments[1], "https://github.com/acme/repo/pull/13") {
		t.Fatalf("final comment = %q", jiraClient.comments[1])
	}
	if jiraClient.transitionID != "31" {
		t.Fatalf("transitionID = %q", jiraClient.transitionID)
	}
	if len(stateStore.marked) != 1 || stateStore.marked[0].SHA != "1111111abcdef" {
		t.Fatalf("marked = %#v", stateStore.marked)
	}
	for _, want := range []string{"Created pull request: https://github.com/acme/repo/pull/13", "Posted final note to ABC-123.", "Transitioned ABC-123 to Done."} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in output %q", want, out.String())
		}
	}
}

func TestRunFinishUsesExistingPRAndSkipsRequiredFieldTransition(t *testing.T) {
	gitClient := &fakeCommitGitClient{
		analysis: gitworkflow.Analysis{
			Repo:     gitworkflow.RepoStatus{Path: "/repo", CurrentBranch: "feature/ABC-123-work", Detected: true},
			IssueKey: "ABC-123",
			Commits:  []gitworkflow.Commit{{SHA: "1111111", Subject: "ABC-123: already reported"}},
		},
	}
	jiraClient := &fakeFinishJiraClient{
		issue: jira.Issue{Key: "ABC-123", Summary: "Prepare release"},
		transitions: []jira.Transition{
			{
				ID:          "41",
				Name:        "Resolve",
				ToStatus:    "Done",
				IsAvailable: true,
				Fields:      []jira.TransitionField{{ID: "resolution", Required: true}},
			},
		},
	}
	stateStore := &fakeCommitStateStore{
		reported: []gitstate.ReportedCommit{{RepoPath: "/repo", Branch: "feature/ABC-123-work", IssueKey: "ABC-123", SHA: "1111111"}},
	}
	prProvider := &fakePRProvider{pr: prprovider.PullRequest{URL: "https://github.com/acme/repo/pull/12", Created: false}}
	var out bytes.Buffer

	err := runFinishWithDeps(context.Background(), nil, &out, gitClient, jiraClient, stateStore, prProvider, alwaysConfirmFinish)

	if err != nil {
		t.Fatalf("runFinishWithDeps() error = %v", err)
	}
	if gitClient.commitMessage != "" {
		t.Fatalf("unexpected commit = %q", gitClient.commitMessage)
	}
	if jiraClient.transitionID != "" {
		t.Fatalf("unexpected transition = %q", jiraClient.transitionID)
	}
	if len(jiraClient.comments) != 1 || !strings.Contains(jiraClient.comments[0], "https://github.com/acme/repo/pull/12") {
		t.Fatalf("comments = %#v", jiraClient.comments)
	}
	if !strings.Contains(out.String(), "Using pull request: https://github.com/acme/repo/pull/12") ||
		!strings.Contains(out.String(), "Skipped transition: no safe terminal transition available.") {
		t.Fatalf("output = %q", out.String())
	}
}

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
	if len(jiraClient.comments) != 2 || !strings.Contains(jiraClient.comments[1], "Release validation tightened") {
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

func TestRunFinishCancelsBeforeWrites(t *testing.T) {
	gitClient := &fakeCommitGitClient{
		analysis: gitworkflow.Analysis{
			Repo:     gitworkflow.RepoStatus{Path: "/repo", CurrentBranch: "feature/ABC-123-work", Detected: true},
			Changes:  gitworkflow.ChangeSummary{Dirty: true},
			IssueKey: "ABC-123",
		},
		commit: gitworkflow.Commit{SHA: "1111111", Subject: "ABC-123: Prepare release"},
	}
	jiraClient := &fakeFinishJiraClient{issue: jira.Issue{Key: "ABC-123", Summary: "Prepare release"}}
	prProvider := &fakePRProvider{pr: prprovider.PullRequest{URL: "https://github.com/acme/repo/pull/13", Created: true}}
	var out bytes.Buffer

	err := runFinishWithDeps(context.Background(), nil, &out, gitClient, jiraClient, &fakeCommitStateStore{}, prProvider, neverConfirmFinish)

	if err != nil {
		t.Fatalf("runFinishWithDeps() error = %v", err)
	}
	if gitClient.commitMessage != "" || gitClient.pushed || len(jiraClient.comments) > 0 || len(prProvider.requests) > 0 {
		t.Fatalf("writes happened: commit=%q pushed=%v comments=%#v requests=%#v", gitClient.commitMessage, gitClient.pushed, jiraClient.comments, prProvider.requests)
	}
	if !strings.Contains(out.String(), "Finish canceled.") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestChooseFinishTransitionRanksTerminalTransitions(t *testing.T) {
	transition, ok := chooseFinishTransition([]jira.Transition{
		{ID: "11", Name: "Start Review", ToStatus: "In Review", IsAvailable: true},
		{ID: "12", Name: "Resolve", ToStatus: "Done", IsAvailable: true, Fields: []jira.TransitionField{{ID: "resolution", Required: true}}},
		{ID: "13", Name: "Close", ToStatus: "Closed", IsAvailable: true},
	})

	if !ok || transition.ID != "13" {
		t.Fatalf("transition = %#v ok=%v", transition, ok)
	}
}

type fakeFinishJiraClient struct {
	issue        jira.Issue
	transitions  []jira.Transition
	comments     []string
	transitionID string
	err          error
}

func (f *fakeFinishJiraClient) GetIssue(context.Context, string) (jira.IssueDetail, error) {
	if f.err != nil {
		return jira.IssueDetail{}, f.err
	}
	return jira.IssueDetail{Issue: f.issue}, nil
}

func (f *fakeFinishJiraClient) AddComment(_ context.Context, _ string, body string, _ []jira.Mention) (jira.Comment, error) {
	if f.err != nil {
		return jira.Comment{}, f.err
	}
	f.comments = append(f.comments, body)
	return jira.Comment{ID: "10001", Body: body}, nil
}

func (f *fakeFinishJiraClient) GetTransitions(context.Context, string) ([]jira.Transition, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]jira.Transition(nil), f.transitions...), nil
}

func (f *fakeFinishJiraClient) TransitionIssue(_ context.Context, _ string, request jira.TransitionIssueRequest) error {
	if f.err != nil {
		return f.err
	}
	f.transitionID = request.TransitionID
	return nil
}

type fakeFinishDrafter struct {
	draft    finishDraft
	err      error
	requests []finishDraftRequest
}

func (f *fakeFinishDrafter) DraftFinishText(_ context.Context, request finishDraftRequest) (finishDraft, error) {
	f.requests = append(f.requests, request)
	return f.draft, f.err
}

type fakePRProvider struct {
	pr       prprovider.PullRequest
	requests []prprovider.Request
	err      error
}

func (f *fakePRProvider) CurrentPR(context.Context, string) (prprovider.PullRequest, bool, error) {
	if f.err != nil {
		return prprovider.PullRequest{}, false, f.err
	}
	return f.pr, strings.TrimSpace(f.pr.URL) != "", nil
}

func (f *fakePRProvider) CreateOrUpdatePR(_ context.Context, request prprovider.Request) (prprovider.PullRequest, error) {
	if f.err != nil {
		return prprovider.PullRequest{}, f.err
	}
	f.requests = append(f.requests, request)
	return f.pr, nil
}

func alwaysConfirmFinish(io.Writer, finishReview) bool {
	return true
}

func confirmAndWriteFinishReview(out io.Writer, review finishReview) bool {
	writeFinishReview(out, review)
	return true
}

func neverConfirmFinish(io.Writer, finishReview) bool {
	return false
}

var _ finishJiraClient = (*fakeFinishJiraClient)(nil)
var _ prprovider.Provider = (*fakePRProvider)(nil)
