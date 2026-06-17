package app

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jcharette/jira-tui/internal/cache"
	"github.com/jcharette/jira-tui/internal/claude"
	"github.com/jcharette/jira-tui/internal/config"
	"github.com/jcharette/jira-tui/internal/configui"
	"github.com/jcharette/jira-tui/internal/events"
	"github.com/jcharette/jira-tui/internal/jira"
	jiratui "github.com/jcharette/jira-tui/internal/tui"
	"github.com/spf13/cobra"
)

func Execute() error {
	return NewRootCommand().Execute()
}

func NewRootCommand() *cobra.Command {
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
	cfgPath, err := config.PathOrDefault("")
	if err != nil {
		return fmt.Errorf("config path error: %w", err)
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
	eventStream := events.NewStream()
	defer eventStream.Close()
	client := jira.NewClient(cfg)
	options := []jiratui.Option{
		jiratui.WithViews(cfg.Views, cfg.ActiveView),
		jiratui.WithRefreshInterval(cfg.RefreshInterval),
		jiratui.WithRequestTimeout(cfg.RequestTimeout),
		jiratui.WithWorkerCount(cfg.WorkerCount),
		jiratui.WithQueueSize(cfg.QueueSize),
		jiratui.WithTheme(cfg.Theme),
		jiratui.WithDisplay(cfg.Display),
		jiratui.WithEventStream(eventStream),
		jiratui.WithSavedViewWriter(savedViewWriter(cfgPath, &cfg)),
		jiratui.WithClaudeConfig(jiratui.ClaudeConfig{
			Enabled:             cfg.Claude.Enabled,
			TicketPlan:          cfg.Claude.Features.TicketPlan,
			TicketAssist:        cfg.Claude.Features.TicketAssist,
			DraftTicket:         cfg.Claude.Features.DraftTicket,
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
	}
	if cacheStore, cacheErr := cache.OpenDefault(); cacheErr == nil {
		defer cacheStore.Close()
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_, _ = cacheStore.DeleteRowsUpdatedBefore(ctx, time.Now().Add(-cache.DefaultCleanupMaxAge))
		}()
		options = append(options, jiratui.WithActiveViewStore(cacheStore, cfg.BaseURL))
	}
	model := jiratui.NewModel(client, cfg.DefaultJQL, options...)

	if _, err := tea.NewProgram(model).Run(); err != nil {
		return fmt.Errorf("runtime error: %w", err)
	}
	return nil
}

func savedViewWriter(path string, cfg *config.Config) jiratui.SavedViewWriter {
	return func(view config.IssueView) error {
		next, err := config.AddSavedView(*cfg, view)
		if err != nil {
			return err
		}
		if err := config.Save(path, next); err != nil {
			return err
		}
		*cfg = next
		return nil
	}
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
