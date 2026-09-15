package extract

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tragent-io/acurl/pkg/types"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestProductShopify(t *testing.T) {
	p := Product(fixture(t, "product-shopify.html"))
	want := types.Product{Source: "json-ld", Name: "Wool Runner Sneakers", SKU: "WR-GREY-10", Price: "98.00", Currency: "USD", Availability: "InStock"}
	if p == nil || p.Name != want.Name || p.Price != want.Price || p.Currency != want.Currency || p.Availability != want.Availability || p.SKU != want.SKU {
		t.Errorf("Product = %+v, want %+v", p, want)
	}
}

func TestProductNone(t *testing.T) {
	for _, doc := range []string{
		"<html><body>no data</body></html>",
		`<script type="application/ld+json">{not json}</script>`,
		`<script type="application/ld+json">{"@type":"Organization","name":"X"}</script>`,
	} {
		if p := Product([]byte(doc)); p != nil {
			t.Errorf("Product(%q) = %+v, want nil", doc, p)
		}
	}
}

func TestNumericPrice(t *testing.T) {
	doc := `<script type='application/ld+json'>{"@type":"https://schema.org/Product","name":"Mat","offers":{"price":49.5,"priceCurrency":"EUR"}}</script>`
	p := Product([]byte(doc))
	if p == nil || p.Price != "49.5" {
		t.Errorf("Product = %+v", p)
	}
}

func TestProductGraphWooCommerce(t *testing.T) {
	p := Product(fixture(t, "product-woocommerce.html"))
	if p == nil || p.Name != "Premium Yoga Mat" || p.Price != "49.99" || p.Currency != "EUR" || p.Availability != "InStock" {
		t.Errorf("Product = %+v", p)
	}
}

func TestProductVariantsAndAggregates(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want types.Product
	}{
		{
			"aggregate offer range",
			`{"@type":"Product","name":"Tee","brand":"Acme","offers":{"@type":"AggregateOffer","lowPrice":"10","highPrice":"20","priceCurrency":"USD"}}`,
			types.Product{Name: "Tee", Brand: "Acme", Price: "10-20", Currency: "USD"},
		},
		{
			"price specification",
			`{"@type":"Product","name":"Mug","offers":{"@type":"Offer","priceSpecification":{"price":"7.5","priceCurrency":"GBP"},"availability":"http://schema.org/OutOfStock"}}`,
			types.Product{Name: "Mug", Price: "7.5", Currency: "GBP", Availability: "OutOfStock"},
		},
		{
			"product group variant",
			`{"@type":"ProductGroup","name":"Runner","hasVariant":[{"@type":"Product","sku":"R-9","offers":{"price":"98","priceCurrency":"USD"}}]}`,
			types.Product{Name: "Runner", SKU: "R-9", Price: "98", Currency: "USD"},
		},
		{
			"array root with rating",
			`[{"@type":"BreadcrumbList"},{"@type":["Product","Thing"],"name":"Hat","aggregateRating":{"ratingValue":4.5,"ratingCount":"12"},"offers":[{"price":"15","priceCurrency":"USD"}]}]`,
			types.Product{Name: "Hat", Price: "15", Currency: "USD", Rating: "4.5", ReviewCount: "12"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Product([]byte(`<script type="application/ld+json">` + tt.doc + `</script>`))
			if p == nil {
				t.Fatal("no product")
			}
			tt.want.Source = "json-ld"
			if *p != tt.want {
				t.Errorf("Product = %+v\nwant      %+v", *p, tt.want)
			}
		})
	}
}
