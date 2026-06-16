package upgrade

import "runtime"

const defaultRepo = "koda-claw/web-tools"

// Options configures an upgrade run.
type Options struct {
	CurrentVersion       string
	TargetVersion        string
	Repo                 string
	BaseURL              string
	Bin                  string
	BinDir               string
	SkillDir             string
	SkillSource          string
	SkipSkill            bool
	OnlySkill            bool
	Check                bool
	Force                bool
	InsecureSkipChecksum bool
}

// Result is the structured upgrade outcome.
type Result struct {
	OK                    bool   `json:"ok"`
	CurrentVersion        string `json:"current_version"`
	TargetVersion         string `json:"target_version"`
	Asset                 string `json:"asset,omitempty"`
	ChecksumVerified      bool   `json:"checksum_verified"`
	BinaryPath            string `json:"binary_path,omitempty"`
	BinaryMode            string `json:"binary_mode,omitempty"`
	BinaryIsSymlink       bool   `json:"binary_is_symlink,omitempty"`
	BinaryWritable        bool   `json:"binary_writable,omitempty"`
	DownloadedPath        string `json:"downloaded_path,omitempty"`
	ManualReplaceRequired bool   `json:"manual_replace_required,omitempty"`
	CLIUpdated            bool   `json:"cli_updated"`
	SkillPath             string `json:"skill_path,omitempty"`
	SkillSource           string `json:"skill_source,omitempty"`
	SkillUpdated          bool   `json:"skill_updated"`
}

// Platform identifies a build target.
type Platform struct {
	GOOS   string
	GOARCH string
}

func currentPlatform() Platform {
	return Platform{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
}
