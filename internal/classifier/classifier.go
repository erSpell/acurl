// Package classifier labels each fetch result with how the site treated it.
package classifier

import (
	"strings"

	"github.com/tragent-io/acurl/pkg/types"
)

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

// ClassifyAll sets Classification on every result in place.
func ClassifyAll(results []types.FetchResult) {
	for i := range results {
		results[i].Classification = Classify(results[i])
	}
}

// BaselineIndex returns the index of the browser baseline result, or -1.
func BaselineIndex(results []types.FetchResult) int {
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
