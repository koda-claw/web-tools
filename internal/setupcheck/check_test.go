package setupcheck

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/koda-claw/web-tools/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunReportsMissingSetupAndSuggestions(t *testing.T) {
	dir := tempHome(t)
	unsetEnvForTest(t, "ZHIPU_APIKEY")

	report := Run(Options{Version: "test", SkillDir: filepath.Join(dir, "skills")})

	assert.True(t, report.OK)
	assert.False(t, report.Skill.Installed)
	assert.False(t, report.Config.Exists)
	assert.False(t, report.Provider.Configured)
	assertSuggestion(t, report, "install_skill")
	assertSuggestion(t, report, "configure_provider")
	assert.NotContains(t, report.RenderText(), "secret")
}

func TestRunReportsProviderAuthWithoutLeakingSecret(t *testing.T) {
	dir := tempHome(t)
	unsetEnvForTest(t, "ZHIPU_APIKEY")
	cfg := &config.EditableConfig{}
	config.AddBigModelProvider(cfg, "ZHIPU_APIKEY", false, false)
	require.NoError(t, config.SaveEditableConfig(config.UserConfigPath(), cfg))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "skills", "web-tools"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "skills", "web-tools", "SKILL.md"), []byte("name: web-tools\n"), 0644))

	report := Run(Options{Version: "test", SkillDir: filepath.Join(dir, "skills")})

	assert.True(t, report.Provider.Configured)
	assert.False(t, report.Provider.AuthConfigured)
	assertSuggestion(t, report, "configure_auth")
	assert.Contains(t, report.RenderText(), "ZHIPU_APIKEY=<redacted>")
	assert.NotContains(t, report.RenderText(), "super-secret-token")
}

func TestRunSuggestsReaderAutoWhenProviderReady(t *testing.T) {
	dir := tempHome(t)
	t.Setenv("ZHIPU_APIKEY", "super-secret-token")
	cfg := &config.EditableConfig{}
	config.AddBigModelProvider(cfg, "ZHIPU_APIKEY", false, false)
	require.NoError(t, config.SaveEditableConfig(config.UserConfigPath(), cfg))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "skills", "web-tools"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "skills", "web-tools", "SKILL.md"), []byte("name: web-tools\n"), 0644))

	report := Run(Options{Version: "test", SkillDir: filepath.Join(dir, "skills")})

	assert.True(t, report.Provider.AuthConfigured)
	assert.False(t, report.ReaderAuto.Contains)
	assertSuggestion(t, report, "enable_reader_auto")
	assert.NotContains(t, report.RenderText(), "super-secret-token")
}

func tempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("WEB_TOOLS_CONFIG", "")
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })
	return dir
}

func assertSuggestion(t *testing.T, report Report, id string) {
	t.Helper()
	for _, suggestion := range report.Suggestions {
		if suggestion.ID == id {
			return
		}
	}
	t.Fatalf("missing suggestion %q in %#v", id, report.Suggestions)
}

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
