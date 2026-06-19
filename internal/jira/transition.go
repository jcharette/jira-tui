package jira

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	model "github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"
)

type Transition struct {
	ID          string
	Name        string
	ToStatus    string
	HasScreen   bool
	IsAvailable bool
	Fields      []TransitionField
}

type TransitionField struct {
	ID              string
	Name            string
	Required        bool
	SchemaType      string
	SchemaSystem    string
	SchemaItems     string
	SchemaCustom    string
	SchemaCustomID  int
	AutoCompleteURL string
	AllowedValues   []FieldOption
}

type TransitionFieldValue struct {
	FieldID      string
	SchemaType   string
	SchemaSystem string
	SchemaItems  string
	Option       FieldOption
	Options      []FieldOption
	Text         string
}

type TransitionIssueRequest struct {
	TransitionID string
	Fields       []TransitionFieldValue
}

type transitionFieldsResponse struct {
	Transitions []transitionFieldsRaw `json:"transitions"`
}

type transitionFieldsRaw struct {
	ID          string                        `json:"id"`
	Name        string                        `json:"name"`
	To          *model.StatusScheme           `json:"to"`
	HasScreen   bool                          `json:"hasScreen"`
	IsAvailable bool                          `json:"isAvailable"`
	Fields      map[string]transitionFieldRaw `json:"fields"`
}

type transitionFieldRaw struct {
	Name            string                   `json:"name"`
	Required        bool                     `json:"required"`
	AutoCompleteURL string                   `json:"autoCompleteUrl"`
	Schema          transitionFieldSchema    `json:"schema"`
	AllowedValues   []transitionAllowedValue `json:"allowedValues"`
}

type transitionFieldSchema struct {
	Type     string `json:"type"`
	System   string `json:"system"`
	Items    string `json:"items"`
	Custom   string `json:"custom"`
	CustomID int    `json:"customId"`
}

type transitionAllowedValue struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Value       string `json:"value"`
	DisplayName string `json:"displayName"`
	Key         string `json:"key"`
	AccountID   string `json:"accountId"`
}

func (c *Client) getTransitionsWithFields(ctx context.Context, key string) ([]Transition, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, fmt.Errorf("get jira transitions: empty issue key")
	}
	endpoint := fmt.Sprintf("rest/api/3/issue/%s/transitions?expand=transitions.fields", key)
	request, err := c.rest.NewRequest(ctx, http.MethodGet, endpoint, "", nil)
	if err != nil {
		return nil, fmt.Errorf("get jira transitions %s: %w", key, err)
	}
	var response transitionFieldsResponse
	if _, err := c.rest.Call(request, &response); err != nil {
		return nil, fmt.Errorf("get jira transitions %s: %w", key, err)
	}
	transitions := make([]Transition, 0, len(response.Transitions))
	for _, raw := range response.Transitions {
		if transition, ok := parseTransitionFields(raw); ok {
			transitions = append(transitions, transition)
		}
	}
	return transitions, nil
}

func (c *Client) TransitionIssue(ctx context.Context, key string, request TransitionIssueRequest) error {
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	options, err := transitionMoveOptions(request.Fields)
	if err != nil {
		return fmt.Errorf("transition jira issue %s: %w", key, err)
	}
	if _, err := c.issue.Move(ctx, key, request.TransitionID, options); err != nil {
		return fmt.Errorf("transition jira issue %s: %w", key, err)
	}
	return nil
}

func transitionMoveOptions(values []TransitionFieldValue) (*model.IssueMoveOptionsV3, error) {
	if len(values) == 0 {
		return nil, nil
	}
	fields := &model.IssueFieldsScheme{}
	customFields := &model.CustomFields{}
	operations := &model.UpdateOperations{}
	hasField := false
	hasCustomField := false
	hasOperation := false
	for _, value := range values {
		fieldID := strings.TrimSpace(value.FieldID)
		switch fieldID {
		case "":
			continue
		case "resolution":
			if value.Option.ID == "" && value.Option.Name == "" {
				return nil, fmt.Errorf("resolution transition field requires a selected value")
			}
			fields.Resolution = &model.ResolutionScheme{ID: value.Option.ID, Name: value.Option.Name}
			hasField = true
		case "comment":
			body := strings.TrimSpace(value.Text)
			if body == "" {
				continue
			}
			if err := operations.AddMultiRawOperation("comment", []map[string]interface{}{
				{
					"add": map[string]interface{}{
						"body": plainTextADF(body, nil),
					},
				},
			}); err != nil {
				return nil, err
			}
			hasOperation = true
		default:
			if strings.HasPrefix(fieldID, "customfield_") {
				raw, ok := transitionCustomFieldPayload(value)
				if !ok {
					return nil, fmt.Errorf("transition field %s requires a value", fieldID)
				}
				if err := customFields.Raw(fieldID, raw); err != nil {
					return nil, err
				}
				hasCustomField = true
				continue
			}
			return nil, fmt.Errorf("unsupported transition field %s", value.FieldID)
		}
	}
	if !hasField && !hasCustomField && !hasOperation {
		return nil, nil
	}
	options := &model.IssueMoveOptionsV3{
		Fields: &model.IssueScheme{Fields: fields},
	}
	if hasCustomField {
		options.CustomFields = customFields
	}
	if hasOperation {
		options.Operations = operations
	}
	return options, nil
}

func transitionCustomFieldPayload(value TransitionFieldValue) (interface{}, bool) {
	text := strings.TrimSpace(value.Text)
	schemaType := strings.ToLower(strings.TrimSpace(value.SchemaType))
	schemaItems := strings.ToLower(strings.TrimSpace(value.SchemaItems))
	switch schemaType {
	case "number":
		if text == "" {
			return nil, false
		}
		number, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return nil, false
		}
		return number, true
	case "string", "text", "textarea", "date", "datetime":
		if text == "" {
			return nil, false
		}
		return text, true
	case "user":
		return transitionUserPayload(value.Option)
	case "array":
		options := value.Options
		if len(options) == 0 && (value.Option.ID != "" || value.Option.Name != "") {
			options = []FieldOption{value.Option}
		}
		if len(options) == 0 {
			return nil, false
		}
		items := make([]map[string]interface{}, 0, len(options))
		for _, option := range options {
			var item map[string]interface{}
			if schemaItems == "user" {
				item, _ = transitionUserPayload(option)
			} else {
				item = createFieldOptionPayload(option)
			}
			if len(item) > 0 {
				items = append(items, item)
			}
		}
		if len(items) == 0 {
			return nil, false
		}
		return items, true
	case "option", "priority":
		option := createFieldOptionPayload(value.Option)
		if len(option) == 0 {
			return nil, false
		}
		return option, true
	default:
		if value.Option.ID != "" || value.Option.Name != "" {
			option := createFieldOptionPayload(value.Option)
			if len(option) > 0 {
				return option, true
			}
		}
		if text == "" {
			return nil, false
		}
		return text, true
	}
}

func transitionUserPayload(option FieldOption) (map[string]interface{}, bool) {
	accountID := strings.TrimSpace(option.ID)
	if accountID == "" {
		return nil, false
	}
	return map[string]interface{}{"accountId": accountID}, true
}

func parseTransition(raw *model.IssueTransitionScheme) (Transition, bool) {
	if raw == nil || raw.ID == "" {
		return Transition{}, false
	}
	transition := Transition{
		ID:          raw.ID,
		Name:        raw.Name,
		HasScreen:   raw.HasScreen,
		IsAvailable: raw.IsAvailable,
	}
	if raw.To != nil {
		transition.ToStatus = raw.To.Name
	}
	return transition, true
}

func parseTransitionFields(raw transitionFieldsRaw) (Transition, bool) {
	if raw.ID == "" {
		return Transition{}, false
	}
	transition := Transition{
		ID:          raw.ID,
		Name:        raw.Name,
		HasScreen:   raw.HasScreen,
		IsAvailable: raw.IsAvailable,
		Fields:      parseTransitionFieldMap(raw.Fields),
	}
	if raw.To != nil {
		transition.ToStatus = raw.To.Name
	}
	return transition, true
}

func parseTransitionFieldMap(rawFields map[string]transitionFieldRaw) []TransitionField {
	if len(rawFields) == 0 {
		return nil
	}
	ids := make([]string, 0, len(rawFields))
	for id := range rawFields {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	fields := make([]TransitionField, 0, len(ids))
	for _, id := range ids {
		raw := rawFields[id]
		allowedValues := make([]FieldOption, 0, len(raw.AllowedValues))
		for _, value := range raw.AllowedValues {
			option := transitionFieldOption(value)
			if option.ID == "" && option.Name == "" {
				continue
			}
			allowedValues = append(allowedValues, option)
		}
		name := raw.Name
		if name == "" {
			name = id
		}
		fields = append(fields, TransitionField{
			ID:              id,
			Name:            name,
			Required:        raw.Required,
			SchemaType:      raw.Schema.Type,
			SchemaSystem:    raw.Schema.System,
			SchemaItems:     raw.Schema.Items,
			SchemaCustom:    raw.Schema.Custom,
			SchemaCustomID:  raw.Schema.CustomID,
			AutoCompleteURL: raw.AutoCompleteURL,
			AllowedValues:   allowedValues,
		})
	}
	return fields
}

func transitionFieldOption(value transitionAllowedValue) FieldOption {
	return FieldOption{
		ID:   firstNonEmpty(value.ID, value.AccountID, value.Key),
		Name: firstNonEmpty(value.Name, value.Value, value.DisplayName, value.Key),
	}
}
