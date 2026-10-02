package sign

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strconv"
	"time"
)

// WebBotAuthTag is the signature tag Web Bot Auth verifiers look for.
const WebBotAuthTag = "web-bot-auth"

// Signer signs requests the way Web Bot Auth agents do: an Ed25519
// signature over @authority and Signature-Agent, with a short validity
// window, a nonce and the web-bot-auth tag.
type Signer struct {
	Key ed25519.PrivateKey
	// Agent is the Signature-Agent URL where the key directory is published,
	// e.g. "https://bot.example". Empty omits the header.
	Agent string
	// Validity is how long a signature is valid. Default one minute.
	Validity time.Duration
	// now and nonce are replaceable in tests.
	now   func() time.Time
	nonce func() string
}

// KeyID returns the JWK thumbprint used as keyid.
func (s *Signer) KeyID() string {
	return Thumbprint(s.Key.Public().(ed25519.PublicKey))
}

// Headers returns the Signature-Agent, Signature-Input and Signature
// headers for a request.
func (s *Signer) Headers(method, rawURL string) (map[string]string, error) {
	req, err := http.NewRequest(method, rawURL, nil)
	if err != nil {
		return nil, err
	}
	components := []string{"@authority"}
	if s.Agent != "" {
		req.Header.Set("Signature-Agent", strconv.Quote(s.Agent))
		components = append(components, "signature-agent")
	}
	now := time.Now
	if s.now != nil {
		now = s.now
	}
	nonce := randomNonce
	if s.nonce != nil {
		nonce = s.nonce
	}
	validity := s.Validity
	if validity <= 0 {
		validity = time.Minute
	}
	created := now().Truncate(time.Second)
	p := Params{
		Created: created,
		Expires: created.Add(validity),
		KeyID:   s.KeyID(),
		Alg:     "ed25519",
		Nonce:   nonce(),
		Tag:     WebBotAuthTag,
	}
	if err := Sign(req, s.Key, "sig1", components, p); err != nil {
		return nil, err
	}
	out := map[string]string{
		"Signature-Input": req.Header.Get("Signature-Input"),
		"Signature":       req.Header.Get("Signature"),
	}
	if s.Agent != "" {
		out["Signature-Agent"] = req.Header.Get("Signature-Agent")
	}
	return out, nil
}

func randomNonce() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawStdEncoding.EncodeToString(b)
}
