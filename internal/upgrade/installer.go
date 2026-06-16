package upgrade

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/koda-claw/web-tools/internal/config"
	apperrors "github.com/koda-claw/web-tools/internal/errors"
)

const (
	BinaryModeReplaceCurrent        = "replace_current"
	BinaryModeInstallToDir          = "install_to_dir"
	BinaryModeManualReplaceRequired = "manual_replace_required"
)

type binaryPlan struct {
	Path      string
	Mode      string
	IsSymlink bool
	Writable  bool
}

func planBinary(bin string, binDir string, goos string) (binaryPlan, error) {
	if bin != "" && binDir != "" {
		return binaryPlan{}, apperrors.NewInputError("invalid binary target", "--bin and --bin-dir are mutually exclusive", []string{"choose either --bin or --bin-dir"})
	}
	current, _ := os.Executable()
	current = config.ExpandHome(current)

	var target string
	mode := BinaryModeReplaceCurrent
	if bin != "" {
		target = config.ExpandHome(bin)
	} else if binDir != "" {
		target = filepath.Join(config.ExpandHome(binDir), binaryName(goos))
		if current == "" || !samePath(target, current) {
			mode = BinaryModeInstallToDir
		}
	} else {
		if current == "" {
			return binaryPlan{}, apperrors.NewInputError("cannot locate current binary", "os.Executable returned an empty path", []string{"rerun with --bin /path/to/web-tools", "rerun with --bin-dir \"$HOME/.local/bin\""})
		}
		target = current
	}

	if bin == "" && binDir == "" {
		info, err := os.Lstat(target)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return binaryPlan{Path: target, IsSymlink: true}, apperrors.NewInputError(
				"binary path is a symlink",
				fmt.Sprintf("%s is a symlink", target),
				[]string{"rerun with --bin pointing to the real binary", "rerun with --bin-dir \"$HOME/.local/bin\""},
			)
		}
	}

	writable := isDirWritable(filepath.Dir(target))
	return binaryPlan{Path: target, Mode: mode, Writable: writable}, nil
}

func installBinary(ctx context.Context, plan binaryPlan, downloaded string, targetVersion string, goos string) (Result, error) {
	result := Result{
		BinaryPath:      plan.Path,
		BinaryMode:      plan.Mode,
		BinaryIsSymlink: plan.IsSymlink,
		BinaryWritable:  plan.Writable,
		DownloadedPath:  downloaded,
	}
	if !plan.Writable {
		return result, apperrors.NewInputError(
			"binary directory is not writable",
			filepath.Dir(plan.Path),
			[]string{"choose a writable --bin-dir such as \"$HOME/.local/bin\"", "move the binary manually"},
		)
	}

	if err := os.Chmod(downloaded, 0755); err != nil {
		return result, apperrors.NewInputError("cannot mark binary executable", err.Error(), []string{"check target directory permissions"})
	}
	if err := verifyBinaryVersion(ctx, downloaded, targetVersion); err != nil {
		return result, err
	}

	if goos == "windows" && samePath(plan.Path, currentExecutable()) {
		result.BinaryMode = BinaryModeManualReplaceRequired
		result.ManualReplaceRequired = true
		return result, nil
	}

	if err := os.MkdirAll(filepath.Dir(plan.Path), 0755); err != nil {
		return result, apperrors.NewInputError("cannot create binary directory", err.Error(), []string{"check --bin-dir permissions"})
	}
	if plan.Mode == BinaryModeInstallToDir {
		if err := replaceFile(downloaded, plan.Path); err != nil {
			return result, err
		}
		if err := verifyBinaryVersion(ctx, plan.Path, targetVersion); err != nil {
			return result, err
		}
		result.CLIUpdated = true
		return result, nil
	}

	backup := fmt.Sprintf("%s.bak.%d", plan.Path, time.Now().Unix())
	hadExisting := false
	if _, err := os.Stat(plan.Path); err == nil {
		hadExisting = true
		if err := os.Rename(plan.Path, backup); err != nil {
			return result, apperrors.NewInputError("cannot backup current binary", err.Error(), []string{"check target permissions", "rerun with --bin-dir \"$HOME/.local/bin\""})
		}
	}
	if err := os.Rename(downloaded, plan.Path); err != nil {
		if hadExisting {
			_ = os.Rename(backup, plan.Path)
		}
		return result, apperrors.NewInputError("cannot replace binary", err.Error(), []string{"original binary was restored if backup succeeded", "check target permissions"})
	}
	if err := verifyBinaryVersion(ctx, plan.Path, targetVersion); err != nil {
		if hadExisting {
			_ = os.Remove(plan.Path)
			_ = os.Rename(backup, plan.Path)
		}
		return result, err
	}
	if hadExisting {
		_ = os.Remove(backup)
	}
	result.CLIUpdated = true
	return result, nil
}

func replaceFile(src, dst string) error {
	if err := os.Rename(src, dst); err != nil {
		return apperrors.NewInputError("cannot install binary", err.Error(), []string{"check --bin-dir permissions"})
	}
	return nil
}

func verifyBinaryVersion(ctx context.Context, bin string, targetVersion string) error {
	cmd := exec.CommandContext(ctx, bin, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return apperrors.NewInputError("cannot execute downloaded binary", err.Error(), []string{"check release asset for the current platform"})
	}
	if !strings.Contains(string(out), targetVersion) {
		return apperrors.NewInputError(
			"binary version mismatch",
			fmt.Sprintf("expected %s in --version output, got %q", targetVersion, strings.TrimSpace(string(out))),
			[]string{"check the release asset", "retry after release assets are complete"},
		)
	}
	return nil
}

func isDirWritable(dir string) bool {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return false
	}
	f, err := os.CreateTemp(dir, ".web-tools-write-test-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA == nil {
		a = absA
	}
	if errB == nil {
		b = absB
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func currentExecutable() string {
	exe, _ := os.Executable()
	return exe
}
