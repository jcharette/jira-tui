package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jcharette/jira-tui/internal/claude"
	"github.com/jcharette/jira-tui/internal/events"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/jcharette/jira-tui/internal/worker"
)

func TestClaudeSectionRequiresEnabledAvailableTicketPlan(t *testing.T) {
	base := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer base.workers.Stop()
	base.mode = modeDetail
	base.issues = []jira.Issue{{Key: "ABC-1", Summary: "Plan this", Status: "To Do"}}

	if hasDetailSection(base, "claude") {
		t.Fatal("Claude section should be hidden by default")
	}

	enabled := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer enabled.workers.Stop()
	enabled.mode = modeDetail
	enabled.issues = base.issues

	if !hasDetailSection(enabled, "claude") {
		t.Fatal("Claude section should be visible when enabled, available, and ticket_plan is true")
	}
}

func TestClaudeSectionVisibleWithTicketAssistOnly(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Sanitize this", Status: "To Do"}}
	model.jumpDetailSection("Claude")

	if !hasDetailSection(model, "claude") {
		t.Fatal("Claude section should be visible when ticket_assist is enabled")
	}
	view := model.render()
	for _, want := range []string{"Ticket Assist", "whole-ticket draft", "subtask recommendations"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestClaudeSectionShowsQualityReviewAndDraftComment(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.mode = modeDetail
	issue := jira.Issue{Key: "ABC-1", Summary: "Unclear rollout"}
	model.details = map[string]jira.IssueDetail{"ABC-1": {Issue: issue}}

	view := model.renderClaudeSection(detailRenderContext{selected: issue, display: issue}, 100)

	for _, want := range []string{"Quality Review", "Draft Comment"} {
		if !strings.Contains(view, want) {
			t.Fatalf("Claude section missing %q:\n%s", want, view)
		}
	}
}

func TestClaudeQualityReviewPromptIncludesTicketReviewSections(t *testing.T) {
	model := newInlineDescriptionAIModel(t)
	ctx, ok := model.detailRenderContext()
	if !ok {
		t.Fatal("expected detail context")
	}

	prompt := model.buildClaudeQualityReviewPrompt(ctx)

	for _, want := range []string{"Acceptance Criteria", "Open Questions", "Recent Comments", "Do not update Jira"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestClaudeDraftCommentPromptIncludesRecentComments(t *testing.T) {
	model := newInlineDescriptionAIModel(t)
	ctx, ok := model.detailRenderContext()
	if !ok {
		t.Fatal("expected detail context")
	}

	prompt := model.buildClaudeDraftCommentPrompt(ctx)

	for _, want := range []string{"Jira comment draft", "Recent Comments", "Please clarify done.", "Return only"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestInlineAIPickerCanCancel(t *testing.T) {
	model := newInlineDescriptionAIModel(t)
	model.jumpDetailSection("Description")
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "a"}))
	model = updated.(Model)

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "esc"}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("cancel should not start work")
	}
	if next.inlineAIOpen {
		t.Fatal("expected inline AI picker closed")
	}
}

func TestInlineAIImproveTicketStartsGuidedTicketAssist(t *testing.T) {
	stream := events.NewStream()
	defer stream.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	received, err := stream.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	runner := &fakeClaudeRunner{result: claude.Result{Text: "Review\n- Description is vague\n\nDraft\nSummary: Clearer ticket"}}
	model := newInlineDescriptionAIModel(t)
	model.eventStream = stream
	model.claudeRunner = runner
	model.jumpDetailSection("Description")
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "a"}))
	model = updated.(Model)

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if cmd == nil {
		t.Fatal("expected Claude command")
	}
	if !next.claudeAssistLoading || !next.claudeAssistOpen {
		t.Fatal("expected Claude assist loading modal")
	}

	resultMsg := <-runClaudePlanCommandAsyncForTest(cmd)
	result := resultMsg.(claudeAssistResultMsg)
	if result.err != nil {
		t.Fatalf("Claude result error = %v", result.err)
	}
	taskEvents := collectEventTypesForTest(t, received, events.TypeAITaskRequested, events.TypeAITaskCompleted)
	requested := decodeAITaskPayloadForTest(t, taskEvents[events.TypeAITaskRequested])
	if requested.Operation != events.AIOperationTicketAssist || requested.IssueKey != "ABC-1" {
		t.Fatalf("requested payload = %#v", requested)
	}
	for _, want := range []string{"guided drafting session", "Subtask Recommendations", "Do not update Jira", "ABC-1", "Current unclear description", "Please clarify done."} {
		if !strings.Contains(runner.request.Prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, runner.request.Prompt)
		}
	}
}

func TestInlineDescriptionAIAskQuestionUsesInstruction(t *testing.T) {
	runner := &fakeClaudeRunner{result: claude.Result{Text: "Review\n- Answered question\n\nDraft\nThe missing scope is deployment rollback."}}
	model := newInlineDescriptionAIModel(t)
	model.claudeRunner = runner
	model.jumpDetailSection("Description")
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "a"}))
	model = updated.(Model)
	model.selectedInlineAIAction = 2

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	model = updated.(Model)
	if cmd != nil || !model.inlineAIInstructionOpen {
		t.Fatal("expected local instruction editor before Claude")
	}
	model.inlineAIInstruction = "What is missing from this ticket?"
	model.inlineAIInstructionEditor = newClaudeAssistRefineEditor(model.inlineAIInstruction)
	model.inlineAIInstructionReady = true

	updated, cmd = model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	if cmd == nil {
		t.Fatal("expected Claude command")
	}
	<-runClaudePlanCommandAsyncForTest(cmd)
	if !strings.Contains(runner.request.Prompt, "What is missing from this ticket?") {
		t.Fatalf("prompt missing question:\n%s", runner.request.Prompt)
	}
}

func newInlineDescriptionAIModel(t *testing.T) Model {
	t.Helper()
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second, AllowJiraWrites: true, RequireConfirmation: true}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	model.loading = false
	model.mode = modeDetail
	model.width = 130
	model.height = 42
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Improve this", Status: "To Do", Priority: "P2", Assignee: "Jon"}}
	model.details = map[string]jira.IssueDetail{"ABC-1": {Issue: model.issues[0], Description: "Current unclear description", Reporter: "Rae"}}
	model.comments = map[string][]jira.Comment{"ABC-1": {{Author: "Rae", Body: "Please clarify done."}}}
	t.Cleanup(model.workers.Stop)
	return model
}

func TestClaudeSectionSubmitsTicketPlanWithSelectedContext(t *testing.T) {
	runner := &fakeClaudeRunner{result: claude.Result{Text: "Implementation plan"}}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "/usr/local/bin/claude"}),
		WithClaudeRunner(runner),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "To Do", Priority: "High", Assignee: "Jon"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       model.issues[0],
			Description: "The deployment fails during migration.",
		},
	}
	model.comments = map[string][]jira.Comment{
		"ABC-1": {{Author: "Rae", Body: "Please include tests."}},
	}
	model.jumpDetailSection("Claude")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if !next.claudePlanLoading {
		t.Fatal("expected Claude plan loading")
	}
	if cmd == nil {
		t.Fatal("expected Claude plan command")
	}

	resultMsg := <-runClaudePlanCommandAsyncForTest(cmd)
	result, ok := resultMsg.(claudePlanResultMsg)
	if !ok {
		t.Fatalf("message = %#v", resultMsg)
	}
	if result.err != nil {
		t.Fatalf("Claude result error = %v", result.err)
	}
	if !strings.Contains(runner.request.Prompt, "ABC-1") ||
		!strings.Contains(runner.request.Prompt, "Fix production thing") ||
		!strings.Contains(runner.request.Prompt, "The deployment fails during migration.") ||
		!strings.Contains(runner.request.Prompt, "Please include tests.") {
		t.Fatalf("prompt missing ticket context:\n%s", runner.request.Prompt)
	}

	updated, _ = next.Update(result)
	next = updated.(Model)
	view := next.render()
	for _, want := range []string{"Claude Ticket Plan", "Implementation plan", "esc close"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	var sawSubmit bool
	var sawResult bool
	for _, event := range next.diagnosticsEvents {
		if event.Kind != diagnosticKindClaude || event.Label != "ticket_plan" {
			continue
		}
		if event.Status == "submit" && strings.Contains(event.Detail, "ABC-1") {
			sawSubmit = true
		}
		if event.Status == "ok" && strings.Contains(event.Detail, "ABC-1") {
			sawResult = true
		}
	}
	if !sawSubmit || !sawResult {
		t.Fatalf("missing Claude diagnostics submit=%t result=%t events=%#v", sawSubmit, sawResult, next.diagnosticsEvents)
	}
}

func TestClaudeTicketPlanPublishesProviderNeutralAIEvents(t *testing.T) {
	stream := events.NewStream()
	defer stream.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	received, err := stream.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	runner := &fakeClaudeRunner{result: claude.Result{Text: "Implementation plan"}}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithEventStream(stream),
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "/usr/local/bin/claude"}),
		WithClaudeRunner(runner),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "To Do"}}
	model.jumpDetailSection("Claude")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	resultMsg := <-runClaudePlanCommandAsyncForTest(cmd)
	result, ok := resultMsg.(claudePlanResultMsg)
	if !ok {
		t.Fatalf("message = %#v", resultMsg)
	}
	if result.err != nil {
		t.Fatalf("Claude result error = %v", result.err)
	}

	taskEvents := collectEventTypesForTest(t, received, events.TypeAITaskRequested, events.TypeAITaskCompleted)
	requestedEvent := taskEvents[events.TypeAITaskRequested]
	requested := decodeAITaskPayloadForTest(t, requestedEvent)
	if requested.Operation != events.AIOperationTicketPlan ||
		requested.PreferredProvider != events.AIProviderAuto ||
		requested.Provider != events.AIProviderClaude ||
		requested.IssueKey != "ABC-1" ||
		requested.RequestID != next.activeClaudePlanReqID ||
		requested.PromptBytes == 0 {
		t.Fatalf("requested payload = %#v", requested)
	}
	completedEvent := taskEvents[events.TypeAITaskCompleted]
	completed := decodeAITaskPayloadForTest(t, completedEvent)
	if completed.Operation != events.AIOperationTicketPlan ||
		completed.Provider != events.AIProviderClaude ||
		completed.IssueKey != "ABC-1" ||
		completed.ResultBytes != len("Implementation plan") {
		t.Fatalf("completed payload = %#v", completed)
	}
}

func TestClaudeTicketPlanPublishesAIProgressEvents(t *testing.T) {
	stream := events.NewStream()
	defer stream.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	received, err := stream.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	runner := &fakeClaudeRunner{
		result: claude.Result{Text: "Implementation plan"},
		events: []claude.Event{{Kind: "stderr", Text: "receiving response"}},
	}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithEventStream(stream),
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "/usr/local/bin/claude"}),
		WithClaudeRunner(runner),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "To Do"}}
	model.jumpDetailSection("Claude")

	_, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	resultMsg := <-runClaudePlanCommandAsyncForTest(cmd)
	if result, ok := resultMsg.(claudePlanResultMsg); !ok || result.err != nil {
		t.Fatalf("result = %#v", resultMsg)
	}

	taskEvents := collectEventTypesForTest(t, received, events.TypeAITaskRequested, events.TypeAITaskProgress, events.TypeAITaskCompleted)
	progressEvent := taskEvents[events.TypeAITaskProgress]
	progress := decodeAITaskPayloadForTest(t, progressEvent)
	if progress.Operation != events.AIOperationTicketPlan ||
		progress.ProgressKind != "stderr" ||
		progress.ProgressBytes != len("receiving response") {
		t.Fatalf("progress payload = %#v", progress)
	}
	if data, err := json.Marshal(progress); err != nil || strings.Contains(string(data), "receiving response") {
		t.Fatalf("progress payload leaked raw text: %s err=%v", data, err)
	}
}

func TestClaudeTicketPlanPublishesAIFailedEvents(t *testing.T) {
	stream := events.NewStream()
	defer stream.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	received, err := stream.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	runner := &fakeClaudeRunner{err: errors.New("provider failed")}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithEventStream(stream),
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "/usr/local/bin/claude"}),
		WithClaudeRunner(runner),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "To Do"}}
	model.jumpDetailSection("Claude")

	_, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	resultMsg := <-runClaudePlanCommandAsyncForTest(cmd)
	if result, ok := resultMsg.(claudePlanResultMsg); !ok || result.err == nil {
		t.Fatalf("result = %#v", resultMsg)
	}

	taskEvents := collectEventTypesForTest(t, received, events.TypeAITaskRequested, events.TypeAITaskFailed)
	failedEvent := taskEvents[events.TypeAITaskFailed]
	failed := decodeAITaskPayloadForTest(t, failedEvent)
	if failed.Operation != events.AIOperationTicketPlan ||
		failed.Provider != events.AIProviderClaude ||
		!strings.Contains(failed.Error, "provider failed") {
		t.Fatalf("failed payload = %#v", failed)
	}
}

func decodeAITaskPayloadForTest(t *testing.T, event events.Event) events.AITaskPayload {
	t.Helper()
	var payload events.AITaskPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("decode AI task payload: %v", err)
	}
	return payload
}

func collectEventTypesForTest(t *testing.T, received <-chan events.Event, eventTypes ...events.Type) map[events.Type]events.Event {
	t.Helper()
	wanted := make(map[events.Type]struct{}, len(eventTypes))
	for _, eventType := range eventTypes {
		wanted[eventType] = struct{}{}
	}
	got := make(map[events.Type]events.Event, len(eventTypes))
	deadline := time.After(time.Second)
	for {
		select {
		case event := <-received:
			if _, ok := wanted[event.Type]; ok {
				got[event.Type] = event
				if len(got) == len(wanted) {
					return got
				}
			}
		case <-deadline:
			t.Fatalf("timed out waiting for event types %v, got %#v", eventTypes, got)
		}
	}
}

func TestClaudeSectionSubmitsTicketAssistWithSelectedContext(t *testing.T) {
	runner := &fakeClaudeRunner{result: claude.Result{Text: "Review\n- Missing acceptance criteria\n\nDraft\nSummary: Better summary\n\nAcceptance Criteria\n- [ ] User can edit this before Jira changes."}}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "/usr/local/bin/claude"}),
		WithClaudeRunner(runner),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix unclear deployment story", Status: "To Do", Priority: "High", Assignee: "Jon"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {
			Issue:       model.issues[0],
			Description: "Need to do deploy stuff.",
		},
	}
	model.issues = append(model.issues,
		jira.Issue{Key: "ABC-2", Summary: "Install platform controllers", Status: "In Progress", IssueType: "Sub-task", ParentKey: "ABC-1", IsSubtask: true, Assignee: "Rae"},
		jira.Issue{Key: "ABC-3", Summary: "Automate Helm releases", Status: "To Do", IssueType: "Task", ParentKey: "ABC-1", Assignee: "Jon"},
	)
	model.comments = map[string][]jira.Comment{
		"ABC-1": {{Author: "Rae", Body: "What does done mean?"}},
	}
	model.jumpDetailSection("Claude")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if !next.claudeAssistLoading {
		t.Fatal("expected Claude assist loading")
	}
	if cmd == nil {
		t.Fatal("expected Claude assist command")
	}

	resultMsg := <-runClaudePlanCommandAsyncForTest(cmd)
	result, ok := resultMsg.(claudeAssistResultMsg)
	if !ok {
		t.Fatalf("message = %#v", resultMsg)
	}
	if result.err != nil {
		t.Fatalf("Claude assist result error = %v", result.err)
	}
	for _, want := range []string{
		"Evaluate and improve this existing Jira ticket as a guided drafting session",
		"Do not update Jira",
		"Acceptance Criteria",
		"Subtask Recommendations",
		"Loaded hierarchy",
		"Subtasks",
		"ABC-2",
		"Install platform controllers",
		"Children",
		"ABC-3",
		"Automate Helm releases",
		"ABC-1",
		"Need to do deploy stuff.",
		"What does done mean?",
	} {
		if !strings.Contains(runner.request.Prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, runner.request.Prompt)
		}
	}

	updated, _ = next.Update(result)
	next = updated.(Model)
	view := next.render()
	for _, want := range []string{"Claude Ticket Assist", "Review", "Missing acceptance criteria", "Local Draft", "Acceptance Criteria", "ctrl+r refine"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	if next.claudeAssistDraftValue() == "" {
		t.Fatal("expected editable Claude assist draft")
	}
}

func TestClaudeTicketAssistResultExtractsOpenQuestions(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 130
	model.height = 42
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Original summary", Status: "To Do"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.activeClaudeAssistReqID = 7
	model.claudeAssistLoading = true

	updated, _ := model.Update(claudeAssistResultMsg{
		id:  7,
		key: "ABC-1",
		text: strings.Join([]string{
			"Review",
			"- Needs sharper scope",
			"",
			"Draft",
			"Summary: Better ticket",
			"",
			"Acceptance Criteria",
			"- [ ] Clear and testable",
			"",
			"Open Questions",
			"- Which controllers are in the platform baseline?",
			"- Is Helm deployment required for day one?",
			"",
			"Subtask Recommendations",
			"- Add: Document controller ownership",
		}, "\n"),
	})
	next := updated.(Model)

	if len(next.claudeAssistQuestions) != 2 {
		t.Fatalf("questions = %#v", next.claudeAssistQuestions)
	}
	if next.claudeAssistQuestions[0].Question != "Which controllers are in the platform baseline?" {
		t.Fatalf("question[0] = %#v", next.claudeAssistQuestions[0])
	}
	view := next.render()
	for _, want := range []string{"Open Questions", "> Which controllers are in the platform baseline?", "enter answer", "ctrl+r refine with answers"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in:\n%s", want, view)
		}
	}
}

func TestClaudeTicketAssistAnswersQuestionsAndRefinesWithFeedback(t *testing.T) {
	runner := &fakeClaudeRunner{result: claude.Result{Text: "Review\n- Used answers\n\nDraft\nSummary: Refined ticket"}}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
		WithClaudeRunner(runner),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 130
	model.height = 42
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Original summary", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0], Description: "Original description."},
	}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistDraft = "Summary: Better ticket\n\nAcceptance Criteria\n- [ ] Clear"
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true
	model.claudeAssistQuestions = []createAIQuestion{
		{Question: "Which controllers are in the platform baseline?"},
		{Question: "Is Helm deployment required for day one?", Answer: "Yes, for the core chart set."},
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if !next.claudeAssistQuestionAnswering {
		t.Fatal("expected answer editor to open")
	}
	updated, _ = next.Update(tea.PasteMsg{Content: "external-dns and cert-manager"})
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)
	if next.claudeAssistQuestionAnswering {
		t.Fatal("expected answer editor to close")
	}
	if next.claudeAssistQuestions[0].Answer != "external-dns and cert-manager" {
		t.Fatalf("answer = %q", next.claudeAssistQuestions[0].Answer)
	}

	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+r"}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected refinement command")
	}
	resultMsg := <-runClaudePlanCommandAsyncForTest(cmd)
	if _, ok := resultMsg.(claudeAssistResultMsg); !ok {
		t.Fatalf("message = %#v", resultMsg)
	}
	for _, want := range []string{
		"Refine this Jira ticket draft",
		"User answers to Open Questions",
		"Q: Which controllers are in the platform baseline?",
		"A: external-dns and cert-manager",
		"Q: Is Helm deployment required for day one?",
		"A: Yes, for the core chart set.",
	} {
		if !strings.Contains(runner.request.Prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, runner.request.Prompt)
		}
	}
}

func TestClaudeTicketAssistQuestionsDoNotBlockApplyShortcut(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second, AllowJiraWrites: true, RequireConfirmation: true}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 130
	model.height = 42
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Original summary", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{"ABC-1": {Issue: model.issues[0], Description: "Old description"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistTarget = claudeAssistTargetTicket
	model.claudeAssistDraft = "Summary: Clearer ticket\n\nDescription: Better scope."
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true
	model.claudeAssistQuestions = []createAIQuestion{{Question: "Which controllers are required?"}}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("expected confirmation before applying")
	}
	if !next.claudeAssistConfirmApply {
		t.Fatal("expected apply confirmation")
	}
}

func TestClaudeTicketAssistQuestionAnswerSelectionCanBeDeleted(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 130
	model.height = 42
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Original summary", Status: "To Do"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistDraft = "Summary: Better ticket"
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true
	model.claudeAssistQuestions = []createAIQuestion{{Question: "Which controllers are required?", Answer: "external-dns and cert-manager"}}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	next.claudeAssistQuestionEditor.MoveToBegin()

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+space"}))
	next = updated.(Model)
	for range len("external-dns") {
		updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "right", Code: tea.KeyRight}))
		next = updated.(Model)
	}
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "backspace", Code: tea.KeyBackspace}))
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)

	if got := next.claudeAssistQuestions[0].Answer; got != "and cert-manager" {
		t.Fatalf("answer = %q", got)
	}
}

func TestClaudeTicketAssistDraftIsBoundedAndPaged(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Sanitize this", Status: "To Do"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistText = "Review\n" + strings.Repeat("- Review finding with detail.\n", 20) + "\nDraft\n" + strings.Repeat("Acceptance criterion line with enough detail to wrap cleanly.\n", 40)
	model.claudeAssistDraft = claudeAssistDraftFromText(model.claudeAssistText)
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	view := model.render()
	if len(strings.Split(view, "\n")) > model.height {
		t.Fatalf("view height exceeded terminal height:\n%s", view)
	}
	for _, want := range []string{"Local Draft", "Draft Lines 1-", "pgup/pgdn page", "ctrl+y copy"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "pgdown", Code: tea.KeyPgDown}))
	next := updated.(Model)
	if next.claudeAssistEditor.ScrollYOffset() == 0 {
		t.Fatal("expected pgdown to scroll the Claude assist draft editor")
	}
}

func TestClaudeTicketAssistDraftGetsPrimaryModalSpace(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 150
	model.height = 64
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Sanitize this", Status: "To Do"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistText = "Review\n" + strings.Repeat("- Review finding with detail.\n", 12) + "\nDraft\n" + strings.Repeat("Acceptance criterion line with enough detail to wrap cleanly.\n", 40)
	model.claudeAssistDraft = claudeAssistDraftFromText(model.claudeAssistText)
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	if rows := model.claudeAssistEditorRows(); rows <= 10 {
		t.Fatalf("expected draft editor to get more than old 10-row cap, rows=%d", rows)
	}
	view := model.render()
	if !strings.Contains(view, "Local Draft") {
		t.Fatalf("expected local draft label in %q", view)
	}
	if len(strings.Split(view, "\n")) > model.height {
		t.Fatalf("view height exceeded terminal height:\n%s", view)
	}
}

func TestClaudeTicketAssistOutputHasDistinctZonesAndActions(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second, AllowJiraWrites: true, RequireConfirmation: true}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 150
	model.height = 48
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Sanitize this", Status: "To Do"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistText = "Review\n- Claude found stale context.\n\nDraft\nSummary: Better ticket\n\nAcceptance Criteria\n- [ ] Clear and testable"
	model.claudeAssistDraft = claudeAssistDraftFromText(model.claudeAssistText)
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	view := model.render()
	for _, want := range []string{"Claude Review", "Local Draft", "Not Applied", "Available Actions", "ctrl+s apply", "ctrl+c comment", "ctrl+r refine", "ctrl+y copy"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestInlineDescriptionAIResultUsesTicketAssistModal(t *testing.T) {
	model := newInlineDescriptionAIModel(t)
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistTarget = claudeAssistTargetDescription
	model.claudeAssistText = "Review\n- Description was unclear\n\nDraft\nA clearer replacement description."
	model.claudeAssistDraft = claudeAssistDraftFromText(model.claudeAssistText)
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	view := model.render()
	for _, want := range []string{"Claude Ticket Assist", "Claude Review", "Local Draft", "Available Actions", "ctrl+s apply", "ctrl+c comment", "ctrl+r refine"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestInlineDescriptionAIApplyUpdatesDescriptionOnly(t *testing.T) {
	searcher := &fakeIssueSearcher{}
	model := NewModel(
		searcher,
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second, AllowJiraWrites: true, RequireConfirmation: true}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 130
	model.height = 42
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Original summary", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{"ABC-1": {Issue: model.issues[0], Description: "Old description"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistTarget = claudeAssistTargetDescription
	model.claudeAssistDraft = "Replacement description only."
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("expected confirmation before applying")
	}
	if !next.claudeAssistConfirmApply {
		t.Fatal("expected apply confirmation")
	}
	view := next.render()
	if !strings.Contains(view, "Apply Description Draft") {
		t.Fatalf("missing Description apply confirmation:\n%s", view)
	}
	if strings.Contains(view, "Apply Ticket Assist Draft") {
		t.Fatalf("inline Description apply should not use ticket apply confirmation:\n%s", view)
	}

	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected description update command")
	}
	msg := cmd()
	if _, ok := msg.(workSubmittedMsg); !ok {
		t.Fatalf("submit message = %#v", msg)
	}
	resultMsg := next.waitForWorkerResult()()
	result := resultMsg.(workerResultMsg)
	updated, _ = next.Update(result)
	if searcher.updateSummaryValue != "" {
		t.Fatalf("summary should not be updated, got %q", searcher.updateSummaryValue)
	}
	if searcher.updateDescriptionValue != "Replacement description only." {
		t.Fatalf("description = %q", searcher.updateDescriptionValue)
	}
}

func TestInlineDescriptionAIDraftCanBePostedAsComment(t *testing.T) {
	searcher := &fakeIssueSearcher{comments: []jira.Comment{{ID: "1", Author: "Current User", Body: "posted"}}}
	model := NewModel(
		searcher,
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second, AllowJiraWrites: true, RequireConfirmation: true}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 130
	model.height = 42
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Original summary", Status: "To Do"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistTarget = claudeAssistTargetDescription
	model.claudeAssistDraft = "Could you confirm the rollback requirement?"
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+c"}))
	next := updated.(Model)
	if !next.claudeAssistConfirmComment {
		t.Fatal("expected comment confirmation")
	}
	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected AddComment command")
	}
	_ = cmd()
	resultMsg := next.waitForWorkerResult()()
	result := resultMsg.(workerResultMsg)
	_, _ = next.Update(result)
	if searcher.addedBody != "Could you confirm the rollback requirement?" {
		t.Fatalf("comment body = %q", searcher.addedBody)
	}
}

func TestClaudeTicketAssistDraftCanBePostedAsComment(t *testing.T) {
	searcher := &fakeIssueSearcher{comments: []jira.Comment{{ID: "10001", Author: "Current User", Body: "posted"}}}
	model := NewModel(
		searcher,
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second, AllowJiraWrites: true, RequireConfirmation: true}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 36
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Original summary", Status: "To Do"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistDraft = "Summary: Better ticket\n\nAcceptance Criteria\n- Clear and testable"
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+c"}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("expected confirmation before posting comment")
	}
	if !next.claudeAssistConfirmComment {
		t.Fatal("expected comment confirmation")
	}
	if view := next.render(); !strings.Contains(view, "Post Draft As Comment") || !strings.Contains(view, "ctrl+s post") {
		t.Fatalf("missing comment confirmation in %q", view)
	}

	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected comment post command")
	}
	if !next.claudeAssistPostingComment {
		t.Fatal("expected posting comment state")
	}
	msg := cmd()
	if _, ok := msg.(workSubmittedMsg); !ok {
		t.Fatalf("submit message = %#v", msg)
	}
	resultMsg := next.waitForWorkerResult()()
	result, ok := resultMsg.(workerResultMsg)
	if !ok {
		t.Fatalf("worker message = %#v", resultMsg)
	}
	updated, cmd = next.Update(result)
	next = updated.(Model)
	if searcher.addedBody != model.claudeAssistDraft {
		t.Fatalf("posted body = %q", searcher.addedBody)
	}
	if next.claudeAssistOpen || next.claudeAssistPostingComment {
		t.Fatalf("expected modal closed after comment, open=%v posting=%v", next.claudeAssistOpen, next.claudeAssistPostingComment)
	}
	if !strings.Contains(next.detailNotice, "Ticket assist draft posted as a comment") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
	if cmd == nil {
		t.Fatal("expected comments refresh command")
	}
}

func TestClaudeTicketAssistDraftCanBeCopied(t *testing.T) {
	var copied string
	withLinkActions(t, nil, func(value string) error {
		copied = value
		return nil
	})
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Sanitize this", Status: "To Do"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistDraft = "Summary: Better ticket\n\nAcceptance Criteria\n- [ ] Clear and testable"
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+y"}))
	next := updated.(Model)
	if cmd == nil {
		t.Fatal("expected copy command")
	}
	msg := cmd()
	linkMsg, ok := msg.(linkActionMsg)
	if !ok {
		t.Fatalf("message = %#v", msg)
	}
	updated, _ = next.Update(linkMsg)
	next = updated.(Model)
	if copied != model.claudeAssistDraft {
		t.Fatalf("copied = %q", copied)
	}
	if !strings.Contains(next.detailNotice, "Copied Claude ticket draft") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestClaudeTicketAssistDraftSelectionCanBeCopiedAndDeleted(t *testing.T) {
	var copied string
	withLinkActions(t, nil, func(value string) error {
		copied = value
		return nil
	})
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Sanitize this", Status: "To Do"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistDraft = "Summary: Better ticket"
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditor.MoveToBegin()
	model.claudeAssistEditorReady = true

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+space"}))
	next := updated.(Model)
	for range len("Summary") {
		updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "right", Code: tea.KeyRight}))
		next = updated.(Model)
	}
	if selected := next.claudeAssistDraftSelection.SelectedText(next.claudeAssistEditor); selected != "Summary" {
		t.Fatalf("selected = %q", selected)
	}

	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+y"}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected copy command")
	}
	msg := cmd()
	linkMsg, ok := msg.(linkActionMsg)
	if !ok {
		t.Fatalf("message = %#v", msg)
	}
	updated, _ = next.Update(linkMsg)
	next = updated.(Model)
	if copied != "Summary" {
		t.Fatalf("copied = %q", copied)
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "delete", Code: tea.KeyDelete}))
	next = updated.(Model)
	if got := next.claudeAssistDraftValue(); got != ": Better ticket" {
		t.Fatalf("draft = %q", got)
	}
}

func TestClaudeTicketAssistDraftShiftArrowStartsSelection(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Sanitize this", Status: "To Do"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistDraft = "Summary: Better ticket"
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditor.MoveToBegin()
	model.claudeAssistEditorReady = true

	var updated tea.Model = model
	for range len("Summary") {
		updated, _ = updated.Update(tea.KeyPressMsg(tea.Key{Text: "shift+right"}))
	}
	next := updated.(Model)

	if selected := next.claudeAssistDraftSelection.SelectedText(next.claudeAssistEditor); selected != "Summary" {
		t.Fatalf("selected = %q", selected)
	}
}

func TestClaudeTicketAssistDraftPrintableLettersStayInEditor(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second, AllowJiraWrites: true, RequireConfirmation: true}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 36
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Original summary", Status: "To Do"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistDraft = ""
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	next := model
	for _, key := range []string{"o", "u", "r", " ", "c", "o", "n", "t", "e", "x", "t"} {
		updated, _ := next.Update(tea.KeyPressMsg(tea.Key{Text: key, Code: rune(key[0])}))
		next = updated.(Model)
		if next.claudeAssistRefining || next.claudeAssistConfirmComment {
			t.Fatalf("typing %q changed modal state: refining=%v confirmComment=%v", key, next.claudeAssistRefining, next.claudeAssistConfirmComment)
		}
	}

	if got := next.claudeAssistDraftValue(); got != "our context" {
		t.Fatalf("draft = %q, want typed text", got)
	}
}

func TestClaudeTicketAssistCtrlROpensRefinementEditor(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 36
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Original summary", Status: "To Do"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistDraft = "Summary: Better ticket\n\nAcceptance Criteria\n- Clear and testable"
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+r"}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("opening refinement editor should not run Claude")
	}
	if !next.claudeAssistRefining {
		t.Fatal("expected refinement editor state")
	}
	view := next.render()
	for _, want := range []string{"Refine Draft", "Instruction", "ctrl+s send", "esc cancel"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestClaudeTicketAssistRefinementPromptIncludesCurrentEditedDraft(t *testing.T) {
	stream := events.NewStream()
	defer stream.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	received, err := stream.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	runner := &fakeClaudeRunner{result: claude.Result{Text: "Review\n- Tightened acceptance criteria\n\nDraft\nSummary: Refined ticket\n\nAcceptance Criteria\n- [ ] Refined criterion"}}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithEventStream(stream),
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
		WithClaudeRunner(runner),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 36
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Original summary", Status: "To Do", Priority: "High"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0], Description: "Original Jira description."},
	}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistDraft = "Summary: User edited summary\n\nAcceptance Criteria\n- User edited criterion"
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+r"}))
	next := updated.(Model)
	next.claudeAssistRefineInstruction = "make the acceptance criteria measurable"
	next.claudeAssistRefineEditor = newClaudeAssistRefineEditor(next.claudeAssistRefineInstruction)
	next.claudeAssistRefineEditorReady = true
	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected refinement Claude command")
	}
	if !next.claudeAssistLoading {
		t.Fatal("expected loading while refining")
	}

	resultMsg := <-runClaudePlanCommandAsyncForTest(cmd)
	result, ok := resultMsg.(claudeAssistResultMsg)
	if !ok {
		t.Fatalf("message = %#v", resultMsg)
	}
	for _, want := range []string{
		"Refine this Jira ticket draft",
		"make the acceptance criteria measurable",
		"Current user-edited draft",
		"Summary: User edited summary",
		"User edited criterion",
		"Original Jira description.",
	} {
		if !strings.Contains(runner.request.Prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, runner.request.Prompt)
		}
	}
	taskEvents := collectEventTypesForTest(t, received, events.TypeAITaskRequested, events.TypeAITaskCompleted)
	requested := decodeAITaskPayloadForTest(t, taskEvents[events.TypeAITaskRequested])
	if requested.Operation != events.AIOperationRefineDraft || requested.IssueKey != "ABC-1" {
		t.Fatalf("requested payload = %#v", requested)
	}

	updated, _ = next.Update(result)
	next = updated.(Model)
	if !strings.Contains(next.claudeAssistDraftValue(), "Refined criterion") {
		t.Fatalf("draft = %q", next.claudeAssistDraftValue())
	}
}

func TestClaudeTicketAssistCtrlSRequiresJiraWriteGate(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second, AllowJiraWrites: false, RequireConfirmation: true}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 36
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Original summary", Status: "To Do"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistDraft = "Summary: Better ticket\n\nAcceptance Criteria\n- Clear and testable"
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("expected no Jira write command when writes are disabled")
	}
	if !next.claudeAssistOpen {
		t.Fatal("draft should stay open when writes are disabled")
	}
	if next.claudeAssistConfirmApply || next.claudeAssistApplying {
		t.Fatalf("unexpected apply state confirm=%v applying=%v", next.claudeAssistConfirmApply, next.claudeAssistApplying)
	}
	if !strings.Contains(next.detailNotice, "Jira writes are disabled") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestClaudeTicketAssistCtrlSOpensApplyConfirmation(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second, AllowJiraWrites: true, RequireConfirmation: true}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 36
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Original summary", Status: "To Do"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistDraft = "Summary: Better ticket\n\nProblem / Goal\nMake it clearer.\n\nAcceptance Criteria\n- Clear and testable"
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("expected confirmation before Jira write command")
	}
	if !next.claudeAssistConfirmApply {
		t.Fatal("expected apply confirmation")
	}
	if next.claudeAssistApplySummary != "Better ticket" {
		t.Fatalf("apply summary = %q", next.claudeAssistApplySummary)
	}
	if !strings.Contains(next.claudeAssistApplyDescription, "Acceptance Criteria") {
		t.Fatalf("apply description = %q", next.claudeAssistApplyDescription)
	}
	view := next.render()
	for _, want := range []string{"Apply Ticket Assist Draft", "Summary", "Description", "ctrl+s apply", "esc cancel"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestClaudeTicketAssistConfirmAppliesSummaryAndDescription(t *testing.T) {
	searcher := &fakeIssueSearcher{}
	model := NewModel(
		searcher,
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second, AllowJiraWrites: true, RequireConfirmation: true}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 36
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Original summary", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: jira.Issue{Key: "ABC-1", Summary: "Original summary"}, Description: "Original description"},
	}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistDraft = "Summary: Better ticket\n\nProblem / Goal\nMake it clearer.\n\nAcceptance Criteria\n- Clear and testable"
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next := updated.(Model)
	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected apply command")
	}
	if !next.claudeAssistApplying {
		t.Fatal("expected applying state")
	}

	submitBatch := cmd()
	batch, ok := submitBatch.(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("submit command = %#v", submitBatch)
	}
	for _, sub := range batch {
		if msg := sub(); msg == nil {
			t.Fatal("expected work submitted message")
		}
	}
	for i := 0; i < 2; i++ {
		msg := next.waitForWorkerResult()()
		result, ok := msg.(workerResultMsg)
		if !ok {
			t.Fatalf("worker message = %#v", msg)
		}
		updated, _ = next.Update(result)
		next = updated.(Model)
	}

	if searcher.updateSummaryKey != "ABC-1" || searcher.updateSummaryValue != "Better ticket" {
		t.Fatalf("summary update = %s/%s", searcher.updateSummaryKey, searcher.updateSummaryValue)
	}
	if searcher.updateDescriptionKey != "ABC-1" || !strings.Contains(searcher.updateDescriptionValue, "Acceptance Criteria") {
		t.Fatalf("description update = %s/%s", searcher.updateDescriptionKey, searcher.updateDescriptionValue)
	}
	if next.claudeAssistOpen || next.claudeAssistApplying {
		t.Fatalf("expected modal closed after apply, open=%v applying=%v", next.claudeAssistOpen, next.claudeAssistApplying)
	}
	if next.issues[0].Summary != "Better ticket" {
		t.Fatalf("issue summary = %q", next.issues[0].Summary)
	}
	if next.details["ABC-1"].Description != searcher.updateDescriptionValue {
		t.Fatalf("detail description = %q", next.details["ABC-1"].Description)
	}
	if !strings.Contains(next.detailNotice, "Ticket assist draft applied") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestParseClaudeSubtaskReviewItems(t *testing.T) {
	items := parseClaudeSubtaskReviewItems(strings.Join([]string{
		"- Keep: ABC-2 because controller installation is still required.",
		"- Add: Document platform controller ownership.",
		"- Rescope: ABC-3 to cover Helm release automation only.",
		"- Remove: ABC-4 from this epic because it belongs to the platform backlog.",
	}, "\n"))

	if len(items) != 4 {
		t.Fatalf("items = %#v", items)
	}
	if items[0].Kind != claudeSubtaskReviewKeep || items[0].Key != "ABC-2" {
		t.Fatalf("keep item = %#v", items[0])
	}
	if items[1].Kind != claudeSubtaskReviewAdd || items[1].Summary != "Document platform controller ownership" {
		t.Fatalf("add item = %#v", items[1])
	}
	if items[2].Kind != claudeSubtaskReviewModify || items[2].Key != "ABC-3" {
		t.Fatalf("modify item = %#v", items[2])
	}
	if items[3].Kind != claudeSubtaskReviewClose || items[3].Key != "ABC-4" {
		t.Fatalf("close item = %#v", items[3])
	}
}

func TestClaudeTicketAssistApplyOpensSubtaskReview(t *testing.T) {
	searcher := &fakeIssueSearcher{comments: []jira.Comment{{ID: "20001", Author: "Current User", Body: "recommendations posted"}}}
	model := NewModel(
		searcher,
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second, AllowJiraWrites: true, RequireConfirmation: true}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 36
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Original summary", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: jira.Issue{Key: "ABC-1", Summary: "Original summary"}, Description: "Original description"},
	}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistDraft = strings.Join([]string{
		"Summary: Better ticket",
		"",
		"Problem / Goal",
		"Make it clearer.",
		"",
		"Acceptance Criteria",
		"- Clear and testable",
		"",
		"Open Questions",
		"- None",
		"",
		"Subtask Recommendations",
		"- Keep: ABC-2 because controller installation is still required.",
		"- Add: Document platform controller ownership.",
		"- Rescope: ABC-3 to cover Helm release automation only.",
	}, "\n")
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true
	model.claudeAssistQuestions = []createAIQuestion{
		{Question: "Which controllers are required?", Answer: "external-dns and cert-manager"},
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next := updated.(Model)
	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected apply command")
	}
	submitBatch := cmd()
	batch, ok := submitBatch.(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("submit command = %#v", submitBatch)
	}
	for _, sub := range batch {
		if msg := sub(); msg == nil {
			t.Fatal("expected work submitted message")
		}
	}
	for i := 0; i < 2; i++ {
		msg := next.waitForWorkerResult()()
		result, ok := msg.(workerResultMsg)
		if !ok {
			t.Fatalf("worker message = %#v", msg)
		}
		updated, _ = next.Update(result)
		next = updated.(Model)
	}

	if searcher.updateSummaryValue != "Better ticket" {
		t.Fatalf("summary = %q", searcher.updateSummaryValue)
	}
	if !strings.Contains(searcher.updateDescriptionValue, "Acceptance Criteria") {
		t.Fatalf("description = %q", searcher.updateDescriptionValue)
	}
	if searcher.addedBody != "" {
		t.Fatalf("recommendations should not be posted as a parent comment, got %q", searcher.addedBody)
	}
	if next.claudeAssistOpen || next.claudeAssistApplying || !next.claudeSubtaskReviewOpen {
		t.Fatalf("expected subtask review after apply, assistOpen=%v applying=%v reviewOpen=%v", next.claudeAssistOpen, next.claudeAssistApplying, next.claudeSubtaskReviewOpen)
	}
	if len(next.claudeSubtaskReviewItems) != 3 {
		t.Fatalf("review items = %#v", next.claudeSubtaskReviewItems)
	}
	if !strings.Contains(next.detailNotice, "Review 3 subtask recommendations") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
	view := next.render()
	for _, want := range []string{"Review Subtask Changes", "Add child", "Modify", "ABC-3", "enter apply"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in:\n%s", want, view)
		}
	}
}

func TestClaudeSubtaskReviewClosesInvalidWhenSafeTransitionAvailable(t *testing.T) {
	searcher := &fakeIssueSearcher{
		transitions: []jira.Transition{{ID: "31", Name: "Close Invalid", ToStatus: "Done", IsAvailable: true}},
	}
	model := NewModel(
		searcher,
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second, AllowJiraWrites: true, RequireConfirmation: true}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 36
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Epic", Status: "To Do"},
		{Key: "ABC-2", Summary: "Old child", Status: "To Do", ParentKey: "ABC-1"},
	}
	model.details = map[string]jira.IssueDetail{"ABC-1": {Issue: model.issues[0], Description: "Parent"}}
	model = model.openClaudeSubtaskReview("ABC-1", "Epic", "- Remove: ABC-2 from this epic because it is invalid.")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if cmd == nil {
		t.Fatal("expected transitions command")
	}
	submitted := submittedWorkMessagesForTest(t, cmd)
	if len(submitted) != 1 || submitted[0].kind != worker.KindGetTransitions || submitted[0].key != "ABC-2" {
		t.Fatalf("transition load submit = %#v", submitted)
	}
	updated, cmd = next.Update(workerResultMsg{result: worker.Result{
		ID:   next.activeClaudeSubtaskReviewReqID,
		Kind: worker.KindGetTransitions,
		GetTransitions: &worker.GetTransitionsResult{
			Key:         "ABC-2",
			Transitions: searcher.transitions,
			SyncedAt:    time.Now(),
		},
	}})
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected transition command after loading transitions")
	}
	submitted = submittedWorkMessagesForTest(t, cmd)
	var sawTransition bool
	for _, msg := range submitted {
		if msg.kind == worker.KindTransitionIssue && msg.key == "ABC-2" {
			sawTransition = true
		}
	}
	if !sawTransition {
		t.Fatalf("transition submit = %#v item=%#v active=%d", submitted, next.claudeSubtaskReviewItems[0], next.activeClaudeSubtaskReviewReqID)
	}
	updated, _ = next.Update(workerResultMsg{result: worker.Result{
		ID:   next.activeClaudeSubtaskReviewReqID,
		Kind: worker.KindTransitionIssue,
		TransitionIssue: &worker.TransitionIssueResult{
			Key:      "ABC-2",
			ToStatus: "Done",
		},
	}})
	next = updated.(Model)

	if next.issues[1].Status != "Done" {
		t.Fatalf("child status = %q", next.issues[1].Status)
	}
	if !next.claudeSubtaskReviewItems[0].Done || next.claudeSubtaskReviewItems[0].Status != "closed" {
		t.Fatalf("review item = %#v", next.claudeSubtaskReviewItems[0])
	}
}

func TestClaudeSubtaskReviewCommentsWhenCloseNeedsFields(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second, AllowJiraWrites: true, RequireConfirmation: true}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 36
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "Epic", Status: "To Do"},
		{Key: "ABC-2", Summary: "Old child", Status: "To Do", ParentKey: "ABC-1"},
	}
	model.details = map[string]jira.IssueDetail{"ABC-1": {Issue: model.issues[0], Description: "Parent"}}
	model = model.openClaudeSubtaskReview("ABC-1", "Epic", "- Remove: ABC-2 from this epic because it is invalid.")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if len(submittedWorkMessagesForTest(t, cmd)) != 1 {
		t.Fatal("expected transition load submit")
	}
	updated, cmd = next.Update(workerResultMsg{result: worker.Result{
		ID:   next.activeClaudeSubtaskReviewReqID,
		Kind: worker.KindGetTransitions,
		GetTransitions: &worker.GetTransitionsResult{
			Key: "ABC-2",
			Transitions: []jira.Transition{{
				ID:          "31",
				Name:        "Close Invalid",
				ToStatus:    "Done",
				IsAvailable: true,
				Fields:      []jira.TransitionField{{ID: "resolution", Name: "Resolution", Required: true}},
			}},
			SyncedAt: time.Now(),
		},
	}})
	next = updated.(Model)
	submitted := submittedWorkMessagesForTest(t, cmd)
	if len(submitted) == 0 || submitted[0].kind != worker.KindAddComment || submitted[0].key != "ABC-2" {
		t.Fatalf("fallback submit = %#v", submitted)
	}
	updated, _ = next.Update(workerResultMsg{result: worker.Result{
		ID:   next.activeClaudeSubtaskReviewReqID,
		Kind: worker.KindAddComment,
		AddComment: &worker.AddCommentResult{
			Key:     "ABC-2",
			Comment: jira.Comment{ID: "10001", Body: "commented"},
		},
	}})
	next = updated.(Model)

	if next.issues[1].Status != "To Do" {
		t.Fatalf("child status should not change, got %q", next.issues[1].Status)
	}
	if !next.claudeSubtaskReviewItems[0].Done || next.claudeSubtaskReviewItems[0].Status != "commented" {
		t.Fatalf("review item = %#v", next.claudeSubtaskReviewItems[0])
	}
}

func submittedWorkMessagesForTest(t *testing.T, cmd tea.Cmd) []workSubmittedMsg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	msg := cmd()
	switch typed := msg.(type) {
	case workSubmittedMsg:
		return []workSubmittedMsg{typed}
	case tea.BatchMsg:
		submitted := make([]workSubmittedMsg, 0, len(typed))
		for _, sub := range typed {
			if sub == nil {
				continue
			}
			if submittedMsg, ok := sub().(workSubmittedMsg); ok {
				submitted = append(submitted, submittedMsg)
			}
		}
		return submitted
	default:
		t.Fatalf("command message = %#v", msg)
		return nil
	}
}

func TestClaudeTicketAssistApplyFailureKeepsDraftOpen(t *testing.T) {
	applyErr := errors.New("jira rejected description")
	model := NewModel(
		&fakeIssueSearcher{err: applyErr},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Second, AllowJiraWrites: true, RequireConfirmation: true}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 36
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Original summary", Status: "To Do"}}
	model.claudeAssistOpen = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistDraft = "Summary: Better ticket\n\nAcceptance Criteria\n- Clear and testable"
	model.claudeAssistEditor = newClaudeAssistEditor(model.claudeAssistDraft)
	model.claudeAssistEditorReady = true

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next := updated.(Model)
	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected apply command")
	}
	batch := cmd().(tea.BatchMsg)
	for _, sub := range batch {
		_ = sub()
	}
	msg := next.waitForWorkerResult()()
	result := msg.(workerResultMsg)
	updated, _ = next.Update(result)
	next = updated.(Model)

	if !next.claudeAssistOpen {
		t.Fatal("draft should stay open after apply failure")
	}
	if next.claudeAssistDraftValue() != model.claudeAssistDraft {
		t.Fatalf("draft changed to %q", next.claudeAssistDraftValue())
	}
	if !strings.Contains(next.detailNotice, "Ticket assist apply failed") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestClaudeTicketAssistLoadingSuppressesAssistantPreview(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketAssist: true, Timeout: time.Minute}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Sanitize this", Status: "To Do"}}
	model.claudeAssistOpen = true
	model.claudeAssistLoading = true
	model.claudeAssistKey = "ABC-1"
	model.claudeAssistStartedAt = time.Date(2026, 6, 14, 15, 13, 0, 0, time.Local)
	model.claudeAssistProgress = []claude.Event{
		{Kind: "output", Text: "produce the expected precondition/validation errors"},
	}

	view := model.render()
	if !strings.Contains(view, "Output: receiving response") {
		t.Fatalf("missing calm output status in %q", view)
	}
	if strings.Contains(view, "produce the expected precondition") || strings.Contains(view, "Assistant:") {
		t.Fatalf("assist loading modal leaked assistant preview: %q", view)
	}
}

func TestClaudePlanShowsSubprocessActivityAndCancelHintWhileRunning(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Timeout: 2 * time.Minute}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
		WithClaudeRunner(&fakeClaudeRunner{result: claude.Result{Text: "Implementation plan"}}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "To Do"}}
	model.jumpDetailSection("Claude")

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	next.claudePlanStartedAt = time.Date(2026, 6, 14, 15, 13, 0, 0, time.Local)
	now := next.claudePlanStartedAt.Add(3 * time.Second)
	next.now = func() time.Time { return now }

	view := next.render()
	for _, want := range []string{
		"Asking Claude",
		"Activity:",
		"Claude subprocess running",
		"Elapsed: 3s of 2m0s",
		"Output: waiting for first response",
		"esc cancel",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	for _, unwanted := range []string{
		"Request:",
		"Mode:",
		"Command:",
		"Started:",
		"Deadline:",
	} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("debug field %q leaked into loading modal: %q", unwanted, view)
		}
	}
}

func TestClaudePlanTimeoutShowsElapsedDeadlineContext(t *testing.T) {
	started := time.Date(2026, 6, 14, 15, 13, 0, 0, time.Local)
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Command: "/Users/joncha/.local/bin/claude", Timeout: 2 * time.Minute}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "/Users/joncha/.local/bin/claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "To Do"}}
	model.claudePlanOpen = true
	model.claudePlanKey = "ABC-1"
	model.claudePlanStartedAt = started
	model.claudePlanErr = context.DeadlineExceeded
	model.now = func() time.Time { return started.Add(2*time.Minute + 3*time.Second) }

	view := model.render()
	for _, want := range []string{
		"Claude plan timed out after 2m0s",
		"Started: 15:13:00",
		"Deadline: 15:15:00",
		"Elapsed: 2m3s",
		"Command: /Users/joncha/.local/bin/claude -p <prompt>",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestClaudePlanProgressAppearsBeforeFinalResult(t *testing.T) {
	runner := &fakeClaudeRunner{
		events: []claude.Event{
			{Kind: "stderr", Text: "Not logged in - please run /login"},
		},
		waitForCancel: true,
	}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Timeout: time.Minute}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
		WithClaudeRunner(runner),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "To Do"}}
	model.jumpDetailSection("Claude")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	progress := <-runClaudePlanProgressAsyncForTest(cmd)

	updated, _ = next.Update(progress)
	next = updated.(Model)
	view := next.render()
	for _, want := range []string{"Output: receiving response"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	if strings.Contains(view, "Not logged in - please run /login") || strings.Contains(view, "Assistant:") {
		t.Fatalf("loading modal should not stream assistant text: %q", view)
	}

	next = next.cancelClaudeTicketPlan()
}

func TestClaudeProgressModalSuppressesAssistantPreviewWhileLoading(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Timeout: time.Minute}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "To Do"}}
	model.claudePlanOpen = true
	model.claudePlanLoading = true
	model.claudePlanKey = "ABC-1"
	model.claudePlanStartedAt = time.Date(2026, 6, 14, 15, 13, 0, 0, time.Local)
	model.claudePlanProgress = []claude.Event{
		{Kind: "stream_event", Text: `{"type":"stream_event","event":{"type":"message_delta"}}`},
		{Kind: "output", Text: "use ALB target groups"},
	}

	view := model.render()
	if !strings.Contains(view, "Output: receiving response") {
		t.Fatalf("missing calm output status in %q", view)
	}
	if strings.Contains(view, `{"type":"stream_event"`) || strings.Contains(view, "use ALB target groups") || strings.Contains(view, "Assistant:") {
		t.Fatalf("loading modal leaked stream detail: %q", view)
	}
}

func TestClaudeProgressModalIgnoresProtocolNoise(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Timeout: time.Minute}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "To Do"}}
	model.claudePlanOpen = true
	model.claudePlanLoading = true
	model.claudePlanKey = "ABC-1"
	model.claudePlanStartedAt = time.Date(2026, 6, 14, 15, 13, 0, 0, time.Local)
	model.claudePlanProgress = []claude.Event{
		{Kind: "user", Text: "Create a read-only implementation plan"},
		{Kind: "status", Text: "message stop"},
	}

	view := model.render()
	if !strings.Contains(view, "Output: receiving CLI messages") {
		t.Fatalf("missing waiting state in %q", view)
	}
	if strings.Contains(view, "Output: user") || strings.Contains(view, "Create a read-only implementation plan") {
		t.Fatalf("protocol noise leaked into modal: %q", view)
	}
}

func TestClaudeProgressModalDedupesRepeatedOutput(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Timeout: time.Minute}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "To Do"}}
	model.claudePlanOpen = true
	model.claudePlanLoading = true
	model.claudePlanKey = "ABC-1"
	model.claudePlanStartedAt = time.Date(2026, 6, 14, 15, 13, 0, 0, time.Local)
	repeated := "I'll start by exploring the local environment to ground the plan."
	model.claudePlanProgress = []claude.Event{
		{Kind: "output", Text: repeated},
		{Kind: "output", Text: repeated},
	}

	preview := claudeAssistantPreview(model.claudePlanProgress)
	if preview != repeated {
		t.Fatalf("preview = %q", preview)
	}
}

func TestClaudeProgressAssemblesDeltaChunks(t *testing.T) {
	events := []claude.Event{
		{Kind: "output", Text: "I'll start by checking the "},
		{Kind: "output", Text: "Terraform modules and then outline "},
		{Kind: "output", Text: "the verification plan."},
	}

	preview := claudeAssistantPreview(events)
	want := "Assistant: I'll start by checking the Terraform modules"
	if !strings.Contains("Assistant: "+preview, want) {
		t.Fatalf("missing assembled preview %q in %q", want, preview)
	}
	if strings.Contains("Assistant: "+preview, "Assistant: the verification plan") {
		t.Fatalf("preview showed tail fragment instead of assembled beginning: %q", preview)
	}
}

func TestClaudePlanLongResultStaysInsideViewport(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Timeout: time.Minute}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 32
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "To Do"}}
	model.claudePlanOpen = true
	model.claudePlanKey = "ABC-1"
	model.claudePlanText = strings.Repeat("Claude result line with implementation detail.\n", 80)

	view := model.render()
	lines := strings.Split(view, "\n")
	if len(lines) > model.height {
		t.Fatalf("view height = %d, want <= %d\n%s", len(lines), model.height, view)
	}
	if !strings.Contains(view, "Claude Lines 1-") {
		t.Fatalf("missing Claude line range in %q", view)
	}
	if !strings.Contains(view, "j/k scroll") || !strings.Contains(view, "pgup/pgdn page") {
		t.Fatalf("missing scroll footer in %q", view)
	}
}

func TestClaudePlanMarkdownTableRendersAsBoundedTable(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Timeout: time.Minute}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 130
	model.height = 36
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "To Do"}}
	model.claudePlanOpen = true
	model.claudePlanKey = "ABC-1"
	model.claudePlanText = "Findings:\n\n" +
		"| Signal | Value |\n" +
		"|---|---|\n" +
		"| Current repo | terraform-module-eks with only README.md and no Terraform implementation |\n" +
		"| Ticket subject | Internal ALBs for ECS deployments |\n\n" +
		"Next steps."

	view := model.render()
	for _, want := range []string{"Signal", "Current repo", "terraform-module-eks", "Next steps."} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	if strings.Contains(view, "|---|---|") {
		t.Fatalf("raw markdown table separator leaked into Claude modal: %q", view)
	}
	if !strings.Contains(view, "╭") || !strings.Contains(view, "│") {
		t.Fatalf("expected styled table block in %q", view)
	}
}

func TestClaudePlanDialogUsesWideTerminalSpace(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Timeout: time.Minute}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 160
	model.height = 40
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "To Do"}}
	model.claudePlanOpen = true
	model.claudePlanKey = "ABC-1"
	phrase := "This Claude plan sentence should remain readable on a wide terminal without forced narrow wrapping."
	model.claudePlanText = phrase

	view := model.render()
	if !strings.Contains(view, phrase) {
		t.Fatalf("wide Claude modal wrapped phrase unexpectedly:\n%s", view)
	}
}

func TestClaudePlanResultScrollsWithNavigationKeys(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Timeout: time.Minute}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 32
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "To Do"}}
	model.claudePlanOpen = true
	model.claudePlanKey = "ABC-1"
	for i := 1; i <= 40; i++ {
		model.claudePlanText += fmt.Sprintf("Claude result line %02d.\n", i)
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	next := updated.(Model)
	if next.claudePlanOffset != 1 {
		t.Fatalf("offset = %d, want 1", next.claudePlanOffset)
	}
	view := next.render()
	if !strings.Contains(view, "Claude Lines 2-") {
		t.Fatalf("missing scrolled line range in %q", view)
	}
	if strings.Contains(view, "Claude result line 01.") {
		t.Fatalf("first line should be scrolled out of view: %q", view)
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "G", Code: 'G'}))
	next = updated.(Model)
	view = next.render()
	if !strings.Contains(view, "Claude Lines 35-40 of 40") {
		t.Fatalf("missing bottom line range in %q", view)
	}
}

func TestClaudePlanEscCancelsRunningRequest(t *testing.T) {
	runner := &fakeClaudeRunner{waitForCancel: true}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, TicketPlan: true, Timeout: time.Minute}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
		WithClaudeRunner(runner),
	)
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 120
	model.height = 35
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Fix production thing", Status: "To Do"}}
	model.jumpDetailSection("Claude")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if !next.claudePlanLoading {
		t.Fatal("expected Claude plan loading")
	}
	results := runClaudePlanCommandAsyncForTest(cmd)

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "esc", Code: tea.KeyEsc}))
	next = updated.(Model)
	if next.claudePlanLoading {
		t.Fatal("expected Claude plan loading to stop after cancel")
	}
	if next.claudePlanErr == nil || !strings.Contains(next.claudePlanErr.Error(), "cancel") {
		t.Fatalf("expected cancel error, got %v", next.claudePlanErr)
	}

	select {
	case msg := <-results:
		result, ok := msg.(claudePlanResultMsg)
		if !ok {
			t.Fatalf("message = %#v", msg)
		}
		if result.err == nil || !strings.Contains(result.err.Error(), "cancel") {
			t.Fatalf("expected cancelled result error, got %v", result.err)
		}
	case <-time.After(time.Second):
		t.Fatal("Claude command did not unblock after cancel")
	}
}

func runClaudePlanProgressAsyncForTest(cmd tea.Cmd) <-chan tea.Msg {
	results := make(chan tea.Msg, 1)
	go func() {
		if cmd == nil {
			close(results)
			return
		}
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, sub := range batch {
				if sub == nil {
					continue
				}
				go func(sub tea.Cmd) {
					if result, ok := sub().(claudePlanProgressMsg); ok {
						results <- result
					}
				}(sub)
			}
			return
		}
		results <- msg
	}()
	return results
}

func runClaudePlanCommandAsyncForTest(cmd tea.Cmd) <-chan tea.Msg {
	results := make(chan tea.Msg, 1)
	go func() {
		if cmd == nil {
			close(results)
			return
		}
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, sub := range batch {
				if sub == nil {
					continue
				}
				go func(sub tea.Cmd) {
					result := sub()
					if _, ok := result.(claudePlanResultMsg); ok {
						results <- result
					}
					if _, ok := result.(claudeAssistResultMsg); ok {
						results <- result
					}
					if _, ok := result.(createAIPromptResultMsg); ok {
						results <- result
					}
				}(sub)
			}
			return
		}
		results <- msg
	}()
	return results
}
