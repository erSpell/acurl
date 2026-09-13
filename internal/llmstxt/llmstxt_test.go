package llmstxt

import (
	"context"
	"strings"
	"testing"
)

const good = `# Example Store

> Sustainable running shoes, shipped worldwide.

## Products

- [Runners](https://shop.example/runners.md): everyday trainers
- [Trail](https://shop.example/trail.md)

## Optional

- [About](https://shop.example/about.md)
`

func getter(files map[string]string) Getter {
	return func(_ context.Context, u string) (int, []byte, error) {
		for path, body := range files {
			if strings.HasSuffix(u, path) {
				return 200, []byte(body), nil
			}
		}
		return 404, nil, nil
	}
}

func TestCheckValidFile(t *testing.T) {
	res, err := Check(context.Background(), getter(map[string]string{"/llms.txt": good}), "https://shop.example/products/1")
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res))
	}
	l := res[0]
	if !l.Found || l.URL != "https://shop.example/llms.txt" || l.Title != "Example Store" || l.Links != 3 || l.Sections != 2 || len(l.Problems) != 0 {
		t.Errorf("unexpected result: %+v", l)
	}
	if res[1].Found || res[1].Status != 404 {
		t.Errorf("llms-full.txt should be missing: %+v", res[1])
	}
}

func TestValidateProblems(t *testing.T) {
	tests := map[string]string{
		"<!DOCTYPE html><html><body>Not found</body></html>": "HTML",
		"Example Store\n\n- [a](b)":                          "H1",
		"# Store\n\nSome text\n":                             "summary",
		"   ":                                                "empty",
	}
	for body, want := range tests {
		res, _ := Check(context.Background(), getter(map[string]string{"/llms.txt": body}), "https://x.example")
		if !strings.Contains(strings.Join(res[0].Problems, ";"), want) {
			t.Errorf("body %q: problems %v, want one mentioning %q", body, res[0].Problems, want)
		}
	}
}
