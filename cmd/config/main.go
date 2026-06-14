package configcmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/koda-claw/web-tools/internal/config"
	apperrors "github.com/koda-claw/web-tools/internal/errors"
	"github.com/spf13/cobra"
)

// Cmd returns the config command tree.
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage web-tools configuration",
	}
	cmd.AddCommand(pathCmd())
	cmd.AddCommand(providerCmd())
	return cmd
}

func pathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the default user config path",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(config.UserConfigPath())
		},
	}
}

func providerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provider",
		Short: "Manage configured providers",
	}
	cmd.AddCommand(providerAddCmd())
	cmd.AddCommand(providerListCmd())
	return cmd
}

func providerAddCmd() *cobra.Command {
	var (
		flagConfig           string
		flagPreset           string
		flagAuthEnv          string
		flagEnableSearchAuto bool
		flagEnableReaderAuto bool
		flagJSON             bool
	)

	cmd := &cobra.Command{
		Use:   "add <id>",
		Short: "Add or update a provider config",
		Example: `  web-tools config provider add bigmodel --preset bigmodel --auth-env ZHIPU_APIKEY
  web-tools config provider add bigmodel --preset bigmodel --enable-search-auto`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id := args[0]
			if flagPreset == "" {
				flagPreset = id
			}
			if err := addProvider(flagConfig, id, flagPreset, flagAuthEnv, flagEnableSearchAuto, flagEnableReaderAuto, flagJSON); err != nil {
				apperrors.HandleError(err)
			}
		},
	}

	cmd.Flags().StringVar(&flagConfig, "config", "", "Config file path (default: ~/.config/web-tools/config.json)")
	cmd.Flags().StringVar(&flagPreset, "preset", "", "Provider preset: bigmodel")
	cmd.Flags().StringVar(&flagAuthEnv, "auth-env", "ZHIPU_APIKEY", "Environment variable name that stores the provider token")
	cmd.Flags().BoolVar(&flagEnableSearchAuto, "enable-search-auto", false, "Add provider to search.default_provider_chain")
	cmd.Flags().BoolVar(&flagEnableReaderAuto, "enable-reader-auto", false, "Add provider to reader.default_provider_chain")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "JSON structured output")
	return cmd
}

func addProvider(path string, id string, preset string, authEnv string, enableSearchAuto bool, enableReaderAuto bool, jsonOut bool) error {
	if path == "" {
		path = config.UserConfigPath()
	} else {
		path = config.ExpandHome(path)
	}
	cfg, err := config.LoadEditableConfig(path)
	if err != nil {
		return apperrors.NewInputError(
			"cannot load config",
			err.Error(),
			[]string{"check config file JSON", "use --config to point to another file"},
		)
	}

	switch preset {
	case "bigmodel":
		if id != "bigmodel" {
			return apperrors.NewInputError(
				"preset id mismatch",
				fmt.Sprintf("preset %q must use provider id %q", preset, "bigmodel"),
				[]string{"run: web-tools config provider add bigmodel --preset bigmodel"},
			)
		}
		config.AddBigModelProvider(cfg, authEnv, enableSearchAuto, enableReaderAuto)
	default:
		return apperrors.NewInputError(
			"unknown provider preset",
			fmt.Sprintf("preset %q is not supported", preset),
			[]string{"use --preset bigmodel"},
		)
	}

	if err := config.SaveEditableConfig(path, cfg); err != nil {
		return apperrors.NewInputError(
			"cannot write config",
			err.Error(),
			[]string{"check config directory permissions", "use --config to point to another file"},
		)
	}

	if jsonOut {
		out := map[string]any{
			"ok":          true,
			"config_path": path,
			"provider":    id,
			"preset":      preset,
			"auth_env":    authEnv,
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Fprintf(os.Stdout, "Configured provider %q in %s\n", id, path)
	fmt.Fprintf(os.Stdout, "Set %s in the environment before using this provider.\n", authEnv)
	return nil
}

func providerListCmd() *cobra.Command {
	var (
		flagConfig string
		flagJSON   bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured providers",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if err := listProviders(flagConfig, flagJSON); err != nil {
				apperrors.HandleError(err)
			}
		},
	}
	cmd.Flags().StringVar(&flagConfig, "config", "", "Config file path (default: ~/.config/web-tools/config.json)")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "JSON structured output")
	return cmd
}

func listProviders(path string, jsonOut bool) error {
	if path == "" {
		path = config.UserConfigPath()
	} else {
		path = config.ExpandHome(path)
	}
	cfg, err := config.LoadEditableConfig(path)
	if err != nil {
		return apperrors.NewInputError(
			"cannot load config",
			err.Error(),
			[]string{"check config file JSON", "use --config to point to another file"},
		)
	}

	if jsonOut {
		out := map[string]any{
			"ok":          true,
			"config_path": path,
			"providers":   cfg.Providers,
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	if len(cfg.Providers) == 0 {
		fmt.Fprintf(os.Stdout, "No providers configured in %s\n", path)
		return nil
	}
	ids := make([]string, 0, len(cfg.Providers))
	for id := range cfg.Providers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		provider := cfg.Providers[id]
		fmt.Fprintf(os.Stdout, "%s\ttype=%s\tcapabilities=%v\tauth_env=%s\n", id, provider.Type, provider.Capabilities, provider.AuthEnv)
	}
	return nil
}
