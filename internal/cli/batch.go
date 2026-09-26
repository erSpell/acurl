package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tragent-io/acurl/internal/batch"
	"github.com/tragent-io/acurl/internal/output"
	"github.com/tragent-io/acurl/internal/robots"
	"github.com/tragent-io/acurl/pkg/acurl"
)

type batchFlags struct {
	check    checkFlags
	csv      bool
	parallel int
	sitemap  string
	limit    int
	delay    time.Duration
}

func newBatchCmd(a *app) *cobra.Command {
	var fl batchFlags
	cmd := &cobra.Command{
		Use:   "batch [file]",
		Short: "Check many URLs from a file, CSV or stdin",
		Long: `batch checks every URL in a file (one per line, # comments allowed), a CSV
file (the "url" column, or the first column), or stdin when the file is "-" or omitted.`,
		Example: `  acurl batch urls.txt
  acurl batch products.csv -f json -o report.json
  cat urls.txt | acurl batch
  acurl batch --sitemap https://shop.example/sitemap.xml --limit 20
  acurl batch --sitemap auto:shop.example`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runBatch(cmd.Context(), &fl, args)
		},
	}
	f := cmd.Flags()
	fl.check.http.register(f)
	fl.check.registerOutput(f, "table, json, markdown, csv, junit, sarif")
	f.BoolVar(&fl.csv, "csv", false, "treat the input as CSV (automatic for .csv files)")
	f.IntVarP(&fl.parallel, "parallel", "p", 2, "URLs to check at the same time")
	f.StringVar(&fl.sitemap, "sitemap", "", `read URLs from a sitemap URL, or "auto:<site>" to use the sitemaps listed in robots.txt`)
	f.IntVar(&fl.limit, "limit", 50, "maximum URLs to check (0 = no limit)")
	f.DurationVar(&fl.delay, "delay", 500*time.Millisecond, "minimum time between checks of URLs on the same host")
	fl.check.renderWait = 2 * time.Second
	return cmd
}

func (a *app) runBatch(ctx context.Context, fl *batchFlags, args []string) error {
	policy, err := output.ParsePolicy(fl.check.failOn)
	if err != nil {
		return err
	}
	opts, err := a.checkOptions(&fl.check)
	if err != nil {
		return err
	}
	c, err := acurl.New(opts)
	if err != nil {
		return err
	}
	var urls []string
	if fl.sitemap != "" {
		if len(args) > 0 {
			return fmt.Errorf("use either a file or --sitemap, not both")
		}
		urls, err = a.sitemapURLs(ctx, c, fl.sitemap, fl.limit)
	} else {
		urls, err = a.batchURLs(fl, args)
	}
	if err != nil {
		return err
	}
	if fl.limit > 0 && len(urls) > fl.limit {
		urls = urls[:fl.limit]
	}
	if len(urls) == 0 {
		return fmt.Errorf("no URLs to check")
	}

	a.logf("Checking %d URLs with %d profiles each...\n", len(urls), len(c.Profiles()))
	rep := batch.Run(ctx, c, urls, batch.Options{
		Parallel: fl.parallel,
		Delay:    fl.delay,
		Progress: func(done, total int, u string) {
			a.logf("  [%d/%d] %s\n", done, total, u)
		},
	})
	rep.Timestamp = time.Now().UTC()

	w, closeOut, err := a.outputWriter(fl.check.output)
	if err != nil {
		return err
	}
	if err := output.WriteBatch(w, fl.check.format, rep, fl.check.output == "" && !fl.check.http.noColor); err != nil {
		return err
	}
	if err := closeOut(); err != nil {
		return err
	}
	if code := output.BatchExitCode(rep, policy); code != output.ExitOK {
		return exitError(code)
	}
	return nil
}

func (a *app) batchURLs(fl *batchFlags, args []string) ([]string, error) {
	var (
		in   io.Reader = os.Stdin
		name           = "-"
	)
	if len(args) == 1 && args[0] != "-" {
		f, err := os.Open(args[0])
		if err != nil {
			return nil, err
		}
		defer f.Close()
		in, name = f, args[0]
	}
	raw, err := batch.ReadURLs(in, fl.csv || strings.HasSuffix(strings.ToLower(name), ".csv"))
	if err != nil {
		return nil, err
	}
	urls := make([]string, 0, len(raw))
	for _, u := range raw {
		n, err := normalizeURL(u)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		urls = append(urls, n)
	}
	return batch.Dedupe(urls), nil
}

// outputWriter returns stdout, or a created file when path is set.
func (a *app) outputWriter(path string) (io.Writer, func() error, error) {
	if path == "" {
		return a.stdout, func() error { return nil }, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	return f, func() error {
		if err := f.Close(); err != nil {
			return err
		}
		a.logf("Report written to %s\n", path)
		return nil
	}, nil
}

// sitemapURLs reads URLs from a sitemap, or with "auto:<site>" from every
// sitemap that the site's robots.txt lists.
func (a *app) sitemapURLs(ctx context.Context, c *acurl.Checker, spec string, limit int) ([]string, error) {
	get := func(ctx context.Context, u string) (int, []byte, error) {
		return c.Get(ctx, u, batch.MaxSitemapSize)
	}
	roots := []string{spec}
	if site, ok := strings.CutPrefix(spec, "auto:"); ok {
		u, err := normalizeURL(site)
		if err != nil {
			return nil, err
		}
		cache := robots.NewCache(func(ctx context.Context, u string) (int, []byte, error) {
			return c.Get(ctx, u, robots.MaxSize)
		})
		roots = cache.Get(ctx, u).Sitemaps()
		if len(roots) == 0 {
			return nil, fmt.Errorf("robots.txt for %s lists no sitemaps", u)
		}
	}
	var urls []string
	for _, root := range roots {
		left := 0
		if limit > 0 {
			left = limit - len(urls)
			if left <= 0 {
				break
			}
		}
		got, err := batch.SitemapURLs(ctx, get, root, left)
		if err != nil {
			return nil, err
		}
		urls = append(urls, got...)
	}
	a.logf("Found %d URLs in %d sitemap(s)\n", len(urls), len(roots))
	return batch.Dedupe(urls), nil
}
