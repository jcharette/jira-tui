package tui

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"charm.land/bubbles/v2/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/jcharette/jira-tui/internal/worker"
)

type issueDisplayRow struct {
	issue          jira.Issue
	parent         *jira.Issue
	parentVisible  bool
	index          int
	childCount     int
	hiddenChildren int
}

type issueListColumns struct {
	width         int
	gutterWidth   int
	typeWidth     int
	keyWidth      int
	statusWidth   int
	priorityWidth int
	assigneeWidth int
	showStatus    bool
	showPriority  bool
	showAssignee  bool
	summaryWidth  int
}

type issueLaneGroup struct {
	Status string
	Count  int
	Lines  []issueRenderLine
}

type issueDisplayTree struct {
	issues          []jira.Issue
	roots           []issueDisplayRoot
	children        map[string][]int
	indexByKey      map[string]int
	missingParents  map[string]missingParentGroup
	missingParentOf map[string]bool
}

type issueRenderLine struct {
	text       string
	issueIndex int
}

type issueDisplayRoot struct {
	issueIndex       int
	missingParentKey string
}

type missingParentGroup struct {
	key      string
	summary  string
	children []int
}

type issueSymbolMode string

const (
	symbolModeAuto    issueSymbolMode = "auto"
	symbolModePlain   issueSymbolMode = "plain"
	symbolModeSymbols issueSymbolMode = "symbols"
	symbolModeEmoji   issueSymbolMode = "emoji"
	symbolModeNerd    issueSymbolMode = "nerd"
)

type issueSymbols struct {
	Epic      string
	Story     string
	Task      string
	Bug       string
	Subtask   string
	Issue     string
	Collapsed string
	Expanded  string
}

func (m Model) renderIssueWorkspace(layout browserLayout) string {
	if m.issueLayout == issueLayoutWorkbench {
		return m.renderIssueWorkbench(layout)
	}
	if m.issueLayout == issueLayoutLanes {
		return m.renderIssueLanes(layout)
	}
	return m.renderIssueList(layout)
}

func (m Model) renderIssueWorkbench(layout browserLayout) string {
	if layout.listWidth < 128 {
		return m.renderIssueList(layout)
	}
	panelWidth := 38
	gapWidth := 2
	leftWidth := max(72, layout.listWidth-panelWidth-gapWidth)
	if leftWidth+panelWidth+gapWidth > layout.listWidth {
		return m.renderIssueList(layout)
	}
	leftLayout := layout
	leftLayout.listWidth = leftWidth
	left := m.renderIssueList(leftLayout)
	right := m.renderSelectedIssueContextPanel(panelWidth)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gapWidth), right)
}

func (m Model) renderSelectedIssueContextPanel(width int) string {
	bodyWidth := max(20, width-4)
	var lines []string
	lines = append(lines, m.detailSectionHeader("issue-context", "Context", "", bodyWidth))
	selected, ok := m.selectedIssue()
	if !ok {
		lines = append(lines, "", m.detailEmptyState("No issue selected.", bodyWidth))
		return m.theme.Panel.Width(width).Render(strings.Join(lines, "\n"))
	}
	lines = append(lines, "", m.renderSelectedIssueSelectionContext(selected, bodyWidth))
	lines = append(lines, "", m.renderSelectedIssueLatestContext(selected.Key, bodyWidth))
	lines = append(lines, "", m.renderSelectedIssueHierarchyContext(selected, bodyWidth))
	if detail := m.renderSelectedIssueDetailContext(selected.Key, bodyWidth); detail != "" {
		lines = append(lines, "", detail)
	}
	lines = append(lines, "", m.detailEmptyState("enter open detail  . actions", bodyWidth))
	return m.theme.Panel.Width(width).Render(strings.Join(lines, "\n"))
}

func (m Model) renderSelectedIssueSelectionContext(issue jira.Issue, width int) string {
	header := m.theme.Muted.Render("Selected")
	visible := m.currentVisibleIssueIndexes(m.browserLayout(m.width))
	position := -1
	for index, issueIndex := range visible {
		if issueIndex == m.selected {
			position = index
			break
		}
	}
	var parts []string
	if position >= 0 && len(visible) > 0 {
		parts = append(parts, fmt.Sprintf("%d of %d", position+1, len(visible)))
	}
	issueType := displayValue(issue.IssueType, "Issue")
	if issue.IsSubtask {
		issueType = "Subtask"
	}
	parts = append(parts, issueType)
	if issue.SubtaskCount > 0 {
		parts = append(parts, fmt.Sprintf("%d reported subtasks", issue.SubtaskCount))
	}
	if len(parts) == 0 {
		return header + "\n" + m.detailEmptyState("Loaded row context unavailable.", width)
	}
	return header + "\n" + m.theme.Text.Render(truncate(strings.Join(parts, "  "), width))
}

func (m Model) renderSelectedIssueLatestContext(key string, width int) string {
	header := m.theme.Muted.Render("Latest")
	if comments, ok := m.comments[key]; ok {
		if len(comments) == 0 {
			return header + "\n" + m.detailEmptyState("No comments yet.", width)
		}
		comment := comments[len(comments)-1]
		body := singleLine(comment.Body)
		if body == "" {
			body = "Comment has no text."
		}
		return header + "\n" + m.theme.Text.Render(shortName(comment.Author)+": "+truncate(body, max(18, width-4)))
	}
	if m.commentsLoading && m.commentsRequestKey == key {
		return header + "\n" + m.detailStatusBlock("Loading comments...", width, false)
	}
	if m.commentsErr != nil && m.commentsRequestKey == key {
		return header + "\n" + m.detailStatusBlock("Comments failed: "+m.commentsErr.Error(), width, true)
	}
	return header + "\n" + m.detailEmptyState("Comments not loaded.", width)
}

func (m Model) renderSelectedIssueHierarchyContext(issue jira.Issue, width int) string {
	header := m.theme.Muted.Render("Hierarchy")
	var parts []string
	if issue.ParentKey != "" {
		parts = append(parts, "parent "+issue.ParentKey)
	}
	children := m.currentHierarchyChildrenFor(issue.Key)
	if len(children) == 1 {
		parts = append(parts, "1 loaded child")
	} else if len(children) > 1 {
		parts = append(parts, fmt.Sprintf("%d loaded children", len(children)))
	}
	if len(parts) == 0 {
		return header + "\n" + m.detailEmptyState("No loaded hierarchy.", width)
	}
	return header + "\n" + m.theme.Text.Render(truncate(strings.Join(parts, "  "), width))
}

func (m Model) renderSelectedIssueDetailContext(key string, width int) string {
	detail, ok := m.details[key]
	if !ok {
		return ""
	}
	description := singleLine(detail.Description)
	if description == "" {
		return ""
	}
	return m.theme.Muted.Render("Description") + "\n" + m.theme.Text.Render(truncate(description, max(18, width-4)))
}

func (m Model) currentHierarchyChildrenFor(key string) []jira.Issue {
	if key == "" {
		return nil
	}
	rows := m.hierarchyRows(key)
	children := make([]jira.Issue, 0, len(rows))
	for _, row := range rows {
		children = append(children, row.Issue)
	}
	return children
}

func (m Model) renderIssueList(layout browserLayout) string {
	var b strings.Builder
	rowCount := layout.rows
	rows := m.issueRows(layout)
	start := clamp(m.offset, 0, max(0, len(rows)-rowCount))
	end := min(len(rows), start+rowCount)

	b.WriteString(m.issueListHeader(layout))
	b.WriteByte('\n')
	if len(rows) > 0 {
		vp := viewport.New(
			viewport.WithWidth(max(1, layout.listWidth-4)),
			viewport.WithHeight(max(1, rowCount)),
		)
		vp.SoftWrap = false
		vp.FillHeight = false
		vp.SetContent(strings.Join(rows, "\n"))
		vp.SetYOffset(start)
		b.WriteString(strings.TrimRight(vp.View(), "\n "))
	} else if len(m.issues) > 0 && m.statusFilter == issueStatusFilterActive {
		b.WriteString(m.theme.Muted.Render("Active filter hides all loaded issues. Press f show all."))
	} else {
		b.WriteString(m.theme.Muted.Render("No issues found for this view."))
	}

	title := m.issueListTitle(len(rows), rowCount, start, end)
	content := m.issueListTitleLine(title, layout) + "\n" + strings.TrimRight(b.String(), "\n")
	if len(rows) > rowCount {
		content += "\n" + m.theme.Muted.Render(m.pageIndicator(start, end, len(rows)))
	}
	return m.theme.ActivePane.Width(layout.listWidth).Render(content)
}

func (m Model) renderIssueLanes(layout browserLayout) string {
	lines := m.issueLaneRenderLines(layout)
	rows := issueRenderRows(lines)
	if len(rows) == 0 {
		if len(m.issues) > 0 && m.statusFilter == issueStatusFilterActive {
			rows = append(rows, m.theme.Muted.Render("Active filter hides all loaded issues. Press f show all."))
		} else {
			rows = append(rows, m.theme.Muted.Render("No issues found for this view."))
		}
	}
	rowCount := layout.rows
	start := clamp(m.offset, 0, max(0, len(rows)-rowCount))
	end := min(len(rows), start+rowCount)
	vp := viewport.New(
		viewport.WithWidth(max(1, layout.listWidth-4)),
		viewport.WithHeight(max(1, rowCount)),
	)
	vp.SoftWrap = false
	vp.FillHeight = false
	vp.SetContent(strings.Join(rows, "\n"))
	vp.SetYOffset(start)

	visibleCount := len(m.currentVisibleIssueIndexes(layout))
	title := m.issueListTitle(visibleCount, rowCount, start, end)
	content := m.issueListTitleLine(title, layout) + "\n" + strings.TrimRight(vp.View(), "\n ")
	if len(rows) > rowCount {
		content += "\n" + m.theme.Muted.Render(m.pageIndicator(start, end, len(rows)))
	}
	return m.theme.ActivePane.Width(layout.listWidth).Render(content)
}

func (m Model) issueLaneRenderLines(layout browserLayout) []issueRenderLine {
	var lines []issueRenderLine
	groups := m.issueLaneGroups(layout)
	for groupIndex, group := range groups {
		if groupIndex > 0 {
			lines = append(lines, issueRenderLine{issueIndex: -1})
		}
		header := displayValue(group.Status, "Unknown")
		lines = append(lines, issueRenderLine{
			text:       statusStyle(m.theme, group.Status).Render(fmt.Sprintf("%s %d", header, group.Count)),
			issueIndex: -1,
		})
		lines = append(lines, group.Lines...)
	}
	return lines
}

func (m Model) issueLaneGroups(layout browserLayout) []issueLaneGroup {
	_ = layout
	displayTree := buildIssueDisplayTree(m.issues)
	groups := make([]issueLaneGroup, 0)
	groupByStatus := make(map[string]int)
	for _, root := range displayTree.roots {
		lines := m.issueLaneRootRenderLines(displayTree, root, layout)
		status, count, ok := m.issueLaneGroupMetrics(displayTree, lines)
		if !ok {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(status))
		groupIndex, ok := groupByStatus[key]
		if !ok {
			groupIndex = len(groups)
			groupByStatus[key] = groupIndex
			groups = append(groups, issueLaneGroup{Status: status})
		}
		groups[groupIndex].Count += count
		groups[groupIndex].Lines = append(groups[groupIndex].Lines, lines...)
	}
	sort.SliceStable(groups, func(left, right int) bool {
		leftRank := issueLaneStatusRank(groups[left].Status)
		rightRank := issueLaneStatusRank(groups[right].Status)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return strings.ToLower(groups[left].Status) < strings.ToLower(groups[right].Status)
	})
	return groups
}

func (m Model) issueLaneRootRenderLines(displayTree issueDisplayTree, root issueDisplayRoot, layout browserLayout) []issueRenderLine {
	if root.missingParentKey != "" {
		return m.missingParentRenderLines(displayTree, root.missingParentKey, layout)
	}
	return m.issueTreeRenderLines(displayTree, root.issueIndex, "", true, layout)
}

func (m Model) issueLaneGroupMetrics(displayTree issueDisplayTree, lines []issueRenderLine) (string, int, bool) {
	status := ""
	count := 0
	for _, line := range lines {
		if line.issueIndex < 0 || line.issueIndex >= len(displayTree.issues) {
			continue
		}
		count++
		if status == "" {
			status = displayValue(displayTree.issues[line.issueIndex].Status, "Unknown")
		}
	}
	return status, count, count > 0
}

func issueLaneStatusRank(status string) int {
	normalized := strings.Join(strings.Fields(strings.ToLower(status)), " ")
	switch {
	case normalized == "":
		return 50
	case strings.Contains(normalized, "progress"), strings.Contains(normalized, "doing"), strings.Contains(normalized, "implement"):
		return 10
	case strings.Contains(normalized, "block"), strings.Contains(normalized, "impediment"):
		return 20
	case strings.Contains(normalized, "review"), strings.Contains(normalized, "test"), strings.Contains(normalized, "qa"), strings.Contains(normalized, "verify"):
		return 30
	case strings.Contains(normalized, "ready"), strings.Contains(normalized, "todo"), strings.Contains(normalized, "to do"), strings.Contains(normalized, "open"), strings.Contains(normalized, "backlog"):
		return 40
	case terminalIssueStatus(status):
		return 90
	default:
		return 50
	}
}

func (m *Model) cycleIssueLayoutMode() {
	switch m.issueLayout {
	case issueLayoutTable:
		m.issueLayout = issueLayoutWorkbench
	case issueLayoutWorkbench:
		m.issueLayout = issueLayoutLanes
	default:
		m.issueLayout = issueLayoutTable
	}
	m.detailNotice = "Layout: " + m.issueLayoutModeLabel() + "."
	detail := fmt.Sprintf(
		"layout=%s view=%s include_children=%t issues=%d",
		diagnosticToken(m.issueLayoutModeLabel()),
		diagnosticToken(m.activeViewName()),
		m.activeViewIncludeChildren(),
		len(m.issues),
	)
	m.recordDiagnosticEvent(diagnosticKindState, "layout", "change", detail)
}

func (m Model) issueLayoutModeLabel() string {
	switch m.issueLayout {
	case issueLayoutWorkbench:
		return "Workbench"
	case issueLayoutLanes:
		return "Lanes"
	default:
		return "Table"
	}
}

func (m Model) issueListTitle(rowTotal int, rowCount int, start int, end int) string {
	title := "0 issues"
	if len(m.issues) > 0 {
		title = fmt.Sprintf("%d issues", len(m.issues))
		if m.statusFilter == issueStatusFilterActive {
			title += fmt.Sprintf("  Active %d shown", rowTotal)
		}
		if rowTotal > rowCount {
			title += fmt.Sprintf("  rows %d-%d", start+1, end)
		}
	}
	return title
}

func (m Model) issueListTitleLine(title string, layout browserLayout) string {
	line := m.issueListControlStrip(title, layout)
	if m.useCompactIssueListChrome(layout) {
		return line
	}
	return line + "\n"
}

func (m Model) issueListControlStrip(title string, layout browserLayout) string {
	parts := []string{
		m.issueListChip("View", m.activeViewName(), false),
		m.issueListChip("Filter", m.activeStatusFilterLabel(), m.statusFilter == issueStatusFilterActive),
		m.issueListChip("Layout", m.issueLayoutModeLabel(), m.issueLayout != issueLayoutTable),
		m.issueListChip("Sort", titleCase(m.sortLabel()), m.sort != sortJira),
		m.theme.PaneTitle.Render(title),
		m.theme.Muted.Render(m.viewFreshnessLabel()),
	}
	if planning := m.planningSprintLabel(); planning != "" {
		parts = append(parts, m.theme.Muted.Render(planning))
	}
	for len(parts) > 0 {
		line := strings.Join(parts, m.theme.Muted.Render("  "))
		if lipgloss.Width(line) <= layout.listWidth {
			return line
		}
		parts = parts[:len(parts)-1]
	}
	return ""
}

func (m Model) issueListChip(label string, value string, active bool) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "-"
	}
	style := m.theme.Text
	if active {
		style = m.theme.Selected
	}
	return m.theme.Muted.Render(label+" ") + style.Render(value)
}

func titleCase(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func singleLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func (m Model) issueListHeader(layout browserLayout) string {
	columns := m.issueListColumns(layout)
	left := fmt.Sprintf("%s %s %-*s  %s", strings.Repeat(" ", columns.gutterWidth), padRight("T", columns.typeWidth), columns.keyWidth, "KEY", "SUMMARY")
	right := m.issueListMetaHeader(layout)
	spacer := max(1, columns.width-lipgloss.Width(left)-lipgloss.Width(right))
	return m.theme.Muted.Render(left + strings.Repeat(" ", spacer) + right)
}

func (m Model) issueListMetaHeader(layout browserLayout) string {
	columns := m.issueListColumns(layout)
	return m.issueListMetaPlain(columns, "STATUS", "PRI", "OWNER")
}

func terminalIssueStatus(status string) bool {
	normalized := strings.Join(strings.Fields(strings.ToLower(status)), " ")
	if normalized == "" || normalized == "unknown" {
		return false
	}
	for _, terminal := range []string{"done", "closed", "resolved", "canceled", "cancelled"} {
		if strings.Contains(normalized, terminal) {
			return true
		}
	}
	return false
}

func (m Model) issueVisibleByStatus(issue jira.Issue) bool {
	if m.statusFilter != issueStatusFilterActive {
		return true
	}
	return !terminalIssueStatus(issue.Status)
}

func (m Model) activeStatusFilterLabel() string {
	if m.statusFilter == issueStatusFilterActive {
		return "Active"
	}
	return "All"
}

func (m Model) issueRows(layout browserLayout) []string {
	lines := m.issueRenderLines(layout)
	return issueRenderRows(lines)
}

func issueRenderRows(lines []issueRenderLine) []string {
	rows := make([]string, 0, len(lines))
	for _, line := range lines {
		rows = append(rows, line.text)
	}
	return rows
}

func (m Model) currentIssueRenderLines(layout browserLayout) []issueRenderLine {
	if m.issueLayout == issueLayoutLanes {
		return m.issueLaneRenderLines(layout)
	}
	return m.issueRenderLines(layout)
}

func (m Model) currentVisibleIssueIndexes(layout browserLayout) []int {
	lines := m.currentIssueRenderLines(layout)
	indexes := make([]int, 0, len(lines))
	seen := make(map[int]bool, len(lines))
	for _, line := range lines {
		if line.issueIndex < 0 || line.issueIndex >= len(m.issues) || seen[line.issueIndex] {
			continue
		}
		seen[line.issueIndex] = true
		indexes = append(indexes, line.issueIndex)
	}
	return indexes
}

func (m Model) issueRenderLines(layout browserLayout) []issueRenderLine {
	displayTree := buildIssueDisplayTree(m.issues)
	var lines []issueRenderLine
	for _, root := range displayTree.roots {
		if root.missingParentKey != "" {
			lines = append(lines, m.missingParentRenderLines(displayTree, root.missingParentKey, layout)...)
			continue
		}
		lines = append(lines, m.issueTreeRenderLines(displayTree, root.issueIndex, "", true, layout)...)
	}
	return lines
}

func buildIssueDisplayTree(issues []jira.Issue) issueDisplayTree {
	children := make(map[string][]int)
	indexByKey := make(map[string]int, len(issues))
	seen := make(map[string]bool, len(issues))
	for index, issue := range issues {
		seen[issue.Key] = true
		indexByKey[issue.Key] = index
	}
	var roots []issueDisplayRoot
	missingParents := make(map[string]missingParentGroup)
	missingParentOf := make(map[string]bool)
	seenMissingParent := make(map[string]bool)
	for index, issue := range issues {
		if issue.ParentKey != "" {
			if seen[issue.ParentKey] {
				children[issue.ParentKey] = append(children[issue.ParentKey], index)
				continue
			}
			group := missingParents[issue.ParentKey]
			group.key = issue.ParentKey
			if group.summary == "" {
				group.summary = issue.ParentSummary
			}
			group.children = append(group.children, index)
			missingParents[issue.ParentKey] = group
			missingParentOf[issue.ParentKey] = true
			if !seenMissingParent[issue.ParentKey] {
				roots = append(roots, issueDisplayRoot{missingParentKey: issue.ParentKey})
				seenMissingParent[issue.ParentKey] = true
			}
			continue
		}
		roots = append(roots, issueDisplayRoot{issueIndex: index})
	}
	return issueDisplayTree{
		issues:          issues,
		roots:           roots,
		children:        children,
		indexByKey:      indexByKey,
		missingParents:  missingParents,
		missingParentOf: missingParentOf,
	}
}

func (m Model) missingParentRows(displayTree issueDisplayTree, parentKey string, layout browserLayout) []string {
	lines := m.missingParentRenderLines(displayTree, parentKey, layout)
	rows := make([]string, 0, len(lines))
	for _, line := range lines {
		rows = append(rows, line.text)
	}
	return rows
}

func (m Model) missingParentRenderLines(displayTree issueDisplayTree, parentKey string, layout browserLayout) []issueRenderLine {
	group := displayTree.missingParents[parentKey]
	var childLines []issueRenderLine
	for index, child := range group.children {
		childLines = append(childLines, m.issueTreeRenderLines(displayTree, child, "  ", index == len(group.children)-1, layout)...)
	}
	if len(childLines) == 0 {
		return nil
	}
	label := "Parent outside view: " + parentKey
	if group.summary != "" {
		label += "  " + group.summary
	}
	gutterWidth := issueTreeGutterWidth(layout)
	lines := []issueRenderLine{{
		text:       m.theme.Muted.Render(padRight("  ◇", gutterWidth) + truncate(label, max(20, layout.listWidth-gutterWidth-2))),
		issueIndex: -1,
	}}
	lines = append(lines, childLines...)
	return lines
}

func (m Model) issueTreeRows(displayTree issueDisplayTree, index int, prefix string, last bool, layout browserLayout) []string {
	lines := m.issueTreeRenderLines(displayTree, index, prefix, last, layout)
	rows := make([]string, 0, len(lines))
	for _, line := range lines {
		rows = append(rows, line.text)
	}
	return rows
}

func (m Model) issueTreeRenderLines(displayTree issueDisplayTree, index int, prefix string, last bool, layout browserLayout) []issueRenderLine {
	row := displayTree.issueRow(index)
	visible := m.issueVisibleByStatus(row.issue)
	hiddenDescendants := 0
	if visible && m.issueCollapsed(row.issue.Key) {
		hiddenDescendants = m.loadedDescendantCount(displayTree, row.issue.Key)
	}
	var lines []issueRenderLine
	if visible {
		gutter := issueTreeGutter(prefix, last, index == m.selected, layout)
		label := m.renderIssueDisplayRowWithHidden(row, gutter, hiddenDescendants, m.issueCollapsed(row.issue.Key), layout)
		if index == m.selected {
			label = m.theme.Selected.Render(label)
			if detail := m.selectedIssueListDetail(row, layout); detail != "" {
				label += "\n" + m.theme.Muted.Render(padRight("", issueTreeGutterWidth(layout)+issueTypeColumnWidth+1)+detail)
			}
		}
		rawRows := strings.Split(label, "\n")
		lines = make([]issueRenderLine, 0, len(rawRows))
		for lineIndex, raw := range rawRows {
			issueIndex := index
			if lineIndex > 0 {
				issueIndex = -1
			}
			lines = append(lines, issueRenderLine{text: raw, issueIndex: issueIndex})
		}
	}
	if hiddenDescendants > 0 {
		return lines
	}
	children := displayTree.children[row.issue.Key]
	nextPrefix := nextIssueTreePrefix(prefix, last, len(children))
	for childPosition, childIndex := range children {
		lines = append(lines, m.issueTreeRenderLines(displayTree, childIndex, nextPrefix, childPosition == len(children)-1, layout)...)
	}
	return lines
}

func nextIssueTreePrefix(prefix string, last bool, childCount int) string {
	nextPrefix := prefix
	if prefix != "" || childCount > 0 {
		if last {
			nextPrefix += "  "
		} else {
			nextPrefix += "│ "
		}
	}
	return nextPrefix
}

func (m Model) visibleIssueIndexes(displayTree issueDisplayTree) []int {
	indexes := make([]int, 0, len(displayTree.issues))
	for _, root := range displayTree.roots {
		if root.missingParentKey != "" {
			for _, child := range displayTree.missingParents[root.missingParentKey].children {
				indexes = m.appendVisibleIssueIndexes(indexes, displayTree, child)
			}
			continue
		}
		indexes = m.appendVisibleIssueIndexes(indexes, displayTree, root.issueIndex)
	}
	return indexes
}

func (m Model) appendVisibleIssueIndexes(indexes []int, displayTree issueDisplayTree, index int) []int {
	issue := displayTree.issues[index]
	visible := m.issueVisibleByStatus(issue)
	if visible {
		indexes = append(indexes, index)
	}
	if visible && m.issueCollapsed(issue.Key) {
		return indexes
	}
	for _, child := range displayTree.children[issue.Key] {
		indexes = m.appendVisibleIssueIndexes(indexes, displayTree, child)
	}
	return indexes
}

func (m Model) issueCollapsed(key string) bool {
	return key != "" && m.collapsedIssueKeys != nil && m.collapsedIssueKeys[key]
}

func (m Model) loadedDescendantCount(displayTree issueDisplayTree, issueKey string) int {
	count := 0
	var walk func(string)
	walk = func(key string) {
		for _, child := range displayTree.children[key] {
			count++
			walk(displayTree.issues[child].Key)
		}
	}
	walk(issueKey)
	return count
}

func issueTreeGutter(prefix string, last bool, selected bool, layout browserLayout) string {
	cursor := " "
	if selected {
		cursor = "➜"
	}
	if prefix == "" {
		return padRight(cursor, issueTreeRootGutter)
	}
	connector := prefix
	if last {
		connector += "╰─"
	} else {
		connector += "├─"
	}
	return cursor + fitLeft(connector, issueTreeGutterWidth(layout)-2) + " "
}

func (t issueDisplayTree) issueRow(index int) issueDisplayRow {
	issue := t.issues[index]
	childIndexes := t.children[issue.Key]
	hiddenChildren := issue.SubtaskCount - len(childIndexes)
	if hiddenChildren < 0 {
		hiddenChildren = 0
	}
	row := issueDisplayRow{
		issue:          issue,
		index:          index,
		childCount:     len(childIndexes),
		hiddenChildren: hiddenChildren,
	}
	if parentIndex, ok := t.indexByKey[issue.ParentKey]; ok {
		parent := t.issues[parentIndex]
		row.parent = &parent
		row.parentVisible = true
	}
	if t.missingParentOf[issue.ParentKey] {
		row.parentVisible = true
	}
	return row
}

func (m Model) renderIssueDisplayRow(row issueDisplayRow, gutter string, layout browserLayout) string {
	issue := row.issue
	columns := m.issueListColumns(layout)
	gutter = fitIssueTreeGutter(gutter, layout)
	kind, kindWidth := m.issueKindColumn(row, false, columns)
	keyText := truncate(issue.Key, columns.keyWidth)
	key := m.theme.Key.Render(fmt.Sprintf("%-*s", columns.keyWidth, keyText))
	statusText := truncate(issue.Status, columns.statusWidth)
	if statusText == "" {
		statusText = "Unknown"
	}
	status := statusStyle(m.theme, issue.Status).Render(fmt.Sprintf("%-*s", columns.statusWidth, statusText))
	priorityText := priorityBadge(issue.Priority)
	priority := priorityStyle(m.theme, issue.Priority).Render(fmt.Sprintf("%-*s", columns.priorityWidth, truncate(priorityText, columns.priorityWidth)))
	assigneeText := truncate(shortName(displayValue(issue.Assignee, "Unassigned")), columns.assigneeWidth)
	assignee := m.theme.Muted.Render(fmt.Sprintf("%-*s", columns.assigneeWidth, assigneeText))
	right := m.issueListMeta(columns, status, priority, assignee)

	leftPlain := fmt.Sprintf("%s %s %-*s  ", gutter, fitRight(kind, kindWidth), columns.keyWidth, keyText)
	summaryWidth := max(12, columns.width-lipgloss.Width(leftPlain)-lipgloss.Width(m.issueListMetaPlain(columns, statusText, truncate(priorityText, columns.priorityWidth), assigneeText))-1)
	summary := truncate(issue.Summary, summaryWidth)
	if summary == "" {
		summary = "(no summary)"
	}
	left := fmt.Sprintf("%s %s %s  %s", m.theme.Muted.Render(gutter), m.issueKindStyle(row.issue).Render(kind), key, summary)
	spacer := max(1, columns.width-lipgloss.Width(left)-lipgloss.Width(right))
	return left + strings.Repeat(" ", spacer) + right
}

func (m Model) renderIssueDisplayRowWithHidden(row issueDisplayRow, gutter string, hiddenDescendants int, collapsed bool, layout browserLayout) string {
	issue := row.issue
	columns := m.issueListColumns(layout)
	gutter = fitIssueTreeGutter(gutter, layout)
	kind, kindWidth := m.issueKindColumn(row, collapsed, columns)
	keyText := truncate(issue.Key, columns.keyWidth)
	key := m.theme.Key.Render(fmt.Sprintf("%-*s", columns.keyWidth, keyText))
	statusText := truncate(issue.Status, columns.statusWidth)
	if statusText == "" {
		statusText = "Unknown"
	}
	status := statusStyle(m.theme, issue.Status).Render(fmt.Sprintf("%-*s", columns.statusWidth, statusText))
	priorityText := priorityBadge(issue.Priority)
	priority := priorityStyle(m.theme, issue.Priority).Render(fmt.Sprintf("%-*s", columns.priorityWidth, truncate(priorityText, columns.priorityWidth)))
	assigneeText := truncate(shortName(displayValue(issue.Assignee, "Unassigned")), columns.assigneeWidth)
	assignee := m.theme.Muted.Render(fmt.Sprintf("%-*s", columns.assigneeWidth, assigneeText))
	right := m.issueListMeta(columns, status, priority, assignee)

	leftPlain := fmt.Sprintf("%s %s %-*s  ", gutter, fitRight(kind, kindWidth), columns.keyWidth, keyText)
	summaryWidth := max(12, columns.width-lipgloss.Width(leftPlain)-lipgloss.Width(m.issueListMetaPlain(columns, statusText, truncate(priorityText, columns.priorityWidth), assigneeText))-1)
	summary := issue.Summary
	if hiddenDescendants > 0 {
		summary = hiddenDescendantSummary(issue.Summary, hiddenDescendants, summaryWidth)
	} else {
		summary = truncate(summary, summaryWidth)
	}
	if summary == "" {
		summary = "(no summary)"
	}
	left := fmt.Sprintf("%s %s %s  %s", m.theme.Muted.Render(gutter), m.issueKindStyle(row.issue).Render(kind), key, summary)
	spacer := max(1, columns.width-lipgloss.Width(left)-lipgloss.Width(right))
	return left + strings.Repeat(" ", spacer) + right
}

func (m Model) issueKindColumn(row issueDisplayRow, collapsed bool, columns issueListColumns) (string, int) {
	kind := m.issueKindSymbol(row.issue) + m.issueHierarchySymbol(row, collapsed)
	width := max(columns.typeWidth, lipgloss.Width(kind))
	return fitRight(kind, width), width
}

func hiddenDescendantSummary(summary string, hiddenDescendants int, width int) string {
	hiddenLabel := fmt.Sprintf("%d hidden", hiddenDescendants)
	if width <= lipgloss.Width(hiddenLabel) {
		return hiddenLabel
	}
	summary = truncate(summary, max(0, width-lipgloss.Width(hiddenLabel)-2))
	if summary == "" {
		return hiddenLabel
	}
	return summary + "  " + hiddenLabel
}

func fitIssueTreeGutter(gutter string, layout browserLayout) string {
	maxWidth := issueTreeGutterWidth(layout)
	if lipgloss.Width(gutter) < issueTreeRootGutter {
		return padRight(gutter, issueTreeRootGutter)
	}
	if lipgloss.Width(gutter) > maxWidth {
		return fitRight(gutter, maxWidth)
	}
	return gutter
}

func issueTreeGutterWidth(layout browserLayout) int {
	switch {
	case layout.listWidth < 76:
		return 6
	case layout.listWidth < 90:
		return 8
	default:
		return issueTreeMaxGutter
	}
}

func (m Model) issueListColumns(layout browserLayout) issueListColumns {
	width := max(40, layout.listWidth-4)
	gutterWidth := issueTreeRootGutter
	typeWidth := issueTypeColumnWidth
	keyWidth := 12
	statusWidth := 14
	priorityWidth := 4
	assigneeWidth := 14
	showStatus := true
	showPriority := true
	showAssignee := layout.listWidth >= 96

	switch {
	case layout.listWidth < 64:
		keyWidth = 8
		statusWidth = 8
		showPriority = false
		showAssignee = false
	case layout.listWidth < 76:
		keyWidth = 10
		statusWidth = 10
		showPriority = false
		showAssignee = false
	case layout.listWidth < 90:
		keyWidth = 10
		statusWidth = 12
		showAssignee = false
	}

	rightPlain := m.issueListMetaPlain(issueListColumns{
		statusWidth:   statusWidth,
		priorityWidth: priorityWidth,
		assigneeWidth: assigneeWidth,
		showStatus:    showStatus,
		showPriority:  showPriority,
		showAssignee:  showAssignee,
	}, strings.Repeat("S", statusWidth), strings.Repeat("P", priorityWidth), strings.Repeat("O", assigneeWidth))
	leftWidth := gutterWidth + 1 + typeWidth + 1 + keyWidth + 2
	summaryWidth := max(12, width-leftWidth-lipgloss.Width(rightPlain)-1)
	return issueListColumns{
		width:         width,
		gutterWidth:   gutterWidth,
		typeWidth:     typeWidth,
		keyWidth:      keyWidth,
		statusWidth:   statusWidth,
		priorityWidth: priorityWidth,
		assigneeWidth: assigneeWidth,
		showStatus:    showStatus,
		showPriority:  showPriority,
		showAssignee:  showAssignee,
		summaryWidth:  summaryWidth,
	}
}

func (m Model) issueListMeta(columns issueListColumns, status, priority, assignee string) string {
	var parts []string
	if columns.showStatus {
		parts = append(parts, status)
	}
	if columns.showPriority {
		parts = append(parts, priority)
	}
	if columns.showAssignee {
		parts = append(parts, assignee)
	}
	return strings.Join(parts, " ")
}

func (m Model) issueListMetaPlain(columns issueListColumns, status, priority, assignee string) string {
	var parts []string
	if columns.showStatus {
		parts = append(parts, fmt.Sprintf("%-*s", columns.statusWidth, truncate(status, columns.statusWidth)))
	}
	if columns.showPriority {
		parts = append(parts, fmt.Sprintf("%-*s", columns.priorityWidth, truncate(priority, columns.priorityWidth)))
	}
	if columns.showAssignee {
		parts = append(parts, fmt.Sprintf("%-*s", columns.assigneeWidth, truncate(assignee, columns.assigneeWidth)))
	}
	return strings.Join(parts, " ")
}

func (m Model) selectedIssueListDetail(row issueDisplayRow, layout browserLayout) string {
	issue := row.issue
	width := max(40, layout.listWidth-4)
	var parts []string
	if issue.ParentKey != "" && !row.parentVisible {
		parts = append(parts, "parent "+issue.ParentKey)
	}
	if row.childCount > 0 && row.hiddenChildren > 0 {
		parts = append(parts, fmt.Sprintf("%d children", row.childCount))
	}
	if row.hiddenChildren > 0 {
		parts = append(parts, fmt.Sprintf("+%d hidden", row.hiddenChildren))
	}
	if issue.SubtaskCount > 0 && row.childCount == 0 {
		parts = append(parts, fmt.Sprintf("%d subtasks reported", issue.SubtaskCount))
	}
	if len(parts) == 0 {
		return ""
	}
	line := m.theme.Muted.Render(strings.Join(parts, "  |  "))
	return truncate(line, width)
}

func (m Model) issueKindSymbol(issue jira.Issue) string {
	symbols := m.issueSymbols()
	normalized := strings.ToLower(issue.IssueType)
	switch {
	case strings.Contains(normalized, "epic"):
		return symbols.Epic
	case issue.IsSubtask || strings.Contains(normalized, "sub-task") || strings.Contains(normalized, "subtask"):
		return symbols.Subtask
	case strings.Contains(normalized, "story"):
		return symbols.Story
	case strings.Contains(normalized, "bug"):
		return symbols.Bug
	case strings.Contains(normalized, "enhancement"):
		return m.issueTypeSymbol("✦", "✨")
	case strings.Contains(normalized, "task"):
		return symbols.Task
	default:
		if badge := issueTypeBadge(issue.IssueType); badge != "" {
			return badge
		}
		return symbols.Issue
	}
}

func (m Model) issueKindStyle(issue jira.Issue) lipgloss.Style {
	normalized := strings.ToLower(issue.IssueType)
	switch {
	case strings.Contains(normalized, "epic"):
		return m.theme.Selected
	case strings.Contains(normalized, "story"):
		return m.theme.Key
	case strings.Contains(normalized, "enhancement"):
		return m.theme.Warning
	case strings.Contains(normalized, "bug"):
		return m.theme.Error
	case issue.IsSubtask || strings.Contains(normalized, "sub-task") || strings.Contains(normalized, "subtask"):
		return m.theme.Muted
	case strings.Contains(normalized, "task"):
		return m.theme.Success
	default:
		return m.theme.Muted
	}
}

func (m Model) issueSymbols() issueSymbols {
	switch m.effectiveSymbolMode() {
	case symbolModePlain:
		return issueSymbols{Epic: "EP", Story: "ST", Task: "TK", Bug: "BG", Subtask: "SU", Issue: "IS", Collapsed: "+", Expanded: "-"}
	case symbolModeEmoji:
		return issueSymbols{Epic: "🟣", Story: "🟦", Task: "🟨", Bug: "🐞", Subtask: "🔹", Issue: "•", Collapsed: "▸", Expanded: "▾"}
	case symbolModeNerd:
		return issueSymbols{Epic: "", Story: "", Task: "", Bug: "", Subtask: "◇", Issue: "", Collapsed: "▸", Expanded: "▾"}
	default:
		return issueSymbols{Epic: "◈", Story: "▣", Task: "●", Bug: "!", Subtask: "◇", Issue: "•", Collapsed: "▸", Expanded: "▾"}
	}
}

func (m Model) issueHierarchySymbol(row issueDisplayRow, collapsed bool) string {
	if collapsed {
		return m.issueSymbols().Collapsed
	}
	if row.childCount > 0 {
		return m.issueSymbols().Expanded
	}
	if row.hiddenChildren > 0 || row.issue.SubtaskCount > 0 || m.issueHasCachedChildren(row.issue.Key) {
		return "▸"
	}
	return " "
}

func (m Model) issueHasCachedChildren(key string) bool {
	if key == "" {
		return false
	}
	for _, mode := range []worker.ExpandMode{worker.ExpandModeOpen, worker.ExpandModeAll} {
		record, ok := m.cachedExpandedChildren(key, mode)
		if ok && len(record.Value) > 0 {
			return true
		}
	}
	return false
}

func (m Model) issueTypeSymbol(symbolsValue string, emojiValue string) string {
	if m.effectiveSymbolMode() == symbolModeEmoji {
		return emojiValue
	}
	if m.effectiveSymbolMode() == symbolModeNerd {
		return ""
	}
	return symbolsValue
}

func issueTypeBadge(issueType string) string {
	words := strings.FieldsFunc(strings.TrimSpace(issueType), func(r rune) bool {
		return r == ' ' || r == '-' || r == '_' || r == '/' || r == '\\'
	})
	if len(words) == 0 {
		return ""
	}
	var initials []rune
	for _, word := range words {
		for _, r := range word {
			initials = append(initials, r)
			break
		}
		if len(initials) == 2 {
			break
		}
	}
	if len(initials) == 1 {
		runes := []rune(words[0])
		if len(runes) > 1 {
			initials = append(initials, runes[1])
		}
	}
	return strings.ToUpper(string(initials))
}

func (m Model) effectiveSymbolMode() issueSymbolMode {
	mode := m.symbolMode
	if mode == "" || mode == symbolModeAuto {
		return detectSymbolMode()
	}
	return mode
}

func detectSymbolMode() issueSymbolMode {
	term := strings.ToLower(os.Getenv("TERM"))
	lang := strings.ToLower(os.Getenv("LC_ALL") + os.Getenv("LC_CTYPE") + os.Getenv("LANG"))
	if term == "dumb" || (!strings.Contains(lang, "utf-8") && !strings.Contains(lang, "utf8")) {
		return symbolModePlain
	}
	if strings.EqualFold(os.Getenv("TERM_PROGRAM"), "iTerm.app") && strings.TrimSpace(os.Getenv("ITERM_PROFILE")) != "" {
		return symbolModeNerd
	}
	return symbolModeSymbols
}
