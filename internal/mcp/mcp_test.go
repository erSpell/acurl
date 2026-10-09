package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tragent-io/acurl/pkg/acurl"
)

// session runs the server over in-memory pipes and returns a function that
// sends a request and waits for the response with the same id.
func session(t *testing.T, base acurl.Options) func(id int, method string, params any) map[string]any {
	t.Helper()
	srv, err := New(base, "test")
	if err != nil {
		t.Fatal(err)
	}
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = srv.Serve(ctx, inR, outW); outW.Close() }()
	t.Cleanup(func() { cancel(); inW.Close() })

	lines := bufio.NewScanner(outR)
	lines.Buffer(make([]byte, 0, 64<<10), 16<<20)
	pending := map[float64]map[string]any{}
	return func(id int, method string, params any) map[string]any {
		msg := map[string]any{"jsonrpc": "2.0", "method": method}
		if id != 0 {
			msg["id"] = id
		}
		if params != nil {
			msg["params"] = params
		}
		b, _ := json.Marshal(msg)
		if _, err := inW.Write(append(b, '\n')); err != nil {
			t.Fatal(err)
		}
		if id == 0 {
			return nil
		}
		for {
			if r, ok := pending[float64(id)]; ok {
				return r
			}
			if !lines.Scan() {
				t.Fatalf("server closed before answering %d", id)
			}
			var resp map[string]any
			if err := json.Unmarshal(lines.Bytes(), &resp); err != nil {
				t.Fatalf("bad response %s: %v", lines.Bytes(), err)
			}
			pending[resp["id"].(float64)] = resp
		}
	}
}

func TestHandshakeAndToolList(t *testing.T) {
	call := session(t, acurl.Options{BlockPrivate: true})
	init := call(1, "initialize", map[string]any{"protocolVersion": "2025-03-26", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "t", "version": "1"}})
	res := init["result"].(map[string]any)
	if res["protocolVersion"] != "2025-03-26" || res["serverInfo"].(map[string]any)["name"] != "acurl" {
		t.Errorf("initialize = %v", res)
	}
	call(0, "notifications/initialized", nil)

	list := call(2, "tools/list", nil)["result"].(map[string]any)["tools"].([]any)
	names := map[string]bool{}
	for _, tool := range list {
		m := tool.(map[string]any)
		names[m["name"].(string)] = true
		if m["inputSchema"].(map[string]any)["type"] != "object" {
			t.Errorf("tool %v has no object schema", m["name"])
		}
	}
	for _, want := range []string{"check_url", "readiness_score", "robots_audit", "list_profiles"} {
		if !names[want] {
			t.Errorf("missing tool %s", want)
		}
	}
	if call(3, "ping", nil)["result"] == nil {
		t.Error("ping should return an empty result")
	}
	if e := call(4, "resources/list", nil)["error"].(map[string]any); e["code"].(float64) != codeMethodNotFound {
		t.Errorf("unknown method error = %v", e)
	}
	init = call(5, "initialize", map[string]any{"protocolVersion": "1999-01-01"})
	if init["result"].(map[string]any)["protocolVersion"] != ProtocolVersion {
		t.Error("unsupported versions should get the latest version back")
	}
}

func TestCheckURLTool(t *testing.T) {
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.UserAgent(), "GPTBot") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte("<html><body><p>Hello agents.</p></body></html>"))
	}))
	defer site.Close()

	call := session(t, acurl.Options{SkipRobots: true})
	res := call(1, "tools/call", map[string]any{"name": "check_url", "arguments": map[string]any{"url": site.URL, "agents": []string{"Chrome (Browser)", "GPTBot"}}})
	result := res["result"].(map[string]any)
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "| GPTBot | 403 |") || result["structuredContent"] == nil {
		t.Errorf("check_url text = %s", text)
	}
}

func TestToolErrors(t *testing.T) {
	call := session(t, acurl.Options{BlockPrivate: true})
	res := call(1, "tools/call", map[string]any{"name": "check_url", "arguments": map[string]any{}})
	if r := res["result"].(map[string]any); r["isError"] != true {
		t.Errorf("missing url should be a tool error: %v", r)
	}
	res = call(2, "tools/call", map[string]any{"name": "nope"})
	if res["error"] == nil {
		t.Error("unknown tool should be a protocol error")
	}
	res = call(3, "tools/call", map[string]any{"name": "list_profiles", "arguments": map[string]any{"category": "ai-assistant"}})
	text := res["result"].(map[string]any)["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "ChatGPT-User (ai-assistant, OpenAI)") || strings.Contains(text, "GPTBot") {
		t.Errorf("list_profiles = %s", text)
	}
}
