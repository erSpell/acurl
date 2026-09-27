package cli

import (
	"net/http"
	"time"

	"github.com/spf13/pflag"

	"github.com/tragent-io/acurl/internal/fetcher"
	"github.com/tragent-io/acurl/internal/profiles"
	"github.com/tragent-io/acurl/pkg/acurl"
	"github.com/tragent-io/acurl/pkg/types"
)

// httpFlags are shared by every command that fetches pages.
type httpFlags struct {
	agents       []string
	categories   []string
	profilesFile string
	timeout      time.Duration
	headers      []string
	concurrency  int
	retries      int
	proxy        string
	insecure     bool
	httpVersion  string
	method       string
	head         bool
	botTLS       string
	maxBody      int64
	noColor      bool
}

func (h *httpFlags) register(f *pflag.FlagSet) {
	f.StringSliceVarP(&h.agents, "agents", "a", nil, "only use these profiles (comma-separated)")
	f.StringSliceVar(&h.categories, "category", nil, "only use profiles in these categories: ai-assistant, ai-search, ai-training, search, other (the browser baseline is always kept)")
	f.StringVar(&h.profilesFile, "profiles", "", "custom profiles YAML file")
	f.DurationVarP(&h.timeout, "timeout", "t", 30*time.Second, "per-request timeout")
	f.StringArrayVarP(&h.headers, "header", "H", nil, `extra header "Name: value" (repeatable)`)
	f.IntVarP(&h.concurrency, "concurrency", "c", 5, "maximum concurrent requests")
	f.IntVar(&h.retries, "retries", 1, "retries after network or gateway errors (exponential backoff with jitter)")
	f.StringVar(&h.proxy, "proxy", "", "proxy URL (http://, https:// or socks5://)")
	f.BoolVarP(&h.insecure, "insecure", "k", false, "skip TLS certificate verification")
	f.StringVar(&h.httpVersion, "http", "", `force HTTP version: "1.1" or "2"`)
	f.StringVarP(&h.method, "request", "X", "GET", "HTTP method to use")
	f.BoolVarP(&h.head, "head", "I", false, "send HEAD requests (status and headers only)")
	f.StringVar(&h.botTLS, "tls", "go", `TLS fingerprint for bot profiles: "go" (like real crawlers) or "chrome" (only the user agent differs)`)
	f.Int64Var(&h.maxBody, "max-body", fetcher.DefaultMaxBody, "maximum response body bytes to read per profile")
	f.BoolVar(&h.noColor, "no-color", false, "disable colored output")
}

// options builds checker options from the flags. Profile selection is
// included so every command filters the same way.
func (h *httpFlags) options(a *app) (acurl.Options, error) {
	headers, err := parseHeaders(h.headers)
	if err != nil {
		return acurl.Options{}, err
	}
	ps, err := h.loadProfiles()
	if err != nil {
		return acurl.Options{}, err
	}
	method := h.method
	if h.head {
		method = http.MethodHead
	}
	return acurl.Options{
		Profiles:    ps,
		Concurrency: h.concurrency,
		Timeout:     h.timeout,
		Headers:     headers,
		MaxBody:     h.maxBody,
		Retries:     h.retries,
		Proxy:       h.proxy,
		Insecure:    h.insecure,
		HTTPVersion: h.httpVersion,
		Method:      method,
		BotTLS:      h.botTLS,
		UserAgent:   a.userAgent(),
		Trace:       a.trace(),
	}, nil
}

func (h *httpFlags) loadProfiles() ([]types.Profile, error) {
	all, err := profiles.Load(h.profilesFile)
	if err != nil {
		return nil, err
	}
	selected := profiles.FilterCategories(profiles.Filter(all, h.agents), h.categories)
	if len(selected) == 0 {
		return nil, errNoProfiles(append(h.agents, h.categories...))
	}
	return selected, nil
}
