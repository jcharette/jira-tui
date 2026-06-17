package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/jcharette/jira-tui/internal/ui"
)

func priorityBadge(priority string) string {
	trimmed := strings.TrimSpace(priority)
	if trimmed == "" {
		return "P?"
	}
	rank := priorityRank(trimmed)
	switch rank {
	case 5:
		return "!!!"
	case 4:
		return "!!"
	case 3:
		return "P3"
	case 2:
		return "P4"
	case 1:
		return "P5"
	default:
		return truncate(trimmed, 6)
	}
}

func indexFieldOptionByName(options []jira.FieldOption, name string) int {
	name = strings.TrimSpace(name)
	for index, option := range options {
		if strings.EqualFold(strings.TrimSpace(option.Name), name) {
			return index
		}
	}
	return 0
}

func displayValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func shortName(value string) string {
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return value
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return parts[0] + " " + string([]rune(parts[len(parts)-1])[0]) + "."
}

func issueTypeLabel(issue jira.Issue) string {
	prefix := ""
	if issue.ParentKey != "" || issue.IsSubtask {
		prefix = "|-"
	}
	if issue.IssueType == "" || issue.IssueType == "Unknown" {
		return prefix + "Issue"
	}
	return prefix + issue.IssueType
}

func listText(values []string) string {
	if len(values) == 0 {
		return "None"
	}
	return strings.Join(values, ", ")
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "Unknown"
	}
	return value.Local().Format("2006-01-02 15:04")
}

func statusStyle(theme ui.Theme, status string) lipgloss.Style {
	normalized := strings.ToLower(status)
	switch {
	case strings.Contains(normalized, "done"), strings.Contains(normalized, "closed"), strings.Contains(normalized, "resolved"):
		return theme.Success
	case strings.Contains(normalized, "block"), strings.Contains(normalized, "fail"):
		return theme.Error
	case strings.Contains(normalized, "progress"), strings.Contains(normalized, "review"):
		return theme.Warning
	default:
		return theme.Muted
	}
}

func priorityStyle(theme ui.Theme, priority string) lipgloss.Style {
	normalized := strings.ToLower(priority)
	switch {
	case strings.Contains(normalized, "highest"), strings.Contains(normalized, "critical"), strings.Contains(normalized, "blocker"):
		return theme.Error
	case strings.Contains(normalized, "high"):
		return theme.Warning
	case strings.Contains(normalized, "medium"):
		return theme.Text
	default:
		return theme.Muted
	}
}

func wrapText(value string, width int) string {
	if width <= 0 {
		return value
	}
	words := strings.Fields(value)
	if len(words) == 0 {
		return ""
	}

	var lines []string
	line := words[0]
	for _, word := range words[1:] {
		if len(line)+1+len(word) > width {
			lines = append(lines, line)
			line = word
			continue
		}
		line += " " + word
	}
	lines = append(lines, line)
	return strings.Join(lines, "\n")
}

func priorityRank(priority string) int {
	normalized := strings.ToLower(priority)
	switch {
	case strings.Contains(normalized, "highest"), strings.Contains(normalized, "blocker"), strings.Contains(normalized, "critical"):
		return 5
	case strings.Contains(normalized, "high"):
		return 4
	case strings.Contains(normalized, "medium"):
		return 3
	case strings.Contains(normalized, "low"):
		return 2
	case strings.Contains(normalized, "lowest"):
		return 1
	default:
		return 0
	}
}

func truncate(value string, width int) string {
	if width <= 0 || len(value) <= width {
		return value
	}
	if width <= 1 {
		return value[:width]
	}
	if width <= 3 {
		return value[:width]
	}
	return value[:width-3] + "..."
}

func indentLines(value string, prefix string) string {
	if value == "" || prefix == "" {
		return value
	}
	lines := strings.Split(value, "\n")
	for index, line := range lines {
		if line == "" {
			continue
		}
		lines[index] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func padRight(value string, width int) string {
	padding := width - lipgloss.Width(value)
	if padding <= 0 {
		return value
	}
	return value + strings.Repeat(" ", padding)
}

func fitRight(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return padRight(value, width)
	}
	result := ""
	for _, r := range reverseRunes(value) {
		candidate := string(r) + result
		if lipgloss.Width(candidate) > width {
			continue
		}
		result = candidate
		if lipgloss.Width(result) == width {
			break
		}
	}
	return padRight(result, width)
}

func fitLeft(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	result := ""
	for _, r := range value {
		candidate := result + string(r)
		if lipgloss.Width(candidate) > width {
			break
		}
		result = candidate
	}
	return result
}

func reverseRunes(value string) []rune {
	runes := []rune(value)
	for left, right := 0, len(runes)-1; left < right; left, right = left+1, right-1 {
		runes[left], runes[right] = runes[right], runes[left]
	}
	return runes
}

func projectKeyFromJQL(jql string) string {
	fields := strings.Fields(strings.ReplaceAll(jql, "\"", " "))
	for index := 0; index+2 < len(fields); index++ {
		if strings.EqualFold(fields[index], "project") && fields[index+1] == "=" {
			return strings.Trim(fields[index+2], "'\"()")
		}
	}
	return ""
}

func boundedSelectionWindow(total int, selected int, limit int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	if limit <= 0 || total <= limit {
		return 0, total
	}
	selected = clamp(selected, 0, total-1)
	start := selected - limit/2
	start = clamp(start, 0, total-limit)
	return start, start + limit
}

func prependIssue(issues []jira.Issue, issue jira.Issue) []jira.Issue {
	if strings.TrimSpace(issue.Key) == "" {
		return issues
	}
	result := []jira.Issue{issue}
	for _, existing := range issues {
		if existing.Key == issue.Key {
			continue
		}
		result = append(result, existing)
	}
	return result
}

func clamp(value, low, high int) int {
	return min(max(value, low), high)
}
