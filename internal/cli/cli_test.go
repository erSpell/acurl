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

func TestCheckRobotsOnlyTokens(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			_, _ = w.Write([]byte("User-agent: Google-Extended\nDisallow: /\n"))
			return
		}
		if r.UserAgent() == "" || strings.Contains(r.UserAgent(), "Google-Extended") {
			t.Errorf("robots-only token was fetched with UA %q", r.UserAgent())
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	_, stdout, _ := run(t, srv.URL, "-f", "json", "-a", "Google-Extended,Applebot-Extended,Googlebot")
	var rep types.ComparisonResult
	if err := json.Unmarshal([]byte(stdout), &rep); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, r := range rep.Results {
		got[r.Profile] = r.Classification
	}
	if got["Google-Extended"] != "robots-disallowed" || got["Applebot-Extended"] != "allowed" || got["Googlebot"] != "allowed" {
		t.Errorf("classifications = %v", got)
	}
}

func TestProfilesList(t *testing.T) {
	code, stdout, stderr := run(t, "profiles", "list", "--category", "ai-assistant")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	for _, want := range []string{"ChatGPT-User", "Claude-User", "baseline"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("list output missing %q", want)
		}
	}
	if strings.Contains(stdout, "GPTBot") {
		t.Error("category filter did not exclude GPTBot")
	}
}

func TestProfilesShow(t *testing.T) {
	code, stdout, _ := run(t, "profiles", "show", "gptbot")
	if code != 0 || !strings.Contains(stdout, "robots_key: GPTBot") {
		t.Errorf("code=%d output=%s", code, stdout)
	}
	if code, _, _ := run(t, "profiles", "show", "nobody"); code != output.ExitError {
		t.Errorf("unknown profile exit code = %d", code)
	}
}

func TestRobotsAudit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			_, _ = w.Write([]byte("User-agent: *\nContent-Signal: search=yes, ai-train=no\nAllow: /\n\nUser-agent: GPTBot\nDisallow: /\n\nSitemap: https://shop.example/sitemap.xml\n"))
		case "/llms.txt":
			_, _ = w.Write([]byte("# Shop\n\n> Shoes.\n\n## Docs\n\n- [Runners](/runners.md)\n"))
		case "/products/1":
			t.Error("robots audit must not fetch the page itself")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	code, stdout, stderr := run(t, "robots", srv.URL+"/products/1", "-f", "json", "-a", "GPTBot,ClaudeBot,Google-Extended")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	var a types.RobotsAudit
	if err := json.Unmarshal([]byte(stdout), &a); err != nil {
		t.Fatal(err)
	}
	verdict := map[string]bool{}
	for _, ag := range a.Agents {
		verdict[ag.Profile] = ag.Allowed
		if ag.Profile == "ClaudeBot" && ag.Signals["ai-train"] != "no" {
			t.Errorf("ClaudeBot signals = %v", ag.Signals)
		}
	}
	if verdict["GPTBot"] || !verdict["ClaudeBot"] || !verdict["Google-Extended"] {
		t.Errorf("verdicts = %v", verdict)
	}
	if len(a.Sitemaps) != 1 || len(a.LLMsTxt) != 2 || !a.LLMsTxt[0].Found || a.LLMsTxt[0].Links != 1 || a.LLMsTxt[1].Found {
		t.Errorf("sitemaps=%v llms=%+v", a.Sitemaps, a.LLMsTxt)
	}
}

func TestRobotsAuditUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	code, stdout, _ := run(t, "robots", srv.URL, "--no-color", "-a", "GPTBot")
	if code != output.ExitError || !strings.Contains(stdout, "full disallow") {
		t.Errorf("code=%d output=%s", code, stdout)
	}
}
