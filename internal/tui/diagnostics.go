package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/jon/jira-tui/internal/jira"
	"github.com/jon/jira-tui/internal/worker"
)

const maxDiagnosticsEvents = 80

type diagnosticKind string

const (
	diagnosticKindWorker diagnosticKind = "worker"
	diagnosticKindCache  diagnosticKind = "cache"
	diagnosticKindClaude diagnosticKind = "claude"
)

type diagnosticEvent struct {
	At     time.Time
	Kind   diagnosticKind
	Label  string
	Status string
	Detail string
}

type diagnosticStats struct {
	Workers int
	Cache   int
	Errors  int
	Active  int
}

func (m Model) renderDiagnostics(layout browserLayout) string {
	rows := m.boundedPanelBodyRows(12)
	events := m.recentDiagnosticEvents(rows)
	var b strings.Builder
	b.WriteString(m.detailSectionHeader("diagnostics", "Diagnostics", "Background Activity", max(32, layout.contentWidth-4)))
	b.WriteString("\n\n")
	if len(events) == 0 {
		b.WriteString(m.theme.Muted.Render("No background activity recorded yet."))
		return m.theme.ActivePane.Width(layout.contentWidth).Render(strings.TrimRight(b.String(), "\n"))
	}
	b.WriteString(m.renderDiagnosticsSummary(events, max(20, layout.contentWidth-6)))
	b.WriteString("\n\n")
	b.WriteString(m.theme.Muted.Render(fmt.Sprintf("%-8s  %-8s  %-8s  %s", "TIME", "KIND", "STATUS", "DETAIL")))
	for _, event := range events {
		line := fmt.Sprintf("%-8s  %-8s  %-8s  %s", event.At.Format("15:04:05"), event.Kind, event.Status, diagnosticEventDetail(event))
		b.WriteString("\n")
		b.WriteString(truncate(line, max(20, layout.contentWidth-6)))
	}
	return m.theme.ActivePane.Width(layout.contentWidth).Render(strings.TrimRight(b.String(), "\n"))
}

func (m Model) renderDiagnosticsSummary(events []diagnosticEvent, width int) string {
	stats := diagnosticStatsFor(events)
	last := "none"
	if len(events) > 0 {
		event := events[len(events)-1]
		last = strings.TrimSpace(event.Label + " " + event.Status)
	}
	summary := fmt.Sprintf("Workers %d   Cache %d   Errors %d   Active %d   Last %s", stats.Workers, stats.Cache, stats.Errors, stats.Active, last)
	bars := fmt.Sprintf("Activity  worker %s  cache  %s", diagnosticActivityBar(stats.Workers, len(events), 12), diagnosticActivityBar(stats.Cache, len(events), 12))
	return truncate(summary, width) + "\n" + truncate(bars, width)
}

func diagnosticStatsFor(events []diagnosticEvent) diagnosticStats {
	var stats diagnosticStats
	activeRequests := make(map[string]struct{})
	for _, event := range events {
		switch event.Kind {
		case diagnosticKindWorker:
			stats.Workers++
			switch event.Status {
			case "submit":
				if requestID := diagnosticEventRequestID(event); requestID != "" {
					activeRequests[requestID] = struct{}{}
				} else {
					stats.Active++
				}
			case "ok", "error":
				if requestID := diagnosticEventRequestID(event); requestID != "" {
					delete(activeRequests, requestID)
				} else {
					stats.Active = max(0, stats.Active-1)
				}
			}
		case diagnosticKindCache:
			stats.Cache++
		}
		if event.Status == "error" {
			stats.Errors++
		}
	}
	stats.Active += len(activeRequests)
	return stats
}

func diagnosticActivityBar(count int, total int, width int) string {
	if width <= 0 {
		return "[]"
	}
	filled := 0
	if total > 0 && count > 0 {
		filled = max(1, min(width, count*width/total))
	}
	return "[" + strings.Repeat("#", filled) + strings.Repeat("-", max(0, width-filled)) + "]"
}

func diagnosticEventDetail(event diagnosticEvent) string {
	detail := strings.TrimSpace(event.Detail)
	label := strings.TrimSpace(event.Label)
	switch {
	case label == "":
		return detail
	case detail == "":
		return label
	case strings.HasPrefix(detail, label+" "):
		return detail
	default:
		return label + " " + detail
	}
}

func diagnosticEventRequestID(event diagnosticEvent) string {
	for _, field := range strings.Fields(event.Detail) {
		if strings.HasPrefix(field, "#") {
			return field
		}
	}
	return ""
}

func (m *Model) recordDiagnosticEvent(kind diagnosticKind, label string, status string, detail string) {
	if label == "" && detail == "" {
		return
	}
	m.diagnosticsEvents = append(m.diagnosticsEvents, diagnosticEvent{
		At:     time.Now(),
		Kind:   kind,
		Label:  label,
		Status: status,
		Detail: detail,
	})
	if len(m.diagnosticsEvents) > maxDiagnosticsEvents {
		start := len(m.diagnosticsEvents) - maxDiagnosticsEvents
		m.diagnosticsEvents = append([]diagnosticEvent(nil), m.diagnosticsEvents[start:]...)
	}
}

func (m Model) recentDiagnosticEvents(limit int) []diagnosticEvent {
	if limit <= 0 || len(m.diagnosticsEvents) == 0 {
		return nil
	}
	start := max(0, len(m.diagnosticsEvents)-limit)
	events := append([]diagnosticEvent(nil), m.diagnosticsEvents[start:]...)
	return events
}

func resultDiagnosticEvent(result worker.Result) diagnosticEvent {
	status := "ok"
	if result.Err != nil {
		status = "error"
	}
	detailParts := []string{workerDiagnosticDetail(result.ID, resultDiagnosticKey(result), result.Err)}
	if metrics := resultDiagnosticMetrics(result); metrics != "" {
		detailParts = append(detailParts, metrics)
	}
	return diagnosticEvent{
		Kind:   diagnosticKindWorker,
		Label:  string(result.Kind),
		Status: status,
		Detail: strings.Join(detailParts, " "),
	}
}

func (m *Model) recordWorkerResult(event diagnosticEvent) {
	m.recordDiagnosticEvent(event.Kind, event.Label, event.Status, event.Detail)
}

func workerDiagnosticDetail(id int, key string, err error) string {
	var parts []string
	if id > 0 {
		parts = append(parts, fmt.Sprintf("#%d", id))
	}
	if key != "" {
		parts = append(parts, key)
	}
	if err != nil {
		parts = append(parts, truncate(err.Error(), 80))
	}
	return strings.Join(parts, " ")
}

func resultDiagnosticKey(result worker.Result) string {
	switch {
	case result.GetIssue != nil:
		return result.GetIssue.Key
	case result.GetComments != nil:
		return result.GetComments.Key
	case result.AddComment != nil:
		return result.AddComment.Key
	case result.SearchUsers != nil:
		return result.SearchUsers.Query
	case result.ExpandIssues != nil:
		return result.ExpandIssues.ParentKey
	case result.GetTransitions != nil:
		return result.GetTransitions.Key
	case result.TransitionIssue != nil:
		return result.TransitionIssue.Key
	case result.GetEditMetadata != nil:
		return result.GetEditMetadata.Key
	case result.GetCreateIssueTypes != nil:
		return result.GetCreateIssueTypes.ProjectKey
	case result.GetCreateFields != nil:
		return strings.TrimSpace(result.GetCreateFields.ProjectKey + " " + result.GetCreateFields.IssueTypeID)
	case result.CreateIssue != nil:
		return result.CreateIssue.Issue.Key
	case result.UpdateSummary != nil:
		return result.UpdateSummary.Key
	case result.UpdateDescription != nil:
		return result.UpdateDescription.Key
	case result.UpdatePriority != nil:
		return result.UpdatePriority.Key
	case result.UpdateAssignee != nil:
		return result.UpdateAssignee.Key
	default:
		return ""
	}
}

func resultDiagnosticMetrics(result worker.Result) string {
	switch {
	case result.GetCreateIssueTypes != nil:
		return fmt.Sprintf("types=%d", len(result.GetCreateIssueTypes.IssueTypes))
	case result.GetCreateFields != nil:
		fields := result.GetCreateFields.Fields
		return fmt.Sprintf(
			"fields=%d supported=%d required_unsupported=%d sample=%s",
			len(fields),
			len(supportedCreateFields(fields)),
			len(unsupportedRequiredCreateFields(fields)),
			createFieldDiagnosticSample(fields, 6),
		)
	default:
		return ""
	}
}

func createFieldDiagnosticSample(fields []jira.CreateField, limit int) string {
	if limit <= 0 || len(fields) == 0 {
		return "-"
	}
	var parts []string
	for index, field := range fields {
		if index >= limit {
			parts = append(parts, "...")
			break
		}
		id := displayValue(field.ID, field.Key)
		name := displayValue(field.Name, id)
		schema := displayValue(field.SchemaSystem, displayValue(field.SchemaType, "unknown"))
		parts = append(parts, strings.ReplaceAll(fmt.Sprintf("%s/%s/%s", id, name, schema), " ", "_"))
	}
	return strings.Join(parts, ",")
}
