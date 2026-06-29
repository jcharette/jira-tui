package tui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/jcharette/jira-tui/internal/cache"
	"github.com/jcharette/jira-tui/internal/claude"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/jcharette/jira-tui/internal/worker"
)

func TestCreateShortcutOpensFromDetailWithoutMovingFocus(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.mode = modeDetail
	model.width = 100
	model.height = 30
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Story", Assignee: "Jane Doe", Priority: "Medium", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{
		"ABC-1": {Issue: model.issues[0]},
	}

	if target := model.focusedDetailTargetID(); target != "overview" {
		t.Fatalf("initial focused target = %q", target)
	}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "n", Code: 'n'}))
	next := updated.(Model)

	if !next.createOpen {
		t.Fatal("expected create modal to open")
	}
	if target := next.focusedDetailTargetID(); target != "overview" {
		t.Fatalf("focused target moved to %q", target)
	}
	if cmd == nil {
		t.Fatal("expected metadata request command")
	}
}

func TestCreateShortcutLoadsCreateIssueTypes(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.width = 100
	model.height = 30

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "n", Code: 'n'}))
	next := updated.(Model)

	if !next.createOpen {
		t.Fatal("expected create modal to open")
	}
	if next.createProjectKey != "ABC" {
		t.Fatalf("createProjectKey = %q", next.createProjectKey)
	}
	if !next.createIssueTypesLoading {
		t.Fatal("expected issue type metadata loading")
	}
	if cmd == nil {
		t.Fatal("expected metadata request command")
	}
	view := next.render()
	for _, want := range []string{"Create Ticket", "Loading issue types"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestToilShortcutLoadsIssueTypes(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.width = 100
	model.height = 30

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "T", Code: 'T'}))
	next := updated.(Model)

	if !next.toilOpen {
		t.Fatal("expected toil modal to open")
	}
	if next.toilProjectKey != "ABC" {
		t.Fatalf("toilProjectKey = %q", next.toilProjectKey)
	}
	if !next.toilIssueTypesLoading {
		t.Fatal("expected issue type metadata loading")
	}
	if cmd == nil {
		t.Fatal("expected issue type request command")
	}
	view := next.render()
	for _, want := range []string{"Create Toil Ticket", "Loading issue types", "esc cancel"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestToilCreateLogsWorkAndQueuesClose(t *testing.T) {
	searcher := &fakeIssueSearcher{}
	model := NewModel(searcher, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.width = 100
	model.height = 30
	model.toilOpen = true
	model.toilProjectKey = "ABC"
	model.toilIssueTypes = []jira.CreateIssueType{
		{ID: "10001", Name: "Task"},
		{ID: "10002", Name: "Toil"},
	}
	model.toilSummaryEditor = newSummaryEditor("Rotate certs")
	model.toilSummaryEditorReady = true
	model.toilTimeEditor = newWorklogTimeInput("45m")
	model.toilTimeEditorReady = true
	model.toilNoteEditor = newWorklogCommentEditor("prod cert cleanup")
	model.toilNoteEditorReady = true
	model.toilCloseAfterCreate = true

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next := updated.(Model)

	if !next.toilSubmitting {
		t.Fatal("expected toil submission")
	}
	if next.toilSubmitIssueType.ID != "10002" {
		t.Fatalf("toilSubmitIssueType = %#v", next.toilSubmitIssueType)
	}
	if cmd == nil {
		t.Fatal("expected create issue command")
	}

	msg := cmd()
	submitted, ok := msg.(workSubmittedMsg)
	if !ok || submitted.kind != worker.KindCreateIssue {
		t.Fatalf("create cmd msg = %#v", msg)
	}

	next = next.handleToilCreateIssueResult(worker.Result{
		ID:   next.activeToilCreateReqID,
		Kind: worker.KindCreateIssue,
		CreateIssue: &worker.CreateIssueResult{
			Issue: jira.Issue{Key: "ABC-123", Summary: "Rotate certs"},
		},
	})
	if !next.toilLoggingWork || next.toilCreatedKey != "ABC-123" {
		t.Fatalf("expected worklog queue, logging=%v key=%q", next.toilLoggingWork, next.toilCreatedKey)
	}
	if next.toilWorklogRequest.TimeSpent != "45m" || next.toilWorklogRequest.Comment != "prod cert cleanup" {
		t.Fatalf("toilWorklogRequest = %#v", next.toilWorklogRequest)
	}
	queued, queueCmd := next.handleToilCreateIssueResultWithCmd(worker.Result{
		ID:   next.activeToilCreateReqID,
		Kind: worker.KindCreateIssue,
		CreateIssue: &worker.CreateIssueResult{
			Issue: jira.Issue{Key: "ABC-124", Summary: "Rotate certs"},
		},
	})
	if queueCmd == nil || !queued.toilLoggingWork {
		t.Fatal("expected worklog command after create result")
	}
	msg = queueCmd()
	submitted, ok = msg.(workSubmittedMsg)
	if !ok || submitted.kind != worker.KindAddWorklog {
		t.Fatalf("worklog cmd msg = %#v", msg)
	}

	updatedModel, closeCmd := next.handleToilAddWorklogResult(worker.Result{
		ID:   next.activeToilAddWorklogReqID,
		Kind: worker.KindAddWorklog,
		AddWorklog: &worker.AddWorklogResult{
			Key:     "ABC-123",
			Worklog: jira.Worklog{ID: "10001", TimeSpent: "45m"},
		},
	})
	next = updatedModel
	if !next.toilLoadingTransitions {
		t.Fatal("expected transition load after worklog")
	}
	if closeCmd == nil {
		t.Fatal("expected transition metadata command")
	}
}

func TestToilCloseSkipsRequiredFieldTransition(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.width = 100
	model.height = 30
	model.toilOpen = true
	model.toilCreatedKey = "ABC-123"
	model.toilCloseAfterCreate = true
	model.activeToilTransitionsReqID = 44
	model.toilLoadingTransitions = true

	updated, cmd := model.handleToilGetTransitionsResult(worker.Result{
		ID:   44,
		Kind: worker.KindGetTransitions,
		GetTransitions: &worker.GetTransitionsResult{
			Key: "ABC-123",
			Transitions: []jira.Transition{
				{ID: "31", Name: "Resolve", ToStatus: "Done", IsAvailable: true, Fields: []jira.TransitionField{{ID: "resolution", Required: true}}},
			},
		},
	})

	if cmd != nil {
		t.Fatal("required-field transition should not submit")
	}
	if updated.toilOpen {
		t.Fatal("expected toil modal to close")
	}
	if !strings.Contains(updated.detailNotice, "Created ABC-123") || !strings.Contains(updated.detailNotice, "no safe terminal transition") {
		t.Fatalf("detailNotice = %q", updated.detailNotice)
	}
}

func TestCreateSubtaskFiltersIssueTypesAndSubmitsParent(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.width = 100
	model.height = 30
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.createParentKey = "ABC-1"
	model.createParentSummary = "Parent story"
	model.createIssueTypes = []jira.CreateIssueType{
		{ID: "10001", Name: "Task", Subtask: false},
		{ID: "10002", Name: "Sub-task", Subtask: true},
	}

	view := model.render()
	if strings.Contains(view, "Task") && !strings.Contains(view, "Sub-task") {
		t.Fatalf("normal issue type should not render for subtask creation:\n%s", view)
	}
	for _, want := range []string{"Create Subtask", "ABC-1", "Sub-task"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in:\n%s", want, view)
		}
	}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if cmd == nil {
		t.Fatal("expected create field metadata request")
	}
	if next.createIssueType.ID != "10002" {
		t.Fatalf("createIssueType = %#v", next.createIssueType)
	}

	next.createFieldsLoading = false
	next.createFields = []jira.CreateField{{ID: "summary", Name: "Summary", Required: true, SchemaSystem: "summary"}}
	next.beginCreateForm()
	updated, _ = next.Update(tea.PasteMsg{Content: "Add regression coverage"})
	next = updated.(Model)
	updated, cmd = next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)
	if !next.createSubmitting {
		t.Fatal("expected create submission")
	}
	if next.createSubmitParentKey != "ABC-1" {
		t.Fatalf("createSubmitParentKey = %q", next.createSubmitParentKey)
	}
	if cmd == nil {
		t.Fatal("expected create issue command")
	}
}

func TestCreateShortcutUsesFreshPersistentIssueTypes(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.Local)
	store := newFakeActiveViewStore()
	store.createIssueTypes = cache.CreateIssueTypesRecord{
		Namespace:  "https://example.atlassian.net",
		ProjectKey: "ABC",
		IssueTypes: []jira.CreateIssueType{{ID: "10001", Name: "Story"}},
		SyncedAt:   now.Add(-10 * time.Second),
		FreshTill:  now.Add(time.Minute),
	}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithActiveViewStore(store, "https://example.atlassian.net"),
	)
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = false
	model.width = 100
	model.height = 30

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "n", Code: 'n'}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("fresh persistent issue types should not submit Jira work")
	}
	if next.createIssueTypesLoading {
		t.Fatal("fresh persistent issue types should render without loading")
	}
	if len(next.createIssueTypes) != 1 || next.createIssueTypes[0].Name != "Story" {
		t.Fatalf("createIssueTypes = %#v", next.createIssueTypes)
	}
	view := next.render()
	if !strings.Contains(view, "> Story") {
		t.Fatalf("cached issue type should render in picker:\n%s", view)
	}
}

func TestCreateEmptyIssueTypesOnlyShowsCancelFooter(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.width = 100
	model.height = 30

	view := model.render()

	for _, want := range []string{"Jira returned 0 creatable issue types for DEVOPS.", "ctrl+d diagnostics", "esc cancel"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	for _, hidden := range []string{"j/k type", "tab field", "ctrl+s submit"} {
		if strings.Contains(view, hidden) {
			t.Fatalf("inactive create command %q should be hidden in %q", hidden, view)
		}
	}
}

func TestCreateEmptyIssueTypesCanOpenDiagnostics(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.width = 100
	model.height = 30
	model.diagnosticsEvents = []diagnosticEvent{
		{At: time.Now(), Kind: diagnosticKindWorker, Label: string(worker.KindGetCreateIssueTypes), Status: "ok", Detail: "#77 DEVOPS types=0"},
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+d"}))
	next := updated.(Model)

	if !next.diagnosticsOpen {
		t.Fatal("expected diagnostics to open from create modal")
	}
	view := next.render()
	if !strings.Contains(view, "get_create_issue_types #77 DEVOPS types=0") {
		t.Fatalf("missing create metadata diagnostics in %q", view)
	}
}

func TestCreateIssueTypePickerLoadsFieldsForSelection(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.createIssueTypes = []jira.CreateIssueType{
		{ID: "10001", Name: "Task"},
		{ID: "10002", Name: "Bug"},
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "down", Code: tea.KeyDown}))
	next := updated.(Model)
	if next.selectedCreateIssueType != 1 {
		t.Fatalf("selectedCreateIssueType = %d", next.selectedCreateIssueType)
	}

	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)
	if next.createIssueType.ID != "10002" {
		t.Fatalf("createIssueType = %#v", next.createIssueType)
	}
	if !next.createFieldsLoading {
		t.Fatal("expected create fields loading")
	}
	if cmd == nil {
		t.Fatal("expected field metadata request command")
	}
}

func TestCreateIssueTypePickerUsesFreshPersistentFields(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.Local)
	store := newFakeActiveViewStore()
	store.createFields = cache.CreateFieldsRecord{
		Namespace:   "https://example.atlassian.net",
		ProjectKey:  "ABC",
		IssueTypeID: "10002",
		Fields: []jira.CreateField{{
			ID:       "components",
			Name:     "Components",
			Required: true,
			AllowedValues: []jira.FieldOption{
				{ID: "20001", Name: "csp_gateway"},
			},
		}},
		SyncedAt:  now.Add(-10 * time.Second),
		FreshTill: now.Add(time.Minute),
	}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithActiveViewStore(store, "https://example.atlassian.net"),
	)
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.loading = false
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.createIssueTypes = []jira.CreateIssueType{
		{ID: "10001", Name: "Task"},
		{ID: "10002", Name: "Bug"},
	}
	model.selectedCreateIssueType = 1

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd != nil {
		t.Fatal("fresh persistent create fields should not submit Jira work")
	}
	if next.createFieldsLoading {
		t.Fatal("fresh persistent create fields should render without loading")
	}
	if next.createIssueType.ID != "10002" {
		t.Fatalf("createIssueType = %#v", next.createIssueType)
	}
	if len(next.createFields) != 1 || next.createFields[0].ID != "components" {
		t.Fatalf("createFields = %#v", next.createFields)
	}
	if next.createFieldFocus != createSummaryFieldIndex {
		t.Fatalf("createFieldFocus = %d", next.createFieldFocus)
	}
}

func TestCreateMetadataResultsPersistToStore(t *testing.T) {
	now := time.Date(2026, 6, 16, 10, 0, 0, 0, time.Local)
	store := newFakeActiveViewStore()
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithActiveViewStore(store, "https://example.atlassian.net"),
	)
	defer model.workers.Stop()
	model.now = func() time.Time { return now }
	model.activeCreateIssueTypesReqID = 11
	model.createProjectKey = "ABC"

	updated, _ := model.Update(workerResultMsg{
		result: worker.Result{
			ID:   11,
			Kind: worker.KindGetCreateIssueTypes,
			GetCreateIssueTypes: &worker.GetCreateIssueTypesResult{
				ProjectKey: "ABC",
				IssueTypes: []jira.CreateIssueType{
					{ID: "10001", Name: "Story"},
				},
				SyncedAt: now,
			},
		},
	})
	next := updated.(Model)

	if store.putCreateIssueTypes.Namespace != "https://example.atlassian.net" || store.putCreateIssueTypes.ProjectKey != "ABC" {
		t.Fatalf("putCreateIssueTypes = %#v", store.putCreateIssueTypes)
	}
	if len(store.putCreateIssueTypes.IssueTypes) != 1 || store.putCreateIssueTypes.IssueTypes[0].Name != "Story" {
		t.Fatalf("persisted issue types = %#v", store.putCreateIssueTypes.IssueTypes)
	}
	if !store.putCreateIssueTypes.SyncedAt.Equal(now) || !store.putCreateIssueTypes.FreshTill.Equal(now.Add(createIssueTypesCacheTTL)) {
		t.Fatalf("issue type timestamps = %s/%s", store.putCreateIssueTypes.SyncedAt, store.putCreateIssueTypes.FreshTill)
	}

	next.activeCreateFieldsReqID = 12
	next.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Story"}
	updated, _ = next.Update(workerResultMsg{
		result: worker.Result{
			ID:   12,
			Kind: worker.KindGetCreateFields,
			GetCreateFields: &worker.GetCreateFieldsResult{
				ProjectKey:  "ABC",
				IssueTypeID: "10001",
				Fields: []jira.CreateField{
					{ID: "components", Name: "Components"},
				},
				SyncedAt: now,
			},
		},
	})
	next = updated.(Model)

	if store.putCreateFields.Namespace != "https://example.atlassian.net" || store.putCreateFields.ProjectKey != "ABC" || store.putCreateFields.IssueTypeID != "10001" {
		t.Fatalf("putCreateFields = %#v", store.putCreateFields)
	}
	if len(store.putCreateFields.Fields) != 1 || store.putCreateFields.Fields[0].ID != "components" {
		t.Fatalf("persisted fields = %#v", store.putCreateFields.Fields)
	}
	if !store.putCreateFields.SyncedAt.Equal(now) || !store.putCreateFields.FreshTill.Equal(now.Add(createFieldsCacheTTL)) {
		t.Fatalf("field timestamps = %s/%s", store.putCreateFields.SyncedAt, store.putCreateFields.FreshTill)
	}
}

func TestCreateIssueTypePickerUsesSharedChoiceListRendering(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.width = 100
	model.height = 30
	model.createIssueTypes = []jira.CreateIssueType{
		{ID: "1", Name: "Type 01"},
		{ID: "2", Name: "Type 02"},
		{ID: "3", Name: "Type 03"},
		{ID: "4", Name: "Type 04"},
		{ID: "5", Name: "Type 05"},
		{ID: "6", Name: "Type 06"},
		{ID: "7", Name: "Type 07"},
		{ID: "8", Name: "Type 08"},
	}
	model.selectedCreateIssueType = 6

	view := model.render()

	if !strings.Contains(view, "> Type 07") {
		t.Fatalf("expected selected issue type from shared choice list:\n%s", view)
	}
	if strings.Contains(view, "Type 01") {
		t.Fatalf("expected shared choice list to page long issue type results:\n%s", view)
	}
	if !strings.Contains(view, "of 8") {
		t.Fatalf("expected shared range indicator for issue type results:\n%s", view)
	}
	if strings.Contains(view, "ISSUE TYPE") {
		t.Fatalf("issue type picker should not render the old table header:\n%s", view)
	}
}

func TestCreateDraftShortcutNotAvailableInIssueTypeSelection(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.createIssueTypes = []jira.CreateIssueType{
		{ID: "10001", Name: "Task"},
		{ID: "10002", Name: "Bug"},
	}
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "g", Code: 'g'}))
	next := updated.(Model)
	if next.detailNotice != "Select an issue type before generating a ticket draft." {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestCreateIssueTypeSelectionShowsAIGeneratedTab(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, DraftTicket: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.width = 120
	model.height = 40
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.createIssueTypes = []jira.CreateIssueType{
		{ID: "10001", Name: "Task"},
		{ID: "10002", Name: "Bug"},
	}

	view := model.render()
	for _, want := range []string{"Manual", "AI Generated", "> Task", "Bug", "tab mode"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestCreateIssueModeTabsDoNotTruncateStyledLabels(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, DraftTicket: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.createAIGeneratedMode = true

	tabs := model.renderCreateModeTabs(64)

	for _, want := range []string{"Manual", "AI Generated"} {
		if !strings.Contains(tabs, want) {
			t.Fatalf("missing %q in %q", want, tabs)
		}
	}
	if strings.Contains(tabs, "...") {
		t.Fatalf("tabs should not byte-truncate styled labels: %q", tabs)
	}
	if lipgloss.Width(tabs) < len("Manual AI Generated") {
		t.Fatalf("visible tab width too small: width=%d tabs=%q", lipgloss.Width(tabs), tabs)
	}
	if strings.Contains(tabs, "\x1b[48;") {
		t.Fatalf("create mode tabs should avoid filled backgrounds: %q", tabs)
	}
}

func TestCreateDraftPromptCanRunBeforeIssueTypeSelection(t *testing.T) {
	runner := &fakeClaudeRunner{
		result: claude.Result{Text: strings.Join([]string{
			"Issue Type: Initiative",
			"Summary: Stand up EKS clusters in staging",
			"Description: Create a staging EKS cluster rollout plan with platform controllers.",
		}, "\n")},
	}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, DraftTicket: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
		WithClaudeRunner(runner),
	)
	defer model.workers.Stop()
	model.loading = false
	model.width = 120
	model.height = 40
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.createIssueTypes = []jira.CreateIssueType{
		{ID: "10001", Name: "Initiative"},
		{ID: "10002", Name: "Story"},
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next := updated.(Model)
	if !next.createAIGeneratedMode {
		t.Fatal("expected tab to switch to AI generated mode")
	}
	next.createAIPrompt = "I want to create an epic about building EKS clusters in staging."
	next.createAIPromptEditor = newCreateAIPromptEditor(next.createAIPrompt)
	next.createAIPromptEditorReady = true

	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected create AI prompt command before issue type selection")
	}
	resultMsg := <-runClaudePlanCommandAsyncForTest(cmd)
	if !strings.Contains(runner.request.Prompt, "Issue Type: Not selected yet") {
		t.Fatalf("prompt should allow missing issue type, got %q", runner.request.Prompt)
	}
	for _, want := range []string{"Available Jira Issue Types:", "- Initiative", "- Story"} {
		if !strings.Contains(runner.request.Prompt, want) {
			t.Fatalf("prompt missing Jira issue type %q in %q", want, runner.request.Prompt)
		}
	}
	if strings.Contains(runner.request.Prompt, "- Task") {
		t.Fatalf("prompt should not guess unsupported issue types: %q", runner.request.Prompt)
	}

	updated, cmd = next.Update(resultMsg)
	next = updated.(Model)
	if next.createIssueType.Name != "Initiative" {
		t.Fatalf("issue type should apply Claude's Jira-supported recommendation, got %#v", next.createIssueType)
	}
	if !next.createFieldsLoading {
		t.Fatal("expected recommended issue type to start loading create fields")
	}
	if cmd == nil {
		t.Fatal("expected create fields command after AI issue type recommendation")
	}
	if next.createAIGeneratedMode {
		t.Fatal("expected generated draft to return to manual mode")
	}
	if next.createSummaryDraft != "Stand up EKS clusters in staging" {
		t.Fatalf("createSummaryDraft = %q", next.createSummaryDraft)
	}
	if !strings.Contains(next.detailNotice, "selected Initiative") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestCreateDraftPromptIncludesAvailableComponents(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, DraftTicket: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Story"}
	model.createFields = []jira.CreateField{
		{ID: "components", Name: "Components", SchemaSystem: "components", AllowedValues: []jira.FieldOption{
			{ID: "101", Name: "csp-adapter"},
			{ID: "102", Name: "csp-gateway"},
		}},
	}

	prompt := model.buildCreateIssueDraftPrompt("Audit DNS domain usage.")

	for _, want := range []string{"Available Components:", "- csp-adapter", "- csp-gateway", "Components: <one of the Available Components"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q in:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "- made-up-component") {
		t.Fatalf("prompt should only include Jira-returned components:\n%s", prompt)
	}
}

func TestCreateDraftComponentRecommendationSelectsMatchingJiraComponent(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Story"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
		{ID: "components", Name: "Components", SchemaSystem: "components", AllowedValues: []jira.FieldOption{
			{ID: "101", Name: "csp-adapter"},
			{ID: "102", Name: "csp-gateway"},
		}},
	}
	model.beginCreateForm()
	model.activeCreateAIPromptReqID = 8
	model.createAIPromptLoading = true

	updated, _ := model.Update(createAIPromptResultMsg{
		id: 8,
		text: strings.Join([]string{
			"Issue Type: Story",
			"Summary: Audit DNS domains",
			"Description: Make Terraform modules domain agnostic.",
			"",
			"Components",
			"csp-gateway",
		}, "\n"),
	})
	next := updated.(Model)

	key := createFieldValueKey(model.createFields[2])
	if next.createDynamicSelections[key] != 1 {
		t.Fatalf("component selection = %d", next.createDynamicSelections[key])
	}
}

func TestCreateDraftUnknownComponentDoesNotSelectRandomComponent(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Story"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
		{ID: "components", Name: "Components", SchemaSystem: "components", AllowedValues: []jira.FieldOption{
			{ID: "101", Name: "csp-adapter"},
			{ID: "102", Name: "csp-gateway"},
		}},
	}
	model.beginCreateForm()
	model.activeCreateAIPromptReqID = 9
	model.createAIPromptLoading = true

	updated, _ := model.Update(createAIPromptResultMsg{
		id: 9,
		text: strings.Join([]string{
			"Issue Type: Story",
			"Summary: Audit DNS domains",
			"Description: Make Terraform modules domain agnostic.",
			"",
			"Components",
			"unknown-service",
		}, "\n"),
	})
	next := updated.(Model)

	key := createFieldValueKey(model.createFields[2])
	if next.createDynamicSelections[key] != -1 {
		t.Fatalf("component selection = %d; should remain unselected", next.createDynamicSelections[key])
	}
}

func TestCreateDraftPromptLeavesTypeSelectionWhenRecommendationUnsupported(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.createIssueTypes = []jira.CreateIssueType{
		{ID: "10001", Name: "Initiative"},
		{ID: "10002", Name: "Story"},
	}
	model.activeCreateAIPromptReqID = 7
	model.createAIPromptLoading = true

	updated, cmd := model.Update(createAIPromptResultMsg{
		id: 7,
		text: strings.Join([]string{
			"Issue Type: Task",
			"Summary: Stand up EKS clusters in staging",
			"Description: Create a staging EKS cluster rollout plan with platform controllers.",
		}, "\n"),
	})
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("unsupported issue type recommendation should not submit create fields")
	}
	if next.createIssueType.ID != "" {
		t.Fatalf("issue type should remain unselected, got %#v", next.createIssueType)
	}
	if next.createSummaryDraft != "Stand up EKS clusters in staging" {
		t.Fatalf("createSummaryDraft = %q", next.createSummaryDraft)
	}
	if !strings.Contains(next.detailNotice, "Select an issue type") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestCreateComponentPickerFiltersAndSelectsWithTypeahead(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.width = 120
	model.height = 45
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Story"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
		{ID: "components", Name: "Components", SchemaSystem: "components", AllowedValues: []jira.FieldOption{
			{ID: "101", Name: "csp-adapter"},
			{ID: "102", Name: "csp-report"},
			{ID: "103", Name: "csp-gateway"},
		}},
	}
	model.beginCreateForm()
	model.createFieldFocus = model.createDynamicFieldFocusIndex(0)

	for _, key := range []string{"g", "a", "t"} {
		updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: key, Code: []rune(key)[0]}))
		model = updated.(Model)
	}

	fieldKey := createFieldValueKey(model.createFields[2])
	if model.createDynamicFilters[fieldKey] != "gat" {
		t.Fatalf("filter = %q", model.createDynamicFilters[fieldKey])
	}
	view := model.render()
	if !strings.Contains(view, "Filter: gat") || !strings.Contains(view, "csp-gateway") {
		t.Fatalf("expected filtered gateway view:\n%s", view)
	}
	if strings.Contains(view, "csp-adapter") || strings.Contains(view, "csp-report") {
		t.Fatalf("filter should hide non-matches:\n%s", view)
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	model = updated.(Model)
	if model.createDynamicSelections[fieldKey] != 2 {
		t.Fatalf("component selection = %d", model.createDynamicSelections[fieldKey])
	}
	if model.createDynamicFilters[fieldKey] != "" {
		t.Fatalf("filter should clear after enter, got %q", model.createDynamicFilters[fieldKey])
	}
}

func TestCreateAutocompleteFieldFetchesOptionsWithTypeahead(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.width = 120
	model.height = 45
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Story"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
		{
			ID:              "customfield_10010",
			Name:            "Investment Category",
			SchemaType:      "option",
			AutoCompleteURL: "https://example.atlassian.net/rest/api/3/customFieldOption/suggest",
		},
	}
	model.beginCreateForm()
	model.createFieldFocus = model.createDynamicFieldFocusIndex(0)

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "p", Code: 'p'}))
	next := updated.(Model)
	fieldKey := createFieldValueKey(model.createFields[2])
	if cmd == nil {
		t.Fatal("expected autocomplete option request command")
	}
	if !next.createFieldOptionsLoading[fieldKey] {
		t.Fatalf("createFieldOptionsLoading = %#v", next.createFieldOptionsLoading)
	}
	if next.createFieldOptionsQuery[fieldKey] != "p" {
		t.Fatalf("createFieldOptionsQuery = %#v", next.createFieldOptionsQuery)
	}
	if !strings.Contains(next.render(), "Loading Jira options") {
		t.Fatalf("expected loading state:\n%s", next.render())
	}

	updated, _ = next.Update(workerResultMsg{result: worker.Result{
		ID:   next.activeCreateFieldOptionsReqID,
		Kind: worker.KindSearchFieldOptions,
		SearchFieldOptions: &worker.SearchFieldOptionsResult{
			FieldID: "customfield_10010",
			Query:   "p",
			Options: []jira.FieldOption{{ID: "101", Name: "Platform"}, {ID: "102", Name: "Product"}},
		},
	}})
	next = updated.(Model)
	if next.createFieldOptionsLoading[fieldKey] {
		t.Fatalf("createFieldOptionsLoading = %#v", next.createFieldOptionsLoading)
	}
	if len(next.createFields[2].AllowedValues) != 2 {
		t.Fatalf("AllowedValues = %#v", next.createFields[2].AllowedValues)
	}
	if next.createDynamicSelections[fieldKey] != 0 {
		t.Fatalf("selection = %d", next.createDynamicSelections[fieldKey])
	}
	view := next.render()
	if !strings.Contains(view, "Platform") || !strings.Contains(view, "Product") {
		t.Fatalf("expected autocomplete options in picker:\n%s", view)
	}
}

func TestCreateComponentPickerBackspaceAndEscEditFilter(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Story"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
		{ID: "components", Name: "Components", SchemaSystem: "components", AllowedValues: []jira.FieldOption{
			{ID: "101", Name: "csp-adapter"},
			{ID: "102", Name: "csp-gateway"},
		}},
	}
	model.beginCreateForm()
	model.createFieldFocus = model.createDynamicFieldFocusIndex(0)
	fieldKey := createFieldValueKey(model.createFields[2])

	for _, key := range []string{"g", "a"} {
		updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: key, Code: []rune(key)[0]}))
		model = updated.(Model)
	}
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "backspace", Code: tea.KeyBackspace}))
	model = updated.(Model)
	if model.createDynamicFilters[fieldKey] != "g" {
		t.Fatalf("filter after backspace = %q", model.createDynamicFilters[fieldKey])
	}

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "esc", Code: tea.KeyEsc}))
	model = updated.(Model)
	if model.createDynamicFilters[fieldKey] != "" {
		t.Fatalf("filter after esc = %q", model.createDynamicFilters[fieldKey])
	}
	if !model.createOpen {
		t.Fatal("esc should clear filter before closing create modal")
	}
}

func TestCreateComponentPickerFilterUsesCursorAwareTextInput(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Story"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
		{ID: "components", Name: "Components", SchemaSystem: "components", AllowedValues: []jira.FieldOption{
			{ID: "101", Name: "abxc"},
			{ID: "102", Name: "abcx"},
		}},
	}
	model.beginCreateForm()
	model.createFieldFocus = model.createDynamicFieldFocusIndex(0)
	fieldKey := createFieldValueKey(model.createFields[2])

	for _, key := range []string{"a", "b", "c"} {
		updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: key, Code: []rune(key)[0]}))
		model = updated.(Model)
	}
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "left", Code: tea.KeyLeft}))
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "x", Code: 'x'}))
	model = updated.(Model)

	if model.createDynamicFilters[fieldKey] != "abxc" {
		t.Fatalf("filter = %q, want cursor-aware insertion", model.createDynamicFilters[fieldKey])
	}
	if model.createDynamicSelections[fieldKey] != 0 {
		t.Fatalf("component selection = %d", model.createDynamicSelections[fieldKey])
	}
}

func TestCreateComponentPickerUsesSharedChoiceListRendering(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.width = 120
	model.height = 45
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Story"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
		{ID: "components", Name: "Components", SchemaSystem: "components", AllowedValues: []jira.FieldOption{
			{ID: "101", Name: "component-01"},
			{ID: "102", Name: "component-02"},
			{ID: "103", Name: "component-03"},
			{ID: "104", Name: "component-04"},
			{ID: "105", Name: "component-05"},
			{ID: "106", Name: "component-06"},
			{ID: "107", Name: "component-07"},
			{ID: "108", Name: "component-08"},
		}},
	}
	model.beginCreateForm()
	model.createFieldFocus = model.createDynamicFieldFocusIndex(0)
	model.createDynamicSelections["components"] = 6

	view := model.render()

	if !strings.Contains(view, "> component-07") {
		t.Fatalf("expected selected component from shared choice list:\n%s", view)
	}
	if strings.Contains(view, "component-01") {
		t.Fatalf("expected shared choice list to page long component results:\n%s", view)
	}
	if !strings.Contains(view, "of 8") {
		t.Fatalf("expected shared range indicator for component results:\n%s", view)
	}
	if strings.Contains(view, "Options ") {
		t.Fatalf("create picker should not render old custom range label:\n%s", view)
	}
}

func TestCreateDraftPromptCanOpenFromCreateForm(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, DraftTicket: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.width = 120
	model.height = 35
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Task"}
	model.createFields = nil
	model.beginCreateForm()

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next := updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)

	if !next.createAIPromptOpen {
		t.Fatal("expected create AI prompt editor to open")
	}
	if next.createAIPromptErr != nil {
		t.Fatalf("unexpected prompt error: %v", next.createAIPromptErr)
	}
}

func TestCreateDraftPromptCanOpenFromCreateFormWithEnter(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, DraftTicket: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.width = 120
	model.height = 35
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Task"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", Required: true},
		{ID: "description", Name: "Description"},
	}
	model.beginCreateForm()
	model.createSummaryDraft = "internal load balancer"
	model.createDescriptionDraft = "need ecs service-to-service traffic over private alb"
	model.createSummaryEditor = newSummaryEditor(model.createSummaryDraft)
	model.createDescriptionEditor = newCommentEditor(model.createDescriptionDraft)

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next := updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)

	if !next.createAIPromptOpen {
		t.Fatal("expected create AI prompt editor to open via tab selection")
	}
}

func TestCreateDraftPromptVisibleButUnavailableShowsNotice(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, DraftTicket: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: false, Command: "claude", Message: "Claude version check failed", Err: errors.New("some error")}),
	)
	defer model.workers.Stop()
	model.loading = false
	model.width = 120
	model.height = 35
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Task"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", Required: true},
		{ID: "description", Name: "Description"},
	}
	model.beginCreateForm()

	view := model.render()
	if !strings.Contains(view, "enter generate") {
		t.Fatalf("expected create draft key in footer, got %q", view)
	}

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next := updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)
	if next.createAIPromptOpen {
		t.Fatal("create draft prompt should not open while Claude is unavailable")
	}
	if next.detailNotice != "Claude ticket draft generation is currently unavailable." {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestCreateDraftPromptSubmissionParsesSummaryAndDescription(t *testing.T) {
	runner := &fakeClaudeRunner{
		result: claude.Result{
			Text: strings.Join([]string{
				"Summary: Add internal load balancer support",
				"Description: Create a private ALB for ECS internal traffic.",
				"",
				"Implementation notes",
				"- add module wiring",
			}, "\n"),
		},
	}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, DraftTicket: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
		WithClaudeRunner(runner),
	)
	defer model.workers.Stop()
	model.loading = false
	model.width = 120
	model.height = 35
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Task"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", Required: true},
		{ID: "description", Name: "Description"},
	}
	model.beginCreateForm()
	model.createSummaryDraft = "internal load balancer"
	model.createDescriptionDraft = "need ecs service-to-service traffic over private alb"
	model.createSummaryEditor = newSummaryEditor(model.createSummaryDraft)
	model.createDescriptionEditor = newCommentEditor(model.createDescriptionDraft)

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next := updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)

	if !next.createAIPromptOpen {
		t.Fatal("expected create AI prompt editor to open")
	}
	next.createAIPrompt = "Draft a ticket for internal-only load balancers."
	next.createAIPromptEditor = newCreateAIPromptEditor(next.createAIPrompt)
	next.createAIPromptEditorReady = true
	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected create AI prompt command")
	}
	resultMsg := <-runClaudePlanCommandAsyncForTest(cmd)
	result, ok := resultMsg.(createAIPromptResultMsg)
	if !ok {
		t.Fatalf("expected createAIPromptResultMsg, got %T", resultMsg)
	}
	if result.err != nil {
		t.Fatalf("expected no error, got %v", result.err)
	}
	for _, want := range []string{"Project: ABC", "Issue Type: Task", "User request", "Draft a ticket for internal-only load balancers."} {
		if !strings.Contains(runner.request.Prompt, want) {
			t.Fatalf("prompt missing %q in %q", want, runner.request.Prompt)
		}
	}
	for _, want := range []string{"Current draft", "internal load balancer", "need ecs service-to-service traffic over private alb"} {
		if !strings.Contains(runner.request.Prompt, want) {
			t.Fatalf("prompt missing current draft %q in %q", want, runner.request.Prompt)
		}
	}

	updated, _ = next.Update(resultMsg)
	next = updated.(Model)
	if next.createAIPromptOpen {
		t.Fatal("expected prompt modal to close after applying generated draft")
	}
	if next.createSummaryDraft != "Add internal load balancer support" {
		t.Fatalf("createSummaryDraft = %q", next.createSummaryDraft)
	}
	if next.createDescriptionDraft != "Create a private ALB for ECS internal traffic." {
		t.Fatalf("createDescriptionDraft = %q", next.createDescriptionDraft)
	}
	if !strings.Contains(next.detailNotice, "Applied Claude ticket draft") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}

func TestCreateDraftPromptKeepsOpenOnMissingSummary(t *testing.T) {
	runner := &fakeClaudeRunner{
		result: claude.Result{
			Text: strings.Join([]string{
				"Description: Create a private ALB for ECS internal traffic.",
				"",
				"Implementation notes",
				"- add module wiring",
			}, "\n"),
		},
	}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = ABC",
		WithClaudeConfig(ClaudeConfig{Enabled: true, DraftTicket: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
		WithClaudeRunner(runner),
	)
	defer model.workers.Stop()
	model.loading = false
	model.width = 120
	model.height = 35
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Task"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", Required: true},
		{ID: "description", Name: "Description"},
	}
	model.beginCreateForm()

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next := updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)

	if !next.createAIPromptOpen {
		t.Fatal("expected create AI prompt editor to open")
	}
	next.createAIPrompt = "Draft a ticket for internal-only load balancers."
	next.createAIPromptEditor = newCreateAIPromptEditor(next.createAIPrompt)
	next.createAIPromptEditorReady = true
	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)
	if cmd == nil {
		t.Fatal("expected create AI prompt command")
	}
	resultMsg := <-runClaudePlanCommandAsyncForTest(cmd)
	result, ok := resultMsg.(createAIPromptResultMsg)
	if !ok {
		t.Fatalf("expected createAIPromptResultMsg, got %T", resultMsg)
	}
	if result.err != nil {
		t.Fatalf("expected no error, got %v", result.err)
	}

	updated, _ = next.Update(resultMsg)
	next = updated.(Model)
	if !next.createAIPromptOpen {
		t.Fatal("expected create AI prompt to stay open on parse failure")
	}
	if next.createAIPromptErr == nil || !strings.Contains(next.createAIPromptErr.Error(), "missing a summary") {
		t.Fatalf("expected summary parse error, got %v", next.createAIPromptErr)
	}
	if next.createSummaryDraft != "" {
		t.Fatalf("createSummaryDraft = %q", next.createSummaryDraft)
	}
}

func TestParseCreateIssueDraftExtractsSummaryAndDescription(t *testing.T) {
	summary, description := parseCreateIssueDraft(strings.Join([]string{
		"Review",
		"Some quick thoughts.",
		"Summary: Add internal load balancer support",
		"Description:",
		"Create a private ALB for ECS internal traffic.",
		"",
		"Acceptance Criteria",
		"- should expose route53 record",
	}, "\n"))
	if summary != "Add internal load balancer support" {
		t.Fatalf("summary = %q", summary)
	}
	if description != "Create a private ALB for ECS internal traffic." {
		t.Fatalf("description = %q", description)
	}
}

func TestParseCreateIssueDraftFieldsExtractsNamedSections(t *testing.T) {
	fields := parseCreateIssueDraftFields(strings.Join([]string{
		"Summary: Install Kubernetes",
		"Description:",
		"Provision clusters.",
		"",
		"Components",
		"acheron-migrations",
		"active-users-report",
		"",
		"Investment Category",
		"Growth",
		"",
		"Priority",
		"P0 - Critical",
	}, "\n"))

	if fields["components"] != "acheron-migrations\nactive-users-report" {
		t.Fatalf("components = %q", fields["components"])
	}
	if fields["investmentcategory"] != "Growth" {
		t.Fatalf("investment category = %q fields=%#v", fields["investmentcategory"], fields)
	}
	if fields["priority"] != "P0 - Critical" {
		t.Fatalf("priority = %q", fields["priority"])
	}
}

func TestParseCreateIssueOpenQuestionsExtractsActionableQuestions(t *testing.T) {
	questions := parseCreateIssueOpenQuestions(strings.Join([]string{
		"Summary: Build Kubernetes platform",
		"Description:",
		"Provision clusters.",
		"",
		"Open Questions",
		"- Which AWS accounts are in scope?",
		"- Is this for staging only or production too?",
		"",
		"Priority",
		"P2 - Medium",
	}, "\n"))

	if len(questions) != 2 {
		t.Fatalf("questions = %#v", questions)
	}
	if questions[0].Question != "Which AWS accounts are in scope?" {
		t.Fatalf("question[0] = %#v", questions[0])
	}
	if questions[1].Question != "Is this for staging only or production too?" {
		t.Fatalf("question[1] = %#v", questions[1])
	}
}

func TestCreateIssueIgnoresMetadataProjectAndIssueTypeRequiredFields(t *testing.T) {
	fields := []jira.CreateField{
		{ID: "project", Name: "Project", Required: true, SchemaSystem: "project", SchemaType: "project"},
		{ID: "issuetype", Name: "Issue Type", Required: true, SchemaSystem: "issuetype", SchemaType: "issuetype"},
		{ID: "summary", Name: "Summary", Required: true, SchemaSystem: "summary", SchemaType: "string"},
	}

	if unsupported := unsupportedRequiredCreateFields(fields); len(unsupported) != 0 {
		t.Fatalf("unsupported = %#v", unsupported)
	}
	if supported := supportedCreateFields(fields); len(supported) != 0 {
		t.Fatalf("supported = %#v", supported)
	}
}

func TestCreateAIDraftKeepsOpenQuestionsOutOfDescription(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Epic"}
	model.activeCreateAIPromptReqID = 12
	model.createAIPromptLoading = true

	updated, _ := model.Update(createAIPromptResultMsg{
		id: 12,
		text: strings.Join([]string{
			"Summary: Build Kubernetes platform",
			"Description:",
			"Provision clusters.",
			"",
			"Open Questions",
			"- Which AWS accounts are in scope?",
			"- Is this staging only?",
		}, "\n"),
	})
	next := updated.(Model)

	if strings.Contains(next.createDescriptionDraft, "Open Questions") {
		t.Fatalf("description should not include open questions: %q", next.createDescriptionDraft)
	}
	if len(next.createAIQuestions) != 2 {
		t.Fatalf("questions = %#v", next.createAIQuestions)
	}
}

func TestCreateFormRendersOpenQuestionsPanel(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.width = 140
	model.height = 45
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Epic"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
	}
	model.createAIQuestions = []createAIQuestion{
		{Question: "Which AWS accounts are in scope?"},
		{Question: "Is this staging only?", Answer: "Staging first."},
	}
	model.beginCreateForm()
	model.createFieldFocus = model.createQuestionsFieldIndex()

	view := model.render()

	for _, want := range []string{"Open Questions", "> Which AWS accounts are in scope?", "answered Is this staging only?", "enter answer"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in:\n%s", want, view)
		}
	}
}

func TestCreateQuestionAnswerStoresLocalFeedback(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.width = 140
	model.height = 45
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Epic"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
	}
	model.createAIQuestions = []createAIQuestion{{Question: "Which AWS accounts are in scope?"}}
	model.beginCreateForm()
	model.createFieldFocus = model.createQuestionsFieldIndex()

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if !next.createAIQuestionAnswering {
		t.Fatal("expected answer editor to open")
	}
	updated, _ = next.Update(tea.PasteMsg{Content: "All development AWS accounts."})
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)

	if next.createAIQuestionAnswering {
		t.Fatal("expected answer editor to close")
	}
	if next.createAIQuestions[0].Answer != "All development AWS accounts." {
		t.Fatalf("answer = %q", next.createAIQuestions[0].Answer)
	}
}

func TestCreateQuestionAnswerEnterSavesAndMovesToNextQuestion(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.width = 140
	model.height = 45
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Epic"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
	}
	model.createAIQuestions = []createAIQuestion{
		{Question: "Which Kubernetes flavor should we use?"},
		{Question: "Which AWS accounts are in scope?"},
	}
	model.beginCreateForm()
	model.createFieldFocus = model.createQuestionsFieldIndex()

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	updated, _ = next.Update(tea.PasteMsg{Content: "EKS"})
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)

	if next.createAIQuestions[0].Answer != "EKS" {
		t.Fatalf("first answer = %q", next.createAIQuestions[0].Answer)
	}
	if next.selectedCreateAIQuestion != 1 {
		t.Fatalf("selected question = %d, want 1", next.selectedCreateAIQuestion)
	}
	if !next.createAIQuestionAnswering {
		t.Fatal("expected answer editor to stay open on the next question")
	}
	if value := next.createAIQuestionEditor.Value(); value != "" {
		t.Fatalf("next answer editor should start empty, got %q", value)
	}
}

func TestCreateQuestionsPanelShowsAndRunsRefineWithAnswers(t *testing.T) {
	runner := &fakeClaudeRunner{
		result: claude.Result{Text: strings.Join([]string{
			"Summary: Build Kubernetes platform",
			"Description: Refined draft.",
		}, "\n")},
	}
	model := NewModel(
		&fakeIssueSearcher{},
		"project = DEVOPS",
		WithClaudeConfig(ClaudeConfig{Enabled: true, DraftTicket: true, Timeout: time.Second}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "claude"}),
		WithClaudeRunner(runner),
	)
	defer model.workers.Stop()
	model.loading = false
	model.width = 140
	model.height = 45
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Epic"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
	}
	model.createSummaryDraft = "Build Kubernetes platform"
	model.createDescriptionDraft = "Provision clusters."
	model.createAIQuestions = []createAIQuestion{
		{Question: "Which Kubernetes flavor should we use?", Answer: "EKS"},
		{Question: "Which AWS accounts are in scope?", Answer: "All development accounts."},
	}
	model.beginCreateForm()
	model.createFieldFocus = model.createQuestionsFieldIndex()

	view := model.render()
	if !strings.Contains(view, "ctrl+r refine with answers") {
		t.Fatalf("questions panel should expose refine action:\n%s", view)
	}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+r"}))
	next := updated.(Model)
	if cmd == nil {
		t.Fatal("expected ctrl+r to submit Claude refinement")
	}
	if !next.createAIPromptLoading {
		t.Fatal("expected create AI prompt loading after refine")
	}
	resultMsg := <-runClaudePlanCommandAsyncForTest(cmd)
	if _, ok := resultMsg.(createAIPromptResultMsg); !ok {
		t.Fatalf("expected createAIPromptResultMsg, got %T", resultMsg)
	}
	for _, want := range []string{"User answers to Open Questions", "Q: Which Kubernetes flavor should we use?", "A: EKS", "Q: Which AWS accounts are in scope?", "A: All development accounts."} {
		if !strings.Contains(runner.request.Prompt, want) {
			t.Fatalf("prompt missing %q in:\n%s", want, runner.request.Prompt)
		}
	}
}

func TestCreateDraftPromptIncludesOpenQuestionAnswers(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Epic"}
	model.createSummaryDraft = "Build Kubernetes platform"
	model.createDescriptionDraft = "Provision clusters."
	model.createAIQuestions = []createAIQuestion{
		{Question: "Which AWS accounts are in scope?", Answer: "All development AWS accounts."},
		{Question: "Is this staging only?"},
	}

	prompt := model.buildCreateIssueDraftPrompt("Refine with my answers.")

	for _, want := range []string{"User answers to Open Questions", "Q: Which AWS accounts are in scope?", "A: All development AWS accounts.", "Still unanswered Open Questions", "- Is this staging only?"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q in:\n%s", want, prompt)
		}
	}
}

func TestCreateAIDraftAppliesSupportedFieldsAfterMetadataLoads(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.activeCreateAIPromptReqID = 12
	model.createAIPromptLoading = true

	updated, _ := model.Update(createAIPromptResultMsg{
		id: 12,
		text: strings.Join([]string{
			"Summary: Install Kubernetes across all development AWS accounts",
			"Description:",
			"Provision and standardize Kubernetes across development workloads.",
			"",
			"Components",
			"acheron-migrations",
			"active-users-report",
			"",
			"Investment Category",
			"Growth",
			"",
			"Priority",
			"P0 - Critical",
		}, "\n"),
	})
	next := updated.(Model)

	next.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Initiative"}
	next.activeCreateFieldsReqID = 22
	updated, _ = next.Update(workerResultMsg{result: worker.Result{
		ID:   22,
		Kind: worker.KindGetCreateFields,
		GetCreateFields: &worker.GetCreateFieldsResult{
			ProjectKey:  "DEVOPS",
			IssueTypeID: "10001",
			Fields: []jira.CreateField{
				{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
				{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
				{ID: "components", Name: "Components", SchemaSystem: "components", SchemaType: "array", AllowedValues: []jira.FieldOption{
					{ID: "c1", Name: "acheron-migrations"},
					{ID: "c2", Name: "active-users-report"},
				}},
				{ID: "customfield_10010", Name: "Investment Category", SchemaType: "option", AllowedValues: []jira.FieldOption{
					{ID: "i1", Name: "KTLO"},
					{ID: "i2", Name: "Growth"},
				}},
				{ID: "priority", Name: "Priority", SchemaSystem: "priority", SchemaType: "priority", AllowedValues: []jira.FieldOption{
					{ID: "p0", Name: "P0 - Critical"},
					{ID: "p2", Name: "P2 - Medium"},
				}},
			},
		},
	}})
	next = updated.(Model)

	if next.createSummaryDraft != "Install Kubernetes across all development AWS accounts" {
		t.Fatalf("summary = %q", next.createSummaryDraft)
	}
	if got := next.createDynamicSelections["components"]; got != 0 {
		t.Fatalf("components selection = %d", got)
	}
	if got := next.createDynamicSelections["customfield_10010"]; got != 1 {
		t.Fatalf("investment category selection = %d", got)
	}
	if got := next.createDynamicSelections["priority"]; got != 0 {
		t.Fatalf("priority selection = %d", got)
	}
}

func TestCreateFormScrollsToFocusedLaterFields(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.width = 100
	model.height = 24
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Initiative"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
		{ID: "components", Name: "Components", SchemaSystem: "components", SchemaType: "array", AllowedValues: []jira.FieldOption{{ID: "c1", Name: "Component One"}}},
		{ID: "customfield_1", Name: "Release Instructions", SchemaType: "string"},
		{ID: "customfield_2", Name: "Investment Category", SchemaType: "option", AllowedValues: []jira.FieldOption{{ID: "g", Name: "Growth"}}},
		{ID: "labels", Name: "Labels", SchemaSystem: "labels", SchemaType: "string"},
		{ID: "priority", Name: "Priority", SchemaSystem: "priority", SchemaType: "priority", AllowedValues: []jira.FieldOption{{ID: "p0", Name: "P0 - Critical"}}},
	}
	model.beginCreateForm()
	model.createFieldFocus = model.createDynamicFieldFocusIndex(4)

	view := model.render()

	if !strings.Contains(view, "Priority") || !strings.Contains(view, "P0 - Critical") {
		t.Fatalf("focused later field should be visible:\n%s", view)
	}
	if strings.Contains(view, "Components") && strings.Contains(view, "Release Instructions") && strings.Contains(view, "Investment Category") {
		t.Fatalf("create modal did not window long body:\n%s", view)
	}
	if !strings.Contains(view, "Create Lines") {
		t.Fatalf("expected scroll indicator in long create form:\n%s", view)
	}
}

func TestCreateFormRendersCompactMetadataRowsWhenNotFocused(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS", WithClaudeConfig(ClaudeConfig{Enabled: true, DraftTicket: true}), WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true}))
	defer model.workers.Stop()
	model.loading = false
	model.width = 120
	model.height = 40
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Epic"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
		{ID: "components", Name: "Components", SchemaSystem: "components", SchemaType: "array", AllowedValues: []jira.FieldOption{
			{ID: "c1", Name: "acheron-migrations"},
			{ID: "c2", Name: "active-users-report"},
			{ID: "c3", Name: "ad-hoc-projects"},
		}},
		{ID: "priority", Name: "Priority", SchemaSystem: "priority", SchemaType: "priority", AllowedValues: []jira.FieldOption{
			{ID: "p0", Name: "P0 - Critical"},
			{ID: "p1", Name: "P1 - High"},
		}},
	}
	model.beginCreateForm()
	model.createDynamicSelections["components"] = 1
	model.createFieldFocus = createSummaryFieldIndex

	view := model.render()

	for _, want := range []string{"Components: active-users-report", "Priority: P0 - Critical", "Generate Draft"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in:\n%s", want, view)
		}
	}
	for _, unwanted := range []string{"ad-hoc-projects", "P1 - High", "Options 1-"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("unfocused metadata should stay compact, found %q in:\n%s", unwanted, view)
		}
	}
	if strings.Index(view, "Generate Draft") > strings.Index(view, "Components") {
		t.Fatalf("Generate Draft should appear before metadata fields:\n%s", view)
	}
}

func TestCreateFormFocusedPickerExpandsOptions(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.width = 120
	model.height = 35
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Epic"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
		{ID: "components", Name: "Components", SchemaSystem: "components", SchemaType: "array", AllowedValues: []jira.FieldOption{
			{ID: "c1", Name: "acheron-migrations"},
			{ID: "c2", Name: "active-users-report"},
		}},
	}
	model.beginCreateForm()
	model.createFieldFocus = model.createDynamicFieldFocusIndex(0)

	view := model.render()

	for _, want := range []string{"Components", "acheron-migrations", "active-users-report"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing focused picker value %q in:\n%s", want, view)
		}
	}
}

func TestCreateFormCanChangeTypeWithoutClearingDraft(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.width = 120
	model.height = 35
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueTypes = []jira.CreateIssueType{
		{ID: "10001", Name: "Epic"},
		{ID: "10002", Name: "Story"},
	}
	model.selectedCreateIssueType = 0
	model.createIssueType = model.createIssueTypes[0]
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
	}
	model.createSummaryDraft = "Deploy ArgoCD to Kubernetes clusters"
	model.createDescriptionDraft = "Enable GitOps based deployments across clusters."
	model.beginCreateForm()
	model.createFieldFocus = createTypeFieldIndex

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next := updated.(Model)
	if !next.createChangingType {
		t.Fatal("expected type picker to open from focused Type row")
	}
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "down", Code: tea.KeyDown}))
	next = updated.(Model)
	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "enter", Code: tea.KeyEnter}))
	next = updated.(Model)

	if next.createIssueType.Name != "Story" {
		t.Fatalf("issue type = %#v", next.createIssueType)
	}
	if next.createSummaryDraft != "Deploy ArgoCD to Kubernetes clusters" {
		t.Fatalf("summary cleared while changing type: %q", next.createSummaryDraft)
	}
	if next.createDescriptionDraft != "Enable GitOps based deployments across clusters." {
		t.Fatalf("description cleared while changing type: %q", next.createDescriptionDraft)
	}
	if !next.createFieldsLoading {
		t.Fatal("expected changing type to load new create fields")
	}
	if cmd == nil {
		t.Fatal("expected create fields command for new issue type")
	}
}

func TestCreateAIPromptLoadingSuppressesPartialAssistantAndDebugDetails(t *testing.T) {
	model := NewModel(
		&fakeIssueSearcher{},
		"project = DEVOPS",
		WithClaudeConfig(ClaudeConfig{Enabled: true, DraftTicket: true, Timeout: 2 * time.Minute, Command: "/Users/joncha/.local/bin/claude"}),
		WithClaudeStatus(ClaudeStatus{Enabled: true, Available: true, Command: "/Users/joncha/.local/bin/claude"}),
	)
	defer model.workers.Stop()
	model.createAIPromptLoading = true
	model.createAIPromptStartedAt = time.Date(2026, 6, 15, 12, 2, 30, 0, time.Local)
	model.now = func() time.Time { return model.createAIPromptStartedAt.Add(21 * time.Second) }
	model.createAIPromptProgress = []claude.Event{
		{Kind: "output", Text: "Assistant: activity, IAM/access, logging, baseline add-ons. - Could ask follow-up questions."},
	}

	body := strings.Join(model.renderCreateAIPromptBody(90), "\n")

	if !strings.Contains(body, "receiving response") {
		t.Fatalf("loading body should keep a stable receiving status:\n%s", body)
	}
	for _, noisy := range []string{"Assistant:", "activity, IAM/access", "/Users/joncha", "Deadline:", "Started:"} {
		if strings.Contains(body, noisy) {
			t.Fatalf("loading body should hide noisy/debug detail %q:\n%s", noisy, body)
		}
	}
}

func TestCreateDialogUsesResponsiveWidth(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.width = 150
	model.height = 40
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Epic"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
	}
	model.beginCreateForm()

	width := dialogBorderWidth(model.render())

	if width < 104 {
		t.Fatalf("create dialog width = %d, want wider responsive modal", width)
	}
}

func TestCreateSummaryFocusedUsesMultilineEditor(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.width = 140
	model.height = 40
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Epic"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
	}
	model.createSummaryDraft = "Build Kubernetes clusters across all FloQast development AWS accounts"
	model.beginCreateForm()
	model.createFieldFocus = createSummaryFieldIndex

	value := model.renderCreateSummaryValue(100)

	if got := strings.Count(value, "\n") + 1; got < 3 {
		t.Fatalf("focused summary editor should be multiline, rows=%d value:\n%s", got, value)
	}
}

func TestCreateDescriptionFocusedGetsMoreEditorRows(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.width = 140
	model.height = 50
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Epic"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
	}
	model.beginCreateForm()
	model.createFieldFocus = createDescriptionFieldIndex

	value := model.renderCreateDescriptionValue(64)

	if got := strings.Count(value, "\n") + 1; got < 16 {
		t.Fatalf("focused description editor should have more vertical space, rows=%d value:\n%s", got, value)
	}
}

func TestCreateFormSubmitsSummaryAndDescription(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.width = 100
	model.height = 30
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Task"}
	model.createFields = []jira.CreateField{{ID: "summary", Name: "Summary", Required: true}, {ID: "description", Name: "Description"}}
	model.beginCreateForm()

	updated, _ := model.Update(tea.PasteMsg{Content: "New internal load balancer"})
	next := updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	updated, _ = next.Update(tea.PasteMsg{Content: "Prepare services for internal ECS traffic."})
	next = updated.(Model)
	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)

	if !next.createSubmitting {
		t.Fatal("expected create submission")
	}
	if next.createSubmitSummary != "New internal load balancer" {
		t.Fatalf("createSubmitSummary = %q", next.createSubmitSummary)
	}
	if next.createSubmitDescription != "Prepare services for internal ECS traffic." {
		t.Fatalf("createSubmitDescription = %q", next.createSubmitDescription)
	}
	if cmd == nil {
		t.Fatal("expected create issue command")
	}
}

func TestCreateFormRendersAndSubmitsSupportedDynamicFields(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.width = 120
	model.height = 40
	model.createOpen = true
	model.createProjectKey = "ABC"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Task"}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", Required: true, SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
		{ID: "priority", Name: "Priority", Required: true, SchemaSystem: "priority", SchemaType: "priority", AllowedValues: []jira.FieldOption{
			{ID: "4", Name: "Low"},
			{ID: "3", Name: "Medium"},
		}},
		{ID: "customfield_10010", Name: "Team", Required: true, SchemaType: "string"},
	}
	model.beginCreateForm()

	view := model.render()
	for _, want := range []string{"Priority", "Low", "Team"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}

	updated, _ := model.Update(tea.PasteMsg{Content: "New internal load balancer"})
	next := updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "down", Code: tea.KeyDown}))
	next = updated.(Model)
	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "tab", Code: tea.KeyTab}))
	next = updated.(Model)
	updated, _ = next.Update(tea.PasteMsg{Content: "Platform"})
	next = updated.(Model)
	updated, cmd := next.Update(tea.KeyPressMsg(tea.Key{Text: "ctrl+s"}))
	next = updated.(Model)

	if !next.createSubmitting {
		t.Fatal("expected create submission")
	}
	if len(next.createSubmitFields) != 2 {
		t.Fatalf("createSubmitFields = %#v", next.createSubmitFields)
	}
	if next.createSubmitFields[0].FieldID != "priority" || next.createSubmitFields[0].Option.Name != "Medium" {
		t.Fatalf("priority field = %#v", next.createSubmitFields[0])
	}
	if next.createSubmitFields[1].FieldID != "customfield_10010" || next.createSubmitFields[1].Text != "Platform" {
		t.Fatalf("team field = %#v", next.createSubmitFields[1])
	}
	if cmd == nil {
		t.Fatal("expected create issue command")
	}
}

func TestCreateFormBoundsLongPickerOptions(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = DEVOPS")
	defer model.workers.Stop()
	model.loading = false
	model.width = 100
	model.height = 30
	model.createOpen = true
	model.createProjectKey = "DEVOPS"
	model.createIssueType = jira.CreateIssueType{ID: "10001", Name: "Initiative"}
	options := make([]jira.FieldOption, 0, 30)
	for index := 0; index < 30; index++ {
		options = append(options, jira.FieldOption{ID: strconv.Itoa(index + 1), Name: fmt.Sprintf("component-%02d", index+1)})
	}
	model.createFields = []jira.CreateField{
		{ID: "summary", Name: "Summary", Required: true, SchemaSystem: "summary", SchemaType: "string"},
		{ID: "description", Name: "Description", SchemaSystem: "description", SchemaType: "string"},
		{ID: "components", Name: "Components", SchemaSystem: "components", SchemaType: "array", AllowedValues: options},
	}
	model.beginCreateForm()
	model.createFieldFocus = model.createDynamicFieldFocusIndex(0)

	view := model.render()

	if lines := strings.Count(strings.TrimRight(view, "\n"), "\n") + 1; lines > model.height {
		t.Fatalf("create modal rendered %d lines, terminal height is %d:\n%s", lines, model.height, view)
	}
	if strings.Contains(view, "component-30") {
		t.Fatalf("long picker rendered too many options:\n%s", view)
	}
	for _, want := range []string{"Components", "component-01", "component-06", "1-6 of 30"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in:\n%s", want, view)
		}
	}

	for index := 0; index < 17; index++ {
		updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "down", Code: tea.KeyDown}))
		model = updated.(Model)
	}
	view = model.render()
	for _, want := range []string{"component-17", "of 30"} {
		if !strings.Contains(view, want) {
			t.Fatalf("moving through long picker did not keep %q visible in:\n%s", want, view)
		}
	}
	if strings.Contains(view, "component-01") {
		t.Fatalf("long picker should scroll away from the first option:\n%s", view)
	}
}

func TestCreateIssueSuccessAddsIssueToList(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	model.loading = false
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Existing"}}
	model.createOpen = true
	model.activeCreateIssueReqID = 22
	model.createSubmitting = true
	model.createSubmitSummary = "New issue"

	updated, _ := model.Update(workerResultMsg{result: worker.Result{
		ID:   22,
		Kind: worker.KindCreateIssue,
		CreateIssue: &worker.CreateIssueResult{
			Issue: jira.Issue{Key: "ABC-2", Summary: "New issue"},
		},
	}})
	next := updated.(Model)

	if next.createOpen {
		t.Fatal("expected create modal to close")
	}
	if len(next.issues) == 0 || next.issues[0].Key != "ABC-2" {
		t.Fatalf("issues = %#v", next.issues)
	}
	if next.selected != 0 {
		t.Fatalf("selected = %d", next.selected)
	}
	if !strings.Contains(next.detailNotice, "Created ABC-2") {
		t.Fatalf("detailNotice = %q", next.detailNotice)
	}
}
