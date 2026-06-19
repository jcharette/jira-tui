package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/jcharette/jira-tui/internal/jira"
)

func (m Model) renderWorklogs(issueKey string, width int) string {
	help := ""
	if m.worklogListFocus {
		help = "j/k select  e edit  d delete"
	}
	lines := []string{m.detailSectionHeader("worklog", "Worklog", help, width)}
	if m.worklogsLoading && m.worklogsRequestKey == issueKey {
		lines = append(lines, m.detailStatusBlock("Loading worklogs...", width, false))
	}
	if m.worklogsErr != nil && m.worklogsRequestKey == issueKey {
		lines = append(lines, m.detailStatusBlock("Worklogs failed: "+m.worklogsErr.Error(), width, true))
	}
	worklogs := m.worklogs[issueKey]
	if len(worklogs) == 0 {
		if !(m.worklogsLoading && m.worklogsRequestKey == issueKey) && !(m.worklogsErr != nil && m.worklogsRequestKey == issueKey) {
			lines = append(lines, m.detailEmptyState("No work logged.", width))
		}
		return strings.Join(lines, "\n")
	}
	rows := make([][]string, 0, len(worklogs))
	cursor := clamp(m.selectedWorklog, 0, len(worklogs)-1)
	for index, worklog := range worklogs {
		comment := strings.TrimSpace(worklog.Comment)
		if comment == "" {
			comment = "-"
		}
		marker := " "
		timeStyle := m.theme.Key
		textStyle := m.theme.Text
		if m.worklogListFocus && index == cursor {
			marker = ">"
			timeStyle = m.theme.Selected
			textStyle = m.theme.Selected
		}
		rows = append(rows, []string{
			m.theme.Muted.Render(marker),
			timeStyle.Render(displayValue(worklog.TimeSpent, fmt.Sprintf("%ds", worklog.TimeSpentSeconds))),
			textStyle.Render(truncate(displayValue(worklog.Author, "Unknown"), 18)),
			m.theme.Muted.Render(formatWorklogTime(worklog.Started)),
			textStyle.Render(truncate(firstLine(comment), max(12, width-48))),
		})
	}
	lines = append(lines, m.detailTable(0, []string{"", "TIME", "AUTHOR", "STARTED", "NOTE"}, rows, nil))
	return strings.Join(lines, "\n")
}

func (m *Model) focusWorklogs() {
	selected, ok := m.selectedIssue()
	if !ok {
		return
	}
	worklogs := m.worklogs[selected.Key]
	if len(worklogs) == 0 {
		m.worklogListFocus = false
		m.detailNotice = "No worklogs are loaded for this ticket."
		return
	}
	m.linkFocus = false
	m.hierarchyFocus = false
	m.actionFocus = false
	m.transitionFocus = false
	m.priorityFocus = false
	m.assigneeFocus = false
	m.summaryFocus = false
	m.commentFocus = false
	m.worklogListFocus = true
	m.selectedWorklog = clamp(m.selectedWorklog, 0, len(worklogs)-1)
	m.detailNotice = ""
	m.jumpDetailSection("Worklog")
}

func (m *Model) moveSelectedWorklog(delta int) {
	worklogs := m.currentIssueWorklogs()
	if len(worklogs) == 0 {
		m.selectedWorklog = 0
		m.worklogListFocus = false
		return
	}
	m.selectedWorklog = clamp(m.selectedWorklog+delta, 0, len(worklogs)-1)
}

func (m Model) selectedWorklogEntry() (jira.Worklog, bool) {
	worklogs := m.currentIssueWorklogs()
	if len(worklogs) == 0 {
		return jira.Worklog{}, false
	}
	return worklogs[clamp(m.selectedWorklog, 0, len(worklogs)-1)], true
}

func (m Model) currentIssueWorklogs() []jira.Worklog {
	selected, ok := m.selectedIssue()
	if !ok {
		return nil
	}
	return m.worklogs[selected.Key]
}

func (m Model) startWorklogEditor() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok || strings.TrimSpace(selected.Key) == "" {
		m.detailNotice = "Select an issue before logging work."
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
	m.issueLinkFocus = false
	m.worklogFocus = true
	m.worklogEditing = false
	m.worklogFieldFocus = 0
	m.worklogTimeDraft = ""
	m.worklogCommentDraft = ""
	m.worklogTimeEditor = newWorklogTimeInput("")
	m.worklogTimeEditorReady = true
	m.worklogCommentEditor = newWorklogCommentEditor("")
	m.worklogCommentEditorReady = true
	m.detailNotice = ""
	return m, nil
}

func (m Model) startSelectedWorklogEditor() (Model, tea.Cmd) {
	worklog, ok := m.selectedWorklogEntry()
	if !ok || strings.TrimSpace(worklog.ID) == "" {
		m.detailNotice = "Select a Jira worklog before editing."
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
	m.issueLinkFocus = false
	m.worklogFocus = true
	m.worklogEditing = true
	m.worklogFieldFocus = 0
	m.worklogTimeDraft = strings.TrimSpace(worklog.TimeSpent)
	m.worklogCommentDraft = strings.TrimSpace(worklog.Comment)
	m.worklogTimeEditor = newWorklogTimeInput(m.worklogTimeDraft)
	m.worklogTimeEditorReady = true
	m.worklogCommentEditor = newWorklogCommentEditor(m.worklogCommentDraft)
	m.worklogCommentEditorReady = true
	m.worklogUpdateRequest = jira.UpdateWorklogRequest{
		ID:        worklog.ID,
		TimeSpent: worklog.TimeSpent,
		Started:   worklog.Started,
		Comment:   worklog.Comment,
	}
	m.detailNotice = ""
	return m, nil
}

func (m *Model) closeWorklogEditor() {
	m.worklogFocus = false
	m.worklogEditing = false
	m.worklogFieldFocus = 0
	m.worklogTimeDraft = ""
	m.worklogCommentDraft = ""
	m.worklogTimeEditor = textinput.Model{}
	m.worklogTimeEditorReady = false
	m.worklogCommentEditor = textarea.Model{}
	m.worklogCommentEditorReady = false
	m.worklogSubmitting = false
	m.worklogSubmitKey = ""
	m.worklogSubmitRequest = jira.AddWorklogRequest{}
	m.worklogUpdateRequest = jira.UpdateWorklogRequest{}
}

func (m Model) renderWorklogDialog(width int) string {
	selected, _ := m.selectedIssue()
	bodyWidth := max(36, width-6)
	lines := []string{}
	if m.worklogSubmitting {
		action := "Logging work."
		if m.worklogEditing {
			action = "Updating worklog."
		}
		lines = append(lines, m.theme.Muted.Render(action))
	}
	timeLabel := m.theme.Muted.Render("Duration")
	noteLabel := m.theme.Muted.Render("Note")
	if m.worklogFieldFocus == 0 {
		timeLabel = m.theme.Selected.Render("> Duration")
	} else {
		timeLabel = m.theme.Muted.Render("  Duration")
	}
	if m.worklogFieldFocus == 1 {
		noteLabel = m.theme.Selected.Render("> Note")
	} else {
		noteLabel = m.theme.Muted.Render("  Note")
	}
	lines = append(lines,
		m.theme.Muted.Render("Issue")+" "+m.theme.Key.Render(displayValue(selected.Key, "selected")),
		timeLabel,
		m.configuredWorklogTimeInput(bodyWidth).View(),
		noteLabel,
		m.configuredWorklogCommentEditor(bodyWidth, 4).View(),
	)
	title := "Log Work"
	if m.worklogEditing {
		title = "Edit Worklog"
	}
	return m.renderDetailDialog(width, title, selected.Key, strings.Join(lines, "\n"), "ctrl+s save  tab field  esc cancel")
}

func (m Model) updateWorklogEditor(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.closeWorklogEditor()
		return m, nil
	case "tab", "shift+tab", "backtab":
		if m.worklogFieldFocus == 0 {
			m.worklogFieldFocus = 1
		} else {
			m.worklogFieldFocus = 0
		}
		return m, nil
	case "ctrl+s":
		return m.submitWorklog()
	}
	if m.worklogFieldFocus == 0 {
		editor := m.configuredWorklogTimeInput(max(20, m.browserLayout(m.width).contentWidth-12))
		updated, cmd := editor.Update(msg)
		m.worklogTimeEditor = updated
		m.worklogTimeDraft = strings.TrimSpace(updated.Value())
		return m, cmd
	}
	editor := m.configuredWorklogCommentEditor(max(24, m.browserLayout(m.width).contentWidth-12), 4)
	updated, cmd := editor.Update(msg)
	m.worklogCommentEditor = updated
	m.worklogCommentDraft = updated.Value()
	return m, cmd
}

func (m Model) submitWorklog() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok || strings.TrimSpace(selected.Key) == "" {
		return m, nil
	}
	if m.worklogSubmitting {
		return m, nil
	}
	timeSpent := strings.TrimSpace(m.worklogTimeDraft)
	if timeSpent == "" && m.worklogTimeEditorReady {
		timeSpent = strings.TrimSpace(m.worklogTimeEditor.Value())
	}
	if !validWorklogDuration(timeSpent) {
		m.detailNotice = "Enter a Jira duration like 30m, 1h, or 1h 30m."
		return m, nil
	}
	comment := strings.TrimSpace(m.worklogCommentDraft)
	if comment == "" && m.worklogCommentEditorReady {
		comment = strings.TrimSpace(m.worklogCommentEditor.Value())
	}
	request := jira.AddWorklogRequest{
		TimeSpent: timeSpent,
		Started:   m.currentTime(),
		Comment:   comment,
	}
	if m.worklogEditing {
		updateRequest := jira.UpdateWorklogRequest{
			ID:        strings.TrimSpace(m.worklogUpdateRequest.ID),
			TimeSpent: timeSpent,
			Started:   m.worklogUpdateRequest.Started,
			Comment:   comment,
		}
		if updateRequest.Started.IsZero() {
			updateRequest.Started = m.currentTime()
		}
		m.nextRequestID++
		m.activeUpdateWorklogReqID = m.nextRequestID
		m.worklogSubmitting = true
		m.worklogSubmitKey = selected.Key
		m.worklogUpdateRequest = updateRequest
		m.detailNotice = "Updating worklog."
		return m, m.submitUpdateWorklog(m.activeUpdateWorklogReqID, selected.Key, updateRequest)
	}
	m.nextRequestID++
	m.activeAddWorklogReqID = m.nextRequestID
	m.worklogSubmitting = true
	m.worklogSubmitKey = selected.Key
	m.worklogSubmitRequest = request
	m.detailNotice = "Logging work."
	return m, m.submitAddWorklog(m.activeAddWorklogReqID, selected.Key, request)
}

func (m Model) startSelectedWorklogDelete() (Model, tea.Cmd) {
	worklog, ok := m.selectedWorklogEntry()
	if !ok || strings.TrimSpace(worklog.ID) == "" {
		m.detailNotice = "Select a Jira worklog before deleting."
		return m, nil
	}
	m.worklogDeleteConfirm = true
	m.worklogDeleteSubmitting = false
	m.worklogDeleteID = strings.TrimSpace(worklog.ID)
	m.detailNotice = ""
	return m, nil
}

func (m Model) submitSelectedWorklogDelete() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok || strings.TrimSpace(selected.Key) == "" {
		return m, nil
	}
	if m.worklogDeleteSubmitting {
		return m, nil
	}
	worklogID := strings.TrimSpace(m.worklogDeleteID)
	if worklogID == "" {
		m.worklogDeleteConfirm = false
		m.detailNotice = "Worklog delete failed: missing worklog ID."
		return m, nil
	}
	m.nextRequestID++
	m.activeDeleteWorklogReqID = m.nextRequestID
	m.worklogDeleteSubmitting = true
	m.detailNotice = "Deleting worklog."
	return m, m.submitDeleteWorklog(m.activeDeleteWorklogReqID, selected.Key, worklogID)
}

func (m *Model) cancelWorklogDelete() {
	m.worklogDeleteConfirm = false
	m.worklogDeleteSubmitting = false
	m.worklogDeleteID = ""
	m.detailNotice = ""
}

func (m Model) renderWorklogDeleteDialog(width int) string {
	selected, _ := m.selectedIssue()
	bodyWidth := max(36, width-6)
	worklog, _ := m.selectedWorklogEntry()
	target := displayValue(worklog.TimeSpent, worklog.ID)
	lines := []string{
		m.theme.Muted.Render("Issue") + " " + m.theme.Key.Render(displayValue(selected.Key, "selected")),
		m.theme.Muted.Render("Worklog") + " " + m.theme.Text.Render(target),
		"",
		m.theme.Error.Render("Delete this Jira worklog?"),
	}
	if m.worklogDeleteSubmitting {
		lines = append(lines, "", m.theme.Muted.Render("Deleting worklog."))
	}
	if m.detailNotice != "" {
		lines = append(lines, "", m.renderDetailNotice(m.detailNotice, bodyWidth))
	}
	return m.renderDetailDialog(width, "Delete Worklog", selected.Key, strings.Join(lines, "\n"), "enter delete  esc cancel")
}

func (m Model) configuredWorklogTimeInput(width int) textinput.Model {
	editor := m.worklogTimeEditor
	if !m.worklogTimeEditorReady {
		editor = newWorklogTimeInput(m.worklogTimeDraft)
	}
	editor.SetWidth(max(10, min(24, width)))
	return editor
}

func newWorklogTimeInput(value string) textinput.Model {
	editor := textinput.New()
	editor.Placeholder = "1h 30m"
	editor.SetValue(value)
	editor.CharLimit = 32
	editor.SetWidth(18)
	editor.Prompt = ""
	editor.Focus()
	return editor
}

func (m Model) configuredWorklogCommentEditor(width int, height int) textarea.Model {
	editor := m.worklogCommentEditor
	if !m.worklogCommentEditorReady {
		editor = newWorklogCommentEditor(m.worklogCommentDraft)
	}
	editor.MaxWidth = width
	editor.SetWidth(width)
	editor.SetHeight(max(2, height))
	return editor
}

func newWorklogCommentEditor(value string) textarea.Model {
	editor := textarea.New()
	editor.Placeholder = "What did you work on?"
	editor.SetValue(value)
	editor.ShowLineNumbers = false
	editor.MaxHeight = 5
	editor.SetHeight(4)
	editor.Focus()
	return editor
}

func validWorklogDuration(value string) bool {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) == 0 {
		return false
	}
	for _, field := range fields {
		if len(field) < 2 {
			return false
		}
		unit := field[len(field)-1]
		if !strings.ContainsRune("wdhm", rune(unit)) {
			return false
		}
		number := field[:len(field)-1]
		if number == "" {
			return false
		}
		for _, char := range number {
			if (char < '0' || char > '9') && char != '.' {
				return false
			}
		}
	}
	return true
}

func formatWorklogTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Format("Jan 2 15:04")
}

func firstLine(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if index := strings.IndexByte(value, '\n'); index >= 0 {
		return strings.TrimSpace(value[:index])
	}
	return value
}

func worklogBindings() []keyBinding {
	return []keyBinding{
		{Keys: []string{"type"}, Label: "edit", Description: "Edit the active worklog field.", Group: "Worklog", Footer: true},
		{Keys: []string{"j", "k"}, FooterKey: "j/k", Label: "select", Description: "Select a worklog when the Worklog section is focused.", Group: "Worklog"},
		{Keys: []string{"e"}, Label: "edit-row", Description: "Edit the selected Jira worklog.", Group: "Worklog"},
		{Keys: []string{"d"}, Label: "delete-row", Description: "Delete the selected Jira worklog after confirmation.", Group: "Worklog"},
		{Keys: []string{"tab"}, Label: "field", Description: "Switch between duration and note.", Group: "Worklog", Footer: true},
		{Keys: []string{"ctrl+s"}, Label: "save", Description: "Log work through Jira.", Group: "Worklog", Footer: true},
		{Keys: []string{"esc"}, Label: "cancel", Description: "Cancel work logging.", Group: "Worklog", Footer: true},
	}
}
