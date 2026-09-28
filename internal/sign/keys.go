// Package sign implements HTTP Message Signatures (RFC 9421) with Ed25519,
// as used by the Web Bot Auth proposal for cryptographically identifying
// automated clients.
package sign

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

// GenerateKey returns a new Ed25519 private key.
func GenerateKey() (ed25519.PrivateKey, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	return priv, err
}

// EncodePrivateKey returns key as a PKCS #8 PEM block.
func EncodePrivateKey(key ed25519.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

// ParsePrivateKey parses a PKCS #8 PEM Ed25519 private key.
func ParsePrivateKey(data []byte) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("sign: no PEM block found")
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}
	priv, ok := k.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("sign: key is %T, want Ed25519", k)
	}
	return priv, nil
}

// LoadPrivateKey reads a PEM private key file.
func LoadPrivateKey(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}
	return ParsePrivateKey(data)
}

// JWK is an Ed25519 public key in JSON Web Key form (RFC 8037).
type JWK struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Kid string `json:"kid,omitempty"`
}

// PublicJWK returns the JWK for pub with its thumbprint as kid.
func PublicJWK(pub ed25519.PublicKey) JWK {
	return JWK{Kty: "OKP", Crv: "Ed25519", X: base64.RawURLEncoding.EncodeToString(pub), Kid: Thumbprint(pub)}
}

// Thumbprint returns the RFC 7638 JWK SHA-256 thumbprint of pub,
// base64url-encoded. Web Bot Auth uses it as the signature keyid.
func Thumbprint(pub ed25519.PublicKey) string {
	// Members in lexicographic order, no whitespace (RFC 7638 §3).
	canonical := `{"crv":"Ed25519","kty":"OKP","x":"` + base64.RawURLEncoding.EncodeToString(pub) + `"}`
	sum := sha256.Sum256([]byte(canonical))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// Directory returns the JSON key directory a signing agent publishes at
// /.well-known/http-message-signatures-directory.
func Directory(pubs ...ed25519.PublicKey) ([]byte, error) {
	dir := struct {
		Keys []JWK `json:"keys"`
	}{}
	for _, p := range pubs {
		dir.Keys = append(dir.Keys, PublicJWK(p))
	}
	return json.MarshalIndent(dir, "", "  ")
}
