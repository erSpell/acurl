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
	{
		Vendor: "DataDome",
		Header: func(h http.Header) bool { return h.Get("X-DataDome") != "" && h.Get("X-DD-B") != "" },
		Body:   regexp.MustCompile(`(?i)(geo\.captcha-delivery\.com|ct\.captcha-delivery\.com|dd=\{'rt':'c')`),
	},
	{
		Vendor:   "PerimeterX",
		Statuses: []int{403, 429},
		Body:     regexp.MustCompile(`(?i)(/_px[a-z0-9]*/|px-captcha|_pxAppId|window\._pxUuid|Press &amp; Hold|Press & Hold)`),
	},
	{
		Vendor: "Imperva",
		Body:   regexp.MustCompile(`(?i)(_Incapsula_Resource|Incapsula incident ID|Request unsuccessful\. Incapsula)`),
	},
	{
		Vendor: "Kasada",
		Header: func(h http.Header) bool { return h.Get("X-Kpsdk-Ct") != "" || h.Get("X-Kpsdk-C") != "" },
		Body:   regexp.MustCompile(`(?i)(/149e9513-01fa-4fb0-aad4-566afd725d1b/|KPSDK\.configure)`),
	},
	{
		Vendor: "AWS WAF",
		Header: func(h http.Header) bool {
			return strings.EqualFold(h.Get("X-Amzn-Waf-Action"), "challenge") || strings.EqualFold(h.Get("X-Amzn-Waf-Action"), "captcha")
		},
		Body: regexp.MustCompile(`(?i)(awswaf\.com/[^"']*challenge\.js|AwsWafIntegration)`),
	},
	{
		Vendor:   "Sucuri",
		Statuses: []int{403, 503},
		Body:     regexp.MustCompile(`(?i)(Sucuri WebSite Firewall|sucuri\.net/privacy-policy|cloudproxy@sucuri)`),
	},
	{
		Vendor: "CAPTCHA",
		// Many normal pages embed a captcha in a login or contact form, so a
		// generic captcha only counts on error responses.
		Statuses: []int{401, 403, 405, 429, 503},
		Body:     regexp.MustCompile(`(?i)(class=["'][^"']*g-recaptcha|class=["'][^"']*h-captcha|class=["'][^"']*cf-turnstile|data-sitekey=)`),
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
