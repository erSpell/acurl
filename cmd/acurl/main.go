// Command acurl shows how AI agents see a URL compared to a real browser.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	neturl "net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tragent-io/acurl/internal/classifier"
	"github.com/tragent-io/acurl/internal/fetcher"
	"github.com/tragent-io/acurl/internal/output"
	"github.com/tragent-io/acurl/internal/profiles"
	"github.com/tragent-io/acurl/internal/robots"
	"github.com/tragent-io/acurl/internal/textdiff"
	"github.com/tragent-io/acurl/pkg/types"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var (
	flagAgents         []string
	flagTimeout        time.Duration
	flagHeaders        []string
	flagConcurrency    int
	flagProfilesFile   string
	flagNoColor        bool
	flagNoRobots       bool
	flagFormat         string
	flagOutput         string
	flagMaxBody        int64
	flagCloakThreshold float64
	flagDiff           string
	flagRetries        int
	flagProxy          string
	flagInsecure       bool
	flagHTTP           string
	flagMethod         string
	flagHead           bool
)

var rootCmd = &cobra.Command{
	Use:   "acurl <url>",
	Short: "Show how AI agents see a URL compared to a real browser",
	Long: `acurl fetches a URL with multiple user agent profiles (a real browser,
AI agents, search bots) and shows how each one is treated.`,
	Version:       fmt.Sprintf("%s (commit %s, built %s)", version, commit, date),
	Args:          cobra.ExactArgs(1),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          run,
}

func init() {
	f := rootCmd.Flags()
	f.StringSliceVarP(&flagAgents, "agents", "a", nil, "only use these profiles (comma-separated)")
	f.DurationVarP(&flagTimeout, "timeout", "t", 30*time.Second, "per-request timeout")
	f.StringArrayVarP(&flagHeaders, "header", "H", nil, `extra header "Name: value" (repeatable)`)
	f.IntVarP(&flagConcurrency, "concurrency", "c", 5, "maximum concurrent requests")
	f.StringVar(&flagProfilesFile, "profiles", "", "custom profiles YAML file")
	f.BoolVar(&flagNoColor, "no-color", false, "disable colored output")
	f.BoolVar(&flagNoRobots, "no-robots", false, "skip robots.txt checks")
	f.StringVarP(&flagFormat, "format", "f", "table", "output format: table, json, markdown")
	f.StringVarP(&flagOutput, "output", "o", "", "write the report to a file instead of stdout")
	f.Float64Var(&flagCloakThreshold, "cloak-threshold", classifier.DefaultCloakThreshold, "visible-text similarity below which a response counts as cloaked (0 disables)")
	f.StringVar(&flagDiff, "diff", "", "print a visible-text diff between the browser baseline and this profile")
	f.IntVar(&flagRetries, "retries", 1, "retries after network or gateway errors (exponential backoff with jitter)")
	f.StringVar(&flagProxy, "proxy", "", "proxy URL (http://, https:// or socks5://)")
	f.BoolVarP(&flagInsecure, "insecure", "k", false, "skip TLS certificate verification")
	f.StringVar(&flagHTTP, "http", "", `force HTTP version: "1.1" or "2"`)
	f.StringVarP(&flagMethod, "request", "X", "GET", "HTTP method to use")
	f.BoolVarP(&flagHead, "head", "I", false, "send HEAD requests (status and headers only)")
	f.Int64Var(&flagMaxBody, "max-body", fetcher.DefaultMaxBody, "maximum response body bytes to read per profile")
}

func run(cmd *cobra.Command, args []string) error {
	url, err := normalizeURL(args[0])
	if err != nil {
		return err
	}

	all, err := profiles.Load(flagProfilesFile)
	if err != nil {
		return err
	}
	selected := profiles.Filter(all, flagAgents)
	if len(selected) == 0 {
		return fmt.Errorf("no profiles match %v", flagAgents)
	}
	headers, err := parseHeaders(flagHeaders)
	if err != nil {
		return err
	}

	f, err := fetcher.New(fetcher.Options{
		Timeout:     flagTimeout,
		Headers:     headers,
		MaxBody:     flagMaxBody,
		Retries:     flagRetries,
		Proxy:       flagProxy,
		Insecure:    flagInsecure,
		HTTPVersion: flagHTTP,
		Method:      method(),
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Fetching %s with %d profiles...\n", url, len(selected))
	results := f.FetchAll(cmd.Context(), url, selected, flagConcurrency)
	if !flagNoRobots {
		applyRobots(cmd.Context(), f, url, selected, results)
	}

	classifier.ClassifyAll(results, flagCloakThreshold)
	report := types.ComparisonResult{
		URL:       url,
		Timestamp: time.Now().UTC(),
		Results:   results,
		Summary:   classifier.Summarize(results),
	}
	if err := writeReport(report); err != nil {
		return err
	}
	if flagDiff != "" {
		if err := printDiff(results, flagDiff); err != nil {
			return err
		}
	}
	if code := output.ExitCode(report.Summary); code != output.ExitOK {
		return exitError(code)
	}
	return nil
}

// printDiff writes the visible-text diff between the baseline and the named
// profile to stderr, so machine-readable stdout stays clean.
func printDiff(results []types.FetchResult, name string) error {
	b := classifier.BaselineIndex(results)
	if b < 0 {
		return fmt.Errorf("--diff needs the browser baseline profile in the run")
	}
	for _, r := range results {
		if strings.EqualFold(r.Profile, name) {
			lines := textdiff.Lines(textdiff.VisibleText(results[b].Body), textdiff.VisibleText(r.Body), 3)
			fmt.Fprintln(os.Stderr)
			return output.WriteDiff(os.Stderr, results[b].Profile, r.Profile, lines, !flagNoColor)
		}
	}
	return fmt.Errorf("--diff: profile %q was not part of this run", name)
}

// exitError carries a non-zero exit code without an error message.
type exitError int

func (e exitError) Error() string { return fmt.Sprintf("exit status %d", int(e)) }

func writeReport(report types.ComparisonResult) error {
	if flagOutput == "" {
		return output.Write(os.Stdout, flagFormat, report, !flagNoColor)
	}
	out, err := os.Create(flagOutput)
	if err != nil {
		return err
	}
	defer out.Close()
	if err := output.Write(out, flagFormat, report, false); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Report written to %s\n", flagOutput)
	return out.Close()
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

func method() string {
	if flagHead {
		return http.MethodHead
	}
	return flagMethod
}

// userAgent identifies acurl itself when fetching auxiliary files.
func userAgent() string {
	return "acurl/" + version + " (+https://github.com/tragent-io/acurl)"
}

func applyRobots(ctx context.Context, f *fetcher.Fetcher, url string, ps []types.Profile, results []types.FetchResult) {
	cache := robots.NewCache(func(ctx context.Context, u string) (int, []byte, error) {
		return f.Get(ctx, u, userAgent(), robots.MaxSize)
	})
	data := cache.Get(ctx, url)
	for i, p := range ps {
		if p.RobotsKey == "" {
			continue
		}
		if data.Unreachable() {
			results[i].RobotsError = data.Err.Error()
		}
		results[i].RobotsDisallowed = !data.Allowed(p.RobotsKey, url)
	}
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

func main() {
	err := rootCmd.ExecuteContext(context.Background())
	var code exitError
	switch {
	case err == nil:
	case errors.As(err, &code):
		os.Exit(int(code))
	default:
		fmt.Fprintln(os.Stderr, "acurl:", err)
		os.Exit(output.ExitError)
	}
}
