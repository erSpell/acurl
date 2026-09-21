package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestAnalyzeReactShell(t *testing.T) {
	info := Analyze(fixture(t, "spa-react-shell.html"))
	if info == nil || !info.ClientRendered {
		t.Fatalf("expected client-rendered shell, got %+v", info)
	}
	if info.Framework != "React" {
		t.Errorf("framework = %q, want React", info.Framework)
	}
	for _, want := range []string{"enable JavaScript", "empty app root"} {
		if !contains(info.Reason, want) {
			t.Errorf("reason %q missing %q", info.Reason, want)
		}
	}
}

func TestAnalyzeServerRendered(t *testing.T) {
	info := Analyze(fixture(t, "ssr-product.html"))
	if info == nil || info.ClientRendered {
		t.Fatalf("server-rendered page flagged as client-rendered: %+v", info)
	}
	if info.Framework != "Next.js" || info.TextLength < minText {
		t.Errorf("unexpected analysis: %+v", info)
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	if Analyze(nil) != nil {
		t.Error("empty body should return nil")
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func TestAnalyzeSmallStaticPageWithScript(t *testing.T) {
	// example.com: ~170 chars of text and one small external script.
	if info := Analyze(fixture(t, "static-small.html")); info == nil || info.ClientRendered {
		t.Errorf("small static page flagged as client-rendered: %+v", info)
	}
}

func TestAnalyzeHeavyScriptNoText(t *testing.T) {
	body := []byte("<html><body><script>" + strings.Repeat("x=1;", 6000) + "</script></body></html>")
	if info := Analyze(body); info == nil || !info.ClientRendered {
		t.Errorf("script-only page not flagged: %+v", info)
	}
}
