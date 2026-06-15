package setupcheck

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/koda-claw/web-tools/internal/config"
)

const (
	StatusOK    = "ok"
	StatusWarn  = "warn"
	StatusError = "error"
)

// Options controls setup readiness checks.
type Options struct {
	Version  string
	SkillDir string
	Provider string
	AuthEnv  string
}

// Report is a non-sensitive setup readiness report.
type Report struct {
	OK          bool          `json:"ok"`
	Version     string        `json:"version"`
	Skill       SkillStatus   `json:"skill"`
	Config      ConfigStatus  `json:"config"`
	EnvFile     EnvStatus     `json:"env_file"`
	Provider    ProviderState `json:"provider"`
	ReaderAuto  ChainStatus   `json:"reader_auto"`
	SearchAuto  ChainStatus   `json:"search_auto"`
	Checks      []Check       `json:"checks"`
	Suggestions []Suggestion  `json:"suggestions,omitempty"`
}

// Check is a single readiness check.
type Check struct {
	Name    string            `json:"name"`
	Status  string            `json:"status"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// Suggestion is a repair command without secret values.
type Suggestion struct {
	ID      string `json:"id"`
	Message string `json:"message"`
	Command string `json:"command"`
}

type SkillStatus struct {
	Path      string `json:"path"`
	Exists    bool   `json:"exists"`
	Installed bool   `json:"installed"`
}

type ConfigStatus struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
}

type EnvStatus struct {
	UserPath       string `json:"user_path"`
	UserExists     bool   `json:"user_exists"`
	UserLoaded     bool   `json:"user_loaded"`
	UserMode       string `json:"user_mode,omitempty"`
	UserPermission string `json:"user_permission"`
	ExplicitPath   string `json:"explicit_path,omitempty"`
	ExplicitExists bool   `json:"explicit_exists,omitempty"`
	ExplicitLoaded bool   `json:"explicit_loaded,omitempty"`
}

type ProviderState struct {
	ID             string   `json:"id"`
	Configured     bool     `json:"configured"`
	Type           string   `json:"type,omitempty"`
	Capabilities   []string `json:"capabilities,omitempty"`
	AuthEnv        string   `json:"auth_env,omitempty"`
	AuthConfigured bool     `json:"auth_configured"`
	Enabled        bool     `json:"enabled"`
}

type ChainStatus struct {
	Chain    []string `json:"chain"`
	Contains bool     `json:"contains"`
}

// Run returns a setup readiness report.
func Run(opts Options) Report {
	if opts.Provider == "" {
		opts.Provider = "bigmodel"
	}
	if opts.AuthEnv == "" {
		opts.AuthEnv = "ZHIPU_APIKEY"
	}
	if opts.SkillDir == "" {
		opts.SkillDir = "~/.codex/skills"
	}

	cfg, cfgErr := config.Load()
	envReport := config.LoadEnvFiles()
	report := Report{
		Version: opts.Version,
		Skill:   inspectSkill(opts.SkillDir),
		Config:  inspectConfig(),
		EnvFile: EnvStatus{
			UserPath:       envReport.User.Path,
			UserExists:     envReport.User.Exists,
			UserLoaded:     envReport.User.Loaded,
			UserPermission: permissionStatus(envReport.User),
			ExplicitPath:   envReport.Explicit.Path,
			ExplicitExists: envReport.Explicit.Exists,
			ExplicitLoaded: envReport.Explicit.Loaded,
		},
		Provider: ProviderState{ID: opts.Provider, AuthEnv: opts.AuthEnv},
	}
	if envReport.User.Mode != 0 {
		report.EnvFile.UserMode = envReport.User.Mode.String()
	}

	report.addCheck("skill", report.Skill.Installed, "skill installed", "skill is missing", map[string]string{"path": report.Skill.Path})
	report.addCheck("config", report.Config.Exists, "user config exists", "user config is missing", map[string]string{"path": report.Config.Path})
	if envReport.User.Exists {
		if envReport.User.OverPermissive {
			report.Checks = append(report.Checks, Check{Name: "env_file", Status: StatusWarn, Message: "env file permissions are broader than 0600", Details: map[string]string{"path": envReport.User.Path, "mode": envReport.User.Mode.String()}})
		} else {
			report.Checks = append(report.Checks, Check{Name: "env_file", Status: StatusOK, Message: "env file loaded", Details: map[string]string{"path": envReport.User.Path}})
		}
	} else {
		report.Checks = append(report.Checks, Check{Name: "env_file", Status: StatusWarn, Message: "user env file is missing", Details: map[string]string{"path": envReport.User.Path}})
	}

	if cfgErr != nil {
		report.Checks = append(report.Checks, Check{Name: "load_config", Status: StatusError, Message: "configuration failed to load", Details: map[string]string{"error": cfgErr.Error()}})
	} else {
		fillConfigState(&report, cfg, opts)
	}
	report.Suggestions = suggestions(report)
	report.OK = reportOK(report.Checks)
	return report
}

func inspectSkill(skillDir string) SkillStatus {
	root := config.ExpandHome(skillDir)
	path := filepath.Join(root, "web-tools", "SKILL.md")
	_, err := os.Stat(path)
	exists := err == nil
	return SkillStatus{Path: path, Exists: exists, Installed: exists}
}

func inspectConfig() ConfigStatus {
	path := config.UserConfigPath()
	_, err := os.Stat(path)
	return ConfigStatus{Path: path, Exists: err == nil}
}

func fillConfigState(report *Report, cfg *config.Config, opts Options) {
	provider, ok := cfg.Providers[opts.Provider]
	report.Provider.Configured = ok
	if ok {
		report.Provider.Type = provider.Type
		report.Provider.Capabilities = append([]string(nil), provider.Capabilities...)
		report.Provider.AuthEnv = provider.AuthEnv
		report.Provider.AuthConfigured = provider.AuthEnv != "" && os.Getenv(provider.AuthEnv) != ""
		report.Provider.Enabled = provider.EnabledIfEnv == "" || os.Getenv(provider.EnabledIfEnv) != ""
	}
	report.ReaderAuto = ChainStatus{
		Chain:    append([]string(nil), cfg.Reader.DefaultProviderChain...),
		Contains: contains(cfg.Reader.DefaultProviderChain, opts.Provider),
	}
	report.SearchAuto = ChainStatus{
		Chain:    append([]string(nil), cfg.Search.DefaultProviderChain...),
		Contains: contains(cfg.Search.DefaultProviderChain, opts.Provider),
	}
	report.addCheck("provider", ok, "provider configured", "provider is missing", map[string]string{"provider": opts.Provider})
	if ok && report.Provider.AuthEnv != "" {
		report.addCheck("provider_auth", report.Provider.AuthConfigured, "provider auth configured", "provider auth is missing", map[string]string{"auth_env": report.Provider.AuthEnv})
	}
	if ok && report.Provider.AuthConfigured && !report.ReaderAuto.Contains {
		report.Checks = append(report.Checks, Check{Name: "reader_auto", Status: StatusWarn, Message: "reader auto fallback is not enabled", Details: map[string]string{"provider": opts.Provider}})
	}
}

func (r *Report) addCheck(name string, ok bool, okMessage string, warnMessage string, details map[string]string) {
	status := StatusOK
	message := okMessage
	if !ok {
		status = StatusWarn
		message = warnMessage
	}
	r.Checks = append(r.Checks, Check{Name: name, Status: status, Message: message, Details: details})
}

func suggestions(report Report) []Suggestion {
	out := []Suggestion{}
	if !report.Skill.Installed {
		out = append(out, Suggestion{ID: "install_skill", Message: "Install or update the Agent skill", Command: "web-tools skill install --force"})
	}
	if !report.Provider.Configured {
		out = append(out, Suggestion{ID: "configure_provider", Message: "Configure BigModel provider", Command: "web-tools setup --provider bigmodel --auth-env ZHIPU_APIKEY --skip-doctor"})
	}
	if report.Provider.Configured && report.Provider.AuthEnv != "" && !report.Provider.AuthConfigured {
		out = append(out, Suggestion{ID: "configure_auth", Message: "Store provider auth in user env file", Command: fmt.Sprintf("web-tools setup --provider %s --auth-env %s --set-env %s=<redacted> --skip-doctor", report.Provider.ID, report.Provider.AuthEnv, report.Provider.AuthEnv)})
	}
	if report.Provider.Configured && report.Provider.AuthConfigured && !report.ReaderAuto.Contains {
		out = append(out, Suggestion{ID: "enable_reader_auto", Message: "Enable BigModel as reader fallback after privacy/cost confirmation", Command: fmt.Sprintf("web-tools setup --provider %s --auth-env %s --enable-reader-auto --skip-doctor", report.Provider.ID, report.Provider.AuthEnv)})
	}
	return out
}

func permissionStatus(status config.EnvFileStatus) string {
	if !status.Exists {
		return "missing"
	}
	if status.OverPermissive {
		return "warn"
	}
	return "ok"
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func reportOK(checks []Check) bool {
	for _, check := range checks {
		if check.Status == StatusError {
			return false
		}
	}
	return true
}

// RenderText renders a concise human-readable report.
func (r Report) RenderText() string {
	var sb strings.Builder
	sb.WriteString("web-tools setup check\n\n")
	for _, check := range r.Checks {
		sb.WriteString(fmt.Sprintf("[%s] %s: %s\n", check.Status, check.Name, check.Message))
	}
	if len(r.Suggestions) > 0 {
		sb.WriteString("\nSuggestions:\n")
		for _, suggestion := range r.Suggestions {
			sb.WriteString(fmt.Sprintf("- %s\n  Run: %s\n", suggestion.Message, suggestion.Command))
		}
	}
	return sb.String()
}
