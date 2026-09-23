package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/olekukonko/tablewriter"

	"github.com/tragent-io/acurl/pkg/types"
)

// WriteBatch renders a batch report as a table, JSON or Markdown.
func WriteBatch(w io.Writer, format string, rep *types.BatchReport, color bool) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		return enc.Encode(rep)
	case "markdown", "md":
		return writeBatchMarkdown(w, rep)
	case "table", "":
	default:
		return fmt.Errorf("unknown format %q for batch (supported: table, json, markdown)", format)
	}

	style := func(s string, st func(...string) string) string {
		if color {
			return st(s)
		}
		return s
	}
	var sb strings.Builder
	t := tablewriter.NewTable(&sb)
	t.Header("URL", "Score", "Allowed", "Blocked", "Challenged", "Cloaked", "Errors")
	for _, it := range rep.Items {
		if it.Report == nil {
			if err := t.Append(truncate(it.URL, 60), style("ERR", badStyle.Render), "-", "-", "-", "-", truncate(it.Error, 30)); err != nil {
				return err
			}
			continue
		}
		s := it.Report.Summary
		score := "-"
		if sc := it.Report.Score; sc != nil {
			score = style(fmt.Sprintf("%d %s", sc.Value, sc.Grade), gradeStyle(sc.Grade).Render)
		}
		if err := t.Append(truncate(it.URL, 60), score, strconv.Itoa(s.Allowed), strconv.Itoa(s.Blocked),
			strconv.Itoa(s.Challenged), strconv.Itoa(s.Cloaked), strconv.Itoa(s.Errors)); err != nil {
			return err
		}
	}
	if err := t.Render(); err != nil {
		return err
	}
	sm := rep.Summary
	fmt.Fprintf(&sb, "\n%d URLs checked, %d failed. Average AI readiness: %d/100\n", sm.URLs, sm.Failed, sm.AverageScore)
	_, err := io.WriteString(w, sb.String())
	return err
}

func writeBatchMarkdown(w io.Writer, rep *types.BatchReport) error {
	var sb strings.Builder
	sb.WriteString("# acurl batch report\n\n| URL | Score | Allowed | Blocked | Challenged | Cloaked | Errors |\n|---|---|---|---|---|---|---|\n")
	for _, it := range rep.Items {
		if it.Report == nil {
			fmt.Fprintf(&sb, "| %s | error | | | | | %s |\n", it.URL, mdEscape(it.Error))
			continue
		}
		s, score := it.Report.Summary, "-"
		if sc := it.Report.Score; sc != nil {
			score = fmt.Sprintf("%d (%s)", sc.Value, sc.Grade)
		}
		fmt.Fprintf(&sb, "| %s | %s | %d | %d | %d | %d | %d |\n", it.URL, score, s.Allowed, s.Blocked, s.Challenged, s.Cloaked, s.Errors)
	}
	fmt.Fprintf(&sb, "\n**%d URLs**, %d failed, average AI readiness **%d/100**.\n", rep.Summary.URLs, rep.Summary.Failed, rep.Summary.AverageScore)
	_, err := io.WriteString(w, sb.String())
	return err
}

// BatchExitCode returns the worst exit code across the batch.
func BatchExitCode(rep *types.BatchReport) int {
	code := ExitOK
	for _, it := range rep.Items {
		c := ExitError
		if it.Report != nil {
			c = ExitCode(it.Report.Summary)
		}
		code = max(code, c)
	}
	return code
}
