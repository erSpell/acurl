// Package render estimates how much of a page depends on JavaScript, which
// most AI agents do not execute.
package render

import (
	"bytes"
	"regexp"
	"strings"

	"golang.org/x/net/html"

	"github.com/tragent-io/acurl/internal/textdiff"
	"github.com/tragent-io/acurl/pkg/types"
)

// minText is the visible text length below which a page that ships a lot
// of JavaScript is treated as a client-side rendered shell.
const minText = 250

var frameworks = []struct {
	name string
	re   *regexp.Regexp
}{
	{"Next.js", regexp.MustCompile(`id="__next"|/_next/static/|__NEXT_DATA__`)},
	{"Nuxt", regexp.MustCompile(`id="__nuxt"|/_nuxt/|window\.__NUXT__`)},
	{"Shopify Hydrogen", regexp.MustCompile(`(?i)hydrogen|shopify-hydrogen`)},
	{"Angular", regexp.MustCompile(`ng-version=|<app-root`)},
	{"Gatsby", regexp.MustCompile(`id="___gatsby"`)},
	{"SvelteKit", regexp.MustCompile(`data-sveltekit|__sveltekit`)},
	{"Vue", regexp.MustCompile(`id="app"[^>]*>\s*</div>|data-v-app|__VUE__`)},
	{"React", regexp.MustCompile(`id="root"[^>]*>\s*</div>|data-reactroot|react-dom`)},
}

var noscriptJS = regexp.MustCompile(`(?is)<noscript[^>]*>[^<]*(enable|turn on)[^<]*javascript`)

// Analyze inspects a raw (unrendered) HTML response.
func Analyze(body []byte) *types.JSInfo {
	if len(body) == 0 {
		return nil
	}
	info := &types.JSInfo{
		TextLength:  len(textdiff.VisibleText(body)),
		ScriptBytes: scriptBytes(body),
	}
	for _, f := range frameworks {
		if f.re.Match(body) {
			info.Framework = f.name
			break
		}
	}

	var reasons []string
	if info.TextLength < minText {
		if info.ScriptBytes > 20*max(info.TextLength, 1) {
			reasons = append(reasons, "little visible text and a lot of JavaScript")
		}
		if noscriptJS.Match(body) {
			reasons = append(reasons, "asks visitors to enable JavaScript")
		}
		if emptyRoot(body) {
			reasons = append(reasons, "empty app root element")
		}
	}
	if len(reasons) > 0 {
		info.ClientRendered = true
		info.Reason = strings.Join(reasons, "; ")
	}
	return info
}

// scriptBytes counts inline script bytes plus a nominal cost per external script.
func scriptBytes(body []byte) int {
	const externalCost = 4096
	n := 0
	z := html.NewTokenizer(bytes.NewReader(body))
	inScript := false
	for {
		switch z.Next() {
		case html.ErrorToken:
			return n
		case html.StartTagToken:
			name, hasAttr := z.TagName()
			if string(name) != "script" {
				continue
			}
			inScript = true
			for hasAttr {
				var k []byte
				k, _, hasAttr = z.TagAttr()
				if string(k) == "src" {
					n += externalCost
				}
			}
		case html.TextToken:
			if inScript {
				n += len(z.Text())
			}
		case html.EndTagToken:
			inScript = false
		}
	}
}

var emptyRootRe = regexp.MustCompile(`<div[^>]+id=["'](root|app|__next|__nuxt|___gatsby)["'][^>]*>\s*</div>`)

func emptyRoot(body []byte) bool {
	return emptyRootRe.Match(body) || bytes.Contains(body, []byte("<app-root></app-root>"))
}
