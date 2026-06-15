package main

import (
	"context"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/jon/jira-tui/internal/claude"
	"github.com/jon/jira-tui/internal/config"
	"github.com/jon/jira-tui/internal/configui"
	"github.com/jon/jira-tui/internal/jira"
	jiratui "github.com/jon/jira-tui/internal/tui"
	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "jira",
		Short: "Browse Jira from the terminal",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApp()
		},
	}

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Edit Jira TUI configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := runConfig()
			return err
		},
	}
	root.AddCommand(configCmd)

	return root
}

func runApp() error {
	cfg, err := config.Load(config.LoadOptions{})
	if err != nil {
		if !config.IsValidationError(err) {
			return fmt.Errorf("config error: %w", err)
		}
		saved, configErr := runConfig()
		if configErr != nil {
			return configErr
		}
		if !saved {
			return fmt.Errorf("config is required before starting Jira TUI")
		}
		cfg, err = config.Load(config.LoadOptions{})
		if err != nil {
			return fmt.Errorf("config error: %w", err)
		}
	}

	claudeStatus := claude.LocalRunner{}.Check(context.Background(), claude.Config{
		Enabled: cfg.Claude.Enabled,
		Command: cfg.Claude.Command,
		Timeout: cfg.Claude.Timeout,
	})
	claudeCommand := claudeStatus.Command
	if claudeCommand == "" {
		claudeCommand = cfg.Claude.Command
	}
	client := jira.NewClient(cfg)
	model := jiratui.NewModel(
		client,
		cfg.DefaultJQL,
		jiratui.WithViews(cfg.Views, cfg.ActiveView),
		jiratui.WithRefreshInterval(cfg.RefreshInterval),
		jiratui.WithRequestTimeout(cfg.RequestTimeout),
		jiratui.WithWorkerCount(cfg.WorkerCount),
		jiratui.WithQueueSize(cfg.QueueSize),
		jiratui.WithTheme(cfg.Theme),
		jiratui.WithDisplay(cfg.Display),
		jiratui.WithClaudeConfig(jiratui.ClaudeConfig{
			Enabled:             cfg.Claude.Enabled,
			TicketPlan:          cfg.Claude.Features.TicketPlan,
			TicketAssist:        cfg.Claude.Features.TicketAssist,
			Command:             claudeCommand,
			Timeout:             cfg.Claude.Timeout,
			RequireConfirmation: cfg.Claude.Gates.RequireConfirmation,
			AllowJiraWrites:     cfg.Claude.Gates.AllowJiraWrites,
		}),
		jiratui.WithClaudeStatus(jiratui.ClaudeStatus{
			Enabled:   claudeStatus.Enabled,
			Available: claudeStatus.Available,
			Command:   claudeStatus.Command,
			Version:   claudeStatus.Version,
			Message:   claudeStatus.Message,
			Err:       claudeStatus.Err,
		}),
	)

	if _, err := tea.NewProgram(model).Run(); err != nil {
		return fmt.Errorf("runtime error: %w", err)
	}
	return nil
}

func runConfig() (bool, error) {
	cfg, path, problems, err := config.LoadEditable(config.LoadOptions{})
	if err != nil {
		return false, fmt.Errorf("config error: %w", err)
	}
	model := configui.NewModel(path, cfg, problems)
	finalModel, err := tea.NewProgram(model).Run()
	if err != nil {
		return false, fmt.Errorf("config runtime error: %w", err)
	}
	configModel, ok := finalModel.(configui.Model)
	if !ok {
		return false, fmt.Errorf("config runtime returned unexpected model")
	}
	return configModel.Saved(), nil
}
