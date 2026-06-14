package configcmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddProviderBigModelWritesConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	require.NoError(t, addProvider(path, "bigmodel", "bigmodel", "ZHIPU_APIKEY", true, false, false))

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var cfg struct {
		Providers map[string]struct {
			Type         string   `json:"type"`
			AuthEnv      string   `json:"auth_env"`
			EnabledIfEnv string   `json:"enabled_if_env"`
			Capabilities []string `json:"capabilities"`
			Search       struct {
				URL  string `json:"url"`
				Tool string `json:"tool"`
			} `json:"search"`
			Reader struct {
				URL  string `json:"url"`
				Tool string `json:"tool"`
			} `json:"reader"`
		} `json:"providers"`
		Search struct {
			DefaultProviderChain []string `json:"default_provider_chain"`
		} `json:"search"`
	}
	require.NoError(t, json.Unmarshal(data, &cfg))

	provider := cfg.Providers["bigmodel"]
	assert.Equal(t, "mcp", provider.Type)
	assert.Equal(t, "ZHIPU_APIKEY", provider.AuthEnv)
	assert.Equal(t, "ZHIPU_APIKEY", provider.EnabledIfEnv)
	assert.Equal(t, []string{"search", "reader"}, provider.Capabilities)
	assert.Equal(t, "https://open.bigmodel.cn/api/mcp/web_search_prime/mcp", provider.Search.URL)
	assert.Equal(t, "web_search_prime", provider.Search.Tool)
	assert.Equal(t, "https://open.bigmodel.cn/api/mcp/web_reader/mcp", provider.Reader.URL)
	assert.Equal(t, "webReader", provider.Reader.Tool)
	assert.Equal(t, []string{"searxng", "bigmodel", "duckduckgo"}, cfg.Search.DefaultProviderChain)
	assert.NotContains(t, string(data), "Bearer")
}

func TestAddProviderRejectsUnknownPreset(t *testing.T) {
	err := addProvider(filepath.Join(t.TempDir(), "config.json"), "example", "unknown", "TOKEN", false, false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown provider preset")
}
