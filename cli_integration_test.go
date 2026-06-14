package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIIntegrationSearchThenRead(t *testing.T) {
	bin := buildCLITestBinary(t)

	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			assert.Equal(t, "json", r.URL.Query().Get("format"))
			assert.Equal(t, "agent research fixture", r.URL.Query().Get("q"))
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{
				"number_of_results": 1,
				"results": [
					{
						"title": "Fixture Research Article",
						"url": %q,
						"content": "A local article about agent research workflow.",
						"engines": ["fixture"],
						"parsed_url": ["http", %q, "/article"]
					}
				]
			}`, serverURL+"/article", strings.TrimPrefix(serverURL, "http://"))
		case "/article":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `<!doctype html>
<html>
<head>
  <title>Fixture Research Article</title>
  <meta name="description" content="A local integration fixture">
</head>
<body>
  <article>
    <h1>Fixture Research Article</h1>
    <p>Agent research starts with explicit search, selected sources, and structured reading.</p>
    <p>The reader returns quality metadata so the caller can decide whether browser fallback is required.</p>
    <p>This fixture has enough useful words to be treated as a high quality extraction result.</p>
  </article>
</body>
</html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	serverURL = server.URL

	configPath := writeCLITestConfig(t, serverURL, map[string]any{
		"default_engine": "searxng",
		"default_limit":  5,
	})

	searchStdout, searchStderr := runCLI(t, bin, []string{
		"web-search", "agent research fixture", "--json",
	}, map[string]string{"WEB_TOOLS_CONFIG": configPath})
	assert.Empty(t, searchStderr)

	var searchEnvelope struct {
		OK     bool `json:"ok"`
		Result struct {
			Engine  string `json:"engine"`
			Total   int    `json:"total"`
			Results []struct {
				Title  string   `json:"title"`
				URL    string   `json:"url"`
				Source string   `json:"source"`
				Engine []string `json:"engines"`
			} `json:"results"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal([]byte(searchStdout), &searchEnvelope))
	require.True(t, searchEnvelope.OK)
	assert.Equal(t, "searxng", searchEnvelope.Result.Engine)
	require.Len(t, searchEnvelope.Result.Results, 1)
	assert.Equal(t, serverURL+"/article", searchEnvelope.Result.Results[0].URL)

	readerStdout, readerStderr := runCLI(t, bin, []string{
		"web-reader", searchEnvelope.Result.Results[0].URL, "--json", "--no-cache",
	}, map[string]string{"WEB_TOOLS_CONFIG": configPath})
	assert.Empty(t, readerStderr)

	var readerEnvelope struct {
		OK     bool `json:"ok"`
		Result struct {
			Source    string `json:"source"`
			URL       string `json:"url"`
			Title     string `json:"title"`
			Content   string `json:"content"`
			WordCount int    `json:"word_count"`
			Quality   struct {
				Score         string `json:"score"`
				NeedsFallback bool   `json:"needs_fallback"`
			} `json:"quality"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal([]byte(readerStdout), &readerEnvelope))
	require.True(t, readerEnvelope.OK)
	assert.Equal(t, serverURL+"/article", readerEnvelope.Result.Source)
	assert.Equal(t, serverURL+"/article", readerEnvelope.Result.URL)
	assert.Equal(t, "Fixture Research Article", readerEnvelope.Result.Title)
	assert.Contains(t, readerEnvelope.Result.Content, "Agent research starts with explicit search")
	assert.Greater(t, readerEnvelope.Result.WordCount, 20)
	assert.Equal(t, "high", readerEnvelope.Result.Quality.Score)
	assert.False(t, readerEnvelope.Result.Quality.NeedsFallback)
}

func TestCLIIntegrationSearchConfigAndDomainFilters(t *testing.T) {
	bin := buildCLITestBinary(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		require.Equal(t, "/search", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"number_of_results": 3,
			"results": [
				{"title": "Keep One", "url": "https://docs.example.com/a#intro", "content": "first", "parsed_url": ["https", "docs.example.com", "/a"]},
				{"title": "Keep Duplicate", "url": "https://docs.example.com/a", "content": "duplicate", "parsed_url": ["https", "docs.example.com", "/a"]},
				{"title": "Drop", "url": "https://noise.example.net/b", "content": "drop", "parsed_url": ["https", "noise.example.net", "/b"]}
			]
		}`)
	}))
	defer server.Close()

	configPath := writeCLITestConfig(t, server.URL, map[string]any{
		"default_engine": "searxng",
		"default_limit":  10,
		"default_locale": "en-US",
	})

	stdout, stderr := runCLI(t, bin, []string{
		"web-search", "domain filter fixture",
		"--include-domain", "example.com",
		"--exclude-domain", "noise.example.net",
		"--json",
	}, map[string]string{"WEB_TOOLS_CONFIG": configPath})
	assert.Empty(t, stderr)

	var envelope struct {
		OK     bool `json:"ok"`
		Result struct {
			Engine  string `json:"engine"`
			Locale  string `json:"locale"`
			Total   int    `json:"total"`
			Results []struct {
				Rank   int    `json:"rank"`
				Title  string `json:"title"`
				URL    string `json:"url"`
				Source string `json:"source"`
			} `json:"results"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &envelope))
	require.True(t, envelope.OK)
	assert.Equal(t, "searxng", envelope.Result.Engine)
	assert.Equal(t, "en-US", envelope.Result.Locale)
	require.Len(t, envelope.Result.Results, 1)
	assert.Equal(t, 1, envelope.Result.Results[0].Rank)
	assert.Equal(t, "Keep One", envelope.Result.Results[0].Title)
	assert.Equal(t, "https://docs.example.com/a", envelope.Result.Results[0].URL)
	assert.Equal(t, "docs.example.com", envelope.Result.Results[0].Source)
}

func TestCLIIntegrationReaderSparseQualityWarning(t *testing.T) {
	bin := buildCLITestBinary(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><head><title>Sparse</title></head><body><article><p>short body only</p></article></body></html>`)
	}))
	defer server.Close()

	configPath := writeCLITestConfig(t, server.URL, map[string]any{
		"default_engine": "searxng",
	})

	stdout, stderr := runCLI(t, bin, []string{
		"web-reader", server.URL, "--json", "--no-cache",
	}, map[string]string{"WEB_TOOLS_CONFIG": configPath})
	assert.Contains(t, stderr, "extracted content quality")

	var envelope struct {
		OK     bool `json:"ok"`
		Result struct {
			Quality struct {
				Score         string `json:"score"`
				NeedsFallback bool   `json:"needs_fallback"`
			} `json:"quality"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &envelope))
	require.True(t, envelope.OK)
	assert.Equal(t, "low", envelope.Result.Quality.Score)
	assert.True(t, envelope.Result.Quality.NeedsFallback)
}

func buildCLITestBinary(t *testing.T) string {
	t.Helper()

	bin := filepath.Join(t.TempDir(), "web-tools")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", bin, ".")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return bin
}

func writeCLITestConfig(t *testing.T, searxngURL string, searchOverrides map[string]any) string {
	t.Helper()

	cacheDir := filepath.Join(t.TempDir(), "cache")
	searchCfg := map[string]any{
		"searxng_url": searxngURL,
	}
	for k, v := range searchOverrides {
		searchCfg[k] = v
	}

	cfg := map[string]any{
		"reader": map[string]any{
			"cache_dir":          cacheDir,
			"browser_fallback":   false,
			"min_content_length": 20,
		},
		"search": searchCfg,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "web-tools.json")
	require.NoError(t, os.WriteFile(path, data, 0644))
	return path
}

func runCLI(t *testing.T, bin string, args []string, env map[string]string) (string, string) {
	t.Helper()

	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "WEB_READER_NO_BROWSER=1")
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	require.NoError(t, err, "stderr:\n%s\nstdout:\n%s", stderr.String(), stdout.String())
	return stdout.String(), stderr.String()
}
