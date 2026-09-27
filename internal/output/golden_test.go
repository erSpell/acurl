package output

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tragent-io/acurl/pkg/types"
)

var update = flag.Bool("update", false, "rewrite golden files")

func goldenReport() types.ComparisonResult {
	sim := 0.31
	return types.ComparisonResult{
		URL:       "https://shop.example/products/runner",
		Timestamp: time.Date(2026, 9, 27, 9, 30, 0, 0, time.UTC),
		Results: []types.FetchResult{
			{Profile: "Chrome (Browser)", Browser: true, Category: "browser", StatusCode: 200, DurationMs: 120, Duration: 120 * time.Millisecond, BodySize: 48213, Classification: "allowed", Title: "Wool Runner",
				Product: &types.Product{Source: "json-ld", Name: "Wool Runner", Price: "98.00", Currency: "USD", Availability: "InStock"}},
			{Profile: "ChatGPT-User", Category: "ai-assistant", StatusCode: 403, DurationMs: 45, Duration: 45 * time.Millisecond, BodySize: 8650, BotChallenge: "Cloudflare", Classification: "challenged"},
			{Profile: "OAI-SearchBot", Category: "ai-search", StatusCode: 200, DurationMs: 98, Duration: 98 * time.Millisecond, BodySize: 2101, Similarity: &sim, Classification: "cloaked"},
			{Profile: "GPTBot", Category: "ai-training", StatusCode: 200, DurationMs: 101, Duration: 101 * time.Millisecond, BodySize: 48190, RobotsDisallowed: true, Classification: "robots-disallowed", Directives: []string{"noai"}},
			{Profile: "Google-Extended", Category: "ai-training", RobotsOnly: true, RobotsDisallowed: true, Classification: "robots-disallowed"},
			{Profile: "Perplexity-User", Category: "ai-assistant", StatusCode: 200, DurationMs: 130, Duration: 130 * time.Millisecond, BodySize: 48200, Classification: "allowed", ProductIssue: "price 89.00 vs browser 98.00"},
		},
		Summary: types.Summary{Total: 6, Allowed: 2, Challenged: 1, Cloaked: 1, RobotsDisallow: 2},
		Score: &types.Score{Value: 58, Grade: "D", Checks: []types.ScoreCheck{
			{ID: "assistants-allowed", Title: "AI assistants can fetch the page", Weight: 25, Applies: true, Points: 12.5, Detail: "1 of 2 allowed: ChatGPT-User (challenged)", Recommendation: "Allow AI assistants."},
			{ID: "structured-data", Title: "Product structured data is present", Weight: 10, Applies: true, Passed: true, Points: 10, Detail: "JSON-LD Product with price and availability"},
		}},
	}
}

func TestGoldenFormats(t *testing.T) {
	old := Version
	Version = "test"
	defer func() { Version = old }()

	for _, format := range []string{"table", "json", "markdown", "csv", "junit", "sarif"} {
		t.Run(format, func(t *testing.T) {
			var buf bytes.Buffer
			if err := Write(&buf, format, goldenReport(), false); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join("testdata", "report."+format+".golden")
			if *update {
				if err := os.MkdirAll("testdata", 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v (run go test ./internal/output -update)", err)
			}
			if !bytes.Equal(buf.Bytes(), want) {
				t.Errorf("%s output changed; run go test ./internal/output -update and review the diff\n%s", format, buf.String())
			}
		})
	}
}
