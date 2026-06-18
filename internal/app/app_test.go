package app

import (
	"path/filepath"
	"testing"

	"github.com/jcharette/jira-tui/internal/config"
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
