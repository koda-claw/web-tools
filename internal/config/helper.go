package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

// FindExecutable checks if a command exists in PATH.
func FindExecutable(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return path
}

// ExpandHome expands a leading ~/ in user-facing paths.
func ExpandHome(path string) string {
	return expandHome(path)
}

// UserConfigPath returns the default user config file path.
func UserConfigPath() string {
	return expandHome("~/.config/web-tools/config.json")
}
