// Package detect recognises bot-challenge and WAF block pages.
package detect

import (
	"net/http"
	"regexp"
	"strings"
)

// Signature describes how one vendor's challenge or block page looks.
// A signature matches when its status filter (if any) passes and either
// its header check or its body pattern matches.
type Signature struct {
	Vendor string
	// Statuses restricts body matching to these status codes. Empty means any.
	Statuses []int
	// Header reports whether response headers alone identify the vendor.
	Header func(h http.Header) bool
	// Body matches the response body.
	Body *regexp.Regexp
}

func (s Signature) statusOK(code int) bool {
	if len(s.Statuses) == 0 {
		return true
	}
	for _, c := range s.Statuses {
		if c == code {
			return true
		}
	}
	return false
}

func (s Signature) match(code int, h http.Header, body []byte) bool {
	if s.Header != nil && s.Header(h) {
		return true
	}
	return s.Body != nil && s.statusOK(code) && s.Body.Match(body)
}

// Signatures is checked in order; the first match wins.
var Signatures = []Signature{
	{
		Vendor: "Cloudflare",
		Header: func(h http.Header) bool { return strings.EqualFold(h.Get("cf-mitigated"), "challenge") },
		Body:   regexp.MustCompile(`(?i)(/cdn-cgi/challenge-platform/|__cf_chl_|cf-browser-verification|cf_chl_opt|<title>Just a moment\.\.\.</title>|Attention Required! \| Cloudflare)`),
	},
	{
		Vendor:   "Akamai",
		Statuses: []int{403},
		// AkamaiGHost is on every Akamai response, so only the block page counts.
		Body: regexp.MustCompile(`(?is)Access Denied.*Reference\s*#\d+\.[0-9a-f]+`),
	},
}

// Challenge returns the vendor whose challenge or block page this response
// is, or "" when none is recognised.
func Challenge(status int, h http.Header, body []byte) string {
	if h == nil {
		h = http.Header{}
	}
	for _, s := range Signatures {
		if s.match(status, h, body) {
			return s.Vendor
		}
	}
	return ""
}
