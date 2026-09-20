package fetcher

import (
	"context"
	"sync"

	"github.com/tragent-io/acurl/pkg/types"
)

// FetchAll fetches url once per profile, running at most concurrency
// requests at a time. Results keep the order of profiles.
func (f *Fetcher) FetchAll(ctx context.Context, url string, profiles []types.Profile, concurrency int) []types.FetchResult {
	if concurrency < 1 {
		concurrency = 1
	}
	results := make([]types.FetchResult, len(profiles))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, p := range profiles {
		wg.Add(1)
		go func(i int, p types.Profile) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				results[i] = types.FetchResult{Profile: p.Name, Error: ctx.Err().Error()}
				return
			}
			defer func() { <-sem }()
			if p.RobotsOnly {
				results[i] = types.FetchResult{Profile: p.Name, Category: p.Category, RobotsOnly: true}
				return
			}
			results[i] = f.Fetch(ctx, url, p)
		}(i, p)
	}
	wg.Wait()
	return results
}
