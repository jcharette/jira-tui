package prprovider

import "context"

type PullRequest struct {
	URL     string
	Title   string
	State   string
	Created bool
}

type Request struct {
	RepoPath   string
	Branch     string
	BaseBranch string
	Title      string
	Body       string
	Draft      bool
}

type Provider interface {
	CurrentPR(ctx context.Context, repoPath string) (PullRequest, bool, error)
	CreateOrUpdatePR(ctx context.Context, request Request) (PullRequest, error)
}
