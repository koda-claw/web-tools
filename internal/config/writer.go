package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// EditableConfig is the on-disk config shape used by config subcommands.
type EditableConfig struct {
	Providers map[string]ProviderConfig `json:"providers,omitempty"`
	Reader    *ReaderConfig             `json:"reader,omitempty"`
	Search    *SearchConfig             `json:"search,omitempty"`
}

// LoadEditableConfig reads a user-editable config file. Missing files return an empty config.
func LoadEditableConfig(path string) (*EditableConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &EditableConfig{}, nil
		}
		return nil, err
	}
	var cfg EditableConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid config JSON in %s: %w", path, err)
	}
	return &cfg, nil
}

// SaveEditableConfig writes an editable config with stable indentation.
func SaveEditableConfig(path string, cfg *EditableConfig) error {
	if cfg == nil {
		cfg = &EditableConfig{}
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// AddBigModelProvider adds or updates the BigModel/Zhipu MCP provider.
func AddBigModelProvider(cfg *EditableConfig, authEnv string, addToSearchAuto bool, addToReaderAuto bool) {
	if cfg.Providers == nil {
		cfg.Providers = map[string]ProviderConfig{}
	}
	if authEnv == "" {
		authEnv = "ZHIPU_APIKEY"
	}
	cfg.Providers["bigmodel"] = ProviderConfig{
		Type:         "mcp",
		AuthEnv:      authEnv,
		EnabledIfEnv: authEnv,
		Timeout:      30,
		Capabilities: []string{"search", "reader"},
		Search: &ProviderEndpointConfig{
			URL:  "https://open.bigmodel.cn/api/mcp/web_search_prime/mcp",
			Tool: "web_search_prime",
		},
		Reader: &ProviderEndpointConfig{
			URL:  "https://open.bigmodel.cn/api/mcp/web_reader/mcp",
			Tool: "webReader",
		},
	}
	if addToSearchAuto {
		if cfg.Search == nil {
			defaultSearch := DefaultConfig().Search
			cfg.Search = &defaultSearch
		}
		if len(cfg.Search.DefaultProviderChain) == 0 {
			cfg.Search.DefaultProviderChain = append([]string(nil), DefaultConfig().Search.DefaultProviderChain...)
		}
		cfg.Search.DefaultProviderChain = insertProviderBeforeFallback(cfg.Search.DefaultProviderChain, "bigmodel", "duckduckgo")
	}
	if addToReaderAuto {
		if cfg.Reader == nil {
			defaultReader := DefaultConfig().Reader
			cfg.Reader = &defaultReader
		}
		if len(cfg.Reader.DefaultProviderChain) == 0 {
			cfg.Reader.DefaultProviderChain = append([]string(nil), DefaultConfig().Reader.DefaultProviderChain...)
		}
		cfg.Reader.DefaultProviderChain = appendUniqueProvider(cfg.Reader.DefaultProviderChain, "bigmodel")
	}
}

func insertProviderBeforeFallback(chain []string, provider string, fallback string) []string {
	chain = appendUniqueProvider(chain, "")
	out := make([]string, 0, len(chain)+1)
	inserted := false
	for _, id := range chain {
		if id == "" || id == provider {
			continue
		}
		if id == fallback && !inserted {
			out = append(out, provider)
			inserted = true
		}
		out = append(out, id)
	}
	if !inserted {
		out = append(out, provider)
	}
	return out
}

func appendUniqueProvider(chain []string, provider string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(chain)+1)
	for _, id := range chain {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	if provider != "" && !seen[provider] {
		out = append(out, provider)
	}
	return out
}
