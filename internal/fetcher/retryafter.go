package fetcher

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// maxRetryAfter caps absurd Retry-After values so they cannot overflow.
const maxRetryAfter = 365 * 24 * time.Hour

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
		if int64(secs) > int64(maxRetryAfter/time.Second) {
			return maxRetryAfter, true
		}
		return time.Duration(secs) * time.Second, true
	}
	t, err := http.ParseTime(v)
	if err != nil {
		return 0, false
	}
	if d := t.Sub(now); d > 0 {
		return min(d.Round(time.Second), maxRetryAfter), true
	}
	return 0, true
}
