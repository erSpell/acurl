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
