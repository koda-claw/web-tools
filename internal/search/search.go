package search

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/koda-claw/web-tools/internal/config"
)

// Search is the main entry point for web search.
type Search struct {
	engines []Engine
	config  config.SearchConfig
}

// SearchOptions holds user-facing search options.
type SearchOptions struct {
	Limit     int
	Locale    string // "auto" / "zh-CN" / "en-US"
	Category  string // "general" / "images" / "news" / "videos" / "files"
	TimeRange string // "" / "any" / "day" / "week" / "month" / "year"
	Engine    string // "auto" / "duckduckgo" / "searxng"
}

// SearchResult is a single normalized search result.
type SearchResult struct {
	Rank          int      `json:"rank"`
	Title         string   `json:"title"`
	URL           string   `json:"url"`
	Snippet       string   `json:"snippet"`
	Source        string   `json:"source"`
	Engines       []string `json:"engines"`
	PublishedDate string   `json:"published_date,omitempty"`
}

// SearchResponse is the final output structure.
type SearchResponse struct {
	Query      string         `json:"query"`
	Engine     string         `json:"engine"`
	Locale     string         `json:"locale"`
	Total      int            `json:"total"`
	Results    []SearchResult `json:"results"`
	SearchedAt time.Time      `json:"searched_at"`
}

// NewSearch creates a new Search instance with all supported engines.
// Engine order determines auto-mode priority: SearXNG first, DDG as fallback.
func NewSearch(cfg config.SearchConfig) *Search {
	return &Search{
		engines: []Engine{
			NewSearXNGEngine(cfg.SearXNGURL),
			NewDuckDuckGoEngine(),
		},
		config: cfg,
	}
}

// Do performs a search using the requested engine strategy.
//
// Engine selection (opts.Engine, falling back to config.DefaultEngine, then "auto"):
//   - "auto": try engines in order; skip unavailable ones (logged to stderr)
//   - "searxng": use SearXNG only
//   - "duckduckgo": use DuckDuckGo Lite only
func (s *Search) Do(query string, opts SearchOptions) (*SearchResponse, error) {
	// Apply defaults
	if opts.Limit <= 0 {
		opts.Limit = s.config.DefaultLimit
	}
	if opts.Category == "" {
		opts.Category = "general"
	}

	engineName := opts.Engine
	if engineName == "" {
		engineName = s.config.DefaultEngine
	}
	if engineName == "" {
		engineName = "auto"
	}

	// Select engines to try based on the requested mode.
	var candidates []Engine
	switch engineName {
	case "auto":
		candidates = s.engines
	default:
		for _, e := range s.engines {
			if e.Name() == engineName {
				candidates = []Engine{e}
				break
			}
		}
		if len(candidates) == 0 {
			return nil, fmt.Errorf("unknown engine %q; supported: auto, duckduckgo, searxng", engineName)
		}
	}

	var (
		rawResults []RawResult
		usedEngine string
		lastErr    error
		ddgFallback bool
	)

	for _, e := range candidates {
		if err := e.HealthCheck(); err != nil {
			if engineName == "auto" {
				fmt.Fprintf(os.Stderr, "[web-search] engine %s unavailable: %v\n", e.Name(), err)
				lastErr = err
				continue
			}
			return nil, err
		}

		results, err := e.Query(query, opts)
		if err != nil {
			if engineName == "auto" {
				fmt.Fprintf(os.Stderr, "[web-search] engine %s query failed: %v\n", e.Name(), err)
				lastErr = err
				continue
			}
			return nil, err
		}

		rawResults = results
		usedEngine = e.Name()
		// Track when auto mode fell back to DDG so we can warn about limitations.
		if engineName == "auto" && e.Name() == "duckduckgo" && len(candidates) > 1 {
			ddgFallback = true
		}
		break
	}

	if usedEngine == "" {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, fmt.Errorf("all search engines failed")
	}

	if ddgFallback {
		if opts.Category != "" && opts.Category != "general" {
			fmt.Fprintf(os.Stderr, "[web-search] warning: fell back to DuckDuckGo Lite; --category %q is not supported and was ignored\n", opts.Category)
		}
		if opts.TimeRange != "" && opts.TimeRange != "any" {
			fmt.Fprintf(os.Stderr, "[web-search] warning: fell back to DuckDuckGo Lite; --time-range %q is not supported and was ignored\n", opts.TimeRange)
		}
	}

	// Normalize results into the public SearchResult type.
	results := make([]SearchResult, 0, len(rawResults))
	for i, r := range rawResults {
		results = append(results, SearchResult{
			Rank:          i + 1,
			Title:         r.Title,
			URL:           r.URL,
			Snippet:       r.Snippet,
			Source:        r.Source,
			Engines:       []string{usedEngine},
			PublishedDate: r.Extra["published_date"],
		})
	}

	locale := opts.Locale
	if locale == "" {
		locale = "auto"
	}

	return &SearchResponse{
		Query:      query,
		Engine:     usedEngine,
		Locale:     locale,
		Total:      len(results),
		Results:    results,
		SearchedAt: time.Now(),
	}, nil
}

// RenderMarkdown outputs the search response as Markdown.
func (r *SearchResponse) RenderMarkdown() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Search: \"%s\"\n", r.Query))
	sb.WriteString(fmt.Sprintf("> Engine: %s | Locale: %s | Results: %d | %s\n\n",
		r.Engine, r.Locale, r.Total, r.SearchedAt.Format(time.RFC3339)))

	for _, result := range r.Results {
		sb.WriteString(fmt.Sprintf("### %d. %s\n", result.Rank, result.Title))
		sb.WriteString(fmt.Sprintf("**Source:** %s\n", result.Source))
		sb.WriteString(fmt.Sprintf("**URL:** %s\n", result.URL))
		if result.PublishedDate != "" {
			sb.WriteString(fmt.Sprintf("**Published:** %s\n", result.PublishedDate))
		}
		if result.Snippet != "" {
			sb.WriteString(fmt.Sprintf("**Snippet:** %s\n", result.Snippet))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// RenderJSON outputs the search response as JSON.
func (r *SearchResponse) RenderJSON() string {
	type jsonOutput struct {
		OK     bool            `json:"ok"`
		Result *SearchResponse `json:"result"`
	}
	resp := jsonOutput{OK: true, Result: r}
	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		data, _ = json.Marshal(resp)
	}
	return string(data)
}
