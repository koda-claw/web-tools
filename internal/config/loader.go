package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

type configOverlay struct {
	Providers map[string]providerConfigOverlay `json:"providers"`
	Reader    readerConfigOverlay              `json:"reader"`
	Search    searchConfigOverlay              `json:"search"`
}

type providerConfigOverlay struct {
	Type         *string                      `json:"type"`
	AuthEnv      *string                      `json:"auth_env"`
	EnabledIfEnv *string                      `json:"enabled_if_env"`
	Timeout      *int                         `json:"timeout"`
	Command      *string                      `json:"command"`
	Capabilities []string                     `json:"capabilities"`
	Search       *providerEndpointOverlay     `json:"search"`
	Reader       *providerEndpointOverlay     `json:"reader"`
	Extra        map[string]map[string]string `json:"extra"`
	Headers      map[string]string            `json:"headers"`
	Metadata     map[string]string            `json:"metadata"`
}

type providerEndpointOverlay struct {
	URL    *string           `json:"url"`
	Tool   *string           `json:"tool"`
	Params map[string]string `json:"params"`
}

type readerConfigOverlay struct {
	CacheDir             *string  `json:"cache_dir"`
	CacheTTL             *int     `json:"cache_ttl"`
	DefaultTimeout       *int     `json:"default_timeout"`
	BrowserFallback      *bool    `json:"browser_fallback"`
	MarkitdownPath       *string  `json:"markitdown_path"`
	AgentBrowserPath     *string  `json:"agent_browser_path"`
	MinContentLength     *int     `json:"min_content_length"`
	DefaultProvider      *string  `json:"default_provider"`
	DefaultProviderChain []string `json:"default_provider_chain"`
}

type searchConfigOverlay struct {
	SearXNGURL           *string  `json:"searxng_url"`
	DefaultLimit         *int     `json:"default_limit"`
	DefaultLocale        *string  `json:"default_locale"`
	DefaultEngine        *string  `json:"default_engine"`
	DefaultProvider      *string  `json:"default_provider"`
	DefaultProviderChain []string `json:"default_provider_chain"`
}

// Load reads config from files and environment variables, merges with defaults.
// Priority (high to low): env vars > current dir config > user config > defaults
func Load() (*Config, error) {
	envReport := LoadEnvFiles()
	if err := envReport.Err(); err != nil {
		return nil, err
	}

	cfg := DefaultConfig()

	// 1. Load user config: ~/.config/web-tools/config.json
	userCfg, err := loadConfigFile(expandHome("~/.config/web-tools/config.json"))
	if err == nil {
		mergeConfigOverlay(&cfg, userCfg)
	}

	// 2. Load local config: ./web-tools.json
	localCfg, err := loadConfigFile("web-tools.json")
	if err == nil {
		mergeConfigOverlay(&cfg, localCfg)
	}

	// 3. Override with environment variables
	applyEnvOverrides(&cfg)

	return &cfg, nil
}

func loadConfigFile(path string) (*configOverlay, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config file not found: %s", path)
	}

	var cfg configOverlay
	if err := json.Unmarshal(data, &cfg); err != nil {
		// Invalid JSON: skip silently, don't break the flow
		return nil, fmt.Errorf("invalid config JSON in %s: %w", path, err)
	}

	return &cfg, nil
}

func mergeConfigOverlay(dst *Config, src *configOverlay) {
	if src == nil {
		return
	}
	mergeProviderConfigOverlay(dst, src.Providers)
	mergeReaderConfigOverlay(&dst.Reader, src.Reader)
	mergeSearchConfigOverlay(&dst.Search, src.Search)
}

func mergeProviderConfigOverlay(dst *Config, src map[string]providerConfigOverlay) {
	if len(src) == 0 {
		return
	}
	if dst.Providers == nil {
		dst.Providers = map[string]ProviderConfig{}
	}
	for id, overlay := range src {
		current := dst.Providers[id]
		if overlay.Type != nil {
			current.Type = *overlay.Type
		}
		if overlay.AuthEnv != nil {
			current.AuthEnv = *overlay.AuthEnv
		}
		if overlay.EnabledIfEnv != nil {
			current.EnabledIfEnv = *overlay.EnabledIfEnv
		}
		if overlay.Timeout != nil {
			current.Timeout = *overlay.Timeout
		}
		if overlay.Command != nil {
			current.Command = *overlay.Command
		}
		if overlay.Capabilities != nil {
			current.Capabilities = append([]string(nil), overlay.Capabilities...)
		}
		if overlay.Search != nil {
			endpoint := mergeProviderEndpoint(current.Search, overlay.Search)
			current.Search = &endpoint
		}
		if overlay.Reader != nil {
			endpoint := mergeProviderEndpoint(current.Reader, overlay.Reader)
			current.Reader = &endpoint
		}
		if overlay.Extra != nil {
			current.Extra = cloneNestedStringMap(overlay.Extra)
		}
		if overlay.Headers != nil {
			current.Headers = cloneStringMap(overlay.Headers)
		}
		if overlay.Metadata != nil {
			current.Metadata = cloneStringMap(overlay.Metadata)
		}
		dst.Providers[id] = current
	}
}

func mergeProviderEndpoint(current *ProviderEndpointConfig, overlay *providerEndpointOverlay) ProviderEndpointConfig {
	var endpoint ProviderEndpointConfig
	if current != nil {
		endpoint = *current
		if current.Params != nil {
			endpoint.Params = cloneStringMap(current.Params)
		}
	}
	if overlay.URL != nil {
		endpoint.URL = *overlay.URL
	}
	if overlay.Tool != nil {
		endpoint.Tool = *overlay.Tool
	}
	if overlay.Params != nil {
		endpoint.Params = cloneStringMap(overlay.Params)
	}
	return endpoint
}

func mergeReaderConfigOverlay(dst *ReaderConfig, src readerConfigOverlay) {
	if src.CacheDir != nil {
		dst.CacheDir = *src.CacheDir
	}
	if src.CacheTTL != nil {
		dst.CacheTTL = *src.CacheTTL
	}
	if src.DefaultTimeout != nil {
		dst.DefaultTimeout = *src.DefaultTimeout
	}
	if src.MarkitdownPath != nil {
		dst.MarkitdownPath = *src.MarkitdownPath
	}
	if src.AgentBrowserPath != nil {
		dst.AgentBrowserPath = *src.AgentBrowserPath
	}
	if src.MinContentLength != nil {
		dst.MinContentLength = *src.MinContentLength
	}
	if src.BrowserFallback != nil {
		dst.BrowserFallback = *src.BrowserFallback
	}
	if src.DefaultProvider != nil {
		dst.DefaultProvider = *src.DefaultProvider
	}
	if src.DefaultProviderChain != nil {
		dst.DefaultProviderChain = append([]string(nil), src.DefaultProviderChain...)
	}
}

func mergeSearchConfigOverlay(dst *SearchConfig, src searchConfigOverlay) {
	if src.SearXNGURL != nil {
		dst.SearXNGURL = *src.SearXNGURL
	}
	if src.DefaultLimit != nil {
		dst.DefaultLimit = *src.DefaultLimit
	}
	if src.DefaultLocale != nil {
		dst.DefaultLocale = *src.DefaultLocale
	}
	if src.DefaultEngine != nil {
		dst.DefaultEngine = *src.DefaultEngine
	}
	if src.DefaultProvider != nil {
		dst.DefaultProvider = *src.DefaultProvider
	}
	if src.DefaultProviderChain != nil {
		dst.DefaultProviderChain = append([]string(nil), src.DefaultProviderChain...)
	}
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("SEARXNG_URL"); v != "" {
		cfg.Search.SearXNGURL = v
	}
	if v := os.Getenv("WEB_READER_CACHE_TTL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Reader.CacheTTL = n
		}
	}
	if v := os.Getenv("WEB_READER_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Reader.DefaultTimeout = n
		}
	}
	if os.Getenv("WEB_READER_NO_BROWSER") != "" {
		cfg.Reader.BrowserFallback = false
	}
	if v := os.Getenv("MARKITDOWN_PATH"); v != "" {
		cfg.Reader.MarkitdownPath = v
	}
	if v := os.Getenv("WEB_TOOLS_CONFIG"); v != "" {
		fileCfg, err := loadConfigFile(v)
		if err == nil {
			mergeConfigOverlay(cfg, fileCfg)
		}
	}
}

func cloneStringMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func cloneNestedStringMap(src map[string]map[string]string) map[string]map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]map[string]string, len(src))
	for key, value := range src {
		dst[key] = cloneStringMap(value)
	}
	return dst
}
