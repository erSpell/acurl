// Package types holds the data structures shared across acurl packages.
package types

import "time"

// Profile categories.
const (
	CategoryBrowser     = "browser"
	CategoryAIAssistant = "ai-assistant" // fetches a page because a user asked
	CategoryAISearch    = "ai-search"    // builds an AI search index
	CategoryAITraining  = "ai-training"  // collects training data
	CategorySearch      = "search"       // classic search engine crawler
	CategoryOther       = "other"
)

// Profile is a user agent profile used to fetch a URL.
type Profile struct {
	Name      string            `json:"name" yaml:"name"`
	UserAgent string            `json:"user_agent" yaml:"user_agent"`
	Headers   map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	RobotsKey string            `json:"robots_key" yaml:"robots_key"`
	Operator  string            `json:"operator,omitempty" yaml:"operator,omitempty"`
	Category  string            `json:"category,omitempty" yaml:"category,omitempty"`
	Docs      string            `json:"docs,omitempty" yaml:"docs,omitempty"`
	// Browser marks a real-browser profile. It is fetched with a full Chrome
	// impersonation and used as the baseline for comparisons.
	Browser bool `json:"browser,omitempty" yaml:"browser,omitempty"`
	// RobotsOnly marks a robots.txt product token that has no user agent of
	// its own (for example Google-Extended). It is checked, never fetched.
	RobotsOnly bool `json:"robots_only,omitempty" yaml:"robots_only,omitempty"`
}

// FetchResult is the outcome of fetching a URL with one profile.
type FetchResult struct {
	Profile           string        `json:"profile"`
	Browser           bool          `json:"browser,omitempty"`
	RobotsOnly        bool          `json:"robots_only,omitempty"`
	Success           bool          `json:"success"`
	StatusCode        int           `json:"status_code"`
	RedirectChain     []string      `json:"redirect_chain,omitempty"`
	FinalURL          string        `json:"final_url,omitempty"`
	Duration          time.Duration `json:"-"`
	DurationMs        int64         `json:"duration_ms"`
	BodySize          int           `json:"body_size"`
	Truncated         bool          `json:"truncated,omitempty"`
	Title             string        `json:"title,omitempty"`
	BotChallenge      string        `json:"bot_challenge,omitempty"`
	RateLimited       bool          `json:"rate_limited"`
	RetryAfter        string        `json:"retry_after,omitempty"`
	RetryAfterSeconds int           `json:"retry_after_seconds,omitempty"`
	RobotsDisallowed  bool          `json:"robots_disallowed"`
	RobotsError       string        `json:"robots_error,omitempty"`
	Directives        []string      `json:"directives,omitempty"` // X-Robots-Tag and meta robots for this agent
	Similarity        *float64      `json:"similarity,omitempty"` // visible-text similarity to the browser baseline
	Classification    string        `json:"classification"`
	Error             string        `json:"error,omitempty"`
	ContentHash       string        `json:"content_hash,omitempty"`

	// Body is kept in memory for comparisons and never serialised.
	Body []byte `json:"-"`
}

// SetDuration records d on the result in both forms.
func (r *FetchResult) SetDuration(d time.Duration) {
	r.Duration = d
	r.DurationMs = d.Milliseconds()
}

// ComparisonResult is the full report for one URL.
type ComparisonResult struct {
	URL       string        `json:"url"`
	Timestamp time.Time     `json:"timestamp"`
	Results   []FetchResult `json:"results"`
	Summary   Summary       `json:"summary"`
	Caveat    string        `json:"caveat"`
}

// Summary holds aggregate counts per classification.
type Summary struct {
	Total          int `json:"total"`
	Allowed        int `json:"allowed"`
	Blocked        int `json:"blocked"`
	Challenged     int `json:"challenged"`
	RateLimited    int `json:"rate_limited"`
	Cloaked        int `json:"cloaked"`
	RobotsDisallow int `json:"robots_disallowed"`
	Errors         int `json:"errors"`
}

// RobotsAudit is the result of `acurl robots`.
type RobotsAudit struct {
	URL       string        `json:"url"`
	RobotsURL string        `json:"robots_url"`
	Status    int           `json:"status"`
	Error     string        `json:"error,omitempty"`
	Sitemaps  []string      `json:"sitemaps,omitempty"`
	Agents    []RobotsAgent `json:"agents"`
	LLMsTxt   []LLMsTxt     `json:"llms_txt,omitempty"`
}

// RobotsAgent is one agent's robots.txt verdict for a URL.
type RobotsAgent struct {
	Profile  string `json:"profile"`
	Token    string `json:"token"`
	Category string `json:"category,omitempty"`
	Allowed  bool   `json:"allowed"`
	// Signals are Content-Signal directives such as ai-train=no.
	Signals map[string]string `json:"content_signals,omitempty"`
}

// LLMsTxt describes one llms.txt or llms-full.txt file.
type LLMsTxt struct {
	URL      string   `json:"url"`
	Status   int      `json:"status"`
	Found    bool     `json:"found"`
	Size     int      `json:"size,omitempty"`
	Title    string   `json:"title,omitempty"`
	Summary  string   `json:"summary,omitempty"`
	Sections int      `json:"sections,omitempty"`
	Links    int      `json:"links,omitempty"`
	Problems []string `json:"problems,omitempty"`
	Error    string   `json:"error,omitempty"`
}
