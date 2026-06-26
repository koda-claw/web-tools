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
	baiduSearchURL       = "https://www.baidu.com/s"
	baiduTimeout         = 10 * time.Second
	baiduRequestInterval = 3 * time.Second
	baiduRetryDelay      = 1 * time.Second
)

var baiduLimiter = newPoliteLimiter(baiduRequestInterval)

// BaiduEngine queries Baidu's server-rendered web search page.
type BaiduEngine struct {
	httpClient *http.Client
	baseURL    string
	limiter    *politeLimiter
}

// NewBaiduEngine creates a new BaiduEngine.
func NewBaiduEngine() *BaiduEngine {
	return &BaiduEngine{
		httpClient: &http.Client{Timeout: baiduTimeout},
		baseURL:    baiduSearchURL,
		limiter:    baiduLimiter,
	}
}

// Name returns the engine identifier.
func (e *BaiduEngine) Name() string { return "baidu" }

// HealthCheck always succeeds; live reachability is checked during Query.
func (e *BaiduEngine) HealthCheck() error { return nil }

// Query sends a search request to Baidu and parses organic web results.
func (e *BaiduEngine) Query(query string, opts SearchOptions) ([]RawResult, error) {
	params := url.Values{}
	params.Set("wd", query)
	if opts.Limit > 0 {
		params.Set("rn", strconv.Itoa(min(opts.Limit+5, 20)))
	}

	reqURL := e.baseURL + "?" + params.Encode()
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, apperrors.NewNetworkError("Baidu request build failed", err.Error(), map[string]string{"url": reqURL}, nil)
	}
	req.Header.Set("User-Agent", ddgUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Referer", "https://www.baidu.com/")

	body, statusCode, err := doPoliteSearchRequest(e.httpClient, req, politeRequestOptions{
		engine:       "baidu",
		timeout:      baiduTimeout,
		limiter:      e.limiter,
		blocked:      isBaiduBlocked,
		retryDelayFn: fixedRetryDelay(baiduRetryDelay),
	})
	if err != nil {
		return nil, err
	}
	if statusCode != http.StatusOK {
		return nil, apperrors.NewEngineError(
			"Baidu returned non-200 status",
			fmt.Sprintf("HTTP %d", statusCode),
			map[string]string{"url": reqURL, "status_code": strconv.Itoa(statusCode)},
			nil,
		)
	}

	bodyText := string(body)
	results, err := parseBaiduHTML(bodyText)
	if err != nil {
		return nil, err
	}
	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}
	return results, nil
}

func parseBaiduHTML(body string) ([]RawResult, error) {
	if isBaiduBlocked(http.StatusOK, body) {
		return nil, newSearchRateLimitError("baidu", baiduSearchURL, "captcha_or_blocked", http.StatusOK)
	}
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, apperrors.NewExtractError("failed to parse Baidu HTML", err.Error(), nil, nil, nil)
	}

	var results []RawResult
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && isBaiduResultContainer(n) {
			if result, ok := parseBaiduResult(n); ok {
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

func isBaiduResultContainer(n *html.Node) bool {
	if hasClass(n, "result") || hasClass(n, "c-container") {
		return true
	}
	return attrVal(n, "tpl") != "" && attrVal(n, "mu") != ""
}

func parseBaiduResult(n *html.Node) (RawResult, bool) {
	link := firstDescendant(n, func(candidate *html.Node) bool {
		return candidate.Type == html.ElementNode && candidate.Data == "a" && nearestAncestor(candidate, "h3") != nil
	})
	if link == nil {
		return RawResult{}, false
	}
	title := strings.TrimSpace(textContent(link))
	href := resolveBaiduURL(attrVal(link, "href"), attrVal(n, "mu"))
	if title == "" || href == "" {
		return RawResult{}, false
	}

	snippet := strings.TrimSpace(textContentExcluding(n, func(candidate *html.Node) bool {
		return candidate == link || nearestAncestor(candidate, "h3") != nil
	}))
	snippet = compactSnippet(snippet)

	source := ""
	if u, err := url.Parse(href); err == nil {
		source = u.Hostname()
	}
	return RawResult{Title: title, URL: href, Snippet: snippet, Source: source, Extra: map[string]string{}}, true
}

func resolveBaiduURL(href string, mu string) string {
	mu = strings.TrimSpace(html.UnescapeString(mu))
	if isUsableBaiduResultURL(mu) {
		return mu
	}
	href = strings.TrimSpace(html.UnescapeString(href))
	if isUsableBaiduResultURL(href) {
		return href
	}
	if strings.Contains(href, "/link?") {
		return href
	}
	return ""
}

func isUsableBaiduResultURL(rawURL string) bool {
	if rawURL == "" {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "www.baidu.com" || host == "baidu.com" {
		return strings.Contains(u.Path, "/link")
	}
	if strings.Contains(host, "top.baidu.com") || strings.Contains(host, "nourl") {
		return false
	}
	return true
}

func isBaiduBlocked(statusCode int, body string) bool {
	if statusCode == http.StatusTooManyRequests || statusCode == http.StatusForbidden {
		return true
	}
	lower := strings.ToLower(body)
	if len(body) < 500 && (strings.Contains(lower, "captcha") || strings.Contains(lower, "verify")) {
		return true
	}
	blockedMarkers := []string{
		"请输入验证码",
		"安全验证",
		"百度安全验证",
		"captcha",
		"verify",
		"blocked",
	}
	for _, marker := range blockedMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func compactSnippet(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 300 {
		return value[:300]
	}
	return value
}

func newSearchRateLimitError(engine string, reqURL string, reason string, statusCode int) error {
	context := map[string]string{
		"url":         reqURL,
		"engine":      engine,
		"reason":      reason,
		"status_code": strconv.Itoa(statusCode),
	}
	return &RateLimitError{
		Engine: engine,
		Reason: reason,
		Err: apperrors.NewEngineError(
			fmt.Sprintf("%s rate limited or blocked the request", engine),
			reason,
			context,
			[]string{"try again later", "use --provider auto with another configured search provider"},
		),
	}
}
