// Package tap is a local reverse proxy that records the requests AI coding
// CLIs (Claude Code, Codex, Gemini CLI, Grok) send to their model providers.
//
// Each CLI is pointed at the proxy through its own base URL environment
// variable, so no CA certificate is needed: the CLI talks plain HTTP to
// 127.0.0.1 and the proxy talks HTTPS to the real provider.
package tap

import "strings"

// Route forwards requests under Prefix to Upstream with the prefix removed.
type Route struct {
	Name     string // shown in captures, e.g. "anthropic"
	Prefix   string // path prefix on the proxy, e.g. "/anthropic"
	Upstream string // scheme and host, e.g. "https://api.anthropic.com"
}

// Binding is one environment variable that points a CLI at a route.
type Binding struct {
	Env    string // e.g. "ANTHROPIC_BASE_URL"
	Route  string // Route.Name
	Suffix string // appended to the route URL, e.g. "/v1" for OpenAI-style bases
}

// CLI is an AI command line tool acurl knows how to tap.
type CLI struct {
	Name     string // the command, e.g. "claude"
	Bindings []Binding
	Note     string // limits worth knowing, shown by `acurl tap env --list`
}

// DefaultRoutes are the provider APIs the proxy forwards to.
var DefaultRoutes = []Route{
	{Name: "anthropic", Prefix: "/anthropic", Upstream: "https://api.anthropic.com"},
	{Name: "openai", Prefix: "/openai", Upstream: "https://api.openai.com"},
	{Name: "gemini", Prefix: "/gemini", Upstream: "https://generativelanguage.googleapis.com"},
	{Name: "code-assist", Prefix: "/code-assist", Upstream: "https://cloudcode-pa.googleapis.com"},
	{Name: "grok", Prefix: "/grok", Upstream: "https://cli-chat-proxy.grok.com"},
}

// CLIs are the tools `acurl tap env` writes wrappers for.
var CLIs = []CLI{
	{
		Name:     "claude",
		Bindings: []Binding{{Env: "ANTHROPIC_BASE_URL", Route: "anthropic"}},
		Note:     "API key and claude.ai login both work",
	},
	{
		Name:     "codex",
		Bindings: []Binding{{Env: "OPENAI_BASE_URL", Route: "openai", Suffix: "/v1"}},
		Note:     "API key login only; ChatGPT login talks to chatgpt.com and bypasses the base URL",
	},
	{
		Name: "gemini",
		Bindings: []Binding{
			{Env: "GOOGLE_GEMINI_BASE_URL", Route: "gemini"},
			{Env: "CODE_ASSIST_ENDPOINT", Route: "code-assist"},
		},
		Note: "Gemini API key and Google login; Vertex AI is not routed",
	},
	{
		Name:     "grok",
		Bindings: []Binding{{Env: "GROK_CLI_CHAT_PROXY_BASE_URL", Route: "grok", Suffix: "/v1"}},
		Note:     "Grok Build CLI with grok.com login",
	},
}

// FindCLI returns the CLI with name, ignoring case.
func FindCLI(name string) (CLI, bool) {
	for _, c := range CLIs {
		if strings.EqualFold(c.Name, name) {
			return c, true
		}
	}
	return CLI{}, false
}

// CLINames lists the known CLI names.
func CLINames() []string {
	names := make([]string, len(CLIs))
	for i, c := range CLIs {
		names[i] = c.Name
	}
	return names
}

func findRoute(routes []Route, name string) (Route, bool) {
	for _, r := range routes {
		if r.Name == name {
			return r, true
		}
	}
	return Route{}, false
}
