package websearch

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSearchConfig_UsesEnvOverrideForSearXNGURL(t *testing.T) {
	t.Setenv("SEARXNG_URL", "http://env-searxng:7777")

	cfg, err := loadSearchConfig()
	require.NoError(t, err)

	assert.Equal(t, "http://env-searxng:7777", cfg.SearXNGURL)
}
