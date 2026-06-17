package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jon/jira-tui/internal/claude"
	"github.com/jon/jira-tui/internal/events"
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
