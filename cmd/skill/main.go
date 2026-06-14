package skillcmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/koda-claw/web-tools/internal/config"
	apperrors "github.com/koda-claw/web-tools/internal/errors"
	"github.com/spf13/cobra"
)

const defaultSkillURL = "https://raw.githubusercontent.com/koda-claw/web-tools/%s/skills/web-tools/SKILL.md"

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
			if err := installSkill(version, flagDir, flagSource, flagForce, flagJSON); err != nil {
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

func installSkill(version string, dir string, source string, force bool, jsonOut bool) error {
	if dir == "" {
		dir = "~/.codex/skills"
	}
	root := config.ExpandHome(dir)
	targetDir := filepath.Join(root, "web-tools")
	targetPath := filepath.Join(targetDir, "SKILL.md")
	if _, err := os.Stat(targetPath); err == nil && !force {
		return apperrors.NewInputError(
			"skill already installed",
			fmt.Sprintf("%s already exists", targetPath),
			[]string{"rerun with --force to overwrite", "use --dir to install elsewhere"},
		)
	}

	content, resolvedSource, err := loadSkillContent(version, source)
	if err != nil {
		return apperrors.NewInputError(
			"cannot load skill",
			err.Error(),
			[]string{"check network access", "use --source ./skills/web-tools/SKILL.md from a source checkout"},
		)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return apperrors.NewInputError("cannot create skill directory", err.Error(), []string{"check --dir permissions"})
	}
	if err := os.WriteFile(targetPath, content, 0644); err != nil {
		return apperrors.NewInputError("cannot write skill", err.Error(), []string{"check --dir permissions"})
	}

	if jsonOut {
		fmt.Fprintf(os.Stdout, "{\n  \"ok\": true,\n  \"skill_path\": %q,\n  \"source\": %q\n}\n", targetPath, resolvedSource)
		return nil
	}
	fmt.Fprintf(os.Stdout, "Installed web-tools skill to %s\n", targetPath)
	fmt.Fprintf(os.Stdout, "Source: %s\n", resolvedSource)
	return nil
}

func loadSkillContent(version string, source string) ([]byte, string, error) {
	if source == "" {
		ref := "main"
		if regexp.MustCompile(`^v[0-9]+[.][0-9]+[.][0-9]+`).MatchString(version) {
			ref = version
		}
		source = fmt.Sprintf(defaultSkillURL, ref)
	}
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		data, err := fetchSkill(source)
		return data, source, err
	}
	path := config.ExpandHome(source)
	data, err := os.ReadFile(path)
	return data, path, err
}

func fetchSkill(rawURL string) ([]byte, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s returned HTTP %d", rawURL, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil, err
	}
	if !strings.Contains(string(data), "name: web-tools") {
		return nil, fmt.Errorf("downloaded file does not look like web-tools SKILL.md")
	}
	return data, nil
}
