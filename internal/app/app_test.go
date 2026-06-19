package app

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jcharette/jira-tui/internal/config"
	"github.com/jcharette/jira-tui/internal/gitworkflow"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/jcharette/jira-tui/internal/startworkflow"
)

func TestNewRootCommandUsesJiraCommandName(t *testing.T) {
	cmd := NewRootCommand()
	if cmd.Use != "jira" {
		t.Fatalf("Use = %q", cmd.Use)
	}
	if cmd.CommandPath() != "jira" {
		t.Fatalf("CommandPath() = %q", cmd.CommandPath())
	}
}

func TestNewRootCommandExposesProfileFlag(t *testing.T) {
	cmd := NewRootCommand()
	flag := cmd.PersistentFlags().Lookup("profile")
	if flag == nil {
		t.Fatal("expected --profile persistent flag")
	}
	if flag.DefValue != "" {
		t.Fatalf("profile default = %q", flag.DefValue)
	}
	configCmd, _, err := cmd.Find([]string{"config"})
	if err != nil {
		t.Fatalf("Find(config) error = %v", err)
	}
	if configCmd.InheritedFlags().Lookup("profile") == nil && configCmd.Flags().Lookup("profile") == nil {
		t.Fatal("expected config command to inherit --profile")
	}
	startCmd, _, err := cmd.Find([]string{"start"})
	if err != nil {
		t.Fatalf("Find(start) error = %v", err)
	}
	if startCmd.InheritedFlags().Lookup("profile") == nil && startCmd.Flags().Lookup("profile") == nil {
		t.Fatal("expected start command to inherit --profile")
	}
}

func TestSavedViewWriterPersistsViewToConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := config.Defaults()
	cfg.BaseURL = "https://example.atlassian.net"
	cfg.Email = "person@example.com"
	cfg.APIToken = "secret"
	cfg.DefaultProject = "ABC"
	cfg.DefaultJQL = config.DefaultJQLForProject("ABC")
	cfg.Views = []config.IssueView{{Name: "Assigned", JQL: "assignee = currentUser()"}}
	cfg.ActiveView = "Assigned"

	writer := savedViewWriter(path, &cfg)
	if err := writer(config.IssueView{Name: "Active Work", JQL: "project = ABC AND status = \"In Progress\""}); err != nil {
		t.Fatalf("writer() error = %v", err)
	}

	if len(cfg.Views) != 2 || cfg.Views[1].Name != "Active Work" {
		t.Fatalf("captured cfg views = %#v", cfg.Views)
	}
	if cfg.ActiveView != "Assigned" {
		t.Fatalf("ActiveView = %q", cfg.ActiveView)
	}
	loaded, err := config.Load(config.LoadOptions{Path: path})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded.Views) != 2 || loaded.Views[1].Name != "Active Work" || loaded.Views[1].JQL != "project = ABC AND status = \"In Progress\"" {
		t.Fatalf("loaded views = %#v", loaded.Views)
	}
}

func TestApplyStartBranchUsesGitAdapterBoundary(t *testing.T) {
	gitClient := &fakeGitClient{}
	result := startworkflow.Result{
		Confirmed:  true,
		Issue:      jira.Issue{Key: "PROJ-123"},
		RepoPath:   "/tmp/repo",
		BranchName: "proj-123-start-work",
		Actions: []startworkflow.ActionPlan{
			{Kind: startworkflow.ActionBranch, Required: true},
			{Kind: startworkflow.ActionAssign, Skip: true},
		},
	}

	outcomes := applyStartActions(context.Background(), gitClient, &jira.Client{}, result)

	if gitClient.branchRepo != "/tmp/repo" || gitClient.branchName != "proj-123-start-work" {
		t.Fatalf("branch call = %q %q", gitClient.branchRepo, gitClient.branchName)
	}
	if len(outcomes) != 2 || outcomes[0].State != "completed" || outcomes[1].State != "skipped" {
		t.Fatalf("outcomes = %#v", outcomes)
	}
}

func TestWriteStartSummaryShowsSkippedOptionalActions(t *testing.T) {
	var out bytes.Buffer
	writeStartSummary(&out, startworkflow.Result{
		Issue:      jira.Issue{Key: "PROJ-123"},
		RepoPath:   "/tmp/repo",
		BranchName: "proj-123-start-work",
	}, []startActionOutcome{
		{Kind: startworkflow.ActionBranch, Label: "Create or switch branch", State: "completed"},
		{Kind: startworkflow.ActionAssign, Label: "Assign to me", State: "skipped"},
		{Kind: startworkflow.ActionTransition, Label: "Move to In Progress", State: "completed"},
	})

	got := out.String()
	for _, want := range []string{"Started PROJ-123.", "Repo: /tmp/repo", "Branch: proj-123-start-work", "Create or switch branch: completed", "Assign to me: skipped", "Move to In Progress: completed"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
}

func TestChooseStartTransitionRanksInProgressWithoutRequiredFields(t *testing.T) {
	transition, ok := chooseStartTransition([]jira.Transition{
		{ID: "11", Name: "Start Review", ToStatus: "In Review", IsAvailable: true},
		{ID: "12", Name: "Start Progress", ToStatus: "In Progress", IsAvailable: true},
		{ID: "13", Name: "Blocked Progress", ToStatus: "In Progress", IsAvailable: true, Fields: []jira.TransitionField{{ID: "resolution", Required: true}}},
	})
	if !ok || transition.ID != "12" {
		t.Fatalf("transition = %#v ok=%v", transition, ok)
	}
}

type fakeGitClient struct {
	branchRepo string
	branchName string
}

func (f *fakeGitClient) DetectRepo(context.Context, string) (gitworkflow.RepoStatus, error) {
	return gitworkflow.RepoStatus{Path: "/tmp/repo", Detected: true}, nil
}

func (f *fakeGitClient) CreateOrSwitchBranch(_ context.Context, repoPath string, branch string) error {
	f.branchRepo = repoPath
	f.branchName = branch
	return nil
}
