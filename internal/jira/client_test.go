package jira

import (
	"context"
	"errors"
	"testing"
	"time"

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
						Priority: &model.PriorityScheme{Name: "High"},
						IssueType: &model.IssueTypeScheme{
							Name:           "Sub-task",
							Subtask:        true,
							HierarchyLevel: -1,
						},
						Parent: &model.ParentScheme{
							Key: "ABC-100",
							Fields: &model.ParentFieldsScheme{
								Summary: "Parent task",
							},
						},
						Subtasks: []*model.IssueScheme{
							{
								Key: "ABC-124",
								Fields: &model.IssueFieldsScheme{
									Summary:  "Nested subtask",
									Status:   &model.StatusScheme{Name: "To Do"},
									Priority: &model.PriorityScheme{Name: "Low"},
									IssueType: &model.IssueTypeScheme{
										Name:    "Sub-task",
										Subtask: true,
									},
								},
							},
						},
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
	wantFields := []string{"summary", "status", "assignee", "priority", "issuetype", "parent", "subtasks"}
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
	if issues[0].IssueType != "Sub-task" {
		t.Fatalf("IssueType = %q", issues[0].IssueType)
	}
	if issues[0].Priority != "High" {
		t.Fatalf("Priority = %q", issues[0].Priority)
	}
	if !issues[0].IsSubtask {
		t.Fatal("expected subtask")
	}
	if issues[0].ParentKey != "ABC-100" {
		t.Fatalf("ParentKey = %q", issues[0].ParentKey)
	}
	if issues[0].ParentSummary != "Parent task" {
		t.Fatalf("ParentSummary = %q", issues[0].ParentSummary)
	}
	if len(issues[0].Subtasks) != 1 {
		t.Fatalf("Subtasks = %#v", issues[0].Subtasks)
	}
	if issues[0].Subtasks[0].Key != "ABC-124" || issues[0].Subtasks[0].ParentKey != "ABC-123" {
		t.Fatalf("Subtasks[0] = %#v", issues[0].Subtasks[0])
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

func TestGetIssueFetchesAndParsesDetail(t *testing.T) {
	created := model.DateTimeScheme(time.Date(2026, 6, 1, 10, 30, 0, 0, time.UTC))
	updated := model.DateTimeScheme(time.Date(2026, 6, 2, 11, 45, 0, 0, time.UTC))
	issue := &fakeIssueService{
		response: &model.IssueScheme{
			Key: "ABC-123",
			Fields: &model.IssueFieldsScheme{
				Summary: "Fix the thing",
				Status:  &model.StatusScheme{Name: "In Progress"},
				Assignee: &model.UserScheme{
					DisplayName: "A Developer",
				},
				Reporter: &model.UserScheme{
					DisplayName: "Reporter Person",
				},
				Creator: &model.UserScheme{
					DisplayName: "Creator Person",
				},
				Priority: &model.PriorityScheme{Name: "High"},
				IssueType: &model.IssueTypeScheme{
					Name: "Task",
				},
				Description: &model.CommentNodeScheme{
					Type: "doc",
					Content: []*model.CommentNodeScheme{
						{
							Type: "paragraph",
							Content: []*model.CommentNodeScheme{
								{Type: "text", Text: "First line."},
							},
						},
						{
							Type: "paragraph",
							Content: []*model.CommentNodeScheme{
								{Type: "text", Text: "Second line."},
							},
						},
					},
				},
				Labels: []string{"backend", "urgent"},
				Components: []*model.ComponentScheme{
					{Name: "API"},
				},
				FixVersions: []*model.VersionScheme{
					{Name: "2026.06"},
				},
				Created: &created,
				Updated: &updated,
			},
		},
	}
	client := &Client{
		baseURL: "https://example.atlassian.net",
		issue:   issue,
	}

	detail, err := client.GetIssue(context.Background(), "ABC-123")
	if err != nil {
		t.Fatalf("GetIssue() error = %v", err)
	}

	if issue.key != "ABC-123" {
		t.Fatalf("key = %q", issue.key)
	}
	wantFields := []string{
		"summary",
		"status",
		"assignee",
		"priority",
		"issuetype",
		"parent",
		"subtasks",
		"description",
		"labels",
		"components",
		"fixVersions",
		"created",
		"updated",
		"reporter",
		"creator",
	}
	if !equalStrings(issue.fields, wantFields) {
		t.Fatalf("fields = %#v", issue.fields)
	}
	if detail.Key != "ABC-123" {
		t.Fatalf("Key = %q", detail.Key)
	}
	if detail.Description != "First line.\nSecond line." {
		t.Fatalf("Description = %q", detail.Description)
	}
	if detail.Reporter != "Reporter Person" {
		t.Fatalf("Reporter = %q", detail.Reporter)
	}
	if detail.Creator != "Creator Person" {
		t.Fatalf("Creator = %q", detail.Creator)
	}
	if !equalStrings(detail.Labels, []string{"backend", "urgent"}) {
		t.Fatalf("Labels = %#v", detail.Labels)
	}
	if !equalStrings(detail.Components, []string{"API"}) {
		t.Fatalf("Components = %#v", detail.Components)
	}
	if !equalStrings(detail.FixVersions, []string{"2026.06"}) {
		t.Fatalf("FixVersions = %#v", detail.FixVersions)
	}
	if !detail.Created.Equal(time.Time(created)) {
		t.Fatalf("Created = %s", detail.Created)
	}
	if !detail.Updated.Equal(time.Time(updated)) {
		t.Fatalf("Updated = %s", detail.Updated)
	}
}

func TestGetIssuePrefersRealUserFieldsOverPrivacyAlias(t *testing.T) {
	issue := &fakeIssueService{
		response: &model.IssueScheme{
			Key: "ABC-123",
			Fields: &model.IssueFieldsScheme{
				Summary: "Fix the thing",
				Assignee: &model.UserScheme{
					DisplayName:  "User e31ec",
					EmailAddress: "jon@example.test",
				},
				Reporter: &model.UserScheme{
					DisplayName: "User abc123",
					Name:        "reporter.name",
				},
				Creator: &model.UserScheme{
					DisplayName: "User def456",
					Key:         "creator-key",
				},
			},
		},
	}
	client := &Client{
		baseURL: "https://example.atlassian.net",
		issue:   issue,
	}

	detail, err := client.GetIssue(context.Background(), "ABC-123")
	if err != nil {
		t.Fatalf("GetIssue() error = %v", err)
	}

	if detail.Assignee != "jon@example.test" {
		t.Fatalf("Assignee = %q", detail.Assignee)
	}
	if detail.Reporter != "reporter.name" {
		t.Fatalf("Reporter = %q", detail.Reporter)
	}
	if detail.Creator != "creator-key" {
		t.Fatalf("Creator = %q", detail.Creator)
	}
}

func TestGetCommentsFetchesAndParsesADFComments(t *testing.T) {
	comments := &fakeCommentService{
		response: &model.IssueCommentPageScheme{
			Comments: []*model.IssueCommentScheme{
				{
					ID: "10001",
					Author: &model.UserScheme{
						DisplayName: "Comment Person",
					},
					Body: &model.CommentNodeScheme{
						Type: "doc",
						Content: []*model.CommentNodeScheme{
							{
								Type: "paragraph",
								Content: []*model.CommentNodeScheme{
									{Type: "text", Text: "Looks good."},
								},
							},
						},
					},
					Created: "2026-06-13T10:15:30.000-0400",
					Updated: "2026-06-13T10:16:30.000-0400",
				},
			},
		},
	}
	client := &Client{
		comment: comments,
	}

	got, err := client.GetComments(context.Background(), "ABC-123", 7)
	if err != nil {
		t.Fatalf("GetComments() error = %v", err)
	}

	if comments.key != "ABC-123" {
		t.Fatalf("key = %q", comments.key)
	}
	if comments.orderBy != "-created" {
		t.Fatalf("orderBy = %q", comments.orderBy)
	}
	if comments.maxResults != 7 {
		t.Fatalf("maxResults = %d", comments.maxResults)
	}
	if len(got) != 1 {
		t.Fatalf("comments = %#v", got)
	}
	if got[0].ID != "10001" {
		t.Fatalf("ID = %q", got[0].ID)
	}
	if got[0].Author != "Comment Person" {
		t.Fatalf("Author = %q", got[0].Author)
	}
	if got[0].Body != "Looks good." {
		t.Fatalf("Body = %q", got[0].Body)
	}
	if got[0].Created.IsZero() {
		t.Fatal("expected Created to parse")
	}
}

func TestAddCommentBuildsADFPayload(t *testing.T) {
	comments := &fakeCommentService{
		addResponse: &model.IssueCommentScheme{
			ID: "10002",
			Author: &model.UserScheme{
				DisplayName: "Comment Person",
			},
			Body: &model.CommentNodeScheme{
				Type: "doc",
				Content: []*model.CommentNodeScheme{
					{
						Type: "paragraph",
						Content: []*model.CommentNodeScheme{
							{Type: "text", Text: "Posted"},
						},
					},
				},
			},
		},
	}
	client := &Client{
		comment: comments,
	}

	comment, err := client.AddComment(context.Background(), "ABC-123", "First line\nSecond line\n\nSecond paragraph", nil)
	if err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}

	if comments.addKey != "ABC-123" {
		t.Fatalf("addKey = %q", comments.addKey)
	}
	if comments.payload == nil || comments.payload.Body == nil {
		t.Fatal("expected payload body")
	}
	body := comments.payload.Body
	if body.Type != "doc" {
		t.Fatalf("body type = %q", body.Type)
	}
	if body.Version != 1 {
		t.Fatalf("body version = %d", body.Version)
	}
	if len(body.Content) != 2 {
		t.Fatalf("paragraphs = %#v", body.Content)
	}
	if body.Content[0].Content[0].Text != "First line" {
		t.Fatalf("first text = %q", body.Content[0].Content[0].Text)
	}
	if body.Content[0].Content[1].Type != "hardBreak" {
		t.Fatalf("second node type = %q", body.Content[0].Content[1].Type)
	}
	if body.Content[0].Content[2].Text != "Second line" {
		t.Fatalf("third text = %q", body.Content[0].Content[2].Text)
	}
	if body.Content[1].Content[0].Text != "Second paragraph" {
		t.Fatalf("second paragraph = %q", body.Content[1].Content[0].Text)
	}
	if comment.ID != "10002" {
		t.Fatalf("ID = %q", comment.ID)
	}
	if comment.Body != "Posted" {
		t.Fatalf("Body = %q", comment.Body)
	}
}

func TestAddCommentBuildsADFLinkMarks(t *testing.T) {
	comments := &fakeCommentService{
		addResponse: &model.IssueCommentScheme{ID: "10002"},
	}
	client := &Client{
		comment: comments,
	}

	_, err := client.AddComment(context.Background(), "ABC-123", "See example.test/path and email ops@example.test", nil)
	if err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}

	nodes := comments.payload.Body.Content[0].Content
	if len(nodes) != 4 {
		t.Fatalf("nodes = %#v", nodes)
	}
	if nodes[0].Text != "See " {
		t.Fatalf("nodes[0].Text = %q", nodes[0].Text)
	}
	if nodes[1].Text != "example.test/path" {
		t.Fatalf("nodes[1].Text = %q", nodes[1].Text)
	}
	if href := nodes[1].Marks[0].Attrs["href"]; href != "https://example.test/path" {
		t.Fatalf("url href = %#v", href)
	}
	if nodes[2].Text != " and email " {
		t.Fatalf("nodes[2].Text = %q", nodes[2].Text)
	}
	if nodes[3].Text != "ops@example.test" {
		t.Fatalf("nodes[3].Text = %q", nodes[3].Text)
	}
	if href := nodes[3].Marks[0].Attrs["href"]; href != "mailto:ops@example.test" {
		t.Fatalf("email href = %#v", href)
	}
}

func TestAddCommentBuildsADFMentionNodes(t *testing.T) {
	comments := &fakeCommentService{
		addResponse: &model.IssueCommentScheme{ID: "10002"},
	}
	client := &Client{
		comment: comments,
	}

	_, err := client.AddComment(context.Background(), "ABC-123", "Please check @Jane Doe and https://example.test", []Mention{
		{AccountID: "abc-123", Text: "@Jane Doe"},
	})
	if err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}

	nodes := comments.payload.Body.Content[0].Content
	if len(nodes) != 4 {
		t.Fatalf("nodes = %#v", nodes)
	}
	if nodes[0].Text != "Please check " {
		t.Fatalf("nodes[0].Text = %q", nodes[0].Text)
	}
	if nodes[1].Type != "mention" {
		t.Fatalf("nodes[1].Type = %q", nodes[1].Type)
	}
	if id := nodes[1].Attrs["id"]; id != "abc-123" {
		t.Fatalf("mention id = %#v", id)
	}
	if text := nodes[1].Attrs["text"]; text != "@Jane Doe" {
		t.Fatalf("mention text = %#v", text)
	}
	if userType := nodes[1].Attrs["userType"]; userType != "DEFAULT" {
		t.Fatalf("mention userType = %#v", userType)
	}
	if nodes[2].Text != " and " {
		t.Fatalf("nodes[2].Text = %q", nodes[2].Text)
	}
	if href := nodes[3].Marks[0].Attrs["href"]; href != "https://example.test" {
		t.Fatalf("url href = %#v", href)
	}
}

func TestSearchUsers(t *testing.T) {
	userSearch := &fakeUserSearchService{
		response: []*model.UserScheme{
			{AccountID: "abc-123", DisplayName: "Jane Doe", EmailAddress: "jane@example.test", Active: true},
			nil,
			{DisplayName: "No Account"},
		},
	}
	client := &Client{
		userSearch: userSearch,
	}

	users, err := client.SearchUsers(context.Background(), "Jane", 5)
	if err != nil {
		t.Fatalf("SearchUsers() error = %v", err)
	}

	if userSearch.query != "Jane" {
		t.Fatalf("query = %q", userSearch.query)
	}
	if userSearch.maxResults != 5 {
		t.Fatalf("maxResults = %d", userSearch.maxResults)
	}
	want := []User{{AccountID: "abc-123", DisplayName: "Jane Doe", Email: "jane@example.test", Active: true}}
	if len(users) != len(want) {
		t.Fatalf("users = %#v", users)
	}
	if users[0] != want[0] {
		t.Fatalf("users[0] = %#v, want %#v", users[0], want[0])
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

type fakeIssueService struct {
	key      string
	fields   []string
	expands  []string
	response *model.IssueScheme
	err      error
}

func (f *fakeIssueService) Get(_ context.Context, key string, fields, expands []string) (*model.IssueScheme, *model.ResponseScheme, error) {
	f.key = key
	f.fields = fields
	f.expands = expands
	return f.response, nil, f.err
}

type fakeCommentService struct {
	key         string
	orderBy     string
	expands     []string
	startAt     int
	maxResults  int
	response    *model.IssueCommentPageScheme
	addKey      string
	payload     *model.CommentPayloadScheme
	addResponse *model.IssueCommentScheme
	err         error
}

type fakeUserSearchService struct {
	query      string
	maxResults int
	response   []*model.UserScheme
	err        error
}

func (f *fakeUserSearchService) Do(_ context.Context, _ string, query string, _ int, maxResults int) ([]*model.UserScheme, *model.ResponseScheme, error) {
	f.query = query
	f.maxResults = maxResults
	return f.response, nil, f.err
}

func (f *fakeCommentService) Gets(_ context.Context, key string, orderBy string, expands []string, startAt, maxResults int) (*model.IssueCommentPageScheme, *model.ResponseScheme, error) {
	f.key = key
	f.orderBy = orderBy
	f.expands = expands
	f.startAt = startAt
	f.maxResults = maxResults
	return f.response, nil, f.err
}

func (f *fakeCommentService) Add(_ context.Context, key string, payload *model.CommentPayloadScheme, expands []string) (*model.IssueCommentScheme, *model.ResponseScheme, error) {
	f.addKey = key
	f.payload = payload
	f.expands = expands
	return f.addResponse, nil, f.err
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
