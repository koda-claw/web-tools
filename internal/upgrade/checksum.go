package upgrade

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	apperrors "github.com/koda-claw/web-tools/internal/errors"
)

// ParseChecksums reads a sha256sum-style manifest.
func ParseChecksums(raw string) map[string]string {
	out := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		hash := strings.ToLower(fields[0])
		name := strings.TrimPrefix(fields[1], "*")
		out[name] = hash
	}
	return out
}

func verifyChecksum(path string, manifest string, asset string) error {
	want := ParseChecksums(manifest)[asset]
	if want == "" {
		return apperrors.NewInputError(
			"checksum missing",
			fmt.Sprintf("checksums.txt does not contain %s", asset),
			[]string{"choose a release with checksums.txt", "retry after the release assets are complete"},
		)
	}
	got, err := fileSHA256(path)
	if err != nil {
		return apperrors.NewInputError("cannot hash download", err.Error(), []string{"retry the upgrade"})
	}
	if got != strings.ToLower(want) {
		return apperrors.NewInputError(
			"checksum mismatch",
			fmt.Sprintf("expected %s but got %s", want, got),
			[]string{"delete the downloaded file and retry", "check the release integrity"},
		)
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
