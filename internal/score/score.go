// Package score turns an acurl report into an AI-readiness score from 0 to
// 100, with the checks behind it.
package score

import (
	"fmt"
	"math"
	"strings"

	"github.com/tragent-io/acurl/internal/classifier"
	"github.com/tragent-io/acurl/pkg/types"
)

// check computes one weighted check. It returns the passed fraction in
// [0, 1], a detail line, and whether the check applies to this page.
type check struct {
	id     string
	title  string
	weight int
	eval   func(rep *types.ComparisonResult) (passed float64, detail string, applies bool)
}

var checks = []check{
	{"assistants-allowed", "AI assistants can fetch the page", 25, allowedShare(types.CategoryAIAssistant)},
	{"ai-search-allowed", "AI search crawlers can fetch the page", 20, allowedShare(types.CategoryAISearch)},
	{"no-challenges", "No bot challenges for AI agents", 10, noChallenges},
	{"no-cloaking", "AI agents see the same content as browsers", 10, noCloaking},
	{"server-rendered", "Content is in the HTML without JavaScript", 15, serverRendered},
	{"structured-data", "Product structured data is present", 10, structuredData},
	{"product-parity", "AI agents see the same price and availability", 10, productParity},
}

// Compute scores rep. Checks that do not apply (for example product checks
// on a page without products) are left out and the rest re-weighted.
func Compute(rep *types.ComparisonResult) *types.Score {
	s := &types.Score{}
	total, earned := 0.0, 0.0
	for _, c := range checks {
		passed, detail, applies := c.eval(rep)
		res := types.ScoreCheck{ID: c.id, Title: c.title, Weight: c.weight, Applies: applies, Detail: detail}
		if applies {
			res.Passed = passed >= 1
			if !res.Passed {
				res.Recommendation = recommendations[c.id]
			}
			res.Points = math.Round(passed*float64(c.weight)*10) / 10
			total += float64(c.weight)
			earned += passed * float64(c.weight)
		}
		s.Checks = append(s.Checks, res)
	}
	if total > 0 {
		s.Value = int(math.Round(earned / total * 100))
	}
	s.Grade = grade(s.Value)
	return s
}

func grade(v int) string {
	switch {
	case v >= 90:
		return "A"
	case v >= 80:
		return "B"
	case v >= 65:
		return "C"
	case v >= 50:
		return "D"
	default:
		return "F"
	}
}

// aiAgent reports whether a result belongs to an AI agent category.
func aiAgent(cat string) bool {
	return cat == types.CategoryAIAssistant || cat == types.CategoryAISearch || cat == types.CategoryAITraining
}

func allowedShare(category string) func(*types.ComparisonResult) (float64, string, bool) {
	return func(rep *types.ComparisonResult) (float64, string, bool) {
		var total, ok int
		var bad []string
		for _, r := range rep.Results {
			if r.Category != category || r.RobotsOnly {
				continue
			}
			total++
			if r.Classification == classifier.Allowed {
				ok++
			} else {
				bad = append(bad, r.Profile+" ("+r.Classification+")")
			}
		}
		if total == 0 {
			return 0, "no " + category + " profiles in this run", false
		}
		detail := fmt.Sprintf("%d of %d allowed", ok, total)
		if len(bad) > 0 {
			detail += ": " + strings.Join(bad, ", ")
		}
		return float64(ok) / float64(total), detail, true
	}
}

func countAI(rep *types.ComparisonResult, match func(types.FetchResult) bool) (total int, names []string) {
	for _, r := range rep.Results {
		if !aiAgent(r.Category) || r.RobotsOnly {
			continue
		}
		total++
		if match(r) {
			names = append(names, r.Profile)
		}
	}
	return total, names
}

func noChallenges(rep *types.ComparisonResult) (float64, string, bool) {
	total, hit := countAI(rep, func(r types.FetchResult) bool { return r.Classification == classifier.Challenged })
	if total == 0 {
		return 0, "", false
	}
	if len(hit) == 0 {
		return 1, "no challenge pages", true
	}
	return 1 - float64(len(hit))/float64(total), "challenged: " + strings.Join(hit, ", "), true
}

func noCloaking(rep *types.ComparisonResult) (float64, string, bool) {
	if classifier.BaselineIndex(rep.Results) < 0 {
		return 0, "needs the browser baseline", false
	}
	total, hit := countAI(rep, func(r types.FetchResult) bool { return r.Classification == classifier.Cloaked })
	if total == 0 {
		return 0, "", false
	}
	if len(hit) == 0 {
		return 1, "content matches the browser", true
	}
	return 1 - float64(len(hit))/float64(total), "different content for: " + strings.Join(hit, ", "), true
}

func baseline(rep *types.ComparisonResult) *types.FetchResult {
	if b := classifier.BaselineIndex(rep.Results); b >= 0 && rep.Results[b].Classification == classifier.Allowed {
		return &rep.Results[b]
	}
	return nil
}

func serverRendered(rep *types.ComparisonResult) (float64, string, bool) {
	if rr := rep.Render; rr != nil && rr.Error == "" {
		share := rr.JSOnlyShare
		if share < 0.2 {
			return 1, fmt.Sprintf("%.0f%% of text needs JavaScript", share*100), true
		}
		return 1 - share, fmt.Sprintf("%.0f%% of text only appears after JavaScript runs", share*100), true
	}
	b := baseline(rep)
	if b == nil || b.JS == nil {
		return 0, "needs a successful browser baseline", false
	}
	if b.JS.ClientRendered {
		return 0, "client-rendered page: " + b.JS.Reason, true
	}
	return 1, "page text is in the raw HTML", true
}

func structuredData(rep *types.ComparisonResult) (float64, string, bool) {
	b := baseline(rep)
	if b == nil {
		return 0, "needs a successful browser baseline", false
	}
	p := b.Product
	switch {
	case p == nil:
		// Only product pages need product data; an og:type product or a
		// price-looking page would have produced one.
		return 0, "no schema.org Product found (fine for non-product pages)", false
	case p.Source != "json-ld":
		return 0.6, "product data only in " + p.Source + "; JSON-LD is the most widely read format", true
	case p.Price == "" || p.Availability == "":
		return 0.7, "JSON-LD Product is missing price or availability", true
	}
	return 1, "JSON-LD Product with price and availability", true
}

func productParity(rep *types.ComparisonResult) (float64, string, bool) {
	b := baseline(rep)
	if b == nil || b.Product == nil {
		return 0, "", false
	}
	total, hit := countAI(rep, func(r types.FetchResult) bool { return r.ProductIssue != "" })
	if total == 0 {
		return 0, "", false
	}
	if len(hit) == 0 {
		return 1, "every allowed AI agent sees the same product data", true
	}
	return 1 - float64(len(hit))/float64(total), "different product data for: " + strings.Join(hit, ", "), true
}
