package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/jcharette/jira-tui/internal/jira"
)

type issueLinkRelation struct {
	Type        jira.IssueLinkType
	Direction   string
	Description string
}

func (m Model) startIssueLinkEditor() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok || strings.TrimSpace(selected.Key) == "" {
		m.detailNotice = "Select an issue before linking it."
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
	m.issueLinkFocus = true
	m.issueLinkTargetDraft = ""
	m.issueLinkTargetEditor = newIssueLinkTargetInput("")
	m.issueLinkTargetEditorReady = true
	m.issueLinkTargetEditor.Focus()
	m.selectedIssueLinkRelation = 0
	m.issueLinkTypesErr = nil
	m.detailNotice = ""
	if len(m.issueLinkTypes) > 0 {
		return m, nil
	}
	if m.issueLinkTypesLoading {
		return m, nil
	}
	m.nextRequestID++
	m.activeIssueLinkTypesReqID = m.nextRequestID
	m.issueLinkTypesLoading = true
	return m, m.submitIssueLinkTypes(m.activeIssueLinkTypesReqID)
}

func (m *Model) closeIssueLinkEditor() {
	m.issueLinkFocus = false
	m.issueLinkTypesLoading = false
	m.issueLinkTypesErr = nil
	m.issueLinkTargetDraft = ""
	m.issueLinkTargetEditor = textinput.Model{}
	m.issueLinkTargetEditorReady = false
	m.selectedIssueLinkRelation = 0
	m.issueLinkSubmitting = false
	m.issueLinkSubmitRequest = jira.CreateIssueLinkRequest{}
}

func (m Model) startIssueLinkDelete() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok || strings.TrimSpace(selected.Key) == "" {
		return m, nil
	}
	link, ok := m.selectedDetailLink()
	if !ok {
		m.detailNotice = "Select an issue link before removing it."
		return m, nil
	}
	if strings.TrimSpace(link.LinkID) == "" {
		m.detailNotice = "Only Jira issue links can be removed here."
		return m, nil
	}
	m.issueLinkDeleteConfirm = true
	m.issueLinkDeleteSubmitting = false
	m.issueLinkDeleteID = strings.TrimSpace(link.LinkID)
	m.issueLinkDeleteTarget = displayValue(link.CopyText, linkDisplayText(link))
	m.detailNotice = ""
	return m, nil
}

func (m Model) submitIssueLinkDelete() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok || strings.TrimSpace(selected.Key) == "" {
		return m, nil
	}
	if m.issueLinkDeleteSubmitting {
		return m, nil
	}
	linkID := strings.TrimSpace(m.issueLinkDeleteID)
	if linkID == "" {
		m.issueLinkDeleteConfirm = false
		m.detailNotice = "Issue link removal failed: missing link ID."
		return m, nil
	}
	m.nextRequestID++
	m.activeDeleteIssueLinkReqID = m.nextRequestID
	m.issueLinkDeleteSubmitting = true
	m.detailNotice = "Removing issue link."
	return m, m.submitDeleteIssueLink(m.activeDeleteIssueLinkReqID, selected.Key, linkID, m.issueLinkDeleteTarget)
}

func (m *Model) cancelIssueLinkDelete() {
	m.issueLinkDeleteConfirm = false
	m.issueLinkDeleteSubmitting = false
	m.issueLinkDeleteID = ""
	m.issueLinkDeleteTarget = ""
	m.detailNotice = ""
}

func (m Model) renderIssueLinkDeleteDialog(width int) string {
	selected, _ := m.selectedIssue()
	bodyWidth := max(36, width-6)
	target := displayValue(m.issueLinkDeleteTarget, "selected link")
	lines := []string{
		m.theme.Muted.Render("Issue") + " " + m.theme.Key.Render(displayValue(selected.Key, "selected")),
		m.theme.Muted.Render("Link") + " " + m.theme.Text.Render(target),
		"",
		m.theme.Error.Render("Remove this Jira issue link?"),
	}
	if m.issueLinkDeleteSubmitting {
		lines = append(lines, "", m.theme.Muted.Render("Removing link."))
	}
	if m.detailNotice != "" {
		lines = append(lines, "", m.renderDetailNotice(m.detailNotice, bodyWidth))
	}
	return m.renderDetailDialog(width, "Remove Link", selected.Key, strings.Join(lines, "\n"), "enter remove  esc cancel")
}

func (m Model) renderIssueLinkDialog(width int) string {
	selected, _ := m.selectedIssue()
	bodyWidth := max(36, width-6)
	lines := []string{m.detailSectionHeader("issue-link", "Link Issue", "", bodyWidth)}
	if m.issueLinkTypesLoading {
		lines = append(lines, "", m.theme.Muted.Render("Loading Jira link types."))
	}
	if m.issueLinkTypesErr != nil {
		lines = append(lines, "", m.theme.Error.Render("Link types failed: "+m.issueLinkTypesErr.Error()))
	}
	if m.issueLinkSubmitting {
		target := displayValue(m.issueLinkSubmitRequest.TargetKey, "target")
		lines = append(lines, "", m.theme.Muted.Render("Creating link to "+target+"."))
	}
	lines = append(lines, "", m.theme.Muted.Render("From")+" "+m.theme.Key.Render(displayValue(selected.Key, "selected")))
	lines = append(lines, m.theme.Muted.Render("Target")+" "+m.configuredIssueLinkTargetInput(bodyWidth).View())

	relations := m.issueLinkRelations()
	if len(relations) == 0 {
		lines = append(lines, "", m.theme.Muted.Render("No Jira link types are available."))
	} else {
		rows := make([][]string, 0, min(len(relations), 8))
		cursor := clamp(m.selectedIssueLinkRelation, 0, len(relations)-1)
		start := max(0, min(cursor-3, max(0, len(relations)-8)))
		end := min(len(relations), start+8)
		target := displayValue(m.issueLinkTargetDraft, "target")
		for index := start; index < end; index++ {
			relation := relations[index]
			marker := " "
			style := m.theme.Text
			if index == cursor {
				marker = ">"
				style = m.theme.Selected
			}
			rows = append(rows, []string{
				style.Render(marker),
				style.Render(relationLabel(relation, displayValue(selected.Key, "selected"), target)),
				m.theme.Muted.Render(displayValue(relation.Type.Name, relation.Type.ID)),
			})
		}
		lines = append(lines, "")
		lines = append(lines, m.detailTable(0, []string{"", "RELATION", "TYPE"}, rows, nil))
		if len(relations) > 8 {
			lines = append(lines, m.theme.Muted.Render(fmt.Sprintf("+%d more link relationships", len(relations)-8)))
		}
	}
	return m.renderDetailDialog(width, "Link Issue", selected.Key, strings.Join(lines, "\n"), "enter save  up/down relation  esc cancel")
}

func (m Model) updateIssueLinkEditor(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.closeIssueLinkEditor()
		return m, nil
	case "enter":
		return m.submitSelectedIssueLink()
	case "up":
		m.moveSelectedIssueLinkRelation(-1)
		return m, nil
	case "down":
		m.moveSelectedIssueLinkRelation(1)
		return m, nil
	}
	editor := m.configuredIssueLinkTargetInput(max(20, m.browserLayout(m.width).contentWidth-12))
	before := editor.Value()
	updated, cmd := editor.Update(msg)
	m.issueLinkTargetEditor = updated
	m.issueLinkTargetDraft = strings.ToUpper(strings.TrimSpace(updated.Value()))
	if m.issueLinkTargetDraft != strings.ToUpper(strings.TrimSpace(before)) {
		m.detailNotice = ""
	}
	return m, cmd
}

func (m *Model) moveSelectedIssueLinkRelation(delta int) {
	relations := m.issueLinkRelations()
	if len(relations) == 0 {
		m.selectedIssueLinkRelation = 0
		return
	}
	m.selectedIssueLinkRelation = clamp(m.selectedIssueLinkRelation+delta, 0, len(relations)-1)
}

func (m Model) submitSelectedIssueLink() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok || strings.TrimSpace(selected.Key) == "" {
		return m, nil
	}
	if m.issueLinkSubmitting {
		return m, nil
	}
	target := strings.ToUpper(strings.TrimSpace(m.issueLinkTargetDraft))
	if target == "" && m.issueLinkTargetEditorReady {
		target = strings.ToUpper(strings.TrimSpace(m.issueLinkTargetEditor.Value()))
	}
	if target == "" {
		m.detailNotice = "Enter a target issue key."
		return m, nil
	}
	if strings.EqualFold(target, selected.Key) {
		m.detailNotice = "Target issue must be different."
		return m, nil
	}
	relations := m.issueLinkRelations()
	if len(relations) == 0 {
		m.detailNotice = "No Jira link types are available."
		return m, nil
	}
	relation := relations[clamp(m.selectedIssueLinkRelation, 0, len(relations)-1)]
	request := jira.CreateIssueLinkRequest{
		SourceKey: strings.ToUpper(strings.TrimSpace(selected.Key)),
		TargetKey: target,
		Type:      relation.Type,
		Direction: relation.Direction,
	}
	m.nextRequestID++
	m.activeCreateIssueLinkReqID = m.nextRequestID
	m.issueLinkSubmitting = true
	m.issueLinkSubmitRequest = request
	m.detailNotice = "Creating issue link."
	return m, m.submitCreateIssueLink(m.activeCreateIssueLinkReqID, request)
}

func (m Model) issueLinkRelations() []issueLinkRelation {
	relations := make([]issueLinkRelation, 0, len(m.issueLinkTypes)*2)
	for _, linkType := range m.issueLinkTypes {
		outward := strings.TrimSpace(linkType.Outward)
		if outward == "" {
			outward = displayValue(linkType.Name, linkType.ID)
		}
		inward := strings.TrimSpace(linkType.Inward)
		if inward == "" {
			inward = displayValue(linkType.Name, linkType.ID)
		}
		relations = append(relations, issueLinkRelation{
			Type:        linkType,
			Direction:   "outward",
			Description: outward,
		})
		if !strings.EqualFold(inward, outward) {
			relations = append(relations, issueLinkRelation{
				Type:        linkType,
				Direction:   "inward",
				Description: inward,
			})
		}
	}
	return relations
}

func relationLabel(relation issueLinkRelation, source string, target string) string {
	return strings.TrimSpace(source + " " + displayValue(relation.Description, relation.Type.Name) + " " + target)
}

func (m Model) configuredIssueLinkTargetInput(width int) textinput.Model {
	editor := m.issueLinkTargetEditor
	if !m.issueLinkTargetEditorReady {
		editor = newIssueLinkTargetInput(m.issueLinkTargetDraft)
	}
	editor.SetWidth(max(12, min(24, width-10)))
	return editor
}

func newIssueLinkTargetInput(value string) textinput.Model {
	editor := textinput.New()
	editor.Placeholder = "ABC-123"
	editor.SetValue(value)
	editor.CharLimit = 32
	editor.SetWidth(18)
	editor.Prompt = ""
	editor.Focus()
	return editor
}

func issueLinkBindings() []keyBinding {
	return []keyBinding{
		{Keys: []string{"type"}, Label: "target", Description: "Type the target Jira issue key.", Group: "Issue Link", Footer: true},
		{Keys: []string{"up", "down"}, FooterKey: "up/down", Label: "relation", Description: "Select the Jira link relationship.", Group: "Issue Link", Footer: true},
		{Keys: []string{"enter"}, Label: "save", Description: "Create the issue link through Jira.", Group: "Issue Link", Footer: true},
		{Keys: []string{"esc"}, Label: "cancel", Description: "Cancel issue link creation.", Group: "Issue Link", Footer: true},
	}
}
