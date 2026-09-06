package fetcher

import (
	"testing"
	"time"
)

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		in     string
		want   time.Duration
		wantOK bool
	}{
		{"120", 2 * time.Minute, true},
		{" 0 ", 0, true},
		{"Sun, 06 Sep 2026 12:05:00 GMT", 5 * time.Minute, true},
		{"Sun, 06 Sep 2026 11:00:00 GMT", 0, true},
		{"-5", 0, false},
		{"soon", 0, false},
		{"", 0, false},
	}
	for _, tt := range tests {
		got, ok := parseRetryAfter(tt.in, now)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("parseRetryAfter(%q) = %v, %v; want %v, %v", tt.in, got, ok, tt.want, tt.wantOK)
		}
	}
}
