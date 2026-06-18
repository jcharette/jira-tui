package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jcharette/jira-tui/internal/jira"
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
	if len(searcher.createIssueRequest.Fields) != 1 || searcher.createIssueRequest.Fields[0].FieldID != "priority" {
		t.Fatalf("create fields = %#v", searcher.createIssueRequest.Fields)
	}
	if result.CreateIssue.Issue.Key != "ABC-123" {
		t.Fatalf("CreateIssue = %#v", result.CreateIssue)
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
	issues                 []jira.Issue
	searchResults          map[string][]jira.Issue
	detail                 jira.IssueDetail
	details                map[string]jira.IssueDetail
	comments               []jira.Comment
	addedComment           jira.Comment
	addMentions            []jira.Mention
	users                  []jira.User
	transitions            []jira.Transition
	transitionKey          string
	transitionID           string
	transitionRequest      jira.TransitionIssueRequest
	transitionErr          error
	editMetadata           jira.EditMetadata
	createIssueTypes       []jira.CreateIssueType
	createFields           []jira.CreateField
	createIssueRequest     jira.CreateIssueRequest
	createdIssue           jira.Issue
	updateSummaryKey       string
	updateSummaryValue     string
	updateSummaryErr       error
	updateDescriptionKey   string
	updateDescriptionValue string
	updateDescriptionErr   error
	updatePriorityKey      string
	updatePriorityValue    jira.FieldOption
	updatePriorityErr      error
	updateAssigneeKey      string
	updateAssigneeValue    jira.User
	updateAssigneeErr      error
	err                    error
}

func (f *fakeIssueSearcher) SearchIssues(_ context.Context, jql string, _ int) ([]jira.Issue, error) {
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
	f.addMentions = mentions
	if f.addedComment.ID != "" {
		return f.addedComment, nil
	}
	return jira.Comment{ID: "10002", Body: body, Author: key}, nil
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
