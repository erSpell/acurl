package output

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tragent-io/acurl/pkg/types"
)

// DefaultFailOn is the --fail-on default.
const DefaultFailOn = "blocked,challenged,error"

// Policy decides which outcomes fail a run.
type Policy struct {
	Classes  map[string]bool // classifications that fail the run
	MinScore int             // fail when the score is below this; 0 = off
}

var policyClasses = map[string]string{
	"blocked":           "blocked",
	"challenged":        "challenged",
	"rate-limited":      "rate-limited",
	"cloaked":           "cloaked",
	"robots":            "robots-disallowed",
	"robots-disallowed": "robots-disallowed",
	"error":             "error",
}

// ParsePolicy parses a --fail-on value such as "blocked,cloaked,score<70".
// "none" never fails on results (network errors still exit 2).
func ParsePolicy(spec string) (Policy, error) {
	p := Policy{Classes: map[string]bool{}}
	for _, part := range strings.Split(spec, ",") {
		part = strings.ToLower(strings.TrimSpace(part))
		switch {
		case part == "" || part == "none":
		case strings.HasPrefix(part, "score<"):
			n, err := strconv.Atoi(strings.TrimPrefix(part, "score<"))
			if err != nil || n < 0 || n > 100 {
				return p, fmt.Errorf("--fail-on: invalid score threshold %q", part)
			}
			p.MinScore = n
		case policyClasses[part] != "":
			p.Classes[policyClasses[part]] = true
		default:
			return p, fmt.Errorf("--fail-on: unknown condition %q (use blocked, challenged, rate-limited, cloaked, robots, error, score<N or none)", part)
		}
	}
	return p, nil
}

// ExitCode returns the exit code for one report under p. Errors always map
// to ExitError when "error" is in the policy; every other match is ExitBlocked.
func (p Policy) ExitCode(rep *types.ComparisonResult) int {
	code := ExitOK
	for _, r := range rep.Results {
		if !p.Classes[r.Classification] {
			continue
		}
		if r.Classification == "error" {
			return ExitError
		}
		code = ExitBlocked
	}
	if p.MinScore > 0 && rep.Score != nil && rep.Score.Value < p.MinScore {
		code = max(code, ExitBlocked)
	}
	return code
}

// Reasons lists why rep fails under p, for a one-line CI message.
func (p Policy) Reasons(rep *types.ComparisonResult) []string {
	var out []string
	for _, r := range rep.Results {
		if p.Classes[r.Classification] {
			out = append(out, r.Profile+" "+r.Classification)
		}
	}
	if p.MinScore > 0 && rep.Score != nil && rep.Score.Value < p.MinScore {
		out = append(out, fmt.Sprintf("score %d < %d", rep.Score.Value, p.MinScore))
	}
	return out
}
