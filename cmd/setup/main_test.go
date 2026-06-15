package setupcmd

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/koda-claw/web-tools/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunSetupInstallsSkillAndConfiguresProvider(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "SKILL.md")
	require.NoError(t, os.WriteFile(source, []byte("---\nname: web-tools\ndescription: test\n---\n"), 0644))

	cfgPath := filepath.Join(dir, "config.json")
	skillDir := filepath.Join(dir, "skills")

	require.NoError(t, Run(Options{
		Version:          "dev",
		Provider:         "bigmodel",
		AuthEnv:          "ZHIPU_APIKEY",
		ConfigPath:       cfgPath,
		InstallSkill:     true,
		SkillDir:         skillDir,
		SkillSource:      source,
		ForceSkill:       true,
		EnableSearchAuto: true,
		SkipDoctor:       true,
	}))

	cfgData, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	assert.Contains(t, string(cfgData), `"bigmodel"`)
	assert.Contains(t, string(cfgData), `"auth_env": "ZHIPU_APIKEY"`)

	skillData, err := os.ReadFile(filepath.Join(skillDir, "web-tools", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(skillData), "name: web-tools")
}

func TestRunSetupWritesEnvFileWithoutLeakingValue(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	output := captureStdout(t, func() {
		require.NoError(t, Run(Options{
			InstallSkill: false,
			EnvFile:      envPath,
			SetEnv:       "ZHIPU_APIKEY=super-secret-token",
			SkipDoctor:   true,
		}))
	})

	values, err := config.ParseEnvFile(envPath)
	require.NoError(t, err)
	assert.Equal(t, "super-secret-token", values["ZHIPU_APIKEY"])
	assert.Contains(t, output, "Stored ZHIPU_APIKEY")
	assert.NotContains(t, output, "super-secret-token")
}

func TestRunSetupEnvFileOverwriteRequiresForce(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	require.NoError(t, config.WriteEnvValue(envPath, "ZHIPU_APIKEY", "old", false))

	err := Run(Options{
		InstallSkill: false,
		EnvFile:      envPath,
		SetEnv:       "ZHIPU_APIKEY=new",
		SkipDoctor:   true,
	})
	require.Error(t, err)

	require.NoError(t, Run(Options{
		InstallSkill: false,
		EnvFile:      envPath,
		SetEnv:       "ZHIPU_APIKEY=new",
		ForceEnv:     true,
		SkipDoctor:   true,
	}))
	values, err := config.ParseEnvFile(envPath)
	require.NoError(t, err)
	assert.Equal(t, "new", values["ZHIPU_APIKEY"])
}

func TestCmdSetupCheckJSON(t *testing.T) {
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	origConfig := os.Getenv("WEB_TOOLS_CONFIG")
	origAuth := os.Getenv("ZHIPU_APIKEY")
	t.Setenv("HOME", dir)
	t.Setenv("WEB_TOOLS_CONFIG", "")
	require.NoError(t, os.Unsetenv("ZHIPU_APIKEY"))
	t.Cleanup(func() {
		_ = os.Setenv("HOME", origHome)
		_ = os.Setenv("WEB_TOOLS_CONFIG", origConfig)
		if origAuth == "" {
			_ = os.Unsetenv("ZHIPU_APIKEY")
		} else {
			_ = os.Setenv("ZHIPU_APIKEY", origAuth)
		}
	})

	cmd := Cmd("test")
	cmd.SetArgs([]string{"--check", "--json", "--skill-dir", filepath.Join(dir, "skills")})
	output := captureStdout(t, func() {
		require.NoError(t, cmd.Execute())
	})

	assert.Contains(t, output, `"ok": true`)
	assert.Contains(t, output, `"install_skill"`)
	assert.Contains(t, output, `"configure_provider"`)
	assert.NotContains(t, output, "super-secret-token")
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	read, write, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = write
	defer func() { os.Stdout = orig }()

	fn()
	require.NoError(t, write.Close())
	data, err := io.ReadAll(read)
	require.NoError(t, err)
	return string(data)
}
