package search

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/koda-claw/web-tools/internal/config"
	apperrors "github.com/koda-claw/web-tools/internal/errors"
)

// SearXNGEngine wraps the local SearXNG JSON API and implements Engine.
type SearXNGEngine struct {
	baseURL    string
	httpClient *http.Client
	userAgent  string
}

// searxngResult represents a single result from SearXNG API (private).
type searxngResult struct {
	Title         string          `json:"title"`
	URL           string          `json:"url"`
	Content       string          `json:"content"` // snippet
	Engines       []string        `json:"engines"`
	ParsedURL     json.RawMessage `json:"parsed_url"`
	PublishedDate *string         `json:"publishedDate"`
}

// searxngResponse is the raw JSON response from SearXNG (private).
type searxngResponse struct {
	Results  []searxngResult `json:"results"`
	NumberOf int             `json:"number_of_results"`
}

// NewSearXNGEngine creates a new SearXNG engine for the given base URL.
func NewSearXNGEngine(baseURL string) *SearXNGEngine {
	return &SearXNGEngine{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: time.Duration(config.DefaultTimeout) * time.Second,
		},
		userAgent: "web-tools/1.0 (agent CLI)",
	}
}

// Name returns the engine identifier.
func (e *SearXNGEngine) Name() string { return "searxng" }

// HealthCheck verifies that the SearXNG instance is reachable.
func (e *SearXNGEngine) HealthCheck() error {
	client := &http.Client{Timeout: config.HealthCheckTimeout}
	req, err := http.NewRequest("HEAD", e.baseURL, nil)
	if err != nil {
		return apperrors.NewEngineError(
			"SearXNG health check request failed",
			err.Error(),
			map[string]string{"searxng_url": e.baseURL},
			[]string{
				fmt.Sprintf("check SearXNG: curl -s -o /dev/null -w %%{http_code} %s", e.baseURL),
				"start SearXNG: cd docker && docker compose up -d",
			},
		)
	}
	req.Header.Set("User-Agent", e.userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return apperrors.NewEngineError(
			"SearXNG unreachable",
			err.Error(),
			map[string]string{"searxng_url": e.baseURL, "timeout": config.HealthCheckTimeout.String()},
			[]string{
				"start SearXNG: cd docker && docker compose up -d",
				"check Docker: docker ps",
				fmt.Sprintf("confirm port config: %s", e.baseURL),
			},
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return apperrors.NewEngineError(
			"SearXNG service error",
			fmt.Sprintf("HTTP %d", resp.StatusCode),
			map[string]string{"searxng_url": e.baseURL, "status_code": fmt.Sprintf("%d", resp.StatusCode)},
			[]string{"restart SearXNG: cd docker && docker compose restart", "logs: cd docker && docker compose logs"},
		)
	}

	return nil
}

// Query sends a search query to SearXNG and returns normalized results.
func (e *SearXNGEngine) Query(query string, opts SearchOptions) ([]RawResult, error) {
	params := url.Values{}
	params.Set("q", query)
	params.Set("format", "json")
	params.Set("categories", opts.Category)
	if opts.Locale != "" && opts.Locale != "auto" {
		params.Set("language", opts.Locale)
	}
	if opts.TimeRange != "" && opts.TimeRange != "any" {
		params.Set("time_range", opts.TimeRange)
	}

	reqURL := e.baseURL + "/search?" + params.Encode()
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, apperrors.NewNetworkError(
			"search request build failed",
			err.Error(),
			map[string]string{"url": reqURL},
			nil,
		)
	}
	req.Header.Set("User-Agent", e.userAgent)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, apperrors.NewNetworkError(
			"SearXNG search request failed",
			err.Error(),
			map[string]string{"url": reqURL, "timeout": e.httpClient.Timeout.String()},
			[]string{"check network", "confirm SearXNG container is running"},
		)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.NewNetworkError(
			"failed to read SearXNG response body",
			err.Error(),
			map[string]string{"url": reqURL},
			nil,
		)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, apperrors.NewEngineError(
			"SearXNG returned non-200 status",
			fmt.Sprintf("HTTP %d, body: %s", resp.StatusCode, string(body)),
			map[string]string{"url": reqURL, "status_code": fmt.Sprintf("%d", resp.StatusCode)},
			[]string{"SearXNG logs: cd docker && docker compose logs", "try a simple query"},
		)
	}

	var sxr searxngResponse
	if err := json.Unmarshal(body, &sxr); err != nil {
		return nil, apperrors.NewExtractError(
			"SearXNG response parse failed",
			err.Error(),
			map[string]string{"url": reqURL, "body_length": fmt.Sprintf("%d", len(body))},
			[]string{"json.Unmarshal"},
			[]string{"check SearXNG version compatibility", "check SearXNG logs"},
		)
	}

	// Apply limit
	raw := sxr.Results
	if opts.Limit > 0 && len(raw) > opts.Limit {
		raw = raw[:opts.Limit]
	}

	results := make([]RawResult, 0, len(raw))
	for _, r := range raw {
		extra := map[string]string{}
		if r.PublishedDate != nil {
			extra["published_date"] = *r.PublishedDate
		}
		results = append(results, RawResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
			Source:  extractSearXNGSource(r),
			Extra:   extra,
		})
	}

	return results, nil
}

// extractSearXNGSource gets the source domain from a searxngResult.
func extractSearXNGSource(r searxngResult) string {
	if len(r.ParsedURL) > 0 {
		var strArr []string
		if err := json.Unmarshal(r.ParsedURL, &strArr); err == nil && len(strArr) >= 2 {
			if host := strArr[1]; host != "" {
				return host
			}
		}
		var objArr []struct {
			Netloc string `json:"netloc"`
		}
		if err := json.Unmarshal(r.ParsedURL, &objArr); err == nil && len(objArr) > 0 && objArr[0].Netloc != "" {
			return objArr[0].Netloc
		}
	}
	if u, err := url.Parse(r.URL); err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	return "unknown"
}
