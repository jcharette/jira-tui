package tui

import (
	"fmt"
	"strings"
	"time"

	bubbleshelp "charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/jcharette/jira-tui/internal/ui"
)

const backgroundActivityRecentWindow = 10 * time.Second

type browserLayout struct {
	contentWidth int
	listWidth    int
	rows         int
}

func (m Model) renderHeader(layout browserLayout) string {
	status := "ready"
	if m.loading {
		status = "loading"
	}

	left := m.theme.Header.Render("Jira") + " " + m.theme.Subtitle.Render(status) + " " + m.theme.Selected.Render(m.activeViewName())
	rightParts := []string{fmt.Sprintf("%d issues", len(m.issues)), m.viewFreshnessLabel()}
	if planning := m.planningSprintLabel(); planning != "" {
		rightParts = append(rightParts, planning)
	}
	rightText := strings.Join(rightParts, "  ")
	right := m.theme.Muted.Render(rightText)
	if activity := m.backgroundActivityLabel(); activity != "" {
		withActivity := right + "  " + m.renderBackgroundActivityLabel(activity)
		if lipgloss.Width(withActivity) <= max(0, layout.contentWidth-lipgloss.Width(left)-1) {
			right = withActivity
		}
	}
	rightColumn := lipgloss.PlaceHorizontal(
		max(0, layout.contentWidth-lipgloss.Width(left)-1),
		lipgloss.Right,
		right,
	)
	return lipgloss.NewStyle().Width(layout.contentWidth).Render(left + " " + rightColumn)
}

func (m Model) backgroundActivityLabel() string {
	switch {
	case m.backgroundAIActive():
		return "AI working"
	case m.backgroundJiraActive():
		return "syncing"
	}
	counts := m.recentBackgroundActivityCounts()
	switch {
	case counts.Errors > 0:
		return fmt.Sprintf("%d %s", counts.Errors, pluralize("error", counts.Errors))
	case counts.TicketUpdates > 0:
		return fmt.Sprintf("%d ticket %s", counts.TicketUpdates, pluralize("update", counts.TicketUpdates))
	case counts.Events > 0:
		return fmt.Sprintf("%d %s", counts.Events, pluralize("event", counts.Events))
	default:
		return ""
	}
}

func (m Model) renderBackgroundActivityLabel(label string) string {
	if strings.Contains(label, "error") {
		return m.theme.Error.Render(label)
	}
	return m.theme.Selected.Render(label)
}

func (m Model) planningSprintLabel() string {
	switch {
	case m.planningSprintsLoading:
		return "sprints loading"
	case m.planningSprintsErr != nil:
		return "sprints error"
	case m.planningSprintCount() > 0:
		count := m.planningSprintCount()
		return fmt.Sprintf("%d %s", count, pluralize("sprint", count))
	default:
		return ""
	}
}

func (m Model) planningSprintCount() int {
	count := 0
	for _, sprints := range m.planningSprints {
		count += len(sprints)
	}
	return count
}

func (m Model) backgroundAIActive() bool {
	return m.claudePlanLoading || m.claudeAssistLoading || m.createAIPromptLoading
}

func (m Model) backgroundJiraActive() bool {
	if m.loading || m.refreshing {
		return true
	}
	stats := m.workerStats()
	if stats.Running > 0 || stats.Pending > 0 {
		return true
	}
	return m.detailLoading ||
		m.commentsLoading ||
		m.commentSubmitting ||
		m.mentionSearchLoading ||
		m.assigneeSearchLoading ||
		m.assigneeSubmitting ||
		m.createIssueTypesLoading ||
		m.createFieldsLoading ||
		m.createSubmitting ||
		m.planningBoardsLoading ||
		m.planningSprintsLoading ||
		m.expandLoading ||
		m.transitionLoading ||
		m.transitionSubmitting ||
		m.summaryMetadataLoading ||
		m.summarySubmitting ||
		m.priorityMetadataLoading ||
		m.prioritySubmitting
}

type backgroundActivityCounts struct {
	Errors        int
	TicketUpdates int
	Events        int
}

func (m Model) recentBackgroundActivityCounts() backgroundActivityCounts {
	if len(m.diagnosticsEvents) == 0 {
		return backgroundActivityCounts{}
	}
	cutoff := m.now().Add(-backgroundActivityRecentWindow)
	var counts backgroundActivityCounts
	for _, event := range m.diagnosticsEvents {
		if event.At.IsZero() || event.At.Before(cutoff) {
			continue
		}
		if event.Status == "error" {
			counts.Errors++
			continue
		}
		if event.Kind == diagnosticKindEvent {
			if isTicketEventLabel(event.Label) {
				counts.TicketUpdates++
			} else {
				counts.Events++
			}
		}
	}
	return counts
}

func isTicketEventLabel(label string) bool {
	return label == "jira.ticket.new" || label == "jira.ticket.updated"
}

func pluralize(word string, count int) string {
	if count == 1 {
		return word
	}
	return word + "s"
}

func (m Model) viewFreshnessLabel() string {
	if m.lastSynced.IsZero() {
		return "not synced"
	}
	prefix := "synced "
	if m.err != nil && len(m.issues) > 0 {
		prefix = "refresh failed "
	} else if m.viewStale {
		prefix = "stale "
	}
	return prefix + m.lastSynced.Format("15:04:05")
}

func (m Model) renderQuery(layout browserLayout) string {
	label := m.theme.PaneTitle.Render("Filter")
	labelWidth := lipgloss.Width("Filter  ")
	queryWidth := max(20, layout.contentWidth-labelWidth-2)
	query := m.theme.Muted.Render(truncate(m.filterSummary(), queryWidth))
	return label + "  " + query
}

func (m Model) renderStatePanel(layout browserLayout, title string, body string) string {
	return m.theme.Panel.Width(layout.contentWidth).Render(m.theme.PaneTitle.Render(title) + "\n\n" + body)
}

func (m Model) renderFooterHelp(context keyContext, layout browserLayout) string {
	return m.renderFooterHelpWithBindings(context, footerBindings(context), layout)
}

func (m Model) renderModelFooterHelp(layout browserLayout) string {
	context := activeKeyContext(m)
	bindings := footerBindings(context)
	if context == keyContextDetail {
		bindings = m.detailFooterBindings()
	}
	if context == keyContextCreate {
		bindings = m.createFooterBindings()
	}
	return m.renderFooterHelpWithBindings(context, bindings, layout)
}

func (m Model) createFooterBindings() []keyBinding {
	bindings := []keyBinding{
		{Keys: []string{"?"}, Label: "help", Description: "Open the keyboard help screen.", Group: "Global", Footer: true},
		{Keys: []string{"ctrl+d"}, Label: "diagnostics", Description: "Open recent background worker and cache activity.", Group: "Global", Footer: true},
		{Keys: []string{"esc"}, Label: "cancel", Description: "Cancel ticket creation.", Group: "Create", Footer: true},
	}
	switch {
	case m.createIssueTypesLoading, m.createIssueTypesErr != nil:
		return bindings
	case len(m.createIssueTypes) == 0 && m.createIssueType.ID == "":
		return bindings
	case m.createIssueType.ID == "":
		if m.claudeCreateTicketDraftEnabled() {
			bindings = append(bindings, keyBinding{Keys: []string{"tab"}, Label: "mode", Description: "Switch between manual and AI generated ticket creation.", Group: "Create", Footer: true})
		}
		if m.createAIGeneratedMode {
			return append(bindings,
				keyBinding{Keys: []string{"ctrl+s"}, Label: "generate", Description: "Ask Claude to generate a local ticket draft.", Group: "Create", Footer: true},
			)
		}
		return append(bindings,
			keyBinding{Keys: []string{"up", "down", "j", "k"}, FooterKey: "j/k", Label: "type", Description: "Select an issue type while choosing ticket type.", Group: "Create", Footer: true},
			keyBinding{Keys: []string{"enter"}, Label: "continue", Description: "Continue with the selected issue type.", Group: "Create", Footer: true},
		)
	case m.createFieldsLoading, m.createFieldsErr != nil:
		return bindings
	default:
		return append(bindings,
			keyBinding{Keys: []string{"tab"}, Label: "field", Description: "Move between Summary and Description.", Group: "Create", Footer: true},
			keyBinding{Keys: []string{"ctrl+s"}, Label: "submit", Description: "Create the ticket.", Group: "Create", Footer: true},
		)
	}
}

func (m Model) renderFooterHelpWithBindings(context keyContext, bindings []keyBinding, layout browserLayout) string {
	available := max(20, layout.contentWidth)
	rendered := m.footerContextLabel(context, available)
	currentGroup := ""
	for _, binding := range bindings {
		next := m.renderFooterBindingHelp(binding)
		if currentGroup != "" && binding.Group != currentGroup {
			next = m.theme.Muted.Render("|") + m.theme.Muted.Render("  ") + next
		}
		candidate := next
		if rendered != "" {
			candidate = rendered + m.theme.Muted.Render("  ") + next
		}
		if lipgloss.Width(candidate) > available {
			break
		}
		rendered = candidate
		currentGroup = binding.Group
	}
	return rendered
}

func (m Model) renderFooterBindingHelp(binding keyBinding) string {
	help := bubbleshelp.New()
	help.ShortSeparator = "  "
	help.Ellipsis = ""
	help.Styles = plainBubbleHelpStyles()
	return help.ShortHelpView([]key.Binding{binding.bubbleKeyBinding()})
}

func plainBubbleHelpStyles() bubbleshelp.Styles {
	return bubbleshelp.Styles{}
}

func (m Model) detailFooterBindings() []keyBinding {
	base := footerBindings(keyContextDetail)
	if target, ok := m.focusedDetailTarget(); ok && target.Kind == detailTargetField {
		label := "edit"
		switch target.ID {
		case "status":
			label = "transition"
		case "priority":
			label = "priority"
		case "assignee":
			label = "assignee"
		case "summary":
			label = "summary"
		}
		fieldBindings := []keyBinding{
			{Keys: []string{"enter"}, Label: label, Group: "Field", Footer: true},
		}
		filtered := make([]keyBinding, 0, len(base)+len(fieldBindings))
		for _, binding := range base {
			if binding.keyText() == "enter" || binding.keyText() == "s" || binding.keyText() == "p" {
				continue
			}
			filtered = append(filtered, binding)
		}
		if len(filtered) == 0 {
			return fieldBindings
		}
		result := make([]keyBinding, 0, len(filtered)+len(fieldBindings))
		result = append(result, filtered[0])
		result = append(result, fieldBindings...)
		result = append(result, filtered[1:]...)
		return result
	}
	section, ok := m.focusedDetailSection()
	if !ok {
		return base
	}
	var sectionBindings []keyBinding
	switch section.ID {
	case "description":
		if m.inlineDescriptionAIAvailable() {
			sectionBindings = []keyBinding{
				{Keys: []string{"a"}, Label: "AI", Group: "Section", Footer: true},
			}
		}
	case "hierarchy":
		if len(m.currentHierarchyChildren()) > 0 {
			sectionBindings = []keyBinding{
				{Keys: []string{"j", "k", "up", "down"}, FooterKey: "j/k", Label: "child", Group: "Section", Footer: true},
				{Keys: []string{"enter"}, Label: "focus", Group: "Section", Footer: true},
			}
		}
	case "links":
		if len(m.currentDetailLinks()) > 0 {
			sectionBindings = []keyBinding{
				{Keys: []string{"j", "k", "up", "down"}, FooterKey: "j/k", Label: "link", Group: "Section", Footer: true},
				{Keys: []string{"enter"}, Label: "focus", Group: "Section", Footer: true},
				{Keys: []string{"y"}, Label: "copy", Group: "Section", Footer: true},
			}
		}
	case "comments":
		sectionBindings = []keyBinding{
			{Keys: []string{"enter"}, Label: "add", Group: "Section", Footer: true},
		}
	case "actions":
		sectionBindings = []keyBinding{
			{Keys: []string{"j", "k", "up", "down"}, FooterKey: "j/k", Label: "action", Group: "Section", Footer: true},
			{Keys: []string{"enter"}, Label: "focus", Group: "Section", Footer: true},
		}
	case "status":
		sectionBindings = []keyBinding{
			{Keys: []string{"enter"}, Label: "transition", Group: "Section", Footer: true},
		}
	}
	if len(sectionBindings) == 0 {
		return base
	}
	filtered := make([]keyBinding, 0, len(base)+len(sectionBindings))
	for _, binding := range base {
		if binding.FooterKey == "j/k" || binding.keyText() == "enter" {
			continue
		}
		filtered = append(filtered, binding)
	}
	result := make([]keyBinding, 0, len(filtered)+len(sectionBindings))
	if len(filtered) == 0 {
		return append(result, sectionBindings...)
	}
	result = append(result, filtered[0])
	result = append(result, sectionBindings...)
	result = append(result, filtered[1:]...)
	return result
}

func (m Model) footerContextLabel(context keyContext, width int) string {
	label := string(context)
	if context == keyContextTable {
		label = m.issueLayoutContextLabel()
	}
	if label == "" || lipgloss.Width(label)+2 > width {
		return ""
	}
	return m.theme.Muted.Render(label)
}

func (m Model) renderHelp(layout browserLayout) string {
	context := activeKeyContext(m)
	lines := m.helpLines(context)
	rows := m.helpRows()
	offset := clamp(m.helpOffset, 0, max(0, len(lines)-rows))
	end := min(len(lines), offset+rows)
	var b strings.Builder
	b.WriteString(m.detailSectionHeader("help", "Keyboard Help", m.keyContextDisplayLabel(context), max(32, layout.contentWidth-4)))
	b.WriteString("\n\n")
	if len(lines) > 0 {
		b.WriteString(strings.Join(lines[offset:end], "\n"))
	}
	if len(lines) > rows {
		indicator := m.theme.Muted.Render(fmt.Sprintf("Lines %d-%d of %d", offset+1, end, len(lines)))
		if offset > 0 {
			indicator += m.theme.Muted.Render("  PgUp previous")
		}
		if end < len(lines) {
			indicator += m.theme.Muted.Render("  PgDn next")
		}
		b.WriteString("\n")
		b.WriteString(truncate(indicator, layout.contentWidth-6))
	}
	return m.theme.ActivePane.Width(layout.contentWidth).Render(strings.TrimRight(b.String(), "\n"))
}

func (m Model) keyContextDisplayLabel(context keyContext) string {
	if context == keyContextTable {
		return m.issueLayoutContextLabel()
	}
	return string(context)
}

func (m Model) issueLayoutContextLabel() string {
	switch m.issueLayout {
	case issueLayoutWorkbench:
		return "Issue Workbench"
	case issueLayoutTable:
		return "Issue Table"
	default:
		return "Issue Lanes"
	}
}

func (m Model) helpLines(context keyContext) []string {
	bindings := append(helpBindings(), keyBindings(context)...)
	var lines []string
	currentGroup := ""
	for _, binding := range bindings {
		if binding.Group != currentGroup {
			if currentGroup != "" {
				lines = append(lines, "")
			}
			currentGroup = binding.Group
			lines = append(lines, m.theme.FieldLabel.Render(currentGroup))
		}
		adapted := binding.bubbleKeyBindingForFullHelp()
		help := adapted.Help()
		keys := m.theme.Key.Render(help.Key)
		padding := strings.Repeat(" ", max(1, 16-lipgloss.Width(help.Key)))
		lines = append(lines, keys+m.theme.Muted.Render(padding)+m.theme.Text.Render(help.Desc))
	}
	return lines
}

func (m Model) helpRows() int {
	return m.boundedPanelBodyRows(3)
}

func (m Model) boundedPanelBodyRows(reservedInsidePanel int) int {
	if m.height <= 0 {
		return 18
	}
	return max(1, m.height-appChromeRows-panelFrameRows-reservedInsidePanel)
}

func (m *Model) scrollHelp(delta int) {
	lines := m.helpLines(activeKeyContext(*m))
	m.helpOffset = clamp(m.helpOffset+delta, 0, max(0, len(lines)-m.helpRows()))
}

func (m Model) filterSummary() string {
	query := normalizeJQLForSummary(m.jql)
	var parts []string
	if project := jqlValueAfter(query, "project = "); project != "" {
		parts = append(parts, project)
	}
	switch {
	case strings.Contains(query, "assignee = currentuser()"):
		parts = append(parts, "assigned to me")
	case strings.Contains(query, "creator = currentuser()") && strings.Contains(query, "reporter = currentuser()"):
		parts = append(parts, "created/reported by me")
	case strings.Contains(query, "reporter = currentuser()"):
		parts = append(parts, "reported by me")
	case strings.Contains(query, "creator = currentuser()"):
		parts = append(parts, "created by me")
	case strings.Contains(query, "watcher = currentuser()"):
		parts = append(parts, "watching")
	}
	if strings.Contains(query, "sprint in opensprints()") {
		parts = append(parts, "current sprint")
	}
	if strings.Contains(query, "resolution = unresolved") {
		parts = append(parts, "unresolved")
	}
	if order := jqlOrderSummary(query); order != "" {
		parts = append(parts, order)
	}
	if len(parts) == 0 {
		return m.jql
	}
	return strings.Join(parts, " · ")
}

func normalizeJQLForSummary(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(value)), " ")
}

func jqlValueAfter(query string, marker string) string {
	index := strings.Index(query, marker)
	if index < 0 {
		return ""
	}
	value := strings.TrimSpace(query[index+len(marker):])
	if value == "" {
		return ""
	}
	end := len(value)
	for _, token := range []string{" and ", " or ", " order by ", ")"} {
		if tokenIndex := strings.Index(value, token); tokenIndex >= 0 && tokenIndex < end {
			end = tokenIndex
		}
	}
	return strings.ToUpper(strings.Trim(value[:end], " ()"))
}

func jqlOrderSummary(query string) string {
	index := strings.Index(query, " order by ")
	if index < 0 {
		return ""
	}
	order := strings.TrimSpace(query[index+len(" order by "):])
	switch {
	case strings.Contains(order, "updated desc"):
		return "updated desc"
	case strings.Contains(order, "updated asc"):
		return "updated asc"
	case strings.Contains(order, "priority desc"):
		return "priority desc"
	case order != "":
		return "sorted"
	default:
		return ""
	}
}

func (m Model) sortLabel() string {
	switch m.sort {
	case sortPriority:
		return "priority"
	case sortStatus:
		return "status"
	case sortAssignee:
		return "assignee"
	case sortType:
		return "type"
	case sortKey:
		return "key"
	default:
		return "jira"
	}
}

func (m Model) pageIndicator(start, end, total int) string {
	left := ""
	right := ""
	if start > 0 {
		left = "PgUp previous"
	}
	if end < total {
		right = "PgDn next"
	}
	switch {
	case left != "" && right != "":
		return left + "  " + right
	case left != "":
		return left
	case right != "":
		return right
	default:
		return ""
	}
}

func (m Model) browserLayout(width int) browserLayout {
	if width <= 0 {
		width = 100
	}
	contentWidth := max(42, width-6)
	return browserLayout{
		contentWidth: contentWidth,
		listWidth:    contentWidth,
		rows:         m.tableRows(),
	}
}

func (m Model) tableRows() int {
	if m.height <= 0 {
		return 10
	}

	// Header, query, footer, panel borders, padding, title, and table header all
	// consume vertical space outside the viewport-backed issue rows.
	reserved := 15
	if m.height >= ui.MinTerminalHeight && m.height-reserved < minUsefulIssueRows && !ui.TerminalTooSmall(m.width, m.height) {
		reserved--
	}
	return max(1, m.height-reserved)
}

func (m Model) useCompactIssueListChrome(layout browserLayout) bool {
	return layout.rows <= minUsefulIssueRows
}
