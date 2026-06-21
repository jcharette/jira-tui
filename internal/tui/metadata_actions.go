package tui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/jcharette/jira-tui/internal/worker"
)

const (
	timeTrackingOriginalField = iota
	timeTrackingRemainingField
)

func (m Model) startParentEditor() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok || strings.TrimSpace(selected.Key) == "" {
		m.detailNotice = "Select an issue before changing parent."
		return m, nil
	}
	m.linkFocus = false
	m.hierarchyFocus = false
	m.actionFocus = false
	m.transitionFocus = false
	m.summaryFocus = false
	m.priorityFocus = false
	m.labelsFocus = false
	m.componentsFocus = false
	m.genericFieldFocus = false
	m.assigneeFocus = false
	m.parentFocus = true
	m.parentDraft = currentIssueParentKey(m, selected.Key)
	m.parentEditor = newParentInput(m.parentDraft)
	m.parentEditorReady = true
	m.detailNotice = ""
	return m, nil
}

func (m *Model) closeParentEditor() {
	m.parentFocus = false
	m.parentDraft = ""
	m.parentEditor = textinput.Model{}
	m.parentEditorReady = false
	m.parentSubmitting = false
	m.parentSubmitKey = ""
	m.parentSubmitRequest = jira.UpdateParentRequest{}
	m.detailNotice = ""
}

func (m Model) renderParentDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	bodyWidth := min(max(24, width-12), 64)
	lines := []string{
		m.theme.Muted.Render("Set parent issue key. Leave blank to clear parent."),
		"",
		m.theme.FieldLabel.Render("Parent") + " " + m.configuredParentEditor(bodyWidth).View(),
	}
	if m.parentSubmitting && m.parentSubmitKey == selected.Key {
		lines = append(lines, "", m.detailStatusBlock("Updating parent...", bodyWidth, false))
	}
	if m.detailNotice != "" {
		lines = append(lines, "", m.renderDetailNotice(m.detailNotice, bodyWidth))
	}
	return m.renderDetailDialog(width, "Set Parent", selected.Key, strings.Join(lines, "\n"), "enter save  esc cancel")
}

func (m Model) updateParentEditor(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.closeParentEditor()
		return m, nil
	case "enter":
		m.parentDraft = m.parentEditorValue()
		return m.submitParent()
	}
	m.ensureParentEditor()
	editor, cmd := m.parentEditor.Update(msg)
	m.parentEditor = editor
	m.parentDraft = editor.Value()
	m.detailNotice = ""
	return m, cmd
}

func (m Model) submitParent() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok {
		return m, nil
	}
	parentKey := strings.ToUpper(strings.TrimSpace(m.parentEditorValue()))
	current := strings.ToUpper(strings.TrimSpace(currentIssueParentKey(m, selected.Key)))
	if parentKey == current {
		m.detailNotice = "Parent unchanged."
		return m, nil
	}
	request := jira.UpdateParentRequest{ParentKey: parentKey, Clear: parentKey == ""}
	m.nextRequestID++
	m.activeParentReqID = m.nextRequestID
	m.parentSubmitting = true
	m.parentSubmitKey = selected.Key
	m.parentSubmitRequest = request
	if request.Clear {
		m.detailNotice = "Clearing parent."
	} else {
		m.detailNotice = "Updating parent to " + parentKey + "."
	}
	return m, m.submitUpdateParent(m.activeParentReqID, selected.Key, request)
}

func (m Model) handleUpdateParentResult(result worker.Result) Model {
	if result.ID != m.activeParentReqID {
		return m
	}
	m.parentSubmitting = false
	if result.Err != nil {
		m.detailNotice = "Parent update failed: " + result.Err.Error()
		return m
	}
	if result.UpdateParent == nil {
		m.detailNotice = "Parent update failed: " + worker.ErrInvalidRequest.Error()
		return m
	}
	if result.UpdateParent.Key != m.parentSubmitKey {
		return m
	}
	m.updateIssueParent(result.UpdateParent.Key, result.UpdateParent.Request)
	if result.UpdateParent.Request.Clear {
		m.closeParentEditor()
		m.detailNotice = "Parent cleared."
		return m
	}
	m.closeParentEditor()
	m.detailNotice = "Parent updated."
	return m
}

func (m Model) startTimeTrackingEditor() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok || strings.TrimSpace(selected.Key) == "" {
		m.detailNotice = "Select an issue before editing estimates."
		return m, nil
	}
	m.linkFocus = false
	m.hierarchyFocus = false
	m.actionFocus = false
	m.transitionFocus = false
	m.summaryFocus = false
	m.priorityFocus = false
	m.labelsFocus = false
	m.componentsFocus = false
	m.genericFieldFocus = false
	m.assigneeFocus = false
	m.timeTrackingFocus = true
	m.timeTrackingField = timeTrackingOriginalField
	m.timeTrackingOriginalDraft = ""
	m.timeTrackingRemainingDraft = ""
	m.timeTrackingOriginalEditor = newEstimateInput("")
	m.timeTrackingRemainingEditor = newEstimateInput("")
	m.timeTrackingEditorReady = true
	m.detailNotice = ""
	return m, nil
}

func (m *Model) closeTimeTrackingEditor() {
	m.timeTrackingFocus = false
	m.timeTrackingField = timeTrackingOriginalField
	m.timeTrackingOriginalDraft = ""
	m.timeTrackingRemainingDraft = ""
	m.timeTrackingOriginalEditor = textinput.Model{}
	m.timeTrackingRemainingEditor = textinput.Model{}
	m.timeTrackingEditorReady = false
	m.timeTrackingSubmitting = false
	m.timeTrackingSubmitKey = ""
	m.timeTrackingSubmitRequest = jira.UpdateTimeTrackingRequest{}
	m.detailNotice = ""
}

func (m Model) renderTimeTrackingDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	bodyWidth := min(max(24, width-12), 64)
	lines := []string{
		m.theme.Muted.Render("Set estimates such as 2d, 3h, or 30m. Leave a field blank to keep it unchanged."),
		"",
		m.timeTrackingInputLine("Original", m.configuredOriginalEstimateEditor(bodyWidth), m.timeTrackingField == timeTrackingOriginalField),
		m.timeTrackingInputLine("Remaining", m.configuredRemainingEstimateEditor(bodyWidth), m.timeTrackingField == timeTrackingRemainingField),
	}
	if m.timeTrackingSubmitting && m.timeTrackingSubmitKey == selected.Key {
		lines = append(lines, "", m.detailStatusBlock("Updating estimates...", bodyWidth, false))
	}
	if m.detailNotice != "" {
		lines = append(lines, "", m.renderDetailNotice(m.detailNotice, bodyWidth))
	}
	return m.renderDetailDialog(width, "Edit Estimates", selected.Key, strings.Join(lines, "\n"), "tab field  enter save  esc cancel")
}

func (m Model) timeTrackingInputLine(label string, editor textinput.Model, focused bool) string {
	marker := " "
	style := m.theme.Text
	if focused {
		marker = ">"
		style = m.theme.Selected
	}
	return style.Render(marker+" "+label) + " " + editor.View()
}

func (m Model) updateTimeTrackingEditor(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.closeTimeTrackingEditor()
		return m, nil
	case "enter":
		return m.submitTimeTracking()
	case "tab", "shift+tab", "backtab":
		if m.timeTrackingField == timeTrackingOriginalField {
			m.timeTrackingField = timeTrackingRemainingField
		} else {
			m.timeTrackingField = timeTrackingOriginalField
		}
		m.focusTimeTrackingEditor()
		return m, nil
	}
	m.ensureTimeTrackingEditors()
	var cmd tea.Cmd
	if m.timeTrackingField == timeTrackingOriginalField {
		editor, updateCmd := m.timeTrackingOriginalEditor.Update(msg)
		m.timeTrackingOriginalEditor = editor
		m.timeTrackingOriginalDraft = editor.Value()
		cmd = updateCmd
	} else {
		editor, updateCmd := m.timeTrackingRemainingEditor.Update(msg)
		m.timeTrackingRemainingEditor = editor
		m.timeTrackingRemainingDraft = editor.Value()
		cmd = updateCmd
	}
	m.detailNotice = ""
	return m, cmd
}

func (m Model) submitTimeTracking() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok {
		return m, nil
	}
	request := jira.UpdateTimeTrackingRequest{
		OriginalEstimate:  strings.TrimSpace(m.timeTrackingOriginalEditorValue()),
		RemainingEstimate: strings.TrimSpace(m.timeTrackingRemainingEditorValue()),
	}
	if request.OriginalEstimate == "" && request.RemainingEstimate == "" {
		m.detailNotice = "Enter original or remaining estimate before saving."
		return m, nil
	}
	m.nextRequestID++
	m.activeTimeTrackingReqID = m.nextRequestID
	m.timeTrackingSubmitting = true
	m.timeTrackingSubmitKey = selected.Key
	m.timeTrackingSubmitRequest = request
	m.detailNotice = "Updating estimates."
	return m, m.submitUpdateTimeTracking(m.activeTimeTrackingReqID, selected.Key, request)
}

func (m Model) handleUpdateTimeTrackingResult(result worker.Result) Model {
	if result.ID != m.activeTimeTrackingReqID {
		return m
	}
	m.timeTrackingSubmitting = false
	if result.Err != nil {
		m.detailNotice = "Estimates update failed: " + result.Err.Error()
		return m
	}
	if result.UpdateTimeTracking == nil {
		m.detailNotice = "Estimates update failed: " + worker.ErrInvalidRequest.Error()
		return m
	}
	if result.UpdateTimeTracking.Key != m.timeTrackingSubmitKey {
		return m
	}
	m.closeTimeTrackingEditor()
	m.detailNotice = "Estimates updated."
	return m
}

func currentIssueParentKey(m Model, key string) string {
	if detail, ok := m.details[key]; ok {
		return strings.TrimSpace(detail.ParentKey)
	}
	for _, issue := range m.issues {
		if issue.Key == key {
			return strings.TrimSpace(issue.ParentKey)
		}
	}
	return ""
}

func (m *Model) updateIssueParent(key string, request jira.UpdateParentRequest) {
	parent := strings.ToUpper(strings.TrimSpace(request.ParentKey))
	if request.Clear {
		parent = ""
	}
	for index := range m.issues {
		if m.issues[index].Key == key {
			m.issues[index].ParentKey = parent
		}
	}
	if detail, ok := m.details[key]; ok {
		detail.ParentKey = parent
		m.details[key] = detail
	}
}

func newParentInput(value string) textinput.Model {
	editor := textinput.New()
	editor.Prompt = ""
	editor.Placeholder = "ABC-100"
	editor.SetValue(value)
	editor.CharLimit = 64
	editor.SetWidth(24)
	editor.CursorEnd()
	editor.Focus()
	return editor
}

func newEstimateInput(value string) textinput.Model {
	editor := textinput.New()
	editor.Prompt = ""
	editor.Placeholder = "2d 4h"
	editor.SetValue(value)
	editor.CharLimit = 32
	editor.SetWidth(18)
	editor.CursorEnd()
	editor.Focus()
	return editor
}

func (m *Model) ensureParentEditor() {
	if m.parentEditorReady {
		return
	}
	m.parentEditor = newParentInput(m.parentDraft)
	m.parentEditorReady = true
}

func (m Model) configuredParentEditor(width int) textinput.Model {
	editor := m.parentEditor
	if !m.parentEditorReady {
		editor = newParentInput(m.parentDraft)
	}
	editor.SetWidth(max(18, min(width-12, 32)))
	editor.Focus()
	return editor
}

func (m Model) parentEditorValue() string {
	if m.parentEditorReady {
		return m.parentEditor.Value()
	}
	return m.parentDraft
}

func (m *Model) ensureTimeTrackingEditors() {
	if m.timeTrackingEditorReady {
		return
	}
	m.timeTrackingOriginalEditor = newEstimateInput(m.timeTrackingOriginalDraft)
	m.timeTrackingRemainingEditor = newEstimateInput(m.timeTrackingRemainingDraft)
	m.timeTrackingEditorReady = true
	m.focusTimeTrackingEditor()
}

func (m *Model) focusTimeTrackingEditor() {
	m.timeTrackingOriginalEditor.Blur()
	m.timeTrackingRemainingEditor.Blur()
	if m.timeTrackingField == timeTrackingOriginalField {
		m.timeTrackingOriginalEditor.Focus()
	} else {
		m.timeTrackingRemainingEditor.Focus()
	}
}

func (m Model) configuredOriginalEstimateEditor(width int) textinput.Model {
	editor := m.timeTrackingOriginalEditor
	if !m.timeTrackingEditorReady {
		editor = newEstimateInput(m.timeTrackingOriginalDraft)
	}
	editor.SetWidth(max(12, min(width-16, 24)))
	if m.timeTrackingField == timeTrackingOriginalField {
		editor.Focus()
	} else {
		editor.Blur()
	}
	return editor
}

func (m Model) configuredRemainingEstimateEditor(width int) textinput.Model {
	editor := m.timeTrackingRemainingEditor
	if !m.timeTrackingEditorReady {
		editor = newEstimateInput(m.timeTrackingRemainingDraft)
	}
	editor.SetWidth(max(12, min(width-16, 24)))
	if m.timeTrackingField == timeTrackingRemainingField {
		editor.Focus()
	} else {
		editor.Blur()
	}
	return editor
}

func (m Model) timeTrackingOriginalEditorValue() string {
	if m.timeTrackingEditorReady {
		return m.timeTrackingOriginalEditor.Value()
	}
	return m.timeTrackingOriginalDraft
}

func (m Model) timeTrackingRemainingEditorValue() string {
	if m.timeTrackingEditorReady {
		return m.timeTrackingRemainingEditor.Value()
	}
	return m.timeTrackingRemainingDraft
}
