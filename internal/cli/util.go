package cli

import (
	"fmt"
	neturl "net/url"
	"strings"
)

func errNoProfiles(names []string) error {
	return fmt.Errorf("no profiles match %v (see the profile list in the README)", names)
}

// normalizeURL adds https:// when no scheme is given and rejects anything
// that is not an absolute http(s) URL.
func normalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := neturl.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("unsupported scheme %q: only http and https are supported", u.Scheme)
	}
	if u.Host == "" {
		return "", fmt.Errorf("invalid url %q: missing host", raw)
	}
	return u.String(), nil
}

func parseHeaders(raw []string) (map[string]string, error) {
	out := make(map[string]string, len(raw))
	for _, h := range raw {
		name, value, ok := strings.Cut(h, ":")
		if !ok || strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("invalid header %q, want \"Name: value\"", h)
		}
		out[strings.TrimSpace(name)] = strings.TrimSpace(value)
	}
	return out, nil
}
