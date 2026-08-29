package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tragent-io/acurl/pkg/types"
)

func sample() types.ComparisonResult {
	return types.ComparisonResult{
		URL:       "https://shop.example/p/1",
		Timestamp: time.Date(2026, 8, 29, 10, 0, 0, 0, time.UTC),
		Results: []types.FetchResult{
			{Profile: "Chrome (Browser)", StatusCode: 200, Duration: 120 * time.Millisecond, BodySize: 2048, Title: "Shoes", Classification: "allowed"},
			{Profile: "ChatGPT-User", StatusCode: 403, Duration: 80 * time.Millisecond, BodySize: 900, BotChallenge: "Cloudflare", Classification: "challenged"},
			{Profile: "Pipe|Bot", Error: "dial tcp: timeout", Classification: "error"},
		},
		Summary: types.Summary{Total: 3, Allowed: 1, Challenged: 1, Errors: 1},
	}
}

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, "json", sample(), false); err != nil {
		t.Fatal(err)
	}
	var got types.ComparisonResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.Caveat == "" || len(got.Results) != 3 || got.Summary.Challenged != 1 {
		t.Errorf("unexpected decode: %+v", got)
	}
}

func TestWriteMarkdown(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, "markdown", sample(), false); err != nil {
		t.Fatal(err)
	}
	md := buf.String()
	for _, want := range []string{"# acurl report", "| ChatGPT-User | 403 |", "challenge: Cloudflare", `Pipe\|Bot`, "| ERR |"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

func TestWriteTableNoColor(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, "table", sample(), false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "\x1b[") {
		t.Error("no-color table contains ANSI escapes")
	}
	for _, want := range []string{"Chrome (Browser)", "challenged", "Summary", "120ms", "2.0KB"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q", want)
		}
	}
}

func TestWriteUnknownFormat(t *testing.T) {
	if err := Write(&bytes.Buffer{}, "xml", sample(), false); err == nil {
		t.Error("expected error for unknown format")
	}
}

func TestFormatSize(t *testing.T) {
	for in, want := range map[int]string{0: "0B", 1023: "1023B", 1536: "1.5KB", 3 << 20: "3.0MB"} {
		if got := FormatSize(in); got != want {
			t.Errorf("FormatSize(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestExitCode(t *testing.T) {
	tests := []struct {
		s    types.Summary
		want int
	}{
		{types.Summary{Total: 2, Allowed: 2}, ExitOK},
		{types.Summary{Total: 2, Allowed: 1, RobotsDisallow: 1}, ExitOK},
		{types.Summary{Total: 2, Allowed: 1, Blocked: 1}, ExitBlocked},
		{types.Summary{Total: 2, Allowed: 1, Challenged: 1}, ExitBlocked},
		{types.Summary{Total: 2, Blocked: 1, Errors: 1}, ExitError},
	}
	for _, tt := range tests {
		if got := ExitCode(tt.s); got != tt.want {
			t.Errorf("ExitCode(%+v) = %d, want %d", tt.s, got, tt.want)
		}
	}
}
