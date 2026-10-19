// Package classifier labels each fetch result with how the site treated it.
package classifier

import (
	"fmt"
	"strings"

	"github.com/tragent-io/acurl/internal/textdiff"
	"github.com/tragent-io/acurl/pkg/types"
)

// DefaultCloakThreshold is the visible-text similarity below which an
// allowed response is reported as cloaked.
const DefaultCloakThreshold = 0.5

// Classifications, from most to least severe.
const (
	Error          = "error"
	Challenged     = "challenged"
	RateLimited    = "rate-limited"
	Blocked        = "blocked"
	Cloaked        = "cloaked"
	RobotsDisallow = "robots-disallowed"
	Allowed        = "allowed"
)

// Classify returns the classification for a single result.
func Classify(r types.FetchResult) string {
	if r.RobotsOnly {
		if r.RobotsDisallowed {
			return RobotsDisallow
		}
		return Allowed
	}
	switch {
	case r.Error != "" || r.StatusCode == 0:
		return Error
	case r.BotChallenge != "":
		return Challenged
	case r.RateLimited:
		return RateLimited
	case r.StatusCode >= 400:
		return Blocked
	case r.RobotsDisallowed:
		return RobotsDisallow
	default:
		return Allowed
	}
}

// ClassifyAll sets Classification on every result in place. When a browser
// baseline was fetched successfully, every other allowed result is compared
// with it and marked cloaked if its visible text similarity is below
// threshold. A threshold <= 0 disables cloaking detection.
func ClassifyAll(results []types.FetchResult, threshold float64) {
	for i := range results {
		results[i].Classification = Classify(results[i])
	}

	b := BaselineIndex(results)
	if threshold <= 0 || b < 0 || results[b].Classification != Allowed || len(results[b].Body) == 0 {
		return
	}
	baseText := textdiff.VisibleText(results[b].Body)
	for i := range results {
		r := &results[i]
		if i == b || r.RobotsOnly || (r.Classification != Allowed && r.Classification != RobotsDisallow) {
			continue
		}
		sim := textdiff.Similarity(baseText, textdiff.VisibleText(r.Body))
		r.Similarity = &sim
		if sim < threshold {
			r.Classification = Cloaked
		}
	}
}

// BaselineIndex returns the index of the browser baseline result, or -1.
// Profiles marked browser win; otherwise a profile named like Chrome is used.
func BaselineIndex(results []types.FetchResult) int {
	for i, r := range results {
		if r.Browser {
			return i
		}
	}
	for i, r := range results {
		if strings.Contains(strings.ToLower(r.Profile), "chrome") {
			return i
		}
	}
	return -1
}

// Summarize counts results per classification.
func Summarize(results []types.FetchResult) types.Summary {
	s := types.Summary{Total: len(results)}
	for _, r := range results {
		switch r.Classification {
		case Allowed:
			s.Allowed++
		case Blocked:
			s.Blocked++
		case Challenged:
			s.Challenged++
		case RateLimited:
			s.RateLimited++
		case Cloaked:
			s.Cloaked++
		case RobotsDisallow:
			s.RobotsDisallow++
		case Error:
			s.Errors++
		}
		if r.Flaky {
			s.Flaky++
		}
	}
	return s
}

// CompareProducts flags agents that were shown different product data than
// the browser baseline: missing structured data, or a different price or
// availability. It only looks at allowed results.
func CompareProducts(results []types.FetchResult) {
	b := BaselineIndex(results)
	if b < 0 || results[b].Product == nil {
		return
	}
	base := results[b].Product
	for i := range results {
		r := &results[i]
		if i == b || r.RobotsOnly || r.Classification != Allowed {
			continue
		}
		switch {
		case r.Product == nil:
			r.ProductIssue = "no product data (browser saw " + describe(base) + ")"
		case r.Product.Price != base.Price:
			r.ProductIssue = "price " + orNone(r.Product.Price) + " vs browser " + orNone(base.Price)
		case r.Product.Availability != base.Availability:
			r.ProductIssue = "availability " + orNone(r.Product.Availability) + " vs browser " + orNone(base.Availability)
		}
	}
}

func describe(p *types.Product) string {
	if p.Price != "" {
		return strings.TrimSpace(p.Price + " " + p.Currency)
	}
	return p.Source
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

// severity orders verdicts so that ties between samples resolve to the
// worse one, which is the safer answer for a CI gate.
var severity = map[string]int{
	Allowed: 0, RobotsDisallow: 1, Cloaked: 2, RateLimited: 3, Blocked: 4, Challenged: 5, Error: 6,
}

// MergeSamples combines several runs of the same profiles into one result
// per profile: the most common verdict wins (ties go to the more severe
// one), and profiles whose verdict changed between samples are flaky.
func MergeSamples(samples [][]types.FetchResult) []types.FetchResult {
	if len(samples) == 0 {
		return nil
	}
	merged := make([]types.FetchResult, len(samples[0]))
	for i := range samples[0] {
		counts := map[string]int{}
		var order []string
		for _, run := range samples {
			c := run[i].Classification
			if counts[c] == 0 {
				order = append(order, c)
			}
			counts[c]++
		}
		best := order[0]
		for _, c := range order[1:] {
			if counts[c] > counts[best] || (counts[c] == counts[best] && severity[c] > severity[best]) {
				best = c
			}
		}
		// Keep the most recent sample with the winning verdict.
		for s := len(samples) - 1; s >= 0; s-- {
			if samples[s][i].Classification == best {
				merged[i] = samples[s][i]
				break
			}
		}
		if len(order) > 1 {
			merged[i].Flaky = true
			for _, c := range order {
				merged[i].SampleVerdicts = append(merged[i].SampleVerdicts, fmt.Sprintf("%s %d/%d", c, counts[c], len(samples)))
			}
		}
	}
	return merged
}
