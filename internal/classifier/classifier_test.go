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
		{"robots token allowed", types.FetchResult{RobotsOnly: true}, Allowed},
		{"robots token disallowed", types.FetchResult{RobotsOnly: true, RobotsDisallowed: true}, RobotsDisallow},
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

func page(text string) []byte {
	return []byte("<html><body><main><p>" + text + "</p></main></body></html>")
}

const article = "Our red running shoes are made from recycled materials and ship free to every country we serve. Sizes run true to fit."

func TestClassifyAllCloaking(t *testing.T) {
	tests := []struct {
		name      string
		agentBody []byte
		status    int
		threshold float64
		want      string
	}{
		{"same content", page(article), 200, DefaultCloakThreshold, Allowed},
		{"tiny change", page(article + " Price 49."), 200, DefaultCloakThreshold, Allowed},
		{"different content", page("Bots get a stripped down page without products or prices at all, sorry."), 200, DefaultCloakThreshold, Cloaked},
		{"empty shell", []byte("<html><body><div id=root></div></body></html>"), 200, DefaultCloakThreshold, Cloaked},
		{"blocked is not cloaked", page("Access denied"), 403, DefaultCloakThreshold, Blocked},
		{"detection disabled", page("totally different words here for the bot"), 200, 0, Allowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := []types.FetchResult{
				{Profile: "Chrome (Browser)", StatusCode: 200, Body: page(article)},
				{Profile: "GPTBot", StatusCode: tt.status, Body: tt.agentBody},
			}
			ClassifyAll(rs, tt.threshold)
			if rs[0].Classification != Allowed {
				t.Fatalf("baseline classified %q", rs[0].Classification)
			}
			if rs[1].Classification != tt.want {
				t.Errorf("agent classified %q, want %q", rs[1].Classification, tt.want)
			}
		})
	}
}

func TestClassifyAllSkipsCloakingWhenBaselineFails(t *testing.T) {
	rs := []types.FetchResult{
		{Profile: "Chrome (Browser)", StatusCode: 403, Body: page("denied")},
		{Profile: "GPTBot", StatusCode: 200, Body: page(article)},
	}
	ClassifyAll(rs, DefaultCloakThreshold)
	if rs[1].Classification != Allowed || rs[1].Similarity != nil {
		t.Errorf("got %q similarity=%v, want allowed without similarity", rs[1].Classification, rs[1].Similarity)
	}
}

func TestCompareProducts(t *testing.T) {
	base := &types.Product{Source: "json-ld", Price: "98.00", Currency: "USD", Availability: "InStock"}
	rs := []types.FetchResult{
		{Profile: "Chrome", Browser: true, Classification: Allowed, Product: base},
		{Profile: "same", Classification: Allowed, Product: &types.Product{Price: "98.00", Availability: "InStock"}},
		{Profile: "missing", Classification: Allowed},
		{Profile: "price", Classification: Allowed, Product: &types.Product{Price: "89.00", Availability: "InStock"}},
		{Profile: "stock", Classification: Allowed, Product: &types.Product{Price: "98.00", Availability: "OutOfStock"}},
		{Profile: "blocked", Classification: Blocked},
	}
	CompareProducts(rs)
	want := map[string]string{
		"same":    "",
		"missing": "no product data (browser saw 98.00 USD)",
		"price":   "price 89.00 vs browser 98.00",
		"stock":   "availability OutOfStock vs browser InStock",
		"blocked": "",
	}
	for _, r := range rs[1:] {
		if r.ProductIssue != want[r.Profile] {
			t.Errorf("%s: issue = %q, want %q", r.Profile, r.ProductIssue, want[r.Profile])
		}
	}
}

func TestClassifyAllIgnoresRobotsOnlyTokensForCloaking(t *testing.T) {
	rs := []types.FetchResult{
		{Profile: "Chrome (Browser)", Browser: true, StatusCode: 200, Body: page(article)},
		{Profile: "Google-Extended", RobotsOnly: true},
		{Profile: "Applebot-Extended", RobotsOnly: true, RobotsDisallowed: true},
	}
	ClassifyAll(rs, DefaultCloakThreshold)
	if rs[1].Classification != Allowed || rs[2].Classification != RobotsDisallow || rs[1].Similarity != nil {
		t.Errorf("robots-only tokens: %q %q similarity=%v", rs[1].Classification, rs[2].Classification, rs[1].Similarity)
	}
}

func TestMergeSamplesTieGoesToTheWorseVerdict(t *testing.T) {
	s := [][]types.FetchResult{
		{{Profile: "A", Classification: Allowed}},
		{{Profile: "A", Classification: Challenged}},
	}
	got := MergeSamples(s)[0]
	if got.Classification != Challenged || !got.Flaky {
		t.Errorf("tie merged to %q flaky=%v", got.Classification, got.Flaky)
	}
}
