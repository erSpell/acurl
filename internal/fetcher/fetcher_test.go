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

func TestFetchSendsProfileHeaders(t *testing.T) {
	var gotUA, gotCustom, gotExtra string
	srv := newServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotCustom = r.Header.Get("X-Profile")
		gotExtra = r.Header.Get("X-Extra")
		_, _ = w.Write([]byte("<html><head><title> Hello &amp; welcome </title></head></html>"))
	})

	f := New(Options{Timeout: 5 * time.Second, Headers: map[string]string{"X-Extra": "yes"}})
	p := types.Profile{Name: "Test", UserAgent: "TestBot/1.0", Headers: map[string]string{"X-Profile": "p"}}
	res := f.Fetch(context.Background(), srv.URL, p)

	if res.Error != "" {
		t.Fatalf("unexpected error: %s", res.Error)
	}
	if gotUA != "TestBot/1.0" || gotCustom != "p" || gotExtra != "yes" {
		t.Errorf("headers not sent: ua=%q profile=%q extra=%q", gotUA, gotCustom, gotExtra)
	}
	if res.StatusCode != 200 || !res.Success {
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
	res := New(Options{}).Fetch(context.Background(), srv.URL, types.Profile{Name: "x", UserAgent: "x"})
	if !res.RateLimited || res.RetryAfter != "120" {
		t.Errorf("rate limit not detected: %+v", res)
	}
}

func TestFetchNetworkError(t *testing.T) {
	res := New(Options{Timeout: time.Second}).Fetch(context.Background(), "http://127.0.0.1:1", types.Profile{Name: "x", UserAgent: "x"})
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
	res := New(Options{Timeout: 100 * time.Millisecond}).Fetch(context.Background(), srv.URL, types.Profile{Name: "x", UserAgent: "x"})
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
	results := New(Options{}).FetchAll(context.Background(), srv.URL, ps, 2)

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
