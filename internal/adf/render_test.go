package adf

import (
	"testing"

	model "github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"
)

func TestRenderPreservesBreaksAndLists(t *testing.T) {
	text := Render(&model.CommentNodeScheme{
		Type: "doc",
		Content: []*model.CommentNodeScheme{
			{
				Type: "paragraph",
				Content: []*model.CommentNodeScheme{
					{Type: "text", Text: "Need: first sentence."},
					{Type: "hardBreak"},
					{Type: "text", Text: "Scope: second sentence."},
				},
			},
			{
				Type: "bulletList",
				Content: []*model.CommentNodeScheme{
					listItem("first item"),
					listItem("second item"),
				},
			},
		},
	})

	want := "Need: first sentence.\nScope: second sentence.\n- first item\n- second item"
	if text != want {
		t.Fatalf("text = %q", text)
	}
}

func TestRenderInlineMarksLinksAndMentions(t *testing.T) {
	text := Render(&model.CommentNodeScheme{
		Type: "paragraph",
		Content: []*model.CommentNodeScheme{
			{Type: "text", Text: "Ask "},
			{Type: "mention", Attrs: map[string]interface{}{"text": "Jane Doe"}},
			{Type: "text", Text: " to run "},
			{Type: "text", Text: "make check", Marks: []*model.MarkScheme{{Type: "code"}}},
			{Type: "text", Text: " and read "},
			{Type: "text", Text: "docs", Marks: []*model.MarkScheme{{Type: "link", Attrs: map[string]interface{}{"href": "https://example.test/docs"}}}},
			{Type: "text", Text: "."},
		},
	})

	want := "Ask @Jane Doe to run `make check` and read docs <https://example.test/docs>."
	if text != want {
		t.Fatalf("text = %q", text)
	}
}

func TestRenderMailtoLinkDoesNotDuplicateVisibleEmail(t *testing.T) {
	text := Render(&model.CommentNodeScheme{
		Type: "paragraph",
		Content: []*model.CommentNodeScheme{
			{Type: "text", Text: "ops@example.test", Marks: []*model.MarkScheme{{Type: "link", Attrs: map[string]interface{}{"href": "mailto:ops@example.test"}}}},
		},
	})

	if text != "ops@example.test" {
		t.Fatalf("text = %q", text)
	}
}

func TestRenderTable(t *testing.T) {
	text := Render(&model.CommentNodeScheme{
		Type: "table",
		Content: []*model.CommentNodeScheme{
			tableRow("Field", "Value"),
			tableRow("Status", "Ready"),
		},
	})

	want := "[table]\n| Field  | Value |\n|--------|-------|\n| Status | Ready |\n[/table]"
	if text != want {
		t.Fatalf("text = %q", text)
	}
}

func TestRenderNestedListsFixture(t *testing.T) {
	text := Render(&model.CommentNodeScheme{
		Type: "bulletList",
		Content: []*model.CommentNodeScheme{
			{
				Type: "listItem",
				Content: []*model.CommentNodeScheme{
					paragraph("Parent item"),
					{
						Type: "bulletList",
						Content: []*model.CommentNodeScheme{
							listItem("child one"),
							listItem("child two"),
						},
					},
				},
			},
			listItem("Sibling item"),
		},
	})

	want := "- Parent item\n  - child one\n  - child two\n- Sibling item"
	if text != want {
		t.Fatalf("text = %q", text)
	}
}

func TestRenderTableFixtureWithRichCells(t *testing.T) {
	text := Render(&model.CommentNodeScheme{
		Type: "table",
		Content: []*model.CommentNodeScheme{
			tableRow("Workspace", "Affected?"),
			{
				Type: "tableRow",
				Content: []*model.CommentNodeScheme{
					tableCell(
						&model.CommentNodeScheme{Type: "text", Text: "stream_processing"},
						&model.CommentNodeScheme{Type: "hardBreak"},
						&model.CommentNodeScheme{Type: "text", Text: "module.tf", Marks: []*model.MarkScheme{{Type: "code"}}},
					),
					tableCell(
						&model.CommentNodeScheme{Type: "mention", Attrs: map[string]interface{}{"text": "@Jane Doe"}},
						&model.CommentNodeScheme{Type: "text", Text: " confirmed in "},
						&model.CommentNodeScheme{Type: "text", Text: "Jira", Marks: []*model.MarkScheme{{Type: "link", Attrs: map[string]interface{}{"href": "https://example.test/JIRA-1"}}}},
					),
				},
			},
		},
	})

	want := "[table]\n" +
		"| Workspace                       | Affected?                                                 |\n" +
		"|---------------------------------|-----------------------------------------------------------|\n" +
		"| stream_processing / `module.tf` | @Jane Doe confirmed in Jira <https://example.test/JIRA-1> |\n" +
		"[/table]"
	if text != want {
		t.Fatalf("text = %q", text)
	}
}

func listItem(text string) *model.CommentNodeScheme {
	return &model.CommentNodeScheme{
		Type: "listItem",
		Content: []*model.CommentNodeScheme{
			{
				Type: "paragraph",
				Content: []*model.CommentNodeScheme{
					{Type: "text", Text: text},
				},
			},
		},
	}
}

func paragraph(text string) *model.CommentNodeScheme {
	return &model.CommentNodeScheme{
		Type: "paragraph",
		Content: []*model.CommentNodeScheme{
			{Type: "text", Text: text},
		},
	}
}

func tableCell(content ...*model.CommentNodeScheme) *model.CommentNodeScheme {
	return &model.CommentNodeScheme{
		Type: "tableCell",
		Content: []*model.CommentNodeScheme{
			{
				Type:    "paragraph",
				Content: content,
			},
		},
	}
}

func tableRow(values ...string) *model.CommentNodeScheme {
	row := &model.CommentNodeScheme{Type: "tableRow"}
	for _, value := range values {
		row.Content = append(row.Content, tableCell(&model.CommentNodeScheme{Type: "text", Text: value}))
	}
	return row
}
