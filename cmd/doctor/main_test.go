package doctor

import (
	"errors"
	"testing"
	"time"

	"github.com/koda-claw/web-tools/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfig() *config.Config {
	cfg := config.DefaultConfig()
	cfg.Reader.CacheDir = "/tmp/web-tools-test-cache"
	cfg.Reader.MarkitdownPath = "markitdown"
	cfg.Reader.AgentBrowserPath = "agent-browser"
	cfg.Search.SearXNGURL = "http://localhost:8888"
	return &cfg
}

func TestDoctorRun_AllOK(t *testing.T) {
	c := checker{
		loadConfig: func() (*config.Config, error) { return testConfig(), nil },
		checkCache: func(string) error { return nil },
		lookPath: func(name string) (string, error) {
			return "/usr/bin/" + name, nil
		},
		httpHead: func(string, time.Duration) (int, error) { return 200, nil },
	}

	report := c.Run()

	assert.True(t, report.OK)
	assert.Equal(t, StatusOK, report.Checks[0].Status)
	assert.Equal(t, "auto", report.Config.Search.DefaultEngine)
	assert.Equal(t, "auto", report.Config.Search.DefaultProvider)
	assert.Equal(t, "/tmp/web-tools-test-cache", report.Config.Reader.CacheDir)
	assert.Equal(t, StatusOK, findCheck(t, report, "provider.searxng").Status)
	assert.Equal(t, StatusOK, findCheck(t, report, "provider.duckduckgo").Status)
	assert.Equal(t, StatusOK, findCheck(t, report, "provider.builtin-reader").Status)
}

func TestDoctorRun_OptionalDependenciesWarnWithoutFailure(t *testing.T) {
	c := checker{
		loadConfig: func() (*config.Config, error) { return testConfig(), nil },
		checkCache: func(string) error { return nil },
		lookPath: func(name string) (string, error) {
			return "", errors.New("not found")
		},
		httpHead: func(string, time.Duration) (int, error) {
			return 0, errors.New("connection refused")
		},
	}

	report := c.Run()

	assert.True(t, report.OK)
	assert.Equal(t, StatusWarn, findCheck(t, report, "markitdown").Status)
	assert.Equal(t, StatusWarn, findCheck(t, report, "agent-browser").Status)
	assert.Equal(t, StatusWarn, findCheck(t, report, "searxng").Status)
}

func TestDoctorRun_ProviderAuthStatusDoesNotLeakSecret(t *testing.T) {
	t.Setenv("ZHIPU_APIKEY", "super-secret-token")
	cfg := testConfig()
	cfg.Providers["bigmodel"] = config.ProviderConfig{
		Type:         "mcp",
		AuthEnv:      "ZHIPU_APIKEY",
		EnabledIfEnv: "ZHIPU_APIKEY",
		Capabilities: []string{"search", "reader"},
	}
	c := checker{
		loadConfig: func() (*config.Config, error) { return cfg, nil },
		checkCache: func(string) error { return nil },
		lookPath: func(name string) (string, error) {
			return "/usr/bin/" + name, nil
		},
		httpHead: func(string, time.Duration) (int, error) { return 200, nil },
	}

	report := c.Run()
	output := report.RenderJSON()

	check := findCheck(t, report, "provider.bigmodel")
	assert.Equal(t, StatusOK, check.Status)
	assert.Equal(t, "ZHIPU_APIKEY", check.Details["auth_env"])
	assert.Equal(t, "true", check.Details["auth_configured"])
	assert.NotContains(t, output, "super-secret-token")
	assert.True(t, report.Config.Providers["bigmodel"].AuthConfigured)
}

func TestDoctorRun_ConfigFailureIsError(t *testing.T) {
	c := checker{
		loadConfig: func() (*config.Config, error) { return nil, errors.New("bad config") },
	}

	report := c.Run()

	assert.False(t, report.OK)
	require.Len(t, report.Checks, 1)
	assert.Equal(t, "config", report.Checks[0].Name)
	assert.Equal(t, StatusError, report.Checks[0].Status)
}

func TestDoctorRun_CacheFailureIsError(t *testing.T) {
	c := checker{
		loadConfig: func() (*config.Config, error) { return testConfig(), nil },
		checkCache: func(string) error { return errors.New("permission denied") },
		lookPath: func(name string) (string, error) {
			return "/usr/bin/" + name, nil
		},
		httpHead: func(string, time.Duration) (int, error) { return 200, nil },
	}

	report := c.Run()

	assert.False(t, report.OK)
	assert.Equal(t, StatusError, findCheck(t, report, "cache").Status)
}

func TestDoctorRenderJSON(t *testing.T) {
	report := Report{
		OK:     true,
		Checks: []Check{okCheck("config", "configuration loaded", nil)},
		Config: summarizeConfig(*testConfig()),
	}

	output := report.RenderJSON()

	assert.Contains(t, output, `"ok": true`)
	assert.Contains(t, output, `"checks"`)
	assert.Contains(t, output, `"default_engine": "auto"`)
	assert.Contains(t, output, `"default_provider": "auto"`)
}

func TestDoctorRenderText(t *testing.T) {
	report := Report{
		OK:     true,
		Checks: []Check{okCheck("config", "configuration loaded", nil)},
		Config: summarizeConfig(*testConfig()),
	}

	output := report.RenderText()

	assert.Contains(t, output, "web-tools doctor: ok")
	assert.Contains(t, output, "[ok] config: configuration loaded")
	assert.Contains(t, output, "search.default_engine: auto")
	assert.Contains(t, output, "search.default_provider: auto")
	assert.Contains(t, output, "Providers:")
}

func findCheck(t *testing.T, report Report, name string) Check {
	t.Helper()
	for _, check := range report.Checks {
		if check.Name == name {
			return check
		}
	}
	t.Fatalf("missing check %q", name)
	return Check{}
}
