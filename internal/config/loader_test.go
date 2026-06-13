package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, DefaultCacheTTL, cfg.Reader.CacheTTL)
	assert.Equal(t, DefaultTimeout, cfg.Reader.DefaultTimeout)
	assert.Equal(t, true, cfg.Reader.BrowserFallback)
	assert.Equal(t, "markitdown", cfg.Reader.MarkitdownPath)
	assert.Equal(t, "agent-browser", cfg.Reader.AgentBrowserPath)
	assert.Equal(t, DefaultMinContentLength, cfg.Reader.MinContentLength)
	assert.Equal(t, DefaultSearXNGURL, cfg.Search.SearXNGURL)
	assert.Equal(t, DefaultSearchLimit, cfg.Search.DefaultLimit)
	assert.Equal(t, "auto", cfg.Search.DefaultLocale)
	assert.Equal(t, "auto", cfg.Search.DefaultEngine)
}

func TestLoad_WithNoConfigFiles(t *testing.T) {
	// In a clean temp dir, Load should return defaults
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	cfg, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, DefaultSearXNGURL, cfg.Search.SearXNGURL)
	assert.Equal(t, DefaultSearchLimit, cfg.Search.DefaultLimit)
}

func TestLoad_WithLocalConfigFile(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	os.WriteFile("web-tools.json", []byte(`{
		"reader": {"browser_fallback": false},
		"search": {
			"searxng_url": "http://custom:9999",
			"default_limit": 10,
			"default_locale": "zh-CN",
			"default_engine": "duckduckgo"
		}
	}`), 0644)

	cfg, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, "http://custom:9999", cfg.Search.SearXNGURL)
	assert.Equal(t, 10, cfg.Search.DefaultLimit)
	assert.Equal(t, "zh-CN", cfg.Search.DefaultLocale)
	assert.Equal(t, "duckduckgo", cfg.Search.DefaultEngine)
	assert.False(t, cfg.Reader.BrowserFallback)
}

func TestLoad_LocalOverridesUser(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	// Create user config dir
	userDir := filepath.Join(dir, ".config", "web-tools")
	os.MkdirAll(userDir, 0755)

	os.WriteFile(filepath.Join(userDir, "config.json"), []byte(`{
		"reader": {"browser_fallback": false},
		"search": {
			"searxng_url": "http://user:8888",
			"default_limit": 3,
			"default_locale": "en-US",
			"default_engine": "duckduckgo"
		}
	}`), 0644)

	// Override HOME to point to temp dir
	t.Setenv("HOME", dir)

	// Create local config that overrides
	os.WriteFile("web-tools.json", []byte(`{
		"search": {
			"searxng_url": "http://local:9999",
			"default_engine": "searxng"
		}
	}`), 0644)

	cfg, err := Load()
	assert.NoError(t, err)
	// Local overrides user
	assert.Equal(t, "http://local:9999", cfg.Search.SearXNGURL)
	// Local only overrides what it sets, user's limit stays
	assert.Equal(t, 3, cfg.Search.DefaultLimit)
	assert.Equal(t, "en-US", cfg.Search.DefaultLocale)
	assert.Equal(t, "searxng", cfg.Search.DefaultEngine)
	assert.False(t, cfg.Reader.BrowserFallback)
}

func TestLoad_EnvOverrides(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	t.Setenv("SEARXNG_URL", "http://env:7777")
	t.Setenv("WEB_READER_CACHE_TTL", "600")

	cfg, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, "http://env:7777", cfg.Search.SearXNGURL)
	assert.Equal(t, 600, cfg.Reader.CacheTTL)
}

func TestLoad_EnvOverridesConfigFile(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	os.WriteFile("web-tools.json", []byte(`{"search":{"searxng_url":"http://file:8888"}}`), 0644)

	t.Setenv("SEARXNG_URL", "http://env:7777")

	cfg, err := Load()
	assert.NoError(t, err)
	// Env overrides file
	assert.Equal(t, "http://env:7777", cfg.Search.SearXNGURL)
}

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	os.WriteFile("web-tools.json", []byte("not json"), 0644)

	cfg, err := Load()
	// Should still work, just skip invalid file
	assert.NoError(t, err)
	assert.Equal(t, DefaultSearXNGURL, cfg.Search.SearXNGURL)
}

func TestLoad_BrowserFallbackOmittedKeepsDefault(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	os.WriteFile("web-tools.json", []byte(`{"reader":{"cache_ttl":120}}`), 0644)

	cfg, err := Load()
	assert.NoError(t, err)
	assert.Equal(t, 120, cfg.Reader.CacheTTL)
	assert.True(t, cfg.Reader.BrowserFallback)
}

func TestLoad_WebToolsConfigOverride(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	os.WriteFile("web-tools.json", []byte(`{
		"reader": {"browser_fallback": true},
		"search": {
			"searxng_url": "http://local:8888",
			"default_engine": "searxng"
		}
	}`), 0644)

	overridePath := filepath.Join(dir, "override.json")
	os.WriteFile(overridePath, []byte(`{
		"reader": {"browser_fallback": false},
		"search": {
			"searxng_url": "http://override:9999",
			"default_limit": 9,
			"default_locale": "zh-CN",
			"default_engine": "duckduckgo"
		}
	}`), 0644)
	t.Setenv("WEB_TOOLS_CONFIG", overridePath)

	cfg, err := Load()
	assert.NoError(t, err)
	assert.False(t, cfg.Reader.BrowserFallback)
	assert.Equal(t, "http://override:9999", cfg.Search.SearXNGURL)
	assert.Equal(t, 9, cfg.Search.DefaultLimit)
	assert.Equal(t, "zh-CN", cfg.Search.DefaultLocale)
	assert.Equal(t, "duckduckgo", cfg.Search.DefaultEngine)
}
