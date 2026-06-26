package search

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixtureBaiduNormal = `<!doctype html>
<html>
<body>
  <div class="result c-container" mu="https://go.dev/">
    <h3 class="t"><a href="https://www.baidu.com/link?url=abc">Go 编程语言</a></h3>
    <div class="c-abstract">Go 是一种开源编程语言。</div>
  </div>
  <div class="result c-container" mu="https://pkg.go.dev/">
    <h3 class="t"><a href="https://www.baidu.com/link?url=def">Go Packages</a></h3>
    <div class="c-abstract">查找 Go 包文档。</div>
  </div>
</body>
</html>`

const fixtureBaiduCaptcha = `<!doctype html>
<html><body><h1>百度安全验证</h1><p>请输入验证码</p></body></html>`

func TestParseBaiduHTML(t *testing.T) {
	results, err := parseBaiduHTML(fixtureBaiduNormal)
	require.NoError(t, err)
	require.Len(t, results, 2)

	assert.Equal(t, "Go 编程语言", results[0].Title)
	assert.Equal(t, "https://go.dev/", results[0].URL)
	assert.Contains(t, results[0].Snippet, "开源编程语言")
	assert.Equal(t, "go.dev", results[0].Source)
	assert.Equal(t, "Go Packages", results[1].Title)
}

func TestParseBaiduHTMLDetectsCaptcha(t *testing.T) {
	results, err := parseBaiduHTML(fixtureBaiduCaptcha)

	assert.Nil(t, results)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRateLimited))
}

func TestBaiduQueryLimitAndHeaders(t *testing.T) {
	engine := NewBaiduEngine()
	engine.baseURL = "https://www.baidu.test/s"
	engine.limiter = nil
	engine.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "人工智能 最新进展", req.URL.Query().Get("wd"))
			assert.Equal(t, "7", req.URL.Query().Get("rn"))
			assert.Contains(t, req.Header.Get("Accept-Language"), "zh-CN")
			assert.Equal(t, "https://www.baidu.com/", req.Header.Get("Referer"))
			return testResponse(http.StatusOK, fixtureBaiduNormal), nil
		}),
	}

	results, err := engine.Query("人工智能 最新进展", SearchOptions{Limit: 2, Locale: "zh-CN"})

	require.NoError(t, err)
	require.Len(t, results, 2)
}

func TestBaiduQueryDoesNotRetryCaptcha(t *testing.T) {
	t.Cleanup(func() { politeSleep = time.Sleep })
	politeSleep = func(time.Duration) {}
	attempts := 0
	engine := NewBaiduEngine()
	engine.baseURL = "https://www.baidu.test/s"
	engine.limiter = nil
	engine.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			return testResponse(http.StatusOK, fixtureBaiduCaptcha), nil
		}),
	}

	_, err := engine.Query("人工智能 最新进展", SearchOptions{Limit: 1})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRateLimited))
	assert.Equal(t, 1, attempts)
}

func TestResolveBaiduURLPrefersMU(t *testing.T) {
	got := resolveBaiduURL("https://www.baidu.com/link?url=abc", "https://example.com/article")
	assert.Equal(t, "https://example.com/article", got)
}
