package skillcmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	apperrors "github.com/koda-claw/web-tools/internal/errors"
	"github.com/koda-claw/web-tools/internal/skillinstall"
	"github.com/spf13/cobra"
)

// Cmd returns the skill command tree.
func Cmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Install the web-tools Agent skill",
	}
	cmd.AddCommand(installCmd(version))
	return cmd
}

func installCmd(version string) *cobra.Command {
	var (
		flagDir    string
		flagSource string
		flagForce  bool
		flagJSON   bool
	)

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the web-tools skill from this CLI",
		Example: `  web-tools skill install
  web-tools skill install --dir ~/.agents/skills
  web-tools skill install --source ./skills/web-tools/SKILL.md`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if err := InstallSkill(version, flagDir, flagSource, flagForce, flagJSON); err != nil {
				apperrors.HandleError(err)
			}
		},
	}

	cmd.Flags().StringVar(&flagDir, "dir", "~/.codex/skills", "Skill root directory")
	cmd.Flags().StringVar(&flagSource, "source", "", "Local SKILL.md path or HTTP(S) URL (default: matching GitHub release)")
	cmd.Flags().BoolVar(&flagForce, "force", false, "Overwrite existing web-tools skill")
	cmd.Flags().BoolVar(&flagJSON, "json", false, "JSON structured output")
	return cmd
}

func InstallSkill(version string, dir string, source string, force bool, jsonOut bool) error {
	result, err := skillinstall.Install(context.Background(), skillinstall.Options{
		Version: version,
		Dir:     dir,
		Source:  source,
		Force:   force,
	})
	if err != nil {
		return err
	}

	if jsonOut {
		data, _ := json.MarshalIndent(struct {
			OK bool `json:"ok"`
			skillinstall.Result
		}{OK: true, Result: result}, "", "  ")
		fmt.Fprintln(os.Stdout, string(data))
		return nil
	}
	fmt.Fprintf(os.Stdout, "Installed web-tools skill to %s\n", result.SkillPath)
	fmt.Fprintf(os.Stdout, "Source: %s\n", result.Source)
	return nil
}
