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
	for _, want := range []string{"create-toil", "update-toil", "close-toil", "toil"} {
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
	issueTypes        []jira.CreateIssueType
	createdIssue      jira.Issue
	createRequest     jira.CreateIssueRequest
	searchIssues      []jira.Issue
	searchJQL         string
	addWorklogKey     string
	addWorklogRequest jira.AddWorklogRequest
	transitions       []jira.Transition
	transitionKey     string
	transitionID      string
	now               time.Time
	err               error
}

func (f *fakeToilJiraClient) SearchIssues(_ context.Context, jql string, _ int) ([]jira.Issue, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.searchJQL = jql
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

func (f *fakeToilJiraClient) currentTime() time.Time {
	if f.now.IsZero() {
		return time.Now()
	}
	return f.now
}

var _ toilJiraClient = (*fakeToilJiraClient)(nil)
var _ io.Reader = (*strings.Reader)(nil)
