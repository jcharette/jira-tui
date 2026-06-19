package fixture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	model "github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"
)

func TestSanitizeNormalizesPrivateADFAttributes(t *testing.T) {
	node := &model.CommentNodeScheme{
		Type: "doc",
		Content: []*model.CommentNodeScheme{
			{
				Type: "paragraph",
				Content: []*model.CommentNodeScheme{
					{Type: "text", Text: "Ask "},
					{Type: "mention", Attrs: map[string]interface{}{
						"id":           "712020:private-account",
						"accountId":    "712020:private-account",
						"text":         "@Jane Private",
						"displayName":  "Jane Private",
						"emailAddress": "jane.private@example-corp.test",
					}},
					{Type: "text", Text: " to review "},
					{Type: "text", Text: "internal docs", Marks: []*model.MarkScheme{
						{Type: "link", Attrs: map[string]interface{}{"href": "https://jira.example-corp.test/wiki/private"}},
					}},
					{Type: "text", Text: "."},
				},
			},
			{
				Type:  "inlineCard",
				Attrs: map[string]interface{}{"url": "https://jira.example-corp.test/browse/DEVOPS-1"},
			},
		},
	}

	sanitized := Sanitize(node)
	data, err := json.Marshal(sanitized)
	if err != nil {
		t.Fatalf("marshal sanitized: %v", err)
	}
	value := string(data)

	for _, private := range []string{"private-account", "Jane Private", "example-corp", "DEVOPS-1"} {
		if strings.Contains(value, private) {
			t.Fatalf("sanitized output still contains %q: %s", private, value)
		}
	}
	for _, want := range []string{"account-1", "@User 1", "User 1", "user1@example.test", "https://example.test/link-1", "https://example.test/card-2"} {
		if !strings.Contains(value, want) {
			t.Fatalf("sanitized output missing %q: %s", want, value)
		}
	}
}

func TestSanitizePreservesStructureAndCodeText(t *testing.T) {
	node := &model.CommentNodeScheme{
		Type: "table",
		Content: []*model.CommentNodeScheme{
			{
				Type: "tableRow",
				Content: []*model.CommentNodeScheme{
					{Type: "tableHeader", Content: []*model.CommentNodeScheme{{Type: "paragraph", Content: []*model.CommentNodeScheme{{Type: "text", Text: "Step"}}}}},
					{Type: "tableHeader", Content: []*model.CommentNodeScheme{{Type: "paragraph", Content: []*model.CommentNodeScheme{{Type: "text", Text: "Command"}}}}},
				},
			},
			{
				Type: "tableRow",
				Content: []*model.CommentNodeScheme{
					{Type: "tableCell", Content: []*model.CommentNodeScheme{{Type: "paragraph", Content: []*model.CommentNodeScheme{{Type: "text", Text: "Verify"}}}}},
					{Type: "tableCell", Content: []*model.CommentNodeScheme{{Type: "codeBlock", Content: []*model.CommentNodeScheme{{Type: "text", Text: "go test ./internal/adf"}}}}},
				},
			},
		},
	}

	sanitized := Sanitize(node)

	if sanitized.Type != "table" || len(sanitized.Content) != 2 {
		t.Fatalf("sanitized structure = %#v", sanitized)
	}
	code := sanitized.Content[1].Content[1].Content[0].Content[0].Text
	if code != "go test ./internal/adf" {
		t.Fatalf("code text = %q", code)
	}
}

func TestWriteSanitizedWritesDeterministicFormattedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.adf.json")
	node := &model.CommentNodeScheme{
		Type: "paragraph",
		Content: []*model.CommentNodeScheme{
			{Type: "inlineCard", Attrs: map[string]interface{}{"url": "https://jira.example-corp.test/browse/DEVOPS-2"}},
		},
	}

	if err := WriteSanitized(path, node); err != nil {
		t.Fatalf("WriteSanitized() error = %v", err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read first: %v", err)
	}
	if err := WriteSanitized(path, node); err != nil {
		t.Fatalf("WriteSanitized() second error = %v", err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read second: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("output not deterministic:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	if !strings.HasSuffix(string(first), "\n") || !strings.Contains(string(first), "\n  ") {
		t.Fatalf("expected formatted JSON with trailing newline, got %q", first)
	}
}
