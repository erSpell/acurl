// Package classifier labels each fetch result with how the site treated it.
package classifier

import (
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
		if i == b || (r.Classification != Allowed && r.Classification != RobotsDisallow) {
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
	}
	return s
}
