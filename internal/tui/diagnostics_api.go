package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/jcharette/jira-tui/internal/worker"
)

func apiDiagnosticEvent(result worker.Result, now time.Time, startedAt time.Time) diagnosticEvent {
	status := "ok"
	if result.Err != nil {
		status = "error"
	}
	parts := []string{
		workerDiagnosticDetail(result.ID, "", nil),
		"endpoint=" + apiEndpointFamily(result.Kind),
		"scope=" + apiDiagnosticScope(result),
		"result=" + apiResultClass(result),
	}
	if metrics := apiDiagnosticMetrics(result); metrics != "" {
		parts = append(parts, metrics)
	}
	if !startedAt.IsZero() {
		parts = append(parts, "elapsed="+formatDiagnosticDuration(now.Sub(startedAt)))
	}
	if result.Err != nil {
		parts = append(parts, "error="+sanitizeAPIError(result.Err))
	}
	return diagnosticEvent{
		Kind:   diagnosticKindAPI,
		Label:  string(result.Kind),
		Status: status,
		Detail: strings.Join(nonEmptyStrings(parts), " "),
	}
}

func apiEndpointFamily(kind worker.Kind) string {
	switch kind {
	case worker.KindSearchIssues:
		return "search"
	case worker.KindGetIssue, worker.KindUpdateSummary, worker.KindUpdateDescription, worker.KindUpdatePriority, worker.KindUpdateLabels, worker.KindUpdateAssignee, worker.KindUpdateComponents, worker.KindUpdateEditField, worker.KindUpdateParent, worker.KindUpdateTimeTracking:
		return "issue"
	case worker.KindGetComments, worker.KindAddComment, worker.KindUpdateComment:
		return "comment"
	case worker.KindSearchUsers, worker.KindGetCurrentUser:
		return "user"
	case worker.KindExpandIssues:
		return "hierarchy"
	case worker.KindGetTransitions, worker.KindTransitionIssue, worker.KindStartIssue:
		return "transition"
	case worker.KindGetEditMetadata:
		return "edit_meta"
	case worker.KindGetCreateIssueTypes, worker.KindGetCreateFields, worker.KindCreateIssue:
		return "create"
	case worker.KindGetBoards, worker.KindGetBoardSprints, worker.KindMoveIssuesToSprint:
		return "agile"
	case worker.KindGetIssueLinkTypes, worker.KindCreateIssueLink, worker.KindDeleteIssueLink:
		return "issue_link"
	case worker.KindGetWorklogs, worker.KindAddWorklog, worker.KindUpdateWorklog, worker.KindDeleteWorklog:
		return "worklog"
	default:
		return "unknown"
	}
}

func apiDiagnosticScope(result worker.Result) string {
	switch {
	case result.Kind == worker.KindSearchIssues:
		return "jql"
	case result.GetIssue != nil:
		return issueScope(result.GetIssue.Key)
	case result.GetComments != nil:
		return issueScope(result.GetComments.Key)
	case result.AddComment != nil:
		return issueScope(result.AddComment.Key)
	case result.UpdateComment != nil:
		return issueScope(result.UpdateComment.Key)
	case result.SearchUsers != nil:
		return "user_query"
	case result.GetCurrentUser != nil:
		return "current_user"
	case result.StartIssue != nil:
		return issueScope(result.StartIssue.Key)
	case result.ExpandIssues != nil:
		return issueScope(result.ExpandIssues.ParentKey)
	case result.GetTransitions != nil:
		return issueScope(result.GetTransitions.Key)
	case result.TransitionIssue != nil:
		return issueScope(result.TransitionIssue.Key)
	case result.GetEditMetadata != nil:
		return issueScope(result.GetEditMetadata.Key)
	case result.GetCreateIssueTypes != nil:
		return projectScope(result.GetCreateIssueTypes.ProjectKey)
	case result.GetCreateFields != nil:
		return strings.TrimSpace(projectScope(result.GetCreateFields.ProjectKey) + " issue_type:" + result.GetCreateFields.IssueTypeID)
	case result.GetBoards != nil:
		return projectScope(result.GetBoards.ProjectKey)
	case result.GetBoardSprints != nil:
		return fmt.Sprintf("board:%d", result.GetBoardSprints.BoardID)
	case result.MoveIssuesToSprint != nil:
		if len(result.MoveIssuesToSprint.IssueKeys) > 0 {
			return issueScope(result.MoveIssuesToSprint.IssueKeys[0])
		}
		return fmt.Sprintf("sprint:%d", result.MoveIssuesToSprint.Sprint.ID)
	case result.CreateIssue != nil:
		return issueScope(result.CreateIssue.Issue.Key)
	case result.UpdateSummary != nil:
		return issueScope(result.UpdateSummary.Key)
	case result.UpdateDescription != nil:
		return issueScope(result.UpdateDescription.Key)
	case result.UpdatePriority != nil:
		return issueScope(result.UpdatePriority.Key)
	case result.UpdateAssignee != nil:
		return issueScope(result.UpdateAssignee.Key)
	case result.UpdateComponents != nil:
		return issueScope(result.UpdateComponents.Key)
	default:
		return apiFallbackScope(result)
	}
}

func apiFallbackScope(result worker.Result) string {
	key := strings.TrimSpace(resultDiagnosticKey(result))
	switch {
	case key == "":
		return "-"
	case looksLikeIssueKey(key):
		return issueScope(key)
	default:
		return "key"
	}
}

func issueScope(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return "issue"
	}
	return "issue:" + key
}

func projectScope(projectKey string) string {
	projectKey = strings.TrimSpace(projectKey)
	if projectKey == "" {
		return "project"
	}
	return "project:" + projectKey
}

func looksLikeIssueKey(value string) bool {
	parts := strings.Split(value, "-")
	return len(parts) == 2 && strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[1]) != ""
}

func apiResultClass(result worker.Result) string {
	if result.Err != nil {
		return "error"
	}
	return "success"
}

func apiDiagnosticMetrics(result worker.Result) string {
	switch {
	case result.SearchIssues != nil:
		return countMetric("issues", len(result.SearchIssues.Issues))
	case result.GetComments != nil:
		return countMetric("comments", len(result.GetComments.Comments))
	case result.SearchUsers != nil:
		return countMetric("users", len(result.SearchUsers.Users))
	case result.GetCurrentUser != nil:
		return countMetric("users", 1)
	case result.ExpandIssues != nil:
		return countMetric("issues", len(result.ExpandIssues.Issues))
	case result.GetTransitions != nil:
		return countMetric("transitions", len(result.GetTransitions.Transitions))
	case result.StartIssue != nil:
		return countMetric("actions", len(result.StartIssue.Outcomes))
	case result.GetCreateIssueTypes != nil:
		return countMetric("types", len(result.GetCreateIssueTypes.IssueTypes))
	case result.GetCreateFields != nil:
		return countMetric("fields", len(result.GetCreateFields.Fields))
	case result.GetBoards != nil:
		return countMetric("boards", len(result.GetBoards.Page.Boards))
	case result.GetBoardSprints != nil:
		return countMetric("sprints", len(result.GetBoardSprints.Page.Sprints))
	case result.MoveIssuesToSprint != nil:
		return countMetric("issues", len(result.MoveIssuesToSprint.IssueKeys))
	default:
		return ""
	}
}

func countMetric(label string, count int) string {
	return fmt.Sprintf("%s=%d empty=%t", label, count, count == 0)
}

func formatDiagnosticDuration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	switch {
	case duration >= time.Second:
		return duration.Truncate(time.Millisecond).String()
	case duration >= time.Millisecond:
		return duration.Truncate(time.Millisecond).String()
	default:
		return duration.Truncate(time.Microsecond).String()
	}
}

func sanitizeAPIError(err error) string {
	if err == nil {
		return ""
	}
	text := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case text == "":
		return "unknown"
	case strings.Contains(text, "unauthorized") || strings.Contains(text, "401"):
		return "unauthorized"
	case strings.Contains(text, "forbidden") || strings.Contains(text, "403"):
		return "forbidden"
	case strings.Contains(text, "not found") || strings.Contains(text, "404"):
		return "not_found"
	case strings.Contains(text, "timeout") || strings.Contains(text, "deadline exceeded"):
		return "timeout"
	case strings.Contains(text, "too many") || strings.Contains(text, "rate") || strings.Contains(text, "429"):
		return "rate_limited"
	case strings.Contains(text, "400") || strings.Contains(text, "bad request"):
		return "bad_request"
	default:
		return "failed"
	}
}

func nonEmptyStrings(values []string) []string {
	out := values[:0]
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
