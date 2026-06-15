package gui

import (
	"fmt"

	"github.com/koda-claw/web-tools/internal/setupcheck"
)

const repositoryURL = "https://github.com/koda-claw/web-tools"

type AgentGuide struct {
	RepositoryURL   string                  `json:"repository_url"`
	InstallCLI      []string                `json:"install_cli"`
	InstallSkill    []string                `json:"install_skill"`
	CheckCommands   []string                `json:"check_commands"`
	UsageExamples   []string                `json:"usage_examples"`
	RepairCommands  []setupcheck.Suggestion `json:"repair_commands,omitempty"`
	ReaderAutoNote  string                  `json:"reader_auto_note,omitempty"`
	RecommendedMode string                  `json:"recommended_mode"`
}

func buildAgentGuide(version string, status setupcheck.Report) AgentGuide {
	ref := version
	if ref == "" || ref == "dev" {
		ref = "main"
	}
	guide := AgentGuide{
		RepositoryURL: repositoryURL,
		InstallCLI: []string{
			fmt.Sprintf("go install github.com/koda-claw/web-tools@%s", ref),
			fmt.Sprintf("curl -L https://github.com/koda-claw/web-tools/releases/download/%s/web-tools_Darwin_arm64.tar.gz", versionOrPlaceholder(version)),
		},
		InstallSkill: []string{
			"web-tools skill install --force",
			"web-tools setup --check --json",
		},
		CheckCommands: []string{
			"web-tools doctor --json",
			"web-tools setup --check --json",
		},
		UsageExamples: []string{
			`web-tools web-search "Go readability library" --provider auto --json`,
			`web-tools web-reader https://example.com/article --provider auto --json`,
			`web-tools web-search "Go readability library" --provider bigmodel --json`,
		},
		RepairCommands:  append([]setupcheck.Suggestion(nil), status.Suggestions...),
		RecommendedMode: "Agents should use non-interactive CLI commands. Human setup belongs in the local GUI.",
	}
	if status.Provider.Configured && status.Provider.AuthConfigured && !status.ReaderAuto.Contains {
		guide.ReaderAutoNote = "BigModel is configured and authenticated. Enable reader auto only after explicit privacy and cost confirmation."
	}
	return guide
}

func versionOrPlaceholder(version string) string {
	if version == "" || version == "dev" {
		return "v1.5.0"
	}
	return version
}
