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
	"github.com/tragent-io/acurl/pkg/acurl"
)

type batchFlags struct {
	check    checkFlags
	csv      bool
	parallel int
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
  cat urls.txt | acurl batch`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runBatch(cmd.Context(), &fl, args)
		},
	}
	f := cmd.Flags()
	fl.check.http.register(f)
	f.BoolVar(&fl.check.noRobots, "no-robots", false, "skip robots.txt checks")
	f.StringVarP(&fl.check.format, "format", "f", "table", "output format: table, json, markdown")
	f.StringVarP(&fl.check.output, "output", "o", "", "write the report to a file instead of stdout")
	f.Float64Var(&fl.check.cloakThreshold, "cloak-threshold", 0.5, "visible-text similarity below which a response counts as cloaked (0 disables)")
	f.BoolVar(&fl.csv, "csv", false, "treat the input as CSV (automatic for .csv files)")
	f.IntVarP(&fl.parallel, "parallel", "p", 2, "URLs to check at the same time")
	fl.check.renderWait = 2 * time.Second
	return cmd
}

func (a *app) runBatch(ctx context.Context, fl *batchFlags, args []string) error {
	urls, err := a.batchURLs(fl, args)
	if err != nil {
		return err
	}
	if len(urls) == 0 {
		return fmt.Errorf("no URLs to check")
	}
	opts, err := a.checkOptions(&fl.check)
	if err != nil {
		return err
	}
	c, err := acurl.New(opts)
	if err != nil {
		return err
	}

	fmt.Fprintf(a.stderr, "Checking %d URLs with %d profiles each...\n", len(urls), len(c.Profiles()))
	rep := batch.Run(ctx, c, urls, batch.Options{
		Parallel: fl.parallel,
		Progress: func(done, total int, u string) {
			fmt.Fprintf(a.stderr, "  [%d/%d] %s\n", done, total, u)
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
	if code := output.BatchExitCode(rep); code != output.ExitOK {
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
		fmt.Fprintf(a.stderr, "Report written to %s\n", path)
		return nil
	}, nil
}
