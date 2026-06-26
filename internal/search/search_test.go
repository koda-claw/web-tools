package search

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/koda-claw/web-tools/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- RenderMarkdown / RenderJSON tests (unchanged public API) ---

func TestSearchResponse_RenderMarkdown(t *testing.T) {
	resp := &SearchResponse{
		Query:  "golang readability",
		Engine: "searxng",
		Locale: "en-US",
		Total:  2,
		Results: []SearchResult{
			{Rank: 1, Title: "go-readability", URL: "https://github.com/go-shiori/go-readability", Snippet: "Extract content from HTML", Source: "github.com", Engines: []string{"searxng"}},
			{Rank: 2, Title: "Readability.js", URL: "https://github.com/mozilla/readability", Snippet: "Mozilla readability", Source: "github.com", Engines: []string{"searxng"}},
		},
		SearchedAt: time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC),
	}

	md := resp.RenderMarkdown()
	assert.Contains(t, md, `## Search: "golang readability"`)
	assert.Contains(t, md, "Engine: searxng | Locale: en-US | Results: 2")
	assert.Contains(t, md, "### 1. go-readability")
	assert.Contains(t, md, "### 2. Readability.js")
	assert.Contains(t, md, "**URL:** https://github.com/go-shiori/go-readability")
	assert.Contains(t, md, "**Snippet:** Extract content from HTML")
}

func TestSearchResponse_RenderJSON(t *testing.T) {
	resp := &SearchResponse{
		Query:  "test query",
		Engine: "searxng",
		Total:  1,
		Results: []SearchResult{
			{Rank: 1, Title: "Result", URL: "https://example.com", Snippet: "A snippet"},
		},
		SearchedAt: time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC),
	}

	j := resp.RenderJSON()
	assert.Contains(t, j, `"ok": true`)
	assert.Contains(t, j, `"query": "test query"`)
	assert.Contains(t, j, `"rank": 1`)
	assert.Contains(t, j, `"title": "Result"`)
}

func TestSearchResponse_JSONStructure(t *testing.T) {
	resp := &SearchResponse{
		Query:      "test",
		Engine:     "searxng",
		Locale:     "auto",
		Total:      1,
		SearchedAt: time.Now(),
		Results: []SearchResult{
			{Rank: 1, Title: "T", URL: "https://u.com", Snippet: "S", Source: "u.com", Engines: []string{"searxng"}},
		},
	}

	j := resp.RenderJSON()
	var parsed map[string]interface{}
	assert.NoError(t, json.Unmarshal([]byte(j), &parsed))
	assert.Equal(t, true, parsed["ok"])

	result := parsed["result"].(map[string]interface{})
	assert.Equal(t, "test", result["query"])
	assert.Equal(t, float64(1), result["total"])

	results := result["results"].([]interface{})
	require.Len(t, results, 1)
	firstResult := results[0].(map[string]interface{})
	assert.Equal(t, float64(1), firstResult["rank"])
	assert.Equal(t, "T", firstResult["title"])
}

// --- SearXNGEngine tests ---

func TestSearXNGEngine_Query(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/search")
		assert.Equal(t, "json", r.URL.Query().Get("format"))
		assert.Equal(t, "test query", r.URL.Query().Get("q"))
		assert.Equal(t, "general", r.URL.Query().Get("categories"))

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"number_of_results": 2,
			"results": [
				{"title": "Result 1", "url": "https://example.com/1", "content": "Snippet 1", "engines": ["google"], "parsed_url": ["https", "example.com", "/1"]},
				{"title": "Result 2", "url": "https://example.com/2", "content": "Snippet 2", "engines": ["bing"],   "parsed_url": ["https", "example.com", "/2"]}
			]
		}`))
	}))
	defer server.Close()

	engine := NewSearXNGEngine(server.URL)
	opts := SearchOptions{Limit: 5, Category: "general"}
	results, err := engine.Query("test query", opts)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "Result 1", results[0].Title)
	assert.Equal(t, "Snippet 2", results[1].Snippet)
	assert.Equal(t, "example.com", results[0].Source)
}

func TestSearXNGEngine_HealthCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	engine := NewSearXNGEngine(server.URL)
	assert.NoError(t, engine.HealthCheck())
}

func TestSearXNGEngine_Query_Limit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"number_of_results": 5,
			"results": [
				{"title": "R1", "url": "https://example.com/1", "content": "S1", "parsed_url": ["https","example.com","/1"]},
				{"title": "R2", "url": "https://example.com/2", "content": "S2", "parsed_url": ["https","example.com","/2"]},
				{"title": "R3", "url": "https://example.com/3", "content": "S3", "parsed_url": ["https","example.com","/3"]},
				{"title": "R4", "url": "https://example.com/4", "content": "S4", "parsed_url": ["https","example.com","/4"]},
				{"title": "R5", "url": "https://example.com/5", "content": "S5", "parsed_url": ["https","example.com","/5"]}
			]
		}`))
	}))
	defer server.Close()

	engine := NewSearXNGEngine(server.URL)
	results, err := engine.Query("test", SearchOptions{Limit: 2})
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

// --- mockEngine for auto-fallback tests ---

type mockEngine struct {
	name        string
	healthErr   error
	queryErr    error
	queryResult []RawResult
	lastOpts    SearchOptions
	queryCount  int
}

func (m *mockEngine) Name() string       { return m.name }
func (m *mockEngine) HealthCheck() error { return m.healthErr }
func (m *mockEngine) Query(_ string, opts SearchOptions) ([]RawResult, error) {
	m.queryCount++
	m.lastOpts = opts
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	return m.queryResult, nil
}

func makeConfiguredTestSearch(cfg config.SearchConfig, engines ...Engine) *Search {
	return &Search{
		engines:   engines,
		config:    cfg,
		providers: config.DefaultConfig().Providers,
		cache:     newTestSearchCache(),
	}
}

func makeTestSearch(engines ...Engine) *Search {
	return &Search{
		engines: engines,
		config: config.SearchConfig{
			DefaultLimit:         5,
			DefaultLocale:        "auto",
			DefaultEngine:        "auto",
			DefaultProvider:      "auto",
			DefaultProviderChain: []string{"searxng", "duckduckgo"},
		},
		providers: config.DefaultConfig().Providers,
		cache:     newTestSearchCache(),
	}
}

func newTestSearchCache() *searchCache {
	return &searchCache{
		entries:  make(map[string]searchCacheEntry),
		cacheTTL: time.Duration(config.DefaultCacheTTL) * time.Second,
	}
}

// TestAutoMode_FallbackToDDG verifies that in auto mode, when SearXNG is
// unavailable, the search falls back to the next engine (DuckDuckGo).
func TestAutoMode_FallbackToDDG(t *testing.T) {
	ddgResults := []RawResult{
		{Title: "DDG Result", URL: "https://ddg.example.com", Snippet: "via DDG", Source: "ddg.example.com"},
	}

	s := makeTestSearch(
		&mockEngine{name: "searxng", healthErr: errors.New("connection refused")},
		&mockEngine{name: "duckduckgo", queryResult: ddgResults},
	)

	resp, err := s.Do("test query", SearchOptions{Engine: "auto"})
	require.NoError(t, err)
	assert.Equal(t, "duckduckgo", resp.Engine)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, "DDG Result", resp.Results[0].Title)
}

func TestAutoMode_FallbackToDDGWhenFirstEngineReturnsNoResults(t *testing.T) {
	ddgResults := []RawResult{
		{Title: "DDG Result", URL: "https://ddg.example.com", Snippet: "via DDG", Source: "ddg.example.com"},
	}

	s := makeTestSearch(
		&mockEngine{name: "searxng", queryResult: nil},
		&mockEngine{name: "duckduckgo", queryResult: ddgResults},
	)

	resp, err := s.Do("test query", SearchOptions{Engine: "auto"})
	require.NoError(t, err)
	assert.Equal(t, "duckduckgo", resp.Engine)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, "DDG Result", resp.Results[0].Title)
}

func TestAutoMode_FallbackToDDGWhenFirstEngineResultsAreFilteredOut(t *testing.T) {
	ddgResults := []RawResult{
		{Title: "GitHub Result", URL: "https://github.com/example/repo", Snippet: "via DDG", Source: "github.com"},
	}

	s := makeTestSearch(
		&mockEngine{name: "searxng", queryResult: []RawResult{
			{Title: "Filtered Result", URL: "https://noise.example.net/page", Snippet: "via SearXNG", Source: "noise.example.net"},
		}},
		&mockEngine{name: "duckduckgo", queryResult: ddgResults},
	)

	resp, err := s.Do("test query", SearchOptions{Engine: "auto", IncludeDomains: []string{"github.com"}})
	require.NoError(t, err)
	assert.Equal(t, "duckduckgo", resp.Engine)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, "GitHub Result", resp.Results[0].Title)
	assert.Equal(t, "github.com", resp.Results[0].Source)
}

func TestAutoMode_ReturnsEmptyResultsWhenAllEnginesReturnNoResults(t *testing.T) {
	s := makeTestSearch(
		&mockEngine{name: "searxng", queryResult: nil},
		&mockEngine{name: "duckduckgo", queryResult: nil},
	)

	resp, err := s.Do("test query", SearchOptions{Engine: "auto"})
	require.NoError(t, err)
	assert.Equal(t, "duckduckgo", resp.Engine)
	assert.Empty(t, resp.Results)
	assert.Equal(t, 0, resp.Total)
}

// TestAutoMode_AllEnginesFail verifies that an error is returned when all engines fail.
func TestAutoMode_AllEnginesFail(t *testing.T) {
	s := makeTestSearch(
		&mockEngine{name: "searxng", healthErr: errors.New("down")},
		&mockEngine{name: "duckduckgo", healthErr: errors.New("network error")},
	)

	_, err := s.Do("test query", SearchOptions{Engine: "auto"})
	assert.Error(t, err)
}

func TestAutoMode_FallsBackOnRateLimitError(t *testing.T) {
	ddgResults := []RawResult{
		{Title: "DDG Result", URL: "https://ddg.example.com", Snippet: "via DDG", Source: "ddg.example.com"},
	}

	s := makeTestSearch(
		&mockEngine{name: "searxng", queryErr: &RateLimitError{Engine: "searxng", Reason: "rate_limited"}},
		&mockEngine{name: "duckduckgo", queryResult: ddgResults},
	)

	resp, err := s.Do("test query", SearchOptions{Engine: "auto"})

	require.NoError(t, err)
	assert.Equal(t, "duckduckgo", resp.Engine)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, "DDG Result", resp.Results[0].Title)
}

// TestSpecificEngine_SearXNG verifies that --engine searxng skips DDG entirely.
func TestSpecificEngine_SearXNG(t *testing.T) {
	sxResults := []RawResult{
		{Title: "SearXNG Result", URL: "https://sx.example.com", Snippet: "via SearXNG", Source: "sx.example.com"},
	}
	s := makeTestSearch(
		&mockEngine{name: "searxng", queryResult: sxResults},
		&mockEngine{name: "duckduckgo", healthErr: errors.New("should not be called")},
	)

	resp, err := s.Do("test", SearchOptions{Engine: "searxng"})
	require.NoError(t, err)
	assert.Equal(t, "searxng", resp.Engine)
	assert.Equal(t, "SearXNG Result", resp.Results[0].Title)
}

func TestSearchDefaultsFromConfig(t *testing.T) {
	ddg := &mockEngine{
		name: "duckduckgo",
		queryResult: []RawResult{
			{Title: "DDG Result", URL: "https://example.com", Snippet: "snippet", Source: "example.com"},
		},
	}
	s := makeConfiguredTestSearch(config.SearchConfig{
		DefaultLimit:  7,
		DefaultLocale: "zh-CN",
		DefaultEngine: "duckduckgo",
	}, &mockEngine{name: "searxng", healthErr: errors.New("should not be called")}, ddg)

	resp, err := s.Do("test", SearchOptions{})
	require.NoError(t, err)

	assert.Equal(t, "duckduckgo", resp.Engine)
	assert.Equal(t, "zh-CN", resp.Locale)
	assert.Equal(t, 7, ddg.lastOpts.Limit)
	assert.Equal(t, "zh-CN", ddg.lastOpts.Locale)
	assert.Equal(t, "general", ddg.lastOpts.Category)
	assert.Equal(t, "any", ddg.lastOpts.TimeRange)
}

func TestSearchExplicitOptionsOverrideConfig(t *testing.T) {
	sx := &mockEngine{
		name: "searxng",
		queryResult: []RawResult{
			{Title: "SearXNG Result", URL: "https://example.com", Snippet: "snippet", Source: "example.com"},
		},
	}
	s := makeConfiguredTestSearch(config.SearchConfig{
		DefaultLimit:  7,
		DefaultLocale: "zh-CN",
		DefaultEngine: "duckduckgo",
	}, sx, &mockEngine{name: "duckduckgo", healthErr: errors.New("should not be called")})

	resp, err := s.Do("test", SearchOptions{
		Limit:     2,
		Locale:    "en-US",
		Category:  "news",
		TimeRange: "week",
		Engine:    "searxng",
	})
	require.NoError(t, err)

	assert.Equal(t, "searxng", resp.Engine)
	assert.Equal(t, "en-US", resp.Locale)
	assert.Equal(t, 2, sx.lastOpts.Limit)
	assert.Equal(t, "en-US", sx.lastOpts.Locale)
	assert.Equal(t, "news", sx.lastOpts.Category)
	assert.Equal(t, "week", sx.lastOpts.TimeRange)
}

func TestSearchCacheHitSkipsEngine(t *testing.T) {
	ddg := &mockEngine{
		name: "duckduckgo",
		queryResult: []RawResult{
			{Title: "DDG Result", URL: "https://example.com", Snippet: "snippet", Source: "example.com"},
		},
	}
	s := makeTestSearch(ddg)

	first, err := s.Do("test", SearchOptions{Provider: "duckduckgo"})
	require.NoError(t, err)
	second, err := s.Do("test", SearchOptions{Provider: "duckduckgo"})
	require.NoError(t, err)

	assert.Equal(t, 1, ddg.queryCount)
	assert.Equal(t, first.Results, second.Results)
}

func TestSearchDefaultCacheSharedAcrossInstances(t *testing.T) {
	original := defaultSearchCache
	defaultSearchCache = newTestSearchCache()
	t.Cleanup(func() { defaultSearchCache = original })

	firstEngine := &mockEngine{
		name: "duckduckgo",
		queryResult: []RawResult{
			{Title: "DDG Result", URL: "https://example.com", Snippet: "snippet", Source: "example.com"},
		},
	}
	secondEngine := &mockEngine{
		name:     "duckduckgo",
		queryErr: errors.New("should not be called"),
	}

	first := NewSearchWithConfig(config.DefaultConfig())
	first.engines = []Engine{firstEngine}
	_, err := first.Do("test", SearchOptions{Provider: "duckduckgo"})
	require.NoError(t, err)

	second := NewSearchWithConfig(config.DefaultConfig())
	second.engines = []Engine{secondEngine}
	resp, err := second.Do("test", SearchOptions{Provider: "duckduckgo"})

	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, 1, firstEngine.queryCount)
	assert.Equal(t, 0, secondEngine.queryCount)
}

func TestSearchNoCacheBypassesCache(t *testing.T) {
	ddg := &mockEngine{
		name: "duckduckgo",
		queryResult: []RawResult{
			{Title: "DDG Result", URL: "https://example.com", Snippet: "snippet", Source: "example.com"},
		},
	}
	s := makeTestSearch(ddg)

	_, err := s.Do("test", SearchOptions{Provider: "duckduckgo"})
	require.NoError(t, err)
	_, err = s.Do("test", SearchOptions{Provider: "duckduckgo", NoCache: true})
	require.NoError(t, err)

	assert.Equal(t, 2, ddg.queryCount)
}

func TestSearchCacheKeyIncludesTimeRange(t *testing.T) {
	ddg := &mockEngine{
		name: "duckduckgo",
		queryResult: []RawResult{
			{Title: "DDG Result", URL: "https://example.com", Snippet: "snippet", Source: "example.com"},
		},
	}
	s := makeTestSearch(ddg)

	_, err := s.Do("test", SearchOptions{Provider: "duckduckgo", TimeRange: "day"})
	require.NoError(t, err)
	_, err = s.Do("test", SearchOptions{Provider: "duckduckgo", TimeRange: "year"})
	require.NoError(t, err)

	assert.Equal(t, 2, ddg.queryCount)
}

func TestSearchProviderOptionSelectsEngine(t *testing.T) {
	ddg := &mockEngine{
		name: "duckduckgo",
		queryResult: []RawResult{
			{Title: "DDG Result", URL: "https://example.com", Snippet: "snippet", Source: "example.com"},
		},
	}
	s := makeTestSearch(&mockEngine{name: "searxng", healthErr: errors.New("should not be called")}, ddg)

	resp, err := s.Do("test", SearchOptions{Provider: "duckduckgo"})

	require.NoError(t, err)
	assert.Equal(t, "duckduckgo", resp.Engine)
	assert.Equal(t, "duckduckgo", resp.Provider)
}

func TestSearchProviderChainSkipsUnconfiguredRemoteProvider(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Search.DefaultProviderChain = []string{"searxng", "bigmodel", "duckduckgo"}
	cfg.Providers["bigmodel"] = config.ProviderConfig{
		Type:         "mcp",
		AuthEnv:      "ZHIPU_APIKEY",
		EnabledIfEnv: "ZHIPU_APIKEY",
		Capabilities: []string{"search"},
	}
	ddg := &mockEngine{
		name: "duckduckgo",
		queryResult: []RawResult{
			{Title: "DDG Result", URL: "https://example.com", Snippet: "snippet", Source: "example.com"},
		},
	}
	s := &Search{
		engines: []Engine{
			&mockEngine{name: "searxng", queryResult: nil},
			ddg,
		},
		config:    cfg.Search,
		providers: cfg.Providers,
	}

	resp, err := s.Do("test", SearchOptions{Provider: "auto"})

	require.NoError(t, err)
	assert.Equal(t, "duckduckgo", resp.Engine)
	require.Len(t, resp.ProviderChain, 3)
	assert.Equal(t, "bigmodel", resp.ProviderChain[1].Provider)
	assert.Equal(t, "skipped:not_configured", resp.ProviderChain[1].Status)
}

func TestSearchProviderChainUsesConfiguredMCPProvider(t *testing.T) {
	t.Setenv("ZHIPU_APIKEY", "test-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.Header.Get("Accept"), "text/event-stream")
		w.Header().Set("Content-Type", "text/event-stream;charset=UTF-8")
		var req map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		switch req["method"] {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "test-session")
			fmt.Fprintf(w, "id:1\nevent:message\ndata:{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"protocolVersion\":\"2024-11-05\",\"capabilities\":{\"tools\":{\"listChanged\":true}},\"serverInfo\":{\"name\":\"mock\",\"version\":\"0.0.1\"}}}\n\n")
		case "notifications/initialized":
			fmt.Fprintf(w, "id:1\nevent:message\ndata:{\"jsonrpc\":\"2.0\",\"result\":{}}\n\n")
		case "tools/call":
			text, _ := json.Marshal(`[{"title":"MCP Result","link":"https://example.com","content":"via MCP","refer":"ref_1"}]`)
			payload, _ := json.Marshal(map[string]any{
				"jsonrpc": "2.0",
				"id":      3,
				"result": map[string]any{
					"content": []map[string]any{{"type": "text", "text": string(text)}},
					"isError": false,
				},
			})
			fmt.Fprintf(w, "id:1\nevent:message\ndata:%s\n\n", payload)
		default:
			t.Fatalf("unexpected method %v", req["method"])
		}
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Search.DefaultProviderChain = []string{"bigmodel", "duckduckgo"}
	cfg.Providers["bigmodel"] = config.ProviderConfig{
		Type:         "mcp",
		AuthEnv:      "ZHIPU_APIKEY",
		EnabledIfEnv: "ZHIPU_APIKEY",
		Capabilities: []string{"search"},
		Search:       &config.ProviderEndpointConfig{URL: server.URL, Tool: "web_search_prime"},
	}
	s := &Search{
		engines: []Engine{
			&mockEngine{name: "duckduckgo", healthErr: errors.New("should not be called")},
		},
		config:    cfg.Search,
		providers: cfg.Providers,
	}

	resp, err := s.Do("test", SearchOptions{Provider: "auto"})

	require.NoError(t, err)
	assert.Equal(t, "bigmodel", resp.Engine)
	assert.Equal(t, "bigmodel", resp.Provider)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, "MCP Result", resp.Results[0].Title)
}

func TestSearchExplicitMCPProvider(t *testing.T) {
	t.Setenv("ZHIPU_APIKEY", "test-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream;charset=UTF-8")
		var req map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		switch req["method"] {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "test-session")
			fmt.Fprintf(w, "id:1\nevent:message\ndata:{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"protocolVersion\":\"2024-11-05\",\"capabilities\":{\"tools\":{\"listChanged\":true}},\"serverInfo\":{\"name\":\"mock\",\"version\":\"0.0.1\"}}}\n\n")
		case "notifications/initialized":
			fmt.Fprintf(w, "id:1\nevent:message\ndata:{\"jsonrpc\":\"2.0\",\"result\":{}}\n\n")
		case "tools/call":
			text, _ := json.Marshal(`[{"title":"Explicit MCP","link":"https://example.com","content":"via MCP","refer":"ref_1"}]`)
			payload, _ := json.Marshal(map[string]any{
				"jsonrpc": "2.0",
				"id":      3,
				"result": map[string]any{
					"content": []map[string]any{{"type": "text", "text": string(text)}},
					"isError": false,
				},
			})
			fmt.Fprintf(w, "id:1\nevent:message\ndata:%s\n\n", payload)
		}
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.Providers["bigmodel"] = config.ProviderConfig{
		Type:         "mcp",
		AuthEnv:      "ZHIPU_APIKEY",
		Capabilities: []string{"search"},
		Search:       &config.ProviderEndpointConfig{URL: server.URL, Tool: "web_search_prime"},
	}
	s := &Search{
		config:    cfg.Search,
		providers: cfg.Providers,
	}

	resp, err := s.Do("test", SearchOptions{Provider: "bigmodel"})

	require.NoError(t, err)
	assert.Equal(t, "bigmodel", resp.Engine)
	require.Len(t, resp.Results, 1)
	assert.Equal(t, "Explicit MCP", resp.Results[0].Title)
}

func TestSearchUnknownEngineError(t *testing.T) {
	s := makeTestSearch(&mockEngine{name: "duckduckgo"})

	_, err := s.Do("test", SearchOptions{Engine: "invalid"})

	require.Error(t, err)
	var appErr interface{ ExitCode() int }
	assert.ErrorAs(t, err, &appErr)
	assert.Contains(t, err.Error(), "unknown search engine")
}

func TestNormalizeResultURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"lowercase host", "HTTPS://Example.COM/Path", "https://example.com/Path"},
		{"drop fragment", "https://example.com/path#section", "https://example.com/path"},
		{"drop root slash", "https://example.com/", "https://example.com"},
		{"sort query", "https://example.com/path?b=2&a=1", "https://example.com/path?a=1&b=2"},
		{"keep meaningful query", "https://example.com/path?q=golang", "https://example.com/path?q=golang"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeResultURL(tt.in))
		})
	}
}

func TestDomainAllowed(t *testing.T) {
	assert.True(t, domainAllowed("docs.example.com", []string{"example.com"}, nil))
	assert.True(t, domainAllowed("www.example.com", []string{"example.com"}, nil))
	assert.False(t, domainAllowed("example.org", []string{"example.com"}, nil))
	assert.False(t, domainAllowed("docs.example.com", nil, []string{"example.com"}))
	assert.True(t, domainAllowed("example.org", nil, []string{"example.com"}))
}

func TestNormalizeResultsFiltersAndDedupes(t *testing.T) {
	raw := []RawResult{
		{Title: "A", URL: "https://Example.com/path#top", Snippet: "first", Source: "Example.com"},
		{Title: "A duplicate", URL: "https://example.com/path", Snippet: "duplicate", Source: "example.com"},
		{Title: "B", URL: "https://other.com/path", Snippet: "other", Source: "other.com"},
		{Title: "C", URL: "https://sub.example.com/path", Snippet: "sub", Source: "sub.example.com"},
	}

	results := normalizeResults(raw, "searxng", SearchOptions{
		IncludeDomains: []string{"example.com"},
	})

	require.Len(t, results, 2)
	assert.Equal(t, 1, results[0].Rank)
	assert.Equal(t, "https://example.com/path", results[0].URL)
	assert.Equal(t, "example.com", results[0].Source)
	assert.Equal(t, []string{"searxng"}, results[0].Engines)
	assert.Equal(t, 2, results[1].Rank)
	assert.Equal(t, "sub.example.com", results[1].Source)
}

func TestNormalizeResultsExcludeDomain(t *testing.T) {
	raw := []RawResult{
		{Title: "A", URL: "https://example.com/path", Source: "example.com"},
		{Title: "B", URL: "https://keep.com/path", Source: "keep.com"},
	}

	results := normalizeResults(raw, "duckduckgo", SearchOptions{
		ExcludeDomains: []string{"example.com"},
	})

	require.Len(t, results, 1)
	assert.Equal(t, "keep.com", results[0].Source)
}

func TestAppendUniquePreservesEngineProvenance(t *testing.T) {
	engines := []string{"searxng"}

	engines = appendUnique(engines, "duckduckgo")
	engines = appendUnique(engines, "searxng")

	assert.Equal(t, []string{"searxng", "duckduckgo"}, engines)
}
