package tap

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestWriteEnv(t *testing.T) {
	claude, _ := FindCLI("claude")
	gemini, _ := FindCLI("Gemini")
	tests := []struct {
		shell string
		want  []string
	}{
		{"zsh", []string{
			`claude() { if curl -fsS -m 1 'http://127.0.0.1:8899/_acurl/ping' >/dev/null 2>&1; then ANTHROPIC_BASE_URL='http://127.0.0.1:8899/anthropic' command claude "$@"; else command claude "$@"; fi; }`,
			`GOOGLE_GEMINI_BASE_URL='http://127.0.0.1:8899/gemini' CODE_ASSIST_ENDPOINT='http://127.0.0.1:8899/code-assist' command gemini`,
		}},
		{"fish", []string{"function claude\n", "env ANTHROPIC_BASE_URL='http://127.0.0.1:8899/anthropic' claude $argv\n", "command claude $argv\n"}},
		{"powershell", []string{"function claude {", "$env:ANTHROPIC_BASE_URL = 'http://127.0.0.1:8899/anthropic'", "$old0 = $env:ANTHROPIC_BASE_URL", "finally { $env:ANTHROPIC_BASE_URL = $old0 }"}},
	}
	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			var b bytes.Buffer
			if err := WriteEnv(&b, tt.shell, "http://127.0.0.1:8899/", []CLI{claude, gemini}); err != nil {
				t.Fatal(err)
			}
			for _, w := range tt.want {
				if !strings.Contains(b.String(), w) {
					t.Errorf("output lacks %q:\n%s", w, b.String())
				}
			}
		})
	}
	if err := WriteEnv(&bytes.Buffer{}, "tcsh", "http://127.0.0.1:8899", CLIs); err == nil {
		t.Error("unsupported shell accepted")
	}
}

func TestEveryCLIBindingHasARoute(t *testing.T) {
	for _, c := range CLIs {
		for _, b := range c.Bindings {
			if _, ok := findRoute(DefaultRoutes, b.Route); !ok {
				t.Errorf("%s: %s uses unknown route %q", c.Name, b.Env, b.Route)
			}
		}
	}
}

func TestWriteLine(t *testing.T) {
	x := &Exchange{
		ID: 4, Start: time.Date(2026, 10, 21, 12, 1, 3, 0, time.UTC), Duration: 1234 * time.Millisecond,
		Route: "anthropic", Method: "POST", URL: "https://api.anthropic.com/v1/messages?beta=true",
		Model: "claude-opus-5-5", Status: 200, Streaming: true,
		ReqBody: Body{Size: 41000}, RespBody: Body{Size: 3100},
	}
	var b bytes.Buffer
	if err := WriteLine(&b, x); err != nil {
		t.Fatal(err)
	}
	want := "12:01:03 #4 anthropic POST /v1/messages?beta=true claude-opus-5-5 200 1.234s 41.0KB > 3.1KB stream\n"
	if b.String() != want {
		t.Errorf("got  %q\nwant %q", b.String(), want)
	}
}
