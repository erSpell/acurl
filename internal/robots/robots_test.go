package robots

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

func staticGetter(status int, body string, err error) Getter {
	return func(context.Context, string) (int, []byte, error) {
		return status, []byte(body), err
	}
}

func TestAllowedComplexFixture(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", "robots-complex.txt"))
	if err != nil {
		t.Fatal(err)
	}
	d := NewCache(staticGetter(200, string(b), nil)).Get(context.Background(), "https://shop.example")

	tests := []struct {
		agent, url string
		want       bool
	}{
		{"GPTBot", "https://shop.example/products/1", false},
		{"ChatGPT-User", "https://shop.example/", false},
		{"anthropic-ai", "https://shop.example/blog/post", false},
		{"anthropic-ai", "https://shop.example/products/1", true},
		{"Googlebot", "https://shop.example/admin/", true},
		{"SomeOtherBot", "https://shop.example/admin/", false},
		{"SomeOtherBot", "https://shop.example/products/1", true},
	}
	for _, tt := range tests {
		if got := d.Allowed(tt.agent, tt.url); got != tt.want {
			t.Errorf("Allowed(%s, %s) = %v, want %v", tt.agent, tt.url, got, tt.want)
		}
	}
}

func TestStatusHandling(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		err         error
		allowed     bool
		unreachable bool
	}{
		{"404 allows all", 404, nil, true, false},
		{"401 allows all", 401, nil, true, false},
		{"503 disallows all", 503, nil, false, true},
		{"network error disallows all", 0, errors.New("dial tcp: refused"), false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewCache(staticGetter(tt.status, "User-agent: *\nDisallow: /", tt.err)).Get(context.Background(), "https://x.example/page")
			if got := d.Allowed("AnyBot", "https://x.example/page"); got != tt.allowed {
				t.Errorf("Allowed = %v, want %v", got, tt.allowed)
			}
			if d.Unreachable() != tt.unreachable {
				t.Errorf("Unreachable = %v, want %v", d.Unreachable(), tt.unreachable)
			}
		})
	}
}

func TestCacheFetchesOncePerOrigin(t *testing.T) {
	var calls int32
	var seen sync.Map
	c := NewCache(func(_ context.Context, u string) (int, []byte, error) {
		atomic.AddInt32(&calls, 1)
		seen.Store(u, true)
		return 200, []byte("User-agent: *\nAllow: /"), nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Get(context.Background(), "https://a.example/x")
		}()
	}
	wg.Wait()
	c.Get(context.Background(), "https://b.example/y")

	if calls != 2 {
		t.Errorf("getter called %d times, want 2", calls)
	}
	if _, ok := seen.Load("https://a.example/robots.txt"); !ok {
		t.Error("expected robots.txt URL for a.example")
	}
}

func TestURLFor(t *testing.T) {
	got, err := URLFor("https://user@shop.example:8443/a/b?q=1")
	if err != nil || got != "https://shop.example:8443/robots.txt" {
		t.Errorf("URLFor = %q, %v", got, err)
	}
	if _, err := URLFor("not a url"); err == nil {
		t.Error("expected error for relative url")
	}
	d := NewCache(staticGetter(200, "", nil)).Get(context.Background(), "::bad")
	if !d.Unreachable() {
		t.Error("bad target should be reported as unreachable")
	}
}

func TestOversizedBodyIsTruncated(t *testing.T) {
	big := make([]byte, MaxSize+100)
	for i := range big {
		big[i] = '#'
	}
	d := NewCache(staticGetter(200, string(big), nil)).Get(context.Background(), "https://x.example")
	if len(d.Content) != MaxSize {
		t.Errorf("content length = %d, want %d", len(d.Content), MaxSize)
	}
}
