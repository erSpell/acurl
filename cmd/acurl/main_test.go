package main

import "testing"

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		in, want string
		wantErr  bool
	}{
		{"example.com", "https://example.com", false},
		{"  https://shop.example/p?id=1 ", "https://shop.example/p?id=1", false},
		{"http://localhost:8080/", "http://localhost:8080/", false},
		{"ftp://example.com", "", true},
		{"https://", "", true},
	}
	for _, tt := range tests {
		got, err := normalizeURL(tt.in)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("normalizeURL(%q) = %q, %v; want %q, err=%v", tt.in, got, err, tt.want, tt.wantErr)
		}
	}
}

func TestParseHeaders(t *testing.T) {
	h, err := parseHeaders([]string{"Cookie: a=b", "X-Empty:"})
	if err != nil || h["Cookie"] != "a=b" || h["X-Empty"] != "" {
		t.Errorf("parseHeaders = %v, %v", h, err)
	}
	if _, err := parseHeaders([]string{"no-colon"}); err == nil {
		t.Error("expected error for header without colon")
	}
}
