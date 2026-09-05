// Package fetcher retrieves a URL once per agent profile.
package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/imroc/req/v3"

	"github.com/tragent-io/acurl/internal/detect"
	"github.com/tragent-io/acurl/pkg/types"
)

// Options configures a Fetcher.
type Options struct {
	// Timeout bounds each request, including reading the body.
	Timeout time.Duration
	// Headers are added to every request after the profile headers.
	Headers map[string]string
	// MaxBody caps how many body bytes are read and kept. Zero means DefaultMaxBody.
	MaxBody int64
	// Retries is how many times a request is retried after a network error
	// or a 502/504 gateway error. Other statuses are never retried because
	// they are exactly what acurl reports on.
	Retries int
}

// DefaultMaxBody is the default response body cap (5 MiB).
const DefaultMaxBody = 5 << 20

// Fetcher fetches URLs with a browser-like TLS fingerprint so that the only
// variable between profiles is what the profile itself sends.
type Fetcher struct {
	client *req.Client
	opts   Options
}

// New returns a Fetcher configured with opts.
func New(opts Options) *Fetcher {
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.MaxBody <= 0 {
		opts.MaxBody = DefaultMaxBody
	}
	client := req.C().
		SetTimeout(opts.Timeout).
		SetRedirectPolicy(recordRedirects).
		ImpersonateChrome()
	if opts.Retries > 0 {
		client.
			SetCommonRetryCount(opts.Retries).
			SetCommonRetryBackoffInterval(250*time.Millisecond, 4*time.Second).
			SetCommonRetryCondition(shouldRetry)
	}
	return &Fetcher{client: client, opts: opts}
}

func shouldRetry(resp *req.Response, err error) bool {
	if err != nil {
		return true
	}
	return resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusGatewayTimeout
}

// Fetch retrieves url using profile. Network failures are reported in the
// result rather than returned, so one failing profile never aborts a run.
func (f *Fetcher) Fetch(ctx context.Context, url string, profile types.Profile) types.FetchResult {
	result := types.FetchResult{Profile: profile.Name}
	start := time.Now()

	var chain []string
	r := f.client.R().
		SetContext(withRedirectChain(ctx, &chain)).
		SetHeader("User-Agent", profile.UserAgent)
	for k, v := range profile.Headers {
		r.SetHeader(k, v)
	}
	for k, v := range f.opts.Headers {
		r.SetHeader(k, v)
	}

	resp, err := r.Get(url)
	result.RedirectChain = chain
	if err != nil {
		result.SetDuration(time.Since(start))
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()

	result.Success = true
	result.StatusCode = resp.StatusCode
	// resp.Request is the request acurl built; the underlying response
	// carries the request that produced it, i.e. the last redirect hop.
	if resp.Response != nil && resp.Response.Request != nil {
		result.FinalURL = resp.Response.Request.URL.String()
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		result.RateLimited = true
		result.RetryAfter = resp.Header.Get("Retry-After")
	}

	// Read one byte past the cap to know whether the body was truncated.
	body, err := io.ReadAll(io.LimitReader(resp.Body, f.opts.MaxBody+1))
	result.SetDuration(time.Since(start))
	if err != nil {
		result.Error = fmt.Sprintf("read body: %v", err)
		return result
	}
	if int64(len(body)) > f.opts.MaxBody {
		body = body[:f.opts.MaxBody]
		result.Truncated = true
	}
	result.Body = body
	result.BodySize = len(body)
	result.ContentHash = contentHash(body)
	result.Title = extractTitle(string(body))
	result.BotChallenge = detect.Challenge(resp.StatusCode, resp.Header, body)
	return result
}

// Get performs a plain GET with the given user agent and returns at most
// limit bytes of the body. It is used for auxiliary files such as robots.txt.
func (f *Fetcher) Get(ctx context.Context, url, userAgent string, limit int64) (int, []byte, error) {
	resp, err := f.client.R().
		SetContext(ctx).
		SetHeader("User-Agent", userAgent).
		Get(url)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	return resp.StatusCode, body, err
}
