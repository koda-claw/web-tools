package websearch

import (
	"testing"

	"github.com/koda-claw/web-tools/internal/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSearchConfig_UsesEnvOverrideForSearXNGURL(t *testing.T) {
	t.Setenv("SEARXNG_URL", "http://env-searxng:7777")

	cfg, err := loadSearchConfig()
	require.NoError(t, err)

	assert.Equal(t, "http://env-searxng:7777", cfg.SearXNGURL)
}

func TestBuildSearchOptions_OmittedFlagsStayZero(t *testing.T) {
	cmd := Cmd()

	opts, err := buildSearchOptions(cmd, 5, "auto", "auto", "auto", "general", "any", false, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, search.SearchOptions{}, opts)
}

func TestBuildSearchOptions_ExplicitFlagsOverride(t *testing.T) {
	cmd := Cmd()
	require.NoError(t, cmd.Flags().Set("limit", "2"))
	require.NoError(t, cmd.Flags().Set("engine", "searxng"))
	require.NoError(t, cmd.Flags().Set("provider", "searxng"))
	require.NoError(t, cmd.Flags().Set("locale", "en-US"))
	require.NoError(t, cmd.Flags().Set("category", "news"))
	require.NoError(t, cmd.Flags().Set("time-range", "week"))
	require.NoError(t, cmd.Flags().Set("no-cache", "true"))
	require.NoError(t, cmd.Flags().Set("include-domain", "example.com,docs.example.com"))
	require.NoError(t, cmd.Flags().Set("exclude-domain", "spam.example"))

	opts, err := buildSearchOptions(cmd, 2, "searxng", "searxng", "en-US", "news", "week", true, []string{"example.com", "docs.example.com"}, []string{"spam.example"})

	require.NoError(t, err)
	assert.Equal(t, 2, opts.Limit)
	assert.Equal(t, "searxng", opts.Engine)
	assert.Equal(t, "searxng", opts.Provider)
	assert.Equal(t, "en-US", opts.Locale)
	assert.Equal(t, "news", opts.Category)
	assert.Equal(t, "week", opts.TimeRange)
	assert.True(t, opts.NoCache)
	assert.Equal(t, []string{"example.com", "docs.example.com"}, opts.IncludeDomains)
	assert.Equal(t, []string{"spam.example"}, opts.ExcludeDomains)
}

func TestBuildSearchOptions_ProviderWithoutEngine(t *testing.T) {
	cmd := Cmd()
	require.NoError(t, cmd.Flags().Set("provider", "duckduckgo"))

	opts, err := buildSearchOptions(cmd, 5, "auto", "duckduckgo", "auto", "general", "any", false, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "duckduckgo", opts.Provider)
	assert.Empty(t, opts.Engine)
}

func TestBuildSearchOptions_ConflictingEngineAndProvider(t *testing.T) {
	cmd := Cmd()
	require.NoError(t, cmd.Flags().Set("engine", "searxng"))
	require.NoError(t, cmd.Flags().Set("provider", "duckduckgo"))

	_, err := buildSearchOptions(cmd, 5, "searxng", "duckduckgo", "auto", "general", "any", false, nil, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflicting provider flags")
}

func TestNormalizeDomainFlags(t *testing.T) {
	got := normalizeDomainFlags([]string{"example.com, docs.example.com", "blog.example.com"})

	assert.Equal(t, []string{"example.com", "docs.example.com", "blog.example.com"}, got)
}
