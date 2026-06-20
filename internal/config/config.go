package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/jcharette/jira-tui/internal/secretstore"
)

const defaultJQL = "assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC"
const defaultRefreshInterval = 2 * time.Minute
const defaultRequestTimeout = 20 * time.Second
const defaultClaudeTimeout = 2 * time.Minute
const defaultBranchTemplate = "{key}-{summary_slug}"
const defaultWorkerCount = 2
const defaultQueueSize = 16
const currentVersion = 1
const tokenSourceKeyring = "keyring"
const tokenSourcePlaintext = "plaintext"

var defaultSecretStore = struct {
	sync.Mutex
	new func() secretstore.Store
}{
	new: func() secretstore.Store { return secretstore.NewKeyringStore() },
}

type Config struct {
	BaseURL         string
	Email           string
	APIToken        string
	APITokenSource  string
	ActiveProfile   string
	Profiles        map[string]Profile
	DefaultProject  string
	DefaultBoardID  int
	DefaultJQL      string
	ActiveView      string
	Views           []IssueView
	Theme           Theme
	Display         Display
	RefreshInterval time.Duration
	RequestTimeout  time.Duration
	WorkerCount     int
	QueueSize       int
	Git             Git
	Claude          Claude
	Notifications   Notifications
}

type Profile struct {
	BaseURL        string
	Email          string
	APIToken       string
	APITokenSource string
}

type IssueView struct {
	Name            string
	JQL             string
	IncludeChildren bool
}

type Theme struct {
	Name      string
	Symbols   ThemeSymbols
	Primary   string
	Secondary string
	Accent    string
	Success   string
	Warning   string
	Error     string
	Muted     string
	Border    string
	Surface   string
	Text      string
}

type ThemeSymbols struct {
	Epic      string
	Story     string
	Task      string
	Bug       string
	Subtask   string
	Issue     string
	Collapsed string
	Expanded  string
}

type Display struct {
	SymbolMode string
}

type Git struct {
	BranchTemplate string
}

type Claude struct {
	Enabled  bool
	Command  string
	Timeout  time.Duration
	Features ClaudeFeatures
	Gates    ClaudeGates
}

type ClaudeFeatures struct {
	TicketPlan          bool
	TicketAssist        bool
	ClarifyingQuestions bool
	DraftComment        bool
	DraftTicket         bool
	BranchPlan          bool
	CodeChanges         bool
	PRCreation          bool
	PRReviewResponse    bool
}

type ClaudeGates struct {
	RequireConfirmation bool
	AllowJiraWrites     bool
	AllowGitWrites      bool
	AllowGitHubWrites   bool
	AllowCodeEdits      bool
}

type Notifications struct {
	Enabled                   bool
	SystemEnabled             bool
	SystemOnNew               bool
	SystemOnUpdates           bool
	AutoOpenPanel             bool
	KeepPanelOpenUntilCleared bool
	MaxItems                  int
}

type LoadOptions struct {
	Path        string
	Profile     string
	SecretStore secretstore.Store
}

type ValidationError struct {
	Problems []string
}

func (e ValidationError) Error() string {
	return "invalid config: " + strings.Join(e.Problems, "; ")
}

func IsValidationError(err error) bool {
	var validationErr ValidationError
	return errors.As(err, &validationErr)
}

func DefaultPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find user config directory: %w", err)
	}
	return filepath.Join(configDir, "jira", "config.toml"), nil
}

func Defaults() Config {
	return Config{
		ActiveProfile:   "default",
		DefaultJQL:      defaultJQL,
		Theme:           DefaultTheme(),
		Display:         DefaultDisplay(),
		RefreshInterval: defaultRefreshInterval,
		RequestTimeout:  defaultRequestTimeout,
		WorkerCount:     defaultWorkerCount,
		QueueSize:       defaultQueueSize,
		Git: Git{
			BranchTemplate: defaultBranchTemplate,
		},
		Claude: Claude{
			Timeout: defaultClaudeTimeout,
			Gates: ClaudeGates{
				RequireConfirmation: true,
			},
		},
		Notifications: DefaultNotifications(),
	}
}

func DefaultNotifications() Notifications {
	return Notifications{
		Enabled:                   true,
		SystemOnNew:               true,
		AutoOpenPanel:             true,
		KeepPanelOpenUntilCleared: true,
		MaxItems:                  50,
	}
}

func DefaultDisplay() Display {
	return Display{SymbolMode: "auto"}
}

func DefaultTheme() Theme {
	theme, _, _ := BuiltInTheme("default")
	return theme
}

func BuiltInThemeNames() []string {
	return []string{"default", "focus", "ops", "high-contrast"}
}

func BuiltInTheme(name string) (Theme, string, bool) {
	switch normalizeThemeName(name) {
	case "", "default":
		return Theme{
			Name:      "default",
			Symbols:   ThemeSymbols{Epic: "◈", Story: "▣", Task: "●", Bug: "!", Subtask: "◇", Issue: "•", Collapsed: "▸", Expanded: "▾"},
			Primary:   "#7DD3FC",
			Secondary: "#A78BFA",
			Accent:    "#F59E0B",
			Success:   "#34D399",
			Warning:   "#FBBF24",
			Error:     "#F87171",
			Muted:     "#6B7280",
			Border:    "#374151",
			Surface:   "#111827",
			Text:      "#E5E7EB",
		}, "auto", true
	case "focus":
		return Theme{
			Name:      "focus",
			Symbols:   ThemeSymbols{Epic: "◇", Story: "□", Task: "•", Bug: "!", Subtask: "-", Issue: "·", Collapsed: "+", Expanded: "-"},
			Primary:   "#C7D2FE",
			Secondary: "#A5B4FC",
			Accent:    "#FDE68A",
			Success:   "#A7F3D0",
			Warning:   "#FCD34D",
			Error:     "#FDA4AF",
			Muted:     "#94A3B8",
			Border:    "#475569",
			Surface:   "#0F172A",
			Text:      "#E2E8F0",
		}, "symbols", true
	case "ops":
		return Theme{
			Name:      "ops",
			Symbols:   ThemeSymbols{Epic: "◆", Story: "●", Task: "▪", Bug: "!", Subtask: "◇", Issue: "•", Collapsed: "▸", Expanded: "▾"},
			Primary:   "#22D3EE",
			Secondary: "#34D399",
			Accent:    "#FACC15",
			Success:   "#4ADE80",
			Warning:   "#F97316",
			Error:     "#FB7185",
			Muted:     "#6EE7B7",
			Border:    "#059669",
			Surface:   "#052E2B",
			Text:      "#ECFEFF",
		}, "symbols", true
	case "high-contrast":
		return Theme{
			Name:      "high-contrast",
			Symbols:   ThemeSymbols{Epic: "EP", Story: "ST", Task: "TK", Bug: "!!", Subtask: "SU", Issue: "IS", Collapsed: "+", Expanded: "-"},
			Primary:   "#FFFFFF",
			Secondary: "#00FFFF",
			Accent:    "#FFFF00",
			Success:   "#00FF66",
			Warning:   "#FFCC00",
			Error:     "#FF3366",
			Muted:     "#BFC7D5",
			Border:    "#FFFFFF",
			Surface:   "#000000",
			Text:      "#FFFFFF",
		}, "plain", true
	default:
		return Theme{}, "", false
	}
}

func normalizeThemeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func DefaultThemeColors() Theme {
	return Theme{
		Name:      "default",
		Symbols:   ThemeSymbols{Epic: "◈", Story: "▣", Task: "●", Bug: "!", Subtask: "◇", Issue: "•", Collapsed: "▸", Expanded: "▾"},
		Primary:   "#7DD3FC",
		Secondary: "#A78BFA",
		Accent:    "#F59E0B",
		Success:   "#34D399",
		Warning:   "#FBBF24",
		Error:     "#F87171",
		Muted:     "#6B7280",
		Border:    "#374151",
		Surface:   "#111827",
		Text:      "#E5E7EB",
	}
}

func Load(options LoadOptions) (Config, error) {
	path, err := PathOrDefault(options.Path)
	if err != nil {
		return Config{}, err
	}

	cfg := Defaults()
	fileCfg, err := readFile(path)
	if err != nil {
		return Config{}, err
	}
	if err := applyFile(&cfg, fileCfg, options.Profile, secretStoreOrDefault(options.SecretStore)); err != nil {
		return Config{}, err
	}
	ensureViews(&cfg)
	if err := Validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func PathOrDefault(path string) (string, error) {
	if strings.TrimSpace(path) != "" {
		return path, nil
	}
	return DefaultPath()
}

func LoadEditable(options LoadOptions) (Config, string, []string, error) {
	path, err := PathOrDefault(options.Path)
	if err != nil {
		return Config{}, "", nil, err
	}

	cfg := Defaults()
	fileCfg, err := readFile(path)
	if err != nil {
		return Config{}, "", nil, err
	}
	if err := applyFile(&cfg, fileCfg, options.Profile, secretStoreOrDefault(options.SecretStore)); err != nil {
		return Config{}, "", nil, err
	}
	ensureViews(&cfg)

	var problems []string
	if err := Validate(cfg); err != nil {
		var validationErr ValidationError
		if !errors.As(err, &validationErr) {
			return Config{}, "", nil, err
		}
		problems = validationErr.Problems
	}
	return cfg, path, problems, nil
}

func Save(path string, cfg Config) error {
	return SaveWithSecretStore(path, cfg, secretStoreOrDefault(nil))
}

func SaveWithSecretStore(path string, cfg Config, store secretstore.Store) error {
	ensureViews(&cfg)
	if err := Validate(cfg); err != nil {
		return err
	}
	if store == nil {
		store = secretstore.NewKeyringStore()
	}

	path, err := PathOrDefault(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := os.Chmod(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("secure config directory: %w", err)
	}

	activeProfile := normalizeProfileName(cfg.ActiveProfile)
	profiles := profilesForSave(cfg, activeProfile)
	profileConfigs, err := profileConfigsForSave(context.Background(), store, profiles)
	if err != nil {
		return err
	}

	fileCfg := fileConfig{
		Version:       currentVersion,
		ActiveProfile: activeProfile,
		Profiles:      profileConfigs,
		Queries: queriesConfig{
			DefaultProject: cfg.DefaultProject,
			DefaultBoardID: cfg.DefaultBoardID,
			DefaultJQL:     cfg.DefaultJQL,
		},
		Views: viewsConfig{
			Active: cfg.ActiveView,
			Saved:  viewConfigs(cfg.Views),
		},
		Appearance: appearanceConfigFromTheme(cfg.Theme),
		Display: displayConfig{
			SymbolMode: cfg.Display.SymbolMode,
		},
		Runtime: runtimeConfig{
			RefreshInterval: cfg.RefreshInterval.String(),
			RequestTimeout:  cfg.RequestTimeout.String(),
			Workers:         cfg.WorkerCount,
			QueueSize:       cfg.QueueSize,
		},
		Git: gitConfig{
			BranchTemplate: cfg.Git.BranchTemplate,
		},
		Claude: claudeConfig{
			Enabled: cfg.Claude.Enabled,
			Command: cfg.Claude.Command,
			Timeout: cfg.Claude.Timeout.String(),
			Features: claudeFeaturesConfig{
				TicketPlan:          cfg.Claude.Features.TicketPlan,
				TicketAssist:        cfg.Claude.Features.TicketAssist,
				ClarifyingQuestions: cfg.Claude.Features.ClarifyingQuestions,
				DraftComment:        cfg.Claude.Features.DraftComment,
				DraftTicket:         cfg.Claude.Features.DraftTicket,
				BranchPlan:          cfg.Claude.Features.BranchPlan,
				CodeChanges:         cfg.Claude.Features.CodeChanges,
				PRCreation:          cfg.Claude.Features.PRCreation,
				PRReviewResponse:    cfg.Claude.Features.PRReviewResponse,
			},
			Gates: claudeGatesConfig{
				RequireConfirmation: boolPtr(cfg.Claude.Gates.RequireConfirmation),
				AllowJiraWrites:     cfg.Claude.Gates.AllowJiraWrites,
				AllowGitWrites:      cfg.Claude.Gates.AllowGitWrites,
				AllowGitHubWrites:   cfg.Claude.Gates.AllowGitHubWrites,
				AllowCodeEdits:      cfg.Claude.Gates.AllowCodeEdits,
			},
		},
		Notifications: notificationsConfig{
			Enabled:                   boolPtr(cfg.Notifications.Enabled),
			SystemEnabled:             cfg.Notifications.SystemEnabled,
			SystemOnNew:               boolPtr(cfg.Notifications.SystemOnNew),
			SystemOnUpdates:           cfg.Notifications.SystemOnUpdates,
			AutoOpenPanel:             boolPtr(cfg.Notifications.AutoOpenPanel),
			KeepPanelOpenUntilCleared: boolPtr(cfg.Notifications.KeepPanelOpenUntilCleared),
			MaxItems:                  cfg.Notifications.MaxItems,
		},
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open config file: %w", err)
	}
	defer file.Close()

	if err := toml.NewEncoder(file).Encode(fileCfg); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		return fmt.Errorf("secure config file: %w", err)
	}
	return nil
}

func AddSavedView(cfg Config, view IssueView) (Config, error) {
	view.Name = strings.TrimSpace(view.Name)
	view.JQL = strings.TrimSpace(view.JQL)
	if view.Name == "" {
		return Config{}, errors.New("saved view name is required")
	}
	if view.JQL == "" {
		return Config{}, errors.New("saved view JQL is required")
	}
	for _, existing := range cfg.Views {
		if strings.EqualFold(strings.TrimSpace(existing.Name), view.Name) {
			return Config{}, fmt.Errorf("saved view %q already exists", view.Name)
		}
	}
	next := cfg
	next.Views = append(append([]IssueView(nil), cfg.Views...), view)
	return next, nil
}

func SetSavedViews(cfg Config, views []IssueView) (Config, error) {
	if len(views) == 0 {
		return Config{}, errors.New("at least one issue view is required")
	}
	nextViews := make([]IssueView, 0, len(views))
	seen := make(map[string]struct{}, len(views))
	activeFound := false
	for _, view := range views {
		view.Name = strings.TrimSpace(view.Name)
		view.JQL = strings.TrimSpace(view.JQL)
		if view.Name == "" {
			return Config{}, errors.New("issue view name is required")
		}
		if view.JQL == "" {
			return Config{}, errors.New("issue view JQL is required")
		}
		key := strings.ToLower(view.Name)
		if _, ok := seen[key]; ok {
			return Config{}, fmt.Errorf("saved view %q already exists", view.Name)
		}
		seen[key] = struct{}{}
		if strings.EqualFold(view.Name, strings.TrimSpace(cfg.ActiveView)) {
			activeFound = true
		}
		nextViews = append(nextViews, view)
	}
	next := cfg
	next.Views = nextViews
	if !activeFound {
		next.ActiveView = nextViews[0].Name
	}
	return next, nil
}

func Validate(cfg Config) error {
	var problems []string
	if strings.TrimSpace(cfg.BaseURL) == "" {
		problems = append(problems, "Jira base URL is required")
	} else if !strings.HasPrefix(cfg.BaseURL, "https://") {
		problems = append(problems, "Jira base URL must start with https://")
	}
	if strings.TrimSpace(cfg.Email) == "" {
		problems = append(problems, "Jira email is required")
	}
	if strings.TrimSpace(cfg.APIToken) == "" {
		problems = append(problems, "Jira API token is required")
	}
	if strings.TrimSpace(cfg.DefaultProject) == "" {
		problems = append(problems, "default Jira project is required")
	}
	if cfg.DefaultBoardID < 0 {
		problems = append(problems, "default Jira board ID must be zero or greater")
	}
	if cfg.RefreshInterval < 0 {
		problems = append(problems, "refresh interval cannot be negative")
	}
	if cfg.RequestTimeout < 0 {
		problems = append(problems, "request timeout cannot be negative")
	}
	if cfg.WorkerCount <= 0 {
		problems = append(problems, "worker count must be greater than zero")
	}
	if cfg.QueueSize <= 0 {
		problems = append(problems, "queue size must be greater than zero")
	}
	if strings.TrimSpace(cfg.Git.BranchTemplate) == "" {
		problems = append(problems, "git branch_template is required")
	}
	if cfg.Claude.Timeout < 0 {
		problems = append(problems, "Claude timeout cannot be negative")
	}
	if len(cfg.Views) == 0 {
		problems = append(problems, "at least one issue view is required")
	}
	for _, view := range cfg.Views {
		if strings.TrimSpace(view.Name) == "" {
			problems = append(problems, "issue view name is required")
		}
		if strings.TrimSpace(view.JQL) == "" {
			problems = append(problems, "issue view JQL is required")
		}
	}
	if strings.TrimSpace(cfg.Theme.Name) != "" {
		if _, _, ok := BuiltInTheme(cfg.Theme.Name); !ok {
			problems = append(problems, "appearance theme must be one of "+strings.Join(BuiltInThemeNames(), ", "))
		}
	}
	for name, value := range cfg.Theme.colorValues() {
		if !validHexColor(value) {
			problems = append(problems, fmt.Sprintf("%s color must be a hex color like #7DD3FC", name))
		}
	}
	if !validSymbolMode(cfg.Display.SymbolMode) {
		problems = append(problems, "display symbol_mode must be one of auto, plain, symbols, emoji, or nerd")
	}
	if cfg.Notifications.MaxItems <= 0 {
		problems = append(problems, "notifications max_items must be greater than zero")
	}
	if len(problems) > 0 {
		return ValidationError{Problems: problems}
	}
	return nil
}

type fileConfig struct {
	Version       int                      `toml:"version"`
	ActiveProfile string                   `toml:"active_profile"`
	Profiles      map[string]profileConfig `toml:"profiles"`
	Queries       queriesConfig            `toml:"queries"`
	Views         viewsConfig              `toml:"views"`
	Appearance    appearanceConfig         `toml:"appearance"`
	Display       displayConfig            `toml:"display"`
	Runtime       runtimeConfig            `toml:"runtime"`
	Git           gitConfig                `toml:"git"`
	Claude        claudeConfig             `toml:"claude"`
	Notifications notificationsConfig      `toml:"notifications"`
}

type profileConfig struct {
	BaseURL        string `toml:"base_url"`
	Email          string `toml:"email"`
	APIToken       string `toml:"api_token"`
	APITokenSource string `toml:"api_token_source"`
}

type queriesConfig struct {
	DefaultProject string `toml:"default_project"`
	DefaultBoardID int    `toml:"default_board_id"`
	DefaultJQL     string `toml:"default_jql"`
}

type viewsConfig struct {
	Active string       `toml:"active"`
	Saved  []viewConfig `toml:"saved"`
}

type viewConfig struct {
	Name            string `toml:"name"`
	JQL             string `toml:"jql"`
	IncludeChildren bool   `toml:"include_children"`
}

type appearanceConfig struct {
	Theme     string `toml:"theme"`
	Primary   string `toml:"primary"`
	Secondary string `toml:"secondary"`
	Accent    string `toml:"accent"`
	Success   string `toml:"success"`
	Warning   string `toml:"warning"`
	Error     string `toml:"error"`
	Muted     string `toml:"muted"`
	Border    string `toml:"border"`
	Surface   string `toml:"surface"`
	Text      string `toml:"text"`
}

type displayConfig struct {
	SymbolMode string `toml:"symbol_mode"`
}

type runtimeConfig struct {
	RefreshInterval string `toml:"refresh_interval"`
	RequestTimeout  string `toml:"request_timeout"`
	Workers         int    `toml:"workers"`
	QueueSize       int    `toml:"queue_size"`
}

type gitConfig struct {
	BranchTemplate string `toml:"branch_template"`
}

type claudeConfig struct {
	Enabled  bool                 `toml:"enabled"`
	Command  string               `toml:"command"`
	Timeout  string               `toml:"timeout"`
	Features claudeFeaturesConfig `toml:"features"`
	Gates    claudeGatesConfig    `toml:"gates"`
}

type claudeFeaturesConfig struct {
	TicketPlan          bool `toml:"ticket_plan"`
	TicketAssist        bool `toml:"ticket_assist"`
	ClarifyingQuestions bool `toml:"clarifying_questions"`
	DraftComment        bool `toml:"draft_comment"`
	DraftTicket         bool `toml:"draft_ticket"`
	BranchPlan          bool `toml:"branch_plan"`
	CodeChanges         bool `toml:"code_changes"`
	PRCreation          bool `toml:"pr_creation"`
	PRReviewResponse    bool `toml:"pr_review_response"`
}

type claudeGatesConfig struct {
	RequireConfirmation *bool `toml:"require_confirmation"`
	AllowJiraWrites     bool  `toml:"allow_jira_writes"`
	AllowGitWrites      bool  `toml:"allow_git_writes"`
	AllowGitHubWrites   bool  `toml:"allow_github_writes"`
	AllowCodeEdits      bool  `toml:"allow_code_edits"`
}

type notificationsConfig struct {
	Enabled                   *bool `toml:"enabled"`
	SystemEnabled             bool  `toml:"system_enabled"`
	SystemOnNew               *bool `toml:"system_on_new"`
	SystemOnUpdates           bool  `toml:"system_on_updates"`
	AutoOpenPanel             *bool `toml:"auto_open_panel"`
	KeepPanelOpenUntilCleared *bool `toml:"keep_panel_open_until_cleared"`
	MaxItems                  int   `toml:"max_items"`
}

func readFile(path string) (fileConfig, error) {
	var fileCfg fileConfig
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return fileCfg, nil
	}
	if err != nil {
		return fileCfg, fmt.Errorf("stat config file: %w", err)
	}
	if _, err := toml.DecodeFile(path, &fileCfg); err != nil {
		return fileCfg, fmt.Errorf("read config file: %w", err)
	}
	return fileCfg, nil
}

func applyFile(cfg *Config, fileCfg fileConfig, requestedProfile string, store secretstore.Store) error {
	savedProfile := normalizeProfileName(fileCfg.ActiveProfile)
	profileName := strings.TrimSpace(requestedProfile)
	if profileName == "" {
		profileName = savedProfile
	}
	profileName = normalizeProfileName(profileName)
	cfg.ActiveProfile = profileName
	cfg.Profiles = profilesFromConfig(fileCfg.Profiles)
	if err := resolveProfileSecrets(context.Background(), cfg.Profiles, store); err != nil {
		return err
	}
	if len(cfg.Profiles) > 0 {
		profile, ok := cfg.Profiles[profileName]
		if !ok {
			return fmt.Errorf("profile %q is not defined", profileName)
		}
		cfg.BaseURL = profile.BaseURL
		cfg.Email = profile.Email
		cfg.APIToken = profile.APIToken
		cfg.APITokenSource = profile.APITokenSource
	}
	if strings.TrimSpace(fileCfg.Queries.DefaultJQL) != "" {
		cfg.DefaultJQL = strings.TrimSpace(fileCfg.Queries.DefaultJQL)
	}
	if strings.TrimSpace(fileCfg.Queries.DefaultProject) != "" {
		cfg.DefaultProject = strings.TrimSpace(fileCfg.Queries.DefaultProject)
		if fileCfg.Queries.DefaultJQL == "" {
			cfg.DefaultJQL = DefaultJQLForProject(cfg.DefaultProject)
		}
	}
	if fileCfg.Queries.DefaultBoardID > 0 {
		cfg.DefaultBoardID = fileCfg.Queries.DefaultBoardID
	}
	if strings.TrimSpace(fileCfg.Views.Active) != "" {
		cfg.ActiveView = strings.TrimSpace(fileCfg.Views.Active)
	}
	if len(fileCfg.Views.Saved) > 0 {
		cfg.Views = issueViews(fileCfg.Views.Saved)
	}
	themeSymbolMode, err := applyAppearance(&cfg.Theme, fileCfg.Appearance)
	if err != nil {
		return err
	}
	if themeSymbolMode != "" && strings.TrimSpace(fileCfg.Display.SymbolMode) == "" {
		cfg.Display.SymbolMode = themeSymbolMode
	}
	applyDisplay(&cfg.Display, fileCfg.Display)
	if strings.TrimSpace(fileCfg.Runtime.RefreshInterval) != "" {
		duration, err := parseDuration("refresh interval", fileCfg.Runtime.RefreshInterval)
		if err != nil {
			return err
		}
		cfg.RefreshInterval = duration
	}
	if strings.TrimSpace(fileCfg.Runtime.RequestTimeout) != "" {
		duration, err := parseDuration("request timeout", fileCfg.Runtime.RequestTimeout)
		if err != nil {
			return err
		}
		cfg.RequestTimeout = duration
	}
	if fileCfg.Runtime.Workers != 0 {
		cfg.WorkerCount = fileCfg.Runtime.Workers
	}
	if fileCfg.Runtime.QueueSize != 0 {
		cfg.QueueSize = fileCfg.Runtime.QueueSize
	}
	if strings.TrimSpace(fileCfg.Git.BranchTemplate) != "" {
		cfg.Git.BranchTemplate = strings.TrimSpace(fileCfg.Git.BranchTemplate)
	}
	cfg.Claude.Enabled = fileCfg.Claude.Enabled
	if strings.TrimSpace(fileCfg.Claude.Command) != "" {
		cfg.Claude.Command = strings.TrimSpace(fileCfg.Claude.Command)
	}
	if strings.TrimSpace(fileCfg.Claude.Timeout) != "" {
		duration, err := parseDuration("Claude timeout", fileCfg.Claude.Timeout)
		if err != nil {
			return err
		}
		cfg.Claude.Timeout = duration
	}
	cfg.Claude.Features = ClaudeFeatures{
		TicketPlan:          fileCfg.Claude.Features.TicketPlan,
		TicketAssist:        fileCfg.Claude.Features.TicketAssist,
		ClarifyingQuestions: fileCfg.Claude.Features.ClarifyingQuestions,
		DraftComment:        fileCfg.Claude.Features.DraftComment,
		DraftTicket:         fileCfg.Claude.Features.DraftTicket,
		BranchPlan:          fileCfg.Claude.Features.BranchPlan,
		CodeChanges:         fileCfg.Claude.Features.CodeChanges,
		PRCreation:          fileCfg.Claude.Features.PRCreation,
		PRReviewResponse:    fileCfg.Claude.Features.PRReviewResponse,
	}
	if fileCfg.Claude.Gates.RequireConfirmation != nil {
		cfg.Claude.Gates.RequireConfirmation = *fileCfg.Claude.Gates.RequireConfirmation
	}
	cfg.Claude.Gates.AllowJiraWrites = fileCfg.Claude.Gates.AllowJiraWrites
	cfg.Claude.Gates.AllowGitWrites = fileCfg.Claude.Gates.AllowGitWrites
	cfg.Claude.Gates.AllowGitHubWrites = fileCfg.Claude.Gates.AllowGitHubWrites
	cfg.Claude.Gates.AllowCodeEdits = fileCfg.Claude.Gates.AllowCodeEdits
	applyNotifications(&cfg.Notifications, fileCfg.Notifications)
	return nil
}

func boolPtr(value bool) *bool {
	return &value
}

func normalizeProfileName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "default"
	}
	return value
}

func profilesFromConfig(configs map[string]profileConfig) map[string]Profile {
	if len(configs) == 0 {
		return nil
	}
	profiles := make(map[string]Profile, len(configs))
	for name, profile := range configs {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		token := strings.TrimSpace(profile.APIToken)
		profiles[name] = Profile{
			BaseURL:        normalizeBaseURL(profile.BaseURL),
			Email:          strings.TrimSpace(profile.Email),
			APIToken:       token,
			APITokenSource: normalizeLoadedTokenSource(profile.APITokenSource, token),
		}
	}
	return profiles
}

func profilesForSave(cfg Config, activeProfile string) map[string]Profile {
	profiles := make(map[string]Profile, len(cfg.Profiles)+1)
	for name, profile := range cfg.Profiles {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		profiles[name] = Profile{
			BaseURL:        normalizeBaseURL(profile.BaseURL),
			Email:          strings.TrimSpace(profile.Email),
			APIToken:       strings.TrimSpace(profile.APIToken),
			APITokenSource: normalizeTokenSource(profile.APITokenSource),
		}
	}
	profiles[activeProfile] = Profile{
		BaseURL:        normalizeBaseURL(cfg.BaseURL),
		Email:          strings.TrimSpace(cfg.Email),
		APIToken:       strings.TrimSpace(cfg.APIToken),
		APITokenSource: normalizeTokenSource(cfg.APITokenSource),
	}
	return profiles
}

func profileConfigsForSave(ctx context.Context, store secretstore.Store, profiles map[string]Profile) (map[string]profileConfig, error) {
	if len(profiles) == 0 {
		return nil, nil
	}
	configs := make(map[string]profileConfig, len(profiles))
	for name, profile := range profiles {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		tokenSource := normalizeTokenSource(profile.APITokenSource)
		apiToken := strings.TrimSpace(profile.APIToken)
		if tokenSource != tokenSourcePlaintext && apiToken != "" {
			account := secretstore.AccountKey(name, normalizeBaseURL(profile.BaseURL), profile.Email)
			if err := store.Set(ctx, account, apiToken); err != nil {
				return nil, fmt.Errorf("store Jira API token for profile %q in OS keychain: %w", name, err)
			}
			tokenSource = tokenSourceKeyring
			apiToken = ""
		}
		configs[name] = profileConfig{
			BaseURL:        normalizeBaseURL(profile.BaseURL),
			Email:          strings.TrimSpace(profile.Email),
			APIToken:       apiToken,
			APITokenSource: tokenSource,
		}
	}
	return configs, nil
}

func resolveProfileSecrets(ctx context.Context, profiles map[string]Profile, store secretstore.Store) error {
	if len(profiles) == 0 {
		return nil
	}
	for name, profile := range profiles {
		if normalizeTokenSource(profile.APITokenSource) != tokenSourceKeyring {
			continue
		}
		account := secretstore.AccountKey(name, profile.BaseURL, profile.Email)
		token, err := store.Get(ctx, account)
		if errors.Is(err, secretstore.ErrNotFound) {
			profile.APIToken = ""
			profiles[name] = profile
			continue
		}
		if err != nil {
			return fmt.Errorf("load Jira API token for profile %q from OS keychain: %w", name, err)
		}
		profile.APIToken = token
		profiles[name] = profile
	}
	return nil
}

func secretStoreOrDefault(store secretstore.Store) secretstore.Store {
	if store != nil {
		return store
	}
	defaultSecretStore.Lock()
	defer defaultSecretStore.Unlock()
	return defaultSecretStore.new()
}

func SetDefaultSecretStoreForTest(store secretstore.Store) func() {
	defaultSecretStore.Lock()
	previous := defaultSecretStore.new
	defaultSecretStore.new = func() secretstore.Store { return store }
	defaultSecretStore.Unlock()
	return func() {
		defaultSecretStore.Lock()
		defaultSecretStore.new = previous
		defaultSecretStore.Unlock()
	}
}

func normalizeTokenSource(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case tokenSourcePlaintext:
		return tokenSourcePlaintext
	default:
		return tokenSourceKeyring
	}
}

func normalizeLoadedTokenSource(value string, token string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case tokenSourceKeyring:
		return tokenSourceKeyring
	case tokenSourcePlaintext:
		return tokenSourcePlaintext
	default:
		if strings.TrimSpace(token) != "" {
			return tokenSourcePlaintext
		}
		return tokenSourceKeyring
	}
}

func parsePositiveInt(name string, value string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", name, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", name)
	}
	return parsed, nil
}

func parseDuration(name, value string) (time.Duration, error) {
	duration, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid Go duration, for example 30s or 2m: %w", name, err)
	}
	if duration < 0 {
		return 0, fmt.Errorf("%s cannot be negative", name)
	}
	return duration, nil
}

func normalizeBaseURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func DefaultJQLForProject(project string) string {
	return fmt.Sprintf("project = %s AND assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC", strings.TrimSpace(project))
}

func DefaultViews(project string) []IssueView {
	project = strings.TrimSpace(project)
	if project == "" {
		return nil
	}
	return []IssueView{
		{
			Name: "Assigned",
			JQL:  fmt.Sprintf("project = %s AND assignee = currentUser() AND resolution = Unresolved ORDER BY updated DESC", project),
		},
		{
			Name: "Created/Reported",
			JQL:  fmt.Sprintf("project = %s AND (creator = currentUser() OR reporter = currentUser()) AND resolution = Unresolved ORDER BY updated DESC", project),
		},
		{
			Name: "Project Open",
			JQL:  fmt.Sprintf("project = %s AND resolution = Unresolved ORDER BY updated DESC", project),
		},
		{
			Name: "Current Sprint",
			JQL:  fmt.Sprintf("project = %s AND sprint in openSprints() AND resolution = Unresolved ORDER BY assignee ASC, status ASC, priority DESC", project),
		},
		{
			Name: "Watching",
			JQL:  fmt.Sprintf("project = %s AND watcher = currentUser() AND resolution = Unresolved ORDER BY updated DESC", project),
		},
		{
			Name:            "Epics",
			JQL:             fmt.Sprintf("project = %s AND issuetype = Epic AND resolution = Unresolved ORDER BY updated DESC", project),
			IncludeChildren: true,
		},
	}
}

func ensureViews(cfg *Config) {
	if cfg.DefaultProject != "" && (cfg.DefaultJQL == "" || cfg.DefaultJQL == defaultJQL) {
		cfg.DefaultJQL = DefaultJQLForProject(cfg.DefaultProject)
	}
	if len(cfg.Views) == 0 {
		cfg.Views = DefaultViews(cfg.DefaultProject)
	}
	if cfg.ActiveView == "" && len(cfg.Views) > 0 {
		cfg.ActiveView = cfg.Views[0].Name
	}
	if cfg.DefaultJQL != "" && len(cfg.Views) == 0 {
		cfg.Views = []IssueView{{Name: "Default", JQL: cfg.DefaultJQL}}
		cfg.ActiveView = "Default"
	}
}

func viewConfigs(views []IssueView) []viewConfig {
	configs := make([]viewConfig, 0, len(views))
	for _, view := range views {
		configs = append(configs, viewConfig{
			Name:            strings.TrimSpace(view.Name),
			JQL:             strings.TrimSpace(view.JQL),
			IncludeChildren: view.IncludeChildren,
		})
	}
	return configs
}

func issueViews(configs []viewConfig) []IssueView {
	views := make([]IssueView, 0, len(configs))
	for _, view := range configs {
		views = append(views, IssueView{
			Name:            strings.TrimSpace(view.Name),
			JQL:             strings.TrimSpace(view.JQL),
			IncludeChildren: view.IncludeChildren,
		})
	}
	return views
}

func applyAppearance(theme *Theme, appearance appearanceConfig) (string, error) {
	symbolMode := ""
	if strings.TrimSpace(appearance.Theme) != "" {
		skin, skinSymbolMode, ok := BuiltInTheme(appearance.Theme)
		if !ok {
			return "", ValidationError{Problems: []string{"appearance theme must be one of " + strings.Join(BuiltInThemeNames(), ", ")}}
		}
		*theme = skin
		symbolMode = skinSymbolMode
	}
	if strings.TrimSpace(appearance.Primary) != "" {
		theme.Primary = strings.TrimSpace(appearance.Primary)
	}
	if strings.TrimSpace(appearance.Secondary) != "" {
		theme.Secondary = strings.TrimSpace(appearance.Secondary)
	}
	if strings.TrimSpace(appearance.Accent) != "" {
		theme.Accent = strings.TrimSpace(appearance.Accent)
	}
	if strings.TrimSpace(appearance.Success) != "" {
		theme.Success = strings.TrimSpace(appearance.Success)
	}
	if strings.TrimSpace(appearance.Warning) != "" {
		theme.Warning = strings.TrimSpace(appearance.Warning)
	}
	if strings.TrimSpace(appearance.Error) != "" {
		theme.Error = strings.TrimSpace(appearance.Error)
	}
	if strings.TrimSpace(appearance.Muted) != "" {
		theme.Muted = strings.TrimSpace(appearance.Muted)
	}
	if strings.TrimSpace(appearance.Border) != "" {
		theme.Border = strings.TrimSpace(appearance.Border)
	}
	if strings.TrimSpace(appearance.Surface) != "" {
		theme.Surface = strings.TrimSpace(appearance.Surface)
	}
	if strings.TrimSpace(appearance.Text) != "" {
		theme.Text = strings.TrimSpace(appearance.Text)
	}
	return symbolMode, nil
}

func applyDisplay(display *Display, cfg displayConfig) {
	if strings.TrimSpace(cfg.SymbolMode) != "" {
		display.SymbolMode = strings.ToLower(strings.TrimSpace(cfg.SymbolMode))
	}
}

func applyNotifications(notifications *Notifications, cfg notificationsConfig) {
	if cfg.Enabled != nil {
		notifications.Enabled = *cfg.Enabled
	}
	notifications.SystemEnabled = cfg.SystemEnabled
	if cfg.SystemOnNew != nil {
		notifications.SystemOnNew = *cfg.SystemOnNew
	}
	notifications.SystemOnUpdates = cfg.SystemOnUpdates
	if cfg.AutoOpenPanel != nil {
		notifications.AutoOpenPanel = *cfg.AutoOpenPanel
	}
	if cfg.KeepPanelOpenUntilCleared != nil {
		notifications.KeepPanelOpenUntilCleared = *cfg.KeepPanelOpenUntilCleared
	}
	if cfg.MaxItems != 0 {
		notifications.MaxItems = cfg.MaxItems
	}
}

func (t Theme) colorValues() map[string]string {
	return map[string]string{
		"primary":   t.Primary,
		"secondary": t.Secondary,
		"accent":    t.Accent,
		"success":   t.Success,
		"warning":   t.Warning,
		"error":     t.Error,
		"muted":     t.Muted,
		"border":    t.Border,
		"surface":   t.Surface,
		"text":      t.Text,
	}
}

func appearanceConfigFromTheme(theme Theme) appearanceConfig {
	name := normalizeThemeName(theme.Name)
	if name == "" {
		name = "default"
	}
	return appearanceConfig{
		Theme:     name,
		Primary:   theme.Primary,
		Secondary: theme.Secondary,
		Accent:    theme.Accent,
		Success:   theme.Success,
		Warning:   theme.Warning,
		Error:     theme.Error,
		Muted:     theme.Muted,
		Border:    theme.Border,
		Surface:   theme.Surface,
		Text:      theme.Text,
	}
}

func validHexColor(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	for _, char := range value[1:] {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') && (char < 'A' || char > 'F') {
			return false
		}
	}
	return true
}

func validSymbolMode(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "auto", "plain", "symbols", "emoji", "nerd":
		return true
	default:
		return false
	}
}
