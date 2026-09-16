// Package extract pulls schema.org product data out of HTML pages, so acurl
// can show whether an agent was given the price and availability a browser
// sees.
package extract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/net/html"

	"github.com/tragent-io/acurl/pkg/types"
)

// Product returns the first product found in doc, or nil. JSON-LD is
// preferred, then microdata, then OpenGraph product tags.
func Product(doc []byte) *types.Product {
	for _, block := range jsonLDBlocks(doc) {
		if p := fromJSONLD(block); p != nil {
			return p
		}
	}
	if p := fromMicrodata(doc); p != nil {
		return p
	}
	return fromOpenGraph(doc)
}

// jsonLDBlocks returns the contents of every <script type="application/ld+json">.
func jsonLDBlocks(doc []byte) [][]byte {
	var out [][]byte
	z := html.NewTokenizer(bytes.NewReader(doc))
	inLD := false
	for {
		switch z.Next() {
		case html.ErrorToken:
			return out
		case html.StartTagToken:
			name, hasAttr := z.TagName()
			if string(name) != "script" {
				continue
			}
			for hasAttr {
				var k, v []byte
				k, v, hasAttr = z.TagAttr()
				if string(k) == "type" && strings.EqualFold(strings.TrimSpace(string(v)), "application/ld+json") {
					inLD = true
				}
			}
		case html.TextToken:
			if inLD {
				out = append(out, append([]byte(nil), z.Text()...))
			}
		case html.EndTagToken:
			inLD = false
		}
	}
}

func fromJSONLD(block []byte) *types.Product {
	var v any
	if err := json.Unmarshal(bytes.TrimSpace(block), &v); err != nil {
		return nil
	}
	return findProduct(v)
}

// findProduct walks JSON-LD looking for a Product node, descending into
// arrays and @graph.
func findProduct(v any) *types.Product {
	switch n := v.(type) {
	case []any:
		for _, item := range n {
			if p := findProduct(item); p != nil {
				return p
			}
		}
	case map[string]any:
		if isType(n, "Product", "ProductGroup", "IndividualProduct", "ProductModel") {
			return productFrom(n)
		}
		if g, ok := n["@graph"]; ok {
			return findProduct(g)
		}
	}
	return nil
}

func isType(n map[string]any, names ...string) bool {
	var ts []string
	switch t := n["@type"].(type) {
	case string:
		ts = []string{t}
	case []any:
		for _, x := range t {
			if s, ok := x.(string); ok {
				ts = append(ts, s)
			}
		}
	}
	for _, t := range ts {
		t = strings.TrimPrefix(strings.TrimPrefix(t, "http://schema.org/"), "https://schema.org/")
		for _, want := range names {
			if strings.EqualFold(t, want) {
				return true
			}
		}
	}
	return false
}

func productFrom(n map[string]any) *types.Product {
	p := &types.Product{
		Source: "json-ld",
		Name:   str(n["name"]),
		SKU:    str(n["sku"]),
		Brand:  str(n["brand"]),
	}
	if r := firstMap(n["aggregateRating"]); r != nil {
		p.Rating = str(r["ratingValue"])
		p.ReviewCount = firstNonEmpty(str(r["reviewCount"]), str(r["ratingCount"]))
	}
	offers := n["offers"]
	if offers == nil {
		// ProductGroup lists offers on its variants.
		if v := firstMap(n["hasVariant"]); v != nil {
			offers = v["offers"]
			p.SKU = firstNonEmpty(p.SKU, str(v["sku"]))
		}
	}
	if offer := firstMap(offers); offer != nil {
		applyOffer(p, offer)
	}
	return p
}

func applyOffer(p *types.Product, o map[string]any) {
	p.Currency = str(o["priceCurrency"])
	p.Availability = availability(str(o["availability"]))
	switch {
	case str(o["price"]) != "":
		p.Price = str(o["price"])
	case isType(o, "AggregateOffer") && str(o["lowPrice"]) != "":
		p.Price = str(o["lowPrice"])
		if high := str(o["highPrice"]); high != "" && high != p.Price {
			p.Price += "-" + high
		}
	default:
		if spec := firstMap(o["priceSpecification"]); spec != nil {
			p.Price = str(spec["price"])
			p.Currency = firstNonEmpty(p.Currency, str(spec["priceCurrency"]))
		}
	}
	// AggregateOffer may nest the concrete offers.
	if p.Price == "" {
		if inner := firstMap(o["offers"]); inner != nil {
			applyOffer(p, inner)
		}
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstMap(v any) map[string]any {
	switch x := v.(type) {
	case map[string]any:
		return x
	case []any:
		for _, item := range x {
			if m, ok := item.(map[string]any); ok {
				return m
			}
		}
	}
	return nil
}

// str renders a JSON scalar as a string. Numbers keep their natural form.
func str(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", x), "0"), ".")
	case map[string]any:
		return str(x["name"])
	}
	return ""
}

// availability normalises schema.org availability URLs to their short name.
func availability(v string) string {
	v = strings.TrimSpace(v)
	if i := strings.LastIndex(v, "/"); i >= 0 {
		v = v[i+1:]
	}
	return v
}
