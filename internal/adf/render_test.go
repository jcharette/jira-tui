package adf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

	want := "Need: first sentence.\nScope: second sentence.\n\n- first item\n- second item"
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

	want := "Ask @Jane Doe to run `make check` and read [docs](https://example.test/docs)."
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

	want := "- Parent item\n\n  - child one\n  - child two\n- Sibling item"
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

func TestRenderRealisticJiraDescriptionFixture(t *testing.T) {
	text := Render(&model.CommentNodeScheme{
		Type: "doc",
		Content: []*model.CommentNodeScheme{
			{
				Type:  "heading",
				Attrs: map[string]interface{}{"level": 2},
				Content: []*model.CommentNodeScheme{
					{Type: "text", Text: "Implementation Notes"},
				},
			},
			{
				Type: "paragraph",
				Content: []*model.CommentNodeScheme{
					{Type: "text", Text: "Ask "},
					{Type: "mention", Attrs: map[string]interface{}{"id": "abc", "text": "@Jane Doe"}},
					{Type: "text", Text: " to run "},
					{Type: "text", Text: "make check", Marks: []*model.MarkScheme{{Type: "code"}}},
					{Type: "text", Text: " and review "},
					{Type: "text", Text: "docs", Marks: []*model.MarkScheme{{Type: "link", Attrs: map[string]interface{}{"href": "https://example.test/docs"}}}},
					{Type: "text", Text: "."},
				},
			},
			{
				Type:  "codeBlock",
				Attrs: map[string]interface{}{"language": "go"},
				Content: []*model.CommentNodeScheme{
					{Type: "text", Text: "go test ./internal/adf"},
				},
			},
			{
				Type:  "panel",
				Attrs: map[string]interface{}{"panelType": "warning"},
				Content: []*model.CommentNodeScheme{
					{
						Type: "paragraph",
						Content: []*model.CommentNodeScheme{
							{Type: "status", Attrs: map[string]interface{}{"text": "BLOCKED", "color": "red"}},
							{Type: "text", Text: " waiting on API response."},
						},
					},
				},
			},
			{
				Type: "table",
				Content: []*model.CommentNodeScheme{
					{
						Type: "tableRow",
						Content: []*model.CommentNodeScheme{
							tableHeader("Field"),
							tableHeader("Value"),
						},
					},
					{
						Type: "tableRow",
						Content: []*model.CommentNodeScheme{
							tableCell(&model.CommentNodeScheme{Type: "text", Text: "Path"}),
							tableCell(&model.CommentNodeScheme{Type: "text", Text: "internal/adf/render.go", Marks: []*model.MarkScheme{{Type: "code"}}}),
						},
					},
				},
			},
		},
	})

	want := "## Implementation Notes\n\n" +
		"Ask @Jane Doe to run `make check` and review [docs](https://example.test/docs).\n\n" +
		"```\n" +
		"go test ./internal/adf\n" +
		"```\n\n" +
		"> [!WARNING]\n" +
		"> [Status: BLOCKED] waiting on API response.\n\n" +
		"[table]\n" +
		"| Field | Value                    |\n" +
		"|-------|--------------------------|\n" +
		"| Path  | `internal/adf/render.go` |\n" +
		"[/table]"
	if text != want {
		t.Fatalf("text = %q", text)
	}
}

func TestRenderJiraExtendedNodesFixture(t *testing.T) {
	text := Render(&model.CommentNodeScheme{
		Type: "doc",
		Content: []*model.CommentNodeScheme{
			{
				Type: "paragraph",
				Content: []*model.CommentNodeScheme{
					{Type: "text", Text: "Due "},
					{Type: "date", Attrs: map[string]interface{}{"timestamp": "1767139200000"}},
					{Type: "text", Text: " "},
					{Type: "emoji", Attrs: map[string]interface{}{"shortName": ":white_check_mark:"}},
				},
			},
			{
				Type: "paragraph",
				Content: []*model.CommentNodeScheme{
					{Type: "inlineCard", Attrs: map[string]interface{}{"url": "https://example.test/card"}},
				},
			},
			{
				Type:  "blockCard",
				Attrs: map[string]interface{}{"url": "https://example.test/block"},
			},
			{
				Type:  "expand",
				Attrs: map[string]interface{}{"title": "More context"},
				Content: []*model.CommentNodeScheme{
					paragraph("Hidden details"),
				},
			},
		},
	})

	want := "Due 2025-12-31 :white_check_mark:\n\n" +
		"[https://example.test/card](https://example.test/card)\n\n" +
		"[https://example.test/block](https://example.test/block)\n\n" +
		"> **More context**\n" +
		">\n" +
		"> Hidden details"
	if text != want {
		t.Fatalf("text = %q", text)
	}
}

func TestRenderJSONFixtures(t *testing.T) {
	fixtures := []string{
		"implementation-notes",
		"extended-nodes",
		"long-table-nested-notes",
		"real-description",
		"real-comment-code",
	}

	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			node := readADFJSONFixture(t, name)
			want := readGoldenFixture(t, name)

			if got := Render(node); got != want {
				t.Fatalf("Render(%s) =\n%s\n\nwant:\n%s", name, got, want)
			}
		})
	}
}

func readADFJSONFixture(t *testing.T, name string) *model.CommentNodeScheme {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", name+".adf.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var node model.CommentNodeScheme
	if err := json.Unmarshal(data, &node); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	return &node
}

func readGoldenFixture(t *testing.T, name string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", name+".golden"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	return strings.TrimRight(string(data), "\n")
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

func tableHeader(value string) *model.CommentNodeScheme {
	return &model.CommentNodeScheme{
		Type: "tableHeader",
		Content: []*model.CommentNodeScheme{
			{
				Type: "paragraph",
				Content: []*model.CommentNodeScheme{
					{Type: "text", Text: value},
				},
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
