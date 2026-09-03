package classifier

import (
	"testing"

	"github.com/tragent-io/acurl/pkg/types"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		r    types.FetchResult
		want string
	}{
		{"ok", types.FetchResult{StatusCode: 200}, Allowed},
		{"redirect", types.FetchResult{StatusCode: 304}, Allowed},
		{"network error", types.FetchResult{Error: "dial tcp"}, Error},
		{"no status", types.FetchResult{}, Error},
		{"challenge beats status", types.FetchResult{StatusCode: 403, BotChallenge: "Cloudflare"}, Challenged},
		{"challenge on 200", types.FetchResult{StatusCode: 200, BotChallenge: "Imperva"}, Challenged},
		{"rate limited", types.FetchResult{StatusCode: 429, RateLimited: true}, RateLimited},
		{"forbidden", types.FetchResult{StatusCode: 403}, Blocked},
		{"not found", types.FetchResult{StatusCode: 404}, Blocked},
		{"server error", types.FetchResult{StatusCode: 503}, Blocked},
		{"blocked beats robots", types.FetchResult{StatusCode: 403, RobotsDisallowed: true}, Blocked},
		{"robots only", types.FetchResult{StatusCode: 200, RobotsDisallowed: true}, RobotsDisallow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Classify(tt.r); got != tt.want {
				t.Errorf("Classify() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSummarize(t *testing.T) {
	rs := []types.FetchResult{
		{StatusCode: 200}, {StatusCode: 200}, {StatusCode: 403}, {StatusCode: 429, RateLimited: true},
		{StatusCode: 403, BotChallenge: "x"}, {StatusCode: 200, RobotsDisallowed: true}, {Error: "boom"},
	}
	ClassifyAll(rs, DefaultCloakThreshold)
	got := Summarize(rs)
	want := types.Summary{Total: 7, Allowed: 2, Blocked: 1, RateLimited: 1, Challenged: 1, RobotsDisallow: 1, Errors: 1}
	if got != want {
		t.Errorf("Summarize() = %+v, want %+v", got, want)
	}
}

func TestBaselineIndex(t *testing.T) {
	rs := []types.FetchResult{{Profile: "Googlebot"}, {Profile: "Chrome (Browser)"}}
	if i := BaselineIndex(rs); i != 1 {
		t.Errorf("BaselineIndex = %d, want 1", i)
	}
	if i := BaselineIndex(rs[:1]); i != -1 {
		t.Errorf("BaselineIndex without browser = %d, want -1", i)
	}
}
