package render

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

const jsPage = `<!doctype html><html><head><title>JS shop</title></head><body>
<div id="root"></div>
<script>
document.getElementById('root').innerHTML =
  '<h1>Wool Runner</h1><p>' + 'Comfortable merino sneakers with free shipping and returns. '.repeat(10) + '</p>';
</script></body></html>`

func TestCompare(t *testing.T) {
	raw := []byte(`<html><body><div id="root"></div></body></html>`)
	rendered := []byte(`<html><body><div id="root"><h1>Wool Runner</h1><p>` + strings.Repeat("Comfortable merino sneakers. ", 10) + `</p></div></body></html>`)
	r := Compare(raw, rendered)
	if r.RawTextLength != 0 || r.RenderedTextLength == 0 || r.JSOnlyShare != 1 {
		t.Errorf("Compare = %+v", r)
	}
	same := Compare(rendered, rendered)
	if same.JSOnlyShare != 0 || same.Similarity != 1 {
		t.Errorf("identical pages: %+v", same)
	}
}

// TestHeadless needs a local Chrome. CI runs it in a dedicated job.
func TestHeadless(t *testing.T) {
	if os.Getenv("ACURL_TEST_RENDER") == "" {
		t.Skip("set ACURL_TEST_RENDER=1 to run headless Chrome tests")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(jsPage))
	}))
	defer srv.Close()

	html, err := Headless(context.Background(), srv.URL, Options{Timeout: 30 * time.Second, Wait: 300 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	r := Compare([]byte(jsPage), html)
	if r.JSOnlyShare < 0.8 {
		t.Errorf("expected most text to need JS, got %+v", r)
	}
	if info := Analyze([]byte(jsPage)); info == nil || !info.ClientRendered {
		t.Errorf("Analyze should flag the raw page: %+v", info)
	}
}
