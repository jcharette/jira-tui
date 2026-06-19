package tui

import (
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/jcharette/jira-tui/internal/jira"
)

func newComponentsFilterInput(value string) textinput.Model {
	editor := textinput.New()
	editor.Prompt = ""
	editor.Placeholder = "Filter components..."
	editor.SetValue(value)
	editor.CursorEnd()
	editor.Focus()
	editor.CharLimit = 80
	return editor
}

func (m *Model) ensureComponentsFilterEditor() {
	if m.componentsFilterEditorReady {
		return
	}
	m.componentsFilterEditor = newComponentsFilterInput(m.componentsFilter)
	m.componentsFilterEditorReady = true
}

func (m *Model) configureComponentsFilterEditor(width int) {
	m.ensureComponentsFilterEditor()
	m.componentsFilterEditor.SetWidth(max(24, width))
	m.componentsFilterEditor.Focus()
}

func (m Model) componentsFilterValue() string {
	if m.componentsFilterEditorReady {
		return m.componentsFilterEditor.Value()
	}
	return m.componentsFilter
}

func (m Model) updateComponentsEditor(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.Key().Code == tea.KeySpace {
		m.toggleSelectedComponent()
		return m, nil
	}
	switch msg.String() {
	case "esc":
		m.componentsFocus = false
		m.componentsDirty = false
		m.componentsFilter = ""
		m.componentsFilterEditor = textinput.Model{}
		m.componentsFilterEditorReady = false
		m.detailNotice = ""
		return m, nil
	case "enter":
		return m.submitSelectedComponents()
	case "up", "k":
		m.moveSelectedComponent(-1)
		return m, nil
	case "down", "j":
		m.moveSelectedComponent(1)
		return m, nil
	case " ":
		m.toggleSelectedComponent()
		return m, nil
	}
	m.configureComponentsFilterEditor(max(32, m.browserLayout(m.width).contentWidth-16))
	editor, cmd := m.componentsFilterEditor.Update(msg)
	m.componentsFilterEditor = editor
	m.componentsFilter = editor.Value()
	m.selectedComponent = clamp(m.selectedComponent, 0, max(0, len(m.filteredComponentIndexes())-1))
	return m, cmd
}

func (m *Model) moveSelectedComponent(delta int) {
	matches := m.filteredComponentIndexes()
	if len(matches) == 0 {
		m.selectedComponent = 0
		return
	}
	m.selectedComponent = clamp(m.selectedComponent+delta, 0, len(matches)-1)
}

func (m *Model) toggleSelectedComponent() {
	selected, ok := m.selectedIssue()
	if !ok {
		return
	}
	options := m.componentOptions(selected.Key)
	matches := m.filteredComponentIndexes()
	if len(options) == 0 || len(matches) == 0 {
		return
	}
	option := options[matches[clamp(m.selectedComponent, 0, len(matches)-1)]]
	key := componentSelectionKey(option)
	if key == "" {
		return
	}
	if m.selectedComponents == nil {
		m.selectedComponents = map[string]bool{}
	}
	m.selectedComponents[key] = !m.selectedComponents[key]
	m.componentsDirty = true
}

func (m Model) filteredComponentIndexes() []int {
	selected, ok := m.selectedIssue()
	if !ok {
		return nil
	}
	options := m.componentOptions(selected.Key)
	filter := strings.ToLower(strings.TrimSpace(m.componentsFilterValue()))
	indexes := make([]int, 0, len(options))
	for index, option := range options {
		if filter == "" || strings.Contains(strings.ToLower(displayValue(option.Name, option.ID)), filter) {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

func (m Model) selectedComponentOptions(key string) []jira.FieldOption {
	options := m.componentOptions(key)
	selected := make([]jira.FieldOption, 0, len(options))
	for _, option := range options {
		if m.selectedComponents[componentSelectionKey(option)] {
			selected = append(selected, option)
		}
	}
	return selected
}

func componentSelectionKey(option jira.FieldOption) string {
	return strings.ToLower(strings.TrimSpace(displayValue(option.ID, option.Name)))
}

func componentNamesFromOptions(options []jira.FieldOption) []string {
	names := make([]string, 0, len(options))
	for _, option := range options {
		name := strings.TrimSpace(displayValue(option.Name, option.ID))
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func componentOptionsEqual(a []jira.FieldOption, names []string) bool {
	left := componentNamesFromOptions(a)
	right := append([]string{}, names...)
	for i := range left {
		left[i] = strings.ToLower(strings.TrimSpace(left[i]))
	}
	for i := range right {
		right[i] = strings.ToLower(strings.TrimSpace(right[i]))
	}
	sort.Strings(left)
	sort.Strings(right)
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
