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
	hosts map[string]*hostSlot
}

// hostSlot serialises waiters for one host. The gap is measured from when
// the previous check really started, so a timer that fires late (Windows
// timers are coarse) cannot shorten the next gap.
type hostSlot struct {
	gate chan struct{}
	last time.Time
}

func newHostLimiter(delay time.Duration) *hostLimiter {
	return &hostLimiter{delay: delay, hosts: map[string]*hostSlot{}}
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
	s := l.hosts[host]
	if s == nil {
		s = &hostSlot{gate: make(chan struct{}, 1)}
		l.hosts[host] = s
	}
	l.mu.Unlock()

	select {
	case s.gate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-s.gate }()

	if !s.last.IsZero() {
		for {
			d := time.Until(s.last.Add(l.delay))
			if d <= 0 {
				break
			}
			t := time.NewTimer(d)
			select {
			case <-t.C:
			case <-ctx.Done():
				t.Stop()
				return ctx.Err()
			}
		}
	}
	s.last = time.Now()
	return nil
}
