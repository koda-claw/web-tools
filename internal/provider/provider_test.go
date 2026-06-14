package provider

import (
	"testing"

	"github.com/koda-claw/web-tools/internal/config"
	apperrors "github.com/koda-claw/web-tools/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRegistry(t *testing.T) *Registry {
	t.Helper()
	reg, err := NewRegistry(map[string]config.ProviderConfig{
		"searxng": {
			Type:         "builtin",
			Capabilities: []string{CapabilitySearch},
		},
		"duckduckgo": {
			Type:         "builtin",
			Capabilities: []string{CapabilitySearch},
		},
		"builtin-reader": {
			Type:         "builtin",
			Capabilities: []string{CapabilityReader},
		},
		"bigmodel": {
			Type:         "mcp",
			AuthEnv:      "ZHIPU_APIKEY",
			EnabledIfEnv: "ZHIPU_APIKEY",
			Capabilities: []string{CapabilitySearch, CapabilityReader},
		},
	})
	require.NoError(t, err)
	reg.SetEnvLookup(func(string) string { return "" })
	return reg
}

func TestRegistryGet(t *testing.T) {
	reg := testRegistry(t)

	provider, err := reg.Get("duckduckgo", CapabilitySearch)

	require.NoError(t, err)
	assert.Equal(t, "duckduckgo", provider.ID)
}

func TestRegistryGetUnknownProvider(t *testing.T) {
	reg := testRegistry(t)

	_, err := reg.Get("missing", CapabilitySearch)

	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, "input", appErr.Category)
	assert.Contains(t, err.Error(), "unknown provider")
}

func TestRegistryGetCapabilityMismatch(t *testing.T) {
	reg := testRegistry(t)

	_, err := reg.Get("builtin-reader", CapabilitySearch)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "capability mismatch")
}

func TestRegistryGetExplicitProviderRequiresAuthEnv(t *testing.T) {
	reg := testRegistry(t)

	_, err := reg.Get("bigmodel", CapabilitySearch)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider auth is not configured")
	assert.Contains(t, err.Error(), "ZHIPU_APIKEY")
}

func TestRegistryResolveChainSkipsUnconfiguredOptionalProvider(t *testing.T) {
	reg := testRegistry(t)

	providers, attempts, err := reg.ResolveChain([]string{"searxng", "bigmodel", "duckduckgo"}, CapabilitySearch)

	require.NoError(t, err)
	require.Len(t, providers, 2)
	assert.Equal(t, "searxng", providers[0].ID)
	assert.Equal(t, "duckduckgo", providers[1].ID)
	require.Len(t, attempts, 3)
	assert.Equal(t, AttemptStatusNotConfigured, attempts[1].Status)
}

func TestRegistryResolveChainIncludesConfiguredOptionalProvider(t *testing.T) {
	reg := testRegistry(t)
	reg.SetEnvLookup(func(name string) string {
		if name == "ZHIPU_APIKEY" {
			return "set"
		}
		return ""
	})

	providers, attempts, err := reg.ResolveChain([]string{"searxng", "bigmodel", "duckduckgo"}, CapabilitySearch)

	require.NoError(t, err)
	require.Len(t, providers, 3)
	assert.Equal(t, "bigmodel", providers[1].ID)
	assert.Equal(t, AttemptStatusSelected, attempts[1].Status)
}

func TestRegistryResolveChainCapabilityMismatch(t *testing.T) {
	reg := testRegistry(t)

	_, attempts, err := reg.ResolveChain([]string{"searxng"}, CapabilityReader)

	require.Error(t, err)
	require.Len(t, attempts, 1)
	assert.Equal(t, "capability mismatch", attempts[0].Reason)
}
