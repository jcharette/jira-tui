package jira

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	model "github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"
	"github.com/jcharette/jira-tui/internal/adf"
	"github.com/tidwall/gjson"
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

func TestGetBoardsParsesProjectBoards(t *testing.T) {
	boardService := &fakeAgileBoardService{
		boards: &model.BoardPageScheme{
			StartAt:    50,
			MaxResults: 25,
			Total:      75,
			IsLast:     false,
			Values: []*model.BoardScheme{
				{
					ID:   100,
					Name: "ABC Scrum",
					Type: "scrum",
					Location: &model.BoardLocationScheme{
						ProjectKey:  "ABC",
						ProjectName: "App Backend",
					},
				},
			},
		},
	}
	client := &Client{board: boardService}

	page, err := client.GetBoards(context.Background(), "ABC", 50, 25)
	if err != nil {
		t.Fatalf("GetBoards() error = %v", err)
	}

	if boardService.projectKey != "ABC" {
		t.Fatalf("projectKey = %q", boardService.projectKey)
	}
	if boardService.startAt != 50 || boardService.maxResults != 25 {
		t.Fatalf("pagination = %d/%d", boardService.startAt, boardService.maxResults)
	}
	if page.StartAt != 50 || page.MaxResults != 25 || page.Total != 75 || page.IsLast {
		t.Fatalf("page = %#v", page)
	}
	if len(page.Boards) != 1 {
		t.Fatalf("Boards = %#v", page.Boards)
	}
	board := page.Boards[0]
	if board.ID != 100 || board.Name != "ABC Scrum" || board.Type != "scrum" || board.ProjectKey != "ABC" || board.ProjectName != "App Backend" {
		t.Fatalf("board = %#v", board)
	}
}

func TestGetBoardSprintsParsesIncrementalSprintPage(t *testing.T) {
	start := time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC)
	end := start.Add(14 * 24 * time.Hour)
	boardService := &fakeAgileBoardService{
		sprints: &model.BoardSprintPageScheme{
			StartAt:    25,
			MaxResults: 25,
			Total:      26,
			IsLast:     true,
			Values: []*model.BoardSprintScheme{
				{ID: 300, OriginBoardID: 100, Name: "Sprint 42", State: "active", Goal: "Ship the thing", StartDate: start, EndDate: end},
			},
		},
	}
	client := &Client{board: boardService}

	page, err := client.GetBoardSprints(context.Background(), 100, []string{"active", "future"}, 25, 25)
	if err != nil {
		t.Fatalf("GetBoardSprints() error = %v", err)
	}

	if boardService.boardID != 100 {
		t.Fatalf("boardID = %d", boardService.boardID)
	}
	if !reflect.DeepEqual(boardService.states, []string{"active", "future"}) {
		t.Fatalf("states = %#v", boardService.states)
	}
	if page.BoardID != 100 || page.StartAt != 25 || page.MaxResults != 25 || page.Total != 26 || !page.IsLast {
		t.Fatalf("page = %#v", page)
	}
	if len(page.Sprints) != 1 {
		t.Fatalf("Sprints = %#v", page.Sprints)
	}
	sprint := page.Sprints[0]
	if sprint.ID != 300 || sprint.BoardID != 100 || sprint.Name != "Sprint 42" || sprint.State != "active" || sprint.Goal != "Ship the thing" {
		t.Fatalf("sprint = %#v", sprint)
	}
	if !sprint.StartDate.Equal(start) || !sprint.EndDate.Equal(end) {
		t.Fatalf("sprint dates = %#v", sprint)
	}
}

func TestMoveIssuesToSprintPostsIssueKeys(t *testing.T) {
	rest := &fakeRESTConnector{}
	client := &Client{rest: rest}

	err := client.MoveIssuesToSprint(context.Background(), 300, []string{" abc-1 ", "ABC-2"})
	if err != nil {
		t.Fatalf("MoveIssuesToSprint() error = %v", err)
	}

	if rest.method != http.MethodPost {
		t.Fatalf("method = %q", rest.method)
	}
	if rest.endpoint != "rest/agile/1.0/sprint/300/issue" {
		t.Fatalf("endpoint = %q", rest.endpoint)
	}
	body, ok := rest.body.(map[string][]string)
	if !ok {
		t.Fatalf("body type = %T", rest.body)
	}
	want := []string{"ABC-1", "ABC-2"}
	if !reflect.DeepEqual(body["issues"], want) {
		t.Fatalf("issues = %#v, want %#v", body["issues"], want)
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
				IssueLinks: []*model.IssueLinkScheme{
					{
						Type: &model.LinkTypeScheme{Name: "Blocks", Outward: "blocks"},
						OutwardIssue: &model.LinkedIssueScheme{
							Key: "ABC-200",
							Fields: &model.IssueLinkFieldsScheme{
								Summary:   "Blocked downstream task",
								Status:    &model.StatusScheme{Name: "To Do"},
								IssueType: &model.IssueTypeScheme{Name: "Task"},
							},
						},
					},
					{
						Type: &model.LinkTypeScheme{Name: "Blocks", Inward: "is blocked by"},
						InwardIssue: &model.LinkedIssueScheme{
							Key: "ABC-300",
							Fields: &model.IssueLinkFieldsScheme{
								Summary:   "Upstream blocker",
								Status:    &model.StatusScheme{Name: "In Progress"},
								IssueType: &model.IssueTypeScheme{Name: "Bug"},
							},
						},
					},
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
		"issuelinks",
		"timetracking",
	}
	if !equalStrings(issue.fields, wantFields) {
		t.Fatalf("fields = %#v", issue.fields)
	}
	if detail.Key != "ABC-123" {
		t.Fatalf("Key = %q", detail.Key)
	}
	if detail.Description != "First line.\n\nSecond line." {
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
	wantLinks := []IssueLink{
		{
			Direction:    "outward",
			Relationship: "blocks",
			Key:          "ABC-200",
			Summary:      "Blocked downstream task",
			Status:       "To Do",
			IssueType:    "Task",
			URL:          "https://example.atlassian.net/browse/ABC-200",
		},
		{
			Direction:    "inward",
			Relationship: "is blocked by",
			Key:          "ABC-300",
			Summary:      "Upstream blocker",
			Status:       "In Progress",
			IssueType:    "Bug",
			URL:          "https://example.atlassian.net/browse/ABC-300",
		},
	}
	if !reflect.DeepEqual(detail.IssueLinks, wantLinks) {
		t.Fatalf("IssueLinks = %#v", detail.IssueLinks)
	}
}

func TestGetIssueViaRESTParsesTimeTracking(t *testing.T) {
	rest := &fakeRESTConnector{
		issueResponse: issueDetailResponse{
			IssueScheme: model.IssueScheme{
				Key: "ABC-123",
				Fields: &model.IssueFieldsScheme{
					Summary: "Estimate work",
				},
			},
			RawFields: map[string]json.RawMessage{
				"timetracking": []byte(`{"originalEstimate":"2d","remainingEstimate":"3h"}`),
			},
		},
	}
	client := &Client{
		baseURL: "https://example.atlassian.net",
		rest:    rest,
	}

	detail, err := client.GetIssue(context.Background(), "ABC-123")
	if err != nil {
		t.Fatalf("GetIssue() error = %v", err)
	}
	if rest.method != http.MethodGet {
		t.Fatalf("method = %q", rest.method)
	}
	if rest.endpoint != "rest/api/3/issue/ABC-123?fields=summary%2Cstatus%2Cassignee%2Cpriority%2Cissuetype%2Cparent%2Csubtasks%2Cdescription%2Clabels%2Ccomponents%2CfixVersions%2Ccreated%2Cupdated%2Creporter%2Ccreator%2Cissuelinks%2Ctimetracking" {
		t.Fatalf("endpoint = %q", rest.endpoint)
	}
	if detail.OriginalEstimate != "2d" || detail.RemainingEstimate != "3h" {
		t.Fatalf("estimates = %q/%q", detail.OriginalEstimate, detail.RemainingEstimate)
	}
}

func TestGetIssueDescriptionADFReturnsRawDescription(t *testing.T) {
	description := &model.CommentNodeScheme{
		Type: "doc",
		Content: []*model.CommentNodeScheme{
			{
				Type: "paragraph",
				Content: []*model.CommentNodeScheme{
					{Type: "text", Text: "Raw description."},
				},
			},
		},
	}
	issue := &fakeIssueService{
		response: &model.IssueScheme{
			Key: "ABC-123",
			Fields: &model.IssueFieldsScheme{
				Description: description,
			},
		},
	}
	client := &Client{issue: issue}

	got, err := client.GetIssueDescriptionADF(context.Background(), "ABC-123")
	if err != nil {
		t.Fatalf("GetIssueDescriptionADF() error = %v", err)
	}

	if issue.key != "ABC-123" {
		t.Fatalf("key = %q", issue.key)
	}
	if !equalStrings(issue.fields, []string{"description"}) {
		t.Fatalf("fields = %#v", issue.fields)
	}
	if got != description {
		t.Fatalf("raw description pointer changed: got %#v want %#v", got, description)
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

func TestGetTransitionsParsesAvailableStatusTransitions(t *testing.T) {
	issue := &fakeIssueService{
		transitions: &model.IssueTransitionsScheme{
			Transitions: []*model.IssueTransitionScheme{
				{
					ID:          "21",
					Name:        "Start Progress",
					IsAvailable: true,
					HasScreen:   true,
					To:          &model.StatusScheme{Name: "In Progress"},
				},
				{
					ID:          "31",
					Name:        "Done",
					IsAvailable: true,
					To:          &model.StatusScheme{Name: "Done"},
				},
			},
		},
	}
	client := &Client{
		issue: issue,
	}

	transitions, err := client.GetTransitions(context.Background(), "ABC-123")
	if err != nil {
		t.Fatalf("GetTransitions() error = %v", err)
	}

	if issue.transitionKey != "ABC-123" {
		t.Fatalf("transitionKey = %q", issue.transitionKey)
	}
	if len(transitions) != 2 {
		t.Fatalf("transitions = %#v", transitions)
	}
	if transitions[0].ID != "21" || transitions[0].Name != "Start Progress" {
		t.Fatalf("transitions[0] = %#v", transitions[0])
	}
	if transitions[0].ToStatus != "In Progress" {
		t.Fatalf("ToStatus = %q", transitions[0].ToStatus)
	}
	if !transitions[0].HasScreen {
		t.Fatal("expected HasScreen")
	}
	if !transitions[0].IsAvailable {
		t.Fatal("expected IsAvailable")
	}
}

func TestGetTransitionsParsesTransitionFields(t *testing.T) {
	rest := &fakeRESTConnector{
		transitionResponse: transitionFieldsResponse{
			Transitions: []transitionFieldsRaw{
				{
					ID:          "31",
					Name:        "Done",
					IsAvailable: true,
					HasScreen:   true,
					To:          &model.StatusScheme{Name: "Done"},
					Fields: map[string]transitionFieldRaw{
						"customfield_10022": {
							Name:            "Reviewer",
							Required:        true,
							AutoCompleteURL: "https://example.atlassian.net/rest/api/3/user/picker?fieldName=customfield_10022",
							Schema: transitionFieldSchema{
								Type:   "user",
								Custom: "com.atlassian.jira.plugin.system.customfieldtypes:userpicker",
							},
							AllowedValues: []transitionAllowedValue{
								{AccountID: "abc-123", DisplayName: "Jane Doe"},
							},
						},
						"resolution": {
							Name:     "Resolution",
							Required: true,
							Schema: transitionFieldSchema{
								Type:   "resolution",
								System: "resolution",
							},
							AllowedValues: []transitionAllowedValue{
								{ID: "10000", Name: "Done"},
								{ID: "10001", Name: "Won't Do"},
							},
						},
						"comment": {
							Name:     "Comment",
							Required: false,
							Schema: transitionFieldSchema{
								Type:   "array",
								System: "comment",
							},
						},
					},
				},
			},
		},
	}
	client := &Client{rest: rest}

	transitions, err := client.GetTransitions(context.Background(), "ABC-123")
	if err != nil {
		t.Fatalf("GetTransitions() error = %v", err)
	}

	if rest.method != http.MethodGet {
		t.Fatalf("method = %q", rest.method)
	}
	if rest.endpoint != "rest/api/3/issue/ABC-123/transitions?expand=transitions.fields" {
		t.Fatalf("endpoint = %q", rest.endpoint)
	}
	if len(transitions) != 1 {
		t.Fatalf("transitions = %#v", transitions)
	}
	fields := transitions[0].Fields
	if len(fields) != 3 {
		t.Fatalf("fields = %#v", fields)
	}
	if fields[0].ID != "comment" || fields[0].Name != "Comment" || fields[0].Required {
		t.Fatalf("comment field = %#v", fields[0])
	}
	if fields[1].ID != "customfield_10022" || fields[1].SchemaType != "user" || fields[1].AutoCompleteURL == "" {
		t.Fatalf("custom user field = %#v", fields[1])
	}
	if len(fields[1].AllowedValues) != 1 || fields[1].AllowedValues[0].ID != "abc-123" || fields[1].AllowedValues[0].Name != "Jane Doe" {
		t.Fatalf("custom user allowed values = %#v", fields[1].AllowedValues)
	}
	if fields[2].ID != "resolution" || fields[2].Name != "Resolution" || !fields[2].Required {
		t.Fatalf("resolution field = %#v", fields[2])
	}
	if len(fields[2].AllowedValues) != 2 || fields[2].AllowedValues[0].ID != "10000" {
		t.Fatalf("resolution allowed values = %#v", fields[2].AllowedValues)
	}
}

func TestGetTransitionsWrapsTransitionError(t *testing.T) {
	issue := &fakeIssueService{
		transitionErr: errors.New("jira unavailable"),
	}
	client := &Client{
		issue: issue,
	}

	_, err := client.GetTransitions(context.Background(), "ABC-123")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, issue.transitionErr) {
		t.Fatalf("error = %v", err)
	}
}

func TestTransitionIssueSendsTransitionID(t *testing.T) {
	issue := &fakeIssueService{}
	client := &Client{
		issue: issue,
	}

	err := client.TransitionIssue(context.Background(), "ABC-123", TransitionIssueRequest{TransitionID: "21"})
	if err != nil {
		t.Fatalf("TransitionIssue() error = %v", err)
	}

	if issue.moveKey != "ABC-123" {
		t.Fatalf("moveKey = %q", issue.moveKey)
	}
	if issue.transitionID != "21" {
		t.Fatalf("transitionID = %q", issue.transitionID)
	}
	if issue.moveOptions != nil {
		t.Fatalf("moveOptions = %#v", issue.moveOptions)
	}
}

func TestTransitionIssueSendsResolutionAndCommentFields(t *testing.T) {
	issue := &fakeIssueService{}
	client := &Client{
		issue: issue,
	}

	err := client.TransitionIssue(context.Background(), "ABC-123", TransitionIssueRequest{
		TransitionID: "31",
		Fields: []TransitionFieldValue{
			{FieldID: "resolution", Option: FieldOption{ID: "10000", Name: "Done"}},
			{FieldID: "comment", Text: "Ship **this** now"},
		},
	})
	if err != nil {
		t.Fatalf("TransitionIssue() error = %v", err)
	}

	if issue.moveKey != "ABC-123" || issue.transitionID != "31" {
		t.Fatalf("move = %s/%s", issue.moveKey, issue.transitionID)
	}
	if issue.moveOptions == nil || issue.moveOptions.Fields == nil || issue.moveOptions.Fields.Fields == nil {
		t.Fatalf("moveOptions missing fields: %#v", issue.moveOptions)
	}
	if issue.moveOptions.Fields.Fields.Resolution == nil || issue.moveOptions.Fields.Fields.Resolution.ID != "10000" {
		t.Fatalf("resolution = %#v", issue.moveOptions.Fields.Fields.Resolution)
	}
	if issue.moveOptions.Operations == nil || len(issue.moveOptions.Operations.Fields) != 1 {
		t.Fatalf("operations = %#v", issue.moveOptions.Operations)
	}
	update := issue.moveOptions.Operations.Fields[0]["update"].(map[string]interface{})
	commentOps := update["comment"].([]map[string]interface{})
	add := commentOps[0]["add"].(map[string]interface{})
	body := add["body"].(*model.CommentNodeScheme)
	nodes := body.Content[0].Content
	if nodes[0].Text != "Ship " || nodes[1].Text != "this" || nodes[1].Marks[0].Type != "strong" {
		t.Fatalf("comment nodes = %#v", nodes)
	}
}

func TestTransitionIssueSendsCustomOptionField(t *testing.T) {
	issue := &fakeIssueService{}
	client := &Client{
		issue: issue,
	}

	err := client.TransitionIssue(context.Background(), "ABC-123", TransitionIssueRequest{
		TransitionID: "31",
		Fields: []TransitionFieldValue{
			{FieldID: "customfield_10010", SchemaType: "option", Option: FieldOption{ID: "20001", Name: "Production"}},
		},
	})
	if err != nil {
		t.Fatalf("TransitionIssue() error = %v", err)
	}

	if issue.moveOptions == nil || issue.moveOptions.CustomFields == nil || len(issue.moveOptions.CustomFields.Fields) != 1 {
		t.Fatalf("custom fields = %#v", issue.moveOptions)
	}
	fieldsNode := issue.moveOptions.CustomFields.Fields[0]["fields"].(map[string]interface{})
	option := fieldsNode["customfield_10010"].(map[string]interface{})
	if option["id"] != "20001" || option["value"] != "Production" {
		t.Fatalf("custom option = %#v", option)
	}
}

func TestTransitionIssueSendsCustomTextDateUserAndMultiSelectFields(t *testing.T) {
	issue := &fakeIssueService{}
	client := &Client{
		issue: issue,
	}

	err := client.TransitionIssue(context.Background(), "ABC-123", TransitionIssueRequest{
		TransitionID: "31",
		Fields: []TransitionFieldValue{
			{FieldID: "customfield_10020", SchemaType: "string", Text: "Root cause text"},
			{FieldID: "customfield_10021", SchemaType: "date", Text: "2026-06-20"},
			{FieldID: "customfield_10022", SchemaType: "user", Option: FieldOption{ID: "abc-123", Name: "Jane Doe"}},
			{
				FieldID:     "customfield_10023",
				SchemaType:  "array",
				SchemaItems: "option",
				Options: []FieldOption{
					{ID: "1", Name: "Backend"},
					{ID: "2", Name: "Frontend"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("TransitionIssue() error = %v", err)
	}

	custom := mergedTransitionCustomFieldsForTest(t, issue.moveOptions.CustomFields)
	if custom["customfield_10020"] != "Root cause text" {
		t.Fatalf("text field = %#v", custom["customfield_10020"])
	}
	if custom["customfield_10021"] != "2026-06-20" {
		t.Fatalf("date field = %#v", custom["customfield_10021"])
	}
	user := custom["customfield_10022"].(map[string]interface{})
	if user["accountId"] != "abc-123" {
		t.Fatalf("user field = %#v", user)
	}
	options := custom["customfield_10023"].([]map[string]interface{})
	if len(options) != 2 || options[0]["id"] != "1" || options[1]["value"] != "Frontend" {
		t.Fatalf("multi-select field = %#v", options)
	}
}

func mergedTransitionCustomFieldsForTest(t *testing.T, customFields *model.CustomFields) map[string]interface{} {
	t.Helper()
	if customFields == nil {
		t.Fatal("expected custom fields")
	}
	merged := make(map[string]interface{})
	for _, field := range customFields.Fields {
		fieldsNode, ok := field["fields"].(map[string]interface{})
		if !ok {
			t.Fatalf("custom field node = %#v", field)
		}
		for key, value := range fieldsNode {
			merged[key] = value
		}
	}
	return merged
}

func TestTransitionIssueWrapsTransitionError(t *testing.T) {
	issue := &fakeIssueService{
		moveErr: errors.New("transition failed"),
	}
	client := &Client{
		issue: issue,
	}

	err := client.TransitionIssue(context.Background(), "ABC-123", TransitionIssueRequest{TransitionID: "21"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, issue.moveErr) {
		t.Fatalf("error = %v", err)
	}
}

func TestGetEditMetadataParsesSummaryField(t *testing.T) {
	metadata := &fakeMetadataService{
		response: `{
			"fields": {
				"summary": {
					"required": true,
					"name": "Summary",
					"schema": {"type": "string"},
					"operations": ["set"]
				}
			}
		}`,
	}
	client := &Client{
		metadata: metadata,
	}

	edit, err := client.GetEditMetadata(context.Background(), "ABC-123")
	if err != nil {
		t.Fatalf("GetEditMetadata() error = %v", err)
	}

	if metadata.key != "ABC-123" {
		t.Fatalf("key = %q", metadata.key)
	}
	if !edit.Summary.Editable {
		t.Fatal("expected summary editable")
	}
	if !edit.Summary.Required {
		t.Fatal("expected summary required")
	}
	if edit.Summary.Name != "Summary" {
		t.Fatalf("Name = %q", edit.Summary.Name)
	}
}

func TestGetEditMetadataParsesPriorityAllowedValues(t *testing.T) {
	metadata := &fakeMetadataService{
		response: `{
			"fields": {
				"priority": {
					"required": false,
					"name": "Priority",
					"schema": {"type": "priority"},
					"operations": ["set"],
					"allowedValues": [
						{"id": "2", "name": "High"},
						{"id": "3", "name": "Medium"},
						{"id": "4", "name": "Low"}
					]
				}
			}
		}`,
	}
	client := &Client{
		metadata: metadata,
	}

	edit, err := client.GetEditMetadata(context.Background(), "ABC-123")
	if err != nil {
		t.Fatalf("GetEditMetadata() error = %v", err)
	}

	if !edit.Priority.Editable {
		t.Fatal("expected priority editable")
	}
	if edit.Priority.Name != "Priority" {
		t.Fatalf("Name = %q", edit.Priority.Name)
	}
	want := []FieldOption{
		{ID: "2", Name: "High"},
		{ID: "3", Name: "Medium"},
		{ID: "4", Name: "Low"},
	}
	if !reflect.DeepEqual(edit.Priority.AllowedValues, want) {
		t.Fatalf("AllowedValues = %#v, want %#v", edit.Priority.AllowedValues, want)
	}
}

func TestGetEditMetadataParsesLabelsField(t *testing.T) {
	metadata := &fakeMetadataService{
		response: `{
			"fields": {
				"labels": {
					"required": false,
					"name": "Labels",
					"schema": {"type": "array", "items": "string", "system": "labels"},
					"operations": ["set"]
				}
			}
		}`,
	}
	client := &Client{
		metadata: metadata,
	}

	edit, err := client.GetEditMetadata(context.Background(), "ABC-123")
	if err != nil {
		t.Fatalf("GetEditMetadata() error = %v", err)
	}

	if !edit.Labels.Editable {
		t.Fatal("expected labels editable")
	}
	if edit.Labels.Name != "Labels" {
		t.Fatalf("Name = %q", edit.Labels.Name)
	}
}

func TestGetEditMetadataParsesComponentsAllowedValues(t *testing.T) {
	metadata := &fakeMetadataService{
		response: `{
			"fields": {
				"components": {
					"required": false,
					"name": "Components",
					"schema": {"type": "array", "items": "component", "system": "components"},
					"operations": ["set"],
					"allowedValues": [
						{"id": "101", "name": "Platform"},
						{"id": "102", "name": "API"}
					]
				}
			}
		}`,
	}
	client := &Client{
		metadata: metadata,
	}

	edit, err := client.GetEditMetadata(context.Background(), "ABC-123")
	if err != nil {
		t.Fatalf("GetEditMetadata() error = %v", err)
	}

	if !edit.Components.Editable {
		t.Fatal("expected components editable")
	}
	want := []FieldOption{{ID: "101", Name: "Platform"}, {ID: "102", Name: "API"}}
	if !reflect.DeepEqual(edit.Components.AllowedValues, want) {
		t.Fatalf("AllowedValues = %#v, want %#v", edit.Components.AllowedValues, want)
	}
}

func TestGetEditMetadataParsesAllEditableFields(t *testing.T) {
	metadata := &fakeMetadataService{
		response: `{
			"fields": {
				"priority": {
					"required": false,
					"name": "Priority",
					"schema": {"type": "priority", "system": "priority"},
					"operations": ["set"],
					"allowedValues": [{"id": "3", "name": "Medium"}]
				},
				"customfield_10016": {
					"required": false,
					"name": "Story Points",
					"schema": {
						"type": "number",
						"custom": "com.atlassian.jira.plugin.system.customfieldtypes:float",
						"customId": 10016
					},
					"operations": ["set"]
				},
				"summary": {
					"required": true,
					"name": "Summary",
					"schema": {"type": "string", "system": "summary"},
					"operations": ["set"]
				},
				"customfield_10017": {
					"required": false,
					"name": "Read Only Team",
					"schema": {"type": "string"},
					"operations": ["view"]
				}
			}
		}`,
	}
	client := &Client{
		metadata: metadata,
	}

	edit, err := client.GetEditMetadata(context.Background(), "ABC-123")
	if err != nil {
		t.Fatalf("GetEditMetadata() error = %v", err)
	}

	if !edit.Summary.Editable || edit.Summary.Name != "Summary" {
		t.Fatalf("Summary = %#v", edit.Summary)
	}
	if !edit.Priority.Editable || len(edit.Priority.AllowedValues) != 1 {
		t.Fatalf("Priority = %#v", edit.Priority)
	}
	want := []EditField{
		{ID: "priority", Name: "Priority", Editable: true, SchemaType: "priority", SchemaSystem: "priority", Operations: []string{"set"}, AllowedValues: []FieldOption{{ID: "3", Name: "Medium"}}},
		{ID: "customfield_10017", Name: "Read Only Team", SchemaType: "string", Operations: []string{"view"}},
		{ID: "customfield_10016", Name: "Story Points", Editable: true, SchemaType: "number", SchemaCustom: "com.atlassian.jira.plugin.system.customfieldtypes:float", SchemaCustomID: 10016, Operations: []string{"set"}},
		{ID: "summary", Name: "Summary", Required: true, Editable: true, SchemaType: "string", SchemaSystem: "summary", Operations: []string{"set"}},
	}
	if !reflect.DeepEqual(edit.Fields, want) {
		t.Fatalf("Fields = %#v, want %#v", edit.Fields, want)
	}
}

func TestGetEditMetadataParsesAutocompleteOptionSource(t *testing.T) {
	metadata := &fakeMetadataService{
		response: `{
			"fields": {
				"customfield_10020": {
					"required": false,
					"name": "Target Team",
					"hasDefaultValue": true,
					"schema": {
						"type": "array",
						"items": "option",
						"custom": "com.atlassian.jira.plugin.system.customfieldtypes:multiselect",
						"customId": 10020
					},
					"operations": ["add", "remove", "set"],
					"autoCompleteUrl": "https://example.atlassian.net/rest/api/3/customFieldOption/suggest"
				}
			}
		}`,
	}
	client := &Client{metadata: metadata}

	edit, err := client.GetEditMetadata(context.Background(), "ABC-123")
	if err != nil {
		t.Fatalf("GetEditMetadata() error = %v", err)
	}

	if len(edit.Fields) != 1 {
		t.Fatalf("Fields = %#v", edit.Fields)
	}
	want := EditField{
		ID:              "customfield_10020",
		Name:            "Target Team",
		HasDefaultValue: true,
		Editable:        true,
		SchemaType:      "array",
		SchemaItems:     "option",
		SchemaCustom:    "com.atlassian.jira.plugin.system.customfieldtypes:multiselect",
		SchemaCustomID:  10020,
		Operations:      []string{"add", "remove", "set"},
		AutoCompleteURL: "https://example.atlassian.net/rest/api/3/customFieldOption/suggest",
	}
	if !reflect.DeepEqual(edit.Fields[0], want) {
		t.Fatalf("field = %#v, want %#v", edit.Fields[0], want)
	}
}

func TestSearchFieldOptionsUsesAutocompleteURLAndParsesResults(t *testing.T) {
	rest := &fakeRESTConnector{
		fieldOptionResponse: fieldOptionSearchResponse{
			Results: []fieldOptionSearchItem{
				{ID: "101", DisplayName: "Platform"},
				{Value: "Security"},
			},
		},
	}
	client := &Client{
		baseURL: "https://example.atlassian.net",
		rest:    rest,
	}

	options, err := client.SearchFieldOptions(context.Background(), "https://example.atlassian.net/rest/api/3/customFieldOption/suggest?fieldName=customfield_10010", "plat", 25)
	if err != nil {
		t.Fatalf("SearchFieldOptions() error = %v", err)
	}

	if rest.method != http.MethodGet {
		t.Fatalf("method = %q", rest.method)
	}
	wantEndpoint := "rest/api/3/customFieldOption/suggest?fieldName=customfield_10010&maxResults=25&query=plat"
	if rest.endpoint != wantEndpoint {
		t.Fatalf("endpoint = %q, want %q", rest.endpoint, wantEndpoint)
	}
	want := []FieldOption{{ID: "101", Name: "Platform"}, {Name: "Security"}}
	if !reflect.DeepEqual(options, want) {
		t.Fatalf("options = %#v, want %#v", options, want)
	}
}

func TestGetEditMetadataWrapsMetadataError(t *testing.T) {
	metadata := &fakeMetadataService{
		err: errors.New("metadata unavailable"),
	}
	client := &Client{
		metadata: metadata,
	}

	_, err := client.GetEditMetadata(context.Background(), "ABC-123")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, metadata.err) {
		t.Fatalf("error = %v", err)
	}
}

func TestGetCreateIssueTypesParsesProjectIssueTypes(t *testing.T) {
	metadata := &fakeMetadataService{
		issueTypesResponse: `{
			"values": [
				{
					"id": "10001",
					"name": "Task",
					"description": "Work item",
					"subtask": false,
					"hierarchyLevel": 0
				},
				{
					"id": "10002",
					"name": "Sub-task",
					"description": "Smaller work item",
					"subtask": true,
					"hierarchyLevel": -1
				}
			]
		}`,
	}
	client := &Client{metadata: metadata}

	issueTypes, err := client.GetCreateIssueTypes(context.Background(), "ABC")
	if err != nil {
		t.Fatalf("GetCreateIssueTypes() error = %v", err)
	}

	if metadata.projectKey != "ABC" {
		t.Fatalf("projectKey = %q", metadata.projectKey)
	}
	want := []CreateIssueType{
		{ID: "10001", Name: "Task", Description: "Work item", HierarchyLevel: 0},
		{ID: "10002", Name: "Sub-task", Description: "Smaller work item", Subtask: true, HierarchyLevel: -1},
	}
	if !reflect.DeepEqual(issueTypes, want) {
		t.Fatalf("issueTypes = %#v, want %#v", issueTypes, want)
	}
}

func TestGetCreateIssueTypesFallsBackToExpandedCreateMetadata(t *testing.T) {
	metadata := &fakeMetadataService{
		issueTypesResponse: `{"values": []}`,
		createResponse: `{
			"projects": [
				{
					"key": "DEVOPS",
					"issuetypes": [
						{
							"id": "10001",
							"name": "Task",
							"description": "Work item",
							"subtask": false,
							"hierarchyLevel": 0
						},
						{
							"id": "10002",
							"name": "Epic",
							"description": "Large work item",
							"subtask": false,
							"hierarchyLevel": 1
						}
					]
				}
			]
		}`,
	}
	client := &Client{metadata: metadata}

	issueTypes, err := client.GetCreateIssueTypes(context.Background(), "DEVOPS")
	if err != nil {
		t.Fatalf("GetCreateIssueTypes() error = %v", err)
	}

	if !metadata.createCalled {
		t.Fatal("expected expanded create metadata fallback")
	}
	if !reflect.DeepEqual(metadata.createOpts.ProjectKeys, []string{"DEVOPS"}) {
		t.Fatalf("ProjectKeys = %#v", metadata.createOpts.ProjectKeys)
	}
	if metadata.createOpts.Expand != "projects.issuetypes" {
		t.Fatalf("Expand = %q", metadata.createOpts.Expand)
	}
	want := []CreateIssueType{
		{ID: "10001", Name: "Task", Description: "Work item", HierarchyLevel: 0},
		{ID: "10002", Name: "Epic", Description: "Large work item", HierarchyLevel: 1},
	}
	if !reflect.DeepEqual(issueTypes, want) {
		t.Fatalf("issueTypes = %#v, want %#v", issueTypes, want)
	}
}

func TestGetCreateFieldsParsesRequiredAllowedAndSchemaData(t *testing.T) {
	metadata := &fakeMetadataService{
		fieldsResponse: `{
			"values": [
				{
					"fieldId": "summary",
					"key": "summary",
					"name": "Summary",
					"required": true,
					"hasDefaultValue": false,
					"schema": {"type": "string", "system": "summary"},
					"operations": ["set"]
				},
				{
					"fieldId": "priority",
					"key": "priority",
					"name": "Priority",
					"required": false,
					"hasDefaultValue": true,
					"schema": {"type": "priority", "system": "priority"},
					"operations": ["set"],
					"allowedValues": [
						{"id": "2", "name": "High"},
						{"id": "3", "name": "Medium"}
					]
				},
				{
					"fieldId": "customfield_10010",
					"key": "customfield_10010",
					"name": "Business Unit",
					"required": false,
					"schema": {
						"type": "array",
						"items": "option",
						"custom": "com.atlassian.jira.plugin.system.customfieldtypes:multiselect",
						"customId": 10010
					},
					"operations": ["add", "remove", "set"],
					"allowedValues": [
						{"id": "11", "value": "Platform"}
					],
					"autoCompleteUrl": "https://example.atlassian.net/rest/api/3/custom"
				}
			]
		}`,
	}
	client := &Client{metadata: metadata}

	fields, err := client.GetCreateFields(context.Background(), "ABC", "10001")
	if err != nil {
		t.Fatalf("GetCreateFields() error = %v", err)
	}

	if metadata.fieldsProjectKey != "ABC" {
		t.Fatalf("fieldsProjectKey = %q", metadata.fieldsProjectKey)
	}
	if metadata.issueTypeID != "10001" {
		t.Fatalf("issueTypeID = %q", metadata.issueTypeID)
	}
	want := []CreateField{
		{
			ID:           "summary",
			Key:          "summary",
			Name:         "Summary",
			Required:     true,
			SchemaType:   "string",
			SchemaSystem: "summary",
			Operations:   []string{"set"},
		},
		{
			ID:              "priority",
			Key:             "priority",
			Name:            "Priority",
			HasDefaultValue: true,
			SchemaType:      "priority",
			SchemaSystem:    "priority",
			Operations:      []string{"set"},
			AllowedValues:   []FieldOption{{ID: "2", Name: "High"}, {ID: "3", Name: "Medium"}},
		},
		{
			ID:              "customfield_10010",
			Key:             "customfield_10010",
			Name:            "Business Unit",
			SchemaType:      "array",
			SchemaItems:     "option",
			SchemaCustom:    "com.atlassian.jira.plugin.system.customfieldtypes:multiselect",
			SchemaCustomID:  10010,
			Operations:      []string{"add", "remove", "set"},
			AllowedValues:   []FieldOption{{ID: "11", Name: "Platform"}},
			AutoCompleteURL: "https://example.atlassian.net/rest/api/3/custom",
		},
	}
	if !reflect.DeepEqual(fields, want) {
		t.Fatalf("fields = %#v, want %#v", fields, want)
	}
}

func TestGetCreateFieldsFallsBackToExpandedCreateMetadata(t *testing.T) {
	metadata := &fakeMetadataService{
		fieldsResponse: `{"values": []}`,
		createResponse: `{
			"projects": [
				{
					"key": "DEVOPS",
					"issuetypes": [
						{
							"id": "10001",
							"name": "Task",
							"fields": {
								"summary": {
									"required": true,
									"schema": {"type": "string", "system": "summary"},
									"name": "Summary",
									"key": "summary",
									"operations": ["set"]
								},
								"priority": {
									"required": false,
									"schema": {"type": "priority", "system": "priority"},
									"name": "Priority",
									"key": "priority",
									"operations": ["set"],
									"allowedValues": [
										{"id": "3", "name": "Medium"}
									]
								}
							}
						},
						{
							"id": "10002",
							"name": "Story",
							"fields": {
								"customfield_10020": {
									"name": "Team",
									"schema": {"type": "option", "customId": 10020}
								}
							}
						}
					]
				}
			]
		}`,
	}
	client := &Client{metadata: metadata}

	fields, err := client.GetCreateFields(context.Background(), "DEVOPS", "10001")
	if err != nil {
		t.Fatalf("GetCreateFields() error = %v", err)
	}

	if !metadata.createCalled {
		t.Fatal("expected expanded create metadata fallback")
	}
	if !reflect.DeepEqual(metadata.createOpts.ProjectKeys, []string{"DEVOPS"}) {
		t.Fatalf("ProjectKeys = %#v", metadata.createOpts.ProjectKeys)
	}
	if !reflect.DeepEqual(metadata.createOpts.IssueTypeIDs, []string{"10001"}) {
		t.Fatalf("IssueTypeIDs = %#v", metadata.createOpts.IssueTypeIDs)
	}
	if metadata.createOpts.Expand != "projects.issuetypes.fields" {
		t.Fatalf("Expand = %q", metadata.createOpts.Expand)
	}
	want := []CreateField{
		{
			ID:            "priority",
			Key:           "priority",
			Name:          "Priority",
			SchemaType:    "priority",
			SchemaSystem:  "priority",
			Operations:    []string{"set"},
			AllowedValues: []FieldOption{{ID: "3", Name: "Medium"}},
		},
		{
			ID:           "summary",
			Key:          "summary",
			Name:         "Summary",
			Required:     true,
			SchemaType:   "string",
			SchemaSystem: "summary",
			Operations:   []string{"set"},
		},
	}
	if !reflect.DeepEqual(fields, want) {
		t.Fatalf("fields = %#v, want %#v", fields, want)
	}
}

func TestGetCreateMetadataWrapsMetadataErrors(t *testing.T) {
	metadata := &fakeMetadataService{err: errors.New("metadata unavailable")}
	client := &Client{metadata: metadata}

	_, err := client.GetCreateIssueTypes(context.Background(), "ABC")
	if err == nil {
		t.Fatal("expected create issue types error")
	}
	if !errors.Is(err, metadata.err) {
		t.Fatalf("issue types error = %v", err)
	}

	_, err = client.GetCreateFields(context.Background(), "ABC", "10001")
	if err == nil {
		t.Fatal("expected create fields error")
	}
	if !errors.Is(err, metadata.err) {
		t.Fatalf("fields error = %v", err)
	}
}

func TestCreateIssueSendsProjectTypeSummaryAndDescription(t *testing.T) {
	issue := &fakeIssueService{
		createResponse: &model.IssueResponseScheme{ID: "10001", Key: "ABC-123", Self: "https://example.atlassian.net/rest/api/3/issue/10001"},
	}
	client := &Client{
		baseURL: "https://example.atlassian.net",
		issue:   issue,
	}

	created, err := client.CreateIssue(context.Background(), CreateIssueRequest{
		ProjectKey:  "ABC",
		IssueTypeID: "10001",
		Summary:     "New platform task",
		Description: "First paragraph.\nSecond line.",
	})
	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}

	if issue.createPayload == nil || issue.createPayload.Fields == nil {
		t.Fatal("expected create payload fields")
	}
	fields := issue.createPayload.Fields
	if fields.Project == nil || fields.Project.Key != "ABC" {
		t.Fatalf("Project = %#v", fields.Project)
	}
	if fields.IssueType == nil || fields.IssueType.ID != "10001" {
		t.Fatalf("IssueType = %#v", fields.IssueType)
	}
	if fields.Summary != "New platform task" {
		t.Fatalf("Summary = %q", fields.Summary)
	}
	if fields.Description == nil {
		t.Fatal("expected ADF description")
	}
	if got := adf.Render(fields.Description); got != "First paragraph.\nSecond line." {
		t.Fatalf("Description = %q", got)
	}
	if issue.createCustomFields != nil {
		t.Fatalf("custom fields = %#v", issue.createCustomFields)
	}
	if created.Key != "ABC-123" || created.URL != "https://example.atlassian.net/browse/ABC-123" {
		t.Fatalf("created = %#v", created)
	}
}

func TestCreateIssueSendsParentKeyForSubtask(t *testing.T) {
	issue := &fakeIssueService{
		createResponse: &model.IssueResponseScheme{ID: "10002", Key: "ABC-124", Self: "https://example.atlassian.net/rest/api/3/issue/10002"},
	}
	client := &Client{
		baseURL: "https://example.atlassian.net",
		issue:   issue,
	}

	_, err := client.CreateIssue(context.Background(), CreateIssueRequest{
		ProjectKey:  "ABC",
		IssueTypeID: "10002",
		ParentKey:   "ABC-123",
		Summary:     "Add regression coverage",
	})
	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}

	if issue.createPayload == nil || issue.createPayload.Fields == nil {
		t.Fatal("expected create payload fields")
	}
	if issue.createPayload.Fields.Parent == nil || issue.createPayload.Fields.Parent.Key != "ABC-123" {
		t.Fatalf("Parent = %#v", issue.createPayload.Fields.Parent)
	}
}

func TestCreateIssueSendsPriorityLabelsAndCustomFieldValues(t *testing.T) {
	issue := &fakeIssueService{
		createResponse: &model.IssueResponseScheme{ID: "10001", Key: "ABC-123"},
	}
	client := &Client{
		baseURL: "https://example.atlassian.net",
		issue:   issue,
	}

	_, err := client.CreateIssue(context.Background(), CreateIssueRequest{
		ProjectKey:  "ABC",
		IssueTypeID: "10001",
		Summary:     "New platform task",
		Fields: []CreateIssueFieldValue{
			{FieldID: "priority", SchemaSystem: "priority", Option: FieldOption{ID: "3", Name: "Medium"}},
			{FieldID: "labels", SchemaSystem: "labels", Text: "platform, ecs"},
			{FieldID: "components", SchemaSystem: "components", Option: FieldOption{ID: "101", Name: "Platform"}},
			{FieldID: "customfield_10010", SchemaType: "string", Text: "Internal ALB"},
			{FieldID: "customfield_10011", SchemaType: "option", Option: FieldOption{ID: "20001", Name: "Team A"}},
		},
	})
	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}

	fields := issue.createPayload.Fields
	if fields.Priority == nil || fields.Priority.ID != "3" || fields.Priority.Name != "Medium" {
		t.Fatalf("Priority = %#v", fields.Priority)
	}
	if !reflect.DeepEqual(fields.Labels, []string{"platform", "ecs"}) {
		t.Fatalf("Labels = %#v", fields.Labels)
	}
	if len(fields.Components) != 1 || fields.Components[0].ID != "101" || fields.Components[0].Name != "Platform" {
		t.Fatalf("Components = %#v", fields.Components)
	}
	if issue.createCustomFields == nil {
		t.Fatal("expected custom fields")
	}
	if len(issue.createCustomFields.Fields) != 2 {
		t.Fatalf("custom fields = %#v", issue.createCustomFields.Fields)
	}
	custom := mergedCustomFieldPayloadForTest(t, issue.createPayload, issue.createCustomFields)
	if custom["customfield_10010"] != "Internal ALB" {
		t.Fatalf("customfield_10010 = %#v", custom["customfield_10010"])
	}
	if got := custom["customfield_10011"]; !reflect.DeepEqual(got, map[string]interface{}{"id": "20001", "value": "Team A"}) {
		t.Fatalf("customfield_10011 = %#v", got)
	}
}

func TestCreateIssueRejectsMissingRequiredFields(t *testing.T) {
	client := &Client{issue: &fakeIssueService{}}

	for _, request := range []CreateIssueRequest{
		{IssueTypeID: "10001", Summary: "Summary"},
		{ProjectKey: "ABC", Summary: "Summary"},
		{ProjectKey: "ABC", IssueTypeID: "10001"},
	} {
		if _, err := client.CreateIssue(context.Background(), request); err == nil {
			t.Fatalf("expected error for request %#v", request)
		}
	}
}

func TestCreateIssueWrapsCreateError(t *testing.T) {
	issue := &fakeIssueService{createErr: errors.New("create failed")}
	client := &Client{issue: issue}

	_, err := client.CreateIssue(context.Background(), CreateIssueRequest{
		ProjectKey:  "ABC",
		IssueTypeID: "10001",
		Summary:     "New issue",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, issue.createErr) {
		t.Fatalf("error = %v", err)
	}
}

func mergedCustomFieldPayloadForTest(t *testing.T, payload *model.IssueScheme, customFields *model.CustomFields) map[string]interface{} {
	t.Helper()
	merged, err := payload.MergeCustomFields(customFields)
	if err != nil {
		t.Fatalf("MergeCustomFields() error = %v", err)
	}
	fields, ok := merged["fields"].(map[string]interface{})
	if !ok {
		t.Fatalf("fields payload = %#v", merged["fields"])
	}
	return fields
}

func TestUpdateSummarySendsSummaryOnly(t *testing.T) {
	issue := &fakeIssueService{}
	client := &Client{
		issue: issue,
	}

	err := client.UpdateSummary(context.Background(), "ABC-123", "New summary")
	if err != nil {
		t.Fatalf("UpdateSummary() error = %v", err)
	}

	if issue.updateKey != "ABC-123" {
		t.Fatalf("updateKey = %q", issue.updateKey)
	}
	if issue.updateNotify {
		t.Fatal("notify should be false")
	}
	if issue.updatePayload == nil || issue.updatePayload.Fields == nil {
		t.Fatal("expected update payload fields")
	}
	if issue.updatePayload.Fields.Summary != "New summary" {
		t.Fatalf("Summary = %q", issue.updatePayload.Fields.Summary)
	}
	if issue.updateCustomFields != nil {
		t.Fatalf("custom fields = %#v", issue.updateCustomFields)
	}
	if issue.updateOperations != nil {
		t.Fatalf("operations = %#v", issue.updateOperations)
	}
}

func TestUpdateSummaryRejectsEmptySummary(t *testing.T) {
	client := &Client{issue: &fakeIssueService{}}

	err := client.UpdateSummary(context.Background(), "ABC-123", "  ")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateSummaryWrapsUpdateError(t *testing.T) {
	issue := &fakeIssueService{
		updateErr: errors.New("update failed"),
	}
	client := &Client{
		issue: issue,
	}

	err := client.UpdateSummary(context.Background(), "ABC-123", "New summary")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, issue.updateErr) {
		t.Fatalf("error = %v", err)
	}
}

func TestUpdateDescriptionSendsADFDescriptionOnly(t *testing.T) {
	issue := &fakeIssueService{}
	client := &Client{
		issue: issue,
	}

	err := client.UpdateDescription(context.Background(), "ABC-123", "Problem\n\nAcceptance Criteria\n- Works")
	if err != nil {
		t.Fatalf("UpdateDescription() error = %v", err)
	}

	if issue.updateKey != "ABC-123" {
		t.Fatalf("updateKey = %q", issue.updateKey)
	}
	if issue.updateNotify {
		t.Fatal("notify should be false")
	}
	if issue.updatePayload == nil || issue.updatePayload.Fields == nil || issue.updatePayload.Fields.Description == nil {
		t.Fatal("expected ADF description update payload")
	}
	if got := adf.Render(issue.updatePayload.Fields.Description); got != "Problem\n\nAcceptance Criteria\n- Works" {
		t.Fatalf("Description = %q", got)
	}
	if issue.updatePayload.Fields.Summary != "" {
		t.Fatalf("Summary = %q", issue.updatePayload.Fields.Summary)
	}
	if issue.updateCustomFields != nil {
		t.Fatalf("custom fields = %#v", issue.updateCustomFields)
	}
	if issue.updateOperations != nil {
		t.Fatalf("operations = %#v", issue.updateOperations)
	}
}

func TestUpdateDescriptionRejectsEmptyDescription(t *testing.T) {
	client := &Client{issue: &fakeIssueService{}}

	err := client.UpdateDescription(context.Background(), "ABC-123", "  ")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateDescriptionWrapsUpdateError(t *testing.T) {
	issue := &fakeIssueService{
		updateErr: errors.New("update failed"),
	}
	client := &Client{
		issue: issue,
	}

	err := client.UpdateDescription(context.Background(), "ABC-123", "New description")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, issue.updateErr) {
		t.Fatalf("error = %v", err)
	}
}

func TestUpdatePrioritySendsPriorityOnly(t *testing.T) {
	issue := &fakeIssueService{}
	client := &Client{
		issue: issue,
	}

	err := client.UpdatePriority(context.Background(), "ABC-123", FieldOption{ID: "3", Name: "Medium"})
	if err != nil {
		t.Fatalf("UpdatePriority() error = %v", err)
	}

	if issue.updateKey != "ABC-123" {
		t.Fatalf("updateKey = %q", issue.updateKey)
	}
	if issue.updatePayload == nil || issue.updatePayload.Fields == nil || issue.updatePayload.Fields.Priority == nil {
		t.Fatal("expected priority update payload")
	}
	if issue.updatePayload.Fields.Priority.ID != "3" {
		t.Fatalf("Priority.ID = %q", issue.updatePayload.Fields.Priority.ID)
	}
	if issue.updatePayload.Fields.Priority.Name != "Medium" {
		t.Fatalf("Priority.Name = %q", issue.updatePayload.Fields.Priority.Name)
	}
	if issue.updatePayload.Fields.Summary != "" {
		t.Fatalf("Summary = %q", issue.updatePayload.Fields.Summary)
	}
}

func TestUpdatePriorityRejectsEmptyPriority(t *testing.T) {
	client := &Client{issue: &fakeIssueService{}}

	err := client.UpdatePriority(context.Background(), "ABC-123", FieldOption{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdatePriorityWrapsUpdateError(t *testing.T) {
	issue := &fakeIssueService{
		updateErr: errors.New("update failed"),
	}
	client := &Client{
		issue: issue,
	}

	err := client.UpdatePriority(context.Background(), "ABC-123", FieldOption{ID: "3", Name: "Medium"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, issue.updateErr) {
		t.Fatalf("error = %v", err)
	}
}

func TestUpdateLabelsSendsLabelsOnly(t *testing.T) {
	issue := &fakeIssueService{}
	client := &Client{
		issue: issue,
	}

	err := client.UpdateLabels(context.Background(), "ABC-123", []string{"platform", "needs-review"})
	if err != nil {
		t.Fatalf("UpdateLabels() error = %v", err)
	}

	if issue.updateKey != "ABC-123" {
		t.Fatalf("updateKey = %q", issue.updateKey)
	}
	if issue.updatePayload == nil || issue.updatePayload.Fields == nil {
		t.Fatal("expected update payload fields")
	}
	if !reflect.DeepEqual(issue.updatePayload.Fields.Labels, []string{"platform", "needs-review"}) {
		t.Fatalf("Labels = %#v", issue.updatePayload.Fields.Labels)
	}
	if issue.updatePayload.Fields.Summary != "" {
		t.Fatalf("Summary = %q", issue.updatePayload.Fields.Summary)
	}
	if issue.updatePayload.Fields.Priority != nil {
		t.Fatalf("Priority = %#v", issue.updatePayload.Fields.Priority)
	}
}

func TestUpdateLabelsTrimsAndDropsEmptyLabels(t *testing.T) {
	issue := &fakeIssueService{}
	client := &Client{
		issue: issue,
	}

	err := client.UpdateLabels(context.Background(), "ABC-123", []string{" platform ", "", "needs-review", "platform"})
	if err != nil {
		t.Fatalf("UpdateLabels() error = %v", err)
	}

	want := []string{"platform", "needs-review"}
	if !reflect.DeepEqual(issue.updatePayload.Fields.Labels, want) {
		t.Fatalf("Labels = %#v, want %#v", issue.updatePayload.Fields.Labels, want)
	}
}

func TestUpdateLabelsAllowsClearingLabels(t *testing.T) {
	issue := &fakeIssueService{}
	client := &Client{
		issue: issue,
	}

	err := client.UpdateLabels(context.Background(), "ABC-123", nil)
	if err != nil {
		t.Fatalf("UpdateLabels() error = %v", err)
	}

	if issue.updatePayload == nil || issue.updatePayload.Fields == nil {
		t.Fatal("expected update payload fields")
	}
	if issue.updatePayload.Fields.Labels == nil {
		t.Fatal("expected empty labels slice so Jira clears labels")
	}
	if len(issue.updatePayload.Fields.Labels) != 0 {
		t.Fatalf("Labels = %#v", issue.updatePayload.Fields.Labels)
	}
}

func TestUpdateLabelsWrapsUpdateError(t *testing.T) {
	issue := &fakeIssueService{
		updateErr: errors.New("update failed"),
	}
	client := &Client{
		issue: issue,
	}

	err := client.UpdateLabels(context.Background(), "ABC-123", []string{"platform"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, issue.updateErr) {
		t.Fatalf("error = %v", err)
	}
}

func TestUpdateComponentsSendsComponentsOnly(t *testing.T) {
	issue := &fakeIssueService{}
	client := &Client{
		issue: issue,
	}

	err := client.UpdateComponents(context.Background(), "ABC-123", []FieldOption{{ID: "101", Name: "Platform"}, {ID: "102", Name: "API"}})
	if err != nil {
		t.Fatalf("UpdateComponents() error = %v", err)
	}

	if issue.updateKey != "ABC-123" {
		t.Fatalf("updateKey = %q", issue.updateKey)
	}
	if issue.updatePayload == nil || issue.updatePayload.Fields == nil {
		t.Fatal("expected update payload fields")
	}
	got := issue.updatePayload.Fields.Components
	if len(got) != 2 || got[0].ID != "101" || got[0].Name != "Platform" || got[1].ID != "102" || got[1].Name != "API" {
		t.Fatalf("Components = %#v", got)
	}
	if issue.updatePayload.Fields.Summary != "" {
		t.Fatalf("Summary = %q", issue.updatePayload.Fields.Summary)
	}
	if issue.updatePayload.Fields.Priority != nil {
		t.Fatalf("Priority = %#v", issue.updatePayload.Fields.Priority)
	}
}

func TestUpdateComponentsAllowsClearingComponents(t *testing.T) {
	issue := &fakeIssueService{}
	client := &Client{
		issue: issue,
	}

	err := client.UpdateComponents(context.Background(), "ABC-123", nil)
	if err != nil {
		t.Fatalf("UpdateComponents() error = %v", err)
	}

	if issue.updatePayload == nil || issue.updatePayload.Fields == nil {
		t.Fatal("expected update payload fields")
	}
	if issue.updatePayload.Fields.Components == nil {
		t.Fatal("expected empty components slice so Jira clears components")
	}
	if len(issue.updatePayload.Fields.Components) != 0 {
		t.Fatalf("Components = %#v", issue.updatePayload.Fields.Components)
	}
}

func TestUpdateComponentsWrapsUpdateError(t *testing.T) {
	issue := &fakeIssueService{
		updateErr: errors.New("update failed"),
	}
	client := &Client{
		issue: issue,
	}

	err := client.UpdateComponents(context.Background(), "ABC-123", []FieldOption{{ID: "101", Name: "Platform"}})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, issue.updateErr) {
		t.Fatalf("error = %v", err)
	}
}

func TestUpdateEditFieldSendsCustomNumberField(t *testing.T) {
	issue := &fakeIssueService{}
	client := &Client{
		issue: issue,
	}

	err := client.UpdateEditField(context.Background(), "ABC-123", EditFieldValue{
		FieldID:    "customfield_10016",
		SchemaType: "number",
		Text:       "8",
	})
	if err != nil {
		t.Fatalf("UpdateEditField() error = %v", err)
	}

	if issue.updateKey != "ABC-123" {
		t.Fatalf("updateKey = %q", issue.updateKey)
	}
	fields := mergedCustomFieldPayloadForTest(t, issue.updatePayload, issue.updateCustomFields)
	if got := fields["customfield_10016"]; got != float64(8) {
		t.Fatalf("customfield_10016 = %#v", got)
	}
	if issue.updateOperations != nil {
		t.Fatalf("operations = %#v", issue.updateOperations)
	}
}

func TestUpdateEditFieldSendsStandardDueDateField(t *testing.T) {
	issue := &fakeIssueService{}
	client := &Client{issue: issue}

	err := client.UpdateEditField(context.Background(), "ABC-123", EditFieldValue{
		FieldID:    "duedate",
		SchemaType: "date",
		Text:       "2026-06-19",
	})
	if err != nil {
		t.Fatalf("UpdateEditField() error = %v", err)
	}
	fields := mergedCustomFieldPayloadForTest(t, issue.updatePayload, issue.updateCustomFields)
	if got := fields["duedate"]; got != "2026-06-19" {
		t.Fatalf("duedate = %#v", got)
	}
}

func TestUpdateEditFieldSendsStandardVersionArrayField(t *testing.T) {
	issue := &fakeIssueService{}
	client := &Client{issue: issue}

	err := client.UpdateEditField(context.Background(), "ABC-123", EditFieldValue{
		FieldID:     "fixVersions",
		SchemaType:  "array",
		SchemaItems: "version",
		Options: []FieldOption{
			{ID: "10001", Name: "1.0.0"},
			{ID: "10002", Name: "1.1.0"},
		},
	})
	if err != nil {
		t.Fatalf("UpdateEditField() error = %v", err)
	}
	fields := mergedCustomFieldPayloadForTest(t, issue.updatePayload, issue.updateCustomFields)
	got, ok := fields["fixVersions"].([]map[string]interface{})
	if !ok {
		t.Fatalf("fixVersions type = %T", fields["fixVersions"])
	}
	want := []map[string]interface{}{{"id": "10001"}, {"id": "10002"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fixVersions = %#v, want %#v", got, want)
	}
}

func TestUpdateParentSendsParentKey(t *testing.T) {
	rest := &fakeRESTConnector{}
	client := &Client{rest: rest}

	err := client.UpdateParent(context.Background(), "ABC-123", UpdateParentRequest{ParentKey: "ABC-100"})
	if err != nil {
		t.Fatalf("UpdateParent() error = %v", err)
	}
	if rest.method != http.MethodPut {
		t.Fatalf("method = %q", rest.method)
	}
	if rest.endpoint != "rest/api/3/issue/ABC-123" {
		t.Fatalf("endpoint = %q", rest.endpoint)
	}
	payload, ok := rest.body.(map[string]interface{})
	if !ok {
		t.Fatalf("body type = %T", rest.body)
	}
	fields, ok := payload["fields"].(map[string]interface{})
	if !ok {
		t.Fatalf("fields = %#v", payload["fields"])
	}
	parent, ok := fields["parent"].(map[string]string)
	if !ok || parent["key"] != "ABC-100" {
		t.Fatalf("parent = %#v", fields["parent"])
	}
}

func TestUpdateParentClearsParent(t *testing.T) {
	rest := &fakeRESTConnector{}
	client := &Client{rest: rest}

	err := client.UpdateParent(context.Background(), "ABC-123", UpdateParentRequest{Clear: true})
	if err != nil {
		t.Fatalf("UpdateParent() error = %v", err)
	}
	payload, ok := rest.body.(map[string]interface{})
	if !ok {
		t.Fatalf("body type = %T", rest.body)
	}
	update, ok := payload["update"].(map[string]interface{})
	if !ok {
		t.Fatalf("update = %#v", payload["update"])
	}
	parent, ok := update["parent"].([]map[string]interface{})
	if !ok || len(parent) != 1 {
		t.Fatalf("parent update = %#v", update["parent"])
	}
	set, ok := parent[0]["set"].(map[string]bool)
	if !ok || !set["none"] {
		t.Fatalf("parent set = %#v", parent[0]["set"])
	}
}

func TestUpdateTimeTrackingSendsEstimateFields(t *testing.T) {
	rest := &fakeRESTConnector{}
	client := &Client{rest: rest}

	err := client.UpdateTimeTracking(context.Background(), "ABC-123", UpdateTimeTrackingRequest{
		OriginalEstimate:  "2d",
		RemainingEstimate: "3h",
	})
	if err != nil {
		t.Fatalf("UpdateTimeTracking() error = %v", err)
	}
	if rest.method != http.MethodPut {
		t.Fatalf("method = %q", rest.method)
	}
	if rest.endpoint != "rest/api/3/issue/ABC-123" {
		t.Fatalf("endpoint = %q", rest.endpoint)
	}
	payload, ok := rest.body.(map[string]interface{})
	if !ok {
		t.Fatalf("body type = %T", rest.body)
	}
	fields, ok := payload["fields"].(map[string]interface{})
	if !ok {
		t.Fatalf("fields = %#v", payload["fields"])
	}
	timetracking, ok := fields["timetracking"].(map[string]string)
	if !ok {
		t.Fatalf("timetracking = %#v", fields["timetracking"])
	}
	if timetracking["originalEstimate"] != "2d" || timetracking["remainingEstimate"] != "3h" {
		t.Fatalf("timetracking = %#v", timetracking)
	}
}

func TestUpdateAssigneeSendsAccountID(t *testing.T) {
	issue := &fakeIssueService{}
	client := &Client{
		issue: issue,
	}

	err := client.UpdateAssignee(context.Background(), "ABC-123", User{AccountID: "abc-123", DisplayName: "Jane Doe"})
	if err != nil {
		t.Fatalf("UpdateAssignee() error = %v", err)
	}

	if issue.assignKey != "ABC-123" {
		t.Fatalf("assignKey = %q", issue.assignKey)
	}
	if issue.assignAccountID != "abc-123" {
		t.Fatalf("assignAccountID = %q", issue.assignAccountID)
	}
}

func TestUpdateAssigneeRejectsMissingAccountID(t *testing.T) {
	client := &Client{issue: &fakeIssueService{}}

	err := client.UpdateAssignee(context.Background(), "ABC-123", User{DisplayName: "Jane Doe"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateAssigneeWrapsAssignError(t *testing.T) {
	issue := &fakeIssueService{
		assignErr: errors.New("assign failed"),
	}
	client := &Client{
		issue: issue,
	}

	err := client.UpdateAssignee(context.Background(), "ABC-123", User{AccountID: "abc-123", DisplayName: "Jane Doe"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, issue.assignErr) {
		t.Fatalf("error = %v", err)
	}
}

func TestCurrentUserParsesMyselfAccount(t *testing.T) {
	myself := &fakeMyselfService{
		response: &model.UserScheme{
			AccountID:    "account-123",
			DisplayName:  "Person Example",
			EmailAddress: "person@example.test",
			Active:       true,
		},
	}
	client := &Client{myself: myself}

	user, err := client.CurrentUser(context.Background())
	if err != nil {
		t.Fatalf("CurrentUser() error = %v", err)
	}

	if user.AccountID != "account-123" || user.DisplayName != "Person Example" || user.Email != "person@example.test" || !user.Active {
		t.Fatalf("user = %#v", user)
	}
	if !myself.called {
		t.Fatal("expected MySelf.Details to be called")
	}
}

func TestCurrentUserRejectsMissingAccountID(t *testing.T) {
	client := &Client{myself: &fakeMyselfService{response: &model.UserScheme{DisplayName: "Person Example"}}}

	err := func() error {
		_, err := client.CurrentUser(context.Background())
		return err
	}()
	if err == nil || !strings.Contains(err.Error(), "missing account ID") {
		t.Fatalf("error = %v", err)
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

func TestGetCommentADFReturnsRawCommentBody(t *testing.T) {
	body := &model.CommentNodeScheme{
		Type: "doc",
		Content: []*model.CommentNodeScheme{
			{
				Type: "paragraph",
				Content: []*model.CommentNodeScheme{
					{Type: "text", Text: "Raw comment."},
				},
			},
		},
	}
	comments := &fakeCommentService{
		response: &model.IssueCommentPageScheme{
			Comments: []*model.IssueCommentScheme{
				{ID: "10001", Body: &model.CommentNodeScheme{Type: "paragraph"}},
				{ID: "10002", Body: body},
			},
		},
	}
	client := &Client{comment: comments}

	got, err := client.GetCommentADF(context.Background(), "ABC-123", "10002")
	if err != nil {
		t.Fatalf("GetCommentADF() error = %v", err)
	}

	if comments.key != "ABC-123" {
		t.Fatalf("key = %q", comments.key)
	}
	if comments.maxResults != 100 {
		t.Fatalf("maxResults = %d", comments.maxResults)
	}
	if got != body {
		t.Fatalf("raw comment pointer changed: got %#v want %#v", got, body)
	}
}

func TestGetIssueLinkTypesUsesRESTEndpoint(t *testing.T) {
	rest := &fakeRESTConnector{
		issueLinkTypeResponse: model.IssueLinkTypeSearchScheme{
			IssueLinkTypes: []*model.LinkTypeScheme{
				{ID: "10000", Name: "Blocks", Inward: "is blocked by", Outward: "blocks"},
				{ID: "10001", Name: "Relates", Inward: "relates to", Outward: "relates to"},
			},
		},
	}
	client := &Client{rest: rest}

	types, err := client.GetIssueLinkTypes(context.Background())
	if err != nil {
		t.Fatalf("GetIssueLinkTypes() error = %v", err)
	}
	if rest.method != http.MethodGet {
		t.Fatalf("method = %q", rest.method)
	}
	if rest.endpoint != "rest/api/3/issueLinkType" {
		t.Fatalf("endpoint = %q", rest.endpoint)
	}
	if len(types) != 2 || types[0].Name != "Blocks" || types[0].Outward != "blocks" {
		t.Fatalf("types = %#v", types)
	}
}

func TestCreateIssueLinkSendsOutwardPayload(t *testing.T) {
	rest := &fakeRESTConnector{}
	client := &Client{rest: rest}

	err := client.CreateIssueLink(context.Background(), CreateIssueLinkRequest{
		SourceKey: "ABC-1",
		TargetKey: "ABC-2",
		Type:      IssueLinkType{ID: "10000", Name: "Blocks", Inward: "is blocked by", Outward: "blocks"},
		Direction: "outward",
	})
	if err != nil {
		t.Fatalf("CreateIssueLink() error = %v", err)
	}
	if rest.method != http.MethodPost {
		t.Fatalf("method = %q", rest.method)
	}
	if rest.endpoint != "rest/api/3/issueLink" {
		t.Fatalf("endpoint = %q", rest.endpoint)
	}
	payload, ok := rest.body.(*model.LinkPayloadSchemeV3)
	if !ok {
		t.Fatalf("payload = %T", rest.body)
	}
	if payload.Type.ID != "10000" || payload.Type.Name != "Blocks" {
		t.Fatalf("type = %#v", payload.Type)
	}
	if payload.InwardIssue.Key != "ABC-1" || payload.OutwardIssue.Key != "ABC-2" {
		t.Fatalf("issues = inward:%#v outward:%#v", payload.InwardIssue, payload.OutwardIssue)
	}
}

func TestCreateIssueLinkSendsInwardPayload(t *testing.T) {
	rest := &fakeRESTConnector{}
	client := &Client{rest: rest}

	err := client.CreateIssueLink(context.Background(), CreateIssueLinkRequest{
		SourceKey: "ABC-1",
		TargetKey: "ABC-2",
		Type:      IssueLinkType{Name: "Blocks", Inward: "is blocked by", Outward: "blocks"},
		Direction: "inward",
	})
	if err != nil {
		t.Fatalf("CreateIssueLink() error = %v", err)
	}
	payload, ok := rest.body.(*model.LinkPayloadSchemeV3)
	if !ok {
		t.Fatalf("payload = %T", rest.body)
	}
	if payload.InwardIssue.Key != "ABC-2" || payload.OutwardIssue.Key != "ABC-1" {
		t.Fatalf("issues = inward:%#v outward:%#v", payload.InwardIssue, payload.OutwardIssue)
	}
}

func TestGetWorklogsUsesRESTEndpoint(t *testing.T) {
	started := "2026-06-19T09:30:00.000+0000"
	rest := &fakeRESTConnector{
		worklogPageResponse: model.IssueWorklogADFPageScheme{
			Worklogs: []*model.IssueWorklogADFScheme{
				{
					ID:               "10001",
					Author:           &model.UserDetailScheme{DisplayName: "Jane Doe"},
					TimeSpent:        "1h",
					TimeSpentSeconds: 3600,
					Started:          started,
					Comment:          plainTextADF("Reviewed ABC-2", nil),
				},
			},
		},
	}
	client := &Client{rest: rest}

	worklogs, err := client.GetWorklogs(context.Background(), "ABC-1", 20)
	if err != nil {
		t.Fatalf("GetWorklogs() error = %v", err)
	}
	if rest.method != http.MethodGet {
		t.Fatalf("method = %q", rest.method)
	}
	if rest.endpoint != "rest/api/3/issue/ABC-1/worklog?maxResults=20&startAt=0" {
		t.Fatalf("endpoint = %q", rest.endpoint)
	}
	if len(worklogs) != 1 || worklogs[0].ID != "10001" || worklogs[0].Author != "Jane Doe" || worklogs[0].Comment != "Reviewed ABC-2" {
		t.Fatalf("worklogs = %#v", worklogs)
	}
}

func TestAddWorklogSendsADFPayload(t *testing.T) {
	started := time.Date(2026, 6, 19, 9, 30, 0, 0, time.UTC)
	rest := &fakeRESTConnector{
		worklogResponse: model.IssueWorklogADFScheme{
			ID:               "10001",
			Author:           &model.UserDetailScheme{DisplayName: "Jane Doe"},
			TimeSpent:        "45m",
			TimeSpentSeconds: 2700,
			Started:          "2026-06-19T09:30:00.000+0000",
			Comment:          plainTextADF("Reviewed ABC-2", nil),
		},
	}
	client := &Client{rest: rest}

	worklog, err := client.AddWorklog(context.Background(), "ABC-1", AddWorklogRequest{
		TimeSpent: "45m",
		Started:   started,
		Comment:   "Reviewed ABC-2",
	})
	if err != nil {
		t.Fatalf("AddWorklog() error = %v", err)
	}
	if rest.method != http.MethodPost {
		t.Fatalf("method = %q", rest.method)
	}
	if rest.endpoint != "rest/api/3/issue/ABC-1/worklog" {
		t.Fatalf("endpoint = %q", rest.endpoint)
	}
	payload, ok := rest.body.(*model.WorklogADFPayloadScheme)
	if !ok {
		t.Fatalf("payload = %T", rest.body)
	}
	if payload.TimeSpent != "45m" || payload.Started != "2026-06-19T09:30:00.000+0000" {
		t.Fatalf("payload time = %#v", payload)
	}
	if adf.Render(payload.Comment) != "Reviewed ABC-2" {
		t.Fatalf("comment = %#v", payload.Comment)
	}
	if worklog.ID != "10001" || worklog.TimeSpent != "45m" || worklog.Comment != "Reviewed ABC-2" {
		t.Fatalf("worklog = %#v", worklog)
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

func TestUpdateCommentBuildsADFPayload(t *testing.T) {
	comments := &fakeCommentService{
		updateResponse: &model.IssueCommentScheme{
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
							{Type: "text", Text: "Updated body"},
						},
					},
				},
			},
		},
	}
	client := &Client{comment: comments}

	comment, err := client.UpdateComment(context.Background(), "ABC-123", "10001", "Updated **body**", nil)
	if err != nil {
		t.Fatalf("UpdateComment() error = %v", err)
	}

	if comments.updateKey != "ABC-123" || comments.updateID != "10001" {
		t.Fatalf("update target = %s/%s", comments.updateKey, comments.updateID)
	}
	if comments.updatePayload == nil || comments.updatePayload.Body == nil {
		t.Fatal("expected update payload body")
	}
	nodes := comments.updatePayload.Body.Content[0].Content
	if len(nodes) != 2 || nodes[0].Text != "Updated " || nodes[1].Text != "body" {
		t.Fatalf("nodes = %#v", nodes)
	}
	if len(nodes[1].Marks) != 1 || nodes[1].Marks[0].Type != "strong" {
		t.Fatalf("bold marks = %#v", nodes[1].Marks)
	}
	if comment.ID != "10001" || comment.Body != "Updated body" {
		t.Fatalf("comment = %#v", comment)
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

func TestAddCommentBuildsADFFormattingMarks(t *testing.T) {
	comments := &fakeCommentService{
		addResponse: &model.IssueCommentScheme{ID: "10002"},
	}
	client := &Client{
		comment: comments,
	}

	_, err := client.AddComment(context.Background(), "ABC-123", "Ship **bold** _carefully_ with `main.go` and see https://example.test", nil)
	if err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}

	nodes := comments.payload.Body.Content[0].Content
	if len(nodes) != 8 {
		t.Fatalf("nodes = %#v", nodes)
	}
	if nodes[1].Text != "bold" || nodes[1].Marks[0].Type != "strong" {
		t.Fatalf("bold node = %#v", nodes[1])
	}
	if nodes[3].Text != "carefully" || nodes[3].Marks[0].Type != "em" {
		t.Fatalf("italic node = %#v", nodes[3])
	}
	if nodes[5].Text != "main.go" || nodes[5].Marks[0].Type != "code" {
		t.Fatalf("code node = %#v", nodes[5])
	}
	if nodes[7].Text != "https://example.test" || nodes[7].Marks[0].Type != "link" {
		t.Fatalf("link node = %#v", nodes[7])
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

func TestSearchAssignableUsersUsesIssueScopedEndpoint(t *testing.T) {
	rest := &fakeRESTConnector{
		assignableUsers: []model.UserScheme{
			{AccountID: "abc-123", DisplayName: "Jane Doe", EmailAddress: "jane@example.test", Active: true},
			{DisplayName: "No Account"},
		},
	}
	client := &Client{rest: rest}

	users, err := client.SearchAssignableUsers(context.Background(), "ABC-123", "Jane Doe", 5)
	if err != nil {
		t.Fatalf("SearchAssignableUsers() error = %v", err)
	}

	if rest.method != http.MethodGet {
		t.Fatalf("method = %q", rest.method)
	}
	wantEndpoint := "rest/api/3/user/assignable/search?issueKey=ABC-123&maxResults=5&query=Jane+Doe"
	if rest.endpoint != wantEndpoint {
		t.Fatalf("endpoint = %q, want %q", rest.endpoint, wantEndpoint)
	}
	want := []User{{AccountID: "abc-123", DisplayName: "Jane Doe", Email: "jane@example.test", Active: true}}
	if len(users) != len(want) || users[0] != want[0] {
		t.Fatalf("users = %#v, want %#v", users, want)
	}
}

func TestSearchAssignableUsersRejectsMissingIssueKey(t *testing.T) {
	client := &Client{rest: &fakeRESTConnector{}}

	_, err := client.SearchAssignableUsers(context.Background(), " ", "Jane", 5)

	if err == nil || !strings.Contains(err.Error(), "empty issue key") {
		t.Fatalf("SearchAssignableUsers() error = %v", err)
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

type fakeAgileBoardService struct {
	boards     *model.BoardPageScheme
	sprints    *model.BoardSprintPageScheme
	err        error
	projectKey string
	boardID    int
	states     []string
	startAt    int
	maxResults int
}

func (f *fakeAgileBoardService) Gets(_ context.Context, opts *model.GetBoardsOptions, startAt, maxResults int) (*model.BoardPageScheme, *model.ResponseScheme, error) {
	if opts != nil {
		f.projectKey = opts.ProjectKeyOrID
	}
	f.startAt = startAt
	f.maxResults = maxResults
	if f.err != nil {
		return nil, nil, f.err
	}
	return f.boards, nil, nil
}

func (f *fakeAgileBoardService) Sprints(_ context.Context, boardID, startAt, maxResults int, states []string) (*model.BoardSprintPageScheme, *model.ResponseScheme, error) {
	f.boardID = boardID
	f.startAt = startAt
	f.maxResults = maxResults
	f.states = append([]string(nil), states...)
	if f.err != nil {
		return nil, nil, f.err
	}
	return f.sprints, nil, nil
}

type fakeIssueService struct {
	key                string
	fields             []string
	expands            []string
	response           *model.IssueScheme
	transitions        *model.IssueTransitionsScheme
	transitionKey      string
	transitionErr      error
	moveKey            string
	transitionID       string
	moveOptions        *model.IssueMoveOptionsV3
	moveErr            error
	assignKey          string
	assignAccountID    string
	assignErr          error
	createPayload      *model.IssueScheme
	createCustomFields *model.CustomFields
	createResponse     *model.IssueResponseScheme
	createErr          error
	updateKey          string
	updateNotify       bool
	updatePayload      *model.IssueScheme
	updateCustomFields *model.CustomFields
	updateOperations   *model.UpdateOperations
	updateErr          error
	err                error
}

type fakeRESTConnector struct {
	method                string
	endpoint              string
	body                  interface{}
	issueResponse         issueDetailResponse
	transitionResponse    transitionFieldsResponse
	assignableUsers       []model.UserScheme
	fieldOptionResponse   fieldOptionSearchResponse
	issueLinkTypeResponse model.IssueLinkTypeSearchScheme
	worklogPageResponse   model.IssueWorklogADFPageScheme
	worklogResponse       model.IssueWorklogADFScheme
	err                   error
}

type fakeMyselfService struct {
	response *model.UserScheme
	err      error
	called   bool
}

func (f *fakeMyselfService) Details(_ context.Context, _ []string) (*model.UserScheme, *model.ResponseScheme, error) {
	f.called = true
	return f.response, nil, f.err
}

func (f *fakeRESTConnector) NewRequest(_ context.Context, method, endpoint, _ string, body interface{}) (*http.Request, error) {
	f.method = method
	f.endpoint = endpoint
	f.body = body
	return &http.Request{}, f.err
}

func (f *fakeRESTConnector) Call(_ *http.Request, v interface{}) (*model.ResponseScheme, error) {
	if f.err != nil {
		return nil, f.err
	}
	if v == nil {
		return &model.ResponseScheme{}, nil
	}
	switch target := v.(type) {
	case *issueDetailResponse:
		*target = f.issueResponse
	case *transitionFieldsResponse:
		*target = f.transitionResponse
	case *[]model.UserScheme:
		*target = append([]model.UserScheme(nil), f.assignableUsers...)
	case *fieldOptionSearchResponse:
		*target = f.fieldOptionResponse
	case *model.IssueLinkTypeSearchScheme:
		*target = f.issueLinkTypeResponse
	case *model.IssueWorklogADFPageScheme:
		*target = f.worklogPageResponse
	case *model.IssueWorklogADFScheme:
		*target = f.worklogResponse
	default:
		return nil, fmt.Errorf("unexpected REST target %T", v)
	}
	return &model.ResponseScheme{}, nil
}

func (f *fakeIssueService) Get(_ context.Context, key string, fields, expands []string) (*model.IssueScheme, *model.ResponseScheme, error) {
	f.key = key
	f.fields = fields
	f.expands = expands
	return f.response, nil, f.err
}

func (f *fakeIssueService) Transitions(_ context.Context, key string) (*model.IssueTransitionsScheme, *model.ResponseScheme, error) {
	f.transitionKey = key
	return f.transitions, nil, f.transitionErr
}

func (f *fakeIssueService) Move(_ context.Context, key string, transitionID string, options *model.IssueMoveOptionsV3) (*model.ResponseScheme, error) {
	f.moveKey = key
	f.transitionID = transitionID
	f.moveOptions = options
	return nil, f.moveErr
}

func (f *fakeIssueService) Assign(_ context.Context, key string, accountID string) (*model.ResponseScheme, error) {
	f.assignKey = key
	f.assignAccountID = accountID
	return nil, f.assignErr
}

func (f *fakeIssueService) Create(_ context.Context, payload *model.IssueScheme, customFields *model.CustomFields) (*model.IssueResponseScheme, *model.ResponseScheme, error) {
	f.createPayload = payload
	f.createCustomFields = customFields
	return f.createResponse, nil, f.createErr
}

func (f *fakeIssueService) Update(_ context.Context, key string, notify bool, payload *model.IssueScheme, customFields *model.CustomFields, operations *model.UpdateOperations) (*model.ResponseScheme, error) {
	f.updateKey = key
	f.updateNotify = notify
	f.updatePayload = payload
	f.updateCustomFields = customFields
	f.updateOperations = operations
	return nil, f.updateErr
}

type fakeMetadataService struct {
	key                    string
	projectKey             string
	fieldsProjectKey       string
	issueTypeID            string
	overrideScreenSecurity bool
	overrideEditableFlag   bool
	response               string
	issueTypesResponse     string
	createResponse         string
	createCalled           bool
	createOpts             model.IssueMetadataCreateOptions
	fieldsResponse         string
	err                    error
}

func (f *fakeMetadataService) Get(_ context.Context, key string, overrideScreenSecurity bool, overrideEditableFlag bool) (gjson.Result, *model.ResponseScheme, error) {
	f.key = key
	f.overrideScreenSecurity = overrideScreenSecurity
	f.overrideEditableFlag = overrideEditableFlag
	return gjson.Parse(f.response), nil, f.err
}

func (f *fakeMetadataService) FetchIssueMappings(_ context.Context, projectKey string, _, _ int) (gjson.Result, *model.ResponseScheme, error) {
	f.projectKey = projectKey
	return gjson.Parse(f.issueTypesResponse), nil, f.err
}

func (f *fakeMetadataService) Create(_ context.Context, opts *model.IssueMetadataCreateOptions) (gjson.Result, *model.ResponseScheme, error) {
	f.createCalled = true
	if opts != nil {
		f.createOpts = *opts
	}
	return gjson.Parse(f.createResponse), nil, f.err
}

func (f *fakeMetadataService) FetchFieldMappings(_ context.Context, projectKey string, issueTypeID string, _, _ int) (gjson.Result, *model.ResponseScheme, error) {
	f.fieldsProjectKey = projectKey
	f.issueTypeID = issueTypeID
	return gjson.Parse(f.fieldsResponse), nil, f.err
}

type fakeCommentService struct {
	key            string
	orderBy        string
	expands        []string
	startAt        int
	maxResults     int
	response       *model.IssueCommentPageScheme
	addKey         string
	payload        *model.CommentPayloadScheme
	addResponse    *model.IssueCommentScheme
	updateKey      string
	updateID       string
	updatePayload  *model.CommentPayloadScheme
	updateResponse *model.IssueCommentScheme
	err            error
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

func (f *fakeCommentService) Update(_ context.Context, key string, commentID string, payload *model.CommentPayloadScheme, expands []string) (*model.IssueCommentScheme, *model.ResponseScheme, error) {
	f.updateKey = key
	f.updateID = commentID
	f.updatePayload = payload
	f.expands = expands
	return f.updateResponse, nil, f.err
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
