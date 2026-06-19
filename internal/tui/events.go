package tui

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jcharette/jira-tui/internal/events"
	"github.com/jcharette/jira-tui/internal/jira"
)

type eventStream interface {
	Publish(context.Context, events.Event) error
	Subscribe(context.Context) (<-chan events.Event, error)
}

func (m Model) waitForAppEvent() tea.Cmd {
	if m.eventInbox == nil {
		return nil
	}
	return func() tea.Msg {
		event, ok := <-m.eventInbox
		if !ok {
			return noDetailRequestMsg{}
		}
		return appEventMsg{event: event}
	}
}

func (m *Model) recordAppEvent(event events.Event) {
	detail := strings.TrimSpace(strings.Join([]string{event.Source, event.DedupeKey}, " "))
	m.recordDiagnosticEvent(diagnosticKindEvent, string(event.Type), "published", detail)
}

func (m *Model) publishTicketEvents(newIssues []jira.Issue, syncedAt time.Time) {
	if m.eventStream == nil || len(m.issues) == 0 {
		return
	}
	if syncedAt.IsZero() {
		syncedAt = m.currentTime()
	}
	previousByKey := make(map[string]jira.Issue, len(m.issues))
	for _, issue := range m.issues {
		if strings.TrimSpace(issue.Key) == "" {
			continue
		}
		previousByKey[issue.Key] = issue
	}
	for _, issue := range newIssues {
		if strings.TrimSpace(issue.Key) == "" {
			continue
		}
		previous, exists := previousByKey[issue.Key]
		eventType := events.TypeJiraTicketNew
		var previousIssue *jira.Issue
		var changedFields []string
		if exists {
			changedFields = changedIssueFields(previous, issue)
			if len(changedFields) == 0 {
				continue
			}
			eventType = events.TypeJiraTicketUpdated
			previousCopy := previous
			previousIssue = &previousCopy
		}
		payload, err := json.Marshal(events.TicketPayload{
			IssueKey:      issue.Key,
			Previous:      previousIssue,
			Current:       issue,
			ChangedFields: changedFields,
			ViewName:      m.activeViewName(),
			JQL:           m.jql,
			SyncedAt:      syncedAt,
		})
		if err != nil {
			m.recordDiagnosticEvent(diagnosticKindEvent, string(eventType), "error", err.Error())
			continue
		}
		err = m.eventStream.Publish(context.Background(), events.Event{
			Type:      eventType,
			Source:    "active_view",
			DedupeKey: issue.Key,
			Payload:   payload,
		})
		if err != nil {
			m.recordDiagnosticEvent(diagnosticKindEvent, string(eventType), "error", err.Error())
		}
	}
}

func changedIssueFields(previous jira.Issue, current jira.Issue) []string {
	var fields []string
	if previous.Summary != current.Summary {
		fields = append(fields, "summary")
	}
	if previous.Status != current.Status {
		fields = append(fields, "status")
	}
	if previous.Assignee != current.Assignee {
		fields = append(fields, "assignee")
	}
	if previous.Priority != current.Priority {
		fields = append(fields, "priority")
	}
	if previous.IssueType != current.IssueType {
		fields = append(fields, "issue_type")
	}
	if previous.IsSubtask != current.IsSubtask {
		fields = append(fields, "is_subtask")
	}
	if previous.HierarchyLevel != current.HierarchyLevel {
		fields = append(fields, "hierarchy_level")
	}
	if previous.ParentKey != current.ParentKey {
		fields = append(fields, "parent_key")
	}
	if previous.ParentSummary != current.ParentSummary {
		fields = append(fields, "parent_summary")
	}
	if previous.SubtaskCount != current.SubtaskCount {
		fields = append(fields, "subtask_count")
	}
	if previous.URL != current.URL {
		fields = append(fields, "url")
	}
	return fields
}
