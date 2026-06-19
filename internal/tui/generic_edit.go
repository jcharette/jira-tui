package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"github.com/jcharette/jira-tui/internal/jira"
	"github.com/jcharette/jira-tui/internal/worker"
)

func (m Model) genericEditFieldActions() []detailAction {
	selected, ok := m.selectedIssue()
	if !ok || strings.TrimSpace(selected.Key) == "" {
		return nil
	}
	metadata, ok := m.editMetadata[selected.Key]
	if !ok {
		return nil
	}
	supported := supportedEditFieldIDs()
	actions := make([]detailAction, 0, len(metadata.Fields))
	for _, field := range metadata.Fields {
		if !field.Editable {
			continue
		}
		fieldID := strings.TrimSpace(field.ID)
		if fieldID == "" || supported[fieldID] {
			continue
		}
		name := strings.TrimSpace(displayValue(field.Name, fieldID))
		enabled := genericEditFieldSupported(field)
		action := detailAction{
			ID:          "field:" + fieldID,
			Label:       "Edit " + name,
			Description: genericEditFieldDescription(field, enabled),
			Enabled:     enabled,
		}
		if !enabled {
			action.DisabledState = "Unsupported"
			action.DisabledReason = name + " is editable in Jira but needs a field-specific workflow before it can be changed safely."
		}
		actions = append(actions, action)
	}
	return actions
}

func genericEditFieldSupported(field jira.EditField) bool {
	if !field.Editable || !strings.HasPrefix(strings.TrimSpace(field.ID), "customfield_") {
		return false
	}
	schemaType := strings.ToLower(strings.TrimSpace(field.SchemaType))
	schemaItems := strings.ToLower(strings.TrimSpace(field.SchemaItems))
	schemaCustom := strings.ToLower(strings.TrimSpace(field.SchemaCustom))
	hasOptions := len(field.AllowedValues) > 0 || strings.TrimSpace(field.AutoCompleteURL) != ""
	switch schemaType {
	case "string", "text", "textarea", "number", "date", "datetime":
		return true
	case "option", "user", "version":
		return hasOptions
	case "array":
		switch {
		case strings.Contains(schemaCustom, "gh-sprint") || schemaItems == "sprint":
			return hasOptions
		case schemaItems == "option" || schemaItems == "user" || schemaItems == "version":
			return hasOptions
		default:
			return false
		}
	default:
		return strings.Contains(schemaCustom, "gh-sprint") && hasOptions
	}
}

func genericEditFieldUsesText(field jira.EditField) bool {
	switch strings.ToLower(strings.TrimSpace(field.SchemaType)) {
	case "string", "text", "textarea", "number", "date", "datetime":
		return true
	default:
		return false
	}
}

func genericEditFieldUsesMultiSelect(field jira.EditField) bool {
	return strings.EqualFold(strings.TrimSpace(field.SchemaType), "array")
}

func genericEditFieldUsesPicker(field jira.EditField) bool {
	return !genericEditFieldUsesText(field)
}

func genericEditFieldUsesAutocomplete(field jira.EditField) bool {
	return strings.TrimSpace(field.AutoCompleteURL) != ""
}

func genericEditFieldDescription(field jira.EditField, enabled bool) string {
	if !enabled {
		return unsupportedEditFieldDescription(field)
	}
	parts := []string{editFieldOptionSourceLabel(field)}
	if schema := editFieldSchemaLabel(field); schema != "" {
		parts = append(parts, "schema: "+schema)
	}
	parts = append(parts, "generic editor")
	return strings.Join(parts, "; ")
}

func (m Model) startGenericFieldEditor(fieldID string) (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok || strings.TrimSpace(selected.Key) == "" {
		return m, nil
	}
	fieldID = strings.TrimSpace(fieldID)
	if fieldID == "" {
		m.detailNotice = "No Jira field selected."
		return m, nil
	}
	m.linkFocus = false
	m.hierarchyFocus = false
	m.actionFocus = false
	m.transitionFocus = false
	m.summaryFocus = false
	m.priorityFocus = false
	m.labelsFocus = false
	m.componentsFocus = false
	m.assigneeFocus = false
	m.genericFieldFocus = true
	m.genericFieldEditingID = fieldID
	m.hydrateIssueEditMetadata(selected.Key)
	if metadata, ok := m.editMetadata[selected.Key]; ok {
		if _, cached := m.cachedIssueEditMetadata(selected.Key); !cached || m.isIssueEditMetadataFresh(selected.Key) {
			return m.beginGenericFieldEditing(metadata), nil
		}
	}
	if m.genericFieldMetadataLoading && m.genericFieldMetadataRequestKey == selected.Key {
		return m, nil
	}
	m.nextRequestID++
	m.activeGenericFieldMetadataReqID = m.nextRequestID
	m.genericFieldMetadataRequestKey = selected.Key
	m.genericFieldMetadataLoading = true
	m.genericFieldMetadataErr = nil
	m.detailNotice = ""
	return m, m.submitEditMetadata(m.activeGenericFieldMetadataReqID, selected.Key)
}

func (m Model) beginGenericFieldEditing(metadata jira.EditMetadata) Model {
	field, ok := editFieldByID(metadata.Fields, m.genericFieldEditingID)
	if !ok {
		m.closeGenericFieldEditor()
		m.detailNotice = "Jira field metadata is not available."
		return m
	}
	if !genericEditFieldSupported(field) {
		m.closeGenericFieldEditor()
		m.detailNotice = displayValue(field.Name, field.ID) + " needs a field-specific workflow before it can be changed safely."
		return m
	}
	m.genericField = field
	m.genericFieldFocus = true
	m.genericFieldDraft = ""
	m.genericFieldDirty = false
	m.selectedGenericFieldOption = 0
	m.selectedGenericFieldOptions = map[string]bool{}
	m.genericFieldOptionsLoading = false
	m.genericFieldOptionsErr = nil
	m.genericFieldOptionsQuery = ""
	m.genericFieldEditor = newGenericFieldTextEditor("")
	m.genericFieldEditorReady = true
	m.detailNotice = ""
	return m
}

func editFieldByID(fields []jira.EditField, fieldID string) (jira.EditField, bool) {
	fieldID = strings.TrimSpace(fieldID)
	for _, field := range fields {
		if strings.TrimSpace(field.ID) == fieldID {
			return field, true
		}
	}
	return jira.EditField{}, false
}

func (m *Model) closeGenericFieldEditor() {
	m.genericFieldFocus = false
	m.genericFieldMetadataLoading = false
	m.genericFieldMetadataErr = nil
	m.genericFieldEditingID = ""
	m.genericField = jira.EditField{}
	m.genericFieldDraft = ""
	m.genericFieldEditor = textarea.Model{}
	m.genericFieldEditorReady = false
	m.selectedGenericFieldOption = 0
	m.selectedGenericFieldOptions = nil
	m.genericFieldOptionsLoading = false
	m.genericFieldOptionsErr = nil
	m.genericFieldOptionsQuery = ""
	m.genericFieldDirty = false
	m.genericFieldSubmitting = false
	m.genericFieldSubmitKey = ""
	m.genericFieldSubmitField = jira.EditField{}
	m.genericFieldSubmitValue = jira.EditFieldValue{}
	m.detailNotice = ""
}

func (m Model) renderGenericFieldLoadingDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	bodyWidth := min(max(24, width-12), 60)
	body := m.detailStatusBlock("Loading field metadata...", bodyWidth, false)
	return m.renderDetailDialog(width, "Edit Field", selected.Key, body, "esc cancel")
}

func (m Model) renderGenericFieldDialog(width int) string {
	selected, ok := m.selectedIssue()
	if !ok {
		return ""
	}
	bodyWidth := min(max(24, width-12), 64)
	field := m.genericField
	fieldName := displayValue(field.Name, field.ID)
	lines := []string{
		m.theme.Muted.Render("Field: ") + m.theme.Text.Render(fieldName),
		m.theme.Muted.Render("Type: ") + m.theme.Text.Render(displayValue(editFieldSchemaLabel(field), "custom field")),
	}
	if m.genericFieldSubmitting && m.genericFieldSubmitKey == selected.Key {
		lines = append(lines, "", m.detailStatusBlock("Updating field...", bodyWidth, false))
	} else if genericEditFieldUsesText(field) {
		lines = append(lines, "", m.configuredGenericFieldEditor(bodyWidth, genericFieldEditorRows(field)).View())
	} else {
		if genericEditFieldUsesAutocomplete(field) {
			filter := displayValue(m.genericFieldDraft, "")
			lines = append(lines, "", m.theme.Muted.Render("Filter: ")+m.theme.Text.Render(displayValue(filter, "type to search")))
			if m.genericFieldOptionsLoading {
				lines = append(lines, m.theme.Muted.Render("Searching Jira options."))
			}
			if m.genericFieldOptionsErr != nil {
				lines = append(lines, m.theme.Error.Render("Options failed: "+m.genericFieldOptionsErr.Error()))
			}
		}
		lines = append(lines, "")
		lines = append(lines, m.renderGenericFieldOptions(bodyWidth)...)
	}
	if m.detailNotice != "" {
		lines = append(lines, "", m.renderDetailNotice(m.detailNotice, bodyWidth))
	}
	footer := "enter save  esc cancel"
	if !genericEditFieldUsesText(field) {
		footer = "type filter  j/k select  space toggle  enter save  esc cancel"
	}
	return m.renderDetailDialog(width, "Edit "+fieldName, selected.Key, strings.Join(lines, "\n"), footer)
}

func (m Model) renderGenericFieldOptions(width int) []string {
	field := m.genericField
	if len(field.AllowedValues) == 0 {
		if genericEditFieldUsesAutocomplete(field) {
			return []string{m.detailEmptyState("Type to search Jira values.", width)}
		}
		return []string{m.detailEmptyState("No Jira values are available.", width)}
	}
	cursor := clamp(m.selectedGenericFieldOption, 0, len(field.AllowedValues)-1)
	choices := make([]choiceListOption, 0, len(field.AllowedValues))
	for _, option := range field.AllowedValues {
		label := displayValue(option.Name, option.ID)
		if genericEditFieldUsesMultiSelect(field) {
			if m.selectedGenericFieldOptions[fieldOptionSelectionKey(option)] {
				label = "[x] " + label
			} else {
				label = "[ ] " + label
			}
		}
		choices = append(choices, choiceListOption{Label: label})
	}
	return m.renderChoiceList(choices, cursor, width, createPickerMaxRows)
}

func (m Model) updateGenericFieldEditor(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.closeGenericFieldEditor()
		return m, nil
	case "enter":
		return m.submitGenericField()
	case "up", "k":
		m.moveSelectedGenericFieldOption(-1)
		return m, nil
	case "down", "j":
		m.moveSelectedGenericFieldOption(1)
		return m, nil
	case " ":
		m.toggleSelectedGenericFieldOption()
		return m, nil
	}
	if !genericEditFieldUsesText(m.genericField) {
		if genericEditFieldUsesAutocomplete(m.genericField) {
			switch msg.String() {
			case "backspace", "ctrl+h":
				query := []rune(m.genericFieldDraft)
				if len(query) > 0 {
					m.genericFieldDraft = string(query[:len(query)-1])
					m.genericFieldDirty = false
					return m, m.requestGenericFieldOptions()
				}
				return m, nil
			}
			if len(msg.String()) == 1 {
				m.genericFieldDraft += msg.String()
				m.genericFieldDirty = false
				return m, m.requestGenericFieldOptions()
			}
		}
		return m, nil
	}
	m.ensureGenericFieldEditor()
	before := m.genericFieldEditor.Value()
	editor, cmd := m.genericFieldEditor.Update(msg)
	m.genericFieldEditor = editor
	m.genericFieldDraft = m.genericFieldEditor.Value()
	if m.genericFieldDraft != before {
		m.genericFieldDirty = true
		m.detailNotice = ""
	}
	return m, cmd
}

func (m *Model) moveSelectedGenericFieldOption(delta int) {
	if genericEditFieldUsesText(m.genericField) {
		return
	}
	options := m.genericField.AllowedValues
	if len(options) == 0 {
		m.selectedGenericFieldOption = 0
		return
	}
	m.selectedGenericFieldOption = clamp(m.selectedGenericFieldOption+delta, 0, len(options)-1)
	if !genericEditFieldUsesMultiSelect(m.genericField) {
		m.genericFieldDirty = true
	}
}

func (m *Model) toggleSelectedGenericFieldOption() {
	if !genericEditFieldUsesMultiSelect(m.genericField) {
		return
	}
	options := m.genericField.AllowedValues
	if len(options) == 0 {
		return
	}
	cursor := clamp(m.selectedGenericFieldOption, 0, len(options)-1)
	key := fieldOptionSelectionKey(options[cursor])
	if m.selectedGenericFieldOptions == nil {
		m.selectedGenericFieldOptions = map[string]bool{}
	}
	m.selectedGenericFieldOptions[key] = !m.selectedGenericFieldOptions[key]
	m.genericFieldDirty = true
	m.detailNotice = ""
}

func (m Model) submitGenericField() (Model, tea.Cmd) {
	selected, ok := m.selectedIssue()
	if !ok {
		return m, nil
	}
	if genericEditFieldUsesText(m.genericField) && !m.genericFieldDirty {
		m.detailNotice = "Edit field before saving."
		return m, nil
	}
	value, ok := m.genericFieldValue()
	if !ok {
		m.detailNotice = "Select or enter a value before saving."
		return m, nil
	}
	m.nextRequestID++
	m.activeGenericFieldReqID = m.nextRequestID
	m.genericFieldSubmitting = true
	m.genericFieldSubmitKey = selected.Key
	m.genericFieldSubmitField = m.genericField
	m.genericFieldSubmitValue = value
	m.detailNotice = "Updating " + displayValue(m.genericField.Name, m.genericField.ID) + "."
	return m, m.submitUpdateEditField(m.activeGenericFieldReqID, selected.Key, m.genericField, value)
}

func (m Model) genericFieldValue() (jira.EditFieldValue, bool) {
	field := m.genericField
	value := jira.EditFieldValue{
		FieldID:      field.ID,
		SchemaType:   field.SchemaType,
		SchemaSystem: field.SchemaSystem,
		SchemaItems:  field.SchemaItems,
		SchemaCustom: field.SchemaCustom,
	}
	if genericEditFieldUsesText(field) {
		value.Text = strings.TrimSpace(m.genericFieldEditorValue())
		return value, value.Text != ""
	}
	if genericEditFieldUsesMultiSelect(field) {
		for _, option := range field.AllowedValues {
			if m.selectedGenericFieldOptions[fieldOptionSelectionKey(option)] {
				value.Options = append(value.Options, option)
			}
		}
		return value, len(value.Options) > 0
	}
	if len(field.AllowedValues) == 0 {
		return value, false
	}
	value.Option = field.AllowedValues[clamp(m.selectedGenericFieldOption, 0, len(field.AllowedValues)-1)]
	return value, value.Option.ID != "" || value.Option.Name != ""
}

func (m *Model) replaceGenericFieldOptions(options []jira.FieldOption) {
	m.genericField.AllowedValues = append([]jira.FieldOption(nil), options...)
	if len(options) > 0 {
		m.selectedGenericFieldOption = 0
	} else {
		m.selectedGenericFieldOption = 0
	}
	m.selectedGenericFieldOptions = map[string]bool{}
}

func (m *Model) requestGenericFieldOptions() tea.Cmd {
	field := m.genericField
	fieldID := strings.TrimSpace(field.ID)
	query := strings.TrimSpace(m.genericFieldDraft)
	if fieldID == "" || strings.TrimSpace(field.AutoCompleteURL) == "" {
		return nil
	}
	if m.genericFieldOptionsLoading && m.genericFieldOptionsQuery == query {
		return nil
	}
	m.nextRequestID++
	m.activeGenericFieldOptionsReqID = m.nextRequestID
	m.genericFieldOptionsLoading = true
	m.genericFieldOptionsErr = nil
	m.genericFieldOptionsQuery = query
	return m.submitGenericFieldOptions(m.activeGenericFieldOptionsReqID, field, query)
}

func (m Model) handleGenericFieldOptionsResult(result worker.Result) Model {
	if result.ID != m.activeGenericFieldOptionsReqID {
		return m
	}
	m.genericFieldOptionsLoading = false
	if result.SearchFieldOptions == nil {
		m.genericFieldOptionsErr = worker.ErrInvalidRequest
		return m
	}
	if result.SearchFieldOptions.Query != m.genericFieldOptionsQuery {
		return m
	}
	if result.Err != nil {
		m.genericFieldOptionsErr = result.Err
		return m
	}
	m.genericFieldOptionsErr = nil
	m.replaceGenericFieldOptions(result.SearchFieldOptions.Options)
	return m
}

func fieldOptionSelectionKey(option jira.FieldOption) string {
	return strings.ToLower(displayValue(option.ID, option.Name))
}

func (m *Model) ensureGenericFieldEditor() {
	if m.genericFieldEditorReady {
		return
	}
	m.genericFieldEditor = newGenericFieldTextEditor(m.genericFieldDraft)
	m.genericFieldEditorReady = true
}

func (m Model) configuredGenericFieldEditor(width int, rows int) textarea.Model {
	editor := m.genericFieldEditor
	if !m.genericFieldEditorReady {
		editor = newGenericFieldTextEditor(m.genericFieldDraft)
	}
	editor.SetWidth(max(24, width))
	editor.SetHeight(rows)
	editor.MaxWidth = max(24, width)
	editor.MaxHeight = rows
	editor.Focus()
	return editor
}

func (m Model) genericFieldEditorValue() string {
	if m.genericFieldEditorReady {
		return m.genericFieldEditor.Value()
	}
	return m.genericFieldDraft
}

func newGenericFieldTextEditor(value string) textarea.Model {
	editor := textarea.New()
	editor.Prompt = ""
	editor.Placeholder = "Enter Jira field value"
	editor.SetValue(value)
	editor.Focus()
	editor.CharLimit = 2000
	editor.ShowLineNumbers = false
	return editor
}

func genericFieldEditorRows(field jira.EditField) int {
	switch strings.ToLower(strings.TrimSpace(field.SchemaType)) {
	case "text", "textarea":
		return 5
	default:
		return 2
	}
}

func debugGenericFieldValue(value jira.EditFieldValue) string {
	if len(value.Options) > 0 {
		return fmt.Sprintf("%d values", len(value.Options))
	}
	if value.Option.ID != "" || value.Option.Name != "" {
		return displayValue(value.Option.Name, value.Option.ID)
	}
	return value.Text
}
