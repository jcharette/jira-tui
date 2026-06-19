package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLoadReadsConfigFile(t *testing.T) {
	path := writeConfig(t, `
version = 1
active_profile = "default"

[profiles.default]
base_url = "https://example.atlassian.net/"
email = "person@example.com"
api_token = "secret"

[queries]
default_project = "ABC"

[appearance]
primary = "#7DD3FC"
accent = "#F59E0B"

[display]
symbol_mode = "symbols"

[runtime]
refresh_interval = "30s"
request_timeout = "5s"
workers = 4
queue_size = 32

[git]
branch_template = "feature/{key}-{summary_slug}"

[claude]
enabled = true
command = "/opt/homebrew/bin/claude"
timeout = "45s"

[claude.features]
ticket_plan = true
ticket_assist = true
clarifying_questions = true
draft_comment = false
draft_ticket = true
branch_plan = false
code_changes = false
pr_creation = false
pr_review_response = true

[claude.gates]
require_confirmation = true
allow_jira_writes = false
allow_git_writes = true
allow_github_writes = false
allow_code_edits = false
`)

	cfg, err := Load(LoadOptions{Path: path})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.BaseURL != "https://example.atlassian.net" {
		t.Fatalf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.DefaultProject != "ABC" {
		t.Fatalf("DefaultProject = %q", cfg.DefaultProject)
	}
	if cfg.DefaultJQL != DefaultJQLForProject("ABC") {
		t.Fatalf("DefaultJQL = %q", cfg.DefaultJQL)
	}
	if cfg.Theme.Primary != "#7DD3FC" {
		t.Fatalf("Theme.Primary = %q", cfg.Theme.Primary)
	}
	if cfg.Theme.Accent != "#F59E0B" {
		t.Fatalf("Theme.Accent = %q", cfg.Theme.Accent)
	}
	if cfg.Display.SymbolMode != "symbols" {
		t.Fatalf("Display.SymbolMode = %q", cfg.Display.SymbolMode)
	}
	if !hasView(cfg.Views, "Current Sprint") {
		t.Fatalf("views = %#v", cfg.Views)
	}
	if cfg.RefreshInterval != 30*time.Second {
		t.Fatalf("RefreshInterval = %s", cfg.RefreshInterval)
	}
	if cfg.RequestTimeout != 5*time.Second {
		t.Fatalf("RequestTimeout = %s", cfg.RequestTimeout)
	}
	if cfg.WorkerCount != 4 {
		t.Fatalf("WorkerCount = %d", cfg.WorkerCount)
	}
	if cfg.QueueSize != 32 {
		t.Fatalf("QueueSize = %d", cfg.QueueSize)
	}
	if cfg.Git.BranchTemplate != "feature/{key}-{summary_slug}" {
		t.Fatalf("Git.BranchTemplate = %q", cfg.Git.BranchTemplate)
	}
	if !cfg.Claude.Enabled {
		t.Fatal("expected Claude to be enabled")
	}
	if cfg.Claude.Command != "/opt/homebrew/bin/claude" {
		t.Fatalf("Claude.Command = %q", cfg.Claude.Command)
	}
	if cfg.Claude.Timeout != 45*time.Second {
		t.Fatalf("Claude.Timeout = %s", cfg.Claude.Timeout)
	}
	if !cfg.Claude.Features.TicketPlan || !cfg.Claude.Features.TicketAssist || !cfg.Claude.Features.ClarifyingQuestions || !cfg.Claude.Features.DraftTicket || !cfg.Claude.Features.PRReviewResponse {
		t.Fatalf("Claude.Features = %#v", cfg.Claude.Features)
	}
	if !cfg.Claude.Gates.RequireConfirmation || !cfg.Claude.Gates.AllowGitWrites {
		t.Fatalf("Claude.Gates = %#v", cfg.Claude.Gates)
	}
}

func TestLoadUsesRequestedProfileAndRetainsProfiles(t *testing.T) {
	path := writeConfig(t, `
version = 1
active_profile = "default"

[profiles.default]
base_url = "https://default.atlassian.net"
email = "default@example.com"
api_token = "default-token"

[profiles.work]
base_url = "https://work.atlassian.net/"
email = "work@example.com"
api_token = "work-token"

[queries]
default_project = "ABC"
`)

	cfg, err := Load(LoadOptions{Path: path, Profile: "work"})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.ActiveProfile != "work" {
		t.Fatalf("ActiveProfile = %q", cfg.ActiveProfile)
	}
	if cfg.BaseURL != "https://work.atlassian.net" || cfg.Email != "work@example.com" || cfg.APIToken != "work-token" {
		t.Fatalf("selected credentials = %#v", cfg)
	}
	if len(cfg.Profiles) != 2 {
		t.Fatalf("Profiles = %#v", cfg.Profiles)
	}
	if cfg.Profiles["default"].Email != "default@example.com" || cfg.Profiles["work"].Email != "work@example.com" {
		t.Fatalf("Profiles = %#v", cfg.Profiles)
	}
}

func TestLoadRejectsUnknownRequestedProfile(t *testing.T) {
	path := writeConfig(t, `
version = 1
active_profile = "default"

[profiles.default]
base_url = "https://default.atlassian.net"
email = "default@example.com"
api_token = "default-token"

[queries]
default_project = "ABC"
`)

	_, err := Load(LoadOptions{Path: path, Profile: "missing"})
	if err == nil || !strings.Contains(err.Error(), `profile "missing" is not defined`) {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestSavePreservesNonActiveProfiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jira", "config.toml")
	cfg := Defaults()
	cfg.ActiveProfile = "work"
	cfg.Profiles = map[string]Profile{
		"default": {
			BaseURL:  "https://default.atlassian.net",
			Email:    "default@example.com",
			APIToken: "default-token",
		},
		"work": {
			BaseURL:  "https://old-work.atlassian.net",
			Email:    "old-work@example.com",
			APIToken: "old-work-token",
		},
	}
	cfg.BaseURL = "https://work.atlassian.net"
	cfg.Email = "work@example.com"
	cfg.APIToken = "work-token"
	cfg.DefaultProject = "ABC"
	cfg.DefaultJQL = DefaultJQLForProject("ABC")
	cfg.Views = DefaultViews("ABC")
	cfg.ActiveView = cfg.Views[0].Name

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	work, err := Load(LoadOptions{Path: path, Profile: "work"})
	if err != nil {
		t.Fatalf("Load(work) error = %v", err)
	}
	if work.BaseURL != "https://work.atlassian.net" || work.Email != "work@example.com" || work.APIToken != "work-token" {
		t.Fatalf("work profile credentials = %#v", work)
	}
	defaultProfile, err := Load(LoadOptions{Path: path, Profile: "default"})
	if err != nil {
		t.Fatalf("Load(default) error = %v", err)
	}
	if defaultProfile.BaseURL != "https://default.atlassian.net" || defaultProfile.Email != "default@example.com" || defaultProfile.APIToken != "default-token" {
		t.Fatalf("default profile credentials = %#v", defaultProfile)
	}
}

func TestDefaultsDisableClaudeWithSafeGates(t *testing.T) {
	cfg := Defaults()

	if cfg.Claude.Enabled {
		t.Fatal("Claude should default to disabled")
	}
	if cfg.Claude.Command != "" {
		t.Fatalf("Claude.Command = %q, want auto-detect empty command", cfg.Claude.Command)
	}
	if cfg.Claude.Timeout != 2*time.Minute {
		t.Fatalf("Claude.Timeout = %s", cfg.Claude.Timeout)
	}
	if !cfg.Claude.Gates.RequireConfirmation {
		t.Fatalf("Claude.Gates = %#v", cfg.Claude.Gates)
	}
	if cfg.Claude.Gates.AllowJiraWrites || cfg.Claude.Gates.AllowGitWrites || cfg.Claude.Gates.AllowGitHubWrites || cfg.Claude.Gates.AllowCodeEdits {
		t.Fatalf("Claude write gates should default closed: %#v", cfg.Claude.Gates)
	}
}

func TestDefaultsIncludeGitBranchTemplate(t *testing.T) {
	cfg := Defaults()

	if cfg.Git.BranchTemplate != "{key}-{summary_slug}" {
		t.Fatalf("Git.BranchTemplate = %q", cfg.Git.BranchTemplate)
	}
}

func TestDefaultViewsIncludeEpicFocusedView(t *testing.T) {
	views := DefaultViews("ABC")

	for _, name := range []string{"Assigned", "Created/Reported", "Project Open", "Current Sprint", "Watching", "Epics"} {
		if !hasView(views, name) {
			t.Fatalf("missing view %q in %#v", name, views)
		}
	}
	epics, ok := findView(views, "Epics")
	if !ok {
		t.Fatalf("missing Epics view in %#v", views)
	}
	for _, want := range []string{"project = ABC", "issuetype = Epic", "resolution = Unresolved", "ORDER BY updated DESC"} {
		if !strings.Contains(epics.JQL, want) {
			t.Fatalf("Epics JQL = %q, missing %q", epics.JQL, want)
		}
	}
	if !epics.IncludeChildren {
		t.Fatalf("Epics IncludeChildren = false, want true")
	}
	for _, view := range views {
		if view.Name != "Epics" && view.IncludeChildren {
			t.Fatalf("%s IncludeChildren = true, want false", view.Name)
		}
	}
}

func TestLoadReadsViewIncludeChildren(t *testing.T) {
	path := writeConfig(t, `
version = 1
active_profile = "default"

[profiles.default]
base_url = "https://example.atlassian.net"
email = "person@example.com"
api_token = "secret"

[queries]
default_project = "ABC"

[views]
active = "Epics"

[[views.saved]]
name = "Assigned"
jql = "assignee = currentUser()"

[[views.saved]]
name = "Epics"
jql = "issuetype = Epic"
include_children = true
`)

	cfg, err := Load(LoadOptions{Path: path})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	assigned, ok := findView(cfg.Views, "Assigned")
	if !ok {
		t.Fatalf("missing Assigned view in %#v", cfg.Views)
	}
	if assigned.IncludeChildren {
		t.Fatalf("Assigned IncludeChildren = true, want false")
	}
	epics, ok := findView(cfg.Views, "Epics")
	if !ok {
		t.Fatalf("missing Epics view in %#v", cfg.Views)
	}
	if !epics.IncludeChildren {
		t.Fatalf("Epics IncludeChildren = false, want true")
	}
}

func TestAddSavedViewAppendsTrimmedViewAndPreservesActiveView(t *testing.T) {
	cfg := Defaults()
	cfg.BaseURL = "https://example.atlassian.net"
	cfg.Email = "person@example.com"
	cfg.APIToken = "secret"
	cfg.ActiveProfile = "default"
	cfg.Profiles = map[string]Profile{
		"default": {
			BaseURL:  "https://example.atlassian.net",
			Email:    "person@example.com",
			APIToken: "secret",
		},
	}
	cfg.DefaultProject = "ABC"
	cfg.Views = []IssueView{{Name: "Assigned", JQL: "assignee = currentUser()"}}
	cfg.ActiveView = "Assigned"

	next, err := AddSavedView(cfg, IssueView{
		Name:            "  My Active Work  ",
		JQL:             "  project = ABC AND status = \"In Progress\"  ",
		IncludeChildren: true,
	})
	if err != nil {
		t.Fatalf("AddSavedView() error = %v", err)
	}

	if next.ActiveView != cfg.ActiveView {
		t.Fatalf("ActiveView = %q, want %q", next.ActiveView, cfg.ActiveView)
	}
	if len(next.Views) != 2 {
		t.Fatalf("Views count = %d, want 2", len(next.Views))
	}
	got := next.Views[1]
	if got.Name != "My Active Work" || got.JQL != "project = ABC AND status = \"In Progress\"" || !got.IncludeChildren {
		t.Fatalf("saved view = %#v", got)
	}
	if len(cfg.Views) != 1 {
		t.Fatalf("original config views mutated: %#v", cfg.Views)
	}
}

func TestAddSavedViewRejectsInvalidView(t *testing.T) {
	cfg := Defaults()
	cfg.Views = []IssueView{{Name: "Assigned", JQL: "assignee = currentUser()"}}

	for _, tc := range []struct {
		name string
		view IssueView
		want string
	}{
		{name: "blank name", view: IssueView{Name: "  ", JQL: "project = ABC"}, want: "name"},
		{name: "blank jql", view: IssueView{Name: "My View", JQL: "  "}, want: "JQL"},
		{name: "duplicate name", view: IssueView{Name: " assigned ", JQL: "project = ABC"}, want: "already exists"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := AddSavedView(cfg, tc.view)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("AddSavedView() error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestSetSavedViewsReplacesTrimmedViewsAndPreservesActiveView(t *testing.T) {
	cfg := Defaults()
	cfg.Views = []IssueView{
		{Name: "Assigned", JQL: "assignee = currentUser()"},
		{Name: "Project", JQL: "project = ABC"},
	}
	cfg.ActiveView = "Project"

	next, err := SetSavedViews(cfg, []IssueView{
		{Name: "  Active Work  ", JQL: "  project = ABC AND status = \"In Progress\"  "},
		{Name: "Epics", JQL: "project = ABC AND issuetype = Epic", IncludeChildren: true},
	})
	if err != nil {
		t.Fatalf("SetSavedViews() error = %v", err)
	}

	if next.ActiveView != "Active Work" {
		t.Fatalf("ActiveView = %q, want first replacement when previous active is removed", next.ActiveView)
	}
	if len(next.Views) != 2 {
		t.Fatalf("Views count = %d, want 2", len(next.Views))
	}
	if next.Views[0].Name != "Active Work" || next.Views[0].JQL != "project = ABC AND status = \"In Progress\"" {
		t.Fatalf("first view = %#v", next.Views[0])
	}
	if !next.Views[1].IncludeChildren {
		t.Fatalf("include children not preserved: %#v", next.Views[1])
	}
	if cfg.Views[0].Name != "Assigned" {
		t.Fatalf("original config views mutated: %#v", cfg.Views)
	}
}

func TestSetSavedViewsRejectsInvalidViews(t *testing.T) {
	cfg := Defaults()

	for _, tc := range []struct {
		name  string
		views []IssueView
		want  string
	}{
		{name: "empty", views: nil, want: "at least one"},
		{name: "blank name", views: []IssueView{{Name: " ", JQL: "project = ABC"}}, want: "name"},
		{name: "blank jql", views: []IssueView{{Name: "Mine", JQL: " "}}, want: "JQL"},
		{name: "duplicate", views: []IssueView{
			{Name: "Mine", JQL: "project = ABC"},
			{Name: " mine ", JQL: "assignee = currentUser()"},
		}, want: "already exists"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := SetSavedViews(cfg, tc.views)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("SetSavedViews() error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func hasView(views []IssueView, name string) bool {
	_, ok := findView(views, name)
	return ok
}

func findView(views []IssueView, name string) (IssueView, bool) {
	for _, view := range views {
		if view.Name == name {
			return view, true
		}
	}
	return IssueView{}, false
}

func TestLoadIgnoresEnvironmentVariables(t *testing.T) {
	path := writeConfig(t, `
version = 1
active_profile = "default"

[profiles.default]
base_url = "https://file.atlassian.net"
email = "file@example.com"
api_token = "file-token"

[queries]
default_project = "FILE"

[runtime]
workers = 2
queue_size = 16
`)
	t.Setenv("JIRA_BASE_URL", "https://env.atlassian.net")
	t.Setenv("JIRA_EMAIL", "env@example.com")
	t.Setenv("JIRA_API_TOKEN", "env-token")
	t.Setenv("JIRA_PROJECT", "ENV")
	t.Setenv("JIRA_WORKERS", "6")

	cfg, err := Load(LoadOptions{Path: path})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.BaseURL != "https://file.atlassian.net" {
		t.Fatalf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.Email != "file@example.com" {
		t.Fatalf("Email = %q", cfg.Email)
	}
	if cfg.APIToken != "file-token" {
		t.Fatalf("APIToken = %q", cfg.APIToken)
	}
	if cfg.DefaultProject != "FILE" {
		t.Fatalf("DefaultProject = %q", cfg.DefaultProject)
	}
	if cfg.WorkerCount != 2 {
		t.Fatalf("WorkerCount = %d", cfg.WorkerCount)
	}
}

func TestSaveWritesConfigFileWithPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jira", "config.toml")
	cfg := Defaults()
	cfg.BaseURL = "https://example.atlassian.net"
	cfg.Email = "person@example.com"
	cfg.APIToken = "secret"
	cfg.ActiveProfile = "default"
	cfg.Profiles = map[string]Profile{
		"default": {
			BaseURL:  "https://example.atlassian.net",
			Email:    "person@example.com",
			APIToken: "secret",
		},
	}
	cfg.DefaultProject = "ABC"
	cfg.DefaultJQL = DefaultJQLForProject("ABC")
	cfg.Views = DefaultViews("ABC")
	cfg.ActiveView = cfg.Views[0].Name
	cfg.RefreshInterval = 30 * time.Second
	cfg.RequestTimeout = 5 * time.Second
	cfg.WorkerCount = 4
	cfg.QueueSize = 32
	cfg.Display.SymbolMode = "emoji"
	cfg.Claude.Enabled = true
	cfg.Claude.Command = "/usr/local/bin/claude"
	cfg.Claude.Timeout = 90 * time.Second
	cfg.Claude.Features.TicketPlan = true
	cfg.Claude.Features.TicketAssist = true
	cfg.Claude.Features.ClarifyingQuestions = true
	cfg.Claude.Gates.AllowGitWrites = true

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %v", info.Mode().Perm())
	}

	loaded, err := Load(LoadOptions{Path: path})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(loaded, cfg) {
		t.Fatalf("loaded config = %#v", loaded)
	}
}

func TestLoadRejectsInvalidClaudeTimeout(t *testing.T) {
	path := writeConfig(t, `
version = 1
active_profile = "default"

[profiles.default]
base_url = "https://example.atlassian.net"
email = "person@example.com"
api_token = "secret"

[queries]
default_project = "ABC"

[claude]
enabled = true
timeout = "-1s"
`)

	_, err := Load(LoadOptions{Path: path})
	if err == nil {
		t.Fatal("expected invalid Claude timeout error")
	}
}

func TestLoadHonorsExplicitClaudeRequireConfirmationFalse(t *testing.T) {
	path := writeConfig(t, `
version = 1
active_profile = "default"

[profiles.default]
base_url = "https://example.atlassian.net"
email = "person@example.com"
api_token = "secret"

[queries]
default_project = "ABC"

[claude.gates]
require_confirmation = false
`)

	cfg, err := Load(LoadOptions{Path: path})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Claude.Gates.RequireConfirmation {
		t.Fatal("expected explicit require_confirmation=false to be honored")
	}
}

func TestLoadRejectsInvalidDisplaySymbolMode(t *testing.T) {
	path := writeConfig(t, `
version = 1
active_profile = "default"

[profiles.default]
base_url = "https://example.atlassian.net"
email = "person@example.com"
api_token = "secret"

[queries]
default_project = "ABC"

[display]
symbol_mode = "sparkles"
`)

	_, err := Load(LoadOptions{Path: path})
	if err == nil {
		t.Fatal("expected invalid symbol mode error")
	}
}

func TestLoadEditableReturnsValidationProblems(t *testing.T) {
	cfg, _, problems, err := LoadEditable(LoadOptions{Path: filepath.Join(t.TempDir(), "missing.toml")})
	if err != nil {
		t.Fatalf("LoadEditable() error = %v", err)
	}
	if cfg.DefaultJQL == "" {
		t.Fatal("expected defaults to be populated")
	}
	if len(problems) == 0 {
		t.Fatal("expected validation problems")
	}
}

func TestLoadRejectsInvalidFileDuration(t *testing.T) {
	path := writeConfig(t, `
version = 1
active_profile = "default"

[profiles.default]
base_url = "https://example.atlassian.net"
email = "person@example.com"
api_token = "secret"

[queries]
default_project = "ABC"

[runtime]
refresh_interval = "eventually"
`)

	_, err := Load(LoadOptions{Path: path})
	if err == nil {
		t.Fatal("expected invalid duration error")
	}
}

func TestLoadRejectsInvalidFileWorkerCount(t *testing.T) {
	path := writeConfig(t, `
version = 1
active_profile = "default"

[profiles.default]
base_url = "https://example.atlassian.net"
email = "person@example.com"
api_token = "secret"

[queries]
default_project = "ABC"

[runtime]
workers = -1
`)

	_, err := Load(LoadOptions{Path: path})
	if err == nil {
		t.Fatal("expected invalid worker count error")
	}
}

func TestLoadRejectsInvalidAppearanceColor(t *testing.T) {
	path := writeConfig(t, `
version = 1
active_profile = "default"

[profiles.default]
base_url = "https://example.atlassian.net"
email = "person@example.com"
api_token = "secret"

[queries]
default_project = "ABC"

[appearance]
primary = "blue"
`)

	_, err := Load(LoadOptions{Path: path})
	if err == nil {
		t.Fatal("expected invalid color error")
	}
}

func writeConfig(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(contents)), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}
