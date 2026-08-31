package fetcher

import (
	"context"
	"errors"
	"net/http"
)

// maxRedirects matches the limit most browsers and crawlers use.
const maxRedirects = 10

var errTooManyRedirects = errors.New("stopped after 10 redirects")

type chainKey struct{}

// withRedirectChain returns a context that records redirect hops into chain.
func withRedirectChain(ctx context.Context, chain *[]string) context.Context {
	return context.WithValue(ctx, chainKey{}, chain)
}

// recordRedirects is a req redirect policy that appends every hop to the
// chain stored in the request context.
func recordRedirects(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return errTooManyRedirects
	}
	if chain, ok := req.Context().Value(chainKey{}).(*[]string); ok {
		*chain = append(*chain, req.URL.String())
	}
	return nil
}
