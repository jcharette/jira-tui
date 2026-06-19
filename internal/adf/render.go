package adf

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	model "github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"
	"github.com/rgonek/jira-adf-converter/converter"
)

var mailtoMarkdownPattern = regexp.MustCompile(`\[([^\]]+)\]\(mailto:([^)]+)\)`)

// Render turns Jira ADF into terminal-friendly plain text. It intentionally
// preserves structure over exact styling because the TUI owns final colors.
func Render(node *model.CommentNodeScheme) string {
	if node == nil {
		return ""
	}
	if rendered, err := renderMarkdown(node); err == nil && strings.TrimSpace(rendered) != "" {
		return rendered
	}
	return renderLegacy(node)
}

func renderMarkdown(node *model.CommentNodeScheme) (string, error) {
	if node.Type == "doc" {
		blocks := make([]string, 0, len(node.Content))
		var chunk []*model.CommentNodeScheme
		flushChunk := func() error {
			if len(chunk) == 0 {
				return nil
			}
			text, err := renderMarkdownDoc(chunk)
			if err != nil {
				return err
			}
			if strings.TrimSpace(text) != "" {
				blocks = append(blocks, text)
			}
			chunk = nil
			return nil
		}
		for _, child := range node.Content {
			if child == nil {
				continue
			}
			if child.Type == "table" {
				if err := flushChunk(); err != nil {
					return "", err
				}
				if text := strings.TrimSpace(renderLegacy(child)); text != "" {
					blocks = append(blocks, text)
				}
				continue
			}
			chunk = append(chunk, child)
		}
		if err := flushChunk(); err != nil {
			return "", err
		}
		return strings.TrimSpace(strings.Join(blocks, "\n\n")), nil
	}
	return renderMarkdownBlock(node)
}

func renderMarkdownDoc(content []*model.CommentNodeScheme) (string, error) {
	return renderMarkdownBlock(&model.CommentNodeScheme{
		Type:    "doc",
		Content: content,
	})
}

func renderMarkdownBlock(node *model.CommentNodeScheme) (string, error) {
	if node.Type == "table" {
		return renderLegacy(node), nil
	}
	payload, err := json.Marshal(node)
	if err != nil {
		return "", err
	}
	conv, err := converter.New(converter.Config{
		MentionStyle: converter.MentionText,
		ExpandStyle:  converter.ExpandBlockquote,
		TableMode:    converter.TablePipe,
	})
	if err != nil {
		return "", err
	}
	result, err := conv.Convert(payload)
	if err != nil {
		return "", err
	}
	return normalizeMarkdown(result.Markdown), nil
}

func renderLegacy(node *model.CommentNodeScheme) string {
	var blocks []string
	collectBlocks(node, &blocks)
	return strings.TrimSpace(strings.Join(blocks, "\n"))
}

func normalizeMarkdown(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = strings.ReplaceAll(value, "\\\n", "\n")
	value = mailtoMarkdownPattern.ReplaceAllStringFunc(value, func(match string) string {
		parts := mailtoMarkdownPattern.FindStringSubmatch(match)
		if len(parts) == 3 && strings.EqualFold(strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2])) {
			return parts[1]
		}
		return match
	})

	lines := strings.Split(value, "\n")
	normalized := make([]string, 0, len(lines))
	for index := 0; index < len(lines); {
		line := strings.TrimRight(lines[index], " ")
		if strings.HasPrefix(strings.TrimSpace(line), "```") && strings.TrimSpace(line) != "```" {
			normalized = append(normalized, "```")
			index++
			continue
		}
		if index+1 < len(lines) && isMarkdownTableRow(line) && isTableSeparatorLine(lines[index+1]) {
			normalized = append(normalized, "[table]")
			for index < len(lines) && isMarkdownTableRow(lines[index]) {
				normalized = append(normalized, strings.TrimSpace(lines[index]))
				index++
			}
			normalized = append(normalized, "[/table]")
			continue
		}
		normalized = append(normalized, line)
		index++
	}
	return strings.TrimSpace(strings.Join(normalized, "\n"))
}

func isMarkdownTableRow(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|") && strings.Count(trimmed, "|") >= 2
}

func isTableSeparatorLine(line string) bool {
	trimmed := strings.Trim(line, "| ")
	if trimmed == "" {
		return false
	}
	for _, r := range trimmed {
		if r != '-' && r != ':' && r != '|' && r != ' ' {
			return false
		}
	}
	return strings.Contains(trimmed, "-")
}

func collectBlocks(node *model.CommentNodeScheme, blocks *[]string) {
	if node == nil {
		return
	}

	switch node.Type {
	case "paragraph", "heading":
		appendBlock(blocks, inlineText(node))
	case "bulletList":
		appendBlock(blocks, strings.Join(renderList(node, 0, false), "\n"))
	case "orderedList":
		appendBlock(blocks, strings.Join(renderList(node, 0, true), "\n"))
	case "listItem":
		appendBlock(blocks, inlineText(node))
	case "blockquote":
		for _, child := range node.Content {
			text := strings.TrimSpace(inlineText(child))
			if text != "" {
				appendBlock(blocks, "> "+text)
			}
		}
	case "codeBlock":
		appendBlock(blocks, renderCodeBlock(node))
	case "panel":
		appendBlock(blocks, "[panel] "+inlineText(node))
	case "table":
		appendBlock(blocks, renderTable(node))
	case "rule":
		appendBlock(blocks, "---")
	default:
		for _, child := range node.Content {
			collectBlocks(child, blocks)
		}
	}
}

func renderList(node *model.CommentNodeScheme, depth int, ordered bool) []string {
	if node == nil {
		return nil
	}
	var lines []string
	for index, child := range node.Content {
		if child == nil || child.Type != "listItem" {
			continue
		}
		itemLines := renderListItem(child, depth, ordered, index+1)
		lines = append(lines, itemLines...)
	}
	return lines
}

func renderListItem(node *model.CommentNodeScheme, depth int, ordered bool, number int) []string {
	var lines []string
	var textParts []string
	for _, child := range node.Content {
		if child == nil {
			continue
		}
		switch child.Type {
		case "bulletList":
			if text := strings.TrimSpace(strings.Join(textParts, "")); text != "" {
				lines = append(lines, renderListItemLine(text, depth, ordered, number))
				textParts = nil
			}
			lines = append(lines, renderList(child, depth+1, false)...)
		case "orderedList":
			if text := strings.TrimSpace(strings.Join(textParts, "")); text != "" {
				lines = append(lines, renderListItemLine(text, depth, ordered, number))
				textParts = nil
			}
			lines = append(lines, renderList(child, depth+1, true)...)
		default:
			textParts = append(textParts, inlineText(child))
		}
	}
	if text := strings.TrimSpace(strings.Join(textParts, "")); text != "" {
		lines = append(lines, renderListItemLine(text, depth, ordered, number))
	}
	return lines
}

func renderListItemLine(text string, depth int, ordered bool, number int) string {
	prefix := "- "
	if ordered {
		prefix = fmt.Sprintf("%d. ", number)
	}
	return strings.Repeat("  ", depth) + prefix + text
}

func appendBlock(blocks *[]string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	*blocks = append(*blocks, value)
}

func inlineText(node *model.CommentNodeScheme) string {
	if node == nil {
		return ""
	}

	switch node.Type {
	case "hardBreak":
		return "\n"
	case "mention":
		return mentionText(node)
	case "emoji":
		return attrString(node, "shortName")
	case "inlineCard", "blockCard":
		if url := attrString(node, "url"); url != "" {
			return url
		}
	case "status":
		return "[" + attrString(node, "text") + "]"
	}

	text := node.Text
	if text == "" {
		parts := make([]string, 0, len(node.Content))
		for _, child := range node.Content {
			childText := inlineText(child)
			if childText != "" {
				parts = append(parts, childText)
			}
		}
		text = strings.Join(parts, "")
	}
	return applyMarks(text, node.Marks)
}

func applyMarks(text string, marks []*model.MarkScheme) string {
	for _, mark := range marks {
		if mark == nil {
			continue
		}
		switch mark.Type {
		case "code":
			text = "`" + text + "`"
		case "link":
			href := attrStringFrom(mark.Attrs, "href")
			if href != "" {
				if mailtoAddress(href) == strings.TrimSpace(text) {
					continue
				}
				text = text + " <" + href + ">"
			}
		}
	}
	return text
}

func mailtoAddress(value string) string {
	if !strings.HasPrefix(strings.ToLower(value), "mailto:") {
		return ""
	}
	return strings.TrimSpace(value[len("mailto:"):])
}

func mentionText(node *model.CommentNodeScheme) string {
	text := attrString(node, "text")
	if text == "" {
		text = attrString(node, "displayName")
	}
	if text == "" {
		text = attrString(node, "id")
	}
	if text == "" {
		return "@unknown"
	}
	if strings.HasPrefix(text, "@") {
		return text
	}
	return "@" + text
}

func renderCodeBlock(node *model.CommentNodeScheme) string {
	text := strings.TrimRight(inlineText(node), "\n")
	if text == "" {
		return "```"
	}
	return "```\n" + text + "\n```"
}

func renderTable(node *model.CommentNodeScheme) string {
	var rows [][]string
	for _, row := range node.Content {
		if row == nil || row.Type != "tableRow" {
			continue
		}
		var cells []string
		for _, cell := range row.Content {
			if cell == nil {
				continue
			}
			cells = append(cells, tableCellText(cell))
		}
		if len(cells) > 0 {
			rows = append(rows, cells)
		}
	}
	if len(rows) == 0 {
		return ""
	}

	widths := tableWidths(rows)
	var lines []string
	for index, row := range rows {
		lines = append(lines, renderTableRow(row, widths))
		if index == 0 {
			lines = append(lines, renderTableSeparator(widths))
		}
	}
	return "[table]\n" + strings.Join(lines, "\n") + "\n[/table]"
}

func tableCellText(node *model.CommentNodeScheme) string {
	text := strings.TrimSpace(inlineText(node))
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\n", " / ")
	return strings.Join(strings.Fields(text), " ")
}

func tableWidths(rows [][]string) []int {
	maxCells := 0
	for _, row := range rows {
		if len(row) > maxCells {
			maxCells = len(row)
		}
	}
	widths := make([]int, maxCells)
	for _, row := range rows {
		for index, cell := range row {
			if len(cell) > widths[index] {
				widths[index] = len(cell)
			}
		}
	}
	return widths
}

func renderTableRow(row []string, widths []int) string {
	cells := make([]string, len(widths))
	for index := range widths {
		cell := ""
		if index < len(row) {
			cell = row[index]
		}
		cells[index] = " " + cell + strings.Repeat(" ", widths[index]-len(cell)) + " "
	}
	return "|" + strings.Join(cells, "|") + "|"
}

func renderTableSeparator(widths []int) string {
	cells := make([]string, len(widths))
	for index, width := range widths {
		cells[index] = strings.Repeat("-", width+2)
	}
	return "|" + strings.Join(cells, "|") + "|"
}

func attrString(node *model.CommentNodeScheme, key string) string {
	if node == nil {
		return ""
	}
	return attrStringFrom(node.Attrs, key)
}

func attrStringFrom(attrs map[string]interface{}, key string) string {
	if attrs == nil {
		return ""
	}
	value, ok := attrs[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprint(typed)
	}
}
