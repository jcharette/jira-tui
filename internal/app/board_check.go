package app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

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
	GetBoards(ctx context.Context, projectKey string, startAt, maxResults int) (jira.BoardPage, error)
	SearchBoardIssues(ctx context.Context, boardID int, jql string, maxResults int) ([]jira.Issue, error)
	GetIssue(ctx context.Context, key string) (jira.IssueDetail, error)
	GetBoardSprints(ctx context.Context, boardID int, states []string, startAt, maxResults int) (jira.SprintPage, error)
	MoveIssuesToSprint(ctx context.Context, sprintID int, issueKeys []string) error
	GetCreateIssueTypes(ctx context.Context, projectKey string) ([]jira.CreateIssueType, error)
	UpdateIssueType(ctx context.Context, key string, issueTypeID string) error
	UpdateEditField(ctx context.Context, key string, value jira.EditFieldValue) error
}

type boardCheckOptions struct {
	Yes     bool
	BoardID int
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
	cmd.Flags().BoolVar(&opts.Yes, "yes", false, "apply fixes without prompting")
	cmd.Flags().IntVar(&opts.BoardID, "board", 0, "Jira Agile board ID used to find the active sprint")
	return cmd
}

func runCheckBoardWithDeps(_ context.Context, cfg config.Config, client boardCheckClient, args []string, in io.Reader, out io.Writer, opts boardCheckOptions) error {
	if opts.BoardID > 0 {
		cfg.DefaultBoardID = opts.BoardID
	}
	readCtx := context.Background()
	issues, err := boardCheckIssues(readCtx, client, args)
	if err != nil {
		return err
	}
	activeSprint, sprintKnown, err := activeBoardSprint(readCtx, cfg, client, issues)
	if err != nil {
		return err
	}
	findings, err := collectBoardFindings(readCtx, client, issues, activeSprint, sprintKnown)
	if err != nil {
		return err
	}
	if len(findings) == 0 {
		_, _ = fmt.Fprintln(out, "OK: no board hygiene findings.")
		return nil
	}
	printBoardFindings(out, findings)
	if !sprintKnown {
		_, _ = fmt.Fprintln(out, "WARN: active sprint fixes skipped because no active sprint is available.")
	}
	if !hasApplicableBoardFix(cfg, findings, sprintKnown) {
		printManualBoardReview(out, cfg, findings, activeSprint)
		printNoApplicableBoardFix(out, findings, sprintKnown)
		return nil
	}
	printBoardFixPlan(out, cfg, findings, activeSprint, sprintKnown)
	if !opts.Yes && !isInteractiveReader(in) {
		return fmt.Errorf("confirmation required; pass --yes for non-interactive use")
	}
	if !opts.Yes && !confirmBoardFix(in, out) {
		_, _ = fmt.Fprintln(out, "No fixes applied.")
		return nil
	}
	writeCtx, cancel := freshBoardWriteContext(cfg)
	defer cancel()
	return applyBoardFixes(writeCtx, cfg, client, out, findings, activeSprint, sprintKnown)
}

func boardCheckIssues(ctx context.Context, client boardCheckClient, args []string) ([]jira.Issue, error) {
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		key := strings.ToUpper(strings.TrimSpace(args[0]))
		return client.SearchIssues(ctx, "key = "+key, 1)
	}
	return client.SearchIssues(ctx, "assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC", boardCheckLimit)
}

func collectBoardFindings(ctx context.Context, client boardCheckClient, issues []jira.Issue, activeSprint jira.Sprint, sprintKnown bool) ([]boardCheckFinding, error) {
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
		matches, err := client.SearchIssues(ctx, sprintMembershipJQL(issue.Key, activeSprint, sprintKnown), 1)
		if err != nil {
			return nil, fmt.Errorf("check active sprint for %s: %w", issue.Key, err)
		}
		for _, finding := range boardcheck.CheckIssue(issue, boardcheck.Options{Parent: parent, RequireActiveSprint: true, ActiveSprintKnown: true, InActiveSprint: len(matches) > 0}) {
			findings = append(findings, boardCheckFinding{Issue: issue, Parent: parent, Finding: finding})
		}
		if sprintKnown && activeSprint.BoardID > 0 && len(matches) > 0 && boardcheck.ShouldBeInSprint(issue) {
			boardMatches, err := client.SearchBoardIssues(ctx, activeSprint.BoardID, "key = "+strings.ToUpper(strings.TrimSpace(issue.Key)), 1)
			if err != nil {
				return nil, fmt.Errorf("check board visibility for %s: %w", issue.Key, err)
			}
			if len(boardMatches) == 0 {
				findings = append(findings, boardCheckFinding{
					Issue:  issue,
					Parent: parent,
					Finding: boardcheck.Finding{
						Severity: boardcheck.SeverityWarn,
						Code:     boardcheck.CodeMissingBoard,
						IssueKey: issue.Key,
						Message:  fmt.Sprintf("%s is in sprint %s but not visible on board %d", issue.Key, activeSprint.Name, activeSprint.BoardID),
						Fix:      "check the board filter or status column mapping",
					},
				})
			}
		}
	}
	return findings, nil
}

func sprintMembershipJQL(key string, activeSprint jira.Sprint, sprintKnown bool) string {
	key = strings.ToUpper(strings.TrimSpace(key))
	if sprintKnown && activeSprint.ID > 0 {
		return fmt.Sprintf("key = %s AND sprint = %d", key, activeSprint.ID)
	}
	return "key = " + key + " AND sprint in openSprints()"
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

func activeBoardSprint(ctx context.Context, cfg config.Config, client boardCheckClient, issues []jira.Issue) (jira.Sprint, bool, error) {
	if cfg.DefaultBoardID > 0 {
		return activeSprintForBoard(ctx, client, cfg.DefaultBoardID)
	}
	return discoverActiveSprint(ctx, client, issues)
}

func activeSprintForBoard(ctx context.Context, client boardCheckClient, boardID int) (jira.Sprint, bool, error) {
	page, err := client.GetBoardSprints(ctx, boardID, []string{"active"}, 0, 10)
	if err != nil {
		return jira.Sprint{}, false, fmt.Errorf("load active sprint for board %d: %w", boardID, err)
	}
	for _, sprint := range page.Sprints {
		if strings.EqualFold(sprint.State, "active") {
			if sprint.BoardID == 0 {
				sprint.BoardID = boardID
			}
			return sprint, true, nil
		}
	}
	return jira.Sprint{}, false, nil
}

func discoverActiveSprint(ctx context.Context, client boardCheckClient, issues []jira.Issue) (jira.Sprint, bool, error) {
	var found jira.Sprint
	for _, project := range projectKeysForIssues(issues) {
		sprint, ok, err := discoverActiveSprintForProject(ctx, client, project, project)
		if err != nil {
			return jira.Sprint{}, false, err
		}
		if !ok {
			sprint, ok, err = discoverActiveSprintForProject(ctx, client, project, "")
			if err != nil {
				return jira.Sprint{}, false, err
			}
		}
		if !ok {
			continue
		}
		if found.ID > 0 && found.ID != sprint.ID {
			return jira.Sprint{}, false, nil
		}
		found = sprint
	}
	return found, found.ID > 0, nil
}

func discoverActiveSprintForProject(ctx context.Context, client boardCheckClient, project string, boardProject string) (jira.Sprint, bool, error) {
	boards, err := client.GetBoards(ctx, boardProject, 0, 50)
	if err != nil {
		if boardProject == "" {
			return jira.Sprint{}, false, fmt.Errorf("load boards: %w", err)
		}
		return jira.Sprint{}, false, fmt.Errorf("load boards for project %s: %w", boardProject, err)
	}
	var found jira.Sprint
	for _, board := range boards.Boards {
		if !strings.EqualFold(board.Type, "scrum") {
			continue
		}
		sprint, ok, err := activeSprintForBoard(ctx, client, board.ID)
		if err != nil {
			return jira.Sprint{}, false, err
		}
		if !ok {
			continue
		}
		matches, err := client.SearchIssues(ctx, fmt.Sprintf("project = %s AND sprint = %d", project, sprint.ID), 1)
		if err != nil {
			return jira.Sprint{}, false, fmt.Errorf("check sprint %d project %s: %w", sprint.ID, project, err)
		}
		if len(matches) == 0 {
			continue
		}
		if found.ID > 0 && found.ID != sprint.ID {
			return jira.Sprint{}, false, nil
		}
		found = sprint
	}
	return found, found.ID > 0, nil
}

func projectKeysForIssues(issues []jira.Issue) []string {
	seen := map[string]bool{}
	var projects []string
	for _, issue := range issues {
		project := projectKeyFromIssueKey(issue.Key)
		if project == "" || seen[project] {
			continue
		}
		seen[project] = true
		projects = append(projects, project)
	}
	return projects
}

func printBoardFindings(out io.Writer, findings []boardCheckFinding) {
	for _, item := range findings {
		_, _ = fmt.Fprintf(out, "%s %s: %s\n", item.Finding.Severity, item.Issue.Key, item.Finding.Message)
	}
}

func printBoardFixPlan(out io.Writer, cfg config.Config, findings []boardCheckFinding, sprint jira.Sprint, sprintKnown bool) {
	printManualBoardReview(out, cfg, findings, sprint)
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
		case boardcheck.CodeMissingBoard:
			if defaultTeamConfigured(cfg) && !isEpicIssue(item.Issue) {
				_, _ = fmt.Fprintf(out, "- Set %s Team to %s if currently empty.\n", item.Issue.Key, displayValue(cfg.DefaultTeamName, cfg.DefaultTeamID))
			}
		}
	}
}

func printManualBoardReview(out io.Writer, cfg config.Config, findings []boardCheckFinding, sprint jira.Sprint) {
	if !hasBoardFinding(findings, boardcheck.CodeMissingBoard) {
		return
	}
	_, _ = fmt.Fprintln(out, "\nManual review:")
	for _, item := range findings {
		if item.Finding.Code != boardcheck.CodeMissingBoard {
			continue
		}
		if isEpicIssue(item.Issue) {
			_, _ = fmt.Fprintf(out, "- %s is an Epic; board %d issue cards do not include Epics. Track Story/Task children in the sprint board or use the Epic view.\n", item.Issue.Key, sprint.BoardID)
			continue
		}
		if strings.TrimSpace(cfg.DefaultTeamID) == "" {
			_, _ = fmt.Fprintf(out, "- Review board %d filter/status mapping for %s.\n", sprint.BoardID, item.Issue.Key)
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

func freshBoardWriteContext(cfg config.Config) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), maxDuration(cfg.RequestTimeout, 30*time.Second))
}

func applyBoardFixes(ctx context.Context, cfg config.Config, client boardCheckClient, out io.Writer, findings []boardCheckFinding, sprint jira.Sprint, sprintKnown bool) error {
	var currentUser jira.User
	var sprintIssueKeys []string
	var teamVerifyKeys []string
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
				sprintIssueKeys = append(sprintIssueKeys, item.Issue.Key)
			}
		case boardcheck.CodeSubtaskUnderEpic:
			if err := convertSubtaskToStandardIssue(ctx, cfg, client, item.Issue); err != nil {
				_, _ = fmt.Fprintf(out, "Manual fix needed for %s: %v\n", item.Issue.Key, err)
				continue
			}
			_, _ = fmt.Fprintf(out, "Converted %s to Story/Task.\n", item.Issue.Key)
		case boardcheck.CodeMissingBoard:
			if !defaultTeamConfigured(cfg) || isEpicIssue(item.Issue) {
				continue
			}
			detail, err := client.GetIssue(ctx, item.Issue.Key)
			if err != nil {
				_, _ = fmt.Fprintf(out, "Manual fix needed for %s: load current Team: %v\n", item.Issue.Key, err)
				continue
			}
			if strings.TrimSpace(detail.TeamID) != "" {
				teamVerifyKeys = append(teamVerifyKeys, item.Issue.Key)
				_, _ = fmt.Fprintf(out, "Skipped Team update for %s: already %s.\n", item.Issue.Key, displayValue(detail.TeamName, detail.TeamID))
				continue
			}
			value := jira.EditFieldValue{
				FieldID:    strings.TrimSpace(cfg.DefaultTeamFieldID),
				SchemaType: "team",
				Option: jira.FieldOption{
					ID:   strings.TrimSpace(cfg.DefaultTeamID),
					Name: strings.TrimSpace(cfg.DefaultTeamName),
				},
			}
			if err := client.UpdateEditField(ctx, item.Issue.Key, value); err != nil {
				_, _ = fmt.Fprintf(out, "Manual fix needed for %s: set Team: %v\n", item.Issue.Key, err)
				continue
			}
			teamVerifyKeys = append(teamVerifyKeys, item.Issue.Key)
			_, _ = fmt.Fprintf(out, "Set %s Team to %s.\n", item.Issue.Key, displayValue(cfg.DefaultTeamName, cfg.DefaultTeamID))
		}
	}
	if len(sprintIssueKeys) > 0 {
		if err := client.MoveIssuesToSprint(ctx, sprint.ID, sprintIssueKeys); err != nil {
			return fmt.Errorf("add %s to sprint %s: %w", strings.Join(sprintIssueKeys, ", "), sprint.Name, err)
		}
		if len(sprintIssueKeys) == 1 {
			_, _ = fmt.Fprintf(out, "Added %s to sprint %s.\n", sprintIssueKeys[0], sprint.Name)
		} else {
			_, _ = fmt.Fprintf(out, "Added %d tickets to sprint %s.\n", len(sprintIssueKeys), sprint.Name)
		}
		stillMissing, err := boardMissingIssueKeys(ctx, client, sprint.BoardID, sprintIssueKeys)
		if err != nil {
			return err
		}
		if len(stillMissing) > 0 {
			_, _ = fmt.Fprintf(out, "Still not visible on board %d after sprint move: %s.\n", sprint.BoardID, strings.Join(stillMissing, ", "))
		}
	}
	if len(teamVerifyKeys) > 0 {
		stillMissing, err := boardMissingIssueKeys(ctx, client, sprint.BoardID, teamVerifyKeys)
		if err != nil {
			return err
		}
		if len(stillMissing) > 0 {
			_, _ = fmt.Fprintf(out, "Still not visible on board %d after Team update: %s.\n", sprint.BoardID, strings.Join(stillMissing, ", "))
		}
	}
	return nil
}

func boardMissingIssueKeys(ctx context.Context, client boardCheckClient, boardID int, issueKeys []string) ([]string, error) {
	if boardID <= 0 {
		return nil, nil
	}
	var missing []string
	for _, key := range issueKeys {
		key = strings.ToUpper(strings.TrimSpace(key))
		if key == "" {
			continue
		}
		matches, err := client.SearchBoardIssues(ctx, boardID, "key = "+key, 1)
		if err != nil {
			return nil, fmt.Errorf("verify board visibility for %s: %w", key, err)
		}
		if len(matches) == 0 {
			missing = append(missing, key)
		}
	}
	return missing, nil
}

func hasApplicableBoardFix(cfg config.Config, findings []boardCheckFinding, sprintKnown bool) bool {
	for _, item := range findings {
		switch item.Finding.Code {
		case boardcheck.CodeUnassigned, boardcheck.CodeSubtaskUnderEpic:
			return true
		case boardcheck.CodeMissingBoard:
			if defaultTeamConfigured(cfg) && !isEpicIssue(item.Issue) {
				return true
			}
		case boardcheck.CodeMissingActiveSprint:
			if sprintKnown {
				return true
			}
		}
	}
	return false
}

func printNoApplicableBoardFix(out io.Writer, findings []boardCheckFinding, sprintKnown bool) {
	if !sprintKnown && hasBoardFinding(findings, boardcheck.CodeMissingActiveSprint) {
		_, _ = fmt.Fprintln(out, "No fixes can be applied. Pass --board <id> or set queries.default_board_id to enable active sprint fixes.")
		return
	}
	if hasBoardFinding(findings, boardcheck.CodeMissingBoard) {
		_, _ = fmt.Fprintln(out, "No automatic fixes can be applied. Review the manual board notes above.")
		return
	}
	_, _ = fmt.Fprintln(out, "No automatic fixes can be applied. Review the board filter or status column mapping.")
}

func isEpicIssue(issue jira.Issue) bool {
	return strings.EqualFold(strings.TrimSpace(issue.IssueType), "Epic") || issue.HierarchyLevel > 0
}

func defaultTeamConfigured(cfg config.Config) bool {
	return strings.TrimSpace(cfg.DefaultTeamFieldID) != "" && strings.TrimSpace(cfg.DefaultTeamID) != ""
}

func hasBoardFinding(findings []boardCheckFinding, code boardcheck.Code) bool {
	for _, item := range findings {
		if item.Finding.Code == code {
			return true
		}
	}
	return false
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
