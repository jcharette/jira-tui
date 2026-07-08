package app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jcharette/jira-tui/internal/boardcheck"
	"github.com/jcharette/jira-tui/internal/config"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/spf13/cobra"
)

const boardCheckLimit = 50

type boardCheckClient interface {
	SearchIssues(ctx context.Context, jql string, maxResults int) ([]jira.Issue, error)
	CurrentUser(ctx context.Context) (jira.User, error)
	UpdateAssignee(ctx context.Context, key string, assignee jira.User) error
	GetBoardSprints(ctx context.Context, boardID int, states []string, startAt, maxResults int) (jira.SprintPage, error)
	MoveIssuesToSprint(ctx context.Context, sprintID int, issueKeys []string) error
	GetCreateIssueTypes(ctx context.Context, projectKey string) ([]jira.CreateIssueType, error)
	UpdateIssueType(ctx context.Context, key string, issueTypeID string) error
}

type boardCheckOptions struct {
	Fix bool
	Yes bool
}

type boardCheckFinding struct {
	Issue   jira.Issue
	Parent  *jira.Issue
	Finding boardcheck.Finding
}

func newCheckBoardCommand(profile *string) *cobra.Command {
	var opts boardCheckOptions
	cmd := &cobra.Command{
		Use:   "check-board [ticket]",
		Short: "Audit tickets for sprint-board visibility problems",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, client, ctx, cancel, err := toilDeps(*profile)
			if err != nil {
				return err
			}
			defer cancel()
			return runCheckBoardWithDeps(ctx, cfg, client, args, cmd.InOrStdin(), cmd.OutOrStdout(), opts)
		},
	}
	cmd.Flags().BoolVar(&opts.Fix, "fix", false, "prompt to apply safe board hygiene fixes")
	cmd.Flags().BoolVar(&opts.Yes, "yes", false, "apply fixes without prompting")
	return cmd
}

func runCheckBoardWithDeps(ctx context.Context, cfg config.Config, client boardCheckClient, args []string, in io.Reader, out io.Writer, opts boardCheckOptions) error {
	issues, err := boardCheckIssues(ctx, client, args)
	if err != nil {
		return err
	}
	activeSprint, sprintKnown, err := activeBoardSprint(ctx, cfg, client)
	if err != nil {
		return err
	}
	findings, err := collectBoardFindings(ctx, client, issues, sprintKnown)
	if err != nil {
		return err
	}
	if len(findings) == 0 {
		_, _ = fmt.Fprintln(out, "OK: no board hygiene findings.")
		return nil
	}
	printBoardFindings(out, findings)
	if !opts.Fix {
		return nil
	}
	if !sprintKnown {
		_, _ = fmt.Fprintln(out, "WARN: active sprint fixes skipped because no active sprint is available.")
	}
	printBoardFixPlan(out, findings, activeSprint, sprintKnown)
	if opts.Fix && !opts.Yes && !isInteractiveReader(in) {
		return fmt.Errorf("--fix requires confirmation; pass --yes for non-interactive use")
	}
	if !opts.Yes && !confirmBoardFix(in, out) {
		_, _ = fmt.Fprintln(out, "No fixes applied.")
		return nil
	}
	return applyBoardFixes(ctx, cfg, client, out, findings, activeSprint, sprintKnown)
}

func boardCheckIssues(ctx context.Context, client boardCheckClient, args []string) ([]jira.Issue, error) {
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		key := strings.ToUpper(strings.TrimSpace(args[0]))
		return client.SearchIssues(ctx, "key = "+key, 1)
	}
	return client.SearchIssues(ctx, "assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC", boardCheckLimit)
}

func collectBoardFindings(ctx context.Context, client boardCheckClient, issues []jira.Issue, sprintKnown bool) ([]boardCheckFinding, error) {
	byKey := map[string]jira.Issue{}
	for _, issue := range issues {
		byKey[issue.Key] = issue
	}
	for _, issue := range append([]jira.Issue(nil), issues...) {
		children, err := client.SearchIssues(ctx, "parent = "+issue.Key+" ORDER BY key ASC", boardCheckLimit)
		if err != nil {
			return nil, fmt.Errorf("load child issues for %s: %w", issue.Key, err)
		}
		for _, child := range children {
			if _, ok := byKey[child.Key]; !ok {
				issues = append(issues, child)
				byKey[child.Key] = child
			}
		}
	}
	var findings []boardCheckFinding
	for _, issue := range issues {
		parent := parentIssue(ctx, client, byKey, issue.ParentKey)
		inSprint := false
		if sprintKnown {
			matches, err := client.SearchIssues(ctx, "key = "+issue.Key+" AND sprint in openSprints()", 1)
			if err != nil {
				return nil, fmt.Errorf("check active sprint for %s: %w", issue.Key, err)
			}
			inSprint = len(matches) > 0
		}
		for _, finding := range boardcheck.CheckIssue(issue, boardcheck.Options{Parent: parent, RequireActiveSprint: true, ActiveSprintKnown: sprintKnown, InActiveSprint: inSprint}) {
			findings = append(findings, boardCheckFinding{Issue: issue, Parent: parent, Finding: finding})
		}
	}
	return findings, nil
}

func parentIssue(ctx context.Context, client boardCheckClient, byKey map[string]jira.Issue, key string) *jira.Issue {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	if issue, ok := byKey[key]; ok {
		return &issue
	}
	parents, err := client.SearchIssues(ctx, "key = "+key, 1)
	if err != nil || len(parents) == 0 {
		return nil
	}
	issue := parents[0]
	byKey[issue.Key] = issue
	return &issue
}

func activeBoardSprint(ctx context.Context, cfg config.Config, client boardCheckClient) (jira.Sprint, bool, error) {
	if cfg.DefaultBoardID <= 0 {
		return jira.Sprint{}, false, nil
	}
	page, err := client.GetBoardSprints(ctx, cfg.DefaultBoardID, []string{"active"}, 0, 10)
	if err != nil {
		return jira.Sprint{}, false, fmt.Errorf("load active sprint for board %d: %w", cfg.DefaultBoardID, err)
	}
	for _, sprint := range page.Sprints {
		if strings.EqualFold(sprint.State, "active") {
			return sprint, true, nil
		}
	}
	return jira.Sprint{}, false, nil
}

func printBoardFindings(out io.Writer, findings []boardCheckFinding) {
	for _, item := range findings {
		_, _ = fmt.Fprintf(out, "%s %s: %s\n", item.Finding.Severity, item.Issue.Key, item.Finding.Message)
	}
}

func printBoardFixPlan(out io.Writer, findings []boardCheckFinding, sprint jira.Sprint, sprintKnown bool) {
	_, _ = fmt.Fprintln(out, "\nProposed fixes:")
	for _, item := range findings {
		switch item.Finding.Code {
		case boardcheck.CodeUnassigned:
			_, _ = fmt.Fprintf(out, "- Assign %s to current user.\n", item.Issue.Key)
		case boardcheck.CodeMissingActiveSprint:
			if sprintKnown {
				_, _ = fmt.Fprintf(out, "- Add %s to sprint %s.\n", item.Issue.Key, sprint.Name)
			}
		case boardcheck.CodeSubtaskUnderEpic:
			_, _ = fmt.Fprintf(out, "- Try converting %s to Story/Task under Epic %s; if Jira rejects it, fix in Jira Move UI.\n", item.Issue.Key, displayParentKey(item.Parent))
		}
	}
}

func confirmBoardFix(in io.Reader, out io.Writer) bool {
	_, _ = fmt.Fprint(out, "Apply these fixes? [y/N] ")
	answer, _ := bufio.NewReader(in).ReadString('\n')
	return strings.EqualFold(strings.TrimSpace(answer), "y") || strings.EqualFold(strings.TrimSpace(answer), "yes")
}

func isInteractiveReader(in io.Reader) bool {
	file, ok := in.(*os.File)
	if !ok {
		return true
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func applyBoardFixes(ctx context.Context, cfg config.Config, client boardCheckClient, out io.Writer, findings []boardCheckFinding, sprint jira.Sprint, sprintKnown bool) error {
	var currentUser jira.User
	for _, item := range findings {
		switch item.Finding.Code {
		case boardcheck.CodeUnassigned:
			if currentUser.AccountID == "" {
				user, err := client.CurrentUser(ctx)
				if err != nil {
					return fmt.Errorf("load current user: %w", err)
				}
				currentUser = user
			}
			if err := client.UpdateAssignee(ctx, item.Issue.Key, currentUser); err != nil {
				return fmt.Errorf("assign %s: %w", item.Issue.Key, err)
			}
			_, _ = fmt.Fprintf(out, "Assigned %s to %s.\n", item.Issue.Key, displayValue(currentUser.DisplayName, currentUser.AccountID))
		case boardcheck.CodeMissingActiveSprint:
			if sprintKnown {
				if err := client.MoveIssuesToSprint(ctx, sprint.ID, []string{item.Issue.Key}); err != nil {
					return fmt.Errorf("add %s to sprint %s: %w", item.Issue.Key, sprint.Name, err)
				}
				_, _ = fmt.Fprintf(out, "Added %s to sprint %s.\n", item.Issue.Key, sprint.Name)
			}
		case boardcheck.CodeSubtaskUnderEpic:
			if err := convertSubtaskToStandardIssue(ctx, cfg, client, item.Issue); err != nil {
				_, _ = fmt.Fprintf(out, "Manual fix needed for %s: %v\n", item.Issue.Key, err)
				continue
			}
			_, _ = fmt.Fprintf(out, "Converted %s to Story/Task.\n", item.Issue.Key)
		}
	}
	return nil
}

func convertSubtaskToStandardIssue(ctx context.Context, cfg config.Config, client boardCheckClient, issue jira.Issue) error {
	project := projectKeyFromIssueKey(issue.Key)
	if project == "" {
		project = strings.TrimSpace(cfg.DefaultProject)
	}
	if project == "" {
		return fmt.Errorf("missing project key")
	}
	issueTypes, err := client.GetCreateIssueTypes(ctx, project)
	if err != nil {
		return err
	}
	for _, name := range []string{"Story", "Task"} {
		for _, issueType := range issueTypes {
			if !issueType.Subtask && strings.EqualFold(issueType.Name, name) {
				return client.UpdateIssueType(ctx, issue.Key, issueType.ID)
			}
		}
	}
	return fmt.Errorf("no Story/Task issue type found")
}

func displayParentKey(parent *jira.Issue) string {
	if parent == nil || strings.TrimSpace(parent.Key) == "" {
		return "parent Epic"
	}
	return parent.Key
}

func projectKeyFromIssueKey(key string) string {
	key = strings.ToUpper(strings.TrimSpace(key))
	if index := strings.Index(key, "-"); index > 0 {
		return key[:index]
	}
	return ""
}
