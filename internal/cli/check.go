package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/tragent-io/acurl/internal/classifier"
	"github.com/tragent-io/acurl/internal/output"
	"github.com/tragent-io/acurl/internal/textdiff"
	"github.com/tragent-io/acurl/pkg/acurl"
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
	failOn         string
}

func (fl *checkFlags) registerOutput(f *pflag.FlagSet, formats string) {
	f.BoolVar(&fl.noRobots, "no-robots", false, "skip robots.txt checks")
	f.StringVarP(&fl.format, "format", "f", "table", "output format: "+formats)
	f.StringVarP(&fl.output, "output", "o", "", "write the report to a file instead of stdout")
	f.Float64Var(&fl.cloakThreshold, "cloak-threshold", classifier.DefaultCloakThreshold, "visible-text similarity below which a response counts as cloaked (0 disables)")
	f.StringVar(&fl.failOn, "fail-on", output.DefaultFailOn, "exit 1 when any of these occur: blocked, challenged, rate-limited, cloaked, robots, error, score<N, or none")
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
	fl.registerOutput(f, "table, json, markdown, csv, junit, sarif")
	f.StringVar(&fl.diff, "diff", "", "print a visible-text diff between the browser baseline and this profile")
	f.BoolVar(&fl.render, "render", false, "also render the page in headless Chrome to measure how much text needs JavaScript")
	f.DurationVar(&fl.renderWait, "render-wait", 2*time.Second, "how long scripts may run before the rendered page is captured")
	return cmd
}

// checkOptions builds checker options from the check flags.
func (a *app) checkOptions(fl *checkFlags) (acurl.Options, error) {
	opts, err := fl.http.options(a)
	if err != nil {
		return opts, err
	}
	opts.SkipRobots = fl.noRobots
	opts.CloakThreshold = fl.cloakThreshold
	if opts.CloakThreshold == 0 {
		opts.CloakThreshold = -1 // the flag uses 0 for "disabled"
	}
	opts.Render = fl.render
	opts.RenderWait = fl.renderWait
	return opts, nil
}

func (a *app) runCheck(ctx context.Context, fl *checkFlags, rawURL string) error {
	url, err := normalizeURL(rawURL)
	if err != nil {
		return err
	}
	policy, err := output.ParsePolicy(fl.failOn)
	if err != nil {
		return err
	}
	opts, err := a.checkOptions(fl)
	if err != nil {
		return err
	}
	c, err := acurl.New(opts)
	if err != nil {
		return err
	}

	a.logf("Fetching %s with %d profiles...\n", url, len(c.Profiles()))
	if fl.render {
		a.logf("Rendering in headless Chrome...\n")
	}
	report, err := c.Check(ctx, url)
	if err != nil {
		return err
	}
	if err := a.writeReport(*report, fl.format, fl.output, !fl.http.noColor); err != nil {
		return err
	}
	if fl.diff != "" {
		if err := a.printDiff(report.Results, fl.diff, !fl.http.noColor); err != nil {
			return err
		}
	}
	if code := policy.ExitCode(report); code != output.ExitOK {
		fmt.Fprintf(a.stderr, "acurl: failing (--fail-on %s): %s\n", fl.failOn, strings.Join(policy.Reasons(report), ", "))
		return exitError(code)
	}
	return nil
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
	a.logf("Report written to %s\n", path)
	return out.Close()
}
