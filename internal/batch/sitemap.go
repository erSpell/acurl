package batch

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// MaxSitemapSize is the largest sitemap file read (the protocol limit is
// 50 MB uncompressed).
const MaxSitemapSize = 50 << 20

// Getter performs a GET and returns the status and body.
type Getter func(ctx context.Context, url string) (status int, body []byte, err error)

type sitemapDoc struct {
	XMLName xml.Name
	URLs    []struct {
		Loc string `xml:"loc"`
	} `xml:"url"`
	Sitemaps []struct {
		Loc string `xml:"loc"`
	} `xml:"sitemap"`
}

// SitemapURLs returns up to limit page URLs from the sitemap at root,
// following sitemap indexes up to two levels deep. Gzipped sitemaps are
// supported. A limit <= 0 means no limit.
func SitemapURLs(ctx context.Context, get Getter, root string, limit int) ([]string, error) {
	var out []string
	var walk func(u string, depth int) error
	walk = func(u string, depth int) error {
		if limit > 0 && len(out) >= limit {
			return nil
		}
		status, body, err := get(ctx, u)
		if err != nil {
			return fmt.Errorf("sitemap %s: %w", u, err)
		}
		if status != 200 {
			return fmt.Errorf("sitemap %s returned %d", u, status)
		}
		doc, err := parseSitemap(body)
		if err != nil {
			return fmt.Errorf("sitemap %s: %w", u, err)
		}
		for _, x := range doc.URLs {
			if loc := strings.TrimSpace(x.Loc); loc != "" {
				out = append(out, loc)
				if limit > 0 && len(out) >= limit {
					return nil
				}
			}
		}
		if depth >= 2 {
			return nil
		}
		for _, s := range doc.Sitemaps {
			if loc := strings.TrimSpace(s.Loc); loc != "" {
				if err := walk(loc, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(root, 0); err != nil {
		return out, err
	}
	return out, nil
}

func parseSitemap(body []byte) (*sitemapDoc, error) {
	if len(body) > 2 && body[0] == 0x1f && body[1] == 0x8b {
		zr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		body, err = io.ReadAll(io.LimitReader(zr, MaxSitemapSize))
		if err != nil {
			return nil, err
		}
	}
	var doc sitemapDoc
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("not a sitemap: %w", err)
	}
	switch doc.XMLName.Local {
	case "urlset", "sitemapindex":
		return &doc, nil
	}
	return nil, fmt.Errorf("unexpected root element <%s>", doc.XMLName.Local)
}
