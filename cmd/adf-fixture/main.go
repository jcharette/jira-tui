package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	model "github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"
	adffixture "github.com/jcharette/jira-tui/internal/adf/fixture"
	"github.com/jcharette/jira-tui/internal/config"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	var inputPath string
	var outputPath string
	var issueKey string
	var commentID string
	var configPath string

	cmd := &cobra.Command{
		Use:   "adf-fixture",
		Short: "Capture or sanitize Jira ADF test fixtures",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			node, err := loadADFNode(ctx, inputPath, configPath, issueKey, commentID)
			if err != nil {
				return err
			}
			if strings.TrimSpace(outputPath) == "" {
				return fmt.Errorf("--output is required")
			}
			return adffixture.WriteSanitized(outputPath, node)
		},
	}
	cmd.Flags().StringVar(&inputPath, "input", "", "raw ADF JSON file to sanitize")
	cmd.Flags().StringVar(&outputPath, "output", "", "destination sanitized *.adf.json file")
	cmd.Flags().StringVar(&issueKey, "issue", "", "Jira issue key for live capture")
	cmd.Flags().StringVar(&commentID, "comment", "", "Jira comment ID to capture; omit to capture issue description")
	cmd.Flags().StringVar(&configPath, "config", "", "Jira config path; defaults to the normal app config")
	return cmd
}

func loadADFNode(ctx context.Context, inputPath string, configPath string, issueKey string, commentID string) (*model.CommentNodeScheme, error) {
	if strings.TrimSpace(inputPath) != "" {
		if strings.TrimSpace(issueKey) != "" || strings.TrimSpace(commentID) != "" {
			return nil, fmt.Errorf("--input cannot be combined with --issue or --comment")
		}
		return readRawADF(inputPath)
	}
	if strings.TrimSpace(issueKey) == "" {
		return nil, fmt.Errorf("provide --input or --issue")
	}
	cfg, err := config.Load(config.LoadOptions{Path: configPath})
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	client := jira.NewClient(cfg)
	if strings.TrimSpace(commentID) != "" {
		return client.GetCommentADF(ctx, issueKey, commentID)
	}
	return client.GetIssueDescriptionADF(ctx, issueKey)
}

func readRawADF(path string) (*model.CommentNodeScheme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read raw ADF: %w", err)
	}
	var node model.CommentNodeScheme
	if err := json.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("parse raw ADF: %w", err)
	}
	return &node, nil
}
