package upgradecmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	apperrors "github.com/koda-claw/web-tools/internal/errors"
	"github.com/koda-claw/web-tools/internal/metrics"
	"github.com/koda-claw/web-tools/internal/upgrade"
	"github.com/spf13/cobra"
)

// Cmd returns the upgrade command.
func Cmd(version string) *cobra.Command {
	var (
		flagVersion              string
		flagRepo                 string
		flagBaseURL              string
		flagBin                  string
		flagBinDir               string
		flagSkillDir             string
		flagSkillSource          string
		flagSkipSkill            bool
		flagOnlySkill            bool
		flagCheck                bool
		flagForce                bool
		flagInsecureSkipChecksum bool
		flagJSON                 bool
	)

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade web-tools CLI and Agent skill",
		Example: `  web-tools upgrade
  web-tools upgrade --version v1.6.0
  web-tools upgrade --bin-dir "$HOME/.local/bin"
  web-tools upgrade --only-skill --skill-dir "$HOME/.codex/skills"`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			opts := upgrade.Options{
				CurrentVersion:       version,
				TargetVersion:        flagVersion,
				Repo:                 flagRepo,
				BaseURL:              flagBaseURL,
				Bin:                  flagBin,
				BinDir:               flagBinDir,
				SkillDir:             flagSkillDir,
				SkillSource:          flagSkillSource,
				SkipSkill:            flagSkipSkill,
				OnlySkill:            flagOnlySkill,
				Check:                flagCheck,
				Force:                flagForce,
				InsecureSkipChecksum: flagInsecureSkipChecksum,
			}
			result, err := upgrade.Runner{}.Run(context.Background(), opts)
			metrics.ObserveCommand(start, "upgrade", metrics.Event{
				Upgrade: metrics.UpgradeEvent{
					TargetVersion:    result.TargetVersion,
					ChecksumVerified: result.ChecksumVerified,
					BinaryMode:       result.BinaryMode,
				},
			}, err)
			if flagJSON {
				if err != nil {
					if appErr, ok := err.(*apperrors.AppError); ok {
						fmt.Fprintln(os.Stdout, string(appErr.ToJSON()))
						os.Exit(appErr.ExitCode())
					}
				} else {
					data, _ := json.MarshalIndent(result, "", "  ")
					fmt.Fprintln(os.Stdout, string(data))
					return
				}
			}
			if err != nil {
				apperrors.HandleError(err)
			}
			renderHuman(result)
		},
	}

	cmd.Flags().StringVar(&flagVersion, "version", "latest", "Target version: latest or vX.Y.Z")
	cmd.Flags().StringVar(&flagRepo, "repo", "koda-claw/web-tools", "GitHub repository for releases")
	cmd.Flags().StringVar(&flagBaseURL, "base-url", "", "Release asset base URL for mirrors or tests")
	cmd.Flags().StringVar(&flagBin, "bin", "", "CLI binary path to replace")
	cmd.Flags().StringVar(&flagBinDir, "bin-dir", "", "Directory to install web-tools binary into")
	cmd.Flags().StringVar(&flagSkillDir, "skill-dir", "~/.codex/skills", "Skill root directory")
	cmd.Flags().StringVar(&flagSkillSource, "skill-source", "", "Local SKILL.md path or HTTP(S) URL")
	cmd.Flags().BoolVar(&flagSkipSkill, "skip-skill", false, "Upgrade CLI only")
	cmd.Flags().BoolVar(&flagOnlySkill, "only-skill", false, "Install skill only")
	cmd.Flags().BoolVar(&flagCheck, "check", false, "Check target release without modifying files")
	cmd.Flags().BoolVar(&flagForce, "force", false, "Reinstall even when current version matches target")
	cmd.Flags().BoolVar(&flagInsecureSkipChecksum, "insecure-skip-checksum", false, "Skip SHA256 verification for local tests only")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "JSON structured output")
	return cmd
}

func renderHuman(result upgrade.Result) {
	fmt.Fprintln(os.Stdout, "web-tools upgrade")
	fmt.Fprintf(os.Stdout, "current: %s\n", result.CurrentVersion)
	fmt.Fprintf(os.Stdout, "target:  %s\n", result.TargetVersion)
	if result.Asset != "" {
		fmt.Fprintf(os.Stdout, "asset:   %s\n", result.Asset)
	}
	if result.BinaryPath != "" {
		fmt.Fprintf(os.Stdout, "binary:  %s\n", result.BinaryPath)
		fmt.Fprintf(os.Stdout, "mode:    %s\n", result.BinaryMode)
	}
	if result.SkillPath != "" {
		fmt.Fprintf(os.Stdout, "skill:   %s\n", result.SkillPath)
	}
	if result.ManualReplaceRequired {
		fmt.Fprintf(os.Stdout, "\nDownloaded target binary to %s\n", result.DownloadedPath)
		fmt.Fprintln(os.Stdout, "Manual replace required on this platform.")
		return
	}
	if result.CLIUpdated {
		fmt.Fprintln(os.Stdout, "\nReplaced CLI")
	}
	if result.SkillUpdated {
		fmt.Fprintln(os.Stdout, "Installed skill")
	}
	fmt.Fprintf(os.Stdout, "Done: web-tools version %s\n", result.TargetVersion)
}
