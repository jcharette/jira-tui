package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	lipglosstable "github.com/charmbracelet/lipgloss/table"
	"github.com/jcharette/jira-tui/internal/linkdetect"
	"github.com/jcharette/jira-tui/internal/ui"
)

func (m Model) renderRichDescriptionBody(value string, width int) string {
	source := strings.Split(value, "\n")
	lines := make([]string, 0, len(source))
	inCodeBlock := false
	inTable := false
	var codeLines []string
	var tableLines []string
	for _, line := range source {
		if line == "[table]" || line == "[/table]" {
			if line == "[table]" {
				inTable = true
				tableLines = nil
			} else {
				inTable = false
				lines = append(lines, m.renderTableBlock(tableLines, width))
				tableLines = nil
			}
			continue
		}
		if strings.TrimSpace(line) == "```" {
			if inCodeBlock {
				inCodeBlock = false
				lines = appendCodeBlock(lines, m.renderCodeBlockLines(codeLines, width))
				codeLines = nil
			} else {
				inCodeBlock = true
				codeLines = nil
			}
			continue
		}
		if inTable {
			tableLines = append(tableLines, line)
			continue
		}
		if inCodeBlock {
			codeLines = append(codeLines, line)
			continue
		}
		lines = append(lines, m.renderRichLine(line, width))
	}
	if inCodeBlock && len(codeLines) > 0 {
		lines = appendCodeBlock(lines, m.renderCodeBlockLines(codeLines, width))
	}
	if inTable && len(tableLines) > 0 {
		lines = append(lines, m.renderTableBlock(tableLines, width))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderTableBlock(tableLines []string, width int) string {
	rows := parseTableRows(tableLines)
	if len(rows) == 0 {
		return ""
	}
	headers := rows[0]
	body := rows[1:]
	tableWidth := max(24, width)
	t := lipglosstable.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(m.theme.Muted).
		Headers(headers...).
		Rows(body...).
		Width(tableWidth).
		StyleFunc(func(row, _ int) lipgloss.Style {
			switch {
			case row == lipglosstable.HeaderRow:
				return m.theme.FieldLabel.Padding(0, 1)
			case row%2 == 0:
				return m.theme.Text.Padding(0, 1)
			default:
				return m.theme.Muted.Padding(0, 1)
			}
		})
	rendered := t.String()
	if rendered == "" {
		return ""
	}
	return rendered
}

func (m Model) renderCodeBlockLines(lines []string, width int) string {
	lines = trimBlankCodeLines(lines)
	if len(lines) == 0 {
		return ""
	}
	blockWidth := max(12, width)
	contentWidth := max(1, blockWidth-2)
	rendered := make([]string, 0, len(lines))
	for _, line := range lines {
		line = truncate(line, contentWidth)
		padded := line + strings.Repeat(" ", contentWidth-len(line))
		rendered = append(rendered, m.theme.CodeBlock.Width(contentWidth).Render(padded))
	}
	return strings.Join(rendered, "\n")
}

func appendCodeBlock(lines []string, block string) []string {
	if block == "" {
		return trimTrailingBlankLines(lines)
	}
	lines = trimTrailingBlankLines(lines)
	if len(lines) > 0 {
		lines = append(lines, "")
	}
	return append(lines, block)
}

func trimTrailingBlankLines(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func trimBlankCodeLines(lines []string) []string {
	start := 0
	end := len(lines)
	for start < end && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[start:end]
}

func renderTableLine(theme ui.Theme, line string) string {
	if strings.TrimSpace(line) == "" {
		return ""
	}
	if isTableSeparator(line) {
		return theme.Muted.Render(line)
	}
	cells := strings.Split(line, "|")
	for index, cell := range cells {
		if index == 0 || index == len(cells)-1 {
			continue
		}
		cells[index] = renderRichInline(theme, cell)
	}
	return theme.Muted.Render("|") + strings.Join(cells[1:len(cells)-1], theme.Muted.Render("|")) + theme.Muted.Render("|")
}

func isTableSeparator(line string) bool {
	trimmed := strings.Trim(line, "| ")
	if trimmed == "" {
		return false
	}
	for _, r := range trimmed {
		if r != '-' && r != '|' {
			return false
		}
	}
	return strings.Contains(trimmed, "-")
}

func (m Model) renderRichLine(line string, width int) string {
	trimmed := strings.TrimSpace(line)
	if body, ok := strings.CutPrefix(trimmed, "[panel] "); ok {
		contentWidth := max(12, width-2)
		body = strings.TrimSpace(body)
		return m.theme.NoticeBlock.Width(contentWidth + 2).Render(m.theme.Warning.Render("Panel") + " " + renderRichInline(m.theme, body))
	}
	if body, ok := strings.CutPrefix(trimmed, "> "); ok {
		contentWidth := max(12, width-2)
		body = strings.TrimSpace(body)
		return m.theme.CommentBlock.Width(contentWidth + 2).Render(renderRichInline(m.theme, body))
	}
	return renderRichInline(m.theme, line)
}

func renderRichInline(theme ui.Theme, line string) string {
	var b strings.Builder
	remaining := line
	for {
		start := strings.Index(remaining, "`")
		statusStart, statusEnd, statusText := nextADFStatusMarker(remaining)
		linkStart, linkEnd := nextRichLink(remaining)
		mentionStart, mentionEnd := nextRichMention(remaining)
		if start < 0 && statusStart < 0 && linkStart < 0 && mentionStart < 0 {
			b.WriteString(theme.Text.Render(unescapeMarkdownPunctuation(remaining)))
			break
		}
		if statusStart >= 0 && isFirstRichToken(statusStart, start, linkStart, mentionStart) {
			b.WriteString(theme.Text.Render(unescapeMarkdownPunctuation(remaining[:statusStart])))
			b.WriteString(theme.Warning.Copy().Bold(true).Render(statusText))
			remaining = remaining[statusEnd:]
			continue
		}
		if linkStart >= 0 && isFirstRichToken(linkStart, start, statusStart, mentionStart) {
			b.WriteString(theme.Text.Render(unescapeMarkdownPunctuation(remaining[:linkStart])))
			b.WriteString(theme.Key.Copy().Underline(true).Render(remaining[linkStart:linkEnd]))
			remaining = remaining[linkEnd:]
			continue
		}
		if mentionStart >= 0 && isFirstRichToken(mentionStart, start, statusStart, linkStart) {
			b.WriteString(theme.Text.Render(unescapeMarkdownPunctuation(remaining[:mentionStart])))
			b.WriteString(theme.Selected.Render(remaining[mentionStart:mentionEnd]))
			remaining = remaining[mentionEnd:]
			continue
		}
		b.WriteString(theme.Text.Render(unescapeMarkdownPunctuation(remaining[:start])))
		remaining = remaining[start+1:]
		end := strings.Index(remaining, "`")
		if end < 0 {
			b.WriteString(theme.Text.Render("`" + unescapeMarkdownPunctuation(remaining)))
			break
		}
		code := remaining[:end]
		if code == "" {
			b.WriteString(theme.Text.Render("``"))
		} else {
			b.WriteString(theme.Code.Render(code))
		}
		remaining = remaining[end+1:]
	}
	return b.String()
}

func unescapeMarkdownPunctuation(value string) string {
	replacer := strings.NewReplacer(
		`\(`, "(",
		`\)`, ")",
		`\[`, "[",
		`\]`, "]",
		`\{`, "{",
		`\}`, "}",
		`\.`, ".",
		`\,`, ",",
		`\:`, ":",
		`\;`, ";",
		`\!`, "!",
		`\?`, "?",
	)
	return replacer.Replace(value)
}

func isFirstRichToken(candidate int, others ...int) bool {
	if candidate < 0 {
		return false
	}
	for _, other := range others {
		if other >= 0 && other < candidate {
			return false
		}
	}
	return true
}

func nextRichLink(value string) (int, int) {
	links := linkdetect.Detect(value)
	if len(links) == 0 {
		return -1, -1
	}
	return links[0].Start, links[0].End
}

func nextRichMention(value string) (int, int) {
	start := strings.Index(value, "@")
	for start >= 0 {
		if start > 0 && !isMentionBoundary(value[start-1]) {
			next := strings.Index(value[start+1:], "@")
			if next < 0 {
				return -1, -1
			}
			start += next + 1
			continue
		}
		end := start + 1
		for end < len(value) && isMentionChar(value[end]) {
			end++
		}
		if end > start+1 {
			return start, end
		}
		next := strings.Index(value[start+1:], "@")
		if next < 0 {
			return -1, -1
		}
		start += next + 1
	}
	return -1, -1
}

func isMentionBoundary(value byte) bool {
	return value == ' ' || value == '\t' || value == '\n' || value == '(' || value == '['
}

func isMentionChar(value byte) bool {
	return (value >= 'A' && value <= 'Z') ||
		(value >= 'a' && value <= 'z') ||
		(value >= '0' && value <= '9') ||
		value == '_' ||
		value == '-' ||
		value == '.'
}

func nextADFStatusMarker(value string) (int, int, string) {
	start := strings.Index(value, "[")
	for start >= 0 {
		endRelative := strings.Index(value[start+1:], "]")
		if endRelative < 0 {
			return -1, -1, ""
		}
		end := start + 1 + endRelative
		text := strings.TrimSpace(value[start+1 : end])
		if isADFStatusText(text) {
			return start, end + 1, text
		}
		next := strings.Index(value[start+1:], "[")
		if next < 0 {
			return -1, -1, ""
		}
		start += next + 1
	}
	return -1, -1, ""
}

func isADFStatusText(value string) bool {
	if value == "" || len(value) > 32 {
		return false
	}
	hasLetterOrDigit := false
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z':
			hasLetterOrDigit = true
		case r >= '0' && r <= '9':
			hasLetterOrDigit = true
		case r == ' ' || r == '-' || r == '_':
		default:
			return false
		}
	}
	return hasLetterOrDigit
}

func wrapRichText(value string, width int) string {
	normalized := strings.ReplaceAll(value, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	blocks := strings.Split(normalized, "\n")

	var lines []string
	previousBlank := false
	inCodeBlock := false
	for index := 0; index < len(blocks); index++ {
		block := strings.TrimSpace(blocks[index])
		block = strings.TrimSpace(block)
		if block == "```" {
			inCodeBlock = !inCodeBlock
			lines = append(lines, block)
			previousBlank = false
			continue
		}
		if inCodeBlock {
			lines = append(lines, fitCodeLine(block, width))
			previousBlank = false
			continue
		}
		if block == "[table]" {
			var tableLines []string
			for index++; index < len(blocks); index++ {
				tableBlock := strings.TrimSpace(blocks[index])
				if tableBlock == "[/table]" {
					break
				}
				if tableBlock != "" {
					tableLines = append(tableLines, tableBlock)
				}
			}
			if len(tableLines) > 0 {
				lines = append(lines, "[table]")
				lines = append(lines, renderFittedTable(tableLines, width)...)
				lines = append(lines, "[/table]")
			}
			previousBlank = false
			continue
		}
		if block == "" {
			if !previousBlank && len(lines) > 0 {
				lines = append(lines, "")
			}
			previousBlank = true
			continue
		}
		previousBlank = false
		lines = append(lines, wrapRichLine(block, width)...)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func markdownTablesToRichMarkers(value string) string {
	normalized := strings.ReplaceAll(value, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	lines := strings.Split(normalized, "\n")
	result := make([]string, 0, len(lines))
	for index := 0; index < len(lines); {
		if index+1 < len(lines) && isMarkdownTableRow(lines[index]) && isTableSeparator(lines[index+1]) {
			var tableLines []string
			for index < len(lines) && isMarkdownTableRow(lines[index]) {
				tableLines = append(tableLines, strings.TrimSpace(lines[index]))
				index++
			}
			result = append(result, "[table]")
			result = append(result, tableLines...)
			result = append(result, "[/table]")
			continue
		}
		result = append(result, lines[index])
		index++
	}
	return strings.Join(result, "\n")
}

func isMarkdownTableRow(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|") && strings.Count(trimmed, "|") >= 2
}

func fitCodeLine(line string, width int) string {
	if width <= 0 || len(line) <= width {
		return line
	}
	return truncate(line, width)
}

func renderFittedTable(lines []string, width int) []string {
	rows := parseTableRows(lines)
	if len(rows) == 0 {
		return nil
	}
	widths := fittedTableWidths(rows, width)
	var rendered []string
	for index, row := range rows {
		rendered = append(rendered, renderWrappedTableRow(row, widths)...)
		if index == 0 {
			rendered = append(rendered, renderTableSeparatorLine(widths))
		}
	}
	return rendered
}

func parseTableRows(lines []string) [][]string {
	var rows [][]string
	for _, line := range lines {
		if isTableSeparator(line) {
			continue
		}
		parts := strings.Split(strings.Trim(line, "|"), "|")
		row := make([]string, 0, len(parts))
		for _, part := range parts {
			row = append(row, strings.TrimSpace(part))
		}
		if len(row) > 0 {
			rows = append(rows, row)
		}
	}
	return rows
}

func fittedTableWidths(rows [][]string, width int) []int {
	columns := 0
	for _, row := range rows {
		columns = max(columns, len(row))
	}
	if columns == 0 {
		return nil
	}

	available := width - columns*2 - (columns + 1)
	if available < columns {
		available = columns
	}
	widths := make([]int, columns)
	for _, row := range rows {
		for index, cell := range row {
			widths[index] = max(widths[index], len(cell))
		}
	}

	for index := range widths {
		widths[index] = clamp(widths[index], 1, max(1, available/columns))
	}
	for remaining := available - sumInts(widths); remaining > 0; remaining-- {
		target := -1
		targetNeed := 0
		for index := range widths {
			need := naturalTableWidth(rows, index) - widths[index]
			if need > targetNeed {
				target = index
				targetNeed = need
			}
		}
		if target < 0 {
			break
		}
		widths[target]++
	}
	return widths
}

func naturalTableWidth(rows [][]string, column int) int {
	natural := 0
	for _, row := range rows {
		if column < len(row) {
			natural = max(natural, len(row[column]))
		}
	}
	return natural
}

func renderWrappedTableRow(row []string, widths []int) []string {
	wrappedCells := make([][]string, len(widths))
	height := 1
	for index, width := range widths {
		cell := ""
		if index < len(row) {
			cell = row[index]
		}
		wrapped := strings.Split(wrapText(cell, width), "\n")
		if len(wrapped) == 0 {
			wrapped = []string{""}
		}
		wrappedCells[index] = wrapped
		height = max(height, len(wrapped))
	}

	lines := make([]string, 0, height)
	for rowLine := 0; rowLine < height; rowLine++ {
		cells := make([]string, len(widths))
		for index, width := range widths {
			cell := ""
			if rowLine < len(wrappedCells[index]) {
				cell = wrappedCells[index][rowLine]
			}
			cell = truncate(cell, width)
			cells[index] = " " + cell + strings.Repeat(" ", width-len(cell)) + " "
		}
		lines = append(lines, "|"+strings.Join(cells, "|")+"|")
	}
	return lines
}

func renderTableSeparatorLine(widths []int) string {
	parts := make([]string, len(widths))
	for index, width := range widths {
		parts[index] = strings.Repeat("-", width+2)
	}
	return "|" + strings.Join(parts, "|") + "|"
}

func sumInts(values []int) int {
	total := 0
	for _, value := range values {
		total += value
	}
	return total
}

func wrapRichLine(line string, width int) []string {
	marker, body := richLineMarker(line)
	if marker == "" {
		return strings.Split(wrapText(line, width), "\n")
	}

	bodyWidth := max(12, width-len(marker))
	wrapped := strings.Split(wrapText(body, bodyWidth), "\n")
	if len(wrapped) == 0 {
		return []string{marker}
	}
	lines := make([]string, 0, len(wrapped))
	lines = append(lines, marker+wrapped[0])
	indent := strings.Repeat(" ", len(marker))
	for _, continuation := range wrapped[1:] {
		lines = append(lines, indent+continuation)
	}
	return lines
}

func richLineMarker(line string) (string, string) {
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
		return line[:2], strings.TrimSpace(line[2:])
	}
	if strings.HasPrefix(line, "> ") {
		return line[:2], strings.TrimSpace(line[2:])
	}
	for index, r := range line {
		if r == '.' && index > 0 && index < 4 {
			prefix := line[:index+1]
			for _, digit := range prefix[:len(prefix)-1] {
				if digit < '0' || digit > '9' {
					return "", line
				}
			}
			rest := strings.TrimSpace(line[index+1:])
			if rest != "" {
				return prefix + " ", rest
			}
		}
		if r < '0' || r > '9' {
			break
		}
	}
	return "", line
}
