package tui

import (
	"sort"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
)

func newLabelsEditor(value string) textarea.Model {
	editor := textarea.New()
	editor.Prompt = ""
	editor.Placeholder = "platform, backend"
	editor.ShowLineNumbers = false
	editor.EndOfBufferCharacter = ' '
	editor.SetVirtualCursor(true)
	editor.SetValue(value)
	editor.Focus()
	return editor
}

func (m *Model) ensureLabelsEditor() {
	if m.labelsEditorReady {
		return
	}
	m.labelsEditor = newLabelsEditor(m.labelsDraft)
	m.labelsEditorReady = true
}

func (m *Model) configureLabelsEditor() {
	m.ensureLabelsEditor()
	width := max(32, m.browserLayout(m.width).contentWidth-16)
	m.labelsEditor.MaxHeight = 3
	m.labelsEditor.MaxWidth = width
	m.labelsEditor.SetWidth(width)
	m.labelsEditor.SetHeight(3)
}

func (m Model) configuredLabelsEditor(width int, rows int) textarea.Model {
	editor := m.labelsEditor
	if !m.labelsEditorReady {
		editor = newLabelsEditor(m.labelsDraft)
	}
	editor.MaxHeight = max(rows, 1)
	editor.MaxWidth = width
	editor.SetWidth(width)
	editor.SetHeight(rows)
	if !m.labelsSubmitting {
		editor.Focus()
	} else {
		editor.Blur()
	}
	return editor
}

func (m Model) labelsEditorValue() string {
	if !m.labelsEditorReady {
		return m.labelsDraft
	}
	return m.labelsEditor.Value()
}

func (m Model) updateLabelsEditor(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.labelsFocus = false
		m.labelsEditing = false
		m.labelsDirty = false
		m.labelsDraft = ""
		m.labelsEditor = textarea.Model{}
		m.labelsEditorReady = false
		m.detailNotice = ""
		return m, nil
	case "enter":
		return m.submitSelectedLabels()
	}
	m.configureLabelsEditor()
	editor, cmd := m.labelsEditor.Update(msg)
	m.labelsEditor = editor
	m.labelsDraft = m.labelsEditor.Value()
	m.labelsDirty = true
	return m, cmd
}

func parseLabelsDraft(value string) []string {
	parts := strings.Split(value, ",")
	labels := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		label := strings.TrimSpace(part)
		if label == "" {
			continue
		}
		key := strings.ToLower(label)
		if seen[key] {
			continue
		}
		seen[key] = true
		labels = append(labels, label)
	}
	if labels == nil {
		return []string{}
	}
	return labels
}

func labelsEqual(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	left := append([]string{}, a...)
	right := append([]string{}, b...)
	for i := range left {
		left[i] = strings.ToLower(strings.TrimSpace(left[i]))
	}
	for i := range right {
		right[i] = strings.ToLower(strings.TrimSpace(right[i]))
	}
	sort.Strings(left)
	sort.Strings(right)
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
