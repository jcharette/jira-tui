package tui

import (
	"net/url"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jcharette/jira-tui/internal/jira"
)

func TestBugReportOpensPrefilledGitHubIssueWithDiagnostics(t *testing.T) {
	var opened string
	withLinkActions(t, func(target string) error {
		opened = target
		return nil
	}, func(string) error { return nil })

	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.width = 120
	model.height = 35
	model.mode = modeTable
	model.issueLayout = issueLayoutLanes
	model.issues = []jira.Issue{{Key: "ABC-123", Summary: "Example story"}}
	model.diagnosticLogPath = "/tmp/jira-tui/diagnostics.jsonl"
	model.recordDiagnosticEvent(diagnosticKindAPI, "search", "error", "request_id=7 status=timeout jql=sha256:abc123")

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "B", Code: 'B'}))
	next := updated.(Model)
	if !next.bugReportOpen {
		t.Fatal("expected bug report composer to open")
	}

	next.bugReportTitleEditor.SetValue("Refresh freezes the issue list")
	next.bugReportBodyEditor.SetValue("Pressed refresh and the issue list stopped responding.")
	next.bugReportIncludeDiagnostics = true

	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected submit to open GitHub issue composer")
	}
	if next.bugReportOpen {
		t.Fatal("expected bug report composer to close after submit")
	}

	msg := cmd()
	if linkMsg, ok := msg.(linkActionMsg); !ok || linkMsg.err != nil {
		t.Fatalf("submit msg = %#v, want successful linkActionMsg", msg)
	}
	if opened == "" {
		t.Fatal("expected browser URL to be opened")
	}

	parsed, err := url.Parse(opened)
	if err != nil {
		t.Fatalf("parse opened URL: %v", err)
	}
	if parsed.Scheme != "https" || parsed.Host != "github.com" || parsed.Path != "/jcharette/jira-tui/issues/new" {
		t.Fatalf("opened URL = %s", opened)
	}
	values := parsed.Query()
	if got := values.Get("title"); got != "Refresh freezes the issue list" {
		t.Fatalf("title = %q", got)
	}
	if got := values.Get("labels"); got != "bug" {
		t.Fatalf("labels = %q, want bug", got)
	}
	body := values.Get("body")
	for _, want := range []string{
		"## What happened",
		"Pressed refresh and the issue list stopped responding.",
		"Selected issue: ABC-123",
		"Layout: Lanes",
		"Sanitized diagnostics excerpt",
		"request_id=7",
		"jql=sha256:abc123",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("issue body missing %q:\n%s", want, body)
		}
	}
}

func TestBugReportRequiresUserDetails(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()

	model = model.startBugReport()
	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("expected no command for an empty bug report")
	}
	if !next.bugReportOpen {
		t.Fatal("expected composer to stay open")
	}
	if !strings.Contains(next.detailNotice, "Add a short title or description") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestBugReportDiagnosticsDefaultOffUntilUserOptsIn(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.recordDiagnosticEvent(diagnosticKindState, "layout", "change", "layout=Lanes")

	model = model.startBugReport()
	if model.bugReportIncludeDiagnostics {
		t.Fatal("expected diagnostics to default off even when events exist")
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "x", Code: 'x'}))
	next := updated.(Model)
	if !strings.Contains(next.bugReportTitleValue(), "x") {
		t.Fatalf("typing in title should edit title, got %q", next.bugReportTitleValue())
	}
	if next.bugReportIncludeDiagnostics {
		t.Fatal("typing in title should not toggle diagnostics")
	}

	next.bugReportFieldFocus = 2
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeySpace}))
	next = updated.(Model)
	if !next.bugReportIncludeDiagnostics {
		t.Fatal("space on checkbox should opt into diagnostics")
	}
}

func TestBugReportDiagnosticsExcerptRedactsSensitiveKeyValues(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.recordDiagnosticEvent(diagnosticKindState, "unsafe", "error", "token=secret-token password=hunter2 api_token=abc123 safe=value")

	excerpt := model.bugReportDiagnosticsExcerpt()
	for _, leak := range []string{"secret-token", "hunter2", "abc123"} {
		if strings.Contains(excerpt, leak) {
			t.Fatalf("diagnostics excerpt leaked %q:\n%s", leak, excerpt)
		}
	}
	for _, want := range []string{"token=[redacted]", "password=[redacted]", "api_token=[redacted]", "safe=value"} {
		if !strings.Contains(excerpt, want) {
			t.Fatalf("diagnostics excerpt missing %q:\n%s", want, excerpt)
		}
	}
}
