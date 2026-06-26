package search

import (
	"encoding/base64"
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
	bingSearchURL       = "https://www.bing.com/search"
	bingTimeout         = 10 * time.Second
	bingRequestInterval = 2 * time.Second
	bingRetryDelay      = 800 * time.Millisecond
)

var bingLimiter = newPoliteLimiter(bingRequestInterval)

// BingEngine queries Bing's server-rendered web search page.
type BingEngine struct {
	httpClient *http.Client
	baseURL    string
	limiter    *politeLimiter
}

// NewBingEngine creates a new BingEngine.
func NewBingEngine() *BingEngine {
	return &BingEngine{
		httpClient: &http.Client{Timeout: bingTimeout},
		baseURL:    bingSearchURL,
		limiter:    bingLimiter,
	}
}

// Name returns the engine identifier.
func (e *BingEngine) Name() string { return "bing" }

// HealthCheck always succeeds; live reachability is checked during Query.
func (e *BingEngine) HealthCheck() error { return nil }

// Query sends a search request to Bing and parses organic web results.
func (e *BingEngine) Query(query string, opts SearchOptions) ([]RawResult, error) {
	params := url.Values{}
	params.Set("q", query)
	if opts.Limit > 0 {
		params.Set("count", strconv.Itoa(min(opts.Limit+5, 20)))
	}
	if opts.Locale == "zh-CN" {
		params.Set("cc", "cn")
		params.Set("setlang", "zh-hans")
	} else if opts.Locale == "en-US" {
		params.Set("cc", "us")
		params.Set("setlang", "en")
	}

	reqURL := e.baseURL + "?" + params.Encode()
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, apperrors.NewNetworkError("Bing request build failed", err.Error(), map[string]string{"url": reqURL}, nil)
	}
	req.Header.Set("User-Agent", ddgUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", acceptLanguage(opts.Locale))

	body, statusCode, err := doPoliteSearchRequest(e.httpClient, req, politeRequestOptions{
		engine:       "bing",
		timeout:      bingTimeout,
		limiter:      e.limiter,
		blocked:      isBingBlocked,
		retryDelayFn: fixedRetryDelay(bingRetryDelay),
	})
	if err != nil {
		return nil, err
	}
	if statusCode != http.StatusOK {
		return nil, apperrors.NewEngineError(
			"Bing returned non-200 status",
			fmt.Sprintf("HTTP %d", statusCode),
			map[string]string{"url": reqURL, "status_code": strconv.Itoa(statusCode)},
			nil,
		)
	}

	bodyText := string(body)
	results, err := parseBingHTML(bodyText)
	if err != nil {
		return nil, err
	}
	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}
	return results, nil
}

func parseBingHTML(body string) ([]RawResult, error) {
	if isBingBlocked(http.StatusOK, body) {
		return nil, newSearchRateLimitError("bing", bingSearchURL, "captcha_or_blocked", http.StatusOK)
	}
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, apperrors.NewExtractError("failed to parse Bing HTML", err.Error(), nil, nil, nil)
	}

	var results []RawResult
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "li" && hasClass(n, "b_algo") {
			if result, ok := parseBingResult(n); ok {
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

func parseBingResult(n *html.Node) (RawResult, bool) {
	link := firstDescendant(n, func(candidate *html.Node) bool {
		return candidate.Type == html.ElementNode && candidate.Data == "a" && nearestAncestor(candidate, "h2") != nil
	})
	if link == nil {
		return RawResult{}, false
	}
	title := strings.TrimSpace(textContent(link))
	href := resolveBingURL(attrVal(link, "href"))
	if title == "" || href == "" {
		return RawResult{}, false
	}

	snippet := ""
	if caption := firstDescendant(n, func(candidate *html.Node) bool {
		return candidate.Type == html.ElementNode && candidate.Data == "p"
	}); caption != nil {
		snippet = strings.TrimSpace(textContent(caption))
	}

	source := ""
	if u, err := url.Parse(href); err == nil {
		source = u.Hostname()
	}
	return RawResult{Title: title, URL: href, Snippet: snippet, Source: source, Extra: map[string]string{}}, true
}

func resolveBingURL(href string) string {
	href = strings.TrimSpace(html.UnescapeString(href))
	if !strings.Contains(href, "bing.com/ck/a") {
		return href
	}
	u, err := url.Parse(href)
	if err != nil {
		return href
	}
	encoded := u.Query().Get("u")
	if encoded == "" {
		return href
	}
	encoded = strings.TrimPrefix(encoded, "a1")
	encoded = strings.ReplaceAll(encoded, "-", "+")
	encoded = strings.ReplaceAll(encoded, "_", "/")
	if pad := len(encoded) % 4; pad != 0 {
		encoded += strings.Repeat("=", 4-pad)
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return href
	}
	return string(decoded)
}

func isBingBlocked(statusCode int, body string) bool {
	if statusCode == http.StatusTooManyRequests || statusCode == http.StatusForbidden {
		return true
	}
	lower := strings.ToLower(body)
	if len(body) < 500 && (strings.Contains(lower, "captcha") || strings.Contains(lower, "verify")) {
		return true
	}
	blockedMarkers := []string{
		"captcha",
		"our systems have detected unusual traffic",
		"verify you are a human",
		"enable cookies to continue",
		"bing.com/ck/a?u=a1ahr0chm6ly93d3cuymluzy5jb20v",
	}
	for _, marker := range blockedMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func acceptLanguage(locale string) string {
	switch locale {
	case "zh-CN":
		return "zh-CN,zh;q=0.9,en;q=0.8"
	case "en-US":
		return "en-US,en;q=0.9"
	default:
		return "en-US,en;q=0.9,zh-CN;q=0.6,zh;q=0.5"
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
