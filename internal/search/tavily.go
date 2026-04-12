package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/koda-claw/web-tools/internal/config"
	apperrors "github.com/koda-claw/web-tools/internal/errors"
)

// TavilyEngine wraps the Tavily REST API and implements Engine.
type TavilyEngine struct {
	apiKey     string
	httpClient *http.Client
}

// tavilyRequest is the POST body for the Tavily search API.
type tavilyRequest struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results,omitempty"`
	Topic      string `json:"topic,omitempty"`
	TimeRange  string `json:"time_range,omitempty"`
	SearchDepth string `json:"search_depth,omitempty"`
}

// tavilyResult represents a single result from the Tavily API.
type tavilyResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

// tavilyResponse is the raw JSON response from Tavily.
type tavilyResponse struct {
	Results []tavilyResult `json:"results"`
}

// NewTavilyEngine creates a new Tavily engine with the given API key.
func NewTavilyEngine(apiKey string) *TavilyEngine {
	return &TavilyEngine{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: time.Duration(config.DefaultTimeout) * time.Second,
		},
	}
}

// Name returns the engine identifier.
func (e *TavilyEngine) Name() string { return "tavily" }

// HealthCheck validates that the API key is configured.
func (e *TavilyEngine) HealthCheck() error {
	if e.apiKey == "" {
		return apperrors.NewEngineError(
			"Tavily API key not configured",
			"TAVILY_API_KEY is empty",
			nil,
			[]string{"set TAVILY_API_KEY environment variable", "get a key at https://app.tavily.com"},
		)
	}
	return nil
}

// Query sends a search query to the Tavily API and returns normalized results.
func (e *TavilyEngine) Query(query string, opts SearchOptions) ([]RawResult, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 5
	}

	reqBody := tavilyRequest{
		Query:       query,
		MaxResults:  limit,
		SearchDepth: "basic",
	}

	// Map category to Tavily topic.
	switch opts.Category {
	case "news":
		reqBody.Topic = "news"
	case "general", "":
		reqBody.Topic = "general"
	default:
		reqBody.Topic = "general"
	}

	// Map time_range (Tavily supports: day, week, month, year).
	if opts.TimeRange != "" && opts.TimeRange != "any" {
		reqBody.TimeRange = opts.TimeRange
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Tavily request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.tavily.com/search", bytes.NewReader(body))
	if err != nil {
		return nil, apperrors.NewNetworkError(
			"Tavily search request build failed",
			err.Error(),
			nil,
			nil,
		)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, apperrors.NewNetworkError(
			"Tavily search request failed",
			err.Error(),
			map[string]string{"timeout": e.httpClient.Timeout.String()},
			[]string{"check network connectivity", "verify TAVILY_API_KEY is valid"},
		)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.NewNetworkError(
			"failed to read Tavily response body",
			err.Error(),
			nil,
			nil,
		)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, apperrors.NewEngineError(
			"Tavily returned non-200 status",
			fmt.Sprintf("HTTP %d, body: %s", resp.StatusCode, string(respBody)),
			map[string]string{"status_code": fmt.Sprintf("%d", resp.StatusCode)},
			[]string{"verify TAVILY_API_KEY is valid", "check API usage limits at https://app.tavily.com"},
		)
	}

	var tr tavilyResponse
	if err := json.Unmarshal(respBody, &tr); err != nil {
		return nil, apperrors.NewExtractError(
			"Tavily response parse failed",
			err.Error(),
			map[string]string{"body_length": fmt.Sprintf("%d", len(respBody))},
			[]string{"json.Unmarshal"},
			[]string{"check Tavily API response format"},
		)
	}

	results := make([]RawResult, 0, len(tr.Results))
	for _, r := range tr.Results {
		source := "unknown"
		if u, err := url.Parse(r.URL); err == nil && u.Hostname() != "" {
			source = u.Hostname()
		}
		results = append(results, RawResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
			Source:  source,
			Extra:   map[string]string{},
		})
	}

	return results, nil
}
