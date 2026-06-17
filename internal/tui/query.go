package tui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/jcharette/jira-tui/internal/cache"
	"github.com/jcharette/jira-tui/internal/config"
	"github.com/jcharette/jira-tui/internal/events"
	"github.com/jcharette/jira-tui/internal/worker"
)

func (m *Model) startQueryModal() {
	m.queryOpen = true
	m.queryMode = queryModeJQL
	m.queryJQLDraft = strings.TrimSpace(m.jql)
	m.queryJQLEditor = newQueryTextArea(m.queryJQLDraft, "Enter JQL")
	m.queryJQLEditorReady = true
	m.queryHistory = m.loadQueryHistory()
	m.queryHistorySelected = min(m.queryHistorySelected, max(0, len(m.queryHistory)-1))
	m.querySaveViewOpen = false
	if strings.TrimSpace(m.queryAIPrompt) == "" {
		m.queryAIEditor = newQueryTextArea("", "Describe the issues you want to see")
		m.queryAIEditorReady = true
	}
	m.detailNotice = ""
}

func (m *Model) setQueryJQLDraft(value string) {
	m.queryJQLDraft = value
	m.queryJQLEditor = newQueryTextArea(value, "Enter JQL")
	m.queryJQLEditorReady = true
}

func (m *Model) setQueryAIPrompt(value string) {
	m.queryAIPrompt = value
	m.queryAIEditor = newQueryTextArea(value, "Describe the issues you want to see")
	m.queryAIEditorReady = true
}

func (m *Model) setQuerySaveViewName(value string) {
	m.querySaveViewName = value
	m.querySaveViewEditor = newQueryNameInput(value, "Saved view name")
	m.querySaveViewReady = true
}

func (m Model) updateQueryModal(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.queryAILoading {
		if msg.String() == "esc" {
			return m.cancelQueryAI(), nil
		}
		return m, nil
	}
	if m.querySaveViewOpen {
		switch msg.String() {
		case "esc":
			m.querySaveViewOpen = false
			m.detailNotice = ""
			return m, nil
		case "enter", "ctrl+s":
			return m.saveSelectedRecentQueryAsView(), nil
		}
		editor, cmd := m.configuredQuerySaveViewEditor(max(24, m.browserLayout(m.width).contentWidth-12)).Update(msg)
		m.querySaveViewEditor = editor
		m.querySaveViewName = m.querySaveViewEditor.Value()
		return m, cmd
	}
	switch msg.String() {
	case "esc":
		m.queryOpen = false
		m.detailNotice = ""
		return m, nil
	case "tab":
		m.toggleQueryMode()
		return m, nil
	case "enter":
		if m.queryMode == queryModeAI && strings.TrimSpace(m.queryGeneratedJQL) != "" {
			m.queryMode = queryModeJQL
			m.setQueryJQLDraft(m.queryGeneratedJQL)
			m.detailNotice = "Generated JQL copied for review."
		} else if m.queryMode == queryModeRecent {
			m.loadSelectedRecentQueryForReview()
		}
		return m, nil
	case "ctrl+s":
		if m.queryMode == queryModeAI {
			if strings.TrimSpace(m.queryGeneratedJQL) != "" && strings.TrimSpace(m.queryAIPromptValue()) == strings.TrimSpace(m.queryGeneratedPrompt) {
				return m.applyQueryJQLWithHistory(m.queryGeneratedJQL, cache.QueryHistorySourceAI, m.queryGeneratedPrompt)
			}
			return m.submitQueryAI()
		}
		if m.queryMode == queryModeRecent {
			if record, ok := m.selectedQueryHistoryRecord(); ok {
				return m.applyQueryJQLWithHistory(record.JQL, record.Source, record.Prompt)
			}
			m.detailNotice = "No recent queries yet."
			return m, nil
		}
		return m.applyQueryJQLWithHistory(m.queryJQLValue(), cache.QueryHistorySourceDirect, "")
	}
	if m.queryMode == queryModeRecent {
		switch msg.String() {
		case "s":
			m.openQuerySaveViewPrompt()
		case "j", "down":
			if len(m.queryHistory) > 0 {
				m.queryHistorySelected = min(len(m.queryHistory)-1, m.queryHistorySelected+1)
			}
		case "k", "up":
			if len(m.queryHistory) > 0 {
				m.queryHistorySelected = max(0, m.queryHistorySelected-1)
			}
		}
		return m, nil
	}
	if m.queryMode == queryModeAI {
		m.configureQueryAIEditor(max(32, m.browserLayout(m.width).contentWidth-12), 5)
		editor, cmd := m.queryAIEditor.Update(msg)
		m.queryAIEditor = editor
		m.queryAIPrompt = m.queryAIEditor.Value()
		return m, cmd
	}
	m.configureQueryJQLEditor(max(32, m.browserLayout(m.width).contentWidth-12), 4)
	editor, cmd := m.queryJQLEditor.Update(msg)
	m.queryJQLEditor = editor
	m.queryJQLDraft = m.queryJQLEditor.Value()
	return m, cmd
}

func (m *Model) toggleQueryMode() {
	if m.queryMode == queryModeJQL {
		m.queryMode = queryModeAI
		m.configureQueryAIEditor(max(32, m.browserLayout(m.width).contentWidth-12), 5)
		return
	}
	if m.queryMode == queryModeAI {
		m.queryMode = queryModeRecent
		return
	}
	m.queryMode = queryModeJQL
	m.configureQueryJQLEditor(max(32, m.browserLayout(m.width).contentWidth-12), 4)
}

func (m Model) applyQueryJQL(jql string) (Model, tea.Cmd) {
	return m.applyQueryJQLWithHistory(jql, cache.QueryHistorySourceDirect, "")
}

func (m Model) applyQueryJQLWithHistory(jql string, source cache.QueryHistorySource, prompt string) (Model, tea.Cmd) {
	jql = strings.TrimSpace(jql)
	if jql == "" {
		m.detailNotice = "JQL cannot be empty."
		return m, nil
	}
	m.persistQueryHistory(jql, source, prompt)
	m.jql = jql
	m.view = -1
	m.mode = modeTable
	m.queryOpen = false
	m.queryJQLDraft = jql
	m.statusFilter = issueStatusFilterAll
	m.collapsedIssueKeys = nil
	m.selected = 0
	m.offset = 0
	m.err = nil
	m.viewStale = false
	m.detailNotice = "Running JQL query."
	if record, ok := m.cachedActiveIssueView(m.jql); ok {
		fresh := m.activeIssueViewCacheFresh(record)
		m.applyActiveIssueView(record, !fresh)
		if fresh {
			m.detailNotice = "Loaded cached JQL query."
			return m, nil
		}
	}
	m.nextRequestID++
	m.activeRequestID = m.nextRequestID
	m.loading = true
	m.refreshing = false
	return m, m.submitIssueSearch(m.activeRequestID, worker.PriorityForeground)
}

func (m Model) queryJQLValue() string {
	if m.queryJQLEditorReady {
		return m.queryJQLEditor.Value()
	}
	return m.queryJQLDraft
}

func (m Model) queryAIPromptValue() string {
	if m.queryAIEditorReady {
		return m.queryAIEditor.Value()
	}
	return m.queryAIPrompt
}

func (m *Model) configureQueryJQLEditor(width int, rows int) {
	value := m.queryJQLDraft
	if m.queryJQLEditorReady {
		value = m.queryJQLEditor.Value()
	}
	m.queryJQLEditor = newQueryTextArea(value, "Enter JQL")
	m.queryJQLEditor.SetWidth(width)
	m.queryJQLEditor.SetHeight(rows)
	m.queryJQLEditorReady = true
}

func (m *Model) configureQueryAIEditor(width int, rows int) {
	value := m.queryAIPrompt
	if m.queryAIEditorReady {
		value = m.queryAIEditor.Value()
	}
	m.queryAIEditor = newQueryTextArea(value, "Describe the issues you want to see")
	m.queryAIEditor.SetWidth(width)
	m.queryAIEditor.SetHeight(rows)
	m.queryAIEditorReady = true
}

func newQueryTextArea(value string, placeholder string) textarea.Model {
	editor := textarea.New()
	editor.Placeholder = placeholder
	editor.SetValue(value)
	editor.Focus()
	editor.ShowLineNumbers = false
	editor.CharLimit = 2000
	return editor
}

func newQueryNameInput(value string, placeholder string) textinput.Model {
	input := textinput.New()
	input.Placeholder = placeholder
	input.SetValue(value)
	input.Focus()
	input.CharLimit = 80
	return input
}

func (m Model) renderQueryModal(layout browserLayout) string {
	width := max(40, layout.contentWidth-4)
	bodyWidth := max(32, width-4)
	var lines []string
	lines = append(lines, m.theme.PaneTitle.Render("Query"))
	lines = append(lines, m.theme.Muted.Render(m.queryModeLabel()))
	lines = append(lines, "")
	if m.queryMode == queryModeAI {
		if m.queryAILoading {
			lines = append(lines, m.theme.Muted.Render("Generating JQL..."))
		} else {
			lines = append(lines, m.configuredQueryAIEditor(bodyWidth, 5).View())
		}
		if strings.TrimSpace(m.queryGeneratedJQL) != "" {
			lines = append(lines, "", m.theme.Muted.Render("Preview"))
			lines = append(lines, m.theme.Text.Render(wrapText(m.queryGeneratedJQL, bodyWidth)))
		}
	} else if m.querySaveViewOpen {
		lines = append(lines, m.theme.Muted.Render("Save Selected Recent Query"))
		lines = append(lines, m.configuredQuerySaveViewEditor(bodyWidth).View())
	} else if m.queryMode == queryModeRecent {
		lines = append(lines, m.renderQueryHistory(bodyWidth)...)
	} else {
		lines = append(lines, m.configuredQueryJQLEditor(bodyWidth, 4).View())
	}
	if m.detailNotice != "" {
		lines = append(lines, "", m.renderDetailNotice(m.detailNotice, bodyWidth))
	}
	lines = append(lines, "", m.theme.Muted.Render(m.queryFooterText()))
	return m.theme.ActivePane.Width(width).Render(strings.Join(lines, "\n"))
}

func (m Model) configuredQueryJQLEditor(width int, rows int) textarea.Model {
	editor := m.queryJQLEditor
	if !m.queryJQLEditorReady {
		editor = newQueryTextArea(m.queryJQLDraft, "Enter JQL")
	}
	editor.SetWidth(width)
	editor.SetHeight(rows)
	return editor
}

func (m Model) configuredQueryAIEditor(width int, rows int) textarea.Model {
	editor := m.queryAIEditor
	if !m.queryAIEditorReady {
		editor = newQueryTextArea(m.queryAIPrompt, "Describe the issues you want to see")
	}
	editor.SetWidth(width)
	editor.SetHeight(rows)
	return editor
}

func (m Model) configuredQuerySaveViewEditor(width int) textinput.Model {
	editor := m.querySaveViewEditor
	if !m.querySaveViewReady {
		editor = newQueryNameInput(m.querySaveViewName, "Saved view name")
	}
	editor.SetWidth(width)
	return editor
}

func (m Model) queryModeLabel() string {
	switch m.queryMode {
	case queryModeAI:
		return "JQL  |  AI selected  |  Recent"
	case queryModeRecent:
		return "JQL  |  AI  |  Recent selected"
	default:
		return "JQL selected  |  AI  |  Recent"
	}
}

func (m Model) queryFooterText() string {
	if m.querySaveViewOpen {
		return "ctrl+s save  enter save  esc cancel"
	}
	if m.queryMode == queryModeAI {
		if strings.TrimSpace(m.queryGeneratedJQL) != "" {
			return "ctrl+s run preview  enter edit preview  tab recent  esc cancel"
		}
		return "ctrl+s generate  tab recent  esc cancel"
	}
	if m.queryMode == queryModeRecent {
		if len(m.queryHistory) == 0 {
			return "tab direct JQL  esc cancel"
		}
		return "j/k select  enter edit  ctrl+s run  s save view  tab direct JQL  esc cancel"
	}
	return "ctrl+s run  tab AI  esc cancel"
}

func (m Model) renderQueryHistory(width int) []string {
	if len(m.queryHistory) == 0 {
		return []string{m.theme.Muted.Render("No recent queries yet.")}
	}
	rows := make([]string, 0, min(len(m.queryHistory), 8))
	for i, record := range m.queryHistory {
		prefix := "  "
		style := m.theme.Text
		if i == m.queryHistorySelected {
			prefix = "> "
			style = m.theme.Selected
		}
		source := "JQL"
		if record.Source == cache.QueryHistorySourceAI {
			source = "AI"
		}
		label := record.JQL
		if strings.TrimSpace(record.Prompt) != "" {
			label = record.Prompt + " -> " + record.JQL
		}
		rows = append(rows, style.Render(truncate(fmt.Sprintf("%s[%s] %s", prefix, source, label), width)))
	}
	return rows
}

func (m Model) selectedQueryHistoryRecord() (cache.QueryHistoryRecord, bool) {
	if len(m.queryHistory) == 0 || m.queryHistorySelected < 0 || m.queryHistorySelected >= len(m.queryHistory) {
		return cache.QueryHistoryRecord{}, false
	}
	return m.queryHistory[m.queryHistorySelected], true
}

func (m *Model) loadSelectedRecentQueryForReview() {
	record, ok := m.selectedQueryHistoryRecord()
	if !ok {
		m.detailNotice = "No recent queries yet."
		return
	}
	m.queryMode = queryModeJQL
	m.setQueryJQLDraft(record.JQL)
	m.detailNotice = "Recent query loaded for review."
}

func (m *Model) openQuerySaveViewPrompt() {
	record, ok := m.selectedQueryHistoryRecord()
	if !ok {
		m.detailNotice = "No recent queries yet."
		return
	}
	name := strings.TrimSpace(record.Prompt)
	if name == "" {
		name = "Recent Query"
	}
	m.querySaveViewOpen = true
	m.setQuerySaveViewName(name)
	m.detailNotice = ""
}

func (m Model) querySaveViewNameValue() string {
	if m.querySaveViewReady {
		return m.querySaveViewEditor.Value()
	}
	return m.querySaveViewName
}

func (m Model) saveSelectedRecentQueryAsView() Model {
	record, ok := m.selectedQueryHistoryRecord()
	if !ok {
		m.detailNotice = "No recent queries yet."
		return m
	}
	view := config.IssueView{
		Name: m.querySaveViewNameValue(),
		JQL:  record.JQL,
	}
	cfg, err := config.AddSavedView(config.Config{Views: m.views}, view)
	if err != nil {
		m.detailNotice = err.Error()
		return m
	}
	view = cfg.Views[len(cfg.Views)-1]
	if m.savedViewWriter == nil {
		m.detailNotice = "Saved-view persistence is not available."
		return m
	}
	if err := m.savedViewWriter(view); err != nil {
		m.detailNotice = "Saved view failed: " + err.Error()
		return m
	}
	m.views = cfg.Views
	m.querySaveViewOpen = false
	m.detailNotice = "Saved view " + view.Name + "."
	return m
}

func (m Model) queryAIAvailable() bool {
	return m.claudeConfig.Enabled && m.claudeStatus.Enabled && m.claudeStatus.Available
}

func (m Model) submitQueryAI() (Model, tea.Cmd) {
	if !m.queryAIAvailable() {
		m.detailNotice = "AI-assisted JQL generation is not enabled or available."
		return m, nil
	}
	prompt := strings.TrimSpace(m.queryAIPromptValue())
	if prompt == "" {
		m.detailNotice = "Describe the Jira issues you want before generating JQL."
		return m, nil
	}
	m.queryAIPrompt = prompt
	m.queryAIEditor = newQueryTextArea(prompt, "Describe the issues you want to see")
	m.queryAIEditorReady = true
	m.nextRequestID++
	m.activeQueryAIReqID = m.nextRequestID
	m.queryAILoading = true
	m.queryAIErr = nil
	m.detailNotice = ""
	runCtx, cancel := context.WithCancel(context.Background())
	m.queryAICancel = cancel
	return m, m.submitQueryAIRequest(runCtx, m.activeQueryAIReqID, m.buildQueryAIPrompt(prompt))
}

func (m Model) submitQueryAIRequest(ctx context.Context, reqID int, prompt string) tea.Cmd {
	return m.submitAIRequest(ctx, aiTaskRequest{
		RequestID:         reqID,
		Operation:         events.AIOperationGenerateJQL,
		PreferredProvider: events.AIProviderAuto,
		ProjectKey:        projectKeyFromJQL(m.jql),
		Prompt:            prompt,
		ResultMsg: func(id int, _ string, text string, err error) tea.Msg {
			return queryAIResultMsg{id: id, text: text, err: err}
		},
	})
}

func (m Model) buildQueryAIPrompt(request string) string {
	var b strings.Builder
	b.WriteString("Generate one Jira Cloud JQL query for the user's requested issue view.\n")
	b.WriteString("Return only the JQL, or a line starting with JQL: followed by the query.\n")
	b.WriteString("Do not call Jira, edit files, or make external changes.\n")
	b.WriteString("Prefer explicit ORDER BY updated DESC unless the user asks for another order.\n\n")
	b.WriteString("Current JQL:\n")
	b.WriteString(strings.TrimSpace(m.jql))
	if len(m.views) > 0 {
		b.WriteString("\n\nSaved views:\n")
		for _, view := range m.views {
			b.WriteString("- ")
			b.WriteString(view.Name)
			b.WriteString(": ")
			b.WriteString(view.JQL)
			b.WriteString("\n")
		}
	}
	if current := strings.TrimSpace(m.queryGeneratedJQL); current != "" {
		b.WriteString("\n\nCurrent generated preview:\n")
		b.WriteString(current)
	}
	b.WriteString("\n\nUser request:\n")
	b.WriteString(strings.TrimSpace(request))
	return strings.TrimSpace(b.String())
}

func (m Model) handleQueryAIResult(msg queryAIResultMsg) Model {
	if msg.id != m.activeQueryAIReqID {
		return m
	}
	m.queryAILoading = false
	m.queryAICancel = nil
	if msg.err != nil {
		m.queryAIErr = msg.err
		m.detailNotice = "AI-assisted JQL generation failed: " + msg.err.Error()
		return m
	}
	jql := parseGeneratedJQL(msg.text)
	if jql == "" {
		m.detailNotice = "AI response did not include a JQL query."
		return m
	}
	m.queryGeneratedJQL = jql
	m.queryGeneratedPrompt = strings.TrimSpace(m.queryAIPromptValue())
	m.detailNotice = "Generated JQL preview ready."
	return m
}

func (m Model) cancelQueryAI() Model {
	if m.queryAICancel != nil {
		m.queryAICancel()
	}
	m.queryAICancel = nil
	m.queryAILoading = false
	m.detailNotice = "AI-assisted JQL generation cancelled."
	return m
}

func parseGeneratedJQL(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(trimmed), "jql:") {
			return strings.TrimSpace(trimmed[len("jql:"):])
		}
	}
	if fenced := firstFencedBlock(lines); fenced != "" {
		return fenced
	}
	return strings.Trim(strings.TrimSpace(text), "`")
}

func firstFencedBlock(lines []string) string {
	inFence := false
	var block []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inFence {
				return strings.TrimSpace(strings.Join(block, "\n"))
			}
			inFence = true
			continue
		}
		if inFence {
			block = append(block, line)
		}
	}
	return ""
}
