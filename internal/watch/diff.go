// Package watch detects changes in how a site treats AI agents between runs.
package watch

import (
	"fmt"
	"sort"

	"github.com/tragent-io/acurl/pkg/types"
)

// Change is one difference between two runs for the same URL.
type Change struct {
	URL     string `json:"url"`
	Profile string `json:"profile,omitempty"` // empty for score changes
	From    string `json:"from"`
	To      string `json:"to"`
	// Worse is true when the change reduces agent access.
	Worse bool `json:"worse"`
}

func (c Change) String() string {
	if c.Profile == "" {
		return fmt.Sprintf("%s: AI readiness %s -> %s", c.URL, c.From, c.To)
	}
	return fmt.Sprintf("%s: %s %s -> %s", c.URL, c.Profile, c.From, c.To)
}

// severity orders classifications from best to worst.
var severity = map[string]int{
	"allowed": 0, "robots-disallowed": 1, "cloaked": 2, "rate-limited": 3,
	"challenged": 4, "blocked": 5, "error": 6,
}

// Diff compares two reports for the same URL. Score changes smaller than
// minScoreDelta points are ignored.
func Diff(prev, cur *types.ComparisonResult, minScoreDelta int) []Change {
	if prev == nil || cur == nil {
		return nil
	}
	before := map[string]string{}
	for _, r := range prev.Results {
		before[r.Profile] = r.Classification
	}
	var out []Change
	for _, r := range cur.Results {
		old, ok := before[r.Profile]
		if !ok || old == r.Classification {
			continue
		}
		out = append(out, Change{URL: cur.URL, Profile: r.Profile, From: old, To: r.Classification, Worse: severity[r.Classification] > severity[old]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Profile < out[j].Profile })

	if ps, cs := prev.Score, cur.Score; ps != nil && cs != nil && ps.Note == "" && cs.Note == "" {
		if d := cs.Value - ps.Value; d >= minScoreDelta || -d >= minScoreDelta {
			out = append(out, Change{URL: cur.URL, From: fmt.Sprintf("%d (%s)", ps.Value, ps.Grade), To: fmt.Sprintf("%d (%s)", cs.Value, cs.Grade), Worse: d < 0})
		}
	}
	return out
}
