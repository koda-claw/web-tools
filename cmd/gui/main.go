package guicmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/koda-claw/web-tools/internal/gui"
	"github.com/spf13/cobra"
)

// Cmd returns the local GUI command.
func Cmd(version string) *cobra.Command {
	var (
		flagHost     string
		flagPort     int
		flagNoOpen   bool
		flagSkillDir string
	)

	cmd := &cobra.Command{
		Use:   "gui",
		Short: "Start the local web-tools GUI",
		Long: `Start a local-only management console for checking setup, configuring providers,
writing the user env file, running smoke tests, exporting diagnostics, and
copying Agent handoff commands.`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			server := gui.NewServer(gui.Options{
				Version:  version,
				Host:     flagHost,
				Port:     flagPort,
				NoOpen:   flagNoOpen,
				SkillDir: flagSkillDir,
			})
			if err := server.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "failed to start GUI: %v\n", err)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stdout, "web-tools GUI: %s\n", server.URL())
			if err := server.OpenBrowser(); err != nil {
				fmt.Fprintf(os.Stderr, "could not open browser automatically: %v\n", err)
			}

			waitForShutdown(server)
		},
	}

	cmd.Flags().StringVar(&flagHost, "host", "127.0.0.1", "Host to bind; default is local-only")
	cmd.Flags().IntVar(&flagPort, "port", 0, "Port to bind; 0 chooses a free port")
	cmd.Flags().BoolVar(&flagNoOpen, "no-open", false, "Do not open the browser automatically")
	cmd.Flags().StringVar(&flagSkillDir, "skill-dir", "~/.codex/skills", "Skill root directory")
	return cmd
}

func waitForShutdown(server *gui.Server) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	<-signals
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}
