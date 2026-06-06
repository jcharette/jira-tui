package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultJQL = "assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC"
const defaultRefreshInterval = 2 * time.Minute
const defaultRequestTimeout = 20 * time.Second
const defaultWorkerCount = 2
const defaultQueueSize = 16

type Config struct {
	BaseURL         string
	Email           string
	APIToken        string
	DefaultJQL      string
	RefreshInterval time.Duration
	RequestTimeout  time.Duration
	WorkerCount     int
	QueueSize       int
}

func FromEnv() (Config, error) {
	cfg := Config{
		BaseURL:    strings.TrimRight(strings.TrimSpace(os.Getenv("JIRA_BASE_URL")), "/"),
		Email:      strings.TrimSpace(os.Getenv("JIRA_EMAIL")),
		APIToken:   strings.TrimSpace(os.Getenv("JIRA_API_TOKEN")),
		DefaultJQL: strings.TrimSpace(os.Getenv("JIRA_JQL")),
	}
	if cfg.DefaultJQL == "" {
		cfg.DefaultJQL = defaultJQL
	}
	refreshInterval, err := durationFromEnv("JIRA_REFRESH_INTERVAL", defaultRefreshInterval)
	if err != nil {
		return Config{}, err
	}
	cfg.RefreshInterval = refreshInterval

	requestTimeout, err := durationFromEnv("JIRA_REQUEST_TIMEOUT", defaultRequestTimeout)
	if err != nil {
		return Config{}, err
	}
	cfg.RequestTimeout = requestTimeout

	workerCount, err := intFromEnv("JIRA_WORKERS", defaultWorkerCount)
	if err != nil {
		return Config{}, err
	}
	cfg.WorkerCount = workerCount

	queueSize, err := intFromEnv("JIRA_QUEUE_SIZE", defaultQueueSize)
	if err != nil {
		return Config{}, err
	}
	cfg.QueueSize = queueSize

	var missing []string
	if cfg.BaseURL == "" {
		missing = append(missing, "JIRA_BASE_URL")
	}
	if cfg.Email == "" {
		missing = append(missing, "JIRA_EMAIL")
	}
	if cfg.APIToken == "" {
		missing = append(missing, "JIRA_API_TOKEN")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	if !strings.HasPrefix(cfg.BaseURL, "https://") && !strings.HasPrefix(cfg.BaseURL, "http://") {
		return Config{}, errors.New("JIRA_BASE_URL must start with https:// or http://")
	}

	return cfg, nil
}

func durationFromEnv(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid Go duration, for example 30s or 2m: %w", name, err)
	}
	if duration < 0 {
		return 0, fmt.Errorf("%s cannot be negative", name)
	}
	return duration, nil
}

func intFromEnv(name string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", name, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", name)
	}
	return parsed, nil
}
