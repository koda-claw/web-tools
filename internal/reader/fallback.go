package reader

import apperrors "github.com/koda-claw/web-tools/internal/errors"

// ShouldProviderFallbackError reports whether a reader error may benefit from
// trying the next configured provider in an auto chain.
func ShouldProviderFallbackError(err error) bool {
	var appErr *apperrors.AppError
	if !apperrors.As(err, &appErr) {
		return false
	}
	switch appErr.Category {
	case "network", "extract", "engine":
		return true
	default:
		return false
	}
}
