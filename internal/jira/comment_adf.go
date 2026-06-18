package jira

import (
	"sort"
	"strings"

	model "github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"
	"github.com/jcharette/jira-tui/internal/linkdetect"
)

type inlineSpan struct {
	start   int
	end     int
	kind    string
	href    string
	mention Mention
}

func plainTextADF(value string, mentions []Mention) *model.CommentNodeScheme {
	doc := &model.CommentNodeScheme{
		Version: 1,
		Type:    "doc",
	}
	paragraphs := splitParagraphs(value)
	for _, paragraph := range paragraphs {
		node := &model.CommentNodeScheme{
			Type: "paragraph",
		}
		lines := strings.Split(paragraph, "\n")
		for index, line := range lines {
			if index > 0 {
				node.Content = append(node.Content, &model.CommentNodeScheme{Type: "hardBreak"})
			}
			if line != "" {
				node.Content = append(node.Content, textNodesWithLinksAndMentions(line, mentions)...)
			}
		}
		doc.Content = append(doc.Content, node)
	}
	return doc
}

func textNodesWithLinks(line string) []*model.CommentNodeScheme {
	return textNodesWithLinksAndMentions(line, nil)
}

func textNodesWithLinksAndMentions(line string, mentions []Mention) []*model.CommentNodeScheme {
	spans := inlineSpans(line, mentions)
	if len(spans) == 0 {
		return markedTextNodes(line, nil)
	}

	nodes := make([]*model.CommentNodeScheme, 0, len(spans)*2+1)
	offset := 0
	for _, span := range spans {
		if span.start < offset || span.end > len(line) || span.start >= span.end {
			continue
		}
		if span.start > offset {
			nodes = append(nodes, markedTextNodes(line[offset:span.start], nil)...)
		}
		switch span.kind {
		case "mention":
			nodes = append(nodes, mentionNode(span.mention))
		case "link":
			nodes = append(nodes, linkTextNode(line[span.start:span.end], span.href))
		}
		offset = span.end
	}
	if offset < len(line) {
		nodes = append(nodes, markedTextNodes(line[offset:], nil)...)
	}
	if len(nodes) == 0 {
		return markedTextNodes(line, nil)
	}
	return nodes
}

func inlineSpans(line string, mentions []Mention) []inlineSpan {
	var spans []inlineSpan
	occupied := make([]inlineSpan, 0, len(mentions))
	for _, mention := range mentions {
		if mention.AccountID == "" || mention.Text == "" {
			continue
		}
		searchFrom := 0
		for {
			relative := strings.Index(line[searchFrom:], mention.Text)
			if relative < 0 {
				break
			}
			start := searchFrom + relative
			end := start + len(mention.Text)
			span := inlineSpan{start: start, end: end, kind: "mention", mention: mention}
			if !overlapsAny(span, occupied) {
				spans = append(spans, span)
				occupied = append(occupied, span)
			}
			searchFrom = end
		}
	}
	for _, link := range linkdetect.Detect(line) {
		span := inlineSpan{start: link.Start, end: link.End, kind: "link", href: linkHref(link)}
		if span.start < span.end && span.end <= len(line) && !overlapsAny(span, occupied) {
			spans = append(spans, span)
		}
	}
	sort.SliceStable(spans, func(i, j int) bool {
		return spans[i].start < spans[j].start
	})
	return spans
}

func overlapsAny(span inlineSpan, spans []inlineSpan) bool {
	for _, existing := range spans {
		if span.start < existing.end && existing.start < span.end {
			return true
		}
	}
	return false
}

func textNode(text string) *model.CommentNodeScheme {
	return textNodeWithMarks(text, nil)
}

func textNodeWithMarks(text string, marks []*model.MarkScheme) *model.CommentNodeScheme {
	node := &model.CommentNodeScheme{
		Type: "text",
		Text: text,
	}
	if len(marks) > 0 {
		node.Marks = append([]*model.MarkScheme(nil), marks...)
	}
	return node
}

func markedTextNodes(text string, baseMarks []*model.MarkScheme) []*model.CommentNodeScheme {
	if text == "" {
		return nil
	}
	markers := []struct {
		open string
		mark string
	}{
		{open: "`", mark: "code"},
		{open: "**", mark: "strong"},
		{open: "_", mark: "em"},
	}
	bestStart := -1
	bestEnd := -1
	bestMarker := markers[0]
	for _, marker := range markers {
		start := strings.Index(text, marker.open)
		if start < 0 {
			continue
		}
		end := strings.Index(text[start+len(marker.open):], marker.open)
		if end < 0 {
			continue
		}
		end = start + len(marker.open) + end
		if end == start+len(marker.open) {
			continue
		}
		if bestStart < 0 || start < bestStart || (start == bestStart && len(marker.open) > len(bestMarker.open)) {
			bestStart = start
			bestEnd = end
			bestMarker = marker
		}
	}
	if bestStart < 0 {
		return []*model.CommentNodeScheme{textNodeWithMarks(text, baseMarks)}
	}
	nodes := make([]*model.CommentNodeScheme, 0, 3)
	if bestStart > 0 {
		nodes = append(nodes, markedTextNodes(text[:bestStart], baseMarks)...)
	}
	innerStart := bestStart + len(bestMarker.open)
	inner := text[innerStart:bestEnd]
	marks := append([]*model.MarkScheme(nil), baseMarks...)
	marks = append(marks, &model.MarkScheme{Type: bestMarker.mark})
	if bestMarker.mark == "code" {
		nodes = append(nodes, textNodeWithMarks(inner, marks))
	} else {
		nodes = append(nodes, markedTextNodes(inner, marks)...)
	}
	restStart := bestEnd + len(bestMarker.open)
	if restStart < len(text) {
		nodes = append(nodes, markedTextNodes(text[restStart:], baseMarks)...)
	}
	return nodes
}

func linkTextNode(text string, href string) *model.CommentNodeScheme {
	return &model.CommentNodeScheme{
		Type: "text",
		Text: text,
		Marks: []*model.MarkScheme{
			{
				Type: "link",
				Attrs: map[string]interface{}{
					"href": href,
				},
			},
		},
	}
}

func mentionNode(mention Mention) *model.CommentNodeScheme {
	return &model.CommentNodeScheme{
		Type: "mention",
		Attrs: map[string]interface{}{
			"id":       mention.AccountID,
			"text":     mention.Text,
			"userType": "DEFAULT",
		},
	}
}

func linkHref(link linkdetect.Link) string {
	if link.Kind == linkdetect.KindEmail {
		return link.Target
	}
	if strings.Contains(link.Target, "://") {
		return link.Target
	}
	return "https://" + link.Target
}

func splitParagraphs(value string) []string {
	normalized := strings.ReplaceAll(value, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	blocks := strings.Split(normalized, "\n\n")
	paragraphs := make([]string, 0, len(blocks))
	for _, block := range blocks {
		block = strings.Trim(block, "\n")
		if strings.TrimSpace(block) != "" {
			paragraphs = append(paragraphs, block)
		}
	}
	if len(paragraphs) == 0 {
		return []string{strings.TrimSpace(value)}
	}
	return paragraphs
}
