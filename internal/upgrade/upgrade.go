package upgrade

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	apperrors "github.com/koda-claw/web-tools/internal/errors"
	"github.com/koda-claw/web-tools/internal/skillinstall"
)

// Runner executes upgrades.
type Runner struct {
	Client   HTTPClient
	Platform Platform
}

// Run executes an upgrade or check.
func (r Runner) Run(ctx context.Context, opts Options) (Result, error) {
	if opts.Repo == "" {
		opts.Repo = defaultRepo
	}
	if opts.TargetVersion == "" {
		opts.TargetVersion = "latest"
	}
	if opts.SkipSkill && opts.OnlySkill {
		return Result{}, apperrors.NewInputError("invalid upgrade options", "--skip-skill and --only-skill are mutually exclusive", []string{"choose one skill mode"})
	}
	if opts.Bin != "" && opts.BinDir != "" {
		return Result{}, apperrors.NewInputError("invalid binary target", "--bin and --bin-dir are mutually exclusive", []string{"choose either --bin or --bin-dir"})
	}
	if opts.BaseURL != "" && opts.TargetVersion == "latest" {
		return Result{}, apperrors.NewInputError("invalid --base-url usage", "--base-url requires explicit --version vX.Y.Z", []string{"rerun with --version vX.Y.Z", "omit --base-url to use GitHub latest"})
	}

	platform := r.Platform
	if platform.GOOS == "" {
		platform = currentPlatform()
	}
	client := newReleaseClient(r.Client)

	targetVersion := opts.TargetVersion
	if targetVersion == "latest" {
		tag, err := client.resolveLatest(ctx, opts.Repo)
		if err != nil {
			return Result{}, err
		}
		targetVersion = tag
	}

	asset := ""
	if !opts.OnlySkill {
		var err error
		asset, err = AssetName(platform)
		if err != nil {
			return Result{}, err
		}
	}

	result := Result{
		OK:             true,
		CurrentVersion: opts.CurrentVersion,
		TargetVersion:  targetVersion,
		Asset:          asset,
	}

	if !opts.OnlySkill {
		plan, err := planBinary(opts.Bin, opts.BinDir, platform.GOOS)
		if err != nil {
			return result, err
		}
		result.BinaryPath = plan.Path
		result.BinaryMode = plan.Mode
		result.BinaryIsSymlink = plan.IsSymlink
		result.BinaryWritable = plan.Writable
	}

	if opts.Check {
		if !opts.OnlySkill && !opts.InsecureSkipChecksum {
			checkURL := checksumURL(opts.Repo, opts.BaseURL, targetVersion)
			manifest, err := client.client.GetBytes(ctx, checkURL)
			if err != nil {
				return result, apperrors.NewNetworkError(
					"cannot download checksum manifest",
					err.Error(),
					map[string]string{"url": checkURL},
					[]string{"choose a release with checksums.txt", "retry after release assets are complete"},
				)
			}
			if ParseChecksums(string(manifest))[asset] == "" {
				return result, apperrors.NewInputError(
					"checksum missing",
					"checksums.txt does not contain "+asset,
					[]string{"check target version and platform", "retry after release assets are complete"},
				)
			}
			result.ChecksumVerified = true
		}
		return result, nil
	}

	if !opts.OnlySkill && !opts.Force && normalizeVersion(opts.CurrentVersion) == normalizeVersion(targetVersion) {
		result.ChecksumVerified = false
	} else if !opts.OnlySkill {
		binaryResult, err := r.upgradeBinary(ctx, opts, targetVersion, asset, platform)
		result.BinaryPath = binaryResult.BinaryPath
		result.BinaryMode = binaryResult.BinaryMode
		result.BinaryIsSymlink = binaryResult.BinaryIsSymlink
		result.BinaryWritable = binaryResult.BinaryWritable
		result.DownloadedPath = binaryResult.DownloadedPath
		result.ManualReplaceRequired = binaryResult.ManualReplaceRequired
		result.CLIUpdated = binaryResult.CLIUpdated
		result.ChecksumVerified = binaryResult.ChecksumVerified
		if err != nil {
			return result, err
		}
	}

	if !opts.SkipSkill {
		skillResult, err := skillinstall.Install(ctx, skillinstall.Options{
			Version: targetVersion,
			Dir:     opts.SkillDir,
			Source:  opts.SkillSource,
			Force:   true,
		})
		if err != nil {
			return result, err
		}
		result.SkillPath = skillResult.SkillPath
		result.SkillSource = skillResult.Source
		result.SkillUpdated = true
	}

	return result, nil
}

func (r Runner) upgradeBinary(ctx context.Context, opts Options, targetVersion string, asset string, platform Platform) (Result, error) {
	plan, err := planBinary(opts.Bin, opts.BinDir, platform.GOOS)
	if err != nil {
		return Result{}, err
	}
	dir := filepath.Dir(plan.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return Result{BinaryPath: plan.Path, BinaryMode: plan.Mode}, apperrors.NewInputError("cannot create binary directory", err.Error(), []string{"check --bin-dir permissions"})
	}
	tmp, err := os.CreateTemp(dir, ".web-tools.tmp-*")
	if err != nil {
		return Result{BinaryPath: plan.Path, BinaryMode: plan.Mode}, apperrors.NewInputError("cannot create temporary binary", err.Error(), []string{"check target directory permissions"})
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	_ = os.Remove(tmpPath)

	client := newReleaseClient(r.Client)
	assetURL := releaseAssetURL(opts.Repo, opts.BaseURL, targetVersion, asset)
	if err := client.client.DownloadFile(ctx, assetURL, tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		return Result{BinaryPath: plan.Path, BinaryMode: plan.Mode, DownloadedPath: tmpPath}, apperrors.NewNetworkError(
			"cannot download release asset",
			err.Error(),
			map[string]string{"url": assetURL},
			[]string{"check network access", "check --base-url", "check target version and platform"},
		)
	}

	checksumVerified := false
	if opts.InsecureSkipChecksum {
		checksumVerified = false
	} else {
		checkURL := checksumURL(opts.Repo, opts.BaseURL, targetVersion)
		manifest, err := client.client.GetBytes(ctx, checkURL)
		if err != nil {
			_ = os.Remove(tmpPath)
			return Result{BinaryPath: plan.Path, BinaryMode: plan.Mode, DownloadedPath: tmpPath}, apperrors.NewNetworkError(
				"cannot download checksum manifest",
				err.Error(),
				map[string]string{"url": checkURL},
				[]string{"choose a release with checksums.txt", "retry after release assets are complete"},
			)
		}
		if err := verifyChecksum(tmpPath, string(manifest), asset); err != nil {
			_ = os.Remove(tmpPath)
			return Result{BinaryPath: plan.Path, BinaryMode: plan.Mode, DownloadedPath: tmpPath}, err
		}
		checksumVerified = true
	}

	result, err := installBinary(ctx, plan, tmpPath, targetVersion, platform.GOOS)
	result.ChecksumVerified = checksumVerified
	if err != nil {
		_ = os.Remove(tmpPath)
		return result, err
	}
	if result.ManualReplaceRequired {
		result.ChecksumVerified = checksumVerified
		return result, nil
	}
	return result, nil
}

func normalizeVersion(v string) string {
	return strings.TrimSpace(v)
}
