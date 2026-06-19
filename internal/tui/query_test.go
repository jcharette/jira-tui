package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jcharette/jira-tui/internal/cache"
	"github.com/jcharette/jira-tui/internal/claude"
	"github.com/jcharette/jira-tui/internal/config"
	"github.com/jcharette/jira-tui/internal/events"
)

func TestQueryModalOpensWithCurrentJQLFromTable(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC ORDER BY updated DESC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeTable

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "/", Code: '/'}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("opening query modal should not submit work")
	}
	if !next.queryOpen {
		t.Fatal("query modal should be open")
	}
	if next.queryMode != queryModeJQL {
		t.Fatalf("queryMode = %v, want queryModeJQL", next.queryMode)
	}
	if next.queryJQLDraft != model.jql {
		t.Fatalf("queryJQLDraft = %q, want current JQL %q", next.queryJQLDraft, model.jql)
	}
}

func TestQueryModalAppliesDirectJQLWithExplicitSave(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeTable
	model.view = 1
	model.statusFilter = issueStatusFilterActive
	model.collapsedIssueKeys = map[string]bool{"ABC-1": true}
	model.startQueryModal()
	model.setQueryJQLDraft("project = XYZ AND status = \"In Progress\" ORDER BY updated DESC")
	requestID := model.activeRequestID

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("applying direct JQL should submit a foreground search")
	}
	if next.queryOpen {
		t.Fatal("query modal should close after direct JQL applies")
	}
	if next.jql != "project = XYZ AND status = \"In Progress\" ORDER BY updated DESC" {
		t.Fatalf("jql = %q", next.jql)
	}
	if next.view != -1 {
		t.Fatalf("view = %d, want ad hoc view -1", next.view)
	}
	if next.statusFilter != issueStatusFilterAll {
		t.Fatalf("statusFilter = %v, want all", next.statusFilter)
	}
	if len(next.collapsedIssueKeys) != 0 {
		t.Fatalf("collapsedIssueKeys should reset, got %#v", next.collapsedIssueKeys)
	}
	if !next.loading || next.activeRequestID <= requestID {
		t.Fatalf("expected loading with newer request, loading=%v activeRequestID=%d previous=%d", next.loading, next.activeRequestID, requestID)
	}
}

func TestQueryModalRejectsEmptyDirectJQL(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.startQueryModal()
	model.setQueryJQLDraft("   ")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("empty JQL should not submit work")
	}
	if next.jql != model.jql {
		t.Fatalf("jql changed to %q", next.jql)
	}
	if !next.queryOpen {
		t.Fatal("query modal should stay open")
	}
	if !strings.Contains(next.detailNotice, "JQL cannot be empty") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestHelpIncludesQueryBinding(t *testing.T) {
	bindings := keyBindings(keyContextTable)
	for _, binding := range bindings {
		if len(binding.Keys) == 1 && binding.Keys[0] == "/" && strings.Contains(binding.Description, "JQL") {
			return
		}
	}
	t.Fatal("table help should include / JQL query binding")
}

func TestQueryModalSubmitsAIJQLGeneration(t *testing.T) {
	stream := events.NewStream()
	defer stream.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	received, err := stream.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	runner := &fakeClaudeRunner{result: claude.Result{Text: "JQL: project = ABC AND assignee = currentUser() ORDER BY updated DESC"}}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC ORDER BY updated DESC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
		WithClaudeRunner(runner),
		WithEventStream(stream),
	)
	defer model.workers.Stop()
	model.loading = false
	model.startQueryModal()
	model.queryMode = queryModeAI
	model.setQueryAIPrompt("show my assigned work")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("AI JQL generation should submit an AI command")
	}
	if !next.queryAILoading {
		t.Fatal("query AI should be loading")
	}
	resultMsg := <-runClaudePlanCommandAsyncForTest(cmd)
	result := resultMsg.(queryAIResultMsg)
	if result.err != nil {
		t.Fatalf("AI result error = %v", result.err)
	}

	updated, _ = next.Update(result)
	next = updated.(Model)
	if next.jql != model.jql {
		t.Fatalf("AI result should not run automatically, jql = %q", next.jql)
	}
	if next.queryGeneratedJQL != "project = ABC AND assignee = currentUser() ORDER BY updated DESC" {
		t.Fatalf("queryGeneratedJQL = %q", next.queryGeneratedJQL)
	}
	if !strings.Contains(runner.request.Prompt, "show my assigned work") || !strings.Contains(runner.request.Prompt, model.jql) {
		t.Fatalf("prompt missing request or current JQL:\n%s", runner.request.Prompt)
	}
	taskEvents := collectEventTypesForTest(t, received, events.TypeAITaskRequested, events.TypeAITaskCompleted)
	requested := decodeAITaskPayloadForTest(t, taskEvents[events.TypeAITaskRequested])
	if requested.Operation != events.AIOperationGenerateJQL {
		t.Fatalf("operation = %q, want generate_jql", requested.Operation)
	}
}

func TestQueryModalConfirmsGeneratedJQLWithExplicitSave(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.startQueryModal()
	model.queryMode = queryModeAI
	model.queryGeneratedJQL = "project = ABC AND status = \"To Do\" ORDER BY updated DESC"

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("confirming generated JQL should submit a search")
	}
	if next.jql != model.queryGeneratedJQL {
		t.Fatalf("jql = %q, want generated %q", next.jql, model.queryGeneratedJQL)
	}
	if next.queryOpen {
		t.Fatal("query modal should close after confirming generated JQL")
	}
}

func TestQueryModalRevisionPromptIncludesCurrentPreview(t *testing.T) {
	runner := &fakeClaudeRunner{result: claude.Result{Text: "project = ABC AND priority = High ORDER BY updated DESC"}}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
		WithClaudeRunner(runner),
	)
	defer model.workers.Stop()
	model.loading = false
	model.startQueryModal()
	model.queryMode = queryModeAI
	model.queryGeneratedJQL = "project = ABC AND assignee = currentUser() ORDER BY updated DESC"
	model.setQueryAIPrompt("make it high priority")

	_, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	if cmd == nil {
		t.Fatal("revision should submit AI command")
	}
	<-runClaudePlanCommandAsyncForTest(cmd)
	if !strings.Contains(runner.request.Prompt, model.queryGeneratedJQL) {
		t.Fatalf("revision prompt missing current preview:\n%s", runner.request.Prompt)
	}
}

func TestQueryModalRecordsConfirmedDirectJQLHistory(t *testing.T) {
	store := newFakeActiveViewStore()
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithActiveViewStore(store, "https://example.atlassian.net"),
		WithNow(func() time.Time { return now }),
	)
	defer model.workers.Stop()
	model.loading = false
	model.startQueryModal()
	model.setQueryJQLDraft("project = ABC   ORDER BY updated DESC")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("direct JQL should submit search")
	}
	if store.putQueryHistory.JQL != "project = ABC   ORDER BY updated DESC" {
		t.Fatalf("query history JQL = %q", store.putQueryHistory.JQL)
	}
	if store.putQueryHistory.CacheKey != "project = ABC ORDER BY updated DESC" {
		t.Fatalf("query history cache key = %q", store.putQueryHistory.CacheKey)
	}
	if store.putQueryHistory.Source != cache.QueryHistorySourceDirect {
		t.Fatalf("query history source = %q", store.putQueryHistory.Source)
	}
	if store.putQueryHistory.Prompt != "" {
		t.Fatalf("direct query prompt = %q", store.putQueryHistory.Prompt)
	}
	if !store.putQueryHistory.LastUsedAt.Equal(now) {
		t.Fatalf("LastUsedAt = %s, want %s", store.putQueryHistory.LastUsedAt, now)
	}
	if next.jql != store.putQueryHistory.JQL {
		t.Fatalf("model jql = %q", next.jql)
	}
}

func TestQueryModalRecordsConfirmedAIJQLHistory(t *testing.T) {
	store := newFakeActiveViewStore()
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithActiveViewStore(store, "https://example.atlassian.net"),
	)
	defer model.workers.Stop()
	model.loading = false
	model.startQueryModal()
	model.queryMode = queryModeAI
	model.setQueryAIPrompt("show assigned work")
	model.queryGeneratedPrompt = "show assigned work"
	model.queryGeneratedJQL = "project = ABC AND assignee = currentUser() ORDER BY updated DESC"

	_, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))

	if cmd == nil {
		t.Fatal("confirmed AI JQL should submit search")
	}
	if store.putQueryHistory.Source != cache.QueryHistorySourceAI {
		t.Fatalf("query history source = %q", store.putQueryHistory.Source)
	}
	if store.putQueryHistory.Prompt != "show assigned work" {
		t.Fatalf("query history prompt = %q", store.putQueryHistory.Prompt)
	}
	if store.putQueryHistory.JQL != model.queryGeneratedJQL {
		t.Fatalf("query history JQL = %q", store.putQueryHistory.JQL)
	}
}

func TestQueryModalLoadsPersistedRecentQueries(t *testing.T) {
	store := newFakeActiveViewStore()
	store.queryHistory = []cache.QueryHistoryRecord{
		{JQL: "project = ABC AND assignee = currentUser()", Source: cache.QueryHistorySourceAI, Prompt: "my work"},
		{JQL: "project = ABC ORDER BY updated DESC", Source: cache.QueryHistorySourceDirect},
	}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithActiveViewStore(store, "https://example.atlassian.net"),
	)
	defer model.workers.Stop()
	model.loading = false

	model.startQueryModal()

	if len(model.queryHistory) != 2 {
		t.Fatalf("queryHistory count = %d", len(model.queryHistory))
	}
	if model.queryHistory[0].Prompt != "my work" {
		t.Fatalf("first history prompt = %q", model.queryHistory[0].Prompt)
	}
	if model.queryHistorySelected != 0 {
		t.Fatalf("queryHistorySelected = %d", model.queryHistorySelected)
	}
}

func TestQueryModalRecentSelectionLoadsJQLForReview(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.startQueryModal()
	model.queryMode = queryModeRecent
	model.queryHistory = []cache.QueryHistoryRecord{
		{JQL: "project = ABC ORDER BY updated DESC", Source: cache.QueryHistorySourceDirect},
		{JQL: "project = ABC AND status = \"In Progress\"", Source: cache.QueryHistorySourceAI, Prompt: "active work"},
	}
	model.queryHistorySelected = 1

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("loading recent query for review should not submit work")
	}
	if next.queryMode != queryModeJQL {
		t.Fatalf("queryMode = %v, want JQL", next.queryMode)
	}
	if next.queryJQLDraft != "project = ABC AND status = \"In Progress\"" {
		t.Fatalf("queryJQLDraft = %q", next.queryJQLDraft)
	}
	if !strings.Contains(next.detailNotice, "Recent query loaded") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestQueryModalRecentSelectionRunsSelectedJQL(t *testing.T) {
	store := newFakeActiveViewStore()
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithActiveViewStore(store, "https://example.atlassian.net"),
	)
	defer model.workers.Stop()
	model.loading = false
	model.startQueryModal()
	model.queryMode = queryModeRecent
	model.queryHistory = []cache.QueryHistoryRecord{
		{JQL: "project = ABC ORDER BY updated DESC", Source: cache.QueryHistorySourceDirect},
		{JQL: "project = ABC AND status = \"In Progress\"", Source: cache.QueryHistorySourceAI, Prompt: "active work"},
	}
	model.queryHistorySelected = 1

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("running selected recent query should submit a search")
	}
	if next.jql != "project = ABC AND status = \"In Progress\"" {
		t.Fatalf("jql = %q", next.jql)
	}
	if store.putQueryHistory.Source != cache.QueryHistorySourceAI || store.putQueryHistory.Prompt != "active work" {
		t.Fatalf("query history = %#v", store.putQueryHistory)
	}
	if next.queryOpen {
		t.Fatal("query modal should close after running recent query")
	}
}

func TestQueryModalRecentSaveOpensNamePrompt(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.startQueryModal()
	model.queryMode = queryModeRecent
	model.queryHistory = []cache.QueryHistoryRecord{
		{JQL: "project = ABC AND assignee = currentUser()", Source: cache.QueryHistorySourceAI, Prompt: "my work"},
	}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "s", Code: 's'}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("opening saved-view prompt should not submit work")
	}
	if !next.querySaveViewOpen {
		t.Fatal("querySaveViewOpen should be true")
	}
	if next.querySaveViewNameValue() != "my work" {
		t.Fatalf("default save name = %q", next.querySaveViewNameValue())
	}
}

func TestQueryModalRecentSavePersistsNamedViewWithoutRunningQuery(t *testing.T) {
	var saved config.IssueView
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithViews([]config.IssueView{{Name: "Assigned", JQL: "assignee = currentUser()"}}, "Assigned"),
		WithSavedViewWriter(func(view config.IssueView) error {
			saved = view
			return nil
		}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.startQueryModal()
	model.queryMode = queryModeRecent
	model.queryHistory = []cache.QueryHistoryRecord{
		{JQL: "project = ABC AND status = \"In Progress\"", Source: cache.QueryHistorySourceDirect},
	}
	model.querySaveViewOpen = true
	model.querySaveViewJQL = "project = ABC AND status = \"In Progress\""
	model.setQuerySaveViewName("Active Work")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("saving a named view should not submit Jira work")
	}
	if saved.Name != "Active Work" || saved.JQL != "project = ABC AND status = \"In Progress\"" {
		t.Fatalf("saved view = %#v", saved)
	}
	if len(next.views) != 2 || next.views[1].Name != "Active Work" {
		t.Fatalf("views = %#v", next.views)
	}
	if next.jql != model.jql || next.view != model.view {
		t.Fatalf("save should not change active query/view, jql=%q view=%d", next.jql, next.view)
	}
	if next.querySaveViewOpen {
		t.Fatal("save prompt should close after success")
	}
	if !strings.Contains(next.detailNotice, "Saved view") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestQueryModalRecentSaveRejectsDuplicateViewName(t *testing.T) {
	called := false
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithViews([]config.IssueView{{Name: "Assigned", JQL: "assignee = currentUser()"}}, "Assigned"),
		WithSavedViewWriter(func(view config.IssueView) error {
			called = true
			return nil
		}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.startQueryModal()
	model.queryMode = queryModeRecent
	model.queryHistory = []cache.QueryHistoryRecord{
		{JQL: "project = ABC AND status = \"In Progress\"", Source: cache.QueryHistorySourceDirect},
	}
	model.querySaveViewOpen = true
	model.querySaveViewJQL = "project = ABC AND status = \"In Progress\""
	model.setQuerySaveViewName(" assigned ")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("duplicate save should not submit work")
	}
	if called {
		t.Fatal("writer should not be called for duplicate name")
	}
	if !next.querySaveViewOpen {
		t.Fatal("save prompt should stay open")
	}
	if !strings.Contains(next.detailNotice, "already exists") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestTableViewKeySavesCurrentQueryAsViewWithoutRunningQuery(t *testing.T) {
	var saved config.IssueView
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC AND issuetype = Epic ORDER BY updated DESC",
		WithViews([]config.IssueView{
			{Name: "Epics", JQL: "project = ABC AND issuetype = Epic ORDER BY updated DESC", IncludeChildren: true},
		}, "Epics"),
		WithSavedViewWriter(func(view config.IssueView) error {
			saved = view
			return nil
		}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeTable

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "v", Code: 'v'}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("opening current-query save prompt should not submit work")
	}
	if !next.queryOpen || !next.querySaveViewOpen {
		t.Fatalf("queryOpen=%v querySaveViewOpen=%v, want both true", next.queryOpen, next.querySaveViewOpen)
	}
	if next.querySaveViewJQL != model.jql {
		t.Fatalf("querySaveViewJQL = %q, want current JQL %q", next.querySaveViewJQL, model.jql)
	}
	if !next.querySaveViewIncludeChildren {
		t.Fatal("current saved view include_children should seed the save prompt")
	}

	next.setQuerySaveViewName("Epic Copy")
	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)

	if cmd != nil {
		t.Fatal("saving current query should not submit Jira work")
	}
	if saved.Name != "Epic Copy" || saved.JQL != model.jql || !saved.IncludeChildren {
		t.Fatalf("saved view = %#v", saved)
	}
	if next.jql != model.jql || next.view != model.view {
		t.Fatalf("save should not change active query/view, jql=%q view=%d", next.jql, next.view)
	}
}

func TestQueryModalDirectSavePersistsDraftWithoutRunningQuery(t *testing.T) {
	var saved config.IssueView
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithSavedViewWriter(func(view config.IssueView) error {
			saved = view
			return nil
		}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.startQueryModal()
	model.setQueryJQLDraft("project = ABC AND status = \"In Progress\" ORDER BY updated DESC")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+v"}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("opening direct-save prompt should not submit work")
	}
	if !next.querySaveViewOpen {
		t.Fatal("direct save prompt should open")
	}
	if next.querySaveViewJQL != "project = ABC AND status = \"In Progress\" ORDER BY updated DESC" {
		t.Fatalf("querySaveViewJQL = %q", next.querySaveViewJQL)
	}

	next.setQuerySaveViewName("Active Work")
	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)

	if cmd != nil {
		t.Fatal("saving direct draft should not submit Jira work")
	}
	if saved.Name != "Active Work" || saved.JQL != "project = ABC AND status = \"In Progress\" ORDER BY updated DESC" {
		t.Fatalf("saved view = %#v", saved)
	}
	if next.jql != model.jql {
		t.Fatalf("active JQL changed to %q", next.jql)
	}
}

func TestQueryModalAIPreviewSavePersistsGeneratedJQL(t *testing.T) {
	var saved config.IssueView
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithSavedViewWriter(func(view config.IssueView) error {
			saved = view
			return nil
		}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.startQueryModal()
	model.queryMode = queryModeAI
	model.queryGeneratedPrompt = "my bugs"
	model.queryGeneratedJQL = "project = ABC AND issuetype = Bug ORDER BY updated DESC"

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+v"}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("opening AI preview save prompt should not submit work")
	}
	if next.querySaveViewNameValue() != "my bugs" {
		t.Fatalf("default name = %q", next.querySaveViewNameValue())
	}

	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)

	if cmd != nil {
		t.Fatal("saving AI preview should not submit Jira work")
	}
	if saved.Name != "my bugs" || saved.JQL != model.queryGeneratedJQL {
		t.Fatalf("saved view = %#v", saved)
	}
	if next.queryOpen == false {
		t.Fatal("saving AI preview should keep query modal open")
	}
}

func TestQueryModalTemplateSavePersistsSelectedTemplate(t *testing.T) {
	var saved config.IssueView
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC ORDER BY updated DESC",
		WithSavedViewWriter(func(view config.IssueView) error {
			saved = view
			return nil
		}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.startQueryModal()
	model.queryMode = queryModeTemplates
	model.queryTemplateSelected = 2

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "s", Code: 's'}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("opening template save prompt should not submit work")
	}
	if !next.querySaveViewOpen {
		t.Fatal("template save prompt should open")
	}
	if next.querySaveViewNameValue() != "Project Open" {
		t.Fatalf("default template name = %q", next.querySaveViewNameValue())
	}

	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))

	if cmd != nil {
		t.Fatal("saving template should not submit Jira work")
	}
	if saved.Name != "Project Open" || saved.JQL != "project = ABC AND resolution = Unresolved ORDER BY updated DESC" {
		t.Fatalf("saved template = %#v", saved)
	}
}

func TestQueryModalViewManagerRenamesReordersTogglesAndDeletesViews(t *testing.T) {
	var writes [][]config.IssueView
	var activeWrites []string
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithViews([]config.IssueView{
			{Name: "Assigned", JQL: "assignee = currentUser()"},
			{Name: "Project", JQL: "project = ABC"},
			{Name: "Epics", JQL: "issuetype = Epic"},
		}, "Assigned"),
		WithSavedViewsWriter(func(views []config.IssueView, activeView string) error {
			writes = append(writes, append([]config.IssueView(nil), views...))
			activeWrites = append(activeWrites, activeView)
			return nil
		}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.startQueryModal()
	model.queryMode = queryModeViews
	model.queryViewSelected = 1

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "r", Code: 'r'}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("opening rename prompt should not submit work")
	}
	if !next.querySaveViewOpen || next.querySaveViewNameValue() != "Project" {
		t.Fatalf("rename prompt state name=%q open=%v", next.querySaveViewNameValue(), next.querySaveViewOpen)
	}
	next.setQuerySaveViewName("Project Open")
	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)
	if cmd != nil {
		t.Fatal("renaming view should not submit Jira work")
	}
	if next.views[1].Name != "Project Open" || writes[len(writes)-1][1].Name != "Project Open" {
		t.Fatalf("views=%#v writes=%#v", next.views, writes)
	}

	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "K", Code: 'K'}))
	next = updated.(Model)
	if cmd != nil {
		t.Fatal("reordering view should not submit Jira work")
	}
	if next.views[0].Name != "Project Open" || next.queryViewSelected != 0 {
		t.Fatalf("reordered views=%#v selected=%d", next.views, next.queryViewSelected)
	}
	if next.view != 1 || activeWrites[len(activeWrites)-1] != "Assigned" {
		t.Fatalf("active view changed after reorder: view=%d activeWrites=%#v", next.view, activeWrites)
	}

	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "i", Code: 'i'}))
	next = updated.(Model)
	if cmd != nil {
		t.Fatal("toggling view metadata should not submit Jira work")
	}
	if !next.views[0].IncludeChildren {
		t.Fatalf("include_children was not toggled: %#v", next.views[0])
	}

	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "d", Code: 'd'}))
	next = updated.(Model)
	if cmd != nil {
		t.Fatal("deleting view should not submit Jira work")
	}
	if len(next.views) != 2 || next.views[0].Name == "Project Open" {
		t.Fatalf("delete did not remove selected view: %#v", next.views)
	}
	if len(writes) != 4 {
		t.Fatalf("writes count = %d, want 4", len(writes))
	}
}
