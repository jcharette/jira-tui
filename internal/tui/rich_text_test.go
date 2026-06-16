package tui

import (
	"strings"
	"testing"
	"time"
)

func TestWrapRichTextPreservesParagraphsAndListIndent(t *testing.T) {
	got := wrapRichText("Need: this is a longer sentence that should wrap cleanly.\n\n- first item has enough words to wrap onto a continuation line", 28)
	want := "Need: this is a longer\nsentence that should wrap\ncleanly.\n\n- first item has enough\n  words to wrap onto a\n  continuation line"
	if got != want {
		t.Fatalf("wrapped = %q", got)
	}
}

func TestRenderRichDescriptionStylesInlineCode(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()

	rendered := model.renderRichDescriptionBody("Use `locals.tf` and `main.tf`.", 80)

	if strings.Contains(rendered, "`locals.tf`") {
		t.Fatalf("expected inline code markers to be styled away: %q", rendered)
	}
	if !strings.Contains(rendered, "locals.tf") || !strings.Contains(rendered, "main.tf") {
		t.Fatalf("rendered = %q", rendered)
	}
	if strings.Contains(rendered, "main.tf .") {
		t.Fatalf("inline code styling should not add padding before punctuation: %q", rendered)
	}
}

func TestRenderRichDescriptionFormatsCodeBlock(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()

	rendered := model.renderRichDescriptionBody("```\n{\"Sid\":\"DenyS3Deletes\"}\n```", 40)

	if strings.Contains(rendered, "```") {
		t.Fatalf("expected code fences to be styled away: %q", rendered)
	}
	if !strings.Contains(rendered, "{\"Sid\":\"DenyS3Deletes\"}") {
		t.Fatalf("rendered = %q", rendered)
	}
	for _, unwanted := range []string{"+--------------------------------------+", "| {\"Sid\":\"DenyS3Deletes\"}", "│"} {
		if strings.Contains(rendered, unwanted) {
			t.Fatalf("expected compact code styling without ASCII borders, rendered = %q", rendered)
		}
	}
}

func TestRenderRichDescriptionDoesNotLeaveExtraBlankLinesBeforeCodeBlock(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()

	rendered := model.renderRichDescriptionBody("The failure is:\n\n```\n\nError: missing resource\n\n```", 40)

	if strings.Contains(rendered, "\n\n\n") {
		t.Fatalf("expected code block spacing to be collapsed, rendered = %q", rendered)
	}
	if strings.Contains(rendered, "|                                      |") {
		t.Fatalf("expected leading/trailing blank code lines to be trimmed, rendered = %q", rendered)
	}
	if !strings.Contains(rendered, "The failure is:\n\n") || strings.Contains(rendered, "\n\n\n") {
		t.Fatalf("expected one separator before code block, rendered = %q", rendered)
	}
	for _, unwanted := range []string{"+--------------------------------------+", "| Error: missing resource", "│"} {
		if strings.Contains(rendered, unwanted) {
			t.Fatalf("expected compact code styling without ASCII borders, rendered = %q", rendered)
		}
	}
}

func TestRenderRichDescriptionUsesLipglossTable(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()

	rendered := model.renderRichDescriptionBody("[table]\n| Field | Value |\n|-------|-------|\n| Status | Ready |\n[/table]", 60)

	if strings.Contains(rendered, "[table]") || strings.Contains(rendered, "|-------|") {
		t.Fatalf("expected semantic table markers and ASCII separator to be styled away: %q", rendered)
	}
	for _, want := range []string{"Field", "Value", "Status", "Ready"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("missing %q in %q", want, rendered)
		}
	}
	if !strings.Contains(rendered, "╭") || !strings.Contains(rendered, "│") {
		t.Fatalf("expected lipgloss rounded table border, rendered = %q", rendered)
	}
}

func TestRenderRichDescriptionStylesPanelAndStatusMarkers(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()

	rendered := model.renderRichDescriptionBody("[panel] [BLOCKED] Roll back the deploy before retrying.", 72)

	for _, unwanted := range []string{"[panel]", "[BLOCKED]"} {
		if strings.Contains(rendered, unwanted) {
			t.Fatalf("expected ADF marker %q to be styled away: %q", unwanted, rendered)
		}
	}
	for _, want := range []string{"Panel", "BLOCKED", "Roll back the deploy"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("missing %q in %q", want, rendered)
		}
	}
}

func TestRenderDescriptionSeparatesHeaderFromRichBody(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()

	rendered := model.renderDescription("Intro paragraph.\n\n- first item\n\n```\nterraform plan\n```", 80)

	lines := strings.Split(rendered, "\n")
	bodyLine := -1
	for index, line := range lines {
		if strings.Contains(line, "Intro paragraph.") {
			bodyLine = index
			break
		}
	}
	if bodyLine < 2 || strings.TrimSpace(lines[bodyLine-1]) != "" {
		t.Fatalf("expected blank line between description header and body, rendered = %q", rendered)
	}
	for _, want := range []string{"- first item", "terraform plan"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("missing %q in %q", want, rendered)
		}
	}
}

func TestDetailStatesRenderConsistentStatusBlocks(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()

	description := model.renderDescriptionState("Loading issue detail...", 80, false)
	comments := model.renderComments("ABC-1", 80)

	for _, rendered := range []string{description, comments} {
		for _, want := range []string{"Status", "──"} {
			if !strings.Contains(rendered, want) {
				t.Fatalf("missing %q in %q", want, rendered)
			}
		}
	}
	if !strings.Contains(description, "Loading issue detail...") {
		t.Fatalf("missing description state message in %q", description)
	}
	if !strings.Contains(comments, "Comments not loaded.") {
		t.Fatalf("missing comments state message in %q", comments)
	}
}

func TestCollectDetailLinksFindsURLsMailtoAndEmails(t *testing.T) {
	links := collectDetailLinks("Failed run: https://example.test/run/1. Contact ops@example.test or mailto:oncall@example.test. Read https://example.test/run/1")

	want := []detailLink{
		{Kind: "URL", Label: "https://example.test/run/1", Target: "https://example.test/run/1", Start: 12, End: 38},
		{Kind: "Email", Label: "ops@example.test", Target: "mailto:ops@example.test", Start: 48, End: 64},
		{Kind: "Email", Label: "oncall@example.test", Target: "mailto:oncall@example.test", Start: 68, End: 94},
	}
	if len(links) != len(want) {
		t.Fatalf("links = %#v", links)
	}
	for index := range want {
		if links[index] != want[index] {
			t.Fatalf("links[%d] = %#v, want %#v", index, links[index], want[index])
		}
	}
}

func TestCommentBlockLeadsWithAuthorAndSeparatesBody(t *testing.T) {
	model := NewModel(&fakeIssueSearcher{}, "project = ABC")
	defer model.workers.Stop()
	created := time.Date(2026, 6, 13, 10, 15, 0, 0, time.Local)

	rendered := model.renderCommentBlock(2, 4, "Comment Person", formatTime(created), "Please check `main.tf`.", 80)

	authorIndex := strings.Index(rendered, "Comment Person")
	countIndex := strings.Index(rendered, "Comment 2/4")
	if authorIndex < 0 || countIndex < 0 {
		t.Fatalf("expected author and count in %q", rendered)
	}
	if authorIndex > countIndex {
		t.Fatalf("author should lead comment header, rendered = %q", rendered)
	}
	if !strings.Contains(rendered, "2026-06-13 10:15") {
		t.Fatalf("missing created timestamp in %q", rendered)
	}
	lines := strings.Split(rendered, "\n")
	bodyLine := -1
	for index, line := range lines {
		if strings.Contains(line, "Please check") {
			bodyLine = index
			break
		}
	}
	if bodyLine < 2 || strings.TrimSpace(strings.TrimPrefix(lines[bodyLine-1], "│")) != "" {
		t.Fatalf("expected blank line between comment header and body, rendered = %q", rendered)
	}
}

func TestWrapRichTextPreservesTableRows(t *testing.T) {
	got := wrapRichText("[table]\n| Module block | Workspace name pattern | Affected? |\n|--------------|------------------------|-----------|\n| stream_processing_1 | ${env}-dpmetadata-stream-instance | Yes |\n[/table]", 46)
	want := "[table]\n| Module block | Workspace name  | Affected? |\n|              | pattern         |           |\n|--------------|-----------------|-----------|\n| stream_pr... | ${env}-dpmet... | Yes       |\n[/table]"
	if got != want {
		t.Fatalf("wrapped table = %q", got)
	}
}
