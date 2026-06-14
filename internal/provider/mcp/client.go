package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/koda-claw/web-tools/internal/config"
	apperrors "github.com/koda-claw/web-tools/internal/errors"
)

const defaultTimeout = 30 * time.Second

// Client calls a Streamable HTTP MCP endpoint.
type Client struct {
	httpClient *http.Client
	authToken  string
	cfg        config.ProviderConfig
}

// NewClient creates an MCP client for a provider config.
func NewClient(cfg config.ProviderConfig, authToken string) *Client {
	timeout := defaultTimeout
	if cfg.Timeout > 0 {
		timeout = time.Duration(cfg.Timeout) * time.Second
	}
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		authToken:  authToken,
		cfg:        cfg,
	}
}

// SearchOptions holds MCP search adapter options.
type SearchOptions struct {
	Locale         string
	TimeRange      string
	IncludeDomains []string
}

// SearchResult is a normalized MCP search row.
type SearchResult struct {
	Title   string
	URL     string
	Snippet string
	Source  string
}

// Search calls the configured search tool and maps results to raw search rows.
func (c *Client) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	if c.cfg.Search == nil {
		return nil, apperrors.NewInputError("missing MCP search endpoint", "provider.search is not configured", nil)
	}
	args := map[string]any{
		"search_query": query,
	}
	if opts.TimeRange != "" && opts.TimeRange != "any" {
		args["search_recency_filter"] = mapTimeRange(opts.TimeRange)
	}
	if len(opts.IncludeDomains) > 0 {
		args["search_domain_filter"] = strings.Join(opts.IncludeDomains, ",")
	}
	if opts.Locale == "en-US" {
		args["location"] = "us"
	} else if opts.Locale == "zh-CN" {
		args["location"] = "cn"
	}

	payload, err := c.callTool(ctx, c.cfg.Search.URL, c.cfg.Search.Tool, args)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Title   string `json:"title"`
		Link    string `json:"link"`
		Content string `json:"content"`
		Refer   string `json:"refer"`
	}
	if err := parseTextJSON(payload, &rows); err != nil {
		return nil, apperrors.NewEngineError(
			"cannot map MCP search result",
			err.Error(),
			map[string]string{"tool": c.cfg.Search.Tool},
			[]string{"check MCP search response mapping"},
		)
	}
	results := make([]SearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, SearchResult{
			Title:   row.Title,
			URL:     row.Link,
			Snippet: row.Content,
			Source:  row.Refer,
		})
	}
	return results, nil
}

// ReaderResult is a normalized MCP reader payload.
type ReaderResult struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	URL         string            `json:"url"`
	Content     string            `json:"content"`
	Metadata    map[string]string `json:"metadata"`
}

// Read calls the configured reader tool.
func (c *Client) Read(ctx context.Context, rawURL string) (*ReaderResult, error) {
	if c.cfg.Reader == nil {
		return nil, apperrors.NewInputError("missing MCP reader endpoint", "provider.reader is not configured", nil)
	}
	args := map[string]any{
		"url":           rawURL,
		"return_format": "markdown",
		"no_cache":      true,
	}
	payload, err := c.callTool(ctx, c.cfg.Reader.URL, c.cfg.Reader.Tool, args)
	if err != nil {
		return nil, err
	}
	var result ReaderResult
	if err := parseTextJSON(payload, &result); err != nil {
		if strings.TrimSpace(payload) != "" {
			result = ReaderResult{URL: rawURL, Content: payload}
			return &result, nil
		}
		return nil, apperrors.NewExtractError(
			"cannot map MCP reader result",
			err.Error(),
			map[string]string{"tool": c.cfg.Reader.Tool},
			nil,
			[]string{"check MCP reader response mapping"},
		)
	}
	return &result, nil
}

func (c *Client) callTool(ctx context.Context, endpoint string, tool string, args map[string]any) (string, error) {
	if endpoint == "" || tool == "" {
		return "", apperrors.NewInputError("invalid MCP endpoint", "endpoint URL and tool name are required", nil)
	}
	sessionID, err := c.initialize(ctx, endpoint)
	if err != nil {
		return "", err
	}
	if err := c.notifyInitialized(ctx, endpoint, sessionID); err != nil {
		return "", err
	}
	resp, err := c.rpc(ctx, endpoint, sessionID, rpcRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params: map[string]any{
			"name":      tool,
			"arguments": args,
		},
	})
	if err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", mcpError(resp.Error)
	}
	var result toolCallResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", apperrors.NewEngineError("cannot decode MCP tool result", err.Error(), nil, nil)
	}
	if result.IsError {
		return "", apperrors.NewEngineError("MCP tool returned an error", firstText(result.Content), nil, nil)
	}
	return firstText(result.Content), nil
}

func (c *Client) initialize(ctx context.Context, endpoint string) (string, error) {
	body, sessionID, err := c.rpcRaw(ctx, endpoint, "", rpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]string{
				"name":    "web-tools",
				"version": "0.0.0",
			},
		},
	})
	if err != nil {
		return "", err
	}
	var resp rpcResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", apperrors.NewEngineError("cannot decode MCP initialize response", err.Error(), nil, nil)
	}
	if resp.Error != nil {
		return "", mcpError(resp.Error)
	}
	return sessionID, nil
}

func (c *Client) notifyInitialized(ctx context.Context, endpoint string, sessionID string) error {
	_, _, err := c.rpcRaw(ctx, endpoint, sessionID, rpcRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
		Params:  map[string]any{},
	})
	return err
}

func (c *Client) rpc(ctx context.Context, endpoint string, sessionID string, req rpcRequest) (*rpcResponse, error) {
	body, _, err := c.rpcRaw(ctx, endpoint, sessionID, req)
	if err != nil {
		return nil, err
	}
	var resp rpcResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, apperrors.NewEngineError("cannot decode MCP response", err.Error(), nil, nil)
	}
	return &resp, nil
}

func (c *Client) rpcRaw(ctx context.Context, endpoint string, sessionID string, payload rpcRequest) ([]byte, string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", apperrors.NewNetworkError("MCP request failed", err.Error(), map[string]string{"url": endpoint}, nil)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", apperrors.NewNetworkError("cannot read MCP response", err.Error(), nil, nil)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", apperrors.NewNetworkError(
			"MCP endpoint returned error status",
			fmt.Sprintf("HTTP %d", resp.StatusCode),
			map[string]string{"status_code": fmt.Sprintf("%d", resp.StatusCode)},
			nil,
		)
	}
	parsed, err := parseResponseBody(resp.Header.Get("Content-Type"), body)
	if err != nil {
		return nil, "", err
	}
	return parsed, resp.Header.Get("Mcp-Session-Id"), nil
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolCallResult struct {
	Content []toolContent `json:"content"`
	IsError bool          `json:"isError"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func parseResponseBody(contentType string, body []byte) ([]byte, error) {
	if strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		return parseSSE(body)
	}
	return body, nil
}

func parseSSE(body []byte) ([]byte, error) {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	var dataLines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, apperrors.NewEngineError("cannot parse MCP SSE response", err.Error(), nil, nil)
	}
	if len(dataLines) == 0 {
		return nil, apperrors.NewEngineError("cannot parse MCP SSE response", "missing data event", nil, nil)
	}
	return []byte(strings.Join(dataLines, "\n")), nil
}

func parseTextJSON(text string, target any) error {
	text = strings.TrimSpace(text)
	var decoded string
	if err := json.Unmarshal([]byte(text), &decoded); err == nil {
		text = decoded
	}
	return json.Unmarshal([]byte(text), target)
}

func firstText(content []toolContent) string {
	for _, item := range content {
		if item.Type == "text" {
			return item.Text
		}
	}
	return ""
}

func mcpError(err *rpcError) error {
	return apperrors.NewEngineError(
		"MCP tool call failed",
		err.Message,
		map[string]string{"code": fmt.Sprintf("%d", err.Code)},
		nil,
	)
}

func mapTimeRange(value string) string {
	switch value {
	case "day":
		return "oneDay"
	case "week":
		return "oneWeek"
	case "month":
		return "oneMonth"
	case "year":
		return "oneYear"
	default:
		return "noLimit"
	}
}
