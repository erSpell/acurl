package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tragent-io/acurl/internal/output"
	"github.com/tragent-io/acurl/pkg/types"
)

// shop blocks ChatGPT-User and disallows PerplexityBot in robots.txt.
func shop(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			_, _ = w.Write([]byte("User-agent: PerplexityBot\nDisallow: /\n"))
			return
		}
		if strings.Contains(r.UserAgent(), "ChatGPT-User") {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("<html><title>Forbidden</title></html>"))
			return
		}
		_, _ = w.Write([]byte("<html><head><title>Shop</title></head><body><p>Red shoes in every size, shipped free worldwide.</p></body></html>"))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = Execute(context.Background(), BuildInfo{Version: "test"}, args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestCheckJSONReport(t *testing.T) {
	srv := shop(t)
	code, stdout, stderr := run(t, srv.URL, "-f", "json", "-a", "Chrome (Browser),ChatGPT-User,PerplexityBot,Googlebot")

	if code != output.ExitBlocked {
		t.Errorf("exit code = %d, want %d (stderr: %s)", code, output.ExitBlocked, stderr)
	}
	var rep types.ComparisonResult
	if err := json.Unmarshal([]byte(stdout), &rep); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	got := map[string]string{}
	for _, r := range rep.Results {
		got[r.Profile] = r.Classification
	}
	want := map[string]string{
		"Chrome (Browser)": "allowed",
		"ChatGPT-User":     "blocked",
		"PerplexityBot":    "robots-disallowed",
		"Googlebot":        "allowed",
	}
	for p, c := range want {
		if got[p] != c {
			t.Errorf("%s = %q, want %q", p, got[p], c)
		}
	}
}

func TestCheckAllAllowedExitsZero(t *testing.T) {
	srv := shop(t)
	code, _, stderr := run(t, srv.URL, "-a", "Googlebot", "--no-color")
	if code != output.ExitOK {
		t.Errorf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
}

func TestCheckBadInput(t *testing.T) {
	if code, _, _ := run(t, "ftp://example.com"); code != output.ExitError {
		t.Errorf("bad scheme exit code = %d", code)
	}
	if code, _, stderr := run(t, "https://example.com", "-a", "NoSuchBot"); code != output.ExitError || !strings.Contains(stderr, "no profiles match") {
		t.Errorf("unknown agent: code=%d stderr=%q", code, stderr)
	}
}
