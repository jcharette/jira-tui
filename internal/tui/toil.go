package tui

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/jcharette/jira-tui/internal/worker"
)

func (m Model) startToilTicket() (Model, tea.Cmd) {
	projectKey := projectKeyFromJQL(m.jql)
	if projectKey == "" {
		projectKey = projectKeyFromJQL(m.filterSummary())
	}
	if projectKey == "" {
		m.detailNotice = "Create toil ticket needs a project key in the active view JQL."
		return m, nil
	}
	m.resetToilState()
	m.toilOpen = true
	m.toilProjectKey = projectKey
	m.toilSummaryEditor = newSummaryEditor("")
	m.toilSummaryEditorReady = true
	m.toilTimeEditor = newWorklogTimeInput("")
	m.toilTimeEditorReady = true
	m.toilNoteEditor = newWorklogCommentEditor("")
	m.toilNoteEditorReady = true
	m.toilCloseAfterCreate = false
	m.toilIssueTypesLoading = true
	m.nextRequestID++
	m.activeToilIssueTypesReqID = m.nextRequestID
	return m, m.submitCreateIssueTypes(m.activeToilIssueTypesReqID, projectKey)
}

func (m *Model) resetToilState() {
	m.toilOpen = false
	m.toilProjectKey = ""
	m.toilIssueTypes = nil
	m.toilIssueTypesLoading = false
	m.toilIssueTypesErr = nil
	m.toilFieldFocus = 0
	m.toilSummaryDraft = ""
	m.toilTimeDraft = ""
	m.toilNoteDraft = ""
	m.toilSummaryEditorReady = false
	m.toilTimeEditorReady = false
	m.toilNoteEditorReady = false
	m.toilCloseAfterCreate = false
	m.toilSubmitting = false
	m.toilLoggingWork = false
	m.toilLoadingTransitions = false
	m.toilClosing = false
	m.toilCreatedKey = ""
	m.toilSubmitIssueType = jira.CreateIssueType{}
	m.toilWorklogRequest = jira.AddWorklogRequest{}
}

func (m Model) renderToilTicket(layout browserLayout) string {
	width := layout.contentWidth
	dialogWidth := min(72, max(40, width-4))
	bodyWidth := max(36, dialogWidth-4)
	lines := []string{}
	switch {
	case m.toilIssueTypesLoading:
		lines = append(lines, m.detailStatusBlock("Loading issue types...", bodyWidth, false))
	case m.toilIssueTypesErr != nil:
		lines = append(lines, m.renderDetailNotice("Issue type metadata failed: "+m.toilIssueTypesErr.Error(), bodyWidth))
	case len(m.toilIssueTypes) == 0:
		lines = append(lines, m.detailEmptyState("Jira returned 0 creatable issue types for "+displayValue(m.toilProjectKey, "this project")+".", bodyWidth))
	default:
		issueType, _ := chooseToilCreateIssueType(m.toilIssueTypes)
		lines = append(lines,
			m.theme.Muted.Render("Project")+" "+m.theme.Key.Render(displayValue(m.toilProjectKey, "unknown")),
			m.theme.Muted.Render("Type")+" "+m.theme.Text.Render(displayValue(issueType.Name, issueType.ID)),
			"",
			m.toilFieldLabel("Summary", 0),
			m.configuredToilSummaryEditor(bodyWidth, 2).View(),
			m.toilFieldLabel("Duration", 1),
			m.configuredToilTimeInput(bodyWidth).View(),
			m.toilFieldLabel("Note", 2),
			m.configuredToilNoteEditor(bodyWidth, 4).View(),
			m.toilFieldLabel("Close after create", 3),
			m.theme.Text.Render(toilToggleLabel(m.toilCloseAfterCreate)),
		)
		if m.toilSubmitting || m.toilLoggingWork || m.toilLoadingTransitions || m.toilClosing {
			lines = append(lines, "", m.theme.Muted.Render(m.toilProgressLabel()))
		}
	}
	if m.detailNotice != "" {
		lines = append(lines, "", m.renderDetailNotice(m.detailNotice, bodyWidth))
	}
	return m.renderDetailDialog(width, "Create Toil Ticket", displayValue(m.toilProjectKey, "Project"), strings.Join(lines, "\n"), "ctrl+s create  tab field  space toggle  esc cancel")
}

func (m Model) toilProgressLabel() string {
	switch {
	case m.toilClosing:
		return "Closing toil ticket."
	case m.toilLoadingTransitions:
		return "Loading close transitions."
	case m.toilLoggingWork:
		return "Logging work."
	case m.toilSubmitting:
		return "Creating toil ticket."
	default:
		return ""
	}
}

func (m Model) toilFieldLabel(label string, index int) string {
	prefix := "  "
	style := m.theme.Muted
	if m.toilFieldFocus == index {
		prefix = "> "
		style = m.theme.Selected
	}
	return style.Render(prefix + label)
}

func toilToggleLabel(value bool) string {
	if value {
		return "[x] yes"
	}
	return "[ ] no"
}

func (m Model) updateToilTicket(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.toilSubmitting || m.toilLoggingWork || m.toilLoadingTransitions || m.toilClosing {
		if msg.String() == "esc" {
			m.detailNotice = "Toil ticket write is already in progress."
		}
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.resetToilState()
		m.detailNotice = ""
		return m, nil
	case "tab":
		m.toilFieldFocus = (m.toilFieldFocus + 1) % 4
		return m, nil
	case "shift+tab", "backtab":
		m.toilFieldFocus = (m.toilFieldFocus + 3) % 4
		return m, nil
	case "space":
		if m.toilFieldFocus == 3 {
			m.toilCloseAfterCreate = !m.toilCloseAfterCreate
			return m, nil
		}
	case "ctrl+s":
		return m.submitToilTicket()
	}
	switch m.toilFieldFocus {
	case 0:
		editor := m.configuredToilSummaryEditor(max(24, m.browserLayout(m.width).contentWidth-12), 2)
		updated, cmd := editor.Update(msg)
		m.toilSummaryEditor = updated
		m.toilSummaryDraft = strings.TrimSpace(updated.Value())
		return m, cmd
	case 1:
		editor := m.configuredToilTimeInput(max(20, m.browserLayout(m.width).contentWidth-12))
		updated, cmd := editor.Update(msg)
		m.toilTimeEditor = updated
		m.toilTimeDraft = strings.TrimSpace(updated.Value())
		return m, cmd
	case 2:
		editor := m.configuredToilNoteEditor(max(24, m.browserLayout(m.width).contentWidth-12), 4)
		updated, cmd := editor.Update(msg)
		m.toilNoteEditor = updated
		m.toilNoteDraft = updated.Value()
		return m, cmd
	}
	return m, nil
}

func (m Model) updateToilPaste(msg tea.PasteMsg) (Model, tea.Cmd) {
	if !m.toilOpen || m.toilSubmitting || m.toilLoggingWork || m.toilLoadingTransitions || m.toilClosing {
		return m, nil
	}
	switch m.toilFieldFocus {
	case 0:
		m.toilSummaryEditor.InsertString(msg.String())
		m.toilSummaryDraft = strings.TrimSpace(m.toilSummaryEditor.Value())
	case 1:
		m.toilTimeEditor.SetValue(m.toilTimeEditor.Value() + msg.String())
		m.toilTimeDraft = strings.TrimSpace(m.toilTimeEditor.Value())
	case 2:
		m.toilNoteEditor.InsertString(msg.String())
		m.toilNoteDraft = m.toilNoteEditor.Value()
	}
	return m, nil
}

func (m Model) configuredToilSummaryEditor(width, rows int) textarea.Model {
	editor := m.toilSummaryEditor
	if !m.toilSummaryEditorReady {
		editor = newSummaryEditor(m.toilSummaryDraft)
	}
	editor.MaxWidth = width
	editor.SetWidth(width)
	editor.SetHeight(max(1, rows))
	return editor
}

func (m Model) configuredToilTimeInput(width int) textinput.Model {
	editor := m.toilTimeEditor
	if !m.toilTimeEditorReady {
		editor = newWorklogTimeInput(m.toilTimeDraft)
	}
	editor.SetWidth(max(10, min(24, width)))
	return editor
}

func (m Model) configuredToilNoteEditor(width, rows int) textarea.Model {
	editor := m.toilNoteEditor
	if !m.toilNoteEditorReady {
		editor = newWorklogCommentEditor(m.toilNoteDraft)
	}
	editor.MaxWidth = width
	editor.SetWidth(width)
	editor.SetHeight(max(2, rows))
	return editor
}

func (m Model) submitToilTicket() (Model, tea.Cmd) {
	if m.toilIssueTypesLoading {
		m.detailNotice = "Wait for issue type metadata before creating toil."
		return m, nil
	}
	issueType, ok := chooseToilCreateIssueType(m.toilIssueTypes)
	if !ok {
		m.detailNotice = "Create toil failed: no usable issue type."
		return m, nil
	}
	summary := strings.TrimSpace(m.toilSummaryDraft)
	if summary == "" && m.toilSummaryEditorReady {
		summary = strings.TrimSpace(m.toilSummaryEditor.Value())
	}
	if summary == "" {
		m.detailNotice = "Summary cannot be empty."
		return m, nil
	}
	timeSpent := strings.TrimSpace(m.toilTimeDraft)
	if timeSpent == "" && m.toilTimeEditorReady {
		timeSpent = strings.TrimSpace(m.toilTimeEditor.Value())
	}
	if !validWorklogDuration(timeSpent) {
		m.detailNotice = "Enter a Jira duration like 30m, 1h, or 1h 30m."
		return m, nil
	}
	note := strings.TrimSpace(m.toilNoteDraft)
	if note == "" && m.toilNoteEditorReady {
		note = strings.TrimSpace(m.toilNoteEditor.Value())
	}
	m.nextRequestID++
	m.activeToilCreateReqID = m.nextRequestID
	m.toilSubmitting = true
	m.toilSubmitIssueType = issueType
	m.toilWorklogRequest = jira.AddWorklogRequest{TimeSpent: timeSpent, Started: m.currentTime(), Comment: note}
	m.detailNotice = ""
	return m, m.submitCreateIssue(m.activeToilCreateReqID, worker.CreateIssueRequest{
		ProjectKey:  m.toilProjectKey,
		IssueTypeID: issueType.ID,
		Summary:     summary,
		Description: note,
		Fields:      []jira.CreateIssueFieldValue{{FieldID: "labels", SchemaSystem: "labels", Text: "toil"}},
	})
}

func (m Model) handleToilIssueTypesResult(result worker.Result) Model {
	if result.ID != m.activeToilIssueTypesReqID {
		return m
	}
	m.toilIssueTypesLoading = false
	if result.Err != nil {
		m.toilIssueTypesErr = result.Err
		return m
	}
	if result.GetCreateIssueTypes == nil {
		m.toilIssueTypesErr = worker.ErrInvalidRequest
		return m
	}
	m.toilIssueTypes = result.GetCreateIssueTypes.IssueTypes
	m.toilIssueTypesErr = nil
	return m
}

func (m Model) handleToilCreateIssueResult(result worker.Result) Model {
	if result.ID != m.activeToilCreateReqID {
		return m
	}
	m.toilSubmitting = false
	if result.Err != nil {
		m.detailNotice = "Create toil failed: " + result.Err.Error()
		return m
	}
	if result.CreateIssue == nil || strings.TrimSpace(result.CreateIssue.Issue.Key) == "" {
		m.detailNotice = "Create toil failed: " + worker.ErrInvalidRequest.Error()
		return m
	}
	issue := result.CreateIssue.Issue
	m.toilCreatedKey = issue.Key
	m.issues = prependIssue(m.issues, issue)
	m.selected = 0
	m.offset = 0
	m.nextRequestID++
	m.activeToilAddWorklogReqID = m.nextRequestID
	m.toilLoggingWork = true
	m.detailNotice = "Created " + issue.Key + "; logging work."
	return m
}

func (m Model) handleToilAddWorklogResult(result worker.Result) (Model, tea.Cmd) {
	if result.ID != m.activeToilAddWorklogReqID {
		return m, nil
	}
	m.toilLoggingWork = false
	if result.Err != nil {
		m.detailNotice = "Toil worklog failed: " + result.Err.Error()
		return m, nil
	}
	if result.AddWorklog == nil {
		m.detailNotice = "Toil worklog failed: " + worker.ErrInvalidRequest.Error()
		return m, nil
	}
	key := displayValue(m.toilCreatedKey, result.AddWorklog.Key)
	if !m.toilCloseAfterCreate {
		m.resetToilState()
		m.detailNotice = "Created " + key + " and logged work."
		return m, nil
	}
	m.nextRequestID++
	m.activeToilTransitionsReqID = m.nextRequestID
	m.toilLoadingTransitions = true
	m.detailNotice = "Created " + key + " and logged work; loading close transitions."
	return m, m.submitIssueTransitions(m.activeToilTransitionsReqID, key)
}

func (m Model) handleToilGetTransitionsResult(result worker.Result) (Model, tea.Cmd) {
	if result.ID != m.activeToilTransitionsReqID {
		return m, nil
	}
	m.toilLoadingTransitions = false
	key := strings.TrimSpace(m.toilCreatedKey)
	if result.Err != nil {
		m.resetToilState()
		m.detailNotice = "Created " + displayValue(key, "toil ticket") + " and logged work; close skipped: " + result.Err.Error() + "."
		return m, nil
	}
	if result.GetTransitions == nil {
		m.resetToilState()
		m.detailNotice = "Created " + displayValue(key, "toil ticket") + " and logged work; close skipped: " + worker.ErrInvalidRequest.Error() + "."
		return m, nil
	}
	transition, ok := chooseSafeTerminalTransition(result.GetTransitions.Transitions)
	if !ok {
		m.resetToilState()
		m.detailNotice = "Created " + displayValue(key, result.GetTransitions.Key) + " and logged work; close skipped: no safe terminal transition available."
		return m, nil
	}
	m.nextRequestID++
	m.activeToilTransitionReqID = m.nextRequestID
	m.toilClosing = true
	m.detailNotice = "Closing " + displayValue(key, result.GetTransitions.Key) + "."
	return m, m.submitIssueTransition(m.activeToilTransitionReqID, displayValue(key, result.GetTransitions.Key), transition, nil)
}

func (m Model) handleToilTransitionResult(result worker.Result) Model {
	if result.ID != m.activeToilTransitionReqID {
		return m
	}
	m.toilClosing = false
	key := strings.TrimSpace(m.toilCreatedKey)
	if result.Err != nil {
		m.resetToilState()
		m.detailNotice = "Created " + displayValue(key, "toil ticket") + " and logged work; close failed: " + result.Err.Error() + "."
		return m
	}
	if result.TransitionIssue == nil {
		m.resetToilState()
		m.detailNotice = "Created " + displayValue(key, "toil ticket") + " and logged work; close failed: " + worker.ErrInvalidRequest.Error() + "."
		return m
	}
	m.updateIssueStatus(result.TransitionIssue.Key, result.TransitionIssue.ToStatus)
	m.resetToilState()
	m.detailNotice = "Created " + displayValue(key, result.TransitionIssue.Key) + ", logged work, and closed as " + displayValue(result.TransitionIssue.ToStatus, "Done") + "."
	return m
}

func chooseToilCreateIssueType(issueTypes []jira.CreateIssueType) (jira.CreateIssueType, bool) {
	for _, issueType := range issueTypes {
		if strings.EqualFold(issueType.Name, "Toil") {
			return issueType, true
		}
	}
	for _, issueType := range issueTypes {
		if !issueType.Subtask {
			return issueType, true
		}
	}
	return jira.CreateIssueType{}, false
}

func chooseSafeTerminalTransition(transitions []jira.Transition) (jira.Transition, bool) {
	bestIndex := -1
	bestScore := 0
	for index, transition := range transitions {
		if !transition.IsAvailable || transitionHasRequiredFields(transition) {
			continue
		}
		score := terminalTransitionScore(transition)
		if score > bestScore {
			bestIndex = index
			bestScore = score
		}
	}
	if bestIndex < 0 {
		return jira.Transition{}, false
	}
	return transitions[bestIndex], true
}

func transitionHasRequiredFields(transition jira.Transition) bool {
	for _, field := range transition.Fields {
		if field.Required {
			return true
		}
	}
	return false
}

func terminalTransitionScore(transition jira.Transition) int {
	text := strings.ToLower(strings.Join([]string{transition.ToStatus, transition.Name}, " "))
	score := 0
	for _, keyword := range []string{"done", "closed", "resolved", "complete", "finished"} {
		if strings.Contains(text, keyword) {
			score += 10
		}
	}
	if strings.EqualFold(strings.TrimSpace(transition.ToStatus), "Done") {
		score += 5
	}
	return score
}

func (m Model) submitToilAddWorklogCmd() tea.Cmd {
	return m.submitAddWorklog(m.activeToilAddWorklogReqID, m.toilCreatedKey, m.toilWorklogRequest)
}

func (m Model) handleToilCreateIssueResultWithCmd(result worker.Result) (Model, tea.Cmd) {
	next := m.handleToilCreateIssueResult(result)
	if next.toilLoggingWork && result.ID == m.activeToilCreateReqID {
		return next, next.submitToilAddWorklogCmd()
	}
	return next, nil
}

func toilBindings() []keyBinding {
	return []keyBinding{
		{Keys: []string{"type"}, Label: "edit", Description: "Edit the active toil field.", Group: "Toil", Footer: true},
		{Keys: []string{"tab"}, Label: "field", Description: "Switch between toil fields.", Group: "Toil", Footer: true},
		{Keys: []string{"space"}, Label: "toggle", Description: "Toggle close after create.", Group: "Toil", Footer: true},
		{Keys: []string{"ctrl+s"}, Label: "create", Description: "Create the toil ticket and log work.", Group: "Toil", Footer: true},
		{Keys: []string{"esc"}, Label: "cancel", Description: "Cancel toil ticket creation.", Group: "Toil", Footer: true},
	}
}
