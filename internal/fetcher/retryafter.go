package fetcher

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// parseRetryAfter converts a Retry-After header (RFC 9110 §10.2.3), given
// either as delay-seconds or as an HTTP-date, into a duration from now.
// It returns false when the header is missing or malformed.
func parseRetryAfter(v string, now time.Time) (time.Duration, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0, false
		}
		return time.Duration(secs) * time.Second, true
	}
	t, err := http.ParseTime(v)
	if err != nil {
		return 0, false
	}
	if d := t.Sub(now); d > 0 {
		return d.Round(time.Second), true
	}
	return 0, true
}
