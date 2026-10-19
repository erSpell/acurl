package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tragent-io/acurl/internal/textdiff"
	"github.com/tragent-io/acurl/pkg/types"
)

func batchReport() *types.BatchReport {
	ok := goldenReport()
	return &types.BatchReport{
		Items: []types.BatchItem{
			{URL: ok.URL, Report: &ok},
			{URL: "not a url", Error: "invalid url"},
		},
		Summary: types.BatchSummary{URLs: 2, Failed: 1, Scored: 1, AverageScore: 58},
	}
}

func TestWriteBatchFormats(t *testing.T) {
	for _, format := range []string{"table", "json", "markdown", "csv", "junit", "sarif", "html"} {
		var buf bytes.Buffer
		if err := WriteBatch(&buf, format, batchReport(), false); err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		if buf.Len() == 0 {
			t.Errorf("%s: empty output", format)
		}
	}
	var buf bytes.Buffer
	_ = WriteBatch(&buf, "markdown", batchReport(), false)
	if !strings.Contains(buf.String(), "| not a url | error |") || !strings.Contains(buf.String(), "58/100") {
		t.Errorf("markdown batch:\n%s", buf.String())
	}
	if err := WriteBatch(&buf, "xml", batchReport(), false); err == nil {
		t.Error("unknown format should fail")
	}
}

func TestBatchExitCodeAndReasons(t *testing.T) {
	p, _ := ParsePolicy("challenged,score<60")
	if got := BatchExitCode(batchReport(), p); got != ExitError {
		t.Errorf("a failed item should exit 2, got %d", got)
	}
	rep := goldenReport()
	if r := p.Reasons(&rep); len(r) != 2 || r[0] != "ChatGPT-User challenged" || r[1] != "score 58 < 60" {
		t.Errorf("Reasons = %v", r)
	}
}

func TestWriteDiff(t *testing.T) {
	var buf bytes.Buffer
	lines := textdiff.Lines("a\nb\nprice 98", "a\nb\nprice 89", 1)
	if err := WriteDiff(&buf, "Chrome", "GPTBot", lines, false); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "- price 98") || !strings.Contains(out, "+ price 89") || !strings.Contains(out, "--- Chrome") {
		t.Errorf("diff:\n%s", out)
	}
	buf.Reset()
	_ = WriteDiff(&buf, "Chrome", "GPTBot", textdiff.Lines("same", "same", 1), false)
	if !strings.Contains(buf.String(), "identical") {
		t.Errorf("identical diff: %s", buf.String())
	}
}

func TestWriteRobotsAudit(t *testing.T) {
	a := types.RobotsAudit{
		URL: "https://shop.example/", RobotsURL: "https://shop.example/robots.txt", Status: 200,
		Sitemaps: []string{"https://shop.example/sitemap.xml"},
		Agents: []types.RobotsAgent{
			{Profile: "GPTBot", Token: "GPTBot", Allowed: false, Signals: map[string]string{"ai-train": "no", "search": "yes", "x-custom": "1"}},
			{Profile: "ClaudeBot", Token: "ClaudeBot", Allowed: true},
		},
		LLMsTxt: []types.LLMsTxt{
			{URL: "https://shop.example/llms.txt", Found: true, Links: 3, Sections: 1, Problems: []string{"no summary"}},
			{URL: "https://shop.example/llms-full.txt", Status: 404},
		},
	}
	var buf bytes.Buffer
	if err := WriteRobotsAudit(&buf, "table", a, false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"search=yes ai-train=no x-custom=1", "llms.txt: found, 3 links in 1 sections", "! no summary", "llms-full.txt: not found (404)", "sitemap.xml"} {
		if !strings.Contains(out, want) {
			t.Errorf("robots table missing %q:\n%s", want, out)
		}
	}
	for _, tc := range []struct {
		a    types.RobotsAudit
		want string
	}{
		{types.RobotsAudit{RobotsURL: "u", Error: "robots.txt returned 503"}, "full disallow"},
		{types.RobotsAudit{RobotsURL: "u", Status: 404}, "every agent is allowed"},
	} {
		buf.Reset()
		_ = WriteRobotsAudit(&buf, "table", tc.a, false)
		if !strings.Contains(buf.String(), tc.want) {
			t.Errorf("robots audit missing %q", tc.want)
		}
	}
	buf.Reset()
	if err := WriteRobotsAudit(&buf, "json", a, false); err != nil || !json.Valid(buf.Bytes()) {
		t.Errorf("json audit: %v", err)
	}
	if err := WriteRobotsAudit(&buf, "csv", a, false); err == nil {
		t.Error("unsupported robots format should fail")
	}
}

func TestDetailsCoverEveryField(t *testing.T) {
	sim := 0.4
	r := types.FetchResult{
		Profile: "x", Classification: "cloaked", StatusCode: 200, Similarity: &sim, Signed: true,
		RedirectChain: []string{"https://a.example/1"}, FinalURL: "https://a.example/a-very-long-final-destination-path/that-gets-cut",
		JS: &types.JSInfo{ClientRendered: true, Framework: "Next.js"}, Flaky: true, SampleVerdicts: []string{"allowed 1/2", "cloaked 1/2"},
		RateLimited: true, RetryAfterSeconds: 90, Directives: []string{"noindex", "max-snippet:5"},
	}
	d := details(r)
	for _, want := range []string{"signed", "flaky: allowed 1/2, cloaked 1/2", "40% similar", "JS-only page (Next.js)", "noindex", "retry in 1m30s", "1 redirect(s) -> a.example/"} {
		if !strings.Contains(d, want) {
			t.Errorf("details %q missing %q", d, want)
		}
	}
	if d := details(types.FetchResult{StatusCode: 403, Edge: "CloudFront", Classification: "blocked"}); d != "HTTP 403 from CloudFront" {
		t.Errorf("blocked details = %q", d)
	}
}

func TestHTMLIncludesDiffs(t *testing.T) {
	sim := 0.2
	rep := goldenReport()
	rep.Results[0].Body = []byte("<html><body><p>Wool runner shoes, free shipping and returns.</p></body></html>")
	rep.Results[2].Body = []byte("<html><body><p>Welcome.</p></body></html>")
	rep.Results[2].Similarity = &sim
	var buf bytes.Buffer
	if err := WriteHTML(&buf, []*types.ComparisonResult{&rep}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "What agents saw differently") || !strings.Contains(buf.String(), "OAI-SearchBot vs Chrome (Browser)") {
		t.Error("HTML report should include the diff section")
	}
	for _, g := range []string{"A", "C", "F"} {
		_ = gradeStyle(g)
	}
}
