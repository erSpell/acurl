package output

import (
	"testing"

	"github.com/tragent-io/acurl/pkg/types"
)

func rep(score int, classes ...string) *types.ComparisonResult {
	r := &types.ComparisonResult{Score: &types.Score{Value: score}}
	for i, c := range classes {
		r.Results = append(r.Results, types.FetchResult{Profile: string(rune('A' + i)), Classification: c})
	}
	return r
}

func TestPolicy(t *testing.T) {
	tests := []struct {
		spec string
		rep  *types.ComparisonResult
		want int
	}{
		{DefaultFailOn, rep(90, "allowed", "robots-disallowed", "cloaked"), ExitOK},
		{DefaultFailOn, rep(90, "allowed", "blocked"), ExitBlocked},
		{DefaultFailOn, rep(90, "blocked", "error"), ExitError},
		{"cloaked", rep(90, "allowed", "cloaked"), ExitBlocked},
		{"robots", rep(90, "robots-disallowed"), ExitBlocked},
		{"score<80", rep(79, "allowed"), ExitBlocked},
		{"score<80", rep(80, "allowed"), ExitOK},
		{"none", rep(10, "blocked", "challenged"), ExitOK},
	}
	for _, tt := range tests {
		p, err := ParsePolicy(tt.spec)
		if err != nil {
			t.Fatalf("ParsePolicy(%q): %v", tt.spec, err)
		}
		if got := p.ExitCode(tt.rep); got != tt.want {
			t.Errorf("%q: exit %d, want %d (reasons %v)", tt.spec, got, tt.want, p.Reasons(tt.rep))
		}
	}
}

func TestParsePolicyErrors(t *testing.T) {
	for _, spec := range []string{"blokced", "score<abc", "score<101"} {
		if _, err := ParsePolicy(spec); err == nil {
			t.Errorf("ParsePolicy(%q) should fail", spec)
		}
	}
}
