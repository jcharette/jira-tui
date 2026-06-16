package fixture

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	model "github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"
)

type sanitizer struct {
	accounts map[string]int
	links    map[string]int
}

// Sanitize returns a copy of an ADF node with private identifiers and URLs
// replaced by stable placeholders suitable for checked-in fixtures.
func Sanitize(node *model.CommentNodeScheme) *model.CommentNodeScheme {
	s := sanitizer{
		accounts: map[string]int{},
		links:    map[string]int{},
	}
	return s.node(node)
}

// WriteSanitized writes a deterministic, formatted sanitized ADF fixture.
func WriteSanitized(path string, node *model.CommentNodeScheme) error {
	sanitized := Sanitize(node)
	data, err := json.MarshalIndent(sanitized, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sanitized ADF: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write fixture: %w", err)
	}
	return nil
}

func (s sanitizer) node(node *model.CommentNodeScheme) *model.CommentNodeScheme {
	if node == nil {
		return nil
	}
	copied := *node
	copied.Attrs = s.attrs(node.Type, node.Attrs)
	copied.Marks = make([]*model.MarkScheme, 0, len(node.Marks))
	for _, mark := range node.Marks {
		if mark == nil {
			continue
		}
		markCopy := *mark
		markCopy.Attrs = s.attrs(mark.Type, mark.Attrs)
		copied.Marks = append(copied.Marks, &markCopy)
	}
	copied.Content = make([]*model.CommentNodeScheme, 0, len(node.Content))
	for _, child := range node.Content {
		copied.Content = append(copied.Content, s.node(child))
	}
	return &copied
}

func (s sanitizer) attrs(owner string, attrs map[string]interface{}) map[string]interface{} {
	if len(attrs) == 0 {
		return nil
	}
	copied := make(map[string]interface{}, len(attrs))
	accountPlaceholder := ""
	if strings.EqualFold(owner, "mention") {
		accountPlaceholder = s.accountPlaceholder(attrs)
	}
	for key, value := range attrs {
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "id", "accountid":
			if accountPlaceholder != "" {
				copied[key] = accountPlaceholder
			} else {
				copied[key] = value
			}
		case "text":
			if accountPlaceholder != "" {
				copied[key] = "@" + displayNameForAccount(accountPlaceholder)
			} else {
				copied[key] = value
			}
		case "displayname":
			if accountPlaceholder != "" {
				copied[key] = displayNameForAccount(accountPlaceholder)
			} else {
				copied[key] = value
			}
		case "emailaddress":
			if accountPlaceholder != "" {
				copied[key] = emailForAccount(accountPlaceholder)
			} else {
				copied[key] = value
			}
		case "href":
			copied[key] = s.linkPlaceholder(value, "link")
		case "url":
			copied[key] = s.linkPlaceholder(value, "card")
		default:
			copied[key] = value
		}
	}
	return copied
}

func (s sanitizer) accountPlaceholder(attrs map[string]interface{}) string {
	for _, key := range []string{"accountId", "id", "text", "displayName", "emailAddress"} {
		value := strings.TrimSpace(fmt.Sprint(attrs[key]))
		if value == "" {
			continue
		}
		if existing, ok := s.accounts[value]; ok {
			return fmt.Sprintf("account-%d", existing)
		}
		next := len(s.accounts) + 1
		s.accounts[value] = next
		return fmt.Sprintf("account-%d", next)
	}
	return ""
}

func displayNameForAccount(account string) string {
	return "User " + strings.TrimPrefix(account, "account-")
}

func emailForAccount(account string) string {
	return "user" + strings.TrimPrefix(account, "account-") + "@example.test"
}

func (s sanitizer) linkPlaceholder(value interface{}, kind string) string {
	raw := strings.TrimSpace(fmt.Sprint(value))
	if raw == "" {
		return raw
	}
	if existing, ok := s.links[raw]; ok {
		return fmt.Sprintf("https://example.test/%s-%d", kind, existing)
	}
	next := len(s.links) + 1
	s.links[raw] = next
	if parsed, err := url.Parse(raw); err == nil && parsed.Scheme == "mailto" {
		return fmt.Sprintf("mailto:user%d@example.test", next)
	}
	return fmt.Sprintf("https://example.test/%s-%d", kind, next)
}
