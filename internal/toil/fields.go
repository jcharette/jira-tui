package toil

import (
	"strings"

	"github.com/jcharette/jira-tui/internal/config"
	"github.com/jcharette/jira-tui/internal/jira"
)

func CreateFields(cfg config.Config) []jira.CreateIssueFieldValue {
	fields := []jira.CreateIssueFieldValue{{FieldID: "labels", SchemaSystem: "labels", Text: "toil"}}
	teamFieldID := strings.TrimSpace(cfg.DefaultTeamFieldID)
	teamID := strings.TrimSpace(cfg.DefaultTeamID)
	if teamFieldID == "" || teamID == "" {
		return fields
	}
	fields = append(fields, jira.CreateIssueFieldValue{
		FieldID:    teamFieldID,
		SchemaType: "team",
		Option: jira.FieldOption{
			ID:   teamID,
			Name: strings.TrimSpace(cfg.DefaultTeamName),
		},
	})
	return fields
}
