// Package fetcher retrieves a URL once per agent profile.
package fetcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/imroc/req/v3"
	utls "github.com/refraction-networking/utls"

	"github.com/tragent-io/acurl/internal/detect"
	"github.com/tragent-io/acurl/internal/extract"
	"github.com/tragent-io/acurl/internal/netguard"
	"github.com/tragent-io/acurl/internal/render"
	"github.com/tragent-io/acurl/internal/robots"
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
	// Proxy is an http, https or socks5 proxy URL.
	Proxy string
	// Insecure skips TLS certificate verification.
	Insecure bool
	// HTTPVersion forces "1.1" or "2". Empty negotiates as a browser would.
	HTTPVersion string
	// Method is the HTTP method for page requests. Empty means GET.
	Method string
	// BotTLS selects the TLS fingerprint for non-browser profiles: "go"
	// (default, like most real crawlers) or "chrome" to make the user agent
	// the only difference from the browser baseline.
	BotTLS string
	// Trace, if set, is called after every page request.
	Trace func(TraceEvent)
	// Signer, if set, signs every bot-profile request (Web Bot Auth).
	Signer RequestSigner
	// BlockPrivate refuses connections to private, loopback and other
	// internal addresses (SSRF protection for acurl serve and mcp).
	BlockPrivate bool
}

// RequestSigner returns signature headers for a request.
type RequestSigner interface {
	Headers(method, url string) (map[string]string, error)
}

// TraceEvent describes one completed page request.
type TraceEvent struct {
	Profile  string
	Method   string
	URL      string
	Status   int
	Duration time.Duration
	Err      error
}

// DefaultMaxBody is the default response body cap (5 MiB).
const DefaultMaxBody = 5 << 20

// Fetcher fetches URLs once per profile. Browser profiles get a full Chrome
// impersonation (TLS, HTTP/2 settings and default headers); bot profiles get
// a plain client that only sends the profile's own headers, because real
// crawlers do not send Chrome client hints.
type Fetcher struct {
	browser *req.Client
	bot     *req.Client
	opts    Options
}

// New returns a Fetcher configured with opts.
func New(opts Options) (*Fetcher, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.MaxBody <= 0 {
		opts.MaxBody = DefaultMaxBody
	}
	opts.Method = strings.ToUpper(opts.Method)
	if opts.Method == "" {
		opts.Method = http.MethodGet
	}
	browser, err := newClient(opts)
	if err != nil {
		return nil, err
	}
	// ImpersonateChrome sets Chrome's HTTP/2 settings and header order; the
	// TLS hello is then moved to the newest Chrome fingerprint uTLS ships,
	// which has the post-quantum key share current Chrome sends.
	browser.ImpersonateChrome().SetTLSFingerprint(utls.HelloChrome_133)

	bot, err := newClient(opts)
	if err != nil {
		return nil, err
	}
	switch opts.BotTLS {
	case "", "go":
	case "chrome":
		bot.SetTLSFingerprintChrome()
	default:
		return nil, fmt.Errorf("unsupported bot TLS fingerprint %q (use go or chrome)", opts.BotTLS)
	}
	return &Fetcher{browser: browser, bot: bot, opts: opts}, nil
}

func newClient(opts Options) (*req.Client, error) {
	c := req.C().
		SetTimeout(opts.Timeout).
		SetRedirectPolicy(recordRedirects).
		// Profiles set Accept-Encoding themselves (Chrome sends br and zstd),
		// which turns off Go's transparent gzip; decode every encoding here.
		EnableAutoDecompress()
	if opts.Retries > 0 {
		c.SetCommonRetryCount(opts.Retries).
			SetCommonRetryBackoffInterval(250*time.Millisecond, 4*time.Second).
			SetCommonRetryCondition(shouldRetry)
	}
	if opts.Proxy != "" {
		c.SetProxyURL(opts.Proxy)
	}
	if opts.BlockPrivate {
		c.SetDial(netguard.DialContext(opts.Timeout))
	}
	if opts.Insecure {
		c.EnableInsecureSkipVerify()
	}
	switch opts.HTTPVersion {
	case "":
	case "1.1", "1":
		c.EnableForceHTTP1()
	case "2":
		c.EnableForceHTTP2()
	default:
		return nil, fmt.Errorf("unsupported HTTP version %q (use 1.1 or 2)", opts.HTTPVersion)
	}
	return c, nil
}

func shouldRetry(resp *req.Response, err error) bool {
	if errors.Is(err, errTooManyRedirects) || errors.Is(err, netguard.ErrBlocked) {
		return false // retrying cannot change the answer
	}
	if err != nil {
		return true
	}
	return resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusGatewayTimeout
}

// Fetch retrieves url using profile. Network failures are reported in the
// result rather than returned, so one failing profile never aborts a run.
func (f *Fetcher) Fetch(ctx context.Context, url string, profile types.Profile) types.FetchResult {
	result := types.FetchResult{Profile: profile.Name, Browser: profile.Browser, Category: profile.Category}
	start := time.Now()

	client := f.bot
	if profile.Browser {
		client = f.browser
	}
	var chain []string
	r := client.R().
		SetContext(withRedirectChain(ctx, &chain)).
		SetHeader("User-Agent", profile.UserAgent)
	for k, v := range profile.Headers {
		r.SetHeader(k, v)
	}
	for k, v := range f.opts.Headers {
		r.SetHeader(k, v)
	}
	if f.opts.Signer != nil && !profile.Browser {
		sig, err := f.opts.Signer.Headers(f.opts.Method, url)
		if err != nil {
			result.Error = err.Error()
			return result
		}
		for k, v := range sig {
			r.SetHeader(k, v)
		}
		result.Signed = true
	}

	resp, err := r.Send(f.opts.Method, url)
	if f.opts.Trace != nil {
		ev := TraceEvent{Profile: profile.Name, Method: f.opts.Method, URL: url, Duration: time.Since(start), Err: err}
		if err == nil {
			ev.Status = resp.StatusCode
		}
		f.opts.Trace(ev)
	}
	result.RedirectChain = chain
	if err != nil {
		result.SetDuration(time.Since(start))
		result.Error = err.Error()
		if errors.Is(err, errTooManyRedirects) {
			result.Error = errTooManyRedirects.Error()
		}
		if n := len(chain); n > 0 {
			result.FinalURL = chain[n-1]
		}
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
		if d, ok := parseRetryAfter(result.RetryAfter, time.Now()); ok {
			result.RetryAfterSeconds = int(d.Seconds())
		}
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
	result.ContentType = resp.Header.Get("Content-Type")
	result.BotChallenge = detect.Challenge(resp.StatusCode, resp.Header, body)
	if result.BotChallenge == "" {
		result.Edge = detect.Edge(resp.StatusCode, resp.Header)
	}
	if !isHTML(result.ContentType, body) {
		// PDFs, images and JSON have no title, meta tags or product markup;
		// parsing them as HTML is wasted work and produces noise.
		result.Directives = robots.Directives(resp.Header, nil, profile.RobotsKey)
		return result
	}
	result.Title = extractTitle(string(body))
	result.Directives = robots.Directives(resp.Header, body, profile.RobotsKey)
	result.Product = extract.Product(body)
	result.JS = render.Analyze(body)
	return result
}

// Get performs a plain GET with the given user agent and returns at most
// limit bytes of the body. It is used for auxiliary files such as robots.txt.
func (f *Fetcher) Get(ctx context.Context, url, userAgent string, limit int64) (int, []byte, error) {
	resp, err := f.bot.R().
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
