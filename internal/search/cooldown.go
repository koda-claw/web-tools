package search

import (
	"sync"
	"time"
)

const defaultProviderCooldown = 10 * time.Minute

var defaultSearchCooldowns = newSearchCooldowns(defaultProviderCooldown)

type searchCooldowns struct {
	mu       sync.Mutex
	until    map[string]time.Time
	duration time.Duration
	now      func() time.Time
}

func newSearchCooldowns(duration time.Duration) *searchCooldowns {
	return &searchCooldowns{
		until:    map[string]time.Time{},
		duration: duration,
		now:      time.Now,
	}
}

func (c *searchCooldowns) active(provider string) (time.Duration, bool) {
	if c == nil {
		return 0, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	until, ok := c.until[provider]
	if !ok {
		return 0, false
	}
	remaining := time.Until(until)
	if c.now != nil {
		remaining = until.Sub(c.now())
	}
	if remaining <= 0 {
		delete(c.until, provider)
		return 0, false
	}
	return remaining, true
}

func (c *searchCooldowns) mark(provider string) time.Duration {
	if c == nil || c.duration <= 0 {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if c.now != nil {
		now = c.now()
	}
	until := now.Add(c.duration)
	c.until[provider] = until
	return c.duration
}

func (c *searchCooldowns) clear(provider string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.until, provider)
}
