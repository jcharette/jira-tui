package tui

import (
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/jon/jira-tui/internal/jira"
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
	Epic    string
	Story   string
	Task    string
	Bug     string
	Subtask string
	Issue   string
}

func (m Model) renderIssueWorkspace(layout browserLayout) string {
	return m.renderIssueList(layout)
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
	line := m.theme.PaneTitle.Render(title) + " " + m.theme.Muted.Render("sorted by "+m.sortLabel())
	if m.useCompactIssueListChrome(layout) {
		return line
	}
	return line + "\n"
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
	rows := make([]string, 0, len(lines))
	for _, line := range lines {
		rows = append(rows, line.text)
	}
	return rows
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
		label := m.renderIssueDisplayRowWithHidden(row, gutter, hiddenDescendants, layout)
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
		cursor = "▌"
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
	kind := m.issueKindSymbol(issue)
	columns := m.issueListColumns(layout)
	gutter = fitIssueTreeGutter(gutter, layout)
	kind = fitRight(kind, columns.typeWidth)
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

	leftPlain := fmt.Sprintf("%s %s %-*s  ", gutter, kind, columns.keyWidth, keyText)
	summaryWidth := max(12, columns.width-lipgloss.Width(leftPlain)-lipgloss.Width(m.issueListMetaPlain(columns, statusText, truncate(priorityText, columns.priorityWidth), assigneeText))-1)
	summary := truncate(issue.Summary, summaryWidth)
	if summary == "" {
		summary = "(no summary)"
	}
	left := fmt.Sprintf("%s %s %s  %s", m.theme.Muted.Render(gutter), m.theme.Muted.Render(kind), key, summary)
	spacer := max(1, columns.width-lipgloss.Width(left)-lipgloss.Width(right))
	return left + strings.Repeat(" ", spacer) + right
}

func (m Model) renderIssueDisplayRowWithHidden(row issueDisplayRow, gutter string, hiddenDescendants int, layout browserLayout) string {
	if hiddenDescendants <= 0 {
		return m.renderIssueDisplayRow(row, gutter, layout)
	}
	row.issue.Summary = strings.TrimSpace(row.issue.Summary + "  " + fmt.Sprintf("%d hidden", hiddenDescendants))
	return m.renderIssueDisplayRow(row, gutter, layout)
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
	case strings.Contains(normalized, "task"):
		return symbols.Task
	default:
		return symbols.Issue
	}
}

func (m Model) issueSymbols() issueSymbols {
	switch m.effectiveSymbolMode() {
	case symbolModePlain:
		return issueSymbols{Epic: "E", Story: "S", Task: "T", Bug: "B", Subtask: "-", Issue: "*"}
	case symbolModeEmoji:
		return issueSymbols{Epic: "🟣", Story: "🟦", Task: "🟨", Bug: "🐞", Subtask: "↳", Issue: "•"}
	case symbolModeNerd:
		return issueSymbols{Epic: "◆", Story: "󰧭", Task: "󰄬", Bug: "", Subtask: "↳", Issue: "•"}
	default:
		return issueSymbols{Epic: "◆", Story: "■", Task: "●", Bug: "!", Subtask: "↳", Issue: "•"}
	}
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
	return symbolModeSymbols
}
