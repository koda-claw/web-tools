package search

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
