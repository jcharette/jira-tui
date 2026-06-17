package tui

import (
	"context"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/jon/jira-tui/internal/events"
	"github.com/jon/jira-tui/internal/worker"
)

func (m *Model) startQueryModal() {
	m.queryOpen = true
	m.queryMode = queryModeJQL
	m.queryJQLDraft = strings.TrimSpace(m.jql)
	m.queryJQLEditor = newQueryTextArea(m.queryJQLDraft, "Enter JQL")
	m.queryJQLEditorReady = true
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

func (m Model) updateQueryModal(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.queryAILoading {
		if msg.String() == "esc" {
			return m.cancelQueryAI(), nil
		}
		return m, nil
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
		}
		return m, nil
	case "ctrl+s":
		if m.queryMode == queryModeAI {
			if strings.TrimSpace(m.queryGeneratedJQL) != "" && strings.TrimSpace(m.queryAIPromptValue()) == strings.TrimSpace(m.queryGeneratedPrompt) {
				return m.applyQueryJQL(m.queryGeneratedJQL)
			}
			return m.submitQueryAI()
		}
		return m.applyQueryJQL(m.queryJQLValue())
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
	m.queryMode = queryModeJQL
	m.configureQueryJQLEditor(max(32, m.browserLayout(m.width).contentWidth-12), 4)
}

func (m Model) applyQueryJQL(jql string) (Model, tea.Cmd) {
	jql = strings.TrimSpace(jql)
	if jql == "" {
		m.detailNotice = "JQL cannot be empty."
		return m, nil
	}
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

func (m Model) queryModeLabel() string {
	if m.queryMode == queryModeAI {
		return "JQL  |  AI selected"
	}
	return "JQL selected  |  AI"
}

func (m Model) queryFooterText() string {
	if m.queryMode == queryModeAI {
		if strings.TrimSpace(m.queryGeneratedJQL) != "" {
			return "ctrl+s run preview  enter edit preview  tab direct JQL  esc cancel"
		}
		return "ctrl+s generate  tab direct JQL  esc cancel"
	}
	return "ctrl+s run  tab AI  esc cancel"
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
