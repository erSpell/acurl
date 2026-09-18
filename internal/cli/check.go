package cli

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
	"github.com/tragent-io/acurl/internal/render"
	"github.com/tragent-io/acurl/internal/robots"
	"github.com/tragent-io/acurl/internal/textdiff"
	"github.com/tragent-io/acurl/pkg/types"
)

type checkFlags struct {
	http           httpFlags
	noRobots       bool
	format         string
	output         string
	cloakThreshold float64
	diff           string
	render         bool
	renderWait     time.Duration
}

func newCheckCmd(a *app) *cobra.Command {
	var fl checkFlags
	cmd := &cobra.Command{
		Use:   "acurl <url>",
		Short: "Show how AI agents see a URL compared to a real browser",
		Long: `acurl fetches a URL with multiple user agent profiles (a real browser,
AI agents, search bots) and shows how each one is treated.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runCheck(cmd.Context(), &fl, args[0])
		},
	}
	f := cmd.Flags()
	fl.http.register(f)
	f.BoolVar(&fl.noRobots, "no-robots", false, "skip robots.txt checks")
	f.StringVarP(&fl.format, "format", "f", "table", "output format: table, json, markdown")
	f.StringVarP(&fl.output, "output", "o", "", "write the report to a file instead of stdout")
	f.Float64Var(&fl.cloakThreshold, "cloak-threshold", classifier.DefaultCloakThreshold, "visible-text similarity below which a response counts as cloaked (0 disables)")
	f.StringVar(&fl.diff, "diff", "", "print a visible-text diff between the browser baseline and this profile")
	f.BoolVar(&fl.render, "render", false, "also render the page in headless Chrome to measure how much text needs JavaScript")
	f.DurationVar(&fl.renderWait, "render-wait", 2*time.Second, "how long scripts may run before the rendered page is captured")
	return cmd
}

func (a *app) runCheck(ctx context.Context, fl *checkFlags, rawURL string) error {
	url, err := normalizeURL(rawURL)
	if err != nil {
		return err
	}
	selected, err := fl.http.loadProfiles()
	if err != nil {
		return err
	}
	if fl.noRobots {
		selected = profiles.WithoutRobotsOnly(selected)
	}
	f, err := fl.http.newFetcher()
	if err != nil {
		return err
	}

	fmt.Fprintf(a.stderr, "Fetching %s with %d profiles...\n", url, len(selected))
	results := f.FetchAll(ctx, url, selected, fl.http.concurrency)
	if !fl.noRobots {
		a.applyRobots(ctx, f, url, selected, results)
	}
	classifier.ClassifyAll(results, fl.cloakThreshold)
	classifier.CompareProducts(results)

	report := types.ComparisonResult{
		URL:       url,
		Timestamp: time.Now().UTC(),
		Results:   results,
		Summary:   classifier.Summarize(results),
	}
	if fl.render {
		report.Render = a.renderBaseline(ctx, url, results, fl)
	}
	if err := a.writeReport(report, fl.format, fl.output, !fl.http.noColor); err != nil {
		return err
	}
	if fl.diff != "" {
		if err := a.printDiff(results, fl.diff, !fl.http.noColor); err != nil {
			return err
		}
	}
	if code := output.ExitCode(report.Summary); code != output.ExitOK {
		return exitError(code)
	}
	return nil
}

func (a *app) applyRobots(ctx context.Context, f *fetcher.Fetcher, url string, ps []types.Profile, results []types.FetchResult) {
	cache := robots.NewCache(func(ctx context.Context, u string) (int, []byte, error) {
		return f.Get(ctx, u, a.userAgent(), robots.MaxSize)
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

// printDiff writes the visible-text diff between the baseline and the named
// profile to stderr, so machine-readable stdout stays clean.
func (a *app) printDiff(results []types.FetchResult, name string, color bool) error {
	b := classifier.BaselineIndex(results)
	if b < 0 {
		return fmt.Errorf("--diff needs the browser baseline profile in the run")
	}
	for _, r := range results {
		if strings.EqualFold(r.Profile, name) {
			lines := textdiff.Lines(textdiff.VisibleText(results[b].Body), textdiff.VisibleText(r.Body), 3)
			fmt.Fprintln(a.stderr)
			return output.WriteDiff(a.stderr, results[b].Profile, r.Profile, lines, color)
		}
	}
	return fmt.Errorf("--diff: profile %q was not part of this run", name)
}

func (a *app) writeReport(report types.ComparisonResult, format, path string, color bool) error {
	if path == "" {
		return output.Write(a.stdout, format, report, color)
	}
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	if err := output.Write(out, format, report, false); err != nil {
		return err
	}
	fmt.Fprintf(a.stderr, "Report written to %s\n", path)
	return out.Close()
}

// renderBaseline renders url in headless Chrome and compares it with the
// raw HTML the browser profile received.
func (a *app) renderBaseline(ctx context.Context, url string, results []types.FetchResult, fl *checkFlags) *types.RenderResult {
	b := classifier.BaselineIndex(results)
	if b < 0 || len(results[b].Body) == 0 {
		return &types.RenderResult{Error: "--render needs a successful browser baseline"}
	}
	fmt.Fprintln(a.stderr, "Rendering in headless Chrome...")
	html, err := render.Headless(ctx, url, render.Options{Timeout: fl.http.timeout + fl.renderWait, Wait: fl.renderWait})
	if err != nil {
		return &types.RenderResult{Error: err.Error()}
	}
	return render.Compare(results[b].Body, html)
}
