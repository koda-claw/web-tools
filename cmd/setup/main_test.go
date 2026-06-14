package setupcmd

import (
	"os"
	"path/filepath"
	"testing"

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
