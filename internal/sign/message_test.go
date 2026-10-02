package sign

import (
	"crypto/ed25519"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSignVerifyRoundTrip(t *testing.T) {
	key := testKey(t)
	req, _ := http.NewRequest(http.MethodGet, "https://Shop.Example:443/products/1?color=red", nil)
	req.Header.Set("Signature-Agent", `"https://bot.example"`)
	p := Params{Created: time.Unix(1735689600, 0), Expires: time.Unix(1735689660, 0), KeyID: "kid-1", Alg: "ed25519", Tag: "web-bot-auth"}
	comps := []string{"@authority", "signature-agent"}

	if err := Sign(req, key, "sig1", comps, p); err != nil {
		t.Fatal(err)
	}
	wantInput := `sig1=("@authority" "signature-agent");created=1735689600;expires=1735689660;keyid="kid-1";alg="ed25519";tag="web-bot-auth"`
	if got := req.Header.Get("Signature-Input"); got != wantInput {
		t.Errorf("Signature-Input = %s\nwant            %s", got, wantInput)
	}
	if err := Verify(req, key.Public().(ed25519.PublicKey), "sig1", comps, p); err != nil {
		t.Errorf("Verify: %v", err)
	}
	base, _ := Base(req, comps, p)
	if !strings.HasPrefix(base, "\"@authority\": shop.example\n\"signature-agent\": \"https://bot.example\"\n") {
		t.Errorf("base = %q", base)
	}

	req.Header.Set("Signature-Agent", `"https://evil.example"`)
	if err := Verify(req, key.Public().(ed25519.PublicKey), "sig1", comps, p); err == nil {
		t.Error("tampered header should not verify")
	}
}

func TestComponentValues(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "http://example.com:8080/a%20b?x=1", nil)
	for comp, want := range map[string]string{
		"@method": "POST", "@authority": "example.com:8080", "@scheme": "http",
		"@path": "/a%20b", "@query": "?x=1", "@request-target": "/a%20b?x=1",
	} {
		if got, err := componentValue(req, comp); err != nil || got != want {
			t.Errorf("%s = %q, %v; want %q", comp, got, err, want)
		}
	}
	if _, err := componentValue(req, "x-missing"); err == nil {
		t.Error("missing header should error")
	}
}
