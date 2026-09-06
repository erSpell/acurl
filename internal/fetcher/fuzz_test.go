package fetcher

import (
	"testing"
	"time"
	"unicode/utf8"
)

func FuzzExtractTitle(f *testing.F) {
	for _, s := range []string{"<title>Hi</title>", "<TITLE>\n</TITLE>", "<title>&amp;&#x1F600;</title>", "<title>", "\xff<title>\xfe</title>"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, body string) {
		title := extractTitle(body)
		if utf8.ValidString(body) && !utf8.ValidString(title) {
			t.Errorf("valid input produced invalid UTF-8 title %q", title)
		}
		if utf8.RuneCountInString(title) > maxTitleLen+3 {
			t.Errorf("title too long: %d runes", utf8.RuneCountInString(title))
		}
	})
}

func FuzzParseRetryAfter(f *testing.F) {
	for _, s := range []string{"120", "Wed, 21 Oct 2026 07:28:00 GMT", "-1", "", "99999999999999999999"} {
		f.Add(s)
	}
	now := time.Now()
	f.Fuzz(func(t *testing.T, v string) {
		if d, ok := parseRetryAfter(v, now); ok && d < 0 {
			t.Errorf("negative duration %v for %q", d, v)
		}
	})
}
