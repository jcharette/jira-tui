package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jon/jira-tui/internal/jira"
	"github.com/panjf2000/ants/v2"
)

const (
	defaultWorkerCount = 2
	defaultQueueSize   = 16
)

var (
	ErrQueueFull      = errors.New("worker queue is full")
	ErrPoolClosed     = errors.New("worker pool is closed")
	ErrInvalidRequest = errors.New("invalid worker request")
	ErrWorkerPanic    = errors.New("worker panicked")
)

type Kind string

const (
	KindSearchIssues Kind = "search_issues"
	KindGetIssue     Kind = "get_issue"
	KindGetComments  Kind = "get_comments"
	KindAddComment   Kind = "add_comment"
	KindSearchUsers  Kind = "search_users"
	KindExpandIssues Kind = "expand_issues"
)

type JiraClient interface {
	SearchIssues(ctx context.Context, jql string, maxResults int) ([]jira.Issue, error)
	GetIssue(ctx context.Context, key string) (jira.IssueDetail, error)
	GetComments(ctx context.Context, key string, maxResults int) ([]jira.Comment, error)
	AddComment(ctx context.Context, key string, body string, mentions []jira.Mention) (jira.Comment, error)
	SearchUsers(ctx context.Context, query string, maxResults int) ([]jira.User, error)
}

type Request struct {
	ID      int
	Kind    Kind
	Timeout time.Duration

	SearchIssues *SearchIssuesRequest
	GetIssue     *GetIssueRequest
	GetComments  *GetCommentsRequest
	AddComment   *AddCommentRequest
	SearchUsers  *SearchUsersRequest
	ExpandIssues *ExpandIssuesRequest
}

type SearchIssuesRequest struct {
	JQL        string
	MaxResults int
}

type GetIssueRequest struct {
	Key string
}

type GetCommentsRequest struct {
	Key        string
	MaxResults int
}

type AddCommentRequest struct {
	Key      string
	Body     string
	Mentions []jira.Mention
}

type SearchUsersRequest struct {
	Query      string
	MaxResults int
}

type ExpandMode string

const (
	ExpandModeOpen ExpandMode = "open"
	ExpandModeAll  ExpandMode = "all"
)

type ExpandIssuesRequest struct {
	ParentKey  string
	Mode       ExpandMode
	MaxResults int
}

type Result struct {
	ID   int
	Kind Kind
	Err  error

	SearchIssues *SearchIssuesResult
	GetIssue     *GetIssueResult
	GetComments  *GetCommentsResult
	AddComment   *AddCommentResult
	SearchUsers  *SearchUsersResult
	ExpandIssues *ExpandIssuesResult
}

type SearchIssuesResult struct {
	Issues   []jira.Issue
	SyncedAt time.Time
}

type GetIssueResult struct {
	Key      string
	Detail   jira.IssueDetail
	SyncedAt time.Time
}

type GetCommentsResult struct {
	Key      string
	Comments []jira.Comment
	SyncedAt time.Time
}

type AddCommentResult struct {
	Key      string
	Comment  jira.Comment
	SyncedAt time.Time
}

type SearchUsersResult struct {
	Query    string
	Users    []jira.User
	SyncedAt time.Time
}

type ExpandIssuesResult struct {
	ParentKey string
	Mode      ExpandMode
	Issues    []jira.Issue
	SyncedAt  time.Time
}

type Option func(*Pool)

type Pool struct {
	client JiraClient

	workerCount int
	queueSize   int

	engine    *ants.Pool
	admission chan struct{}
	results   chan Result
	done      chan struct{}
	initErr   error
	wg        sync.WaitGroup
	stopOnce  sync.Once
	mu        sync.Mutex
	closed    bool
}

func NewPool(client JiraClient, options ...Option) *Pool {
	pool := &Pool{
		client:      client,
		workerCount: defaultWorkerCount,
		queueSize:   defaultQueueSize,
		done:        make(chan struct{}),
	}
	for _, option := range options {
		option(pool)
	}
	if pool.workerCount <= 0 {
		pool.workerCount = defaultWorkerCount
	}
	if pool.queueSize <= 0 {
		pool.queueSize = defaultQueueSize
	}

	capacity := pool.workerCount + pool.queueSize
	pool.results = make(chan Result, capacity)
	pool.admission = make(chan struct{}, capacity)
	pool.engine, pool.initErr = ants.NewPool(pool.workerCount)
	return pool
}

func WithWorkerCount(count int) Option {
	return func(pool *Pool) {
		pool.workerCount = count
	}
}

func WithQueueSize(size int) Option {
	return func(pool *Pool) {
		pool.queueSize = size
	}
}

func (p *Pool) Submit(request Request) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.initErr != nil {
		return p.initErr
	}
	if p.closed {
		return ErrPoolClosed
	}

	select {
	case p.admission <- struct{}{}:
	default:
		return ErrQueueFull
	}

	p.wg.Add(1)
	go p.submitToEngine(request)
	return nil
}

func (p *Pool) Results() <-chan Result {
	return p.results
}

func (p *Pool) Stop() {
	p.stopOnce.Do(func() {
		p.mu.Lock()
		p.closed = true
		close(p.done)
		p.mu.Unlock()

		if p.engine != nil {
			p.engine.Release()
		}
		p.wg.Wait()
		close(p.results)
	})
}

func (p *Pool) send(result Result) {
	select {
	case p.results <- result:
	case <-p.done:
	}
}

func (p *Pool) submitToEngine(request Request) {
	err := p.engine.Submit(func() {
		defer p.wg.Done()
		defer p.releaseAdmission()
		defer func() {
			if recovered := recover(); recovered != nil {
				p.send(Result{ID: request.ID, Kind: request.Kind, Err: ErrWorkerPanic})
			}
		}()

		p.send(p.handle(request))
	})
	if err != nil {
		p.wg.Done()
		p.releaseAdmission()
		p.send(Result{ID: request.ID, Kind: request.Kind, Err: mapAntsError(err)})
	}
}

func (p *Pool) releaseAdmission() {
	select {
	case <-p.admission:
	default:
	}
}

func mapAntsError(err error) error {
	switch {
	case errors.Is(err, ants.ErrPoolClosed):
		return ErrPoolClosed
	case errors.Is(err, ants.ErrPoolOverload):
		return ErrQueueFull
	default:
		return err
	}
}

func (p *Pool) handle(request Request) Result {
	switch request.Kind {
	case KindSearchIssues:
		return p.handleSearchIssues(request)
	case KindGetIssue:
		return p.handleGetIssue(request)
	case KindGetComments:
		return p.handleGetComments(request)
	case KindAddComment:
		return p.handleAddComment(request)
	case KindSearchUsers:
		return p.handleSearchUsers(request)
	case KindExpandIssues:
		return p.handleExpandIssues(request)
	default:
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}
}

func (p *Pool) handleExpandIssues(request Request) Result {
	if request.ExpandIssues == nil || request.ExpandIssues.ParentKey == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	maxResults := request.ExpandIssues.MaxResults
	if maxResults <= 0 {
		maxResults = 50
	}
	mode := request.ExpandIssues.Mode
	if mode == "" {
		mode = ExpandModeOpen
	}
	issues, err := p.client.SearchIssues(ctx, expandIssuesJQL(request.ExpandIssues.ParentKey, mode), maxResults)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		ExpandIssues: &ExpandIssuesResult{
			ParentKey: request.ExpandIssues.ParentKey,
			Mode:      mode,
			Issues:    issues,
			SyncedAt:  time.Now(),
		},
	}
}

func (p *Pool) handleSearchUsers(request Request) Result {
	if request.SearchUsers == nil || request.SearchUsers.Query == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	users, err := p.client.SearchUsers(ctx, request.SearchUsers.Query, request.SearchUsers.MaxResults)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		SearchUsers: &SearchUsersResult{
			Query:    request.SearchUsers.Query,
			Users:    users,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) withMissingParents(ctx context.Context, issues []jira.Issue) []jira.Issue {
	if len(issues) == 0 {
		return issues
	}
	seen := make(map[string]bool, len(issues))
	for _, issue := range issues {
		seen[issue.Key] = true
	}

	parentKeys := make([]string, 0)
	parentSeen := make(map[string]bool)
	for _, issue := range issues {
		if issue.ParentKey == "" || seen[issue.ParentKey] || parentSeen[issue.ParentKey] {
			continue
		}
		parentKeys = append(parentKeys, issue.ParentKey)
		parentSeen[issue.ParentKey] = true
	}
	if len(parentKeys) == 0 {
		return issues
	}

	parents := make(map[string]jira.Issue, len(parentKeys))
	for _, key := range parentKeys {
		detail, err := p.client.GetIssue(ctx, key)
		if err != nil || detail.Key == "" {
			continue
		}
		parents[key] = detail.Issue
	}
	if len(parents) == 0 {
		return issues
	}

	enriched := make([]jira.Issue, 0, len(issues)+len(parents))
	inserted := make(map[string]bool, len(parents))
	for _, issue := range issues {
		if parent, ok := parents[issue.ParentKey]; ok && !inserted[parent.Key] {
			enriched = append(enriched, parent)
			inserted[parent.Key] = true
		}
		enriched = append(enriched, issue)
	}
	return enriched
}

func withKnownSubtasks(issues []jira.Issue) []jira.Issue {
	if len(issues) == 0 {
		return issues
	}
	seen := make(map[string]bool, len(issues))
	for _, issue := range issues {
		seen[issue.Key] = true
	}
	enriched := append([]jira.Issue(nil), issues...)
	for _, issue := range issues {
		for _, subtask := range issue.Subtasks {
			if subtask.Key == "" || seen[subtask.Key] {
				continue
			}
			enriched = append(enriched, subtask)
			seen[subtask.Key] = true
		}
	}
	return enriched
}

func (p *Pool) handleAddComment(request Request) Result {
	if request.AddComment == nil || request.AddComment.Key == "" || request.AddComment.Body == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	comment, err := p.client.AddComment(ctx, request.AddComment.Key, request.AddComment.Body, request.AddComment.Mentions)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		AddComment: &AddCommentResult{
			Key:      request.AddComment.Key,
			Comment:  comment,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) handleGetComments(request Request) Result {
	if request.GetComments == nil || request.GetComments.Key == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	comments, err := p.client.GetComments(ctx, request.GetComments.Key, request.GetComments.MaxResults)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		GetComments: &GetCommentsResult{
			Key:      request.GetComments.Key,
			Comments: comments,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) handleSearchIssues(request Request) Result {
	if request.SearchIssues == nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	issues, err := p.client.SearchIssues(
		ctx,
		request.SearchIssues.JQL,
		request.SearchIssues.MaxResults,
	)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}
	issues = p.withMissingParents(ctx, issues)
	issues = withKnownSubtasks(issues)
	issues = p.withChildIssues(ctx, issues, request.SearchIssues.MaxResults)

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		SearchIssues: &SearchIssuesResult{
			Issues:   issues,
			SyncedAt: time.Now(),
		},
	}
}

func (p *Pool) withChildIssues(ctx context.Context, issues []jira.Issue, maxResults int) []jira.Issue {
	parentKeys := childLookupParentKeys(issues)
	if len(parentKeys) == 0 {
		return issues
	}
	if maxResults <= 0 {
		maxResults = 50
	}
	children, err := p.client.SearchIssues(ctx, childLookupJQL(parentKeys), maxResults)
	if err != nil {
		return issues
	}
	if len(children) == 0 {
		return issues
	}
	seen := make(map[string]bool, len(issues)+len(children))
	for _, issue := range issues {
		seen[issue.Key] = true
	}
	enriched := append([]jira.Issue(nil), issues...)
	for _, child := range children {
		if child.Key == "" || child.ParentKey == "" || seen[child.Key] {
			continue
		}
		enriched = append(enriched, child)
		seen[child.Key] = true
	}
	return enriched
}

func childLookupParentKeys(issues []jira.Issue) []string {
	keys := make([]string, 0, len(issues))
	seen := make(map[string]bool, len(issues))
	for _, issue := range issues {
		if issue.Key == "" || issue.IsSubtask || seen[issue.Key] {
			continue
		}
		keys = append(keys, issue.Key)
		seen[issue.Key] = true
	}
	return keys
}

func childLookupJQL(parentKeys []string) string {
	return fmt.Sprintf("parent in (%s) ORDER BY key ASC", joinJQLKeys(parentKeys))
}

func expandIssuesJQL(parentKey string, mode ExpandMode) string {
	base := fmt.Sprintf("parent = %s", parentKey)
	if mode == ExpandModeOpen {
		base += " AND statusCategory != Done"
	}
	return base + " ORDER BY key ASC"
}

func joinJQLKeys(keys []string) string {
	result := ""
	for index, key := range keys {
		if index > 0 {
			result += ", "
		}
		result += key
	}
	return result
}

func (p *Pool) handleGetIssue(request Request) Result {
	if request.GetIssue == nil || request.GetIssue.Key == "" {
		return Result{ID: request.ID, Kind: request.Kind, Err: ErrInvalidRequest}
	}

	ctx := context.Background()
	if request.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, request.Timeout)
		defer cancel()
	}

	detail, err := p.client.GetIssue(ctx, request.GetIssue.Key)
	if err != nil {
		return Result{ID: request.ID, Kind: request.Kind, Err: err}
	}

	return Result{
		ID:   request.ID,
		Kind: request.Kind,
		GetIssue: &GetIssueResult{
			Key:      request.GetIssue.Key,
			Detail:   detail,
			SyncedAt: time.Now(),
		},
	}
}
