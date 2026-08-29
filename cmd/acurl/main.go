// Command acurl shows how AI agents see a URL compared to a real browser.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tragent-io/acurl/internal/classifier"
	"github.com/tragent-io/acurl/internal/fetcher"
	"github.com/tragent-io/acurl/internal/output"
	"github.com/tragent-io/acurl/internal/profiles"
	"github.com/tragent-io/acurl/internal/robots"
	"github.com/tragent-io/acurl/pkg/types"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var (
	flagAgents       []string
	flagTimeout      time.Duration
	flagHeaders      []string
	flagConcurrency  int
	flagProfilesFile string
	flagNoColor      bool
	flagNoRobots     bool
	flagFormat       string
	flagOutput       string
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
}

func run(cmd *cobra.Command, args []string) error {
	url := args[0]

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

	f := fetcher.New(fetcher.Options{Timeout: flagTimeout, Headers: headers})
	fmt.Fprintf(os.Stderr, "Fetching %s with %d profiles...\n", url, len(selected))
	results := f.FetchAll(cmd.Context(), url, selected, flagConcurrency)
	if !flagNoRobots {
		applyRobots(cmd.Context(), f, url, selected, results)
	}

	classifier.ClassifyAll(results)
	report := types.ComparisonResult{
		URL:       url,
		Timestamp: time.Now().UTC(),
		Results:   results,
		Summary:   classifier.Summarize(results),
	}
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
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "acurl:", err)
		os.Exit(2)
	}
}
