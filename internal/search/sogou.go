package search

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	apperrors "github.com/koda-claw/web-tools/internal/errors"
	"golang.org/x/net/html"
)

const (
	sogouSearchURL       = "https://www.sogou.com/web"
	sogouTimeout         = 10 * time.Second
	sogouRequestInterval = 4 * time.Second
	sogouRetryDelay      = 1 * time.Second
)

var sogouLimiter = newPoliteLimiter(sogouRequestInterval)

// SogouEngine queries Sogou's server-rendered web search page.
type SogouEngine struct {
	httpClient *http.Client
	baseURL    string
	limiter    *politeLimiter
}

// NewSogouEngine creates a new SogouEngine.
func NewSogouEngine() *SogouEngine {
	return &SogouEngine{
		httpClient: &http.Client{Timeout: sogouTimeout},
		baseURL:    sogouSearchURL,
		limiter:    sogouLimiter,
	}
}

// Name returns the engine identifier.
func (e *SogouEngine) Name() string { return "sogou" }

// HealthCheck always succeeds; live reachability is checked during Query.
func (e *SogouEngine) HealthCheck() error { return nil }

// Query sends a search request to Sogou and parses organic web results.
func (e *SogouEngine) Query(query string, opts SearchOptions) ([]RawResult, error) {
	params := url.Values{}
	params.Set("query", query)
	if opts.Limit > 0 {
		params.Set("num", strconv.Itoa(min(opts.Limit+5, 20)))
	}

	reqURL := e.baseURL + "?" + params.Encode()
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, apperrors.NewNetworkError("Sogou request build failed", err.Error(), map[string]string{"url": reqURL}, nil)
	}
	req.Header.Set("User-Agent", ddgUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Referer", "https://www.sogou.com/")

	body, statusCode, err := doPoliteSearchRequest(e.httpClient, req, politeRequestOptions{
		engine:       "sogou",
		timeout:      sogouTimeout,
		limiter:      e.limiter,
		blocked:      isSogouBlocked,
		retryDelayFn: fixedRetryDelay(sogouRetryDelay),
	})
	if err != nil {
		return nil, err
	}
	if statusCode != http.StatusOK {
		return nil, apperrors.NewEngineError(
			"Sogou returned non-200 status",
			fmt.Sprintf("HTTP %d", statusCode),
			map[string]string{"url": reqURL, "status_code": strconv.Itoa(statusCode)},
			nil,
		)
	}

	bodyText := string(body)
	results, err := parseSogouHTML(bodyText)
	if err != nil {
		return nil, err
	}
	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}
	return results, nil
}

func parseSogouHTML(body string) ([]RawResult, error) {
	if isSogouBlocked(http.StatusOK, body) {
		return nil, newSearchRateLimitError("sogou", sogouSearchURL, "captcha_or_blocked", http.StatusOK)
	}
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, apperrors.NewExtractError("failed to parse Sogou HTML", err.Error(), nil, nil, nil)
	}

	var results []RawResult
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && isSogouResultContainer(n) {
			if result, ok := parseSogouResult(n); ok {
				results = append(results, result)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return results, nil
}

func isSogouResultContainer(n *html.Node) bool {
	if n.Data != "div" {
		return false
	}
	id := attrVal(n, "id")
	if strings.HasPrefix(id, "sogou_vr_") || strings.HasPrefix(id, "vr_") {
		return true
	}
	return hasClass(n, "vrwrap") || hasClass(n, "results") || hasClass(n, "rb")
}

func parseSogouResult(n *html.Node) (RawResult, bool) {
	link := firstDescendant(n, func(candidate *html.Node) bool {
		if candidate.Type != html.ElementNode || candidate.Data != "a" {
			return false
		}
		if nearestAncestor(candidate, "h3") != nil {
			return true
		}
		return hasClass(candidate, "title")
	})
	if link == nil {
		return RawResult{}, false
	}
	title := strings.TrimSpace(textContent(link))
	href := resolveSogouURL(attrVal(link, "href"))
	if title == "" || href == "" {
		return RawResult{}, false
	}

	snippet := ""
	if desc := firstDescendant(n, func(candidate *html.Node) bool {
		return candidate.Type == html.ElementNode &&
			(candidate.Data == "p" || hasClass(candidate, "str_info") || hasClass(candidate, "ft") || hasClass(candidate, "text-layout"))
	}); desc != nil {
		snippet = compactSnippet(strings.TrimSpace(textContent(desc)))
	}

	source := ""
	if u, err := url.Parse(href); err == nil {
		source = u.Hostname()
	}
	return RawResult{Title: title, URL: href, Snippet: snippet, Source: source, Extra: map[string]string{}}, true
}

func resolveSogouURL(href string) string {
	href = strings.TrimSpace(html.UnescapeString(href))
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "//") {
		return "https:" + href
	}
	if strings.HasPrefix(href, "/") {
		return "https://www.sogou.com" + href
	}
	return href
}

func isSogouBlocked(statusCode int, body string) bool {
	if statusCode == http.StatusTooManyRequests || statusCode == http.StatusForbidden {
		return true
	}
	lower := strings.ToLower(body)
	blockedMarkers := []string{
		"此验证码用于确认",
		"请输入验证码",
		"sourceverifycode",
		"anti.min.css",
		"verify.css",
		"captcha",
		"seccode",
	}
	for _, marker := range blockedMarkers {
		if strings.Contains(lower, strings.ToLower(marker)) {
			return true
		}
	}
	return false
}
