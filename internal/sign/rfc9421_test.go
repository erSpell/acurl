package sign

import (
	"net/http"
	"testing"
	"time"
)

// RFC 9421 Appendix B.2.6: signing a request using ed25519.
func TestRFC9421VectorB26(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "https://example.com/foo?param=Value&Pet=dog", nil)
	req.Header.Set("Date", "Tue, 20 Apr 2021 02:07:55 GMT")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", "18")

	comps := []string{"date", "@method", "@path", "@authority", "content-type", "content-length"}
	p := Params{Created: time.Unix(1618884473, 0), KeyID: "test-key-ed25519"}

	wantBase := `"date": Tue, 20 Apr 2021 02:07:55 GMT
"@method": POST
"@path": /foo
"@authority": example.com
"content-type": application/json
"content-length": 18
"@signature-params": ("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519"`
	base, err := Base(req, comps, p)
	if err != nil {
		t.Fatal(err)
	}
	if base != wantBase {
		t.Fatalf("signature base:\n%s\nwant:\n%s", base, wantBase)
	}

	if err := Sign(req, testKey(t), "sig-b26", comps, p); err != nil {
		t.Fatal(err)
	}
	want := "sig-b26=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:"
	if got := req.Header.Get("Signature"); got != want {
		t.Errorf("Signature = %s\nwant        %s", got, want)
	}
	wantInput := `sig-b26=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519"`
	if got := req.Header.Get("Signature-Input"); got != wantInput {
		t.Errorf("Signature-Input = %s", got)
	}
}
