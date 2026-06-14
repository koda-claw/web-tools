package config

// Config is the top-level configuration.
type Config struct {
	Providers map[string]ProviderConfig `json:"providers,omitempty"`
	Reader    ReaderConfig              `json:"reader"`
	Search    SearchConfig              `json:"search"`
}

// ProviderConfig holds provider/plugin settings.
type ProviderConfig struct {
	Type         string                       `json:"type"`
	AuthEnv      string                       `json:"auth_env,omitempty"`
	EnabledIfEnv string                       `json:"enabled_if_env,omitempty"`
	Timeout      int                          `json:"timeout,omitempty"`
	Command      string                       `json:"command,omitempty"`
	Capabilities []string                     `json:"capabilities,omitempty"`
	Search       *ProviderEndpointConfig      `json:"search,omitempty"`
	Reader       *ProviderEndpointConfig      `json:"reader,omitempty"`
	Extra        map[string]map[string]string `json:"extra,omitempty"`
	Headers      map[string]string            `json:"headers,omitempty"`
	Metadata     map[string]string            `json:"metadata,omitempty"`
}

// ProviderEndpointConfig holds a provider endpoint/tool pair.
type ProviderEndpointConfig struct {
	URL    string            `json:"url,omitempty"`
	Tool   string            `json:"tool,omitempty"`
	Params map[string]string `json:"params,omitempty"`
}

// ReaderConfig holds web-reader specific settings.
type ReaderConfig struct {
	CacheDir             string   `json:"cache_dir"`
	CacheTTL             int      `json:"cache_ttl"`
	DefaultTimeout       int      `json:"default_timeout"`
	BrowserFallback      bool     `json:"browser_fallback"`
	MarkitdownPath       string   `json:"markitdown_path"`
	AgentBrowserPath     string   `json:"agent_browser_path"`
	MinContentLength     int      `json:"min_content_length"`
	DefaultProvider      string   `json:"default_provider"`
	DefaultProviderChain []string `json:"default_provider_chain"`
}

// SearchConfig holds web-search specific settings.
type SearchConfig struct {
	SearXNGURL           string   `json:"searxng_url"`
	DefaultLimit         int      `json:"default_limit"`
	DefaultLocale        string   `json:"default_locale"`
	DefaultEngine        string   `json:"default_engine"` // "auto" / "duckduckgo" / "searxng"
	DefaultProvider      string   `json:"default_provider"`
	DefaultProviderChain []string `json:"default_provider_chain"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Providers: defaultProviders(),
		Reader: ReaderConfig{
			CacheDir:             expandHome("~/.cache/web-tools"),
			CacheTTL:             DefaultCacheTTL,
			DefaultTimeout:       DefaultTimeout,
			BrowserFallback:      true,
			MarkitdownPath:       "markitdown",
			AgentBrowserPath:     "agent-browser",
			MinContentLength:     DefaultMinContentLength,
			DefaultProvider:      "auto",
			DefaultProviderChain: []string{"builtin-reader"},
		},
		Search: SearchConfig{
			SearXNGURL:           DefaultSearXNGURL,
			DefaultLimit:         DefaultSearchLimit,
			DefaultLocale:        "auto",
			DefaultEngine:        "auto",
			DefaultProvider:      "auto",
			DefaultProviderChain: []string{"searxng", "duckduckgo"},
		},
	}
}

func defaultProviders() map[string]ProviderConfig {
	return map[string]ProviderConfig{
		"searxng": {
			Type:         "builtin",
			Capabilities: []string{"search"},
		},
		"duckduckgo": {
			Type:         "builtin",
			Capabilities: []string{"search"},
		},
		"builtin-reader": {
			Type:         "builtin",
			Capabilities: []string{"reader"},
		},
	}
}
