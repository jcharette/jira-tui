package tui

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jcharette/jira-tui/internal/events"
	"github.com/jcharette/jira-tui/internal/jira"
)

func TestTicketEventAutoOpensPersistentNotificationPanel(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithNotificationConfig(NotificationConfig{
		Enabled:                   true,
		AutoOpenPanel:             true,
		KeepPanelOpenUntilCleared: true,
		MaxItems:                  50,
	}))
	defer model.workers.Stop()
	model.width = 120
	model.height = 30

	event := ticketNotificationEventForTest(t, events.TypeJiraTicketUpdated, jira.Issue{
		Key:      "ABC-1",
		Summary:  "Refresh side panel",
		Status:   "In Progress",
		Priority: "High",
	}, []string{"status", "priority"})

	updated, _ := model.Update(appEventMsg{event: event})
	next := updated.(Model)

	if !next.notificationPanelOpen {
		t.Fatal("notification panel should auto-open")
	}
	if len(next.notifications) != 1 {
		t.Fatalf("notifications = %#v", next.notifications)
	}
	view := next.render()
	for _, want := range []string{"Notification Center", "ABC-1", "Refresh side panel", "status, priority", "clear selected"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestNotificationPanelStaysVisibleUntilCleared(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithNotificationConfig(NotificationConfig{
		Enabled:                   true,
		AutoOpenPanel:             true,
		KeepPanelOpenUntilCleared: true,
		MaxItems:                  50,
	}))
	defer model.workers.Stop()
	event := ticketNotificationEventForTest(t, events.TypeJiraTicketNew, jira.Issue{
		Key:     "ABC-2",
		Summary: "New blocking ticket",
	}, nil)
	updated, _ := model.Update(appEventMsg{event: event})
	next := updated.(Model)

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "esc", Code: tea.KeyEsc}))
	next = updated.(Model)
	if !next.notificationPanelOpen {
		t.Fatal("panel should remain open while uncleared notifications exist")
	}

	updated, _ = next.Update(tea.KeyPressMsg(tea.Key{Text: "x", Code: 'x'}))
	next = updated.(Model)
	if next.notificationPanelOpen {
		t.Fatal("panel should close after clearing the final notification")
	}
	if len(next.notifications) != 0 {
		t.Fatalf("notifications after clear = %#v", next.notifications)
	}
}

func TestNotificationPanelEnterOpensSelectedUnloadedTicket(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithNotificationConfig(NotificationConfig{
		Enabled:  true,
		MaxItems: 50,
	}))
	defer model.workers.Stop()
	model.notificationPanelOpen = true
	model.notifications = []notification{{
		IssueKey: "ABC-2",
		Summary:  "Selected from notification",
	}}
	model.issues = []jira.Issue{
		{Key: "ABC-1", Summary: "First"},
	}

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	next := updated.(Model)

	if cmd == nil {
		t.Fatal("expected detail request command")
	}
	if next.mode != modeDetail {
		t.Fatalf("mode = %v, want %v", next.mode, modeDetail)
	}
	if next.notificationPanelOpen {
		t.Fatal("notification panel should close after opening the selected ticket")
	}
	if next.selected != 1 {
		t.Fatalf("selected = %d, want 1", next.selected)
	}
	if next.detailRequestKey != "ABC-2" {
		t.Fatalf("detailRequestKey = %q, want ABC-2", next.detailRequestKey)
	}
	if len(next.issues) != 2 || next.issues[1].Key != "ABC-2" {
		t.Fatalf("issues = %#v, want appended ABC-2", next.issues)
	}
}

func TestMainPanelShowsCompactNotificationAlert(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithNotificationConfig(NotificationConfig{
		Enabled:  true,
		MaxItems: 50,
	}))
	defer model.workers.Stop()
	model.width = 120
	model.height = 30
	model.loading = false
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Existing"}}

	event := ticketNotificationEventForTest(t, events.TypeJiraTicketUpdated, jira.Issue{
		Key:     "ABC-1",
		Summary: "Existing",
		Status:  "Done",
	}, []string{"status"})
	updated, _ := model.Update(appEventMsg{event: event})
	next := updated.(Model)
	next.notificationPanelOpen = false

	view := next.render()
	for _, want := range []string{"Notifications: 1", "ABC-1", "ctrl+n open"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
}

func TestDetailShowsTicketScopedNotifications(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC", WithNotificationConfig(NotificationConfig{
		Enabled:  true,
		MaxItems: 50,
	}))
	defer model.workers.Stop()
	model.width = 120
	model.height = 30
	model.loading = false
	model.mode = modeDetail
	model.issues = []jira.Issue{{Key: "ABC-1", Summary: "Selected"}, {Key: "ABC-2", Summary: "Other"}}
	model.selected = 0

	event := ticketNotificationEventForTest(t, events.TypeJiraTicketUpdated, jira.Issue{
		Key:     "ABC-1",
		Summary: "Selected",
		Status:  "Blocked",
	}, []string{"status"})
	updated, _ := model.Update(appEventMsg{event: event})
	next := updated.(Model)
	next.notificationPanelOpen = false

	view := next.render()
	for _, want := range []string{"Ticket Notifications", "ABC-1", "status"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q in %q", want, view)
		}
	}
	if strings.Contains(view, "ABC-2") {
		t.Fatalf("detail notifications should be scoped to selected issue: %q", view)
	}
}

func ticketNotificationEventForTest(t *testing.T, eventType events.Type, issue jira.Issue, fields []string) events.Event {
	t.Helper()
	payload, err := json.Marshal(events.TicketPayload{
		IssueKey:      issue.Key,
		Current:       issue,
		ChangedFields: fields,
		ViewName:      "Assigned",
		SyncedAt:      time.Date(2026, 6, 19, 10, 30, 0, 0, time.Local),
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	return events.Event{
		Type:      eventType,
		At:        time.Date(2026, 6, 19, 10, 30, 0, 0, time.Local),
		Source:    "active_view",
		DedupeKey: issue.Key,
		Payload:   payload,
	}
}
