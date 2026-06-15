# Inline Description AI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Description-scoped inline AI from ticket detail using the approved A+ hybrid pattern.

**Architecture:** Keep normal detail navigation simple. When Description is focused and Claude Ticket Assist is available, `a` opens a small `AI for Description` picker. Selected actions submit through the existing Claude Ticket Assist runner and reuse the same result modal, but mark the draft target as Description-only so apply writes only Description.

**Tech Stack:** Go, Bubble Tea, Bubbles textarea, Lip Gloss, existing Claude CLI runner, existing Jira worker pool.

---

## File Map

- Modify `internal/tui/model.go`: add inline AI state, render picker/instruction modal, handle `a` from Description focus, build inline prompts, and route Description-only apply.
- Modify `internal/tui/model_test.go`: add failing tests for entry, picker, prompt, question editor, result modal, Description-only apply, and comment post reuse.
- Modify `internal/tui/keymap.go`: update help text so `a` is described as contextual AI when applicable without adding redundant semantic keys.
- Modify `docs/project-state.md`: document inline Description AI behavior after implementation.
- Modify `docs/releases/CHANGELOG.md`: add user-visible Unreleased bullet after implementation.
- Modify `tasks/todo.md`: track execution checklist and final review evidence.
- Modify `tasks/lessons.md`: only if implementation reveals a reusable lesson or user correction.

## Task 1: Footer And Entry Gate

**Files:**
- Modify: `internal/tui/model_test.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/keymap.go`

- [ ] **Step 1: Write failing footer test**

Add a test near existing Claude/detail tests:

```go
func TestDescriptionFocusShowsInlineAIWhenClaudeTicketAssistAvailable(t *testing.T) {
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
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Improve this", Status: "To Do"}}
	model.details = map[string]jira.IssueDetail{"ABC-1": {Issue: model.issues[0], Description: "Old description"}}
	model.jumpDetailSection("Description")

	view := model.render()
	if !strings.Contains(view, "a AI") {
		t.Fatalf("expected inline AI footer hint in %q", view)
	}
}
```

- [ ] **Step 2: Verify test fails**

Run:

```bash
go test ./internal/tui -run TestDescriptionFocusShowsInlineAIWhenClaudeTicketAssistAvailable -count=1
```

Expected: FAIL because the footer still uses the generic `a ai` jump behavior or does not expose the Description-specific inline hint.

- [ ] **Step 3: Implement entry gate helper**

Add a helper in `internal/tui/model.go` near Claude availability helpers:

```go
func (m Model) inlineDescriptionAIAvailable() bool {
	if !m.claudeTicketAssistAvailable() {
		return false
	}
	section, ok := m.focusedDetailSection()
	return ok && section.ID == "description"
}
```

Update `detailFooterBindings()` so when `inlineDescriptionAIAvailable()` is true, the footer keeps normal detail navigation and shows a section command:

```go
case "description":
	if m.inlineDescriptionAIAvailable() {
		sectionBindings = []keyBinding{
			{Keys: []string{"a"}, Label: "AI", Group: "Section", Footer: true},
		}
	}
```

Update the `a` key handler before the generic Claude jump:

```go
case "a":
	if m.mode == modeDetail {
		if m.inlineDescriptionAIAvailable() {
			return m.openInlineDescriptionAI()
		}
		if m.claudeAvailable() {
			m.jumpDetailSection("Claude")
			return m, nil
		}
		m.startCommentComposer()
		return m, nil
	}
```

Add a stub so this task compiles:

```go
func (m Model) openInlineDescriptionAI() (Model, tea.Cmd) {
	m.detailNotice = "Inline Description AI is not implemented yet."
	return m, nil
}
```

- [ ] **Step 4: Verify footer test passes**

Run:

```bash
go test ./internal/tui -run TestDescriptionFocusShowsInlineAIWhenClaudeTicketAssistAvailable -count=1
```

Expected: PASS.

## Task 2: Inline AI Picker

**Files:**
- Modify: `internal/tui/model_test.go`
- Modify: `internal/tui/model.go`

- [ ] **Step 1: Write failing picker tests**

Add tests:

```go
func TestDescriptionAKeyOpensInlineAIPicker(t *testing.T) {
	model := newInlineDescriptionAIModel(t)
	model.jumpDetailSection("Description")

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: "a"}))
	next := updated.(Model)
	if cmd != nil {
		t.Fatal("picker should not start Claude")
	}
	if !next.inlineAIOpen {
		t.Fatal("expected inline AI picker open")
	}
	view := next.render()
	for _, want := range []string{"AI for Description", "Improve clarity", "Extract acceptance criteria", "Ask Claude a question", "Draft clarifying comment", "enter run", "esc cancel"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
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
```

Add test helper:

```go
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
```

- [ ] **Step 2: Verify picker tests fail**

Run:

```bash
go test ./internal/tui -run 'TestDescriptionAKeyOpensInlineAIPicker|TestInlineAIPickerCanCancel' -count=1
```

Expected: FAIL because `inlineAIOpen` and picker rendering do not exist.

- [ ] **Step 3: Implement picker state and render**

Add model fields:

```go
inlineAIOpen                bool
selectedInlineAIAction      int
inlineAIInstructionOpen     bool
inlineAIInstruction         string
inlineAIInstructionEditor   textarea.Model
inlineAIInstructionReady    bool
claudeAssistTarget          claudeAssistTarget
```

Add target type:

```go
type claudeAssistTarget int

const (
	claudeAssistTargetTicket claudeAssistTarget = iota
	claudeAssistTargetDescription
)
```

Add action type/helpers:

```go
type inlineAIAction struct {
	ID          string
	Label       string
	Description string
}

func inlineDescriptionAIActions() []inlineAIAction {
	return []inlineAIAction{
		{ID: "improve_clarity", Label: "Improve clarity", Description: "Rewrite the Description for clearer scope and verification."},
		{ID: "extract_acceptance", Label: "Extract acceptance criteria", Description: "Draft explicit acceptance criteria and open questions."},
		{ID: "ask_question", Label: "Ask Claude a question", Description: "Ask about this ticket and draft a local answer."},
		{ID: "draft_comment", Label: "Draft clarifying comment", Description: "Draft a Jira comment without editing fields."},
	}
}
```

Change `openInlineDescriptionAI()`:

```go
func (m Model) openInlineDescriptionAI() (Model, tea.Cmd) {
	if !m.inlineDescriptionAIAvailable() {
		m.detailNotice = "Claude ticket assistance is not enabled or available."
		return m, nil
	}
	m.inlineAIOpen = true
	m.inlineAIInstructionOpen = false
	m.selectedInlineAIAction = clamp(m.selectedInlineAIAction, 0, len(inlineDescriptionAIActions())-1)
	m.detailNotice = ""
	return m, nil
}
```

Add a dialog branch before Claude dialogs:

```go
if m.inlineAIOpen {
	return m.renderInlineAIDialog(width)
}
```

Render picker:

```go
func (m Model) renderInlineAIDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	bodyWidth := min(max(24, width-12), 72)
	actions := inlineDescriptionAIActions()
	cursor := clamp(m.selectedInlineAIAction, 0, len(actions)-1)
	rows := make([][]string, 0, len(actions))
	for index, action := range actions {
		marker := " "
		labelStyle := m.theme.Text
		descStyle := m.theme.Muted
		if index == cursor {
			marker = ">"
			labelStyle = m.theme.Selected
		}
		rows = append(rows, []string{labelStyle.Render(marker), labelStyle.Render(action.Label), descStyle.Render(action.Description)})
	}
	body := m.detailTable(0, []string{"", "ACTION", "DETAIL"}, rows, nil)
	return m.renderDetailDialog(width, "AI for Description", selected.Key, body, "j/k select  enter run  esc cancel")
}
```

Route update before normal key switch:

```go
if m.inlineAIOpen {
	return m.updateInlineAIPicker(msg)
}
```

Implement movement/cancel:

```go
func (m Model) updateInlineAIPicker(msg tea.KeyMsg) (Model, tea.Cmd) {
	actions := inlineDescriptionAIActions()
	switch msg.String() {
	case "esc":
		m.inlineAIOpen = false
		m.inlineAIInstructionOpen = false
		return m, nil
	case "j", "down":
		m.selectedInlineAIAction = clamp(m.selectedInlineAIAction+1, 0, len(actions)-1)
		return m, nil
	case "k", "up":
		m.selectedInlineAIAction = clamp(m.selectedInlineAIAction-1, 0, len(actions)-1)
		return m, nil
	case "enter":
		return m.runSelectedInlineAIAction()
	default:
		return m, nil
	}
}
```

Add a stub `runSelectedInlineAIAction()` returning a notice for now so tests compile.

- [ ] **Step 4: Verify picker tests pass**

Run:

```bash
go test ./internal/tui -run 'TestDescriptionAKeyOpensInlineAIPicker|TestInlineAIPickerCanCancel' -count=1
```

Expected: PASS.

## Task 3: Prompt Submission And Question Editor

**Files:**
- Modify: `internal/tui/model_test.go`
- Modify: `internal/tui/model.go`

- [ ] **Step 1: Write failing prompt tests**

Add tests:

```go
func TestInlineDescriptionAIImproveClaritySubmitsScopedPrompt(t *testing.T) {
	runner := &fakeClaudeRunner{result: claude.Result{Text: "Review\n- Description is vague\n\nDraft\nClearer description"}}
	model := newInlineDescriptionAIModel(t)
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
	for _, want := range []string{"Improve clarity", "Description-scoped", "Do not update Jira", "ABC-1", "Current unclear description", "Please clarify done."} {
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
```

- [ ] **Step 2: Verify prompt tests fail**

Run:

```bash
go test ./internal/tui -run 'TestInlineDescriptionAI(ImproveClaritySubmitsScopedPrompt|AskQuestionUsesInstruction)' -count=1
```

Expected: FAIL because action execution, instruction editor, and prompt builder are incomplete.

- [ ] **Step 3: Implement action execution**

Add `runSelectedInlineAIAction()`:

```go
func (m Model) runSelectedInlineAIAction() (Model, tea.Cmd) {
	actions := inlineDescriptionAIActions()
	action := actions[clamp(m.selectedInlineAIAction, 0, len(actions)-1)]
	if action.ID == "ask_question" {
		m.inlineAIInstructionOpen = true
		m.inlineAIInstruction = ""
		m.inlineAIInstructionEditor = newClaudeAssistRefineEditor("")
		m.inlineAIInstructionReady = true
		return m, nil
	}
	return m.submitInlineDescriptionAI(action, "")
}
```

Update `renderInlineAIDialog()` so `inlineAIInstructionOpen` renders a four-row textarea and footer `ctrl+s send  esc cancel`.

Update `updateInlineAIPicker()` so when `inlineAIInstructionOpen` is true, `ctrl+s` submits, `esc` returns to picker, and other keys update the textarea.

Add:

```go
func (m Model) submitInlineDescriptionAI(action inlineAIAction, instruction string) (Model, tea.Cmd) {
	ctx, ok := m.detailRenderContext()
	if !ok {
		m.detailNotice = "No selected ticket for inline AI."
		return m, nil
	}
	if strings.TrimSpace(ctx.description) == "" && strings.TrimSpace(ctx.detail.Description) == "" {
		m.detailNotice = "Description is not loaded yet."
		return m, nil
	}
	key := ctx.display.Key
	if key == "" {
		key = ctx.selected.Key
	}
	m.nextRequestID++
	reqID := m.nextRequestID
	m.activeClaudeAssistReqID = reqID
	m.claudeAssistKey = key
	m.claudeAssistText = ""
	m.claudeAssistErr = nil
	m.claudeAssistLoading = true
	m.claudeAssistOpen = true
	m.claudeAssistStartedAt = m.claudeNow()
	m.claudeAssistProgress = nil
	m.claudeAssistDraft = ""
	m.claudeAssistEditor = newClaudeAssistEditor("")
	m.claudeAssistEditorReady = true
	m.claudeAssistTarget = claudeAssistTargetDescription
	m.inlineAIOpen = false
	m.inlineAIInstructionOpen = false
	m.claudeAssistEvents = make(chan claude.Event, 16)
	runCtx, cancel := context.WithCancel(context.Background())
	m.claudeAssistCancel = cancel
	m.detailNotice = ""
	m.recordDiagnosticEvent(diagnosticKindClaude, "inline_description_ai", "submit", workerDiagnosticDetail(reqID, key, nil))
	return m, tea.Batch(
		m.submitClaudeTicketAssist(runCtx, reqID, key, m.buildInlineDescriptionAIPrompt(ctx, action, instruction), m.claudeAssistEvents),
		m.waitForClaudeAssistProgress(reqID, key),
		m.scheduleClaudeAssistTick(reqID),
	)
}
```

Add prompt builder with exact output structure:

```go
func (m Model) buildInlineDescriptionAIPrompt(ctx detailRenderContext, action inlineAIAction, instruction string) string {
	var b strings.Builder
	b.WriteString("Provide Description-scoped Jira ticket assistance.\n")
	b.WriteString("Do not update Jira, create tickets, edit files, create branches, run git commands, call GitHub, edit code, or make external changes.\n")
	b.WriteString("Return practical writing help only. The draft must be local TUI text for user review.\n")
	b.WriteString("Selected inline action: ")
	b.WriteString(action.Label)
	b.WriteString("\n")
	if strings.TrimSpace(instruction) != "" {
		b.WriteString("User question/instruction:\n")
		b.WriteString(strings.TrimSpace(instruction))
		b.WriteString("\n")
	}
	b.WriteString("Use this exact high-level structure:\n")
	b.WriteString("Review\n- What changed or what you noticed\n- Risks or open questions\n\n")
	b.WriteString("Draft\n")
	if action.ID == "draft_comment" {
		b.WriteString("<Jira comment draft>\n\n")
	} else {
		b.WriteString("<replacement Description draft>\n\n")
	}
	b.WriteString("Ticket:\n")
	m.writeClaudeTicketContext(&b, ctx)
	return strings.TrimSpace(b.String())
}
```

- [ ] **Step 4: Verify prompt tests pass**

Run:

```bash
go test ./internal/tui -run 'TestInlineDescriptionAI(ImproveClaritySubmitsScopedPrompt|AskQuestionUsesInstruction)' -count=1
```

Expected: PASS.

## Task 4: Result Modal And Description-Only Apply

**Files:**
- Modify: `internal/tui/model_test.go`
- Modify: `internal/tui/model.go`

- [ ] **Step 1: Write failing apply target tests**

Add tests:

```go
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
	for _, want := range []string{"Claude Ticket Assist", "Claude Review", "Local Draft", "Available Actions", "ctrl+s apply", "c comment", "r refine"} {
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
	if strings.Contains(next.render(), "Summary") {
		t.Fatalf("inline Description apply should not preview Summary:\n%s", next.render())
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
	next = updated.(Model)
	if searcher.updatedSummary != "" {
		t.Fatalf("summary should not be updated, got %q", searcher.updatedSummary)
	}
	if searcher.updatedDescription != "Replacement description only." {
		t.Fatalf("description = %q", searcher.updatedDescription)
	}
}
```

If the fake searcher does not yet track `updatedDescription`, add that field and assign it in `UpdateDescription`.

- [ ] **Step 2: Verify apply tests fail**

Run:

```bash
go test ./internal/tui -run 'TestInlineDescriptionAI(ResultUsesTicketAssistModal|ApplyUpdatesDescriptionOnly)' -count=1
```

Expected: FAIL because target-specific apply is not implemented.

- [ ] **Step 3: Implement target-specific apply**

Update `startClaudeTicketAssist()` to set `m.claudeAssistTarget = claudeAssistTargetTicket`.

Update `beginClaudeAssistApply()`:

```go
if m.claudeAssistTarget == claudeAssistTargetDescription {
	description := strings.TrimSpace(m.claudeAssistDraft)
	if description == "" {
		m.detailNotice = "Claude description draft has no text to apply."
		return m, nil
	}
	m.claudeAssistKey = selected.Key
	m.claudeAssistApplySummary = ""
	m.claudeAssistApplyDescription = description
	m.claudeAssistConfirmApply = m.claudeConfig.RequireConfirmation
	if m.claudeAssistConfirmApply {
		return m, nil
	}
	return m.submitClaudeAssistApply()
}
```

Update `renderClaudeAssistApplyConfirmation()` so Description target renders only:

```go
if m.claudeAssistTarget == claudeAssistTargetDescription {
	lines = append(lines, m.theme.FieldLabel.Render("Apply Description Draft"))
	lines = append(lines, m.theme.Muted.Render("Issue: ")+m.theme.Text.Render(displayValue(m.claudeAssistKey, "selected ticket")))
	lines = append(lines, "")
	lines = append(lines, m.theme.Muted.Render("Description"))
	// existing four-line preview loop
	return strings.Join(lines, "\n")
}
```

Update `submitClaudeAssistApply()`:

```go
if m.claudeAssistTarget == claudeAssistTargetDescription {
	if description == "" {
		m.detailNotice = "Claude description draft needs text before applying."
		return m, nil
	}
	m.nextRequestID++
	m.activeClaudeAssistDescriptionReqID = m.nextRequestID
	m.claudeAssistApplying = true
	m.claudeAssistConfirmApply = false
	m.claudeAssistSummaryApplied = true
	m.claudeAssistDescriptionApplied = false
	m.detailNotice = ""
	return m, m.submitUpdateDescription(m.activeClaudeAssistDescriptionReqID, key, description)
}
```

Keep the existing ticket target path unchanged.

- [ ] **Step 4: Verify apply tests pass**

Run:

```bash
go test ./internal/tui -run 'TestInlineDescriptionAI(ResultUsesTicketAssistModal|ApplyUpdatesDescriptionOnly)' -count=1
```

Expected: PASS.

## Task 5: Comment Reuse And Regression Pass

**Files:**
- Modify: `internal/tui/model_test.go`
- Modify: `internal/tui/model.go`

- [ ] **Step 1: Write failing comment reuse test**

Add:

```go
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

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Text: "c"}))
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
	updated, _ = next.Update(result)
	if searcher.addedBody != "Could you confirm the rollback requirement?" {
		t.Fatalf("comment body = %q", searcher.addedBody)
	}
}
```

- [ ] **Step 2: Verify comment test fails or passes intentionally**

Run:

```bash
go test ./internal/tui -run TestInlineDescriptionAIDraftCanBePostedAsComment -count=1
```

Expected: If existing comment path already works with the new target, PASS. If it fails, fix only the target-specific state that blocks comment confirmation/posting.

- [ ] **Step 3: Run focused inline AI suite**

Run:

```bash
go test ./internal/tui -run 'Test(DescriptionFocusShowsInlineAI|DescriptionAKeyOpensInlineAI|InlineAIPicker|InlineDescriptionAI)' -count=1
```

Expected: PASS.

- [ ] **Step 4: Run existing Ticket Assist regression suite**

Run:

```bash
go test ./internal/tui -run 'TestClaudeTicketAssist' -count=1
```

Expected: PASS. This guards normal Ticket Assist Summary+Description apply behavior.

## Task 6: Docs, Changelog, And Final Verification

**Files:**
- Modify: `docs/project-state.md`
- Modify: `docs/releases/CHANGELOG.md`
- Modify: `tasks/todo.md`

- [ ] **Step 1: Update docs**

Add to `docs/project-state.md` near the Ticket Assist behavior:

```markdown
When Description is the focused ticket-detail section and Claude Ticket Assist is enabled and
available, `a` opens `AI for Description` instead of jumping to the Claude tab. The picker can
improve clarity, extract acceptance criteria, answer a user question, or draft a clarifying
comment. Results reuse the Ticket Assist modal, but Description-scoped apply writes only
Description through the worker-backed Jira update path.
```

Add to `docs/releases/CHANGELOG.md` under `Unreleased`:

```markdown
- Added inline Description AI from ticket detail: pressing `a` on Description opens a scoped AI
  picker and returns editable drafts that can be refined, copied, posted as comments, or applied to
  Description behind existing write gates.
```

- [ ] **Step 2: Run full verification**

Run:

```bash
go test ./internal/tui
make check
make install-user
```

Expected: all pass; `/Users/joncha/bin/jira` is rebuilt.

- [ ] **Step 3: Update task review**

In `tasks/todo.md`, mark the inline Description AI checklist complete and add a review section with:

```markdown
- Added Description-scoped inline AI entry with `a` from the Description section.
- Added `AI for Description` picker with improve, extract, ask, and comment actions.
- Reused the existing Claude runner, Ticket Assist modal, refine, copy, comment, and diagnostics flows.
- Added Description-only apply so inline AI cannot change Summary.
- Final verification passed with focused inline AI tests, `go test ./internal/tui`, `make check`, and `make install-user`.
```

## Self-Review

- Spec coverage: Covers Description-focused entry, contextual picker, prompts, result modal reuse, Description-only apply, comment reuse, and verification.
- Placeholder scan: No TBD/TODO placeholders or vague implementation steps remain.
- Type consistency: The plan consistently uses `inlineAIAction`, `claudeAssistTarget`, `claudeAssistTargetTicket`, and `claudeAssistTargetDescription`.
