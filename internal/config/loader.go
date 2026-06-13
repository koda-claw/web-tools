package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

type configOverlay struct {
	Reader readerConfigOverlay `json:"reader"`
	Search searchConfigOverlay `json:"search"`
}

type readerConfigOverlay struct {
	CacheDir         *string `json:"cache_dir"`
	CacheTTL         *int    `json:"cache_ttl"`
	DefaultTimeout   *int    `json:"default_timeout"`
	BrowserFallback  *bool   `json:"browser_fallback"`
	MarkitdownPath   *string `json:"markitdown_path"`
	AgentBrowserPath *string `json:"agent_browser_path"`
	MinContentLength *int    `json:"min_content_length"`
}

type searchConfigOverlay struct {
	SearXNGURL    *string `json:"searxng_url"`
	DefaultLimit  *int    `json:"default_limit"`
	DefaultLocale *string `json:"default_locale"`
	DefaultEngine *string `json:"default_engine"`
}

// Load reads config from files and environment variables, merges with defaults.
// Priority (high to low): env vars > current dir config > user config > defaults
func Load() (*Config, error) {
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
	mergeReaderConfigOverlay(&dst.Reader, src.Reader)
	mergeSearchConfigOverlay(&dst.Search, src.Search)
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
