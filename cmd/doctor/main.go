package doctor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/koda-claw/web-tools/internal/config"
	"github.com/spf13/cobra"
)

const (
	StatusOK    = "ok"
	StatusWarn  = "warn"
	StatusError = "error"
)

type Check struct {
	Name    string            `json:"name"`
	Status  string            `json:"status"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

type ConfigSummary struct {
	Reader ReaderSummary `json:"reader"`
	Search SearchSummary `json:"search"`
}

type ReaderSummary struct {
	CacheDir         string `json:"cache_dir"`
	CacheTTL         int    `json:"cache_ttl"`
	DefaultTimeout   int    `json:"default_timeout"`
	BrowserFallback  bool   `json:"browser_fallback"`
	MarkitdownPath   string `json:"markitdown_path"`
	AgentBrowserPath string `json:"agent_browser_path"`
	MinContentLength int    `json:"min_content_length"`
}

type SearchSummary struct {
	SearXNGURL    string `json:"searxng_url"`
	DefaultLimit  int    `json:"default_limit"`
	DefaultLocale string `json:"default_locale"`
	DefaultEngine string `json:"default_engine"`
}

type Report struct {
	OK     bool          `json:"ok"`
	Checks []Check       `json:"checks"`
	Config ConfigSummary `json:"config"`
}

type checker struct {
	lookPath   func(string) (string, error)
	httpHead   func(string, time.Duration) (int, error)
	checkCache func(string) error
	loadConfig func() (*config.Config, error)
}

func Cmd() *cobra.Command {
	var flagJSON bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check local web-tools configuration and optional dependencies",
		Long: `Check web-tools configuration, cache directory access, optional reader tools,
and the optional SearXNG search backend. Missing optional dependencies are reported
as warnings instead of hard failures.`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			report := defaultChecker().Run()
			if flagJSON {
				fmt.Println(report.RenderJSON())
			} else {
				fmt.Print(report.RenderText())
			}
			if !report.OK {
				os.Exit(1)
			}
		},
	}

	cmd.Flags().BoolVar(&flagJSON, "json", false, "JSON structured output")
	return cmd
}

func defaultChecker() checker {
	return checker{
		lookPath: func(name string) (string, error) {
			path := config.FindExecutable(name)
			if path == "" {
				return "", fmt.Errorf("%s not found", name)
			}
			return path, nil
		},
		httpHead: func(rawURL string, timeout time.Duration) (int, error) {
			client := &http.Client{Timeout: timeout}
			req, err := http.NewRequest(http.MethodHead, rawURL, nil)
			if err != nil {
				return 0, err
			}
			resp, err := client.Do(req)
			if err != nil {
				return 0, err
			}
			defer resp.Body.Close()
			return resp.StatusCode, nil
		},
		checkCache: func(dir string) error {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}
			probe := filepath.Join(dir, ".web-tools-doctor")
			if err := os.WriteFile(probe, []byte("ok"), 0644); err != nil {
				return err
			}
			return os.Remove(probe)
		},
		loadConfig: config.Load,
	}
}

func (c checker) Run() Report {
	cfg, err := c.loadConfig()
	if err != nil {
		report := Report{
			OK:     false,
			Checks: []Check{errorCheck("config", "configuration failed to load", err, nil)},
		}
		return report
	}

	checks := []Check{
		okCheck("config", "configuration loaded", nil),
		c.cacheCheck(cfg.Reader.CacheDir),
		c.executableCheck("markitdown", cfg.Reader.MarkitdownPath, "optional file conversion dependency"),
		c.executableCheck("agent-browser", cfg.Reader.AgentBrowserPath, "optional browser fallback dependency"),
		c.searxngCheck(cfg.Search.SearXNGURL),
	}

	return Report{
		OK:     allRequiredChecksOK(checks),
		Checks: checks,
		Config: summarizeConfig(*cfg),
	}
}

func (c checker) cacheCheck(dir string) Check {
	if err := c.checkCache(dir); err != nil {
		return errorCheck("cache", "cache directory is not writable", err, map[string]string{"path": dir})
	}
	return okCheck("cache", "cache directory is writable", map[string]string{"path": dir})
}

func (c checker) executableCheck(name string, path string, message string) Check {
	if path == "" {
		path = name
	}
	resolved, err := c.lookPath(path)
	if err != nil {
		return warnCheck(name, message+" is missing", map[string]string{"path": path})
	}
	return okCheck(name, message+" is available", map[string]string{"path": path, "resolved": resolved})
}

func (c checker) searxngCheck(rawURL string) Check {
	statusCode, err := c.httpHead(rawURL, config.HealthCheckTimeout)
	if err != nil {
		return warnCheck("searxng", "optional SearXNG backend is unreachable", map[string]string{
			"url":   rawURL,
			"error": err.Error(),
		})
	}
	if statusCode >= 500 {
		return warnCheck("searxng", "optional SearXNG backend returned a server error", map[string]string{
			"url":         rawURL,
			"status_code": fmt.Sprintf("%d", statusCode),
		})
	}
	return okCheck("searxng", "optional SearXNG backend is reachable", map[string]string{
		"url":         rawURL,
		"status_code": fmt.Sprintf("%d", statusCode),
	})
}

func allRequiredChecksOK(checks []Check) bool {
	for _, check := range checks {
		if check.Status == StatusError {
			return false
		}
	}
	return true
}

func okCheck(name string, message string, details map[string]string) Check {
	return Check{Name: name, Status: StatusOK, Message: message, Details: details}
}

func warnCheck(name string, message string, details map[string]string) Check {
	return Check{Name: name, Status: StatusWarn, Message: message, Details: details}
}

func errorCheck(name string, message string, err error, details map[string]string) Check {
	if details == nil {
		details = map[string]string{}
	}
	if err != nil {
		details["error"] = err.Error()
	}
	return Check{Name: name, Status: StatusError, Message: message, Details: details}
}

func summarizeConfig(cfg config.Config) ConfigSummary {
	return ConfigSummary{
		Reader: ReaderSummary{
			CacheDir:         cfg.Reader.CacheDir,
			CacheTTL:         cfg.Reader.CacheTTL,
			DefaultTimeout:   cfg.Reader.DefaultTimeout,
			BrowserFallback:  cfg.Reader.BrowserFallback,
			MarkitdownPath:   cfg.Reader.MarkitdownPath,
			AgentBrowserPath: cfg.Reader.AgentBrowserPath,
			MinContentLength: cfg.Reader.MinContentLength,
		},
		Search: SearchSummary{
			SearXNGURL:    cfg.Search.SearXNGURL,
			DefaultLimit:  cfg.Search.DefaultLimit,
			DefaultLocale: cfg.Search.DefaultLocale,
			DefaultEngine: cfg.Search.DefaultEngine,
		},
	}
}

func (r Report) RenderJSON() string {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		data, _ = json.Marshal(r)
	}
	return string(data)
}

func (r Report) RenderText() string {
	var sb strings.Builder
	if r.OK {
		sb.WriteString("web-tools doctor: ok\n\n")
	} else {
		sb.WriteString("web-tools doctor: errors found\n\n")
	}
	for _, check := range r.Checks {
		sb.WriteString(fmt.Sprintf("[%s] %s: %s\n", check.Status, check.Name, check.Message))
	}
	sb.WriteString("\n")
	sb.WriteString("Config:\n")
	sb.WriteString(fmt.Sprintf("  reader.cache_dir: %s\n", r.Config.Reader.CacheDir))
	sb.WriteString(fmt.Sprintf("  reader.browser_fallback: %t\n", r.Config.Reader.BrowserFallback))
	sb.WriteString(fmt.Sprintf("  search.default_engine: %s\n", r.Config.Search.DefaultEngine))
	sb.WriteString(fmt.Sprintf("  search.searxng_url: %s\n", r.Config.Search.SearXNGURL))
	return sb.String()
}
