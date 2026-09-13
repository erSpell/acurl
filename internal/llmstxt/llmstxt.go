// Package llmstxt finds and validates /llms.txt files (https://llmstxt.org).
package llmstxt

import (
	"bytes"
	"context"
	"net/url"
	"regexp"
	"strings"

	"github.com/tragent-io/acurl/pkg/types"
)

// MaxSize is the largest file read.
const MaxSize = 2 << 20

// Getter performs a GET and returns the status and body.
type Getter func(ctx context.Context, url string) (status int, body []byte, err error)

var linkRe = regexp.MustCompile(`\[[^\]]+\]\(([^)\s]+)\)`)

// Paths are the well-known files checked at the site root.
var Paths = []string{"/llms.txt", "/llms-full.txt"}

// Check fetches each well-known llms.txt path at the origin of target.
func Check(ctx context.Context, get Getter, target string) ([]types.LLMsTxt, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	var out []types.LLMsTxt
	for _, p := range Paths {
		fileURL := u.Scheme + "://" + u.Host + p
		status, body, err := get(ctx, fileURL)
		r := types.LLMsTxt{URL: fileURL, Status: status}
		switch {
		case err != nil:
			r.Error = err.Error()
		case status == 200:
			r.Found = true
			validate(&r, body)
		}
		out = append(out, r)
	}
	return out, nil
}

// validate checks body against the llms.txt format: an H1 title, an
// optional "> summary" blockquote and Markdown links in H2 sections.
func validate(r *types.LLMsTxt, body []byte) {
	r.Size = len(body)
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		r.Problems = append(r.Problems, "file is empty")
		return
	}
	if lower := bytes.ToLower(trimmed[:min(len(trimmed), 512)]); bytes.Contains(lower, []byte("<html")) || bytes.Contains(lower, []byte("<!doctype")) {
		r.Problems = append(r.Problems, "served HTML instead of Markdown (probably a catch-all page)")
		return
	}

	lines := strings.Split(string(trimmed), "\n")
	first := strings.TrimSpace(lines[0])
	if strings.HasPrefix(first, "# ") {
		r.Title = strings.TrimSpace(first[2:])
	} else {
		r.Problems = append(r.Problems, "first line should be an H1 title (\"# Name\")")
	}
	for _, l := range lines[1:] {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if strings.HasPrefix(l, ">") {
			r.Summary = strings.TrimSpace(strings.TrimPrefix(l, ">"))
		}
		break
	}
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "## ") {
			r.Sections++
		}
	}
	r.Links = len(linkRe.FindAllString(string(trimmed), -1))
	if r.Summary == "" {
		r.Problems = append(r.Problems, "no \"> summary\" blockquote right after the title (recommended)")
	}
	if r.Links == 0 && strings.HasSuffix(r.URL, "/llms.txt") {
		r.Problems = append(r.Problems, "no Markdown links to pages")
	}
}
