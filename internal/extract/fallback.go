package extract

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"

	"github.com/tragent-io/acurl/pkg/types"
)

// fromMicrodata reads itemprop values inside an itemtype=".../Product"
// scope. Nested scopes such as offers are flattened, which is enough for the
// fields acurl reports.
func fromMicrodata(doc []byte) *types.Product {
	z := html.NewTokenizer(bytes.NewReader(doc))
	var (
		p       *types.Product
		depth   int // element depth inside the product scope; 0 = outside
		pending string
	)
	set := func(prop, val string) {
		val = strings.TrimSpace(val)
		if val == "" {
			return
		}
		switch prop {
		case "name":
			if p.Name == "" {
				p.Name = val
			}
		case "sku":
			p.SKU = val
		case "brand":
			p.Brand = val
		case "price", "lowPrice":
			if p.Price == "" {
				p.Price = val
			}
		case "priceCurrency":
			p.Currency = val
		case "availability":
			p.Availability = availability(val)
		case "ratingValue":
			p.Rating = val
		case "reviewCount", "ratingCount":
			if p.ReviewCount == "" {
				p.ReviewCount = val
			}
		}
	}

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return finish(p)
		case html.StartTagToken, html.SelfClosingTagToken:
			attrs := attributes(z)
			if p == nil {
				if strings.HasSuffix(attrs["itemtype"], "/Product") && hasAttr(attrs, "itemscope") {
					p = &types.Product{Source: "microdata"}
					depth = 1
				}
				continue
			}
			if tt == html.StartTagToken && !voidElements[attrs["\x00tag"]] {
				depth++
			}
			prop := attrs["itemprop"]
			if prop == "" {
				continue
			}
			if v, ok := attrs["content"]; ok {
				set(prop, v)
			} else if v, ok := attrs["href"]; ok && prop == "availability" {
				set(prop, v)
			} else if tt == html.StartTagToken {
				pending = prop
			}
		case html.TextToken:
			if p != nil && pending != "" {
				set(pending, string(z.Text()))
				pending = ""
			}
		case html.EndTagToken:
			if p == nil {
				continue
			}
			pending = ""
			depth--
			if depth <= 0 {
				return finish(p)
			}
		}
	}
}

// fromOpenGraph reads og:/product: meta tags (Facebook and Pinterest
// product pins use these).
func fromOpenGraph(doc []byte) *types.Product {
	meta := map[string]string{}
	z := html.NewTokenizer(bytes.NewReader(doc))
loop:
	for {
		switch z.Next() {
		case html.ErrorToken:
			break loop
		case html.StartTagToken, html.SelfClosingTagToken:
			attrs := attributes(z)
			if attrs["\x00tag"] == "body" {
				break loop
			}
			if attrs["\x00tag"] == "meta" {
				key := firstNonEmpty(attrs["property"], attrs["name"])
				if key != "" {
					meta[strings.ToLower(key)] = attrs["content"]
				}
			}
		}
	}
	if !strings.Contains(meta["og:type"], "product") && meta["product:price:amount"] == "" {
		return nil
	}
	return finish(&types.Product{
		Source:       "opengraph",
		Name:         meta["og:title"],
		Brand:        meta["product:brand"],
		Price:        firstNonEmpty(meta["product:price:amount"], meta["og:price:amount"]),
		Currency:     firstNonEmpty(meta["product:price:currency"], meta["og:price:currency"]),
		Availability: availability(firstNonEmpty(meta["product:availability"], meta["og:availability"])),
	})
}

// finish drops products that carry no useful data.
func finish(p *types.Product) *types.Product {
	if p == nil || (p.Name == "" && p.Price == "") {
		return nil
	}
	return p
}

var voidElements = map[string]bool{
	"meta": true, "link": true, "img": true, "br": true, "hr": true, "input": true,
	"source": true, "area": true, "base": true, "col": true, "embed": true, "wbr": true,
}

// attributes returns the current tag's attributes; the tag name is stored
// under the key "\x00tag".
func attributes(z *html.Tokenizer) map[string]string {
	name, more := z.TagName()
	attrs := map[string]string{"\x00tag": string(name)}
	for more {
		var k, v []byte
		k, v, more = z.TagAttr()
		attrs[string(k)] = string(v)
	}
	return attrs
}

func hasAttr(attrs map[string]string, name string) bool {
	_, ok := attrs[name]
	return ok
}
