package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func unsetEnvForTest(t *testing.T, key string) {
	t.Helper()
	old, existed := os.LookupEnv(key)
	require.NoError(t, os.Unsetenv(key))
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(key, old)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

func TestParseEnvFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	require.NoError(t, os.WriteFile(path, []byte(`
# comment
ZHIPU_APIKEY=abc123
QUOTED="hello world"
SINGLE='hello'
EMPTY=
`), 0600))

	values, err := ParseEnvFile(path)

	require.NoError(t, err)
	assert.Equal(t, "abc123", values["ZHIPU_APIKEY"])
	assert.Equal(t, "hello world", values["QUOTED"])
	assert.Equal(t, "hello", values["SINGLE"])
	assert.Equal(t, "", values["EMPTY"])
}

func TestParseEnvFileRejectsInvalidLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	require.NoError(t, os.WriteFile(path, []byte("INVALID\n"), 0600))

	_, err := ParseEnvFile(path)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing '='")
}

func TestWriteEnvValueCreatesFileWith0600(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")

	require.NoError(t, WriteEnvValue(path, "ZHIPU_APIKEY", "abc123", false))

	values, err := ParseEnvFile(path)
	require.NoError(t, err)
	assert.Equal(t, "abc123", values["ZHIPU_APIKEY"])
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestWriteEnvValueRefusesOverwriteUnlessForced(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	require.NoError(t, WriteEnvValue(path, "ZHIPU_APIKEY", "old", false))

	err := WriteEnvValue(path, "ZHIPU_APIKEY", "new", false)
	require.Error(t, err)

	require.NoError(t, WriteEnvValue(path, "ZHIPU_APIKEY", "new", true))
	values, err := ParseEnvFile(path)
	require.NoError(t, err)
	assert.Equal(t, "new", values["ZHIPU_APIKEY"])
}

func TestLoadEnvFilesDoesNotOverrideStartupEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("ZHIPU_APIKEY", "shell-value")
	userEnv := filepath.Join(dir, ".config", "web-tools", ".env")
	require.NoError(t, os.MkdirAll(filepath.Dir(userEnv), 0755))
	require.NoError(t, os.WriteFile(userEnv, []byte("ZHIPU_APIKEY=file-value\n"), 0600))

	report := LoadEnvFiles()

	assert.True(t, report.User.Loaded)
	assert.Equal(t, "shell-value", os.Getenv("ZHIPU_APIKEY"))
}

func TestLoadEnvFilesExplicitOverridesUserFileButNotStartupEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	unsetEnvForTest(t, "ZHIPU_APIKEY")
	unsetEnvForTest(t, "USER_ONLY")
	userEnv := filepath.Join(dir, ".config", "web-tools", ".env")
	explicitEnv := filepath.Join(dir, "custom.env")
	require.NoError(t, os.MkdirAll(filepath.Dir(userEnv), 0755))
	require.NoError(t, os.WriteFile(userEnv, []byte("ZHIPU_APIKEY=user-value\nUSER_ONLY=ok\n"), 0600))
	require.NoError(t, os.WriteFile(explicitEnv, []byte("ZHIPU_APIKEY=explicit-value\n"), 0600))
	t.Setenv(explicitEnvFileVar, explicitEnv)

	report := LoadEnvFiles()

	assert.True(t, report.User.Loaded)
	assert.True(t, report.Explicit.Loaded)
	assert.Equal(t, "explicit-value", os.Getenv("ZHIPU_APIKEY"))
	assert.Equal(t, "ok", os.Getenv("USER_ONLY"))
}

func TestInspectEnvFileReportsOverPermissiveMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	require.NoError(t, os.WriteFile(path, []byte("KEY=value\n"), 0644))

	status := inspectEnvFile(path)

	assert.True(t, status.Exists)
	assert.True(t, status.OverPermissive)
}
