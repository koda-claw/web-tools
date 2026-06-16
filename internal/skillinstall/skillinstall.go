package skillinstall

import (
	"context"
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
)

const defaultSkillURL = "https://raw.githubusercontent.com/koda-claw/web-tools/%s/skills/web-tools/SKILL.md"

var releaseVersionPattern = regexp.MustCompile(`^v[0-9]+[.][0-9]+[.][0-9]+`)

// Options configures skill installation.
type Options struct {
	Version string
	Dir     string
	Source  string
	Force   bool
	Client  *http.Client
}

// Result describes an installed skill.
type Result struct {
	SkillPath string `json:"skill_path"`
	Source    string `json:"source"`
}

// Install writes the web-tools skill into the target skill root.
func Install(ctx context.Context, opts Options) (Result, error) {
	if opts.Dir == "" {
		opts.Dir = "~/.codex/skills"
	}
	root := config.ExpandHome(opts.Dir)
	targetDir := filepath.Join(root, "web-tools")
	targetPath := filepath.Join(targetDir, "SKILL.md")
	if _, err := os.Stat(targetPath); err == nil && !opts.Force {
		return Result{}, apperrors.NewInputError(
			"skill already installed",
			fmt.Sprintf("%s already exists", targetPath),
			[]string{"rerun with --force to overwrite", "use --dir to install elsewhere"},
		)
	}

	content, resolvedSource, err := loadSkillContent(ctx, opts)
	if err != nil {
		return Result{}, apperrors.NewInputError(
			"cannot load skill",
			err.Error(),
			[]string{"check network access", "use --source ./skills/web-tools/SKILL.md from a source checkout"},
		)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return Result{}, apperrors.NewInputError("cannot create skill directory", err.Error(), []string{"check --dir permissions"})
	}
	if err := os.WriteFile(targetPath, content, 0644); err != nil {
		return Result{}, apperrors.NewInputError("cannot write skill", err.Error(), []string{"check --dir permissions"})
	}

	return Result{SkillPath: targetPath, Source: resolvedSource}, nil
}

// DefaultSource returns the SKILL.md source for a CLI version.
func DefaultSource(version string) string {
	ref := "main"
	if releaseVersionPattern.MatchString(version) {
		ref = version
	}
	return fmt.Sprintf(defaultSkillURL, ref)
}

func loadSkillContent(ctx context.Context, opts Options) ([]byte, string, error) {
	source := opts.Source
	if source == "" {
		source = DefaultSource(opts.Version)
	}
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		data, err := fetchSkill(ctx, opts.Client, source)
		return data, source, err
	}
	path := config.ExpandHome(source)
	data, err := os.ReadFile(path)
	return data, path, err
}

func fetchSkill(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
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
