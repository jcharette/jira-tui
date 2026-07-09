package worker

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/jcharette/jira-tui/internal/startworkflow"
)

func TestPoolSearchIssuesSuccess(t *testing.T) {
	pool := NewPool(&fakeIssueSearcher{
		issues: []jira.Issue{{Key: "ABC-1"}},
	}, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   7,
		Kind: KindSearchIssues,
		SearchIssues: &SearchIssuesRequest{
			JQL:        "project = ABC",
			MaxResults: 50,
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 7 {
		t.Fatalf("ID = %d", result.ID)
	}
	if result.Kind != KindSearchIssues {
		t.Fatalf("Kind = %s", result.Kind)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if len(result.SearchIssues.Issues) != 1 || result.SearchIssues.Issues[0].Key != "ABC-1" {
		t.Fatalf("Issues = %#v", result.SearchIssues.Issues)
	}
}

func TestPoolGetCurrentUserSuccess(t *testing.T) {
	pool := NewPool(&fakeIssueSearcher{
		currentUser: jira.User{AccountID: "account-123", DisplayName: "Person Example"},
	}, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:             8,
		Kind:           KindGetCurrentUser,
		GetCurrentUser: &GetCurrentUserRequest{},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 8 || result.Kind != KindGetCurrentUser {
		t.Fatalf("result = %#v", result)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if result.GetCurrentUser == nil || result.GetCurrentUser.User.AccountID != "account-123" {
		t.Fatalf("GetCurrentUser = %#v", result.GetCurrentUser)
	}
}

func TestPoolSearchIssuesFetchesMissingParents(t *testing.T) {
	pool := NewPool(&fakeIssueSearcher{
		issues: []jira.Issue{
			{Key: "ABC-2", Summary: "Child", ParentKey: "ABC-1", ParentSummary: "Parent"},
		},
		details: map[string]jira.IssueDetail{
			"ABC-1": {Issue: jira.Issue{Key: "ABC-1", Summary: "Parent", IssueType: "Epic"}},
		},
	}, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   7,
		Kind: KindSearchIssues,
		SearchIssues: &SearchIssuesRequest{
			JQL:        "project = ABC",
			MaxResults: 50,
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	keys := []string{result.SearchIssues.Issues[0].Key, result.SearchIssues.Issues[1].Key}
	want := []string{"ABC-1", "ABC-2"}
	for index := range want {
		if keys[index] != want[index] {
			t.Fatalf("keys = %#v", keys)
		}
	}
}

func TestPoolSearchIssuesAddsKnownSubtasks(t *testing.T) {
	pool := NewPool(&fakeIssueSearcher{
		issues: []jira.Issue{
			{
				Key:       "ABC-1",
				Summary:   "Parent",
				IssueType: "Task",
				Subtasks: []jira.Issue{
					{Key: "ABC-2", Summary: "Subtask", IssueType: "Sub-task", ParentKey: "ABC-1", ParentSummary: "Parent"},
				},
			},
		},
	}, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   7,
		Kind: KindSearchIssues,
		SearchIssues: &SearchIssuesRequest{
			JQL:        "project = ABC",
			MaxResults: 50,
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	keys := []string{result.SearchIssues.Issues[0].Key, result.SearchIssues.Issues[1].Key}
	want := []string{"ABC-1", "ABC-2"}
	for index := range want {
		if keys[index] != want[index] {
			t.Fatalf("keys = %#v", keys)
		}
	}
}

func TestPoolSearchIssuesError(t *testing.T) {
	searchErr := errors.New("jira unavailable")
	pool := NewPool(&fakeIssueSearcher{err: searchErr}, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:           7,
		Kind:         KindSearchIssues,
		SearchIssues: &SearchIssuesRequest{JQL: "project = ABC"},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if !errors.Is(result.Err, searchErr) {
		t.Fatalf("Err = %v", result.Err)
	}
}

func TestPoolSearchIssuesFetchesChildIssues(t *testing.T) {
	pool := NewPool(&fakeIssueSearcher{
		issues: []jira.Issue{
			{Key: "ABC-1", Summary: "Parent", IssueType: "Story"},
		},
		searchResults: map[string][]jira.Issue{
			"parent in (ABC-1) ORDER BY key ASC": {
				{Key: "ABC-2", Summary: "Subtask", Status: "To Do", Priority: "P4", IssueType: "Sub-task", IsSubtask: true, ParentKey: "ABC-1"},
			},
		},
	}, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   7,
		Kind: KindSearchIssues,
		SearchIssues: &SearchIssuesRequest{
			JQL:             "project = ABC",
			MaxResults:      50,
			IncludeChildren: true,
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	keys := []string{result.SearchIssues.Issues[0].Key, result.SearchIssues.Issues[1].Key}
	want := []string{"ABC-1", "ABC-2"}
	for index := range want {
		if keys[index] != want[index] {
			t.Fatalf("keys = %#v", keys)
		}
	}
}

func TestPoolSearchIssuesSkipsChildIssuesUnlessRequested(t *testing.T) {
	searcher := &fakeIssueSearcher{
		issues: []jira.Issue{
			{Key: "ABC-1", Summary: "Parent", IssueType: "Story"},
		},
		searchResults: map[string][]jira.Issue{
			"parent in (ABC-1) ORDER BY key ASC": {
				{Key: "ABC-2", Summary: "Child", IssueType: "Story", ParentKey: "ABC-1"},
			},
		},
	}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   17,
		Kind: KindSearchIssues,
		SearchIssues: &SearchIssuesRequest{
			JQL:        "project = ABC",
			MaxResults: 50,
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if len(result.SearchIssues.Issues) != 1 || result.SearchIssues.Issues[0].Key != "ABC-1" {
		t.Fatalf("Issues = %#v", result.SearchIssues.Issues)
	}
	for _, jql := range searcher.searches {
		if jql == "parent in (ABC-1) ORDER BY key ASC" {
			t.Fatalf("unexpected child lookup query: searches=%#v", searcher.searches)
		}
	}
}

func TestPoolGetBoardsSuccess(t *testing.T) {
	pool := NewPool(&fakeIssueSearcher{
		boardPage: jira.BoardPage{
			Boards:     []jira.Board{{ID: 100, Name: "ABC Scrum"}},
			StartAt:    25,
			MaxResults: 25,
			Total:      50,
			IsLast:     false,
		},
	}, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   21,
		Kind: KindGetBoards,
		GetBoards: &GetBoardsRequest{
			ProjectKey: "ABC",
			StartAt:    25,
			MaxResults: 25,
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 21 || result.Kind != KindGetBoards {
		t.Fatalf("result identity = %#v", result)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if result.GetBoards.ProjectKey != "ABC" {
		t.Fatalf("ProjectKey = %q", result.GetBoards.ProjectKey)
	}
	if len(result.GetBoards.Page.Boards) != 1 || result.GetBoards.Page.Boards[0].ID != 100 {
		t.Fatalf("Page = %#v", result.GetBoards.Page)
	}
	if result.GetBoards.Page.StartAt != 25 || result.GetBoards.Page.Total != 50 || result.GetBoards.Page.IsLast {
		t.Fatalf("Page pagination = %#v", result.GetBoards.Page)
	}
}

func TestPoolGetBoardSprintsSuccess(t *testing.T) {
	pool := NewPool(&fakeIssueSearcher{
		sprintPage: jira.SprintPage{
			BoardID:    100,
			Sprints:    []jira.Sprint{{ID: 300, BoardID: 100, Name: "Sprint 42", State: "active"}},
			StartAt:    0,
			MaxResults: 25,
			Total:      1,
			IsLast:     true,
		},
	}, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   22,
		Kind: KindGetBoardSprints,
		GetBoardSprints: &GetBoardSprintsRequest{
			BoardID:    100,
			States:     []string{"active", "future"},
			StartAt:    0,
			MaxResults: 25,
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 22 || result.Kind != KindGetBoardSprints {
		t.Fatalf("result identity = %#v", result)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if result.GetBoardSprints.BoardID != 100 {
		t.Fatalf("BoardID = %d", result.GetBoardSprints.BoardID)
	}
	if len(result.GetBoardSprints.Page.Sprints) != 1 || result.GetBoardSprints.Page.Sprints[0].ID != 300 {
		t.Fatalf("Page = %#v", result.GetBoardSprints.Page)
	}
	if !result.GetBoardSprints.Page.IsLast {
		t.Fatalf("Page pagination = %#v", result.GetBoardSprints.Page)
	}
}

func TestPoolGetBoardSprintsRejectsMissingBoard(t *testing.T) {
	pool := NewPool(&fakeIssueSearcher{}, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:              23,
		Kind:            KindGetBoardSprints,
		GetBoardSprints: &GetBoardSprintsRequest{},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if !errors.Is(result.Err, ErrInvalidRequest) {
		t.Fatalf("Err = %v", result.Err)
	}
}

func TestPoolExpandIssuesFetchesOpenChildren(t *testing.T) {
	pool := NewPool(&fakeIssueSearcher{
		searchResults: map[string][]jira.Issue{
			"parent = ABC-1 AND statusCategory != Done ORDER BY key ASC": {
				{Key: "ABC-2", Summary: "Open child", ParentKey: "ABC-1"},
			},
		},
	}, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   12,
		Kind: KindExpandIssues,
		ExpandIssues: &ExpandIssuesRequest{
			ParentKey:  "ABC-1",
			Mode:       ExpandModeOpen,
			MaxResults: 25,
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 12 {
		t.Fatalf("ID = %d", result.ID)
	}
	if result.Kind != KindExpandIssues {
		t.Fatalf("Kind = %s", result.Kind)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if result.ExpandIssues.ParentKey != "ABC-1" || result.ExpandIssues.Mode != ExpandModeOpen {
		t.Fatalf("ExpandIssues = %#v", result.ExpandIssues)
	}
	if len(result.ExpandIssues.Issues) != 1 || result.ExpandIssues.Issues[0].Key != "ABC-2" {
		t.Fatalf("Issues = %#v", result.ExpandIssues.Issues)
	}
}

func TestPoolExpandIssuesFetchesAllChildren(t *testing.T) {
	pool := NewPool(&fakeIssueSearcher{
		searchResults: map[string][]jira.Issue{
			"parent = ABC-1 ORDER BY key ASC": {
				{Key: "ABC-3", Summary: "Done child", ParentKey: "ABC-1", Status: "Done"},
			},
		},
	}, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   13,
		Kind: KindExpandIssues,
		ExpandIssues: &ExpandIssuesRequest{
			ParentKey: "ABC-1",
			Mode:      ExpandModeAll,
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if result.ExpandIssues.Mode != ExpandModeAll {
		t.Fatalf("Mode = %q", result.ExpandIssues.Mode)
	}
	if len(result.ExpandIssues.Issues) != 1 || result.ExpandIssues.Issues[0].Key != "ABC-3" {
		t.Fatalf("Issues = %#v", result.ExpandIssues.Issues)
	}
}

func TestPoolGetIssueSuccess(t *testing.T) {
	pool := NewPool(&fakeIssueSearcher{
		detail: jira.IssueDetail{Issue: jira.Issue{Key: "ABC-1"}, Description: "Loaded"},
	}, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:       8,
		Kind:     KindGetIssue,
		GetIssue: &GetIssueRequest{Key: "ABC-1"},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 8 {
		t.Fatalf("ID = %d", result.ID)
	}
	if result.Kind != KindGetIssue {
		t.Fatalf("Kind = %s", result.Kind)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if result.GetIssue.Detail.Key != "ABC-1" {
		t.Fatalf("Detail = %#v", result.GetIssue.Detail)
	}
	if result.GetIssue.Detail.Description != "Loaded" {
		t.Fatalf("Description = %q", result.GetIssue.Detail.Description)
	}
}

func TestPoolGetCommentsSuccess(t *testing.T) {
	pool := NewPool(&fakeIssueSearcher{
		comments: []jira.Comment{{ID: "10001", Body: "Loaded comment"}},
	}, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:          9,
		Kind:        KindGetComments,
		GetComments: &GetCommentsRequest{Key: "ABC-1", MaxResults: 5},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 9 {
		t.Fatalf("ID = %d", result.ID)
	}
	if result.Kind != KindGetComments {
		t.Fatalf("Kind = %s", result.Kind)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if len(result.GetComments.Comments) != 1 || result.GetComments.Comments[0].ID != "10001" {
		t.Fatalf("Comments = %#v", result.GetComments.Comments)
	}
	if result.GetComments.Comments[0].Body != "Loaded comment" {
		t.Fatalf("Body = %q", result.GetComments.Comments[0].Body)
	}
}

func TestPoolAddCommentSuccess(t *testing.T) {
	searcher := &fakeIssueSearcher{
		addedComment: jira.Comment{ID: "10002", Body: "Posted"},
	}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   10,
		Kind: KindAddComment,
		AddComment: &AddCommentRequest{
			Key:      "ABC-1",
			Body:     "Please review @Jane Doe.",
			Mentions: []jira.Mention{{AccountID: "abc-123", Text: "@Jane Doe"}},
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 10 {
		t.Fatalf("ID = %d", result.ID)
	}
	if result.Kind != KindAddComment {
		t.Fatalf("Kind = %s", result.Kind)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if result.AddComment.Comment.ID != "10002" {
		t.Fatalf("Comment = %#v", result.AddComment.Comment)
	}
	if result.AddComment.Comment.Body != "Posted" {
		t.Fatalf("Body = %q", result.AddComment.Comment.Body)
	}
	if len(searcher.addMentions) != 1 || searcher.addMentions[0].AccountID != "abc-123" {
		t.Fatalf("addMentions = %#v", searcher.addMentions)
	}
}

func TestPoolUpdateCommentSuccess(t *testing.T) {
	searcher := &fakeIssueSearcher{
		updatedComment: jira.Comment{ID: "10001", Body: "Updated"},
	}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   11,
		Kind: KindUpdateComment,
		UpdateComment: &UpdateCommentRequest{
			Key:       "ABC-1",
			CommentID: "10001",
			Body:      "Updated @Jane Doe.",
			Mentions:  []jira.Mention{{AccountID: "abc-123", Text: "@Jane Doe"}},
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 11 {
		t.Fatalf("ID = %d", result.ID)
	}
	if result.Kind != KindUpdateComment {
		t.Fatalf("Kind = %s", result.Kind)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if result.UpdateComment.Comment.ID != "10001" || result.UpdateComment.Comment.Body != "Updated" {
		t.Fatalf("UpdateComment = %#v", result.UpdateComment)
	}
	if searcher.updateCommentKey != "ABC-1" || searcher.updateCommentID != "10001" || searcher.updateCommentBody != "Updated @Jane Doe." {
		t.Fatalf("update request = %s/%s %q", searcher.updateCommentKey, searcher.updateCommentID, searcher.updateCommentBody)
	}
	if len(searcher.updateMentions) != 1 || searcher.updateMentions[0].AccountID != "abc-123" {
		t.Fatalf("updateMentions = %#v", searcher.updateMentions)
	}
}

func TestPoolGetTransitionsSuccess(t *testing.T) {
	pool := NewPool(&fakeIssueSearcher{
		transitions: []jira.Transition{{ID: "21", Name: "Start Progress", ToStatus: "In Progress"}},
	}, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:             14,
		Kind:           KindGetTransitions,
		GetTransitions: &GetTransitionsRequest{Key: "ABC-1"},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 14 {
		t.Fatalf("ID = %d", result.ID)
	}
	if result.Kind != KindGetTransitions {
		t.Fatalf("Kind = %s", result.Kind)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if result.GetTransitions.Key != "ABC-1" {
		t.Fatalf("Key = %q", result.GetTransitions.Key)
	}
	if len(result.GetTransitions.Transitions) != 1 || result.GetTransitions.Transitions[0].ID != "21" {
		t.Fatalf("Transitions = %#v", result.GetTransitions.Transitions)
	}
}

func TestPoolStartIssueAppliesConfirmedJiraActions(t *testing.T) {
	searcher := &fakeIssueSearcher{
		transitions: []jira.Transition{{ID: "21", Name: "Start Progress", ToStatus: "In Progress", IsAvailable: true}},
		currentUser: jira.User{AccountID: "account-456", DisplayName: "Example User"},
	}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	result := startworkflow.Result{
		Issue:      jira.Issue{Key: "ABC-123", Summary: "Prepare release"},
		RepoPath:   "/tmp/repo",
		BranchName: "abc-123-prepare-release",
		Actions: []startworkflow.ActionPlan{
			{Kind: startworkflow.ActionBranch, Label: "Create or switch branch", Required: true},
			{Kind: startworkflow.ActionAssign, Label: "Assign to me"},
			{Kind: startworkflow.ActionTransition, Label: "Move to In Progress"},
			{Kind: startworkflow.ActionComment, Label: "Add branch comment"},
		},
	}
	err := pool.Submit(Request{
		ID:         15,
		Kind:       KindStartIssue,
		StartIssue: &StartIssueRequest{Result: result, BranchSucceeded: true},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	workerResult := readResult(t, pool)
	if workerResult.Err != nil {
		t.Fatalf("Err = %v", workerResult.Err)
	}
	if workerResult.StartIssue == nil || workerResult.StartIssue.Key != "ABC-123" {
		t.Fatalf("StartIssue = %#v", workerResult.StartIssue)
	}
	if len(workerResult.StartIssue.Outcomes) != 3 {
		t.Fatalf("outcomes = %#v", workerResult.StartIssue.Outcomes)
	}
	if searcher.updateAssigneeKey != "ABC-123" || searcher.updateAssigneeValue.AccountID != "account-456" {
		t.Fatalf("assignee update = %q %#v", searcher.updateAssigneeKey, searcher.updateAssigneeValue)
	}
	if searcher.transitionKey != "ABC-123" || searcher.transitionID != "21" {
		t.Fatalf("transition update = %q %q", searcher.transitionKey, searcher.transitionID)
	}
	if !strings.Contains(searcher.addCommentBody, "abc-123-prepare-release") {
		t.Fatalf("comment body = %q", searcher.addCommentBody)
	}
}

func TestPoolTransitionIssueSuccess(t *testing.T) {
	searcher := &fakeIssueSearcher{}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   15,
		Kind: KindTransitionIssue,
		TransitionIssue: &TransitionIssueRequest{
			Key:          "ABC-1",
			TransitionID: "21",
			ToStatus:     "In Progress",
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 15 {
		t.Fatalf("ID = %d", result.ID)
	}
	if result.Kind != KindTransitionIssue {
		t.Fatalf("Kind = %s", result.Kind)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if searcher.transitionKey != "ABC-1" || searcher.transitionID != "21" {
		t.Fatalf("transition = %s/%s", searcher.transitionKey, searcher.transitionID)
	}
	if result.TransitionIssue.Key != "ABC-1" || result.TransitionIssue.ToStatus != "In Progress" {
		t.Fatalf("TransitionIssue = %#v", result.TransitionIssue)
	}
}

func TestPoolTransitionIssuePassesFieldValues(t *testing.T) {
	searcher := &fakeIssueSearcher{}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   12,
		Kind: KindTransitionIssue,
		TransitionIssue: &TransitionIssueRequest{
			Key:          "ABC-1",
			TransitionID: "31",
			ToStatus:     "Done",
			Fields: []jira.TransitionFieldValue{
				{FieldID: "resolution", Option: jira.FieldOption{ID: "10000", Name: "Done"}},
				{FieldID: "comment", Text: "Ship it"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.Err != nil {
		t.Fatalf("Result.Err = %v", result.Err)
	}
	if searcher.transitionRequest.TransitionID != "31" {
		t.Fatalf("transition request = %#v", searcher.transitionRequest)
	}
	if len(searcher.transitionRequest.Fields) != 2 {
		t.Fatalf("transition fields = %#v", searcher.transitionRequest.Fields)
	}
}

func TestPoolTransitionIssueError(t *testing.T) {
	transitionErr := errors.New("jira rejected transition")
	pool := NewPool(&fakeIssueSearcher{transitionErr: transitionErr}, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   16,
		Kind: KindTransitionIssue,
		TransitionIssue: &TransitionIssueRequest{
			Key:          "ABC-1",
			TransitionID: "21",
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if !errors.Is(result.Err, transitionErr) {
		t.Fatalf("Err = %v", result.Err)
	}
}

func TestPoolGetEditMetadataSuccess(t *testing.T) {
	pool := NewPool(&fakeIssueSearcher{
		editMetadata: jira.EditMetadata{
			Summary: jira.EditField{ID: "summary", Name: "Summary", Editable: true, Required: true},
		},
	}, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:              19,
		Kind:            KindGetEditMetadata,
		GetEditMetadata: &GetEditMetadataRequest{Key: "ABC-1"},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 19 {
		t.Fatalf("ID = %d", result.ID)
	}
	if result.Kind != KindGetEditMetadata {
		t.Fatalf("Kind = %s", result.Kind)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if result.GetEditMetadata.Key != "ABC-1" {
		t.Fatalf("Key = %q", result.GetEditMetadata.Key)
	}
	if !result.GetEditMetadata.Metadata.Summary.Editable {
		t.Fatalf("Metadata = %#v", result.GetEditMetadata.Metadata)
	}
}

func TestPoolGetCreateIssueTypesSuccess(t *testing.T) {
	pool := NewPool(&fakeIssueSearcher{
		createIssueTypes: []jira.CreateIssueType{
			{ID: "10001", Name: "Task"},
			{ID: "10002", Name: "Bug"},
		},
	}, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   31,
		Kind: KindGetCreateIssueTypes,
		GetCreateIssueTypes: &GetCreateIssueTypesRequest{
			ProjectKey: "ABC",
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 31 {
		t.Fatalf("ID = %d", result.ID)
	}
	if result.Kind != KindGetCreateIssueTypes {
		t.Fatalf("Kind = %s", result.Kind)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if result.GetCreateIssueTypes.ProjectKey != "ABC" {
		t.Fatalf("ProjectKey = %q", result.GetCreateIssueTypes.ProjectKey)
	}
	if len(result.GetCreateIssueTypes.IssueTypes) != 2 || result.GetCreateIssueTypes.IssueTypes[0].Name != "Task" {
		t.Fatalf("IssueTypes = %#v", result.GetCreateIssueTypes.IssueTypes)
	}
}

func TestPoolGetCreateFieldsSuccess(t *testing.T) {
	pool := NewPool(&fakeIssueSearcher{
		createFields: []jira.CreateField{
			{ID: "summary", Name: "Summary", Required: true},
			{ID: "priority", Name: "Priority", AllowedValues: []jira.FieldOption{{ID: "3", Name: "Medium"}}},
		},
	}, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   32,
		Kind: KindGetCreateFields,
		GetCreateFields: &GetCreateFieldsRequest{
			ProjectKey:  "ABC",
			IssueTypeID: "10001",
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 32 {
		t.Fatalf("ID = %d", result.ID)
	}
	if result.Kind != KindGetCreateFields {
		t.Fatalf("Kind = %s", result.Kind)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if result.GetCreateFields.ProjectKey != "ABC" || result.GetCreateFields.IssueTypeID != "10001" {
		t.Fatalf("GetCreateFields = %#v", result.GetCreateFields)
	}
	if len(result.GetCreateFields.Fields) != 2 || !result.GetCreateFields.Fields[0].Required {
		t.Fatalf("Fields = %#v", result.GetCreateFields.Fields)
	}
}

func TestPoolSearchFieldOptionsSuccess(t *testing.T) {
	searcher := &fakeIssueSearcher{
		fieldOptions: []jira.FieldOption{{ID: "101", Name: "Platform"}},
	}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   33,
		Kind: KindSearchFieldOptions,
		SearchFieldOptions: &SearchFieldOptionsRequest{
			FieldID:         "customfield_10010",
			AutoCompleteURL: "https://example.atlassian.net/rest/api/3/customFieldOption/suggest",
			Query:           "plat",
			MaxResults:      25,
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 33 {
		t.Fatalf("ID = %d", result.ID)
	}
	if result.Kind != KindSearchFieldOptions {
		t.Fatalf("Kind = %s", result.Kind)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if result.SearchFieldOptions.FieldID != "customfield_10010" || result.SearchFieldOptions.Query != "plat" {
		t.Fatalf("SearchFieldOptions = %#v", result.SearchFieldOptions)
	}
	if searcher.fieldOptionURL != "https://example.atlassian.net/rest/api/3/customFieldOption/suggest" || searcher.fieldOptionQuery != "plat" || searcher.fieldOptionMaxResults != 25 {
		t.Fatalf("searcher option request = %q/%q/%d", searcher.fieldOptionURL, searcher.fieldOptionQuery, searcher.fieldOptionMaxResults)
	}
	if !reflect.DeepEqual(result.SearchFieldOptions.Options, []jira.FieldOption{{ID: "101", Name: "Platform"}}) {
		t.Fatalf("Options = %#v", result.SearchFieldOptions.Options)
	}
}

func TestPoolCreateIssueSuccess(t *testing.T) {
	searcher := &fakeIssueSearcher{
		createdIssue: jira.Issue{Key: "ABC-123", Summary: "New platform task"},
	}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   34,
		Kind: KindCreateIssue,
		CreateIssue: &CreateIssueRequest{
			ProjectKey:  "ABC",
			IssueTypeID: "10001",
			ParentKey:   "ABC-1",
			Summary:     "New platform task",
			Description: "Description body",
			Fields: []jira.CreateIssueFieldValue{
				{FieldID: "priority", SchemaSystem: "priority", Option: jira.FieldOption{ID: "3", Name: "Medium"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 34 {
		t.Fatalf("ID = %d", result.ID)
	}
	if result.Kind != KindCreateIssue {
		t.Fatalf("Kind = %s", result.Kind)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if searcher.createIssueRequest.ProjectKey != "ABC" || searcher.createIssueRequest.IssueTypeID != "10001" {
		t.Fatalf("create request = %#v", searcher.createIssueRequest)
	}
	if searcher.createIssueRequest.ParentKey != "ABC-1" {
		t.Fatalf("parent key = %q", searcher.createIssueRequest.ParentKey)
	}
	if len(searcher.createIssueRequest.Fields) != 1 || searcher.createIssueRequest.Fields[0].FieldID != "priority" {
		t.Fatalf("create fields = %#v", searcher.createIssueRequest.Fields)
	}
	if result.CreateIssue.Issue.Key != "ABC-123" {
		t.Fatalf("CreateIssue = %#v", result.CreateIssue)
	}
}

func TestPoolCreateIssueAddsCreatedIssueToConfiguredActiveSprint(t *testing.T) {
	searcher := &fakeIssueSearcher{
		createdIssue: jira.Issue{Key: "ABC-123", Summary: "New toil"},
		sprintPage: jira.SprintPage{
			Sprints: []jira.Sprint{{ID: 300, BoardID: 100, Name: "Sprint 42", State: "active"}},
			IsLast:  true,
		},
		boardIssuesByJQL: map[string][]jira.Issue{
			"key = ABC-123": {{Key: "ABC-123"}},
		},
	}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   35,
		Kind: KindCreateIssue,
		CreateIssue: &CreateIssueRequest{
			ProjectKey:    "ABC",
			IssueTypeID:   "10001",
			Summary:       "New toil",
			SprintBoardID: 100,
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if searcher.sprintBoardID != 100 || searcher.moveSprintID != 300 || !reflect.DeepEqual(searcher.moveIssueKeys, []string{"ABC-123"}) {
		t.Fatalf("sprint tracking = board %d sprint %d keys %#v", searcher.sprintBoardID, searcher.moveSprintID, searcher.moveIssueKeys)
	}
	if result.CreateIssue.Sprint == nil || result.CreateIssue.Sprint.Name != "Sprint 42" {
		t.Fatalf("CreateIssue sprint = %#v", result.CreateIssue)
	}
}

func TestPoolCreateIssueAssignsCreatedIssueToCurrentUserWhenRequested(t *testing.T) {
	searcher := &fakeIssueSearcher{
		createdIssue: jira.Issue{Key: "ABC-123", Summary: "New toil"},
		currentUser:  jira.User{AccountID: "account-123", DisplayName: "Jon"},
	}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   36,
		Kind: KindCreateIssue,
		CreateIssue: &CreateIssueRequest{
			ProjectKey:        "ABC",
			IssueTypeID:       "10001",
			Summary:           "New toil",
			AssignCurrentUser: true,
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if searcher.updateAssigneeKey != "ABC-123" || searcher.updateAssigneeValue.AccountID != "account-123" {
		t.Fatalf("assignee update = %s/%#v", searcher.updateAssigneeKey, searcher.updateAssigneeValue)
	}
	if result.CreateIssue.Assignee == nil || result.CreateIssue.Assignee.DisplayName != "Jon" {
		t.Fatalf("CreateIssue assignee = %#v", result.CreateIssue)
	}
}

func TestPoolUpdateSummarySuccess(t *testing.T) {
	searcher := &fakeIssueSearcher{}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   20,
		Kind: KindUpdateSummary,
		UpdateSummary: &UpdateSummaryRequest{
			Key:     "ABC-1",
			Summary: "Updated summary",
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 20 {
		t.Fatalf("ID = %d", result.ID)
	}
	if result.Kind != KindUpdateSummary {
		t.Fatalf("Kind = %s", result.Kind)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if searcher.updateSummaryKey != "ABC-1" || searcher.updateSummaryValue != "Updated summary" {
		t.Fatalf("update summary = %s/%s", searcher.updateSummaryKey, searcher.updateSummaryValue)
	}
	if result.UpdateSummary.Key != "ABC-1" || result.UpdateSummary.Summary != "Updated summary" {
		t.Fatalf("UpdateSummary = %#v", result.UpdateSummary)
	}
}

func TestPoolUpdateSummaryError(t *testing.T) {
	updateErr := errors.New("jira rejected summary")
	pool := NewPool(&fakeIssueSearcher{updateSummaryErr: updateErr}, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:            21,
		Kind:          KindUpdateSummary,
		UpdateSummary: &UpdateSummaryRequest{Key: "ABC-1", Summary: "Updated summary"},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if !errors.Is(result.Err, updateErr) {
		t.Fatalf("Err = %v", result.Err)
	}
}

func TestPoolUpdateDescriptionSuccess(t *testing.T) {
	searcher := &fakeIssueSearcher{}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   22,
		Kind: KindUpdateDescription,
		UpdateDescription: &UpdateDescriptionRequest{
			Key:         "ABC-1",
			Description: "Updated description",
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 22 {
		t.Fatalf("ID = %d", result.ID)
	}
	if result.Kind != KindUpdateDescription {
		t.Fatalf("Kind = %s", result.Kind)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if searcher.updateDescriptionKey != "ABC-1" || searcher.updateDescriptionValue != "Updated description" {
		t.Fatalf("update description = %s/%s", searcher.updateDescriptionKey, searcher.updateDescriptionValue)
	}
	if result.UpdateDescription.Key != "ABC-1" || result.UpdateDescription.Description != "Updated description" {
		t.Fatalf("UpdateDescription = %#v", result.UpdateDescription)
	}
}

func TestPoolUpdateDescriptionError(t *testing.T) {
	updateErr := errors.New("jira rejected description")
	pool := NewPool(&fakeIssueSearcher{updateDescriptionErr: updateErr}, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:                23,
		Kind:              KindUpdateDescription,
		UpdateDescription: &UpdateDescriptionRequest{Key: "ABC-1", Description: "Updated description"},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if !errors.Is(result.Err, updateErr) {
		t.Fatalf("Err = %v", result.Err)
	}
}

func TestPoolUpdatePrioritySuccess(t *testing.T) {
	searcher := &fakeIssueSearcher{}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   22,
		Kind: KindUpdatePriority,
		UpdatePriority: &UpdatePriorityRequest{
			Key:      "ABC-1",
			Priority: jira.FieldOption{ID: "3", Name: "Medium"},
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 22 {
		t.Fatalf("ID = %d", result.ID)
	}
	if result.Kind != KindUpdatePriority {
		t.Fatalf("Kind = %s", result.Kind)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if searcher.updatePriorityKey != "ABC-1" || searcher.updatePriorityValue.ID != "3" {
		t.Fatalf("update priority = %s/%#v", searcher.updatePriorityKey, searcher.updatePriorityValue)
	}
	if result.UpdatePriority.Key != "ABC-1" || result.UpdatePriority.Priority.Name != "Medium" {
		t.Fatalf("UpdatePriority = %#v", result.UpdatePriority)
	}
}

func TestPoolUpdatePriorityError(t *testing.T) {
	updateErr := errors.New("jira rejected priority")
	pool := NewPool(&fakeIssueSearcher{updatePriorityErr: updateErr}, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   23,
		Kind: KindUpdatePriority,
		UpdatePriority: &UpdatePriorityRequest{
			Key:      "ABC-1",
			Priority: jira.FieldOption{ID: "3", Name: "Medium"},
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if !errors.Is(result.Err, updateErr) {
		t.Fatalf("Err = %v", result.Err)
	}
}

func TestPoolUpdateLabelsSuccess(t *testing.T) {
	searcher := &fakeIssueSearcher{}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   24,
		Kind: KindUpdateLabels,
		UpdateLabels: &UpdateLabelsRequest{
			Key:    "ABC-1",
			Labels: []string{"platform", "needs-review"},
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 24 {
		t.Fatalf("ID = %d", result.ID)
	}
	if result.Kind != KindUpdateLabels {
		t.Fatalf("Kind = %s", result.Kind)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if searcher.updateLabelsKey != "ABC-1" || !reflect.DeepEqual(searcher.updateLabelsValue, []string{"platform", "needs-review"}) {
		t.Fatalf("update labels = %s/%#v", searcher.updateLabelsKey, searcher.updateLabelsValue)
	}
	if result.UpdateLabels.Key != "ABC-1" || !reflect.DeepEqual(result.UpdateLabels.Labels, []string{"platform", "needs-review"}) {
		t.Fatalf("UpdateLabels = %#v", result.UpdateLabels)
	}
}

func TestPoolUpdateLabelsAllowsClearing(t *testing.T) {
	searcher := &fakeIssueSearcher{}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:           25,
		Kind:         KindUpdateLabels,
		UpdateLabels: &UpdateLabelsRequest{Key: "ABC-1"},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if searcher.updateLabelsValue == nil {
		t.Fatal("expected empty labels slice to clear labels")
	}
	if len(searcher.updateLabelsValue) != 0 {
		t.Fatalf("updateLabelsValue = %#v", searcher.updateLabelsValue)
	}
}

func TestPoolUpdateLabelsError(t *testing.T) {
	updateErr := errors.New("jira rejected labels")
	pool := NewPool(&fakeIssueSearcher{updateLabelsErr: updateErr}, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   26,
		Kind: KindUpdateLabels,
		UpdateLabels: &UpdateLabelsRequest{
			Key:    "ABC-1",
			Labels: []string{"platform"},
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if !errors.Is(result.Err, updateErr) {
		t.Fatalf("Err = %v", result.Err)
	}
}

func TestPoolUpdateComponentsSuccess(t *testing.T) {
	searcher := &fakeIssueSearcher{}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	components := []jira.FieldOption{{ID: "101", Name: "Platform"}, {ID: "102", Name: "API"}}
	err := pool.Submit(Request{
		ID:   27,
		Kind: KindUpdateComponents,
		UpdateComponents: &UpdateComponentsRequest{
			Key:        "ABC-1",
			Components: components,
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 27 {
		t.Fatalf("ID = %d", result.ID)
	}
	if result.Kind != KindUpdateComponents {
		t.Fatalf("Kind = %s", result.Kind)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if searcher.updateComponentsKey != "ABC-1" || !reflect.DeepEqual(searcher.updateComponentsValue, components) {
		t.Fatalf("update components = %s/%#v", searcher.updateComponentsKey, searcher.updateComponentsValue)
	}
	if result.UpdateComponents.Key != "ABC-1" || !reflect.DeepEqual(result.UpdateComponents.Components, components) {
		t.Fatalf("UpdateComponents = %#v", result.UpdateComponents)
	}
}

func TestPoolUpdateComponentsAllowsClearing(t *testing.T) {
	searcher := &fakeIssueSearcher{}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:               28,
		Kind:             KindUpdateComponents,
		UpdateComponents: &UpdateComponentsRequest{Key: "ABC-1"},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if searcher.updateComponentsValue == nil {
		t.Fatal("expected empty components slice to clear components")
	}
	if len(searcher.updateComponentsValue) != 0 {
		t.Fatalf("updateComponentsValue = %#v", searcher.updateComponentsValue)
	}
}

func TestPoolUpdateComponentsError(t *testing.T) {
	updateErr := errors.New("jira rejected components")
	pool := NewPool(&fakeIssueSearcher{updateComponentsErr: updateErr}, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   29,
		Kind: KindUpdateComponents,
		UpdateComponents: &UpdateComponentsRequest{
			Key:        "ABC-1",
			Components: []jira.FieldOption{{ID: "101", Name: "Platform"}},
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if !errors.Is(result.Err, updateErr) {
		t.Fatalf("Err = %v", result.Err)
	}
}

func TestPoolUpdateEditFieldSuccess(t *testing.T) {
	searcher := &fakeIssueSearcher{}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	value := jira.EditFieldValue{FieldID: "customfield_10016", SchemaType: "number", Text: "8"}
	field := jira.EditField{ID: "customfield_10016", Name: "Story Points", SchemaType: "number"}
	err := pool.Submit(Request{
		ID:   30,
		Kind: KindUpdateEditField,
		UpdateEditField: &UpdateEditFieldRequest{
			Key:   "ABC-1",
			Field: field,
			Value: value,
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 30 {
		t.Fatalf("ID = %d", result.ID)
	}
	if result.Kind != KindUpdateEditField {
		t.Fatalf("Kind = %s", result.Kind)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if searcher.updateEditFieldKey != "ABC-1" || searcher.updateEditFieldValue.FieldID != "customfield_10016" || searcher.updateEditFieldValue.Text != "8" {
		t.Fatalf("update edit field = %s/%#v", searcher.updateEditFieldKey, searcher.updateEditFieldValue)
	}
	if result.UpdateEditField.Key != "ABC-1" || result.UpdateEditField.Field.Name != "Story Points" || result.UpdateEditField.Value.Text != "8" {
		t.Fatalf("UpdateEditField = %#v", result.UpdateEditField)
	}
}

func TestPoolUpdateParentSuccess(t *testing.T) {
	searcher := &fakeIssueSearcher{}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   41,
		Kind: KindUpdateParent,
		UpdateParent: &UpdateParentRequest{
			Key:     "ABC-1",
			Request: jira.UpdateParentRequest{ParentKey: "ABC-100"},
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 41 || result.Kind != KindUpdateParent || result.Err != nil {
		t.Fatalf("result = %#v", result)
	}
	if searcher.updateParentKey != "ABC-1" || searcher.updateParentRequest.ParentKey != "ABC-100" {
		t.Fatalf("update parent = %s/%#v", searcher.updateParentKey, searcher.updateParentRequest)
	}
	if result.UpdateParent.Key != "ABC-1" || result.UpdateParent.Request.ParentKey != "ABC-100" {
		t.Fatalf("UpdateParent = %#v", result.UpdateParent)
	}
}

func TestPoolUpdateTimeTrackingSuccess(t *testing.T) {
	searcher := &fakeIssueSearcher{}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	request := jira.UpdateTimeTrackingRequest{OriginalEstimate: "2d", RemainingEstimate: "3h"}
	err := pool.Submit(Request{
		ID:                 42,
		Kind:               KindUpdateTimeTracking,
		UpdateTimeTracking: &UpdateTimeTrackingRequest{Key: "ABC-1", Request: request},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 42 || result.Kind != KindUpdateTimeTracking || result.Err != nil {
		t.Fatalf("result = %#v", result)
	}
	if searcher.updateTimeTrackingKey != "ABC-1" || searcher.updateTimeTrackingRequest.OriginalEstimate != "2d" || searcher.updateTimeTrackingRequest.RemainingEstimate != "3h" {
		t.Fatalf("update time tracking = %s/%#v", searcher.updateTimeTrackingKey, searcher.updateTimeTrackingRequest)
	}
	if result.UpdateTimeTracking.Key != "ABC-1" || result.UpdateTimeTracking.Request != request {
		t.Fatalf("UpdateTimeTracking = %#v", result.UpdateTimeTracking)
	}
}

func TestPoolGetIssueLinkTypesSuccess(t *testing.T) {
	searcher := &fakeIssueSearcher{
		issueLinkTypes: []jira.IssueLinkType{{ID: "10000", Name: "Blocks", Inward: "is blocked by", Outward: "blocks"}},
	}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:                31,
		Kind:              KindGetIssueLinkTypes,
		GetIssueLinkTypes: &GetIssueLinkTypesRequest{},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 31 || result.Kind != KindGetIssueLinkTypes {
		t.Fatalf("result = %#v", result)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if len(result.GetIssueLinkTypes.Types) != 1 || result.GetIssueLinkTypes.Types[0].Name != "Blocks" {
		t.Fatalf("GetIssueLinkTypes = %#v", result.GetIssueLinkTypes)
	}
}

func TestPoolCreateIssueLinkSuccess(t *testing.T) {
	searcher := &fakeIssueSearcher{}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	request := jira.CreateIssueLinkRequest{
		SourceKey: "ABC-1",
		TargetKey: "ABC-2",
		Type:      jira.IssueLinkType{ID: "10000", Name: "Blocks", Inward: "is blocked by", Outward: "blocks"},
		Direction: "outward",
	}
	err := pool.Submit(Request{
		ID:   32,
		Kind: KindCreateIssueLink,
		CreateIssueLink: &CreateIssueLinkRequest{
			Request: request,
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 32 || result.Kind != KindCreateIssueLink {
		t.Fatalf("result = %#v", result)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if searcher.issueLinkRequest.SourceKey != "ABC-1" || searcher.issueLinkRequest.TargetKey != "ABC-2" || searcher.issueLinkRequest.Direction != "outward" {
		t.Fatalf("issueLinkRequest = %#v", searcher.issueLinkRequest)
	}
	if result.CreateIssueLink.Request.TargetKey != "ABC-2" {
		t.Fatalf("CreateIssueLink = %#v", result.CreateIssueLink)
	}
}

func TestPoolDeleteIssueLinkSuccess(t *testing.T) {
	searcher := &fakeIssueSearcher{}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   35,
		Kind: KindDeleteIssueLink,
		DeleteIssueLink: &DeleteIssueLinkRequest{
			IssueKey: "ABC-1",
			LinkID:   "20001",
			Target:   "ABC-2",
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 35 || result.Kind != KindDeleteIssueLink {
		t.Fatalf("result = %#v", result)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if searcher.deleteIssueLinkID != "20001" {
		t.Fatalf("deleteIssueLinkID = %q", searcher.deleteIssueLinkID)
	}
	if result.DeleteIssueLink.IssueKey != "ABC-1" || result.DeleteIssueLink.Target != "ABC-2" {
		t.Fatalf("DeleteIssueLink = %#v", result.DeleteIssueLink)
	}
}

func TestPoolGetWorklogsSuccess(t *testing.T) {
	searcher := &fakeIssueSearcher{
		worklogs: []jira.Worklog{{ID: "10001", Author: "Jane Doe", TimeSpent: "1h"}},
	}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   33,
		Kind: KindGetWorklogs,
		GetWorklogs: &GetWorklogsRequest{
			Key:        "ABC-1",
			MaxResults: 20,
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 33 || result.Kind != KindGetWorklogs {
		t.Fatalf("result = %#v", result)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if searcher.worklogKey != "ABC-1" || searcher.worklogMaxResults != 20 {
		t.Fatalf("worklog request = %s/%d", searcher.worklogKey, searcher.worklogMaxResults)
	}
	if len(result.GetWorklogs.Worklogs) != 1 || result.GetWorklogs.Worklogs[0].ID != "10001" {
		t.Fatalf("GetWorklogs = %#v", result.GetWorklogs)
	}
}

func TestPoolUpdateAndDeleteWorklogSuccess(t *testing.T) {
	searcher := &fakeIssueSearcher{}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(2))
	defer pool.Stop()

	started := time.Date(2026, 6, 19, 10, 0, 0, 0, time.UTC)
	updateRequest := jira.UpdateWorklogRequest{ID: "10001", TimeSpent: "2h", Started: started, Comment: "Reviewed implementation"}
	if err := pool.Submit(Request{
		ID:   36,
		Kind: KindUpdateWorklog,
		UpdateWorklog: &UpdateWorklogRequest{
			Key:     "ABC-1",
			Request: updateRequest,
		},
	}); err != nil {
		t.Fatalf("Submit update error = %v", err)
	}

	updateResult := readResult(t, pool)
	if updateResult.Err != nil {
		t.Fatalf("update Err = %v", updateResult.Err)
	}
	if searcher.updateWorklogKey != "ABC-1" || searcher.updateWorklogRequest.TimeSpent != "2h" {
		t.Fatalf("update worklog = %s/%#v", searcher.updateWorklogKey, searcher.updateWorklogRequest)
	}
	if updateResult.UpdateWorklog.Worklog.ID != "10001" || updateResult.UpdateWorklog.Worklog.TimeSpent != "2h" {
		t.Fatalf("UpdateWorklog = %#v", updateResult.UpdateWorklog)
	}

	if err := pool.Submit(Request{
		ID:   37,
		Kind: KindDeleteWorklog,
		DeleteWorklog: &DeleteWorklogRequest{
			Key:       "ABC-1",
			WorklogID: "10001",
		},
	}); err != nil {
		t.Fatalf("Submit delete error = %v", err)
	}

	deleteResult := readResult(t, pool)
	if deleteResult.Err != nil {
		t.Fatalf("delete Err = %v", deleteResult.Err)
	}
	if searcher.deleteWorklogKey != "ABC-1" || searcher.deleteWorklogID != "10001" {
		t.Fatalf("delete worklog = %s/%s", searcher.deleteWorklogKey, searcher.deleteWorklogID)
	}
	if deleteResult.DeleteWorklog.Key != "ABC-1" || deleteResult.DeleteWorklog.WorklogID != "10001" {
		t.Fatalf("DeleteWorklog = %#v", deleteResult.DeleteWorklog)
	}
}

func TestPoolAddWorklogSuccess(t *testing.T) {
	started := time.Date(2026, 6, 19, 9, 30, 0, 0, time.UTC)
	searcher := &fakeIssueSearcher{
		addedWorklog: jira.Worklog{ID: "10001", Author: "Jane Doe", TimeSpent: "45m", Started: started},
	}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	request := jira.AddWorklogRequest{TimeSpent: "45m", Started: started, Comment: "Reviewed ABC-2"}
	err := pool.Submit(Request{
		ID:   34,
		Kind: KindAddWorklog,
		AddWorklog: &AddWorklogRequest{
			Key:     "ABC-1",
			Request: request,
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 34 || result.Kind != KindAddWorklog {
		t.Fatalf("result = %#v", result)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if searcher.addWorklogKey != "ABC-1" || searcher.addWorklogRequest.TimeSpent != "45m" {
		t.Fatalf("add worklog request = %s/%#v", searcher.addWorklogKey, searcher.addWorklogRequest)
	}
	if result.AddWorklog.Worklog.ID != "10001" || result.AddWorklog.Request.Comment != "Reviewed ABC-2" {
		t.Fatalf("AddWorklog = %#v", result.AddWorklog)
	}
}

func TestPoolUpdateAssigneeSuccess(t *testing.T) {
	searcher := &fakeIssueSearcher{}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   24,
		Kind: KindUpdateAssignee,
		UpdateAssignee: &UpdateAssigneeRequest{
			Key:      "ABC-1",
			Assignee: jira.User{AccountID: "abc-123", DisplayName: "Jane Doe"},
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 24 {
		t.Fatalf("ID = %d", result.ID)
	}
	if result.Kind != KindUpdateAssignee {
		t.Fatalf("Kind = %s", result.Kind)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if searcher.updateAssigneeKey != "ABC-1" || searcher.updateAssigneeValue.AccountID != "abc-123" {
		t.Fatalf("update assignee = %s/%#v", searcher.updateAssigneeKey, searcher.updateAssigneeValue)
	}
	if result.UpdateAssignee.Key != "ABC-1" || result.UpdateAssignee.Assignee.DisplayName != "Jane Doe" {
		t.Fatalf("UpdateAssignee = %#v", result.UpdateAssignee)
	}
}

func TestPoolUpdateAssigneeError(t *testing.T) {
	updateErr := errors.New("jira rejected assignee")
	pool := NewPool(&fakeIssueSearcher{updateAssigneeErr: updateErr}, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   25,
		Kind: KindUpdateAssignee,
		UpdateAssignee: &UpdateAssigneeRequest{
			Key:      "ABC-1",
			Assignee: jira.User{AccountID: "abc-123", DisplayName: "Jane Doe"},
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if !errors.Is(result.Err, updateErr) {
		t.Fatalf("Err = %v", result.Err)
	}
}

func TestPoolSearchUsersSuccess(t *testing.T) {
	pool := NewPool(&fakeIssueSearcher{
		users: []jira.User{{AccountID: "abc-123", DisplayName: "Jane Doe"}},
	}, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:          11,
		Kind:        KindSearchUsers,
		SearchUsers: &SearchUsersRequest{Query: "Jane", MaxResults: 5},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 11 {
		t.Fatalf("ID = %d", result.ID)
	}
	if result.Kind != KindSearchUsers {
		t.Fatalf("Kind = %s", result.Kind)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if result.SearchUsers.Query != "Jane" {
		t.Fatalf("Query = %q", result.SearchUsers.Query)
	}
	if len(result.SearchUsers.Users) != 1 || result.SearchUsers.Users[0].AccountID != "abc-123" {
		t.Fatalf("Users = %#v", result.SearchUsers.Users)
	}
}

func TestPoolSearchUsersUsesAssignableSearchWhenIssueKeyProvided(t *testing.T) {
	searcher := &fakeIssueSearcher{
		assignableUsers: []jira.User{{AccountID: "def-456", DisplayName: "John Doe"}},
	}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   12,
		Kind: KindSearchUsers,
		SearchUsers: &SearchUsersRequest{
			Query:      "John",
			IssueKey:   "ABC-1",
			MaxResults: 5,
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if searcher.assignableIssueKey != "ABC-1" || searcher.assignableQuery != "John" || searcher.assignableMaxResults != 5 {
		t.Fatalf("assignable search = issue %q query %q max %d", searcher.assignableIssueKey, searcher.assignableQuery, searcher.assignableMaxResults)
	}
	if len(result.SearchUsers.Users) != 1 || result.SearchUsers.Users[0].AccountID != "def-456" {
		t.Fatalf("Users = %#v", result.SearchUsers.Users)
	}
}

func TestPoolRejectsFullQueue(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	pool := NewPool(
		&blockingIssueSearcher{release: release, started: started},
		WithWorkerCount(1),
		WithQueueSize(1),
	)
	t.Cleanup(func() {
		close(release)
		pool.Stop()
	})

	if err := pool.Submit(searchRequest(1)); err != nil {
		t.Fatalf("Submit first request error = %v", err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first request to start")
	}
	if err := pool.Submit(searchRequest(2)); err != nil {
		t.Fatalf("Submit second request error = %v", err)
	}
	if err := pool.Submit(searchRequest(3)); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("Submit third request error = %v", err)
	}
}

func TestPoolCoalescesDuplicateReadRequests(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	pool := NewPool(
		&blockingIssueSearcher{release: release, started: started},
		WithWorkerCount(1),
		WithQueueSize(2),
	)
	t.Cleanup(pool.Stop)

	first := searchRequest(1)
	first.CoalesceKey = "search:project=ABC"
	second := searchRequest(2)
	second.CoalesceKey = first.CoalesceKey

	if err := pool.Submit(first); err != nil {
		t.Fatalf("Submit first request error = %v", err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first request to start")
	}
	if err := pool.Submit(second); err != nil {
		t.Fatalf("Submit duplicate request error = %v", err)
	}

	close(release)
	resultA := readResult(t, pool)
	resultB := readResult(t, pool)
	ids := map[int]bool{resultA.ID: true, resultB.ID: true}
	if !ids[1] || !ids[2] {
		t.Fatalf("result IDs = %d, %d", resultA.ID, resultB.ID)
	}
}

func TestPoolDropsQueuedBackgroundForForegroundRequest(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	pool := NewPool(
		&blockingIssueSearcher{release: release, started: started},
		WithWorkerCount(1),
		WithQueueSize(1),
	)
	t.Cleanup(pool.Stop)

	if err := pool.Submit(searchRequest(1)); err != nil {
		t.Fatalf("Submit running request error = %v", err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first request to start")
	}
	background := searchRequest(2)
	background.Priority = PriorityBackground
	if err := pool.Submit(background); err != nil {
		t.Fatalf("Submit background request error = %v", err)
	}
	foreground := searchRequest(3)
	foreground.Priority = PriorityForeground
	if err := pool.Submit(foreground); err != nil {
		t.Fatalf("Submit foreground request error = %v", err)
	}

	dropped := readResult(t, pool)
	if dropped.ID != 2 || !errors.Is(dropped.Err, ErrQueueFull) {
		t.Fatalf("dropped result = %#v", dropped)
	}

	close(release)
	resultA := readResult(t, pool)
	resultB := readResult(t, pool)
	ids := map[int]bool{resultA.ID: true, resultB.ID: true}
	if !ids[1] || !ids[3] {
		t.Fatalf("result IDs = %d, %d", resultA.ID, resultB.ID)
	}
}

func TestPoolTreatsDefaultPriorityAsForeground(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	pool := NewPool(
		&blockingIssueSearcher{release: release, started: started},
		WithWorkerCount(1),
		WithQueueSize(1),
	)
	t.Cleanup(pool.Stop)

	if err := pool.Submit(searchRequest(1)); err != nil {
		t.Fatalf("Submit running request error = %v", err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first request to start")
	}
	background := searchRequest(2)
	background.Priority = PriorityBackground
	if err := pool.Submit(background); err != nil {
		t.Fatalf("Submit background request error = %v", err)
	}
	if err := pool.Submit(searchRequest(3)); err != nil {
		t.Fatalf("default-priority request should be admitted as foreground: %v", err)
	}

	dropped := readResult(t, pool)
	if dropped.ID != 2 || !errors.Is(dropped.Err, ErrQueueFull) {
		t.Fatalf("dropped result = %#v", dropped)
	}
	close(release)
}

func TestPoolStatsReportSchedulerState(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	pool := NewPool(
		&blockingIssueSearcher{release: release, started: started},
		WithWorkerCount(1),
		WithQueueSize(1),
	)
	t.Cleanup(func() {
		close(release)
		pool.Stop()
	})

	if err := pool.Submit(searchRequest(1)); err != nil {
		t.Fatalf("Submit running request error = %v", err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first request to start")
	}
	pending := searchRequest(2)
	pending.CoalesceKey = "search:project=ABC"
	if err := pool.Submit(pending); err != nil {
		t.Fatalf("Submit pending request error = %v", err)
	}
	coalesced := searchRequest(3)
	coalesced.CoalesceKey = pending.CoalesceKey
	if err := pool.Submit(coalesced); err != nil {
		t.Fatalf("Submit coalesced request error = %v", err)
	}

	stats := pool.Stats()
	if stats.Running != 1 || stats.Pending != 1 || stats.Coalesced != 1 || stats.Capacity != 2 {
		t.Fatalf("Stats() = %#v", stats)
	}
}

func TestPoolReturnsInvalidRequestResult(t *testing.T) {
	pool := NewPool(&fakeIssueSearcher{}, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	if err := pool.Submit(Request{ID: 7, Kind: KindSearchIssues}); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if !errors.Is(result.Err, ErrInvalidRequest) {
		t.Fatalf("Err = %v", result.Err)
	}
}

func TestPoolReturnsInvalidCurrentUserRequestResult(t *testing.T) {
	pool := NewPool(&fakeIssueSearcher{}, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	if err := pool.Submit(Request{ID: 8, Kind: KindGetCurrentUser}); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if !errors.Is(result.Err, ErrInvalidRequest) {
		t.Fatalf("Err = %v", result.Err)
	}
}

func TestPoolReturnsInvalidTransitionRequestResults(t *testing.T) {
	pool := NewPool(&fakeIssueSearcher{}, WithWorkerCount(1), WithQueueSize(2))
	defer pool.Stop()

	requests := []Request{
		{ID: 17, Kind: KindGetTransitions},
		{ID: 18, Kind: KindTransitionIssue, TransitionIssue: &TransitionIssueRequest{Key: "ABC-1"}},
	}
	for _, request := range requests {
		if err := pool.Submit(request); err != nil {
			t.Fatalf("Submit() error = %v", err)
		}
	}

	for range requests {
		result := readResult(t, pool)
		if !errors.Is(result.Err, ErrInvalidRequest) {
			t.Fatalf("Err = %v", result.Err)
		}
	}
}

func TestPoolReturnsInvalidSummaryEditRequestResults(t *testing.T) {
	pool := NewPool(&fakeIssueSearcher{}, WithWorkerCount(1), WithQueueSize(16))
	defer pool.Stop()

	requests := []Request{
		{ID: 24, Kind: KindGetEditMetadata},
		{ID: 25, Kind: KindUpdateSummary, UpdateSummary: &UpdateSummaryRequest{Key: "ABC-1"}},
		{ID: 26, Kind: KindUpdateSummary, UpdateSummary: &UpdateSummaryRequest{Summary: "Updated summary"}},
		{ID: 27, Kind: KindUpdateDescription, UpdateDescription: &UpdateDescriptionRequest{Key: "ABC-1"}},
		{ID: 28, Kind: KindUpdateDescription, UpdateDescription: &UpdateDescriptionRequest{Description: "Updated description"}},
		{ID: 29, Kind: KindUpdatePriority, UpdatePriority: &UpdatePriorityRequest{Key: "ABC-1"}},
		{ID: 30, Kind: KindUpdatePriority, UpdatePriority: &UpdatePriorityRequest{Priority: jira.FieldOption{ID: "3", Name: "Medium"}}},
		{ID: 31, Kind: KindUpdateAssignee, UpdateAssignee: &UpdateAssigneeRequest{Key: "ABC-1"}},
		{ID: 32, Kind: KindUpdateAssignee, UpdateAssignee: &UpdateAssigneeRequest{Assignee: jira.User{AccountID: "abc-123", DisplayName: "Jane Doe"}}},
		{ID: 33, Kind: KindGetCreateIssueTypes},
		{ID: 34, Kind: KindGetCreateFields, GetCreateFields: &GetCreateFieldsRequest{ProjectKey: "ABC"}},
		{ID: 35, Kind: KindGetCreateFields, GetCreateFields: &GetCreateFieldsRequest{IssueTypeID: "10001"}},
		{ID: 36, Kind: KindCreateIssue},
		{ID: 37, Kind: KindCreateIssue, CreateIssue: &CreateIssueRequest{ProjectKey: "ABC", IssueTypeID: "10001"}},
		{ID: 38, Kind: KindCreateIssue, CreateIssue: &CreateIssueRequest{ProjectKey: "ABC", Summary: "New issue"}},
		{ID: 39, Kind: KindCreateIssue, CreateIssue: &CreateIssueRequest{IssueTypeID: "10001", Summary: "New issue"}},
	}
	for _, request := range requests {
		if err := pool.Submit(request); err != nil {
			t.Fatalf("Submit() error = %v", err)
		}
	}

	for range requests {
		result := readResult(t, pool)
		if !errors.Is(result.Err, ErrInvalidRequest) {
			t.Fatalf("Err = %v", result.Err)
		}
	}
}

func TestPoolRejectsSubmitAfterStop(t *testing.T) {
	pool := NewPool(&fakeIssueSearcher{}, WithWorkerCount(1), WithQueueSize(1))
	pool.Stop()

	if err := pool.Submit(searchRequest(1)); !errors.Is(err, ErrPoolClosed) {
		t.Fatalf("Submit() error = %v", err)
	}
}

func readResult(t *testing.T, pool *Pool) Result {
	t.Helper()

	select {
	case result := <-pool.Results():
		return result
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for worker result")
		return Result{}
	}
}

func searchRequest(id int) Request {
	return Request{
		ID:           id,
		Kind:         KindSearchIssues,
		SearchIssues: &SearchIssuesRequest{JQL: "project = ABC"},
	}
}

type fakeIssueSearcher struct {
	issues                    []jira.Issue
	searchResults             map[string][]jira.Issue
	searches                  []string
	detail                    jira.IssueDetail
	details                   map[string]jira.IssueDetail
	comments                  []jira.Comment
	addedComment              jira.Comment
	addCommentBody            string
	addMentions               []jira.Mention
	updatedComment            jira.Comment
	updateCommentKey          string
	updateCommentID           string
	updateCommentBody         string
	updateMentions            []jira.Mention
	users                     []jira.User
	currentUser               jira.User
	assignableUsers           []jira.User
	assignableIssueKey        string
	assignableQuery           string
	assignableMaxResults      int
	transitions               []jira.Transition
	transitionKey             string
	transitionID              string
	transitionRequest         jira.TransitionIssueRequest
	transitionErr             error
	editMetadata              jira.EditMetadata
	createIssueTypes          []jira.CreateIssueType
	createFields              []jira.CreateField
	fieldOptions              []jira.FieldOption
	fieldOptionURL            string
	fieldOptionQuery          string
	fieldOptionMaxResults     int
	issueLinkTypes            []jira.IssueLinkType
	issueLinkRequest          jira.CreateIssueLinkRequest
	deleteIssueLinkID         string
	issueLinkErr              error
	worklogs                  []jira.Worklog
	worklogKey                string
	worklogMaxResults         int
	worklogErr                error
	addWorklogKey             string
	addWorklogRequest         jira.AddWorklogRequest
	addedWorklog              jira.Worklog
	addWorklogErr             error
	updateWorklogKey          string
	updateWorklogRequest      jira.UpdateWorklogRequest
	updatedWorklog            jira.Worklog
	updateWorklogErr          error
	deleteWorklogKey          string
	deleteWorklogID           string
	deleteWorklogErr          error
	createIssueRequest        jira.CreateIssueRequest
	createdIssue              jira.Issue
	boardIssuesByJQL          map[string][]jira.Issue
	boardPage                 jira.BoardPage
	sprintPage                jira.SprintPage
	boardProjectKey           string
	boardStartAt              int
	boardMaxResults           int
	sprintBoardID             int
	sprintStates              []string
	sprintStartAt             int
	sprintMaxResults          int
	moveSprintID              int
	moveIssueKeys             []string
	moveSprintErr             error
	updateSummaryKey          string
	updateSummaryValue        string
	updateSummaryErr          error
	updateDescriptionKey      string
	updateDescriptionValue    string
	updateDescriptionErr      error
	updatePriorityKey         string
	updatePriorityValue       jira.FieldOption
	updatePriorityErr         error
	updateLabelsKey           string
	updateLabelsValue         []string
	updateLabelsErr           error
	updateComponentsKey       string
	updateComponentsValue     []jira.FieldOption
	updateComponentsErr       error
	updateEditFieldKey        string
	updateEditFieldValue      jira.EditFieldValue
	updateEditFieldErr        error
	updateParentKey           string
	updateParentRequest       jira.UpdateParentRequest
	updateParentErr           error
	updateTimeTrackingKey     string
	updateTimeTrackingRequest jira.UpdateTimeTrackingRequest
	updateTimeTrackingErr     error
	updateAssigneeKey         string
	updateAssigneeValue       jira.User
	updateAssigneeErr         error
	err                       error
}

func (f *fakeIssueSearcher) SearchIssues(_ context.Context, jql string, _ int) ([]jira.Issue, error) {
	f.searches = append(f.searches, jql)
	if f.err != nil {
		return nil, f.err
	}
	if f.searchResults != nil {
		if issues, ok := f.searchResults[jql]; ok {
			return issues, nil
		}
	}
	return f.issues, nil
}

func (f *fakeIssueSearcher) CurrentUser(_ context.Context) (jira.User, error) {
	if f.err != nil {
		return jira.User{}, f.err
	}
	if f.currentUser.AccountID != "" {
		return f.currentUser, nil
	}
	return jira.User{AccountID: "account-123", DisplayName: "Person Example"}, nil
}

func (f *fakeIssueSearcher) GetIssue(_ context.Context, key string) (jira.IssueDetail, error) {
	if f.err != nil {
		return jira.IssueDetail{}, f.err
	}
	if f.detail.Key != "" {
		return f.detail, nil
	}
	if f.details != nil {
		if detail, ok := f.details[key]; ok {
			return detail, nil
		}
	}
	return jira.IssueDetail{Issue: jira.Issue{Key: key}}, nil
}

func (f *fakeIssueSearcher) GetComments(_ context.Context, key string, _ int) ([]jira.Comment, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.comments != nil {
		return f.comments, nil
	}
	return []jira.Comment{{ID: "10001", Body: key}}, nil
}

func (f *fakeIssueSearcher) AddComment(_ context.Context, key string, body string, mentions []jira.Mention) (jira.Comment, error) {
	if f.err != nil {
		return jira.Comment{}, f.err
	}
	f.addCommentBody = body
	f.addMentions = mentions
	if f.addedComment.ID != "" {
		return f.addedComment, nil
	}
	return jira.Comment{ID: "10002", Body: body, Author: key}, nil
}

func (f *fakeIssueSearcher) UpdateComment(_ context.Context, key string, commentID string, body string, mentions []jira.Mention) (jira.Comment, error) {
	if f.err != nil {
		return jira.Comment{}, f.err
	}
	f.updateCommentKey = key
	f.updateCommentID = commentID
	f.updateCommentBody = body
	f.updateMentions = mentions
	if f.updatedComment.ID != "" {
		return f.updatedComment, nil
	}
	return jira.Comment{ID: commentID, Body: body, Author: key}, nil
}

func (f *fakeIssueSearcher) SearchUsers(_ context.Context, query string, _ int) ([]jira.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.users != nil {
		return f.users, nil
	}
	return []jira.User{{AccountID: "abc-123", DisplayName: query}}, nil
}

func (f *fakeIssueSearcher) SearchAssignableUsers(_ context.Context, issueKey string, query string, maxResults int) ([]jira.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.assignableIssueKey = issueKey
	f.assignableQuery = query
	f.assignableMaxResults = maxResults
	if f.assignableUsers != nil {
		return f.assignableUsers, nil
	}
	return []jira.User{{AccountID: "assignable-123", DisplayName: query}}, nil
}

func (f *fakeIssueSearcher) GetTransitions(_ context.Context, key string) ([]jira.Transition, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.transitionKey = key
	return f.transitions, nil
}

func (f *fakeIssueSearcher) TransitionIssue(_ context.Context, key string, request jira.TransitionIssueRequest) error {
	if f.transitionErr != nil {
		return f.transitionErr
	}
	if f.err != nil {
		return f.err
	}
	f.transitionKey = key
	f.transitionID = request.TransitionID
	f.transitionRequest = request
	return nil
}

func (f *fakeIssueSearcher) GetEditMetadata(_ context.Context, key string) (jira.EditMetadata, error) {
	if f.err != nil {
		return jira.EditMetadata{}, f.err
	}
	f.transitionKey = key
	return f.editMetadata, nil
}

func (f *fakeIssueSearcher) GetCreateIssueTypes(_ context.Context, projectKey string) ([]jira.CreateIssueType, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.transitionKey = projectKey
	return f.createIssueTypes, nil
}

func (f *fakeIssueSearcher) GetCreateFields(_ context.Context, projectKey string, issueTypeID string) ([]jira.CreateField, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.transitionKey = projectKey
	f.transitionID = issueTypeID
	return f.createFields, nil
}

func (f *fakeIssueSearcher) SearchFieldOptions(_ context.Context, autocompleteURL string, query string, maxResults int) ([]jira.FieldOption, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.fieldOptionURL = autocompleteURL
	f.fieldOptionQuery = query
	f.fieldOptionMaxResults = maxResults
	return f.fieldOptions, nil
}

func (f *fakeIssueSearcher) GetIssueLinkTypes(_ context.Context) ([]jira.IssueLinkType, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.issueLinkTypes, nil
}

func (f *fakeIssueSearcher) CreateIssueLink(_ context.Context, request jira.CreateIssueLinkRequest) error {
	if f.issueLinkErr != nil {
		return f.issueLinkErr
	}
	if f.err != nil {
		return f.err
	}
	f.issueLinkRequest = request
	return nil
}

func (f *fakeIssueSearcher) DeleteIssueLink(_ context.Context, linkID string) error {
	if f.issueLinkErr != nil {
		return f.issueLinkErr
	}
	if f.err != nil {
		return f.err
	}
	f.deleteIssueLinkID = linkID
	return nil
}

func (f *fakeIssueSearcher) GetWorklogs(_ context.Context, key string, maxResults int) ([]jira.Worklog, error) {
	if f.worklogErr != nil {
		return nil, f.worklogErr
	}
	if f.err != nil {
		return nil, f.err
	}
	f.worklogKey = key
	f.worklogMaxResults = maxResults
	return f.worklogs, nil
}

func (f *fakeIssueSearcher) AddWorklog(_ context.Context, key string, request jira.AddWorklogRequest) (jira.Worklog, error) {
	if f.addWorklogErr != nil {
		return jira.Worklog{}, f.addWorklogErr
	}
	if f.err != nil {
		return jira.Worklog{}, f.err
	}
	f.addWorklogKey = key
	f.addWorklogRequest = request
	if f.addedWorklog.ID != "" {
		return f.addedWorklog, nil
	}
	return jira.Worklog{ID: "10001", Author: "Current User", TimeSpent: request.TimeSpent, Comment: request.Comment, Started: request.Started}, nil
}

func (f *fakeIssueSearcher) UpdateWorklog(_ context.Context, key string, request jira.UpdateWorklogRequest) (jira.Worklog, error) {
	if f.updateWorklogErr != nil {
		return jira.Worklog{}, f.updateWorklogErr
	}
	if f.err != nil {
		return jira.Worklog{}, f.err
	}
	f.updateWorklogKey = key
	f.updateWorklogRequest = request
	if f.updatedWorklog.ID != "" {
		return f.updatedWorklog, nil
	}
	return jira.Worklog{ID: request.ID, Author: "Current User", TimeSpent: request.TimeSpent, Comment: request.Comment, Started: request.Started}, nil
}

func (f *fakeIssueSearcher) DeleteWorklog(_ context.Context, key string, worklogID string) error {
	if f.deleteWorklogErr != nil {
		return f.deleteWorklogErr
	}
	if f.err != nil {
		return f.err
	}
	f.deleteWorklogKey = key
	f.deleteWorklogID = worklogID
	return nil
}

func (f *fakeIssueSearcher) CreateIssue(_ context.Context, request jira.CreateIssueRequest) (jira.Issue, error) {
	if f.err != nil {
		return jira.Issue{}, f.err
	}
	f.createIssueRequest = request
	if f.createdIssue.Key != "" {
		return f.createdIssue, nil
	}
	return jira.Issue{Key: "ABC-123", Summary: request.Summary}, nil
}

func (f *fakeIssueSearcher) GetBoards(_ context.Context, projectKey string, startAt, maxResults int) (jira.BoardPage, error) {
	if f.err != nil {
		return jira.BoardPage{}, f.err
	}
	f.boardProjectKey = projectKey
	f.boardStartAt = startAt
	f.boardMaxResults = maxResults
	return f.boardPage, nil
}

func (f *fakeIssueSearcher) GetBoardSprints(_ context.Context, boardID int, states []string, startAt, maxResults int) (jira.SprintPage, error) {
	if f.err != nil {
		return jira.SprintPage{}, f.err
	}
	f.sprintBoardID = boardID
	f.sprintStates = append([]string(nil), states...)
	f.sprintStartAt = startAt
	f.sprintMaxResults = maxResults
	return f.sprintPage, nil
}

func (f *fakeIssueSearcher) MoveIssuesToSprint(_ context.Context, sprintID int, issueKeys []string) error {
	if f.moveSprintErr != nil {
		return f.moveSprintErr
	}
	if f.err != nil {
		return f.err
	}
	f.moveSprintID = sprintID
	f.moveIssueKeys = append([]string{}, issueKeys...)
	return nil
}

func (f *fakeIssueSearcher) SearchBoardIssues(_ context.Context, boardID int, jql string, maxResults int) ([]jira.Issue, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.boardIssuesByJQL != nil {
		return append([]jira.Issue(nil), f.boardIssuesByJQL[jql]...), nil
	}
	key := strings.TrimSpace(strings.TrimPrefix(jql, "key = "))
	if key == "" {
		return nil, nil
	}
	return []jira.Issue{{Key: key}}, nil
}

func (f *fakeIssueSearcher) UpdateSummary(_ context.Context, key string, summary string) error {
	if f.updateSummaryErr != nil {
		return f.updateSummaryErr
	}
	if f.err != nil {
		return f.err
	}
	f.updateSummaryKey = key
	f.updateSummaryValue = summary
	return nil
}

func (f *fakeIssueSearcher) UpdateDescription(_ context.Context, key string, description string) error {
	if f.updateDescriptionErr != nil {
		return f.updateDescriptionErr
	}
	if f.err != nil {
		return f.err
	}
	f.updateDescriptionKey = key
	f.updateDescriptionValue = description
	return nil
}

func (f *fakeIssueSearcher) UpdatePriority(_ context.Context, key string, priority jira.FieldOption) error {
	if f.updatePriorityErr != nil {
		return f.updatePriorityErr
	}
	if f.err != nil {
		return f.err
	}
	f.updatePriorityKey = key
	f.updatePriorityValue = priority
	return nil
}

func (f *fakeIssueSearcher) UpdateLabels(_ context.Context, key string, labels []string) error {
	if f.updateLabelsErr != nil {
		return f.updateLabelsErr
	}
	if f.err != nil {
		return f.err
	}
	f.updateLabelsKey = key
	f.updateLabelsValue = append([]string{}, labels...)
	return nil
}

func (f *fakeIssueSearcher) UpdateComponents(_ context.Context, key string, components []jira.FieldOption) error {
	if f.updateComponentsErr != nil {
		return f.updateComponentsErr
	}
	if f.err != nil {
		return f.err
	}
	f.updateComponentsKey = key
	f.updateComponentsValue = append([]jira.FieldOption{}, components...)
	return nil
}

func (f *fakeIssueSearcher) UpdateEditField(_ context.Context, key string, value jira.EditFieldValue) error {
	if f.updateEditFieldErr != nil {
		return f.updateEditFieldErr
	}
	if f.err != nil {
		return f.err
	}
	f.updateEditFieldKey = key
	f.updateEditFieldValue = value
	return nil
}

func (f *fakeIssueSearcher) UpdateParent(_ context.Context, key string, request jira.UpdateParentRequest) error {
	if f.updateParentErr != nil {
		return f.updateParentErr
	}
	if f.err != nil {
		return f.err
	}
	f.updateParentKey = key
	f.updateParentRequest = request
	return nil
}

func (f *fakeIssueSearcher) UpdateTimeTracking(_ context.Context, key string, request jira.UpdateTimeTrackingRequest) error {
	if f.updateTimeTrackingErr != nil {
		return f.updateTimeTrackingErr
	}
	if f.err != nil {
		return f.err
	}
	f.updateTimeTrackingKey = key
	f.updateTimeTrackingRequest = request
	return nil
}

func (f *fakeIssueSearcher) UpdateAssignee(_ context.Context, key string, assignee jira.User) error {
	if f.updateAssigneeErr != nil {
		return f.updateAssigneeErr
	}
	if f.err != nil {
		return f.err
	}
	f.updateAssigneeKey = key
	f.updateAssigneeValue = assignee
	return nil
}

type blockingIssueSearcher struct {
	release <-chan struct{}
	started chan<- struct{}
}

func (b *blockingIssueSearcher) SearchIssues(ctx context.Context, _ string, _ int) ([]jira.Issue, error) {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (b *blockingIssueSearcher) CurrentUser(ctx context.Context) (jira.User, error) {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return jira.User{AccountID: "account-123", DisplayName: "Person Example"}, nil
	case <-ctx.Done():
		return jira.User{}, ctx.Err()
	}
}

func (b *blockingIssueSearcher) GetIssue(ctx context.Context, _ string) (jira.IssueDetail, error) {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return jira.IssueDetail{}, nil
	case <-ctx.Done():
		return jira.IssueDetail{}, ctx.Err()
	}
}

func (b *blockingIssueSearcher) GetComments(ctx context.Context, _ string, _ int) ([]jira.Comment, error) {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (b *blockingIssueSearcher) AddComment(ctx context.Context, _ string, _ string, _ []jira.Mention) (jira.Comment, error) {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return jira.Comment{}, nil
	case <-ctx.Done():
		return jira.Comment{}, ctx.Err()
	}
}

func (b *blockingIssueSearcher) UpdateComment(ctx context.Context, _ string, _ string, _ string, _ []jira.Mention) (jira.Comment, error) {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return jira.Comment{}, nil
	case <-ctx.Done():
		return jira.Comment{}, ctx.Err()
	}
}

func (b *blockingIssueSearcher) SearchUsers(ctx context.Context, _ string, _ int) ([]jira.User, error) {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (b *blockingIssueSearcher) SearchAssignableUsers(ctx context.Context, _ string, _ string, _ int) ([]jira.User, error) {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (b *blockingIssueSearcher) GetTransitions(ctx context.Context, _ string) ([]jira.Transition, error) {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (b *blockingIssueSearcher) TransitionIssue(ctx context.Context, _ string, _ jira.TransitionIssueRequest) error {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *blockingIssueSearcher) GetEditMetadata(ctx context.Context, _ string) (jira.EditMetadata, error) {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return jira.EditMetadata{}, nil
	case <-ctx.Done():
		return jira.EditMetadata{}, ctx.Err()
	}
}

func (b *blockingIssueSearcher) GetCreateIssueTypes(ctx context.Context, _ string) ([]jira.CreateIssueType, error) {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (b *blockingIssueSearcher) GetCreateFields(ctx context.Context, _ string, _ string) ([]jira.CreateField, error) {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (b *blockingIssueSearcher) SearchFieldOptions(ctx context.Context, _ string, _ string, _ int) ([]jira.FieldOption, error) {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (b *blockingIssueSearcher) GetIssueLinkTypes(ctx context.Context) ([]jira.IssueLinkType, error) {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (b *blockingIssueSearcher) CreateIssueLink(ctx context.Context, _ jira.CreateIssueLinkRequest) error {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *blockingIssueSearcher) DeleteIssueLink(ctx context.Context, _ string) error {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *blockingIssueSearcher) GetWorklogs(ctx context.Context, _ string, _ int) ([]jira.Worklog, error) {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (b *blockingIssueSearcher) AddWorklog(ctx context.Context, _ string, _ jira.AddWorklogRequest) (jira.Worklog, error) {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return jira.Worklog{}, nil
	case <-ctx.Done():
		return jira.Worklog{}, ctx.Err()
	}
}

func (b *blockingIssueSearcher) UpdateWorklog(ctx context.Context, _ string, _ jira.UpdateWorklogRequest) (jira.Worklog, error) {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return jira.Worklog{}, nil
	case <-ctx.Done():
		return jira.Worklog{}, ctx.Err()
	}
}

func (b *blockingIssueSearcher) DeleteWorklog(ctx context.Context, _ string, _ string) error {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *blockingIssueSearcher) CreateIssue(ctx context.Context, _ jira.CreateIssueRequest) (jira.Issue, error) {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return jira.Issue{}, nil
	case <-ctx.Done():
		return jira.Issue{}, ctx.Err()
	}
}

func (b *blockingIssueSearcher) GetBoards(ctx context.Context, _ string, _, _ int) (jira.BoardPage, error) {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return jira.BoardPage{}, nil
	case <-ctx.Done():
		return jira.BoardPage{}, ctx.Err()
	}
}

func (b *blockingIssueSearcher) GetBoardSprints(ctx context.Context, _ int, _ []string, _, _ int) (jira.SprintPage, error) {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return jira.SprintPage{}, nil
	case <-ctx.Done():
		return jira.SprintPage{}, ctx.Err()
	}
}

func (b *blockingIssueSearcher) MoveIssuesToSprint(ctx context.Context, _ int, _ []string) error {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *blockingIssueSearcher) SearchBoardIssues(ctx context.Context, _ int, _ string, _ int) ([]jira.Issue, error) {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (b *blockingIssueSearcher) UpdateSummary(ctx context.Context, _ string, _ string) error {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *blockingIssueSearcher) UpdateDescription(ctx context.Context, _ string, _ string) error {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *blockingIssueSearcher) UpdatePriority(ctx context.Context, _ string, _ jira.FieldOption) error {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *blockingIssueSearcher) UpdateLabels(ctx context.Context, _ string, _ []string) error {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *blockingIssueSearcher) UpdateComponents(ctx context.Context, _ string, _ []jira.FieldOption) error {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *blockingIssueSearcher) UpdateEditField(ctx context.Context, _ string, _ jira.EditFieldValue) error {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *blockingIssueSearcher) UpdateParent(ctx context.Context, _ string, _ jira.UpdateParentRequest) error {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *blockingIssueSearcher) UpdateTimeTracking(ctx context.Context, _ string, _ jira.UpdateTimeTrackingRequest) error {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *blockingIssueSearcher) UpdateAssignee(ctx context.Context, _ string, _ jira.User) error {
	if b.started != nil {
		close(b.started)
		b.started = nil
	}
	select {
	case <-b.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestPoolMoveIssuesToSprintSuccess(t *testing.T) {
	searcher := &fakeIssueSearcher{}
	pool := NewPool(searcher, WithWorkerCount(1), WithQueueSize(1))
	defer pool.Stop()

	err := pool.Submit(Request{
		ID:   41,
		Kind: KindMoveIssuesToSprint,
		MoveIssuesToSprint: &MoveIssuesToSprintRequest{
			Sprint:    jira.Sprint{ID: 300, Name: "Platform Sprint 24", State: "active"},
			IssueKeys: []string{"ABC-1"},
		},
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	result := readResult(t, pool)
	if result.ID != 41 || result.Kind != KindMoveIssuesToSprint {
		t.Fatalf("result = %#v", result)
	}
	if result.Err != nil {
		t.Fatalf("Err = %v", result.Err)
	}
	if searcher.moveSprintID != 300 || !reflect.DeepEqual(searcher.moveIssueKeys, []string{"ABC-1"}) {
		t.Fatalf("move request = %d/%#v", searcher.moveSprintID, searcher.moveIssueKeys)
	}
	if result.MoveIssuesToSprint == nil || result.MoveIssuesToSprint.Sprint.Name != "Platform Sprint 24" || !reflect.DeepEqual(result.MoveIssuesToSprint.IssueKeys, []string{"ABC-1"}) {
		t.Fatalf("MoveIssuesToSprint = %#v", result.MoveIssuesToSprint)
	}
}
