package search

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixtureBingNormal = `<!doctype html>
<html>
<body>
<ol id="b_results">
  <li class="b_algo">
    <h2><a href="https://go.dev/">The Go Programming Language</a></h2>
    <div class="b_caption"><p>Build simple, secure, scalable systems with Go.</p></div>
  </li>
  <li class="b_algo">
    <h2><a href="https://pkg.go.dev/">Go Packages</a></h2>
    <p>Discover Go packages and documentation.</p>
  </li>
</ol>
</body>
</html>`

const fixtureBingCaptcha = `<!doctype html>
<html><body><h1>Verify you are a human</h1><p>captcha required</p></body></html>`

func TestParseBingHTML(t *testing.T) {
	results, err := parseBingHTML(fixtureBingNormal)
	require.NoError(t, err)
	require.Len(t, results, 2)

	assert.Equal(t, "The Go Programming Language", results[0].Title)
	assert.Equal(t, "https://go.dev/", results[0].URL)
	assert.Equal(t, "Build simple, secure, scalable systems with Go.", results[0].Snippet)
	assert.Equal(t, "go.dev", results[0].Source)
	assert.Equal(t, "Go Packages", results[1].Title)
}

func TestParseBingHTMLDetectsCaptcha(t *testing.T) {
	results, err := parseBingHTML(fixtureBingCaptcha)

	assert.Nil(t, results)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRateLimited))
}

func TestBingQueryLimitAndHeaders(t *testing.T) {
	engine := NewBingEngine()
	engine.baseURL = "https://www.bing.test/search"
	engine.limiter = nil
	engine.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "golang readability", req.URL.Query().Get("q"))
			assert.Equal(t, "7", req.URL.Query().Get("count"))
			assert.Contains(t, req.Header.Get("Accept-Language"), "en-US")
			return testResponse(http.StatusOK, fixtureBingNormal), nil
		}),
	}

	results, err := engine.Query("golang readability", SearchOptions{Limit: 2, Locale: "en-US"})

	require.NoError(t, err)
	require.Len(t, results, 2)
}

func TestBingQueryRetriesTemporaryServerError(t *testing.T) {
	t.Cleanup(func() { politeSleep = time.Sleep })
	politeSleep = func(time.Duration) {}
	attempts := 0
	engine := NewBingEngine()
	engine.baseURL = "https://www.bing.test/search"
	engine.limiter = nil
	engine.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			if attempts == 1 {
				return testResponse(http.StatusServiceUnavailable, "temporary"), nil
			}
			return testResponse(http.StatusOK, fixtureBingNormal), nil
		}),
	}

	results, err := engine.Query("golang readability", SearchOptions{Limit: 1})

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, 2, attempts)
}

func TestResolveBingURL(t *testing.T) {
	got := resolveBingURL("https://www.bing.com/ck/a?u=a1aHR0cHM6Ly9leGFtcGxlLmNvbS9wYXRo")
	assert.Equal(t, "https://example.com/path", got)
}
