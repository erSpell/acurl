package sign

import (
	"crypto/ed25519"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSignerHeaders(t *testing.T) {
	s := &Signer{
		Key:   testKey(t),
		Agent: "https://bot.example",
		now:   func() time.Time { return time.Unix(1735689600, 500) },
		nonce: func() string { return "n0nce" },
	}
	h, err := s.Headers(http.MethodGet, "https://shop.example/p/1")
	if err != nil {
		t.Fatal(err)
	}
	if h["Signature-Agent"] != `sig1="https://bot.example"` {
		t.Errorf("Signature-Agent = %s", h["Signature-Agent"])
	}
	in := h["Signature-Input"]
	for _, want := range []string{`sig1=("@authority" "signature-agent";key="sig1")`, ";created=1735689600", ";expires=1735689660", `;keyid="` + s.KeyID() + `"`, `;nonce="n0nce"`, `;tag="web-bot-auth"`} {
		if !strings.Contains(in, want) {
			t.Errorf("Signature-Input %q missing %q", in, want)
		}
	}

	// A verifier reconstructing the request must accept the signature.
	req, _ := http.NewRequest(http.MethodGet, "https://shop.example/p/1", nil)
	for k, v := range h {
		req.Header.Set(k, v)
	}
	p := Params{Created: time.Unix(1735689600, 0), Expires: time.Unix(1735689660, 0), KeyID: s.KeyID(), Nonce: "n0nce", Tag: WebBotAuthTag}
	comps := []string{"@authority", `signature-agent;key="sig1"`}
	if err := Verify(req, s.Key.Public().(ed25519.PublicKey), "sig1", comps, p); err != nil {
		t.Errorf("verify: %v", err)
	}
	// The signature base matches the Web Bot Auth draft's layout.
	base, err := Base(req, comps, p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(base, "\"@authority\": shop.example\n\"signature-agent\";key=\"sig1\": \"https://bot.example\"\n") {
		t.Errorf("signature base:\n%s", base)
	}
}

func TestSignerWithoutAgent(t *testing.T) {
	h, err := (&Signer{Key: testKey(t)}).Headers(http.MethodGet, "https://shop.example/")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := h["Signature-Agent"]; ok || !strings.HasPrefix(h["Signature-Input"], `sig1=("@authority");`) || strings.Contains(h["Signature-Input"], "alg=") {
		t.Errorf("headers = %v", h)
	}
}
