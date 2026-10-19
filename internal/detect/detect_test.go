package detect

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "fixtures", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestChallengeFixtures(t *testing.T) {
	tests := []struct {
		file   string
		status int
		want   string
	}{
		{"cloudflare-challenge.html", 403, "Cloudflare"},
		{"akamai-denied.html", 403, "Akamai"},
		{"akamai-interstitial.html", 200, "Akamai"},
		{"datadome-block.html", 403, "DataDome"},
		{"perimeterx-challenge.html", 403, "PerimeterX"},
		{"imperva-incident.html", 200, "Imperva"},
		{"sucuri-block.html", 403, "Sucuri"},
		{"plain-page.html", 200, ""},
		// A login form with reCAPTCHA is a normal page, not a challenge.
		{"login-with-recaptcha.html", 200, ""},
		{"login-with-recaptcha.html", 403, "CAPTCHA"},
		// Akamai's page only counts as a block on 403.
		{"akamai-denied.html", 200, ""},
		// Healthy pages that load a vendor's script are not challenges (B1).
		{"healthy-cloudflare-jsd.html", 200, ""},
		{"healthy-awswaf-sdk.html", 200, ""},
		{"healthy-imperva-script.html", 200, ""},
		// The same scripts on an error or AWS's 202 challenge still count.
		{"healthy-cloudflare-jsd.html", 403, "Cloudflare"},
		{"healthy-awswaf-sdk.html", 202, "AWS WAF"},
		{"healthy-imperva-script.html", 403, "Imperva"},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			if got := Challenge(tt.status, nil, fixture(t, tt.file)); got != tt.want {
				t.Errorf("Challenge(%d, %s) = %q, want %q", tt.status, tt.file, got, tt.want)
			}
		})
	}
}

func TestChallengeHeaders(t *testing.T) {
	tests := []struct {
		name   string
		header http.Header
		want   string
	}{
		{"cloudflare mitigated", http.Header{"Cf-Mitigated": {"challenge"}}, "Cloudflare"},
		{"kasada", http.Header{"X-Kpsdk-Ct": {"abc"}}, "Kasada"},
		{"aws waf captcha", http.Header{"X-Amzn-Waf-Action": {"captcha"}}, "AWS WAF"},
		{"datadome block", http.Header{"X-Datadome": {"protected"}, "X-Dd-B": {"1"}}, "DataDome"},
		{"datadome pass", http.Header{"X-Datadome": {"protected"}}, ""},
		{"plain", http.Header{"Server": {"nginx"}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Challenge(200, tt.header, []byte("<html></html>")); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
