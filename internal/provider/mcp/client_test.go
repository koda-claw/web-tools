package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/koda-claw/web-tools/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSSE(t *testing.T) {
	got, err := parseSSE([]byte("id:1\nevent:message\ndata:{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}\n\n"))

	require.NoError(t, err)
	assert.JSONEq(t, `{"jsonrpc":"2.0","id":1,"result":{}}`, string(got))
}

func TestParseTextJSONDoubleEncoded(t *testing.T) {
	var rows []struct {
		Title string `json:"title"`
	}

	err := parseTextJSON(`"[{\"title\":\"A\"}]"`, &rows)

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "A", rows[0].Title)
}

func TestClientSearchMapsSSEToolResult(t *testing.T) {
	server := newMockMCPServer(t, func(tool string, args map[string]any) any {
		assert.Equal(t, "web_search_prime", tool)
		assert.Equal(t, "Go readability", args["search_query"])
		text, _ := json.Marshal(`[{"title":"Result","link":"https://example.com","content":"Snippet","refer":"ref_1"}]`)
		return map[string]any{
			"content": []map[string]any{{"type": "text", "text": string(text)}},
			"isError": false,
		}
	})
	defer server.Close()

	client := NewClient(config.ProviderConfig{
		Timeout: 5,
		Search:  &config.ProviderEndpointConfig{URL: server.URL, Tool: "web_search_prime"},
	}, "test-token")

	results, err := client.Search(context.Background(), "Go readability", SearchOptions{})

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Result", results[0].Title)
	assert.Equal(t, "https://example.com", results[0].URL)
	assert.Equal(t, "Snippet", results[0].Snippet)
	assert.Equal(t, "ref_1", results[0].Source)
}

func TestClientReadMapsSSEToolResult(t *testing.T) {
	server := newMockMCPServer(t, func(tool string, args map[string]any) any {
		assert.Equal(t, "webReader", tool)
		assert.Equal(t, "https://example.com", args["url"])
		text, _ := json.Marshal(`{"title":"Example","url":"https://example.com","content":"Body","metadata":{"site":"example"}}`)
		return map[string]any{
			"content": []map[string]any{{"type": "text", "text": string(text)}},
			"isError": false,
		}
	})
	defer server.Close()

	client := NewClient(config.ProviderConfig{
		Timeout: 5,
		Reader:  &config.ProviderEndpointConfig{URL: server.URL, Tool: "webReader"},
	}, "test-token")

	result, err := client.Read(context.Background(), "https://example.com")

	require.NoError(t, err)
	assert.Equal(t, "Example", result.Title)
	assert.Equal(t, "Body", result.Content)
	assert.Equal(t, "example", result.Metadata["site"])
}

func TestClientSearchMalformedResult(t *testing.T) {
	server := newMockMCPServer(t, func(tool string, args map[string]any) any {
		return map[string]any{
			"content": []map[string]any{{"type": "text", "text": `"not-json-array"`}},
			"isError": false,
		}
	})
	defer server.Close()

	client := NewClient(config.ProviderConfig{
		Timeout: 5,
		Search:  &config.ProviderEndpointConfig{URL: server.URL, Tool: "web_search_prime"},
	}, "test-token")

	_, err := client.Search(context.Background(), "query", SearchOptions{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot map MCP search result")
}

func TestClientHTTPErrorDoesNotLeakAuthorization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer server.Close()

	client := NewClient(config.ProviderConfig{
		Timeout: 5,
		Search:  &config.ProviderEndpointConfig{URL: server.URL, Tool: "web_search_prime"},
	}, "test-token")

	_, err := client.Search(context.Background(), "query", SearchOptions{})

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "test-token")
	assert.NotContains(t, err.Error(), "Authorization")
}

func newMockMCPServer(t *testing.T, toolHandler func(tool string, args map[string]any) any) *httptest.Server {
	t.Helper()
	var sessionSeen bool
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.Header.Get("Accept"), "text/event-stream")
		var req rpcRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		w.Header().Set("Content-Type", "text/event-stream;charset=UTF-8")
		switch req.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "test-session")
			writeSSEResult(t, w, req.ID, map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]any{
					"tools": map[string]any{"listChanged": true},
				},
				"serverInfo": map[string]string{"name": "mock", "version": "0.0.1"},
			})
		case "notifications/initialized":
			require.Equal(t, "test-session", r.Header.Get("Mcp-Session-Id"))
			sessionSeen = true
			writeSSEResult(t, w, req.ID, map[string]any{})
		case "tools/call":
			require.True(t, sessionSeen)
			params := req.Params.(map[string]any)
			toolName := params["name"].(string)
			args := params["arguments"].(map[string]any)
			writeSSEResult(t, w, req.ID, toolHandler(toolName, args))
		default:
			t.Fatalf("unexpected MCP method %s", req.Method)
		}
	}))
}

func writeSSEResult(t *testing.T, w http.ResponseWriter, id int, result any) {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
	require.NoError(t, err)
	_, err = w.Write([]byte("id:1\nevent:message\ndata:" + strings.TrimSpace(string(data)) + "\n\n"))
	require.NoError(t, err)
}
