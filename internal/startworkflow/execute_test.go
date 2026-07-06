package startworkflow

import (
	"context"
	"errors"
	"testing"

	"github.com/jcharette/jira-tui/internal/gitworkflow"
	"github.com/jcharette/jira-tui/internal/jira"
)

func TestApplyActionsStopsJiraWritesAfterRequiredBranchFailure(t *testing.T) {
	gitClient := fakeStartGitClient{err: errors.New("branch failed")}
	jiraClient := &fakeStartJiraClient{}
	result := Result{
		RepoPath:   "/repo",
		BranchName: "ABC-1-work",
		Issue:      jira.Issue{Key: "ABC-1"},
		Actions: []ActionPlan{
			{Kind: ActionBranch, Label: "Create branch", Required: true},
			{Kind: ActionAssign, Label: "Assign"},
			{Kind: ActionTransition, Label: "Transition"},
			{Kind: ActionComment, Label: "Comment"},
		},
	}

	outcomes := ApplyActions(context.Background(), gitClient, jiraClient, result)

	if len(outcomes) != 4 {
		t.Fatalf("outcomes = %#v", outcomes)
	}
	if outcomes[0].State != "failed" {
		t.Fatalf("branch outcome = %#v", outcomes[0])
	}
	for _, outcome := range outcomes[1:] {
		if outcome.State != "skipped" {
			t.Fatalf("jira outcome = %#v", outcome)
		}
	}
	if jiraClient.called {
		t.Fatal("Jira client should not be called after required branch failure")
	}
}

type fakeStartGitClient struct {
	err error
}

func (f fakeStartGitClient) DetectRepo(context.Context, string) (gitworkflow.RepoStatus, error) {
	return gitworkflow.RepoStatus{}, nil
}

func (f fakeStartGitClient) Analyze(context.Context, string) (gitworkflow.Analysis, error) {
	return gitworkflow.Analysis{}, nil
}

func (f fakeStartGitClient) CreateOrSwitchBranch(context.Context, string, string) error {
	return f.err
}

func (f fakeStartGitClient) CommitAll(context.Context, string, string) (gitworkflow.Commit, error) {
	return gitworkflow.Commit{}, nil
}

func (f fakeStartGitClient) PushCurrentBranch(context.Context, string) error {
	return nil
}

type fakeStartJiraClient struct {
	called bool
}

func (f *fakeStartJiraClient) CurrentUser(context.Context) (jira.User, error) {
	f.called = true
	return jira.User{}, nil
}

func (f *fakeStartJiraClient) GetTransitions(context.Context, string) ([]jira.Transition, error) {
	f.called = true
	return nil, nil
}

func (f *fakeStartJiraClient) TransitionIssue(context.Context, string, jira.TransitionIssueRequest) error {
	f.called = true
	return nil
}

func (f *fakeStartJiraClient) UpdateAssignee(context.Context, string, jira.User) error {
	f.called = true
	return nil
}

func (f *fakeStartJiraClient) AddComment(context.Context, string, string, []jira.Mention) (jira.Comment, error) {
	f.called = true
	return jira.Comment{}, nil
}
