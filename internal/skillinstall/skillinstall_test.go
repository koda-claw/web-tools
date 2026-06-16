package skillinstall

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallFromLocalSource(t *testing.T) {
	source := writeSkillSource(t, "---\nname: web-tools\ndescription: test\n---\n")
	dir := t.TempDir()

	result, err := Install(context.Background(), Options{
		Version: "dev",
		Dir:     dir,
		Source:  source,
	})
	require.NoError(t, err)

	assert.Equal(t, source, result.Source)
	assert.Equal(t, filepath.Join(dir, "web-tools", "SKILL.md"), result.SkillPath)
	data, err := os.ReadFile(result.SkillPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "name: web-tools")
}

func TestInstallFromHTTPSource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "---\nname: web-tools\ndescription: test\n---\n")
	}))
	defer server.Close()

	result, err := Install(context.Background(), Options{
		Version: "v9.9.9",
		Dir:     t.TempDir(),
		Source:  server.URL + "/SKILL.md",
	})
	require.NoError(t, err)

	assert.Equal(t, server.URL+"/SKILL.md", result.Source)
}

func TestInstallRefusesOverwriteWithoutForce(t *testing.T) {
	source := writeSkillSource(t, "---\nname: web-tools\ndescription: test\n---\n")
	dir := t.TempDir()
	_, err := Install(context.Background(), Options{Version: "dev", Dir: dir, Source: source})
	require.NoError(t, err)

	_, err = Install(context.Background(), Options{Version: "dev", Dir: dir, Source: source})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "skill already installed")

	_, err = Install(context.Background(), Options{Version: "dev", Dir: dir, Source: source, Force: true})
	require.NoError(t, err)
}

func TestDefaultSourceUsesReleaseTagOrMain(t *testing.T) {
	assert.Equal(t, "https://raw.githubusercontent.com/koda-claw/web-tools/v1.2.3/skills/web-tools/SKILL.md", DefaultSource("v1.2.3"))
	assert.Equal(t, "https://raw.githubusercontent.com/koda-claw/web-tools/main/skills/web-tools/SKILL.md", DefaultSource("dev"))
}

func writeSkillSource(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "SKILL.md")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}
