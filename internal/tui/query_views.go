package tui

import (
	"fmt"
	"strings"

	"github.com/jcharette/jira-tui/internal/config"
)

func (m Model) renderQueryTemplates(width int) []string {
	templates := m.queryViewTemplates()
	if len(templates) == 0 {
		return []string{m.theme.Muted.Render("No project found in the current JQL.")}
	}
	rows := make([]string, 0, len(templates))
	for i, view := range templates {
		prefix := "  "
		style := m.theme.Text
		if i == m.queryTemplateSelected {
			prefix = "> "
			style = m.theme.Selected
		}
		label := fmt.Sprintf("%s%s -> %s", prefix, view.Name, view.JQL)
		rows = append(rows, style.Render(truncate(label, width)))
	}
	return rows
}

func (m Model) renderSavedViews(width int) []string {
	if len(m.views) == 0 {
		return []string{m.theme.Muted.Render("No saved views configured.")}
	}
	rows := make([]string, 0, len(m.views))
	for i, view := range m.views {
		prefix := "  "
		style := m.theme.Text
		if i == m.queryViewSelected {
			prefix = "> "
			style = m.theme.Selected
		}
		children := ""
		if view.IncludeChildren {
			children = " children"
		}
		label := fmt.Sprintf("%s%s%s -> %s", prefix, view.Name, children, view.JQL)
		rows = append(rows, style.Render(truncate(label, width)))
	}
	return rows
}

func (m *Model) openQuerySaveViewPromptForCurrentMode() {
	switch m.queryMode {
	case queryModeAI:
		jql := strings.TrimSpace(m.queryGeneratedJQL)
		if jql == "" {
			m.detailNotice = "Generate JQL before saving an AI view."
			return
		}
		name := strings.TrimSpace(m.queryGeneratedPrompt)
		if name == "" {
			name = "AI Query"
		}
		m.openQuerySaveViewPrompt(config.IssueView{Name: name, JQL: jql}, querySaveViewActionAdd, -1)
	case queryModeTemplates:
		templates := m.queryViewTemplates()
		if len(templates) == 0 {
			m.detailNotice = "No template project found."
			return
		}
		index := min(max(0, m.queryTemplateSelected), len(templates)-1)
		m.openQuerySaveViewPrompt(templates[index], querySaveViewActionAdd, -1)
	case queryModeRecent:
		record, ok := m.selectedQueryHistoryRecord()
		if !ok {
			m.detailNotice = "No recent queries yet."
			return
		}
		name := strings.TrimSpace(record.Prompt)
		if name == "" {
			name = "Recent Query"
		}
		m.openQuerySaveViewPrompt(config.IssueView{Name: name, JQL: record.JQL}, querySaveViewActionAdd, -1)
	default:
		jql := strings.TrimSpace(m.queryJQLValue())
		if jql == "" {
			m.detailNotice = "JQL cannot be empty."
			return
		}
		m.openQuerySaveViewPrompt(config.IssueView{Name: suggestQueryViewName(jql), JQL: jql}, querySaveViewActionAdd, -1)
	}
}

func (m *Model) openSelectedSavedViewRenamePrompt() {
	if len(m.views) == 0 || m.queryViewSelected < 0 || m.queryViewSelected >= len(m.views) {
		m.detailNotice = "No saved views configured."
		return
	}
	m.openQuerySaveViewPrompt(m.views[m.queryViewSelected], querySaveViewActionRename, m.queryViewSelected)
}

func (m *Model) openQuerySaveViewPrompt(view config.IssueView, action querySaveViewAction, index int) {
	view.Name = strings.TrimSpace(view.Name)
	view.JQL = strings.TrimSpace(view.JQL)
	if view.Name == "" {
		view.Name = suggestQueryViewName(view.JQL)
	}
	m.querySaveViewOpen = true
	m.querySaveViewJQL = view.JQL
	m.querySaveViewIncludeChildren = view.IncludeChildren
	m.querySaveViewAction = action
	m.querySaveViewIndex = index
	m.setQuerySaveViewName(view.Name)
	m.detailNotice = ""
}

func (m *Model) openCurrentQuerySaveViewPrompt() {
	view := config.IssueView{
		Name:            suggestQueryViewName(m.jql),
		JQL:             m.jql,
		IncludeChildren: m.activeViewIncludeChildren(),
	}
	if len(m.views) > 0 && m.view >= 0 && m.view < len(m.views) {
		view.Name = m.views[m.view].Name + " Copy"
	}
	m.openQuerySaveViewPrompt(view, querySaveViewActionAdd, -1)
}

func (m Model) querySaveViewPromptTitle() string {
	if m.querySaveViewAction == querySaveViewActionRename {
		return "Rename Saved View"
	}
	return "Save View"
}

func (m Model) queryViewTemplates() []config.IssueView {
	project := projectKeyFromJQL(m.queryJQLDraft)
	if project == "" {
		project = projectKeyFromJQL(m.jql)
	}
	return config.DefaultViews(project)
}

func suggestQueryViewName(jql string) string {
	jql = strings.ToLower(strings.TrimSpace(jql))
	switch {
	case strings.Contains(jql, "assignee = currentuser()"):
		return "My Work"
	case strings.Contains(jql, "sprint in opensprints()"):
		return "Current Sprint"
	case strings.Contains(jql, "watcher = currentuser()"):
		return "Watching"
	case strings.Contains(jql, "issuetype = epic"):
		return "Epics"
	default:
		return "Custom Query"
	}
}

func (m *Model) loadSelectedSavedViewForReview() {
	if len(m.views) == 0 || m.queryViewSelected < 0 || m.queryViewSelected >= len(m.views) {
		m.detailNotice = "No saved views configured."
		return
	}
	m.queryMode = queryModeJQL
	m.setQueryJQLDraft(m.views[m.queryViewSelected].JQL)
	m.detailNotice = "Saved view loaded for review."
}

func (m Model) persistSavedViews(views []config.IssueView, activeView string) (Model, bool) {
	cfg, err := config.SetSavedViews(config.Config{Views: m.views, ActiveView: activeView}, views)
	if err != nil {
		m.detailNotice = err.Error()
		return m, false
	}
	if m.savedViewsWriter == nil {
		m.detailNotice = "Saved-view persistence is not available."
		return m, false
	}
	if err := m.savedViewsWriter(cfg.Views, cfg.ActiveView); err != nil {
		m.detailNotice = "Saved view failed: " + err.Error()
		return m, false
	}
	m.views = cfg.Views
	m.view = -1
	for index, view := range m.views {
		if strings.EqualFold(view.Name, cfg.ActiveView) {
			m.view = index
			break
		}
	}
	if m.view < 0 && len(m.views) > 0 {
		m.view = 0
	}
	return m, true
}

func (m *Model) deleteSelectedSavedView() {
	if len(m.views) <= 1 {
		m.detailNotice = "At least one saved view is required."
		return
	}
	if m.queryViewSelected < 0 || m.queryViewSelected >= len(m.views) {
		m.detailNotice = "No saved views configured."
		return
	}
	views := append([]config.IssueView(nil), m.views[:m.queryViewSelected]...)
	views = append(views, m.views[m.queryViewSelected+1:]...)
	next, ok := m.persistSavedViews(views, m.activeViewName())
	*m = next
	if !ok {
		return
	}
	m.queryViewSelected = min(m.queryViewSelected, len(m.views)-1)
	m.detailNotice = "Deleted saved view."
}

func (m *Model) toggleSelectedSavedViewChildren() {
	if len(m.views) == 0 || m.queryViewSelected < 0 || m.queryViewSelected >= len(m.views) {
		m.detailNotice = "No saved views configured."
		return
	}
	views := append([]config.IssueView(nil), m.views...)
	views[m.queryViewSelected].IncludeChildren = !views[m.queryViewSelected].IncludeChildren
	next, ok := m.persistSavedViews(views, m.activeViewName())
	*m = next
	if ok {
		m.detailNotice = "Updated saved view."
	}
}

func (m *Model) moveSelectedSavedView(delta int) {
	if len(m.views) == 0 || m.queryViewSelected < 0 || m.queryViewSelected >= len(m.views) {
		m.detailNotice = "No saved views configured."
		return
	}
	nextIndex := m.queryViewSelected + delta
	if nextIndex < 0 || nextIndex >= len(m.views) {
		return
	}
	views := append([]config.IssueView(nil), m.views...)
	views[m.queryViewSelected], views[nextIndex] = views[nextIndex], views[m.queryViewSelected]
	next, ok := m.persistSavedViews(views, m.activeViewName())
	*m = next
	if ok {
		m.queryViewSelected = nextIndex
		m.detailNotice = "Moved saved view."
	}
}

func (m Model) saveQueryViewPrompt() Model {
	view := config.IssueView{
		Name:            m.querySaveViewNameValue(),
		JQL:             m.querySaveViewJQL,
		IncludeChildren: m.querySaveViewIncludeChildren,
	}
	if m.querySaveViewAction == querySaveViewActionRename {
		if m.querySaveViewIndex < 0 || m.querySaveViewIndex >= len(m.views) {
			m.detailNotice = "No saved view selected."
			return m
		}
		views := append([]config.IssueView(nil), m.views...)
		views[m.querySaveViewIndex] = view
		activeView := m.activeViewName()
		if m.querySaveViewIndex == m.view {
			activeView = strings.TrimSpace(view.Name)
		}
		next, ok := m.persistSavedViews(views, activeView)
		if !ok {
			return next
		}
		next.querySaveViewOpen = false
		next.queryViewSelected = m.querySaveViewIndex
		next.detailNotice = "Renamed saved view."
		return next
	}
	return m.addQuerySavedView(view)
}

func (m Model) addQuerySavedView(view config.IssueView) Model {
	cfg, err := config.AddSavedView(config.Config{Views: m.views}, view)
	if err != nil {
		m.detailNotice = err.Error()
		return m
	}
	view = cfg.Views[len(cfg.Views)-1]
	if m.savedViewWriter == nil {
		m.detailNotice = "Saved-view persistence is not available."
		return m
	}
	if err := m.savedViewWriter(view); err != nil {
		m.detailNotice = "Saved view failed: " + err.Error()
		return m
	}
	m.views = cfg.Views
	m.querySaveViewOpen = false
	m.detailNotice = "Saved view " + view.Name + "."
	return m
}

func (m Model) querySaveViewNameValue() string {
	if m.querySaveViewReady {
		return m.querySaveViewEditor.Value()
	}
	return m.querySaveViewName
}
