package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const explicitEnvFileVar = "WEB_TOOLS_ENV"

// EnvFileStatus describes non-sensitive env file loading state.
type EnvFileStatus struct {
	Path           string
	Exists         bool
	Loaded         bool
	Mode           os.FileMode
	OverPermissive bool
	Error          string
}

// EnvFilesReport describes user and explicit env files without exposing values.
type EnvFilesReport struct {
	User     EnvFileStatus
	Explicit EnvFileStatus
}

// Err returns the first env file loading error, if any.
func (r EnvFilesReport) Err() error {
	if r.User.Error != "" {
		return fmt.Errorf("user env file %s: %s", r.User.Path, r.User.Error)
	}
	if r.Explicit.Error != "" {
		return fmt.Errorf("explicit env file %s: %s", r.Explicit.Path, r.Explicit.Error)
	}
	return nil
}

// EnvFilePath returns the default user env file path.
func EnvFilePath() string {
	return expandHome("~/.config/web-tools/.env")
}

// ExplicitEnvFilePath returns WEB_TOOLS_ENV expanded for user-facing paths.
func ExplicitEnvFilePath() string {
	if v := os.Getenv(explicitEnvFileVar); v != "" {
		return expandHome(v)
	}
	return ""
}

// LoadEnvFiles loads configured env files into the current process.
// Existing process env values win over values from env files.
func LoadEnvFiles() EnvFilesReport {
	return loadEnvFilesWithBase(snapshotEnvKeys())
}

func loadEnvFilesWithBase(base map[string]bool) EnvFilesReport {
	userPath := EnvFilePath()
	explicitPath := ExplicitEnvFilePath()
	report := EnvFilesReport{
		User:     inspectEnvFile(userPath),
		Explicit: EnvFileStatus{Path: explicitPath},
	}
	if explicitPath != "" {
		report.Explicit = inspectEnvFile(explicitPath)
	}

	userValues := map[string]string{}
	if report.User.Exists {
		values, err := ParseEnvFile(userPath)
		if err != nil {
			report.User.Error = err.Error()
		} else {
			report.User.Loaded = true
			userValues = values
			applyEnvValues(values, base)
		}
	}

	if explicitPath != "" && report.Explicit.Exists {
		values, err := ParseEnvFile(explicitPath)
		if err != nil {
			report.Explicit.Error = err.Error()
		} else {
			report.Explicit.Loaded = true
			applyEnvValues(values, base)
			for key, value := range values {
				if !base[key] {
					userValues[key] = value
				}
			}
		}
	}

	for key, value := range userValues {
		if !base[key] {
			_ = os.Setenv(key, value)
		}
	}
	return report
}

func snapshotEnvKeys() map[string]bool {
	keys := map[string]bool{}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		keys[key] = true
	}
	return keys
}

func applyEnvValues(values map[string]string, base map[string]bool) {
	for key, value := range values {
		if base[key] {
			continue
		}
		_ = os.Setenv(key, value)
	}
}

func inspectEnvFile(path string) EnvFileStatus {
	status := EnvFileStatus{Path: path}
	if path == "" {
		return status
	}
	info, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			status.Error = err.Error()
		}
		return status
	}
	status.Exists = true
	status.Mode = info.Mode().Perm()
	status.OverPermissive = status.Mode&0077 != 0
	return status
}

// ParseEnvFile parses a simple .env file without shell evaluation.
func ParseEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d invalid env line: missing '='", path, lineNo)
		}
		key = strings.TrimSpace(key)
		if !validEnvKey(key) {
			return nil, fmt.Errorf("%s:%d invalid env key %q", path, lineNo, key)
		}
		value = strings.TrimSpace(value)
		unquoted, err := unquoteEnvValue(value)
		if err != nil {
			return nil, fmt.Errorf("%s:%d %w", path, lineNo, err)
		}
		values[key] = unquoted
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func validEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		if r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || i > 0 && r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}

func unquoteEnvValue(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if value[0] != '"' && value[0] != '\'' {
		return value, nil
	}
	if len(value) < 2 || value[len(value)-1] != value[0] {
		return "", fmt.Errorf("unterminated quoted env value")
	}
	unquoted := value[1 : len(value)-1]
	if value[0] == '"' {
		unquoted = strings.ReplaceAll(unquoted, `\"`, `"`)
	}
	return unquoted, nil
}

// WriteEnvValue writes or updates one key in an env file.
func WriteEnvValue(path string, key string, value string, force bool) error {
	path = expandHome(path)
	if !validEnvKey(key) {
		return fmt.Errorf("invalid env key %q", key)
	}
	values := map[string]string{}
	if _, err := os.Stat(path); err == nil {
		var parseErr error
		values, parseErr = ParseEnvFile(path)
		if parseErr != nil {
			return parseErr
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	if _, exists := values[key]; exists && !force {
		return fmt.Errorf("%s already exists in %s", key, path)
	}
	values[key] = value
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, renderEnvFile(values), 0600)
}

func renderEnvFile(values map[string]string) []byte {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for _, key := range keys {
		sb.WriteString(key)
		sb.WriteString("=")
		sb.WriteString(quoteEnvValue(values[key]))
		sb.WriteString("\n")
	}
	return []byte(sb.String())
}

func quoteEnvValue(value string) string {
	if value == "" || strings.ContainsAny(value, " \t#'\"") {
		return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
	}
	return value
}
