package websearch

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/koda-claw/web-tools/internal/config"
	apperrors "github.com/koda-claw/web-tools/internal/errors"
	"github.com/koda-claw/web-tools/internal/metrics"
	"github.com/koda-claw/web-tools/internal/search"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	var (
		flagJSON           bool
		flagOutput         string
		flagLimit          int
		flagEngine         string
		flagProvider       string
		flagLocale         string
		flagCat            string
		flagTime           string
		flagIncludeDomains []string
		flagExcludeDomains []string
	)

	cmd := &cobra.Command{
		Use:   "web-search <query>",
		Short: "Search the web (DuckDuckGo Lite by default, SearXNG optional)",
		Long: `Search the web using DuckDuckGo Lite (default, zero dependencies) or a local SearXNG
instance (opt-in, requires Docker). Zero cost, no API keys.`,
		Example: `  web-tools web-search "latest AI news"
  web-tools web-search "AI latest developments" --locale en-US --time-range week
  web-tools web-search "Tesla" --category news --time-range day --limit 10
  web-tools web-search "site:github.com go readability" --limit 3 --json
  web-tools web-search "deep learning" --locale en-US --limit 3 -o /tmp/results.md
  web-tools web-search "climate change 2026" --time-range year --json`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			run(cmd, args[0], flagJSON, flagOutput, flagLimit, flagEngine, flagProvider, flagLocale, flagCat, flagTime, flagIncludeDomains, flagExcludeDomains)
		},
	}

	cmd.Flags().BoolVar(&flagJSON, "json", false, "JSON structured output")
	cmd.Flags().StringVarP(&flagOutput, "output", "o", "", "Output to file")
	cmd.Flags().IntVarP(&flagLimit, "limit", "n", 5, "Number of results")
	cmd.Flags().StringVar(&flagEngine, "engine", "auto", "Search engine: auto / duckduckgo / searxng")
	cmd.Flags().StringVar(&flagProvider, "provider", "auto", "Search provider: auto / duckduckgo / searxng")
	cmd.Flags().StringVar(&flagLocale, "locale", "auto", "Language preference (zh-CN, en-US, auto)")
	cmd.Flags().StringVar(&flagCat, "category", "general", "Search category: general / images / news / videos / files")
	cmd.Flags().StringVar(&flagTime, "time-range", "any", "Time range: any / day / week / month / year")
	cmd.Flags().StringSliceVar(&flagIncludeDomains, "include-domain", nil, "Only include results from domain(s); repeat or comma-separate")
	cmd.Flags().StringSliceVar(&flagExcludeDomains, "exclude-domain", nil, "Exclude results from domain(s); repeat or comma-separate")

	return cmd
}

func run(cmd *cobra.Command, query string, flagJSON bool, flagOutput string, flagLimit int, flagEngine string, flagProvider string, flagLocale string, flagCategory string, flagTimeRange string, flagIncludeDomains []string, flagExcludeDomains []string) {
	start := time.Now()
	var metric metrics.Event
	var runErr error
	defer func() {
		metrics.ObserveCommand(start, "web-search", metric, runErr)
	}()

	cfg, err := loadSearchRuntimeConfig()
	if err != nil {
		runErr = apperrors.NewInputError(
			"cannot load configuration",
			err.Error(),
			[]string{"check config file format", "check environment variables"},
		)
		apperrors.HandleError(runErr)
	}
	s := search.NewSearchWithConfig(*cfg)

	opts, err := buildSearchOptions(cmd, flagLimit, flagEngine, flagProvider, flagLocale, flagCategory, flagTimeRange, flagIncludeDomains, flagExcludeDomains)
	if err != nil {
		runErr = err
		apperrors.HandleError(runErr)
	}

	resp, err := s.Do(query, opts)
	if err != nil {
		runErr = err
		apperrors.HandleError(runErr)
	}
	metric.Provider = firstNonEmpty(resp.Provider, resp.Engine, opts.Provider, opts.Engine)
	metric.ResultCount = resp.Total

	var output string
	if flagJSON {
		output = resp.RenderJSON()
	} else {
		output = resp.RenderMarkdown()
	}

	if flagOutput != "" {
		if err := os.WriteFile(flagOutput, []byte(output), 0644); err != nil {
			runErr = apperrors.NewInputError(
				"cannot write to output file",
				err.Error(),
				[]string{"check output path write permissions"},
			)
			apperrors.HandleError(runErr)
		}
	} else {
		fmt.Println(output)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" && strings.TrimSpace(value) != "auto" {
			return value
		}
	}
	return ""
}

func buildSearchOptions(cmd *cobra.Command, flagLimit int, flagEngine string, flagProvider string, flagLocale string, flagCategory string, flagTimeRange string, flagIncludeDomains []string, flagExcludeDomains []string) (search.SearchOptions, error) {
	var opts search.SearchOptions
	engineChanged := cmd.Flags().Changed("engine")
	providerChanged := cmd.Flags().Changed("provider")
	if engineChanged && providerChanged && flagEngine != flagProvider {
		return opts, apperrors.NewInputError(
			"conflicting provider flags",
			fmt.Sprintf("--engine=%q conflicts with --provider=%q", flagEngine, flagProvider),
			[]string{"use only --provider", "or pass the same value to --engine and --provider"},
		)
	}
	if cmd.Flags().Changed("limit") {
		opts.Limit = flagLimit
	}
	if engineChanged {
		opts.Engine = flagEngine
	}
	if providerChanged {
		opts.Provider = flagProvider
	}
	if cmd.Flags().Changed("locale") {
		opts.Locale = flagLocale
	}
	if cmd.Flags().Changed("category") {
		opts.Category = flagCategory
	}
	if cmd.Flags().Changed("time-range") {
		opts.TimeRange = flagTimeRange
	}
	if cmd.Flags().Changed("include-domain") {
		opts.IncludeDomains = normalizeDomainFlags(flagIncludeDomains)
	}
	if cmd.Flags().Changed("exclude-domain") {
		opts.ExcludeDomains = normalizeDomainFlags(flagExcludeDomains)
	}
	return opts, nil
}

func normalizeDomainFlags(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func loadSearchConfig() (config.SearchConfig, error) {
	cfg, err := loadSearchRuntimeConfig()
	if err != nil {
		return config.SearchConfig{}, err
	}
	return cfg.Search, nil
}

func loadSearchRuntimeConfig() (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
