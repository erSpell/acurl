package robots

import (
	"bytes"
	"net/http"
	"strings"

	"golang.org/x/net/html"
)

// parameterised directives use a colon for their value, so a colon after
// one of these does not introduce a user agent name.
var valueDirectives = map[string]bool{
	"max-snippet": true, "max-image-preview": true, "max-video-preview": true,
	"unavailable_after": true,
}

// Directives returns the indexing directives (noindex, nofollow, noai, ...)
// that apply to the robots.txt token agent, from X-Robots-Tag headers and
// <meta name="robots"> or <meta name="{agent}"> tags. Generic and
// agent-specific directives are merged without duplicates.
func Directives(h http.Header, body []byte, agent string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(list string) {
		for _, d := range strings.Split(list, ",") {
			d = strings.ToLower(strings.TrimSpace(d))
			if d != "" && !seen[d] {
				seen[d] = true
				out = append(out, d)
			}
		}
	}

	for _, v := range h.Values("X-Robots-Tag") {
		name, rest, ok := strings.Cut(v, ":")
		if ok && !strings.Contains(name, ",") && !valueDirectives[strings.ToLower(strings.TrimSpace(name))] {
			if agent != "" && strings.EqualFold(strings.TrimSpace(name), agent) {
				add(rest)
			}
			continue
		}
		add(v)
	}

	for name, content := range metaTags(body) {
		if name == "robots" || (agent != "" && strings.EqualFold(name, agent)) {
			add(content)
		}
	}
	return out
}

// metaTags returns lower-cased meta name -> content for the document head.
// Parsing stops at <body> to keep large pages cheap.
func metaTags(body []byte) map[string]string {
	tags := map[string]string{}
	z := html.NewTokenizer(bytes.NewReader(body))
	for {
		switch z.Next() {
		case html.ErrorToken:
			return tags
		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			switch string(name) {
			case "body":
				return tags
			case "meta":
				var n, c string
				for hasAttr {
					var k, v []byte
					k, v, hasAttr = z.TagAttr()
					switch string(k) {
					case "name":
						n = strings.ToLower(string(v))
					case "content":
						c = string(v)
					}
				}
				if n != "" {
					if prev, ok := tags[n]; ok {
						c = prev + "," + c
					}
					tags[n] = c
				}
			}
		}
	}
}
