package webreader

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadReaderRuntimeConfig_UsesEnvOverrideForTimeout(t *testing.T) {
	t.Setenv("WEB_READER_TIMEOUT", "42")

	cfg, err := loadReaderRuntimeConfig(0)
	require.NoError(t, err)

	assert.Equal(t, 42, cfg.Reader.DefaultTimeout)
}

func TestLoadReaderRuntimeConfig_FlagTimeoutOverridesEnv(t *testing.T) {
	t.Setenv("WEB_READER_TIMEOUT", "42")

	cfg, err := loadReaderRuntimeConfig(9)
	require.NoError(t, err)

	assert.Equal(t, 9, cfg.Reader.DefaultTimeout)
}
