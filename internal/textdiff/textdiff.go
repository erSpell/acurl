// Package textdiff compares what different agents were shown on a page.
//
// Comparisons work on visible text, not raw HTML, so that nonces, CSRF
// tokens, inline scripts and attribute order do not count as differences.
package textdiff

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
)

// skipTags hold content that is never shown to a reader.
var skipTags = map[string]bool{
	"script": true, "style": true, "noscript": true, "template": true,
	"svg": true, "iframe": true, "head": true,
}

// blockTags end a line of visible text.
var blockTags = map[string]bool{
	"p": true, "div": true, "br": true, "li": true, "tr": true, "section": true,
	"article": true, "header": true, "footer": true, "h1": true, "h2": true,
	"h3": true, "h4": true, "h5": true, "h6": true, "title": true, "main": true,
	"nav": true, "aside": true, "table": true, "ul": true, "ol": true, "form": true,
}

// VisibleText extracts the human-visible text of an HTML document, one
// block per line, with whitespace collapsed. The <title> is kept even though
// it lives in <head>.
func VisibleText(doc []byte) string {
	z := html.NewTokenizer(bytes.NewReader(doc))
	var (
		lines []string
		cur   strings.Builder
		skip  int
		title bool
	)
	flush := func() {
		if line := strings.Join(strings.Fields(cur.String()), " "); line != "" {
			lines = append(lines, line)
		}
		cur.Reset()
	}

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			flush()
			return strings.Join(lines, "\n")
		case html.StartTagToken, html.EndTagToken, html.SelfClosingTagToken:
			name, _ := z.TagName()
			tag := string(name)
			if tag == "title" {
				title = tt == html.StartTagToken
				flush()
				continue
			}
			if skipTags[tag] && tt != html.SelfClosingTagToken {
				if tt == html.StartTagToken {
					skip++
				} else if skip > 0 {
					skip--
				}
				continue
			}
			if blockTags[tag] {
				flush()
			}
		case html.TextToken:
			if skip == 0 || title {
				cur.Write(z.Text())
				cur.WriteByte(' ')
			}
		}
	}
}

// Similarity returns a score in [0, 1] for how alike two visible texts are,
// using the Jaccard index over word 3-grams (shingles). Identical texts
// score 1; texts with nothing in common score 0.
func Similarity(a, b string) float64 {
	if a == b {
		return 1
	}
	sa, sb := shingles(a), shingles(b)
	if len(sa) == 0 && len(sb) == 0 {
		return 1
	}
	if len(sa) == 0 || len(sb) == 0 {
		return 0
	}
	inter := 0
	for s := range sa {
		if sb[s] {
			inter++
		}
	}
	return float64(inter) / float64(len(sa)+len(sb)-inter)
}

func shingles(text string) map[string]bool {
	words := strings.Fields(strings.ToLower(text))
	const k = 3
	set := make(map[string]bool)
	if len(words) < k {
		if len(words) > 0 {
			set[strings.Join(words, " ")] = true
		}
		return set
	}
	for i := 0; i+k <= len(words); i++ {
		set[strings.Join(words[i:i+k], " ")] = true
	}
	return set
}
