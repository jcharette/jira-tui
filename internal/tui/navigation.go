package tui

import (
	"sort"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/jcharette/jira-tui/internal/worker"
)

func (m Model) startRefresh() (Model, tea.Cmd) {
	return m.startRefreshWithCache(false, worker.PriorityRefresh)
}

func (m Model) startCachedRefresh(priority worker.Priority) (Model, tea.Cmd) {
	return m.startRefreshWithCache(true, priority)
}

func (m Model) startRefreshWithCache(useCache bool, priority worker.Priority) (Model, tea.Cmd) {
	if useCache {
		if record, ok := m.cachedActiveIssueView(m.jql); ok {
			fresh := m.activeIssueViewCacheFresh(record)
			m.applyActiveIssueView(record, !fresh)
			if fresh {
				m.recordDiagnosticEvent(diagnosticKindCache, "active_view", "hit", m.activeViewName())
				return m, nil
			}
			m.recordDiagnosticEvent(diagnosticKindCache, "active_view", "stale", m.activeViewName())
		} else {
			m.recordDiagnosticEvent(diagnosticKindCache, "active_view", "miss", m.activeViewName())
		}
	} else if record, ok := m.cachedActiveIssueView(m.jql); ok {
		fresh := m.activeIssueViewCacheFresh(record)
		m.applyActiveIssueView(record, !fresh)
		m.recordDiagnosticEvent(diagnosticKindCache, "active_view", "refresh", m.activeViewName())
	}
	m.nextRequestID++
	m.activeRequestID = m.nextRequestID
	m.expandLoading = false
	m.expandRequestKey = ""
	if len(m.issues) == 0 {
		m.loading = true
	} else {
		m.refreshing = true
	}
	return m, m.submitIssueSearch(m.activeRequestID, priority)
}

func (m Model) startExpandSelectedIssue(mode worker.ExpandMode) (Model, tea.Cmd) {
	issue, ok := m.selectedIssue()
	if !ok || issue.Key == "" {
		m.detailNotice = "No issue selected."
		return m, nil
	}
	label := "open children"
	if mode == worker.ExpandModeAll {
		label = "all children"
	}
	m.hydrateExpandedChildren(issue.Key, mode)
	if record, cached := m.cachedExpandedChildren(issue.Key, mode); cached && record.Fresh(m.currentTime()) {
		added := m.mergeExpandedIssues(record.Value)
		if added == 0 {
			m.detailNotice = "No new " + label + " found for " + issue.Key + "."
			return m, nil
		}
		m.detailNotice = "Loaded " + strconv.Itoa(added) + " " + label + " for " + issue.Key + "."
		m.ensureSelectionVisible(m.currentLayoutRows())
		return m, nil
	}
	m.nextRequestID++
	m.activeExpandReqID = m.nextRequestID
	m.expandRequestKey = issue.Key
	m.expandMode = mode
	m.expandLoading = true
	m.detailNotice = "Loading " + label + " for " + issue.Key + "."
	return m, m.submitExpandIssues(m.activeExpandReqID, issue.Key, mode)
}

func (m *Model) toggleSelectedIssueCollapse() {
	issue, ok := m.selectedIssue()
	if !ok || issue.Key == "" {
		m.detailNotice = "No issue selected."
		return
	}
	displayTree := buildIssueDisplayTree(m.issues)
	descendants := m.loadedDescendantCount(displayTree, issue.Key)
	if descendants == 0 {
		m.detailNotice = "No loaded child issues for " + issue.Key + "."
		return
	}
	if m.collapsedIssueKeys == nil {
		m.collapsedIssueKeys = make(map[string]bool)
	}
	if m.collapsedIssueKeys[issue.Key] {
		delete(m.collapsedIssueKeys, issue.Key)
		m.detailNotice = "Expanded " + issue.Key + "."
	} else {
		m.collapsedIssueKeys[issue.Key] = true
		m.detailNotice = "Collapsed " + issue.Key + "."
	}
	m.repairCollapsedSelection()
	m.ensureSelectionVisible(m.currentLayoutRows())
}

func (m Model) switchView(delta int) (Model, tea.Cmd) {
	if len(m.views) == 0 {
		return m, nil
	}
	m.view = (m.view + delta + len(m.views)) % len(m.views)
	m.jql = m.views[m.view].JQL
	m.selected = 0
	m.offset = 0
	m.statusFilter = issueStatusFilterAll
	m.issues = nil
	m.err = nil
	m.mode = modeTable
	m.loading = true
	m.refreshing = false
	m.viewStale = false
	m.expandLoading = false
	m.expandRequestKey = ""
	m.detailNotice = ""
	return m.startCachedRefresh(worker.PriorityForeground)
}

func (m *Model) mergeExpandedIssues(children []jira.Issue) int {
	if len(children) == 0 {
		return 0
	}
	selectedKey := ""
	if selected, ok := m.selectedIssue(); ok {
		selectedKey = selected.Key
	}
	seen := make(map[string]bool, len(m.issues)+len(children))
	for _, issue := range m.issues {
		seen[issue.Key] = true
	}
	added := 0
	for _, child := range children {
		if child.Key == "" || seen[child.Key] {
			continue
		}
		m.issues = append(m.issues, child)
		seen[child.Key] = true
		added++
	}
	if added == 0 {
		return 0
	}
	m.issues = orderIssues(m.issues, m.sort)
	if selectedKey != "" {
		for index, issue := range m.issues {
			if issue.Key == selectedKey {
				m.selected = index
				break
			}
		}
	}
	return added
}

func (m *Model) switchSort(delta int) {
	sortCount := int(sortKey) + 1
	m.sort = sortMode((int(m.sort) + delta + sortCount) % sortCount)
	selectedKey := ""
	if len(m.issues) > 0 && m.selected >= 0 && m.selected < len(m.issues) {
		selectedKey = m.issues[m.selected].Key
	}
	m.issues = orderIssues(m.issues, m.sort)
	if selectedKey != "" {
		for index, issue := range m.issues {
			if issue.Key == selectedKey {
				m.selected = index
				break
			}
		}
	}
	m.ensureSelectionVisible(m.currentLayoutRows())
}

func (m *Model) toggleStatusFilter() {
	if m.statusFilter == issueStatusFilterActive {
		m.statusFilter = issueStatusFilterAll
		m.detailNotice = "Showing all loaded issues."
	} else {
		m.statusFilter = issueStatusFilterActive
		m.detailNotice = "Showing active loaded issues."
	}
	m.repairStatusFilterSelection()
	m.ensureSelectionVisible(m.currentLayoutRows())
}

func (m *Model) repairStatusFilterSelection() {
	if len(m.issues) == 0 {
		m.selected = 0
		m.offset = 0
		return
	}
	displayTree := buildIssueDisplayTree(m.issues)
	visible := m.visibleIssueIndexes(displayTree)
	if len(visible) == 0 {
		m.selected = 0
		m.offset = 0
		return
	}
	for _, index := range visible {
		if index == m.selected {
			return
		}
	}
	for _, index := range visible {
		if index >= m.selected {
			m.selected = index
			return
		}
	}
	m.selected = visible[len(visible)-1]
}

func (m Model) activeViewName() string {
	if len(m.views) == 0 || m.view < 0 || m.view >= len(m.views) {
		return "Default"
	}
	return m.views[m.view].Name
}

func (m *Model) moveSelection(delta int) {
	if len(m.issues) == 0 {
		m.selected = 0
		m.offset = 0
		return
	}
	displayTree := buildIssueDisplayTree(m.issues)
	visible := m.visibleIssueIndexes(displayTree)
	if len(visible) == 0 {
		m.selected = 0
		m.offset = 0
		return
	}
	position := m.visibleSelectionPosition(visible)
	position = clamp(position+delta, 0, len(visible)-1)
	m.selected = visible[position]
	m.resetDetailScroll()
	m.ensureSelectionVisible(m.currentLayoutRows())
}

func (m *Model) pageSelection(delta int) {
	if len(m.issues) == 0 {
		m.selected = 0
		m.offset = 0
		return
	}
	displayTree := buildIssueDisplayTree(m.issues)
	visible := m.visibleIssueIndexes(displayTree)
	if len(visible) == 0 {
		m.selected = 0
		m.offset = 0
		return
	}
	rows := m.currentLayoutRows()
	step := max(1, rows-1)
	position := m.visibleSelectionPosition(visible)
	position = clamp(position+(delta*step), 0, len(visible)-1)
	m.selected = visible[position]
	m.resetDetailScroll()
	m.ensureSelectionVisible(rows)
}

func (m Model) visibleSelectionPosition(visible []int) int {
	if len(visible) == 0 {
		return 0
	}
	for position, index := range visible {
		if index == m.selected {
			return position
		}
	}
	return 0
}

func (m *Model) scrollDetail(delta int) {
	content := m.currentDetailContent()
	rows := max(1, m.fullDetailRows()-1)
	width := m.currentDetailBodyWidth()
	vp := m.newDetailViewport(content, width, rows)
	if delta > 0 {
		vp.ScrollDown(delta)
	} else if delta < 0 {
		vp.ScrollUp(-delta)
	}
	m.detailOffset = vp.YOffset()
	m.saveDetailSectionOffset()
}

func (m *Model) pageDetail(delta int) {
	content := m.currentDetailContent()
	rows := max(1, m.fullDetailRows()-1)
	width := m.currentDetailBodyWidth()
	vp := m.newDetailViewport(content, width, rows)
	if delta > 0 {
		vp.PageDown()
	} else if delta < 0 {
		vp.PageUp()
	}
	m.detailOffset = vp.YOffset()
	m.saveDetailSectionOffset()
}

func (m *Model) scrollDetailToBottom() {
	content := m.currentDetailContent()
	rows := max(1, m.fullDetailRows()-1)
	width := m.currentDetailBodyWidth()
	vp := m.newDetailViewport(content, width, rows)
	vp.GotoBottom()
	m.detailOffset = vp.YOffset()
	m.saveDetailSectionOffset()
}

func (m *Model) ensureSelectionVisible(rows int) {
	rows = max(1, rows)
	lines := m.issueRenderLines(m.browserLayout(m.width))
	selectedRow := m.selectedRenderedLineIndex(lines)
	maxOffset := max(0, len(lines)-rows)
	if selectedRow < m.offset {
		m.offset = selectedRow
	}
	if selectedRow >= m.offset+rows {
		m.offset = selectedRow - rows + 1
	}
	m.offset = clamp(m.offset, 0, maxOffset)
}

func (m Model) selectedRenderedLineIndex(lines []issueRenderLine) int {
	if len(m.issues) == 0 || m.selected < 0 || m.selected >= len(m.issues) {
		return 0
	}
	for index, line := range lines {
		if line.issueIndex == m.selected {
			return index
		}
	}
	return clamp(m.selected, 0, max(0, len(lines)-1))
}

func (m Model) currentLayoutRows() int {
	width := m.width
	if width <= 0 {
		width = 100
	}
	return m.browserLayout(width).rows
}

func (m Model) selectedIssue() (jira.Issue, bool) {
	if len(m.issues) == 0 || m.selected < 0 || m.selected >= len(m.issues) {
		return jira.Issue{}, false
	}
	return m.issues[m.selected], true
}

func (m *Model) repairCollapsedSelection() {
	if len(m.issues) == 0 {
		m.selected = 0
		m.offset = 0
		return
	}
	displayTree := buildIssueDisplayTree(m.issues)
	visible := m.visibleIssueIndexes(displayTree)
	for _, index := range visible {
		if index == m.selected {
			return
		}
	}
	if m.selected >= 0 && m.selected < len(m.issues) {
		parentKey := m.issues[m.selected].ParentKey
		for parentKey != "" {
			parentIndex, ok := displayTree.indexByKey[parentKey]
			if !ok {
				break
			}
			if m.issueCollapsed(parentKey) {
				m.selected = parentIndex
				return
			}
			parentKey = displayTree.issues[parentIndex].ParentKey
		}
	}
	if len(visible) > 0 {
		m.selected = visible[0]
		return
	}
	m.selected = 0
}

func (m *Model) replaceIssues(issues []jira.Issue) {
	selectedKey := ""
	if len(m.issues) > 0 && m.selected >= 0 && m.selected < len(m.issues) {
		selectedKey = m.issues[m.selected].Key
	}

	m.issues = orderIssues(issues, m.sort)
	if len(m.issues) == 0 {
		m.selected = 0
		m.offset = 0
		return
	}

	if selectedKey != "" {
		for index, issue := range m.issues {
			if issue.Key == selectedKey {
				m.selected = index
				m.repairCollapsedSelection()
				m.ensureSelectionVisible(m.currentLayoutRows())
				return
			}
		}
	}
	m.selected = clamp(m.selected, 0, len(m.issues)-1)
	m.repairCollapsedSelection()
	m.ensureSelectionVisible(m.currentLayoutRows())
}

func (m *Model) updateIssueStatus(key string, status string) {
	if key == "" || status == "" {
		return
	}
	for index := range m.issues {
		if m.issues[index].Key == key {
			m.issues[index].Status = status
			break
		}
	}
	if detail, ok := m.details[key]; ok {
		detail.Status = status
		detail.Issue.Status = status
		m.details[key] = detail
	}
	m.patchRetainedIssueDetail(key, func(detail *jira.IssueDetail) {
		detail.Status = status
		detail.Issue.Status = status
	})
	m.patchCurrentActiveViewIssue(key, func(issue *jira.Issue) {
		issue.Status = status
	})
	m.invalidateIssueTransitions(key)
}

func (m *Model) updateIssuePriority(key string, priority string) {
	if key == "" || priority == "" {
		return
	}
	for index := range m.issues {
		if m.issues[index].Key == key {
			m.issues[index].Priority = priority
			break
		}
	}
	if detail, ok := m.details[key]; ok {
		detail.Priority = priority
		detail.Issue.Priority = priority
		m.details[key] = detail
	}
	m.patchRetainedIssueDetail(key, func(detail *jira.IssueDetail) {
		detail.Priority = priority
		detail.Issue.Priority = priority
	})
	m.patchCurrentActiveViewIssue(key, func(issue *jira.Issue) {
		issue.Priority = priority
	})
}

func (m *Model) updateIssueAssignee(key string, assignee string) {
	if key == "" || assignee == "" {
		return
	}
	for index := range m.issues {
		if m.issues[index].Key == key {
			m.issues[index].Assignee = assignee
			break
		}
	}
	if detail, ok := m.details[key]; ok {
		detail.Assignee = assignee
		detail.Issue.Assignee = assignee
		m.details[key] = detail
	}
	m.patchRetainedIssueDetail(key, func(detail *jira.IssueDetail) {
		detail.Assignee = assignee
		detail.Issue.Assignee = assignee
	})
	m.patchCurrentActiveViewIssue(key, func(issue *jira.Issue) {
		issue.Assignee = assignee
	})
}

func (m *Model) updateIssueSummary(key string, summary string) {
	if key == "" || summary == "" {
		return
	}
	for index := range m.issues {
		if m.issues[index].Key == key {
			m.issues[index].Summary = summary
			break
		}
	}
	if detail, ok := m.details[key]; ok {
		detail.Summary = summary
		detail.Issue.Summary = summary
		m.details[key] = detail
	}
	m.patchRetainedIssueDetail(key, func(detail *jira.IssueDetail) {
		detail.Summary = summary
		detail.Issue.Summary = summary
	})
	m.patchCurrentActiveViewIssue(key, func(issue *jira.Issue) {
		issue.Summary = summary
	})
}

func (m *Model) updateIssueDescription(key string, description string) {
	if key == "" {
		return
	}
	if detail, ok := m.details[key]; ok {
		detail.Description = description
		m.details[key] = detail
	}
	m.patchRetainedIssueDetail(key, func(detail *jira.IssueDetail) {
		detail.Description = description
	})
}

func (m *Model) patchRetainedIssueDetail(key string, mutate func(*jira.IssueDetail)) {
	key = strings.TrimSpace(key)
	if key == "" || mutate == nil {
		return
	}
	record, ok := m.cachedIssueDetail(key)
	if !ok {
		return
	}
	detail := record.Value
	mutate(&detail)
	m.cacheIssueDetail(key, detail, record.SyncedAt)
}

func (m *Model) patchCurrentActiveViewIssue(key string, mutate func(*jira.Issue)) {
	key = strings.TrimSpace(key)
	if key == "" || mutate == nil {
		return
	}
	record, ok := m.cachedActiveIssueView(m.jql)
	if !ok {
		return
	}
	changed := false
	issues := append([]jira.Issue(nil), record.Issues...)
	for index := range issues {
		if issues[index].Key == key {
			mutate(&issues[index])
			changed = true
			break
		}
	}
	if !changed {
		return
	}
	m.cacheActiveIssueView(m.jql, issues, record.SyncedAt)
}

func (m *Model) invalidateIssueTransitions(key string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	if m.transitions != nil {
		delete(m.transitions, key)
	}
	if m.transitionsCache != nil {
		m.transitionsCache.Delete(key)
	}
	m.deletePersistentIssueTransitions(key)
}

func orderIssues(issues []jira.Issue, mode sortMode) []jira.Issue {
	byParent := make(map[string][]jira.Issue)
	topLevel := make([]jira.Issue, 0, len(issues))
	seen := make(map[string]bool, len(issues))
	for _, issue := range issues {
		seen[issue.Key] = true
	}
	for _, issue := range issues {
		if issue.ParentKey != "" && seen[issue.ParentKey] {
			byParent[issue.ParentKey] = append(byParent[issue.ParentKey], issue)
			continue
		}
		topLevel = append(topLevel, issue)
	}

	sortIssueGroup(topLevel, mode)
	for parent := range byParent {
		sortIssueGroup(byParent[parent], mode)
	}

	ordered := make([]jira.Issue, 0, len(issues))
	for _, issue := range topLevel {
		ordered = append(ordered, issue)
		ordered = append(ordered, byParent[issue.Key]...)
	}
	return ordered
}

func sortIssueGroup(issues []jira.Issue, mode sortMode) {
	if mode == sortJira {
		return
	}
	sort.SliceStable(issues, func(i, j int) bool {
		left := issues[i]
		right := issues[j]
		switch mode {
		case sortPriority:
			if priorityRank(left.Priority) != priorityRank(right.Priority) {
				return priorityRank(left.Priority) > priorityRank(right.Priority)
			}
		case sortStatus:
			if left.Status != right.Status {
				return left.Status < right.Status
			}
		case sortAssignee:
			if left.Assignee != right.Assignee {
				return left.Assignee < right.Assignee
			}
		case sortType:
			if left.IssueType != right.IssueType {
				return left.IssueType < right.IssueType
			}
		case sortKey:
			if left.Key != right.Key {
				return left.Key < right.Key
			}
		}
		return left.Key < right.Key
	})
}
