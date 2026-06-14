package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jon/jira-tui/internal/jira"
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
	issues        []jira.Issue
	searchResults map[string][]jira.Issue
	detail        jira.IssueDetail
	details       map[string]jira.IssueDetail
	comments      []jira.Comment
	addedComment  jira.Comment
	addMentions   []jira.Mention
	users         []jira.User
	err           error
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
