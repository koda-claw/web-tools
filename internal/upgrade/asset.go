package upgrade

import (
	"fmt"

	apperrors "github.com/koda-claw/web-tools/internal/errors"
)

// AssetName maps a platform to the release asset name.
func AssetName(p Platform) (string, error) {
	switch {
	case p.GOOS == "darwin" && p.GOARCH == "arm64":
		return "web-tools-darwin-arm64", nil
	case p.GOOS == "darwin" && p.GOARCH == "amd64":
		return "web-tools-darwin-amd64", nil
	case p.GOOS == "linux" && p.GOARCH == "amd64":
		return "web-tools-linux-amd64", nil
	case p.GOOS == "linux" && p.GOARCH == "arm64":
		return "web-tools-linux-arm64", nil
	case p.GOOS == "linux" && p.GOARCH == "arm":
		return "web-tools-linux-arm", nil
	case p.GOOS == "windows" && p.GOARCH == "amd64":
		return "web-tools-windows-amd64.exe", nil
	case p.GOOS == "windows" && p.GOARCH == "arm64":
		return "web-tools-windows-arm64.exe", nil
	case p.GOOS == "freebsd" && p.GOARCH == "amd64":
		return "web-tools-freebsd-amd64", nil
	default:
		return "", apperrors.NewInputError(
			"unsupported platform",
			fmt.Sprintf("%s/%s is not supported by release assets", p.GOOS, p.GOARCH),
			[]string{"supported: darwin/arm64, darwin/amd64, linux/amd64, linux/arm64, linux/arm, windows/amd64, windows/arm64, freebsd/amd64"},
		)
	}
}

func binaryName(goos string) string {
	if goos == "windows" {
		return "web-tools.exe"
	}
	return "web-tools"
}
