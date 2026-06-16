package webreader

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/koda-claw/web-tools/internal/config"
	apperrors "github.com/koda-claw/web-tools/internal/errors"
	"github.com/koda-claw/web-tools/internal/metrics"
	"github.com/koda-claw/web-tools/internal/provider"
	mcpprovider "github.com/koda-claw/web-tools/internal/provider/mcp"
	"github.com/koda-claw/web-tools/internal/reader"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	var (
		flagJSON     bool
		flagOutput   string
		flagExtract  string
		flagMaxWord  int
		flagTimeout  int
		flagNoCache  bool
		flagBrowser  bool
		flagSession  string
		flagUA       string
		flagFormat   string
		flagProvider string
	)

	cmd := &cobra.Command{
		Use:   "web-reader <input>",
		Short: "Extract readable content from URL or local file",
		Long: `Fetch a URL or read a local file, extract the main content, and output as Markdown.
Supports web pages, PDFs, Word, PowerPoint, Excel, and text files.`,
		Example: `  web-tools web-reader https://example.com/article
  web-tools web-reader https://example.com/article --max-words 100
  web-tools web-reader https://example.com/article --json
  web-tools web-reader https://spa-site.com/page --browser
  web-tools web-reader https://internal.company.com/doc --session work
  web-tools web-reader ./report.pdf
  web-tools web-reader ./slides.pptx -o /tmp/slides.md
  web-tools web-reader https://example.com/article --no-cache --timeout 30`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			run(args[0], flagJSON, flagOutput, flagExtract, flagMaxWord, flagTimeout, flagNoCache, flagBrowser, flagSession, flagUA, flagFormat, flagProvider)
		},
	}

	cmd.Flags().BoolVar(&flagJSON, "json", false, "JSON structured output")
	cmd.Flags().StringVarP(&flagOutput, "output", "o", "", "Output to file")
	cmd.Flags().StringVar(&flagExtract, "extract", "main", "Extract mode: main (body) / full (full page)")
	cmd.Flags().IntVar(&flagMaxWord, "max-words", 0, "Limit output word count (0 = unlimited)")
	cmd.Flags().IntVar(&flagTimeout, "timeout", 15, "Request timeout in seconds")
	cmd.Flags().BoolVar(&flagNoCache, "no-cache", false, "Ignore cache, force refresh")
	cmd.Flags().BoolVar(&flagBrowser, "browser", false, "Force browser rendering via agent-browser")
	cmd.Flags().StringVar(&flagSession, "session", "", "agent-browser session name for login state")
	cmd.Flags().StringVar(&flagUA, "user-agent", "", "Custom User-Agent")
	cmd.Flags().StringVar(&flagFormat, "format", "markdown", "Output format: markdown / text / html")
	cmd.Flags().StringVar(&flagProvider, "provider", "auto", "Reader provider: auto / builtin-reader")

	return cmd
}

const (
	extractMain = "main"
	extractFull = "full"

	formatMarkdown = "markdown"
	formatText     = "text"
	formatHTML     = "html"
)

// PipelineResult is the final output structure combining fetch + extract info.
type PipelineResult struct {
	Source        string            `json:"source"`
	URL           string            `json:"url,omitempty"`
	Title         string            `json:"title"`
	Content       string            `json:"content"`
	TextContent   string            `json:"text_content,omitempty"`
	HTML          string            `json:"html,omitempty"`
	Format        string            `json:"format"`
	FetchedAt     time.Time         `json:"fetched_at"`
	WordCount     int               `json:"word_count"`
	ContentType   string            `json:"content_type"`
	ExtractMode   string            `json:"extract_mode"`
	Language      string            `json:"language,omitempty"`
	PublishedTime *time.Time        `json:"published_time,omitempty"`
	ModifiedTime  *time.Time        `json:"modified_time,omitempty"`
	SiteName      string            `json:"site_name,omitempty"`
	Image         string            `json:"image,omitempty"`
	NeedsFallback bool              `json:"needs_fallback,omitempty"`
	Quality       *QualityInfo      `json:"quality,omitempty"`
	CacheHit      bool              `json:"cache_hit,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type QualityInfo struct {
	Score         string   `json:"score"`
	WordCount     int      `json:"word_count"`
	MinWords      int      `json:"min_words"`
	NeedsFallback bool     `json:"needs_fallback"`
	Reasons       []string `json:"reasons,omitempty"`
}

func run(rawInput string, flagJSON bool, flagOutput string, flagExtract string, flagMaxWord int, flagTimeout int, flagNoCache bool, flagBrowser bool, flagSession string, flagUA string, flagFormat string, flagProvider string) {
	start := time.Now()
	var metric metrics.Event
	var runErr error
	defer func() {
		metrics.ObserveCommand(start, "web-reader", metric, runErr)
	}()

	if err := validateReaderFlags(flagExtract, flagFormat); err != nil {
		runErr = err
		apperrors.HandleError(runErr)
	}

	// 1. Parse input
	input, err := reader.ParseInput(rawInput)
	if err != nil {
		runErr = err
		apperrors.HandleError(runErr)
	}
	if input == nil {
		runErr = apperrors.NewInputError(
			"cannot recognize input",
			fmt.Sprintf("%q is not a valid URL or file path", rawInput),
			[]string{"provide an http:// or https:// URL", "provide a local file path"},
		)
		apperrors.HandleError(runErr)
	}

	// 2. Load config with flag overrides
	cfg, err := loadReaderRuntimeConfig(flagTimeout)
	if err != nil {
		runErr = apperrors.NewInputError(
			"cannot load configuration",
			err.Error(),
			[]string{"check config file format", "check environment variables"},
		)
		apperrors.HandleError(runErr)
	}
	selectedProvider, err := selectReaderProvider(cfg, flagProvider)
	if err != nil {
		runErr = err
		apperrors.HandleError(runErr)
	}
	metric.Provider = selectedProvider.ID

	// 3. Initialize cache (URL inputs only)
	var cache *reader.Cache
	if input.Type == reader.InputURL {
		cache, _ = reader.NewCache(cfg.Reader.CacheDir, cfg.Reader.CacheTTL)
	}

	// 4. Handle file inputs
	if input.Type == reader.InputFile {
		if selectedProvider.ID != "builtin-reader" {
			runErr = apperrors.NewInputError(
				"reader provider only supports URL input",
				fmt.Sprintf("provider %q cannot read local files", selectedProvider.ID),
				[]string{"use --provider builtin-reader for local files"},
			)
			apperrors.HandleError(runErr)
		}
		result, err := handleFileInput(input, cfg, flagExtract, flagFormat)
		if err != nil {
			runErr = err
			apperrors.HandleError(runErr)
		}
		applyReaderMetrics(&metric, result)
		if err := outputResult(result, flagJSON, flagOutput, flagMaxWord, flagFormat); err != nil {
			runErr = err
			apperrors.HandleError(runErr)
		}
		return
	}

	// 5. Handle URL inputs
	var result *PipelineResult
	if selectedProvider.ID == "builtin-reader" {
		result, err = handleURLInput(input, cfg, flagUA, cache, flagNoCache, flagBrowser, flagSession, flagExtract, flagFormat)
	} else {
		result, err = handleProviderURLInput(input, cfg, selectedProvider, flagFormat)
	}
	if err != nil {
		runErr = err
		apperrors.HandleError(runErr)
	}

	// 6. Check extraction quality
	if result.NeedsFallback {
		if result.Quality != nil {
			fmt.Fprintf(os.Stderr, "[WARN] extracted content quality is %s (%s); try --browser for JS-rendered pages\n",
				result.Quality.Score, strings.Join(result.Quality.Reasons, ", "))
		} else {
			fmt.Fprintln(os.Stderr, "[WARN] extracted content seems sparse, try --browser for JS-rendered pages")
		}
	}

	// 7. Output
	applyReaderMetrics(&metric, result)
	if err := outputResult(result, flagJSON, flagOutput, flagMaxWord, flagFormat); err != nil {
		runErr = err
		apperrors.HandleError(runErr)
	}
}

func applyReaderMetrics(event *metrics.Event, result *PipelineResult) {
	if result == nil {
		return
	}
	event.WordCountBucket = metrics.WordCountBucket(result.WordCount)
	if result.Quality != nil {
		event.Quality = result.Quality.Score
		event.FallbackRecommended = result.Quality.NeedsFallback
	} else {
		event.FallbackRecommended = result.NeedsFallback
	}
}

func validateReaderProvider(cfg config.Config, requested string) error {
	_, err := selectReaderProvider(cfg, requested)
	return err
}

func selectReaderProvider(cfg config.Config, requested string) (provider.Provider, error) {
	if requested == "" {
		requested = cfg.Reader.DefaultProvider
	}
	if requested == "" {
		requested = "auto"
	}
	reg, err := provider.NewRegistry(cfg.Providers)
	if err != nil {
		return provider.Provider{}, err
	}
	switch requested {
	case "auto":
		chain := cfg.Reader.DefaultProviderChain
		if len(chain) == 0 {
			chain = []string{"builtin-reader"}
		}
		providers, _, err := reg.ResolveChain(chain, provider.CapabilityReader)
		if err != nil {
			return provider.Provider{}, err
		}
		if len(providers) > 0 {
			return providers[0], nil
		}
		return provider.Provider{}, apperrors.NewInputError(
			"no reader providers available",
			"reader auto chain did not resolve to any enabled reader providers",
			[]string{"check reader.default_provider_chain", "configure provider auth envs"},
		)
	case "builtin-reader":
		return reg.Get("builtin-reader", provider.CapabilityReader)
	default:
		selected, err := reg.Get(requested, provider.CapabilityReader)
		if err != nil {
			return provider.Provider{}, err
		}
		if selected.Config.Type != "mcp" {
			return provider.Provider{}, apperrors.NewInputError(
				"reader provider is not implemented yet",
				fmt.Sprintf("provider %q uses unsupported type %q", requested, selected.Config.Type),
				[]string{"use --provider builtin-reader", "use a provider with type mcp"},
			)
		}
		return selected, nil
	}
}

func handleProviderURLInput(input *reader.Input, cfg config.Config, selected provider.Provider, format string) (*PipelineResult, error) {
	client := mcpprovider.NewClient(selected.Config, os.Getenv(selected.Config.AuthEnv))
	result, err := client.Read(context.Background(), input.URL.String())
	if err != nil {
		return nil, err
	}
	content := result.Content
	wordCount := len(strings.Fields(content))
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
	return &PipelineResult{
		Source:      input.URL.String(),
		URL:         result.URL,
		Title:       title,
		Content:     content,
		TextContent: content,
		Format:      format,
		FetchedAt:   time.Now(),
		WordCount:   wordCount,
		ContentType: reader.GuessContentType(input.URL.String(), "", metadata),
		ExtractMode: "provider:" + selected.ID,
		Quality:     assessQuality(wordCount, cfg.Reader.MinContentLength, "provider:"+selected.ID),
		Metadata:    metadata,
	}, nil
}

func validateReaderFlags(extractMode string, format string) error {
	switch extractMode {
	case extractMain, extractFull:
	default:
		return apperrors.NewInputError(
			"unsupported extract mode",
			fmt.Sprintf("got %q; supported: main, full", extractMode),
			[]string{"use --extract main", "use --extract full"},
		)
	}

	switch format {
	case formatMarkdown, formatText, formatHTML:
	default:
		return apperrors.NewInputError(
			"unsupported output format",
			fmt.Sprintf("got %q; supported: markdown, text, html", format),
			[]string{"use --format markdown", "use --format text", "use --format html"},
		)
	}

	return nil
}

func loadReaderRuntimeConfig(flagTimeout int) (config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.Config{}, err
	}
	if flagTimeout > 0 {
		cfg.Reader.DefaultTimeout = flagTimeout
	}
	return *cfg, nil
}

// isHTTPStatusError checks if the error is an HTTP 4xx/5xx that should NOT trigger browser fallback.
// Browsers can't help with 403 (bot blocking), 404 (not found), etc.
func isHTTPStatusError(err error) bool {
	var appErr *apperrors.AppError
	if !apperrors.As(err, &appErr) {
		return false
	}
	// network errors with 4xx status or "unreachable" category (404) are pointless for browser
	return appErr.Category == "network" || appErr.Category == "unreachable"
}

func handleURLInput(input *reader.Input, cfg config.Config, customUA string, cache *reader.Cache, noCache bool, useBrowser bool, session string, extractMode string, format string) (*PipelineResult, error) {
	// Check cache first
	if cache != nil && !noCache && !useBrowser {
		entry, content, hit := cache.Get(input.URL.String())
		if hit {
			fmt.Fprintln(os.Stderr, "[CACHE HIT] "+input.URL.String())
			return &PipelineResult{
				Source:      entry.URL,
				Title:       "",
				Content:     content,
				FetchedAt:   entry.CachedAt,
				WordCount:   entry.WordCount,
				ContentType: entry.ContentType,
				ExtractMode: "cached",
				Format:      format,
				CacheHit:    true,
				Quality:     assessQuality(entry.WordCount, cfg.Reader.MinContentLength, "cache"),
			}, nil
		}
	}

	// --browser mode: use agent-browser directly
	if useBrowser {
		return handleBrowserInput(input, cfg, session, extractMode, format)
	}

	// Default: fetch + extract
	fetcher := reader.NewFetcher(cfg.Reader)
	if customUA != "" {
		fetcher.SetUserAgent(customUA)
	}

	fetchResult, err := fetcher.Fetch(input.URL.String())
	if err != nil {
		// HTTP 4xx/5xx and 404: don't waste time with browser fallback
		if isHTTPStatusError(err) {
			return nil, err
		}
		// Network errors (timeout, DNS, connection refused): try browser
		if cfg.Reader.BrowserFallback {
			fmt.Fprintf(os.Stderr, "[WARN] HTTP fetch failed (%v), trying browser fallback\n", err)
			return handleBrowserInput(input, cfg, session, extractMode, format)
		}
		return nil, err
	}
	defer fetchResult.Body.Close()

	extractor := reader.NewExtractor(cfg.Reader)
	extractResult, err := extractor.Extract(fetchResult.Body, input.URL)
	if err != nil {
		// Extraction failure: browser can help with JS-rendered pages
		if cfg.Reader.BrowserFallback {
			fmt.Fprintf(os.Stderr, "[WARN] extraction failed (%v), trying browser fallback\n", err)
			return handleBrowserInput(input, cfg, session, extractMode, format)
		}
		return nil, err
	}

	contentType := reader.GuessContentType(input.URL.String(), extractResult.SiteName, extractResult.Metadata)
	wordCount := len(strings.Fields(extractResult.TextContent))

	quality := assessQuality(wordCount, cfg.Reader.MinContentLength, "readability")
	result := &PipelineResult{
		Source:        input.URL.String(),
		URL:           fetchResult.URL,
		Title:         extractResult.Title,
		Content:       extractResult.Content,
		TextContent:   extractResult.TextContent,
		HTML:          extractResult.HTML,
		Format:        format,
		FetchedAt:     time.Now(),
		WordCount:     wordCount,
		ContentType:   contentType,
		ExtractMode:   extractModeName("readability", extractMode),
		Language:      extractResult.Language,
		PublishedTime: extractResult.PublishedTime,
		ModifiedTime:  extractResult.ModifiedTime,
		SiteName:      extractResult.SiteName,
		Image:         extractResult.Image,
		Metadata:      extractResult.Metadata,
		NeedsFallback: quality.NeedsFallback,
		Quality:       quality,
	}

	if cache != nil {
		cacheEntry := &reader.CacheEntry{
			URL:         input.URL.String(),
			CachedAt:    time.Now(),
			WordCount:   wordCount,
			HTTPStatus:  fetchResult.StatusCode,
			ContentType: fetchResult.ContentType,
		}
		if err := cache.Set(input.URL.String(), cacheEntry, extractResult.Content); err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] cache write failed: %v\n", err)
		}
	}

	return result, nil
}

func handleBrowserInput(input *reader.Input, cfg config.Config, session string, extractMode string, format string) (*PipelineResult, error) {
	browser := reader.NewBrowserFallback(cfg.Reader)

	title, content, err := browser.Extract(input.URL.String(), session)
	if err != nil {
		return nil, err
	}
	defer browser.Close(session)

	wordCount := len(strings.Fields(content))
	quality := assessQuality(wordCount, cfg.Reader.MinContentLength, "browser")

	return &PipelineResult{
		Source:      input.URL.String(),
		Title:       title,
		Content:     content,
		TextContent: content,
		Format:      format,
		FetchedAt:   time.Now(),
		WordCount:   wordCount,
		ContentType: reader.GuessContentType(input.URL.String(), "", nil),
		ExtractMode: extractModeName("browser", extractMode),
		Quality:     quality,
		Metadata: map[string]string{
			"engine": "agent-browser",
		},
	}, nil
}

func handleFileInput(input *reader.Input, cfg config.Config, extractMode string, flagFormat string) (*PipelineResult, error) {
	data, err := os.ReadFile(input.FilePath)
	if err != nil {
		return nil, apperrors.NewInputError(
			"cannot read file",
			err.Error(),
			[]string{"check file path", "check read permissions"},
		)
	}

	content := string(data)

	// Text files: return directly
	if !input.NeedsConversion() {
		return &PipelineResult{
			Source:      input.FilePath,
			Title:       input.FilePath,
			Content:     content,
			TextContent: content,
			FetchedAt:   time.Now(),
			WordCount:   len(strings.Fields(content)),
			Format:      flagFormat,
			ExtractMode: extractModeName("file", extractMode),
			Quality:     assessQuality(len(strings.Fields(content)), cfg.Reader.MinContentLength, "file"),
			Metadata: map[string]string{
				"source_type": "file",
				"extension":   input.Extension(),
			},
		}, nil
	}

	// Binary files: use markitdown converter
	converter := reader.NewConverter(cfg.Reader)
	if !converter.Available() {
		return nil, apperrors.NewEngineError(
			"markitdown not found",
			fmt.Sprintf("file type %s needs markitdown, but %q not found", input.Extension(), cfg.Reader.MarkitdownPath),
			map[string]string{"file": input.FilePath, "extension": input.Extension()},
			[]string{"install markitdown: pip install markitdown", "or: pipx install markitdown"},
		)
	}

	converted, err := converter.Convert(input.FilePath)
	if err != nil {
		return nil, err
	}

	return &PipelineResult{
		Source:      input.FilePath,
		Title:       input.FilePath,
		Content:     converted,
		TextContent: converted,
		FetchedAt:   time.Now(),
		WordCount:   len(strings.Fields(converted)),
		Format:      flagFormat,
		ExtractMode: extractModeName("markitdown", extractMode),
		Quality:     assessQuality(len(strings.Fields(converted)), cfg.Reader.MinContentLength, "markitdown"),
		Metadata: map[string]string{
			"source_type": "file",
			"extension":   input.Extension(),
			"converter":   "markitdown",
		},
	}, nil
}

func assessQuality(wordCount int, minWords int, mode string) *QualityInfo {
	if minWords <= 0 {
		minWords = config.DefaultMinContentLength
	}
	q := &QualityInfo{
		Score:         "high",
		WordCount:     wordCount,
		MinWords:      minWords,
		NeedsFallback: false,
	}
	if wordCount == 0 {
		q.Score = "empty"
		q.NeedsFallback = true
		q.Reasons = append(q.Reasons, "empty content")
	} else if wordCount < minWords {
		q.Score = "low"
		q.NeedsFallback = true
		q.Reasons = append(q.Reasons, fmt.Sprintf("word count %d below minimum %d", wordCount, minWords))
	}
	if mode != "" {
		q.Reasons = append(q.Reasons, "mode: "+mode)
	}
	return q
}

func extractModeName(source string, extractMode string) string {
	if extractMode == extractFull {
		return source + "-full"
	}
	return source
}

func outputResult(result *PipelineResult, flagJSON bool, flagOutput string, flagMaxWord int, flagFormat string) error {
	if flagMaxWord > 0 {
		words := strings.Fields(result.Content)
		if len(words) > flagMaxWord {
			result.Content = strings.Join(words[:flagMaxWord], " ") + "\n\n... (truncated)"
			if result.TextContent != "" {
				textWords := strings.Fields(result.TextContent)
				if len(textWords) > flagMaxWord {
					result.TextContent = strings.Join(textWords[:flagMaxWord], " ") + "\n\n... (truncated)"
				}
			}
			result.WordCount = flagMaxWord
		}
	}

	output, err := renderOutput(result, flagJSON, flagFormat)
	if err != nil {
		return err
	}

	if flagOutput != "" {
		if err := os.WriteFile(flagOutput, []byte(output), 0644); err != nil {
			return apperrors.NewInputError(
				"cannot write to output file",
				err.Error(),
				[]string{"check output path write permissions"},
			)
		}
	} else {
		fmt.Println(output)
	}
	return nil
}

func renderOutput(result *PipelineResult, asJSON bool, format string) (string, error) {
	if asJSON {
		return result.RenderJSON(), nil
	}

	switch format {
	case formatMarkdown:
		return result.RenderMarkdown(), nil
	case formatText:
		return result.RenderText(), nil
	case formatHTML:
		return result.RenderHTML()
	default:
		return "", apperrors.NewInputError(
			"unsupported output format",
			fmt.Sprintf("got %q; supported: markdown, text, html", format),
			[]string{"use --format markdown", "use --format text", "use --format html"},
		)
	}
}

func (r *PipelineResult) RenderMarkdown() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("<!-- source: %s -->\n", r.Source))
	if r.URL != "" && r.URL != r.Source {
		sb.WriteString(fmt.Sprintf("<!-- url: %s -->\n", r.URL))
	}
	sb.WriteString(fmt.Sprintf("<!-- title: %s -->\n", r.Title))
	sb.WriteString(fmt.Sprintf("<!-- fetched: %s -->\n", r.FetchedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("<!-- words: %d -->\n", r.WordCount))
	if r.ContentType != "" {
		sb.WriteString(fmt.Sprintf("<!-- type: %s -->\n", r.ContentType))
	}
	if r.ExtractMode != "" {
		sb.WriteString(fmt.Sprintf("<!-- extract_mode: %s -->\n", r.ExtractMode))
	}
	if r.PublishedTime != nil {
		sb.WriteString(fmt.Sprintf("<!-- published: %s -->\n", r.PublishedTime.Format(time.RFC3339)))
	}
	if r.ModifiedTime != nil {
		sb.WriteString(fmt.Sprintf("<!-- modified: %s -->\n", r.ModifiedTime.Format(time.RFC3339)))
	}
	if r.Language != "" {
		sb.WriteString(fmt.Sprintf("<!-- language: %s -->\n", r.Language))
	}
	if r.SiteName != "" {
		sb.WriteString(fmt.Sprintf("<!-- site: %s -->\n", r.SiteName))
	}
	if r.CacheHit {
		sb.WriteString("<!-- cache: hit -->\n")
	}
	if r.Quality != nil {
		sb.WriteString(fmt.Sprintf("<!-- quality: %s -->\n", r.Quality.Score))
		if r.Quality.NeedsFallback {
			sb.WriteString("<!-- needs_fallback: true -->\n")
		}
	}
	sb.WriteString("\n")

	if r.Title != "" {
		sb.WriteString(fmt.Sprintf("# %s\n\n", r.Title))
	}

	if len(r.Metadata) > 0 {
		for k, v := range r.Metadata {
			if v != "" {
				sb.WriteString(fmt.Sprintf("> **%s:** %s\n", k, v))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString(r.Content)
	sb.WriteString("\n")

	return sb.String()
}

func (r *PipelineResult) RenderText() string {
	if r.TextContent != "" {
		return strings.TrimSpace(r.TextContent)
	}
	return strings.TrimSpace(r.Content)
}

func (r *PipelineResult) RenderHTML() (string, error) {
	if strings.TrimSpace(r.HTML) == "" {
		return "", apperrors.NewInputError(
			"HTML output is unavailable for this input",
			"the input did not produce extracted HTML",
			[]string{"use --format markdown", "use --format text", "use --json to inspect available fields"},
		)
	}
	return strings.TrimSpace(r.HTML), nil
}

func (r *PipelineResult) RenderJSON() string {
	type jsonOutput struct {
		OK     bool            `json:"ok"`
		Result *PipelineResult `json:"result"`
	}
	resp := jsonOutput{OK: true, Result: r}
	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		data, _ = json.Marshal(resp)
	}
	return string(data)
}
