// Package acurl is the Go API behind the acurl command: it fetches a URL
// once per agent profile and reports how each agent was treated.
//
//	c, err := acurl.New(acurl.Options{})
//	if err != nil { ... }
//	rep, err := c.Check(ctx, "https://shop.example/products/1")
//	fmt.Println(rep.Score.Value)
//
// acurl is an open-source project by tragent.io (https://tragent.io).
package acurl

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"net/url"
	"time"

	"github.com/tragent-io/acurl/internal/classifier"
	"github.com/tragent-io/acurl/internal/fetcher"
	"github.com/tragent-io/acurl/internal/profiles"
	"github.com/tragent-io/acurl/internal/render"
	"github.com/tragent-io/acurl/internal/robots"
	"github.com/tragent-io/acurl/internal/score"
	"github.com/tragent-io/acurl/internal/sign"
	"github.com/tragent-io/acurl/pkg/types"
)

// Options configures a Checker. The zero value checks with the default
// profiles and sensible network settings.
type Options struct {
	// Profiles to fetch with. Nil means the built-in catalog.
	Profiles []types.Profile
	// Concurrency is the number of parallel requests per URL. Default 5.
	Concurrency int

	Timeout     time.Duration     // per request, default 30s
	Headers     map[string]string // extra headers for every request
	Retries     int               // retries after network or gateway errors
	Proxy       string            // http, https or socks5 proxy URL
	Insecure    bool              // skip TLS verification
	HTTPVersion string            // "", "1.1" or "2"
	Method      string            // default GET
	BotTLS      string            // "go" (default) or "chrome"
	MaxBody     int64             // body cap per response, default 5 MiB

	// SkipRobots disables robots.txt checks and drops robots-only profiles.
	SkipRobots bool
	// CloakThreshold is the similarity below which a response is cloaked.
	// Zero means the default (0.5); a negative value disables detection.
	CloakThreshold float64
	// Render also renders the page in headless Chrome (must be installed).
	Render     bool
	RenderWait time.Duration // default 2s

	// Samples checks the page this many times and reports each agent's most
	// common verdict, marking agents whose verdict changed as flaky.
	// Default 1. SampleDelay is the pause between samples, default 1s.
	Samples     int
	SampleDelay time.Duration

	// Page says what the page is: "product", "other", or "" to decide from
	// the URL and the page. Product pages are expected to have product data.
	Page string

	// UserAgent identifies acurl when it fetches robots.txt and llms.txt.
	UserAgent string
	// Trace, if set, is called after every page request.
	Trace func(TraceEvent)
	// SigningKey, if set, signs every bot-profile request with Web Bot Auth
	// (HTTP Message Signatures, RFC 9421). SignatureAgent is the URL where
	// the key directory is published.
	SigningKey     ed25519.PrivateKey
	SignatureAgent string
	// BlockPrivate refuses to connect to private, loopback, link-local and
	// cloud-metadata addresses. Enable it when URLs come from untrusted input.
	BlockPrivate bool
}

// TraceEvent describes one completed page request.
type TraceEvent = fetcher.TraceEvent

// Checker runs checks. It is safe for concurrent use and shares its HTTP
// connections and robots.txt cache across URLs.
type Checker struct {
	opts    Options
	fetcher *fetcher.Fetcher
	robots  *robots.Cache
}

// New returns a Checker for opts.
func New(opts Options) (*Checker, error) {
	if opts.Profiles == nil {
		ps, err := profiles.Default()
		if err != nil {
			return nil, err
		}
		opts.Profiles = ps
	}
	if opts.SkipRobots {
		opts.Profiles = profiles.WithoutRobotsOnly(opts.Profiles)
	}
	if len(opts.Profiles) == 0 {
		return nil, fmt.Errorf("acurl: no profiles")
	}
	switch opts.Page {
	case "", "auto":
		opts.Page = ""
	case types.PageProduct, types.PageOther:
	default:
		return nil, fmt.Errorf("acurl: page must be auto, product or other, not %q", opts.Page)
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = 5
	}
	switch {
	case opts.CloakThreshold == 0:
		opts.CloakThreshold = classifier.DefaultCloakThreshold
	case opts.CloakThreshold < 0:
		opts.CloakThreshold = 0
	}
	if opts.Samples < 1 {
		opts.Samples = 1
	}
	if opts.Samples > 10 {
		return nil, fmt.Errorf("acurl: samples must be between 1 and 10, not %d", opts.Samples)
	}
	if opts.SampleDelay <= 0 {
		opts.SampleDelay = time.Second
	}
	if opts.RenderWait <= 0 {
		opts.RenderWait = 2 * time.Second
	}
	if opts.UserAgent == "" {
		opts.UserAgent = "acurl (+https://github.com/tragent-io/acurl)"
	}

	var signer fetcher.RequestSigner
	if opts.SigningKey != nil {
		signer = &sign.Signer{Key: opts.SigningKey, Agent: opts.SignatureAgent}
	}
	f, err := fetcher.New(fetcher.Options{
		Timeout:      opts.Timeout,
		Headers:      opts.Headers,
		MaxBody:      opts.MaxBody,
		Retries:      opts.Retries,
		Proxy:        opts.Proxy,
		Insecure:     opts.Insecure,
		HTTPVersion:  opts.HTTPVersion,
		Method:       opts.Method,
		BotTLS:       opts.BotTLS,
		Trace:        opts.Trace,
		Signer:       signer,
		BlockPrivate: opts.BlockPrivate,
	})
	if err != nil {
		return nil, err
	}
	c := &Checker{opts: opts, fetcher: f}
	c.robots = robots.NewCache(func(ctx context.Context, u string) (int, []byte, error) {
		return f.Get(ctx, u, opts.UserAgent, robots.MaxSize)
	})
	return c, nil
}

// Profiles returns the profiles this Checker fetches with.
func (c *Checker) Profiles() []types.Profile { return c.opts.Profiles }

// Get fetches an auxiliary file (robots.txt, llms.txt, sitemaps) with
// acurl's own user agent, reading at most limit bytes.
func (c *Checker) Get(ctx context.Context, u string, limit int64) (int, []byte, error) {
	return c.fetcher.Get(ctx, u, c.opts.UserAgent, limit)
}

// Check fetches rawURL with every profile and returns the classified,
// scored report. Per-profile network failures are reported in the results;
// the error is only for invalid input.
func (c *Checker) Check(ctx context.Context, rawURL string) (*types.ComparisonResult, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("acurl: invalid http(s) URL %q", rawURL)
	}

	results := c.once(ctx, rawURL)
	if c.opts.Samples > 1 {
		all := [][]types.FetchResult{results}
		for i := 1; i < c.opts.Samples; i++ {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.opts.SampleDelay):
			}
			all = append(all, c.once(ctx, rawURL))
		}
		results = classifier.MergeSamples(all)
	}

	rep := &types.ComparisonResult{
		URL:       rawURL,
		Timestamp: time.Now().UTC(),
		Results:   results,
		Summary:   classifier.Summarize(results),
		Page:      c.opts.Page,
	}
	if c.opts.Render {
		rep.Render = c.render(ctx, rawURL, results)
	}
	rep.Score = score.Compute(rep)
	return rep, nil
}

// once fetches every profile one time and classifies the results.
func (c *Checker) once(ctx context.Context, rawURL string) []types.FetchResult {
	results := c.fetcher.FetchAll(ctx, rawURL, c.opts.Profiles, c.opts.Concurrency)
	if !c.opts.SkipRobots {
		c.applyRobots(ctx, rawURL, results)
	}
	classifier.ClassifyAll(results, c.opts.CloakThreshold)
	classifier.CompareProducts(results)
	return results
}

func (c *Checker) applyRobots(ctx context.Context, rawURL string, results []types.FetchResult) {
	data := c.robots.Get(ctx, rawURL)
	for i, p := range c.opts.Profiles {
		// A failed request already explains itself; a robots verdict for a
		// host that does not resolve would only add noise.
		if p.RobotsKey == "" || results[i].Error != "" {
			continue
		}
		if data.Unreachable() {
			results[i].RobotsError = data.Err.Error()
		}
		results[i].RobotsDisallowed = !data.Allowed(p.RobotsKey, rawURL)
	}
}

func (c *Checker) render(ctx context.Context, rawURL string, results []types.FetchResult) *types.RenderResult {
	b := classifier.BaselineIndex(results)
	if b < 0 || len(results[b].Body) == 0 {
		return &types.RenderResult{Error: "rendering needs a successful browser baseline"}
	}
	html, err := render.Headless(ctx, rawURL, render.Options{Timeout: c.opts.Timeout + c.opts.RenderWait + 15*time.Second, Wait: c.opts.RenderWait})
	if err != nil {
		return &types.RenderResult{Error: err.Error()}
	}
	return render.Compare(results[b].Body, html)
}

// Check is a convenience wrapper for New(opts) followed by Check.
func Check(ctx context.Context, rawURL string, opts Options) (*types.ComparisonResult, error) {
	c, err := New(opts)
	if err != nil {
		return nil, err
	}
	return c.Check(ctx, rawURL)
}
