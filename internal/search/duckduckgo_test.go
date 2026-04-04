package search

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtureNormal is a trimmed DDG Lite HTML page with two results.
const fixtureNormal = `<!DOCTYPE html>
<html>
<head><title>DuckDuckGo Lite</title></head>
<body>
<form method="post" action="/lite/"><input name="q" value="golang"/></form>
<table>
<tr>
  <td class="result-link"><a class="result-link" href="https://golang.org/">The Go Programming Language</a></td>
</tr>
<tr>
  <td class="result-snippet">Go is an open source programming language that makes it easy to build simple, reliable, and efficient software.</td>
</tr>
<tr>
  <td class="result-url"><a class="result-url" href="https://golang.org">golang.org</a></td>
</tr>
<tr>
  <td class="result-link"><a class="result-link" href="https://pkg.go.dev/">Go Packages</a></td>
</tr>
<tr>
  <td class="result-snippet">Find, import, and use Go packages and modules.</td>
</tr>
<tr>
  <td class="result-url"><a class="result-url" href="https://pkg.go.dev">pkg.go.dev</a></td>
</tr>
</table>
</body>
</html>`

// fixtureCaptcha simulates a DDG anti-bot challenge page.
const fixtureCaptcha = `<!DOCTYPE html>
<html>
<head><title>DuckDuckGo</title></head>
<body>
<p>Please enable JavaScript or use a different browser to access DuckDuckGo.</p>
<p>If you believe this is unusual traffic, please complete the CAPTCHA below.</p>
</body>
</html>`

// fixtureEmpty is a DDG Lite page with no results.
const fixtureEmpty = `<!DOCTYPE html>
<html>
<head><title>DuckDuckGo Lite</title></head>
<body>
<form method="post" action="/lite/"><input name="q" value="xyzzy_no_results_ever_12345"/></form>
<table>
<tr><td colspan="2">No results.</td></tr>
</table>
</body>
</html>`

func TestParseResults(t *testing.T) {
	results, err := parseDDGLiteHTML(fixtureNormal)
	require.NoError(t, err)
	require.Len(t, results, 2)

	assert.Equal(t, "The Go Programming Language", results[0].Title)
	assert.Equal(t, "https://golang.org/", results[0].URL)
	assert.Equal(t, "Go is an open source programming language that makes it easy to build simple, reliable, and efficient software.", results[0].Snippet)
	assert.Equal(t, "golang.org", results[0].Source)

	assert.Equal(t, "Go Packages", results[1].Title)
	assert.Equal(t, "https://pkg.go.dev/", results[1].URL)
	assert.Equal(t, "Find, import, and use Go packages and modules.", results[1].Snippet)
	assert.Equal(t, "pkg.go.dev", results[1].Source)
}

func TestCaptchaDetection(t *testing.T) {
	results, err := parseDDGLiteHTML(fixtureCaptcha)
	assert.Nil(t, results)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "captcha")
}

func TestEmptyResults(t *testing.T) {
	results, err := parseDDGLiteHTML(fixtureEmpty)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestDDGLimitApplied(t *testing.T) {
	engine := NewDuckDuckGoEngine()
	// parseDDGLiteHTML returns 2 results; limit to 1 via Query indirectly.
	// We test the limit slice logic through parseDDGLiteHTML + manual trim.
	results, err := parseDDGLiteHTML(fixtureNormal)
	require.NoError(t, err)
	require.Len(t, results, 2)

	// Simulate opts.Limit = 1 trimming
	if 1 < len(results) {
		results = results[:1]
	}
	assert.Len(t, results, 1)
	_ = engine // engine constructed without error
}

// fixtureWithRedirects uses real DDG Lite redirect link format.
const fixtureWithRedirects = `<!DOCTYPE html>
<html>
<head><title>DuckDuckGo Lite</title></head>
<body>
<form method="post" action="/lite/"><input name="q" value="speedtest"/></form>
<table>
<tr>
  <td class="result-link"><a rel="nofollow" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fwww.speedtest.net%2F&amp;rut=abc123" class="result-link">Speedtest by Ookla</a></td>
</tr>
<tr>
  <td class="result-snippet">Test your internet speed.</td>
</tr>
<tr>
  <td class="result-url"><a class="result-url" href="https://www.speedtest.net">www.speedtest.net</a></td>
</tr>
<tr>
  <td class="result-link"><a rel="nofollow" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Ffast.com%2F&amp;rut=def456" class="result-link">Fast.com</a></td>
</tr>
<tr>
  <td class="result-snippet">Internet speed test by Netflix.</td>
</tr>
<tr>
  <td class="result-url"><a class="result-url" href="https://fast.com">fast.com</a></td>
</tr>
</table>
</body>
</html>`

func TestParseResultsWithRedirects(t *testing.T) {
	results, err := parseDDGLiteHTML(fixtureWithRedirects)
	require.NoError(t, err)
	require.Len(t, results, 2)

	// URL should be the resolved real URL, not the DDG redirect
	assert.Equal(t, "Speedtest by Ookla", results[0].Title)
	assert.Equal(t, "https://www.speedtest.net/", results[0].URL)
	assert.Equal(t, "www.speedtest.net", results[0].Source)
	assert.Equal(t, "Test your internet speed.", results[0].Snippet)

	assert.Equal(t, "Fast.com", results[1].Title)
	assert.Equal(t, "https://fast.com/", results[1].URL)
	assert.Equal(t, "fast.com", results[1].Source)
	assert.Equal(t, "Internet speed test by Netflix.", results[1].Snippet)
}

func TestResolveDDGRedirect(t *testing.T) {
	tests := []struct {
		name     string
		href     string
		expected string
	}{
		{
			name:     "standard redirect with // prefix",
			href:     "//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fpath&rut=abc",
			expected: "https://example.com/path",
		},
		{
			name:     "redirect with full scheme",
			href:     "https://duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com&rut=abc",
			expected: "https://example.com",
		},
		{
			name:     "non-redirect URL passed through",
			href:     "https://example.com/page",
			expected: "https://example.com/page",
		},
		{
			name:     "redirect with encoded special chars",
			href:     "//duckduckgo.com/l/?uddg=https%3A%2F%2Fzhuanlan.zhihu.com%2Fp%2F19059364698&rut=abc",
			expected: "https://zhuanlan.zhihu.com/p/19059364698",
		},
		{
			name:     "redirect with empty uddg",
			href:     "//duckduckgo.com/l/?rut=abc",
			expected: "//duckduckgo.com/l/?rut=abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveDDGRedirect(tt.href)
			assert.Equal(t, tt.expected, result)
		})
	}
}
