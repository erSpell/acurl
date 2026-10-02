package sign

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

// RFC 9421 Appendix B.1.4 test key.
const testKeyPEM = `-----BEGIN PRIVATE KEY-----
MC4CAQAwBQYDK2VwBCIEIJ+DYvh6SEqVTm50DFtMDoQikTmiCqirVv9mWG9qfSnF
-----END PRIVATE KEY-----
`

func testKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	k, err := ParsePrivateKey([]byte(testKeyPEM))
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestParseRFCTestKey(t *testing.T) {
	pub := testKey(t).Public().(ed25519.PublicKey)
	// Public key from RFC 9421 Appendix B.1.4.
	want := "JrQLj5P/89iXES9+vFgrIy29clF9CC/oPPsw3c5D0bs="
	if got := base64.StdEncoding.EncodeToString(pub); got != want {
		t.Errorf("public key = %s, want %s", got, want)
	}
}

func TestRoundTripAndDirectory(t *testing.T) {
	k, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	pemBytes, err := EncodePrivateKey(k)
	if err != nil {
		t.Fatal(err)
	}
	back, err := ParsePrivateKey(pemBytes)
	if err != nil || !back.Equal(k) {
		t.Fatalf("round trip failed: %v", err)
	}
	pub := k.Public().(ed25519.PublicKey)
	dir, err := Directory(pub)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct{ Keys []JWK }
	if err := json.Unmarshal(dir, &parsed); err != nil || len(parsed.Keys) != 1 {
		t.Fatalf("directory = %s", dir)
	}
	if j := parsed.Keys[0]; j.Kty != "OKP" || j.Crv != "Ed25519" || j.Kid != Thumbprint(pub) || len(j.Kid) != 43 {
		t.Errorf("jwk = %+v", j)
	}
}

func TestParseErrors(t *testing.T) {
	if _, err := ParsePrivateKey([]byte("not pem")); err == nil {
		t.Error("expected error")
	}
	if _, err := ParsePrivateKey([]byte(strings.Replace(testKeyPEM, "MC4CAQAw", "AAAAAAAA", 1))); err == nil {
		t.Error("expected error for corrupt key")
	}
}
