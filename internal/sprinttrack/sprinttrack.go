package sprinttrack

import (
	"context"
	"fmt"
	"strings"

	"github.com/jcharette/jira-tui/internal/jira"
)

type Client interface {
	GetBoardSprints(ctx context.Context, boardID int, states []string, startAt, maxResults int) (jira.SprintPage, error)
	MoveIssuesToSprint(ctx context.Context, sprintID int, issueKeys []string) error
	SearchBoardIssues(ctx context.Context, boardID int, jql string, maxResults int) ([]jira.Issue, error)
}

type Result struct {
	Sprint  jira.Sprint
	Missing []string
	Applied bool
}

func AddToActiveSprint(ctx context.Context, client Client, boardID int, issueKeys []string) (Result, error) {
	keys := normalizedIssueKeys(issueKeys)
	if boardID <= 0 || len(keys) == 0 {
		return Result{}, nil
	}
	sprint, ok, err := activeSprintForBoard(ctx, client, boardID)
	if err != nil {
		return Result{}, err
	}
	if !ok {
		return Result{}, fmt.Errorf("no active sprint found for board %d", boardID)
	}
	if err := client.MoveIssuesToSprint(ctx, sprint.ID, keys); err != nil {
		return Result{}, fmt.Errorf("add %s to sprint %s: %w", strings.Join(keys, ", "), sprintName(sprint), err)
	}
	missing, err := missingFromBoard(ctx, client, sprint.BoardID, keys)
	if err != nil {
		return Result{}, err
	}
	return Result{Sprint: sprint, Missing: missing, Applied: true}, nil
}

func activeSprintForBoard(ctx context.Context, client Client, boardID int) (jira.Sprint, bool, error) {
	page, err := client.GetBoardSprints(ctx, boardID, []string{"active"}, 0, 10)
	if err != nil {
		return jira.Sprint{}, false, fmt.Errorf("load active sprint for board %d: %w", boardID, err)
	}
	for _, sprint := range page.Sprints {
		if strings.EqualFold(sprint.State, "active") {
			if sprint.BoardID == 0 {
				sprint.BoardID = boardID
			}
			return sprint, true, nil
		}
	}
	return jira.Sprint{}, false, nil
}

func missingFromBoard(ctx context.Context, client Client, boardID int, issueKeys []string) ([]string, error) {
	var missing []string
	for _, key := range issueKeys {
		matches, err := client.SearchBoardIssues(ctx, boardID, "key = "+key, 1)
		if err != nil {
			return nil, fmt.Errorf("check board visibility for %s: %w", key, err)
		}
		if len(matches) == 0 {
			missing = append(missing, key)
		}
	}
	return missing, nil
}

func normalizedIssueKeys(issueKeys []string) []string {
	keys := make([]string, 0, len(issueKeys))
	seen := map[string]bool{}
	for _, key := range issueKeys {
		key = strings.ToUpper(strings.TrimSpace(key))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	return keys
}

func sprintName(sprint jira.Sprint) string {
	if strings.TrimSpace(sprint.Name) != "" {
		return sprint.Name
	}
	return fmt.Sprintf("Sprint %d", sprint.ID)
}
