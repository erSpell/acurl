package server

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/tragent-io/acurl/pkg/acurl"
	"github.com/tragent-io/acurl/pkg/types"
)

func target(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.UserAgent(), "ChatGPT-User") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte("<html><body><p>Shoes for everyone.</p></body></html>"))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func api(t *testing.T, blockPrivate bool) *httptest.Server {
	t.Helper()
	s, err := New(Options{Check: acurl.Options{BlockPrivate: blockPrivate, SkipRobots: true}, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	return srv
}

func TestUIIsServed(t *testing.T) {
	srv := api(t, true)
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "$ acurl") || resp.Header.Get("Content-Security-Policy") == "" {
		t.Errorf("status=%d csp=%q", resp.StatusCode, resp.Header.Get("Content-Security-Policy"))
	}
}

func TestCheckAPI(t *testing.T) {
	srv, site := api(t, false), target(t)
	body := `{"url":"` + site.URL + `","agents":["Chrome (Browser)","ChatGPT-User","Googlebot"]}`
	resp, err := http.Post(srv.URL+"/api/check", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var rep types.ComparisonResult
	if err := json.NewDecoder(resp.Body).Decode(&rep); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || len(rep.Results) != 3 || rep.Results[1].Classification != "blocked" {
		t.Errorf("status=%d results=%+v", resp.StatusCode, rep.Results)
	}
}

func TestCheckAPIValidation(t *testing.T) {
	srv := api(t, true)
	for body, want := range map[string]string{
		`{"url":""}`:        "enter a URL",
		`{"url":"ftp://x"}`: "not an http",
		`not json`:          "invalid JSON",
		`{"url":"https://x.example","agents":["nope"]}`: "no profiles",
	} {
		resp, err := http.Post(srv.URL+"/api/check", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		var e apiError
		_ = json.NewDecoder(resp.Body).Decode(&e)
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest || !strings.Contains(e.Error, want) {
			t.Errorf("%s: status=%d error=%q", body, resp.StatusCode, e.Error)
		}
	}
}

func TestSSRFBlockedByDefault(t *testing.T) {
	srv, site := api(t, true), target(t)
	resp, err := http.Post(srv.URL+"/api/check", "application/json", strings.NewReader(`{"url":"`+site.URL+`","agents":["Googlebot"]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var rep types.ComparisonResult
	_ = json.NewDecoder(resp.Body).Decode(&rep)
	if len(rep.Results) != 1 || !strings.Contains(rep.Results[0].Error, "private or internal") {
		t.Errorf("internal target was fetched: %+v", rep.Results)
	}
}

func TestStream(t *testing.T) {
	srv, site := api(t, false), target(t)
	resp, err := http.Get(srv.URL + "/api/check/stream?" + url.Values{"url": {site.URL}, "agents": {"Chrome (Browser),Googlebot"}}.Encode())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	events := map[string]int{}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64<<10), 4<<20)
	for sc.Scan() {
		if name, ok := strings.CutPrefix(sc.Text(), "event: "); ok {
			events[name]++
		}
	}
	if events["start"] != 1 || events["progress"] != 2 || events["report"] != 1 {
		t.Errorf("events = %v", events)
	}
}

func TestReportFormats(t *testing.T) {
	srv := api(t, true)
	rep := `{"url":"https://x.example","results":[{"profile":"GPTBot","classification":"allowed","status_code":200}]}`
	resp, err := http.Post(srv.URL+"/api/report?format=html", "application/json", strings.NewReader(rep))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") || !strings.Contains(string(body), "GPTBot") {
		t.Errorf("html report: %s", resp.Header.Get("Content-Type"))
	}
	resp, _ = http.Post(srv.URL+"/api/report?format=exe", "application/json", strings.NewReader(rep))
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("unknown format status = %d", resp.StatusCode)
	}
}

func TestRejectsForeignHostOriginAndContentType(t *testing.T) {
	srv := api(t, true)
	do := func(method, path, host, origin, ctype string) int {
		req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(`{"url":"example.com"}`))
		if host != "" {
			req.Host = host
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if ctype != "" {
			req.Header.Set("Content-Type", ctype)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	tests := []struct {
		name                       string
		method, path, host, origin string
		ctype                      string
		want                       int
	}{
		{"rebinding host on UI", "GET", "/", "rebind.attacker.example", "", "", http.StatusMisdirectedRequest},
		{"rebinding host on API", "POST", "/api/check", "attacker.example", "", "application/json", http.StatusMisdirectedRequest},
		{"cross-site origin", "POST", "/api/check", "", "https://attacker.example", "application/json", http.StatusForbidden},
		{"form post", "POST", "/api/check", "", "", "text/plain", http.StatusUnsupportedMediaType},
		{"localhost name", "GET", "/api/version", "localhost:8080", "http://localhost:8080", "", http.StatusOK},
	}
	for _, tt := range tests {
		if got := do(tt.method, tt.path, tt.host, tt.origin, tt.ctype); got != tt.want {
			t.Errorf("%s: status %d, want %d", tt.name, got, tt.want)
		}
	}
}
