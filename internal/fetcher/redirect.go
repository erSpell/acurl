package fetcher

import (
	"context"
	"fmt"
	"net/http"
)

// maxRedirects matches the limit most browsers and crawlers use.
const maxRedirects = 10

var errTooManyRedirects = fmt.Errorf("stopped after %d redirects", maxRedirects)

type chainKey struct{}

// withRedirectChain returns a context that records redirect hops into chain.
func withRedirectChain(ctx context.Context, chain *[]string) context.Context {
	return context.WithValue(ctx, chainKey{}, chain)
}

// recordRedirects is a req redirect policy that appends every hop to the
// chain stored in the request context.
func recordRedirects(req *http.Request, via []*http.Request) error {
	// Record the hop before enforcing the limit so loops show where they go.
	if chain, ok := req.Context().Value(chainKey{}).(*[]string); ok {
		if len(via) == 1 {
			// First hop of an attempt: a retry starts a fresh chain.
			*chain = (*chain)[:0]
		}
		*chain = append(*chain, req.URL.String())
	}
	if len(via) >= maxRedirects {
		return errTooManyRedirects
	}
	return nil
}
