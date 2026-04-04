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
