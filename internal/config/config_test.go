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

func hasView(views []IssueView, name string) bool {
	for _, view := range views {
		if view.Name == name {
			return true
		}
	}
	return false
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
