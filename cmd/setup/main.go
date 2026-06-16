package setupcmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	configcmd "github.com/koda-claw/web-tools/cmd/config"
	"github.com/koda-claw/web-tools/cmd/doctor"
	"github.com/koda-claw/web-tools/internal/config"
	apperrors "github.com/koda-claw/web-tools/internal/errors"
	"github.com/koda-claw/web-tools/internal/metrics"
	"github.com/koda-claw/web-tools/internal/setupcheck"
	"github.com/koda-claw/web-tools/internal/skillinstall"
	"github.com/spf13/cobra"
)

// Cmd returns the setup command.
func Cmd(version string) *cobra.Command {
	var (
		flagProvider         string
		flagAuthEnv          string
		flagConfig           string
		flagInstallSkill     bool
		flagSkillDir         string
		flagSkillSource      string
		flagForceSkill       bool
		flagEnableSearchAuto bool
		flagEnableReaderAuto bool
		flagEnvFile          string
		flagSetEnv           string
		flagForceEnv         bool
		flagSkipDoctor       bool
		flagCheck            bool
		flagJSON             bool
	)

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Run guided Agent setup",
		Long: `Run a practical setup sequence for agents: optionally install the skill,
optionally configure a provider preset, and run doctor so the caller can see
what still needs attention.`,
		Example: `  web-tools setup
  web-tools setup --provider bigmodel --auth-env ZHIPU_APIKEY
  web-tools setup --provider bigmodel --auth-env ZHIPU_APIKEY --set-env ZHIPU_APIKEY=...
  web-tools setup --provider bigmodel --enable-search-auto --install-skill`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			start := time.Now()
			if flagCheck {
				report := setupcheck.Run(setupcheck.Options{
					Version:  version,
					SkillDir: flagSkillDir,
					Provider: flagProvider,
					AuthEnv:  flagAuthEnv,
				})
				if flagJSON {
					data, _ := json.MarshalIndent(report, "", "  ")
					fmt.Println(string(data))
				} else {
					fmt.Print(report.RenderText())
				}
				var err error
				if !report.OK {
					err = apperrors.NewInputError(
						"setup check failed",
						"one or more setup checks need attention",
						[]string{"run web-tools setup", "run web-tools doctor --json"},
					)
					metrics.ObserveCommand(start, "setup", metrics.Event{}, err)
					os.Exit(1)
				}
				metrics.ObserveCommand(start, "setup", metrics.Event{}, nil)
				return
			}
			opts := Options{
				Version:          version,
				Provider:         flagProvider,
				AuthEnv:          flagAuthEnv,
				ConfigPath:       flagConfig,
				InstallSkill:     flagInstallSkill,
				SkillDir:         flagSkillDir,
				SkillSource:      flagSkillSource,
				ForceSkill:       flagForceSkill,
				EnableSearchAuto: flagEnableSearchAuto,
				EnableReaderAuto: flagEnableReaderAuto,
				EnvFile:          flagEnvFile,
				SetEnv:           flagSetEnv,
				ForceEnv:         flagForceEnv,
				SkipDoctor:       flagSkipDoctor,
			}
			if err := Run(opts); err != nil {
				metrics.ObserveCommand(start, "setup", metrics.Event{}, err)
				apperrors.HandleError(err)
			}
			metrics.ObserveCommand(start, "setup", metrics.Event{}, nil)
		},
	}

	cmd.Flags().StringVar(&flagProvider, "provider", "", "Optional provider preset to configure: bigmodel")
	cmd.Flags().StringVar(&flagAuthEnv, "auth-env", "ZHIPU_APIKEY", "Environment variable name that stores the provider token")
	cmd.Flags().StringVar(&flagConfig, "config", "", "Config file path (default: ~/.config/web-tools/config.json)")
	cmd.Flags().BoolVar(&flagInstallSkill, "install-skill", true, "Install or update the Agent skill")
	cmd.Flags().StringVar(&flagSkillDir, "skill-dir", "~/.codex/skills", "Skill root directory")
	cmd.Flags().StringVar(&flagSkillSource, "skill-source", "", "Local SKILL.md path or HTTP(S) URL")
	cmd.Flags().BoolVar(&flagForceSkill, "force-skill", true, "Overwrite existing web-tools skill during setup")
	cmd.Flags().BoolVar(&flagEnableSearchAuto, "enable-search-auto", false, "Add provider to search.default_provider_chain")
	cmd.Flags().BoolVar(&flagEnableReaderAuto, "enable-reader-auto", false, "Add provider to reader.default_provider_chain")
	cmd.Flags().StringVar(&flagEnvFile, "env-file", config.EnvFilePath(), "Env file path for --set-env")
	cmd.Flags().StringVar(&flagSetEnv, "set-env", "", "Write KEY=value to env file without printing the value")
	cmd.Flags().BoolVar(&flagForceEnv, "force-env", false, "Overwrite an existing key in env file")
	cmd.Flags().BoolVar(&flagSkipDoctor, "skip-doctor", false, "Skip final doctor check")
	cmd.Flags().BoolVar(&flagCheck, "check", false, "Check setup readiness without modifying files")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "JSON structured output for --check")
	return cmd
}

// Options configures setup execution.
type Options struct {
	Version          string
	Provider         string
	AuthEnv          string
	ConfigPath       string
	InstallSkill     bool
	SkillDir         string
	SkillSource      string
	ForceSkill       bool
	EnableSearchAuto bool
	EnableReaderAuto bool
	EnvFile          string
	SetEnv           string
	ForceEnv         bool
	SkipDoctor       bool
}

// Run executes setup.
func Run(opts Options) error {
	fmt.Fprintln(os.Stdout, "web-tools setup")

	if opts.InstallSkill {
		fmt.Fprintln(os.Stdout, "\n[1/3] Installing Agent skill")
		result, err := skillinstall.Install(context.Background(), skillinstall.Options{
			Version: opts.Version,
			Dir:     opts.SkillDir,
			Source:  opts.SkillSource,
			Force:   opts.ForceSkill,
		})
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Installed web-tools skill to %s\n", result.SkillPath)
		fmt.Fprintf(os.Stdout, "Source: %s\n", result.Source)
	} else {
		fmt.Fprintln(os.Stdout, "\n[1/3] Skipping Agent skill install")
	}

	if opts.Provider != "" {
		fmt.Fprintf(os.Stdout, "\n[2/3] Configuring provider %q\n", opts.Provider)
		if err := configcmd.AddProvider(opts.ConfigPath, opts.Provider, opts.Provider, opts.AuthEnv, opts.EnableSearchAuto, opts.EnableReaderAuto, false); err != nil {
			return err
		}
	} else {
		fmt.Fprintln(os.Stdout, "\n[2/3] No provider requested")
		fmt.Fprintln(os.Stdout, "To add BigModel later: web-tools config provider add bigmodel --preset bigmodel --auth-env ZHIPU_APIKEY")
	}

	if opts.SetEnv != "" {
		key, value, err := parseSetEnv(opts.SetEnv)
		if err != nil {
			return err
		}
		if opts.EnvFile == "" {
			opts.EnvFile = config.EnvFilePath()
		}
		if err := config.WriteEnvValue(opts.EnvFile, key, value, opts.ForceEnv); err != nil {
			return apperrors.NewInputError(
				"cannot write env file",
				err.Error(),
				[]string{"check --env-file permissions", "rerun with --force-env to overwrite an existing key"},
			)
		}
		fmt.Fprintf(os.Stdout, "Stored %s in %s\n", key, config.ExpandHome(opts.EnvFile))
	}

	if opts.SkipDoctor {
		fmt.Fprintln(os.Stdout, "\n[3/3] Skipping doctor")
		return nil
	}

	fmt.Fprintln(os.Stdout, "\n[3/3] Running doctor")
	report := doctor.DefaultChecker().Run()
	fmt.Print(report.RenderText())
	if !report.OK {
		return apperrors.NewInputError(
			"setup completed with doctor errors",
			"doctor reported one or more hard errors",
			[]string{"run web-tools doctor --json for details"},
		)
	}
	return nil
}

func parseSetEnv(raw string) (string, string, error) {
	key, value, ok := strings.Cut(raw, "=")
	if !ok || key == "" {
		return "", "", apperrors.NewInputError(
			"invalid --set-env",
			"--set-env must use KEY=value format",
			[]string{"run: web-tools setup --provider bigmodel --set-env ZHIPU_APIKEY=<token>"},
		)
	}
	return key, value, nil
}
