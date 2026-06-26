package search

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixtureSogouNormal = `<!doctype html>
<html>
<body>
  <div class="vrwrap" id="sogou_vr_1">
    <h3><a href="https://example.cn/article">人工智能发展报告</a></h3>
    <p class="str_info">人工智能产业和技术趋势摘要。</p>
  </div>
  <div class="rb" id="sogou_vr_2">
    <h3><a href="/link?url=abc">Go 搜索结果</a></h3>
    <p>Go 语言相关内容。</p>
  </div>
</body>
</html>`

const fixtureSogouCaptcha = `<!doctype html>
<html><head><link rel="stylesheet" href="static/css/anti.min.css"></head><body><p>此验证码用于确认这些请求是您的正常行为而不是自动程序发出的</p><input id="seccodeInput"></body></html>`

func TestParseSogouHTML(t *testing.T) {
	results, err := parseSogouHTML(fixtureSogouNormal)
	require.NoError(t, err)
	require.Len(t, results, 2)

	assert.Equal(t, "人工智能发展报告", results[0].Title)
	assert.Equal(t, "https://example.cn/article", results[0].URL)
	assert.Contains(t, results[0].Snippet, "人工智能产业")
	assert.Equal(t, "example.cn", results[0].Source)
	assert.Equal(t, "https://www.sogou.com/link?url=abc", results[1].URL)
}

func TestParseSogouHTMLDetectsCaptcha(t *testing.T) {
	results, err := parseSogouHTML(fixtureSogouCaptcha)

	assert.Nil(t, results)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRateLimited))
}

func TestSogouQueryLimitAndHeaders(t *testing.T) {
	engine := NewSogouEngine()
	engine.baseURL = "https://www.sogou.test/web"
	engine.limiter = nil
	engine.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "人工智能", req.URL.Query().Get("query"))
			assert.Equal(t, "7", req.URL.Query().Get("num"))
			assert.Contains(t, req.Header.Get("Accept-Language"), "zh-CN")
			assert.Equal(t, "https://www.sogou.com/", req.Header.Get("Referer"))
			return testResponse(http.StatusOK, fixtureSogouNormal), nil
		}),
	}

	results, err := engine.Query("人工智能", SearchOptions{Limit: 2, Locale: "zh-CN"})

	require.NoError(t, err)
	require.Len(t, results, 2)
}

func TestSogouQueryDoesNotRetryCaptcha(t *testing.T) {
	t.Cleanup(func() { politeSleep = time.Sleep })
	politeSleep = func(time.Duration) {}
	attempts := 0
	engine := NewSogouEngine()
	engine.baseURL = "https://www.sogou.test/web"
	engine.limiter = nil
	engine.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			return testResponse(http.StatusOK, fixtureSogouCaptcha), nil
		}),
	}

	_, err := engine.Query("人工智能", SearchOptions{Limit: 1})

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrRateLimited))
	assert.Equal(t, 1, attempts)
}
