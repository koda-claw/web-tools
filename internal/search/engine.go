package search

import "errors"

// ErrRateLimited marks backend responses that look like rate limiting or
// anti-bot blocking. Callers can use errors.Is(err, ErrRateLimited).
var ErrRateLimited = errors.New("search engine rate limited")

// RateLimitError preserves the structured backend error while exposing the
// stable ErrRateLimited sentinel for retry and fallback decisions.
type RateLimitError struct {
	Engine string
	Reason string
	Err    error
}

func (e *RateLimitError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	if e.Reason != "" {
		return e.Engine + " rate limited: " + e.Reason
	}
	return e.Engine + " rate limited"
}

func (e *RateLimitError) Unwrap() error { return e.Err }

func (e *RateLimitError) Is(target error) bool {
	return target == ErrRateLimited
}

// Engine is the interface that all search backends must implement.
type Engine interface {
	Name() string
	HealthCheck() error
	Query(query string, opts SearchOptions) ([]RawResult, error)
}

// RawResult is a normalized result returned by any engine.
type RawResult struct {
	Title   string
	URL     string
	Snippet string
	Source  string
	Extra   map[string]string // engine-specific fields, e.g. "published_date"
}
