package tui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

type actionPaletteAction struct {
	Action detailAction
	Index  int
	Group  string
}

func (m *Model) openActionPalette() {
	m.actionPaletteOpen = true
	m.actionPaletteFilter = ""
	m.actionPaletteEditor = newActionPaletteFilterInput("")
	m.actionPaletteEditorReady = true
	m.selectedActionPalette = 0
	m.linkFocus = false
	m.hierarchyFocus = false
	m.commentFocus = false
	m.actionFocus = false
	m.transitionFocus = false
	m.priorityFocus = false
	m.assigneeFocus = false
	m.summaryFocus = false
	m.detailNotice = ""
}

func (m *Model) closeActionPalette() {
	m.actionPaletteOpen = false
	m.actionPaletteFilter = strings.TrimSpace(m.actionPaletteEditorValue())
	m.actionPaletteEditor = textinput.Model{}
	m.actionPaletteEditorReady = false
	m.selectedActionPalette = 0
}

func (m Model) renderActionPalette(layout browserLayout) string {
	selected, ok := m.selectedIssue()
	subtitle := ""
	if ok {
		subtitle = selected.Key
	}
	width := max(44, min(layout.contentWidth-4, 82))
	bodyWidth := max(32, width-8)
	m.configureActionPaletteEditor(bodyWidth)
	actions := m.filteredActionPaletteActions()
	cursor := clamp(m.selectedActionPalette, 0, max(0, len(actions)-1))

	lines := []string{
		m.theme.FieldLabel.Render("Filter"),
		m.actionPaletteEditor.View(),
	}
	if len(actions) == 0 {
		lines = append(lines, "", m.detailEmptyState("No ticket actions matched.", bodyWidth))
		return m.renderDetailDialogWithLimit(layout.contentWidth, "Ticket Actions", subtitle, strings.Join(lines, "\n"), "esc close", width)
	}

	rows := make([][]string, 0, len(actions))
	for index, item := range actions {
		marker := " "
		labelStyle := m.theme.Text
		groupStyle := m.theme.FieldLabel
		descStyle := m.theme.Muted
		state := item.Group
		if index == cursor {
			marker = ">"
			labelStyle = m.theme.Selected
			groupStyle = m.theme.Selected
		} else if !item.Action.Enabled {
			labelStyle = m.theme.Muted
			groupStyle = m.theme.Muted
			descStyle = m.theme.Muted
			state = displayValue(item.Action.DisabledState, "Needs Metadata")
		}
		rows = append(rows, []string{
			labelStyle.Render(marker),
			labelStyle.Render(item.Action.Label),
			groupStyle.Render(state),
			descStyle.Render(truncate(item.Action.Description, max(18, bodyWidth-40))),
		})
	}
	lines = append(lines, "", m.detailTable(0, []string{"", "ACTION", "TYPE", "DETAIL"}, rows, nil))
	footer := "type filter  j/k select  enter run  esc close"
	return m.renderDetailDialogWithLimit(layout.contentWidth, "Ticket Actions", subtitle, strings.Join(lines, "\n"), footer, width)
}

func (m Model) updateActionPalette(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.closeActionPalette()
		return m, nil
	case "enter":
		return m.runSelectedActionPaletteAction()
	case "up", "k":
		m.moveSelectedActionPalette(-1)
		return m, nil
	case "down", "j":
		m.moveSelectedActionPalette(1)
		return m, nil
	}
	m.configureActionPaletteEditor(max(32, m.browserLayout(m.width).contentWidth-12))
	editor, cmd := m.actionPaletteEditor.Update(msg)
	m.actionPaletteEditor = editor
	m.actionPaletteFilter = m.actionPaletteEditor.Value()
	m.selectedActionPalette = clamp(m.selectedActionPalette, 0, max(0, len(m.filteredActionPaletteActions())-1))
	return m, cmd
}

func (m *Model) moveSelectedActionPalette(delta int) {
	actions := m.filteredActionPaletteActions()
	if len(actions) == 0 {
		m.selectedActionPalette = 0
		return
	}
	m.selectedActionPalette = clamp(m.selectedActionPalette+delta, 0, len(actions)-1)
}

func (m Model) runSelectedActionPaletteAction() (Model, tea.Cmd) {
	actions := m.filteredActionPaletteActions()
	if len(actions) == 0 {
		m.detailNotice = "No ticket actions matched the current filter."
		return m, nil
	}
	item := actions[clamp(m.selectedActionPalette, 0, len(actions)-1)]
	m.selectedAction = item.Index
	m.closeActionPalette()
	return m.runSelectedDetailAction()
}

func (m Model) filteredActionPaletteActions() []actionPaletteAction {
	actions := m.detailActions()
	filter := strings.ToLower(strings.TrimSpace(m.actionPaletteEditorValue()))
	filtered := make([]actionPaletteAction, 0, len(actions))
	for index, action := range actions {
		group := actionPaletteGroup(action.ID)
		if filter != "" {
			haystack := strings.ToLower(strings.Join([]string{action.ID, action.Label, group, action.Description}, " "))
			if !strings.Contains(haystack, filter) {
				continue
			}
		}
		filtered = append(filtered, actionPaletteAction{
			Action: action,
			Index:  index,
			Group:  group,
		})
	}
	return filtered
}

func actionPaletteGroup(id string) string {
	if strings.HasPrefix(id, "field:") {
		return "Field"
	}
	switch id {
	case "comment":
		return "Comment"
	case "browser", "copy-key", "copy-url":
		return "Open/Copy"
	case "summary", "priority", "labels", "components", "assign":
		return "Field"
	case "transition":
		return "Workflow"
	case "subtask":
		return "Create"
	default:
		return "Action"
	}
}

func (m *Model) configureActionPaletteEditor(width int) {
	if !m.actionPaletteEditorReady {
		m.actionPaletteEditor = newActionPaletteFilterInput(m.actionPaletteFilter)
		m.actionPaletteEditorReady = true
	}
	m.actionPaletteEditor.SetWidth(max(24, width))
	m.actionPaletteEditor.Focus()
}

func (m *Model) actionPaletteEditorValue() string {
	if m.actionPaletteEditorReady {
		return m.actionPaletteEditor.Value()
	}
	return m.actionPaletteFilter
}

func newActionPaletteFilterInput(value string) textinput.Model {
	editor := textinput.New()
	editor.Prompt = ""
	editor.Placeholder = "Filter actions..."
	editor.SetValue(value)
	editor.CursorEnd()
	editor.Focus()
	editor.CharLimit = 80
	return editor
}
