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
)

type detailLink struct {
	Kind     string
	Label    string
	Target   string
	CopyText string
	Start    int
	End      int
}

type detailAction struct {
	ID          string
	Label       string
	Description string
	Enabled     bool
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
		m.renderDetailHeaderMeta(headerWidth) + "\n" +
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
	if m.assigneeFocus || m.assigneeSubmitting {
		return m.renderAssigneeDialog(width)
	}
	if m.transitionFocus || m.transitionSubmitting {
		return m.renderStatusTransitionDialog(width)
	}
	if m.inlineAIOpen {
		return m.renderInlineAIDialog(width)
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
	return m.renderDetailDialog(width, "Change Status", selected.Key, strings.Join(lines, "\n"), "j/k select  enter apply  esc cancel")
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

func (m Model) detailSections() []detailSection {
	sections := []detailSection{
		{ID: "description", Label: "Description", Short: "Desc"},
		{ID: "hierarchy", Label: "Hierarchy", Short: "Tree"},
		{ID: "comments", Label: "Comments", Short: "Com"},
		{ID: "actions", Label: "Actions", Short: "Act"},
		{ID: "status", Label: "Status", Short: "Stat"},
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
		if childCount := len(m.hierarchyRows(display.Key)); childCount > 0 {
			sections[1].Badge = fmt.Sprintf("%d", childCount)
		}
		if comments, loaded := m.comments[display.Key]; loaded {
			sections[2].Badge = fmt.Sprintf("%d", len(comments))
		} else if m.commentsLoading && m.commentsRequestKey == display.Key {
			sections[2].Badge = "..."
		} else if m.commentsErr != nil && m.commentsRequestKey == display.Key {
			sections[2].Badge = "!"
		}
		if linkCount := len(detailLinks(jira.IssueDetail{Description: description, IssueLinks: issueLinks})); linkCount > 0 {
			links := detailSection{ID: "links", Label: "Links", Short: "Links", Badge: fmt.Sprintf("%d", linkCount)}
			sections = append(sections[:2], append([]detailSection{links}, sections[2:]...)...)
		}
	}
	return sections
}

func (m Model) detailTargets() []detailTarget {
	sections := m.detailSections()
	targets := []detailTarget{
		{ID: "summary", Label: "Summary", Kind: detailTargetField},
		{ID: "assignee", Label: "Assignee", Kind: detailTargetField},
		{ID: "priority", Label: "Priority", Kind: detailTargetField},
	}
	for _, section := range sections {
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
		m.startCommentComposer()
	default:
		m.linkFocus = false
		m.hierarchyFocus = false
		m.actionFocus = false
		m.transitionFocus = false
		m.priorityFocus = false
		m.assigneeFocus = false
		m.jumpDetailSection(section.Label)
	}
	return m, nil
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
	lines := []string{m.detailSectionHeader("comments", "Comments", "", width)}
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
			lines = append(lines, m.renderCommentBlock(index+1, len(comments), comment.Author, formatTime(comment.Created), body, width))
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

func (m Model) renderCommentBlock(index int, total int, author string, created string, body string, width int) string {
	contentWidth := max(20, width-4)
	header := m.theme.Key.Render(displayValue(author, "Unknown")) +
		m.theme.Muted.Render("  "+created+"  "+fmt.Sprintf("Comment %d/%d", index, max(1, total)))
	bodyWidth := max(12, contentWidth-2)
	renderedBody := m.renderRichDescriptionBody(wrapRichText(body, bodyWidth), bodyWidth)
	renderedBody = indentLines(renderedBody, "  ")
	return m.theme.CommentBlock.Width(contentWidth + 2).Render(header + "\n\n" + renderedBody)
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
		stateStyle := m.theme.Success
		descStyle := m.theme.Muted
		state := "ready"
		if m.actionFocus && index == cursor {
			marker = ">"
			labelStyle = m.theme.Selected
		} else if !action.Enabled {
			labelStyle = m.theme.Muted
			stateStyle = m.theme.Muted
			descStyle = m.theme.Muted
			state = "metadata"
		}
		rows = append(rows, []string{
			labelStyle.Render(marker),
			labelStyle.Render(action.Label),
			stateStyle.Render(state),
			descStyle.Render(truncate(action.Description, max(16, width-46))),
		})
	}
	lines = append(lines, m.detailTable(0, []string{"", "ACTION", "STATE", "DETAIL"}, rows, nil))
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
	return []detailAction{
		{ID: "comment", Label: "Add Comment", Description: "Write a Jira comment.", Enabled: true},
		{ID: "browser", Label: "Open In Browser", Description: "Open this ticket in Jira.", Enabled: true},
		{ID: "copy-key", Label: "Copy Key", Description: "Copy the ticket key.", Enabled: true},
		{ID: "copy-url", Label: "Copy URL", Description: "Copy the Jira URL.", Enabled: true},
		{ID: "edit-fields", Label: "Edit Fields", Description: "Will use Jira edit metadata before rendering fields.", Enabled: false},
		{ID: "transition", Label: "Transition Status", Description: "Load available Jira transitions and change status.", Enabled: true},
		{ID: "assign", Label: "Assign", Description: "Search assignable Jira users and change assignee.", Enabled: true},
		{ID: "subtask", Label: "Create Subtask", Description: "Will use Jira create metadata for required fields.", Enabled: false},
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
		m.detailNotice = action.Label + " needs Jira metadata before it can be edited safely."
		return m, nil
	}
	switch action.ID {
	case "comment":
		m.startCommentComposer()
		return m, nil
	case "browser":
		return m.openSelectedIssue()
	case "copy-key":
		return m.copySelectedIssueKey()
	case "copy-url":
		return m.copySelectedIssueURL()
	case "transition":
		return m.startStatusTransitionPicker()
	case "assign":
		return m.startAssigneePicker()
	default:
		return m, nil
	}
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
	m.nextRequestID++
	m.activeTransitionReqID = m.nextRequestID
	m.transitionSubmitting = true
	m.transitionSubmitKey = selected.Key
	m.transitionSubmitToStatus = transition.ToStatus
	m.detailNotice = "Updating status to " + displayValue(transition.ToStatus, transition.Name) + "."
	return m, m.submitIssueTransition(m.activeTransitionReqID, selected.Key, transition)
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
		return m, nil
	}
	if users, ok := m.cachedUserSearch(query); ok {
		m.assigneeUsers = users
		m.assigneeSearchLoading = false
		m.selectedAssignee = clamp(m.selectedAssignee, 0, max(0, len(users)-1))
		return m, nil
	}
	m.nextRequestID++
	m.assigneeSearchReqID = m.nextRequestID
	m.assigneeSearchLoading = true
	return m, m.submitUserSearch(m.assigneeSearchReqID, query)
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
	lines = append(lines, m.detailSectionHeader("links", "Links", "", width))
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
