package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tragent-io/acurl/pkg/types"
)

func newServer(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func mustNew(t *testing.T, opts Options) *Fetcher {
	t.Helper()
	f, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestNewRejectsUnknownHTTPVersion(t *testing.T) {
	if _, err := New(Options{HTTPVersion: "3"}); err == nil {
		t.Error("expected error for HTTP/3")
	}
}

func TestFetchThroughProxy(t *testing.T) {
	var proxied int32
	proxy := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&proxied, 1)
		_, _ = w.Write([]byte("via proxy"))
	})
	res := mustNew(t, Options{Proxy: proxy.URL}).Fetch(context.Background(), "http://target.invalid/", types.Profile{Name: "x", UserAgent: "x"})
	if res.Error != "" || proxied != 1 {
		t.Errorf("proxy not used: err=%q proxied=%d", res.Error, proxied)
	}
}

func TestFetchSendsProfileHeaders(t *testing.T) {
	var gotUA, gotCustom, gotExtra string
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotCustom = r.Header.Get("X-Profile")
		gotExtra = r.Header.Get("X-Extra")
		_, _ = w.Write([]byte("<html><head><title> Hello &amp; welcome </title></head></html>"))
	})

	f := mustNew(t, Options{Timeout: 5 * time.Second, Headers: map[string]string{"X-Extra": "yes"}})
	p := types.Profile{Name: "Test", UserAgent: "TestBot/1.0", Headers: map[string]string{"X-Profile": "p"}}
	res := f.Fetch(context.Background(), srv.URL, p)

	if res.Error != "" {
		t.Fatalf("unexpected error: %s", res.Error)
	}
	if gotUA != "TestBot/1.0" || gotCustom != "p" || gotExtra != "yes" {
		t.Errorf("headers not sent: ua=%q profile=%q extra=%q", gotUA, gotCustom, gotExtra)
	}
	if res.StatusCode != http.StatusOK || !res.Success {
		t.Errorf("status = %d success = %v", res.StatusCode, res.Success)
	}
	if res.Title != "Hello & welcome" {
		t.Errorf("title = %q", res.Title)
	}
	if res.ContentHash == "" || res.BodySize == 0 {
		t.Errorf("expected hash and size, got %q %d", res.ContentHash, res.BodySize)
	}
}

func TestFetchRateLimited(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	res := mustNew(t, Options{}).Fetch(context.Background(), srv.URL, types.Profile{Name: "x", UserAgent: "x"})
	if !res.RateLimited || res.RetryAfter != "120" {
		t.Errorf("rate limit not detected: %+v", res)
	}
}

func TestFetchNetworkError(t *testing.T) {
	res := mustNew(t, Options{Timeout: time.Second}).Fetch(context.Background(), "http://127.0.0.1:1", types.Profile{Name: "x", UserAgent: "x"})
	if res.Error == "" || res.Success {
		t.Errorf("expected error, got %+v", res)
	}
}

func TestFetchTimeout(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	})
	res := mustNew(t, Options{Timeout: 100 * time.Millisecond}).Fetch(context.Background(), srv.URL, types.Profile{Name: "x", UserAgent: "x"})
	if res.Error == "" {
		t.Error("expected timeout error")
	}
}

func TestFetchAllKeepsOrderAndLimitsConcurrency(t *testing.T) {
	var inflight, peak int32
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&inflight, 1)
		for {
			p := atomic.LoadInt32(&peak)
			if n <= p || atomic.CompareAndSwapInt32(&peak, p, n) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		atomic.AddInt32(&inflight, -1)
		_, _ = w.Write([]byte(r.Header.Get("User-Agent")))
	})

	var ps []types.Profile
	for _, n := range []string{"a", "b", "c", "d", "e", "f"} {
		ps = append(ps, types.Profile{Name: n, UserAgent: "ua-" + n})
	}
	results := mustNew(t, Options{}).FetchAll(context.Background(), srv.URL, ps, 2)

	for i, r := range results {
		if r.Profile != ps[i].Name {
			t.Errorf("result %d profile = %q, want %q", i, r.Profile, ps[i].Name)
		}
	}
	if peak > 2 {
		t.Errorf("peak concurrency = %d, want <= 2", peak)
	}
}

func TestExtractTitle(t *testing.T) {
	tests := map[string]string{
		"<title>Simple</title>":                           "Simple",
		"<TITLE lang=en>\n  Multi\n line </TITLE>":        "Multi line",
		"<p>no title</p>":                                 "",
		"<title>" + strings.Repeat("é", 150) + "</title>": strings.Repeat("é", 100) + "...",
	}
	for in, want := range tests {
		if got := extractTitle(in); got != want {
			t.Errorf("extractTitle(%q) = %q, want %q", in[:min(len(in), 30)], got, want)
		}
	}
}

func TestFetchRecordsRedirectChain(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/a", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/b", http.StatusMovedPermanently) })
	mux.HandleFunc("/b", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/c", http.StatusFound) })
	mux.HandleFunc("/c", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("done")) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	res := mustNew(t, Options{}).Fetch(context.Background(), srv.URL+"/a", types.Profile{Name: "x", UserAgent: "x"})
	if res.Error != "" {
		t.Fatal(res.Error)
	}
	want := []string{srv.URL + "/b", srv.URL + "/c"}
	if len(res.RedirectChain) != 2 || res.RedirectChain[0] != want[0] || res.RedirectChain[1] != want[1] {
		t.Errorf("chain = %v, want %v", res.RedirectChain, want)
	}
	if res.FinalURL != srv.URL+"/c" {
		t.Errorf("final url = %q", res.FinalURL)
	}
}

func TestFetchStopsRedirectLoops(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, r.URL.Path, http.StatusFound) })
	res := mustNew(t, Options{}).Fetch(context.Background(), srv.URL+"/loop", types.Profile{Name: "x", UserAgent: "x"})
	if res.Error == "" || len(res.RedirectChain) != maxRedirects {
		t.Errorf("expected redirect error with %d hops, got err=%q hops=%d", maxRedirects, res.Error, len(res.RedirectChain))
	}
}

func TestFetchTruncatesLargeBodies(t *testing.T) {
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 5000)))
	})
	res := mustNew(t, Options{MaxBody: 1000}).Fetch(context.Background(), srv.URL, types.Profile{Name: "x", UserAgent: "x"})
	if !res.Truncated || res.BodySize != 1000 || len(res.Body) != 1000 {
		t.Errorf("truncated=%v size=%d body=%d", res.Truncated, res.BodySize, len(res.Body))
	}
}

func TestFetchRetriesGatewayErrorsOnly(t *testing.T) {
	var calls int32
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})
	res := mustNew(t, Options{Retries: 3}).Fetch(context.Background(), srv.URL, types.Profile{Name: "x", UserAgent: "x"})
	if res.StatusCode != http.StatusOK || calls != 3 {
		t.Errorf("status=%d calls=%d, want 200 after 3 calls", res.StatusCode, calls)
	}

	calls = 0
	forbidden := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusForbidden)
	})
	res = mustNew(t, Options{Retries: 3}).Fetch(context.Background(), forbidden.URL, types.Profile{Name: "x", UserAgent: "x"})
	if res.StatusCode != http.StatusForbidden || calls != 1 {
		t.Errorf("403 must not be retried: status=%d calls=%d", res.StatusCode, calls)
	}
}
