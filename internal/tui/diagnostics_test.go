package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jon/jira-tui/internal/events"
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

	if len(next.diagnosticsEvents) != 3 {
		t.Fatalf("diagnostics events = %#v", next.diagnosticsEvents)
	}
	if next.diagnosticsEvents[0].Kind != diagnosticKindWorker || next.diagnosticsEvents[0].Status != "submit" {
		t.Fatalf("submit event = %#v", next.diagnosticsEvents[0])
	}
	if next.diagnosticsEvents[1].Kind != diagnosticKindWorker || next.diagnosticsEvents[1].Status != "ok" {
		t.Fatalf("result event = %#v", next.diagnosticsEvents[1])
	}
	if next.diagnosticsEvents[2].Kind != diagnosticKindAPI || next.diagnosticsEvents[2].Status != "ok" {
		t.Fatalf("api event = %#v", next.diagnosticsEvents[2])
	}
}

func TestDiagnosticsRecordsSanitizedAPIResult(t *testing.T) {
	now := time.Date(2026, 6, 16, 14, 0, 0, 0, time.UTC)
	model := NewModel(&fakeIssueSearcher{}, "project = ABC AND summary ~ \"secret launch\"")
	defer model.workers.Stop()
	model.now = func() time.Time { return now }

	updated, _ := model.Update(workSubmittedMsg{
		kind: worker.KindSearchIssues,
		id:   42,
		key:  model.jql,
	})
	model = updated.(Model)

	model.now = func() time.Time { return now.Add(150 * time.Millisecond) }
	updated, _ = model.Update(workerResultMsg{result: worker.Result{
		ID:   42,
		Kind: worker.KindSearchIssues,
		SearchIssues: &worker.SearchIssuesResult{
			Issues:   []jira.Issue{{Key: "ABC-1"}, {Key: "ABC-2"}},
			SyncedAt: now,
		},
	}})
	model = updated.(Model)

	event := lastDiagnosticEventOfKindForTest(t, model, diagnosticKindAPI)
	if event.Label != string(worker.KindSearchIssues) || event.Status != "ok" {
		t.Fatalf("api diagnostic event = %#v", event)
	}
	for _, want := range []string{"#42", "endpoint=search", "scope=jql", "result=success", "issues=2", "empty=false", "elapsed=150ms"} {
		if !strings.Contains(event.Detail, want) {
			t.Fatalf("api diagnostic detail = %q, missing %q", event.Detail, want)
		}
	}
	for _, leak := range []string{"secret launch", model.jql} {
		if strings.Contains(event.Detail, leak) {
			t.Fatalf("api diagnostic leaked raw query %q in %q", leak, event.Detail)
		}
	}
}

func TestDiagnosticsSummaryCountsAPIRowsSeparately(t *testing.T) {
	events := []diagnosticEvent{
		{Kind: diagnosticKindWorker, Label: string(worker.KindSearchIssues), Status: "ok", Detail: "#1"},
		{Kind: diagnosticKindAPI, Label: string(worker.KindSearchIssues), Status: "ok", Detail: "#1 endpoint=search"},
	}

	summary := NewModel(&fakeIssueSearcher{}, "project = ABC").renderDiagnosticsSummary(events, 120)

	for _, want := range []string{"Workers 1", "API 1", "Cache 0", "Events 0"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary = %q, missing %q", summary, want)
		}
	}
}

func TestDiagnosticsRecordsSanitizedAPIError(t *testing.T) {
	now := time.Date(2026, 6, 16, 14, 0, 0, 0, time.UTC)
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.now = func() time.Time { return now }

	updated, _ := model.Update(workSubmittedMsg{
		kind: worker.KindUpdateDescription,
		id:   77,
		key:  "ABC-1",
	})
	model = updated.(Model)

	rawErr := fmt.Errorf("jira returned 400 with body {\"token\":\"secret-token\",\"description\":\"raw body\"}")
	model.now = func() time.Time { return now.Add(2 * time.Second) }
	updated, _ = model.Update(workerResultMsg{result: worker.Result{
		ID:   77,
		Kind: worker.KindUpdateDescription,
		Err:  rawErr,
		UpdateDescription: &worker.UpdateDescriptionResult{
			Key: "ABC-1",
		},
	}})
	model = updated.(Model)

	event := lastDiagnosticEventOfKindForTest(t, model, diagnosticKindAPI)
	if event.Label != string(worker.KindUpdateDescription) || event.Status != "error" {
		t.Fatalf("api diagnostic event = %#v", event)
	}
	for _, want := range []string{"#77", "endpoint=issue", "scope=issue:ABC-1", "result=error", "elapsed=2s", "error="} {
		if !strings.Contains(event.Detail, want) {
			t.Fatalf("api diagnostic detail = %q, missing %q", event.Detail, want)
		}
	}
	for _, leak := range []string{"secret-token", "raw body", "description"} {
		if strings.Contains(event.Detail, leak) {
			t.Fatalf("api diagnostic leaked raw error content %q in %q", leak, event.Detail)
		}
	}
}

func TestDiagnosticsRecordsAppEvents(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()

	updated, _ := model.Update(appEventMsg{event: events.Event{
		Type:      events.TypeJiraTicketUpdated,
		Source:    "active_view",
		DedupeKey: "ABC-1",
	}})
	next := updated.(Model)

	if len(next.diagnosticsEvents) != 1 {
		t.Fatalf("diagnostics events = %#v", next.diagnosticsEvents)
	}
	event := next.diagnosticsEvents[0]
	if event.Kind != diagnosticKindEvent || event.Label != string(events.TypeJiraTicketUpdated) || event.Status != "published" {
		t.Fatalf("diagnostic event = %#v", event)
	}
	if !strings.Contains(event.Detail, "active_view") || !strings.Contains(event.Detail, "ABC-1") {
		t.Fatalf("diagnostic detail = %q", event.Detail)
	}
}

func lastDiagnosticEventOfKindForTest(t *testing.T, model Model, kind diagnosticKind) diagnosticEvent {
	t.Helper()
	for index := len(model.diagnosticsEvents) - 1; index >= 0; index-- {
		if model.diagnosticsEvents[index].Kind == kind {
			return model.diagnosticsEvents[index]
		}
	}
	t.Fatalf("missing diagnostic event kind %s in %#v", kind, model.diagnosticsEvents)
	return diagnosticEvent{}
}

func lastDiagnosticEventWithLabelForTest(t *testing.T, model Model, kind diagnosticKind, label string) diagnosticEvent {
	t.Helper()
	for index := len(model.diagnosticsEvents) - 1; index >= 0; index-- {
		event := model.diagnosticsEvents[index]
		if event.Kind == kind && event.Label == label {
			return event
		}
	}
	t.Fatalf("missing diagnostic event kind %s label %s in %#v", kind, label, model.diagnosticsEvents)
	return diagnosticEvent{}
}

func TestDiagnosticsRecordsStreamedAppEvents(t *testing.T) {
	stream := events.NewStream()
	defer stream.Close()
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithEventStream(stream))
	defer model.workers.Stop()

	cmd := model.waitForAppEvent()
	messages := make(chan tea.Msg, 1)
	go func() {
		messages <- cmd()
	}()
	if err := stream.Publish(context.Background(), events.Event{
		Type:      events.TypeJiraTicketUpdated,
		Source:    "active_view",
		DedupeKey: "ABC-1",
	}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	select {
	case msg := <-messages:
		updated, _ := model.Update(msg)
		next := updated.(Model)
		if len(next.diagnosticsEvents) != 1 {
			t.Fatalf("diagnostics events = %#v", next.diagnosticsEvents)
		}
		event := next.diagnosticsEvents[0]
		if event.Kind != diagnosticKindEvent || event.Label != string(events.TypeJiraTicketUpdated) || event.Status != "published" {
			t.Fatalf("diagnostic event = %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for streamed app event")
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
	event := lastDiagnosticEventWithLabelForTest(t, next, diagnosticKindWorker, string(worker.KindGetCreateIssueTypes))
	got := event.Detail
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
	event := lastDiagnosticEventWithLabelForTest(t, next, diagnosticKindWorker, string(worker.KindGetCreateFields))
	got := event.Detail
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

func TestDiagnosticsOverlayShowsCacheFamilySummary(t *testing.T) {
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = false
	model.diagnosticsOpen = true
	model.width = 140
	model.height = 30

	model.cacheActiveIssueView("project = ABC", []jira.Issue{{Key: "ABC-1"}}, now)
	model.cacheActiveIssueView("project = XYZ", []jira.Issue{{Key: "XYZ-1"}}, now.Add(-2*activeViewCacheTTL))
	model.cacheIssueDetail("ABC-1", jira.IssueDetail{Issue: jira.Issue{Key: "ABC-1"}}, now)
	model.cacheIssueDetail("ABC-2", jira.IssueDetail{Issue: jira.Issue{Key: "ABC-2"}}, now.Add(-2*issueDetailCacheTTL))
	model.cacheIssueComments("ABC-1", []jira.Comment{{ID: "10001"}}, now)
	model.cacheIssueTransitions("ABC-1", []jira.Transition{{ID: "21", Name: "Start"}}, now.Add(-2*issueTransitionsCacheTTL))
	model.cacheIssueEditMetadata("ABC-1", jira.EditMetadata{Summary: jira.EditField{ID: "summary"}}, now)
	model.cacheCreateIssueTypes("DEVOPS", []jira.CreateIssueType{{ID: "10001", Name: "Story"}}, now)
	model.cacheCreateFields("DEVOPS", "10001", []jira.CreateField{{ID: "summary", Name: "Summary"}}, now.Add(-2*createFieldsCacheTTL))
	model.cacheExpandedChildren("ABC-1", worker.ExpandModeOpen, []jira.Issue{{Key: "ABC-2"}}, now)

	view := model.render()

	for _, want := range []string{
		"Cache records",
		"active_view 1 fresh 1 stale",
		"issue_detail 1 fresh 1 stale",
		"comments 1 fresh 0 stale",
		"transitions 0 fresh 1 stale",
		"edit_meta 1 fresh 0 stale",
		"create_types 1 fresh 0 stale",
		"create_fields 0 fresh 1 stale",
		"expanded_children 1 fresh 0 stale",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestDiagnosticsOverlayShowsCacheRefreshFailureSummary(t *testing.T) {
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = false
	model.diagnosticsOpen = true
	model.width = 150
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.cacheActiveIssueView(model.jql, model.issues, now.Add(-2*activeViewCacheTTL))
	model.cacheIssueDetail("ABC-1", jira.IssueDetail{Issue: jira.Issue{Key: "ABC-1"}, Description: "Stale detail"}, now.Add(-2*issueDetailCacheTTL))
	model.cacheIssueComments("ABC-1", []jira.Comment{{ID: "10001", Body: "Stale comment"}}, now.Add(-2*issueCommentsCacheTTL))

	model.activeRequestID = 11
	updated, _ := model.Update(workerResultMsg{result: worker.Result{
		ID:   11,
		Kind: worker.KindSearchIssues,
		Err:  fmt.Errorf("search unavailable"),
	}})
	model = updated.(Model)

	model.activeDetailRequestID = 12
	model.detailRequestKey = "ABC-1"
	updated, _ = model.Update(workerResultMsg{result: worker.Result{
		ID:   12,
		Kind: worker.KindGetIssue,
		Err:  fmt.Errorf("detail unavailable"),
	}})
	model = updated.(Model)

	model.activeCommentsReqID = 13
	model.commentsRequestKey = "ABC-1"
	updated, _ = model.Update(workerResultMsg{result: worker.Result{
		ID:   13,
		Kind: worker.KindGetComments,
		Err:  fmt.Errorf("comments unavailable"),
	}})
	model = updated.(Model)

	view := model.render()

	for _, want := range []string{
		"active_view 0 fresh 1 stale 1 error",
		"issue_detail 0 fresh 1 stale 1 error",
		"comments 0 fresh 1 stale 1 error",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	if record, ok := model.cachedIssueDetail("ABC-1"); !ok || record.Err == nil {
		t.Fatalf("detail cache error was not retained: ok=%v record=%#v", ok, record)
	}
	if record, ok := model.cachedIssueComments("ABC-1"); !ok || record.Err == nil {
		t.Fatalf("comments cache error was not retained: ok=%v record=%#v", ok, record)
	}
}

func TestDiagnosticsOverlayShowsMetadataCacheRefreshFailures(t *testing.T) {
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = false
	model.diagnosticsOpen = true
	model.width = 180
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1"}}
	model.cacheIssueTransitions("ABC-1", []jira.Transition{{ID: "21"}}, now.Add(-2*issueTransitionsCacheTTL))
	model.cacheIssueEditMetadata("ABC-1", jira.EditMetadata{Summary: jira.EditField{ID: "summary"}}, now.Add(-2*issueEditMetadataCacheTTL))
	model.cacheCreateIssueTypes("ABC", []jira.CreateIssueType{{ID: "10001"}}, now.Add(-2*createIssueTypesCacheTTL))
	model.cacheCreateFields("ABC", "10001", []jira.CreateField{{ID: "summary"}}, now.Add(-2*createFieldsCacheTTL))
	model.cacheExpandedChildren("ABC-1", worker.ExpandModeOpen, []jira.Issue{{Key: "ABC-2"}}, now.Add(-2*expandedChildrenCacheTTL))

	model.activeTransitionsReqID = 21
	model.transitionRequestKey = "ABC-1"
	updated, _ := model.Update(workerResultMsg{result: worker.Result{
		ID:   21,
		Kind: worker.KindGetTransitions,
		Err:  fmt.Errorf("transitions unavailable"),
	}})
	model = updated.(Model)

	model.activeSummaryMetadataReqID = 22
	model.summaryMetadataRequestKey = "ABC-1"
	updated, _ = model.Update(workerResultMsg{result: worker.Result{
		ID:   22,
		Kind: worker.KindGetEditMetadata,
		Err:  fmt.Errorf("metadata unavailable"),
	}})
	model = updated.(Model)

	model.createOpen = true
	model.createProjectKey = "ABC"
	model.activeCreateIssueTypesReqID = 23
	updated, _ = model.Update(workerResultMsg{result: worker.Result{
		ID:   23,
		Kind: worker.KindGetCreateIssueTypes,
		Err:  fmt.Errorf("issue types unavailable"),
	}})
	model = updated.(Model)

	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Story"}
	model.activeCreateFieldsReqID = 24
	updated, _ = model.Update(workerResultMsg{result: worker.Result{
		ID:   24,
		Kind: worker.KindGetCreateFields,
		Err:  fmt.Errorf("fields unavailable"),
	}})
	model = updated.(Model)

	model.activeExpandReqID = 25
	model.expandRequestKey = "ABC-1"
	model.expandMode = worker.ExpandModeOpen
	updated, _ = model.Update(workerResultMsg{result: worker.Result{
		ID:   25,
		Kind: worker.KindExpandIssues,
		Err:  fmt.Errorf("children unavailable"),
	}})
	model = updated.(Model)

	view := model.render()

	for _, want := range []string{
		"transitions 0 fresh 1 stale 1 error",
		"edit_meta 0 fresh 1 stale 1 error",
		"create_types 0 fresh 1 stale 1 error",
		"create_fields 0 fresh 1 stale 1 error",
		"expanded_children 0 fresh 1 stale 1 error",
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
