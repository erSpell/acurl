package robots

import (
	"context"

	"github.com/tragent-io/acurl/pkg/types"
)

// Audit checks every profile that has a robots.txt token against target.
func Audit(ctx context.Context, cache *Cache, target string, ps []types.Profile) types.RobotsAudit {
	d := cache.Get(ctx, target)
	a := types.RobotsAudit{
		URL:       target,
		RobotsURL: d.URL,
		Status:    d.Status,
		Sitemaps:  d.Sitemaps(),
	}
	if d.Err != nil {
		a.Error = d.Err.Error()
	}
	for _, p := range ps {
		if p.RobotsKey == "" {
			continue
		}
		a.Agents = append(a.Agents, types.RobotsAgent{
			Profile:  p.Name,
			Token:    p.RobotsKey,
			Category: p.Category,
			Allowed:  d.Allowed(p.RobotsKey, target),
		})
	}
	return a
}
