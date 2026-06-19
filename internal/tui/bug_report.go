package tui

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

const (
	bugReportIssueURL              = "https://github.com/jcharette/jira-tui/issues/new"
	bugReportDiagnosticsEventLimit = 24
	bugReportDiagnosticsCharLimit  = 2600
	bugReportBodyCharLimit         = 1800
)

func (m Model) startBugReport() Model {
	m.bugReportOpen = true
	m.bugReportFieldFocus = 0
	m.bugReportTitleDraft = ""
	m.bugReportBodyDraft = ""
	m.bugReportIncludeDiagnostics = false
	m.bugReportTitleEditor = newBugReportTitleInput("")
	m.bugReportTitleEditorReady = true
	m.bugReportBodyEditor = newBugReportBodyEditor("")
	m.bugReportBodyEditorReady = true
	m.detailNotice = ""
	return m
}

func (m *Model) closeBugReport() {
	m.bugReportOpen = false
	m.bugReportFieldFocus = 0
	m.bugReportTitleDraft = ""
	m.bugReportBodyDraft = ""
	m.bugReportTitleEditor = textinput.Model{}
	m.bugReportTitleEditorReady = false
	m.bugReportBodyEditor = textarea.Model{}
	m.bugReportBodyEditorReady = false
}

func (m Model) updateBugReport(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.closeBugReport()
		return m, nil
	case "tab", "down":
		m.moveBugReportFocus(1)
		return m, nil
	case "shift+tab", "backtab", "up":
		m.moveBugReportFocus(-1)
		return m, nil
	case "ctrl+s":
		return m.submitBugReport()
	case "space":
		if m.bugReportFieldFocus == 2 {
			m.bugReportIncludeDiagnostics = !m.bugReportIncludeDiagnostics
			return m, nil
		}
	}
	if m.bugReportFieldFocus == 0 {
		m.ensureBugReportTitleEditor()
		editor, _ := m.bugReportTitleEditor.Update(msg)
		m.bugReportTitleEditor = editor
		m.bugReportTitleDraft = editor.Value()
		return m, nil
	}
	if m.bugReportFieldFocus == 1 {
		m.ensureBugReportBodyEditor()
		editor, _ := m.bugReportBodyEditor.Update(msg)
		m.bugReportBodyEditor = editor
		m.bugReportBodyDraft = editor.Value()
		return m, nil
	}
	return m, nil
}

func (m *Model) moveBugReportFocus(delta int) {
	m.bugReportFieldFocus = clamp(m.bugReportFieldFocus+delta, 0, 2)
}

func (m Model) submitBugReport() (Model, tea.Cmd) {
	title := strings.TrimSpace(m.bugReportTitleValue())
	description := strings.TrimSpace(m.bugReportBodyValue())
	if title == "" && description == "" {
		m.detailNotice = "Add a short title or description before opening a bug report."
		return m, nil
	}
	if title == "" {
		title = truncate(firstLine(description), 80)
	}
	body := m.buildBugReportBody(description, m.bugReportIncludeDiagnostics)
	reportURL := m.buildBugReportURL(title, body)
	includeDiagnostics := m.bugReportIncludeDiagnostics && len(m.diagnosticsEvents) > 0
	m.recordDiagnosticEvent(diagnosticKindState, "bug_report", "open", fmt.Sprintf("include_diagnostics=%t events=%d", includeDiagnostics, len(m.diagnosticsEvents)))
	m.closeBugReport()
	m.detailNotice = "Opening GitHub bug report."
	return m, func() tea.Msg {
		return linkActionMsg{
			action: "open",
			target: "GitHub bug report",
			err:    openExternal(reportURL),
		}
	}
}

func (m Model) renderBugReport(layout browserLayout) string {
	bodyWidth := min(max(32, layout.contentWidth-12), 74)
	var lines []string
	lines = append(lines, m.detailSectionHeader("bug-report-title", "Title", m.bugReportFocusHelp(0), bodyWidth))
	titleEditor := m.configuredBugReportTitleInput(bodyWidth)
	lines = append(lines, titleEditor.View())
	lines = append(lines, "")
	lines = append(lines, m.detailSectionHeader("bug-report-body", "What happened", m.bugReportFocusHelp(1), bodyWidth))
	bodyEditor := m.configuredBugReportBodyEditor(bodyWidth, min(8, max(4, layout.rows/4)))
	lines = append(lines, bodyEditor.View())
	lines = append(lines, "")
	lines = append(lines, m.renderBugReportDiagnosticsToggle(bodyWidth))
	lines = append(lines, "")
	lines = append(lines, m.theme.Muted.Render(truncate("Opens a prefilled GitHub issue. Jira tokens, raw request bodies, response bodies, and full JQL are never included.", bodyWidth)))
	if m.detailNotice != "" {
		lines = append(lines, "", m.renderDetailNotice(m.detailNotice, bodyWidth))
	}
	return m.renderDetailDialogWithLimit(layout.contentWidth, "Report Bug", "GitHub Issues", strings.Join(lines, "\n"), "ctrl+s open  tab field  space diagnostics  esc cancel", 84)
}

func (m Model) renderBugReportDiagnosticsToggle(width int) string {
	marker := "[ ]"
	if m.bugReportIncludeDiagnostics {
		marker = "[x]"
	}
	style := m.theme.Text
	if m.bugReportFieldFocus == 2 {
		style = m.theme.Selected
	}
	available := fmt.Sprintf("%d sanitized events", min(len(m.diagnosticsEvents), bugReportDiagnosticsEventLimit))
	if len(m.diagnosticsEvents) == 0 {
		available = "no Diagnostics events yet"
	}
	line := fmt.Sprintf("%s Include sanitized Diagnostics excerpt (%s)", marker, available)
	return style.Render(truncate(line, width))
}

func (m Model) bugReportFocusHelp(index int) string {
	if m.bugReportFieldFocus == index {
		return "editing"
	}
	return ""
}

func (m Model) buildBugReportURL(title string, body string) string {
	values := url.Values{}
	values.Set("title", title)
	values.Set("body", body)
	values.Set("labels", "bug")
	return bugReportIssueURL + "?" + values.Encode()
}

func (m Model) buildBugReportBody(description string, includeDiagnostics bool) string {
	var b strings.Builder
	b.WriteString("## What happened\n")
	if strings.TrimSpace(description) == "" {
		b.WriteString("_No details provided._\n")
	} else {
		b.WriteString(truncate(strings.TrimSpace(description), bugReportBodyCharLimit))
		b.WriteString("\n")
	}
	b.WriteString("\n## App context\n")
	b.WriteString("- Active view: " + sanitizeIssueBodyLine(m.activeViewName()) + "\n")
	b.WriteString("- Layout: " + sanitizeIssueBodyLine(m.issueLayoutModeLabel()) + "\n")
	if selected, ok := m.selectedIssue(); ok && strings.TrimSpace(selected.Key) != "" {
		b.WriteString("- Selected issue: " + sanitizeIssueBodyLine(selected.Key) + "\n")
	}
	if m.diagnosticLogPath != "" {
		b.WriteString("- Local diagnostics log: " + sanitizeIssueBodyLine(m.diagnosticLogPath) + "\n")
	}
	b.WriteString("- Diagnostics excerpt included: ")
	if includeDiagnostics && len(m.diagnosticsEvents) > 0 {
		b.WriteString("yes\n")
		b.WriteString("\n## Sanitized diagnostics excerpt\n")
		b.WriteString("Generated by jira-tui Diagnostics. It omits tokens, raw request bodies, response bodies, and full JQL.\n\n")
		b.WriteString("```text\n")
		b.WriteString(m.bugReportDiagnosticsExcerpt())
		b.WriteString("\n```\n")
	} else {
		b.WriteString("no\n")
	}
	return b.String()
}

func (m Model) bugReportDiagnosticsExcerpt() string {
	events := m.recentDiagnosticEvents(bugReportDiagnosticsEventLimit)
	if len(events) == 0 {
		return ""
	}
	lines := make([]string, 0, len(events))
	for _, event := range events {
		at := event.At
		if at.IsZero() {
			at = time.Time{}
		}
		timestamp := at.Format("15:04:05")
		if at.IsZero() {
			timestamp = "--:--:--"
		}
		line := fmt.Sprintf("%s %-7s %-8s %s", timestamp, event.Kind, event.Status, redactDiagnosticText(diagnosticEventDetail(event)))
		lines = append(lines, truncate(line, 140))
	}
	return truncate(strings.Join(lines, "\n"), bugReportDiagnosticsCharLimit)
}

func sanitizeIssueBodyLine(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.TrimSpace(value)
}

func (m Model) bugReportTitleValue() string {
	if m.bugReportTitleEditorReady {
		return m.bugReportTitleEditor.Value()
	}
	return m.bugReportTitleDraft
}

func (m Model) bugReportBodyValue() string {
	if m.bugReportBodyEditorReady {
		return m.bugReportBodyEditor.Value()
	}
	return m.bugReportBodyDraft
}

func (m *Model) ensureBugReportTitleEditor() {
	if m.bugReportTitleEditorReady {
		return
	}
	m.bugReportTitleEditor = newBugReportTitleInput(m.bugReportTitleDraft)
	m.bugReportTitleEditorReady = true
}

func (m *Model) ensureBugReportBodyEditor() {
	if m.bugReportBodyEditorReady {
		return
	}
	m.bugReportBodyEditor = newBugReportBodyEditor(m.bugReportBodyDraft)
	m.bugReportBodyEditorReady = true
}

func (m Model) configuredBugReportTitleInput(width int) textinput.Model {
	editor := m.bugReportTitleEditor
	if !m.bugReportTitleEditorReady {
		editor = newBugReportTitleInput(m.bugReportTitleDraft)
	}
	editor.SetWidth(width)
	if m.bugReportFieldFocus == 0 {
		editor.Focus()
	} else {
		editor.Blur()
	}
	return editor
}

func (m Model) configuredBugReportBodyEditor(width int, rows int) textarea.Model {
	editor := m.bugReportBodyEditor
	if !m.bugReportBodyEditorReady {
		editor = newBugReportBodyEditor(m.bugReportBodyDraft)
	}
	editor.MaxWidth = width
	editor.MaxHeight = rows
	editor.SetWidth(width)
	editor.SetHeight(rows)
	if m.bugReportFieldFocus == 1 {
		editor.Focus()
	} else {
		editor.Blur()
	}
	return editor
}

func newBugReportTitleInput(value string) textinput.Model {
	editor := textinput.New()
	editor.Prompt = ""
	editor.Placeholder = "Short bug summary"
	editor.CharLimit = 100
	editor.SetWidth(60)
	editor.SetValue(value)
	editor.Focus()
	return editor
}

func newBugReportBodyEditor(value string) textarea.Model {
	editor := textarea.New()
	editor.Prompt = ""
	editor.Placeholder = "What did you see? What were you trying to do?"
	editor.ShowLineNumbers = false
	editor.EndOfBufferCharacter = ' '
	editor.CharLimit = bugReportBodyCharLimit
	editor.SetHeight(6)
	editor.SetValue(value)
	return editor
}
