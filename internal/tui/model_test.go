package tui

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jon/jira-tui/internal/jira"
	"github.com/jon/jira-tui/internal/worker"
)

func TestLoadedIssuesIgnoreStaleRequest(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.activeRequestID = 2
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Keep me"}}

	updated, _ := model.Update(workerResultMsg{
		result: worker.Result{
			ID:   1,
			Kind: worker.KindSearchIssues,
			SearchIssues: &worker.SearchIssuesResult{
				Issues:   []jira.Issue{{Key: "ABC-2", Summary: "Stale"}},
				SyncedAt: time.Now(),
			},
		},
	})
	next := updated.(Model)

	if len(next.issues) != 1 || next.issues[0].Key != "ABC-1" {
		t.Fatalf("stale response replaced issues: %#v", next.issues)
	}
}

func TestLoadedIssuesPreserveSelectedIssue(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.refreshing = true
	model.activeRequestID = 2
	model.selected = 1
	model.issues = []jira.Issue{
		{Key: "ABC-1"},
		{Key: "ABC-2"},
	}

	updated, _ := model.Update(workerResultMsg{
		result: worker.Result{
			ID:   2,
			Kind: worker.KindSearchIssues,
			SearchIssues: &worker.SearchIssuesResult{
				Issues: []jira.Issue{
					{Key: "ABC-2"},
					{Key: "ABC-3"},
				},
				SyncedAt: time.Now(),
			},
		},
	})
	next := updated.(Model)

	if next.loading {
		t.Fatal("loading should be false")
	}
	if next.refreshing {
		t.Fatal("refreshing should be false")
	}
	if next.selected != 0 {
		t.Fatalf("selected = %d", next.selected)
	}
}

func TestRefreshTickStartsBackgroundRefresh(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithRefreshInterval(time.Minute),
	)
	defer model.workers.Stop()
	model.loading = false
	model.issues = []jira.Issue{{Key: "ABC-1"}}

	updated, cmd := model.Update(refreshTickMsg{})
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("expected refresh command")
	}
	if next.loading {
		t.Fatal("background refresh should not use initial loading state")
	}
	if !next.refreshing {
		t.Fatal("refreshing should be true")
	}
	if next.activeRequestID != initialRequestID+1 {
		t.Fatalf("activeRequestID = %d", next.activeRequestID)
	}
}

func TestSubmitIssueSearchReturnsLoadedResultFromPool(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{
		issues: []jira.Issue{{Key: "ABC-9"}},
	}, "project = ABC")
	defer model.workers.Stop()

	msg := model.submitIssueSearch(9)()
	if _, ok := msg.(workSubmittedMsg); !ok {
		t.Fatalf("msg = %T", msg)
	}

	resultMsg := model.waitForWorkerResult()()
	loaded, ok := resultMsg.(workerResultMsg)
	if !ok {
		t.Fatalf("resultMsg = %T", resultMsg)
	}
	if loaded.result.ID != 9 {
		t.Fatalf("requestID = %d", loaded.result.ID)
	}
	if len(loaded.result.SearchIssues.Issues) != 1 || loaded.result.SearchIssues.Issues[0].Key != "ABC-9" {
		t.Fatalf("issues = %#v", loaded.result.SearchIssues.Issues)
	}
}

func TestSubmitIssueSearchReturnsFailedResultFromPool(t *testing.T) {
	searchErr := errors.New("jira is unavailable")
	model := NewModel(&fakeIssueSearcher{
		err: searchErr,
	}, "project = ABC")
	defer model.workers.Stop()

	msg := model.submitIssueSearch(9)()
	if _, ok := msg.(workSubmittedMsg); !ok {
		t.Fatalf("msg = %T", msg)
	}

	resultMsg := model.waitForWorkerResult()()
	failed, ok := resultMsg.(workerResultMsg)
	if !ok {
		t.Fatalf("resultMsg = %T", resultMsg)
	}
	if failed.result.ID != 9 {
		t.Fatalf("requestID = %d", failed.result.ID)
	}
	if !errors.Is(failed.result.Err, searchErr) {
		t.Fatalf("err = %v", failed.result.Err)
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
	if f.issues != nil {
		return f.issues, nil
	}
	return []jira.Issue{{Key: "ABC-1"}}, nil
}
