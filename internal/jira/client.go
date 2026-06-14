package jira

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	atlassian "github.com/ctreminiom/go-atlassian/v2/jira/v3"
	model "github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"
	jiraservice "github.com/ctreminiom/go-atlassian/v2/service/jira"
	"github.com/jon/jira-tui/internal/adf"
	"github.com/jon/jira-tui/internal/config"
	"github.com/jon/jira-tui/internal/linkdetect"
)

const defaultMaxResults = 25

type Client struct {
	baseURL        string
	search         issueSearchService
	issue          issueService
	comment        commentService
	userSearch     userSearchService
	requestTimeout time.Duration
}

type issueSearchService interface {
	SearchJQL(ctx context.Context, jql string, fields, expands []string, maxResults int, nextPageToken string) (*model.IssueSearchJQLScheme, *model.ResponseScheme, error)
}

type issueService interface {
	Get(ctx context.Context, issueKeyOrID string, fields, expand []string) (*model.IssueScheme, *model.ResponseScheme, error)
}

type commentService interface {
	Gets(ctx context.Context, issueKeyOrID, orderBy string, expand []string, startAt, maxResults int) (*model.IssueCommentPageScheme, *model.ResponseScheme, error)
	Add(ctx context.Context, issueKeyOrID string, payload *model.CommentPayloadScheme, expand []string) (*model.IssueCommentScheme, *model.ResponseScheme, error)
}

type userSearchService interface {
	Do(ctx context.Context, accountID, query string, startAt, maxResults int) ([]*model.UserScheme, *model.ResponseScheme, error)
}

type Issue struct {
	Key            string
	Summary        string
	Status         string
	Assignee       string
	Priority       string
	IssueType      string
	IsSubtask      bool
	HierarchyLevel int
	ParentKey      string
	ParentSummary  string
	SubtaskCount   int
	Subtasks       []Issue
	URL            string
}

type IssueDetail struct {
	Issue
	Description string
	Reporter    string
	Creator     string
	Labels      []string
	Components  []string
	FixVersions []string
	Created     time.Time
	Updated     time.Time
}

type Comment struct {
	ID      string
	Author  string
	Body    string
	Created time.Time
	Updated time.Time
}

type User struct {
	AccountID   string
	DisplayName string
	Email       string
	Active      bool
}

type Mention struct {
	AccountID string
	Text      string
}

func NewClient(cfg config.Config) *Client {
	requestTimeout := cfg.RequestTimeout
	if requestTimeout <= 0 {
		requestTimeout = 20 * time.Second
	}
	httpClient := &http.Client{
		Timeout: requestTimeout,
	}

	api, err := atlassian.New(httpClient, cfg.BaseURL)
	if err != nil {
		return &Client{
			baseURL:        cfg.BaseURL,
			search:         failingSearchService{err: err},
			issue:          failingIssueService{err: err},
			comment:        failingCommentService{err: err},
			userSearch:     failingUserSearchService{err: err},
			requestTimeout: requestTimeout,
		}
	}
	api.Auth.SetBasicAuth(cfg.Email, cfg.APIToken)

	return newClient(cfg.BaseURL, api.Issue.Search, api.Issue, api.Issue.Comment, api.User.Search, requestTimeout)
}

func newClient(baseURL string, search jiraservice.SearchADFConnector, issue jiraservice.IssueADFConnector, comment commentService, userSearch userSearchService, requestTimeout time.Duration) *Client {
	return &Client{
		baseURL:        baseURL,
		search:         search,
		issue:          issue,
		comment:        comment,
		userSearch:     userSearch,
		requestTimeout: requestTimeout,
	}
}

func (c *Client) SearchIssues(ctx context.Context, jql string, maxResults int) ([]Issue, error) {
	if maxResults <= 0 {
		maxResults = defaultMaxResults
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	fields := []string{"summary", "status", "assignee", "priority", "issuetype", "parent", "subtasks"}
	response, _, err := c.search.SearchJQL(ctx, jql, fields, nil, maxResults, "")
	if err != nil {
		return nil, fmt.Errorf("search jira issues: %w", err)
	}
	if response == nil {
		return nil, fmt.Errorf("search jira issues: empty response")
	}

	issues := make([]Issue, 0, len(response.Issues))
	for _, raw := range response.Issues {
		issues = append(issues, c.parseIssue(raw))
	}
	return issues, nil
}

func (c *Client) GetIssue(ctx context.Context, key string) (IssueDetail, error) {
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	fields := []string{
		"summary",
		"status",
		"assignee",
		"priority",
		"issuetype",
		"parent",
		"subtasks",
		"description",
		"labels",
		"components",
		"fixVersions",
		"created",
		"updated",
		"reporter",
		"creator",
	}
	raw, _, err := c.issue.Get(ctx, key, fields, nil)
	if err != nil {
		return IssueDetail{}, fmt.Errorf("get jira issue %s: %w", key, err)
	}
	if raw == nil {
		return IssueDetail{}, fmt.Errorf("get jira issue %s: empty response", key)
	}
	return c.parseIssueDetail(raw), nil
}

func (c *Client) GetComments(ctx context.Context, key string, maxResults int) ([]Comment, error) {
	if maxResults <= 0 {
		maxResults = 10
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	if c.comment == nil {
		return nil, fmt.Errorf("get jira comments %s: comment service unavailable", key)
	}

	response, _, err := c.comment.Gets(ctx, key, "-created", nil, 0, maxResults)
	if err != nil {
		return nil, fmt.Errorf("get jira comments %s: %w", key, err)
	}
	if response == nil {
		return nil, fmt.Errorf("get jira comments %s: empty response", key)
	}

	comments := make([]Comment, 0, len(response.Comments))
	for _, raw := range response.Comments {
		comments = append(comments, parseComment(raw))
	}
	return comments, nil
}

func (c *Client) AddComment(ctx context.Context, key string, body string, mentions []Mention) (Comment, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return Comment{}, fmt.Errorf("add jira comment %s: empty comment", key)
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	if c.comment == nil {
		return Comment{}, fmt.Errorf("add jira comment %s: comment service unavailable", key)
	}

	raw, _, err := c.comment.Add(ctx, key, &model.CommentPayloadScheme{
		Body: plainTextADF(body, mentions),
	}, nil)
	if err != nil {
		return Comment{}, fmt.Errorf("add jira comment %s: %w", key, err)
	}
	if raw == nil {
		return Comment{}, fmt.Errorf("add jira comment %s: empty response", key)
	}
	return parseComment(raw), nil
}

func (c *Client) SearchUsers(ctx context.Context, query string, maxResults int) ([]User, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("search jira users: empty query")
	}
	if maxResults <= 0 {
		maxResults = 10
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	if c.userSearch == nil {
		return nil, fmt.Errorf("search jira users %q: user search service unavailable", query)
	}

	rawUsers, _, err := c.userSearch.Do(ctx, "", query, 0, maxResults)
	if err != nil {
		return nil, fmt.Errorf("search jira users %q: %w", query, err)
	}
	users := make([]User, 0, len(rawUsers))
	for _, raw := range rawUsers {
		if user, ok := parseUser(raw); ok {
			users = append(users, user)
		}
	}
	return users, nil
}

func (c *Client) parseIssue(raw *model.IssueScheme) Issue {
	if raw == nil {
		return Issue{
			Status:   "Unknown",
			Assignee: "Unassigned",
		}
	}

	fields := raw.Fields
	assignee := "Unassigned"
	if fields != nil {
		assignee = jiraUserDisplayName(fields.Assignee, "Unassigned")
	}

	status := "Unknown"
	if fields != nil && fields.Status != nil && fields.Status.Name != "" {
		status = fields.Status.Name
	}

	priority := "None"
	if fields != nil && fields.Priority != nil && fields.Priority.Name != "" {
		priority = fields.Priority.Name
	}

	issueType := "Unknown"
	isSubtask := false
	hierarchyLevel := 0
	if fields != nil && fields.IssueType != nil {
		if fields.IssueType.Name != "" {
			issueType = fields.IssueType.Name
		}
		isSubtask = fields.IssueType.Subtask
		hierarchyLevel = fields.IssueType.HierarchyLevel
	}

	parentKey := ""
	parentSummary := ""
	if fields != nil && fields.Parent != nil {
		parentKey = fields.Parent.Key
		if fields.Parent.Fields != nil {
			parentSummary = fields.Parent.Fields.Summary
		}
	}

	summary := ""
	if fields != nil {
		summary = fields.Summary
	}

	issue := Issue{
		Key:            raw.Key,
		Summary:        summary,
		Status:         status,
		Assignee:       assignee,
		Priority:       priority,
		IssueType:      issueType,
		IsSubtask:      isSubtask,
		HierarchyLevel: hierarchyLevel,
		ParentKey:      parentKey,
		ParentSummary:  parentSummary,
		SubtaskCount:   subtaskCount(fields),
		URL:            c.baseURL + "/browse/" + raw.Key,
	}
	if fields != nil {
		issue.Subtasks = c.parseSubtasks(raw.Key, summary, fields.Subtasks)
	}
	return issue
}

func (c *Client) parseSubtasks(parentKey string, parentSummary string, rawSubtasks []*model.IssueScheme) []Issue {
	subtasks := make([]Issue, 0, len(rawSubtasks))
	for _, rawSubtask := range rawSubtasks {
		subtask := c.parseIssue(rawSubtask)
		if subtask.Key == "" {
			continue
		}
		subtask.ParentKey = parentKey
		subtask.ParentSummary = parentSummary
		subtask.IsSubtask = true
		subtasks = append(subtasks, subtask)
	}
	return subtasks
}

func (c *Client) parseIssueDetail(raw *model.IssueScheme) IssueDetail {
	issue := c.parseIssue(raw)
	detail := IssueDetail{
		Issue:    issue,
		Reporter: "Unknown",
		Creator:  "Unknown",
	}
	if raw == nil || raw.Fields == nil {
		return detail
	}

	fields := raw.Fields
	detail.Description = adf.Render(fields.Description)
	detail.Labels = append([]string(nil), fields.Labels...)
	detail.Components = componentNames(fields.Components)
	detail.FixVersions = versionNames(fields.FixVersions)
	detail.Reporter = jiraUserDisplayName(fields.Reporter, detail.Reporter)
	detail.Creator = jiraUserDisplayName(fields.Creator, detail.Creator)
	if fields.Created != nil {
		detail.Created = time.Time(*fields.Created)
	}
	if fields.Updated != nil {
		detail.Updated = time.Time(*fields.Updated)
	}
	return detail
}

func parseComment(raw *model.IssueCommentScheme) Comment {
	comment := Comment{
		Author: "Unknown",
	}
	if raw == nil {
		return comment
	}
	comment.ID = raw.ID
	if raw.Author != nil && raw.Author.DisplayName != "" {
		comment.Author = raw.Author.DisplayName
	}
	comment.Body = adf.Render(raw.Body)
	comment.Created = parseJiraCommentTime(raw.Created)
	comment.Updated = parseJiraCommentTime(raw.Updated)
	return comment
}

func parseUser(raw *model.UserScheme) (User, bool) {
	if raw == nil || raw.AccountID == "" {
		return User{}, false
	}
	return User{
		AccountID:   raw.AccountID,
		DisplayName: jiraUserDisplayName(raw, ""),
		Email:       raw.EmailAddress,
		Active:      raw.Active,
	}, true
}

func jiraUserDisplayName(raw *model.UserScheme, fallback string) string {
	if raw == nil {
		return fallback
	}
	for _, candidate := range []string{raw.DisplayName, raw.Name, raw.EmailAddress, raw.Key, raw.AccountID} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if IsPrivacyUserAlias(candidate) {
			continue
		}
		return candidate
	}
	if trimmed := strings.TrimSpace(raw.DisplayName); trimmed != "" {
		return trimmed
	}
	return fallback
}

func IsPrivacyUserAlias(value string) bool {
	parts := strings.Fields(strings.TrimSpace(value))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "user") {
		return false
	}
	if len(parts[1]) < 4 || len(parts[1]) > 12 {
		return false
	}
	for _, char := range parts[1] {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return false
		}
	}
	return true
}

func parseJiraCommentTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02T15:04:05.000Z0700",
	} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed
		}
	}
	return time.Time{}
}

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
		return []*model.CommentNodeScheme{textNode(line)}
	}

	nodes := make([]*model.CommentNodeScheme, 0, len(spans)*2+1)
	offset := 0
	for _, span := range spans {
		if span.start < offset || span.end > len(line) || span.start >= span.end {
			continue
		}
		if span.start > offset {
			nodes = append(nodes, textNode(line[offset:span.start]))
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
		nodes = append(nodes, textNode(line[offset:]))
	}
	if len(nodes) == 0 {
		return []*model.CommentNodeScheme{textNode(line)}
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
	return &model.CommentNodeScheme{
		Type: "text",
		Text: text,
	}
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

func subtaskCount(fields *model.IssueFieldsScheme) int {
	if fields == nil {
		return 0
	}
	return len(fields.Subtasks)
}

func componentNames(components []*model.ComponentScheme) []string {
	names := make([]string, 0, len(components))
	for _, component := range components {
		if component != nil && component.Name != "" {
			names = append(names, component.Name)
		}
	}
	return names
}

func versionNames(versions []*model.VersionScheme) []string {
	names := make([]string, 0, len(versions))
	for _, version := range versions {
		if version != nil && version.Name != "" {
			names = append(names, version.Name)
		}
	}
	return names
}

type failingSearchService struct {
	err error
}

func (f failingSearchService) SearchJQL(_ context.Context, _ string, _, _ []string, _ int, _ string) (*model.IssueSearchJQLScheme, *model.ResponseScheme, error) {
	return nil, nil, f.err
}

type failingIssueService struct {
	err error
}

func (f failingIssueService) Get(_ context.Context, _ string, _, _ []string) (*model.IssueScheme, *model.ResponseScheme, error) {
	return nil, nil, f.err
}

type failingCommentService struct {
	err error
}

func (f failingCommentService) Gets(_ context.Context, _ string, _ string, _ []string, _, _ int) (*model.IssueCommentPageScheme, *model.ResponseScheme, error) {
	return nil, nil, f.err
}

func (f failingCommentService) Add(_ context.Context, _ string, _ *model.CommentPayloadScheme, _ []string) (*model.IssueCommentScheme, *model.ResponseScheme, error) {
	return nil, nil, f.err
}

type failingUserSearchService struct {
	err error
}

func (f failingUserSearchService) Do(_ context.Context, _, _ string, _, _ int) ([]*model.UserScheme, *model.ResponseScheme, error) {
	return nil, nil, f.err
}
