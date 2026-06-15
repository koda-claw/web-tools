package gui

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/koda-claw/web-tools/internal/config"
	apperrors "github.com/koda-claw/web-tools/internal/errors"
	"github.com/koda-claw/web-tools/internal/provider"
	mcpprovider "github.com/koda-claw/web-tools/internal/provider/mcp"
	"github.com/koda-claw/web-tools/internal/reader"
	"github.com/koda-claw/web-tools/internal/search"
	"github.com/koda-claw/web-tools/internal/setupcheck"
)

//go:embed assets/*
var embeddedAssets embed.FS

// Options configures the local GUI server.
type Options struct {
	Version  string
	Host     string
	Port     int
	NoOpen   bool
	SkillDir string
}

// Server is the local GUI HTTP server.
type Server struct {
	opts       Options
	httpServer *http.Server
	listener   net.Listener
}

// NewServer creates a GUI server with safe local defaults.
func NewServer(opts Options) *Server {
	if opts.Host == "" {
		opts.Host = "127.0.0.1"
	}
	if opts.SkillDir == "" {
		opts.SkillDir = "~/.codex/skills"
	}
	return &Server{opts: opts}
}

// Start binds and starts the HTTP server.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.opts.Host, s.opts.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.listener = listener
	s.httpServer = &http.Server{
		Handler:           s.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := s.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "web-tools gui server error: %v\n", err)
		}
	}()
	return nil
}

// Shutdown stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

// URL returns the local server URL.
func (s *Server) URL() string {
	if s.listener == nil {
		return ""
	}
	return "http://" + s.listener.Addr().String()
}

// OpenBrowser opens the GUI URL in the platform default browser.
func (s *Server) OpenBrowser() error {
	if s.opts.NoOpen {
		return nil
	}
	url := s.URL()
	if url == "" {
		return nil
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("POST /api/setup/provider", s.handleSetupProvider)
	mux.HandleFunc("POST /api/env", s.handleEnv)
	mux.HandleFunc("POST /api/test/search", s.handleTestSearch)
	mux.HandleFunc("POST /api/test/reader", s.handleTestReader)
	mux.HandleFunc("GET /api/diagnostics", s.handleDiagnostics)
	mux.HandleFunc("GET /api/agent-guide", s.handleAgentGuide)

	assets, err := fs.Sub(embeddedAssets, "assets")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServer(http.FS(assets)))
	return secureHeaders(mux)
}

func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type StatusResponse struct {
	OK            bool              `json:"ok"`
	Version       string            `json:"version"`
	Setup         setupcheck.Report `json:"setup"`
	RepositoryURL string            `json:"repository_url"`
	GeneratedAt   time.Time         `json:"generated_at"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	report := setupcheck.Run(setupcheck.Options{
		Version:  s.opts.Version,
		SkillDir: s.opts.SkillDir,
		Provider: "bigmodel",
		AuthEnv:  "ZHIPU_APIKEY",
	})
	writeJSON(w, http.StatusOK, StatusResponse{
		OK:            report.OK,
		Version:       s.opts.Version,
		Setup:         report,
		RepositoryURL: repositoryURL,
		GeneratedAt:   time.Now(),
	})
}

type providerSetupRequest struct {
	Provider         string `json:"provider"`
	AuthEnv          string `json:"auth_env"`
	EnableSearchAuto bool   `json:"enable_search_auto"`
	EnableReaderAuto bool   `json:"enable_reader_auto"`
}

func (s *Server) handleSetupProvider(w http.ResponseWriter, r *http.Request) {
	var req providerSetupRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Provider == "" {
		req.Provider = "bigmodel"
	}
	if req.AuthEnv == "" {
		req.AuthEnv = "ZHIPU_APIKEY"
	}
	if req.Provider != "bigmodel" {
		writeError(w, apperrors.NewInputError("unsupported provider", "GUI currently supports BigModel setup", []string{"use provider=bigmodel"}))
		return
	}

	path := config.UserConfigPath()
	editable, err := config.LoadEditableConfig(path)
	if err != nil {
		writeError(w, apperrors.NewInputError("cannot load config", err.Error(), []string{"check config file JSON"}))
		return
	}
	config.AddBigModelProvider(editable, req.AuthEnv, req.EnableSearchAuto, req.EnableReaderAuto)
	if err := config.SaveEditableConfig(path, editable); err != nil {
		writeError(w, apperrors.NewInputError("cannot write config", err.Error(), []string{"check config directory permissions"}))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"config_path": path,
		"provider":    req.Provider,
		"auth_env":    req.AuthEnv,
		"status":      setupcheck.Run(setupcheck.Options{Version: s.opts.Version, SkillDir: s.opts.SkillDir}),
	})
}

type envRequest struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	EnvFile string `json:"env_file"`
	Force   bool   `json:"force"`
}

func (s *Server) handleEnv(w http.ResponseWriter, r *http.Request) {
	var req envRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Key == "" {
		req.Key = "ZHIPU_APIKEY"
	}
	if strings.TrimSpace(req.Value) == "" {
		writeError(w, apperrors.NewInputError("missing env value", "env value cannot be empty", []string{"paste an API key before saving"}))
		return
	}
	if req.EnvFile == "" {
		req.EnvFile = config.EnvFilePath()
	}
	if err := config.WriteEnvValue(req.EnvFile, req.Key, req.Value, req.Force); err != nil {
		writeError(w, apperrors.NewInputError("cannot write env file", err.Error(), []string{"enable overwrite if replacing an existing key"}))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"env_file": config.ExpandHome(req.EnvFile),
		"key":      req.Key,
		"status":   setupcheck.Run(setupcheck.Options{Version: s.opts.Version, SkillDir: s.opts.SkillDir}),
	})
}

type searchTestRequest struct {
	Query     string `json:"query"`
	Provider  string `json:"provider"`
	Limit     int    `json:"limit"`
	TimeoutMS int    `json:"timeout_ms"`
}

func (s *Server) handleTestSearch(w http.ResponseWriter, r *http.Request) {
	var req searchTestRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Query) == "" {
		writeError(w, apperrors.NewInputError("missing query", "query cannot be empty", []string{"enter a search query"}))
		return
	}
	if req.Limit <= 0 || req.Limit > 10 {
		req.Limit = 3
	}
	cfg, err := config.Load()
	if err != nil {
		writeError(w, apperrors.NewInputError("cannot load configuration", err.Error(), []string{"check config file format"}))
		return
	}
	opts := search.SearchOptions{Limit: req.Limit}
	if req.Provider != "" {
		opts.Provider = req.Provider
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeoutFromMS(req.TimeoutMS, 20*time.Second))
	defer cancel()
	type result struct {
		resp *search.SearchResponse
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		resp, err := search.NewSearchWithConfig(*cfg).Do(req.Query, opts)
		ch <- result{resp: resp, err: err}
	}()
	select {
	case <-ctx.Done():
		writeError(w, apperrors.NewNetworkError("search test timed out", ctx.Err().Error(), nil, []string{"try a smaller timeout or provider=auto"}))
	case res := <-ch:
		if res.err != nil {
			writeError(w, res.err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "result": res.resp})
	}
}

type readerTestRequest struct {
	URL       string `json:"url"`
	Provider  string `json:"provider"`
	TimeoutMS int    `json:"timeout_ms"`
}

func (s *Server) handleTestReader(w http.ResponseWriter, r *http.Request) {
	var req readerTestRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.URL) == "" {
		writeError(w, apperrors.NewInputError("missing url", "url cannot be empty", []string{"enter an http:// or https:// URL"}))
		return
	}
	input, err := reader.ParseInput(req.URL)
	if err != nil || input == nil || input.Type != reader.InputURL {
		writeError(w, apperrors.NewInputError("invalid reader URL", "reader test only accepts http:// or https:// URLs", []string{"use a web URL"}))
		return
	}
	cfg, err := config.Load()
	if err != nil {
		writeError(w, apperrors.NewInputError("cannot load configuration", err.Error(), []string{"check config file format"}))
		return
	}
	if req.TimeoutMS > 0 {
		cfg.Reader.DefaultTimeout = int(timeoutFromMS(req.TimeoutMS, 20*time.Second).Seconds())
	}
	providerID := req.Provider
	if providerID == "" {
		providerID = "auto"
	}
	if providerID == "auto" && len(cfg.Reader.DefaultProviderChain) == 0 {
		providerID = "builtin-reader"
	}
	selected, err := selectGUIReaderProvider(*cfg, providerID)
	if err != nil {
		writeError(w, err)
		return
	}

	if selected.ID != "builtin-reader" {
		s.handleProviderReaderTest(w, input, *cfg, selected)
		return
	}

	fetcher := reader.NewFetcher(cfg.Reader)
	fetchResult, err := fetcher.Fetch(input.URL.String())
	if err != nil {
		writeError(w, err)
		return
	}
	defer fetchResult.Body.Close()
	extracted, err := reader.NewExtractor(cfg.Reader).Extract(fetchResult.Body, input.URL)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"result": map[string]any{
			"source":       input.URL.String(),
			"url":          fetchResult.URL,
			"title":        extracted.Title,
			"content":      extracted.Content,
			"text_content": extracted.TextContent,
			"word_count":   len(strings.Fields(extracted.TextContent)),
			"content_type": reader.GuessContentType(input.URL.String(), extracted.SiteName, extracted.Metadata),
			"extract_mode": "readability",
			"fetched_at":   time.Now(),
			"provider":     selected.ID,
		},
	})
}

func selectGUIReaderProvider(cfg config.Config, requested string) (provider.Provider, error) {
	reg, err := provider.NewRegistry(cfg.Providers)
	if err != nil {
		return provider.Provider{}, err
	}
	switch requested {
	case "", "auto":
		chain := cfg.Reader.DefaultProviderChain
		if len(chain) == 0 {
			chain = []string{"builtin-reader"}
		}
		providers, _, err := reg.ResolveChain(chain, provider.CapabilityReader)
		if err != nil {
			return provider.Provider{}, err
		}
		if len(providers) == 0 {
			return provider.Provider{}, apperrors.NewInputError(
				"no reader providers available",
				"reader auto chain did not resolve to any enabled reader providers",
				[]string{"check reader.default_provider_chain", "configure provider auth envs"},
			)
		}
		return providers[0], nil
	default:
		selected, err := reg.Get(requested, provider.CapabilityReader)
		if err != nil {
			return provider.Provider{}, err
		}
		if selected.ID != "builtin-reader" && selected.Config.Type != "mcp" {
			return provider.Provider{}, apperrors.NewInputError(
				"reader provider is not implemented in GUI",
				fmt.Sprintf("provider %q uses unsupported type %q", requested, selected.Config.Type),
				[]string{"use builtin-reader", "use a provider with type mcp"},
			)
		}
		return selected, nil
	}
}

func (s *Server) handleProviderReaderTest(w http.ResponseWriter, input *reader.Input, cfg config.Config, selected provider.Provider) {
	client := mcpprovider.NewClient(selected.Config, os.Getenv(selected.Config.AuthEnv))
	result, err := client.Read(context.Background(), input.URL.String())
	if err != nil {
		writeError(w, err)
		return
	}
	content := result.Content
	metadata := result.Metadata
	if metadata == nil {
		metadata = map[string]string{}
	}
	metadata["provider"] = selected.ID
	metadata["provider_type"] = selected.Config.Type
	title := result.Title
	if title == "" {
		title = input.URL.String()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"result": map[string]any{
			"source":       input.URL.String(),
			"url":          result.URL,
			"title":        title,
			"content":      content,
			"text_content": content,
			"word_count":   len(strings.Fields(content)),
			"content_type": reader.GuessContentType(input.URL.String(), "", metadata),
			"extract_mode": "provider:" + selected.ID,
			"fetched_at":   time.Now(),
			"provider":     selected.ID,
			"metadata":     metadata,
			"quality": map[string]any{
				"word_count": len(strings.Fields(content)),
				"min_words":  cfg.Reader.MinContentLength,
			},
		},
	})
}

func (s *Server) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	status := setupcheck.Run(setupcheck.Options{Version: s.opts.Version, SkillDir: s.opts.SkillDir})
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             status.OK,
		"version":        s.opts.Version,
		"repository_url": repositoryURL,
		"generated_at":   time.Now(),
		"setup":          status,
		"agent_guide":    buildAgentGuide(s.opts.Version, status),
	})
}

func (s *Server) handleAgentGuide(w http.ResponseWriter, r *http.Request) {
	status := setupcheck.Run(setupcheck.Options{Version: s.opts.Version, SkillDir: s.opts.SkillDir})
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":    true,
		"guide": buildAgentGuide(s.opts.Version, status),
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		writeError(w, apperrors.NewInputError("invalid JSON request", err.Error(), []string{"send application/json"}))
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	var appErr *apperrors.AppError
	if apperrors.As(err, &appErr) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": appErr})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]any{
		"ok": false,
		"error": map[string]any{
			"category": "internal",
			"message":  "request failed",
			"detail":   err.Error(),
		},
	})
}

func timeoutFromMS(ms int, fallback time.Duration) time.Duration {
	if ms <= 0 {
		return fallback
	}
	d := time.Duration(ms) * time.Millisecond
	if d < time.Second {
		return time.Second
	}
	if d > 60*time.Second {
		return 60 * time.Second
	}
	return d
}
