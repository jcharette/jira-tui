package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jon/jira-tui/internal/jira"
	"github.com/jon/jira-tui/internal/worker"
)

func TestDiagnosticsOverlayTogglesWithoutChangingMode(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "One"}}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+d"}))
	next := updated.(Model)

	if !next.diagnosticsOpen {
		t.Fatal("expected diagnostics overlay to open")
	}
	if next.mode != modeDetail {
		t.Fatalf("mode = %v, want detail", next.mode)
	}
	view := next.render()
	for _, want := range []string{"Diagnostics", "Background Activity", "esc close"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "esc", Code: tea.KeyEsc}))
	next = updated.(Model)
	if next.diagnosticsOpen {
		t.Fatal("expected diagnostics overlay to close")
	}
	if next.mode != modeDetail {
		t.Fatalf("mode after close = %v, want detail", next.mode)
	}
}

func TestDiagnosticsRecordsWorkerSubmitAndResult(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()

	updated, _ := model.Update(workSubmittedMsg{
		kind: worker.KindGetIssue,
		id:   42,
		key:  "ABC-1",
	})
	next := updated.(Model)

	updated, _ = next.Update(workerResultMsg{result: worker.Result{
		ID:   42,
		Kind: worker.KindGetIssue,
		GetIssue: &worker.GetIssueResult{
			Key:    "ABC-1",
			Detail: jira.IssueDetail{Issue: jira.Issue{Key: "ABC-1"}},
		},
	}})
	next = updated.(Model)

	if len(next.diagnosticsEvents) != 2 {
		t.Fatalf("diagnostics events = %#v", next.diagnosticsEvents)
	}
	if next.diagnosticsEvents[0].Kind != diagnosticKindWorker || next.diagnosticsEvents[0].Status != "submit" {
		t.Fatalf("submit event = %#v", next.diagnosticsEvents[0])
	}
	if next.diagnosticsEvents[1].Kind != diagnosticKindWorker || next.diagnosticsEvents[1].Status != "ok" {
		t.Fatalf("result event = %#v", next.diagnosticsEvents[1])
	}
}

func TestDiagnosticsRecordsCreateIssueTypeResultCount(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()

	updated, _ := model.Update(workerResultMsg{result: worker.Result{
		ID:   77,
		Kind: worker.KindGetCreateIssueTypes,
		GetCreateIssueTypes: &worker.GetCreateIssueTypesResult{
			ProjectKey: "DEVOPS",
		},
	}})
	next := updated.(Model)

	if len(next.diagnosticsEvents) == 0 {
		t.Fatal("expected diagnostics event")
	}
	got := next.diagnosticsEvents[len(next.diagnosticsEvents)-1].Detail
	for _, want := range []string{"#77", "DEVOPS", "types=0"} {
		if !strings.Contains(got, want) {
			t.Fatalf("diagnostic detail = %q, missing %q", got, want)
		}
	}
}

func TestDiagnosticsRecordsCreateFieldSupportSummary(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()

	updated, _ := model.Update(workerResultMsg{result: worker.Result{
		ID:   78,
		Kind: worker.KindGetCreateFields,
		GetCreateFields: &worker.GetCreateFieldsResult{
			ProjectKey:  "DEVOPS",
			IssueTypeID: "10001",
			Fields: []jira.CreateField{
				{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
				{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
				{ID: "priority", Name: "Priority", Required: true, SchemaSystem: "priority", SchemaType: "priority", AllowedValues: []jira.FieldOption{{ID: "3", Name: "Medium"}}},
				{ID: "customfield_10020", Name: "Asset", Required: true, SchemaType: "array", SchemaCustom: "com.atlassian.assets"},
			},
		},
	}})
	next := updated.(Model)

	if len(next.diagnosticsEvents) == 0 {
		t.Fatal("expected diagnostics event")
	}
	got := next.diagnosticsEvents[len(next.diagnosticsEvents)-1].Detail
	for _, want := range []string{"fields=4", "supported=1", "required_unsupported=1", "priority", "customfield_10020"} {
		if !strings.Contains(got, want) {
			t.Fatalf("diagnostic detail = %q, missing %q", got, want)
		}
	}
}

func TestDiagnosticsEventsAreBounded(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()

	for i := 0; i < maxDiagnosticsEvents+5; i++ {
		model.recordDiagnosticEvent(diagnosticKindWorker, "search_issues", "submit", fmt.Sprintf("request %d", i))
	}

	if len(model.diagnosticsEvents) != maxDiagnosticsEvents {
		t.Fatalf("diagnostics events = %d, want %d", len(model.diagnosticsEvents), maxDiagnosticsEvents)
	}
	if !strings.Contains(model.diagnosticsEvents[0].Detail, "request 5") {
		t.Fatalf("oldest retained event = %#v", model.diagnosticsEvents[0])
	}
}

func TestDiagnosticsRecordsDetailCacheDecisions(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = false
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.cacheIssueDetail("ABC-1", jira.IssueDetail{Issue: jira.Issue{Key: "ABC-1"}, Description: "Cached detail"}, now)
	model.cacheIssueComments("ABC-1", nil, now)

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	fresh := updated.(Model)

	if len(fresh.diagnosticsEvents) == 0 || fresh.diagnosticsEvents[len(fresh.diagnosticsEvents)-1].Status != "hit" {
		t.Fatalf("fresh cache events = %#v", fresh.diagnosticsEvents)
	}

	model.now = func() time.Time { return now.Add(issueDetailCacheTTL) }
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	stale := updated.(Model)

	if len(stale.diagnosticsEvents) == 0 || stale.diagnosticsEvents[len(stale.diagnosticsEvents)-1].Status != "stale" {
		t.Fatalf("stale cache events = %#v", stale.diagnosticsEvents)
	}

	delete(model.details, "ABC-1")
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	miss := updated.(Model)

	if len(miss.diagnosticsEvents) == 0 || miss.diagnosticsEvents[len(miss.diagnosticsEvents)-1].Status != "miss" {
		t.Fatalf("miss cache events = %#v", miss.diagnosticsEvents)
	}
}

func TestDiagnosticsOverlayShowsSummaryAndActivityBars(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.diagnosticsOpen = true
	model.width = 100
	model.height = 30
	model.diagnosticsEvents = []diagnosticEvent{
		{At: time.Now(), Kind: diagnosticKindWorker, Label: string(worker.KindSearchIssues), Status: "submit", Detail: "#1"},
		{At: time.Now(), Kind: diagnosticKindWorker, Label: string(worker.KindSearchIssues), Status: "ok", Detail: "#1"},
		{At: time.Now(), Kind: diagnosticKindWorker, Label: string(worker.KindGetIssue), Status: "submit", Detail: "#2 ABC-1"},
		{At: time.Now(), Kind: diagnosticKindCache, Label: "issue_detail", Status: "miss", Detail: "ABC-1"},
		{At: time.Now(), Kind: diagnosticKindCache, Label: "issue_detail", Status: "hit", Detail: "ABC-1"},
		{At: time.Now(), Kind: diagnosticKindWorker, Label: string(worker.KindGetComments), Status: "error", Detail: "#3 ABC-1 failed"},
	}

	view := model.render()

	for _, want := range []string{
		"Workers 4",
		"Cache 2",
		"Errors 1",
		"Active 1",
		"Activity",
		"worker [",
		"cache  [",
		"Last get_comments error",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestDiagnosticsQueueSummaryShowsWorkerSchedulerState(t *testing.T) {
	summary := renderWorkerQueueSummary(worker.Stats{
		Running:   1,
		Pending:   2,
		Coalesced: 3,
		Capacity:  4,
	}, 100)

	for _, want := range []string{"Queue running 1", "pending 2", "coalesced 3", "capacity 4"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("missing %q in %q", want, summary)
		}
	}
}

func TestDiagnosticsOverlayShowsQueueSummaryWithoutEvents(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.diagnosticsOpen = true
	model.width = 100
	model.height = 30

	view := model.render()

	for _, want := range []string{"Queue running 0", "pending 0", "No background activity recorded yet."} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestDiagnosticsOverlayRowsIncludeOperationLabels(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.diagnosticsOpen = true
	model.width = 100
	model.height = 30
	model.diagnosticsEvents = []diagnosticEvent{
		{At: time.Now(), Kind: diagnosticKindWorker, Label: string(worker.KindGetIssue), Status: "submit", Detail: "#7 ABC-1"},
		{At: time.Now(), Kind: diagnosticKindCache, Label: "issue_detail", Status: "stale", Detail: "ABC-1"},
	}

	view := model.render()

	for _, want := range []string{"get_issue #7 ABC-1", "issue_detail ABC-1"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestDiagnosticsShowsStartupClaudeStatus(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeStatus(ClaudeStatus{
			Enabled:   true,
			Available: true,
			Command:   "/opt/homebrew/bin/claude",
			Version:   "Claude Code 2.0.0",
			Message:   "Claude ready",
		}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.diagnosticsOpen = true
	model.width = 120
	model.height = 30

	view := model.render()

	for _, want := range []string{"claude", "ready", "/opt/homebrew/bin/claude", "Claude Code 2.0.0"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}
