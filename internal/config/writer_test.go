package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddBigModelProviderIsIdempotent(t *testing.T) {
	cfg := &EditableConfig{}

	AddBigModelProvider(cfg, "ZHIPU_APIKEY", true, true)
	AddBigModelProvider(cfg, "ZHIPU_APIKEY", true, true)

	provider := cfg.Providers["bigmodel"]
	assert.Equal(t, "mcp", provider.Type)
	assert.Equal(t, "ZHIPU_APIKEY", provider.AuthEnv)
	assert.Equal(t, []string{"searxng", "bigmodel", "duckduckgo"}, cfg.Search.DefaultProviderChain)
	assert.Equal(t, []string{"builtin-reader", "bigmodel"}, cfg.Reader.DefaultProviderChain)
}

func TestEditableConfigRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := &EditableConfig{}
	AddBigModelProvider(cfg, "ZHIPU_APIKEY", false, false)

	require.NoError(t, SaveEditableConfig(path, cfg))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "Bearer")

	loaded, err := LoadEditableConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "mcp", loaded.Providers["bigmodel"].Type)
}
