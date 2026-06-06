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
	issues []jira.Issue
	err    error
}

func (f *fakeIssueSearcher) SearchIssues(context.Context, string, int) ([]jira.Issue, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.issues, nil
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
