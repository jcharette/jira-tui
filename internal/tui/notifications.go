package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jcharette/jira-tui/internal/events"
	"github.com/jcharette/jira-tui/internal/jira"
)

type NotificationConfig struct {
	Enabled                   bool
	SystemEnabled             bool
	SystemOnNew               bool
	SystemOnUpdates           bool
	AutoOpenPanel             bool
	KeepPanelOpenUntilCleared bool
	SystemOnNewTickets        bool
	MaxItems                  int
}

type notification struct {
	ID            string
	EventType     events.Type
	IssueKey      string
	Summary       string
	ChangedFields []string
	ViewName      string
	At            time.Time
}

func WithNotificationConfig(config NotificationConfig) Option {
	return func(m *Model) {
		if config.MaxItems <= 0 {
			config.MaxItems = 50
		}
		m.notificationConfig = config
	}
}

func (m *Model) recordNotification(event events.Event) {
	if !m.notificationConfig.Enabled || !isTicketNotificationEvent(event.Type) {
		return
	}
	notice, ok := notificationFromEvent(event, m.currentTime())
	if !ok {
		return
	}
	m.notifications = append([]notification{notice}, m.notifications...)
	limit := m.notificationConfig.MaxItems
	if limit <= 0 {
		limit = 50
	}
	if len(m.notifications) > limit {
		m.notifications = m.notifications[:limit]
	}
	m.selectedNotification = 0
	if m.notificationConfig.AutoOpenPanel {
		m.notificationPanelOpen = true
	}
}

func notificationFromEvent(event events.Event, fallback time.Time) (notification, bool) {
	var payload events.TicketPayload
	if len(event.Payload) > 0 {
		_ = json.Unmarshal(event.Payload, &payload)
	}
	key := strings.TrimSpace(payload.IssueKey)
	if key == "" {
		key = strings.TrimSpace(event.DedupeKey)
	}
	if key == "" {
		return notification{}, false
	}
	at := event.At
	if at.IsZero() {
		at = payload.SyncedAt
	}
	if at.IsZero() {
		at = fallback
	}
	id := strings.TrimSpace(event.ID)
	if id == "" {
		id = fmt.Sprintf("%s:%s:%d", event.Type, key, at.UnixNano())
	}
	return notification{
		ID:            id,
		EventType:     event.Type,
		IssueKey:      key,
		Summary:       strings.TrimSpace(payload.Current.Summary),
		ChangedFields: append([]string(nil), payload.ChangedFields...),
		ViewName:      strings.TrimSpace(payload.ViewName),
		At:            at,
	}, true
}

func isTicketNotificationEvent(eventType events.Type) bool {
	return eventType == events.TypeJiraTicketNew || eventType == events.TypeJiraTicketUpdated
}

func (m Model) notificationSummaryVisible() bool {
	return m.notificationConfig.Enabled && len(m.notifications) > 0 && !m.notificationPanelOpen
}

func (m Model) renderNotificationSummary(layout browserLayout) string {
	if !m.notificationSummaryVisible() {
		return ""
	}
	notice := m.notifications[0]
	text := fmt.Sprintf("Notifications: %d  latest: %s %s  ctrl+n open", len(m.notifications), notice.IssueKey, notice.shortDetail())
	return m.theme.Warning.Render(truncate(text, max(20, layout.contentWidth)))
}

func (m Model) renderNotificationCenter(layout browserLayout) string {
	width := layout.contentWidth
	bodyWidth := max(32, width-4)
	var b strings.Builder
	header := m.detailSectionHeader("notifications", "Notification Center", fmt.Sprintf("%d uncleared", len(m.notifications)), bodyWidth)
	b.WriteString(header)
	b.WriteString("\n\n")
	if len(m.notifications) == 0 {
		b.WriteString(m.detailEmptyState("No uncleared notifications.", bodyWidth))
		return m.theme.ActivePane.Width(width).Render(b.String())
	}
	start := clamp(m.selectedNotification-4, 0, max(0, len(m.notifications)-1))
	rows := min(len(m.notifications), start+max(5, min(12, layout.rows)))
	for index := start; index < rows; index++ {
		notice := m.notifications[index]
		cursor := "  "
		style := m.theme.Text
		if index == m.selectedNotification {
			cursor = "> "
			style = m.theme.Selected
		}
		line := fmt.Sprintf("%s%s  %-3s  %-10s  %s", cursor, notice.At.Format("15:04"), notificationKindLabel(notice.EventType), notice.IssueKey, notice.shortDetail())
		b.WriteString(style.Render(truncate(line, bodyWidth)))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(m.theme.Muted.Render("enter open  x clear selected  ctrl+x clear all"))
	return m.theme.ActivePane.Width(width).Render(strings.TrimRight(b.String(), "\n"))
}

func (m Model) updateNotificationPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+n":
		if m.notificationConfig.KeepPanelOpenUntilCleared && len(m.notifications) > 0 {
			return m, nil
		}
		m.notificationPanelOpen = false
	case "j", "down":
		m.selectedNotification = clamp(m.selectedNotification+1, 0, max(0, len(m.notifications)-1))
	case "k", "up":
		m.selectedNotification = clamp(m.selectedNotification-1, 0, max(0, len(m.notifications)-1))
	case "x":
		m.clearSelectedNotification()
	case "ctrl+x":
		m.notifications = nil
		m.selectedNotification = 0
		m.notificationPanelOpen = false
	case "enter":
		return m.openSelectedNotificationIssue()
	}
	return m, nil
}

func (m *Model) clearSelectedNotification() {
	if len(m.notifications) == 0 {
		m.notificationPanelOpen = false
		m.selectedNotification = 0
		return
	}
	index := clamp(m.selectedNotification, 0, len(m.notifications)-1)
	m.notifications = append(m.notifications[:index], m.notifications[index+1:]...)
	m.selectedNotification = clamp(index, 0, max(0, len(m.notifications)-1))
	if len(m.notifications) == 0 {
		m.notificationPanelOpen = false
	}
}

func (m Model) openSelectedNotificationIssue() (Model, tea.Cmd) {
	if len(m.notifications) == 0 {
		return m, nil
	}
	notice := m.notifications[clamp(m.selectedNotification, 0, len(m.notifications)-1)]
	key := notice.IssueKey
	for index, issue := range m.issues {
		if issue.Key == key {
			m.selected = index
			m.mode = modeDetail
			m.notificationPanelOpen = false
			return m.startDetailRequestForSelected()
		}
	}
	m.issues = append(m.issues, jira.Issue{Key: key, Summary: notice.Summary})
	m.selected = len(m.issues) - 1
	m.mode = modeDetail
	m.notificationPanelOpen = false
	return m.startDetailRequestForSelected()
}

func (m Model) renderTicketNotifications(key string, width int) string {
	if strings.TrimSpace(key) == "" || len(m.notifications) == 0 {
		return ""
	}
	var matches []notification
	for _, notice := range m.notifications {
		if notice.IssueKey == key {
			matches = append(matches, notice)
		}
	}
	if len(matches) == 0 {
		return ""
	}
	var lines []string
	lines = append(lines, m.detailSectionHeader("ticket-notifications", "Ticket Notifications", fmt.Sprintf("%d uncleared", len(matches)), width))
	for _, notice := range matches[:min(len(matches), 3)] {
		line := fmt.Sprintf("%s  %s  %s", notice.At.Format("15:04"), notice.IssueKey, notice.shortDetail())
		lines = append(lines, m.theme.Warning.Render(truncate(line, width)))
	}
	return strings.Join(lines, "\n")
}

func (n notification) shortDetail() string {
	fields := strings.Join(n.ChangedFields, ", ")
	if fields == "" {
		if n.EventType == events.TypeJiraTicketNew {
			fields = "new ticket"
		} else {
			fields = "ticket updated"
		}
	}
	if n.Summary != "" {
		return fields + "  " + n.Summary
	}
	return fields
}

func notificationKindLabel(eventType events.Type) string {
	if eventType == events.TypeJiraTicketNew {
		return "NEW"
	}
	return "UPD"
}
