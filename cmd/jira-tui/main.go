package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/jon/jira-tui/internal/config"
	"github.com/jon/jira-tui/internal/jira"
	jiratui "github.com/jon/jira-tui/internal/tui"
)

func main() {
	cfg, err := config.FromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	client := jira.NewClient(cfg)
	model := jiratui.NewModel(
		client,
		cfg.DefaultJQL,
		jiratui.WithRefreshInterval(cfg.RefreshInterval),
		jiratui.WithRequestTimeout(cfg.RequestTimeout),
		jiratui.WithWorkerCount(cfg.WorkerCount),
		jiratui.WithQueueSize(cfg.QueueSize),
	)

	if _, err := tea.NewProgram(model).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "runtime error: %v\n", err)
		os.Exit(1)
	}
}
