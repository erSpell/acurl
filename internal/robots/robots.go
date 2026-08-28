// Package robots fetches robots.txt and answers RFC 9309 access questions.
package robots

import (
	"context"
	"fmt"
	"net/url"
	"sync"

	"github.com/jimsmart/grobotstxt"
)

// MaxSize is the largest robots.txt body that is parsed. RFC 9309 requires
// crawlers to parse at least 500 KiB.
const MaxSize = 500 * 1024

// Getter performs a GET request and returns the status code and body.
type Getter func(ctx context.Context, url string) (status int, body []byte, err error)

// Data is the robots.txt state for one origin.
type Data struct {
	URL     string `json:"url"`
	Status  int    `json:"status"`
	Content string `json:"-"`
	// Err is set when robots.txt could not be fetched or returned 5xx.
	Err error `json:"-"`
}

// Unreachable reports whether robots.txt failed with a network error or a
// server error. RFC 9309 §2.3.1.4 says crawlers must then assume full disallow.
func (d *Data) Unreachable() bool {
	return d.Err != nil
}

// Allowed reports whether agent may fetch rawURL.
func (d *Data) Allowed(agent, rawURL string) bool {
	switch {
	case d.Unreachable():
		return false
	case d.Status >= 400:
		// §2.3.1.3: an unavailable robots.txt (4xx) allows everything.
		return true
	}
	return grobotstxt.AgentAllowed(d.Content, agent, rawURL)
}

// URLFor returns the robots.txt URL for the origin of rawURL.
func URLFor(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("url %q has no scheme or host", rawURL)
	}
	return u.Scheme + "://" + u.Host + "/robots.txt", nil
}

// Cache fetches robots.txt at most once per origin and is safe for
// concurrent use.
type Cache struct {
	get     Getter
	mu      sync.Mutex
	entries map[string]*entry
}

type entry struct {
	once sync.Once
	data *Data
}

// NewCache returns a Cache that fetches with get.
func NewCache(get Getter) *Cache {
	return &Cache{get: get, entries: make(map[string]*entry)}
}

// Get returns the robots.txt data for the origin of target.
func (c *Cache) Get(ctx context.Context, target string) *Data {
	robotsURL, err := URLFor(target)
	if err != nil {
		return &Data{Err: err}
	}

	c.mu.Lock()
	e, ok := c.entries[robotsURL]
	if !ok {
		e = &entry{}
		c.entries[robotsURL] = e
	}
	c.mu.Unlock()

	e.once.Do(func() { e.data = c.fetch(ctx, robotsURL) })
	return e.data
}

func (c *Cache) fetch(ctx context.Context, robotsURL string) *Data {
	d := &Data{URL: robotsURL}
	status, body, err := c.get(ctx, robotsURL)
	d.Status = status
	switch {
	case err != nil:
		d.Err = err
	case status >= 500:
		d.Err = fmt.Errorf("robots.txt returned %d", status)
	case status >= 200 && status < 300:
		if len(body) > MaxSize {
			body = body[:MaxSize]
		}
		d.Content = string(body)
	}
	return d
}
