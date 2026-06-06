package jira

import (
	"context"
	"fmt"
	"net/http"
	"time"

	atlassian "github.com/ctreminiom/go-atlassian/v2/jira/v3"
	model "github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"
	jiraservice "github.com/ctreminiom/go-atlassian/v2/service/jira"
	"github.com/jon/jira-tui/internal/config"
)

const defaultMaxResults = 25

type Client struct {
	baseURL        string
	search         issueSearchService
	requestTimeout time.Duration
}

type issueSearchService interface {
	SearchJQL(ctx context.Context, jql string, fields, expands []string, maxResults int, nextPageToken string) (*model.IssueSearchJQLScheme, *model.ResponseScheme, error)
}

type Issue struct {
	Key      string
	Summary  string
	Status   string
	Assignee string
	URL      string
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
			requestTimeout: requestTimeout,
		}
	}
	api.Auth.SetBasicAuth(cfg.Email, cfg.APIToken)

	return newClient(cfg.BaseURL, api.Issue.Search, requestTimeout)
}

func newClient(baseURL string, search jiraservice.SearchADFConnector, requestTimeout time.Duration) *Client {
	return &Client{
		baseURL:        baseURL,
		search:         search,
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

	fields := []string{"summary", "status", "assignee"}
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

func (c *Client) parseIssue(raw *model.IssueScheme) Issue {
	if raw == nil {
		return Issue{
			Status:   "Unknown",
			Assignee: "Unassigned",
		}
	}

	fields := raw.Fields
	assignee := "Unassigned"
	if fields != nil && fields.Assignee != nil && fields.Assignee.DisplayName != "" {
		assignee = fields.Assignee.DisplayName
	}

	status := "Unknown"
	if fields != nil && fields.Status != nil && fields.Status.Name != "" {
		status = fields.Status.Name
	}

	summary := ""
	if fields != nil {
		summary = fields.Summary
	}

	return Issue{
		Key:      raw.Key,
		Summary:  summary,
		Status:   status,
		Assignee: assignee,
		URL:      c.baseURL + "/browse/" + raw.Key,
	}
}

type failingSearchService struct {
	err error
}

func (f failingSearchService) SearchJQL(_ context.Context, _ string, _, _ []string, _ int, _ string) (*model.IssueSearchJQLScheme, *model.ResponseScheme, error) {
	return nil, nil, f.err
}
