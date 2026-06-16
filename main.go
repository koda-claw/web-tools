package main

import (
	"fmt"
	"os"

	configcmd "github.com/koda-claw/web-tools/cmd/config"
	"github.com/koda-claw/web-tools/cmd/doctor"
	guicmd "github.com/koda-claw/web-tools/cmd/gui"
	metricscmd "github.com/koda-claw/web-tools/cmd/metrics"
	setupcmd "github.com/koda-claw/web-tools/cmd/setup"
	skillcmd "github.com/koda-claw/web-tools/cmd/skill"
	upgradecmd "github.com/koda-claw/web-tools/cmd/upgrade"
	"github.com/koda-claw/web-tools/cmd/web-reader"
	"github.com/koda-claw/web-tools/cmd/web-search"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	rootCmd := &cobra.Command{
		Use:   "web-tools",
		Short: "Local-first web tools for AI agents",
		Long: `web-tools provides web-search and web-reader as CLI tools, designed for AI agents to consume.

Zero cost. No API keys. No third-party dependencies.`,
		Example: `  web-tools web-search "latest AI news" --limit 3
  web-tools web-search "AI latest developments" --locale en-US --time-range week
  web-tools web-search "site:reuters.com Iran" --category news
  web-tools web-reader https://example.com/article
  web-tools web-reader https://example.com/spa-page --browser
  web-tools web-reader ./report.pdf
  web-tools web-reader ./slides.pptx -o /tmp/slides.md
	  web-tools config provider add bigmodel --preset bigmodel
	  web-tools skill install
	  web-tools setup --provider bigmodel
	  web-tools upgrade --check --json
	  web-tools metrics --json
	  web-tools doctor --json`,
		Version: version,
	}

	rootCmd.AddCommand(configcmd.Cmd())
	rootCmd.AddCommand(doctor.Cmd())
	rootCmd.AddCommand(guicmd.Cmd(version))
	rootCmd.AddCommand(metricscmd.Cmd())
	rootCmd.AddCommand(setupcmd.Cmd(version))
	rootCmd.AddCommand(skillcmd.Cmd(version))
	rootCmd.AddCommand(upgradecmd.Cmd(version))
	rootCmd.AddCommand(webreader.Cmd())
	rootCmd.AddCommand(websearch.Cmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
