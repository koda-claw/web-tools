package search

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	apperrors "github.com/koda-claw/web-tools/internal/errors"
	"golang.org/x/net/html"
)

const (
	ddgLiteURL = "https://lite.duckduckgo.com/lite"
	ddgTimeout = 10 * time.Second
	// Mimic a real browser to avoid empty results / captcha.
	ddgUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
)

// DuckDuckGoEngine queries DuckDuckGo Lite via HTML scraping. Zero dependencies.
type DuckDuckGoEngine struct {
	httpClient *http.Client
}

// NewDuckDuckGoEngine creates a new DuckDuckGoEngine.
func NewDuckDuckGoEngine() *DuckDuckGoEngine {
	return &DuckDuckGoEngine{
		httpClient: &http.Client{Timeout: ddgTimeout},
	}
}

// Name returns the engine identifier.
func (e *DuckDuckGoEngine) Name() string { return "duckduckgo" }

// HealthCheck always succeeds — DDG Lite is a public endpoint.
func (e *DuckDuckGoEngine) HealthCheck() error { return nil }

// Query sends a search to DDG Lite and parses the HTML result table.
// Category and TimeRange are not supported by DDG Lite and are silently ignored.
func (e *DuckDuckGoEngine) Query(query string, opts SearchOptions) ([]RawResult, error) {
	params := url.Values{}
	params.Set("q", query)
	// Map locale to DDG kl param (e.g. "zh-CN" → "zh-cn", "en-US" → "en-us").
	if opts.Locale != "" && opts.Locale != "auto" {
		params.Set("kl", strings.ToLower(opts.Locale))
	} else {
		params.Set("kl", "wt-wt") // worldwide
	}

	reqURL := ddgLiteURL + "?" + params.Encode()
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, apperrors.NewNetworkError(
			"DDG request build failed",
			err.Error(),
			map[string]string{"url": reqURL},
			nil,
		)
	}
	req.Header.Set("User-Agent", ddgUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, apperrors.NewNetworkError(
			"DuckDuckGo Lite request failed",
			err.Error(),
			map[string]string{"url": reqURL, "timeout": ddgTimeout.String()},
			[]string{"check network connectivity"},
		)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, apperrors.NewNetworkError(
			"failed to read DuckDuckGo response",
			err.Error(),
			map[string]string{"url": reqURL},
			nil,
		)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, apperrors.NewEngineError(
			"DuckDuckGo returned non-200 status",
			fmt.Sprintf("HTTP %d", resp.StatusCode),
			map[string]string{"url": reqURL, "status_code": fmt.Sprintf("%d", resp.StatusCode)},
			nil,
		)
	}

	results, err := parseDDGLiteHTML(string(body))
	if err != nil {
		return nil, err
	}

	// Apply limit
	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}

	return results, nil
}

// parseDDGLiteHTML extracts search results from DuckDuckGo Lite's HTML table.
//
// DDG Lite returns a <table> where each result spans three consecutive <tr> rows:
//  1. <tr> containing <td class="result-link"> with an <a href="URL">Title</a>
//  2. <tr> containing <td class="result-snippet"> with snippet text
//  3. <tr> containing <td class="result-url"> with displayed URL
//
// The parser collects all result-link anchors and result-snippet cells,
// then zips them into RawResult values.
func parseDDGLiteHTML(body string) ([]RawResult, error) {
	if isCaptchaPage(body) {
		return nil, apperrors.NewEngineError(
			"DuckDuckGo returned a captcha page",
			"captcha or anti-bot challenge detected",
			nil,
			[]string{"try again later", "use --engine searxng for higher throughput"},
		)
	}

	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil, apperrors.NewExtractError(
			"failed to parse DuckDuckGo HTML",
			err.Error(),
			nil, nil, nil,
		)
	}

	type linkEntry struct {
		title string
		href  string
	}

	var links []linkEntry
	var snippets []string

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "a":
				if hasClass(n, "result-link") {
					href := attrVal(n, "href")
					title := strings.TrimSpace(textContent(n))
					if href != "" && title != "" {
						links = append(links, linkEntry{title: title, href: href})
					}
				}
			case "td":
				if hasClass(n, "result-snippet") {
					snippet := strings.TrimSpace(textContent(n))
					if snippet != "" {
						snippets = append(snippets, snippet)
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	// Zip links and snippets — DDG Lite always produces them in matched order.
	count := len(links)
	if len(snippets) < count {
		count = len(snippets)
	}

	results := make([]RawResult, 0, count)
	for i := 0; i < count; i++ {
		// Resolve DDG redirect: //duckduckgo.com/l/?uddg=<real_url>&rut=<hash>
		realURL := resolveDDGRedirect(links[i].href)

		source := ""
		if u, err := url.Parse(realURL); err == nil {
			source = u.Hostname()
		}
		results = append(results, RawResult{
			Title:   links[i].title,
			URL:     realURL,
			Snippet: snippets[i],
			Source:  source,
			Extra:   map[string]string{},
		})
	}

	return results, nil
}

// resolveDDGRedirect extracts the real URL from a DDG Lite redirect link.
//
// DDG Lite wraps all result URLs as redirects:
//
//	//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com&rut=<hash>
//
// This function parses the uddg parameter and returns the decoded real URL.
// If the href is not a DDG redirect (e.g. already a direct URL), it is returned as-is.
func resolveDDGRedirect(href string) string {
	// Quick check: DDG redirects contain "uddg="
	if !strings.Contains(href, "uddg=") {
		return href
	}

	// Ensure the href has a scheme for url.Parse to work correctly.
	parsed := href
	if strings.HasPrefix(parsed, "//") {
		parsed = "https:" + parsed
	}

	u, err := url.Parse(parsed)
	if err != nil {
		return href
	}

	realURL := u.Query().Get("uddg")
	if realURL == "" {
		return href
	}

	// The uddg value is URL-encoded; decode it.
	decoded, err := url.QueryUnescape(realURL)
	if err != nil {
		return realURL
	}

	return decoded
}

// isCaptchaPage returns true if the HTML looks like a DDG anti-bot challenge.
func isCaptchaPage(body string) bool {
	lower := strings.ToLower(body)
	return strings.Contains(lower, "captcha") ||
		strings.Contains(lower, "unusual traffic") ||
		strings.Contains(lower, "please enable javascript")
}

// hasClass reports whether a node has the given CSS class.
func hasClass(n *html.Node, class string) bool {
	for _, a := range n.Attr {
		if a.Key == "class" {
			for _, c := range strings.Fields(a.Val) {
				if c == class {
					return true
				}
			}
		}
	}
	return false
}

// attrVal returns the value of a named attribute, or "".
func attrVal(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// textContent returns the concatenated text content of a node and its children.
func textContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(textContent(c))
	}
	return sb.String()
}
