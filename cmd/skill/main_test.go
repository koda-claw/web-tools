package skillcmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallSkillFromLocalSource(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(t.TempDir(), "SKILL.md")
	require.NoError(t, os.WriteFile(source, []byte("---\nname: web-tools\ndescription: test\n---\n"), 0644))

	require.NoError(t, InstallSkill("dev", dir, source, false, false))

	data, err := os.ReadFile(filepath.Join(dir, "web-tools", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "name: web-tools")
}

func TestInstallSkillRefusesOverwriteWithoutForce(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(t.TempDir(), "SKILL.md")
	require.NoError(t, os.WriteFile(source, []byte("---\nname: web-tools\ndescription: test\n---\n"), 0644))
	require.NoError(t, InstallSkill("dev", dir, source, false, false))

	err := InstallSkill("dev", dir, source, false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "skill already installed")

	require.NoError(t, InstallSkill("dev", dir, source, true, false))
}
