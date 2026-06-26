package search

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	apperrors "github.com/koda-claw/web-tools/internal/errors"
)

const (
	politeMaxAttempts = 2
)

var politeSleep = time.Sleep

type politeLimiter struct {
	mu       sync.Mutex
	last     time.Time
	interval time.Duration
}

func newPoliteLimiter(interval time.Duration) *politeLimiter {
	return &politeLimiter{interval: interval}
}

func (l *politeLimiter) wait() {
	if l == nil || l.interval <= 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.last.IsZero() {
		elapsed := time.Since(l.last)
		if elapsed < l.interval {
			politeSleep(l.interval - elapsed)
		}
	}
	l.last = time.Now()
}

func doPoliteSearchRequest(client *http.Client, req *http.Request, opts politeRequestOptions) ([]byte, int, error) {
	if client == nil {
		client = http.DefaultClient
	}
	attempts := opts.attempts
	if attempts <= 0 {
		attempts = politeMaxAttempts
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if opts.limiter != nil {
			opts.limiter.wait()
		}
		reqAttempt := req.Clone(req.Context())
		resp, err := client.Do(reqAttempt)
		if err != nil {
			lastErr = apperrors.NewNetworkError(
				opts.engine+" request failed",
				err.Error(),
				map[string]string{"url": req.URL.String(), "timeout": opts.timeout.String()},
				[]string{"check network connectivity", "try --provider auto"},
			)
			if attempt < attempts {
				politeSleep(opts.retryDelay(attempt))
				continue
			}
			return nil, 0, lastErr
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, resp.StatusCode, apperrors.NewNetworkError("failed to read "+opts.engine+" response", readErr.Error(), map[string]string{"url": req.URL.String()}, nil)
		}
		bodyText := string(body)
		if opts.blocked != nil && opts.blocked(resp.StatusCode, bodyText) {
			return nil, resp.StatusCode, newSearchRateLimitError(opts.engine, req.URL.String(), rateLimitReason(resp.StatusCode, bodyText), resp.StatusCode)
		}
		if shouldRetrySearchStatus(resp.StatusCode) && attempt < attempts {
			lastErr = apperrors.NewEngineError(
				opts.engine+" returned retryable status",
				fmt.Sprintf("HTTP %d", resp.StatusCode),
				map[string]string{"url": req.URL.String(), "status_code": fmt.Sprintf("%d", resp.StatusCode)},
				nil,
			)
			politeSleep(opts.retryDelay(attempt))
			continue
		}
		return body, resp.StatusCode, nil
	}
	return nil, 0, lastErr
}

type politeRequestOptions struct {
	engine       string
	timeout      time.Duration
	limiter      *politeLimiter
	attempts     int
	blocked      func(int, string) bool
	retryDelayFn func(int) time.Duration
}

func shouldRetrySearchStatus(statusCode int) bool {
	return statusCode == http.StatusBadGateway ||
		statusCode == http.StatusServiceUnavailable ||
		statusCode == http.StatusGatewayTimeout
}

func fixedRetryDelay(delay time.Duration) func(int) time.Duration {
	return func(int) time.Duration { return delay }
}

func (o politeRequestOptions) retryDelay(attempt int) time.Duration {
	if o.retryDelayFn == nil {
		return 0
	}
	return o.retryDelayFn(attempt)
}
