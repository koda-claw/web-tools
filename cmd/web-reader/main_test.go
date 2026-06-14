package webreader

import (
	jsonpkg "encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/koda-claw/web-tools/internal/config"
	apperrors "github.com/koda-claw/web-tools/internal/errors"
	"github.com/koda-claw/web-tools/internal/provider"
	"github.com/koda-claw/web-tools/internal/reader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadReaderRuntimeConfig_UsesEnvOverrideForTimeout(t *testing.T) {
	t.Setenv("WEB_READER_TIMEOUT", "42")

	cfg, err := loadReaderRuntimeConfig(0)
	require.NoError(t, err)

	assert.Equal(t, 42, cfg.Reader.DefaultTimeout)
}

func TestLoadReaderRuntimeConfig_FlagTimeoutOverridesEnv(t *testing.T) {
	t.Setenv("WEB_READER_TIMEOUT", "42")

	cfg, err := loadReaderRuntimeConfig(9)
	require.NoError(t, err)

	assert.Equal(t, 9, cfg.Reader.DefaultTimeout)
}

func TestValidateReaderFlags(t *testing.T) {
	tests := []struct {
		name        string
		extractMode string
		format      string
		wantErr     bool
	}{
		{"main markdown", "main", "markdown", false},
		{"full text", "full", "text", false},
		{"main html", "main", "html", false},
		{"bad extract", "raw", "markdown", true},
		{"bad format", "main", "xml", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReaderFlags(tt.extractMode, tt.format)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateReaderProviderBuiltin(t *testing.T) {
	cfg := config.DefaultConfig()

	assert.NoError(t, validateReaderProvider(cfg, "auto"))
	assert.NoError(t, validateReaderProvider(cfg, "builtin-reader"))
}

func TestValidateReaderProviderRejectsRemoteUntilAdapterEnabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Providers["bigmodel"] = config.ProviderConfig{
		Type:         "mcp",
		Capabilities: []string{"reader"},
	}

	err := validateReaderProvider(cfg, "bigmodel")

	assert.NoError(t, err)
}

func TestValidateReaderProviderRemoteMissingAuth(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Providers["bigmodel"] = config.ProviderConfig{
		Type:         "mcp",
		AuthEnv:      "ZHIPU_APIKEY",
		Capabilities: []string{"reader"},
	}

	err := validateReaderProvider(cfg, "bigmodel")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider auth is not configured")
}

func TestHandleProviderURLInputMCP(t *testing.T) {
	t.Setenv("ZHIPU_APIKEY", "test-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream;charset=UTF-8")
		var req map[string]any
		require.NoError(t, jsonpkg.NewDecoder(r.Body).Decode(&req))
		switch req["method"] {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "test-session")
			fmt.Fprintf(w, "id:1\nevent:message\ndata:{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"protocolVersion\":\"2024-11-05\",\"capabilities\":{\"tools\":{\"listChanged\":true}},\"serverInfo\":{\"name\":\"mock\",\"version\":\"0.0.1\"}}}\n\n")
		case "notifications/initialized":
			fmt.Fprintf(w, "id:1\nevent:message\ndata:{\"jsonrpc\":\"2.0\",\"result\":{}}\n\n")
		case "tools/call":
			text, _ := jsonpkg.Marshal(`{"title":"Example","url":"https://example.com","content":"Provider body","metadata":{"site":"example"}}`)
			payload, _ := jsonpkg.Marshal(map[string]any{
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
	defer server.Close()
	input, err := reader.ParseInput("https://example.com")
	require.NoError(t, err)

	result, err := handleProviderURLInput(input, config.DefaultConfig(), provider.Provider{
		ID: "bigmodel",
		Config: config.ProviderConfig{
			Type:    "mcp",
			AuthEnv: "ZHIPU_APIKEY",
			Reader:  &config.ProviderEndpointConfig{URL: server.URL, Tool: "webReader"},
		},
	}, "markdown")

	require.NoError(t, err)
	assert.Equal(t, "Example", result.Title)
	assert.Equal(t, "Provider body", result.Content)
	assert.Equal(t, "bigmodel", result.Metadata["provider"])
	assert.Equal(t, "provider:bigmodel", result.ExtractMode)
}

func TestPipelineResultRenderers(t *testing.T) {
	result := &PipelineResult{
		Source:      "https://example.com/article",
		Title:       "Example",
		Content:     "Markdown **body**",
		TextContent: "Plain body",
		HTML:        "<article><p>HTML body</p></article>",
		Format:      "markdown",
		FetchedAt:   time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC),
		WordCount:   2,
		ContentType: "article",
		ExtractMode: "readability",
		Quality:     &QualityInfo{Score: "high", WordCount: 2, MinWords: 1},
	}

	md, err := renderOutput(result, false, "markdown")
	require.NoError(t, err)
	assert.Contains(t, md, "<!-- source: https://example.com/article -->")
	assert.Contains(t, md, "<!-- quality: high -->")
	assert.Contains(t, md, "Markdown **body**")

	text, err := renderOutput(result, false, "text")
	require.NoError(t, err)
	assert.Equal(t, "Plain body", text)

	html, err := renderOutput(result, false, "html")
	require.NoError(t, err)
	assert.Equal(t, "<article><p>HTML body</p></article>", html)

	json, err := renderOutput(result, true, "html")
	require.NoError(t, err)
	assert.Contains(t, json, `"ok": true`)

	var parsed struct {
		Result struct {
			HTML    string       `json:"html"`
			Quality *QualityInfo `json:"quality"`
		} `json:"result"`
	}
	require.NoError(t, jsonpkg.Unmarshal([]byte(json), &parsed))
	assert.Equal(t, "<article><p>HTML body</p></article>", parsed.Result.HTML)
	require.NotNil(t, parsed.Result.Quality)
	assert.Equal(t, "high", parsed.Result.Quality.Score)
}

func TestPipelineResultRenderTextFallsBackToContent(t *testing.T) {
	result := &PipelineResult{Content: " Markdown body \n"}

	text, err := renderOutput(result, false, "text")
	require.NoError(t, err)

	assert.Equal(t, "Markdown body", text)
}

func TestPipelineResultRenderHTMLUnavailable(t *testing.T) {
	result := &PipelineResult{Content: "Plain body"}

	_, err := renderOutput(result, false, "html")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTML output is unavailable")
}

func TestHandleFileInputTextFormat(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/note.txt"
	content := "first line\nsecond line\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	input, err := reader.ParseInput(path)
	require.NoError(t, err)
	require.NotNil(t, input)

	result, err := handleFileInput(input, config.DefaultConfig(), "main", "text")
	require.NoError(t, err)

	output, err := renderOutput(result, false, "text")
	require.NoError(t, err)
	assert.Equal(t, strings.TrimSpace(content), output)
	assert.Equal(t, "text", result.Format)
	assert.Equal(t, "file", result.ExtractMode)
	require.NotNil(t, result.Quality)
	assert.Equal(t, "low", result.Quality.Score)
}

func TestAssessQuality(t *testing.T) {
	high := assessQuality(100, 50, "readability")
	assert.Equal(t, "high", high.Score)
	assert.False(t, high.NeedsFallback)

	low := assessQuality(10, 50, "readability")
	assert.Equal(t, "low", low.Score)
	assert.True(t, low.NeedsFallback)
	assert.Contains(t, low.Reasons[0], "below minimum")

	empty := assessQuality(0, 50, "readability")
	assert.Equal(t, "empty", empty.Score)
	assert.True(t, empty.NeedsFallback)
}

func TestIsHTTPStatusError(t *testing.T) {
	assert.True(t, isHTTPStatusError(apperrors.NewNetworkError(
		"HTTP request returned error status",
		"HTTP 403",
		map[string]string{"status_code": "403"},
		nil,
	)))
	assert.True(t, isHTTPStatusError(apperrors.NewUnreachableError(
		"page not found",
		"HTTP 404",
		map[string]string{"status_code": "404"},
		nil,
	)))
	assert.False(t, isHTTPStatusError(apperrors.NewExtractError(
		"readability failed",
		"no article",
		nil,
		nil,
		nil,
	)))
}
