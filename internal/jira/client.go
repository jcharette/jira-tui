package jira

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	atlassian "github.com/ctreminiom/go-atlassian/v2/jira/v3"
	model "github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"
	jiraservice "github.com/ctreminiom/go-atlassian/v2/service/jira"
	"github.com/jcharette/jira-tui/internal/adf"
	"github.com/jcharette/jira-tui/internal/config"
	"github.com/jcharette/jira-tui/internal/linkdetect"
	"github.com/tidwall/gjson"
)

const defaultMaxResults = 25
const createMetadataMaxResults = 50

type Client struct {
	baseURL        string
	search         issueSearchService
	issue          issueService
	comment        commentService
	userSearch     userSearchService
	metadata       metadataService
	requestTimeout time.Duration
}

type issueSearchService interface {
	SearchJQL(ctx context.Context, jql string, fields, expands []string, maxResults int, nextPageToken string) (*model.IssueSearchJQLScheme, *model.ResponseScheme, error)
}

type issueService interface {
	Create(ctx context.Context, payload *model.IssueScheme, customFields *model.CustomFields) (*model.IssueResponseScheme, *model.ResponseScheme, error)
	Get(ctx context.Context, issueKeyOrID string, fields, expand []string) (*model.IssueScheme, *model.ResponseScheme, error)
	Update(ctx context.Context, issueKeyOrID string, notify bool, payload *model.IssueScheme, customFields *model.CustomFields, operations *model.UpdateOperations) (*model.ResponseScheme, error)
	Transitions(ctx context.Context, issueKeyOrID string) (*model.IssueTransitionsScheme, *model.ResponseScheme, error)
	Move(ctx context.Context, issueKeyOrID, transitionID string, options *model.IssueMoveOptionsV3) (*model.ResponseScheme, error)
	Assign(ctx context.Context, issueKeyOrID, accountID string) (*model.ResponseScheme, error)
}

type commentService interface {
	Gets(ctx context.Context, issueKeyOrID, orderBy string, expand []string, startAt, maxResults int) (*model.IssueCommentPageScheme, *model.ResponseScheme, error)
	Add(ctx context.Context, issueKeyOrID string, payload *model.CommentPayloadScheme, expand []string) (*model.IssueCommentScheme, *model.ResponseScheme, error)
}

type userSearchService interface {
	Do(ctx context.Context, accountID, query string, startAt, maxResults int) ([]*model.UserScheme, *model.ResponseScheme, error)
}

type metadataService interface {
	Get(ctx context.Context, issueKeyOrID string, overrideScreenSecurity, overrideEditableFlag bool) (gjson.Result, *model.ResponseScheme, error)
	Create(ctx context.Context, opts *model.IssueMetadataCreateOptions) (gjson.Result, *model.ResponseScheme, error)
	FetchIssueMappings(ctx context.Context, projectKeyOrID string, startAt, maxResults int) (gjson.Result, *model.ResponseScheme, error)
	FetchFieldMappings(ctx context.Context, projectKeyOrID, issueTypeID string, startAt, maxResults int) (gjson.Result, *model.ResponseScheme, error)
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

type IssueLink struct {
	Direction    string
	Relationship string
	Key          string
	Summary      string
	Status       string
	IssueType    string
	URL          string
}

type IssueDetail struct {
	Issue
	Description string
	Reporter    string
	Creator     string
	Labels      []string
	Components  []string
	FixVersions []string
	IssueLinks  []IssueLink
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

type Transition struct {
	ID          string
	Name        string
	ToStatus    string
	HasScreen   bool
	IsAvailable bool
}

type EditMetadata struct {
	Summary  EditField
	Priority EditField
}

type EditField struct {
	ID            string
	Name          string
	Required      bool
	Editable      bool
	AllowedValues []FieldOption
}

type FieldOption struct {
	ID   string
	Name string
}

type CreateIssueType struct {
	ID             string
	Name           string
	Description    string
	Subtask        bool
	HierarchyLevel int
}

type CreateField struct {
	ID              string
	Key             string
	Name            string
	Required        bool
	HasDefaultValue bool
	SchemaType      string
	SchemaSystem    string
	SchemaItems     string
	SchemaCustom    string
	SchemaCustomID  int
	Operations      []string
	AllowedValues   []FieldOption
	AutoCompleteURL string
}

type CreateIssueRequest struct {
	ProjectKey  string
	IssueTypeID string
	Summary     string
	Description string
	Fields      []CreateIssueFieldValue
}

type CreateIssueFieldValue struct {
	FieldID      string
	SchemaType   string
	SchemaSystem string
	Text         string
	Option       FieldOption
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
			metadata:       failingMetadataService{err: err},
			requestTimeout: requestTimeout,
		}
	}
	api.Auth.SetBasicAuth(cfg.Email, cfg.APIToken)

	return newClient(cfg.BaseURL, api.Issue.Search, api.Issue, api.Issue.Comment, api.User.Search, api.Issue.Metadata, requestTimeout)
}

func newClient(baseURL string, search jiraservice.SearchADFConnector, issue jiraservice.IssueADFConnector, comment commentService, userSearch userSearchService, metadata jiraservice.MetadataConnector, requestTimeout time.Duration) *Client {
	return &Client{
		baseURL:        baseURL,
		search:         search,
		issue:          issue,
		comment:        comment,
		userSearch:     userSearch,
		metadata:       metadata,
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
		"issuelinks",
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

func (c *Client) GetIssueDescriptionADF(ctx context.Context, key string) (*model.CommentNodeScheme, error) {
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	raw, _, err := c.issue.Get(ctx, key, []string{"description"}, nil)
	if err != nil {
		return nil, fmt.Errorf("get jira issue description ADF %s: %w", key, err)
	}
	if raw == nil || raw.Fields == nil || raw.Fields.Description == nil {
		return nil, fmt.Errorf("get jira issue description ADF %s: empty description", key)
	}
	return raw.Fields.Description, nil
}

func (c *Client) GetTransitions(ctx context.Context, key string) ([]Transition, error) {
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	response, _, err := c.issue.Transitions(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("get jira transitions %s: %w", key, err)
	}
	if response == nil {
		return nil, fmt.Errorf("get jira transitions %s: empty response", key)
	}

	transitions := make([]Transition, 0, len(response.Transitions))
	for _, raw := range response.Transitions {
		if transition, ok := parseTransition(raw); ok {
			transitions = append(transitions, transition)
		}
	}
	return transitions, nil
}

func (c *Client) GetEditMetadata(ctx context.Context, key string) (EditMetadata, error) {
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	if c.metadata == nil {
		return EditMetadata{}, fmt.Errorf("get jira edit metadata %s: metadata service unavailable", key)
	}

	response, _, err := c.metadata.Get(ctx, key, false, false)
	if err != nil {
		return EditMetadata{}, fmt.Errorf("get jira edit metadata %s: %w", key, err)
	}
	return parseEditMetadata(response), nil
}

func (c *Client) GetCreateIssueTypes(ctx context.Context, projectKey string) ([]CreateIssueType, error) {
	projectKey = strings.TrimSpace(projectKey)
	if projectKey == "" {
		return nil, fmt.Errorf("get jira create issue types: missing project key")
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	if c.metadata == nil {
		return nil, fmt.Errorf("get jira create issue types %s: metadata service unavailable", projectKey)
	}

	response, _, err := c.metadata.FetchIssueMappings(ctx, projectKey, 0, createMetadataMaxResults)
	if err != nil {
		return nil, fmt.Errorf("get jira create issue types %s: %w", projectKey, err)
	}
	issueTypes := parseCreateIssueTypes(response)
	if len(issueTypes) > 0 {
		return issueTypes, nil
	}
	fallback, _, err := c.metadata.Create(ctx, &model.IssueMetadataCreateOptions{
		ProjectKeys: []string{projectKey},
		Expand:      "projects.issuetypes",
	})
	if err != nil {
		return nil, fmt.Errorf("get jira expanded create metadata %s: %w", projectKey, err)
	}
	return parseExpandedCreateIssueTypes(fallback, projectKey), nil
}

func (c *Client) GetCreateFields(ctx context.Context, projectKey string, issueTypeID string) ([]CreateField, error) {
	projectKey = strings.TrimSpace(projectKey)
	issueTypeID = strings.TrimSpace(issueTypeID)
	if projectKey == "" || issueTypeID == "" {
		return nil, fmt.Errorf("get jira create fields: missing project key or issue type ID")
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	if c.metadata == nil {
		return nil, fmt.Errorf("get jira create fields %s/%s: metadata service unavailable", projectKey, issueTypeID)
	}

	response, _, err := c.metadata.FetchFieldMappings(ctx, projectKey, issueTypeID, 0, createMetadataMaxResults)
	if err != nil {
		return nil, fmt.Errorf("get jira create fields %s/%s: %w", projectKey, issueTypeID, err)
	}
	fields := parseCreateFields(response)
	if len(fields) > 0 {
		return fields, nil
	}
	fallback, _, err := c.metadata.Create(ctx, &model.IssueMetadataCreateOptions{
		ProjectKeys:  []string{projectKey},
		IssueTypeIDs: []string{issueTypeID},
		Expand:       "projects.issuetypes.fields",
	})
	if err != nil {
		return nil, fmt.Errorf("get jira expanded create fields %s/%s: %w", projectKey, issueTypeID, err)
	}
	return parseExpandedCreateFields(fallback, projectKey, issueTypeID), nil
}

func (c *Client) CreateIssue(ctx context.Context, request CreateIssueRequest) (Issue, error) {
	request.ProjectKey = strings.TrimSpace(request.ProjectKey)
	request.IssueTypeID = strings.TrimSpace(request.IssueTypeID)
	request.Summary = strings.TrimSpace(request.Summary)
	if request.ProjectKey == "" || request.IssueTypeID == "" || request.Summary == "" {
		return Issue{}, fmt.Errorf("create jira issue: missing project key, issue type ID, or summary")
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	payload := &model.IssueScheme{
		Fields: &model.IssueFieldsScheme{
			Project:   &model.ProjectScheme{Key: request.ProjectKey},
			IssueType: &model.IssueTypeScheme{ID: request.IssueTypeID},
			Summary:   request.Summary,
		},
	}
	if strings.TrimSpace(request.Description) != "" {
		payload.Fields.Description = plainTextADF(request.Description, nil)
	}
	customFields, err := applyCreateIssueFieldValues(payload.Fields, request.Fields)
	if err != nil {
		return Issue{}, err
	}
	response, _, err := c.issue.Create(ctx, payload, customFields)
	if err != nil {
		return Issue{}, fmt.Errorf("create jira issue: %w", err)
	}
	if response == nil || strings.TrimSpace(response.Key) == "" {
		return Issue{}, fmt.Errorf("create jira issue: empty response")
	}
	return Issue{
		Key:       response.Key,
		Summary:   request.Summary,
		IssueType: request.IssueTypeID,
		URL:       c.baseURL + "/browse/" + response.Key,
	}, nil
}

func applyCreateIssueFieldValues(fields *model.IssueFieldsScheme, values []CreateIssueFieldValue) (*model.CustomFields, error) {
	if len(values) == 0 {
		return nil, nil
	}
	customFields := &model.CustomFields{}
	for _, value := range values {
		fieldID := strings.TrimSpace(value.FieldID)
		if fieldID == "" || fieldID == "summary" || fieldID == "description" {
			continue
		}
		system := strings.TrimSpace(value.SchemaSystem)
		schemaType := strings.TrimSpace(value.SchemaType)
		switch {
		case fieldID == "priority" || system == "priority":
			option := createFieldOptionPayload(value.Option)
			if len(option) == 0 {
				continue
			}
			fields.Priority = &model.PriorityScheme{ID: value.Option.ID, Name: value.Option.Name}
		case fieldID == "labels" || system == "labels":
			labels := splitCreateLabels(value.Text)
			if len(labels) > 0 {
				fields.Labels = labels
			}
		case fieldID == "components" || system == "components":
			option := value.Option
			if strings.TrimSpace(option.ID) != "" || strings.TrimSpace(option.Name) != "" {
				fields.Components = []*model.ComponentScheme{{ID: strings.TrimSpace(option.ID), Name: strings.TrimSpace(option.Name)}}
			}
		case strings.HasPrefix(fieldID, "customfield_"):
			raw, ok := createCustomFieldPayload(value, schemaType)
			if !ok {
				continue
			}
			if err := customFields.Raw(fieldID, raw); err != nil {
				return nil, fmt.Errorf("create jira issue field %s: %w", fieldID, err)
			}
		}
	}
	if len(customFields.Fields) == 0 {
		return nil, nil
	}
	return customFields, nil
}

func createCustomFieldPayload(value CreateIssueFieldValue, schemaType string) (interface{}, bool) {
	text := strings.TrimSpace(value.Text)
	switch schemaType {
	case "number":
		if text == "" {
			return nil, false
		}
		number, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return nil, false
		}
		return number, true
	case "option", "priority":
		option := createFieldOptionPayload(value.Option)
		if len(option) == 0 {
			return nil, false
		}
		return option, true
	default:
		if value.Option.ID != "" || value.Option.Name != "" {
			option := createFieldOptionPayload(value.Option)
			if len(option) > 0 {
				return option, true
			}
		}
		if text == "" {
			return nil, false
		}
		return text, true
	}
}

func createFieldOptionPayload(option FieldOption) map[string]interface{} {
	payload := map[string]interface{}{}
	if strings.TrimSpace(option.ID) != "" {
		payload["id"] = strings.TrimSpace(option.ID)
	}
	if strings.TrimSpace(option.Name) != "" {
		payload["value"] = strings.TrimSpace(option.Name)
	}
	return payload
}

func splitCreateLabels(value string) []string {
	var labels []string
	for _, part := range strings.Split(value, ",") {
		label := strings.TrimSpace(part)
		if label != "" {
			labels = append(labels, label)
		}
	}
	return labels
}

func (c *Client) UpdateSummary(ctx context.Context, key string, summary string) error {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return fmt.Errorf("update jira summary %s: empty summary", key)
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	payload := &model.IssueScheme{
		Fields: &model.IssueFieldsScheme{
			Summary: summary,
		},
	}
	if _, err := c.issue.Update(ctx, key, false, payload, nil, nil); err != nil {
		return fmt.Errorf("update jira summary %s: %w", key, err)
	}
	return nil
}

func (c *Client) UpdateDescription(ctx context.Context, key string, description string) error {
	description = strings.TrimSpace(description)
	if description == "" {
		return fmt.Errorf("update jira description %s: empty description", key)
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	payload := &model.IssueScheme{
		Fields: &model.IssueFieldsScheme{
			Description: plainTextADF(description, nil),
		},
	}
	if _, err := c.issue.Update(ctx, key, false, payload, nil, nil); err != nil {
		return fmt.Errorf("update jira description %s: %w", key, err)
	}
	return nil
}

func (c *Client) UpdatePriority(ctx context.Context, key string, priority FieldOption) error {
	priority.ID = strings.TrimSpace(priority.ID)
	priority.Name = strings.TrimSpace(priority.Name)
	if priority.ID == "" && priority.Name == "" {
		return fmt.Errorf("update jira priority %s: empty priority", key)
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	payload := &model.IssueScheme{
		Fields: &model.IssueFieldsScheme{
			Priority: &model.PriorityScheme{
				ID:   priority.ID,
				Name: priority.Name,
			},
		},
	}
	if _, err := c.issue.Update(ctx, key, false, payload, nil, nil); err != nil {
		return fmt.Errorf("update jira priority %s: %w", key, err)
	}
	return nil
}

func (c *Client) UpdateAssignee(ctx context.Context, key string, assignee User) error {
	assignee.AccountID = strings.TrimSpace(assignee.AccountID)
	if assignee.AccountID == "" {
		return fmt.Errorf("update jira assignee %s: missing account ID", key)
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	if _, err := c.issue.Assign(ctx, key, assignee.AccountID); err != nil {
		return fmt.Errorf("update jira assignee %s: %w", key, err)
	}
	return nil
}

func (c *Client) TransitionIssue(ctx context.Context, key string, transitionID string) error {
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	if _, err := c.issue.Move(ctx, key, transitionID, nil); err != nil {
		return fmt.Errorf("transition jira issue %s: %w", key, err)
	}
	return nil
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

func (c *Client) GetCommentADF(ctx context.Context, key string, commentID string) (*model.CommentNodeScheme, error) {
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	if c.comment == nil {
		return nil, fmt.Errorf("get jira comment ADF %s/%s: comment service unavailable", key, commentID)
	}
	response, _, err := c.comment.Gets(ctx, key, "-created", nil, 0, 100)
	if err != nil {
		return nil, fmt.Errorf("get jira comment ADF %s/%s: %w", key, commentID, err)
	}
	if response == nil {
		return nil, fmt.Errorf("get jira comment ADF %s/%s: empty response", key, commentID)
	}
	for _, raw := range response.Comments {
		if raw != nil && raw.ID == commentID {
			if raw.Body == nil {
				return nil, fmt.Errorf("get jira comment ADF %s/%s: empty body", key, commentID)
			}
			return raw.Body, nil
		}
	}
	return nil, fmt.Errorf("get jira comment ADF %s/%s: comment not found", key, commentID)
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
	detail.IssueLinks = c.parseIssueLinks(fields.IssueLinks)
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

func (c *Client) parseIssueLinks(rawLinks []*model.IssueLinkScheme) []IssueLink {
	links := make([]IssueLink, 0, len(rawLinks))
	for _, raw := range rawLinks {
		if raw == nil {
			continue
		}
		if link, ok := c.parseIssueLink("outward", relationshipText(raw.Type, "outward"), raw.OutwardIssue); ok {
			links = append(links, link)
		}
		if link, ok := c.parseIssueLink("inward", relationshipText(raw.Type, "inward"), raw.InwardIssue); ok {
			links = append(links, link)
		}
	}
	return links
}

func (c *Client) parseIssueLink(direction string, relationship string, raw *model.LinkedIssueScheme) (IssueLink, bool) {
	if raw == nil || strings.TrimSpace(raw.Key) == "" {
		return IssueLink{}, false
	}
	link := IssueLink{
		Direction:    direction,
		Relationship: relationship,
		Key:          raw.Key,
		Status:       "Unknown",
		IssueType:    "Unknown",
		URL:          c.baseURL + "/browse/" + raw.Key,
	}
	if raw.Fields != nil {
		link.Summary = raw.Fields.Summary
		if raw.Fields.Status != nil && raw.Fields.Status.Name != "" {
			link.Status = raw.Fields.Status.Name
		}
		if raw.Fields.IssueType != nil && raw.Fields.IssueType.Name != "" {
			link.IssueType = raw.Fields.IssueType.Name
		}
	}
	return link, true
}

func relationshipText(raw *model.LinkTypeScheme, direction string) string {
	if raw == nil {
		return ""
	}
	switch direction {
	case "outward":
		if strings.TrimSpace(raw.Outward) != "" {
			return raw.Outward
		}
	case "inward":
		if strings.TrimSpace(raw.Inward) != "" {
			return raw.Inward
		}
	}
	return raw.Name
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

func parseTransition(raw *model.IssueTransitionScheme) (Transition, bool) {
	if raw == nil || raw.ID == "" {
		return Transition{}, false
	}
	transition := Transition{
		ID:          raw.ID,
		Name:        raw.Name,
		HasScreen:   raw.HasScreen,
		IsAvailable: raw.IsAvailable,
	}
	if raw.To != nil {
		transition.ToStatus = raw.To.Name
	}
	return transition, true
}

func parseEditMetadata(raw gjson.Result) EditMetadata {
	return EditMetadata{
		Summary:  parseEditField("summary", raw.Get("fields.summary")),
		Priority: parseEditField("priority", raw.Get("fields.priority")),
	}
}

func parseCreateIssueTypes(raw gjson.Result) []CreateIssueType {
	var issueTypes []CreateIssueType
	for _, item := range raw.Get("values").Array() {
		issueType := CreateIssueType{
			ID:             strings.TrimSpace(item.Get("id").String()),
			Name:           strings.TrimSpace(item.Get("name").String()),
			Description:    strings.TrimSpace(item.Get("description").String()),
			Subtask:        item.Get("subtask").Bool(),
			HierarchyLevel: int(item.Get("hierarchyLevel").Int()),
		}
		if issueType.ID == "" && issueType.Name == "" {
			continue
		}
		issueTypes = append(issueTypes, issueType)
	}
	return issueTypes
}

func parseExpandedCreateIssueTypes(raw gjson.Result, projectKey string) []CreateIssueType {
	projectKey = strings.TrimSpace(projectKey)
	var issueTypes []CreateIssueType
	for _, project := range raw.Get("projects").Array() {
		if projectKey != "" && !strings.EqualFold(strings.TrimSpace(project.Get("key").String()), projectKey) {
			continue
		}
		for _, item := range project.Get("issuetypes").Array() {
			issueType := CreateIssueType{
				ID:             strings.TrimSpace(item.Get("id").String()),
				Name:           strings.TrimSpace(item.Get("name").String()),
				Description:    strings.TrimSpace(item.Get("description").String()),
				Subtask:        item.Get("subtask").Bool(),
				HierarchyLevel: int(item.Get("hierarchyLevel").Int()),
			}
			if issueType.ID == "" && issueType.Name == "" {
				continue
			}
			issueTypes = append(issueTypes, issueType)
		}
	}
	return issueTypes
}

func parseCreateFields(raw gjson.Result) []CreateField {
	var fields []CreateField
	for _, item := range raw.Get("values").Array() {
		field := parseCreateField("", item)
		if field.ID == "" && field.Key == "" && field.Name == "" {
			continue
		}
		fields = append(fields, field)
	}
	return fields
}

func parseExpandedCreateFields(raw gjson.Result, projectKey string, issueTypeID string) []CreateField {
	projectKey = strings.TrimSpace(projectKey)
	issueTypeID = strings.TrimSpace(issueTypeID)
	var fields []CreateField
	for _, project := range raw.Get("projects").Array() {
		if projectKey != "" && !strings.EqualFold(strings.TrimSpace(project.Get("key").String()), projectKey) {
			continue
		}
		for _, issueType := range project.Get("issuetypes").Array() {
			if issueTypeID != "" && strings.TrimSpace(issueType.Get("id").String()) != issueTypeID {
				continue
			}
			rawFields := issueType.Get("fields")
			fieldIDs := make([]string, 0, len(rawFields.Map()))
			rawFields.ForEach(func(key, _ gjson.Result) bool {
				if fieldID := strings.TrimSpace(key.String()); fieldID != "" {
					fieldIDs = append(fieldIDs, fieldID)
				}
				return true
			})
			sort.Strings(fieldIDs)
			for _, fieldID := range fieldIDs {
				field := parseCreateField(fieldID, rawFields.Get(fieldID))
				if field.ID == "" && field.Key == "" && field.Name == "" {
					continue
				}
				fields = append(fields, field)
			}
		}
	}
	return fields
}

func parseCreateField(defaultID string, item gjson.Result) CreateField {
	field := CreateField{
		ID:              strings.TrimSpace(firstNonEmpty(item.Get("fieldId").String(), item.Get("id").String(), defaultID)),
		Key:             strings.TrimSpace(firstNonEmpty(item.Get("key").String(), defaultID)),
		Name:            strings.TrimSpace(item.Get("name").String()),
		Required:        item.Get("required").Bool(),
		HasDefaultValue: item.Get("hasDefaultValue").Bool(),
		SchemaType:      strings.TrimSpace(item.Get("schema.type").String()),
		SchemaSystem:    strings.TrimSpace(item.Get("schema.system").String()),
		SchemaItems:     strings.TrimSpace(item.Get("schema.items").String()),
		SchemaCustom:    strings.TrimSpace(item.Get("schema.custom").String()),
		SchemaCustomID:  int(item.Get("schema.customId").Int()),
		AutoCompleteURL: strings.TrimSpace(item.Get("autoCompleteUrl").String()),
	}
	for _, operation := range item.Get("operations").Array() {
		if value := strings.TrimSpace(operation.String()); value != "" {
			field.Operations = append(field.Operations, value)
		}
	}
	for _, allowed := range item.Get("allowedValues").Array() {
		option := parseFieldOption(allowed)
		if option.ID == "" && option.Name == "" {
			continue
		}
		field.AllowedValues = append(field.AllowedValues, option)
	}
	return field
}

func parseEditField(id string, raw gjson.Result) EditField {
	field := EditField{
		ID:       id,
		Name:     raw.Get("name").String(),
		Required: raw.Get("required").Bool(),
	}
	for _, operation := range raw.Get("operations").Array() {
		if operation.String() == "set" {
			field.Editable = true
			break
		}
	}
	for _, allowed := range raw.Get("allowedValues").Array() {
		option := parseFieldOption(allowed)
		if option.ID == "" && option.Name == "" {
			continue
		}
		field.AllowedValues = append(field.AllowedValues, option)
	}
	return field
}

func parseFieldOption(raw gjson.Result) FieldOption {
	return FieldOption{
		ID: strings.TrimSpace(raw.Get("id").String()),
		Name: strings.TrimSpace(firstNonEmpty(
			raw.Get("name").String(),
			raw.Get("value").String(),
			raw.Get("displayName").String(),
			raw.Get("key").String(),
		)),
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
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

func (f failingIssueService) Create(_ context.Context, _ *model.IssueScheme, _ *model.CustomFields) (*model.IssueResponseScheme, *model.ResponseScheme, error) {
	return nil, nil, f.err
}

func (f failingIssueService) Get(_ context.Context, _ string, _, _ []string) (*model.IssueScheme, *model.ResponseScheme, error) {
	return nil, nil, f.err
}

func (f failingIssueService) Update(_ context.Context, _ string, _ bool, _ *model.IssueScheme, _ *model.CustomFields, _ *model.UpdateOperations) (*model.ResponseScheme, error) {
	return nil, f.err
}

func (f failingIssueService) Transitions(_ context.Context, _ string) (*model.IssueTransitionsScheme, *model.ResponseScheme, error) {
	return nil, nil, f.err
}

func (f failingIssueService) Move(_ context.Context, _, _ string, _ *model.IssueMoveOptionsV3) (*model.ResponseScheme, error) {
	return nil, f.err
}

func (f failingIssueService) Assign(_ context.Context, _, _ string) (*model.ResponseScheme, error) {
	return nil, f.err
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

type failingMetadataService struct {
	err error
}

func (f failingMetadataService) Get(_ context.Context, _ string, _, _ bool) (gjson.Result, *model.ResponseScheme, error) {
	return gjson.Result{}, nil, f.err
}

func (f failingMetadataService) Create(_ context.Context, _ *model.IssueMetadataCreateOptions) (gjson.Result, *model.ResponseScheme, error) {
	return gjson.Result{}, nil, f.err
}

func (f failingMetadataService) FetchIssueMappings(_ context.Context, _ string, _, _ int) (gjson.Result, *model.ResponseScheme, error) {
	return gjson.Result{}, nil, f.err
}

func (f failingMetadataService) FetchFieldMappings(_ context.Context, _, _ string, _, _ int) (gjson.Result, *model.ResponseScheme, error) {
	return gjson.Result{}, nil, f.err
}
