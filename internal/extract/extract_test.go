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

func TestProductMicrodata(t *testing.T) {
	doc := `<html><body>
<div itemscope itemtype="https://schema.org/Product">
  <h1 itemprop="name">Trail Runner</h1>
  <meta itemprop="sku" content="TR-1">
  <div itemprop="brand" itemscope itemtype="https://schema.org/Brand"><span itemprop="name">Acme</span></div>
  <div itemprop="offers" itemscope itemtype="https://schema.org/Offer">
    <span itemprop="priceCurrency" content="USD">$</span><span itemprop="price" content="120.00">120</span>
    <link itemprop="availability" href="https://schema.org/InStock">
  </div>
</div>
<p itemprop="name">outside the scope</p>
</body></html>`
	p := Product([]byte(doc))
	want := types.Product{Source: "microdata", Name: "Trail Runner", SKU: "TR-1", Price: "120.00", Currency: "USD", Availability: "InStock"}
	if p == nil || p.Name != want.Name || p.SKU != want.SKU || p.Price != want.Price || p.Currency != want.Currency || p.Availability != want.Availability {
		t.Errorf("Product = %+v, want %+v", p, want)
	}
}

func TestProductOpenGraph(t *testing.T) {
	doc := `<html><head>
<meta property="og:type" content="product">
<meta property="og:title" content="Canvas Tote">
<meta property="product:price:amount" content="25.00">
<meta property="product:price:currency" content="CAD">
<meta property="product:availability" content="in stock">
</head><body></body></html>`
	p := Product([]byte(doc))
	if p == nil || p.Source != "opengraph" || p.Name != "Canvas Tote" || p.Price != "25.00" || p.Currency != "CAD" || p.Availability != "in stock" {
		t.Errorf("Product = %+v", p)
	}
	if Product([]byte(`<meta property="og:type" content="website"><meta property="og:title" content="Home">`)) != nil {
		t.Error("non-product OpenGraph page returned a product")
	}
}
