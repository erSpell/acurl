// Package detect recognises bot-challenge and WAF block pages.
package detect

import (
	"net/http"
	"regexp"
	"strings"
)

// Signature describes how one vendor's challenge or block page looks.
//
// A response matches when the vendor's header check passes, when Body (a
// marker only a challenge or block page contains) matches under the status
// filter, or when Embedded matches on a response that is not a success.
type Signature struct {
	Vendor string
	// Statuses restricts Body matching to these status codes. Empty means any.
	Statuses []int
	// Header reports whether response headers alone identify the vendor.
	Header func(h http.Header) bool
	// Body matches markers that only appear on the challenge or block page.
	Body *regexp.Regexp
	// Embedded matches scripts the vendor also injects into healthy pages
	// (Cloudflare's challenge platform, AWS WAF's challenge.js). They only
	// count when the response is not a success, or a healthy page that
	// happens to load the script would be reported as challenged.
	Embedded *regexp.Regexp
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
	if s.Body != nil && s.statusOK(code) && s.Body.Match(body) {
		return true
	}
	return s.Embedded != nil && notSuccess(code) && s.Embedded.Match(body)
}

// notSuccess reports whether a status can carry a challenge page: any error,
// or 202, which AWS WAF uses for its challenge.
func notSuccess(code int) bool {
	return code >= 400 || code == http.StatusAccepted
}

// Signatures is checked in order; the first match wins.
var Signatures = []Signature{
	{
		Vendor:   "Cloudflare",
		Header:   func(h http.Header) bool { return strings.EqualFold(h.Get("cf-mitigated"), "challenge") },
		Body:     regexp.MustCompile(`(?i)(__cf_chl_|cf-browser-verification|cf_chl_opt|<title>Just a moment\.\.\.</title>|Attention Required! \| Cloudflare)`),
		Embedded: regexp.MustCompile(`(?i)/cdn-cgi/challenge-platform/`),
	},
	{
		// Akamai Bot Manager's interstitial challenge is served with a 200 and
		// reloads the page after a JavaScript proof of work.
		Vendor: "Akamai",
		Body:   regexp.MustCompile(`(?i)(/_sec/verify\?provider=interstitial|triggerInterstitialChallenge|[?&]bm-verify=)`),
	},
	{
		Vendor:   "Akamai",
		Statuses: []int{403},
		// AkamaiGHost is on every Akamai response, so only the block page counts.
		// Akamai HTML-encodes the reference ID ("Reference&#32;&#35;18&#46;...").
		Body: regexp.MustCompile(`(?is)Access Denied.*(Reference(\s|&#32;)*(#|&#35;)\d+(\.|&#46;)|errors(\.|&#46;)edgesuite(\.|&#46;)net)`),
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
		Vendor:   "Imperva",
		Body:     regexp.MustCompile(`(?i)(Incapsula incident ID|Request unsuccessful\. Incapsula)`),
		Embedded: regexp.MustCompile(`(?i)_Incapsula_Resource`),
	},
	{
		Vendor:   "Kasada",
		Header:   func(h http.Header) bool { return h.Get("X-Kpsdk-Ct") != "" || h.Get("X-Kpsdk-C") != "" },
		Embedded: regexp.MustCompile(`(?i)(/149e9513-01fa-4fb0-aad4-566afd725d1b/|KPSDK\.configure)`),
	},
	{
		Vendor: "AWS WAF",
		Header: func(h http.Header) bool {
			return strings.EqualFold(h.Get("X-Amzn-Waf-Action"), "challenge") || strings.EqualFold(h.Get("X-Amzn-Waf-Action"), "captcha")
		},
		Embedded: regexp.MustCompile(`(?i)(awswaf\.com/[^"']*challenge\.js|AwsWafIntegration)`),
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

// Edge names the CDN or edge network that answered an error response, from
// its headers, so a blocked result can say who blocked it. It returns ""
// for successful responses or when no edge is recognised.
func Edge(status int, h http.Header) string {
	if status < 400 || h == nil {
		return ""
	}
	server := strings.ToLower(h.Get("Server"))
	switch {
	case h.Get("X-Amz-Cf-Id") != "" || strings.Contains(server, "cloudfront"):
		return "CloudFront"
	case h.Get("Cf-Ray") != "" || server == "cloudflare":
		return "Cloudflare"
	case strings.Contains(server, "akamaighost") || h.Get("X-Akamai-Transformed") != "":
		return "Akamai"
	case strings.HasPrefix(strings.ToLower(h.Get("X-Served-By")), "cache-") || h.Get("Fastly-Debug-Digest") != "":
		return "Fastly"
	case h.Get("X-Vercel-Id") != "" || server == "vercel":
		return "Vercel"
	case h.Get("X-Iinfo") != "":
		return "Imperva"
	}
	return ""
}
