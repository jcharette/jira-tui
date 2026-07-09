package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	agile "github.com/ctreminiom/go-atlassian/v2/jira/agile"
	atlassian "github.com/ctreminiom/go-atlassian/v2/jira/v3"
	model "github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"
	jiraservice "github.com/ctreminiom/go-atlassian/v2/service/jira"
	"github.com/jcharette/jira-tui/internal/adf"
	"github.com/jcharette/jira-tui/internal/config"
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
	myself         myselfService
	metadata       metadataService
	rest           restConnector
	board          agileBoardService
	requestTimeout time.Duration
	teamFieldID    string
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
	Update(ctx context.Context, issueKeyOrID, commentID string, payload *model.CommentPayloadScheme, expand []string) (*model.IssueCommentScheme, *model.ResponseScheme, error)
}

type userSearchService interface {
	Do(ctx context.Context, accountID, query string, startAt, maxResults int) ([]*model.UserScheme, *model.ResponseScheme, error)
}

type myselfService interface {
	Details(ctx context.Context, expand []string) (*model.UserScheme, *model.ResponseScheme, error)
}

type metadataService interface {
	Get(ctx context.Context, issueKeyOrID string, overrideScreenSecurity, overrideEditableFlag bool) (gjson.Result, *model.ResponseScheme, error)
	Create(ctx context.Context, opts *model.IssueMetadataCreateOptions) (gjson.Result, *model.ResponseScheme, error)
	FetchIssueMappings(ctx context.Context, projectKeyOrID string, startAt, maxResults int) (gjson.Result, *model.ResponseScheme, error)
	FetchFieldMappings(ctx context.Context, projectKeyOrID, issueTypeID string, startAt, maxResults int) (gjson.Result, *model.ResponseScheme, error)
}

type restConnector interface {
	NewRequest(ctx context.Context, method, urlStr, contentType string, body interface{}) (*http.Request, error)
	Call(request *http.Request, v interface{}) (*model.ResponseScheme, error)
}

type agileBoardService interface {
	Gets(ctx context.Context, opts *model.GetBoardsOptions, startAt, maxResults int) (*model.BoardPageScheme, *model.ResponseScheme, error)
	Issues(ctx context.Context, boardID int, opts *model.IssueOptionScheme, startAt, maxResults int) (*model.BoardIssuePageScheme, *model.ResponseScheme, error)
	Sprints(ctx context.Context, boardID, startAt, maxResults int, states []string) (*model.BoardSprintPageScheme, *model.ResponseScheme, error)
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
	TeamID         string
	TeamName       string
	URL            string
}

type IssueLink struct {
	LinkID       string
	Direction    string
	Relationship string
	Key          string
	Summary      string
	Status       string
	IssueType    string
	URL          string
}

type IssueLinkType struct {
	ID      string
	Name    string
	Inward  string
	Outward string
}

type CreateIssueLinkRequest struct {
	SourceKey string
	TargetKey string
	Type      IssueLinkType
	Direction string
}

type IssueDetail struct {
	Issue
	Description       string
	Reporter          string
	Creator           string
	Labels            []string
	Components        []string
	FixVersions       []string
	OriginalEstimate  string
	RemainingEstimate string
	IssueLinks        []IssueLink
	Created           time.Time
	Updated           time.Time
}

type Comment struct {
	ID      string
	Author  string
	Body    string
	Created time.Time
	Updated time.Time
}

type Worklog struct {
	ID               string
	Author           string
	Comment          string
	TimeSpent        string
	TimeSpentSeconds int
	Started          time.Time
	Updated          time.Time
}

type AddWorklogRequest struct {
	TimeSpent string
	Started   time.Time
	Comment   string
}

type UpdateWorklogRequest struct {
	ID        string
	TimeSpent string
	Started   time.Time
	Comment   string
}

type UpdateParentRequest struct {
	ParentKey string
	Clear     bool
}

type UpdateTimeTrackingRequest struct {
	OriginalEstimate  string
	RemainingEstimate string
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

type EditMetadata struct {
	Summary    EditField
	Priority   EditField
	Labels     EditField
	Components EditField
	Fields     []EditField
}

type EditField struct {
	ID              string
	Name            string
	Required        bool
	HasDefaultValue bool
	Editable        bool
	SchemaType      string
	SchemaSystem    string
	SchemaItems     string
	SchemaCustom    string
	SchemaCustomID  int
	Operations      []string
	AllowedValues   []FieldOption
	AutoCompleteURL string
}

type EditFieldValue struct {
	FieldID      string
	SchemaType   string
	SchemaSystem string
	SchemaItems  string
	SchemaCustom string
	Text         string
	Option       FieldOption
	Options      []FieldOption
}

type FieldOption struct {
	ID   string
	Name string
}

type fieldOptionSearchResponse struct {
	Results []fieldOptionSearchItem `json:"results"`
	Values  []fieldOptionSearchItem `json:"values"`
}

type fieldOptionSearchItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Value       string `json:"value"`
	DisplayName string `json:"displayName"`
	Key         string `json:"key"`
}

type issueDetailResponse struct {
	model.IssueScheme
	RawFields map[string]json.RawMessage
}

func (r *issueDetailResponse) UnmarshalJSON(data []byte) error {
	type rawIssueFields struct {
		Fields map[string]json.RawMessage `json:"fields"`
	}
	var issue model.IssueScheme
	if err := json.Unmarshal(data, &issue); err != nil {
		return err
	}
	var raw rawIssueFields
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	r.IssueScheme = issue
	r.RawFields = raw.Fields
	return nil
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
	ParentKey   string
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

type Board struct {
	ID          int
	Name        string
	Type        string
	ProjectKey  string
	ProjectName string
}

type BoardPage struct {
	Boards     []Board
	StartAt    int
	MaxResults int
	Total      int
	IsLast     bool
}

type Sprint struct {
	ID           int
	BoardID      int
	Name         string
	State        string
	Goal         string
	StartDate    time.Time
	EndDate      time.Time
	CompleteDate time.Time
}

type SprintPage struct {
	BoardID    int
	Sprints    []Sprint
	StartAt    int
	MaxResults int
	Total      int
	IsLast     bool
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
			myself:         failingMyselfService{err: err},
			metadata:       failingMetadataService{err: err},
			board:          failingAgileBoardService{err: err},
			requestTimeout: requestTimeout,
			teamFieldID:    strings.TrimSpace(cfg.DefaultTeamFieldID),
		}
	}
	api.Auth.SetBasicAuth(cfg.Email, cfg.APIToken)

	agileAPI, err := agile.New(httpClient, cfg.BaseURL)
	if err != nil {
		return &Client{
			baseURL:        cfg.BaseURL,
			search:         failingSearchService{err: err},
			issue:          failingIssueService{err: err},
			comment:        failingCommentService{err: err},
			userSearch:     failingUserSearchService{err: err},
			myself:         failingMyselfService{err: err},
			metadata:       failingMetadataService{err: err},
			board:          failingAgileBoardService{err: err},
			requestTimeout: requestTimeout,
			teamFieldID:    strings.TrimSpace(cfg.DefaultTeamFieldID),
		}
	}
	agileAPI.Auth.SetBasicAuth(cfg.Email, cfg.APIToken)

	return newClient(cfg.BaseURL, api.Issue.Search, api.Issue, api.Issue.Comment, api.User.Search, api.MySelf, api.Issue.Metadata, api, agileAPI.Board, requestTimeout, cfg.DefaultTeamFieldID)
}

func newClient(baseURL string, search jiraservice.SearchADFConnector, issue jiraservice.IssueADFConnector, comment commentService, userSearch userSearchService, myself myselfService, metadata jiraservice.MetadataConnector, rest restConnector, board agileBoardService, requestTimeout time.Duration, teamFieldID string) *Client {
	return &Client{
		baseURL:        baseURL,
		search:         search,
		issue:          issue,
		comment:        comment,
		userSearch:     userSearch,
		myself:         myself,
		metadata:       metadata,
		rest:           rest,
		board:          board,
		requestTimeout: requestTimeout,
		teamFieldID:    strings.TrimSpace(teamFieldID),
	}
}

func (c *Client) CurrentUser(ctx context.Context) (User, error) {
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	if c.myself == nil {
		return User{}, fmt.Errorf("get current Jira user: service unavailable")
	}
	raw, _, err := c.myself.Details(ctx, nil)
	if err != nil {
		return User{}, fmt.Errorf("get current Jira user: %w", err)
	}
	user, ok := parseUser(raw)
	if !ok {
		return User{}, fmt.Errorf("get current Jira user: missing account ID")
	}
	return user, nil
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

func (c *Client) GetBoards(ctx context.Context, projectKey string, startAt, maxResults int) (BoardPage, error) {
	projectKey = strings.TrimSpace(projectKey)
	if maxResults <= 0 {
		maxResults = defaultMaxResults
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	if c.board == nil {
		return BoardPage{}, fmt.Errorf("get jira boards: agile board service unavailable")
	}

	options := &model.GetBoardsOptions{}
	options.BoardType = "scrum"
	options.IncludePrivate = true
	if projectKey != "" {
		options.ProjectKeyOrID = projectKey
	}
	response, _, err := c.board.Gets(ctx, options, startAt, maxResults)
	if err != nil {
		return BoardPage{}, fmt.Errorf("get jira boards: %w", err)
	}
	if response == nil {
		return BoardPage{}, fmt.Errorf("get jira boards: empty response")
	}
	return parseBoardPage(response), nil
}

func (c *Client) GetBoardSprints(ctx context.Context, boardID int, states []string, startAt, maxResults int) (SprintPage, error) {
	if boardID <= 0 {
		return SprintPage{}, fmt.Errorf("get jira board sprints: missing board ID")
	}
	if maxResults <= 0 {
		maxResults = defaultMaxResults
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	if c.board == nil {
		return SprintPage{}, fmt.Errorf("get jira board sprints %d: agile board service unavailable", boardID)
	}

	response, _, err := c.board.Sprints(ctx, boardID, startAt, maxResults, states)
	if err != nil {
		return SprintPage{}, fmt.Errorf("get jira board sprints %d: %w", boardID, err)
	}
	if response == nil {
		return SprintPage{}, fmt.Errorf("get jira board sprints %d: empty response", boardID)
	}
	page := parseSprintPage(response)
	page.BoardID = boardID
	return page, nil
}

func (c *Client) SearchBoardIssues(ctx context.Context, boardID int, jql string, maxResults int) ([]Issue, error) {
	jql = strings.TrimSpace(jql)
	if boardID <= 0 {
		return nil, fmt.Errorf("search jira board issues: missing board ID")
	}
	if maxResults <= 0 {
		maxResults = defaultMaxResults
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	if c.board == nil {
		return nil, fmt.Errorf("search jira board issues %d: agile board service unavailable", boardID)
	}
	opts := &model.IssueOptionScheme{
		JQL:           jql,
		ValidateQuery: true,
		Fields:        []string{"summary", "status", "assignee", "issuetype", "parent"},
	}
	response, _, err := c.board.Issues(ctx, boardID, opts, 0, maxResults)
	if err != nil {
		return nil, fmt.Errorf("search jira board issues %d: %w", boardID, err)
	}
	if response == nil {
		return nil, fmt.Errorf("search jira board issues %d: empty response", boardID)
	}
	issues := make([]Issue, 0, len(response.Issues))
	for _, raw := range response.Issues {
		if raw == nil {
			continue
		}
		issues = append(issues, Issue{Key: raw.Key})
	}
	return issues, nil
}

func (c *Client) MoveIssuesToSprint(ctx context.Context, sprintID int, issueKeys []string) error {
	if sprintID <= 0 {
		return fmt.Errorf("move jira issues to sprint: missing sprint ID")
	}
	keys := normalizedIssueKeys(issueKeys)
	if len(keys) == 0 {
		return fmt.Errorf("move jira issues to sprint %d: no issue keys", sprintID)
	}
	if len(keys) > 50 {
		return fmt.Errorf("move jira issues to sprint %d: Jira allows at most 50 issues per request", sprintID)
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	if c.rest == nil {
		return fmt.Errorf("move jira issues to sprint %d: REST service unavailable", sprintID)
	}
	endpoint := fmt.Sprintf("rest/agile/1.0/sprint/%d/issue", sprintID)
	payload := map[string][]string{"issues": keys}
	restRequest, err := c.rest.NewRequest(ctx, http.MethodPost, endpoint, "", payload)
	if err != nil {
		return fmt.Errorf("move jira issues to sprint %d: %w", sprintID, err)
	}
	if _, err := c.rest.Call(restRequest, nil); err != nil {
		return fmt.Errorf("move jira issues to sprint %d: %w", sprintID, err)
	}
	return nil
}

func normalizedIssueKeys(keys []string) []string {
	normalized := make([]string, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		key = strings.ToUpper(strings.TrimSpace(key))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, key)
	}
	return normalized
}

func (c *Client) GetIssue(ctx context.Context, key string) (IssueDetail, error) {
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	fields := c.issueDetailFields()
	if c.rest != nil {
		response, err := c.getIssueDetailREST(ctx, key, fields)
		if err != nil {
			return IssueDetail{}, err
		}
		return c.parseIssueDetail(&response.IssueScheme, response.RawFields), nil
	}
	raw, _, err := c.issue.Get(ctx, key, fields, nil)
	if err != nil {
		return IssueDetail{}, fmt.Errorf("get jira issue %s: %w", key, err)
	}
	if raw == nil {
		return IssueDetail{}, fmt.Errorf("get jira issue %s: empty response", key)
	}
	return c.parseIssueDetail(raw, nil), nil
}

func (c *Client) issueDetailFields() []string {
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
		"timetracking",
	}
	if strings.TrimSpace(c.teamFieldID) != "" {
		fields = append(fields, strings.TrimSpace(c.teamFieldID))
	}
	return fields
}

func (c *Client) getIssueDetailREST(ctx context.Context, key string, fields []string) (issueDetailResponse, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return issueDetailResponse{}, fmt.Errorf("get jira issue: empty issue key")
	}
	endpoint := "rest/api/3/issue/" + url.PathEscape(key) + "?fields=" + url.QueryEscape(strings.Join(fields, ","))
	request, err := c.rest.NewRequest(ctx, http.MethodGet, endpoint, "", nil)
	if err != nil {
		return issueDetailResponse{}, fmt.Errorf("get jira issue %s: %w", key, err)
	}
	var response issueDetailResponse
	if _, err := c.rest.Call(request, &response); err != nil {
		return issueDetailResponse{}, fmt.Errorf("get jira issue %s: %w", key, err)
	}
	if strings.TrimSpace(response.Key) == "" {
		return issueDetailResponse{}, fmt.Errorf("get jira issue %s: empty response", key)
	}
	return response, nil
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

	if c.rest != nil {
		return c.getTransitionsWithFields(ctx, key)
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

func (c *Client) SearchFieldOptions(ctx context.Context, autocompleteURL string, query string, maxResults int) ([]FieldOption, error) {
	autocompleteURL = strings.TrimSpace(autocompleteURL)
	query = strings.TrimSpace(query)
	if autocompleteURL == "" {
		return nil, fmt.Errorf("search jira field options: empty autocomplete URL")
	}
	if maxResults <= 0 {
		maxResults = 25
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	if c.rest == nil {
		return nil, fmt.Errorf("search jira field options: REST service unavailable")
	}
	endpoint, err := c.fieldOptionAutocompleteEndpoint(autocompleteURL, query, maxResults)
	if err != nil {
		return nil, err
	}
	request, err := c.rest.NewRequest(ctx, http.MethodGet, endpoint, "", nil)
	if err != nil {
		return nil, fmt.Errorf("search jira field options: %w", err)
	}
	var response fieldOptionSearchResponse
	if _, err := c.rest.Call(request, &response); err != nil {
		return nil, fmt.Errorf("search jira field options: %w", err)
	}
	return parseFieldOptionSearchResponse(response), nil
}

func (c *Client) GetIssueLinkTypes(ctx context.Context) ([]IssueLinkType, error) {
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	if c.rest == nil {
		return nil, fmt.Errorf("get jira issue link types: REST service unavailable")
	}
	request, err := c.rest.NewRequest(ctx, http.MethodGet, "rest/api/3/issueLinkType", "", nil)
	if err != nil {
		return nil, fmt.Errorf("get jira issue link types: %w", err)
	}
	var response model.IssueLinkTypeSearchScheme
	if _, err := c.rest.Call(request, &response); err != nil {
		return nil, fmt.Errorf("get jira issue link types: %w", err)
	}
	return parseIssueLinkTypes(response.IssueLinkTypes), nil
}

func parseIssueLinkTypes(rawTypes []*model.LinkTypeScheme) []IssueLinkType {
	types := make([]IssueLinkType, 0, len(rawTypes))
	seen := make(map[string]struct{}, len(rawTypes))
	for _, raw := range rawTypes {
		if raw == nil {
			continue
		}
		linkType := IssueLinkType{
			ID:      strings.TrimSpace(raw.ID),
			Name:    strings.TrimSpace(raw.Name),
			Inward:  strings.TrimSpace(raw.Inward),
			Outward: strings.TrimSpace(raw.Outward),
		}
		if linkType.ID == "" && linkType.Name == "" {
			continue
		}
		key := linkType.ID
		if key == "" {
			key = strings.ToLower(linkType.Name)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		types = append(types, linkType)
	}
	sort.SliceStable(types, func(i, j int) bool {
		left := strings.ToLower(displayLinkTypeName(types[i]))
		right := strings.ToLower(displayLinkTypeName(types[j]))
		return left < right
	})
	return types
}

func displayLinkTypeName(linkType IssueLinkType) string {
	if strings.TrimSpace(linkType.Name) != "" {
		return strings.TrimSpace(linkType.Name)
	}
	return strings.TrimSpace(linkType.ID)
}

func (c *Client) CreateIssueLink(ctx context.Context, request CreateIssueLinkRequest) error {
	request.SourceKey = strings.ToUpper(strings.TrimSpace(request.SourceKey))
	request.TargetKey = strings.ToUpper(strings.TrimSpace(request.TargetKey))
	request.Direction = strings.ToLower(strings.TrimSpace(request.Direction))
	if request.SourceKey == "" {
		return fmt.Errorf("create jira issue link: empty source issue key")
	}
	if request.TargetKey == "" {
		return fmt.Errorf("create jira issue link %s: empty target issue key", request.SourceKey)
	}
	if request.SourceKey == request.TargetKey {
		return fmt.Errorf("create jira issue link %s: target issue must be different", request.SourceKey)
	}
	linkTypeID := strings.TrimSpace(request.Type.ID)
	linkTypeName := strings.TrimSpace(request.Type.Name)
	if linkTypeID == "" && linkTypeName == "" {
		return fmt.Errorf("create jira issue link %s: empty link type", request.SourceKey)
	}
	if request.Direction != "outward" && request.Direction != "inward" {
		return fmt.Errorf("create jira issue link %s: unsupported direction %s", request.SourceKey, request.Direction)
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	if c.rest == nil {
		return fmt.Errorf("create jira issue link %s: REST service unavailable", request.SourceKey)
	}

	payload := createIssueLinkPayload(request)
	restRequest, err := c.rest.NewRequest(ctx, http.MethodPost, "rest/api/3/issueLink", "", payload)
	if err != nil {
		return fmt.Errorf("create jira issue link %s: %w", request.SourceKey, err)
	}
	if _, err := c.rest.Call(restRequest, nil); err != nil {
		return fmt.Errorf("create jira issue link %s: %w", request.SourceKey, err)
	}
	return nil
}

func (c *Client) DeleteIssueLink(ctx context.Context, linkID string) error {
	linkID = strings.TrimSpace(linkID)
	if linkID == "" {
		return fmt.Errorf("delete jira issue link: empty link ID")
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	if c.rest == nil {
		return fmt.Errorf("delete jira issue link %s: REST service unavailable", linkID)
	}
	restRequest, err := c.rest.NewRequest(ctx, http.MethodDelete, "rest/api/3/issueLink/"+url.PathEscape(linkID), "", nil)
	if err != nil {
		return fmt.Errorf("delete jira issue link %s: %w", linkID, err)
	}
	if _, err := c.rest.Call(restRequest, nil); err != nil {
		return fmt.Errorf("delete jira issue link %s: %w", linkID, err)
	}
	return nil
}

func (c *Client) GetWorklogs(ctx context.Context, key string, maxResults int) ([]Worklog, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, fmt.Errorf("get jira worklogs: empty issue key")
	}
	if maxResults <= 0 {
		maxResults = 20
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	if c.rest == nil {
		return nil, fmt.Errorf("get jira worklogs %s: REST service unavailable", key)
	}
	endpoint := fmt.Sprintf("rest/api/3/issue/%s/worklog?maxResults=%d&startAt=0", url.PathEscape(key), maxResults)
	restRequest, err := c.rest.NewRequest(ctx, http.MethodGet, endpoint, "", nil)
	if err != nil {
		return nil, fmt.Errorf("get jira worklogs %s: %w", key, err)
	}
	var page model.IssueWorklogADFPageScheme
	if _, err := c.rest.Call(restRequest, &page); err != nil {
		return nil, fmt.Errorf("get jira worklogs %s: %w", key, err)
	}
	return parseWorklogs(page.Worklogs), nil
}

func (c *Client) AddWorklog(ctx context.Context, key string, request AddWorklogRequest) (Worklog, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return Worklog{}, fmt.Errorf("add jira worklog: empty issue key")
	}
	timeSpent := strings.TrimSpace(request.TimeSpent)
	if timeSpent == "" {
		return Worklog{}, fmt.Errorf("add jira worklog %s: empty time spent", key)
	}
	started := request.Started
	if started.IsZero() {
		started = time.Now()
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	if c.rest == nil {
		return Worklog{}, fmt.Errorf("add jira worklog %s: REST service unavailable", key)
	}
	payload := &model.WorklogADFPayloadScheme{
		TimeSpent: timeSpent,
		Started:   formatJiraWorklogTime(started),
	}
	if strings.TrimSpace(request.Comment) != "" {
		payload.Comment = plainTextADF(request.Comment, nil)
	}
	restRequest, err := c.rest.NewRequest(ctx, http.MethodPost, "rest/api/3/issue/"+url.PathEscape(key)+"/worklog", "", payload)
	if err != nil {
		return Worklog{}, fmt.Errorf("add jira worklog %s: %w", key, err)
	}
	var raw model.IssueWorklogADFScheme
	if _, err := c.rest.Call(restRequest, &raw); err != nil {
		return Worklog{}, fmt.Errorf("add jira worklog %s: %w", key, err)
	}
	return parseWorklog(&raw), nil
}

func (c *Client) UpdateWorklog(ctx context.Context, key string, request UpdateWorklogRequest) (Worklog, error) {
	key = strings.TrimSpace(key)
	request.ID = strings.TrimSpace(request.ID)
	if key == "" {
		return Worklog{}, fmt.Errorf("update jira worklog: empty issue key")
	}
	if request.ID == "" {
		return Worklog{}, fmt.Errorf("update jira worklog %s: empty worklog ID", key)
	}
	timeSpent := strings.TrimSpace(request.TimeSpent)
	if timeSpent == "" {
		return Worklog{}, fmt.Errorf("update jira worklog %s: empty time spent", key)
	}
	started := request.Started
	if started.IsZero() {
		started = time.Now()
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	if c.rest == nil {
		return Worklog{}, fmt.Errorf("update jira worklog %s: REST service unavailable", key)
	}
	payload := &model.WorklogADFPayloadScheme{
		TimeSpent: timeSpent,
		Started:   formatJiraWorklogTime(started),
	}
	if strings.TrimSpace(request.Comment) != "" {
		payload.Comment = plainTextADF(request.Comment, nil)
	}
	endpoint := fmt.Sprintf("rest/api/3/issue/%s/worklog/%s", url.PathEscape(key), url.PathEscape(request.ID))
	restRequest, err := c.rest.NewRequest(ctx, http.MethodPut, endpoint, "", payload)
	if err != nil {
		return Worklog{}, fmt.Errorf("update jira worklog %s: %w", key, err)
	}
	var raw model.IssueWorklogADFScheme
	if _, err := c.rest.Call(restRequest, &raw); err != nil {
		return Worklog{}, fmt.Errorf("update jira worklog %s: %w", key, err)
	}
	return parseWorklog(&raw), nil
}

func (c *Client) DeleteWorklog(ctx context.Context, key string, worklogID string) error {
	key = strings.TrimSpace(key)
	worklogID = strings.TrimSpace(worklogID)
	if key == "" {
		return fmt.Errorf("delete jira worklog: empty issue key")
	}
	if worklogID == "" {
		return fmt.Errorf("delete jira worklog %s: empty worklog ID", key)
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	if c.rest == nil {
		return fmt.Errorf("delete jira worklog %s: REST service unavailable", key)
	}
	endpoint := fmt.Sprintf("rest/api/3/issue/%s/worklog/%s", url.PathEscape(key), url.PathEscape(worklogID))
	restRequest, err := c.rest.NewRequest(ctx, http.MethodDelete, endpoint, "", nil)
	if err != nil {
		return fmt.Errorf("delete jira worklog %s: %w", key, err)
	}
	if _, err := c.rest.Call(restRequest, nil); err != nil {
		return fmt.Errorf("delete jira worklog %s: %w", key, err)
	}
	return nil
}

func formatJiraWorklogTime(value time.Time) string {
	return value.Format("2006-01-02T15:04:05.000-0700")
}

func createIssueLinkPayload(request CreateIssueLinkRequest) *model.LinkPayloadSchemeV3 {
	linkType := &model.LinkTypeScheme{
		ID:   strings.TrimSpace(request.Type.ID),
		Name: strings.TrimSpace(request.Type.Name),
	}
	payload := &model.LinkPayloadSchemeV3{
		Type: linkType,
	}
	source := &model.LinkedIssueScheme{Key: strings.ToUpper(strings.TrimSpace(request.SourceKey))}
	target := &model.LinkedIssueScheme{Key: strings.ToUpper(strings.TrimSpace(request.TargetKey))}
	if strings.EqualFold(request.Direction, "inward") {
		payload.InwardIssue = target
		payload.OutwardIssue = source
		return payload
	}
	payload.InwardIssue = source
	payload.OutwardIssue = target
	return payload
}

func (c *Client) fieldOptionAutocompleteEndpoint(autocompleteURL string, query string, maxResults int) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(autocompleteURL))
	if err != nil {
		return "", fmt.Errorf("search jira field options: invalid autocomplete URL: %w", err)
	}
	if parsed.IsAbs() {
		if c.baseURL != "" {
			base, err := url.Parse(c.baseURL)
			if err == nil && base.Host != "" && !strings.EqualFold(parsed.Host, base.Host) {
				return "", fmt.Errorf("search jira field options: autocomplete URL host %s does not match Jira host %s", parsed.Host, base.Host)
			}
		}
		parsed.Scheme = ""
		parsed.Host = ""
	}
	params := parsed.Query()
	if query != "" {
		params.Set("query", query)
	}
	params.Set("maxResults", strconv.Itoa(maxResults))
	parsed.RawQuery = params.Encode()
	endpoint := strings.TrimLeft(parsed.String(), "/")
	if endpoint == "" {
		return "", fmt.Errorf("search jira field options: empty autocomplete endpoint")
	}
	return endpoint, nil
}

func (c *Client) CreateIssue(ctx context.Context, request CreateIssueRequest) (Issue, error) {
	request.ProjectKey = strings.TrimSpace(request.ProjectKey)
	request.IssueTypeID = strings.TrimSpace(request.IssueTypeID)
	request.ParentKey = strings.TrimSpace(request.ParentKey)
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
	if request.ParentKey != "" {
		payload.Fields.Parent = &model.ParentScheme{Key: request.ParentKey}
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
	case "team":
		return teamFieldPayload(value.Option)
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

func teamFieldPayload(option FieldOption) (interface{}, bool) {
	id := strings.TrimSpace(option.ID)
	if id == "" {
		return nil, false
	}
	return id, true
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

func (c *Client) UpdateLabels(ctx context.Context, key string, labels []string) error {
	labels = normalizeLabels(labels)
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	payload := &model.IssueScheme{
		Fields: &model.IssueFieldsScheme{
			Labels: labels,
		},
	}
	if _, err := c.issue.Update(ctx, key, false, payload, nil, nil); err != nil {
		return fmt.Errorf("update jira labels %s: %w", key, err)
	}
	return nil
}

func (c *Client) UpdateComponents(ctx context.Context, key string, components []FieldOption) error {
	components = normalizeFieldOptions(components)
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	payload := &model.IssueScheme{
		Fields: &model.IssueFieldsScheme{
			Components: fieldOptionsToComponents(components),
		},
	}
	if _, err := c.issue.Update(ctx, key, false, payload, nil, nil); err != nil {
		return fmt.Errorf("update jira components %s: %w", key, err)
	}
	return nil
}

func (c *Client) UpdateEditField(ctx context.Context, key string, value EditFieldValue) error {
	fieldID := strings.TrimSpace(value.FieldID)
	if fieldID == "" {
		return fmt.Errorf("update jira field %s: empty field ID", key)
	}
	if !supportedEditFieldValueID(fieldID) {
		return fmt.Errorf("update jira field %s: unsupported field %s", key, fieldID)
	}
	raw, ok := editCustomFieldPayload(value)
	if !ok {
		return fmt.Errorf("update jira field %s: missing value for %s", key, fieldID)
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	if c.rest == nil {
		return fmt.Errorf("update jira field %s: REST service unavailable", key)
	}
	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			fieldID: raw,
		},
	}
	endpoint := "rest/api/3/issue/" + url.PathEscape(key)
	restRequest, err := c.rest.NewRequest(ctx, http.MethodPut, endpoint, "", payload)
	if err != nil {
		return fmt.Errorf("update jira field %s: %w", key, err)
	}
	if _, err := c.rest.Call(restRequest, nil); err != nil {
		return fmt.Errorf("update jira field %s: %w", key, err)
	}
	return nil
}

func (c *Client) UpdateParent(ctx context.Context, key string, request UpdateParentRequest) error {
	key = strings.TrimSpace(key)
	parentKey := strings.ToUpper(strings.TrimSpace(request.ParentKey))
	if key == "" {
		return fmt.Errorf("update jira parent: missing issue key")
	}
	if !request.Clear && parentKey == "" {
		return fmt.Errorf("update jira parent %s: missing parent key", key)
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	if c.rest == nil {
		return fmt.Errorf("update jira parent %s: REST service unavailable", key)
	}
	var payload map[string]interface{}
	if request.Clear {
		payload = map[string]interface{}{
			"update": map[string]interface{}{
				"parent": []map[string]interface{}{
					{"set": map[string]bool{"none": true}},
				},
			},
		}
	} else {
		payload = map[string]interface{}{
			"fields": map[string]interface{}{
				"parent": map[string]string{"key": parentKey},
			},
		}
	}
	endpoint := "rest/api/3/issue/" + url.PathEscape(key)
	restRequest, err := c.rest.NewRequest(ctx, http.MethodPut, endpoint, "", payload)
	if err != nil {
		return fmt.Errorf("update jira parent %s: %w", key, err)
	}
	if _, err := c.rest.Call(restRequest, nil); err != nil {
		return fmt.Errorf("update jira parent %s: %w", key, err)
	}
	return nil
}

func (c *Client) UpdateIssueType(ctx context.Context, key string, issueTypeID string) error {
	key = strings.TrimSpace(key)
	issueTypeID = strings.TrimSpace(issueTypeID)
	if key == "" {
		return fmt.Errorf("update jira issue type: missing issue key")
	}
	if issueTypeID == "" {
		return fmt.Errorf("update jira issue type %s: missing issue type ID", key)
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	payload := &model.IssueScheme{
		Fields: &model.IssueFieldsScheme{
			IssueType: &model.IssueTypeScheme{ID: issueTypeID},
		},
	}
	if _, err := c.issue.Update(ctx, key, false, payload, nil, nil); err != nil {
		return fmt.Errorf("update jira issue type %s: %w", key, err)
	}
	return nil
}

func (c *Client) UpdateTimeTracking(ctx context.Context, key string, request UpdateTimeTrackingRequest) error {
	key = strings.TrimSpace(key)
	original := strings.TrimSpace(request.OriginalEstimate)
	remaining := strings.TrimSpace(request.RemainingEstimate)
	if key == "" {
		return fmt.Errorf("update jira time tracking: missing issue key")
	}
	if original == "" && remaining == "" {
		return fmt.Errorf("update jira time tracking %s: enter an original or remaining estimate", key)
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	if c.rest == nil {
		return fmt.Errorf("update jira time tracking %s: REST service unavailable", key)
	}
	timeTracking := map[string]string{}
	if original != "" {
		timeTracking["originalEstimate"] = original
	}
	if remaining != "" {
		timeTracking["remainingEstimate"] = remaining
	}
	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			"timetracking": timeTracking,
		},
	}
	endpoint := "rest/api/3/issue/" + url.PathEscape(key)
	restRequest, err := c.rest.NewRequest(ctx, http.MethodPut, endpoint, "", payload)
	if err != nil {
		return fmt.Errorf("update jira time tracking %s: %w", key, err)
	}
	if _, err := c.rest.Call(restRequest, nil); err != nil {
		return fmt.Errorf("update jira time tracking %s: %w", key, err)
	}
	return nil
}

func supportedEditFieldValueID(fieldID string) bool {
	fieldID = strings.TrimSpace(fieldID)
	if strings.HasPrefix(fieldID, "customfield_") {
		return true
	}
	switch fieldID {
	case "fixVersions", "versions", "duedate":
		return true
	default:
		return false
	}
}

func editCustomFieldPayload(value EditFieldValue) (interface{}, bool) {
	text := strings.TrimSpace(value.Text)
	schemaType := strings.ToLower(strings.TrimSpace(value.SchemaType))
	schemaItems := strings.ToLower(strings.TrimSpace(value.SchemaItems))
	schemaCustom := strings.ToLower(strings.TrimSpace(value.SchemaCustom))
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
	case "string", "text", "textarea", "date", "datetime":
		if text == "" {
			return nil, false
		}
		return text, true
	case "option":
		option := createFieldOptionPayload(value.Option)
		if len(option) == 0 {
			return nil, false
		}
		return option, true
	case "team":
		return teamFieldPayload(value.Option)
	case "user":
		return userFieldPayload(value.Option)
	case "version":
		return versionFieldPayload(value.Option)
	case "array":
		options := normalizeFieldOptions(value.Options)
		if len(options) == 0 && (value.Option.ID != "" || value.Option.Name != "") {
			options = []FieldOption{value.Option}
		}
		switch {
		case strings.Contains(schemaCustom, "gh-sprint") || schemaItems == "sprint":
			return sprintFieldPayloads(options)
		case schemaItems == "option":
			return optionFieldPayloads(options)
		case schemaItems == "user":
			return userFieldPayloads(options)
		case schemaItems == "version":
			return versionFieldPayloads(options)
		default:
			return nil, false
		}
	default:
		if strings.Contains(schemaCustom, "gh-sprint") {
			return sprintFieldPayload(value.Option)
		}
		return nil, false
	}
}

func optionFieldPayloads(options []FieldOption) ([]map[string]interface{}, bool) {
	items := make([]map[string]interface{}, 0, len(options))
	for _, option := range options {
		item := createFieldOptionPayload(option)
		if len(item) > 0 {
			items = append(items, item)
		}
	}
	return items, len(items) > 0
}

func userFieldPayload(option FieldOption) (interface{}, bool) {
	accountID := strings.TrimSpace(option.ID)
	if accountID == "" {
		return nil, false
	}
	return map[string]interface{}{"accountId": accountID}, true
}

func userFieldPayloads(options []FieldOption) ([]map[string]interface{}, bool) {
	items := make([]map[string]interface{}, 0, len(options))
	for _, option := range options {
		accountID := strings.TrimSpace(option.ID)
		if accountID != "" {
			items = append(items, map[string]interface{}{"accountId": accountID})
		}
	}
	return items, len(items) > 0
}

func versionFieldPayload(option FieldOption) (interface{}, bool) {
	id := strings.TrimSpace(option.ID)
	if id == "" {
		return nil, false
	}
	return map[string]interface{}{"id": id}, true
}

func versionFieldPayloads(options []FieldOption) ([]map[string]interface{}, bool) {
	items := make([]map[string]interface{}, 0, len(options))
	for _, option := range options {
		id := strings.TrimSpace(option.ID)
		if id != "" {
			items = append(items, map[string]interface{}{"id": id})
		}
	}
	return items, len(items) > 0
}

func sprintFieldPayload(option FieldOption) (interface{}, bool) {
	id := strings.TrimSpace(option.ID)
	if id == "" {
		return nil, false
	}
	number, err := strconv.Atoi(id)
	if err == nil {
		return number, true
	}
	return map[string]interface{}{"id": id}, true
}

func sprintFieldPayloads(options []FieldOption) (interface{}, bool) {
	numbers := make([]int, 0, len(options))
	fallback := make([]map[string]interface{}, 0, len(options))
	allNumeric := true
	for _, option := range options {
		id := strings.TrimSpace(option.ID)
		if id == "" {
			continue
		}
		number, err := strconv.Atoi(id)
		if err != nil {
			allNumeric = false
			fallback = append(fallback, map[string]interface{}{"id": id})
			continue
		}
		numbers = append(numbers, number)
		fallback = append(fallback, map[string]interface{}{"id": id})
	}
	if len(fallback) == 0 {
		return nil, false
	}
	if allNumeric {
		return numbers, true
	}
	return fallback, true
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

func (c *Client) UpdateComment(ctx context.Context, key string, commentID string, body string, mentions []Mention) (Comment, error) {
	key = strings.TrimSpace(key)
	commentID = strings.TrimSpace(commentID)
	body = strings.TrimSpace(body)
	if key == "" || commentID == "" || body == "" {
		return Comment{}, fmt.Errorf("update jira comment: missing issue key, comment ID, or body")
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	if c.comment == nil {
		return Comment{}, fmt.Errorf("update jira comment %s/%s: comment service unavailable", key, commentID)
	}

	raw, _, err := c.comment.Update(ctx, key, commentID, &model.CommentPayloadScheme{
		Body: plainTextADF(body, mentions),
	}, nil)
	if err != nil {
		return Comment{}, fmt.Errorf("update jira comment %s/%s: %w", key, commentID, err)
	}
	if raw == nil {
		return Comment{}, fmt.Errorf("update jira comment %s/%s: empty response", key, commentID)
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

func (c *Client) SearchAssignableUsers(ctx context.Context, issueKey string, query string, maxResults int) ([]User, error) {
	issueKey = strings.TrimSpace(issueKey)
	query = strings.TrimSpace(query)
	if issueKey == "" {
		return nil, fmt.Errorf("search jira assignable users: empty issue key")
	}
	if query == "" {
		return nil, fmt.Errorf("search jira assignable users %s: empty query", issueKey)
	}
	if maxResults <= 0 {
		maxResults = 10
	}
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}
	if c.rest == nil {
		return nil, fmt.Errorf("search jira assignable users %s: REST service unavailable", issueKey)
	}

	params := url.Values{}
	params.Set("issueKey", issueKey)
	params.Set("maxResults", strconv.Itoa(maxResults))
	params.Set("query", query)
	endpoint := "rest/api/3/user/assignable/search?" + params.Encode()
	request, err := c.rest.NewRequest(ctx, http.MethodGet, endpoint, "", nil)
	if err != nil {
		return nil, fmt.Errorf("search jira assignable users %s: %w", issueKey, err)
	}
	var rawUsers []model.UserScheme
	if _, err := c.rest.Call(request, &rawUsers); err != nil {
		return nil, fmt.Errorf("search jira assignable users %s: %w", issueKey, err)
	}
	users := make([]User, 0, len(rawUsers))
	for index := range rawUsers {
		if user, ok := parseUser(&rawUsers[index]); ok {
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

func (c *Client) parseIssueDetail(raw *model.IssueScheme, rawFields map[string]json.RawMessage) IssueDetail {
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
	detail.OriginalEstimate, detail.RemainingEstimate = parseTimeTracking(rawFields["timetracking"])
	detail.IssueLinks = c.parseIssueLinks(fields.IssueLinks)
	detail.Reporter = jiraUserDisplayName(fields.Reporter, detail.Reporter)
	detail.Creator = jiraUserDisplayName(fields.Creator, detail.Creator)
	if teamFieldID := strings.TrimSpace(c.teamFieldID); teamFieldID != "" {
		detail.TeamID, detail.TeamName = parseTeam(rawFields[teamFieldID])
	}
	if fields.Created != nil {
		detail.Created = time.Time(*fields.Created)
	}
	if fields.Updated != nil {
		detail.Updated = time.Time(*fields.Updated)
	}
	return detail
}

func parseTeam(raw json.RawMessage) (string, string) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", ""
	}
	var payload struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", ""
	}
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		name = strings.TrimSpace(payload.Title)
	}
	return strings.TrimSpace(payload.ID), name
}

func parseTimeTracking(raw json.RawMessage) (string, string) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", ""
	}
	var payload struct {
		OriginalEstimate  string `json:"originalEstimate"`
		RemainingEstimate string `json:"remainingEstimate"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", ""
	}
	return strings.TrimSpace(payload.OriginalEstimate), strings.TrimSpace(payload.RemainingEstimate)
}

func (c *Client) parseIssueLinks(rawLinks []*model.IssueLinkScheme) []IssueLink {
	links := make([]IssueLink, 0, len(rawLinks))
	for _, raw := range rawLinks {
		if raw == nil {
			continue
		}
		if link, ok := c.parseIssueLink(raw.ID, "outward", relationshipText(raw.Type, "outward"), raw.OutwardIssue); ok {
			links = append(links, link)
		}
		if link, ok := c.parseIssueLink(raw.ID, "inward", relationshipText(raw.Type, "inward"), raw.InwardIssue); ok {
			links = append(links, link)
		}
	}
	return links
}

func (c *Client) parseIssueLink(linkID string, direction string, relationship string, raw *model.LinkedIssueScheme) (IssueLink, bool) {
	if raw == nil || strings.TrimSpace(raw.Key) == "" {
		return IssueLink{}, false
	}
	link := IssueLink{
		LinkID:       strings.TrimSpace(linkID),
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

func parseWorklogs(rawWorklogs []*model.IssueWorklogADFScheme) []Worklog {
	worklogs := make([]Worklog, 0, len(rawWorklogs))
	for _, raw := range rawWorklogs {
		worklog := parseWorklog(raw)
		if worklog.ID == "" && worklog.TimeSpent == "" && worklog.Comment == "" {
			continue
		}
		worklogs = append(worklogs, worklog)
	}
	return worklogs
}

func parseWorklog(raw *model.IssueWorklogADFScheme) Worklog {
	worklog := Worklog{
		Author: "Unknown",
	}
	if raw == nil {
		return worklog
	}
	worklog.ID = raw.ID
	worklog.Author = jiraUserDetailDisplayName(raw.Author, worklog.Author)
	worklog.Comment = adf.Render(raw.Comment)
	worklog.TimeSpent = raw.TimeSpent
	worklog.TimeSpentSeconds = raw.TimeSpentSeconds
	worklog.Started = parseJiraCommentTime(raw.Started)
	worklog.Updated = parseJiraCommentTime(raw.Updated)
	return worklog
}

func parseBoardPage(raw *model.BoardPageScheme) BoardPage {
	if raw == nil {
		return BoardPage{}
	}
	page := BoardPage{
		StartAt:    raw.StartAt,
		MaxResults: raw.MaxResults,
		Total:      raw.Total,
		IsLast:     raw.IsLast,
		Boards:     make([]Board, 0, len(raw.Values)),
	}
	for _, rawBoard := range raw.Values {
		if rawBoard == nil {
			continue
		}
		board := Board{
			ID:   rawBoard.ID,
			Name: rawBoard.Name,
			Type: rawBoard.Type,
		}
		if rawBoard.Location != nil {
			board.ProjectKey = rawBoard.Location.ProjectKey
			board.ProjectName = rawBoard.Location.ProjectName
		}
		page.Boards = append(page.Boards, board)
	}
	return page
}

func parseSprintPage(raw *model.BoardSprintPageScheme) SprintPage {
	if raw == nil {
		return SprintPage{}
	}
	page := SprintPage{
		StartAt:    raw.StartAt,
		MaxResults: raw.MaxResults,
		Total:      raw.Total,
		IsLast:     raw.IsLast,
		Sprints:    make([]Sprint, 0, len(raw.Values)),
	}
	for _, rawSprint := range raw.Values {
		if rawSprint == nil {
			continue
		}
		page.Sprints = append(page.Sprints, Sprint{
			ID:           rawSprint.ID,
			BoardID:      rawSprint.OriginBoardID,
			Name:         rawSprint.Name,
			State:        rawSprint.State,
			Goal:         rawSprint.Goal,
			StartDate:    rawSprint.StartDate,
			EndDate:      rawSprint.EndDate,
			CompleteDate: rawSprint.CompleteDate,
		})
	}
	return page
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

func parseEditMetadata(raw gjson.Result) EditMetadata {
	fields := parseEditFields(raw.Get("fields"))
	return EditMetadata{
		Summary:    parseEditField("summary", raw.Get("fields.summary")),
		Priority:   parseEditField("priority", raw.Get("fields.priority")),
		Labels:     parseEditField("labels", raw.Get("fields.labels")),
		Components: parseEditField("components", raw.Get("fields.components")),
		Fields:     fields,
	}
}

func parseEditFields(raw gjson.Result) []EditField {
	var fields []EditField
	raw.ForEach(func(key, value gjson.Result) bool {
		field := parseEditField(strings.TrimSpace(key.String()), value)
		if field.ID == "" && field.Name == "" {
			return true
		}
		fields = append(fields, field)
		return true
	})
	sort.SliceStable(fields, func(i, j int) bool {
		left := strings.ToLower(strings.TrimSpace(firstNonEmpty(fields[i].Name, fields[i].ID)))
		right := strings.ToLower(strings.TrimSpace(firstNonEmpty(fields[j].Name, fields[j].ID)))
		if left == right {
			return fields[i].ID < fields[j].ID
		}
		return left < right
	})
	return fields
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
		ID:              strings.TrimSpace(id),
		Name:            strings.TrimSpace(raw.Get("name").String()),
		Required:        raw.Get("required").Bool(),
		HasDefaultValue: raw.Get("hasDefaultValue").Bool(),
		SchemaType:      strings.TrimSpace(raw.Get("schema.type").String()),
		SchemaSystem:    strings.TrimSpace(raw.Get("schema.system").String()),
		SchemaItems:     strings.TrimSpace(raw.Get("schema.items").String()),
		SchemaCustom:    strings.TrimSpace(raw.Get("schema.custom").String()),
		SchemaCustomID:  int(raw.Get("schema.customId").Int()),
		AutoCompleteURL: strings.TrimSpace(raw.Get("autoCompleteUrl").String()),
	}
	for _, operation := range raw.Get("operations").Array() {
		value := strings.TrimSpace(operation.String())
		if value == "" {
			continue
		}
		field.Operations = append(field.Operations, value)
		if value == "set" {
			field.Editable = true
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

func parseFieldOptionSearchResponse(response fieldOptionSearchResponse) []FieldOption {
	items := response.Results
	if len(items) == 0 {
		items = response.Values
	}
	options := make([]FieldOption, 0, len(items))
	for _, item := range items {
		option := FieldOption{
			ID: strings.TrimSpace(item.ID),
			Name: strings.TrimSpace(firstNonEmpty(
				item.Name,
				item.Value,
				item.DisplayName,
				item.Key,
			)),
		}
		if option.ID == "" && option.Name == "" {
			continue
		}
		options = append(options, option)
	}
	return normalizeFieldOptions(options)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizeLabels(labels []string) []string {
	normalized := make([]string, 0, len(labels))
	seen := map[string]bool{}
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		key := strings.ToLower(label)
		if seen[key] {
			continue
		}
		seen[key] = true
		normalized = append(normalized, label)
	}
	if normalized == nil {
		return []string{}
	}
	return normalized
}

func normalizeFieldOptions(options []FieldOption) []FieldOption {
	normalized := make([]FieldOption, 0, len(options))
	seen := map[string]bool{}
	for _, option := range options {
		option.ID = strings.TrimSpace(option.ID)
		option.Name = strings.TrimSpace(option.Name)
		if option.ID == "" && option.Name == "" {
			continue
		}
		key := strings.ToLower(firstNonEmpty(option.ID, option.Name))
		if seen[key] {
			continue
		}
		seen[key] = true
		normalized = append(normalized, option)
	}
	if normalized == nil {
		return []FieldOption{}
	}
	return normalized
}

func fieldOptionsToComponents(options []FieldOption) []*model.ComponentScheme {
	components := make([]*model.ComponentScheme, 0, len(options))
	for _, option := range options {
		components = append(components, &model.ComponentScheme{
			ID:   strings.TrimSpace(option.ID),
			Name: strings.TrimSpace(option.Name),
		})
	}
	if components == nil {
		return []*model.ComponentScheme{}
	}
	return components
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

func jiraUserDetailDisplayName(raw *model.UserDetailScheme, fallback string) string {
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

func (f failingCommentService) Update(_ context.Context, _, _ string, _ *model.CommentPayloadScheme, _ []string) (*model.IssueCommentScheme, *model.ResponseScheme, error) {
	return nil, nil, f.err
}

type failingUserSearchService struct {
	err error
}

func (f failingUserSearchService) Do(_ context.Context, _, _ string, _, _ int) ([]*model.UserScheme, *model.ResponseScheme, error) {
	return nil, nil, f.err
}

type failingMyselfService struct {
	err error
}

func (f failingMyselfService) Details(_ context.Context, _ []string) (*model.UserScheme, *model.ResponseScheme, error) {
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

type failingAgileBoardService struct {
	err error
}

func (f failingAgileBoardService) Gets(_ context.Context, _ *model.GetBoardsOptions, _, _ int) (*model.BoardPageScheme, *model.ResponseScheme, error) {
	return nil, nil, f.err
}

func (f failingAgileBoardService) Issues(_ context.Context, _ int, _ *model.IssueOptionScheme, _, _ int) (*model.BoardIssuePageScheme, *model.ResponseScheme, error) {
	return nil, nil, f.err
}

func (f failingAgileBoardService) Sprints(_ context.Context, _ int, _, _ int, _ []string) (*model.BoardSprintPageScheme, *model.ResponseScheme, error) {
	return nil, nil, f.err
}
