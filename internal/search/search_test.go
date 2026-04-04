package search

import (
	"encoding/json"
	"errors"
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
}

func (m *mockEngine) Name() string { return m.name }
func (m *mockEngine) HealthCheck() error { return m.healthErr }
func (m *mockEngine) Query(_ string, _ SearchOptions) ([]RawResult, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	return m.queryResult, nil
}

func makeTestSearch(engines ...Engine) *Search {
	return &Search{
		engines: engines,
		config: config.SearchConfig{
			DefaultLimit:  5,
			DefaultLocale: "auto",
			DefaultEngine: "auto",
		},
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

// TestAutoMode_AllEnginesFail verifies that an error is returned when all engines fail.
func TestAutoMode_AllEnginesFail(t *testing.T) {
	s := makeTestSearch(
		&mockEngine{name: "searxng", healthErr: errors.New("down")},
		&mockEngine{name: "duckduckgo", healthErr: errors.New("network error")},
	)

	_, err := s.Do("test query", SearchOptions{Engine: "auto"})
	assert.Error(t, err)
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
