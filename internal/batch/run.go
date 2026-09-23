package batch

import (
	"context"
	"sync"
	"time"

	"github.com/tragent-io/acurl/pkg/types"
)

// Checker is the part of acurl.Checker that batch needs.
type Checker interface {
	Check(ctx context.Context, url string) (*types.ComparisonResult, error)
}

// Options configures a batch run.
type Options struct {
	// Parallel is how many URLs are checked at once. Default 2.
	Parallel int
	// Delay is the minimum time between starting checks on the same host.
	Delay time.Duration
	// Progress, if set, is called after each URL completes.
	Progress func(done, total int, url string)
}

// Run checks every URL and returns the reports in input order.
func Run(ctx context.Context, c Checker, urls []string, opts Options) *types.BatchReport {
	if opts.Parallel <= 0 {
		opts.Parallel = 2
	}
	items := make([]types.BatchItem, len(urls))
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		done int
		sem  = make(chan struct{}, opts.Parallel)
		lim  = newHostLimiter(opts.Delay)
	)
	for i, u := range urls {
		wg.Add(1)
		go func(i int, u string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			item := types.BatchItem{URL: u}
			if err := lim.wait(ctx, u); err != nil {
				item.Error = err.Error()
			} else if rep, err := c.Check(ctx, u); err != nil {
				item.Error = err.Error()
			} else {
				item.Report = rep
			}
			items[i] = item

			if opts.Progress != nil {
				mu.Lock()
				done++
				opts.Progress(done, len(urls), u)
				mu.Unlock()
			}
		}(i, u)
	}
	wg.Wait()
	return Summarize(items)
}

// Summarize builds the batch report and its aggregate summary.
func Summarize(items []types.BatchItem) *types.BatchReport {
	rep := &types.BatchReport{Items: items}
	total := 0
	for _, it := range items {
		rep.Summary.URLs++
		if it.Report == nil {
			rep.Summary.Failed++
			continue
		}
		s := it.Report.Summary
		rep.Summary.Results.Total += s.Total
		rep.Summary.Results.Allowed += s.Allowed
		rep.Summary.Results.Blocked += s.Blocked
		rep.Summary.Results.Challenged += s.Challenged
		rep.Summary.Results.RateLimited += s.RateLimited
		rep.Summary.Results.Cloaked += s.Cloaked
		rep.Summary.Results.RobotsDisallow += s.RobotsDisallow
		rep.Summary.Results.Errors += s.Errors
		if it.Report.Score != nil {
			total += it.Report.Score.Value
			rep.Summary.Scored++
		}
	}
	if rep.Summary.Scored > 0 {
		rep.Summary.AverageScore = total / rep.Summary.Scored
	}
	return rep
}
