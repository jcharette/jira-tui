package prprovider

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestGitHubCLIProviderCurrentPRParsesView(t *testing.T) {
	provider := NewGitHubCLIProviderWithRunner(func(_ context.Context, dir string, name string, args ...string) ([]byte, error) {
		if dir != "/repo" || name != "gh" || strings.Join(args, " ") != "pr view --json url,title,state" {
			t.Fatalf("command = dir:%q name:%q args:%q", dir, name, strings.Join(args, " "))
		}
		return []byte(`{"url":"https://github.com/acme/repo/pull/12","title":"ABC-123: Prepare release","state":"OPEN"}`), nil
	})

	pr, ok, err := provider.CurrentPR(context.Background(), "/repo")

	if err != nil {
		t.Fatalf("CurrentPR() error = %v", err)
	}
	if !ok || pr.URL != "https://github.com/acme/repo/pull/12" || pr.Title != "ABC-123: Prepare release" || pr.State != "OPEN" {
		t.Fatalf("pr = %#v ok=%v", pr, ok)
	}
}

func TestGitHubCLIProviderCurrentPRTreatsNoPRAsEmpty(t *testing.T) {
	provider := NewGitHubCLIProviderWithRunner(func(context.Context, string, string, ...string) ([]byte, error) {
		return []byte("no pull requests found for branch"), errors.New("exit status 1")
	})

	_, ok, err := provider.CurrentPR(context.Background(), "/repo")

	if err != nil {
		t.Fatalf("CurrentPR() error = %v", err)
	}
	if ok {
		t.Fatal("expected no current PR")
	}
}

func TestGitHubCLIProviderCreateOrUpdateReturnsExistingPR(t *testing.T) {
	calls := 0
	provider := NewGitHubCLIProviderWithRunner(func(context.Context, string, string, ...string) ([]byte, error) {
		calls++
		return []byte(`{"url":"https://github.com/acme/repo/pull/12","title":"ABC-123: Prepare release","state":"OPEN"}`), nil
	})

	pr, err := provider.CreateOrUpdatePR(context.Background(), Request{RepoPath: "/repo", Title: "ABC-123: Prepare release"})

	if err != nil {
		t.Fatalf("CreateOrUpdatePR() error = %v", err)
	}
	if calls != 1 || pr.URL != "https://github.com/acme/repo/pull/12" || pr.Created {
		t.Fatalf("calls=%d pr=%#v", calls, pr)
	}
}

func TestGitHubCLIProviderCreateOrUpdateCreatesDraftPR(t *testing.T) {
	var commands []string
	provider := NewGitHubCLIProviderWithRunner(func(_ context.Context, _ string, _ string, args ...string) ([]byte, error) {
		commands = append(commands, strings.Join(args, " "))
		if len(commands) == 1 {
			return []byte("no pull request found"), errors.New("exit status 1")
		}
		return []byte("https://github.com/acme/repo/pull/13\n"), nil
	})

	pr, err := provider.CreateOrUpdatePR(context.Background(), Request{
		RepoPath:   "/repo",
		BaseBranch: "main",
		Title:      "ABC-123: Prepare release",
		Body:       "Summary:\n- Prepare release",
		Draft:      true,
	})

	if err != nil {
		t.Fatalf("CreateOrUpdatePR() error = %v", err)
	}
	if len(commands) != 2 {
		t.Fatalf("commands = %#v", commands)
	}
	if !strings.Contains(commands[1], "pr create --title ABC-123: Prepare release --body Summary:") ||
		!strings.Contains(commands[1], "--draft") ||
		!strings.Contains(commands[1], "--base main") {
		t.Fatalf("create command = %q", commands[1])
	}
	if pr.URL != "https://github.com/acme/repo/pull/13" || !pr.Created {
		t.Fatalf("pr = %#v", pr)
	}
}
