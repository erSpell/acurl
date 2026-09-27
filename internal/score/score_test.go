package score

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/tragent-io/acurl/pkg/types"
)

var update = flag.Bool("update", false, "rewrite golden files")

var product = &types.Product{Source: "json-ld", Price: "98", Currency: "USD", Availability: "InStock"}

func res(name, cat, class string) types.FetchResult {
	return types.FetchResult{Profile: name, Category: cat, Classification: class, JS: &types.JSInfo{TextLength: 900}, Product: product}
}

func baselineResult() types.FetchResult {
	r := res("Chrome", types.CategoryBrowser, "allowed")
	r.Browser = true
	return r
}

func TestComputeGolden(t *testing.T) {
	tests := map[string]types.ComparisonResult{
		"perfect": {Results: []types.FetchResult{
			baselineResult(),
			res("ChatGPT-User", types.CategoryAIAssistant, "allowed"),
			res("OAI-SearchBot", types.CategoryAISearch, "allowed"),
			res("GPTBot", types.CategoryAITraining, "robots-disallowed"),
		}},
		"blocked-assistant": {Results: []types.FetchResult{
			baselineResult(),
			res("ChatGPT-User", types.CategoryAIAssistant, "challenged"),
			res("Claude-User", types.CategoryAIAssistant, "allowed"),
			res("OAI-SearchBot", types.CategoryAISearch, "blocked"),
		}},
		"spa-without-product": {Results: []types.FetchResult{
			{Profile: "Chrome", Browser: true, Category: types.CategoryBrowser, Classification: "allowed", JS: &types.JSInfo{ClientRendered: true, Reason: "empty app root element"}},
			{Profile: "ChatGPT-User", Category: types.CategoryAIAssistant, Classification: "cloaked"},
		}},
	}
	for name, rep := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := json.MarshalIndent(Compute(&rep), "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			golden := filepath.Join("testdata", name+".golden.json")
			if *update {
				if err := os.MkdirAll("testdata", 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(golden, append(got, '\n'), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v (run go test ./internal/score -update)", err)
			}
			if string(want) != string(got)+"\n" {
				t.Errorf("score mismatch for %s:\n%s\nwant:\n%s", name, got, want)
			}
		})
	}
}

func TestGrades(t *testing.T) {
	for v, want := range map[int]string{100: "A", 90: "A", 85: "B", 70: "C", 55: "D", 10: "F"} {
		if g := grade(v); g != want {
			t.Errorf("grade(%d) = %s, want %s", v, g, want)
		}
	}
}

func TestEveryFailedCheckHasRecommendation(t *testing.T) {
	rep := types.ComparisonResult{Results: []types.FetchResult{
		{Profile: "Chrome", Browser: true, Category: types.CategoryBrowser, Classification: "allowed", JS: &types.JSInfo{ClientRendered: true}, Product: &types.Product{Source: "opengraph"}},
		{Profile: "A", Category: types.CategoryAIAssistant, Classification: "challenged"},
		{Profile: "S", Category: types.CategoryAISearch, Classification: "cloaked"},
		{Profile: "T", Category: types.CategoryAITraining, Classification: "allowed", ProductIssue: "price"},
	}}
	for _, c := range Compute(&rep).Checks {
		if c.Applies && !c.Passed && c.Recommendation == "" {
			t.Errorf("check %s failed without a recommendation", c.ID)
		}
	}
}

func TestComputeSkipsWhenBaselineBlocked(t *testing.T) {
	rep := types.ComparisonResult{Results: []types.FetchResult{
		{Profile: "Chrome", Browser: true, Category: types.CategoryBrowser, Classification: "challenged"},
		{Profile: "GPTBot", Category: types.CategoryAITraining, Classification: "allowed"},
	}}
	s := Compute(&rep)
	if s.Grade != "-" || s.Value != 0 || s.Note == "" || len(s.Checks) != 0 {
		t.Errorf("Compute = %+v", s)
	}
}
