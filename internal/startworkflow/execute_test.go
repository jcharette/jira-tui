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

func TestApplyJiraActionsAddsIssueToConfiguredActiveSprint(t *testing.T) {
	jiraClient := &fakeStartJiraClient{
		sprints: []jira.Sprint{{ID: 300, BoardID: 100, Name: "Sprint 42", State: "active"}},
		boardIssuesByJQL: map[string][]jira.Issue{
			"key = ABC-1": {{Key: "ABC-1"}},
		},
	}
	result := Result{
		Issue:   jira.Issue{Key: "ABC-1"},
		BoardID: 100,
		Actions: []ActionPlan{
			{Kind: ActionSprint, Label: "Add to active sprint"},
		},
	}

	outcomes := ApplyJiraActions(context.Background(), jiraClient, result, true)

	if len(outcomes) != 1 || outcomes[0].State != "completed" || outcomes[0].Err != nil {
		t.Fatalf("outcomes = %#v", outcomes)
	}
	if jiraClient.moveSprintID != 300 {
		t.Fatalf("moveSprintID = %d", jiraClient.moveSprintID)
	}
	if len(jiraClient.moveIssueKeys) != 1 || jiraClient.moveIssueKeys[0] != "ABC-1" {
		t.Fatalf("moveIssueKeys = %#v", jiraClient.moveIssueKeys)
	}
	if jiraClient.boardID != 100 || jiraClient.boardJQL != "key = ABC-1" {
		t.Fatalf("board verification = %d %q", jiraClient.boardID, jiraClient.boardJQL)
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
	called           bool
	sprints          []jira.Sprint
	moveSprintID     int
	moveIssueKeys    []string
	boardIssuesByJQL map[string][]jira.Issue
	boardID          int
	boardJQL         string
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

func (f *fakeStartJiraClient) GetBoardSprints(_ context.Context, boardID int, states []string, startAt, maxResults int) (jira.SprintPage, error) {
	f.called = true
	return jira.SprintPage{Sprints: append([]jira.Sprint(nil), f.sprints...), IsLast: true}, nil
}

func (f *fakeStartJiraClient) MoveIssuesToSprint(_ context.Context, sprintID int, issueKeys []string) error {
	f.called = true
	f.moveSprintID = sprintID
	f.moveIssueKeys = append([]string(nil), issueKeys...)
	return nil
}

func (f *fakeStartJiraClient) SearchBoardIssues(_ context.Context, boardID int, jql string, maxResults int) ([]jira.Issue, error) {
	f.called = true
	f.boardID = boardID
	f.boardJQL = jql
	if f.boardIssuesByJQL != nil {
		return append([]jira.Issue(nil), f.boardIssuesByJQL[jql]...), nil
	}
	return []jira.Issue{{Key: "ABC-1"}}, nil
}
