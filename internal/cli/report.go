package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/tragent-io/acurl/internal/output"
	"github.com/tragent-io/acurl/pkg/types"
)

func newReportCmd(a *app) *cobra.Command {
	var format, out string
	var noColor bool
	cmd := &cobra.Command{
		Use:   "report <report.json>",
		Short: "Render a saved JSON report in another format",
		Long: `report reads a JSON report written by "acurl -f json" or "acurl batch -f json"
and renders it as table, markdown, csv, junit, sarif or html, without fetching anything.
Use "-" to read from stdin.`,
		Example: `  acurl -f json -o run.json https://shop.example
  acurl report run.json -f html -o run.html`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := readInput(args[0])
			if err != nil {
				return err
			}
			batch, err := parseReport(data)
			if err != nil {
				return err
			}
			w, closeOut, err := a.outputWriter(out)
			if err != nil {
				return err
			}
			if len(batch.Items) == 1 && batch.Items[0].Report != nil && format != "json" {
				err = output.Write(w, format, *batch.Items[0].Report, out == "" && !noColor)
			} else {
				err = output.WriteBatch(w, format, batch, out == "" && !noColor)
			}
			if err != nil {
				return err
			}
			return closeOut()
		},
	}
	cmd.Flags().StringVarP(&format, "format", "f", "html", "output format: table, json, markdown, csv, junit, sarif, html")
	cmd.Flags().StringVarP(&out, "output", "o", "", "write to a file instead of stdout")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "disable colored output")
	return cmd
}

func readInput(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}

// parseReport accepts a single-URL report or a batch report and returns
// it as a batch.
func parseReport(data []byte) (*types.BatchReport, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("not an acurl JSON report: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if _, ok := probe["items"]; ok {
		var b types.BatchReport
		if err := dec.Decode(&b); err != nil {
			return nil, err
		}
		return &b, nil
	}
	if _, ok := probe["results"]; !ok {
		return nil, fmt.Errorf("not an acurl JSON report: no results or items")
	}
	var r types.ComparisonResult
	if err := dec.Decode(&r); err != nil {
		return nil, err
	}
	// JSON drops in-memory durations; restore them for display.
	for i := range r.Results {
		r.Results[i].SetDuration(time.Duration(r.Results[i].DurationMs) * time.Millisecond)
	}
	return &types.BatchReport{Timestamp: r.Timestamp, Items: []types.BatchItem{{URL: r.URL, Report: &r}}}, nil
}
