package config

import (
	"testing"
	"time"
)

func TestFromEnv(t *testing.T) {
	t.Setenv("JIRA_BASE_URL", "https://example.atlassian.net/")
	t.Setenv("JIRA_EMAIL", "person@example.com")
	t.Setenv("JIRA_API_TOKEN", "secret")
	t.Setenv("JIRA_JQL", "project = ABC")
	t.Setenv("JIRA_REFRESH_INTERVAL", "30s")
	t.Setenv("JIRA_REQUEST_TIMEOUT", "5s")
	t.Setenv("JIRA_WORKERS", "4")
	t.Setenv("JIRA_QUEUE_SIZE", "32")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() error = %v", err)
	}

	if cfg.BaseURL != "https://example.atlassian.net" {
		t.Fatalf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.DefaultJQL != "project = ABC" {
		t.Fatalf("DefaultJQL = %q", cfg.DefaultJQL)
	}
	if cfg.RefreshInterval != 30*time.Second {
		t.Fatalf("RefreshInterval = %s", cfg.RefreshInterval)
	}
	if cfg.RequestTimeout != 5*time.Second {
		t.Fatalf("RequestTimeout = %s", cfg.RequestTimeout)
	}
	if cfg.WorkerCount != 4 {
		t.Fatalf("WorkerCount = %d", cfg.WorkerCount)
	}
	if cfg.QueueSize != 32 {
		t.Fatalf("QueueSize = %d", cfg.QueueSize)
	}
}

func TestFromEnvRequiresCredentials(t *testing.T) {
	_, err := FromEnv()
	if err == nil {
		t.Fatal("expected missing environment error")
	}
}

func TestFromEnvRejectsInvalidDuration(t *testing.T) {
	t.Setenv("JIRA_BASE_URL", "https://example.atlassian.net")
	t.Setenv("JIRA_EMAIL", "person@example.com")
	t.Setenv("JIRA_API_TOKEN", "secret")
	t.Setenv("JIRA_REFRESH_INTERVAL", "eventually")

	_, err := FromEnv()
	if err == nil {
		t.Fatal("expected invalid duration error")
	}
}

func TestFromEnvRejectsInvalidWorkerCount(t *testing.T) {
	t.Setenv("JIRA_BASE_URL", "https://example.atlassian.net")
	t.Setenv("JIRA_EMAIL", "person@example.com")
	t.Setenv("JIRA_API_TOKEN", "secret")
	t.Setenv("JIRA_WORKERS", "0")

	_, err := FromEnv()
	if err == nil {
		t.Fatal("expected invalid worker count error")
	}
}
