package robots

import (
	"context"
	"testing"
)

func FuzzAllowed(f *testing.F) {
	f.Add("User-agent: *\nDisallow: /private\n", "GPTBot", "/private/x")
	f.Add("User-agent: GPTBot\nDisallow: /\n\nUser-agent: *\nAllow: /", "gptbot", "/")
	f.Add("\x00\xff garbage", "", "")
	f.Fuzz(func(t *testing.T, content, agent, path string) {
		d := NewCache(func(context.Context, string) (int, []byte, error) {
			return 200, []byte(content), nil
		}).Get(context.Background(), "https://fuzz.example/")
		// Must never panic, whatever the input.
		_ = d.Allowed(agent, "https://fuzz.example/"+path)
	})
}
