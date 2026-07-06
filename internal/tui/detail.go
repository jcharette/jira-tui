package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
	lipglosstable "github.com/charmbracelet/lipgloss/table"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/jcharette/jira-tui/internal/linkdetect"
	"github.com/jcharette/jira-tui/internal/worker"
)

type detailLink struct {
	Kind     string
	Label    string
	Target   string
	CopyText string
	LinkID   string
	Start    int
	End      int
}

type detailAction struct {
	ID             string
	Label          string
	Description    string
	Enabled        bool
	DisabledState  string
	DisabledReason string
}

type detailSection struct {
	ID    string
	Label string
	Short string
	Badge string
}

type detailTargetKind string

type detailTarget struct {
	ID      string
	Label   string
	Kind    detailTargetKind
	Section detailSection
}

type hierarchyRow struct {
	Issue jira.Issue
	Group string
	Index int
}

type detailRenderContext struct {
	selected    jira.Issue
	display     jira.Issue
	detail      jira.IssueDetail
	hasDetail   bool
	description string
	links       []detailLink
}

func (m Model) renderFullDetail(layout browserLayout) string {
	bodyWidth := max(32, layout.contentWidth-8)
	body := m.renderScrollableDetailBody(m.fullDetailContent(bodyWidth), bodyWidth)
	headerWidth := max(32, layout.contentWidth-4)
	header := m.renderDetailTitleLine(headerWidth) + "\n" +
		m.renderDetailSummaryLine(headerWidth) + "\n" +
		m.renderDetailControlStrip(headerWidth) + "\n" +
		m.renderDetailTabs(headerWidth)
	if overlay := m.renderDetailOverlay(layout); overlay != "" {
		body = placeDetailOverlay(body, overlay, m.fullDetailRows())
	}
	content := header + "\n\n" + body
	return m.theme.ActivePane.Width(layout.contentWidth).Render(content)
}

func placeDetailOverlay(body string, overlay string, rows int) string {
	if strings.TrimSpace(overlay) == "" {
		return body
	}
	rows = max(1, rows)
	overlayLines := strings.Split(strings.TrimRight(overlay, "\n"), "\n")
	rows = max(rows, len(overlayLines))
	bodyLines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	for len(bodyLines) < rows {
		bodyLines = append(bodyLines, "")
	}
	if len(bodyLines) > rows {
		bodyLines = bodyLines[:rows]
	}
	if len(overlayLines) == 0 {
		return strings.Join(bodyLines, "\n")
	}
	start := max(0, (rows-len(overlayLines))/2)
	for index, line := range overlayLines {
		target := start + index
		if target >= rows {
			break
		}
		bodyLines[target] = line
	}
	return strings.Join(bodyLines, "\n")
}

func (m Model) renderDetailOverlay(layout browserLayout) string {
	width := max(32, layout.contentWidth-12)
	if m.summaryMetadataLoading {
		return m.renderSummaryLoadingDialog(width)
	}
	if m.summaryEditing || m.summarySubmitting {
		return m.renderSummaryDialog(width)
	}
	if m.priorityMetadataLoading {
		return m.renderPriorityLoadingDialog(width)
	}
	if m.priorityFocus || m.prioritySubmitting {
		return m.renderPriorityDialog(width)
	}
	if m.labelsMetadataLoading {
		return m.renderLabelsLoadingDialog(width)
	}
	if m.labelsFocus || m.labelsSubmitting {
		return m.renderLabelsDialog(width)
	}
	if m.componentsMetadataLoading {
		return m.renderComponentsLoadingDialog(width)
	}
	if m.componentsFocus || m.componentsSubmitting {
		return m.renderComponentsDialog(width)
	}
	if m.sprintFocus || m.sprintSubmitting {
		return m.renderSprintDialog(width)
	}
	if m.genericFieldMetadataLoading {
		return m.renderGenericFieldLoadingDialog(width)
	}
	if m.parentFocus || m.parentSubmitting {
		return m.renderParentDialog(width)
	}
	if m.timeTrackingFocus || m.timeTrackingSubmitting {
		return m.renderTimeTrackingDialog(width)
	}
	if m.genericFieldFocus || m.genericFieldSubmitting {
		return m.renderGenericFieldDialog(width)
	}
	if m.startWorkflowOpen {
		return m.renderStartWorkflowDialog(width)
	}
	if m.assigneeFocus || m.assigneeSubmitting {
		return m.renderAssigneeDialog(width)
	}
	if m.issueLinkDeleteConfirm || m.issueLinkDeleteSubmitting {
		return m.renderIssueLinkDeleteDialog(width)
	}
	if m.issueLinkFocus || m.issueLinkSubmitting {
		return m.renderIssueLinkDialog(width)
	}
	if m.worklogDeleteConfirm || m.worklogDeleteSubmitting {
		return m.renderWorklogDeleteDialog(width)
	}
	if m.worklogFocus || m.worklogSubmitting {
		return m.renderWorklogDialog(width)
	}
	if m.transitionFocus || m.transitionSubmitting {
		return m.renderStatusTransitionDialog(width)
	}
	if m.inlineAIOpen {
		return m.renderInlineAIDialog(width)
	}
	if m.claudeSubtaskReviewOpen {
		return m.renderClaudeSubtaskReviewDialog(width)
	}
	if m.claudeAssistLoading || m.claudeAssistOpen {
		return m.renderClaudeAssistDialog(width)
	}
	if m.claudePlanLoading || m.claudePlanOpen {
		return m.renderClaudePlanDialog(width)
	}
	return ""
}

func (m Model) renderSummaryLoadingDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	bodyWidth := min(max(24, width-12), 60)
	body := m.detailStatusBlock("Loading summary metadata...", bodyWidth, false)
	return m.renderDetailDialog(width, "Edit Summary", selected.Key, body, "esc cancel")
}

func (m Model) renderPriorityLoadingDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	bodyWidth := min(max(24, width-12), 60)
	body := m.detailStatusBlock("Loading priority metadata...", bodyWidth, false)
	return m.renderDetailDialog(width, "Change Priority", selected.Key, body, "esc cancel")
}

func (m Model) renderLabelsLoadingDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	bodyWidth := min(max(24, width-12), 60)
	body := m.detailStatusBlock("Loading labels metadata...", bodyWidth, false)
	return m.renderDetailDialog(width, "Edit Labels", selected.Key, body, "esc cancel")
}

func (m Model) renderComponentsLoadingDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	bodyWidth := min(max(24, width-12), 60)
	body := m.detailStatusBlock("Loading components metadata...", bodyWidth, false)
	return m.renderDetailDialog(width, "Edit Components", selected.Key, body, "esc cancel")
}

func (m Model) renderDetailDialog(width int, title, subtitle, body, footer string) string {
	return m.renderDetailDialogWithLimit(width, title, subtitle, body, footer, 72)
}

func (m Model) renderDetailDialogWithLimit(width int, title, subtitle, body, footer string, maxDialogWidth int) string {
	dialogWidth := min(max(44, width), maxDialogWidth)
	contentWidth := max(24, dialogWidth-4)
	var lines []string
	lines = append(lines, m.theme.PaneTitle.Render(truncate(title, contentWidth)))
	if strings.TrimSpace(subtitle) != "" {
		lines = append(lines, m.theme.Muted.Render(truncate(subtitle, contentWidth)))
	}
	if strings.TrimSpace(body) != "" {
		lines = append(lines, "")
		lines = append(lines, body)
	}
	if strings.TrimSpace(footer) != "" {
		lines = append(lines, "")
		lines = append(lines, m.theme.Muted.Render(truncate(footer, contentWidth)))
	}
	dialog := lipgloss.NewStyle().
		Width(contentWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Muted.GetForeground()).
		Padding(1, 2).
		Render(strings.Join(lines, "\n"))
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, dialog)
}

func (m Model) renderSummaryDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	bodyWidth := min(max(24, width-12), 60)
	lines := []string{
		m.theme.Muted.Render("Summary"),
		m.configuredSummaryEditor(bodyWidth, 3).View(),
	}
	if m.summarySubmitting && m.summarySubmitKey == selected.Key {
		lines = append(lines, "", m.detailStatusBlock("Updating summary...", bodyWidth, false))
	}
	if m.detailNotice != "" {
		lines = append(lines, "", m.renderDetailNotice(m.detailNotice, bodyWidth))
	}
	return m.renderDetailDialog(width, "Edit Summary", selected.Key, strings.Join(lines, "\n"), "enter save  esc cancel")
}

func (m Model) renderPriorityDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	bodyWidth := min(max(24, width-12), 60)
	current := m.theme.Muted.Render("Current: ") + priorityStyle(m.theme, selected.Priority).Render(displayValue(selected.Priority, "Unknown"))
	lines := []string{current}
	if m.prioritySubmitting && m.prioritySubmitKey == selected.Key {
		lines = append(lines, "", m.detailStatusBlock("Updating priority...", bodyWidth, false))
	} else {
		options := m.priorityOptions(selected.Key)
		if len(options) == 0 {
			lines = append(lines, "", m.detailEmptyState("No Jira priority values are available.", bodyWidth))
		} else {
			cursor := clamp(m.selectedPriority, 0, len(options)-1)
			choices := make([]choiceListOption, 0, len(options))
			for _, option := range options {
				choices = append(choices, choiceListOption{Label: displayValue(option.Name, option.ID)})
			}
			lines = append(lines, "")
			lines = append(lines, m.renderChoiceList(choices, cursor, bodyWidth, 5)...)
		}
	}
	if m.detailNotice != "" {
		lines = append(lines, "", m.renderDetailNotice(m.detailNotice, bodyWidth))
	}
	return m.renderDetailDialog(width, "Change Priority", selected.Key, strings.Join(lines, "\n"), "j/k select  enter apply  esc cancel")
}

func (m Model) renderLabelsDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	bodyWidth := min(max(24, width-12), 60)
	lines := []string{
		m.theme.Muted.Render("Current: ") + m.theme.Text.Render(displayValue(strings.Join(m.currentIssueLabels(selected.Key), ", "), "No labels")),
		m.theme.Muted.Render("Labels"),
		m.configuredLabelsEditor(bodyWidth, 3).View(),
	}
	if m.labelsSubmitting && m.labelsSubmitKey == selected.Key {
		lines = append(lines, "", m.detailStatusBlock("Updating labels...", bodyWidth, false))
	}
	if m.detailNotice != "" {
		lines = append(lines, "", m.renderDetailNotice(m.detailNotice, bodyWidth))
	}
	return m.renderDetailDialog(width, "Edit Labels", selected.Key, strings.Join(lines, "\n"), "comma-separated  enter save  esc cancel")
}

func (m Model) renderComponentsDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	bodyWidth := min(max(24, width-12), 64)
	m.configureComponentsFilterEditor(bodyWidth)
	selectedOptions := m.selectedComponentOptions(selected.Key)
	lines := []string{
		m.theme.Muted.Render("Current: ") + m.theme.Text.Render(displayValue(strings.Join(m.currentIssueComponents(selected.Key), ", "), "No components")),
		m.theme.Muted.Render("Selected: ") + m.theme.Text.Render(displayValue(strings.Join(componentNamesFromOptions(selectedOptions), ", "), "No components")),
		m.theme.Muted.Render("Filter"),
		m.componentsFilterEditor.View(),
	}
	if m.componentsSubmitting && m.componentsSubmitKey == selected.Key {
		lines = append(lines, "", m.detailStatusBlock("Updating components...", bodyWidth, false))
	} else {
		options := m.componentOptions(selected.Key)
		matches := m.filteredComponentIndexes()
		if len(options) == 0 {
			lines = append(lines, "", m.detailEmptyState("No Jira component values are available.", bodyWidth))
		} else if len(matches) == 0 {
			lines = append(lines, "", m.detailEmptyState("No Jira components matched.", bodyWidth))
		} else {
			cursor := clamp(m.selectedComponent, 0, len(matches)-1)
			choices := make([]choiceListOption, 0, len(matches))
			for _, optionIndex := range matches {
				option := options[optionIndex]
				label := displayValue(option.Name, option.ID)
				if m.selectedComponents[componentSelectionKey(option)] {
					label = "[x] " + label
				} else {
					label = "[ ] " + label
				}
				choices = append(choices, choiceListOption{Label: label})
			}
			lines = append(lines, "")
			lines = append(lines, m.renderChoiceList(choices, cursor, bodyWidth, createPickerMaxRows)...)
		}
	}
	if m.detailNotice != "" {
		lines = append(lines, "", m.renderDetailNotice(m.detailNotice, bodyWidth))
	}
	return m.renderDetailDialog(width, "Edit Components", selected.Key, strings.Join(lines, "\n"), "type filter  space toggle  enter save  esc cancel")
}

func (m Model) renderAssigneeDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	bodyWidth := min(max(24, width-12), 60)
	current := m.theme.Muted.Render("Current: ") + m.theme.Text.Render(displayValue(selected.Assignee, "Unassigned"))
	lines := []string{
		current,
		m.theme.Muted.Render("Filter: ") + m.theme.Text.Render(displayValue(m.assigneeQuery, "type to search")),
	}
	if m.assigneeSubmitting && m.assigneeSubmitKey == selected.Key {
		lines = append(lines, "", m.detailStatusBlock("Updating assignee...", bodyWidth, false))
	} else if m.assigneeSearchLoading {
		lines = append(lines, "", m.detailStatusBlock("Searching Jira users...", bodyWidth, false))
	} else if m.assigneeSearchErr != nil {
		lines = append(lines, "", m.renderDetailNotice("Assignee search failed: "+m.assigneeSearchErr.Error(), bodyWidth))
	} else if len(m.assigneeUsers) == 0 {
		lines = append(lines, "", m.detailEmptyState("Type a name to search Jira users.", bodyWidth))
	} else {
		cursor := clamp(m.selectedAssignee, 0, len(m.assigneeUsers)-1)
		options := make([]choiceListOption, 0, len(m.assigneeUsers))
		for _, user := range m.assigneeUsers {
			options = append(options, choiceListOption{Label: displayValue(user.DisplayName, user.Email)})
		}
		lines = append(lines, "")
		lines = append(lines, m.renderChoiceList(options, cursor, bodyWidth, 5)...)
	}
	if m.detailNotice != "" {
		lines = append(lines, "", m.renderDetailNotice(m.detailNotice, bodyWidth))
	}
	return m.renderDetailDialog(width, "Change Assignee", selected.Key, strings.Join(lines, "\n"), "type filter  up/down select  enter apply  esc cancel")
}

func (m Model) renderStatusTransitionDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	bodyWidth := min(max(24, width-12), 60)
	current := m.theme.Muted.Render("Current: ") + statusStyle(m.theme, selected.Status).Render(displayValue(selected.Status, "Unknown"))
	lines := []string{current}
	if m.transitionSubmitting && m.transitionSubmitKey == selected.Key {
		lines = append(lines, "", m.detailStatusBlock("Applying transition...", bodyWidth, false))
	} else if m.transitionFieldEditing {
		lines = append(lines, "", m.renderTransitionFieldForm(bodyWidth))
	} else {
		transitions := m.transitions[selected.Key]
		if len(transitions) == 0 {
			lines = append(lines, "", m.detailEmptyState("No available Jira transitions.", bodyWidth))
		} else {
			cursor := clamp(m.selectedTransition, 0, len(transitions)-1)
			rows := make([][]string, 0, len(transitions))
			for index, transition := range transitions {
				marker := " "
				labelStyle := m.theme.Text
				if index == cursor {
					marker = ">"
					labelStyle = m.theme.Selected
				}
				toStatus := displayValue(transition.ToStatus, "Unknown")
				rows = append(rows, []string{
					labelStyle.Render(marker),
					labelStyle.Render(transition.Name),
					statusStyle(m.theme, toStatus).Render(toStatus),
				})
			}
			lines = append(lines, "", m.detailTable(0, []string{"", "TRANSITION", "TO"}, rows, nil))
		}
	}
	if m.detailNotice != "" {
		lines = append(lines, "", m.renderDetailNotice(m.detailNotice, bodyWidth))
	}
	footer := "j/k select  enter apply  esc cancel"
	if m.transitionFieldEditing {
		footer = "tab field  j/k choose  type comment  ctrl+s apply  esc cancel"
	}
	return m.renderDetailDialog(width, "Change Status", selected.Key, strings.Join(lines, "\n"), footer)
}

func (m Model) renderTransitionFieldForm(width int) string {
	transition, ok := m.selectedStatusTransition()
	if !ok {
		return m.detailEmptyState("No transition selected.", width)
	}
	fields := supportedTransitionFields(transition.Fields)
	lines := []string{
		m.detailSectionHeader("transition-fields", "Required Fields", transition.Name, width),
	}
	if len(fields) == 0 {
		lines = append(lines, m.detailEmptyState("No supported transition fields.", width))
		return strings.Join(lines, "\n")
	}
	for index, field := range fields {
		marker := " "
		labelStyle := m.theme.Text
		if index == clamp(m.selectedTransitionField, 0, len(fields)-1) {
			marker = ">"
			labelStyle = m.theme.Selected
		}
		required := ""
		if field.Required {
			required = " required"
		}
		switch {
		case transitionFieldUsesMultiSelect(field):
			value := m.transitionMultiSelectedLabel(field)
			if value == "" {
				value = "none selected"
			}
			lines = append(lines, labelStyle.Render(marker+" "+field.Name+required)+m.theme.Muted.Render("  ")+m.theme.Text.Render(value))
			if index == clamp(m.selectedTransitionField, 0, len(fields)-1) && len(field.AllowedValues) > 0 {
				lines = append(lines, m.transitionFieldChoiceListLines(field, width)...)
			}
		case transitionFieldUsesPicker(field):
			value := "not selected"
			if selected, ok := m.transitionSelectedOption(field); ok {
				value = displayValue(selected.Name, selected.ID)
			}
			lines = append(lines, labelStyle.Render(marker+" "+field.Name+required)+m.theme.Muted.Render("  ")+m.theme.Text.Render(value))
			if index == clamp(m.selectedTransitionField, 0, len(fields)-1) && len(field.AllowedValues) > 0 {
				lines = append(lines, m.transitionFieldChoiceListLines(field, width)...)
			}
			if strings.TrimSpace(field.AutoCompleteURL) != "" {
				filter := strings.TrimSpace(m.transitionFieldFilters[field.ID])
				if filter != "" {
					lines = append(lines, m.theme.Muted.Render("  search "+filter))
				}
				if m.transitionFieldOptionsLoading[field.ID] {
					lines = append(lines, m.detailStatusBlock("Loading Jira options...", width, false))
				} else if err := m.transitionFieldOptionsErr[field.ID]; err != nil {
					lines = append(lines, m.detailStatusBlock("Options failed: "+err.Error(), width, true))
				} else if len(field.AllowedValues) == 0 {
					lines = append(lines, m.detailEmptyState("Type to search Jira options.", width))
				}
			}
		case transitionFieldUsesText(field):
			value := strings.TrimSpace(m.transitionFieldDrafts[field.ID])
			if value == "" {
				value = "empty"
				if field.ID == "comment" {
					value = "no comment"
				}
			}
			lines = append(lines, labelStyle.Render(marker+" "+field.Name+required))
			lines = append(lines, m.theme.Input.Width(width).Height(3).Render(truncate(value, width)))
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) transitionFieldChoiceListLines(field jira.TransitionField, width int) []string {
	options := make([]choiceListOption, 0, len(field.AllowedValues))
	for index, option := range field.AllowedValues {
		label := displayValue(option.Name, option.ID)
		if transitionFieldUsesMultiSelect(field) && m.transitionFieldMultiSelections[field.ID][index] {
			label = "[x] " + label
		} else if transitionFieldUsesMultiSelect(field) {
			label = "[ ] " + label
		}
		options = append(options, choiceListOption{Label: label})
	}
	return m.renderChoiceList(options, m.transitionFieldSelections[field.ID], width, createPickerMaxRows)
}

func supportedTransitionFields(fields []jira.TransitionField) []jira.TransitionField {
	supported := make([]jira.TransitionField, 0, len(fields))
	for _, field := range fields {
		if isSupportedTransitionField(field) {
			supported = append(supported, field)
		}
	}
	return supported
}

func isSupportedTransitionField(field jira.TransitionField) bool {
	if field.ID == "resolution" || field.ID == "comment" {
		return true
	}
	if strings.HasPrefix(strings.TrimSpace(field.ID), "customfield_") && (transitionFieldUsesPicker(field) || transitionFieldUsesMultiSelect(field) || transitionFieldUsesText(field)) {
		return true
	}
	return false
}

func transitionFieldUsesPicker(field jira.TransitionField) bool {
	if field.ID == "resolution" {
		return true
	}
	schemaType := strings.ToLower(strings.TrimSpace(field.SchemaType))
	if transitionFieldUsesMultiSelect(field) {
		return false
	}
	return (len(field.AllowedValues) > 0 || strings.TrimSpace(field.AutoCompleteURL) != "") && (schemaType == "option" || schemaType == "priority" || schemaType == "user" || schemaType == "")
}

func transitionFieldUsesMultiSelect(field jira.TransitionField) bool {
	schemaType := strings.ToLower(strings.TrimSpace(field.SchemaType))
	if schemaType != "array" {
		return false
	}
	schemaItems := strings.ToLower(strings.TrimSpace(field.SchemaItems))
	return len(field.AllowedValues) > 0 && (schemaItems == "option" || schemaItems == "priority" || schemaItems == "user" || schemaItems == "")
}

func transitionFieldUsesText(field jira.TransitionField) bool {
	if field.ID == "comment" {
		return true
	}
	schemaType := strings.ToLower(strings.TrimSpace(field.SchemaType))
	switch schemaType {
	case "string", "text", "textarea", "date", "datetime", "number":
		return true
	default:
		return false
	}
}

func (m Model) fullDetailContent(bodyWidth int) string {
	ctx, ok := m.detailRenderContext()
	if !ok {
		return m.detailEmptyState("No issue selected.", bodyWidth)
	}
	sections := m.detailSections()
	if len(sections) == 0 {
		return m.detailEmptyState("No detail sections available.", bodyWidth)
	}
	section := sections[0]
	if focused, ok := m.focusedDetailSection(); ok {
		section = focused
	}
	var b strings.Builder
	if notice := m.renderTicketNotifications(ctx.display.Key, bodyWidth); notice != "" {
		b.WriteString(notice)
		b.WriteString("\n\n")
	}
	b.WriteString(m.renderDetailSection(section, ctx, bodyWidth))
	b.WriteString("\n\n")
	if m.detailNotice != "" {
		b.WriteString(m.renderDetailNotice(m.detailNotice, bodyWidth))
	}
	return b.String()
}

func (m Model) detailRenderContext() (detailRenderContext, bool) {
	selected, ok := m.selectedIssue()
	if !ok {
		return detailRenderContext{}, false
	}
	detail, hasDetail := m.details[selected.Key]
	display := m.displayIssueForDetail(selected, detail, hasDetail)
	description := ""
	var links []detailLink
	if hasDetail {
		description = detail.Description
		if strings.TrimSpace(description) == "" {
			description = "No description."
		}
		links = detailLinks(detail)
	}
	return detailRenderContext{
		selected:    selected,
		display:     display,
		detail:      detail,
		hasDetail:   hasDetail,
		description: description,
		links:       links,
	}, true
}

func (m Model) renderDetailSection(section detailSection, ctx detailRenderContext, width int) string {
	switch section.ID {
	case "overview":
		return m.renderOverviewSection(ctx, width)
	case "workbench":
		return m.renderDeveloperWorkbenchSection(ctx, width)
	case "description":
		if ctx.hasDetail {
			return m.renderDescription(ctx.description, width)
		}
		if m.detailLoading && m.detailRequestKey == ctx.selected.Key {
			return m.renderDescriptionState("Loading issue detail...", width, false)
		}
		if m.detailErr != nil && m.detailRequestKey == ctx.selected.Key {
			return m.renderDescriptionState("Detail failed: "+m.detailErr.Error(), width, true)
		}
		return m.renderDescriptionState("Description not loaded.", width, false)
	case "links":
		return m.renderLinksSection(ctx.links, width)
	case "hierarchy":
		return m.renderHierarchySection(ctx.display, width)
	case "comments":
		return m.renderComments(ctx.display.Key, width)
	case "worklog":
		return m.renderWorklogs(ctx.display.Key, width)
	case "actions":
		return m.renderActionsSection(width)
	case "status":
		return m.renderStatusSection(ctx.display, width)
	case "claude":
		return m.renderClaudeSection(ctx, width)
	default:
		return m.detailSectionHeader(section.ID, section.Label, "", width) + "\n" + m.detailEmptyState("Section not available.", width)
	}
}

func (m Model) renderDetailIdentity(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return m.theme.Muted.Render("No issue selected")
	}
	detail, hasDetail := m.details[selected.Key]
	display := m.displayIssueForDetail(selected, detail, hasDetail)
	status := statusStyle(m.theme, display.Status).Render(displayValue(display.Status, "Unknown"))
	return m.theme.Key.Render(display.Key) + " " +
		status + " " +
		m.theme.Muted.Render(displayValue(display.IssueType, "Unknown"))
}

func (m Model) renderDetailTitleLine(width int) string {
	return m.renderDetailIdentity(width)
}

func (m Model) renderDetailSummaryLine(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	detail, hasDetail := m.details[selected.Key]
	display := m.displayIssueForDetail(selected, detail, hasDetail)
	value := displayValue(display.Summary, "No summary")
	if strings.TrimSpace(value) == "" {
		value = " "
	}
	style := m.theme.Text.Bold(true)
	if m.summaryFocus || m.focusedDetailTargetID() == "summary" {
		style = m.theme.Selected
	}
	if m.summaryMetadataLoading && m.summaryMetadataRequestKey == display.Key {
		value = "Loading edit metadata..."
	}
	if m.summarySubmitting && m.summarySubmitKey == display.Key {
		value = "Updating summary..."
	}
	return style.Render(truncate(value, max(12, width)))
}

func (m Model) renderDetailHeaderMeta(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	detail, hasDetail := m.details[selected.Key]
	display := m.displayIssueForDetail(selected, detail, hasDetail)
	updated := "Unknown"
	if hasDetail {
		updated = formatTime(detail.Updated)
	}
	parts := []string{
		m.detailMetaPartWithStyle("Assignee", shortName(displayValue(display.Assignee, "Unassigned")), m.focusedDetailTargetID() == "assignee"),
		m.detailMetaPartWithStyle("Priority", displayValue(display.Priority, "Unknown"), m.focusedDetailTargetID() == "priority"),
		m.detailMetaPart("Updated", updated),
	}
	if hasDetail && strings.TrimSpace(detail.Reporter) != "" && detail.Reporter != "Unknown" {
		parts = append(parts, m.detailMetaPart("Reporter", shortName(detail.Reporter)))
	}
	separator := m.theme.Muted.Render("   ")
	for len(parts) > 0 {
		line := strings.Join(parts, separator)
		if lipgloss.Width(line) <= width {
			return line
		}
		parts = parts[:len(parts)-1]
	}
	return ""
}

func (m Model) renderDetailControlStrip(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	detail, hasDetail := m.details[selected.Key]
	display := m.displayIssueForDetail(selected, detail, hasDetail)
	parts := []string{
		m.detailMetaPartWithStyle("Status", displayValue(display.Status, "Unknown")+" v", m.focusedDetailTargetID() == "status"),
		m.detailMetaPartWithStyle("Priority", displayValue(display.Priority, "Unknown")+" v", m.focusedDetailTargetID() == "priority"),
		m.detailMetaPartWithStyle("Assignee", shortName(displayValue(display.Assignee, "Unassigned"))+" v", m.focusedDetailTargetID() == "assignee"),
	}
	if hasDetail {
		parts = append(parts, m.detailMetaPart("Updated", formatTime(detail.Updated)))
	}
	if hasDetail && strings.TrimSpace(detail.Reporter) != "" && detail.Reporter != "Unknown" {
		parts = append(parts, m.detailMetaPart("Reporter", shortName(detail.Reporter)))
	}
	separator := m.theme.Muted.Render("   ")
	for len(parts) > 0 {
		line := strings.Join(parts, separator)
		if lipgloss.Width(line) <= width {
			return line
		}
		parts = parts[:len(parts)-1]
	}
	return ""
}

func (m Model) detailMetaPart(label string, value string) string {
	return m.theme.Muted.Render(label+" ") + m.theme.Text.Render(value)
}

func (m Model) detailMetaPartWithStyle(label string, value string, selected bool) string {
	style := m.theme.Text
	if selected {
		style = m.theme.Selected
	}
	return m.theme.Muted.Render(label+" ") + style.Render(value)
}

func (m Model) displayIssueForDetail(selected jira.Issue, detail jira.IssueDetail, hasDetail bool) jira.Issue {
	if !hasDetail {
		return selected
	}
	display := detail.Issue
	if display.Key == "" {
		display.Key = selected.Key
	}
	if jira.IsPrivacyUserAlias(display.Assignee) && !jira.IsPrivacyUserAlias(selected.Assignee) {
		display.Assignee = selected.Assignee
	}
	return display
}

func (m Model) renderOverviewSection(ctx detailRenderContext, width int) string {
	lines := []string{m.detailSectionHeader("overview", "Overview", "", width)}
	lines = append(lines, "", m.renderOverviewLatest(ctx.display.Key, width))
	lines = append(lines, "", m.renderOverviewDescription(ctx, width))
	lines = append(lines, "", m.renderOverviewHierarchy(ctx.display, width))
	lines = append(lines, "", m.detailEmptyState("Press . for Ticket Actions.", width))
	return strings.Join(lines, "\n")
}

func (m Model) renderDeveloperWorkbenchSection(ctx detailRenderContext, width int) string {
	lines := []string{m.detailSectionHeader("workbench", "Developer Workbench", "next actions", width), ""}
	rows := [][]string{
		{m.theme.FieldLabel.Render("Start Work"), m.theme.Success.Render("ready"), m.theme.Muted.Render("Ticket Actions -> Start Work creates/switches branch after review.")},
		{m.theme.FieldLabel.Render("Claude Plan"), m.claudeWorkbenchState(), m.theme.Muted.Render("Open Claude actions for a read-only implementation plan.")},
		{m.theme.FieldLabel.Render("Quality Review"), m.claudeWorkbenchState(), m.theme.Muted.Render("Ask Claude for readiness gaps before writing Jira.")},
		{m.theme.FieldLabel.Render("Draft Comment"), m.claudeWorkbenchState(), m.theme.Muted.Render("Draft an editable Jira comment before post confirmation.")},
		{m.theme.FieldLabel.Render("Add Comment"), m.theme.Success.Render("ready"), m.theme.Muted.Render("Write or refine a Jira comment, then review before posting.")},
		{m.theme.FieldLabel.Render("Log Work"), m.theme.Success.Render("ready"), m.theme.Muted.Render("Add a Jira worklog entry for this ticket.")},
		{m.theme.FieldLabel.Render("Open Jira"), m.theme.Success.Render("ready"), m.theme.Muted.Render("Open the issue in the browser.")},
		{m.theme.FieldLabel.Render("Copy Key"), m.theme.Success.Render("ready"), m.theme.Muted.Render("Copy the ticket key for branch names, commits, and notes.")},
	}
	lines = append(lines, m.detailTable(0, []string{"ACTION", "STATE", "DETAIL"}, rows, nil))
	lines = append(lines, "", m.detailEmptyState("Use . for Ticket Actions, a for Claude actions, or tab to move through ticket sections.", width))
	return strings.Join(lines, "\n")
}

func (m Model) claudeWorkbenchState() string {
	if m.claudeAvailable() {
		return m.theme.Success.Render("ready")
	}
	return m.theme.Muted.Render("disabled")
}

func (m Model) renderOverviewLatest(key string, width int) string {
	header := m.theme.Muted.Render("Latest")
	if comments, loaded := m.comments[key]; loaded {
		if len(comments) == 0 {
			return header + "\n" + m.detailEmptyState("No comments yet.", width)
		}
		comment := comments[len(comments)-1]
		body := singleLine(comment.Body)
		if body == "" {
			body = "Comment has no text."
		}
		return header + "\n" + m.theme.Text.Render(shortName(comment.Author)+": "+truncate(body, max(24, width-4)))
	}
	if m.commentsLoading && m.commentsRequestKey == key {
		return header + "\n" + m.detailStatusBlock("Loading comments...", width, false)
	}
	if m.commentsErr != nil && m.commentsRequestKey == key {
		return header + "\n" + m.detailStatusBlock("Comments failed: "+m.commentsErr.Error(), width, true)
	}
	return header + "\n" + m.detailEmptyState("Comments not loaded.", width)
}

func (m Model) renderOverviewDescription(ctx detailRenderContext, width int) string {
	header := m.theme.Muted.Render("Description")
	if !ctx.hasDetail && m.detailLoading && m.detailRequestKey == ctx.selected.Key {
		return header + "\n" + m.detailStatusBlock("Loading issue detail...", width, false)
	}
	if !ctx.hasDetail && m.detailErr != nil && m.detailRequestKey == ctx.selected.Key {
		return header + "\n" + m.detailStatusBlock("Detail failed: "+m.detailErr.Error(), width, true)
	}
	description := ctx.description
	description = strings.TrimSpace(description)
	if description == "" {
		return header + "\n" + m.detailEmptyState("No description.", width)
	}
	return header + "\n" + m.renderRichDescriptionBody(wrapRichText(description, max(24, width)), width)
}

func (m Model) renderOverviewHierarchy(issue jira.Issue, width int) string {
	header := m.theme.Muted.Render("Hierarchy")
	children := m.currentHierarchyChildrenFor(issue.Key)
	switch len(children) {
	case 0:
		return header + "\n" + m.renderHierarchyEmptyState(issue, width)
	case 1:
		return header + "\n" + m.theme.Text.Render("1 loaded child")
	default:
		return header + "\n" + m.theme.Text.Render(fmt.Sprintf("%d loaded children", len(children)))
	}
}

func (m Model) detailSections() []detailSection {
	sections := []detailSection{
		{ID: "overview", Label: "Overview", Short: "Over"},
		{ID: "workbench", Label: "Developer Workbench", Short: "Dev"},
		{ID: "comments", Label: "Comments", Short: "Com"},
		{ID: "worklog", Label: "Worklog", Short: "Work"},
		{ID: "hierarchy", Label: "Hierarchy", Short: "Tree"},
	}
	if m.claudeAvailable() {
		sections = append(sections, detailSection{ID: "claude", Label: "Claude", Short: "AI"})
	}
	if selected, ok := m.selectedIssue(); ok {
		display := selected
		description := ""
		var issueLinks []jira.IssueLink
		if detail, hasDetail := m.details[selected.Key]; hasDetail {
			display = detail.Issue
			if display.Key == "" {
				display.Key = selected.Key
			}
			description = detail.Description
			issueLinks = detail.IssueLinks
		}
		if comments, loaded := m.comments[display.Key]; loaded {
			sections[2].Badge = fmt.Sprintf("%d", len(comments))
		} else if m.commentsLoading && m.commentsRequestKey == display.Key {
			sections[2].Badge = "..."
		} else if m.commentsErr != nil && m.commentsRequestKey == display.Key {
			sections[2].Badge = "!"
		}
		if worklogs, loaded := m.worklogs[display.Key]; loaded {
			sections[3].Badge = fmt.Sprintf("%d", len(worklogs))
		} else if m.worklogsLoading && m.worklogsRequestKey == display.Key {
			sections[3].Badge = "..."
		} else if m.worklogsErr != nil && m.worklogsRequestKey == display.Key {
			sections[3].Badge = "!"
		}
		if childCount := len(m.hierarchyRows(display.Key)); childCount > 0 {
			sections[4].Badge = fmt.Sprintf("%d", childCount)
		}
		if linkCount := len(detailLinks(jira.IssueDetail{Description: description, IssueLinks: issueLinks})); linkCount > 0 {
			links := detailSection{ID: "links", Label: "Links", Short: "Links", Badge: fmt.Sprintf("%d", linkCount)}
			sections = append(sections[:5], append([]detailSection{links}, sections[5:]...)...)
		}
	}
	return sections
}

func (m Model) detailTargets() []detailTarget {
	sections := m.detailSections()
	targets := make([]detailTarget, 0, len(sections)+3)
	for _, section := range sections {
		if section.ID == "overview" {
			targets = append(targets, detailTarget{
				ID:      section.ID,
				Label:   section.Label,
				Kind:    detailTargetSection,
				Section: section,
			})
			break
		}
	}
	targets = append(targets,
		detailTarget{ID: "summary", Label: "Summary", Kind: detailTargetField},
		detailTarget{ID: "status", Label: "Status", Kind: detailTargetField},
		detailTarget{ID: "priority", Label: "Priority", Kind: detailTargetField},
		detailTarget{ID: "assignee", Label: "Assignee", Kind: detailTargetField},
	)
	for _, section := range sections {
		if section.ID == "overview" {
			continue
		}
		targets = append(targets, detailTarget{
			ID:      section.ID,
			Label:   section.Label,
			Kind:    detailTargetSection,
			Section: section,
		})
	}
	return targets
}

func (m Model) detailTabs() []string {
	sections := m.detailSections()
	tabs := make([]string, 0, len(sections))
	for _, section := range sections {
		tabs = append(tabs, section.Label)
	}
	return tabs
}

func (m Model) detailSectionTitle(id string, fallback string, help string) string {
	return m.detailSectionHeader(id, fallback, help, 0)
}

func (m Model) detailSectionHeader(id string, fallback string, help string, width int) string {
	title := fallback
	badge := ""
	if section, ok := m.detailSection(id); ok {
		title = section.Label
		badge = section.Badge
	}
	leftStyle := m.theme.PaneTitle
	left := leftStyle.Render(title)
	if badge != "" {
		left += m.theme.Muted.Render(" " + badge)
	}
	right := ""
	if help != "" {
		right = m.theme.Muted.Render(help)
	}
	if width <= 0 {
		if right == "" {
			return left
		}
		return left + " " + right
	}
	ruleWidth := width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if ruleWidth < 3 {
		if right == "" {
			return left
		}
		return left + " " + right
	}
	rule := m.theme.Muted.Render(strings.Repeat("─", ruleWidth))
	if right == "" {
		return left + " " + rule
	}
	return left + " " + rule + " " + right
}

func (m Model) detailSection(id string) (detailSection, bool) {
	for _, section := range m.detailSections() {
		if section.ID == id {
			return section, true
		}
	}
	return detailSection{}, false
}

func (m Model) renderDetailTabs(width int) string {
	sections := m.detailSections()
	tabs := m.detailTabsLine(sections, width, false)
	line := tabs
	if lipgloss.Width(line) <= width {
		return line
	}
	tabs = m.detailTabsLine(sections, width, true)
	line = tabs
	if lipgloss.Width(line) <= width {
		return line
	}
	return m.detailTabsWrapped(sections, max(8, width))
}

func (m Model) detailTabsLine(sections []detailSection, width int, compact bool) string {
	parts := make([]string, 0, len(sections))
	focusedSectionID := ""
	if section, ok := m.focusedDetailSection(); ok {
		focusedSectionID = section.ID
	}
	for _, section := range sections {
		parts = append(parts, m.renderDetailTab(section, section.ID == focusedSectionID, compact))
	}
	return strings.Join(parts, m.theme.Muted.Render("   "))
}

func (m Model) detailTabsWrapped(sections []detailSection, width int) string {
	if len(sections) == 0 {
		return ""
	}
	separator := m.theme.Muted.Render("   ")
	var rows []string
	var current string
	focusedSectionID := ""
	if section, ok := m.focusedDetailSection(); ok {
		focusedSectionID = section.ID
	}
	for _, section := range sections {
		part := m.renderDetailTab(section, section.ID == focusedSectionID, true)
		candidate := part
		if current != "" {
			candidate = current + separator + part
		}
		if current != "" && lipgloss.Width(candidate) > width {
			rows = append(rows, current)
			current = part
			continue
		}
		current = candidate
	}
	if current != "" {
		rows = append(rows, current)
	}
	return strings.Join(rows, "\n")
}

func (m Model) renderDetailTab(section detailSection, active bool, compact bool) string {
	label := section.Label
	if compact {
		label = section.Short
	}
	if section.Badge != "" {
		label += " " + section.Badge
	}
	if active {
		return m.theme.Selected.Render("> " + label)
	}
	return m.theme.Muted.Render(label)
}

func (m *Model) moveDetailFocus(delta int) {
	targets := m.detailTargets()
	if len(targets) == 0 {
		m.detailFocus = 0
		return
	}
	m.saveDetailSectionOffset()
	m.detailFocus = (m.detailFocus + delta + len(targets)) % len(targets)
	m.linkFocus = false
	m.hierarchyFocus = false
	m.actionFocus = false
	m.transitionFocus = false
	m.priorityFocus = false
	m.assigneeFocus = false
	m.summaryFocus = false
	m.worklogListFocus = false
	m.restoreDetailSectionOffset()
}

func (m *Model) moveDetailSectionFocus(delta int) {
	targets := m.detailTargets()
	if len(targets) == 0 {
		m.detailFocus = 0
		return
	}
	m.saveDetailSectionOffset()
	start := clamp(m.detailFocus, 0, len(targets)-1)
	for step := 1; step <= len(targets); step++ {
		index := (start + delta*step + len(targets)*step) % len(targets)
		if targets[index].Kind == detailTargetSection {
			m.detailFocus = index
			break
		}
	}
	m.linkFocus = false
	m.hierarchyFocus = false
	m.actionFocus = false
	m.transitionFocus = false
	m.priorityFocus = false
	m.assigneeFocus = false
	m.summaryFocus = false
	m.worklogListFocus = false
	m.restoreDetailSectionOffset()
}

func (m Model) activateFocusedDetailTarget() (Model, tea.Cmd) {
	target, ok := m.focusedDetailTarget()
	if !ok {
		return m, nil
	}
	if target.Kind == detailTargetField {
		switch target.ID {
		case "summary":
			return m.startSummaryEditor()
		case "status":
			return m.startStatusTransitionPicker()
		case "assignee":
			return m.startAssigneePicker()
		case "priority":
			return m.startPriorityEditor()
		}
		return m, nil
	}
	section := target.Section
	switch section.ID {
	case "actions":
		m.focusActions()
	case "status":
		return m.startStatusTransitionPicker()
	case "claude":
		return m.runSelectedClaudeAction()
	case "hierarchy":
		m.focusHierarchy()
	case "links":
		m.focusDetailLinks()
	case "comments":
		return m.activateCommentsSection()
	case "worklog":
		m.focusWorklogs()
	default:
		m.linkFocus = false
		m.hierarchyFocus = false
		m.actionFocus = false
		m.transitionFocus = false
		m.priorityFocus = false
		m.assigneeFocus = false
		m.worklogListFocus = false
		m.jumpDetailSection(section.Label)
	}
	return m, nil
}

func (m Model) activateCommentsSection() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok {
		m.detailNotice = "No issue selected."
		return m, nil
	}
	comments := m.comments[selected.Key]
	if len(comments) == 0 {
		m.startCommentComposer()
		return m, nil
	}
	m.linkFocus = false
	m.hierarchyFocus = false
	m.actionFocus = false
	m.transitionFocus = false
	m.priorityFocus = false
	m.assigneeFocus = false
	m.commentFocus = true
	m.selectedComment = clamp(m.selectedComment, 0, len(comments)-1)
	m.jumpDetailSection("Comments")
	m.detailNotice = ""
	return m, nil
}

func (m *Model) moveSelectedComment(delta int) {
	selected, ok := m.selectedIssue()
	if !ok {
		m.selectedComment = 0
		return
	}
	comments := m.comments[selected.Key]
	if len(comments) == 0 {
		m.selectedComment = 0
		return
	}
	m.selectedComment = clamp(m.selectedComment+delta, 0, len(comments)-1)
}

func (m Model) selectedCommentForEdit() (jira.Comment, bool) {
	selected, ok := m.selectedIssue()
	if !ok {
		return jira.Comment{}, false
	}
	comments := m.comments[selected.Key]
	if len(comments) == 0 {
		return jira.Comment{}, false
	}
	return comments[clamp(m.selectedComment, 0, len(comments)-1)], true
}

func (m *Model) activateDetailFocus() {
	updated, _ := m.activateFocusedDetailTarget()
	*m = updated
}

func (m Model) renderIssueTitle(issue jira.Issue, width int) string {
	meta := m.theme.Key.Render(issue.Key) + "  " + statusStyle(m.theme, issue.Status).Render(displayValue(issue.Status, "Unknown")) + "  " + m.theme.Muted.Render(displayValue(issue.IssueType, "Unknown"))
	summary := wrapText(issue.Summary, width)
	if strings.TrimSpace(summary) == "" {
		summary = "No summary"
	}
	return meta + "\n" + m.theme.Text.Bold(true).Render(wrapText(summary, width))
}

func (m Model) renderDescription(description string, width int) string {
	width = max(24, width)
	return m.detailSectionHeader("description", "Description", "", width) + "\n\n" + m.renderRichDescriptionBody(wrapRichText(description, width), width)
}

func (m Model) renderDescriptionState(message string, width int, isError bool) string {
	return m.detailSectionHeader("description", "Description", "", width) + "\n\n" + m.detailStatusBlock(message, width, isError)
}

func (m Model) renderComments(key string, width int) string {
	help := ""
	if m.commentFocus {
		help = "j/k select  e edit  enter add  esc leave"
	} else if comments, ok := m.comments[key]; ok && len(comments) > 0 {
		help = "enter focus"
	}
	lines := []string{m.detailSectionHeader("comments", "Comments", help, width)}
	if comments, ok := m.comments[key]; ok {
		if len(comments) == 0 {
			lines = append(lines, "")
			lines = append(lines, m.detailStatusBlock("No comments yet.", width, false))
			return strings.Join(lines, "\n")
		}
		for index, comment := range comments {
			if index > 0 {
				lines = append(lines, "")
			}
			body := comment.Body
			if strings.TrimSpace(body) == "" {
				body = "No comment body."
			}
			lines = append(lines, m.renderCommentBlock(index+1, len(comments), comment.Author, formatTime(comment.Created), body, width, m.commentFocus && index == clamp(m.selectedComment, 0, len(comments)-1)))
		}
		if len(comments) >= maxComments {
			lines = append(lines, "")
			lines = append(lines, m.detailEmptyState(fmt.Sprintf("Showing latest %d comments.", maxComments), width))
		}
		return strings.Join(lines, "\n")
	}
	if m.commentsLoading && m.commentsRequestKey == key {
		lines = append(lines, "")
		lines = append(lines, m.detailStatusBlock("Loading comments...", width, false))
		return strings.Join(lines, "\n")
	}
	if m.commentsErr != nil && m.commentsRequestKey == key {
		lines = append(lines, "")
		lines = append(lines, m.detailStatusBlock("Comments failed: "+m.commentsErr.Error(), width, true))
		return strings.Join(lines, "\n")
	}
	lines = append(lines, "")
	lines = append(lines, m.detailStatusBlock("Comments not loaded.", width, false))
	return strings.Join(lines, "\n")
}

func (m Model) renderCommentBlock(index int, total int, author string, created string, body string, width int, selected bool) string {
	contentWidth := max(20, width-4)
	marker := "  "
	if selected {
		marker = "> "
	}
	header := m.theme.Key.Render(displayValue(author, "Unknown")) +
		m.theme.Muted.Render("  "+created+"  "+fmt.Sprintf("Comment %d/%d", index, max(1, total)))
	bodyWidth := max(12, contentWidth-2)
	renderedBody := m.renderRichDescriptionBody(wrapRichText(body, bodyWidth), bodyWidth)
	renderedBody = indentLines(renderedBody, "  ")
	style := m.theme.CommentBlock
	if selected {
		style = m.theme.ActivePane
	}
	return style.Width(contentWidth + 2).Render(marker + header + "\n\n" + renderedBody)
}

func (m Model) detailEmptyState(message string, width int) string {
	return m.theme.Muted.Render("  " + truncate(message, max(12, width-2)))
}

func (m Model) detailStatusBlock(message string, width int, isError bool) string {
	header := m.detailSectionHeader("detail-status", "Status", "", width)
	body := m.theme.Text.Render(wrapText(message, max(12, width)))
	if isError {
		body = m.theme.Error.Render(wrapText(message, max(12, width)))
	}
	return header + "\n" + body
}

func (m Model) detailTable(width int, headers []string, rows [][]string, style func(row, col int) lipgloss.Style) string {
	table := lipglosstable.New().
		Border(lipgloss.HiddenBorder()).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == lipglosstable.HeaderRow {
				return m.theme.Muted
			}
			if style != nil {
				return style(row, col)
			}
			return m.theme.Text
		})
	if width > 0 {
		table = table.Width(width)
	}
	if len(headers) > 0 {
		table = table.Headers(headers...)
	}
	return table.String()
}

func (m Model) renderDetailNotice(message string, width int) string {
	contentWidth := max(20, width-4)
	return m.theme.NoticeBlock.Width(contentWidth + 2).Render(m.theme.Muted.Render("Notice") + "\n" + m.theme.Text.Render(wrapText(message, contentWidth)))
}

func (m Model) renderHierarchySection(issue jira.Issue, width int) string {
	rows := m.hierarchyRows(issue.Key)
	children, subtasks := splitHierarchyRows(rows)
	var lines []string
	help := ""
	switch {
	case m.hierarchyFocus:
		help = "j/k select  enter open  esc leave"
	case len(rows) > 0:
		help = "enter focus"
	}
	lines = append(lines, m.detailSectionHeader("hierarchy", "Hierarchy", help, width))
	pathRel := "Current"
	pathIssue := issue.Key
	if issue.ParentKey != "" {
		pathRel = "Parent"
		pathIssue = issue.ParentKey
		if issue.ParentSummary != "" {
			pathIssue += "  " + issue.ParentSummary
		}
	} else if issue.Summary != "" {
		pathIssue += "  " + issue.Summary
	}
	lines = append(lines, m.renderHierarchyPath(pathRel, pathIssue, width))
	if len(rows) == 0 {
		lines = append(lines, m.renderHierarchyEmptyState(issue, width))
		return strings.Join(lines, "\n")
	}
	if len(children) > 0 {
		lines = append(lines, "")
		lines = append(lines, m.renderHierarchyGroup("Children", children, width))
	}
	if len(subtasks) > 0 {
		lines = append(lines, "")
		lines = append(lines, m.renderHierarchyGroup("Subtasks", subtasks, width))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderHierarchyEmptyState(issue jira.Issue, width int) string {
	if issue.ParentKey != "" {
		return m.detailEmptyState("No child or subtask issues loaded in the current view.", width)
	}
	return m.detailEmptyState("No parent, child, or subtask issues loaded in the current view.", width)
}

func (m Model) renderHierarchyGroup(label string, groupRows []hierarchyRow, width int) string {
	lines := []string{m.theme.Muted.Render(fmt.Sprintf("%s %d", label, len(groupRows)))}
	tableRows := make([][]string, 0, len(groupRows))
	cursor := clamp(m.selectedHierarchy, 0, max(0, len(m.currentHierarchyChildren())-1))
	for _, row := range groupRows {
		child := row.Issue
		key := child.Key
		selected := row.Index == cursor
		if selected {
			key = "> " + key
		} else {
			key = "  " + key
		}
		tableRows = append(tableRows, []string{
			key,
			truncate(child.Summary, max(12, width-52)),
			displayValue(child.Status, "Unknown"),
			truncate(priorityBadge(child.Priority), 4),
			truncate(shortName(displayValue(child.Assignee, "Unassigned")), 14),
		})
	}
	lines = append(lines, m.detailTable(0, []string{"KEY", "SUMMARY", "STATUS", "PRI", "OWNER"}, tableRows, func(row, col int) lipgloss.Style {
		if col == 0 {
			if m.hierarchyFocus && row >= 0 && row < len(groupRows) && groupRows[row].Index == cursor {
				return m.theme.Selected
			}
			return m.theme.Key
		}
		return m.theme.Text
	}))
	return strings.Join(lines, "\n")
}

func (m Model) renderHierarchyPath(rel string, issue string, width int) string {
	label := m.theme.Muted.Render("Path")
	relation := m.theme.Muted.Render(rel + ": ")
	return label + "\n" + relation + m.theme.Text.Render(truncate(issue, max(12, width-lipgloss.Width(rel)-8)))
}

func (m Model) renderActionsSection(width int) string {
	actions := m.detailActions()
	lines := make([]string, 0, len(actions)+2)
	help := ""
	if m.actionFocus {
		help = "j/k select  enter run  esc leave"
	}
	lines = append(lines, m.detailSectionHeader("actions", "Actions", help, width))
	cursor := clamp(m.selectedAction, 0, max(0, len(actions)-1))
	rows := make([][]string, 0, len(actions))
	for index, action := range actions {
		marker := " "
		labelStyle := m.theme.Text
		groupStyle := m.theme.FieldLabel
		descStyle := m.theme.Muted
		group := detailActionGroup(action.ID)
		if m.actionFocus && index == cursor {
			marker = ">"
			labelStyle = m.theme.Selected
			groupStyle = m.theme.Selected
		} else if !action.Enabled {
			labelStyle = m.theme.Muted
			groupStyle = m.theme.Muted
			descStyle = m.theme.Muted
			group = displayValue(action.DisabledState, "Needs Metadata")
		}
		rows = append(rows, []string{
			labelStyle.Render(marker),
			groupStyle.Render(group),
			labelStyle.Render(action.Label),
			descStyle.Render(truncate(action.Description, max(16, width-48))),
		})
	}
	lines = append(lines, m.detailTable(0, []string{"", "GROUP", "ACTION", "DETAIL"}, rows, nil))
	return strings.Join(lines, "\n")
}

func (m Model) renderStatusSection(issue jira.Issue, width int) string {
	help := "enter load transitions"
	if m.transitionFocus {
		help = "dialog open"
	} else if m.transitionLoading && m.transitionRequestKey == issue.Key {
		help = "loading"
	} else if m.transitionSubmitting && m.transitionSubmitKey == issue.Key {
		help = "applying"
	}
	lines := []string{m.detailSectionHeader("status", "Status", help, width)}
	current := statusStyle(m.theme, issue.Status).Render(displayValue(issue.Status, "Unknown"))
	lines = append(lines, "")
	lines = append(lines, m.theme.Muted.Render("Current: ")+current)
	if m.transitionLoading && m.transitionRequestKey == issue.Key {
		lines = append(lines, "")
		lines = append(lines, m.detailStatusBlock("Loading available transitions...", width, false))
		return strings.Join(lines, "\n")
	}
	if m.transitionErr != nil && m.transitionRequestKey == issue.Key {
		lines = append(lines, "")
		lines = append(lines, m.detailStatusBlock("Transitions failed: "+m.transitionErr.Error(), width, true))
	}
	if m.transitionFocus || (m.transitionSubmitting && m.transitionSubmitKey == issue.Key) {
		lines = append(lines, "")
		lines = append(lines, m.detailEmptyState("Status change dialog open.", width))
		return strings.Join(lines, "\n")
	}
	transitions := m.transitions[issue.Key]
	if len(transitions) == 0 {
		lines = append(lines, "")
		lines = append(lines, m.detailEmptyState("Press enter to load available Jira transitions.", width))
		return strings.Join(lines, "\n")
	}
	cursor := clamp(m.selectedTransition, 0, len(transitions)-1)
	rows := make([][]string, 0, len(transitions))
	for index, transition := range transitions {
		marker := " "
		labelStyle := m.theme.Text
		if m.transitionFocus && index == cursor {
			marker = ">"
			labelStyle = m.theme.Selected
		}
		toStatus := displayValue(transition.ToStatus, "Unknown")
		rows = append(rows, []string{
			labelStyle.Render(marker),
			labelStyle.Render(transition.Name),
			statusStyle(m.theme, toStatus).Render(toStatus),
		})
	}
	lines = append(lines, "")
	lines = append(lines, m.detailTable(0, []string{"", "TRANSITION", "TO"}, rows, nil))
	return strings.Join(lines, "\n")
}

func (m Model) detailActions() []detailAction {
	actions := []detailAction{
		{ID: "start-work", Label: "Start Work", Description: "Choose repo, create branch, and apply confirmed Jira updates.", Enabled: true},
		{ID: "comment", Label: "Add Comment", Description: "Write or refine a Jira comment before review and post.", Enabled: true},
		{ID: "log-work", Label: "Log Work", Description: "Add a Jira worklog entry.", Enabled: true},
		{ID: "browser", Label: "Open In Browser", Description: "Open this ticket in Jira.", Enabled: true},
		{ID: "copy-key", Label: "Copy Key", Description: "Copy the ticket key.", Enabled: true},
		{ID: "copy-url", Label: "Copy URL", Description: "Copy the Jira URL.", Enabled: true},
		{ID: "transition", Label: "Transition Status", Description: "Load available Jira transitions and change status.", Enabled: true},
		{ID: "assign", Label: "Assign", Description: "Search assignable Jira users and change assignee.", Enabled: true},
		{ID: "summary", Label: "Edit Summary", Description: "Load Jira edit metadata and update summary.", Enabled: true},
		{ID: "priority", Label: "Change Priority", Description: "Load Jira priority options and update priority.", Enabled: true},
		{ID: "labels", Label: "Edit Labels", Description: "Edit comma-separated Jira labels.", Enabled: true},
		{ID: "components", Label: "Edit Components", Description: "Select Jira components from edit metadata.", Enabled: true},
		{ID: "sprint", Label: "Sprint", Description: "Add this ticket to an active or future Jira sprint.", Enabled: true},
		{ID: "link-issue", Label: "Link Issue", Description: "Create a Jira issue link to another ticket.", Enabled: true},
		{ID: "subtask", Label: "Create Subtask", Description: "Use Jira create metadata for required fields.", Enabled: true},
	}
	actions = append(actions, m.genericEditFieldActions()...)
	return actions
}

func detailActionGroup(id string) string {
	if strings.HasPrefix(id, "field:") {
		return "Jira Field"
	}
	switch id {
	case "start-work", "comment", "log-work":
		return "Developer"
	case "browser", "copy-key", "copy-url":
		return "Open/Copy"
	case "transition", "assign", "sprint", "link-issue":
		return "Jira"
	case "summary", "priority", "labels", "components":
		return "Jira Field"
	case "subtask":
		return "Create"
	default:
		return "Action"
	}
}

func (m Model) unsupportedEditFieldActions() []detailAction {
	selected, ok := m.selectedIssue()
	if !ok || strings.TrimSpace(selected.Key) == "" {
		return nil
	}
	metadata, ok := m.editMetadata[selected.Key]
	if !ok {
		return nil
	}
	supported := supportedEditFieldIDs()
	actions := make([]detailAction, 0, len(metadata.Fields))
	for _, field := range metadata.Fields {
		if !field.Editable {
			continue
		}
		fieldID := strings.TrimSpace(field.ID)
		if fieldID == "" || supported[fieldID] {
			continue
		}
		name := strings.TrimSpace(displayValue(field.Name, fieldID))
		actions = append(actions, detailAction{
			ID:             "field:" + fieldID,
			Label:          "Edit " + name,
			Description:    unsupportedEditFieldDescription(field),
			Enabled:        false,
			DisabledState:  "Unsupported",
			DisabledReason: name + " is editable in Jira but not supported yet.",
		})
	}
	return actions
}

func unsupportedEditFieldDescription(field jira.EditField) string {
	parts := []string{editFieldOptionSourceLabel(field)}
	if schema := editFieldSchemaLabel(field); schema != "" {
		parts = append(parts, "schema: "+schema)
	}
	if len(field.Operations) > 0 {
		parts = append(parts, "ops: "+strings.Join(field.Operations, "/"))
	}
	parts = append(parts, "workflow not implemented")
	return strings.Join(parts, "; ")
}

func editFieldOptionSourceLabel(field jira.EditField) string {
	if len(field.AllowedValues) > 0 {
		return fmt.Sprintf("options: %d values", len(field.AllowedValues))
	}
	if strings.TrimSpace(field.AutoCompleteURL) != "" {
		return "options: autocomplete"
	}
	return "options: none"
}

func editFieldSchemaLabel(field jira.EditField) string {
	schemaType := strings.TrimSpace(field.SchemaType)
	items := strings.TrimSpace(field.SchemaItems)
	system := strings.TrimSpace(field.SchemaSystem)
	custom := strings.TrimSpace(field.SchemaCustom)
	switch {
	case schemaType != "" && items != "":
		return schemaType + "<" + items + ">"
	case schemaType != "":
		return schemaType
	case system != "":
		return system
	case custom != "":
		return custom
	default:
		return ""
	}
}

func supportedEditFieldIDs() map[string]bool {
	return map[string]bool{
		"summary":    true,
		"priority":   true,
		"labels":     true,
		"components": true,
	}
}

func (m *Model) focusActions() {
	actions := m.detailActions()
	m.linkFocus = false
	m.hierarchyFocus = false
	m.actionFocus = true
	m.selectedAction = clamp(m.selectedAction, 0, max(0, len(actions)-1))
	m.jumpDetailSection("Actions")
}

func (m *Model) moveSelectedDetailAction(delta int) {
	actions := m.detailActions()
	if len(actions) == 0 {
		m.selectedAction = 0
		return
	}
	m.selectedAction = clamp(m.selectedAction+delta, 0, len(actions)-1)
}

func (m Model) runSelectedDetailAction() (Model, tea.Cmd) {
	actions := m.detailActions()
	if len(actions) == 0 {
		return m, nil
	}
	action := actions[clamp(m.selectedAction, 0, len(actions)-1)]
	if !action.Enabled {
		m.detailNotice = displayValue(action.DisabledReason, action.Label+" needs Jira metadata before it can be edited safely.")
		return m, nil
	}
	switch action.ID {
	case "start-work":
		return m.startSelectedIssueWorkflow()
	case "comment":
		m.startCommentComposer()
		return m, nil
	case "browser":
		return m.openSelectedIssue()
	case "copy-key":
		return m.copySelectedIssueKey()
	case "copy-url":
		return m.copySelectedIssueURL()
	case "summary":
		return m.startSummaryEditor()
	case "priority":
		return m.startPriorityEditor()
	case "labels":
		return m.startLabelsEditor()
	case "components":
		return m.startComponentsEditor()
	case "sprint":
		return m.startSprintPicker()
	case "transition":
		return m.startStatusTransitionPicker()
	case "assign":
		return m.startAssigneePicker()
	case "link-issue":
		return m.startIssueLinkEditor()
	case "log-work":
		return m.startWorklogEditor()
	case "subtask":
		return m.startCreateSubtask()
	default:
		if fieldID, ok := strings.CutPrefix(action.ID, "field:"); ok {
			switch fieldID {
			case "parent":
				return m.startParentEditor()
			case "timetracking":
				return m.startTimeTrackingEditor()
			}
			return m.startGenericFieldEditor(fieldID)
		}
		return m, nil
	}
}

func (m Model) startCreateSubtask() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok || strings.TrimSpace(selected.Key) == "" {
		m.detailNotice = "Select an issue before creating a subtask."
		return m, nil
	}
	return m.startCreateIssueWithParent(selected.Key, selected.Summary)
}

func (m *Model) focusStatusTransitions() {
	m.linkFocus = false
	m.hierarchyFocus = false
	m.actionFocus = false
	m.transitionFocus = true
	m.jumpDetailSection("Status")
}

func (m Model) startStatusTransitionPicker() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok {
		return m, nil
	}
	m.linkFocus = false
	m.hierarchyFocus = false
	m.actionFocus = false
	m.jumpDetailSection("Status")
	m.hydrateIssueTransitions(selected.Key)
	if transitions := m.transitions[selected.Key]; len(transitions) > 0 {
		m.transitionFocus = true
		m.selectedTransition = clamp(m.selectedTransition, 0, len(transitions)-1)
		if _, ok := m.cachedIssueTransitions(selected.Key); !ok || m.isIssueTransitionsFresh(selected.Key) {
			m.detailNotice = ""
			return m, nil
		}
	}
	if m.transitionLoading && m.transitionRequestKey == selected.Key {
		return m, nil
	}
	m.nextRequestID++
	m.activeTransitionsReqID = m.nextRequestID
	m.transitionRequestKey = selected.Key
	m.transitionLoading = true
	m.transitionErr = nil
	m.transitionFocus = false
	m.detailNotice = "Loading status transitions for " + selected.Key + "."
	return m, m.submitIssueTransitions(m.activeTransitionsReqID, selected.Key)
}

func (m *Model) moveSelectedTransition(delta int) {
	selected, ok := m.selectedIssue()
	if !ok {
		m.selectedTransition = 0
		return
	}
	transitions := m.transitions[selected.Key]
	if len(transitions) == 0 {
		m.selectedTransition = 0
		return
	}
	m.selectedTransition = clamp(m.selectedTransition+delta, 0, len(transitions)-1)
}

func (m Model) submitSelectedTransition() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok {
		return m, nil
	}
	transitions := m.transitions[selected.Key]
	if len(transitions) == 0 {
		return m.startStatusTransitionPicker()
	}
	transition := transitions[clamp(m.selectedTransition, 0, len(transitions)-1)]
	if transition.ID == "" {
		m.detailNotice = "Status update failed: missing transition ID."
		return m, nil
	}
	if unsupported := unsupportedRequiredTransitionFields(transition.Fields); len(unsupported) > 0 {
		m.detailNotice = "Status update needs unsupported transition fields: " + strings.Join(unsupported, ", ") + "."
		return m, nil
	}
	if transitionNeedsFieldForm(transition) {
		m.beginTransitionFieldEditing(transition)
		return m, nil
	}
	return m.submitTransitionWithFields(selected.Key, transition, nil)
}

func (m Model) submitTransitionWithFields(key string, transition jira.Transition, fields []jira.TransitionFieldValue) (Model, tea.Cmd) {
	m.nextRequestID++
	m.activeTransitionReqID = m.nextRequestID
	m.transitionSubmitting = true
	m.transitionSubmitKey = key
	m.transitionSubmitToStatus = transition.ToStatus
	m.transitionSubmitFields = append([]jira.TransitionFieldValue(nil), fields...)
	m.transitionFieldEditing = false
	m.detailNotice = "Updating status to " + displayValue(transition.ToStatus, transition.Name) + "."
	return m, m.submitIssueTransition(m.activeTransitionReqID, key, transition, fields)
}

func (m Model) selectedStatusTransition() (jira.Transition, bool) {
	selected, ok := m.selectedIssue()
	if !ok {
		return jira.Transition{}, false
	}
	transitions := m.transitions[selected.Key]
	if len(transitions) == 0 {
		return jira.Transition{}, false
	}
	return transitions[clamp(m.selectedTransition, 0, len(transitions)-1)], true
}

func unsupportedRequiredTransitionFields(fields []jira.TransitionField) []string {
	var unsupported []string
	for _, field := range fields {
		if field.Required && !isSupportedTransitionField(field) {
			unsupported = append(unsupported, displayValue(field.Name, field.ID))
		}
	}
	return unsupported
}

func transitionNeedsFieldForm(transition jira.Transition) bool {
	for _, field := range transition.Fields {
		if field.Required && isSupportedTransitionField(field) {
			return true
		}
	}
	return false
}

func (m *Model) beginTransitionFieldEditing(transition jira.Transition) {
	m.transitionFieldEditing = true
	m.transitionFocus = true
	m.selectedTransitionField = 0
	m.transitionFieldSelections = make(map[string]int)
	m.transitionFieldMultiSelections = make(map[string]map[int]bool)
	m.transitionFieldDrafts = make(map[string]string)
	m.transitionFieldFilters = make(map[string]string)
	m.transitionFieldOptionsLoading = make(map[string]bool)
	m.transitionFieldOptionsErr = make(map[string]error)
	m.transitionFieldOptionsQuery = make(map[string]string)
	for _, field := range supportedTransitionFields(transition.Fields) {
		if transitionFieldUsesMultiSelect(field) {
			m.transitionFieldSelections[field.ID] = -1
			m.transitionFieldMultiSelections[field.ID] = make(map[int]bool)
		} else if transitionFieldUsesPicker(field) {
			m.transitionFieldSelections[field.ID] = -1
		}
	}
	m.transitionFieldCommentEditor = newCommentEditor("")
	m.transitionFieldCommentEditorReady = true
	m.transitionFieldEditorFieldID = ""
	m.detailNotice = "Complete required transition fields before applying status."
}

func (m Model) handleTransitionFieldOptionsResult(result worker.Result) Model {
	if result.ID != m.activeTransitionFieldOptionsReqID {
		return m
	}
	m.ensureTransitionFieldOptionsState()
	if result.SearchFieldOptions == nil {
		return m.finishTransitionFieldOptionsResult("", workerResultError(result))
	}
	fieldID := strings.TrimSpace(result.SearchFieldOptions.FieldID)
	if fieldID == "" {
		return m
	}
	if result.SearchFieldOptions.Query != m.transitionFieldOptionsQuery[fieldID] {
		return m
	}
	if result.Err != nil {
		return m.finishTransitionFieldOptionsResult(fieldID, result.Err)
	}
	m.transitionFieldOptionsLoading[fieldID] = false
	m.transitionFieldOptionsErr[fieldID] = nil
	m.replaceTransitionFieldOptions(fieldID, result.SearchFieldOptions.Options)
	if len(result.SearchFieldOptions.Options) > 0 {
		m.transitionFieldSelections[fieldID] = 0
	} else {
		m.transitionFieldSelections[fieldID] = -1
	}
	return m
}

func (m Model) finishTransitionFieldOptionsResult(fieldID string, err error) Model {
	if fieldID != "" {
		m.ensureTransitionFieldOptionsState()
		m.transitionFieldOptionsLoading[fieldID] = false
		m.transitionFieldOptionsErr[fieldID] = err
	}
	return m
}

func (m *Model) ensureTransitionFieldOptionsState() {
	if m.transitionFieldFilters == nil {
		m.transitionFieldFilters = make(map[string]string)
	}
	if m.transitionFieldOptionsLoading == nil {
		m.transitionFieldOptionsLoading = make(map[string]bool)
	}
	if m.transitionFieldOptionsErr == nil {
		m.transitionFieldOptionsErr = make(map[string]error)
	}
	if m.transitionFieldOptionsQuery == nil {
		m.transitionFieldOptionsQuery = make(map[string]string)
	}
	if m.transitionFieldSelections == nil {
		m.transitionFieldSelections = make(map[string]int)
	}
}

func (m *Model) replaceTransitionFieldOptions(fieldID string, options []jira.FieldOption) {
	selected, ok := m.selectedIssue()
	if !ok {
		return
	}
	transitions := m.transitions[selected.Key]
	for transitionIndex := range transitions {
		for fieldIndex := range transitions[transitionIndex].Fields {
			if transitions[transitionIndex].Fields[fieldIndex].ID == fieldID {
				transitions[transitionIndex].Fields[fieldIndex].AllowedValues = append([]jira.FieldOption(nil), options...)
			}
		}
	}
	m.transitions[selected.Key] = transitions
}

func (m Model) transitionSelectedOption(field jira.TransitionField) (jira.FieldOption, bool) {
	if m.transitionFieldSelections == nil {
		return jira.FieldOption{}, false
	}
	index, ok := m.transitionFieldSelections[field.ID]
	if !ok || index < 0 || index >= len(field.AllowedValues) {
		return jira.FieldOption{}, false
	}
	return field.AllowedValues[index], true
}

func (m Model) transitionSelectedOptions(field jira.TransitionField) []jira.FieldOption {
	if m.transitionFieldMultiSelections == nil {
		return nil
	}
	selected := m.transitionFieldMultiSelections[field.ID]
	if len(selected) == 0 {
		return nil
	}
	options := make([]jira.FieldOption, 0, len(selected))
	for index, option := range field.AllowedValues {
		if selected[index] {
			options = append(options, option)
		}
	}
	return options
}

func (m Model) transitionMultiSelectedLabel(field jira.TransitionField) string {
	options := m.transitionSelectedOptions(field)
	if len(options) == 0 {
		return ""
	}
	labels := make([]string, 0, len(options))
	for _, option := range options {
		labels = append(labels, displayValue(option.Name, option.ID))
	}
	return strings.Join(labels, ", ")
}

func (m Model) transitionFieldValues(transition jira.Transition) ([]jira.TransitionFieldValue, error) {
	fields := supportedTransitionFields(transition.Fields)
	values := make([]jira.TransitionFieldValue, 0, len(fields))
	for _, field := range fields {
		switch {
		case transitionFieldUsesMultiSelect(field):
			options := m.transitionSelectedOptions(field)
			if len(options) == 0 {
				if field.Required {
					return nil, fmt.Errorf("select %s", displayValue(field.Name, field.ID))
				}
				continue
			}
			values = append(values, jira.TransitionFieldValue{FieldID: field.ID, SchemaType: field.SchemaType, SchemaSystem: field.SchemaSystem, SchemaItems: field.SchemaItems, Options: options})
		case transitionFieldUsesPicker(field):
			option, ok := m.transitionSelectedOption(field)
			if !ok {
				if field.Required {
					return nil, fmt.Errorf("select %s", displayValue(field.Name, "Resolution"))
				}
				continue
			}
			values = append(values, jira.TransitionFieldValue{FieldID: field.ID, SchemaType: field.SchemaType, SchemaSystem: field.SchemaSystem, SchemaItems: field.SchemaItems, Option: option})
		case transitionFieldUsesText(field):
			text := strings.TrimSpace(m.transitionFieldDrafts[field.ID])
			if text == "" && m.transitionFieldCommentEditorReady && m.transitionFieldEditorFieldID == field.ID {
				text = strings.TrimSpace(m.transitionFieldCommentEditor.Value())
			}
			if text == "" {
				if field.Required {
					return nil, fmt.Errorf("write %s", displayValue(field.Name, "Comment"))
				}
				continue
			}
			values = append(values, jira.TransitionFieldValue{FieldID: field.ID, SchemaType: field.SchemaType, SchemaSystem: field.SchemaSystem, SchemaItems: field.SchemaItems, Text: text})
		}
	}
	return values, nil
}

func (m Model) updateTransitionFieldForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	transition, ok := m.selectedStatusTransition()
	if !ok {
		m.transitionFieldEditing = false
		return m, nil
	}
	fields := supportedTransitionFields(transition.Fields)
	if len(fields) == 0 {
		m.transitionFieldEditing = false
		return m, nil
	}
	selectedIndex := clamp(m.selectedTransitionField, 0, len(fields)-1)
	selectedField := fields[selectedIndex]
	if transitionFieldUsesPicker(selectedField) && strings.TrimSpace(selectedField.AutoCompleteURL) != "" && len(selectedField.AllowedValues) == 0 {
		switch keyMsg.String() {
		case "backspace", "ctrl+h":
			query := []rune(m.transitionFieldFilters[selectedField.ID])
			if len(query) > 0 {
				m.transitionFieldFilters[selectedField.ID] = string(query[:len(query)-1])
			}
			return m, m.requestTransitionFieldOptions(selectedField)
		}
		if len(keyMsg.String()) == 1 {
			m.ensureTransitionFieldOptionsState()
			m.transitionFieldFilters[selectedField.ID] += keyMsg.String()
			return m, m.requestTransitionFieldOptions(selectedField)
		}
	}
	switch keyMsg.String() {
	case "esc":
		m.transitionFieldEditing = false
		m.detailNotice = ""
		return m, nil
	case "tab":
		m.selectedTransitionField = (selectedIndex + 1) % len(fields)
		m.detailNotice = ""
		return m, nil
	case "shift+tab":
		m.selectedTransitionField = (selectedIndex - 1 + len(fields)) % len(fields)
		m.detailNotice = ""
		return m, nil
	case "ctrl+s", "enter":
		values, err := m.transitionFieldValues(transition)
		if err != nil {
			m.detailNotice = "Transition field required: " + err.Error() + "."
			return m, nil
		}
		selected, ok := m.selectedIssue()
		if !ok {
			return m, nil
		}
		return m.submitTransitionWithFields(selected.Key, transition, values)
	case "up", "k":
		if (transitionFieldUsesPicker(selectedField) || transitionFieldUsesMultiSelect(selectedField)) && len(selectedField.AllowedValues) > 0 {
			current := m.transitionFieldSelections[selectedField.ID]
			if current < 0 {
				current = 0
			}
			m.transitionFieldSelections[selectedField.ID] = clamp(current-1, 0, len(selectedField.AllowedValues)-1)
		}
		return m, nil
	case "down", "j":
		if (transitionFieldUsesPicker(selectedField) || transitionFieldUsesMultiSelect(selectedField)) && len(selectedField.AllowedValues) > 0 {
			current := m.transitionFieldSelections[selectedField.ID]
			m.transitionFieldSelections[selectedField.ID] = clamp(current+1, 0, len(selectedField.AllowedValues)-1)
		}
		return m, nil
	case " ":
		if transitionFieldUsesMultiSelect(selectedField) && len(selectedField.AllowedValues) > 0 {
			current := m.transitionFieldSelections[selectedField.ID]
			if current < 0 {
				current = 0
				m.transitionFieldSelections[selectedField.ID] = current
			}
			if m.transitionFieldMultiSelections == nil {
				m.transitionFieldMultiSelections = make(map[string]map[int]bool)
			}
			if m.transitionFieldMultiSelections[selectedField.ID] == nil {
				m.transitionFieldMultiSelections[selectedField.ID] = make(map[int]bool)
			}
			m.transitionFieldMultiSelections[selectedField.ID][current] = !m.transitionFieldMultiSelections[selectedField.ID][current]
		}
		return m, nil
	}
	if transitionFieldUsesPicker(selectedField) && strings.TrimSpace(selectedField.AutoCompleteURL) != "" {
		switch keyMsg.String() {
		case "backspace", "ctrl+h":
			query := []rune(m.transitionFieldFilters[selectedField.ID])
			if len(query) > 0 {
				m.transitionFieldFilters[selectedField.ID] = string(query[:len(query)-1])
			}
			return m, m.requestTransitionFieldOptions(selectedField)
		}
		if len(keyMsg.String()) == 1 {
			m.ensureTransitionFieldOptionsState()
			m.transitionFieldFilters[selectedField.ID] += keyMsg.String()
			return m, m.requestTransitionFieldOptions(selectedField)
		}
	}
	if transitionFieldUsesText(selectedField) {
		m.ensureTransitionTextEditor(selectedField)
		editor, cmd := m.transitionFieldCommentEditor.Update(msg)
		m.transitionFieldCommentEditor = editor
		m.transitionFieldDrafts[selectedField.ID] = editor.Value()
		return m, cmd
	}
	return m, nil
}

func (m *Model) ensureTransitionTextEditor(field jira.TransitionField) {
	if m.transitionFieldCommentEditorReady && m.transitionFieldEditorFieldID == field.ID {
		return
	}
	m.transitionFieldCommentEditor = newCommentEditor(m.transitionFieldDrafts[field.ID])
	m.transitionFieldCommentEditorReady = true
	m.transitionFieldEditorFieldID = field.ID
}

func (m *Model) requestTransitionFieldOptions(field jira.TransitionField) tea.Cmd {
	m.ensureTransitionFieldOptionsState()
	fieldID := strings.TrimSpace(field.ID)
	query := strings.TrimSpace(m.transitionFieldFilters[fieldID])
	if fieldID == "" || strings.TrimSpace(field.AutoCompleteURL) == "" {
		return nil
	}
	if m.transitionFieldOptionsLoading[fieldID] && m.transitionFieldOptionsQuery[fieldID] == query {
		return nil
	}
	m.nextRequestID++
	m.activeTransitionFieldOptionsReqID = m.nextRequestID
	m.transitionFieldOptionsLoading[fieldID] = true
	m.transitionFieldOptionsErr[fieldID] = nil
	m.transitionFieldOptionsQuery[fieldID] = query
	return m.submitTransitionFieldOptions(m.activeTransitionFieldOptionsReqID, field, query)
}

func (m Model) startPriorityEditor() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok {
		return m, nil
	}
	m.linkFocus = false
	m.hierarchyFocus = false
	m.actionFocus = false
	m.transitionFocus = false
	m.summaryFocus = false
	m.labelsFocus = false
	m.priorityFocus = true
	m.hydrateIssueEditMetadata(selected.Key)
	if metadata, ok := m.editMetadata[selected.Key]; ok {
		if _, cached := m.cachedIssueEditMetadata(selected.Key); !cached || m.isIssueEditMetadataFresh(selected.Key) {
			return m.beginPriorityEditing(metadata), nil
		}
	}
	if m.priorityMetadataLoading && m.priorityMetadataRequestKey == selected.Key {
		return m, nil
	}
	m.nextRequestID++
	m.activePriorityMetadataReqID = m.nextRequestID
	m.priorityMetadataRequestKey = selected.Key
	m.priorityMetadataLoading = true
	m.priorityMetadataErr = nil
	m.detailNotice = ""
	return m, m.submitEditMetadata(m.activePriorityMetadataReqID, selected.Key)
}

func (m Model) beginPriorityEditing(metadata jira.EditMetadata) Model {
	selected, ok := m.selectedIssue()
	if !ok {
		return m
	}
	if !metadata.Priority.Editable {
		m.priorityFocus = false
		m.detailNotice = "Priority is not editable for " + selected.Key + "."
		return m
	}
	if len(metadata.Priority.AllowedValues) == 0 {
		m.priorityFocus = false
		m.detailNotice = "Priority metadata returned no allowed values for " + selected.Key + "."
		return m
	}
	m.priorityFocus = true
	m.selectedPriority = indexFieldOptionByName(metadata.Priority.AllowedValues, selected.Priority)
	m.detailNotice = ""
	return m
}

func (m Model) startLabelsEditor() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok {
		return m, nil
	}
	m.linkFocus = false
	m.hierarchyFocus = false
	m.actionFocus = false
	m.transitionFocus = false
	m.summaryFocus = false
	m.priorityFocus = false
	m.assigneeFocus = false
	m.labelsFocus = true
	m.hydrateIssueEditMetadata(selected.Key)
	if metadata, ok := m.editMetadata[selected.Key]; ok {
		if _, cached := m.cachedIssueEditMetadata(selected.Key); !cached || m.isIssueEditMetadataFresh(selected.Key) {
			return m.beginLabelsEditing(metadata), nil
		}
	}
	if m.labelsMetadataLoading && m.labelsMetadataRequestKey == selected.Key {
		return m, nil
	}
	m.nextRequestID++
	m.activeLabelsMetadataReqID = m.nextRequestID
	m.labelsMetadataRequestKey = selected.Key
	m.labelsMetadataLoading = true
	m.labelsMetadataErr = nil
	m.detailNotice = ""
	return m, m.submitEditMetadata(m.activeLabelsMetadataReqID, selected.Key)
}

func (m Model) beginLabelsEditing(metadata jira.EditMetadata) Model {
	selected, ok := m.selectedIssue()
	if !ok {
		return m
	}
	if !metadata.Labels.Editable {
		m.labelsFocus = false
		m.detailNotice = "Labels are not editable for " + selected.Key + "."
		return m
	}
	m.labelsFocus = true
	m.labelsEditing = true
	m.labelsDraft = strings.Join(m.currentIssueLabels(selected.Key), ", ")
	m.labelsDirty = false
	m.labelsEditor = newLabelsEditor(m.labelsDraft)
	m.labelsEditorReady = true
	m.detailNotice = ""
	return m
}

func (m Model) currentIssueLabels(key string) []string {
	if detail, ok := m.details[key]; ok {
		return append([]string{}, detail.Labels...)
	}
	return nil
}

func (m Model) submitSelectedLabels() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok {
		return m, nil
	}
	if !m.labelsDirty {
		m.detailNotice = "Edit labels before saving."
		return m, nil
	}
	labels := parseLabelsDraft(m.labelsEditorValue())
	if labelsEqual(labels, m.currentIssueLabels(selected.Key)) {
		m.detailNotice = "Labels unchanged."
		return m, nil
	}
	m.nextRequestID++
	m.activeLabelsReqID = m.nextRequestID
	m.labelsSubmitting = true
	m.labelsSubmitKey = selected.Key
	m.labelsSubmitValue = append([]string{}, labels...)
	m.detailNotice = "Updating labels."
	return m, m.submitUpdateLabels(m.activeLabelsReqID, selected.Key, labels)
}

func (m Model) startComponentsEditor() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok {
		return m, nil
	}
	m.linkFocus = false
	m.hierarchyFocus = false
	m.actionFocus = false
	m.transitionFocus = false
	m.summaryFocus = false
	m.priorityFocus = false
	m.labelsFocus = false
	m.assigneeFocus = false
	m.componentsFocus = true
	m.hydrateIssueEditMetadata(selected.Key)
	if metadata, ok := m.editMetadata[selected.Key]; ok {
		if _, cached := m.cachedIssueEditMetadata(selected.Key); !cached || m.isIssueEditMetadataFresh(selected.Key) {
			return m.beginComponentsEditing(metadata), nil
		}
	}
	if m.componentsMetadataLoading && m.componentsMetadataRequestKey == selected.Key {
		return m, nil
	}
	m.nextRequestID++
	m.activeComponentsMetadataReqID = m.nextRequestID
	m.componentsMetadataRequestKey = selected.Key
	m.componentsMetadataLoading = true
	m.componentsMetadataErr = nil
	m.detailNotice = ""
	return m, m.submitEditMetadata(m.activeComponentsMetadataReqID, selected.Key)
}

func (m Model) beginComponentsEditing(metadata jira.EditMetadata) Model {
	selected, ok := m.selectedIssue()
	if !ok {
		return m
	}
	if !metadata.Components.Editable {
		m.componentsFocus = false
		m.detailNotice = "Components are not editable for " + selected.Key + "."
		return m
	}
	if len(metadata.Components.AllowedValues) == 0 {
		m.componentsFocus = false
		m.detailNotice = "Components metadata returned no allowed values for " + selected.Key + "."
		return m
	}
	m.componentsFocus = true
	m.selectedComponent = 0
	m.selectedComponents = map[string]bool{}
	current := m.currentIssueComponents(selected.Key)
	for _, option := range metadata.Components.AllowedValues {
		for _, name := range current {
			if strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(displayValue(option.Name, option.ID))) {
				m.selectedComponents[componentSelectionKey(option)] = true
				break
			}
		}
	}
	m.componentsFilter = ""
	m.componentsFilterEditor = newComponentsFilterInput("")
	m.componentsFilterEditorReady = true
	m.componentsDirty = false
	m.detailNotice = ""
	return m
}

func (m Model) currentIssueComponents(key string) []string {
	if detail, ok := m.details[key]; ok {
		return append([]string{}, detail.Components...)
	}
	return nil
}

func (m Model) componentOptions(key string) []jira.FieldOption {
	if metadata, ok := m.editMetadata[key]; ok {
		return metadata.Components.AllowedValues
	}
	return nil
}

func (m Model) submitSelectedComponents() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok {
		return m, nil
	}
	if !m.componentsDirty {
		m.detailNotice = "Edit components before saving."
		return m, nil
	}
	components := m.selectedComponentOptions(selected.Key)
	if componentOptionsEqual(components, m.currentIssueComponents(selected.Key)) {
		m.detailNotice = "Components unchanged."
		return m, nil
	}
	m.nextRequestID++
	m.activeComponentsReqID = m.nextRequestID
	m.componentsSubmitting = true
	m.componentsSubmitKey = selected.Key
	m.componentsSubmitValue = append([]jira.FieldOption{}, components...)
	m.detailNotice = "Updating components."
	return m, m.submitUpdateComponents(m.activeComponentsReqID, selected.Key, components)
}

func (m Model) priorityOptions(key string) []jira.FieldOption {
	if metadata, ok := m.editMetadata[key]; ok {
		return metadata.Priority.AllowedValues
	}
	return nil
}

func (m *Model) moveSelectedPriority(delta int) {
	selected, ok := m.selectedIssue()
	if !ok {
		m.selectedPriority = 0
		return
	}
	options := m.priorityOptions(selected.Key)
	if len(options) == 0 {
		m.selectedPriority = 0
		return
	}
	m.selectedPriority = clamp(m.selectedPriority+delta, 0, len(options)-1)
}

func (m Model) submitSelectedPriority() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok {
		return m, nil
	}
	options := m.priorityOptions(selected.Key)
	if len(options) == 0 {
		return m.startPriorityEditor()
	}
	priority := options[clamp(m.selectedPriority, 0, len(options)-1)]
	priorityName := displayValue(priority.Name, priority.ID)
	if strings.TrimSpace(priorityName) == "" {
		m.detailNotice = "Priority update failed: missing priority value."
		return m, nil
	}
	if priorityName == strings.TrimSpace(selected.Priority) {
		m.detailNotice = "Priority unchanged."
		return m, nil
	}
	m.nextRequestID++
	m.activePriorityReqID = m.nextRequestID
	m.prioritySubmitting = true
	m.prioritySubmitKey = selected.Key
	m.prioritySubmitValue = priority
	m.detailNotice = "Updating priority to " + priorityName + "."
	return m, m.submitUpdatePriority(m.activePriorityReqID, selected.Key, priority)
}

func (m Model) startAssigneePicker() (Model, tea.Cmd) {
	m.linkFocus = false
	m.hierarchyFocus = false
	m.actionFocus = false
	m.transitionFocus = false
	m.priorityFocus = false
	m.summaryFocus = false
	m.assigneeFocus = true
	m.assigneeQuery = ""
	m.assigneeQueryEditor = newUserSearchInput("")
	m.assigneeQueryEditorReady = true
	m.assigneeUsers = nil
	m.selectedAssignee = 0
	m.assigneeSearchLoading = false
	m.assigneeSearchErr = nil
	m.assigneeSearchIssueKey = ""
	m.detailNotice = ""
	return m, nil
}

func (m Model) updateAssigneePicker(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.assigneeFocus = false
		m.assigneeQuery = ""
		m.assigneeQueryEditor = textinput.Model{}
		m.assigneeQueryEditorReady = false
		m.assigneeUsers = nil
		m.selectedAssignee = 0
		m.assigneeSearchLoading = false
		m.assigneeSearchErr = nil
		m.assigneeSearchIssueKey = ""
		m.detailNotice = ""
		return m, nil
	case "enter":
		return m.submitSelectedAssignee()
	case "up":
		m.moveSelectedAssignee(-1)
		return m, nil
	case "down":
		m.moveSelectedAssignee(1)
		return m, nil
	}
	if !m.assigneeQueryEditorReady {
		m.assigneeQueryEditor = newUserSearchInput(m.assigneeQuery)
		m.assigneeQueryEditorReady = true
	}
	previous := m.assigneeQueryEditor.Value()
	editor, _ := m.assigneeQueryEditor.Update(msg)
	m.assigneeQueryEditor = editor
	query := strings.TrimSpace(editor.Value())
	changed := strings.TrimSpace(previous) != query
	if !changed {
		return m, nil
	}
	m.assigneeQuery = query
	m.selectedAssignee = 0
	m.assigneeSearchErr = nil
	if strings.TrimSpace(query) == "" {
		m.assigneeUsers = nil
		m.assigneeSearchLoading = false
		m.assigneeSearchIssueKey = ""
		return m, nil
	}
	selected, ok := m.selectedIssue()
	if !ok || strings.TrimSpace(selected.Key) == "" {
		m.assigneeUsers = nil
		m.assigneeSearchLoading = false
		m.assigneeSearchIssueKey = ""
		m.detailNotice = "Select an issue before searching assignable users."
		return m, nil
	}
	if users, ok := m.cachedAssignableUserSearch(selected.Key, query); ok {
		m.assigneeUsers = users
		m.assigneeSearchLoading = false
		m.selectedAssignee = clamp(m.selectedAssignee, 0, max(0, len(users)-1))
		return m, nil
	}
	m.nextRequestID++
	m.assigneeSearchReqID = m.nextRequestID
	m.assigneeSearchIssueKey = selected.Key
	m.assigneeSearchLoading = true
	return m, m.submitUserSearch(m.assigneeSearchReqID, query, selected.Key)
}

func (m *Model) moveSelectedAssignee(delta int) {
	if len(m.assigneeUsers) == 0 {
		m.selectedAssignee = 0
		return
	}
	m.selectedAssignee = clamp(m.selectedAssignee+delta, 0, len(m.assigneeUsers)-1)
}

func (m Model) submitSelectedAssignee() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok {
		return m, nil
	}
	if len(m.assigneeUsers) == 0 {
		m.detailNotice = "Search for a Jira user before assigning."
		return m, nil
	}
	assignee := m.assigneeUsers[clamp(m.selectedAssignee, 0, len(m.assigneeUsers)-1)]
	if strings.TrimSpace(assignee.AccountID) == "" {
		m.detailNotice = "Assignee update failed: missing account ID."
		return m, nil
	}
	assigneeName := displayValue(assignee.DisplayName, assignee.Email)
	if assigneeName == strings.TrimSpace(selected.Assignee) {
		m.detailNotice = "Assignee unchanged."
		return m, nil
	}
	m.nextRequestID++
	m.activeAssigneeReqID = m.nextRequestID
	m.assigneeSubmitting = true
	m.assigneeSubmitKey = selected.Key
	m.assigneeSubmitValue = assignee
	m.detailNotice = "Updating assignee to " + displayValue(assigneeName, "Unknown") + "."
	return m, m.submitUpdateAssignee(m.activeAssigneeReqID, selected.Key, assignee)
}

func (m Model) startSummaryEditor() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok {
		return m, nil
	}
	m.linkFocus = false
	m.hierarchyFocus = false
	m.actionFocus = false
	m.transitionFocus = false
	m.summaryFocus = true
	m.hydrateIssueEditMetadata(selected.Key)
	if metadata, ok := m.editMetadata[selected.Key]; ok {
		if _, cached := m.cachedIssueEditMetadata(selected.Key); !cached || m.isIssueEditMetadataFresh(selected.Key) {
			if !metadata.Summary.Editable {
				m.detailNotice = "Summary is not editable for " + selected.Key + "."
				return m, nil
			}
			m.beginSummaryEditing()
			return m, nil
		}
	}
	if m.summaryMetadataLoading && m.summaryMetadataRequestKey == selected.Key {
		return m, nil
	}
	m.nextRequestID++
	m.activeSummaryMetadataReqID = m.nextRequestID
	m.summaryMetadataRequestKey = selected.Key
	m.summaryMetadataLoading = true
	m.summaryMetadataErr = nil
	m.detailNotice = ""
	return m, m.submitEditMetadata(m.activeSummaryMetadataReqID, selected.Key)
}

func (m *Model) beginSummaryEditing() {
	selected, ok := m.selectedIssue()
	if !ok {
		return
	}
	m.summaryFocus = true
	m.summaryEditing = true
	m.summaryDraft = selected.Summary
	m.summaryDirty = false
	if detail, ok := m.details[selected.Key]; ok && strings.TrimSpace(detail.Summary) != "" {
		m.summaryDraft = detail.Summary
	}
	m.summaryEditor = newSummaryEditor(m.summaryDraft)
	m.summaryEditorReady = true
	m.detailNotice = ""
}

func (m Model) updateSummaryEditor(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.summaryEditing = false
		m.summaryDraft = ""
		m.summaryDirty = false
		m.summaryEditor = newSummaryEditor("")
		m.summaryEditorReady = true
		m.detailNotice = ""
		return m, nil
	case "enter":
		m.summaryDraft = m.summaryEditorValue()
		return m.submitSummaryDraft()
	}
	m.ensureSummaryEditor()
	m.configureSummaryEditor()
	before := m.summaryEditor.Value()
	editor, cmd := m.summaryEditor.Update(msg)
	m.summaryEditor = editor
	m.summaryDraft = m.summaryEditor.Value()
	if m.summaryDraft != before {
		m.summaryDirty = true
		m.detailNotice = ""
	}
	return m, cmd
}

func (m Model) submitSummaryDraft() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok {
		return m, nil
	}
	summary := strings.TrimSpace(m.summaryDraft)
	if summary == "" {
		m.detailNotice = "Summary cannot be empty."
		return m, nil
	}
	if !m.summaryDirty {
		m.detailNotice = "Edit summary before saving."
		return m, nil
	}
	current := strings.TrimSpace(selected.Summary)
	if detail, ok := m.details[selected.Key]; ok && strings.TrimSpace(detail.Summary) != "" {
		current = strings.TrimSpace(detail.Summary)
	}
	if summary == current {
		m.detailNotice = "Summary unchanged."
		return m, nil
	}
	m.nextRequestID++
	m.activeSummaryReqID = m.nextRequestID
	m.summarySubmitting = true
	m.summarySubmitKey = selected.Key
	m.summarySubmitValue = summary
	m.detailNotice = "Updating summary for " + selected.Key + "."
	return m, m.submitUpdateSummary(m.activeSummaryReqID, selected.Key, summary)
}

func (m *Model) focusHierarchy() {
	children := m.currentHierarchyChildren()
	m.linkFocus = false
	m.hierarchyFocus = len(children) > 0
	m.selectedHierarchy = clamp(m.selectedHierarchy, 0, max(0, len(children)-1))
	m.jumpDetailSection("Hierarchy")
	if len(children) == 0 {
		m.detailNotice = "No child issues are available for this ticket."
	}
}

func (m *Model) moveSelectedHierarchyIssue(delta int) {
	children := m.currentHierarchyChildren()
	if len(children) == 0 {
		m.selectedHierarchy = 0
		return
	}
	m.selectedHierarchy = clamp(m.selectedHierarchy+delta, 0, len(children)-1)
}

func (m Model) canMoveHierarchySelection() bool {
	if m.mode != modeDetail {
		return false
	}
	section, ok := m.focusedDetailSection()
	if !ok || section.ID != "hierarchy" {
		return false
	}
	return len(m.currentHierarchyChildren()) > 0
}

func (m Model) canUseLinkSelection() bool {
	if m.mode != modeDetail {
		return false
	}
	section, ok := m.focusedDetailSection()
	if !ok || section.ID != "links" {
		return false
	}
	return len(m.currentDetailLinks()) > 0
}

func (m Model) canUseActionSelection() bool {
	if m.mode != modeDetail {
		return false
	}
	section, ok := m.focusedDetailSection()
	return ok && section.ID == "actions"
}

func (m Model) currentHierarchyChildren() []jira.Issue {
	selected, ok := m.selectedIssue()
	if !ok {
		return nil
	}
	display := selected
	if detail, hasDetail := m.details[selected.Key]; hasDetail {
		display = detail.Issue
	}
	rows := m.hierarchyRows(display.Key)
	children := make([]jira.Issue, 0, len(rows))
	for _, row := range rows {
		children = append(children, row.Issue)
	}
	return children
}

func (m Model) openSelectedHierarchyIssue() (Model, tea.Cmd) {
	children := m.currentHierarchyChildren()
	if len(children) == 0 {
		return m, nil
	}
	child := children[clamp(m.selectedHierarchy, 0, len(children)-1)]
	for index, issue := range m.issues {
		if issue.Key == child.Key {
			m.detailBackStack = append(m.detailBackStack, m.selected)
			m.selected = index
			m.resetDetailScroll()
			m.detailFocus = 0
			m.hierarchyFocus = false
			m.linkFocus = false
			m.detailNotice = ""
			return m.startDetailRequestForSelected()
		}
	}
	return m, nil
}

func (m Model) popDetailBackStack() (Model, tea.Cmd) {
	if len(m.detailBackStack) == 0 {
		return m, nil
	}
	previous := m.detailBackStack[len(m.detailBackStack)-1]
	m.detailBackStack = m.detailBackStack[:len(m.detailBackStack)-1]
	m.selected = clamp(previous, 0, max(0, len(m.issues)-1))
	m.resetDetailScroll()
	m.detailFocus = 0
	m.hierarchyFocus = false
	m.linkFocus = false
	m.detailNotice = ""
	return m.startDetailRequestForSelected()
}

func (m Model) detailChildren(parentKey string) []jira.Issue {
	children := make([]jira.Issue, 0)
	for _, issue := range m.issues {
		if issue.ParentKey == parentKey {
			children = append(children, issue)
		}
	}
	return children
}

func (m Model) hierarchyRows(parentKey string) []hierarchyRow {
	related := m.detailChildren(parentKey)
	children := make([]jira.Issue, 0, len(related))
	subtasks := make([]jira.Issue, 0, len(related))
	for _, issue := range related {
		if isSubtaskIssue(issue) {
			subtasks = append(subtasks, issue)
			continue
		}
		children = append(children, issue)
	}
	rows := make([]hierarchyRow, 0, len(related))
	for _, issue := range children {
		rows = append(rows, hierarchyRow{
			Issue: issue,
			Group: "children",
			Index: len(rows),
		})
	}
	for _, issue := range subtasks {
		rows = append(rows, hierarchyRow{
			Issue: issue,
			Group: "subtasks",
			Index: len(rows),
		})
	}
	return rows
}

func splitHierarchyRows(rows []hierarchyRow) ([]hierarchyRow, []hierarchyRow) {
	children := make([]hierarchyRow, 0, len(rows))
	subtasks := make([]hierarchyRow, 0, len(rows))
	for _, row := range rows {
		if row.Group == "subtasks" {
			subtasks = append(subtasks, row)
			continue
		}
		children = append(children, row)
	}
	return children, subtasks
}

func isSubtaskIssue(issue jira.Issue) bool {
	issueType := strings.ToLower(strings.TrimSpace(issue.IssueType))
	return issueType == "subtask" || issueType == "sub-task"
}

func (m Model) renderLinksSection(links []detailLink, width int) string {
	lines := make([]string, 0, len(links)+1)
	help := ""
	if m.linkFocus {
		help = "enter open  y copy  d unlink"
	}
	lines = append(lines, m.detailSectionHeader("links", "Links", help, width))
	rows := make([][]string, 0, len(links))
	for index, link := range links {
		display := linkDisplayText(link)
		targetWidth := max(16, width-18)
		key := fmt.Sprintf("[%d]", index+1)
		keyStyle := m.theme.Key
		kindStyle := m.theme.Muted
		targetStyle := m.theme.Text
		if m.linkFocus && index == clamp(m.selectedLink, 0, len(links)-1) {
			key = "> " + key
			keyStyle = m.theme.Selected
			targetStyle = m.theme.Selected
		} else {
			key = "  " + key
		}
		rows = append(rows, []string{
			keyStyle.Render(key),
			kindStyle.Render(link.Kind),
			targetStyle.Render(truncate(display, targetWidth)),
		})
	}
	lines = append(lines, m.detailTable(0, []string{"", "TYPE", "TARGET"}, rows, nil))
	return strings.Join(lines, "\n")
}

func linkDisplayText(link detailLink) string {
	if link.Kind == linkdetect.KindEmail {
		if address := linkdetect.MailtoAddress(link.Target); address != "" {
			return address
		}
	}
	if link.Label != "" {
		return link.Label
	}
	return link.Target
}

func linkCopyText(link detailLink) string {
	if link.CopyText != "" {
		return link.CopyText
	}
	if link.Kind == linkdetect.KindEmail {
		return linkDisplayText(link)
	}
	return link.Target
}

func collectDetailLinks(value string) []detailLink {
	detected := linkdetect.Detect(value)
	links := make([]detailLink, 0, len(detected))
	for _, link := range detected {
		links = append(links, detailLink{
			Kind:   link.Kind,
			Label:  link.Label,
			Target: link.Target,
			Start:  link.Start,
			End:    link.End,
		})
	}
	return links
}

func detailLinks(detail jira.IssueDetail) []detailLink {
	links := make([]detailLink, 0, len(detail.IssueLinks))
	for _, issueLink := range detail.IssueLinks {
		if strings.TrimSpace(issueLink.Key) == "" {
			continue
		}
		links = append(links, issueDetailLink(issueLink))
	}
	links = append(links, collectDetailLinks(detail.Description)...)
	return links
}

func issueDetailLink(link jira.IssueLink) detailLink {
	parts := []string{link.Key}
	if relationship := strings.TrimSpace(link.Relationship); relationship != "" {
		parts = append(parts, relationship)
	}
	if status := strings.TrimSpace(link.Status); status != "" && !strings.EqualFold(status, "Unknown") {
		parts = append(parts, status)
	}
	if summary := strings.TrimSpace(link.Summary); summary != "" {
		parts = append(parts, summary)
	}
	return detailLink{
		Kind:     "Issue",
		Label:    strings.Join(parts, "  "),
		Target:   link.URL,
		CopyText: link.Key,
		LinkID:   link.LinkID,
	}
}

func (m *Model) focusDetailLinks() {
	links := m.currentDetailLinks()
	if len(links) == 0 {
		m.linkFocus = false
		m.detailNotice = "No links found in this ticket description."
		return
	}
	m.linkFocus = true
	m.selectedLink = clamp(m.selectedLink, 0, len(links)-1)
	m.detailNotice = ""
	m.jumpDetailSection("Links")
}

func (m *Model) moveSelectedDetailLink(delta int) {
	links := m.currentDetailLinks()
	if len(links) == 0 {
		m.linkFocus = false
		m.selectedLink = 0
		return
	}
	m.selectedLink = clamp(m.selectedLink+delta, 0, len(links)-1)
}

func (m *Model) selectDetailLinkNumber(value string) {
	links := m.currentDetailLinks()
	if len(links) == 0 {
		return
	}
	number := int(value[0] - '0')
	if number <= 0 || number > len(links) {
		return
	}
	m.linkFocus = true
	m.selectedLink = number - 1
	m.detailNotice = ""
	m.jumpDetailSection("Links")
}

func (m Model) openSelectedDetailLink() (Model, tea.Cmd) {
	link, ok := m.selectedDetailLink()
	if !ok {
		m.detailNotice = "No link selected."
		return m, nil
	}
	target := link.Target
	return m, func() tea.Msg {
		return linkActionMsg{
			action: "open",
			target: target,
			err:    openExternal(target),
		}
	}
}

func (m Model) copySelectedDetailLink() (Model, tea.Cmd) {
	link, ok := m.selectedDetailLink()
	if !ok {
		m.detailNotice = "No link selected."
		return m, nil
	}
	target := linkCopyText(link)
	return m, func() tea.Msg {
		return linkActionMsg{
			action: "copy",
			target: target,
			err:    copyToClipboard(target),
		}
	}
}

func (m Model) openSelectedIssue() (Model, tea.Cmd) {
	issue, ok := m.selectedIssue()
	if !ok || strings.TrimSpace(issue.URL) == "" {
		m.detailNotice = "No issue URL available."
		return m, nil
	}
	target := issue.URL
	return m, func() tea.Msg {
		return linkActionMsg{
			action: "open",
			target: target,
			err:    openExternal(target),
		}
	}
}

func (m Model) copySelectedIssueKey() (Model, tea.Cmd) {
	issue, ok := m.selectedIssue()
	if !ok || strings.TrimSpace(issue.Key) == "" {
		m.detailNotice = "No issue key available."
		return m, nil
	}
	target := issue.Key
	return m, func() tea.Msg {
		return linkActionMsg{
			action: "copy",
			target: target,
			err:    copyToClipboard(target),
		}
	}
}

func (m Model) copySelectedIssueURL() (Model, tea.Cmd) {
	issue, ok := m.selectedIssue()
	if !ok || strings.TrimSpace(issue.URL) == "" {
		m.detailNotice = "No issue URL available."
		return m, nil
	}
	target := issue.URL
	return m, func() tea.Msg {
		return linkActionMsg{
			action: "copy",
			target: target,
			err:    copyToClipboard(target),
		}
	}
}

func (m Model) selectedDetailLink() (detailLink, bool) {
	links := m.currentDetailLinks()
	if len(links) == 0 {
		return detailLink{}, false
	}
	index := clamp(m.selectedLink, 0, len(links)-1)
	return links[index], true
}

func (m Model) currentDetailLinks() []detailLink {
	if len(m.issues) == 0 || m.selected < 0 || m.selected >= len(m.issues) {
		return nil
	}
	detail, ok := m.details[m.issues[m.selected].Key]
	if !ok {
		return nil
	}
	return detailLinks(detail)
}

func (m *Model) jumpDetailSection(title string) {
	m.saveDetailSectionOffset()
	for index, target := range m.detailTargets() {
		if target.Kind != detailTargetSection {
			continue
		}
		if strings.EqualFold(target.Label, title) || strings.EqualFold(target.ID, title) {
			m.detailFocus = index
			m.restoreDetailSectionOffset()
			return
		}
	}
}

func (m *Model) setDetailOffset(offset int) {
	content := m.currentDetailContent()
	rows := max(1, m.fullDetailRows()-1)
	width := m.currentDetailBodyWidth()
	vp := m.newDetailViewport(content, width, rows)
	vp.SetYOffset(offset)
	m.detailOffset = vp.YOffset()
	m.saveDetailSectionOffset()
}

func (m *Model) saveDetailSectionOffset() {
	section, ok := m.focusedDetailSection()
	if !ok {
		return
	}
	if m.detailSectionOffset == nil {
		m.detailSectionOffset = make(map[string]int)
	}
	m.detailSectionOffset[section.ID] = m.detailOffset
}

func (m *Model) restoreDetailSectionOffset() {
	section, ok := m.focusedDetailSection()
	if !ok {
		m.detailOffset = 0
		return
	}
	offset := 0
	if m.detailSectionOffset != nil {
		offset = m.detailSectionOffset[section.ID]
	}
	m.setDetailOffset(offset)
}

func (m *Model) resetDetailScroll() {
	m.detailOffset = 0
	m.detailSectionOffset = make(map[string]int)
}

func linkActionNotice(msg linkActionMsg) string {
	if msg.err != nil {
		return fmt.Sprintf("Could not %s %s: %v", msg.action, msg.target, msg.err)
	}
	switch msg.action {
	case "copy":
		return "Copied " + msg.target
	case "copy-draft":
		return "Copied " + msg.target
	case "open":
		return "Opened " + msg.target
	default:
		return msg.target
	}
}
