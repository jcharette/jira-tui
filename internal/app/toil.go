package app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/jcharette/jira-tui/internal/config"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/jcharette/jira-tui/internal/sprinttrack"
	toilfields "github.com/jcharette/jira-tui/internal/toil"
	"github.com/spf13/cobra"
)

const toilIssueSearchLimit = 25

type toilJiraClient interface {
	SearchIssues(ctx context.Context, jql string, maxResults int) ([]jira.Issue, error)
	GetCreateIssueTypes(ctx context.Context, projectKey string) ([]jira.CreateIssueType, error)
	CreateIssue(ctx context.Context, request jira.CreateIssueRequest) (jira.Issue, error)
	AddWorklog(ctx context.Context, key string, request jira.AddWorklogRequest) (jira.Worklog, error)
	GetTransitions(ctx context.Context, key string) ([]jira.Transition, error)
	TransitionIssue(ctx context.Context, key string, request jira.TransitionIssueRequest) error
	GetBoardSprints(ctx context.Context, boardID int, states []string, startAt, maxResults int) (jira.SprintPage, error)
	MoveIssuesToSprint(ctx context.Context, sprintID int, issueKeys []string) error
	SearchBoardIssues(ctx context.Context, boardID int, jql string, maxResults int) ([]jira.Issue, error)
}

type createToilOptions struct {
	Summary   string
	Time      string
	Note      string
	Project   string
	IssueType string
	Close     bool
}

type toilWorklogOptions struct {
	Time string
	Note string
}

func newTicketCommand(profile *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ticket",
		Short: "Work with Jira tickets",
	}
	cmd.AddCommand(newTicketToilCommand(profile))
	cmd.AddCommand(newCreateToilCommand(profile))
	cmd.AddCommand(newUpdateToilCommand(profile))
	cmd.AddCommand(newCloseToilCommand(profile))
	cmd.AddCommand(newCheckBoardCommand(profile))
	return cmd
}

func newTicketToilCommand(profile *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "toil",
		Short: "Pick from open assigned toil tickets",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, ctx, cancel, err := toilDeps(*profile)
			if err != nil {
				return err
			}
			defer cancel()
			key, err := pickToilIssue(ctx, cfg, client, cmd.InOrStdin(), cmd.OutOrStdout())
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Selected %s.\n", key)
			return nil
		},
	}
	return cmd
}

func newCreateToilCommand(profile *string) *cobra.Command {
	var opts createToilOptions
	cmd := &cobra.Command{
		Use:   "create-toil",
		Short: "Create a Jira toil ticket",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, ctx, cancel, err := toilDeps(*profile)
			if err != nil {
				return err
			}
			defer cancel()
			return runCreateToilWithDeps(ctx, cfg, client, opts, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&opts.Summary, "summary", "", "ticket summary")
	cmd.Flags().StringVar(&opts.Time, "time", "", "Jira worklog duration, such as 30m or 1h 15m")
	cmd.Flags().StringVar(&opts.Note, "note", "", "worklog note")
	cmd.Flags().StringVar(&opts.Project, "project", "", "Jira project key")
	cmd.Flags().StringVar(&opts.IssueType, "type", "", "Jira issue type name or ID")
	cmd.Flags().BoolVar(&opts.Close, "close", false, "close the toil ticket after creating and logging work")
	return cmd
}

func newUpdateToilCommand(profile *string) *cobra.Command {
	var opts toilWorklogOptions
	cmd := &cobra.Command{
		Use:   "update-toil [ticket]",
		Short: "Log time to a toil ticket",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, ctx, cancel, err := toilDeps(*profile)
			if err != nil {
				return err
			}
			defer cancel()
			return runUpdateToilWithDeps(ctx, cfg, client, args, cmd.InOrStdin(), cmd.OutOrStdout(), opts)
		},
	}
	cmd.Flags().StringVar(&opts.Time, "time", "", "Jira worklog duration, such as 30m or 1h 15m")
	cmd.Flags().StringVar(&opts.Note, "note", "", "worklog note")
	return cmd
}

func newCloseToilCommand(profile *string) *cobra.Command {
	var opts toilWorklogOptions
	cmd := &cobra.Command{
		Use:   "close-toil [ticket]",
		Short: "Log time and close a toil ticket",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, ctx, cancel, err := toilDeps(*profile)
			if err != nil {
				return err
			}
			defer cancel()
			return runCloseToilWithDeps(ctx, cfg, client, args, cmd.InOrStdin(), cmd.OutOrStdout(), opts)
		},
	}
	cmd.Flags().StringVar(&opts.Time, "time", "", "Jira worklog duration, such as 30m or 1h 15m")
	cmd.Flags().StringVar(&opts.Note, "note", "", "worklog note")
	return cmd
}

func toilDeps(profile string) (config.Config, *jira.Client, context.Context, context.CancelFunc, error) {
	cfg, err := loadConfigOrConfigure(profile)
	if err != nil {
		return config.Config{}, nil, nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), maxDuration(cfg.RequestTimeout, 30*time.Second))
	return cfg, jira.NewClient(cfg), ctx, cancel, nil
}

func runCreateToilWithDeps(ctx context.Context, cfg config.Config, client toilJiraClient, opts createToilOptions, out io.Writer) error {
	summary := strings.TrimSpace(opts.Summary)
	if summary == "" {
		return fmt.Errorf("--summary is required")
	}
	if strings.TrimSpace(opts.Time) != "" && !validToilDuration(opts.Time) {
		return fmt.Errorf("invalid --time %q; use Jira duration like 30m, 1h, or 1h 30m", opts.Time)
	}
	projectKey := resolveToilProject(cfg, opts.Project)
	if projectKey == "" {
		return fmt.Errorf("toil ticket needs --project or default_project in config")
	}
	issueTypes, err := client.GetCreateIssueTypes(ctx, projectKey)
	if err != nil {
		return fmt.Errorf("load issue types: %w", err)
	}
	issueType, ok := chooseToilIssueType(issueTypes, opts.IssueType)
	if !ok {
		return fmt.Errorf("no usable issue type found for project %s", projectKey)
	}
	issue, err := client.CreateIssue(ctx, jira.CreateIssueRequest{
		ProjectKey:  projectKey,
		IssueTypeID: issueType.ID,
		Summary:     summary,
		Description: strings.TrimSpace(opts.Note),
		Fields:      toilfields.CreateFields(cfg),
	})
	if err != nil {
		return fmt.Errorf("create toil ticket: %w", err)
	}
	_, _ = fmt.Fprintf(out, "Created %s.\n", issue.Key)
	if cfg.DefaultBoardID > 0 {
		trackResult, err := sprinttrack.AddToActiveSprint(ctx, client, cfg.DefaultBoardID, []string{issue.Key})
		if err != nil {
			return err
		}
		if len(trackResult.Missing) > 0 {
			return fmt.Errorf("%s not visible on board %d after sprint move", strings.Join(trackResult.Missing, ", "), trackResult.Sprint.BoardID)
		}
		if trackResult.Applied {
			_, _ = fmt.Fprintf(out, "Added %s to sprint %s.\n", issue.Key, displaySprintName(trackResult.Sprint))
		}
	}
	if strings.TrimSpace(opts.Time) != "" {
		if err := logToilWork(ctx, client, issue.Key, toilWorklogOptions{Time: opts.Time, Note: opts.Note}); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(out, "Logged %s to %s.\n", strings.TrimSpace(opts.Time), issue.Key)
	}
	if opts.Close {
		return closeToilIssue(ctx, client, issue.Key, out)
	}
	return nil
}

func runUpdateToilWithDeps(ctx context.Context, cfg config.Config, client toilJiraClient, args []string, in io.Reader, out io.Writer, opts toilWorklogOptions) error {
	key, err := resolveToilIssueKey(ctx, cfg, client, args, in, out)
	if err != nil {
		return err
	}
	if err := logToilWork(ctx, client, key, opts); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "Logged %s to %s.\n", strings.TrimSpace(opts.Time), key)
	return nil
}

func runCloseToilWithDeps(ctx context.Context, cfg config.Config, client toilJiraClient, args []string, in io.Reader, out io.Writer, opts toilWorklogOptions) error {
	key, err := resolveToilIssueKey(ctx, cfg, client, args, in, out)
	if err != nil {
		return err
	}
	if strings.TrimSpace(opts.Time) != "" {
		if err := logToilWork(ctx, client, key, opts); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(out, "Logged %s to %s.\n", strings.TrimSpace(opts.Time), key)
	}
	return closeToilIssue(ctx, client, key, out)
}

func resolveToilIssueKey(ctx context.Context, cfg config.Config, client toilJiraClient, args []string, in io.Reader, out io.Writer) (string, error) {
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		return strings.ToUpper(strings.TrimSpace(args[0])), nil
	}
	return pickToilIssue(ctx, cfg, client, in, out)
}

func displaySprintName(sprint jira.Sprint) string {
	if strings.TrimSpace(sprint.Name) != "" {
		return sprint.Name
	}
	return fmt.Sprintf("Sprint %d", sprint.ID)
}

func pickToilIssue(ctx context.Context, cfg config.Config, client toilJiraClient, in io.Reader, out io.Writer) (string, error) {
	issues, err := client.SearchIssues(ctx, toilSearchJQL(cfg), toilIssueSearchLimit)
	if err != nil {
		return "", fmt.Errorf("load toil tickets: %w", err)
	}
	if len(issues) == 0 {
		return "", fmt.Errorf("no open assigned toil tickets found")
	}
	for index, issue := range issues {
		_, _ = fmt.Fprintf(out, "%d. %s  %s\n", index+1, issue.Key, issue.Summary)
	}
	_, _ = fmt.Fprint(out, "Select toil ticket: ")
	reader := bufio.NewReader(in)
	answer, _ := reader.ReadString('\n')
	choice, err := strconv.Atoi(strings.TrimSpace(answer))
	if err != nil || choice < 1 || choice > len(issues) {
		return "", fmt.Errorf("invalid toil ticket selection")
	}
	return issues[choice-1].Key, nil
}

func logToilWork(ctx context.Context, client toilJiraClient, key string, opts toilWorklogOptions) error {
	timeSpent := strings.TrimSpace(opts.Time)
	if timeSpent == "" {
		return fmt.Errorf("--time is required")
	}
	if !validToilDuration(timeSpent) {
		return fmt.Errorf("invalid --time %q; use Jira duration like 30m, 1h, or 1h 30m", opts.Time)
	}
	_, err := client.AddWorklog(ctx, key, jira.AddWorklogRequest{
		TimeSpent: timeSpent,
		Started:   time.Now(),
		Comment:   strings.TrimSpace(opts.Note),
	})
	if err != nil {
		return fmt.Errorf("log work to %s: %w", key, err)
	}
	return nil
}

func closeToilIssue(ctx context.Context, client toilJiraClient, key string, out io.Writer) error {
	transitions, err := client.GetTransitions(ctx, key)
	if err != nil {
		return fmt.Errorf("load transitions for %s: %w", key, err)
	}
	transition, ok := chooseFinishTransition(transitions)
	if !ok {
		_, _ = fmt.Fprintf(out, "Skipped close for %s: no safe terminal transition available.\n", key)
		return nil
	}
	if err := client.TransitionIssue(ctx, key, jira.TransitionIssueRequest{TransitionID: transition.ID}); err != nil {
		return fmt.Errorf("close %s: %w", key, err)
	}
	_, _ = fmt.Fprintf(out, "Closed %s as %s.\n", key, displayValue(transition.ToStatus, transition.Name))
	return nil
}

func resolveToilProject(cfg config.Config, explicit string) string {
	if project := strings.ToUpper(strings.TrimSpace(explicit)); project != "" {
		return project
	}
	if project := strings.ToUpper(strings.TrimSpace(cfg.DefaultProject)); project != "" {
		return project
	}
	return projectFromJQL(cfg.DefaultJQL)
}

func chooseToilIssueType(issueTypes []jira.CreateIssueType, preferred string) (jira.CreateIssueType, bool) {
	preferred = strings.TrimSpace(preferred)
	if preferred != "" {
		for _, issueType := range issueTypes {
			if strings.EqualFold(issueType.ID, preferred) || strings.EqualFold(issueType.Name, preferred) {
				return issueType, true
			}
		}
		return jira.CreateIssueType{}, false
	}
	for _, issueType := range issueTypes {
		if strings.EqualFold(issueType.Name, "Toil") {
			return issueType, true
		}
	}
	for _, issueType := range issueTypes {
		if !issueType.Subtask {
			return issueType, true
		}
	}
	return jira.CreateIssueType{}, false
}

func toilSearchJQL(cfg config.Config) string {
	project := resolveToilProject(cfg, "")
	prefix := ""
	if project != "" {
		prefix = "project = " + project + " AND "
	}
	return prefix + "assignee = currentUser() AND resolution = Unresolved AND (labels = toil OR issuetype = Toil) ORDER BY updated DESC"
}

func projectFromJQL(jql string) string {
	normalized := strings.ReplaceAll(jql, "\"", " ")
	normalized = strings.ReplaceAll(normalized, "=", " = ")
	fields := strings.Fields(normalized)
	for index := 0; index+2 < len(fields); index++ {
		if strings.EqualFold(fields[index], "project") && fields[index+1] == "=" {
			return strings.ToUpper(strings.Trim(fields[index+2], "()"))
		}
	}
	return ""
}

func validToilDuration(value string) bool {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) == 0 {
		return false
	}
	for _, field := range fields {
		if len(field) < 2 {
			return false
		}
		unit := field[len(field)-1]
		if !strings.ContainsRune("wdhm", rune(unit)) {
			return false
		}
		number := field[:len(field)-1]
		if number == "" {
			return false
		}
		for _, char := range number {
			if (char < '0' || char > '9') && char != '.' {
				return false
			}
		}
	}
	return true
}
