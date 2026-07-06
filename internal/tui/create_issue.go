package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/jcharette/jira-tui/internal/claude"
	"github.com/jcharette/jira-tui/internal/events"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/jcharette/jira-tui/internal/worker"
)

type createAIPromptResultMsg struct {
	id   int
	text string
	err  error
}

type createAIPromptTickMsg struct {
	id int
}

type createAIPromptProgressMsg struct {
	id    int
	event claude.Event
}

type createAIQuestion struct {
	Question string
	Answer   string
}

func (m Model) handleGetCreateIssueTypesResult(result worker.Result) Model {
	if result.ID != m.activeCreateIssueTypesReqID {
		return m
	}
	m.createIssueTypesLoading = false
	if result.Err != nil {
		m.createIssueTypesErr = result.Err
		m.markCreateIssueTypesCacheError(m.createProjectKey, result.Err)
		return m
	}
	if result.GetCreateIssueTypes == nil {
		m.createIssueTypesErr = worker.ErrInvalidRequest
		return m
	}
	if result.GetCreateIssueTypes.ProjectKey != m.createProjectKey {
		return m
	}
	m.cacheCreateIssueTypes(result.GetCreateIssueTypes.ProjectKey, result.GetCreateIssueTypes.IssueTypes, result.GetCreateIssueTypes.SyncedAt)
	m.selectedCreateIssueType = clamp(m.selectedCreateIssueType, 0, max(0, len(m.selectableCreateIssueTypes())-1))
	m.createIssueTypesErr = nil
	return m
}

func (m Model) handleGetCreateFieldsResult(result worker.Result) Model {
	if result.ID != m.activeCreateFieldsReqID {
		return m
	}
	m.createFieldsLoading = false
	if result.Err != nil {
		m.createFieldsErr = result.Err
		m.markCreateFieldsCacheError(m.createProjectKey, m.createIssueType.ID, result.Err)
		return m
	}
	if result.GetCreateFields == nil {
		m.createFieldsErr = worker.ErrInvalidRequest
		return m
	}
	if result.GetCreateFields.ProjectKey != m.createProjectKey || result.GetCreateFields.IssueTypeID != m.createIssueType.ID {
		return m
	}
	m.cacheCreateFields(result.GetCreateFields.ProjectKey, result.GetCreateFields.IssueTypeID, result.GetCreateFields.Fields, result.GetCreateFields.SyncedAt)
	m.createFieldsErr = nil
	m.beginCreateForm()
	m.applyCreateAIFieldDrafts()
	return m
}

func (m Model) handleSearchFieldOptionsResult(result worker.Result) Model {
	if result.ID != m.activeCreateFieldOptionsReqID {
		return m
	}
	m.ensureCreateFieldOptionsState()
	if result.SearchFieldOptions == nil {
		return m.finishCreateFieldOptionsResult("", workerResultError(result))
	}
	fieldID := strings.TrimSpace(result.SearchFieldOptions.FieldID)
	if fieldID == "" {
		return m
	}
	if result.SearchFieldOptions.Query != m.createFieldOptionsQuery[fieldID] {
		return m
	}
	if result.Err != nil {
		return m.finishCreateFieldOptionsResult(fieldID, result.Err)
	}
	m.createFieldOptionsLoading[fieldID] = false
	m.createFieldOptionsErr[fieldID] = nil
	m.replaceCreateFieldOptions(fieldID, result.SearchFieldOptions.Options)
	if len(result.SearchFieldOptions.Options) > 0 {
		m.createDynamicSelections[fieldID] = 0
	} else {
		m.createDynamicSelections[fieldID] = -1
	}
	return m
}

func workerResultError(result worker.Result) error {
	if result.Err != nil {
		return result.Err
	}
	return worker.ErrInvalidRequest
}

func (m Model) finishCreateFieldOptionsResult(fieldID string, err error) Model {
	if fieldID != "" {
		m.createFieldOptionsLoading[fieldID] = false
		m.createFieldOptionsErr[fieldID] = err
	}
	return m
}

func (m *Model) replaceCreateFieldOptions(fieldID string, options []jira.FieldOption) {
	for index := range m.createFields {
		if createFieldValueKey(m.createFields[index]) == fieldID {
			m.createFields[index].AllowedValues = append([]jira.FieldOption(nil), options...)
			return
		}
	}
}

func (m Model) handleCreateIssueResult(result worker.Result) Model {
	if result.ID != m.activeCreateIssueReqID {
		return m
	}
	m.createSubmitting = false
	if result.Err != nil {
		m.detailNotice = "Create issue failed: " + result.Err.Error()
		return m
	}
	if result.CreateIssue == nil {
		m.detailNotice = "Create issue failed: " + worker.ErrInvalidRequest.Error()
		return m
	}
	created := result.CreateIssue.Issue
	if strings.TrimSpace(created.Key) == "" {
		m.detailNotice = "Create issue failed: empty Jira response."
		return m
	}
	m.createOpen = false
	m.issues = prependIssue(m.issues, created)
	m.selected = 0
	m.offset = 0
	m.mode = modeTable
	m.detailNotice = "Created " + created.Key + "."
	m.resetCreateIssueState()
	return m
}

func (m Model) renderCreateIssue(layout browserLayout) string {
	if m.createAIPromptOpen {
		return m.renderCreateAIPrompt(layout)
	}
	width := layout.contentWidth
	dialogWidth := m.createDialogMaxWidth(width)
	bodyWidth := max(24, dialogWidth-4)
	var lines []string
	focusLine := -1
	subtitle := displayValue(m.createProjectKey, "Project unknown")
	title := "Create Ticket"
	if strings.TrimSpace(m.createParentKey) != "" {
		title = "Create Subtask"
		subtitle = "Parent " + m.createParentKey
		if strings.TrimSpace(m.createParentSummary) != "" {
			subtitle += "  " + truncate(m.createParentSummary, 48)
		}
	}
	footer := "esc cancel"
	switch {
	case m.createIssueTypesLoading:
		lines = append(lines, m.detailStatusBlock("Loading issue types...", bodyWidth, false))
	case m.createIssueTypesErr != nil:
		lines = append(lines, m.renderDetailNotice("Issue type metadata failed: "+m.createIssueTypesErr.Error(), bodyWidth))
	case len(m.selectableCreateIssueTypes()) == 0 && m.createIssueType.ID == "":
		kind := "issue types"
		if strings.TrimSpace(m.createParentKey) != "" {
			kind = "subtask issue types"
		}
		lines = append(lines, m.detailEmptyState("Jira returned 0 creatable "+kind+" for "+displayValue(m.createProjectKey, "this project")+". Press ctrl+d for request diagnostics.", bodyWidth))
	case m.createIssueType.ID == "":
		if m.claudeCreateTicketDraftEnabled() {
			lines = append(lines, m.renderCreateModeTabs(bodyWidth), "")
		}
		if m.createAIGeneratedMode {
			lines = append(lines, m.renderCreateAIPromptBody(bodyWidth)...)
			if m.createAIPromptLoading {
				footer = "esc cancel"
			} else {
				footer = "tab mode  ctrl+s generate  esc cancel"
			}
		} else {
			lines = append(lines, m.renderCreateIssueTypePickerLines()...)
			if m.detailNotice != "" {
				lines = append(lines, "", m.renderDetailNotice(m.detailNotice, bodyWidth))
			}
			if m.claudeCreateTicketDraftEnabled() {
				footer = "tab mode  j/k select  enter continue  esc cancel"
			} else {
				footer = "j/k select  enter continue  esc cancel"
			}
		}
	case m.createFieldsLoading:
		lines = append(lines, m.detailStatusBlock("Loading create fields...", bodyWidth, false))
	case m.createFieldsErr != nil:
		lines = append(lines, m.renderDetailNotice("Create fields failed: "+m.createFieldsErr.Error(), bodyWidth))
	case m.createChangingType:
		lines = append(lines, m.renderCreateIssueTypePickerLines()...)
		if m.detailNotice != "" {
			lines = append(lines, "", m.renderDetailNotice(m.detailNotice, bodyWidth))
		}
		footer = "j/k select  enter change  esc keep"
	default:
		if m.createFieldFocus == createTypeFieldIndex {
			focusLine = len(lines)
		}
		lines = append(lines, m.createFieldLabel("Type", createTypeFieldIndex))
		lines = append(lines, m.theme.Text.Render(displayValue(m.createIssueType.Name, m.createIssueType.ID)))
		if m.createFieldFocus == createTypeFieldIndex {
			lines = append(lines, m.theme.Muted.Render("Press enter to change issue type."))
		}
		lines = append(lines, "")
		if m.createFieldFocus == createSummaryFieldIndex {
			focusLine = len(lines)
		}
		lines = append(lines, m.createFieldLabel("Summary", createSummaryFieldIndex))
		lines = append(lines, m.renderCreateSummaryValue(bodyWidth))
		lines = append(lines, "")
		if m.createFieldFocus == createDescriptionFieldIndex {
			focusLine = len(lines)
		}
		lines = append(lines, m.createFieldLabel("Description", createDescriptionFieldIndex))
		lines = append(lines, m.renderCreateDescriptionValue(bodyWidth))
		if questionsIndex := m.createQuestionsFieldIndex(); questionsIndex >= 0 {
			lines = append(lines, "")
			if m.createFieldFocus == questionsIndex {
				focusLine = len(lines)
			}
			lines = append(lines, m.createFieldLabel("Open Questions", questionsIndex))
			lines = append(lines, m.renderCreateQuestions(bodyWidth))
		}
		if aiFieldIndex := m.createAIPromptFieldIndex(); aiFieldIndex >= 0 {
			lines = append(lines, "")
			if m.createFieldFocus == aiFieldIndex {
				focusLine = len(lines)
			}
			lines = append(lines, m.createFieldLabel("Generate Draft", aiFieldIndex))
			lines = append(lines, m.theme.Muted.Render("Press enter or ctrl+r to refine the current draft with Claude."))
		}
		for index, field := range supportedCreateFields(m.createFields) {
			focusIndex := m.createDynamicFieldFocusIndex(index)
			lines = append(lines, "")
			if m.createFieldFocus == focusIndex {
				focusLine = len(lines)
				lines = append(lines, m.createFieldLabel(displayValue(field.Name, field.ID), focusIndex))
				lines = append(lines, m.renderCreateDynamicField(field, bodyWidth))
				continue
			}
			lines = append(lines, m.renderCreateDynamicField(field, bodyWidth))
		}
		if unsupported := unsupportedRequiredCreateFields(m.createFields); len(unsupported) > 0 {
			lines = append(lines, "", m.renderDetailNotice("Jira may require more fields: "+strings.Join(unsupported, ", "), bodyWidth))
		}
		if m.createSubmitting {
			lines = append(lines, "", m.detailStatusBlock("Creating ticket...", bodyWidth, false))
		}
		if m.detailNotice != "" {
			lines = append(lines, "", m.renderDetailNotice(m.detailNotice, bodyWidth))
		}
		if m.claudeCreateTicketDraftEnabled() {
			footer = "tab field  enter generate  ctrl+r refine  ctrl+s create  esc cancel"
		} else {
			footer = "tab field  ctrl+s create  esc cancel"
		}
	}
	body := m.windowCreateBody(lines, bodyWidth, focusLine)
	return m.renderDetailDialogWithLimit(width, title, subtitle, body, footer, dialogWidth)
}

func (m Model) windowCreateBody(lines []string, width int, focusLine int) string {
	rows := m.createBodyRows()
	if len(lines) <= rows {
		return strings.Join(lines, "\n")
	}
	offset := 0
	if focusLine >= 0 {
		offset = clamp(focusLine-rows/2, 0, max(0, len(lines)-rows))
	}
	end := min(len(lines), offset+rows)
	visible := append([]string(nil), lines[offset:end]...)
	indicator := fmt.Sprintf("Create Lines %d-%d of %d", offset+1, end, len(lines))
	visible = append(visible, m.theme.Muted.Render(truncate(indicator, width)))
	return strings.Join(visible, "\n")
}

func (m Model) createBodyRows() int {
	return max(6, m.boundedPanelBodyRows(11))
}

func (m Model) createDescriptionEditorRows() int {
	return clamp((m.createBodyRows()*2)/3, 10, 18)
}

func (m Model) createSummaryEditorRows() int {
	return 3
}

func (m Model) createDialogMaxWidth(width int) int {
	if width <= 0 {
		return 72
	}
	return clamp((width*86)/100, 72, min(width, 132))
}

func (m Model) renderCreateModeTabs(width int) string {
	manual := "Manual"
	generated := "AI Generated"
	if m.createAIGeneratedMode {
		manual = m.theme.Muted.Render("  " + manual)
		generated = m.theme.Selected.Render("> " + generated)
	} else {
		manual = m.theme.Selected.Render("> " + manual)
		generated = m.theme.Muted.Render("  " + generated)
	}
	line := manual + "  " + generated
	if width > 0 && lipgloss.Width(line) > width {
		if m.createAIGeneratedMode {
			return m.theme.Selected.Render("> AI")
		}
		return m.theme.Selected.Render("> Manual")
	}
	return line
}

func (m Model) renderCreateIssueTypePickerLines() []string {
	issueTypes := m.selectableCreateIssueTypes()
	cursor := clamp(m.selectedCreateIssueType, 0, len(issueTypes)-1)
	options := make([]choiceListOption, 0, len(issueTypes))
	for _, issueType := range issueTypes {
		options = append(options, choiceListOption{Label: displayValue(issueType.Name, issueType.ID)})
	}
	return m.renderChoiceList(options, cursor, 60, createPickerMaxRows)
}

func (m Model) renderCreateAIPrompt(layout browserLayout) string {
	width := layout.contentWidth
	dialogWidth := m.createDialogMaxWidth(width)
	bodyWidth := max(24, dialogWidth-4)
	lines := m.renderCreateAIPromptBody(bodyWidth)
	subtitle := m.createProjectKey
	if strings.TrimSpace(m.createIssueType.Name) != "" {
		subtitle = subtitle + "  " + m.createIssueType.Name
	}
	footer := "ctrl+s generate  esc cancel"
	if m.createAIPromptLoading {
		footer = "esc cancel"
	} else if m.createAIPromptErr == nil {
		footer = "ctrl+s generate  esc cancel"
	}
	return m.renderDetailDialogWithLimit(width, "Generate Ticket Draft", displayValue(subtitle, "No project"), strings.Join(lines, "\n"), footer, dialogWidth)
}

func (m Model) renderCreateAIPromptBody(bodyWidth int) []string {
	lines := []string{
		m.theme.FieldLabel.Render("Prompt"),
		"",
	}
	if m.createAIPromptLoading {
		lines = append(lines, m.renderClaudeProgressStatus(m.createAIPromptProgress)...)
		lines = append(lines, "", m.theme.Muted.Render("Elapsed: "+formatClaudeDuration(m.claudeNow().Sub(m.createAIPromptStartedAt))))
		lines = append(lines, m.theme.Muted.Render("Claude is drafting. Press esc to cancel."))
	} else if m.createAIPromptErr != nil {
		lines = append(lines, m.renderDetailNotice("Draft generation failed: "+m.createAIPromptErr.Error(), bodyWidth))
		lines = append(lines, "")
		lines = append(lines, m.configuredCreateAIPromptEditor(bodyWidth, 8).View())
	} else {
		lines = append(lines, m.configuredCreateAIPromptEditor(bodyWidth, 8).View())
	}
	if m.detailNotice != "" && !m.createAIPromptLoading {
		lines = append(lines, "", m.renderDetailNotice(m.detailNotice, bodyWidth))
	}
	return lines
}

func (m Model) createFieldLabel(label string, index int) string {
	style := m.theme.Muted
	if m.createFieldFocus == index {
		style = m.theme.PaneTitle
	}
	return style.Render(label)
}

func (m Model) renderCreateSummaryValue(width int) string {
	if m.createFieldFocus == createSummaryFieldIndex {
		return m.configuredCreateSummaryEditor(width, m.createSummaryEditorRows()).View()
	}
	value := strings.TrimSpace(m.createSummaryDraft)
	if value == "" {
		value = "Edit summary..."
	}
	return m.theme.Muted.Render(truncate(value, width))
}

func (m Model) renderCreateDescriptionValue(width int) string {
	if m.createFieldFocus == createDescriptionFieldIndex {
		return m.configuredCreateDescriptionEditor(width, m.createDescriptionEditorRows()).View()
	}
	value := strings.TrimSpace(m.createDescriptionDraft)
	if value == "" {
		value = "Write a Jira comment..."
	}
	return m.theme.Muted.Render(truncate(value, width))
}

func (m Model) renderCreateQuestions(width int) string {
	if len(m.createAIQuestions) == 0 {
		return ""
	}
	selected := clamp(m.selectedCreateAIQuestion, 0, len(m.createAIQuestions)-1)
	var lines []string
	for index, question := range m.createAIQuestions {
		prefix := " "
		style := m.theme.Text
		if m.createFieldFocus == m.createQuestionsFieldIndex() && index == selected {
			prefix = ">"
			style = m.theme.Selected
		}
		status := ""
		if strings.TrimSpace(question.Answer) != "" {
			status = "answered"
		}
		parts := []string{prefix}
		if status != "" {
			parts = append(parts, status)
		}
		parts = append(parts, question.Question)
		line := strings.TrimSpace(strings.Join(parts, " "))
		lines = append(lines, style.Render(truncate(line, width)))
		if m.createAIQuestionAnswering && index == selected {
			lines = append(lines, m.configuredCreateQuestionAnswerEditor(width, 4).View())
		}
	}
	if m.createFieldFocus == m.createQuestionsFieldIndex() && !m.createAIQuestionAnswering {
		lines = append(lines, m.theme.Muted.Render("enter answer  j/k select  ctrl+r refine with answers"))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderCreateDynamicField(field jira.CreateField, width int) string {
	if focused, ok := m.focusedCreateDynamicField(); !ok || createFieldValueKey(focused) != createFieldValueKey(field) {
		return m.renderCreateDynamicFieldSummary(field, width)
	}
	if createFieldUsesPicker(field) {
		if len(field.AllowedValues) == 0 {
			key := createFieldValueKey(field)
			if strings.TrimSpace(field.AutoCompleteURL) != "" {
				if m.createFieldOptionsLoading[key] {
					return m.detailStatusBlock("Loading Jira options...", width, false)
				}
				if err := m.createFieldOptionsErr[key]; err != nil {
					return m.renderDetailNotice("Option lookup failed: "+err.Error(), width)
				}
				if strings.TrimSpace(m.createDynamicFilters[key]) == "" {
					return m.theme.Muted.Render("Type to search Jira options.")
				}
			}
			return m.detailEmptyState("No Jira options available.", width)
		}
		key := createFieldValueKey(field)
		filter := m.createDynamicFilters[key]
		matches := filteredCreateFieldOptionIndexes(field.AllowedValues, filter)
		if len(matches) == 0 {
			return strings.Join([]string{
				m.theme.Muted.Render("Filter: " + filter),
				m.detailEmptyState("No Jira options matched.", width),
			}, "\n")
		}
		selected := m.createDynamicSelections[key]
		matchPosition := createOptionMatchPosition(matches, selected)
		if matchPosition < 0 {
			matchPosition = 0
		}
		var lines []string
		if strings.TrimSpace(filter) != "" {
			lines = append(lines, m.theme.Muted.Render("Filter: "+filter))
		}
		options := make([]choiceListOption, 0, len(matches))
		for _, optionIndex := range matches {
			option := field.AllowedValues[optionIndex]
			options = append(options, choiceListOption{Label: displayValue(option.Name, option.ID)})
		}
		lines = append(lines, m.renderChoiceList(options, matchPosition, width, createPickerMaxRows)...)
		return strings.Join(lines, "\n")
	}
	value := m.createDynamicValues[createFieldValueKey(field)]
	if strings.TrimSpace(value) == "" {
		value = " "
	}
	return m.theme.Text.Render(truncate(value, width))
}

func (m Model) renderCreateDynamicFieldSummary(field jira.CreateField, width int) string {
	label := displayValue(field.Name, field.ID)
	value := ""
	if createFieldUsesPicker(field) {
		selected := m.createDynamicSelections[createFieldValueKey(field)]
		if selected >= 0 && selected < len(field.AllowedValues) {
			value = displayValue(field.AllowedValues[selected].Name, field.AllowedValues[selected].ID)
		}
	} else {
		value = strings.TrimSpace(m.createDynamicValues[createFieldValueKey(field)])
	}
	if value == "" {
		value = "-"
	}
	line := label + ": " + value
	return m.theme.Muted.Render(truncate(line, width))
}

func parseCreateIssueDraft(text string) (summary string, description string) {
	lines := strings.Split(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"), "\n")
	summaryIndex := -1
	descriptionIndex := -1
	for index, line := range lines {
		header := strings.TrimLeft(strings.TrimSpace(line), "#")
		header = strings.TrimSpace(strings.Trim(header, ":"))
		header = strings.ToLower(header)
		switch {
		case strings.HasPrefix(header, "summary"):
			if summaryIndex < 0 {
				summaryIndex = index
			}
		case strings.HasPrefix(header, "description"):
			if descriptionIndex < 0 {
				descriptionIndex = index
			}
		}
	}
	if summaryIndex < 0 && descriptionIndex < 0 {
		return "", ""
	}
	extractAfterHeader := func(start int) string {
		if start < 0 {
			return ""
		}
		line := strings.TrimSpace(lines[start])
		if i := strings.Index(line, ":"); i >= 0 {
			if value := strings.TrimSpace(line[i+1:]); value != "" {
				return value
			}
		}
		parts := make([]string, 0, len(lines)-start-1)
		for index := start + 1; index < len(lines); index++ {
			candidate := strings.TrimSpace(lines[index])
			lower := strings.ToLower(strings.Trim(candidate, "#:"))
			if strings.HasPrefix(lower, "summary") || strings.HasPrefix(lower, "description") || strings.HasPrefix(lower, "acceptance criteria") || strings.HasPrefix(lower, "open questions") || strings.HasPrefix(lower, "test / verification") || strings.HasPrefix(lower, "implementation notes") {
				break
			}
			if candidate != "" || len(parts) > 0 {
				parts = append(parts, lines[index])
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	}
	summary = strings.TrimSpace(extractAfterHeader(summaryIndex))
	description = strings.TrimSpace(extractAfterHeader(descriptionIndex))
	if summary == "" && summaryIndex >= 0 {
		summary = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[summaryIndex]), "Summary"))
	}
	if description == "" && summaryIndex >= 0 && descriptionIndex < 0 && summaryIndex+1 < len(lines) {
		description = strings.TrimSpace(strings.TrimSpace(strings.Join(lines[summaryIndex+1:], "\n")))
	}
	return summary, description
}

func parseCreateIssueDraftFields(text string) map[string]string {
	lines := strings.Split(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"), "\n")
	sections := map[string]string{}
	for index := 0; index < len(lines); index++ {
		line := strings.TrimSpace(lines[index])
		if line == "" || strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			continue
		}
		label := ""
		value := ""
		if before, after, ok := strings.Cut(line, ":"); ok && len(strings.TrimSpace(before)) <= 40 {
			label = strings.TrimSpace(strings.Trim(before, "#* "))
			value = strings.TrimSpace(after)
		} else if isCreateDraftFieldHeader(line) {
			label = strings.TrimSpace(strings.Trim(line, "#* "))
		}
		if label == "" {
			continue
		}
		var values []string
		if value != "" {
			values = append(values, value)
		}
		for next := index + 1; next < len(lines); next++ {
			candidate := strings.TrimSpace(lines[next])
			if candidate == "" {
				if len(values) > 0 {
					break
				}
				continue
			}
			if isCreateDraftFieldHeader(candidate) || isCreateDraftInlineField(candidate) {
				break
			}
			values = append(values, strings.TrimPrefix(strings.TrimPrefix(candidate, "- "), "* "))
			index = next
		}
		if len(values) > 0 {
			sections[normalizeCreateDraftFieldName(label)] = strings.TrimSpace(strings.Join(values, "\n"))
		}
	}
	return sections
}

func parseCreateIssueOpenQuestions(text string) []createAIQuestion {
	fields := parseCreateIssueDraftFields(text)
	value := strings.TrimSpace(fields["openquestions"])
	if value == "" {
		return nil
	}
	var questions []createAIQuestion
	for _, line := range strings.Split(value, "\n") {
		question := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* "))
		if question == "" || strings.EqualFold(question, "none") || strings.EqualFold(question, "n/a") {
			continue
		}
		questions = append(questions, createAIQuestion{Question: question})
	}
	return questions
}

func isCreateDraftInlineField(line string) bool {
	before, _, ok := strings.Cut(strings.TrimSpace(line), ":")
	return ok && len(strings.TrimSpace(before)) <= 40
}

func isCreateDraftFieldHeader(line string) bool {
	line = strings.TrimSpace(strings.Trim(line, "#* "))
	if line == "" || len(line) > 48 {
		return false
	}
	if strings.ContainsAny(line, ".?!") {
		return false
	}
	switch normalizeCreateDraftFieldName(line) {
	case "issuetype", "summary", "description", "components", "component", "priority", "labels", "label", "investmentcategory", "releaseinstructions", "openquestions":
		return true
	default:
		return false
	}
}

func normalizeCreateDraftFieldName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (m Model) startCreateIssue() (Model, tea.Cmd) {
	return m.startCreateIssueWithParent("", "")
}

func (m Model) startCreateIssueWithParent(parentKey string, parentSummary string) (Model, tea.Cmd) {
	projectKey := projectKeyFromJQL(m.jql)
	if project := projectKeyFromIssueKey(parentKey); project != "" {
		projectKey = project
	}
	if projectKey == "" {
		projectKey = projectKeyFromJQL(m.filterSummary())
	}
	if projectKey == "" {
		m.detailNotice = "Create ticket needs a project key in the active view JQL."
		return m, nil
	}
	m.resetCreateIssueState()
	m.createOpen = true
	m.createProjectKey = projectKey
	m.createParentKey = strings.TrimSpace(parentKey)
	m.createParentSummary = strings.TrimSpace(parentSummary)
	m.hydrateCreateIssueTypes(projectKey)
	if _, cached := m.cachedCreateIssueTypes(projectKey); cached && m.isCreateIssueTypesFresh(projectKey) {
		m.selectedCreateIssueType = clamp(m.selectedCreateIssueType, 0, max(0, len(m.selectableCreateIssueTypes())-1))
		m.createIssueTypesErr = nil
		return m, nil
	}
	m.createIssueTypesLoading = true
	m.nextRequestID++
	m.activeCreateIssueTypesReqID = m.nextRequestID
	return m, m.submitCreateIssueTypes(m.activeCreateIssueTypesReqID, projectKey)
}

func (m Model) startCreateIssueWithParentDraft(parentKey string, parentSummary string, summary string, description string) (Model, tea.Cmd) {
	m, cmd := m.startCreateIssueWithParent(parentKey, parentSummary)
	if !m.createOpen {
		return m, cmd
	}
	m.createSummaryDraft = strings.TrimSpace(summary)
	m.createDescriptionDraft = strings.TrimSpace(description)
	m.createSummaryEditor = newSummaryEditor(m.createSummaryDraft)
	m.createSummaryEditorReady = true
	m.createDescriptionEditor = newCommentEditor(m.createDescriptionDraft)
	m.createDescriptionEditorReady = true
	m.createFieldFocus = createSummaryFieldIndex
	return m, cmd
}

func (m Model) selectableCreateIssueTypes() []jira.CreateIssueType {
	if strings.TrimSpace(m.createParentKey) == "" {
		return m.createIssueTypes
	}
	issueTypes := make([]jira.CreateIssueType, 0, len(m.createIssueTypes))
	for _, issueType := range m.createIssueTypes {
		if issueType.Subtask {
			issueTypes = append(issueTypes, issueType)
		}
	}
	return issueTypes
}

func (m Model) updateCreatePaste(msg tea.PasteMsg) (Model, tea.Cmd) {
	if !m.createFormReady() || m.createSubmitting {
		return m, nil
	}
	if m.createAIQuestionAnswering && m.createQuestionsFieldFocused() {
		m.ensureCreateQuestionAnswerEditor()
		m.createAIQuestionEditor.InsertString(msg.String())
	} else if m.createFieldFocus == createSummaryFieldIndex {
		m.ensureCreateSummaryEditor()
		m.createSummaryEditor.InsertString(msg.String())
		m.createSummaryDraft = m.createSummaryEditor.Value()
	} else if m.createFieldFocus == createDescriptionFieldIndex {
		m.ensureCreateDescriptionEditor()
		m.createDescriptionEditor.InsertString(msg.String())
		m.createDescriptionDraft = m.createDescriptionEditor.Value()
	} else if field, ok := m.focusedCreateDynamicField(); ok && !createFieldUsesPicker(field) {
		m.setCreateDynamicValue(field, m.createDynamicValues[createFieldValueKey(field)]+msg.String())
	}
	m.detailNotice = ""
	return m, nil
}

func (m Model) updateCreateIssue(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" && m.createIssueType.ID == "" && m.createAIGeneratedMode && m.createAIPromptLoading {
		return m.cancelCreateAIPrompt(), nil
	}
	if msg.String() == "esc" && m.createAIQuestionAnswering {
		m.createAIQuestionAnswering = false
		m.createAIQuestionEditorReady = false
		m.detailNotice = ""
		return m, nil
	}
	if msg.String() == "esc" && m.createChangingType {
		m.createChangingType = false
		m.detailNotice = ""
		return m, nil
	}
	if msg.String() == "esc" {
		if field, ok := m.focusedCreateDynamicField(); ok && createFieldUsesPicker(field) {
			key := createFieldValueKey(field)
			if strings.TrimSpace(m.createDynamicFilters[key]) != "" {
				m.clearCreateDynamicFilter(field)
				m.detailNotice = ""
				return m, nil
			}
		}
	}
	switch msg.String() {
	case "esc":
		m.resetCreateIssueState()
		return m, nil
	}
	if m.createIssueType.ID == "" {
		if msg.String() == "tab" && m.claudeCreateTicketDraftEnabled() {
			m.createAIGeneratedMode = !m.createAIGeneratedMode
			m.detailNotice = ""
			if m.createAIGeneratedMode {
				m.ensureCreateAIPromptEditor()
			}
			return m, nil
		}
		if m.createAIGeneratedMode {
			if m.createAIPromptLoading {
				return m, nil
			}
			if msg.String() == "ctrl+s" {
				return m.submitCreateAIPrompt()
			}
			m.configureCreateAIPromptEditor(max(32, m.browserLayout(m.width).contentWidth-16), 8)
			editor, cmd := m.createAIPromptEditor.Update(msg)
			m.createAIPromptEditor = editor
			m.createAIPrompt = m.createAIPromptEditor.Value()
			return m, cmd
		}
		switch msg.String() {
		case "up", "k":
			m.moveSelectedCreateIssueType(-1)
			return m, nil
		case "down", "j":
			m.moveSelectedCreateIssueType(1)
			return m, nil
		case "g", "G":
			m.detailNotice = "Select an issue type before generating a ticket draft."
			return m, nil
		case "enter":
			return m.selectCreateIssueType()
		default:
			return m, nil
		}
	}
	if m.createChangingType {
		switch msg.String() {
		case "esc":
			m.createChangingType = false
			m.detailNotice = ""
			return m, nil
		case "up", "k":
			m.moveSelectedCreateIssueType(-1)
			return m, nil
		case "down", "j":
			m.moveSelectedCreateIssueType(1)
			return m, nil
		case "enter":
			m.createChangingType = false
			return m.selectCreateIssueType()
		default:
			return m, nil
		}
	}
	if !m.createFormReady() || m.createSubmitting {
		return m, nil
	}
	if m.createQuestionsFieldFocused() && m.createAIQuestionAnswering {
		return m.updateCreateQuestions(msg)
	}
	if m.createQuestionsFieldFocused() {
		switch msg.String() {
		case "up", "k", "down", "j":
			return m.updateCreateQuestions(msg)
		case "ctrl+r":
			return m.submitCreateQuestionRefinement()
		}
	}
	if field, ok := m.focusedCreateDynamicField(); ok && createFieldUsesPicker(field) {
		switch msg.String() {
		case "up", "k":
			m.moveCreateDynamicSelection(field, -1)
			return m, nil
		case "down", "j":
			m.moveCreateDynamicSelection(field, 1)
			return m, nil
		case "enter":
			m.clearCreateDynamicFilter(field)
			return m, nil
		case "backspace", "ctrl+h", "left", "right", "home", "end":
			cmd := m.updateCreateDynamicFilter(field, msg)
			optionCmd := m.requestCreateFieldOptions(field)
			return m, tea.Batch(cmd, optionCmd)
		}
		if len(msg.String()) == 1 {
			cmd := m.updateCreateDynamicFilter(field, msg)
			optionCmd := m.requestCreateFieldOptions(field)
			return m, tea.Batch(cmd, optionCmd)
		}
	}
	switch msg.String() {
	case "tab":
		m.moveCreateFieldFocus(1)
		return m, m.requestFocusedCreateFieldOptions()
	case "shift+tab", "backtab":
		m.moveCreateFieldFocus(-1)
		return m, m.requestFocusedCreateFieldOptions()
	case "enter":
		if m.createFieldFocus == createTypeFieldIndex {
			m.startCreateIssueTypeChange()
			return m, nil
		}
		if m.createAIPromptFieldFocused() {
			return m.startCreateAIPrompt()
		}
		if m.createQuestionsFieldFocused() {
			return m.updateCreateQuestions(msg)
		}
	case "ctrl+r":
		return m.submitCreateDraftRefinement()
	case "ctrl+s":
		return m.submitCreateIssueDraft()
	}
	if m.createFieldFocus == createSummaryFieldIndex {
		m.ensureCreateSummaryEditor()
		m.configureCreateSummaryEditor()
		editor, cmd := m.createSummaryEditor.Update(msg)
		m.createSummaryEditor = editor
		m.createSummaryDraft = m.createSummaryEditor.Value()
		m.detailNotice = ""
		return m, cmd
	}
	if field, ok := m.focusedCreateDynamicField(); ok {
		if createFieldUsesPicker(field) {
			return m, nil
		}
		switch msg.String() {
		case "backspace", "ctrl+h":
			value := []rune(m.createDynamicValues[createFieldValueKey(field)])
			if len(value) > 0 {
				m.setCreateDynamicValue(field, string(value[:len(value)-1]))
			}
			return m, nil
		}
		if len(msg.String()) == 1 {
			m.setCreateDynamicValue(field, m.createDynamicValues[createFieldValueKey(field)]+msg.String())
			return m, nil
		}
		return m, nil
	}
	m.ensureCreateDescriptionEditor()
	m.configureCreateDescriptionEditor()
	editor, cmd := m.createDescriptionEditor.Update(msg)
	m.createDescriptionEditor = editor
	m.createDescriptionDraft = m.createDescriptionEditor.Value()
	m.detailNotice = ""
	return m, cmd
}

func (m Model) updateCreateQuestions(msg tea.KeyMsg) (Model, tea.Cmd) {
	if len(m.createAIQuestions) == 0 {
		return m, nil
	}
	if m.createAIQuestionAnswering {
		switch msg.String() {
		case "enter":
			m.saveCreateQuestionAnswer()
			if m.selectedCreateAIQuestion < len(m.createAIQuestions)-1 {
				m.selectedCreateAIQuestion++
				m.createAIQuestionEditor = newCreateQuestionAnswerEditor(m.createAIQuestions[m.selectedCreateAIQuestion].Answer)
				m.createAIQuestionEditorReady = true
				m.createAIQuestionAnswering = true
				m.detailNotice = "Saved answer. Next question."
				return m, nil
			}
			m.createAIQuestionAnswering = false
			m.createAIQuestionEditorReady = false
			m.detailNotice = "Saved final question answer locally."
			return m, nil
		case "ctrl+s":
			m.saveCreateQuestionAnswer()
			m.createAIQuestionAnswering = false
			m.createAIQuestionEditorReady = false
			m.detailNotice = "Saved question answer locally."
			return m, nil
		}
		m.ensureCreateQuestionAnswerEditor()
		editor, cmd := m.createAIQuestionEditor.Update(msg)
		m.createAIQuestionEditor = editor
		m.detailNotice = ""
		return m, cmd
	}
	switch msg.String() {
	case "up", "k":
		m.selectedCreateAIQuestion = clamp(m.selectedCreateAIQuestion-1, 0, len(m.createAIQuestions)-1)
		return m, nil
	case "down", "j":
		m.selectedCreateAIQuestion = clamp(m.selectedCreateAIQuestion+1, 0, len(m.createAIQuestions)-1)
		return m, nil
	case "enter":
		selected := clamp(m.selectedCreateAIQuestion, 0, len(m.createAIQuestions)-1)
		m.createAIQuestionEditor = newCreateQuestionAnswerEditor(m.createAIQuestions[selected].Answer)
		m.createAIQuestionEditorReady = true
		m.createAIQuestionAnswering = true
		m.detailNotice = ""
		return m, nil
	}
	return m, nil
}

func (m Model) submitCreateQuestionRefinement() (Model, tea.Cmd) {
	m.createAIPrompt = "Refine the current ticket draft using my answers to the Open Questions."
	return m.submitCreateAIPrompt()
}

func (m Model) submitCreateDraftRefinement() (Model, tea.Cmd) {
	if strings.TrimSpace(m.createIssueAICurrentDraft()) == "" {
		m.detailNotice = "Add a summary or description before refining with Claude."
		return m, nil
	}
	m.createAIPrompt = "Refine the current ticket draft."
	return m.submitCreateAIPrompt()
}

func (m *Model) saveCreateQuestionAnswer() {
	if len(m.createAIQuestions) == 0 {
		return
	}
	selected := clamp(m.selectedCreateAIQuestion, 0, len(m.createAIQuestions)-1)
	m.createAIQuestions[selected].Answer = strings.TrimSpace(m.createAIQuestionEditor.Value())
}

func (m *Model) moveSelectedCreateIssueType(delta int) {
	issueTypes := m.selectableCreateIssueTypes()
	if len(issueTypes) == 0 {
		m.selectedCreateIssueType = 0
		return
	}
	m.selectedCreateIssueType = clamp(m.selectedCreateIssueType+delta, 0, len(issueTypes)-1)
}

func (m Model) selectCreateIssueType() (Model, tea.Cmd) {
	issueTypes := m.selectableCreateIssueTypes()
	if len(issueTypes) == 0 {
		m.detailNotice = "No Jira issue types are available."
		return m, nil
	}
	m.createIssueType = issueTypes[clamp(m.selectedCreateIssueType, 0, len(issueTypes)-1)]
	m.createChangingType = false
	if strings.TrimSpace(m.createIssueType.ID) == "" {
		m.detailNotice = "Create ticket failed: missing issue type ID."
		return m, nil
	}
	m.hydrateCreateFields(m.createProjectKey, m.createIssueType.ID)
	if _, cached := m.cachedCreateFields(m.createProjectKey, m.createIssueType.ID); cached && m.isCreateFieldsFresh(m.createProjectKey, m.createIssueType.ID) {
		m.createFieldsErr = nil
		m.beginCreateForm()
		m.applyCreateAIFieldDrafts()
		return m, nil
	}
	m.createFieldsLoading = true
	m.createFieldsErr = nil
	m.nextRequestID++
	m.activeCreateFieldsReqID = m.nextRequestID
	return m, m.submitCreateFields(m.activeCreateFieldsReqID, m.createProjectKey, m.createIssueType.ID)
}

func (m *Model) startCreateIssueTypeChange() {
	m.createChangingType = true
	m.detailNotice = ""
	for index, issueType := range m.selectableCreateIssueTypes() {
		if createIssueTypeMatches(issueType, m.createIssueType.Name) || createIssueTypeMatches(issueType, m.createIssueType.ID) {
			m.selectedCreateIssueType = index
			return
		}
	}
}

func (m *Model) beginCreateForm() {
	m.createChangingType = false
	m.createFieldFocus = createSummaryFieldIndex
	m.createSummaryEditor = newSummaryEditor(m.createSummaryDraft)
	m.createSummaryEditorReady = true
	m.createDescriptionEditor = newCommentEditor(m.createDescriptionDraft)
	m.createDescriptionEditorReady = true
	m.createDynamicValues = map[string]string{}
	m.createDynamicSelections = map[string]int{}
	m.createDynamicFilters = map[string]string{}
	m.createDynamicFilterEditors = map[string]textinput.Model{}
	m.createFieldOptionsLoading = map[string]bool{}
	m.createFieldOptionsErr = map[string]error{}
	m.createFieldOptionsQuery = map[string]string{}
	for _, field := range supportedCreateFields(m.createFields) {
		key := createFieldValueKey(field)
		m.createDynamicValues[key] = ""
		m.createDynamicSelections[key] = defaultCreateFieldSelection(field)
		m.createDynamicFilters[key] = ""
		if createFieldUsesPicker(field) {
			m.createDynamicFilterEditors[key] = newCreateFilterInput("")
		}
	}
	m.detailNotice = ""
}

func (m *Model) applyCreateAIFieldDrafts() {
	if len(m.createAIFieldDrafts) == 0 {
		return
	}
	for _, field := range supportedCreateFields(m.createFields) {
		value, ok := m.createAIFieldDraftFor(field)
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		key := createFieldValueKey(field)
		if createFieldUsesPicker(field) {
			if index, ok := bestCreateFieldOptionMatch(value, field.AllowedValues); ok {
				m.createDynamicSelections[key] = index
			}
			continue
		}
		m.setCreateDynamicValue(field, value)
	}
}

func (m Model) createAIFieldDraftFor(field jira.CreateField) (string, bool) {
	for _, candidate := range []string{field.Name, field.ID, field.Key, field.SchemaSystem} {
		key := normalizeCreateDraftFieldName(candidate)
		if key == "" {
			continue
		}
		if value, ok := m.createAIFieldDrafts[key]; ok {
			return value, true
		}
	}
	return "", false
}

func bestCreateFieldOptionMatch(value string, options []jira.FieldOption) (int, bool) {
	normalizedValue := normalizeCreateDraftFieldName(value)
	if normalizedValue == "" {
		return 0, false
	}
	for index, option := range options {
		optionName := normalizeCreateDraftFieldName(option.Name)
		optionID := normalizeCreateDraftFieldName(option.ID)
		if optionName != "" && (strings.Contains(normalizedValue, optionName) || strings.Contains(optionName, normalizedValue)) {
			return index, true
		}
		if optionID != "" && (strings.Contains(normalizedValue, optionID) || strings.Contains(optionID, normalizedValue)) {
			return index, true
		}
	}
	return 0, false
}

func createIssueTypeMatches(issueType jira.CreateIssueType, value string) bool {
	normalizedValue := normalizeCreateDraftFieldName(value)
	if normalizedValue == "" {
		return false
	}
	for _, candidate := range []string{issueType.Name, issueType.ID} {
		if normalizeCreateDraftFieldName(candidate) == normalizedValue {
			return true
		}
	}
	return false
}

func projectKeyFromIssueKey(issueKey string) string {
	before, _, ok := strings.Cut(strings.TrimSpace(issueKey), "-")
	if !ok {
		return ""
	}
	return strings.TrimSpace(before)
}

func (m *Model) moveCreateFieldFocus(delta int) {
	total := 3 + len(supportedCreateFields(m.createFields))
	if len(m.createAIQuestions) > 0 {
		total++
	}
	if m.claudeCreateTicketDraftEnabled() {
		total++
	}
	if total <= 0 {
		m.createFieldFocus = createSummaryFieldIndex
		return
	}
	m.createFieldFocus = (m.createFieldFocus + delta + total) % total
}

func (m Model) focusedCreateDynamicField() (jira.CreateField, bool) {
	index := m.createFieldFocus - m.createDynamicFieldStartIndex()
	fields := supportedCreateFields(m.createFields)
	if index < 0 || index >= len(fields) {
		return jira.CreateField{}, false
	}
	return fields[index], true
}

func (m *Model) moveCreateDynamicSelection(field jira.CreateField, delta int) {
	if len(field.AllowedValues) == 0 {
		return
	}
	key := createFieldValueKey(field)
	matches := filteredCreateFieldOptionIndexes(field.AllowedValues, m.createDynamicFilters[key])
	if len(matches) == 0 {
		return
	}
	position := createOptionMatchPosition(matches, m.createDynamicSelections[key])
	if position < 0 {
		position = 0
	} else {
		position = clamp(position+delta, 0, len(matches)-1)
	}
	m.createDynamicSelections[key] = matches[position]
}

func newCreateFilterInput(value string) textinput.Model {
	editor := textinput.New()
	editor.SetValue(value)
	editor.CursorEnd()
	return editor
}

func (m *Model) createDynamicFilterEditor(field jira.CreateField) textinput.Model {
	key := createFieldValueKey(field)
	if m.createDynamicFilters == nil {
		m.createDynamicFilters = map[string]string{}
	}
	if m.createDynamicFilterEditors == nil {
		m.createDynamicFilterEditors = map[string]textinput.Model{}
	}
	editor, ok := m.createDynamicFilterEditors[key]
	if !ok {
		editor = newCreateFilterInput(m.createDynamicFilters[key])
	}
	editor.Focus()
	m.createDynamicFilterEditors[key] = editor
	return editor
}

func (m *Model) setCreateDynamicFilterEditor(field jira.CreateField, editor textinput.Model) {
	key := createFieldValueKey(field)
	if m.createDynamicFilters == nil {
		m.createDynamicFilters = map[string]string{}
	}
	if m.createDynamicFilterEditors == nil {
		m.createDynamicFilterEditors = map[string]textinput.Model{}
	}
	m.createDynamicFilterEditors[key] = editor
	m.createDynamicFilters[key] = editor.Value()
	m.selectFirstFilteredCreateOption(field)
}

func (m *Model) updateCreateDynamicFilter(field jira.CreateField, msg tea.KeyMsg) tea.Cmd {
	editor := m.createDynamicFilterEditor(field)
	updated, cmd := editor.Update(msg)
	m.setCreateDynamicFilterEditor(field, updated)
	return cmd
}

func (m *Model) ensureCreateFieldOptionsState() {
	if m.createFieldOptionsLoading == nil {
		m.createFieldOptionsLoading = map[string]bool{}
	}
	if m.createFieldOptionsErr == nil {
		m.createFieldOptionsErr = map[string]error{}
	}
	if m.createFieldOptionsQuery == nil {
		m.createFieldOptionsQuery = map[string]string{}
	}
}

func (m *Model) requestFocusedCreateFieldOptions() tea.Cmd {
	field, ok := m.focusedCreateDynamicField()
	if !ok {
		return nil
	}
	return m.requestCreateFieldOptions(field)
}

func (m *Model) requestCreateFieldOptions(field jira.CreateField) tea.Cmd {
	if strings.TrimSpace(field.AutoCompleteURL) == "" || !createFieldUsesPicker(field) {
		return nil
	}
	key := createFieldValueKey(field)
	query := strings.TrimSpace(m.createDynamicFilters[key])
	if len(field.AllowedValues) > 0 && query == "" {
		return nil
	}
	m.ensureCreateFieldOptionsState()
	if m.createFieldOptionsLoading[key] && m.createFieldOptionsQuery[key] == query {
		return nil
	}
	m.nextRequestID++
	m.activeCreateFieldOptionsReqID = m.nextRequestID
	m.createFieldOptionsLoading[key] = true
	m.createFieldOptionsErr[key] = nil
	m.createFieldOptionsQuery[key] = query
	return m.submitCreateFieldOptions(m.activeCreateFieldOptionsReqID, field, query)
}

func (m *Model) clearCreateDynamicFilter(field jira.CreateField) {
	key := createFieldValueKey(field)
	if m.createDynamicFilters == nil {
		m.createDynamicFilters = map[string]string{}
	}
	if m.createDynamicFilterEditors == nil {
		m.createDynamicFilterEditors = map[string]textinput.Model{}
	}
	editor := newCreateFilterInput("")
	editor.Focus()
	m.createDynamicFilterEditors[key] = editor
	m.createDynamicFilters[key] = ""
}

func (m *Model) selectFirstFilteredCreateOption(field jira.CreateField) {
	key := createFieldValueKey(field)
	matches := filteredCreateFieldOptionIndexes(field.AllowedValues, m.createDynamicFilters[key])
	if len(matches) == 0 {
		m.createDynamicSelections[key] = -1
		return
	}
	m.createDynamicSelections[key] = matches[0]
}

func (m *Model) setCreateDynamicValue(field jira.CreateField, value string) {
	if m.createDynamicValues == nil {
		m.createDynamicValues = map[string]string{}
	}
	m.createDynamicValues[createFieldValueKey(field)] = value
}

func (m Model) createAIPromptFieldIndex() int {
	if !m.claudeCreateTicketDraftEnabled() {
		return -1
	}
	return m.createDynamicFieldStartIndex() - 1
}

func (m Model) createAIPromptFieldFocused() bool {
	aiIndex := m.createAIPromptFieldIndex()
	return aiIndex >= 0 && m.createFieldFocus == aiIndex
}

func (m Model) createFormReady() bool {
	return m.createIssueType.ID != "" && !m.createFieldsLoading && m.createFieldsErr == nil
}

func (m Model) createDynamicFieldFocusIndex(index int) int {
	return m.createDynamicFieldStartIndex() + index
}

func (m Model) createDynamicFieldStartIndex() int {
	start := 3
	if len(m.createAIQuestions) > 0 {
		start++
	}
	if m.claudeCreateTicketDraftEnabled() {
		start++
	}
	return start
}

func (m Model) createQuestionsFieldIndex() int {
	if len(m.createAIQuestions) == 0 {
		return -1
	}
	return 3
}

func (m Model) createQuestionsFieldFocused() bool {
	index := m.createQuestionsFieldIndex()
	return index >= 0 && m.createFieldFocus == index
}

func (m Model) createIssueFieldValues() ([]jira.CreateIssueFieldValue, error) {
	if unsupported := unsupportedRequiredCreateFields(m.createFields); len(unsupported) > 0 {
		return nil, fmt.Errorf("Jira requires unsupported fields: %s", strings.Join(unsupported, ", "))
	}
	var values []jira.CreateIssueFieldValue
	for _, field := range supportedCreateFields(m.createFields) {
		key := createFieldValueKey(field)
		value := jira.CreateIssueFieldValue{
			FieldID:      displayValue(field.ID, field.Key),
			SchemaType:   field.SchemaType,
			SchemaSystem: field.SchemaSystem,
		}
		if createFieldUsesPicker(field) {
			if len(field.AllowedValues) == 0 {
				if field.Required {
					return nil, fmt.Errorf("%s has no Jira options.", displayValue(field.Name, value.FieldID))
				}
				continue
			}
			selected := m.createDynamicSelections[key]
			if selected < 0 || selected >= len(field.AllowedValues) {
				if field.Required {
					return nil, fmt.Errorf("%s cannot be empty.", displayValue(field.Name, value.FieldID))
				}
				continue
			}
			value.Option = field.AllowedValues[selected]
		} else {
			value.Text = strings.TrimSpace(m.createDynamicValues[key])
			if field.Required && value.Text == "" {
				return nil, fmt.Errorf("%s cannot be empty.", displayValue(field.Name, value.FieldID))
			}
			if value.Text == "" {
				continue
			}
		}
		values = append(values, value)
	}
	return values, nil
}

func (m Model) submitCreateIssueDraft() (Model, tea.Cmd) {
	summary := strings.TrimSpace(m.createSummaryDraft)
	if summary == "" {
		m.detailNotice = "Summary cannot be empty."
		return m, nil
	}
	if strings.TrimSpace(m.createProjectKey) == "" || strings.TrimSpace(m.createIssueType.ID) == "" {
		m.detailNotice = "Create ticket failed: missing project or issue type."
		return m, nil
	}
	m.nextRequestID++
	m.activeCreateIssueReqID = m.nextRequestID
	m.createSubmitting = true
	m.createSubmitParentKey = strings.TrimSpace(m.createParentKey)
	m.createSubmitSummary = summary
	m.createSubmitDescription = strings.TrimSpace(m.createDescriptionDraft)
	fields, err := m.createIssueFieldValues()
	if err != nil {
		m.createSubmitting = false
		m.detailNotice = err.Error()
		return m, nil
	}
	m.createSubmitFields = fields
	m.detailNotice = ""
	return m, m.submitCreateIssue(m.activeCreateIssueReqID, worker.CreateIssueRequest{
		ProjectKey:  m.createProjectKey,
		IssueTypeID: m.createIssueType.ID,
		ParentKey:   m.createSubmitParentKey,
		Summary:     summary,
		Description: m.createSubmitDescription,
		Fields:      fields,
	})
}

func (m Model) startCreateAIPrompt() (Model, tea.Cmd) {
	if !m.claudeCreateTicketDraftEnabled() {
		m.detailNotice = "Claude ticket draft generation is not enabled."
		return m, nil
	}
	if !m.claudeCreateTicketDraftAvailable() {
		m.detailNotice = "Claude ticket draft generation is currently unavailable."
		return m, nil
	}
	if !m.createFormReady() {
		m.detailNotice = "Select an issue type before generating a ticket draft."
		return m, nil
	}
	m.createAIPromptOpen = true
	m.createAIPromptLoading = false
	m.createAIPromptErr = nil
	m.createAIPromptProgress = nil
	m.createAIPrompt = strings.TrimSpace(m.createAIPrompt)
	m.createAIPromptEditor = newCreateAIPromptEditor(m.createAIPrompt)
	m.createAIPromptEditorReady = true
	m.detailNotice = ""
	return m, nil
}

func (m Model) updateCreateAIPrompt(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.createAIPromptLoading {
		if msg.String() == "esc" {
			return m.cancelCreateAIPrompt(), nil
		}
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.createAIPromptOpen = false
		m.createAIPrompt = strings.TrimSpace(m.createAIPromptEditorValue())
		return m, nil
	case "ctrl+s":
		return m.submitCreateAIPrompt()
	}
	m.configureCreateAIPromptEditor(max(32, m.browserLayout(m.width).contentWidth-16), 8)
	editor, cmd := m.createAIPromptEditor.Update(msg)
	m.createAIPromptEditor = editor
	m.createAIPrompt = m.createAIPromptEditor.Value()
	return m, cmd
}

func (m Model) submitCreateAIPrompt() (Model, tea.Cmd) {
	if !m.claudeCreateTicketDraftEnabled() {
		m.detailNotice = "Claude ticket draft generation is not enabled."
		return m, nil
	}
	if !m.claudeCreateTicketDraftAvailable() {
		m.detailNotice = "Claude ticket draft generation is currently unavailable."
		return m, nil
	}
	request := strings.TrimSpace(m.createAIPrompt)
	if request == "" {
		request = "Draft a ticket from the selected project."
		if strings.TrimSpace(m.createIssueType.ID) != "" {
			request = "Draft a ticket from the selected project and issue type."
		}
		m.createAIPrompt = request
	}
	m.nextRequestID++
	m.activeCreateAIPromptReqID = m.nextRequestID
	m.createAIPromptLoading = true
	m.createAIPromptStartedAt = m.claudeNow()
	m.createAIPromptErr = nil
	m.createAIPromptProgress = nil
	m.createAIPromptEvents = make(chan claude.Event, 16)
	m.createAIPromptEditor = newCreateAIPromptEditor(request)
	m.createAIPromptEditorReady = true
	runCtx, cancel := context.WithCancel(context.Background())
	m.createAIPromptCancel = cancel
	m.recordDiagnosticEvent(diagnosticKindClaude, "create_ticket_draft", "submit", workerDiagnosticDetail(m.activeCreateAIPromptReqID, m.createProjectKey, nil))
	return m, tea.Batch(
		m.submitCreatePrompt(
			runCtx,
			m.activeCreateAIPromptReqID,
			m.buildCreateIssueDraftPrompt(request),
			m.createAIPromptEvents,
		),
		m.waitForCreateAIPromptProgress(m.activeCreateAIPromptReqID),
		m.scheduleCreateAIPromptTick(m.activeCreateAIPromptReqID),
	)
}

func (m Model) submitCreatePrompt(ctx context.Context, reqID int, prompt string, progress chan<- claude.Event) tea.Cmd {
	return m.submitAIRequest(ctx, aiTaskRequest{
		RequestID:         reqID,
		Operation:         events.AIOperationCreateDraft,
		PreferredProvider: events.AIProviderAuto,
		ProjectKey:        m.createProjectKey,
		Prompt:            prompt,
		Progress:          progress,
		ResultMsg: func(id int, _ string, text string, err error) tea.Msg {
			return createAIPromptResultMsg{id: id, text: text, err: err}
		},
	})
}

func (m Model) buildCreateIssueDraftPrompt(request string) string {
	var b strings.Builder
	if strings.TrimSpace(m.createIssueType.ID) == "" {
		b.WriteString("Draft a new Jira ticket for this project. The user has not selected the Jira issue type yet.\n")
	} else {
		b.WriteString("Draft a new Jira ticket for this project and issue type.\n")
	}
	b.WriteString("Return plain text in this exact format so it can be parsed:\n")
	b.WriteString("Issue Type: <one of the Available Jira Issue Types, or Unknown if not enough context>\n")
	b.WriteString("Summary: <one concise summary>\n")
	b.WriteString("Description: <full ticket description text>\n")
	if components := createComponentsField(m.createFields); len(components.AllowedValues) > 0 {
		b.WriteString("Components: <one of the Available Components, or Unknown if not enough context>\n")
	}
	b.WriteString("Do not edit files, create branches, run git commands, call Jira, or make external changes.\n")
	b.WriteString("Do not mention assumptions without flagging them as Open Questions.\n")
	b.WriteString("Focus on creating a clear, ready-to-use ticket. If scope is unclear, include questions in the Description under Open Questions.\n\n")
	b.WriteString("Project: ")
	b.WriteString(displayValue(m.createProjectKey, "Unknown"))
	b.WriteString("\nIssue Type: ")
	if strings.TrimSpace(m.createIssueType.ID) == "" {
		b.WriteString("Not selected yet")
	} else {
		b.WriteString(displayValue(m.createIssueType.Name, m.createIssueType.ID))
	}
	if len(m.createIssueTypes) > 0 {
		b.WriteString("\n\nAvailable Jira Issue Types:\n")
		for _, issueType := range m.createIssueTypes {
			name := strings.TrimSpace(displayValue(issueType.Name, issueType.ID))
			if name == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(name)
			b.WriteString("\n")
		}
	}
	if components := createComponentsField(m.createFields); len(components.AllowedValues) > 0 {
		b.WriteString("\n\nAvailable Components:\n")
		for _, option := range components.AllowedValues {
			name := strings.TrimSpace(displayValue(option.Name, option.ID))
			if name == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(name)
			b.WriteString("\n")
		}
	}
	if current := strings.TrimSpace(m.createIssueAICurrentDraft()); current != "" {
		b.WriteString("\n\nCurrent draft:\n")
		b.WriteString(current)
	}
	if feedback := strings.TrimSpace(m.createAIQuestionFeedback()); feedback != "" {
		b.WriteString("\n\n")
		b.WriteString(feedback)
	}
	b.WriteString("\n\nUser request:\n")
	b.WriteString(strings.TrimSpace(request))
	return strings.TrimSpace(b.String())
}

func (m Model) createIssueAICurrentDraft() string {
	var parts []string
	if summary := strings.TrimSpace(m.createSummaryDraft); summary != "" {
		parts = append(parts, "Summary: "+summary)
	}
	if description := strings.TrimSpace(m.createDescriptionDraft); description != "" {
		parts = append(parts, "Description:\n"+description)
	}
	return strings.Join(parts, "\n\n")
}

func (m Model) createAIQuestionFeedback() string {
	if len(m.createAIQuestions) == 0 {
		return ""
	}
	var answered []string
	var unanswered []string
	for _, question := range m.createAIQuestions {
		q := strings.TrimSpace(question.Question)
		if q == "" {
			continue
		}
		answer := strings.TrimSpace(question.Answer)
		if answer == "" {
			unanswered = append(unanswered, "- "+q)
			continue
		}
		answered = append(answered, "Q: "+q+"\nA: "+answer)
	}
	var parts []string
	if len(answered) > 0 {
		parts = append(parts, "User answers to Open Questions:\n"+strings.Join(answered, "\n\n"))
	}
	if len(unanswered) > 0 {
		parts = append(parts, "Still unanswered Open Questions:\n"+strings.Join(unanswered, "\n"))
	}
	return strings.Join(parts, "\n\n")
}

func (m Model) waitForCreateAIPromptProgress(reqID int) tea.Cmd {
	events := m.createAIPromptEvents
	return waitForClaudeProgress(events, reqID, "", func(id int, _ string, event claude.Event) tea.Msg {
		return createAIPromptProgressMsg{id: id, event: event}
	})
}

func (m Model) scheduleCreateAIPromptTick(reqID int) tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return createAIPromptTickMsg{id: reqID}
	})
}

func (m Model) cancelCreateAIPrompt() Model {
	if m.createAIPromptCancel != nil {
		m.createAIPromptCancel()
	}
	reqID := m.activeCreateAIPromptReqID
	m.createAIPromptCancel = nil
	m.createAIPromptEvents = nil
	m.createAIPromptLoading = false
	m.createAIPromptErr = errors.New("Claude ticket draft generation cancelled")
	m.createAIPromptOpen = false
	m.recordDiagnosticEvent(diagnosticKindClaude, "create_ticket_draft", "cancel", workerDiagnosticDetail(reqID, m.createProjectKey, m.createAIPromptErr))
	return m
}

func (m *Model) createAIPromptEditorValue() string {
	if m.createAIPromptEditorReady {
		return m.createAIPromptEditor.Value()
	}
	return m.createAIPrompt
}

func newCreateAIPromptEditor(value string) textarea.Model {
	editor := textarea.New()
	editor.Prompt = ""
	editor.Placeholder = "Describe the ticket you want Claude to draft."
	editor.ShowLineNumbers = false
	editor.EndOfBufferCharacter = ' '
	editor.SetVirtualCursor(true)
	editor.SetValue(value)
	editor.Focus()
	return editor
}

func newCreateQuestionAnswerEditor(value string) textarea.Model {
	editor := textarea.New()
	editor.Prompt = ""
	editor.Placeholder = "Answer this question for Claude."
	editor.ShowLineNumbers = false
	editor.EndOfBufferCharacter = ' '
	editor.SetVirtualCursor(true)
	editor.SetValue(value)
	editor.Focus()
	return editor
}

func (m *Model) ensureCreateQuestionAnswerEditor() {
	if m.createAIQuestionEditorReady {
		return
	}
	value := ""
	if len(m.createAIQuestions) > 0 {
		selected := clamp(m.selectedCreateAIQuestion, 0, len(m.createAIQuestions)-1)
		value = m.createAIQuestions[selected].Answer
	}
	m.createAIQuestionEditor = newCreateQuestionAnswerEditor(value)
	m.createAIQuestionEditorReady = true
}

func (m Model) configuredCreateQuestionAnswerEditor(width int, rows int) textarea.Model {
	editor := m.createAIQuestionEditor
	if !m.createAIQuestionEditorReady {
		value := ""
		if len(m.createAIQuestions) > 0 {
			selected := clamp(m.selectedCreateAIQuestion, 0, len(m.createAIQuestions)-1)
			value = m.createAIQuestions[selected].Answer
		}
		editor = newCreateQuestionAnswerEditor(value)
	}
	editor.MaxHeight = max(rows, 1)
	editor.MaxWidth = width
	editor.SetWidth(width)
	editor.SetHeight(rows)
	editor.Focus()
	return editor
}

func (m *Model) ensureCreateAIPromptEditor() {
	if m.createAIPromptEditorReady {
		return
	}
	m.createAIPromptEditor = newCreateAIPromptEditor(m.createAIPrompt)
	m.createAIPromptEditorReady = true
}

func (m *Model) configureCreateAIPromptEditor(width int, rows int) {
	m.ensureCreateAIPromptEditor()
	m.createAIPromptEditor.MaxHeight = max(rows, 1)
	m.createAIPromptEditor.MaxWidth = width
	m.createAIPromptEditor.SetWidth(width)
	m.createAIPromptEditor.SetHeight(rows)
	m.createAIPromptEditor.Focus()
}

func (m Model) configuredCreateAIPromptEditor(width int, rows int) textarea.Model {
	editor := m.createAIPromptEditor
	if !m.createAIPromptEditorReady {
		editor = newCreateAIPromptEditor(m.createAIPrompt)
	}
	editor.MaxHeight = max(rows, 1)
	editor.MaxWidth = width
	editor.SetWidth(width)
	editor.SetHeight(rows)
	editor.Focus()
	return editor
}

func (m Model) handleCreateAIPromptProgress(msg createAIPromptProgressMsg) Model {
	if msg.id != m.activeCreateAIPromptReqID {
		return m
	}
	if strings.TrimSpace(msg.event.Text) == "" {
		return m
	}
	m.createAIPromptProgress = append(m.createAIPromptProgress, msg.event)
	if len(m.createAIPromptProgress) > 6 {
		m.createAIPromptProgress = append([]claude.Event(nil), m.createAIPromptProgress[len(m.createAIPromptProgress)-6:]...)
	}
	m.recordDiagnosticEvent(diagnosticKindClaude, "create_ticket_draft", "progress", truncate(msg.event.Kind+" "+msg.event.Text, 100))
	return m
}

func (m Model) handleCreateAIPromptResult(msg createAIPromptResultMsg) (Model, tea.Cmd) {
	status := "ok"
	if msg.err != nil {
		status = "error"
		if errors.Is(msg.err, context.Canceled) {
			status = "cancel"
		} else if errors.Is(msg.err, context.DeadlineExceeded) {
			status = "timeout"
		}
	}
	m.recordDiagnosticEvent(diagnosticKindClaude, "create_ticket_draft", status, workerDiagnosticDetail(msg.id, m.createProjectKey, msg.err))
	if msg.id != m.activeCreateAIPromptReqID {
		return m, nil
	}
	m.createAIPromptLoading = false
	m.createAIPromptCancel = nil
	m.createAIPromptEvents = nil
	m.createAIPromptErr = msg.err
	m.createAIPromptOpen = true
	if msg.err != nil {
		return m, nil
	}
	summary, description := parseCreateIssueDraft(msg.text)
	if strings.TrimSpace(summary) == "" {
		m.createAIPromptErr = errors.New("Claude draft is missing a summary")
		m.createAIPromptOpen = true
		return m, nil
	}
	m.createAIFieldDrafts = parseCreateIssueDraftFields(msg.text)
	m.createAIQuestions = mergeCreateAIQuestionAnswers(parseCreateIssueOpenQuestions(msg.text), m.createAIQuestions)
	m.selectedCreateAIQuestion = clamp(m.selectedCreateAIQuestion, 0, max(0, len(m.createAIQuestions)-1))
	m.createAIQuestionAnswering = false
	m.createAIQuestionEditorReady = false
	m.createSummaryDraft = summary
	m.createDescriptionDraft = description
	m.createSummaryEditor = newSummaryEditor(summary)
	m.createSummaryEditorReady = true
	m.createDescriptionEditor = newCommentEditor(description)
	m.createDescriptionEditorReady = true
	m.applyCreateAIFieldDrafts()
	m.createFieldFocus = createSummaryFieldIndex
	m.createAIGeneratedMode = false
	m.createAIPromptOpen = false
	m.createAIPrompt = ""
	m.createAIPromptErr = nil
	m.createAIPromptProgress = nil
	if strings.TrimSpace(m.createIssueType.ID) == "" {
		if issueType, ok := m.createAIRecommendedIssueType(); ok {
			m.selectedCreateIssueType = issueType
			selectedName := displayValue(m.createIssueTypes[issueType].Name, m.createIssueTypes[issueType].ID)
			var cmd tea.Cmd
			m, cmd = m.selectCreateIssueType()
			m.detailNotice = "Applied Claude ticket draft and selected " + selectedName + "."
			return m, cmd
		}
		m.detailNotice = "Applied Claude ticket draft. Select an issue type to continue."
	} else {
		m.detailNotice = "Applied Claude ticket draft."
	}
	return m, nil
}

func (m Model) createAIRecommendedIssueType() (int, bool) {
	value := strings.TrimSpace(m.createAIFieldDrafts["issuetype"])
	if value == "" || strings.EqualFold(value, "unknown") || strings.EqualFold(value, "not selected yet") {
		return 0, false
	}
	for index, issueType := range m.createIssueTypes {
		if createIssueTypeMatches(issueType, value) {
			return index, true
		}
	}
	return 0, false
}

func mergeCreateAIQuestionAnswers(next []createAIQuestion, existing []createAIQuestion) []createAIQuestion {
	if len(next) == 0 {
		return nil
	}
	answers := map[string]string{}
	for _, question := range existing {
		key := normalizeCreateDraftFieldName(question.Question)
		if key != "" && strings.TrimSpace(question.Answer) != "" {
			answers[key] = question.Answer
		}
	}
	for index := range next {
		if answer := strings.TrimSpace(answers[normalizeCreateDraftFieldName(next[index].Question)]); answer != "" {
			next[index].Answer = answer
		}
	}
	return next
}

func (m *Model) resetCreateIssueState() {
	m.createOpen = false
	m.createProjectKey = ""
	m.createParentKey = ""
	m.createParentSummary = ""
	m.createIssueTypes = nil
	m.selectedCreateIssueType = 0
	m.createIssueTypesLoading = false
	m.createIssueTypesErr = nil
	m.createFields = nil
	m.createFieldsLoading = false
	m.createFieldsErr = nil
	m.createIssueType = jira.CreateIssueType{}
	m.createChangingType = false
	m.createAIGeneratedMode = false
	m.createFieldFocus = createSummaryFieldIndex
	m.createSummaryDraft = ""
	m.createDescriptionDraft = ""
	m.createSummaryEditor = newSummaryEditor("")
	m.createSummaryEditorReady = true
	m.createDescriptionEditor = newCommentEditor("")
	m.createDescriptionEditorReady = true
	m.createSubmitting = false
	m.createSubmitParentKey = ""
	m.createSubmitSummary = ""
	m.createSubmitDescription = ""
	m.createDynamicValues = nil
	m.createDynamicSelections = nil
	m.createDynamicFilters = nil
	m.createDynamicFilterEditors = nil
	m.createFieldOptionsLoading = nil
	m.createFieldOptionsErr = nil
	m.createFieldOptionsQuery = nil
	m.createSubmitFields = nil
	m.createAIPromptOpen = false
	m.createAIPrompt = ""
	m.createAIPromptEditor = textarea.Model{}
	m.createAIPromptEditorReady = false
	m.createAIPromptErr = nil
	m.createAIPromptLoading = false
	m.createAIPromptProgress = nil
	m.createAIPromptStartedAt = time.Time{}
	m.createAIPromptCancel = nil
	m.createAIPromptEvents = nil
	m.createAIPrompt = ""
	m.createAIFieldDrafts = nil
	m.createAIQuestions = nil
	m.selectedCreateAIQuestion = 0
	m.createAIQuestionAnswering = false
	m.createAIQuestionEditor = textarea.Model{}
	m.createAIQuestionEditorReady = false
}

func (m *Model) ensureCreateSummaryEditor() {
	if m.createSummaryEditorReady {
		return
	}
	m.createSummaryEditor = newSummaryEditor(m.createSummaryDraft)
	m.createSummaryEditorReady = true
}

func (m *Model) configureCreateSummaryEditor() {
	m.ensureCreateSummaryEditor()
	width := max(32, m.browserLayout(m.width).contentWidth-16)
	m.createSummaryEditor.MaxHeight = 3
	m.createSummaryEditor.MaxWidth = width
	m.createSummaryEditor.SetWidth(width)
	m.createSummaryEditor.SetHeight(3)
}

func (m Model) configuredCreateSummaryEditor(width int, rows int) textarea.Model {
	editor := m.createSummaryEditor
	if !m.createSummaryEditorReady {
		editor = newSummaryEditor(m.createSummaryDraft)
	}
	editor.MaxHeight = max(rows, 1)
	editor.MaxWidth = width
	editor.SetWidth(width)
	editor.SetHeight(rows)
	if !m.createSubmitting && m.createFieldFocus == createSummaryFieldIndex {
		editor.Focus()
	} else {
		editor.Blur()
	}
	return editor
}

func (m *Model) ensureCreateDescriptionEditor() {
	if m.createDescriptionEditorReady {
		return
	}
	m.createDescriptionEditor = newCommentEditor(m.createDescriptionDraft)
	m.createDescriptionEditorReady = true
}

func (m *Model) configureCreateDescriptionEditor() {
	m.ensureCreateDescriptionEditor()
	width := max(32, m.browserLayout(m.width).contentWidth-16)
	m.createDescriptionEditor.MaxHeight = 6
	m.createDescriptionEditor.MaxWidth = width
	m.createDescriptionEditor.SetWidth(width)
	m.createDescriptionEditor.SetHeight(6)
}

func (m Model) configuredCreateDescriptionEditor(width int, rows int) textarea.Model {
	editor := m.createDescriptionEditor
	if !m.createDescriptionEditorReady {
		editor = newCommentEditor(m.createDescriptionDraft)
	}
	editor.MaxHeight = max(rows, 1)
	editor.MaxWidth = width
	editor.SetWidth(width)
	editor.SetHeight(rows)
	if !m.createSubmitting && m.createFieldFocus == createDescriptionFieldIndex {
		editor.Focus()
	} else {
		editor.Blur()
	}
	return editor
}

func unsupportedRequiredCreateFields(fields []jira.CreateField) []string {
	var names []string
	for _, field := range fields {
		if !field.Required {
			continue
		}
		if isBuiltInCreateTextField(field) || supportedCreateField(field) {
			continue
		}
		names = append(names, displayValue(field.Name, displayValue(field.Key, field.ID)))
	}
	return names
}

func supportedCreateFields(fields []jira.CreateField) []jira.CreateField {
	var supported []jira.CreateField
	for _, field := range fields {
		if isBuiltInCreateTextField(field) || !supportedCreateField(field) {
			continue
		}
		supported = append(supported, field)
	}
	return supported
}

func supportedCreateField(field jira.CreateField) bool {
	id := strings.ToLower(strings.TrimSpace(displayValue(field.ID, field.Key)))
	system := strings.ToLower(strings.TrimSpace(field.SchemaSystem))
	schemaType := strings.ToLower(strings.TrimSpace(field.SchemaType))
	if id == "priority" || system == "priority" || id == "labels" || system == "labels" || id == "components" || system == "components" {
		return true
	}
	if len(field.AllowedValues) > 0 && (schemaType == "option" || schemaType == "priority" || schemaType == "") {
		return true
	}
	if strings.TrimSpace(field.AutoCompleteURL) != "" && (schemaType == "option" || schemaType == "array" || schemaType == "") {
		return true
	}
	switch schemaType {
	case "string", "textarea", "text", "number":
		return true
	default:
		return false
	}
}

func isBuiltInCreateTextField(field jira.CreateField) bool {
	id := strings.ToLower(strings.TrimSpace(displayValue(field.ID, field.Key)))
	system := strings.ToLower(strings.TrimSpace(field.SchemaSystem))
	return id == "summary" || system == "summary" ||
		id == "description" || system == "description" ||
		id == "project" || system == "project" ||
		id == "issuetype" || system == "issuetype"
}

func createFieldUsesPicker(field jira.CreateField) bool {
	id := strings.ToLower(strings.TrimSpace(displayValue(field.ID, field.Key)))
	system := strings.ToLower(strings.TrimSpace(field.SchemaSystem))
	return len(field.AllowedValues) > 0 || strings.TrimSpace(field.AutoCompleteURL) != "" || id == "priority" || system == "priority"
}

func createFieldValueKey(field jira.CreateField) string {
	return displayValue(field.ID, displayValue(field.Key, field.Name))
}

func defaultCreateFieldSelection(field jira.CreateField) int {
	if !createFieldUsesPicker(field) || len(field.AllowedValues) == 0 {
		return -1
	}
	id := strings.ToLower(strings.TrimSpace(displayValue(field.ID, field.Key)))
	system := strings.ToLower(strings.TrimSpace(field.SchemaSystem))
	if field.Required || id == "priority" || system == "priority" {
		return 0
	}
	return -1
}

func filteredCreateFieldOptionIndexes(options []jira.FieldOption, filter string) []int {
	filter = normalizeCreateDraftFieldName(filter)
	indexes := make([]int, 0, len(options))
	for index, option := range options {
		if filter == "" {
			indexes = append(indexes, index)
			continue
		}
		name := normalizeCreateDraftFieldName(option.Name)
		id := normalizeCreateDraftFieldName(option.ID)
		if strings.Contains(name, filter) || strings.Contains(id, filter) {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

func createOptionMatchPosition(matches []int, selected int) int {
	for index, match := range matches {
		if match == selected {
			return index
		}
	}
	return -1
}

func createComponentsField(fields []jira.CreateField) jira.CreateField {
	for _, field := range supportedCreateFields(fields) {
		id := strings.ToLower(strings.TrimSpace(displayValue(field.ID, field.Key)))
		system := strings.ToLower(strings.TrimSpace(field.SchemaSystem))
		if id == "components" || system == "components" {
			return field
		}
	}
	return jira.CreateField{}
}
