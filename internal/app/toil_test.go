package app

import (
	"bytes"
	"context"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jcharette/jira-tui/internal/config"
	"github.com/jcharette/jira-tui/internal/jira"
)

func TestNewRootCommandExposesTicketToilCommands(t *testing.T) {
	cmd := NewRootCommand()
	for _, args := range [][]string{
		{"ticket"},
		{"ticket", "toil"},
		{"ticket", "create-toil"},
		{"ticket", "update-toil"},
		{"ticket", "close-toil"},
		{"ticket", "check-board"},
	} {
		found, _, err := cmd.Find(args)
		if err != nil {
			t.Fatalf("Find(%v) error = %v", args, err)
		}
		if found == nil {
			t.Fatalf("Find(%v) returned nil", args)
		}
	}
}

func TestTicketCommandHelpMentionsToilCommands(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"ticket", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	for _, want := range []string{"create-toil", "update-toil", "close-toil", "toil", "check-board"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in help %q", want, out.String())
		}
	}
}

func TestToilCommandsRejectUnexpectedArgs(t *testing.T) {
	for _, args := range [][]string{
		{"ticket", "toil", "ABC-123"},
		{"ticket", "create-toil", "ABC-123"},
	} {
		cmd := NewRootCommand()
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatalf("Execute(%v) expected error", args)
		}
	}
}

func TestRunCreateToilCreatesLabeledTicketAndLogsWork(t *testing.T) {
	client := &fakeToilJiraClient{
		issueTypes: []jira.CreateIssueType{
			{ID: "10001", Name: "Task"},
			{ID: "10002", Name: "Toil"},
		},
		createdIssue: jira.Issue{Key: "ABC-123", Summary: "Rotate certs"},
		now:          time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC),
	}
	cfg := config.Defaults()
	cfg.DefaultProject = "ABC"
	var out bytes.Buffer

	err := runCreateToilWithDeps(context.Background(), cfg, client, createToilOptions{
		Summary: "Rotate certs",
		Time:    "45m",
		Note:    "prod cert cleanup",
	}, &out)

	if err != nil {
		t.Fatalf("runCreateToilWithDeps() error = %v", err)
	}
	if client.createRequest.ProjectKey != "ABC" || client.createRequest.IssueTypeID != "10002" || client.createRequest.Summary != "Rotate certs" {
		t.Fatalf("create request = %#v", client.createRequest)
	}
	if !reflect.DeepEqual(client.createRequest.Fields, []jira.CreateIssueFieldValue{{FieldID: "labels", SchemaSystem: "labels", Text: "toil"}}) {
		t.Fatalf("create fields = %#v", client.createRequest.Fields)
	}
	if client.addWorklogKey != "ABC-123" || client.addWorklogRequest.TimeSpent != "45m" || client.addWorklogRequest.Comment != "prod cert cleanup" {
		t.Fatalf("worklog = %s %#v", client.addWorklogKey, client.addWorklogRequest)
	}
	if !strings.Contains(out.String(), "Created ABC-123.") || !strings.Contains(out.String(), "Logged 45m to ABC-123.") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunUpdateToilUsesPickerWhenKeyOmitted(t *testing.T) {
	client := &fakeToilJiraClient{
		searchIssues: []jira.Issue{
			{Key: "ABC-1", Summary: "First toil"},
			{Key: "ABC-2", Summary: "Second toil"},
		},
	}
	cfg := config.Defaults()
	var out bytes.Buffer

	err := runUpdateToilWithDeps(context.Background(), cfg, client, nil, strings.NewReader("2\n"), &out, toilWorklogOptions{
		Time: "30m",
		Note: "follow-up validation",
	})

	if err != nil {
		t.Fatalf("runUpdateToilWithDeps() error = %v", err)
	}
	if !strings.Contains(client.searchJQL, "labels = toil OR issuetype = Toil") {
		t.Fatalf("search JQL = %q", client.searchJQL)
	}
	if client.addWorklogKey != "ABC-2" || client.addWorklogRequest.TimeSpent != "30m" || client.addWorklogRequest.Comment != "follow-up validation" {
		t.Fatalf("worklog = %s %#v", client.addWorklogKey, client.addWorklogRequest)
	}
	if !strings.Contains(out.String(), "1. ABC-1") || !strings.Contains(out.String(), "2. ABC-2") || !strings.Contains(out.String(), "Logged 30m to ABC-2.") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunCloseToilLogsWorkThenAppliesSafeTerminalTransition(t *testing.T) {
	client := &fakeToilJiraClient{
		transitions: []jira.Transition{
			{ID: "11", Name: "Resolve", ToStatus: "Done", IsAvailable: true, Fields: []jira.TransitionField{{ID: "resolution", Required: true}}},
			{ID: "12", Name: "Close", ToStatus: "Closed", IsAvailable: true},
		},
	}
	var out bytes.Buffer

	err := runCloseToilWithDeps(context.Background(), config.Defaults(), client, []string{"ABC-123"}, nil, &out, toilWorklogOptions{
		Time: "15m",
		Note: "done",
	})

	if err != nil {
		t.Fatalf("runCloseToilWithDeps() error = %v", err)
	}
	if client.addWorklogKey != "ABC-123" || client.addWorklogRequest.TimeSpent != "15m" {
		t.Fatalf("worklog = %s %#v", client.addWorklogKey, client.addWorklogRequest)
	}
	if client.transitionKey != "ABC-123" || client.transitionID != "12" {
		t.Fatalf("transition = %s/%s", client.transitionKey, client.transitionID)
	}
	if !strings.Contains(out.String(), "Closed ABC-123 as Closed.") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunCheckBoardAuditsCurrentUserWork(t *testing.T) {
	client := &fakeToilJiraClient{
		searchByJQL: map[string][]jira.Issue{
			"assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC": {
				{Key: "ABC-1", Summary: "Parent epic", Status: "In Progress", IssueType: "Epic", Assignee: "Jon"},
			},
			"parent = ABC-1 ORDER BY key ASC": {
				{Key: "ABC-2", Summary: "Bad subtask", Status: "In Progress", IssueType: "Sub-task", IsSubtask: true, ParentKey: "ABC-1", Assignee: "Unassigned"},
			},
			"key = ABC-1 AND sprint = 300": {{Key: "ABC-1"}},
			"key = ABC-2 AND sprint = 300": nil,
		},
		sprints: []jira.Sprint{{ID: 300, BoardID: 1255, Name: "Sprint 42", State: "active"}},
	}
	cfg := config.Defaults()
	cfg.DefaultBoardID = 1255
	var out bytes.Buffer

	err := runCheckBoardWithDeps(context.Background(), cfg, client, nil, strings.NewReader(""), &out, boardCheckOptions{})

	if err != nil {
		t.Fatalf("runCheckBoardWithDeps() error = %v", err)
	}
	for _, want := range []string{"ERROR ABC-2", "Sub-task directly under Epic ABC-1", "WARN ABC-2", "not in the active sprint"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in output %q", want, out.String())
		}
	}
}

func TestRunCheckBoardAuditsMissingSprintWithoutConfiguredBoard(t *testing.T) {
	client := &fakeToilJiraClient{
		searchByJQL: map[string][]jira.Issue{
			"assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC": {
				{Key: "ABC-1", Summary: "In progress work", Status: "In Progress", IssueType: "Story", Assignee: "Jon"},
			},
			"parent = ABC-1 ORDER BY key ASC":         nil,
			"key = ABC-1 AND sprint in openSprints()": nil,
		},
	}
	var out bytes.Buffer

	err := runCheckBoardWithDeps(context.Background(), config.Defaults(), client, nil, strings.NewReader(""), &out, boardCheckOptions{})

	if err != nil {
		t.Fatalf("runCheckBoardWithDeps() error = %v", err)
	}
	if !strings.Contains(out.String(), "WARN ABC-1") || !strings.Contains(out.String(), "not in the active sprint") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunCheckBoardFixPromptsBeforeApplying(t *testing.T) {
	client := &fakeToilJiraClient{
		searchByJQL: map[string][]jira.Issue{
			"key = ABC-2":                     {{Key: "ABC-2", Summary: "Unassigned work", Status: "In Progress", IssueType: "Story", Assignee: "Unassigned"}},
			"parent = ABC-2 ORDER BY key ASC": nil,
			"key = ABC-2 AND sprint = 300":    nil,
		},
		sprints:     []jira.Sprint{{ID: 300, BoardID: 1255, Name: "Sprint 42", State: "active"}},
		currentUser: jira.User{AccountID: "account-123", DisplayName: "Jon"},
	}
	cfg := config.Defaults()
	cfg.DefaultBoardID = 1255
	var out bytes.Buffer

	err := runCheckBoardWithDeps(context.Background(), cfg, client, []string{"ABC-2"}, strings.NewReader("y\n"), &out, boardCheckOptions{Fix: true})

	if err != nil {
		t.Fatalf("runCheckBoardWithDeps() error = %v", err)
	}
	if client.updateAssigneeKey != "ABC-2" || client.moveSprintID != 300 {
		t.Fatalf("fixes not applied: assignee=%s sprint=%d", client.updateAssigneeKey, client.moveSprintID)
	}
	if !strings.Contains(out.String(), "Apply these fixes? [y/N]") || !strings.Contains(out.String(), "Assigned ABC-2") || !strings.Contains(out.String(), "Added ABC-2") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunCheckBoardFixUsesBoardOption(t *testing.T) {
	client := &fakeToilJiraClient{
		searchByJQL: map[string][]jira.Issue{
			"key = ABC-2":                     {{Key: "ABC-2", Summary: "Work", Status: "In Progress", IssueType: "Story", Assignee: "Jon"}},
			"parent = ABC-2 ORDER BY key ASC": nil,
			"key = ABC-2 AND sprint = 300":    nil,
		},
		sprints: []jira.Sprint{{ID: 300, BoardID: 1255, Name: "Sprint 42", State: "active"}},
	}
	var out bytes.Buffer

	err := runCheckBoardWithDeps(context.Background(), config.Defaults(), client, []string{"ABC-2"}, strings.NewReader("y\n"), &out, boardCheckOptions{Fix: true, BoardID: 1255})

	if err != nil {
		t.Fatalf("runCheckBoardWithDeps() error = %v", err)
	}
	if client.boardID != 1255 || client.moveSprintID != 300 {
		t.Fatalf("board fix = board %d sprint %d", client.boardID, client.moveSprintID)
	}
	if !strings.Contains(out.String(), "Added ABC-2 to sprint Sprint 42.") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunCheckBoardFixNoKeepsReadOnly(t *testing.T) {
	client := &fakeToilJiraClient{
		searchByJQL: map[string][]jira.Issue{
			"key = ABC-2":                     {{Key: "ABC-2", Status: "In Progress", IssueType: "Story", Assignee: "Unassigned"}},
			"parent = ABC-2 ORDER BY key ASC": nil,
		},
	}
	var out bytes.Buffer

	err := runCheckBoardWithDeps(context.Background(), config.Defaults(), client, []string{"ABC-2"}, strings.NewReader("n\n"), &out, boardCheckOptions{Fix: true})

	if err != nil {
		t.Fatalf("runCheckBoardWithDeps() error = %v", err)
	}
	if client.updateAssigneeKey != "" {
		t.Fatalf("unexpected assignee update: %s", client.updateAssigneeKey)
	}
	if !strings.Contains(out.String(), "No fixes applied.") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunCheckBoardFixConvertsSubtaskWhenStoryTypeExists(t *testing.T) {
	client := &fakeToilJiraClient{
		searchByJQL: map[string][]jira.Issue{
			"key = ABC-2":                     {{Key: "ABC-2", Status: "In Progress", IssueType: "Sub-task", IsSubtask: true, ParentKey: "ABC-1", Assignee: "Jon"}},
			"parent = ABC-2 ORDER BY key ASC": nil,
			"key = ABC-1":                     {{Key: "ABC-1", IssueType: "Epic"}},
		},
		issueTypes: []jira.CreateIssueType{
			{ID: "10001", Name: "Sub-task", Subtask: true},
			{ID: "10002", Name: "Story", Subtask: false},
		},
	}
	var out bytes.Buffer

	err := runCheckBoardWithDeps(context.Background(), config.Defaults(), client, []string{"ABC-2"}, strings.NewReader(""), &out, boardCheckOptions{Fix: true, Yes: true})

	if err != nil {
		t.Fatalf("runCheckBoardWithDeps() error = %v", err)
	}
	if client.updateIssueTypeKey != "ABC-2" || client.updateIssueTypeID != "10002" {
		t.Fatalf("issue type update = %s %s", client.updateIssueTypeKey, client.updateIssueTypeID)
	}
	if !strings.Contains(out.String(), "Converted ABC-2") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunCloseToilReportsWhenNoSafeTransitionExists(t *testing.T) {
	client := &fakeToilJiraClient{
		transitions: []jira.Transition{
			{ID: "11", Name: "Resolve", ToStatus: "Done", IsAvailable: true, Fields: []jira.TransitionField{{ID: "resolution", Required: true}}},
		},
	}
	var out bytes.Buffer

	err := runCloseToilWithDeps(context.Background(), config.Defaults(), client, []string{"ABC-123"}, nil, &out, toilWorklogOptions{})

	if err != nil {
		t.Fatalf("runCloseToilWithDeps() error = %v", err)
	}
	if client.transitionID != "" {
		t.Fatalf("unexpected transition = %q", client.transitionID)
	}
	if !strings.Contains(out.String(), "Skipped close for ABC-123: no safe terminal transition available.") {
		t.Fatalf("output = %q", out.String())
	}
}

type fakeToilJiraClient struct {
	issueTypes         []jira.CreateIssueType
	createdIssue       jira.Issue
	createRequest      jira.CreateIssueRequest
	searchIssues       []jira.Issue
	searchByJQL        map[string][]jira.Issue
	searchJQL          string
	addWorklogKey      string
	addWorklogRequest  jira.AddWorklogRequest
	transitions        []jira.Transition
	transitionKey      string
	transitionID       string
	currentUser        jira.User
	updateAssigneeKey  string
	updateIssueTypeKey string
	updateIssueTypeID  string
	sprints            []jira.Sprint
	boardID            int
	moveSprintID       int
	moveIssueKeys      []string
	now                time.Time
	err                error
}

func (f *fakeToilJiraClient) SearchIssues(_ context.Context, jql string, _ int) ([]jira.Issue, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.searchJQL = jql
	if f.searchByJQL != nil {
		return append([]jira.Issue(nil), f.searchByJQL[jql]...), nil
	}
	return append([]jira.Issue(nil), f.searchIssues...), nil
}

func (f *fakeToilJiraClient) GetCreateIssueTypes(_ context.Context, _ string) ([]jira.CreateIssueType, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]jira.CreateIssueType(nil), f.issueTypes...), nil
}

func (f *fakeToilJiraClient) CreateIssue(_ context.Context, request jira.CreateIssueRequest) (jira.Issue, error) {
	if f.err != nil {
		return jira.Issue{}, f.err
	}
	f.createRequest = request
	if strings.TrimSpace(f.createdIssue.Key) == "" {
		return jira.Issue{Key: "ABC-123", Summary: request.Summary}, nil
	}
	return f.createdIssue, nil
}

func (f *fakeToilJiraClient) AddWorklog(_ context.Context, key string, request jira.AddWorklogRequest) (jira.Worklog, error) {
	if f.err != nil {
		return jira.Worklog{}, f.err
	}
	f.addWorklogKey = key
	f.addWorklogRequest = request
	return jira.Worklog{ID: "10001", TimeSpent: request.TimeSpent, Comment: request.Comment}, nil
}

func (f *fakeToilJiraClient) GetTransitions(_ context.Context, _ string) ([]jira.Transition, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]jira.Transition(nil), f.transitions...), nil
}

func (f *fakeToilJiraClient) TransitionIssue(_ context.Context, key string, request jira.TransitionIssueRequest) error {
	if f.err != nil {
		return f.err
	}
	f.transitionKey = key
	f.transitionID = request.TransitionID
	return nil
}

func (f *fakeToilJiraClient) CurrentUser(context.Context) (jira.User, error) {
	if f.err != nil {
		return jira.User{}, f.err
	}
	if f.currentUser.AccountID == "" {
		return jira.User{AccountID: "account-123", DisplayName: "Jon"}, nil
	}
	return f.currentUser, nil
}

func (f *fakeToilJiraClient) UpdateAssignee(_ context.Context, key string, _ jira.User) error {
	if f.err != nil {
		return f.err
	}
	f.updateAssigneeKey = key
	return nil
}

func (f *fakeToilJiraClient) GetBoardSprints(_ context.Context, boardID int, _ []string, _, _ int) (jira.SprintPage, error) {
	if f.err != nil {
		return jira.SprintPage{}, f.err
	}
	f.boardID = boardID
	return jira.SprintPage{BoardID: boardID, Sprints: append([]jira.Sprint(nil), f.sprints...)}, nil
}

func (f *fakeToilJiraClient) MoveIssuesToSprint(_ context.Context, sprintID int, issueKeys []string) error {
	if f.err != nil {
		return f.err
	}
	f.moveSprintID = sprintID
	f.moveIssueKeys = append([]string(nil), issueKeys...)
	return nil
}

func (f *fakeToilJiraClient) UpdateIssueType(_ context.Context, key string, issueTypeID string) error {
	if f.err != nil {
		return f.err
	}
	f.updateIssueTypeKey = key
	f.updateIssueTypeID = issueTypeID
	return nil
}

func (f *fakeToilJiraClient) currentTime() time.Time {
	if f.now.IsZero() {
		return time.Now()
	}
	return f.now
}

var _ toilJiraClient = (*fakeToilJiraClient)(nil)
var _ boardCheckClient = (*fakeToilJiraClient)(nil)
var _ io.Reader = (*strings.Reader)(nil)
