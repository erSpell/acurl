package fetcher

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
)

const maxTitleLen = 100

var titleRegex = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

// extractTitle returns the trimmed, unescaped <title> of an HTML document.
func extractTitle(body string) string {
	m := titleRegex.FindStringSubmatch(body)
	if len(m) < 2 {
		return ""
	}
	title := strings.Join(strings.Fields(html.UnescapeString(m[1])), " ")
	if utf8.RuneCountInString(title) > maxTitleLen {
		title = string([]rune(title)[:maxTitleLen]) + "..."
	}
	return title
}

// contentHash returns a short, stable fingerprint of a response body.
func contentHash(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:8])
}

// isHTML reports whether a response should be parsed as HTML. A missing
// Content-Type falls back to sniffing the body.
func isHTML(contentType string, body []byte) bool {
	if contentType == "" {
		contentType = http.DetectContentType(body)
	}
	mt, _, _ := strings.Cut(strings.ToLower(contentType), ";")
	mt = strings.TrimSpace(mt)
	return mt == "text/html" || mt == "application/xhtml+xml"
}
