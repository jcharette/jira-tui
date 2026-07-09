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

func TestRunCreateToilAddsConfiguredDefaultTeam(t *testing.T) {
	client := &fakeToilJiraClient{
		issueTypes: []jira.CreateIssueType{{ID: "10002", Name: "Toil"}},
	}
	cfg := config.Defaults()
	cfg.DefaultProject = "ABC"
	cfg.DefaultTeamFieldID = "customfield_12345"
	cfg.DefaultTeamID = "team-123"
	cfg.DefaultTeamName = "Team Alpha"
	var out bytes.Buffer

	err := runCreateToilWithDeps(context.Background(), cfg, client, createToilOptions{
		Summary: "Rotate certs",
		Time:    "45m",
	}, &out)

	if err != nil {
		t.Fatalf("runCreateToilWithDeps() error = %v", err)
	}
	want := []jira.CreateIssueFieldValue{
		{FieldID: "labels", SchemaSystem: "labels", Text: "toil"},
		{FieldID: "customfield_12345", SchemaType: "team", Option: jira.FieldOption{ID: "team-123", Name: "Team Alpha"}},
	}
	if !reflect.DeepEqual(client.createRequest.Fields, want) {
		t.Fatalf("create fields = %#v", client.createRequest.Fields)
	}
}

func TestRunCreateToilAddsCreatedTicketToConfiguredActiveSprint(t *testing.T) {
	client := &fakeToilJiraClient{
		issueTypes:   []jira.CreateIssueType{{ID: "10002", Name: "Toil"}},
		createdIssue: jira.Issue{Key: "ABC-123", Summary: "Rotate certs"},
		sprints:      []jira.Sprint{{ID: 300, BoardID: 1234, Name: "Sprint 42", State: "active"}},
		boardIssuesByJQL: map[string][]jira.Issue{
			"key = ABC-123": {{Key: "ABC-123"}},
		},
	}
	cfg := config.Defaults()
	cfg.DefaultProject = "ABC"
	cfg.DefaultBoardID = 1234
	var out bytes.Buffer

	err := runCreateToilWithDeps(context.Background(), cfg, client, createToilOptions{
		Summary: "Rotate certs",
		Time:    "45m",
	}, &out)

	if err != nil {
		t.Fatalf("runCreateToilWithDeps() error = %v", err)
	}
	if client.moveSprintID != 300 || !reflect.DeepEqual(client.moveIssueKeys, []string{"ABC-123"}) {
		t.Fatalf("move = sprint %d keys %#v", client.moveSprintID, client.moveIssueKeys)
	}
	if !strings.Contains(out.String(), "Added ABC-123 to sprint Sprint 42.") {
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
		sprints: []jira.Sprint{{ID: 300, BoardID: 1234, Name: "Sprint 42", State: "active"}},
	}
	cfg := config.Defaults()
	cfg.DefaultBoardID = 1234
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

func TestRunCheckBoardPromptsBeforeApplying(t *testing.T) {
	client := &fakeToilJiraClient{
		searchByJQL: map[string][]jira.Issue{
			"key = ABC-2":                     {{Key: "ABC-2", Summary: "Unassigned work", Status: "In Progress", IssueType: "Story", Assignee: "Unassigned"}},
			"parent = ABC-2 ORDER BY key ASC": nil,
			"key = ABC-2 AND sprint = 300":    nil,
		},
		sprints:     []jira.Sprint{{ID: 300, BoardID: 1234, Name: "Sprint 42", State: "active"}},
		currentUser: jira.User{AccountID: "account-123", DisplayName: "Jon"},
	}
	cfg := config.Defaults()
	cfg.DefaultBoardID = 1234
	var out bytes.Buffer

	err := runCheckBoardWithDeps(context.Background(), cfg, client, []string{"ABC-2"}, strings.NewReader("y\n"), &out, boardCheckOptions{})

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
		sprints: []jira.Sprint{{ID: 300, BoardID: 1234, Name: "Sprint 42", State: "active"}},
	}
	var out bytes.Buffer

	err := runCheckBoardWithDeps(context.Background(), config.Defaults(), client, []string{"ABC-2"}, strings.NewReader("y\n"), &out, boardCheckOptions{BoardID: 1234})

	if err != nil {
		t.Fatalf("runCheckBoardWithDeps() error = %v", err)
	}
	if client.boardID != 1234 || client.moveSprintID != 300 {
		t.Fatalf("board fix = board %d sprint %d", client.boardID, client.moveSprintID)
	}
	if !strings.Contains(out.String(), "Added ABC-2 to sprint Sprint 42.") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunCheckBoardFixDiscoversActiveSprintFromProjectBoard(t *testing.T) {
	client := &fakeToilJiraClient{
		searchByJQL: map[string][]jira.Issue{
			"key = ABC-2":                     {{Key: "ABC-2", Summary: "Work", Status: "In Progress", IssueType: "Story", Assignee: "Jon"}},
			"parent = ABC-2 ORDER BY key ASC": nil,
			"project = ABC AND sprint = 300":  {{Key: "ABC-9"}},
			"key = ABC-2 AND sprint = 300":    nil,
		},
		boards:           []jira.Board{{ID: 1234, Name: "ABC Scrum", Type: "scrum", ProjectKey: "ABC"}},
		sprintsByBoardID: map[int][]jira.Sprint{1234: {{ID: 300, BoardID: 1234, Name: "Sprint 42", State: "active"}}},
	}
	var out bytes.Buffer

	err := runCheckBoardWithDeps(context.Background(), config.Defaults(), client, []string{"ABC-2"}, strings.NewReader("y\n"), &out, boardCheckOptions{})

	if err != nil {
		t.Fatalf("runCheckBoardWithDeps() error = %v", err)
	}
	if client.boardProjectKey != "ABC" || client.boardID != 1234 || client.moveSprintID != 300 {
		t.Fatalf("discovery = project %q board %d sprint %d", client.boardProjectKey, client.boardID, client.moveSprintID)
	}
	if !strings.Contains(out.String(), "Add ABC-2 to sprint Sprint 42.") || !strings.Contains(out.String(), "Added ABC-2 to sprint Sprint 42.") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunCheckBoardFixDiscoversActiveSprintFromVisibleBoardWhenProjectBoardLookupMisses(t *testing.T) {
	client := &fakeToilJiraClient{
		searchByJQL: map[string][]jira.Issue{
			"key = DEVOPS-2":                     {{Key: "DEVOPS-2", Summary: "Work", Status: "In Progress", IssueType: "Story", Assignee: "Jon"}},
			"parent = DEVOPS-2 ORDER BY key ASC": nil,
			"project = DEVOPS AND sprint = 300":  {{Key: "DEVOPS-9"}},
			"key = DEVOPS-2 AND sprint = 300":    nil,
		},
		boardsByProject:   map[string][]jira.Board{"": {{ID: 1234, Name: "Platform Scrum", Type: "scrum"}}},
		sprintsByBoardID:  map[int][]jira.Sprint{1234: {{ID: 300, BoardID: 1234, Name: "Sprint 42", State: "active"}}},
		boardProjectCalls: []string{},
	}
	var out bytes.Buffer

	err := runCheckBoardWithDeps(context.Background(), config.Defaults(), client, []string{"DEVOPS-2"}, strings.NewReader("y\n"), &out, boardCheckOptions{})

	if err != nil {
		t.Fatalf("runCheckBoardWithDeps() error = %v", err)
	}
	if !reflect.DeepEqual(client.boardProjectCalls, []string{"DEVOPS", ""}) {
		t.Fatalf("board project calls = %#v", client.boardProjectCalls)
	}
	if client.moveSprintID != 300 {
		t.Fatalf("moveSprintID = %d", client.moveSprintID)
	}
	if !strings.Contains(out.String(), "Added DEVOPS-2 to sprint Sprint 42.") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunCheckBoardFixMovesMissingSprintIssuesInOneBatch(t *testing.T) {
	client := &fakeToilJiraClient{
		searchByJQL: map[string][]jira.Issue{
			"assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC": {
				{Key: "ABC-1", Summary: "First", Status: "In Progress", IssueType: "Story", Assignee: "Jon"},
				{Key: "ABC-2", Summary: "Second", Status: "In Progress", IssueType: "Story", Assignee: "Jon"},
			},
			"parent = ABC-1 ORDER BY key ASC": nil,
			"parent = ABC-2 ORDER BY key ASC": nil,
			"project = ABC AND sprint = 300":  {{Key: "ABC-9"}},
			"key = ABC-1 AND sprint = 300":    nil,
			"key = ABC-2 AND sprint = 300":    nil,
		},
		boards:           []jira.Board{{ID: 1234, Name: "ABC Scrum", Type: "scrum", ProjectKey: "ABC"}},
		sprintsByBoardID: map[int][]jira.Sprint{1234: {{ID: 300, BoardID: 1234, Name: "Sprint 42", State: "active"}}},
	}
	var out bytes.Buffer

	err := runCheckBoardWithDeps(context.Background(), config.Defaults(), client, nil, strings.NewReader("y\n"), &out, boardCheckOptions{})

	if err != nil {
		t.Fatalf("runCheckBoardWithDeps() error = %v", err)
	}
	if client.moveSprintID != 300 || !reflect.DeepEqual(client.moveIssueKeys, []string{"ABC-1", "ABC-2"}) || client.moveSprintCalls != 1 {
		t.Fatalf("move = sprint %d keys %#v calls %d", client.moveSprintID, client.moveIssueKeys, client.moveSprintCalls)
	}
	if !strings.Contains(out.String(), "Added 2 tickets to sprint Sprint 42.") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunCheckBoardReportsTicketsStillMissingFromBoardAfterSprintMove(t *testing.T) {
	client := &fakeToilJiraClient{
		searchByJQL: map[string][]jira.Issue{
			"assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC": {
				{Key: "ABC-1", Summary: "First", Status: "In Progress", IssueType: "Story", Assignee: "Jon"},
				{Key: "ABC-2", Summary: "Second", Status: "In Progress", IssueType: "Story", Assignee: "Jon"},
			},
			"parent = ABC-1 ORDER BY key ASC": nil,
			"parent = ABC-2 ORDER BY key ASC": nil,
			"project = ABC AND sprint = 300":  {{Key: "ABC-9"}},
			"key = ABC-1 AND sprint = 300":    nil,
			"key = ABC-2 AND sprint = 300":    nil,
		},
		boardIssuesByJQL: map[string][]jira.Issue{
			"key = ABC-1": nil,
			"key = ABC-2": nil,
		},
		boards:           []jira.Board{{ID: 1234, Name: "ABC Scrum", Type: "scrum", ProjectKey: "ABC"}},
		sprintsByBoardID: map[int][]jira.Sprint{1234: {{ID: 300, BoardID: 1234, Name: "Sprint 42", State: "active"}}},
	}
	var out bytes.Buffer

	err := runCheckBoardWithDeps(context.Background(), config.Defaults(), client, nil, strings.NewReader("y\n"), &out, boardCheckOptions{})

	if err != nil {
		t.Fatalf("runCheckBoardWithDeps() error = %v", err)
	}
	if !strings.Contains(out.String(), "Added 2 tickets to sprint Sprint 42.") {
		t.Fatalf("output = %q", out.String())
	}
	if !strings.Contains(out.String(), "Still not visible on board 1234 after sprint move: ABC-1, ABC-2.") {
		t.Fatalf("missing post-move board warning: %q", out.String())
	}
}

func TestRunCheckBoardSetsDefaultTeamForTicketsMissingFromBoard(t *testing.T) {
	client := &fakeToilJiraClient{
		searchByJQL: map[string][]jira.Issue{
			"key = ABC-2":                     {{Key: "ABC-2", Summary: "Work", Status: "In Progress", IssueType: "Story", Assignee: "Jon"}},
			"parent = ABC-2 ORDER BY key ASC": nil,
			"key = ABC-2 AND sprint = 300":    {{Key: "ABC-2"}},
		},
		sprints: []jira.Sprint{{ID: 300, BoardID: 1234, Name: "Sprint 42", State: "active"}},
		boardIssuesByJQL: map[string][]jira.Issue{
			"key = ABC-2": nil,
		},
	}
	cfg := config.Defaults()
	cfg.DefaultBoardID = 1234
	cfg.DefaultTeamFieldID = "customfield_12345"
	cfg.DefaultTeamID = "team-123"
	cfg.DefaultTeamName = "Team Alpha"
	var out bytes.Buffer

	err := runCheckBoardWithDeps(context.Background(), cfg, client, []string{"ABC-2"}, strings.NewReader("y\n"), &out, boardCheckOptions{})

	if err != nil {
		t.Fatalf("runCheckBoardWithDeps() error = %v", err)
	}
	if client.updateEditFieldKey != "ABC-2" {
		t.Fatalf("updateEditFieldKey = %q", client.updateEditFieldKey)
	}
	if client.updateEditFieldValue.FieldID != "customfield_12345" || client.updateEditFieldValue.SchemaType != "team" || client.updateEditFieldValue.Option.ID != "team-123" {
		t.Fatalf("updateEditFieldValue = %#v", client.updateEditFieldValue)
	}
	if !strings.Contains(out.String(), "Set ABC-2 Team to Team Alpha.") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunCheckBoardSkipsTeamWriteWhenTicketAlreadyHasTeam(t *testing.T) {
	client := &fakeToilJiraClient{
		searchByJQL: map[string][]jira.Issue{
			"key = ABC-2":                     {{Key: "ABC-2", Summary: "Work", Status: "In Progress", IssueType: "Sub-task", IsSubtask: true, ParentKey: "ABC-1", Assignee: "Jon"}},
			"parent = ABC-2 ORDER BY key ASC": nil,
			"key = ABC-1":                     {{Key: "ABC-1", Summary: "Parent", Status: "In Progress", IssueType: "Story", Assignee: "Jon"}},
			"key = ABC-2 AND sprint = 300":    {{Key: "ABC-2"}},
		},
		detailsByKey: map[string]jira.IssueDetail{
			"ABC-2": {Issue: jira.Issue{Key: "ABC-2", TeamID: "team-123", TeamName: "Team Alpha"}},
		},
		sprints: []jira.Sprint{{ID: 300, BoardID: 1234, Name: "Sprint 42", State: "active"}},
		boardIssuesByJQL: map[string][]jira.Issue{
			"key = ABC-2": nil,
		},
	}
	cfg := config.Defaults()
	cfg.DefaultBoardID = 1234
	cfg.DefaultTeamFieldID = "customfield_12345"
	cfg.DefaultTeamID = "team-123"
	cfg.DefaultTeamName = "Team Alpha"
	var out bytes.Buffer

	err := runCheckBoardWithDeps(context.Background(), cfg, client, []string{"ABC-2"}, strings.NewReader("y\n"), &out, boardCheckOptions{})

	if err != nil {
		t.Fatalf("runCheckBoardWithDeps() error = %v", err)
	}
	if client.updateEditFieldKey != "" {
		t.Fatalf("unexpected updateEditFieldKey = %q", client.updateEditFieldKey)
	}
	for _, want := range []string{"Skipped Team update for ABC-2: already Team Alpha.", "Still not visible on board 1234 after Team update: ABC-2."} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in output %q", want, out.String())
		}
	}
}

func TestRunCheckBoardReportsEpicBoardCardLimitationWithoutTeamFix(t *testing.T) {
	client := &fakeToilJiraClient{
		searchByJQL: map[string][]jira.Issue{
			"key = ABC-2":                     {{Key: "ABC-2", Summary: "Epic work", Status: "In Progress", IssueType: "Epic", HierarchyLevel: 1, Assignee: "Jon"}},
			"parent = ABC-2 ORDER BY key ASC": nil,
			"key = ABC-2 AND sprint = 300":    {{Key: "ABC-2"}},
		},
		sprints: []jira.Sprint{{ID: 300, BoardID: 1234, Name: "Sprint 42", State: "active"}},
		boardIssuesByJQL: map[string][]jira.Issue{
			"key = ABC-2": nil,
		},
	}
	cfg := config.Defaults()
	cfg.DefaultBoardID = 1234
	cfg.DefaultTeamFieldID = "customfield_12345"
	cfg.DefaultTeamID = "team-123"
	cfg.DefaultTeamName = "Team Alpha"
	var out bytes.Buffer

	err := runCheckBoardWithDeps(context.Background(), cfg, client, []string{"ABC-2"}, strings.NewReader(""), &out, boardCheckOptions{Yes: true})

	if err != nil {
		t.Fatalf("runCheckBoardWithDeps() error = %v", err)
	}
	if client.updateEditFieldKey != "" {
		t.Fatalf("unexpected updateEditFieldKey = %q", client.updateEditFieldKey)
	}
	if strings.Contains(out.String(), "Set ABC-2 Team") || strings.Contains(out.String(), "Proposed fixes") {
		t.Fatalf("unexpected automatic fix output = %q", out.String())
	}
	for _, want := range []string{"ABC-2 is an Epic", "board 1234 issue cards do not include Epics", "No automatic fixes can be applied"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in output %q", want, out.String())
		}
	}
}

func TestRunCheckBoardReportsSprintIssueMissingFromBoard(t *testing.T) {
	client := &fakeToilJiraClient{
		searchByJQL: map[string][]jira.Issue{
			"key = ABC-2":                     {{Key: "ABC-2", Summary: "Work", Status: "To Do", IssueType: "Story", Assignee: "Jon"}},
			"parent = ABC-2 ORDER BY key ASC": nil,
			"key = ABC-2 AND sprint = 300":    {{Key: "ABC-2"}},
		},
		boardIssuesByJQL: map[string][]jira.Issue{
			"key = ABC-2": nil,
		},
		sprints: []jira.Sprint{{ID: 300, BoardID: 1234, Name: "Sprint 42", State: "active"}},
	}
	cfg := config.Defaults()
	cfg.DefaultBoardID = 1234
	var out bytes.Buffer

	err := runCheckBoardWithDeps(context.Background(), cfg, client, []string{"ABC-2"}, strings.NewReader("y\n"), &out, boardCheckOptions{})

	if err != nil {
		t.Fatalf("runCheckBoardWithDeps() error = %v", err)
	}
	if !strings.Contains(out.String(), "ABC-2 is in sprint Sprint 42 but not visible on board 1234") {
		t.Fatalf("output = %q", out.String())
	}
	if strings.Contains(out.String(), "Apply these fixes?") || client.moveSprintID != 0 {
		t.Fatalf("unexpected automatic fix: output=%q sprint=%d", out.String(), client.moveSprintID)
	}
}

func TestRunCheckBoardSeparatesManualBoardFindingsFromAutomaticFixes(t *testing.T) {
	client := &fakeToilJiraClient{
		searchByJQL: map[string][]jira.Issue{
			"assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC": {
				{Key: "ABC-1", Summary: "On sprint", Status: "To Do", IssueType: "Story", Assignee: "Jon"},
				{Key: "ABC-2", Summary: "Missing sprint", Status: "To Do", IssueType: "Story", Assignee: "Jon"},
			},
			"parent = ABC-1 ORDER BY key ASC": nil,
			"parent = ABC-2 ORDER BY key ASC": nil,
			"key = ABC-1 AND sprint = 300":    {{Key: "ABC-1"}},
			"key = ABC-2 AND sprint = 300":    nil,
		},
		boardIssuesByJQL: map[string][]jira.Issue{
			"key = ABC-1": nil,
		},
		sprints: []jira.Sprint{{ID: 300, BoardID: 1234, Name: "Sprint 42", State: "active"}},
	}
	cfg := config.Defaults()
	cfg.DefaultBoardID = 1234
	var out bytes.Buffer

	err := runCheckBoardWithDeps(context.Background(), cfg, client, nil, strings.NewReader("y\n"), &out, boardCheckOptions{})

	if err != nil {
		t.Fatalf("runCheckBoardWithDeps() error = %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "Manual review:") || !strings.Contains(output, "- Review board 1234 filter/status mapping for ABC-1.") {
		t.Fatalf("missing manual review output = %q", output)
	}
	if strings.Contains(output, "Proposed fixes:\n- Review board 1234") {
		t.Fatalf("manual review included in fix plan = %q", output)
	}
	if !strings.Contains(output, "Proposed fixes:\n- Add ABC-2 to sprint Sprint 42.") {
		t.Fatalf("missing automatic fix output = %q", output)
	}
	if client.moveSprintID != 300 || !reflect.DeepEqual(client.moveIssueKeys, []string{"ABC-2"}) {
		t.Fatalf("move = sprint %d keys %#v", client.moveSprintID, client.moveIssueKeys)
	}
}

func TestRunCheckBoardUsesFreshReadContext(t *testing.T) {
	client := &fakeToilJiraClient{
		respectContext: true,
		searchByJQL: map[string][]jira.Issue{
			"key = ABC-2":                     {{Key: "ABC-2", Summary: "Work", Status: "In Progress", IssueType: "Story", Assignee: "Jon"}},
			"parent = ABC-2 ORDER BY key ASC": nil,
			"key = ABC-2 AND sprint = 300":    nil,
		},
		sprints: []jira.Sprint{{ID: 300, BoardID: 1234, Name: "Sprint 42", State: "active"}},
	}
	cfg := config.Defaults()
	cfg.DefaultBoardID = 1234
	ctx, cancel := context.WithCancel(context.Background())
	var out bytes.Buffer

	cancel()
	err := runCheckBoardWithDeps(ctx, cfg, client, []string{"ABC-2"}, strings.NewReader("n\n"), &out, boardCheckOptions{})

	if err != nil {
		t.Fatalf("runCheckBoardWithDeps() error = %v", err)
	}
	if !strings.Contains(out.String(), "No fixes applied.") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestRunCheckBoardFixUsesFreshContextAfterConfirmation(t *testing.T) {
	client := &fakeToilJiraClient{
		searchByJQL: map[string][]jira.Issue{
			"key = ABC-2":                     {{Key: "ABC-2", Summary: "Work", Status: "In Progress", IssueType: "Story", Assignee: "Jon"}},
			"parent = ABC-2 ORDER BY key ASC": nil,
			"key = ABC-2 AND sprint = 300":    nil,
		},
		sprints: []jira.Sprint{{ID: 300, BoardID: 1234, Name: "Sprint 42", State: "active"}},
	}
	cfg := config.Defaults()
	cfg.DefaultBoardID = 1234
	ctx, cancel := context.WithCancel(context.Background())
	var out bytes.Buffer

	cancel()
	err := runCheckBoardWithDeps(ctx, cfg, client, []string{"ABC-2"}, strings.NewReader("y\n"), &out, boardCheckOptions{})

	if err != nil {
		t.Fatalf("runCheckBoardWithDeps() error = %v", err)
	}
	if client.moveSprintID != 300 {
		t.Fatalf("moveSprintID = %d", client.moveSprintID)
	}
}

func TestRunCheckBoardFixWithoutBoardDoesNotPromptForUnfixableSprintFindings(t *testing.T) {
	client := &fakeToilJiraClient{
		searchByJQL: map[string][]jira.Issue{
			"key = ABC-2":                             {{Key: "ABC-2", Summary: "Work", Status: "In Progress", IssueType: "Story", Assignee: "Jon"}},
			"parent = ABC-2 ORDER BY key ASC":         nil,
			"key = ABC-2 AND sprint in openSprints()": nil,
		},
	}
	var out bytes.Buffer

	err := runCheckBoardWithDeps(context.Background(), config.Defaults(), client, []string{"ABC-2"}, strings.NewReader("y\n"), &out, boardCheckOptions{})

	if err != nil {
		t.Fatalf("runCheckBoardWithDeps() error = %v", err)
	}
	if strings.Contains(out.String(), "Apply these fixes?") || strings.Contains(out.String(), "Proposed fixes:") {
		t.Fatalf("unexpected prompt output = %q", out.String())
	}
	if !strings.Contains(out.String(), "No fixes can be applied") || !strings.Contains(out.String(), "--board") {
		t.Fatalf("output = %q", out.String())
	}
	if client.moveSprintID != 0 {
		t.Fatalf("unexpected sprint move = %d", client.moveSprintID)
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

	err := runCheckBoardWithDeps(context.Background(), config.Defaults(), client, []string{"ABC-2"}, strings.NewReader("n\n"), &out, boardCheckOptions{})

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

	err := runCheckBoardWithDeps(context.Background(), config.Defaults(), client, []string{"ABC-2"}, strings.NewReader(""), &out, boardCheckOptions{Yes: true})

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
	issueTypes           []jira.CreateIssueType
	createdIssue         jira.Issue
	createRequest        jira.CreateIssueRequest
	searchIssues         []jira.Issue
	searchByJQL          map[string][]jira.Issue
	searchJQL            string
	detailsByKey         map[string]jira.IssueDetail
	addWorklogKey        string
	addWorklogRequest    jira.AddWorklogRequest
	transitions          []jira.Transition
	transitionKey        string
	transitionID         string
	currentUser          jira.User
	updateAssigneeKey    string
	updateEditFieldKey   string
	updateEditFieldValue jira.EditFieldValue
	updateIssueTypeKey   string
	updateIssueTypeID    string
	boards               []jira.Board
	boardsByProject      map[string][]jira.Board
	boardProjectKey      string
	boardProjectCalls    []string
	sprints              []jira.Sprint
	sprintsByBoardID     map[int][]jira.Sprint
	boardIssuesByJQL     map[string][]jira.Issue
	boardIssueBoardID    int
	boardIssueJQL        string
	boardID              int
	moveSprintID         int
	moveSprintCalls      int
	moveIssueKeys        []string
	respectContext       bool
	now                  time.Time
	err                  error
}

func (f *fakeToilJiraClient) SearchIssues(ctx context.Context, jql string, _ int) ([]jira.Issue, error) {
	if f.respectContext {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	if f.err != nil {
		return nil, f.err
	}
	f.searchJQL = jql
	if f.searchByJQL != nil {
		return append([]jira.Issue(nil), f.searchByJQL[jql]...), nil
	}
	return append([]jira.Issue(nil), f.searchIssues...), nil
}

func (f *fakeToilJiraClient) GetIssue(_ context.Context, key string) (jira.IssueDetail, error) {
	if f.err != nil {
		return jira.IssueDetail{}, f.err
	}
	if f.detailsByKey != nil {
		if detail, ok := f.detailsByKey[key]; ok {
			return detail, nil
		}
	}
	return jira.IssueDetail{Issue: jira.Issue{Key: key}}, nil
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

func (f *fakeToilJiraClient) UpdateEditField(_ context.Context, key string, value jira.EditFieldValue) error {
	if f.err != nil {
		return f.err
	}
	f.updateEditFieldKey = key
	f.updateEditFieldValue = value
	return nil
}

func (f *fakeToilJiraClient) GetBoards(ctx context.Context, projectKey string, _, _ int) (jira.BoardPage, error) {
	if f.respectContext {
		if err := ctx.Err(); err != nil {
			return jira.BoardPage{}, err
		}
	}
	if f.err != nil {
		return jira.BoardPage{}, f.err
	}
	f.boardProjectKey = projectKey
	f.boardProjectCalls = append(f.boardProjectCalls, projectKey)
	if f.boardsByProject != nil {
		return jira.BoardPage{Boards: append([]jira.Board(nil), f.boardsByProject[projectKey]...)}, nil
	}
	return jira.BoardPage{Boards: append([]jira.Board(nil), f.boards...)}, nil
}

func (f *fakeToilJiraClient) GetBoardSprints(ctx context.Context, boardID int, _ []string, _, _ int) (jira.SprintPage, error) {
	if f.respectContext {
		if err := ctx.Err(); err != nil {
			return jira.SprintPage{}, err
		}
	}
	if f.err != nil {
		return jira.SprintPage{}, f.err
	}
	f.boardID = boardID
	if f.sprintsByBoardID != nil {
		return jira.SprintPage{BoardID: boardID, Sprints: append([]jira.Sprint(nil), f.sprintsByBoardID[boardID]...)}, nil
	}
	return jira.SprintPage{BoardID: boardID, Sprints: append([]jira.Sprint(nil), f.sprints...)}, nil
}

func (f *fakeToilJiraClient) SearchBoardIssues(ctx context.Context, boardID int, jql string, _ int) ([]jira.Issue, error) {
	if f.respectContext {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	if f.err != nil {
		return nil, f.err
	}
	f.boardIssueBoardID = boardID
	f.boardIssueJQL = jql
	if f.boardIssuesByJQL != nil {
		return append([]jira.Issue(nil), f.boardIssuesByJQL[jql]...), nil
	}
	key := strings.TrimSpace(strings.TrimPrefix(jql, "key = "))
	if key == "" {
		return nil, nil
	}
	return []jira.Issue{{Key: key}}, nil
}

func (f *fakeToilJiraClient) MoveIssuesToSprint(ctx context.Context, sprintID int, issueKeys []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if f.err != nil {
		return f.err
	}
	f.moveSprintID = sprintID
	f.moveSprintCalls++
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
