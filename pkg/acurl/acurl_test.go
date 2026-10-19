package acurl_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tragent-io/acurl/pkg/acurl"
	"github.com/tragent-io/acurl/pkg/types"
)

func store() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			_, _ = w.Write([]byte("User-agent: GPTBot\nDisallow: /\n"))
			return
		}
		if strings.Contains(r.UserAgent(), "PerplexityBot") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte(`<html><head><title>Runner</title>
<script type="application/ld+json">{"@type":"Product","name":"Runner","offers":{"price":"98","priceCurrency":"USD","availability":"https://schema.org/InStock"}}</script>
</head><body><main><h1>Runner</h1><p>Merino sneakers with free shipping and returns, in sizes 8 to 14.</p></main></body></html>`))
	}))
}

func profiles(names ...string) []types.Profile {
	all := map[string]types.Profile{
		"Chrome":        {Name: "Chrome", UserAgent: "Mozilla/5.0 Chrome/120", Browser: true, Category: types.CategoryBrowser},
		"ChatGPT-User":  {Name: "ChatGPT-User", UserAgent: "ChatGPT-User/1.0", RobotsKey: "ChatGPT-User", Category: types.CategoryAIAssistant},
		"PerplexityBot": {Name: "PerplexityBot", UserAgent: "PerplexityBot/1.0", RobotsKey: "PerplexityBot", Category: types.CategoryAISearch},
		"GPTBot":        {Name: "GPTBot", UserAgent: "GPTBot/1.1", RobotsKey: "GPTBot", Category: types.CategoryAITraining},
	}
	var out []types.Profile
	for _, n := range names {
		out = append(out, all[n])
	}
	return out
}

func TestCheck(t *testing.T) {
	srv := store()
	defer srv.Close()

	c, err := acurl.New(acurl.Options{Profiles: profiles("Chrome", "ChatGPT-User", "PerplexityBot", "GPTBot")})
	if err != nil {
		t.Fatal(err)
	}
	rep, err := c.Check(context.Background(), srv.URL+"/products/runner")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, r := range rep.Results {
		got[r.Profile] = r.Classification
	}
	want := map[string]string{"Chrome": "allowed", "ChatGPT-User": "allowed", "PerplexityBot": "blocked", "GPTBot": "robots-disallowed"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	if rep.Score == nil || rep.Score.Value >= 100 || rep.Score.Value <= 0 {
		t.Errorf("unexpected score %+v", rep.Score)
	}
	if rep.Results[0].Product == nil || rep.Results[0].Product.Price != "98" {
		t.Errorf("baseline product = %+v", rep.Results[0].Product)
	}
}

func TestCheckRejectsBadURL(t *testing.T) {
	c, err := acurl.New(acurl.Options{Profiles: profiles("Chrome")})
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []string{"ftp://x", "not a url", "https://"} {
		if _, err := c.Check(context.Background(), u); err == nil {
			t.Errorf("Check(%q) should fail", u)
		}
	}
}

func ExampleChecker_Check() {
	srv := store()
	defer srv.Close()

	c, _ := acurl.New(acurl.Options{Profiles: profiles("Chrome", "ChatGPT-User")})
	rep, _ := c.Check(context.Background(), srv.URL+"/products/runner")
	for _, r := range rep.Results {
		fmt.Println(r.Profile, r.Classification)
	}
	// Output:
	// Chrome allowed
	// ChatGPT-User allowed
}

func TestSamplesMarkFlakyVerdicts(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ChatGPT-User is blocked on every other request.
		if strings.Contains(r.UserAgent(), "ChatGPT-User") && atomic.AddInt32(&n, 1)%2 == 0 {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte("<html><body><p>Stable page text for every agent here.</p></body></html>"))
	}))
	defer srv.Close()

	c, err := acurl.New(acurl.Options{Profiles: profiles("Chrome", "ChatGPT-User"), SkipRobots: true, Samples: 3, SampleDelay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	rep, err := c.Check(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	r := rep.Results[1]
	// Samples: allowed, blocked, allowed. Allowed wins 2/3 and the agent is flaky.
	if r.Classification != "allowed" || !r.Flaky || len(r.SampleVerdicts) != 2 || rep.Summary.Flaky != 1 {
		t.Errorf("merged = %q flaky=%v verdicts=%v summary=%+v", r.Classification, r.Flaky, r.SampleVerdicts, rep.Summary)
	}
	if rep.Results[0].Flaky {
		t.Error("the stable browser baseline should not be flaky")
	}
}

func TestPackageCheckAndAccessors(t *testing.T) {
	srv := store()
	defer srv.Close()
	rep, err := acurl.Check(context.Background(), srv.URL+"/products/runner", acurl.Options{Profiles: profiles("Chrome", "ChatGPT-User")})
	if err != nil || len(rep.Results) != 2 {
		t.Fatalf("Check: %v", err)
	}
	c, err := acurl.New(acurl.Options{Profiles: profiles("Chrome")})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Profiles()) != 1 {
		t.Error("Profiles() should return the configured profiles")
	}
	status, body, err := c.Get(context.Background(), srv.URL+"/robots.txt", 1024)
	if err != nil || status != 200 || !strings.Contains(string(body), "GPTBot") {
		t.Errorf("Get = %d %q %v", status, body, err)
	}
}

func TestOptionValidation(t *testing.T) {
	for _, o := range []acurl.Options{
		{Profiles: profiles("Chrome"), Page: "landing"},
		{Profiles: profiles("Chrome"), Samples: 11},
		{Profiles: []types.Profile{}},
	} {
		if _, err := acurl.New(o); err == nil {
			t.Errorf("New(%+v) should fail", o)
		}
	}
}

func TestRenderNeedsBaseline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) }))
	defer srv.Close()
	rep, err := acurl.Check(context.Background(), srv.URL, acurl.Options{Profiles: profiles("Chrome"), SkipRobots: true, Render: true})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Render == nil || !strings.Contains(rep.Render.Error, "browser baseline") {
		t.Errorf("render = %+v", rep.Render)
	}
}
