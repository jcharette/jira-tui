package startworkflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/jcharette/jira-tui/internal/gitworkflow"
	"github.com/jcharette/jira-tui/internal/jira"
)

type JiraClient interface {
	CurrentUser(ctx context.Context) (jira.User, error)
	GetTransitions(ctx context.Context, key string) ([]jira.Transition, error)
	TransitionIssue(ctx context.Context, key string, request jira.TransitionIssueRequest) error
	UpdateAssignee(ctx context.Context, key string, assignee jira.User) error
	AddComment(ctx context.Context, key string, body string, mentions []jira.Mention) (jira.Comment, error)
}

type Outcome struct {
	Kind  ActionKind
	Label string
	State string
	Err   error
}

func ApplyActions(ctx context.Context, gitClient gitworkflow.Client, jiraClient JiraClient, result Result) []Outcome {
	outcomes := make([]Outcome, 0, len(result.Actions))
	branchSucceeded := false
	for _, action := range result.Actions {
		if action.Kind != ActionBranch {
			continue
		}
		outcome := Outcome{Kind: action.Kind, Label: action.Label}
		if action.Skip {
			outcome.State = "skipped"
			outcomes = append(outcomes, outcome)
			continue
		}
		if err := gitClient.CreateOrSwitchBranch(ctx, result.RepoPath, result.BranchName); err != nil {
			outcome.State = "failed"
			outcome.Err = err
		} else {
			branchSucceeded = true
			outcome.State = "completed"
		}
		outcomes = append(outcomes, outcome)
	}
	outcomes = append(outcomes, ApplyJiraActions(ctx, jiraClient, result, branchSucceeded)...)
	return outcomes
}

func ApplyJiraActions(ctx context.Context, jiraClient JiraClient, result Result, branchSucceeded bool) []Outcome {
	outcomes := make([]Outcome, 0, len(result.Actions))
	for _, action := range result.Actions {
		if action.Kind == ActionBranch {
			continue
		}
		outcome := Outcome{Kind: action.Kind, Label: action.Label}
		if action.Skip {
			outcome.State = "skipped"
			outcomes = append(outcomes, outcome)
			continue
		}
		switch action.Kind {
		case ActionAssign:
			user, err := jiraClient.CurrentUser(ctx)
			if err == nil {
				err = jiraClient.UpdateAssignee(ctx, result.Issue.Key, user)
			}
			if err != nil {
				outcome.State = "failed"
				outcome.Err = err
			} else {
				outcome.State = "completed"
			}
		case ActionTransition:
			transition, ok, err := StartTransition(ctx, jiraClient, result.Issue.Key)
			if err != nil {
				outcome.State = "failed"
				outcome.Err = err
			} else if !ok {
				outcome.State = "skipped"
			} else if err := jiraClient.TransitionIssue(ctx, result.Issue.Key, jira.TransitionIssueRequest{TransitionID: transition.ID}); err != nil {
				outcome.State = "failed"
				outcome.Err = err
			} else {
				outcome.State = "completed"
			}
		case ActionComment:
			if !branchSucceeded {
				outcome.State = "skipped"
			} else if _, err := jiraClient.AddComment(ctx, result.Issue.Key, fmt.Sprintf("Started work on branch `%s`.", result.BranchName), nil); err != nil {
				outcome.State = "failed"
				outcome.Err = err
			} else {
				outcome.State = "completed"
			}
		default:
			outcome.State = "skipped"
		}
		outcomes = append(outcomes, outcome)
	}
	return outcomes
}

func StartTransition(ctx context.Context, jiraClient JiraClient, key string) (jira.Transition, bool, error) {
	transitions, err := jiraClient.GetTransitions(ctx, key)
	if err != nil {
		return jira.Transition{}, false, err
	}
	transition, ok := ChooseStartTransition(transitions)
	return transition, ok, nil
}

func ChooseStartTransition(transitions []jira.Transition) (jira.Transition, bool) {
	bestIndex := -1
	bestScore := 0
	for index, transition := range transitions {
		if !transition.IsAvailable || hasRequiredTransitionFields(transition) {
			continue
		}
		score := startTransitionScore(transition)
		if score > bestScore {
			bestIndex = index
			bestScore = score
		}
	}
	if bestIndex < 0 {
		return jira.Transition{}, false
	}
	return transitions[bestIndex], true
}

func hasRequiredTransitionFields(transition jira.Transition) bool {
	for _, field := range transition.Fields {
		if field.Required {
			return true
		}
	}
	return false
}

func startTransitionScore(transition jira.Transition) int {
	text := strings.ToLower(strings.Join([]string{transition.ToStatus, transition.Name}, " "))
	switch {
	case strings.Contains(text, "in progress"):
		return 30
	case strings.Contains(text, "doing"):
		return 20
	case strings.Contains(text, "started"):
		return 10
	default:
		return 0
	}
}
