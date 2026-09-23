package batch

import (
	"context"
	"net/url"
	"sync"
	"time"
)

// hostLimiter spaces out checks of URLs on the same host.
type hostLimiter struct {
	delay time.Duration
	mu    sync.Mutex
	next  map[string]time.Time
}

func newHostLimiter(delay time.Duration) *hostLimiter {
	return &hostLimiter{delay: delay, next: map[string]time.Time{}}
}

// wait blocks until a check of rawURL may start, or ctx is done.
func (l *hostLimiter) wait(ctx context.Context, rawURL string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if l.delay <= 0 {
		return nil
	}
	host := rawURL
	if u, err := url.Parse(rawURL); err == nil {
		host = u.Host
	}
	l.mu.Lock()
	now := time.Now()
	start := l.next[host]
	if start.Before(now) {
		start = now
	}
	l.next[host] = start.Add(l.delay)
	l.mu.Unlock()

	t := time.NewTimer(time.Until(start))
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
