package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/jcharette/jira-tui/internal/jira"
)

type sprintChoice struct {
	Sprint      jira.Sprint
	Label       string
	Description string
}

func (m Model) startSprintPicker() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok || strings.TrimSpace(selected.Key) == "" {
		m.detailNotice = "Select an issue before changing sprint."
		return m, nil
	}
	choices := m.sprintChoices()
	if len(choices) == 0 {
		if m.planningBoardsLoading || m.planningSprintsLoading {
			m.detailNotice = "Sprint metadata is still loading."
			return m, nil
		}
		m.detailNotice = "No active or future sprints found. Set Default Board ID in jira config if needed."
		if next, cmd := m.startPlanningMetadataLoad(); cmd != nil {
			return next, cmd
		}
		return m, nil
	}
	m.linkFocus = false
	m.hierarchyFocus = false
	m.actionFocus = false
	m.transitionFocus = false
	m.priorityFocus = false
	m.labelsFocus = false
	m.componentsFocus = false
	m.genericFieldFocus = false
	m.assigneeFocus = false
	m.issueLinkFocus = false
	m.worklogFocus = false
	m.summaryFocus = false
	m.sprintFocus = true
	m.selectedSprint = clamp(m.selectedSprint, 0, len(choices)-1)
	m.detailNotice = ""
	return m, nil
}

func (m Model) renderSprintDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	bodyWidth := min(max(24, width-12), 68)
	if m.sprintSubmitting {
		name := displayValue(m.sprintSubmit.Name, fmt.Sprintf("Sprint %d", m.sprintSubmit.ID))
		return m.renderDetailDialog(width, "Sprint", selected.Key, m.detailStatusBlock("Adding to "+name+"...", bodyWidth, false), "esc cancel")
	}
	choices := m.sprintChoices()
	lines := []string{m.theme.Muted.Render("Choose the sprint for this ticket. Active sprints are listed first.")}
	if m.defaultBoardID > 0 {
		lines = append(lines, m.theme.Muted.Render(fmt.Sprintf("Board: %d", m.defaultBoardID)))
	}
	lines = append(lines, "")
	if len(choices) == 0 {
		lines = append(lines, m.detailStatusBlock("No active or future sprints loaded.", bodyWidth, false))
		return m.renderDetailDialog(width, "Sprint", selected.Key, strings.Join(lines, "\n"), "esc cancel")
	}
	cursor := clamp(m.selectedSprint, 0, len(choices)-1)
	for index, choice := range choices {
		marker := " "
		labelStyle := m.theme.Text
		if index == cursor {
			marker = ">"
			labelStyle = m.theme.Selected
		}
		lines = append(lines, fmt.Sprintf("%s %-28s %s", labelStyle.Render(marker), labelStyle.Render(truncate(choice.Label, 28)), m.theme.Muted.Render(choice.Description)))
	}
	return m.renderDetailDialog(width, "Sprint", selected.Key, strings.Join(lines, "\n"), "j/k select  enter add  esc cancel")
}

func (m *Model) moveSelectedSprint(delta int) {
	choices := m.sprintChoices()
	if len(choices) == 0 {
		m.selectedSprint = 0
		return
	}
	m.selectedSprint = clamp(m.selectedSprint+delta, 0, len(choices)-1)
}

func (m Model) submitSelectedSprint() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok || strings.TrimSpace(selected.Key) == "" {
		m.detailNotice = "Select an issue before changing sprint."
		return m, nil
	}
	choices := m.sprintChoices()
	if len(choices) == 0 {
		m.detailNotice = "No active or future sprint selected."
		return m, nil
	}
	choice := choices[clamp(m.selectedSprint, 0, len(choices)-1)]
	m.nextRequestID++
	m.activeSprintReqID = m.nextRequestID
	m.sprintSubmitting = true
	m.sprintSubmitKey = selected.Key
	m.sprintSubmit = choice.Sprint
	m.detailNotice = "Adding " + selected.Key + " to " + displayValue(choice.Sprint.Name, "selected sprint") + "."
	return m, m.submitMoveIssuesToSprint(m.activeSprintReqID, choice.Sprint, []string{selected.Key})
}

func (m Model) sprintChoices() []sprintChoice {
	sprints := m.sprintsForSprintAction()
	active := make([]sprintChoice, 0, len(sprints))
	future := make([]sprintChoice, 0, len(sprints))
	for _, sprint := range sprints {
		state := strings.ToLower(strings.TrimSpace(sprint.State))
		choice := sprintChoice{
			Sprint:      sprint,
			Label:       displayValue(sprint.Name, fmt.Sprintf("Sprint %d", sprint.ID)),
			Description: displayValue(state, "unknown"),
		}
		switch state {
		case "active":
			active = append(active, choice)
		case "future":
			future = append(future, choice)
		}
	}
	return append(active, future...)
}

func (m Model) sprintsForSprintAction() []jira.Sprint {
	boardID := m.defaultBoardID
	if boardID <= 0 {
		boardID = m.planningBoardID
	}
	if boardID > 0 {
		return append([]jira.Sprint{}, m.planningSprints[boardID]...)
	}
	var sprints []jira.Sprint
	for _, boardSprints := range m.planningSprints {
		sprints = append(sprints, boardSprints...)
	}
	return sprints
}
