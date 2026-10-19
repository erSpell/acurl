package sign

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Params are the signature parameters of RFC 9421 §2.3.
type Params struct {
	Created time.Time
	Expires time.Time // zero means none
	KeyID   string
	Alg     string // e.g. "ed25519"; empty omits the parameter
	Nonce   string
	Tag     string
}

// serialize renders the inner list with parameters, in the order the
// components were given and the parameters below.
func serialize(components []string, p Params) string {
	var sb strings.Builder
	sb.WriteByte('(')
	for i, c := range components {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(identifier(c))
	}
	sb.WriteByte(')')
	if !p.Created.IsZero() {
		fmt.Fprintf(&sb, ";created=%d", p.Created.Unix())
	}
	if !p.Expires.IsZero() {
		fmt.Fprintf(&sb, ";expires=%d", p.Expires.Unix())
	}
	if p.KeyID != "" {
		fmt.Fprintf(&sb, ";keyid=%q", p.KeyID)
	}
	if p.Alg != "" {
		fmt.Fprintf(&sb, ";alg=%q", p.Alg)
	}
	if p.Nonce != "" {
		fmt.Fprintf(&sb, ";nonce=%q", p.Nonce)
	}
	if p.Tag != "" {
		fmt.Fprintf(&sb, ";tag=%q", p.Tag)
	}
	return sb.String()
}

// A component may carry parameters after a semicolon, as in
// `signature-agent;key="sig1"`, which names one member of a dictionary
// header (RFC 9421 §2.1.2).

// splitComponent separates a component name from its parameters.
func splitComponent(c string) (name, params string) {
	name, params, _ = strings.Cut(c, ";")
	return name, params
}

// identifier serialises a component as it appears in the signature base
// and Signature-Input: the quoted name followed by its parameters.
func identifier(c string) string {
	name, params := splitComponent(c)
	if params == "" {
		return strconv.Quote(name)
	}
	return strconv.Quote(name) + ";" + params
}

// dictionaryMember returns the serialised value of member key in a
// structured-field dictionary header such as `sig1="https://a.example"`.
func dictionaryMember(header, key string) (string, bool) {
	for _, member := range strings.Split(header, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(member), "=")
		if ok && k == key {
			return strings.TrimSpace(v), true
		}
	}
	return "", false
}

// componentValue returns the value of a covered component for req
// (RFC 9421 §2.1 and §2.2).
func componentValue(req *http.Request, component string) (string, error) {
	name, params := splitComponent(component)
	if key, ok := strings.CutPrefix(params, "key="); ok {
		key = strings.Trim(key, `"`)
		v, found := dictionaryMember(req.Header.Get(name), key)
		if !found {
			return "", fmt.Errorf("sign: header %q has no member %q", name, key)
		}
		return v, nil
	}
	u := req.URL
	switch name {
	case "@method":
		return strings.ToUpper(req.Method), nil
	case "@authority":
		return authority(u, req.Host), nil
	case "@scheme":
		return strings.ToLower(u.Scheme), nil
	case "@target-uri":
		return u.String(), nil
	case "@path":
		if p := u.EscapedPath(); p != "" {
			return p, nil
		}
		return "/", nil
	case "@query":
		return "?" + u.RawQuery, nil
	case "@request-target":
		return u.RequestURI(), nil
	}
	if strings.HasPrefix(name, "@") {
		return "", fmt.Errorf("sign: unsupported derived component %q", name)
	}
	vals := req.Header.Values(name)
	if len(vals) == 0 {
		return "", fmt.Errorf("sign: header %q is not set", name)
	}
	for i, v := range vals {
		vals[i] = strings.TrimSpace(v)
	}
	return strings.Join(vals, ", "), nil
}

// authority is the lower-cased host with any non-default port.
func authority(u *url.URL, host string) string {
	if host == "" {
		host = u.Host
	}
	host = strings.ToLower(host)
	if h, port, ok := strings.Cut(host, ":"); ok && !strings.Contains(port, ":") {
		if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
			return h
		}
	}
	return host
}

// Base builds the signature base for req (RFC 9421 §2.5).
func Base(req *http.Request, components []string, p Params) (string, error) {
	var sb strings.Builder
	for _, c := range components {
		v, err := componentValue(req, c)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&sb, "%s: %s\n", identifier(c), v)
	}
	fmt.Fprintf(&sb, "%q: %s", "@signature-params", serialize(components, p))
	return sb.String(), nil
}

// Sign signs req and sets the Signature-Input and Signature headers under
// label (for example "sig1").
func Sign(req *http.Request, key ed25519.PrivateKey, label string, components []string, p Params) error {
	base, err := Base(req, components, p)
	if err != nil {
		return err
	}
	sig := ed25519.Sign(key, []byte(base))
	req.Header.Set("Signature-Input", label+"="+serialize(components, p))
	req.Header.Set("Signature", label+"=:"+base64.StdEncoding.EncodeToString(sig)+":")
	return nil
}

// Verify checks the signature under label on req against pub. It is used
// by tests and by `acurl serve` to show what a server would see.
func Verify(req *http.Request, pub ed25519.PublicKey, label string, components []string, p Params) error {
	base, err := Base(req, components, p)
	if err != nil {
		return err
	}
	raw := req.Header.Get("Signature")
	prefix := label + "=:"
	if !strings.HasPrefix(raw, prefix) || !strings.HasSuffix(raw, ":") {
		return fmt.Errorf("sign: no signature %q", label)
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSuffix(strings.TrimPrefix(raw, prefix), ":"))
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}
	if !ed25519.Verify(pub, []byte(base), sig) {
		return fmt.Errorf("sign: signature does not verify")
	}
	return nil
}
