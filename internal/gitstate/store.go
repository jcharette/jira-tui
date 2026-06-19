package gitstate

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Store struct {
	path string
	now  func() time.Time
}

type Option func(*Store)

type ReportedCommit struct {
	RepoPath   string    `json:"repo_path"`
	Branch     string    `json:"branch"`
	IssueKey   string    `json:"issue_key"`
	SHA        string    `json:"sha"`
	Subject    string    `json:"subject,omitempty"`
	ReportedAt time.Time `json:"reported_at"`
}

type fileState struct {
	ReportedCommits []ReportedCommit `json:"reported_commits"`
}

func DefaultPath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "jira-tui", "git-workflows.json"), nil
}

func Open(path string, options ...Option) (*Store, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("git state path is required")
	}
	store := &Store{path: path, now: time.Now}
	for _, option := range options {
		option(store)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := writeState(path, fileState{}); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	return store, nil
}

func OpenDefault() (*Store, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return Open(path)
}

func WithNow(now func() time.Time) Option {
	return func(store *Store) {
		if now != nil {
			store.now = now
		}
	}
}

func (s *Store) ReportedCommits(ctx context.Context, repoPath, branch, issueKey string) ([]ReportedCommit, error) {
	state, err := s.read(ctx)
	if err != nil {
		return nil, err
	}
	repoPath, branch, issueKey = normalizeKey(repoPath), normalizeKey(branch), normalizeIssueKey(issueKey)
	matches := make([]ReportedCommit, 0)
	for _, record := range state.ReportedCommits {
		if normalizeKey(record.RepoPath) == repoPath && normalizeKey(record.Branch) == branch && normalizeIssueKey(record.IssueKey) == issueKey {
			matches = append(matches, record)
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].ReportedAt.Before(matches[j].ReportedAt)
	})
	return matches, nil
}

func (s *Store) MarkReported(ctx context.Context, records []ReportedCommit) error {
	if len(records) == 0 {
		return nil
	}
	state, err := s.read(ctx)
	if err != nil {
		return err
	}
	index := make(map[string]int, len(state.ReportedCommits))
	for i, record := range state.ReportedCommits {
		index[recordKey(record)] = i
	}
	for _, record := range records {
		record.RepoPath = strings.TrimSpace(record.RepoPath)
		record.Branch = strings.TrimSpace(record.Branch)
		record.IssueKey = normalizeIssueKey(record.IssueKey)
		record.SHA = strings.TrimSpace(record.SHA)
		record.Subject = strings.TrimSpace(record.Subject)
		if record.RepoPath == "" || record.Branch == "" || record.IssueKey == "" || record.SHA == "" {
			return errors.New("reported commit repo, branch, issue key, and sha are required")
		}
		if record.ReportedAt.IsZero() {
			record.ReportedAt = s.now()
		}
		key := recordKey(record)
		if existing, ok := index[key]; ok {
			state.ReportedCommits[existing] = record
			continue
		}
		index[key] = len(state.ReportedCommits)
		state.ReportedCommits = append(state.ReportedCommits, record)
	}
	sort.Slice(state.ReportedCommits, func(i, j int) bool {
		if state.ReportedCommits[i].RepoPath != state.ReportedCommits[j].RepoPath {
			return state.ReportedCommits[i].RepoPath < state.ReportedCommits[j].RepoPath
		}
		if state.ReportedCommits[i].Branch != state.ReportedCommits[j].Branch {
			return state.ReportedCommits[i].Branch < state.ReportedCommits[j].Branch
		}
		if state.ReportedCommits[i].IssueKey != state.ReportedCommits[j].IssueKey {
			return state.ReportedCommits[i].IssueKey < state.ReportedCommits[j].IssueKey
		}
		return state.ReportedCommits[i].SHA < state.ReportedCommits[j].SHA
	})
	return writeState(s.path, state)
}

func (s *Store) read(ctx context.Context) (fileState, error) {
	select {
	case <-ctx.Done():
		return fileState{}, ctx.Err()
	default:
	}
	payload, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return fileState{}, nil
	}
	if err != nil {
		return fileState{}, err
	}
	if len(strings.TrimSpace(string(payload))) == 0 {
		return fileState{}, nil
	}
	var state fileState
	if err := json.Unmarshal(payload, &state); err != nil {
		return fileState{}, err
	}
	return state, nil
}

func writeState(path string, state fileState) error {
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o600)
}

func recordKey(record ReportedCommit) string {
	return strings.Join([]string{
		normalizeKey(record.RepoPath),
		normalizeKey(record.Branch),
		normalizeIssueKey(record.IssueKey),
		strings.TrimSpace(record.SHA),
	}, "\x00")
}

func normalizeKey(value string) string {
	return strings.TrimSpace(value)
}

func normalizeIssueKey(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}
