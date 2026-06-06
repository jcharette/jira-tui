package jira

import (
	"context"
	"errors"
	"testing"

	model "github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"
)

func TestSearchIssues(t *testing.T) {
	search := &fakeSearchService{
		response: &model.IssueSearchJQLScheme{
			Issues: []*model.IssueScheme{
				{
					Key: "ABC-123",
					Fields: &model.IssueFieldsScheme{
						Summary:  "Fix the thing",
						Status:   &model.StatusScheme{Name: "In Progress"},
						Assignee: nil,
					},
				},
			},
		},
	}
	client := &Client{
		baseURL: "https://example.atlassian.net",
		search:  search,
	}

	issues, err := client.SearchIssues(context.Background(), "project = ABC", 10)
	if err != nil {
		t.Fatalf("SearchIssues() error = %v", err)
	}

	if search.jql != "project = ABC" {
		t.Fatalf("jql = %q", search.jql)
	}
	if search.maxResults != 10 {
		t.Fatalf("maxResults = %d", search.maxResults)
	}
	wantFields := []string{"summary", "status", "assignee"}
	if !equalStrings(search.fields, wantFields) {
		t.Fatalf("fields = %#v", search.fields)
	}
	if len(issues) != 1 {
		t.Fatalf("len(issues) = %d", len(issues))
	}
	if issues[0].Key != "ABC-123" {
		t.Fatalf("Key = %q", issues[0].Key)
	}
	if issues[0].Assignee != "Unassigned" {
		t.Fatalf("Assignee = %q", issues[0].Assignee)
	}
	if issues[0].URL != "https://example.atlassian.net/browse/ABC-123" {
		t.Fatalf("URL = %q", issues[0].URL)
	}
}

func TestSearchIssuesUsesDefaultMaxResults(t *testing.T) {
	search := &fakeSearchService{
		response: &model.IssueSearchJQLScheme{},
	}
	client := &Client{
		baseURL: "https://example.atlassian.net",
		search:  search,
	}

	_, err := client.SearchIssues(context.Background(), "project = ABC", 0)
	if err != nil {
		t.Fatalf("SearchIssues() error = %v", err)
	}

	if search.maxResults != defaultMaxResults {
		t.Fatalf("maxResults = %d", search.maxResults)
	}
}

func TestSearchIssuesWrapsSearchError(t *testing.T) {
	search := &fakeSearchService{
		err: errors.New("boom"),
	}
	client := &Client{
		baseURL: "https://example.atlassian.net",
		search:  search,
	}

	_, err := client.SearchIssues(context.Background(), "project = ABC", 10)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, search.err) {
		t.Fatalf("error = %v", err)
	}
}

type fakeSearchService struct {
	jql        string
	fields     []string
	expands    []string
	maxResults int
	token      string
	response   *model.IssueSearchJQLScheme
	err        error
}

func (f *fakeSearchService) SearchJQL(_ context.Context, jql string, fields, expands []string, maxResults int, nextPageToken string) (*model.IssueSearchJQLScheme, *model.ResponseScheme, error) {
	f.jql = jql
	f.fields = fields
	f.expands = expands
	f.maxResults = maxResults
	f.token = nextPageToken
	return f.response, nil, f.err
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
