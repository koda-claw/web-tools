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

	opts := buildSearchOptions(cmd, 5, "auto", "auto", "general", "any")

	assert.Equal(t, search.SearchOptions{}, opts)
}

func TestBuildSearchOptions_ExplicitFlagsOverride(t *testing.T) {
	cmd := Cmd()
	require.NoError(t, cmd.Flags().Set("limit", "2"))
	require.NoError(t, cmd.Flags().Set("engine", "searxng"))
	require.NoError(t, cmd.Flags().Set("locale", "en-US"))
	require.NoError(t, cmd.Flags().Set("category", "news"))
	require.NoError(t, cmd.Flags().Set("time-range", "week"))

	opts := buildSearchOptions(cmd, 2, "searxng", "en-US", "news", "week")

	assert.Equal(t, 2, opts.Limit)
	assert.Equal(t, "searxng", opts.Engine)
	assert.Equal(t, "en-US", opts.Locale)
	assert.Equal(t, "news", opts.Category)
	assert.Equal(t, "week", opts.TimeRange)
}
