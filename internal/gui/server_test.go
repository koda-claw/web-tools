package gui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/koda-claw/web-tools/internal/config"
	"github.com/koda-claw/web-tools/internal/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerBindsLocalHealthz(t *testing.T) {
	dir := isolatedHome(t)
	server := NewServer(Options{Version: "test", Host: "127.0.0.1", Port: 0, SkillDir: filepath.Join(dir, "skills"), NoOpen: true})
	require.NoError(t, server.Start())
	defer server.Shutdown(context.Background())

	assert.Contains(t, server.URL(), "127.0.0.1:")
	resp, err := http.Get(server.URL() + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestStatusDoesNotLeakSecret(t *testing.T) {
	dir := isolatedHome(t)
	t.Setenv("ZHIPU_APIKEY", "super-secret-token")
	cfg := &config.EditableConfig{}
	config.AddBigModelProvider(cfg, "ZHIPU_APIKEY", false, false)
	require.NoError(t, config.SaveEditableConfig(config.UserConfigPath(), cfg))

	body := requestJSON(t, NewServer(Options{Version: "test", SkillDir: filepath.Join(dir, "skills")}), http.MethodGet, "/api/status", nil)

	assert.Contains(t, body, `"auth_configured":true`)
	assert.NotContains(t, body, "super-secret-token")
}

func TestSearchFormIncludesExplicitBuiltinProviders(t *testing.T) {
	data, err := fs.ReadFile(embeddedAssets, "assets/index.html")
	require.NoError(t, err)
	html := string(data)

	assert.Contains(t, html, `<option value="bing">bing</option>`)
	assert.Contains(t, html, `<option value="baidu">baidu</option>`)
	assert.Contains(t, html, `<option value="sogou">sogou</option>`)
}

func TestProviderAndEnvAPIsWriteConfigWithoutLeakingValue(t *testing.T) {
	dir := isolatedHome(t)
	server := NewServer(Options{Version: "test", SkillDir: filepath.Join(dir, "skills")})

	body := requestJSON(t, server, http.MethodPost, "/api/setup/provider", map[string]any{
		"provider":           "bigmodel",
		"auth_env":           "ZHIPU_APIKEY",
		"enable_search_auto": true,
		"enable_reader_auto": true,
	})
	assert.Contains(t, body, `"provider":"bigmodel"`)

	cfgData, err := os.ReadFile(config.UserConfigPath())
	require.NoError(t, err)
	assert.Contains(t, string(cfgData), `"auth_env": "ZHIPU_APIKEY"`)
	assert.NotContains(t, string(cfgData), "fake-token")

	body = requestJSON(t, server, http.MethodPost, "/api/env", map[string]any{
		"key":   "ZHIPU_APIKEY",
		"value": "fake-token",
	})
	assert.Contains(t, body, `"key":"ZHIPU_APIKEY"`)
	assert.NotContains(t, body, "fake-token")

	info, err := os.Stat(config.EnvFilePath())
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
	values, err := config.ParseEnvFile(config.EnvFilePath())
	require.NoError(t, err)
	assert.Equal(t, "fake-token", values["ZHIPU_APIKEY"])

	rr := request(t, server, http.MethodPost, "/api/env", map[string]any{
		"key":   "ZHIPU_APIKEY",
		"value": "new-token",
	})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.NotContains(t, rr.Body.String(), "new-token")
}

func TestReaderTestAPIWithLocalServer(t *testing.T) {
	dir := isolatedHome(t)
	longContent := strings.Repeat("full-content-marker ", 320)
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Local Page</title></head><body><article><h1>Local Page</h1><p>This is enough content for a local reader smoke test with useful words.</p><p>` + longContent + `</p></article></body></html>`))
	}))
	defer page.Close()

	body := requestJSON(t, NewServer(Options{Version: "test", SkillDir: filepath.Join(dir, "skills")}), http.MethodPost, "/api/test/reader", map[string]any{
		"url":      page.URL,
		"provider": "builtin-reader",
	})

	assert.Contains(t, body, `"ok":true`)
	assert.Contains(t, body, "Local Page")
	assert.Greater(t, strings.Count(body, "full-content-marker"), 250)
	assert.NotContains(t, body, "truncated")
}

func TestReaderTestAPIBigModelRequiresAuth(t *testing.T) {
	dir := isolatedHome(t)
	cfg := &config.EditableConfig{}
	config.AddBigModelProvider(cfg, "ZHIPU_APIKEY", false, false)
	require.NoError(t, config.SaveEditableConfig(config.UserConfigPath(), cfg))
	require.NoError(t, os.Unsetenv("ZHIPU_APIKEY"))

	rr := request(t, NewServer(Options{Version: "test", SkillDir: filepath.Join(dir, "skills")}), http.MethodPost, "/api/test/reader", map[string]any{
		"url":      "https://example.com",
		"provider": "bigmodel",
	})

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "provider auth is not configured")
	assert.Contains(t, rr.Body.String(), "ZHIPU_APIKEY")
}

func TestReaderTestAPIAutoFallsBackToBigModelOnLowQualityBuiltin(t *testing.T) {
	dir := isolatedHome(t)
	t.Setenv("ZHIPU_APIKEY", "test-token")
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Sparse</title></head><body><main><p>tiny</p></main></body></html>`))
	}))
	defer page.Close()
	mcp := newGUIReaderMCPServer(t, `{"title":"Provider Article","url":"https://example.com/provider","content":"Provider body has enough words for GUI fallback","metadata":{"site":"provider"}}`)
	defer mcp.Close()

	cfg := map[string]any{
		"providers": map[string]any{
			"bigmodel": map[string]any{
				"type":         "mcp",
				"auth_env":     "ZHIPU_APIKEY",
				"capabilities": []string{"reader"},
				"reader":       map[string]any{"url": mcp.URL, "tool": "webReader"},
			},
		},
		"reader": map[string]any{
			"browser_fallback":       false,
			"min_content_length":     3,
			"default_provider":       "auto",
			"default_provider_chain": []string{"builtin-reader", "bigmodel"},
		},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	require.NoError(t, err)
	configPath := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(configPath, data, 0644))
	t.Setenv("WEB_TOOLS_CONFIG", configPath)

	body := requestJSON(t, NewServer(Options{Version: "test", SkillDir: filepath.Join(dir, "skills")}), http.MethodPost, "/api/test/reader", map[string]any{
		"url":      page.URL,
		"provider": "auto",
	})

	assert.Contains(t, body, `"provider":"bigmodel"`)
	assert.Contains(t, body, `"extract_mode":"provider:bigmodel"`)
	assert.Contains(t, body, "Provider Article")
}

func TestDiagnosticsContainsGuideAndNoSecret(t *testing.T) {
	dir := isolatedHome(t)
	t.Setenv("ZHIPU_APIKEY", "super-secret-token")

	body := requestJSON(t, NewServer(Options{Version: "test", SkillDir: filepath.Join(dir, "skills")}), http.MethodGet, "/api/diagnostics", nil)

	assert.Contains(t, body, `"repository_url"`)
	assert.Contains(t, body, `"agent_guide"`)
	assert.Contains(t, body, "web-tools upgrade")
	assert.NotContains(t, body, ".tar.gz")
	assert.NotContains(t, body, "web-tools_Darwin")
	assert.NotContains(t, body, "super-secret-token")
}

func TestMetricsAPIsReadResetAndDiagnosticsSummary(t *testing.T) {
	dir := isolatedHome(t)
	metricsFile := filepath.Join(dir, "metrics.json")
	t.Setenv("WEB_TOOLS_METRICS_FILE", metricsFile)
	store := metrics.NewStore("")
	require.NoError(t, store.Record(metrics.Event{
		At:         time.Now(),
		Command:    "web-search",
		Provider:   "duckduckgo",
		Status:     "success",
		DurationMS: 12,
	}))
	server := NewServer(Options{Version: "test", SkillDir: filepath.Join(dir, "skills")})

	body := requestJSON(t, server, http.MethodGet, "/api/metrics?range=24h&bucket=auto", nil)
	assert.Contains(t, body, `"ok":true`)
	assert.Contains(t, body, `"web-search"`)
	assert.Contains(t, body, `"search:duckduckgo"`)

	diag := requestJSON(t, server, http.MethodGet, "/api/diagnostics", nil)
	assert.Contains(t, diag, `"metrics"`)
	assert.Contains(t, diag, `"web-search"`)
	assert.NotContains(t, diag, "secret-token")

	reset := requestJSON(t, server, http.MethodPost, "/api/metrics/reset", map[string]any{})
	assert.Contains(t, reset, `"ok":true`)
	after := requestJSON(t, server, http.MethodGet, "/api/metrics", nil)
	assert.NotContains(t, after, `"web-search"`)
}

func TestMetricsAPIRejectsInvalidRange(t *testing.T) {
	dir := isolatedHome(t)
	t.Setenv("WEB_TOOLS_METRICS_FILE", filepath.Join(dir, "metrics.json"))

	rr := request(t, NewServer(Options{Version: "test", SkillDir: filepath.Join(dir, "skills")}), http.MethodGet, "/api/metrics?range=3h", nil)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "invalid metrics range")
}

func TestReaderAndSearchTestsRecordSafeMetrics(t *testing.T) {
	dir := isolatedHome(t)
	metricsFile := filepath.Join(dir, "metrics.json")
	t.Setenv("WEB_TOOLS_METRICS_FILE", metricsFile)
	var serverURL string
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"number_of_results":1,"results":[{"title":"GUI Fixture","url":%q,"content":"safe snippet","parsed_url":["http",%q,"/article"]}]}`, serverURL+"/article", strings.TrimPrefix(serverURL, "http://"))
		case "/article":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><head><title>GUI Fixture</title></head><body><article><p>` + strings.Repeat("private page content marker ", 80) + `</p></article></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer fixture.Close()
	serverURL = fixture.URL

	cfg := map[string]any{
		"reader": map[string]any{"cache_dir": filepath.Join(dir, "cache"), "browser_fallback": false, "min_content_length": 20},
		"search": map[string]any{"searxng_url": serverURL, "default_engine": "searxng"},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	require.NoError(t, err)
	configPath := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(configPath, data, 0644))
	t.Setenv("WEB_TOOLS_CONFIG", configPath)

	guiServer := NewServer(Options{Version: "test", SkillDir: filepath.Join(dir, "skills")})
	searchBody := requestJSON(t, guiServer, http.MethodPost, "/api/test/search", map[string]any{
		"query":    "private search query marker",
		"provider": "searxng",
		"limit":    1,
	})
	assert.Contains(t, searchBody, "GUI Fixture")
	readerBody := requestJSON(t, guiServer, http.MethodPost, "/api/test/reader", map[string]any{
		"url":      serverURL + "/article",
		"provider": "builtin-reader",
	})
	assert.Contains(t, readerBody, "GUI Fixture")

	metricsBody := requestJSON(t, guiServer, http.MethodGet, "/api/metrics", nil)
	assert.Contains(t, metricsBody, `"gui-test-search"`)
	assert.Contains(t, metricsBody, `"gui-test-reader"`)
	assert.NotContains(t, metricsBody, "private search query marker")
	assert.NotContains(t, metricsBody, serverURL)
	assert.NotContains(t, metricsBody, "private page content marker")
}

func isolatedHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	t.Setenv("HOME", dir)
	t.Setenv("WEB_TOOLS_CONFIG", "")
	t.Setenv("WEB_TOOLS_ENV", "")
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})
	return dir
}

func requestJSON(t *testing.T, server *Server, method string, path string, payload any) string {
	t.Helper()
	rr := request(t, server, method, path, payload)
	require.True(t, rr.Code >= 200 && rr.Code < 300, rr.Body.String())
	return compactJSON(t, rr.Body.Bytes())
}

func request(t *testing.T, server *Server, method string, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	if payload != nil {
		require.NoError(t, json.NewEncoder(&body).Encode(payload))
	}
	req := httptest.NewRequest(method, path, &body)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	server.routes().ServeHTTP(rr, req)
	return rr
}

func compactJSON(t *testing.T, data []byte) string {
	t.Helper()
	var v any
	require.NoError(t, json.Unmarshal(data, &v))
	out, err := json.Marshal(v)
	require.NoError(t, err)
	return strings.TrimSpace(string(out))
}

func newGUIReaderMCPServer(t *testing.T, readerPayload string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream;charset=UTF-8")
		var req map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		switch req["method"] {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "test-session")
			fmt.Fprintf(w, "id:1\nevent:message\ndata:{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"protocolVersion\":\"2024-11-05\",\"capabilities\":{\"tools\":{\"listChanged\":true}},\"serverInfo\":{\"name\":\"mock\",\"version\":\"0.0.1\"}}}\n\n")
		case "notifications/initialized":
			fmt.Fprintf(w, "id:1\nevent:message\ndata:{\"jsonrpc\":\"2.0\",\"result\":{}}\n\n")
		case "tools/call":
			text, _ := json.Marshal(readerPayload)
			payload, _ := json.Marshal(map[string]any{
				"jsonrpc": "2.0",
				"id":      3,
				"result": map[string]any{
					"content": []map[string]any{{"type": "text", "text": string(text)}},
					"isError": false,
				},
			})
			fmt.Fprintf(w, "id:1\nevent:message\ndata:%s\n\n", payload)
		default:
			t.Fatalf("unexpected method %v", req["method"])
		}
	}))
}

func TestServerShutdownWithoutStart(t *testing.T) {
	server := NewServer(Options{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, server.Shutdown(ctx))
}
