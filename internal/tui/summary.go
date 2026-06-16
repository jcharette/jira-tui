package tui

import "charm.land/bubbles/v2/textarea"

func newSummaryEditor(value string) textarea.Model {
	editor := textarea.New()
	editor.Prompt = ""
	editor.Placeholder = "Edit summary..."
	editor.ShowLineNumbers = false
	editor.EndOfBufferCharacter = ' '
	editor.SetVirtualCursor(true)
	editor.SetValue(value)
	editor.Focus()
	return editor
}

func (m *Model) ensureSummaryEditor() {
	if m.summaryEditorReady {
		return
	}
	m.summaryEditor = newSummaryEditor(m.summaryDraft)
	m.summaryEditorReady = true
}

func (m *Model) configureSummaryEditor() {
	m.ensureSummaryEditor()
	width := max(32, m.browserLayout(m.width).contentWidth-16)
	m.summaryEditor.MaxHeight = 3
	m.summaryEditor.MaxWidth = width
	m.summaryEditor.SetWidth(width)
	m.summaryEditor.SetHeight(3)
}

func (m Model) configuredSummaryEditor(width int, rows int) textarea.Model {
	editor := m.summaryEditor
	if !m.summaryEditorReady {
		editor = newSummaryEditor(m.summaryDraft)
	}
	editor.MaxHeight = max(rows, 1)
	editor.MaxWidth = width
	editor.SetWidth(width)
	editor.SetHeight(rows)
	if !m.summarySubmitting {
		editor.Focus()
	} else {
		editor.Blur()
	}
	return editor
}

func (m Model) summaryEditorValue() string {
	if !m.summaryEditorReady {
		return m.summaryDraft
	}
	return m.summaryEditor.Value()
}
