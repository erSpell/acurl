package robots

import (
	"context"
	"reflect"
	"testing"
)

const signalsRobots = `# Cloudflare-style managed robots.txt
User-agent: *
Content-Signal: search=yes, ai-train=no
Allow: /

User-agent: GPTBot
User-agent: CCBot
Content-signal: search=no , AI-Train=no, ai-input=no
Disallow: /

Sitemap: https://shop.example/sitemap.xml
User-agent: Googlebot
Allow: /
Sitemap: https://shop.example/products.xml
`

func data(t *testing.T, content string) *Data {
	t.Helper()
	return NewCache(staticGetter(200, content, nil)).Get(context.Background(), "https://shop.example/")
}

func TestContentSignals(t *testing.T) {
	d := data(t, signalsRobots)
	tests := []struct {
		agent string
		want  map[string]string
	}{
		{"GPTBot", map[string]string{"search": "no", "ai-train": "no", "ai-input": "no"}},
		{"ccbot", map[string]string{"search": "no", "ai-train": "no", "ai-input": "no"}},
		{"ClaudeBot", map[string]string{"search": "yes", "ai-train": "no"}},
		{"Googlebot", nil},
	}
	for _, tt := range tests {
		if got := d.ContentSignals(tt.agent); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("ContentSignals(%s) = %v, want %v", tt.agent, got, tt.want)
		}
	}
}

func TestSitemaps(t *testing.T) {
	got := data(t, signalsRobots).Sitemaps()
	want := []string{"https://shop.example/sitemap.xml", "https://shop.example/products.xml"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Sitemaps = %v, want %v", got, want)
	}
}
