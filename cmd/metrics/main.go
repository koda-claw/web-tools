package metricscmd

import (
	"encoding/json"
	"fmt"
	"os"

	apperrors "github.com/koda-claw/web-tools/internal/errors"
	"github.com/koda-claw/web-tools/internal/metrics"
	"github.com/spf13/cobra"
)

// Cmd returns the metrics command tree.
func Cmd() *cobra.Command {
	var (
		flagJSON   bool
		flagFile   string
		flagRange  string
		flagBucket string
	)

	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "Show local non-sensitive usage metrics",
		Example: `  web-tools metrics
  web-tools metrics --json
  web-tools metrics --range 24h --json
  web-tools metrics reset --json`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if err := showMetrics(flagFile, flagRange, flagBucket, flagJSON); err != nil {
				apperrors.HandleError(err)
			}
		},
	}
	cmd.Flags().BoolVar(&flagJSON, "json", false, "JSON structured output")
	cmd.Flags().StringVar(&flagFile, "file", "", "Metrics file path")
	cmd.Flags().StringVar(&flagRange, "range", "all", "Time range: 1h, 24h, 7d, 30d, all")
	cmd.Flags().StringVar(&flagBucket, "bucket", "auto", "Bucket: auto, hour, day")
	cmd.AddCommand(resetCmd(&flagFile, &flagJSON))
	return cmd
}

func resetCmd(flagFile *string, flagJSON *bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset local metrics",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			store := metrics.NewStore(*flagFile)
			if err := store.Reset(); err != nil {
				apperrors.HandleError(apperrors.NewInputError("cannot reset metrics", err.Error(), []string{"check metrics file permissions"}))
			}
			if *flagJSON {
				fmt.Fprintln(os.Stdout, `{"ok":true}`)
				return
			}
			fmt.Fprintln(os.Stdout, "Reset web-tools metrics")
		},
	}
	cmd.Flags().BoolVar(flagJSON, "json", false, "JSON structured output")
	cmd.Flags().StringVar(flagFile, "file", "", "Metrics file path")
	return cmd
}

func showMetrics(file string, rawRange string, rawBucket string, jsonOut bool) error {
	r, err := metrics.ParseRange(rawRange)
	if err != nil {
		return apperrors.NewInputError("invalid metrics range", err.Error(), []string{"use --range 1h, 24h, 7d, 30d, or all"})
	}
	b, err := metrics.ParseBucket(rawBucket)
	if err != nil {
		return apperrors.NewInputError("invalid metrics bucket", err.Error(), []string{"use --bucket auto, hour, or day"})
	}
	snap, err := metrics.NewStore(file).Snapshot(r, b)
	if err != nil {
		return apperrors.NewInputError("cannot load metrics", err.Error(), []string{"check metrics file permissions", "run web-tools metrics reset"})
	}
	if jsonOut {
		data, _ := json.MarshalIndent(snap, "", "  ")
		fmt.Fprintln(os.Stdout, string(data))
		return nil
	}
	renderText(snap)
	return nil
}

func renderText(snap metrics.Snapshot) {
	fmt.Fprintln(os.Stdout, "web-tools metrics")
	if snap.Disabled {
		fmt.Fprintln(os.Stdout, "status: disabled")
		return
	}
	if snap.Period.FirstSeenAt.IsZero() {
		fmt.Fprintln(os.Stdout, "period: no local metrics yet")
	} else {
		fmt.Fprintf(os.Stdout, "period: %s - %s\n", snap.Period.FirstSeenAt.Format("2006-01-02 15:04:05"), snap.Period.LastSeenAt.Format("2006-01-02 15:04:05"))
	}
	fmt.Fprintln(os.Stdout, "\ncommands:")
	if len(snap.Commands) == 0 {
		fmt.Fprintln(os.Stdout, "  none")
	}
	for name, counter := range snap.Commands {
		fmt.Fprintf(os.Stdout, "  %s total=%d success=%d error=%d avg=%dms\n", name, counter.Total, counter.Success, counter.Error, counter.AvgDurationMS)
	}
	fmt.Fprintln(os.Stdout, "\nreader quality:")
	fmt.Fprintf(os.Stdout, "  high=%d medium=%d low=%d fallback_recommended=%d\n", snap.ReaderQuality.High, snap.ReaderQuality.Medium, snap.ReaderQuality.Low, snap.ReaderQuality.FallbackRecommended)
}
