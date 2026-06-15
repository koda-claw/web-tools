package gui

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/koda-claw/web-tools/internal/config"
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
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Local Page</title></head><body><article><h1>Local Page</h1><p>This is enough content for a local reader smoke test with useful words.</p></article></body></html>`))
	}))
	defer page.Close()

	body := requestJSON(t, NewServer(Options{Version: "test", SkillDir: filepath.Join(dir, "skills")}), http.MethodPost, "/api/test/reader", map[string]any{
		"url":      page.URL,
		"provider": "builtin-reader",
	})

	assert.Contains(t, body, `"ok":true`)
	assert.Contains(t, body, "Local Page")
}

func TestDiagnosticsContainsGuideAndNoSecret(t *testing.T) {
	dir := isolatedHome(t)
	t.Setenv("ZHIPU_APIKEY", "super-secret-token")

	body := requestJSON(t, NewServer(Options{Version: "test", SkillDir: filepath.Join(dir, "skills")}), http.MethodGet, "/api/diagnostics", nil)

	assert.Contains(t, body, `"repository_url"`)
	assert.Contains(t, body, `"agent_guide"`)
	assert.NotContains(t, body, "super-secret-token")
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

func TestServerShutdownWithoutStart(t *testing.T) {
	server := NewServer(Options{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, server.Shutdown(ctx))
}
