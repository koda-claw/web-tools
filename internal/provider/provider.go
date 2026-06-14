package provider

import (
	"fmt"
	"os"
	"strings"

	"github.com/koda-claw/web-tools/internal/config"
	apperrors "github.com/koda-claw/web-tools/internal/errors"
)

const (
	CapabilitySearch = "search"
	CapabilityReader = "reader"

	AttemptStatusSelected      = "selected"
	AttemptStatusSkipped       = "skipped"
	AttemptStatusNotConfigured = "skipped:not_configured"
)

// Provider describes a configured backend.
type Provider struct {
	ID     string
	Config config.ProviderConfig
}

// Attempt records provider chain resolution.
type Attempt struct {
	Provider string `json:"provider"`
	Status   string `json:"status"`
	Reason   string `json:"reason,omitempty"`
}

// Registry resolves providers by id, capability, and configured chains.
type Registry struct {
	providers map[string]Provider
	lookupEnv func(string) string
}

// NewRegistry creates a provider registry from config.
func NewRegistry(providers map[string]config.ProviderConfig) (*Registry, error) {
	reg := &Registry{
		providers: make(map[string]Provider, len(providers)),
		lookupEnv: os.Getenv,
	}
	for id, cfg := range providers {
		if strings.TrimSpace(id) == "" {
			return nil, apperrors.NewInputError(
				"invalid provider id",
				"provider id cannot be empty",
				[]string{"set a non-empty providers.<id> key"},
			)
		}
		if _, exists := reg.providers[id]; exists {
			return nil, apperrors.NewInputError(
				"duplicate provider id",
				fmt.Sprintf("provider %q is registered more than once", id),
				[]string{"deduplicate provider configuration"},
			)
		}
		reg.providers[id] = Provider{ID: id, Config: cfg}
	}
	return reg, nil
}

// SetEnvLookup overrides environment lookup for tests.
func (r *Registry) SetEnvLookup(lookup func(string) string) {
	if lookup == nil {
		r.lookupEnv = os.Getenv
		return
	}
	r.lookupEnv = lookup
}

// Get returns a provider by id and required capability.
func (r *Registry) Get(id string, capability string) (Provider, error) {
	provider, ok := r.providers[id]
	if !ok {
		return Provider{}, unknownProviderError(id)
	}
	if !hasCapability(provider.Config, capability) {
		return Provider{}, capabilityMismatchError(id, capability, provider.Config.Capabilities)
	}
	if provider.Config.AuthEnv != "" && r.lookupEnv(provider.Config.AuthEnv) == "" {
		return Provider{}, missingAuthError(id, provider.Config.AuthEnv)
	}
	return provider, nil
}

// ResolveChain returns enabled providers in order. Optional providers gated by
// enabled_if_env are skipped in auto chains instead of failing the whole chain.
func (r *Registry) ResolveChain(chain []string, capability string) ([]Provider, []Attempt, error) {
	resolved := make([]Provider, 0, len(chain))
	attempts := make([]Attempt, 0, len(chain))
	for _, id := range chain {
		provider, ok := r.providers[id]
		if !ok {
			attempts = append(attempts, Attempt{Provider: id, Status: AttemptStatusSkipped, Reason: "unknown provider"})
			return nil, attempts, unknownProviderError(id)
		}
		if !hasCapability(provider.Config, capability) {
			attempts = append(attempts, Attempt{Provider: id, Status: AttemptStatusSkipped, Reason: "capability mismatch"})
			return nil, attempts, capabilityMismatchError(id, capability, provider.Config.Capabilities)
		}
		if provider.Config.EnabledIfEnv != "" && r.lookupEnv(provider.Config.EnabledIfEnv) == "" {
			attempts = append(attempts, Attempt{Provider: id, Status: AttemptStatusNotConfigured, Reason: provider.Config.EnabledIfEnv + " is not set"})
			continue
		}
		attempts = append(attempts, Attempt{Provider: id, Status: AttemptStatusSelected})
		resolved = append(resolved, provider)
	}
	return resolved, attempts, nil
}

func hasCapability(cfg config.ProviderConfig, capability string) bool {
	if capability == "" {
		return true
	}
	for _, item := range cfg.Capabilities {
		if item == capability {
			return true
		}
	}
	return false
}

func unknownProviderError(id string) error {
	return apperrors.NewInputError(
		"unknown provider",
		fmt.Sprintf("provider %q is not configured", id),
		[]string{"check providers config", "use --provider auto"},
	)
}

func capabilityMismatchError(id string, capability string, got []string) error {
	return apperrors.NewInputError(
		"provider capability mismatch",
		fmt.Sprintf("provider %q does not support %q; capabilities: %s", id, capability, strings.Join(got, ",")),
		[]string{"choose a provider with the required capability", "check provider configuration"},
	)
}

func missingAuthError(id string, env string) error {
	return apperrors.NewInputError(
		"provider auth is not configured",
		fmt.Sprintf("provider %q requires environment variable %s", id, env),
		[]string{"export " + env, "choose --provider auto to skip unconfigured optional providers"},
	)
}
