package search

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/koda-claw/web-tools/internal/config"
	apperrors "github.com/koda-claw/web-tools/internal/errors"
	"github.com/koda-claw/web-tools/internal/provider"
	mcpprovider "github.com/koda-claw/web-tools/internal/provider/mcp"
)

// Search is the main entry point for web search.
type Search struct {
	engines   []Engine
	config    config.SearchConfig
	providers map[string]config.ProviderConfig
	cache     *searchCache
}

type searchCache struct {
	mu       sync.Mutex
	entries  map[string]searchCacheEntry
	cacheTTL time.Duration
}

type searchCacheEntry struct {
	response *SearchResponse
	cachedAt time.Time
}

var defaultSearchCache = &searchCache{
	entries:  make(map[string]searchCacheEntry),
	cacheTTL: time.Duration(config.DefaultCacheTTL) * time.Second,
}

// SearchOptions holds user-facing search options.
type SearchOptions struct {
	Limit     int
	Locale    string // "auto" / "zh-CN" / "en-US"
	Category  string // "general" / "images" / "news" / "videos" / "files"
	TimeRange string // "" / "any" / "day" / "week" / "month" / "year"
	Engine    string // "auto" / "duckduckgo" / "searxng"
	Provider  string // "auto" / provider id
	NoCache   bool
	// IncludeDomains and ExcludeDomains filter normalized result hostnames.
	// A filter value matches the exact domain and any subdomain.
	IncludeDomains []string
	ExcludeDomains []string
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
	Query         string             `json:"query"`
	Engine        string             `json:"engine"`
	Provider      string             `json:"provider,omitempty"`
	ProviderChain []provider.Attempt `json:"provider_chain,omitempty"`
	Locale        string             `json:"locale"`
	Total         int                `json:"total"`
	Results       []SearchResult     `json:"results"`
	SearchedAt    time.Time          `json:"searched_at"`
}

// NewSearch creates a new Search instance with all supported engines.
// Engine order determines auto-mode priority: SearXNG first, DDG as fallback.
func NewSearch(cfg config.SearchConfig) *Search {
	return &Search{
		engines: []Engine{
			NewSearXNGEngine(cfg.SearXNGURL),
			NewDuckDuckGoEngine(),
		},
		config:    cfg,
		providers: config.DefaultConfig().Providers,
		cache:     defaultSearchCache,
	}
}

// NewSearchWithConfig creates a search instance with top-level provider config.
func NewSearchWithConfig(cfg config.Config) *Search {
	return &Search{
		engines: []Engine{
			NewSearXNGEngine(cfg.Search.SearXNGURL),
			NewDuckDuckGoEngine(),
		},
		config:    cfg.Search,
		providers: cfg.Providers,
		cache:     defaultSearchCache,
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
	if opts.Locale == "" {
		opts.Locale = s.config.DefaultLocale
	}
	if opts.Locale == "" {
		opts.Locale = "auto"
	}
	if opts.Category == "" {
		opts.Category = "general"
	}
	if opts.TimeRange == "" {
		opts.TimeRange = "any"
	}

	engineName := opts.Engine
	providerMode := false
	if opts.Provider != "" {
		engineName = opts.Provider
		providerMode = true
	}
	if engineName == "" {
		engineName = s.config.DefaultProvider
		if engineName != "" {
			providerMode = true
		}
	}
	if engineName == "" {
		engineName = s.config.DefaultEngine
	}
	if engineName == "" {
		engineName = "auto"
	}

	cacheKey := s.cacheKey(query, opts, engineName, providerMode)
	if !opts.NoCache {
		if cached, ok := s.getCached(cacheKey); ok {
			return cached, nil
		}
	}

	// Select engines to try based on the requested mode.
	var candidates []Engine
	var providerAttempts []provider.Attempt
	switch engineName {
	case "auto":
		candidates = s.engines
		if providerMode && len(s.config.DefaultProviderChain) > 0 {
			reg, err := provider.NewRegistry(s.providers)
			if err != nil {
				return nil, err
			}
			providers, attempts, err := reg.ResolveChain(s.config.DefaultProviderChain, provider.CapabilitySearch)
			providerAttempts = attempts
			if err != nil {
				return nil, err
			}
			candidates = enginesForProviders(s.engines, providers)
			if len(candidates) == 0 {
				return nil, apperrors.NewInputError(
					"no search providers available",
					"provider chain did not resolve to any enabled search providers",
					[]string{"check search.default_provider_chain", "configure provider auth envs"},
				)
			}
		}
	default:
		if providerMode {
			reg, err := provider.NewRegistry(s.providers)
			if err != nil {
				return nil, err
			}
			resolved, err := reg.Get(engineName, provider.CapabilitySearch)
			if err != nil {
				return nil, err
			}
			providerAttempts = []provider.Attempt{{Provider: resolved.ID, Status: provider.AttemptStatusSelected}}
			candidates = enginesForProviders(s.engines, []provider.Provider{resolved})
			if len(candidates) == 0 {
				return nil, apperrors.NewInputError(
					"search provider is not implemented",
					fmt.Sprintf("provider %q is configured but no search adapter is available", engineName),
					[]string{"use --provider auto", "check provider type and endpoint configuration"},
				)
			}
		} else {
			for _, e := range s.engines {
				if e.Name() == engineName {
					candidates = []Engine{e}
					break
				}
			}
			if len(candidates) == 0 {
				return nil, apperrors.NewInputError(
					"unknown search engine",
					fmt.Sprintf("got %q; supported: auto, duckduckgo, searxng", engineName),
					[]string{"use --engine auto", "use --engine duckduckgo", "use --engine searxng"},
				)
			}
		}
	}

	var (
		results             []SearchResult
		usedEngine          string
		lastErr             error
		lastEmptyResults    []SearchResult
		lastEmptyEngine     string
		ddgFallback         bool
		ddgFallbackEmptyHit bool
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

		rawResults, err := e.Query(query, opts)
		if err != nil {
			if engineName == "auto" {
				if errors.Is(err, ErrRateLimited) {
					fmt.Fprintf(os.Stderr, "[web-search] engine %s rate-limited; trying next engine\n", e.Name())
				} else {
					fmt.Fprintf(os.Stderr, "[web-search] engine %s query failed: %v\n", e.Name(), err)
				}
				lastErr = err
				continue
			}
			return nil, err
		}

		normalizedResults := normalizeResults(rawResults, e.Name(), opts)
		if engineName == "auto" && len(normalizedResults) == 0 && len(candidates) > 1 {
			fmt.Fprintf(os.Stderr, "[web-search] engine %s returned no results; trying next engine\n", e.Name())
			lastEmptyResults = normalizedResults
			lastEmptyEngine = e.Name()
			continue
		}

		results = normalizedResults
		usedEngine = e.Name()
		// Track when auto mode fell back to DDG so we can warn about limitations.
		if engineName == "auto" && e.Name() == "duckduckgo" && len(candidates) > 1 {
			ddgFallback = true
			ddgFallbackEmptyHit = lastEmptyEngine != ""
		}
		break
	}

	if usedEngine == "" {
		if lastEmptyEngine != "" {
			results = lastEmptyResults
			usedEngine = lastEmptyEngine
		} else if lastErr != nil {
			return nil, lastErr
		} else {
			return nil, fmt.Errorf("all search engines failed")
		}
	}

	if ddgFallback {
		if ddgFallbackEmptyHit {
			fmt.Fprintf(os.Stderr, "[web-search] warning: fell back to DuckDuckGo Lite because a prior engine returned no results\n")
		}
		if opts.Category != "" && opts.Category != "general" {
			fmt.Fprintf(os.Stderr, "[web-search] warning: fell back to DuckDuckGo Lite; --category %q is not supported and was ignored\n", opts.Category)
		}
		if opts.TimeRange != "" && opts.TimeRange != "any" {
			fmt.Fprintf(os.Stderr, "[web-search] warning: fell back to DuckDuckGo Lite; --time-range %q is not supported and was ignored\n", opts.TimeRange)
		}
	}

	resp := &SearchResponse{
		Query:         query,
		Engine:        usedEngine,
		Provider:      usedEngine,
		ProviderChain: providerAttempts,
		Locale:        opts.Locale,
		Total:         len(results),
		Results:       results,
		SearchedAt:    time.Now(),
	}
	if !opts.NoCache {
		s.setCached(cacheKey, resp)
	}
	return resp, nil
}

func (s *Search) getCached(key string) (*SearchResponse, bool) {
	cache := s.searchCache()
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.entries == nil {
		cache.entries = make(map[string]searchCacheEntry)
	}
	if cache.cacheTTL <= 0 {
		cache.cacheTTL = time.Duration(config.DefaultCacheTTL) * time.Second
	}
	entry, ok := cache.entries[key]
	if !ok {
		return nil, false
	}
	if time.Since(entry.cachedAt) > cache.cacheTTL {
		delete(cache.entries, key)
		return nil, false
	}
	return cloneSearchResponse(entry.response), true
}

func (s *Search) setCached(key string, resp *SearchResponse) {
	cache := s.searchCache()
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.entries == nil {
		cache.entries = make(map[string]searchCacheEntry)
	}
	if cache.cacheTTL <= 0 {
		cache.cacheTTL = time.Duration(config.DefaultCacheTTL) * time.Second
	}
	cache.entries[key] = searchCacheEntry{
		response: cloneSearchResponse(resp),
		cachedAt: time.Now(),
	}
}

func (s *Search) searchCache() *searchCache {
	if s.cache == nil {
		s.cache = &searchCache{
			entries:  make(map[string]searchCacheEntry),
			cacheTTL: time.Duration(config.DefaultCacheTTL) * time.Second,
		}
	}
	return s.cache
}

func (s *Search) cacheKey(query string, opts SearchOptions, engineName string, providerMode bool) string {
	include := append([]string(nil), opts.IncludeDomains...)
	exclude := append([]string(nil), opts.ExcludeDomains...)
	sort.Strings(include)
	sort.Strings(exclude)
	parts := []string{
		strings.TrimSpace(query),
		opts.Locale,
		opts.Category,
		opts.TimeRange,
		engineName,
		fmt.Sprintf("provider_mode=%t", providerMode),
		fmt.Sprintf("limit=%d", opts.Limit),
		strings.Join(include, ","),
		strings.Join(exclude, ","),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return fmt.Sprintf("%x", sum[:])
}

func cloneSearchResponse(resp *SearchResponse) *SearchResponse {
	if resp == nil {
		return nil
	}
	clone := *resp
	if resp.ProviderChain != nil {
		clone.ProviderChain = append([]provider.Attempt(nil), resp.ProviderChain...)
	}
	if resp.Results != nil {
		clone.Results = make([]SearchResult, len(resp.Results))
		for i, result := range resp.Results {
			clone.Results[i] = result
			if result.Engines != nil {
				clone.Results[i].Engines = append([]string(nil), result.Engines...)
			}
		}
	}
	return &clone
}

func enginesForProviders(engines []Engine, providers []provider.Provider) []Engine {
	out := make([]Engine, 0, len(providers))
	for _, p := range providers {
		if p.Config.Type == "mcp" && p.Config.Search != nil {
			out = append(out, &mcpSearchEngine{
				id:     p.ID,
				client: mcpprovider.NewClient(p.Config, os.Getenv(p.Config.AuthEnv)),
			})
			continue
		}
		for _, e := range engines {
			if e.Name() == p.ID {
				out = append(out, e)
				break
			}
		}
	}
	return out
}

type mcpSearchEngine struct {
	id     string
	client *mcpprovider.Client
}

func (e *mcpSearchEngine) Name() string {
	return e.id
}

func (e *mcpSearchEngine) HealthCheck() error {
	return nil
}

func (e *mcpSearchEngine) Query(query string, opts SearchOptions) ([]RawResult, error) {
	results, err := e.client.Search(context.Background(), query, mcpprovider.SearchOptions{
		Locale:         opts.Locale,
		TimeRange:      opts.TimeRange,
		IncludeDomains: opts.IncludeDomains,
	})
	if err != nil {
		return nil, err
	}
	raw := make([]RawResult, 0, len(results))
	for _, result := range results {
		raw = append(raw, RawResult{
			Title:   result.Title,
			URL:     result.URL,
			Snippet: result.Snippet,
			Source:  result.Source,
		})
	}
	return raw, nil
}

func normalizeResults(rawResults []RawResult, engine string, opts SearchOptions) []SearchResult {
	merged := make([]SearchResult, 0, len(rawResults))
	seen := make(map[string]int)

	for _, raw := range rawResults {
		normalizedURL := normalizeResultURL(raw.URL)
		host := resultHost(normalizedURL)
		if host == "" {
			host = normalizeDomain(raw.Source)
		}
		if !domainAllowed(host, opts.IncludeDomains, opts.ExcludeDomains) {
			continue
		}

		key := normalizedURL
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(raw.URL))
		}
		if idx, ok := seen[key]; ok {
			merged[idx].Engines = appendUnique(merged[idx].Engines, engine)
			continue
		}

		result := SearchResult{
			Title:         raw.Title,
			URL:           normalizedURL,
			Snippet:       raw.Snippet,
			Source:        host,
			Engines:       []string{engine},
			PublishedDate: raw.Extra["published_date"],
		}
		if result.URL == "" {
			result.URL = raw.URL
		}
		if result.Source == "" {
			result.Source = raw.Source
		}
		seen[key] = len(merged)
		merged = append(merged, result)
	}

	for i := range merged {
		merged[i].Rank = i + 1
	}
	return merged
}

func normalizeResultURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return rawURL
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	if (u.Scheme == "https" && strings.HasSuffix(u.Host, ":443")) ||
		(u.Scheme == "http" && strings.HasSuffix(u.Host, ":80")) {
		host, _, err := net.SplitHostPort(u.Host)
		if err == nil {
			u.Host = host
		}
	}
	if u.Path == "/" {
		u.Path = ""
	}
	if u.RawQuery != "" {
		values := u.Query()
		keys := make([]string, 0, len(values))
		for key := range values {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		ordered := url.Values{}
		for _, key := range keys {
			vals := append([]string(nil), values[key]...)
			sort.Strings(vals)
			for _, val := range vals {
				ordered.Add(key, val)
			}
		}
		u.RawQuery = ordered.Encode()
	}
	return u.String()
}

func resultHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return normalizeDomain(u.Hostname())
}

func normalizeDomain(domain string) string {
	domain = strings.ToLower(strings.TrimSpace(domain))
	domain = strings.TrimPrefix(domain, "www.")
	return domain
}

func domainAllowed(host string, includeDomains []string, excludeDomains []string) bool {
	host = normalizeDomain(host)
	if host == "" {
		return len(includeDomains) == 0
	}
	for _, domain := range excludeDomains {
		if domainMatches(host, domain) {
			return false
		}
	}
	if len(includeDomains) == 0 {
		return true
	}
	for _, domain := range includeDomains {
		if domainMatches(host, domain) {
			return true
		}
	}
	return false
}

func domainMatches(host string, filter string) bool {
	filter = normalizeDomain(filter)
	if filter == "" {
		return false
	}
	return host == filter || strings.HasSuffix(host, "."+filter)
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
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
