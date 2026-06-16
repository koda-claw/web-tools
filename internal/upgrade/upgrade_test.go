package upgrade

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssetName(t *testing.T) {
	tests := []struct {
		name string
		p    Platform
		want string
	}{
		{"darwin arm64", Platform{"darwin", "arm64"}, "web-tools-darwin-arm64"},
		{"linux amd64", Platform{"linux", "amd64"}, "web-tools-linux-amd64"},
		{"windows amd64", Platform{"windows", "amd64"}, "web-tools-windows-amd64.exe"},
		{"freebsd amd64", Platform{"freebsd", "amd64"}, "web-tools-freebsd-amd64"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AssetName(tt.p)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
	_, err := AssetName(Platform{"plan9", "amd64"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported platform")
}

func TestParseChecksums(t *testing.T) {
	got := ParseChecksums("abc  web-tools-darwin-arm64\nDEF *web-tools-linux-amd64\n")
	assert.Equal(t, "abc", got["web-tools-darwin-arm64"])
	assert.Equal(t, "def", got["web-tools-linux-amd64"])
}

func TestBaseURLRequiresExplicitVersion(t *testing.T) {
	_, err := Runner{Platform: currentPlatform()}.Run(context.Background(), Options{
		CurrentVersion: "v1.0.0",
		TargetVersion:  "latest",
		BaseURL:        "http://example.test/releases",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--base-url requires explicit")
}

func TestCheckDoesNotModifyBinaryOrSkill(t *testing.T) {
	asset, err := AssetName(currentPlatform())
	require.NoError(t, err)
	newBin := buildFakeWebToolsBinary(t, "v9.9.9")
	server := fakeReleaseServer(t, asset, newBin, false)
	defer server.Close()

	dir := t.TempDir()
	bin := filepath.Join(dir, binaryName(runtime.GOOS))
	require.NoError(t, os.WriteFile(bin, []byte("old"), 0755))
	before := readFileHash(t, bin)
	skillDir := filepath.Join(dir, "skills")

	result, err := Runner{Platform: currentPlatform()}.Run(context.Background(), Options{
		CurrentVersion: "v1.0.0",
		TargetVersion:  "v9.9.9",
		BaseURL:        server.URL,
		Bin:            bin,
		SkillDir:       skillDir,
		Check:          true,
	})
	require.NoError(t, err)

	assert.True(t, result.OK)
	assert.True(t, result.ChecksumVerified)
	assert.Equal(t, before, readFileHash(t, bin))
	require.NoFileExists(t, filepath.Join(skillDir, "web-tools", "SKILL.md"))
}

func TestCheckFailsWhenChecksumMissing(t *testing.T) {
	asset, err := AssetName(currentPlatform())
	require.NoError(t, err)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v9.9.9/checksums.txt" {
			fmt.Fprintln(w, "abc  other-asset")
			return
		}
		if r.URL.Path == "/v9.9.9/"+asset {
			fmt.Fprintln(w, "unused")
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	_, err = Runner{Platform: currentPlatform()}.Run(context.Background(), Options{
		CurrentVersion: "v1.0.0",
		TargetVersion:  "v9.9.9",
		BaseURL:        server.URL,
		Bin:            filepath.Join(t.TempDir(), binaryName(runtime.GOOS)),
		Check:          true,
		SkipSkill:      true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum missing")
}

func TestRunUpgradeWithFakeReleaseAndSkillSource(t *testing.T) {
	newBin := buildFakeWebToolsBinary(t, "v9.9.9")
	asset, err := AssetName(currentPlatform())
	require.NoError(t, err)
	server := fakeReleaseServer(t, asset, newBin, false)
	defer server.Close()

	dir := t.TempDir()
	targetBin := filepath.Join(dir, binaryName(runtime.GOOS))
	require.NoError(t, os.WriteFile(targetBin, []byte("old"), 0755))
	skillSource := writeUpgradeSkill(t)
	skillDir := filepath.Join(dir, "skills")

	result, err := Runner{Platform: currentPlatform()}.Run(context.Background(), Options{
		CurrentVersion: "v1.0.0",
		TargetVersion:  "v9.9.9",
		BaseURL:        server.URL,
		Bin:            targetBin,
		SkillDir:       skillDir,
		SkillSource:    skillSource,
	})
	require.NoError(t, err)

	assert.True(t, result.CLIUpdated)
	assert.True(t, result.ChecksumVerified)
	assert.True(t, result.SkillUpdated)
	out, err := exec.Command(targetBin, "--version").CombinedOutput()
	require.NoError(t, err)
	assert.Contains(t, string(out), "v9.9.9")
	require.FileExists(t, filepath.Join(skillDir, "web-tools", "SKILL.md"))
}

func TestRunOnlySkillDoesNotTouchBinary(t *testing.T) {
	dir := t.TempDir()
	targetBin := filepath.Join(dir, binaryName(runtime.GOOS))
	require.NoError(t, os.WriteFile(targetBin, []byte("old"), 0755))
	before := readFileHash(t, targetBin)

	result, err := Runner{Platform: currentPlatform()}.Run(context.Background(), Options{
		CurrentVersion: "v1.0.0",
		TargetVersion:  "v9.9.9",
		Bin:            targetBin,
		OnlySkill:      true,
		SkillDir:       filepath.Join(dir, "skills"),
		SkillSource:    writeUpgradeSkill(t),
	})
	require.NoError(t, err)

	assert.False(t, result.CLIUpdated)
	assert.True(t, result.SkillUpdated)
	assert.Equal(t, before, readFileHash(t, targetBin))
}

func TestRunUpgradeChecksumMismatchDoesNotReplaceBinary(t *testing.T) {
	newBin := buildFakeWebToolsBinary(t, "v9.9.9")
	asset, err := AssetName(currentPlatform())
	require.NoError(t, err)
	server := fakeReleaseServer(t, asset, newBin, true)
	defer server.Close()

	dir := t.TempDir()
	targetBin := filepath.Join(dir, binaryName(runtime.GOOS))
	require.NoError(t, os.WriteFile(targetBin, []byte("old"), 0755))
	before := readFileHash(t, targetBin)

	_, err = Runner{Platform: currentPlatform()}.Run(context.Background(), Options{
		CurrentVersion: "v1.0.0",
		TargetVersion:  "v9.9.9",
		BaseURL:        server.URL,
		Bin:            targetBin,
		SkipSkill:      true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum mismatch")
	assert.Equal(t, before, readFileHash(t, targetBin))
}

func fakeReleaseServer(t *testing.T, asset string, binPath string, badChecksum bool) *httptest.Server {
	t.Helper()
	binData, err := os.ReadFile(binPath)
	require.NoError(t, err)
	hashBytes := sha256.Sum256(binData)
	hash := hex.EncodeToString(hashBytes[:])
	if badChecksum {
		hash = strings.Repeat("0", 64)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v9.9.9/" + asset:
			_, _ = w.Write(binData)
		case "/v9.9.9/checksums.txt":
			fmt.Fprintf(w, "%s  %s\n", hash, asset)
		default:
			http.NotFound(w, r)
		}
	}))
}

func buildFakeWebToolsBinary(t *testing.T, version string) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	code := fmt.Sprintf(`package main
import "fmt"
func main() { fmt.Println("web-tools version %s") }
`, version)
	require.NoError(t, os.WriteFile(src, []byte(code), 0644))
	bin := filepath.Join(dir, binaryName(runtime.GOOS))
	cmd := exec.Command("go", "build", "-o", bin, src)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return bin
}

func writeUpgradeSkill(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "SKILL.md")
	require.NoError(t, os.WriteFile(path, []byte("---\nname: web-tools\ndescription: upgrade test\n---\n"), 0644))
	return path
}

func readFileHash(t *testing.T, path string) string {
	t.Helper()
	sum, err := fileSHA256(path)
	require.NoError(t, err)
	return sum
}
